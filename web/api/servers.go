package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/database"
	"github.com/SkyPanel/SkyPanel/v3/logging"
	"github.com/SkyPanel/SkyPanel/v3/middleware"
	"github.com/SkyPanel/SkyPanel/v3/models"
	"github.com/SkyPanel/SkyPanel/v3/response"
	"github.com/SkyPanel/SkyPanel/v3/scopes"
	"github.com/SkyPanel/SkyPanel/v3/servers"
	"github.com/SkyPanel/SkyPanel/v3/services"
	"github.com/SkyPanel/SkyPanel/v3/utils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

func registerServers(g *gin.RouterGroup) {
	g.Handle("GET", "", searchServers)
	g.Handle("OPTIONS", "", response.CreateOptions("GET"))

	g.Handle("GET", "/:serverId", middleware.RequiresPermission(scopes.ScopeServerView), middleware.ResolveServerPanel, getServer)
	g.Handle("PUT", "/:serverId", middleware.RequiresPermission(scopes.ScopeServerCreate), middleware.HasTransaction, createServer)
	g.Handle("DELETE", "/:serverId", middleware.RequiresPermission(scopes.ScopeServerDelete), middleware.ResolveServerPanel, middleware.HasTransaction, deleteServer)
	g.Handle("POST", "/:serverId/suspend", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, middleware.HasTransaction, toggleServerSuspension)
	g.Handle("OPTIONS", "/:serverId", response.CreateOptions("PUT", "GET", "POST", "DELETE"))
	g.Handle("OPTIONS", "/:serverId/suspend", response.CreateOptions("POST"))

	g.Handle("PUT", "/:serverId/name/:name", middleware.RequiresPermission(scopes.ScopeServerEditName), middleware.ResolveServerPanel, middleware.HasTransaction, renameServer)
	g.Handle("OPTIONS", "/:serverId/name", response.CreateOptions("PUT"))
	g.Handle("OPTIONS", "/:serverId/name/:name", response.CreateOptions("PUT"))

	g.Handle("GET", "/:serverId/definition", middleware.RequiresPermission(scopes.ScopeServerViewDefinition), middleware.ResolveServerPanel, proxyServerRequest)
	g.Handle("PUT", "/:serverId/definition", middleware.RequiresPermission(scopes.ScopeServerEditDefinition), middleware.ResolveServerPanel, middleware.HasTransaction, editServer)
	g.Handle("OPTIONS", "/:serverId/definition", response.CreateOptions("PUT", "GET"))

	g.Handle("GET", "/:serverId/user", middleware.RequiresPermission(scopes.ScopeServerUserView), middleware.ResolveServerPanel, getServerUsers)
	g.Handle("OPTIONS", "/:serverId/user", response.CreateOptions("GET"))

	g.Handle("GET", "/:serverId/user/:email", middleware.RequiresPermission(scopes.ScopeServerUserView), middleware.ResolveServerPanel, getServerUsers)
	g.Handle("PUT", "/:serverId/user/:email", middleware.RequiresPermission(scopes.ScopeServerUserEdit), middleware.ResolveServerPanel, middleware.HasTransaction, editServerUser)
	g.Handle("DELETE", "/:serverId/user/:email", middleware.RequiresPermission(scopes.ScopeServerUserDelete), middleware.ResolveServerPanel, middleware.HasTransaction, removeServerUser)
	g.Handle("OPTIONS", "/:serverId/user/:email", response.CreateOptions("GET", "PUT", "DELETE"))

	g.GET("/:serverId/data", middleware.RequiresPermission(scopes.ScopeServerViewData), middleware.ResolveServerPanel, proxyServerRequest)
	g.POST("/:serverId/data", middleware.RequiresPermission(scopes.ScopeServerEditData), middleware.ResolveServerPanel, editServerData)
	g.PUT("/:serverId/data", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, editServerDataAdmin)
	g.OPTIONS("/:serverId/data", response.CreateOptions("GET", "POST", "PUT"))

	g.POST("/:serverId/transfer", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, transferServer)
	g.OPTIONS("/:serverId/transfer", response.CreateOptions("POST"))

	g.POST("/:serverId/ai/analyze", middleware.RequiresPermission(scopes.ScopeServerConsole), middleware.ResolveServerPanel, analyzeServerLogs)
	g.OPTIONS("/:serverId/ai/analyze", response.CreateOptions("POST"))

	g.POST("/:serverId/extransfer/create", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, CreateExTransfer)
	g.OPTIONS("/:serverId/extransfer/create", response.CreateOptions("POST"))

	g.POST("/:serverId/extransfer/pull", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, pullExTransfer)
	g.OPTIONS("/:serverId/extransfer/pull", response.CreateOptions("POST"))

	g.GET("/:serverId/extransfer/status", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, getExTransferStatus)
	g.OPTIONS("/:serverId/extransfer/status", response.CreateOptions("GET"))

	g.GET("/:serverId/flags", middleware.RequiresPermission(scopes.ScopeServerViewFlags), middleware.ResolveServerPanel, proxyServerRequest)
	g.POST("/:serverId/flags", middleware.RequiresPermission(scopes.ScopeServerEditFlags), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/flags", response.CreateOptions("GET", "POST"))

	g.GET("/:serverId/tasks", middleware.RequiresPermission(scopes.ScopeServerTaskView), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/tasks", response.CreateOptions("GET"))

	g.GET("/:serverId/tasks/:taskId", middleware.RequiresPermission(scopes.ScopeServerTaskRun), middleware.ResolveServerPanel, proxyServerRequest)
	g.PUT("/:serverId/tasks/:taskId", middleware.RequiresPermission(scopes.ScopeServerTaskEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.DELETE("/:serverId/tasks/:taskId", middleware.RequiresPermission(scopes.ScopeServerTaskDelete), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/tasks/:taskId", response.CreateOptions("GET", "PUT", "DELETE"))

	g.POST("/:serverId/tasks/:taskId/run", middleware.RequiresPermission(scopes.ScopeServerTaskRun), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/tasks/:taskId/run", response.CreateOptions("POST"))

	g.POST("/:serverId/reload", middleware.RequiresPermission(scopes.ScopeServerReload), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/reload", response.CreateOptions("POST"))

	g.POST("/:serverId/start", middleware.RequiresPermission(scopes.ScopeServerStart), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/start", response.CreateOptions("POST"))

	g.POST("/:serverId/restart", middleware.RequiresPermission(scopes.ScopeServerStart), middleware.RequiresPermission(scopes.ScopeServerStop), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/restart", response.CreateOptions("POST"))

	g.POST("/:serverId/stop", middleware.RequiresPermission(scopes.ScopeServerStop), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/stop", response.CreateOptions("POST"))

	g.POST("/:serverId/kill", middleware.RequiresPermission(scopes.ScopeServerKill), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/kill", response.CreateOptions("POST"))

	g.POST("/:serverId/install", middleware.RequiresPermission(scopes.ScopeServerInstall), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/install", response.CreateOptions("POST"))

	g.GET("/:serverId/file/*filename", middleware.RequiresPermission(scopes.ScopeServerFileView), middleware.ResolveServerPanel, proxyServerRequest)
	g.PUT("/:serverId/file/*filename", middleware.RequiresPermission(scopes.ScopeServerFileEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.DELETE("/:serverId/file/*filename", middleware.RequiresPermission(scopes.ScopeServerFileEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.POST("/:serverId/file/*filename", middleware.RequiresPermission(scopes.ScopeServerFileEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/file/*filename", response.CreateOptions("GET", "PUT", "DELETE", "POST"))

	g.GET("/:serverId/console", middleware.RequiresPermission(scopes.ScopeServerConsole), middleware.ResolveServerPanel, proxyServerRequest)
	g.POST("/:serverId/console", middleware.RequiresPermission(scopes.ScopeServerSendCommand), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/console", response.CreateOptions("GET", "POST"))

	g.GET("/:serverId/stats", middleware.RequiresPermission(scopes.ScopeServerStats), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/stats", response.CreateOptions("GET"))

	g.HEAD("/:serverId/query", middleware.RequiresPermission(scopes.ScopeServerStats), middleware.ResolveServerPanel, proxyServerRequest)
	g.GET("/:serverId/query", middleware.RequiresPermission(scopes.ScopeServerStats), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/query", response.CreateOptions("POST"))

	g.GET("/:serverId/status", middleware.RequiresPermission(scopes.ScopeServerStatus), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/status", response.CreateOptions("GET"))

	g.HEAD("/:serverId/archive/*filename", middleware.RequiresPermission(scopes.ScopeServerFileEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.POST("/:serverId/archive/*filename", middleware.RequiresPermission(scopes.ScopeServerFileEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/archive/*filename", response.CreateOptions("HEAD", "POST"))

	g.POST("/:serverId/extract/*filename", middleware.RequiresPermission(scopes.ScopeServerFileEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/extract/*filename", response.CreateOptions("POST"))

	g.GET("/:serverId/backup", middleware.RequiresPermission(scopes.ScopeServerBackupView), middleware.ResolveServerPanel, getBackups)
	g.OPTIONS("/:serverId/backup", response.CreateOptions("GET"))
	g.GET("/:serverId/backup/:backupId", middleware.RequiresPermission(scopes.ScopeServerBackupView), middleware.ResolveServerPanel, getBackup)
	g.DELETE("/:serverId/backup/:backupId", middleware.RequiresPermission(scopes.ScopeServerBackupDelete), middleware.ResolveServerPanel, deleteBackup)
	g.OPTIONS("/:serverId/backup/:backupId", response.CreateOptions("GET", "DELETE"))
	g.POST("/:serverId/backup/create", middleware.RequiresPermission(scopes.ScopeServerBackupCreate), middleware.ResolveServerPanel, createBackup)
	g.OPTIONS("/:serverId/backup/create", response.CreateOptions("POST"))
	g.POST("/:serverId/backup/restore/:backupId", middleware.RequiresPermission(scopes.ScopeServerBackupRestore), middleware.ResolveServerPanel, restoreBackup)
	g.OPTIONS("/:serverId/backup/restore/:backupId", response.CreateOptions("POST"))
	g.GET("/:serverId/backup/download/:backupId", middleware.RequiresPermission(scopes.ScopeServerBackupView), middleware.ResolveServerPanel, downloadBackup)
	g.OPTIONS("/:serverId/backup/download/:backupId", response.CreateOptions("GET"))

	g.GET("/:serverId/plugins", middleware.RequiresPermission(scopes.ScopeServerFileView), middleware.ResolveServerPanel, proxyServerRequest)
	g.DELETE("/:serverId/plugins", middleware.RequiresPermission(scopes.ScopeServerFileEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/plugins", response.CreateOptions("GET", "DELETE"))
	g.GET("/:serverId/plugins/search", middleware.RequiresPermission(scopes.ScopeServerFileView), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/plugins/search", response.CreateOptions("GET"))
	g.POST("/:serverId/plugins/:pluginId", middleware.RequiresPermission(scopes.ScopeServerFileEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/plugins/:pluginId", response.CreateOptions("POST"))

	p := g.Group("/:serverId/socket")
	{
		p.GET("", middleware.RequiresPermission(scopes.ScopeServerView), cors.New(cors.Config{
			AllowAllOrigins:  true,
			AllowCredentials: true,
		}), middleware.ResolveServerPanel, proxyServerRequest)
		p.Handle("CONNECT", "", middleware.RequiresPermission(scopes.ScopeServerView), func(c *gin.Context) {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Credentials", "false")
		})
		p.OPTIONS("", response.CreateOptions("GET"))
	}
}

// @Summary Search servers
// @Description Gets servers, and allowing for filtering of servers. * is a wildcard that can be used for text inputs
// @Success 200 {object} models.ServerSearchResponse
// @Param username query string false "Username to filter on, default is current user if NOT admin"
// @Param node query uint false "Node ID to filter on"
// @Param name query string false "Name of server to filter on"
// @Param limit query uint false "Max number of results to return"
// @Param page query uint false "What page to get back for many results"
// @Tags Daemon Servers
// @Router /api/servers [get]
// @Security OAuth2Application[server.view]
func searchServers(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	ss := &services.Server{DB: db}
	ps := &services.Permission{DB: db}

	username := c.DefaultQuery("username", "")
	nodeQuery := c.DefaultQuery("node", "0")
	nameFilter := c.DefaultQuery("name", "*")
	pageSizeQuery := c.DefaultQuery("limit", strconv.Itoa(DefaultPageSize))
	pageQuery := c.DefaultQuery("page", strconv.Itoa(1))

	pageSize, err := strconv.Atoi(pageSizeQuery)
	if response.HandleError(c, err, http.StatusBadRequest) || pageSize <= 0 {
		response.HandleError(c, SkyPanel.ErrFieldTooSmall("pageSize", 0), http.StatusBadRequest)
		return
	}

	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	page, err := strconv.Atoi(pageQuery)
	if response.HandleError(c, err, http.StatusBadRequest) || page <= 0 {
		response.HandleError(c, SkyPanel.ErrFieldTooSmall("page", 0), http.StatusBadRequest)
		return
	}

	node, err := strconv.Atoi(nodeQuery)
	if response.HandleError(c, err, http.StatusBadRequest) || node < 0 {
		response.HandleError(c, SkyPanel.ErrFieldTooSmall("nodeId", 0), http.StatusBadRequest)
		return
	}

	user := c.MustGet("user").(*models.User)
	userScopes := make([]*scopes.Scope, 0)

	// Direct permissions
	perms, err := ps.GetForUser(user.ID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	for _, p := range perms {
		for _, s := range p.Scopes {
			userScopes = scopes.AddScope(userScopes, s)
		}
	}

	// Role permissions
	if user.RoleId != nil && user.Role.ID != 0 {
		for _, s := range user.Role.Scopes {
			userScopes = scopes.AddScope(userScopes, scopes.GetScope(s))
		}
	}

	isAdmin := scopes.ContainsScope(userScopes, scopes.ScopeAdmin)

	if !isAdmin && username != "" && user.Username != username {
		c.JSON(http.StatusOK, &models.ServerSearchResponse{
			Servers: []*models.ServerView{},
			Metadata: &SkyPanel.Metadata{Paging: &SkyPanel.Paging{
				Page:    1,
				Size:    0,
				MaxSize: MaxPageSize,
				Total:   0,
			}},
		})
		return
	} else if !isAdmin {
		username = user.Username
	}

	searchCriteria := services.ServerSearch{
		Username: username,
		NodeId:   uint(node),
		Name:     nameFilter,
		PageSize: uint(pageSize),
		Page:     uint(page),
	}

	results, total, err := ss.Search(searchCriteria)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	var data []*models.ServerView
	if isAdmin {
		data = models.FromServers(results)
	} else {
		data = models.RemoveServerPrivateInfoFromAll(models.FromServers(results))
	}

	for i, v := range data {
		checkGhost(results[i], v)
		if isAdmin {
			v.CanGetStatus = true
			continue
		}

		serverPerms, _ := ps.GetForUserAndServer(user.ID, v.Identifier)
		allPotentialScopes := userScopes
		if serverPerms != nil {
			for _, s := range serverPerms.Scopes {
				allPotentialScopes = scopes.AddScope(allPotentialScopes, s)
			}
		}

		if scopes.ContainsScope(allPotentialScopes, scopes.ScopeServerStatus) {
			v.CanGetStatus = true
		}
	}

	c.JSON(http.StatusOK, &models.ServerSearchResponse{
		Servers: data,
		Metadata: &SkyPanel.Metadata{Paging: &SkyPanel.Paging{
			Page:    uint(page),
			Size:    uint(pageSize),
			MaxSize: MaxPageSize,
			Total:   total,
		}},
	})
}

// @Summary Get a server
// @Description Gets a particular server
// @Success 200 {object} models.GetServerResponse
// @Param id path string true "Server ID"
// @Tags Daemon Servers
// @Router /api/servers/{id} [get]
// @Security OAuth2Application[server.view]
func getServer(c *gin.Context) {
	server := getServerFromGin(c)

	_, includePerms := c.GetQuery("perms")
	var perms *models.PermissionView
	db, err := database.GetConnection()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	u := c.MustGet("user").(*models.User)
	ps := &services.Permission{DB: db}

	if includePerms {
		p, err := ps.GetForUserAndServer(u.ID, server.Identifier)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		perms = models.FromPermission(p)
	}

	userScopes := make([]*scopes.Scope, 0)
	userPerms, err := ps.GetForUser(u.ID)
	if err == nil {
		for _, p := range userPerms {
			for _, s := range p.Scopes {
				userScopes = scopes.AddScope(userScopes, s)
			}
		}
	}
	if u.RoleId != nil && u.Role.ID != 0 {
		for _, s := range u.Role.Scopes {
			userScopes = scopes.AddScope(userScopes, scopes.GetScope(s))
		}
	}
	isAdmin := scopes.ContainsScope(userScopes, scopes.ScopeAdmin)

	var serverView *models.ServerView
	if isAdmin {
		serverView = models.FromServer(server)
	} else {
		serverView = models.RemoveServerPrivateInfo(models.FromServer(server))
	}

	d := &models.GetServerResponse{
		Server: serverView,
		Perms:  perms,
	}

	checkGhost(server, d.Server)

	c.JSON(http.StatusOK, d)
}

// @Summary Create server
// @Description Creates a server
// @Success 200 {object} models.CreateServerResponse
// @Param id path string true "Server ID"
// @Param server body models.ServerCreation true "Creation information"
// @Tags Daemon Servers
// @Router /api/servers/{id} [put]
// @Security OAuth2Application[server.create]
func createServer(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}
	us := &services.User{DB: db}
	ps := &services.Permission{DB: db}

	serverId := c.Param("serverId")

	if serverId == "" {
		gen, err := uuid.NewV4()
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		serverId = gen.String()[:8]
	}

	postBody := &models.ServerCreation{}
	err = c.ShouldBindJSON(&postBody)
	postBody.Identifier = serverId
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if postBody.ParentServerID != nil && *postBody.ParentServerID != "" {
		var parent models.Server
		if err := db.Where("identifier = ?", *postBody.ParentServerID).First(&parent).Error; err != nil {
			response.HandleError(c, errors.New("parent server not found"), http.StatusBadRequest)
			return
		}

		// Force node to match parent
		postBody.NodeId = parent.NodeID

		// Inherit users from parent
		var parentPerms []models.Permissions
		db.Where("server_identifier = ?", parent.Identifier).Find(&parentPerms)

		var inheritedUsers []string
		for _, p := range parentPerms {
			if p.UserId != nil {
				user, err := us.GetById(*p.UserId)
				if err == nil && user != nil {
					inheritedUsers = append(inheritedUsers, user.Username)
				}
			}
		}
		postBody.Users = inheritedUsers
	}

	node, err := ns.Get(postBody.NodeId)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.HandleError(c, SkyPanel.ErrNodeInvalid, http.StatusBadRequest)
		return
	} else if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	port, err := getFromDataOrDefault(postBody.Variables, "port", uint16(0))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	ip, err := getFromDataOrDefault(postBody.Variables, "ip", "0.0.0.0")
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if postBody.Name == "" {
		postBody.Name = postBody.Identifier
	}

	cpuVar, _ := getFromDataOrDefault(postBody.Variables, "cpu", 0)
	memoryVar, _ := getFromDataOrDefault(postBody.Variables, "memory", 0)
	diskVar, _ := getFromDataOrDefault(postBody.Variables, "disk", 0)

	totalCPU := cast.ToInt(cpuVar)
	totalMemory := cast.ToInt64(memoryVar)
	totalDisk := cast.ToInt64(diskVar)

	if totalCPU < 0 || totalMemory < 0 || totalDisk < 0 {
		response.HandleError(c, errors.New("resources cannot be negative"), http.StatusBadRequest)
		return
	}

	server := &models.Server{
		Name:       postBody.Name,
		Identifier: postBody.Identifier,
		NodeID:     node.ID,
		IP:         cast.ToString(ip),
		Port:       cast.ToUint16(port),
		Type:       postBody.Type.Type,
		Icon:       postBody.Icon,
		ParentServerID: postBody.ParentServerID,
		TotalCPU:   totalCPU,
		TotalMemory: totalMemory,
		TotalDisk:  totalDisk,
	}

	users := make([]*models.User, len(postBody.Users))

	for k, v := range postBody.Users {
		user, err := us.Get(v)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}

		users[k] = user
	}

	var parentAvailableCPU int
	var parentAvailableMemory int64
	var parentAvailableDisk int64

	// Transactional validation and creation
	err = db.Transaction(func(tx *gorm.DB) error {
		if server.ParentServerID != nil && *server.ParentServerID != "" {
			var parent models.Server
			if err := tx.Raw("SELECT * FROM servers WHERE identifier = ? FOR UPDATE", *server.ParentServerID).Scan(&parent).Error; err != nil {
				return err
			}
			if parent.Identifier == "" {
				return errors.New("parent server not found")
			}

			var children []*models.Server
			if err := tx.Where("parent_server_id = ?", parent.Identifier).Find(&children).Error; err != nil {
				return err
			}

			var usedCPU int
			var usedMemory, usedDisk int64
			for _, child := range children {
				usedCPU += child.TotalCPU
				usedMemory += child.TotalMemory
				usedDisk += child.TotalDisk
			}

			parentAvailableCPU = parent.TotalCPU - usedCPU - server.TotalCPU
			parentAvailableMemory = parent.TotalMemory - usedMemory - server.TotalMemory
			parentAvailableDisk = parent.TotalDisk - usedDisk - server.TotalDisk

			if parentAvailableCPU < 0 {
				return errors.New("not enough CPU available in parent server")
			}
			if parentAvailableMemory < 0 {
				return errors.New("not enough memory available in parent server")
			}
			if parentAvailableDisk < 0 {
				return errors.New("not enough disk available in parent server")
			}
		}

		// Create server within transaction
		return tx.Create(server).Error
	})

	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	for _, v := range users {
		perm, err := ps.GetForUserAndServer(v.ID, server.Identifier)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}

		perm.Scopes = []*scopes.Scope{
			scopes.ScopeServerView,
			scopes.ScopeServerViewData,
			scopes.ScopeServerEditData,
			scopes.ScopeServerEditFlags,
			scopes.ScopeServerEditName,
			scopes.ScopeServerViewData,
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

		err = ps.UpdatePermissions(perm)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	}

	reader := &bytes.Buffer{}
	err = json.NewEncoder(reader).Encode(&postBody.Server)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	nodeResponse, err := ns.CallNode(node, "PUT", "/daemon/server/"+server.Identifier, io.NopCloser(reader), c.Request.Header)
	defer utils.CloseResponse(nodeResponse)

	if response.HandleError(c, err, http.StatusInternalServerError) {
		// _ = ss.Delete(server.Identifier) //esto es IA
		return
	}

	if nodeResponse.StatusCode != http.StatusOK {
		// _ = ss.Delete(server.Identifier) //esto es IA
		resData, err := io.ReadAll(nodeResponse.Body)
		if err != nil {
			logging.Error.Printf("Failed to parse response from daemon\n%s", err.Error())
		}
		logging.Error.Printf("Unexpected response from daemon: %+v\n%s", nodeResponse.StatusCode, string(resData))
		//assume daemon gives us a valid response, directly forward to client
		c.Header("Content-Type", "application/json")
		c.Status(nodeResponse.StatusCode)
		_, _ = c.Writer.Write(resData)
		c.Abort()
		return
	}

	// Update the parent's actual limits in the daemon if it's a subserver
	if server.ParentServerID != nil && *server.ParentServerID != "" {
		// We send a partial update to the daemon with the new available limits
		parentDataUpdate := map[string]interface{}{
			"cpu":    parentAvailableCPU,
			"memory": parentAvailableMemory,
			"disk":   parentAvailableDisk,
		}
		updateBytes, _ := json.Marshal(parentDataUpdate)
		parentReqBody := io.NopCloser(bytes.NewReader(updateBytes))
		
		var parent models.Server
		db.Where("identifier = ?", *server.ParentServerID).First(&parent)
		// We call the daemon's data endpoint to merge these new limits
		parentRes, _ := ns.CallNode(node, "POST", "/daemon/server/"+parent.Identifier+"/data", parentReqBody, c.Request.Header)
		if parentRes != nil && parentRes.Body != nil {
			parentRes.Body.Close()
		}
	}

	es := services.GetEmailService()
	for _, user := range users {
		err = es.SendEmail(user.Email, "addedToServer", map[string]interface{}{
			"Server":        server,
			"RegisterToken": "",
		}, true)
		if err != nil {
			//since we don't want to tell the user it failed, we'll log and move on
			logging.Error.Printf("Error sending email: %s", err)
		}
	}

	c.JSON(http.StatusOK, &models.CreateServerResponse{Id: serverId})
}

// @Summary Update server definition
// @Description Updates a server definition
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param server body models.ServerWithName true "Server definition"
// @Tags Daemon Servers
// @Router /api/servers/{id}/definition [put]
// @Security OAuth2Application[server.definition.edit]
func editServer(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	ss := &services.Server{DB: db}
	ns := &services.Node{DB: db}

	server := getServerFromGin(c)

	postBody := &models.ServerWithName{}
	err = c.BindJSON(postBody)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	postBody.Identifier = server.Identifier

	port, err := getFromDataOrDefault(postBody.Variables, "port", uint16(0))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	server.Port = cast.ToUint16(port)

	ip, err := getFromDataOrDefault(postBody.Variables, "ip", "0.0.0.0")
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	server.IP = cast.ToString(ip)

	if postBody.Name != "" {
		server.Name = postBody.Name
	}

	if postBody.Type.Type != "" {
		server.Type = postBody.Type.Type
	}

	if postBody.Icon != "" {
		server.Icon = postBody.Icon
	}

	err = ss.Update(server)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	data, _ := json.Marshal(postBody)
	reader := io.NopCloser(bytes.NewReader(data))

	nodeResponse, err := ns.CallNode(&server.Node, "PUT", "/daemon/server/"+postBody.Identifier+"/definition", reader, c.Request.Header)
	defer utils.CloseResponse(nodeResponse)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if nodeResponse.StatusCode != http.StatusNoContent {
		resData, err := io.ReadAll(nodeResponse.Body)
		if err != nil {
			logging.Error.Printf("Failed to parse response from daemon\n%s", err.Error())
		}
		logging.Error.Printf("Unexpected response from daemon: %+v\n%s", nodeResponse.StatusCode, string(resData))
		//assume daemon gives us a valid response, directly forward to client
		c.Header("Content-Type", "application/json")
		c.Status(nodeResponse.StatusCode)
		_, _ = c.Writer.Write(resData)
		e := c.Error(errors.New("unexpected response from daemon"))
		response.HandleError(c, e, http.StatusInternalServerError)
		return
	}

	if response.HandleError(c, db.Commit().Error, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Deletes a server
// @Description Deletes a server from the panel
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Servers
// @Router /api/servers/{id} [delete]
// @Security OAuth2Application[server.delete]
func deleteServer(c *gin.Context) {
	var err error

	db := middleware.GetDatabase(c)
	ss := &services.Server{DB: db}
	ns := &services.Node{DB: db}

	server := getServerFromGin(c)

	node := &server.Node

	//we need to know what users are impacted by a server being deleted
	ps := services.Permission{DB: db}
	users := make([]models.User, 0)
	perms, err := ps.GetForServer(server.Identifier)
	if err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	}
	for _, p := range perms {
		exists := false
		for _, u := range users {
			if u.ID == p.User.ID {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		users = append(users, p.User)
	}

	// Splitter Check: Ensure it has no children
	var childrenCount int64
	if err := db.Model(&models.Server{}).Where("parent_server_id = ?", server.Identifier).Count(&childrenCount).Error; err == nil && childrenCount > 0 {
		response.HandleError(c, errors.New("cannot delete server because it has active child servers"), http.StatusBadRequest)
		return
	}

	_, skipNode := c.GetQuery("skipNode")
	if !skipNode {
		// Primero intentar detener el servidor si está corriendo
		// Verificar si el servidor está corriendo llamando al endpoint de status
		statusRes, err := ns.CallNode(node, "GET", "/daemon/server/"+server.Identifier+"/status", nil, nil)
		if err == nil && statusRes.StatusCode == http.StatusOK {
			var statusData struct {
				Running bool `json:"running"`
			}
			if err := json.NewDecoder(statusRes.Body).Decode(&statusData); err == nil && statusData.Running {
				// El servidor está corriendo, detenerlo primero
				stopRes, err := ns.CallNode(node, "POST", "/daemon/server/"+server.Identifier+"/stop?wait=true", nil, nil)
				if err != nil {
					logging.Error.Printf("Error stopping server before deletion: %s", err)
					response.HandleError(c, err, http.StatusInternalServerError)
					return
				}
				// Cerrar el body de la respuesta de stop
				if stopRes != nil && stopRes.Body != nil {
					stopRes.Body.Close()
				}
			}
			// Cerrar el body de la respuesta de status
			if statusRes.Body != nil {
				statusRes.Body.Close()
			}
		}

		// Ahora intentar eliminar el servidor
		nodeRes, err := ns.CallNode(node, "DELETE", "/daemon/server/"+server.Identifier, nil, nil)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			//node didn't permit it, REVERT!
			return
		}

		if nodeRes.StatusCode != http.StatusNoContent && nodeRes.StatusCode != http.StatusOK && nodeRes.StatusCode != http.StatusNotFound {
			resData, _ := io.ReadAll(nodeRes.Body)
			response.HandleError(c, errors.New("invalid status code response: "+nodeRes.Status+" body: "+string(resData)), http.StatusInternalServerError)
			return
		}
		// Cerrar el body de la respuesta
		if nodeRes.Body != nil {
			nodeRes.Body.Close()
		}
	}

	err = ss.Delete(server.Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// Splitter Restore: Update parent limits in daemon
	if server.ParentServerID != nil && *server.ParentServerID != "" {
		var parent models.Server
		if err := db.Raw("SELECT * FROM servers WHERE identifier = ? FOR UPDATE", *server.ParentServerID).Scan(&parent).Error; err == nil && parent.Identifier != "" {
			var children []*models.Server
			db.Where("parent_server_id = ?", parent.Identifier).Find(&children)
			
			var usedCPU int
			var usedMemory, usedDisk int64
			for _, child := range children {
				if child.Identifier == server.Identifier {
					continue // En caso de que el soft delete lo siga incluyendo
				}
				usedCPU += child.TotalCPU
				usedMemory += child.TotalMemory
				usedDisk += child.TotalDisk
			}
			
			parentAvailableCPU := parent.TotalCPU - usedCPU
			parentAvailableMemory := parent.TotalMemory - usedMemory
			parentAvailableDisk := parent.TotalDisk - usedDisk
			
			// Send to daemon
			parentDataUpdate := map[string]interface{}{
				"cpu":    parentAvailableCPU,
				"memory": parentAvailableMemory,
				"disk":   parentAvailableDisk,
			}
			updateBytes, _ := json.Marshal(parentDataUpdate)
			parentReqBody := io.NopCloser(bytes.NewReader(updateBytes))
			
			parentRes, _ := ns.CallNode(node, "POST", "/daemon/server/"+parent.Identifier+"/data", parentReqBody, c.Request.Header)
			if parentRes != nil && parentRes.Body != nil {
				parentRes.Body.Close()
			}
		}
	}

	// Rely on HasTransaction to commit at end of the block

	es := services.GetEmailService()
	for _, u := range users {
		err = es.SendEmail(u.Email, "deletedServer", map[string]interface{}{
			"Server": server,
		}, true)
		if err != nil {
			//since we don't want to tell the user it failed, we'll log and move on
			logging.Error.Printf("Error sending email: %s\n", err)
		}
	}

	c.Status(http.StatusNoContent)
}

// @Summary Gets all users for a server
// @Success 200 {object} []models.UserPermissionsView
// @Param id path string true "Server ID"
// @Param email path string true "Email"
// @Tags Daemon Servers
// @Router /api/servers/{id}/user [get]
// @Tags Daemon Servers
// @Router /api/servers/{id}/user/{email} [get]
// @Security OAuth2Application[server.users.view]
func getServerUsers(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	ps := &services.Permission{DB: db}

	server := getServerFromGin(c)

	email := c.Param("email")

	var perms []*models.Permissions
	if email != "" {
		us := &services.User{DB: db}
		var user *models.User
		user, err = us.GetByEmail(email)
		if user == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			response.HandleError(c, err, http.StatusNotFound)
			return
		}
		var p *models.Permissions
		p, err = ps.GetForUserAndServer(user.ID, server.Identifier)
		if p != nil {
			perms = []*models.Permissions{p}
		}
	} else {
		perms, err = ps.GetForServer(server.Identifier)
	}

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	users := map[*models.User][]*scopes.Scope{}

	for _, v := range perms {
		p := make([]*scopes.Scope, 0)
		for z, r := range users {
			if v.User.ID == z.ID {
				//this is the user
				p = r
				break
			}
		}
		p = append(p, v.Scopes...)

		found := false
		for z := range users {
			if v.User.ID == z.ID {
				//this is the user
				users[z] = p
				found = true
				break
			}
		}
		if !found {
			users[&v.User] = p
		}
	}

	data := make([]*models.UserPermissionsView, 0)
	for k, v := range users {
		data = append(data, &models.UserPermissionsView{
			Username: k.Username,
			Email:    k.Email,
			Scopes:   v,
		})
	}

	c.JSON(http.StatusOK, data)
}

// @Summary Edits access to a server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param email path string true "Email of user"
// @Param permissions body models.PermissionView true "New permissions to apply"
// @Tags Daemon Servers
// @Router /api/servers/{id}/users/{email} [put]
// @Security OAuth2Application[server.users.edit]
func editServerUser(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}
	ps := &services.Permission{DB: db}

	email := c.Param("email")
	username := c.Param("username")
	if email == "" && username == "" {
		return
	}

	perms := &models.PermissionView{}
	err = c.BindJSON(perms)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	server := getServerFromGin(c)

	currentUser := c.MustGet("user").(*models.User)
	currentPerms, err := ps.GetForUserAndServer(currentUser.ID, server.Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	currentGlobalPerms, err := ps.GetForUserAndServer(currentUser.ID, "")
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	var registerToken string
	var user *models.User
	if email != "" {
		user, err = us.GetByEmail(email)
	} else {
		user, err = us.Get(username)
	}

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && response.HandleError(c, err, http.StatusInternalServerError) {
		return
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		if email == "" {
			response.HandleError(c, err, http.StatusBadRequest)
			return
		}
		//we need to create the user here, since it's a new email we've not seen

		un, err := uuid.NewV4()
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		user = &models.User{
			Username: un.String(),
			Email:    email,
		}
		token, err := uuid.NewV4()
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		registerToken = token.String()
		err = user.SetPassword(registerToken)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}

		err = us.Create(user)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	}

	existing, err := ps.GetForUserAndServer(user.ID, server.Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	var firstTimeAccess = false
	if existing.ID == 0 {
		firstTimeAccess = true
	}

	//update perms to match this "setup", but not stomp over what the user can't change
	if scopes.ContainsScope(currentPerms.Scopes, scopes.ScopeServerAdmin) || scopes.ContainsScope(currentGlobalPerms.Scopes, scopes.ScopeServerAdmin) || scopes.ContainsScope(currentGlobalPerms.Scopes, scopes.ScopeAdmin) {
		existing.Scopes = perms.Scopes
	} else {
		//update perms to match this "setup", but not stomp over what the user can't change
		replacement := scopes.UpdateScopesWhereGranted(existing.Scopes, perms.Scopes, currentPerms.Scopes)
		existing.Scopes = replacement
	}

	err = ps.UpdatePermissions(existing)

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if response.HandleError(c, db.Commit().Error, http.StatusInternalServerError) {
		return
	}

	//now we can send emails to the people
	if firstTimeAccess {
		es := services.GetEmailService()
		err = es.SendEmail(user.Email, "addedToServer", map[string]interface{}{
			"Server":        server,
			"RegisterToken": registerToken,
			"Email":         user.Email,
		}, true)
		if err != nil {
			//since we don't want to tell the user it failed, we'll log and move on
			logging.Error.Printf("Error sending email: %s\n", err)
		}
	}

	c.Status(http.StatusNoContent)
}

// @Summary Removes access to a server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param email path string true "Email of user"
// @Tags Daemon Servers
// @Router /api/servers/{id}/users/{email} [delete]
// @Security OAuth2Application[server.users.delete]
func removeServerUser(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}
	ps := &services.Permission{DB: db}

	email := c.Param("email")
	username := c.Param("username")
	if email == "" && username == "" {
		return
	}

	server := getServerFromGin(c)

	var user *models.User
	if email != "" {
		user, err = us.GetByEmail(email)
	} else {
		user, err = us.Get(username)
	}

	if err != nil && response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	perms, err := ps.GetForUserAndServer(user.ID, server.Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = ps.Remove(perms)

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if response.HandleError(c, db.Commit().Error, http.StatusInternalServerError) {
		return
	}

	es := services.GetEmailService()
	err = es.SendEmail(user.Email, "removedFromServer", map[string]interface{}{
		"Server": server,
	}, true)
	if err != nil {
		//since we don't want to tell the user it failed, we'll log and move on
		logging.Error.Printf("Error sending email: %s\n", err)
	}

	c.Status(http.StatusNoContent)
}

// @Summary Rename server
// @Description Renames a server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param name path string true "New server name"
// @Tags Daemon Servers
// @Router /api/servers/{id}/name/{name} [put]
// @Security OAuth2Application[server.name.edit]
func renameServer(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	ss := &services.Server{DB: db}

	server.Name = c.Param("name")
	err := ss.Update(server)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Update server data
// @Description Updates a server's set of variables
// @Success 202 {object} nil
// @Param id path string true "Server ID"
// @Param server body map[string]interface{} true "Server variables"
// @Tags Daemon Servers
// @Router /api/servers/{id}/data [post]
// @Security OAuth2Application[server.data.edit]
func editServerData(c *gin.Context) {
	proxyServerRequest(c)
}

// @Summary Update server data with admin level rights
// @Description Updates a server's set of variables
// @Success 202 {object} nil
// @Param id path string true "Server ID"
// @Param server body map[string]interface{} true "Server variables"
// @Tags Daemon Servers
// @Router /api/servers/{id}/data [put]
// @Security OAuth2Application[server.data.edit.admin]
func editServerDataAdmin(c *gin.Context) {
	server := getServerFromGin(c)

	//clone request body, so we can re-set it for the proxy call
	useHere := &bytes.Buffer{}
	useThere := &bytes.Buffer{}

	multi := io.MultiWriter(useHere, useThere)
	_, err := io.Copy(multi, c.Request.Body)
	if err != nil && response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	_ = c.Request.Body.Close()
	c.Request.Body = io.NopCloser(useThere)

	var postBody map[string]interface{}
	err = json.NewDecoder(useHere).Decode(&postBody)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	dirty := false
	port, exist := postBody["port"]
	if exist {
		portVal, err := cast.ToUint16E(port)
		if response.HandleError(c, err, http.StatusBadRequest) {
			return
		}
		server.Port = portVal
		dirty = true
	}

	ip, exist := postBody["ip"]
	if exist {
		if response.HandleError(c, err, http.StatusBadRequest) {
			return
		}
		server.IP = cast.ToString(ip)
		dirty = true
	}

	if dirty {
		db := middleware.GetDatabase(c)
		ss := &services.Server{DB: db}
		err = ss.Update(server)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	}

	proxyServerRequest(c)
}

// @Summary Gets servers backups
// @Description Gets all backups made on this server
// @Success 200 {object} models.Backup
// @Param id path string true "Server ID"
// @Tags Daemon Servers
// @Router /api/servers/{id}/backup [get]
// @Security OAuth2Application[server.backup.view]
func getBackups(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	bs := &services.Backup{DB: db}

	records, err := bs.GetAllForServer(server.Identifier)

	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.JSON(http.StatusOK, records)
	}
}

// @Summary Gets a specific backup on a server
// @Description Gets a specific backup made on this server
// @Success 200 {object} models.Backup
// @Param id path string true "Server ID"
// @Param backupId path string true "BackupId"
// @Tags Daemon Servers
// @Router /api/servers/{id}/backup/{backupId} [get]
// @Security OAuth2Application[server.backup.view]
func getBackup(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	bs := &services.Backup{DB: db}
	backupId, err := cast.ToUintE(c.Param("backupId"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	records, err := bs.Get(server.Identifier, backupId)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, records)
}

// @Summary Create backup
// @Description Creates a full backup of the server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Query name query string true "name of the backup"
// @Tags Daemon Servers
// @Router /api/servers/{id}/backup/create [post]
// @Security OAuth2Application[server.backup.create]
func createBackup(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}
	bs := &services.Backup{DB: db}
	name := c.Query("name")
	node := &server.Node

	if name == "" {
		response.HandleError(c, SkyPanel.ErrFieldRequired("name"), http.StatusBadRequest)
		return
	}

	resolvedPath := "/daemon/server/" + strings.TrimPrefix(c.Request.URL.Path, "/api/servers/")
	if c.Request.URL.RawQuery != "" {
		resolvedPath += "?" + c.Request.URL.RawQuery
	}

	callResponse, err := ns.CallNode(node, c.Request.Method, resolvedPath, c.Request.Body, c.Request.Header)
	defer utils.CloseResponse(callResponse)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if callResponse.StatusCode == http.StatusBadRequest { //If its a local node, the err will not be set, have to check the status code
		newHeaders := cleanHttpReturnErrors(callResponse.Header)

		c.DataFromReader(callResponse.StatusCode, callResponse.ContentLength, callResponse.Header.Get("Content-Type"), callResponse.Body, newHeaders)
		c.Abort()
		return
	}

	responseData := &SkyPanel.ServerBackupResponse{}
	err = json.NewDecoder(callResponse.Body).Decode(responseData)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	backup := &models.Backup{Name: name, FileName: responseData.BackupFileName, ServerID: server.Identifier}
	err = bs.Create(backup)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Delete backup
// @Description Removes the backup and its associated file
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param backupId path string true "Backup ID"
// @Tags Daemon Servers
// @Router /api/servers/{id}/backup/Delete/{backupId} [delete]
// @Security OAuth2Application[server.backup.delete]
func deleteBackup(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}
	bs := &services.Backup{DB: db}
	node := &server.Node

	backupId, err := cast.ToUintE(c.Param("backupId"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	backup, err := bs.Get(server.Identifier, backupId)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if backup == nil {
		c.Status(http.StatusNotFound)
		return
	}

	resolvedPath := "/daemon/server/" + server.Identifier + "/backup" + "?fileName=" + backup.FileName

	callResponse, err := ns.CallNode(node, "DELETE", resolvedPath, nil, nil)
	defer utils.CloseResponse(callResponse)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if callResponse.StatusCode == http.StatusBadRequest { //If its a local node, the err will not be set, have to check the status code
		newHeaders := cleanHttpReturnErrors(callResponse.Header)

		c.DataFromReader(callResponse.StatusCode, callResponse.ContentLength, callResponse.Header.Get("Content-Type"), callResponse.Body, newHeaders)
		c.Abort()
		return
	}

	err = bs.Delete(backupId)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Restore backup
// @Description Removes all exisiting files and restores the server to the state of the backup
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param backupId path string true "Backup ID"
// @Tags Daemon Servers
// @Router /api/servers/{id}/backup/Delete/{backupId} [delete]
// @Security OAuth2Application[server.backup.restore]
func restoreBackup(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}
	bs := &services.Backup{DB: db}
	node := &server.Node

	backupId, err := cast.ToUintE(c.Param("backupId"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	backup, err := bs.Get(server.Identifier, backupId)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if backup == nil {
		c.Status(http.StatusNotFound)
		return
	}

	resolvedPath := "/daemon/server/" + server.Identifier + "/backup/restore" + "?fileName=" + backup.FileName

	callResponse, err := ns.CallNode(node, "POST", resolvedPath, nil, nil)
	defer utils.CloseResponse(callResponse)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if callResponse.StatusCode == http.StatusBadRequest { //If its a local node, the err will not be set, have to check the status code
		newHeaders := cleanHttpReturnErrors(callResponse.Header)

		c.DataFromReader(callResponse.StatusCode, callResponse.ContentLength, callResponse.Header.Get("Content-Type"), callResponse.Body, newHeaders)
		c.Abort()
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Download backup
// @Description Download a server backup
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param backupId path string true "Backup ID"
// @Tags Daemon Servers
// @Router /api/servers/{id}/backup/Delete/{backupId} [delete]
// @Security OAuth2Application[server.backup.restore]
func downloadBackup(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}
	bs := &services.Backup{DB: db}
	node := &server.Node

	backupId, err := cast.ToUintE(c.Param("backupId"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	backup, err := bs.Get(server.Identifier, backupId)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if backup == nil {
		c.Status(http.StatusNotFound)
		return
	}

	resolvedPath := "/daemon/server/" + server.Identifier + "/backup/download" + "?fileName=" + backup.FileName

	callResponse, err := ns.CallNode(node, "GET", resolvedPath, nil, nil)
	defer utils.CloseResponse(callResponse)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	newHeaders := cleanHttpReturnErrors(callResponse.Header)

	c.DataFromReader(callResponse.StatusCode, callResponse.ContentLength, callResponse.Header.Get("Content-Type"), callResponse.Body, newHeaders)
}

func getFromData(variables map[string]SkyPanel.Variable, key string) (result interface{}, exists bool) {
	for k, v := range variables {
		if k == key {
			return v.Value, true
		}
	}
	return nil, false
}

func getFromDataOrDefault(variables map[string]SkyPanel.Variable, key string, val interface{}) (interface{}, error) {
	res, exists := getFromData(variables, key)

	if exists {
		return utils.Convert(res, val)
	}

	return val, nil
}

func proxyServerRequest(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}

	resolvedPath := "/daemon/server/" + strings.TrimPrefix(c.Request.URL.Path, "/api/servers/")
	if c.Request.URL.RawQuery != "" {
		resolvedPath += "?" + c.Request.URL.RawQuery
	}

	user := c.MustGet("user").(*models.User)
	server := c.MustGet("server").(*models.Server)
	node := &server.Node

	// Block starting or restarting if the server is suspended
	if server.Suspended && (strings.HasSuffix(c.Request.URL.Path, "/start") || strings.HasSuffix(c.Request.URL.Path, "/restart")) {
		response.HandleError(c, errors.New("cannot start or restart a suspended server"), http.StatusForbidden)
		return
	}

	ts, err := services.NewTokenService()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	token, err := ts.GenerateRequest()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	//switch to our token for auth
	c.Request.Header.Set("Authorization", "Bearer "+token)

	if c.IsWebsocket() {
		//for websocket, nuke the query params to avoid trying to escalate
		resolvedPath = strings.SplitN(resolvedPath, "?", 2)[0]
		if !strings.HasPrefix(resolvedPath, "/") {
			resolvedPath = "/" + resolvedPath
		}

		permService := &services.Permission{DB: db}
		perms, err := permService.GetForUserAndServer(user.ID, server.Identifier)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}

		allScopes := perms.Scopes

		perms, err = permService.GetForUserAndServer(user.ID, "")
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}

		allScopes = append(allScopes, perms.Scopes...)

		//add the params we can grant for this request
		var params []string
		if scopes.ContainsScope(allScopes, scopes.ScopeServerConsole) {
			params = append(params, "console")
		}
		if scopes.ContainsScope(allScopes, scopes.ScopeServerStatus) {
			params = append(params, "status")
		}
		if scopes.ContainsScope(allScopes, scopes.ScopeServerStats) {
			params = append(params, "stats")
		}
		resolvedPath = resolvedPath + "?" + strings.Join(params, "&")

		proxySocketRequest(c, resolvedPath, ns, node)
	} else {
		proxyHttpRequest(c, resolvedPath, ns, node)
	}

	c.Abort()
}

var wsupgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func proxyHttpRequest(c *gin.Context, path string, ns *services.Node, node *models.Node) {
	// If it's a local node, check if the server is a ghost (in DB but not on disk)
	if node.IsLocal() {
		serverId := c.Param("serverId")
		if servers.GetFromCache(serverId) == nil {
			if c.Request.Method == "GET" {
				if strings.HasSuffix(path, "/stats") {
					c.JSON(http.StatusOK, SkyPanel.ServerStats{Running: false})
					return
				}
				if strings.HasSuffix(path, "/status") {
					c.JSON(http.StatusOK, SkyPanel.ServerRunning{Running: false, Installing: false})
					return
				}
				if strings.HasSuffix(path, "/console") {
					c.JSON(http.StatusOK, SkyPanel.ServerLogs{Logs: []byte("")})
					return
				}
			}
			// For ghost servers, return 200 with an error description instead of a red 404
			c.JSON(http.StatusOK, gin.H{
				"error": "El servidor existe en la base de datos pero sus archivos no se encuentran en este nodo. Por favor, reintenta instalarlo o elimínalo.",
			})
			return
		}
	}

	callResponse, err := ns.CallNode(node, c.Request.Method, path, c.Request.Body, c.Request.Header)

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	defer utils.CloseResponse(callResponse)

	// Intercept 404 for stats/status/console to avoid errors in console for "ghost" servers (non-local or fallback)
	if callResponse.StatusCode == http.StatusNotFound && c.Request.Method == "GET" {
		if strings.HasSuffix(path, "/stats") {
			c.JSON(http.StatusOK, SkyPanel.ServerStats{Running: false})
			return
		}
		if strings.HasSuffix(path, "/status") {
			c.JSON(http.StatusOK, SkyPanel.ServerRunning{Running: false, Installing: false})
			return
		}
		if strings.HasSuffix(path, "/console") {
			c.JSON(http.StatusOK, SkyPanel.ServerLogs{Logs: []byte("")})
			return
		}
	}

	newHeaders := cleanHttpReturnErrors(callResponse.Header)

	c.DataFromReader(callResponse.StatusCode, callResponse.ContentLength, callResponse.Header.Get("Content-Type"), callResponse.Body, newHeaders)
}

func cleanHttpReturnErrors(currentHeaders http.Header) map[string]string {
	//Even though apache isn't going to be in place, we can't set certain headers
	newHeaders := make(map[string]string)
	for k, v := range currentHeaders {
		switch k {
		case "Transfer-Encoding":
		case "Content-Type":
		case "Content-Length":
			continue
		default:
			newHeaders[k] = strings.Join(v, ", ")
		}
	}
	return newHeaders
}

func proxySocketRequest(c *gin.Context, path string, ns *services.Node, node *models.Node) {
	if node.IsLocal() {
		serverId := c.Param("serverId")
		// Check if it's a ghost server
		if servers.GetFromCache(serverId) == nil {
			// Upgrade the connection normally to avoid red 404/204 console errors
			conn, err := wsupgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				return
			}
			// Enviar un mensaje de advertencia simulado
			msg := SkyPanel.ServerLogs{
				Logs: []byte("> Error: Los archivos de este servidor han desaparecido del nodo. No se puede conectar a la consola."),
			}
			data, _ := json.Marshal(msg)
			_ = conn.WriteMessage(websocket.TextMessage, data)

			// Esperar un segundo y cerrar
			time.Sleep(1 * time.Second)
			_ = conn.Close()
			return
		}

		//have gin handle the request again, but send it to daemon instead
		//c.Request.URL.Path = path
		addr, err := url.Parse(path)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		c.Request.URL = addr
		SkyPanel.Engine.HandleContext(c)
	} else {
		err := ns.OpenSocket(node, path, c.Writer, c.Request)
		response.HandleError(c, err, http.StatusInternalServerError)
	}
	c.Abort()
}

func toggleServerSuspension(c *gin.Context) {
	db := middleware.GetDatabase(c)
	server := c.MustGet("server").(*models.Server)

	newState := !server.Suspended

	err := db.Transaction(func(tx *gorm.DB) error {
		server.Suspended = newState
		if err := tx.Save(server).Error; err != nil {
			return err
		}

		var children []models.Server
		if err := tx.Where("parent_server_id = ?", server.Identifier).Find(&children).Error; err != nil {
			return err
		}

		for i := range children {
			children[i].Suspended = newState
			if err := tx.Save(&children[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// If we are suspending, try to stop the server and its children
	if newState {
		ns := &services.Node{DB: db}
		
		ts, err := services.NewTokenService()
		var token string
		if err == nil {
			token, _ = ts.GenerateRequest()
		}

		stopServer := func(srvIdentifier string, nodeID uint) {
			if token != "" {
				node, err := ns.Get(nodeID)
				if err != nil {
					return
				}
				headers := make(http.Header)
				headers.Set("Authorization", "Bearer "+token)
				stopPath := "/daemon/server/" + srvIdentifier + "/stop"
				resp, callErr := ns.CallNode(node, "POST", stopPath, nil, headers)
				if callErr == nil && resp != nil {
					resp.Body.Close()
				}
			}
		}

		// Stop parent
		stopServer(server.Identifier, server.NodeID)

		// Find children and stop them
		var children []models.Server
		if err := db.Where("parent_server_id = ?", server.Identifier).Find(&children).Error; err == nil {
			for _, child := range children {
				stopServer(child.Identifier, child.NodeID)
			}
		}
	}

	c.JSON(http.StatusOK, models.FromServer(server))
}

func getServerFromGin(c *gin.Context) *models.Server {
	return c.MustGet("server").(*models.Server)
}

func checkGhost(s *models.Server, v *models.ServerView) {
	if s.Node.IsLocal() && servers.GetFromCache(s.Identifier) == nil {
		v.IsGhost = true
	}
}
