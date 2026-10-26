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

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	skypanel "github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerProvision(g *gin.RouterGroup) {
	g.Handle("GET", "/ping", provisionPing)
	g.Handle("POST", "/provision", provisionServer)
	g.Handle("POST", "/terminate", provisionTerminate)
	g.Handle("POST", "/suspend", provisionSuspend)
	g.Handle("POST", "/unsuspend", provisionUnsuspend)
	g.Handle("POST", "/change-package", provisionChangePackage)
	g.Handle("POST", "/sso", provisionSSO)
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

	// 0. Idempotency: ExternalID is the billing provider's identifier for the
	// service. Never provision the same service twice — if a server already
	// carries that ExternalID, return it instead of creating a duplicate.
	if req.ServiceID != "" && answerAlreadyProvisioned(c, db, req) {
		return
	}

	// 1. Find Product
	var product models.ProvisionProduct
	if err := db.Where("product_id = ?", req.ProductID).First(&product).Error; response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// 2. Find or create user
	ps := &services.Permission{DB: db}
	user, generatedPassword, ok := provisionAccount(c, db, ps, req)
	if !ok {
		return
	}

	// 3. Find Template
	ts := &services.Template{DB: db}
	repos, err := ts.GetRepos()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	template := findTemplateInRepos(ts, repos, product.Template)
	if template == nil {
		response.HandleError(c, errors.New("template not found"), http.StatusBadRequest)
		return
	}

	// 4. Determine Node
	ns := &services.Node{DB: db}
	node, err := ns.Get(provisionNodeID(&product))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// 5. Create Server Models
	serverID, err := models.GenerateServerIdentifier()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

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
	if err := grantProvisionedServerScopes(ps, user.ID, server.Identifier); response.HandleError(c, err, http.StatusInternalServerError) {
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
	injectAssignedPort(db, serverCreation, server.Identifier, assignedPort)
	// and the plan's limits, or the container would run with the template's defaults
	injectResourceLimits(serverCreation, product.CPU, product.Memory, product.Disk)

	reader := &bytes.Buffer{}
	_ = json.NewEncoder(reader).Encode(&serverCreation)

	nodeResponse, err := ns.CallNode(node, "PUT", daemonServerPrefix+server.Identifier, io.NopCloser(reader), c.Request.Header)
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

// answerAlreadyProvisioned answers with the server that already carries the
// request's service id; true when a response has been written.
func answerAlreadyProvisioned(c *gin.Context, db *gorm.DB, req ProvisionRequest) bool {
	var existing models.Server
	err := db.Where("external_id = ?", req.ServiceID).First(&existing).Error
	switch {
	case err == nil:
		// Keep the same response shape as a successful provisioning so
		// billing integrations don't receive missing/misparsed fields. The
		// generated password is only available on the first call, so it is
		// intentionally returned empty on repeated (idempotent) calls.
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"server_id": existing.Identifier,
			"username":  req.Username,
			"password":  "",
			"port":      existing.Port,
		})
		return true
	case !errors.Is(err, gorm.ErrRecordNotFound):
		response.HandleError(c, err, http.StatusInternalServerError)
		return true
	}
	return false
}

// provisionAccount finds the customer's account by email or creates it, and
// returns it with the password to hand back to the billing panel. It writes
// the error response itself and returns false when the request must stop.
func provisionAccount(c *gin.Context, db *gorm.DB, ps *services.Permission, req ProvisionRequest) (*models.User, string, bool) {
	us := &services.User{DB: db}
	user, err := us.GetByEmail(req.Email)

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// New user — generate credentials
		created, generatedPassword, ok := createProvisionedUser(c, us, req)
		if !ok || !setupNewProvisionedUser(c, db, us, ps, created) {
			return nil, "", false
		}
		return created, generatedPassword, true
	case response.HandleError(c, err, http.StatusInternalServerError):
		return nil, "", false
	default:
		generatedPassword, ok := resetProvisionedUserPassword(c, db, us, user)
		return user, generatedPassword, ok
	}
}

// createProvisionedUser creates the account for a new customer, generating
// the username and password the request leaves empty.
func createProvisionedUser(c *gin.Context, us *services.User, req ProvisionRequest) (*models.User, string, bool) {
	generatedPassword := req.Password
	if generatedPassword == "" {
		generatedPassword, _ = utils.GenerateRandomString(12)
	}
	username := req.Username
	if username == "" {
		randStr, _ := utils.GenerateRandomString(6)
		username = "user_" + strings.ReplaceAll(randStr, "-", "")
	}
	user := &models.User{
		Email:    req.Email,
		Username: username,
	}
	_ = user.SetPassword(generatedPassword)
	if err := us.Create(user); response.HandleError(c, err, http.StatusInternalServerError) {
		return nil, "", false
	}
	return user, generatedPassword, true
}

