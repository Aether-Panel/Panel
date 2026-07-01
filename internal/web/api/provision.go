package api

import (
	"bytes"
	cryptoRand "crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	skypanel "github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
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

// pickFreePort returns a random available port that is not already assigned
// to any server on the given node.
// If min/max are both 0 (no range configured), picks from the full user port range (1024–65535).
// If min/max are set, restricts to that range.
func pickFreePort(db *gorm.DB, nodeID uint, portMin, portMax uint16) uint16 {
	// Default to full user port range if no range configured
	if portMin == 0 || portMax == 0 {
		portMin = 1024
		portMax = 65535
	}

	if portMin > portMax {
		return 0
	}

	// Fetch ports already in use on this node
	var usedPorts []uint16
	db.Model(&models.Server{}).
		Where("node_id = ? AND port > 0", nodeID).
		Pluck("port", &usedPorts)

	used := make(map[uint16]bool, len(usedPorts))
	for _, p := range usedPorts {
		used[p] = true
	}

	rangeSize := int(portMax-portMin) + 1

	// For large ranges, use random sampling instead of building a full list
	if rangeSize > 10000 {
		maxAttempts := 100
		for i := 0; i < maxAttempts; i++ {
			randBig, err := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(rangeSize)))
			if err != nil {
				return 0
			}
			candidate := portMin + uint16(randBig.Int64())
			if !used[candidate] {
				return candidate
			}
		}
		return 0
	}

	// For small/medium ranges build the full free list and pick at random
	free := make([]uint16, 0, rangeSize)
	for p := portMin; p <= portMax; p++ {
		if !used[p] {
			free = append(free, p)
		}
	}

	if len(free) == 0 {
		return 0
	}

	randBig, err := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(free))))
	if err != nil {
		return 0
	}
	return free[randBig.Int64()]
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
	ps := &services.Permission{DB: db}
	user, err := us.GetByEmail(req.Email)
	generatedPassword := req.Password
	isNewUser := false

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// New user — generate credentials
		isNewUser = true
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
		_ = user.SetPassword(generatedPassword)
		if err := us.Create(user); response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	case response.HandleError(c, err, http.StatusInternalServerError):
		return
	default:
		// User already exists — reset their password so Paymenter can show it
		generatedPassword, _ = utils.GenerateRandomString(12)
		_ = user.SetPassword(generatedPassword)
		if err := us.Update(user); response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	}

	// Grant global login scope to new users and assign default "Usuario" role
	if isNewUser {
		// Assign the "Usuario" role so they get all standard scopes automatically
		// (templates.view, uptime.view, server.view, etc.)
		rs := &services.Role{DB: db}
		userRole, roleErr := rs.GetByName("Usuario")
		if roleErr == nil && userRole != nil {
			user.RoleID = &userRole.ID
			if err := us.Update(user); response.HandleError(c, err, http.StatusInternalServerError) {
				return
			}
		}

		// Always ensure login scope is set directly as well
		globalPerms, err := ps.GetForUserAndServer(user.ID, "")
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		globalPerms.Scopes = scopes.AddScope(globalPerms.Scopes, scopes.ScopeLogin)
		if err := ps.UpdatePermissions(globalPerms); response.HandleError(c, err, http.StatusInternalServerError) {
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

	// Assign a port from the product's port range (0 if no range configured)
	assignedPort := pickFreePort(db, node.ID, product.PortRangeMin, product.PortRangeMax)

	server := &models.Server{
		Name:        serverName,
		Identifier:  serverID,
		NodeID:      node.ID,
		IP:          "0.0.0.0",
		Port:        assignedPort,
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
		Server: template.Server,
		NodeID: node.ID,
		Name:   server.Name,
		Users:  []string{user.Username},
	}
	serverCreation.Identifier = server.Identifier

	// Inject the assigned port into the template variables so the server starts on it
	if assignedPort > 0 {
		if serverCreation.Server.Variables == nil {
			serverCreation.Server.Variables = make(map[string]skypanel.Variable)
		}
		// Always override the port variable with the assigned port
		existing := serverCreation.Server.Variables["port"]
		existing.Value = assignedPort
		serverCreation.Server.Variables["port"] = existing

		// Also update the server record in DB so the panel shows the correct port
		db.Model(&models.Server{}).
			Where("identifier = ?", server.Identifier).
			Update("port", assignedPort)
	}

	reader := &bytes.Buffer{}
	_ = json.NewEncoder(reader).Encode(&serverCreation)

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
		"password":  generatedPassword,
		"port":      assignedPort,
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

	// terminateOne stops and deletes a single server from daemon + DB
	terminateOne := func(srv *models.Server) {
		node := &srv.Node

		// Stop if running
		statusRes, err := ns.CallNode(node, "GET", "/daemon/server/"+srv.Identifier+"/status", nil, nil)
		if err == nil && statusRes != nil && statusRes.StatusCode == http.StatusOK {
			var statusData struct {
				Running bool `json:"running"`
			}
			if err := json.NewDecoder(statusRes.Body).Decode(&statusData); err == nil && statusData.Running {
				stopRes, err := ns.CallNode(node, "POST", "/daemon/server/"+srv.Identifier+"/stop?wait=true", nil, nil)
				if err == nil && stopRes != nil && stopRes.Body != nil {
					stopRes.Body.Close()
				}
			}
			statusRes.Body.Close()
		}

		// Delete from daemon
		nodeRes, err := ns.CallNode(node, "DELETE", "/daemon/server/"+srv.Identifier, nil, nil)
		if err == nil && nodeRes != nil && nodeRes.Body != nil {
			nodeRes.Body.Close()
		}

		// Delete from DB
		_ = ss.Delete(srv.Identifier)
	}

	// 1. Find and terminate all children first (avoids FK constraint failure)
	var children []models.Server
	if err := db.Preload("Node").Where("parent_server_id = ?", server.Identifier).Find(&children).Error; err == nil {
		for i := range children {
			terminateOne(&children[i])
		}
	}

	// 2. Now terminate the parent
	terminateOne(server)

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
