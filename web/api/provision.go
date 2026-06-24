package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/middleware"
	"github.com/SkyPanel/SkyPanel/v3/models"
	"github.com/SkyPanel/SkyPanel/v3/response"
	"github.com/SkyPanel/SkyPanel/v3/scopes"
	"github.com/SkyPanel/SkyPanel/v3/services"
	"github.com/SkyPanel/SkyPanel/v3/utils"
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

func registerProvision(g *gin.RouterGroup) {
	g.Handle("GET", "/ping", provisionPing)
	g.Handle("POST", "/provision", provisionServer)
	g.Handle("POST", "/terminate", provisionTerminate)
	g.Handle("POST", "/suspend", provisionSuspend)
	g.Handle("POST", "/unsuspend", provisionUnsuspend)
}

func provisionPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Pong"})
}

type ProvisionRequest struct {
	ProductID  string `json:"product_id" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	ServerName string `json:"server_name"`
	ServiceID  string `json:"service_id"`
}

// ProvisionActionRequest accepts either service_id (from Paymenter) or server_id (legacy)
type ProvisionActionRequest struct {
	ServerID  string `json:"server_id"`
	ServiceID string `json:"service_id"`
}

func provisionServer(c *gin.Context) {
	db := middleware.GetDatabase(c)

	var req ProvisionRequest
	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// 1. Find Product
	var product models.ProvisionProduct
	if err := db.Where("product_id = ?", req.ProductID).First(&product).Error; response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// 2. Find or create user
	us := &services.User{DB: db}
	user, err := us.GetByEmail(req.Email)
	generatedPassword := req.Password

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// New user — generate credentials
		if generatedPassword == "" {
			generatedPassword, _ = utils.GenerateRandomString(12)
		}
		username := req.Username
		if username == "" {
			randStr, _ := utils.GenerateRandomString(6)
			username = "user_" + strings.ReplaceAll(randStr, "-", "")
		}
		user = &models.User{
			Email:    req.Email,
			Username: username,
		}
		user.SetPassword(generatedPassword)
		if err := us.Create(user); response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	} else if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	} else {
		// User already exists — reset their password so Paymenter can show it
		generatedPassword, _ = utils.GenerateRandomString(12)
		user.SetPassword(generatedPassword)
		if err := us.Update(user); response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	}

	// 3. Find Template
	ts := &services.Template{DB: db}
	repos, err := ts.GetRepos()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	var template *models.Template
	for _, repo := range repos {
		tmpl, err := ts.Get(repo.ID, product.Template)
		if err == nil && tmpl != nil {
			template = tmpl
			break
		}
	}
	if template == nil {
		response.HandleError(c, errors.New("template not found"), http.StatusBadRequest)
		return
	}

	// 4. Determine Node
	nodeID := product.DefaultNode
	if product.NodeID != nil && *product.NodeID != 0 {
		nodeID = *product.NodeID
	}

	ns := &services.Node{DB: db}
	node, err := ns.Get(nodeID)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// 5. Create Server Models
	uid, _ := uuid.NewV4()
	serverID := uid.String()[:8]

	serverName := req.ServerName
	if serverName == "" {
		serverName = product.DisplayName
	}

	server := &models.Server{
		Name:        serverName,
		Identifier:  serverID,
		NodeID:      node.ID,
		IP:          "0.0.0.0",
		Port:        0,
		Type:        template.Server.Type.Type,
		Icon:        "",
		TotalCPU:    product.CPU,
		TotalMemory: product.Memory,
		TotalDisk:   product.Disk,
		ExternalID:  req.ServiceID,
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(server).Error
	})
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// 6. Grant Permissions
	ps := &services.Permission{DB: db}
	perm, err := ps.GetForUserAndServer(user.ID, server.Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	perm.Scopes = []*scopes.Scope{
		scopes.ScopeServerView,
		scopes.ScopeServerViewData,
		scopes.ScopeServerEditData,
		scopes.ScopeServerEditFlags,
		scopes.ScopeServerEditName,
		scopes.ScopeServerClientView,
		scopes.ScopeServerClientEdit,
		scopes.ScopeServerClientCreate,
		scopes.ScopeServerClientDelete,
		scopes.ScopeServerUserView,
		scopes.ScopeServerUserCreate,
		scopes.ScopeServerUserEdit,
		scopes.ScopeServerUserDelete,
		scopes.ScopeServerTaskView,
		scopes.ScopeServerTaskRun,
		scopes.ScopeServerTaskCreate,
		scopes.ScopeServerTaskDelete,
		scopes.ScopeServerReload,
		scopes.ScopeServerStart,
		scopes.ScopeServerStop,
		scopes.ScopeServerKill,
		scopes.ScopeServerInstall,
		scopes.ScopeServerFileView,
		scopes.ScopeServerFileEdit,
		scopes.ScopeServerSftp,
		scopes.ScopeServerConsole,
		scopes.ScopeServerSendCommand,
		scopes.ScopeServerStats,
		scopes.ScopeServerStatus,
	}

	if err := ps.UpdatePermissions(perm); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// 7. Call Daemon
	serverCreation := &models.ServerCreation{
		Server:    template.Server,
		NodeId:    node.ID,
		Name:      server.Name,
		Users:     []string{user.Username},
	}
	serverCreation.Identifier = server.Identifier

	reader := &bytes.Buffer{}
	json.NewEncoder(reader).Encode(&serverCreation)

	nodeResponse, err := ns.CallNode(node, "PUT", "/daemon/server/"+server.Identifier, io.NopCloser(reader), c.Request.Header)
	defer utils.CloseResponse(nodeResponse)

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if nodeResponse.StatusCode != http.StatusOK {
		resData, _ := io.ReadAll(nodeResponse.Body)
		c.AbortWithStatusJSON(nodeResponse.StatusCode, gin.H{"error": "Daemon error", "details": string(resData)})
		return
	}

	// Send Email
	es := services.GetEmailService()
	_ = es.SendEmail(user.Email, "addedToServer", map[string]interface{}{
		"Server":        server,
		"RegisterToken": "",
	}, true)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"server_id": serverID,
		"username":  user.Username,
		"password":  generatedPassword, // Will be empty if not generated here
	})
}

// getServerByRequest finds a server using either service_id (Paymenter) or server_id (legacy)
func getServerByRequest(db *gorm.DB, req ProvisionActionRequest, ss *services.Server) (*models.Server, error) {
	if req.ServiceID != "" {
		var server models.Server
		result := db.Preload("Node").Where("external_id = ?", req.ServiceID).First(&server)
		if result.Error != nil {
			return nil, result.Error
		}
		return &server, nil
	}
	if req.ServerID != "" {
		return ss.Get(req.ServerID)
	}
	return nil, errors.New("either service_id or server_id is required")
}

func provisionTerminate(c *gin.Context) {
	db := middleware.GetDatabase(c)
	var req ProvisionActionRequest
	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	ss := &services.Server{DB: db}
	server, err := getServerByRequest(db, req, ss)
	if response.HandleError(c, err, http.StatusInternalServerError) || server == nil {
		return
	}

	ns := &services.Node{DB: db}
	node := &server.Node

	// Try to stop first
	statusRes, err := ns.CallNode(node, "GET", "/daemon/server/"+server.Identifier+"/status", nil, nil)
	if err == nil && statusRes.StatusCode == http.StatusOK {
		var statusData struct {
			Running bool `json:"running"`
		}
		if err := json.NewDecoder(statusRes.Body).Decode(&statusData); err == nil && statusData.Running {
			stopRes, err := ns.CallNode(node, "POST", "/daemon/server/"+server.Identifier+"/stop?wait=true", nil, nil)
			if err == nil && stopRes != nil && stopRes.Body != nil {
				stopRes.Body.Close()
			}
		}
		if statusRes.Body != nil {
			statusRes.Body.Close()
		}
	}

	nodeRes, err := ns.CallNode(node, "DELETE", "/daemon/server/"+server.Identifier, nil, nil)
	if err == nil && nodeRes != nil && nodeRes.Body != nil {
		nodeRes.Body.Close()
	}

	err = ss.Delete(server.Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func provisionSuspend(c *gin.Context) {
	db := middleware.GetDatabase(c)
	var req ProvisionActionRequest
	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	ss := &services.Server{DB: db}
	server, err := getServerByRequest(db, req, ss)
	if response.HandleError(c, err, http.StatusInternalServerError) || server == nil {
		return
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		server.Suspended = true
		if err := tx.Save(server).Error; err != nil {
			return err
		}

		var children []models.Server
		if err := tx.Where("parent_server_id = ?", server.Identifier).Find(&children).Error; err != nil {
			return err
		}

		for i := range children {
			children[i].Suspended = true
			if err := tx.Save(&children[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	ns := &services.Node{DB: db}
	node := &server.Node
	stopRes, err := ns.CallNode(node, "POST", "/daemon/server/"+server.Identifier+"/stop", nil, nil)
	if err == nil && stopRes != nil && stopRes.Body != nil {
		stopRes.Body.Close()
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func provisionUnsuspend(c *gin.Context) {
	db := middleware.GetDatabase(c)
	var req ProvisionActionRequest
	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	ss := &services.Server{DB: db}
	server, err := getServerByRequest(db, req, ss)
	if response.HandleError(c, err, http.StatusInternalServerError) || server == nil {
		return
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		server.Suspended = false
		if err := tx.Save(server).Error; err != nil {
			return err
		}

		var children []models.Server
		if err := tx.Where("parent_server_id = ?", server.Identifier).Find(&children).Error; err != nil {
			return err
		}

		for i := range children {
			children[i].Suspended = false
			if err := tx.Save(&children[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