// resetProvisionedUserPassword gives an existing (non-admin) customer a fresh
// password so the billing panel can show it.
func resetProvisionedUserPassword(c *gin.Context, db *gorm.DB, us *services.User, user *models.User) (string, bool) {
	// An API key must never be able to take over an administrator's account
	isAdmin, err := userIsAdmin(db, user)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return "", false
	}
	if isAdmin {
		response.HandleError(c, errors.New("this email belongs to an administrator and cannot be provisioned"), http.StatusConflict)
		return "", false
	}

	// User already exists — reset their password so Paymenter can show it
	generatedPassword, _ := utils.GenerateRandomString(12)
	_ = user.SetPassword(generatedPassword)
	if err := us.Update(user); response.HandleError(c, err, http.StatusInternalServerError) {
		return "", false
	}
	return generatedPassword, true
}

// setupNewProvisionedUser grants a new customer the global login scope and
// assigns the default "Usuario" role.
func setupNewProvisionedUser(c *gin.Context, db *gorm.DB, us *services.User, ps *services.Permission, user *models.User) bool {
	// Assign the "Usuario" role so they get all standard scopes automatically
	// (templates.view, uptime.view, server.view, etc.)
	rs := &services.Role{DB: db}
	userRole, roleErr := rs.GetByName("Usuario")
	if roleErr == nil && userRole != nil {
		user.RoleID = &userRole.ID
		if err := us.Update(user); response.HandleError(c, err, http.StatusInternalServerError) {
			return false
		}
	}

	// Always ensure login scope is set directly as well
	globalPerms, err := ps.GetForUserAndServer(user.ID, "")
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return false
	}
	globalPerms.Scopes = scopes.AddScope(globalPerms.Scopes, scopes.ScopeLogin)
	if err := ps.UpdatePermissions(globalPerms); response.HandleError(c, err, http.StatusInternalServerError) {
		return false
	}
	return true
}

// findTemplateInRepos returns the first repo's template with the given name,
// or nil when no repo has it.
func findTemplateInRepos(ts *services.Template, repos []*models.TemplateRepo, name string) *models.Template {
	for _, repo := range repos {
		tmpl, err := ts.Get(repo.ID, name)
		if err == nil && tmpl != nil {
			return tmpl
		}
	}
	return nil
}

// provisionNodeID is the product's pinned node, or its fallback node.
func provisionNodeID(product *models.ProvisionProduct) uint {
	if product.NodeID != nil && *product.NodeID != 0 {
		return *product.NodeID
	}
	return product.DefaultNode
}

// injectAssignedPort makes the server start on the port picked from the
// product's range and stores that port on the server record as well.
func injectAssignedPort(db *gorm.DB, serverCreation *models.ServerCreation, identifier string, assignedPort uint16) {
	if assignedPort == 0 {
		return
	}
	if serverCreation.Server.Variables == nil {
		serverCreation.Server.Variables = make(map[string]skypanel.Variable)
	}
	// Always override the port variable with the assigned port
	existing := serverCreation.Server.Variables["port"]
	existing.Value = assignedPort
	serverCreation.Server.Variables["port"] = existing

	// Also update the server record in DB so the panel shows the correct port
	db.Model(&models.Server{}).
		Where("identifier = ?", identifier).
		Update("port", assignedPort)
}

// injectResourceLimits sets the cpu, memory and disk variables the daemon builds the
// container's limits from. A zero leaves the template's own value.
func injectResourceLimits(serverCreation *models.ServerCreation, cpu int, memory, disk int64) {
	if serverCreation.Server.Variables == nil {
		serverCreation.Server.Variables = make(map[string]skypanel.Variable)
	}
	for name, value := range resourceLimitData(cpu, memory, disk) {
		v := serverCreation.Server.Variables[name]
		v.Value = value
		serverCreation.Server.Variables[name] = v
	}
}

// resourceLimitData is the variable map for the limits that are set (above zero)
func resourceLimitData(cpu int, memory, disk int64) map[string]interface{} {
	data := map[string]interface{}{}
	if cpu > 0 {
		data["cpu"] = cpu
	}
	if memory > 0 {
		data["memory"] = memory
	}
	if disk > 0 {
		data["disk"] = disk
	}
	return data
}

// grantProvisionedServerScopes gives the customer the standard owner scopes
// on their new server.
func grantProvisionedServerScopes(ps *services.Permission, userID uint, identifier string) error {
	perm, err := ps.GetForUserAndServer(userID, identifier)
	if err != nil {
		return err
	}

	perm.Scopes = []*scopes.Scope{
		scopes.ScopeServerView,
		scopes.ScopeServerViewData,
		scopes.ScopeServerEditData,
		scopes.ScopeServerViewFlags,
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
		scopes.ScopeServerTaskEdit,
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
	return ps.UpdatePermissions(perm)
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

	// 1. Find and terminate all children first (avoids FK constraint failure)
	var children []models.Server
	if err := db.Preload("Node").Where(parentServerIDQuery, server.Identifier).Find(&children).Error; err == nil {
		for i := range children {
			terminateProvisionedServer(ns, ss, &children[i])
		}
	}

	// 2. Now terminate the parent
	terminateProvisionedServer(ns, ss, server)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// terminateProvisionedServer stops and deletes a single server from daemon + DB
func terminateProvisionedServer(ns *services.Node, ss *services.Server, srv *models.Server) {
	node := &srv.Node

	// Stop if running
	stopServerIfRunning(ns, node, srv.Identifier)

	// Delete from daemon
	nodeRes, err := ns.CallNode(node, "DELETE", daemonServerPrefix+srv.Identifier, nil, nil)
	if err == nil && nodeRes != nil && nodeRes.Body != nil {
		nodeRes.Body.Close()
	}

	// Delete from DB
	_ = ss.Delete(srv.Identifier)
}

// stopServerIfRunning asks the daemon for the server's status and, when it is
// running, stops it and waits for it to finish. Failures are ignored, the
// server is deleted either way.
func stopServerIfRunning(ns *services.Node, node *models.Node, identifier string) {
	statusRes, err := ns.CallNode(node, "GET", daemonServerPrefix+identifier+"/status", nil, nil)
	if err != nil || statusRes == nil || statusRes.StatusCode != http.StatusOK {
		return
	}
	var statusData struct {
		Running bool `json:"running"`
	}
	if err := json.NewDecoder(statusRes.Body).Decode(&statusData); err == nil && statusData.Running {
		stopRes, err := ns.CallNode(node, "POST", daemonServerPrefix+identifier+"/stop?wait=true", nil, nil)
		if err == nil && stopRes != nil && stopRes.Body != nil {
			stopRes.Body.Close()
		}
	}
	statusRes.Body.Close()
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
		if err := tx.Where(parentServerIDQuery, server.Identifier).Find(&children).Error; err != nil {
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
	stopRes, err := ns.CallNode(node, "POST", daemonServerPrefix+server.Identifier+"/stop", nil, nil)
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
		if err := tx.Where(parentServerIDQuery, server.Identifier).Find(&children).Error; err != nil {
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

// ChangePackageRequest names the service and either the product it moves to or the new
// limits themselves (cpu in %, memory and disk in MB).
type ChangePackageRequest struct {
	ProvisionActionRequest
	ProductID string `json:"product_id"`
	CPU       int    `json:"cpu"`
	Memory    int64  `json:"memory"`
	Disk      int64  `json:"disk"`
}

// @Summary Change a service's plan
// @Description Moves a provisioned server to another product's limits, or to the cpu, memory
// @Description and disk given. The server keeps its files; a running server gets the new
// @Description CPU and memory on its next restart (restart_required), the disk limit counts at once.
// @Param request body ChangePackageRequest true "Service and its new plan"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Tags Provision
// @Router /api/v1/change-package [post]
func provisionChangePackage(c *gin.Context) {
	db := middleware.GetDatabase(c)
	var req ChangePackageRequest
	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	cpu, memory, disk := req.CPU, req.Memory, req.Disk
	if req.ProductID != "" {
		var product models.ProvisionProduct
		if err := db.Where("product_id = ?", req.ProductID).First(&product).Error; response.HandleError(c, err, http.StatusBadRequest) {
			return
		}
		cpu, memory, disk = product.CPU, product.Memory, product.Disk
	}
	if cpu < 0 || memory < 0 || disk < 0 {
		response.HandleError(c, errors.New("resources cannot be negative"), http.StatusBadRequest)
		return
	}
	data := resourceLimitData(cpu, memory, disk)
	if len(data) == 0 {
		response.HandleError(c, errors.New("give a product_id or at least one of cpu, memory and disk"), http.StatusBadRequest)
		return
	}

	ss := &services.Server{DB: db}
	server, err := getServerByRequest(db, req.ProvisionActionRequest, ss)
	if response.HandleError(c, err, http.StatusNotFound) {
		return
	}

	// the node first: if it refuses, the panel must not show limits the server does not have
	body, _ := json.Marshal(data)
	ns := &services.Node{DB: db}
	nodeResponse, err := ns.CallNode(&server.Node, "PUT", daemonServerPrefix+server.Identifier+"/data", io.NopCloser(bytes.NewReader(body)), nil)
	defer utils.CloseResponse(nodeResponse)
	if response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	if nodeResponse.StatusCode != http.StatusNoContent && nodeResponse.StatusCode != http.StatusOK {
		details, _ := io.ReadAll(nodeResponse.Body)
		c.AbortWithStatusJSON(nodeResponse.StatusCode, gin.H{"error": "Daemon error", "details": string(details)})
		return
	}

	updates := map[string]interface{}{}
	if cpu > 0 {
		updates["total_cpu"] = cpu
	}
	if memory > 0 {
		updates["total_memory"] = memory
	}
	if disk > 0 {
		updates["total_disk"] = disk
	}
	if err := db.Model(&models.Server{}).Where("identifier = ?", server.Identifier).Updates(updates).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"server_id":        server.Identifier,
		"restart_required": serverIsRunning(ns, &server.Node, server.Identifier),
	})
}

// serverIsRunning asks the node; when it cannot tell, it says yes so the caller restarts to be safe
func serverIsRunning(ns *services.Node, node *models.Node, identifier string) bool {
	res, err := ns.CallNode(node, "GET", daemonServerPrefix+identifier+"/status", nil, nil)
	defer utils.CloseResponse(res)
	if err != nil || res.StatusCode != http.StatusOK {
		return true
	}
	var status skypanel.ServerRunning
	if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
		return true
	}
	return status.Running
}

// SSORequest names who signs in: the account's email, and optionally the service whose
// server the browser lands on.
type SSORequest struct {
	ProvisionActionRequest
	Email string `json:"email" binding:"required,email"`
}

// @Summary One-time sign-in link
// @Description For the billing system's "go to the panel" button: returns a link that signs
// @Description the customer in without their password, once, within a minute. Administrator
// @Description accounts are refused, so a leaked billing key cannot open the admin panel.
// @Param request body SSORequest true "Customer email and optional service"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Tags Provision
// @Router /api/v1/sso [post]
func provisionSSO(c *gin.Context) {
	db := middleware.GetDatabase(c)
	var req SSORequest
	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	us := &services.User{DB: db}
	user, err := us.GetByEmail(req.Email)
	if response.HandleError(c, err, http.StatusNotFound) {
		return
	}

	ps := &services.Permission{DB: db}
	if isAdministrator(db, ps, user) {
		response.HandleError(c, skypanel.ErrSSONotForAdmins, http.StatusForbidden)
		return
	}

	redirect := "/servers/"
	if req.ServiceID != "" || req.ServerID != "" {
		ss := &services.Server{DB: db}
		server, err := getServerByRequest(db, req.ProvisionActionRequest, ss)
		if response.HandleError(c, err, http.StatusNotFound) {
			return
		}
		// the link only goes to a server the customer can open
		allowed, err := ps.HasPermission(user.ID, server.Identifier, scopes.ScopeServerView)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		if !allowed {
			response.HandleError(c, skypanel.ErrNoPermission, http.StatusForbidden)
			return
		}
		redirect = "/servers/" + server.Identifier + "/"
	}

	token, err := services.SSOLinks.Create(user.ID, redirect)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"url":        panelURL(c) + "/auth/sso?token=" + token,
		"expires_in": int(services.SSOLinkLifetime.Seconds()),
	})
}

// isAdministrator is true when the user holds the admin scope, directly or through a role
func isAdministrator(db *gorm.DB, ps *services.Permission, user *models.User) bool {
	perms, err := ps.GetForUserAndServer(user.ID, "")
	if err != nil {
		// unsure: treat as an administrator, the safe answer here
		return true
	}
	if scopes.ContainsScope(perms.Scopes, scopes.ScopeAdmin) {
		return true
	}
	if user.RoleID != nil {
		rs := &services.Role{DB: db}
		role, err := rs.Get(*user.RoleID)
		if err != nil {
			return true
		}
		for _, s := range role.Scopes {
			if scopes.ScopeAdmin.Is(s) {
				return true
			}
		}
	}
	return false
}

// panelURL is the address customers reach the panel at: the configured master URL, else
// the address the billing system called.
func panelURL(c *gin.Context) string {
	if master := strings.TrimSuffix(config.MasterURL.Value(), "/"); config.MasterURL.IsSet() && master != "" {
		return master
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}
