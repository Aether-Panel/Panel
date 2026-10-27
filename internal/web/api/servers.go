package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/extensions"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// literals shared by the server handlers
const (
	// contentTypeJSON is the media type of JSON bodies
	contentTypeJSON = "application/json"
	// parentServerIDQuery picks the sub-servers of a server
	parentServerIDQuery = "parent_server_id = ?"
	// serverIdentifierRefQuery picks the rows that point at a server by its identifier
	serverIdentifierRefQuery = "server_identifier = ?"
	// serverIDQuery picks the rows that belong to a server
	serverIDQuery       = "server_id = ?"
	bearerPrefix        = "Bearer "
	fileNameParam       = "?fileName="
	statusSuffix        = "/status"
	daemonParseErrorFmt = "Failed to parse response from daemon\n%s"
	daemonUnexpectedFmt = "Unexpected response from daemon: %+v\n%s"
)

func registerServers(g *gin.RouterGroup) {
	// every change made through these routes lands in the server's activity log
	g.Use(recordServerActivity)
	registerServerActivity(g)
	registerServerDNS(g)
	registerServerModpacks(g)
	registerServerWorldMap(g)
	registerServerFileHistory(g)
	registerServerClone(g)

	g.Handle("GET", "", searchServers)
	g.Handle("OPTIONS", "", response.CreateOptions("GET"))

	g.Handle("GET", "/:serverId", middleware.RequiresPermission(scopes.ScopeServerView), middleware.ResolveServerPanel, getServer)
	g.Handle("PUT", "/:serverId", middleware.RequiresPermission(scopes.ScopeServerCreate), middleware.HasTransaction, createServer)
	// these two talk to the node outside their own short transactions: stopping a server can
	// take a while, and holding a request-wide transaction meanwhile blocks SQLite installs
	g.Handle("DELETE", "/:serverId", middleware.RequiresPermission(scopes.ScopeServerDelete), middleware.ResolveServerPanel, deleteServer)
	g.Handle("POST", "/:serverId/suspend", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, toggleServerSuspension)
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

	g.Handle("PUT", "/:serverId/owner/:email", middleware.RequiresPermission(scopes.ScopeAdmin), middleware.ResolveServerPanel, middleware.HasTransaction, setServerOwner)
	g.Handle("OPTIONS", "/:serverId/owner/:email", response.CreateOptions("PUT"))

	g.GET("/:serverId/data", middleware.RequiresPermission(scopes.ScopeServerViewData), middleware.ResolveServerPanel, proxyServerRequest)
	g.POST("/:serverId/data", middleware.RequiresPermission(scopes.ScopeServerEditData), middleware.ResolveServerPanel, editServerData)
	g.PUT("/:serverId/data", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, editServerDataAdmin)
	g.OPTIONS("/:serverId/data", response.CreateOptions("GET", "POST", "PUT"))

	g.PUT("/:serverId/port-settings", middleware.RequiresPermission(scopes.ScopeServerEditData), middleware.ResolveServerPanel, editPortSettings)
	g.OPTIONS("/:serverId/port-settings", response.CreateOptions("PUT"))

	g.POST("/:serverId/transfer", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, transferServer)
	g.OPTIONS("/:serverId/transfer", response.CreateOptions("POST"))

	g.POST("/:serverId/ai/analyze", middleware.RequiresPermission(scopes.ScopeServerConsole), middleware.ResolveServerPanel, analyzeServerLogs)
	g.POST("/:serverId/ai/chat", middleware.RequiresPermission(scopes.ScopeServerConsole), middleware.ResolveServerPanel, chatWithAssistant)
	g.OPTIONS("/:serverId/ai/analyze", response.CreateOptions("POST"))

	g.POST("/:serverId/extransfer/create", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, CreateExTransfer)
	g.OPTIONS("/:serverId/extransfer/create", response.CreateOptions("POST"))

	g.POST("/:serverId/extransfer/pull", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, pullExTransfer)
	g.OPTIONS("/:serverId/extransfer/pull", response.CreateOptions("POST"))

	g.GET("/:serverId/extransfer/status", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, getExTransferStatus)
	g.OPTIONS("/:serverId/extransfer/status", response.CreateOptions("GET"))

	g.POST("/:serverId/extransfer/cancel", middleware.RequiresPermission(scopes.ScopeServerEditDataAdmin), middleware.ResolveServerPanel, cancelExTransferByUser)
	g.OPTIONS("/:serverId/extransfer/cancel", response.CreateOptions("POST"))

	// owners were only ever given the edit permission, and whoever may change a setting may see it
	g.GET("/:serverId/flags", middleware.RequiresAnyPermission(scopes.ScopeServerViewFlags, scopes.ScopeServerEditFlags), middleware.ResolveServerPanel, proxyServerRequest)
	g.POST("/:serverId/flags", middleware.RequiresPermission(scopes.ScopeServerEditFlags), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/flags", response.CreateOptions("GET", "POST"))

	g.GET("/:serverId/tasks", middleware.RequiresPermission(scopes.ScopeServerTaskView), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/tasks", response.CreateOptions("GET"))

	g.GET("/:serverId/tasks/:taskId", middleware.RequiresPermission(scopes.ScopeServerTaskView), middleware.ResolveServerPanel, proxyServerRequest)
	// one PUT both creates and edits, so either permission lets it through; server owners were
	// only ever given the create one
	g.PUT("/:serverId/tasks/:taskId", middleware.RequiresAnyPermission(scopes.ScopeServerTaskCreate, scopes.ScopeServerTaskEdit), middleware.ResolveServerPanel, putServerTask)
	g.DELETE("/:serverId/tasks/:taskId", middleware.RequiresPermission(scopes.ScopeServerTaskDelete), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/tasks/:taskId", response.CreateOptions("GET", "PUT", "DELETE"))

	g.POST("/:serverId/tasks/:taskId/run", middleware.RequiresPermission(scopes.ScopeServerTaskRun), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/tasks/:taskId/run", response.CreateOptions("POST"))

	// crash reports carry console output, so they need the same permission as the console
	g.GET("/:serverId/crashes", middleware.RequiresPermission(scopes.ScopeServerConsole), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/crashes", response.CreateOptions("GET"))

	g.GET("/:serverId/metrics", middleware.RequiresPermission(scopes.ScopeServerStats), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/metrics", response.CreateOptions("GET"))

	g.GET("/:serverId/version", middleware.RequiresPermission(scopes.ScopeServerView), middleware.ResolveServerPanel, proxyServerRequest)
	// changing the version reinstalls the server, so it needs the same permission as reinstalling
	g.POST("/:serverId/version", requiresExperimental(config.ExperimentalVersionSwitch), middleware.RequiresPermission(scopes.ScopeServerInstall), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/version", response.CreateOptions("GET", "POST"))

	// a server's own Discord alerts sit with its other owner settings (auto start, auto restart)
	g.GET("/:serverId/notifications", middleware.RequiresPermission(scopes.ScopeServerEditFlags), middleware.ResolveServerPanel, proxyServerRequest)
	g.PUT("/:serverId/notifications", middleware.RequiresPermission(scopes.ScopeServerEditFlags), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/notifications", response.CreateOptions("GET", "PUT"))
	g.POST("/:serverId/notifications/test", middleware.RequiresPermission(scopes.ScopeServerEditFlags), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/notifications/test", response.CreateOptions("POST"))

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
	g.GET("/:serverId/backup/:backupID", middleware.RequiresPermission(scopes.ScopeServerBackupView), middleware.ResolveServerPanel, getBackup)
	g.DELETE("/:serverId/backup/:backupID", middleware.RequiresPermission(scopes.ScopeServerBackupDelete), middleware.ResolveServerPanel, deleteBackup)
	g.OPTIONS("/:serverId/backup/:backupID", response.CreateOptions("GET", "DELETE"))
	g.POST("/:serverId/backup/create", middleware.RequiresPermission(scopes.ScopeServerBackupCreate), middleware.ResolveServerPanel, createBackup)
	g.OPTIONS("/:serverId/backup/create", response.CreateOptions("POST"))
	g.POST("/:serverId/backup/restore/:backupID", middleware.RequiresPermission(scopes.ScopeServerBackupRestore), middleware.ResolveServerPanel, restoreBackup)
	g.OPTIONS("/:serverId/backup/restore/:backupID", response.CreateOptions("POST"))
	g.GET("/:serverId/backup/download/:backupID", middleware.RequiresPermission(scopes.ScopeServerBackupView), middleware.ResolveServerPanel, downloadBackup)
	g.OPTIONS("/:serverId/backup/download/:backupID", response.CreateOptions("GET"))

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
	db := middleware.GetDatabase(c)
	ss := &services.Server{DB: db}
	ps := &services.Permission{DB: db}

	username := c.DefaultQuery("username", "")
	nodeQuery := c.DefaultQuery("node", "0")
	nameFilter := c.DefaultQuery("name", "*")
	pageSizeQuery := c.DefaultQuery("limit", strconv.Itoa(DefaultPageSize))
	pageQuery := c.DefaultQuery("page", strconv.Itoa(1))

	pageSize, page, node, err := parseSearchQueryParams(pageSizeQuery, pageQuery, nodeQuery)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	user := c.MustGet("user").(*models.User)
	userScopes := getUserScopesForSearch(user, ps)
	isAdmin := scopes.ContainsScope(userScopes, scopes.ScopeAdmin)

	if !isAdmin && username != "" && user.Username != username {
		c.JSON(http.StatusOK, &models.ServerSearchResponse{
			Servers: []*models.ServerView{},
			Metadata: &skypanel.Metadata{Paging: &skypanel.Paging{
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

	results, total, err := ss.Search(services.ServerSearch{
		Username: username,
		NodeID:   uint(node),
		Name:     nameFilter,
		PageSize: uint(pageSize),
		Page:     uint(page),
	})
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	data := processServerSearchResults(results, isAdmin, user, ps, userScopes)

	c.JSON(http.StatusOK, &models.ServerSearchResponse{
		Servers: data,
		Metadata: &skypanel.Metadata{Paging: &skypanel.Paging{
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
		// what the caller may really do here: their rows, plus the owner's set when it is
		// theirs (resolved by the permission check that let this request in)
		perms = &models.PermissionView{
			ServerIdentifier: server.Identifier,
			Scopes:           callerScopes(c),
			Owner:            server.OwnerID != nil && *server.OwnerID == u.ID,
		}
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
	if u.RoleID != nil && u.Role.ID != 0 {
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
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}
	us := &services.User{DB: db}
	ps := &services.Permission{DB: db}

	serverID, ok := resolveCreateServerID(c)
	if !ok {
		return
	}

	postBody := &models.ServerCreation{}
	if err := c.ShouldBindJSON(&postBody); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	postBody.Identifier = serverID

	user := c.MustGet("user").(*models.User)
	if !scopes.ContainsScope(getUserScopesForSearch(user, ps), scopes.ScopeAdmin) {
		if err := restrictSubServerCreation(ps, user, postBody); response.HandleError(c, err, http.StatusForbidden) {
			return
		}
	}

	server, users, err := validateAndBuildServerCreation(c, db, us, postBody)
	if err != nil {
		return
	}
	server.OwnerID, err = ownerOfNewServer(db, user, users, server.ParentServerID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	node, err := ns.Get(postBody.NodeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.HandleError(c, skypanel.ErrNodeInvalid, http.StatusBadRequest)
		return
	} else if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	var parentAvailableCPU int
	var parentAvailableMemory int64
	var parentAvailableDisk int64

	err = insertServerWithinParentLimits(db, server, &parentAvailableCPU, &parentAvailableMemory, &parentAvailableDisk)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if err := assignServerPermissions(ps, server.Identifier, users); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	// the owner is a member too, also when it came from the parent server
	if server.OwnerID != nil {
		if err := ps.SetServerOwner(server.Identifier, *server.OwnerID); response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	}

	if err := notifyNodeOfServerCreation(c, ns, node, postBody, server); err != nil {
		return
	}

	if hasParentServer(server) {
		updateParentServerLimitsOnNode(c, db, ns, node, server, parentAvailableCPU, parentAvailableMemory, parentAvailableDisk)
	}

	sendServerCreationEmails(users, server)
	// a new server gets its subdomain once; editing it later must not give back one the owner removed
	scheduleAutoSubdomain(server.Identifier)
	extensions.Publish(extensions.EventServerCreated, extensions.ServerData(server.Identifier, server.Name))
	c.JSON(http.StatusOK, &models.CreateServerResponse{ID: serverID})
}

// resolveCreateServerID returns the identifier requested in the path, or a
// freshly generated one when the path leaves it empty. It writes the error
// response itself and returns false when the request must stop.
func resolveCreateServerID(c *gin.Context) (string, bool) {
	serverID := c.Param("serverId")
	if serverID == "" {
		generated, err := models.GenerateServerIdentifier()
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return "", false
		}
		return generated, true
	}
	if !models.IsValidServerIdentifier(serverID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id: only letters, digits, '-' and '_' are allowed (1-20 characters, must not start with '-' or '_')"})
		return "", false
	}
	return serverID, true
}

// hasParentServer reports whether the server is a sub-server.
func hasParentServer(server *models.Server) bool {
	return server.ParentServerID != nil && *server.ParentServerID != ""
}

// insertServerWithinParentLimits creates the server row; for a sub-server the
// parent's remaining resources are checked (and locked) in the same transaction.
func insertServerWithinParentLimits(db *gorm.DB, server *models.Server, parentAvailableCPU *int, parentAvailableMemory, parentAvailableDisk *int64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if hasParentServer(server) {
			if err := checkParentServerLimits(tx, server, parentAvailableCPU, parentAvailableMemory, parentAvailableDisk); err != nil {
				return err
			}
		}
		return tx.Create(server).Error
	})
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
	oldPort, oldType := server.Port, server.Type
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

	// the environment decides what the container may touch on the node (host mounts,
	// privileged mode, running on the host itself): only an administrator may change it
	if !scopes.ContainsScope(callerScopes(c), scopes.ScopeServerEditDataAdmin) {
		current, err := currentDefinition(c, ns, server)
		if response.HandleError(c, err, http.StatusBadGateway) {
			return
		}
		postBody.Environment = lockEnvironment(current.Environment, postBody.Environment)
	}

	err = ss.Update(server)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	data, _ := json.Marshal(postBody)
	reader := io.NopCloser(bytes.NewReader(data))

	ctx, cancel := nodeCallContext(c)
	defer cancel()
	nodeResponse, err := ns.CallNodeContext(ctx, &server.Node, "PUT", daemonServerPrefix+postBody.Identifier+"/definition", reader, c.Request.Header)
	defer utils.CloseResponse(nodeResponse)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if nodeResponse.StatusCode != http.StatusNoContent {
		resData, err := io.ReadAll(nodeResponse.Body)
		if err != nil {
			logging.Error.Printf(daemonParseErrorFmt, err.Error())
		}
		logging.Error.Printf(daemonUnexpectedFmt, nodeResponse.StatusCode, string(resData))
		// forward the daemon's answer as it is, once: answering again after it would put two
		// JSON documents in one body. The recorded error rolls the transaction back.
		c.Header(headerContentType, contentTypeJSON)
		c.Status(nodeResponse.StatusCode)
		_, _ = c.Writer.Write(resData)
		_ = c.Error(errors.New("unexpected response from daemon"))
		c.Abort()
		return
	}

	// the SRV record carries the main port, and the type decides whether there is one
	if server.Port != oldPort || server.Type != oldType {
		scheduleSubdomainRefresh(server.Identifier)
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
	db := middleware.GetDatabase(c)
	ss := &services.Server{DB: db}
	ns := &services.Node{DB: db}

	server := getServerFromGin(c)
	node := &server.Node

	users, err := getImpactedUsersByServerDeletion(db, server.Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	var childrenCount int64
	if err := db.Model(&models.Server{}).Where(parentServerIDQuery, server.Identifier).Count(&childrenCount).Error; err == nil && childrenCount > 0 {
		response.HandleError(c, errors.New("cannot delete server because it has active child servers"), http.StatusBadRequest)
		return
	}

	_, skipNode := c.GetQuery("skipNode")
	if !skipNode {
		if err := shutdownServerOnNodeBeforeDeletion(ns, node, server.Identifier); response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}

		if err := executeServerDeletionOnNode(c, ns, node, server.Identifier); err != nil {
			return
		}
	}

	releaseSubdomain(db, server)

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(serverIdentifierRefQuery, server.Identifier).Delete(&models.Permissions{}).Error; err != nil {
			return err
		}

		var clientIDs []uint
		if err := tx.Model(&models.Client{}).Where(serverIDQuery, server.Identifier).Pluck("id", &clientIDs).Error; err == nil && len(clientIDs) > 0 {
			if err := tx.Where("client_id IN ?", clientIDs).Delete(&models.Permissions{}).Error; err != nil {
				return err
			}
		}

		if err := tx.Where(serverIDQuery, server.Identifier).Delete(&models.UptimeStatus{}).Error; err != nil {
			return err
		}
		if err := tx.Where(serverIDQuery, server.Identifier).Delete(&models.Backup{}).Error; err != nil {
			return err
		}
		if err := tx.Where(serverIDQuery, server.Identifier).Delete(&models.Client{}).Error; err != nil {
			return err
		}
		if err := tx.Where(serverIdentifierRefQuery, server.Identifier).Delete(&models.Session{}).Error; err != nil {
			return err
		}

		return tx.Delete(server).Error
	}); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	_ = ss.Delete(server.Identifier)

	if server.ParentServerID != nil && *server.ParentServerID != "" {
		updateParentServerLimitsAfterDeletion(db, ns, node, server)
	}

	sendServerDeletionEmails(users, server)
	extensions.Publish(extensions.EventServerDeleted, extensions.ServerData(server.Identifier, server.Name))
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

	users := groupScopesByUser(perms)

	data := make([]*models.UserPermissionsView, 0)
	for k, v := range users {
		view := &models.UserPermissionsView{
			Username: k.Username,
			Email:    k.Email,
			Scopes:   v,
		}
		if server.OwnerID != nil && *server.OwnerID == k.ID {
			view.Owner = true
			for _, s := range scopes.OwnerScopes {
				view.Scopes = scopes.AddScope(view.Scopes, s)
			}
		}
		data = append(data, view)
	}

	c.JSON(http.StatusOK, data)
}

// groupScopesByUser merges the permission rows that belong to the same user
// into a single entry with all of their scopes.
func groupScopesByUser(perms []*models.Permissions) map[*models.User][]*scopes.Scope {
	users := map[*models.User][]*scopes.Scope{}

	for _, v := range perms {
		p := make([]*scopes.Scope, 0)
		for z, r := range users {
			if v.User.ID == z.ID {
				// this is the user
				p = r
				break
			}
		}
		p = append(p, v.Scopes...)

		found := false
		for z := range users {
			if v.User.ID == z.ID {
				// this is the user
				users[z] = p
				found = true
				break
			}
		}
		if !found {
			users[&v.User] = p
		}
	}
	return users
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

	user, registerToken, ok := findOrInviteServerUser(c, us, email, username)
	if !ok {
		return
	}

	if server.OwnerID != nil && *server.OwnerID == user.ID {
		response.HandleError(c, skypanel.ErrServerOwner, http.StatusConflict)
		return
	}

	existing, err := ps.GetForUserAndServer(user.ID, server.Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	firstTimeAccess := existing.ID == 0

	// update perms to match this "setup", but not stomp over what the user can't change
	if canGrantAnyServerScope(currentPerms, currentGlobalPerms) {
		existing.Scopes = perms.Scopes
	} else {
		// nobody gives more than they hold on this server: the owner holds OwnerScopes, a
		// sub-user only what was given to them
		replacement := scopes.UpdateScopesWhereGranted(existing.Scopes, perms.Scopes, callerScopes(c))
		existing.Scopes = replacement
	}

	err = ps.UpdatePermissions(existing)

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// now we can send emails to the people
	if firstTimeAccess {
		sendAddedToServerEmail(user, server, registerToken)
	}

	c.Status(http.StatusNoContent)
}

// findOrInviteServerUser looks the target user up by email or username; an
// unknown email gets a new invited account plus its register token. It writes
// the error response itself and returns false when the request must stop.
func findOrInviteServerUser(c *gin.Context, us *services.User, email, username string) (*models.User, string, bool) {
	var user *models.User
	var err error
	if email != "" {
		user, err = us.GetByEmail(email)
	} else {
		user, err = us.Get(username)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if email == "" {
			response.HandleError(c, err, http.StatusBadRequest)
			return nil, "", false
		}
		// we need to create the user here, since it's a new email we've not seen
		invited, registerToken, err := createInvitedUser(us, email)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return nil, "", false
		}
		return invited, registerToken, true
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return nil, "", false
	}
	return user, "", true
}

// createInvitedUser creates a placeholder account for a new email; the
// returned token is its initial password and goes out in the invite email.
func createInvitedUser(us *services.User, email string) (*models.User, string, error) {
	un, err := uuid.NewV4()
	if err != nil {
		return nil, "", err
	}
	user := &models.User{
		Username: un.String(),
		Email:    email,
	}
	token, err := uuid.NewV4()
	if err != nil {
		return nil, "", err
	}
	registerToken := token.String()
	if err = user.SetPassword(registerToken); err != nil {
		return nil, "", err
	}
	if err = us.Create(user); err != nil {
		return nil, "", err
	}
	return user, registerToken, nil
}

// canGrantAnyServerScope reports whether the caller may set every scope on a
// server user instead of only the ones they hold themselves.
func canGrantAnyServerScope(serverPerms, globalPerms *models.Permissions) bool {
	return scopes.ContainsScope(serverPerms.Scopes, scopes.ScopeServerAdmin) ||
		scopes.ContainsScope(globalPerms.Scopes, scopes.ScopeServerAdmin) ||
		scopes.ContainsScope(globalPerms.Scopes, scopes.ScopeAdmin)
}

func sendAddedToServerEmail(user *models.User, server *models.Server, registerToken string) {
	es := services.GetEmailService()
	err := es.SendEmail(user.Email, "addedToServer", map[string]interface{}{
		"Server":        server,
		"RegisterToken": registerToken,
		"Email":         user.Email,
	}, true)
	if err != nil {
		// since we don't want to tell the user it failed, we'll log and move on
		logging.Error.Printf("Error sending email: %s\n", err)
	}
}

// @Summary Give a server to another user
// @Description Makes the user the server's owner (they also become a member). The previous
// @Description owner stays as a sub-user with the scopes their row already had.
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param email path string true "Email of the new owner"
// @Tags Daemon Servers
// @Router /api/servers/{id}/owner/{email} [put]
// @Security OAuth2Application[admin]
func setServerOwner(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}
	ps := &services.Permission{DB: db}
	server := getServerFromGin(c)

	user, err := us.GetByEmail(c.Param("email"))
	if response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	if response.HandleError(c, ps.SetServerOwner(server.Identifier, user.ID), http.StatusInternalServerError) {
		return
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

	if server.OwnerID != nil && *server.OwnerID == user.ID {
		response.HandleError(c, skypanel.ErrServerOwner, http.StatusConflict)
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

	es := services.GetEmailService()
	err = es.SendEmail(user.Email, "removedFromServer", map[string]interface{}{
		"Server": server,
	}, true)
	if err != nil {
		// since we don't want to tell the user it failed, we'll log and move on
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

	postBody, ok := readAndRestoreJSONBody(c)
	if !ok {
		return
	}
	oldPort := server.Port

	dirty, ok := applyAddressFields(c, server, postBody)
	if !ok {
		return
	}

	resourcesChanged, ok := applyResourceFields(c, server, postBody)
	if !ok {
		return
	}
	dirty = dirty || resourcesChanged

	// Extra ports management: the frontend sends "ports" (full list) and
	// "primaryPort" (which one is primary). These are mirrored to the daemon
	// as the port/port2/port3... variables so the container binds them.
	db := middleware.GetDatabase(c)
	portsChanged, ok := applyPortListFields(c, db, server, postBody)
	if !ok {
		return
	}
	dirty = dirty || portsChanged

	// Port notes: user-defined labels per port (e.g. "Game", "RCON", "Query")
	if notes, hasNotes := parsePortNotes(postBody); hasNotes {
		server.PortNotes = notes
		dirty = true
	}

	// limits are checked before anything is sent or saved
	limits, ok := checkAdminResourceChange(c, db, server, resourcesChanged)
	if !ok {
		return
	}
	if limits.childrenUse != nil {
		// a parent's container only gets what its sub-servers leave free
		limitsForParentNode(postBody, server, limits.childrenUse)
	}

	// The node first: if it refuses (or is down), the panel must not show ports and limits
	// the server does not have. The ports/primaryPort fields stay in the body so the daemon
	// can reconcile its port/port2/... variables (removing stale ones too).
	if !sendDataToNode(c, db, server, postBody) {
		return
	}

	if dirty && !saveAdminServerChanges(c, db, server, resourcesChanged) {
		return
	}
	if server.Port != oldPort {
		scheduleSubdomainRefresh(server.Identifier)
	}
	c.Status(http.StatusNoContent)
}

// resourceUse is the cpu, memory and disk a set of servers holds
type resourceUse struct {
	cpu          int
	memory, disk int64
}

// adminResourceLimits is what checkAdminResourceChange found: for a server with sub-servers,
// what they hold (nil otherwise)
type adminResourceLimits struct {
	childrenUse *resourceUse
}

// checkAdminResourceChange refuses new limits that do not fit: a sub-server must fit in what
// its parent has left, and a parent cannot go below what its sub-servers already hold.
// ok is false once an error response has been written.
func checkAdminResourceChange(c *gin.Context, db *gorm.DB, server *models.Server, resourcesChanged bool) (adminResourceLimits, bool) {
	var limits adminResourceLimits
	if !resourcesChanged {
		return limits, true
	}
	if hasParentServer(server) {
		var cpu int
		var memory, disk int64
		err := db.Transaction(func(tx *gorm.DB) error {
			return checkParentServerLimitsOnUpdate(tx, server, &cpu, &memory, &disk)
		})
		return limits, !response.HandleError(c, err, http.StatusBadRequest)
	}

	use, err := childrenResourceUse(db, server.Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return limits, false
	}
	if use == nil {
		return limits, true
	}
	if server.TotalCPU < use.cpu || server.TotalMemory < use.memory || server.TotalDisk < use.disk {
		response.HandleError(c, errors.New("the sub-servers of this server already use more than that"), http.StatusBadRequest)
		return limits, false
	}
	limits.childrenUse = use
	return limits, true
}

// childrenResourceUse adds up the limits of a server's sub-servers, nil when it has none
func childrenResourceUse(db *gorm.DB, identifier string) (*resourceUse, error) {
	var children []*models.Server
	if err := db.Where(parentServerIDQuery, identifier).Find(&children).Error; err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return nil, nil
	}
	use := &resourceUse{}
	for _, child := range children {
		use.cpu += child.TotalCPU
		use.memory += child.TotalMemory
		use.disk += child.TotalDisk
	}
	return use, nil
}

// limitsForParentNode rewrites the resource fields sent to the node: the panel stores the
// parent's whole plan, but its container must only get what the sub-servers leave free
func limitsForParentNode(postBody map[string]interface{}, server *models.Server, children *resourceUse) {
	free := parentLimitsData(server.TotalCPU-children.cpu, server.TotalMemory-children.memory, server.TotalDisk-children.disk)
	for _, key := range []string{"cpu", "memory", "disk"} {
		if _, sent := postBody[key]; sent {
			postBody[key] = free[key]
		}
	}
}

// parentLimitsData is the cpu, memory and disk a parent keeps for itself. Docker treats 0 as
// "no limit", so a parent whose plan is fully split keeps the smallest real limit instead.
func parentLimitsData(cpu int, memory, disk int64) map[string]interface{} {
	return map[string]interface{}{
		"cpu":    max(cpu, 1),
		"memory": max(memory, 1),
		"disk":   max(disk, 1),
	}
}

// sendDataToNode sends the edited data to the server's node and answers with the node's
// refusal when it does not accept it. It returns false once a response has been written.
func sendDataToNode(c *gin.Context, db *gorm.DB, server *models.Server, postBody map[string]interface{}) bool {
	body, err := json.Marshal(postBody)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return false
	}
	ns := &services.Node{DB: db}
	ctx, cancel := nodeCallContext(c)
	defer cancel()
	res, err := ns.CallNodeContext(ctx, &server.Node, "PUT", daemonServerPrefix+server.Identifier+"/data", io.NopCloser(bytes.NewReader(body)), c.Request.Header)
	defer utils.CloseResponse(res)
	if response.HandleError(c, err, http.StatusBadGateway) {
		return false
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		details, _ := io.ReadAll(res.Body)
		c.Header(headerContentType, res.Header.Get(headerContentType))
		c.Status(res.StatusCode)
		_, _ = c.Writer.Write(details)
		c.Abort()
		return false
	}
	return true
}

// readAndRestoreJSONBody decodes the request body as a JSON object and puts an
// unread copy back on the request, so it can still be proxied to the daemon.
func readAndRestoreJSONBody(c *gin.Context) (map[string]interface{}, bool) {
	useHere := &bytes.Buffer{}
	useThere := &bytes.Buffer{}

	multi := io.MultiWriter(useHere, useThere)
	_, err := io.Copy(multi, c.Request.Body)
	if err != nil && response.HandleError(c, err, http.StatusBadRequest) {
		return nil, false
	}

	_ = c.Request.Body.Close()
	c.Request.Body = io.NopCloser(useThere)

	var postBody map[string]interface{}
	err = json.NewDecoder(useHere).Decode(&postBody)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return nil, false
	}
	return postBody, true
}

// applyAddressFields copies the port and ip fields onto the server. The bool
// pair is (changed, ok); ok is false once an error response has been written.
func applyAddressFields(c *gin.Context, server *models.Server, postBody map[string]interface{}) (bool, bool) {
	dirty := false
	if port, exist := postBody["port"]; exist {
		portVal, err := cast.ToUint16E(port)
		if response.HandleError(c, err, http.StatusBadRequest) {
			return false, false
		}
		server.Port = portVal
		dirty = true
	}

	if ip, exist := postBody["ip"]; exist {
		server.IP = cast.ToString(ip)
		dirty = true
	}
	return dirty, true
}

// applyResourceFields copies the cpu, memory and disk limits onto the server.
// The bool pair is (changed, ok); ok is false once an error response has been
// written.
func applyResourceFields(c *gin.Context, server *models.Server, postBody map[string]interface{}) (bool, bool) {
	changed := false
	if raw, exist := postBody["cpu"]; exist {
		newCPU, err := parseResourceValue(raw, cast.ToIntE)
		if response.HandleError(c, err, http.StatusBadRequest) {
			return false, false
		}
		server.TotalCPU = newCPU
		changed = true
	}
	if raw, exist := postBody["memory"]; exist {
		newMemory, err := parseResourceValue(raw, cast.ToInt64E)
		if response.HandleError(c, err, http.StatusBadRequest) {
			return false, false
		}
		server.TotalMemory = newMemory
		changed = true
	}
	if raw, exist := postBody["disk"]; exist {
		newDisk, err := parseResourceValue(raw, cast.ToInt64E)
		if response.HandleError(c, err, http.StatusBadRequest) {
			return false, false
		}
		server.TotalDisk = newDisk
		changed = true
	}
	return changed, true
}

// parseResourceValue converts a resource limit from the request body and
// rejects negative amounts.
func parseResourceValue[T int | int64](raw interface{}, convert func(interface{}) (T, error)) (T, error) {
	value, err := convert(raw)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, errors.New("resources cannot be negative")
	}
	return value, nil
}

// applyPortListFields applies the "ports" and "primaryPort" fields: the list
// is validated against the node and the primary port is moved to the front.
// The bool pair is (changed, ok); ok is false once an error response has been
// written.
func applyPortListFields(c *gin.Context, db *gorm.DB, server *models.Server, postBody map[string]interface{}) (bool, bool) {
	portsField, portsExist := postBody["ports"]
	primaryField, primaryExist := postBody["primaryPort"]
	if !portsExist && !primaryExist {
		return false, true
	}

	var ports []uint16
	if portsExist {
		var err error
		ports, err = parsePortList(portsField)
		if err != nil {
			response.HandleError(c, err, http.StatusBadRequest)
			return false, false
		}
	}
	if len(ports) == 0 {
		ports = []uint16{server.Port}
	}

	// the list is never empty here, so the first port is the default primary
	primary := ports[0]
	if primaryExist {
		primary = cast.ToUint16(primaryField)
	}

	if primary > 0 {
		i := portIndex(ports, primary)
		if i < 0 {
			response.HandleError(c, errors.New("primaryPort must be one of the assigned ports"), http.StatusBadRequest)
			return false, false
		}
		ports[0], ports[i] = ports[i], ports[0]
	}

	if err := validatePortsAgainstNode(db, server.NodeID, server.Identifier, ports); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return false, false
	}
	server.Port = ports[0]
	server.Ports = ports
	return true, true
}

var errPortsNotNumbers = errors.New("ports must be an array of numbers")

// parsePortList reads the "ports" array from the request body, dropping zeros.
func parsePortList(raw interface{}) ([]uint16, error) {
	rawList, ok := raw.([]interface{})
	if !ok {
		return nil, errPortsNotNumbers
	}
	var ports []uint16
	for _, item := range rawList {
		p, err := cast.ToUint16E(item)
		if err != nil {
			return nil, errPortsNotNumbers
		}
		if p > 0 {
			ports = append(ports, p)
		}
	}
	return ports, nil
}

// portIndex returns the position of port in ports, or -1 when it is missing.
func portIndex(ports []uint16, port uint16) int {
	for i, p := range ports {
		if p == port {
			return i
		}
	}
	return -1
}

// parsePortNotes reads the user-defined labels per port from the body; false
// when the body carries no portNotes object.
func parsePortNotes(postBody map[string]interface{}) (map[string]string, bool) {
	notesMap, ok := postBody["portNotes"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	converted := make(map[string]string, len(notesMap))
	for k, v := range notesMap {
		if str, ok := v.(string); ok {
			converted[k] = str
		}
	}
	return converted, true
}

// saveAdminServerChanges stores the edited server. Resource changes on a
// sub-server are checked again against the parent's limits in the same
// transaction (another edit may have landed since), and the parent's new limits
// are pushed to its node afterwards. It returns false once an error response has
// been written.
func saveAdminServerChanges(c *gin.Context, db *gorm.DB, server *models.Server, resourcesChanged bool) bool {
	if !resourcesChanged || !hasParentServer(server) {
		ss := &services.Server{DB: db}
		err := ss.Update(server)
		return !response.HandleError(c, err, http.StatusInternalServerError)
	}

	var parentAvailableCPU int
	var parentAvailableMemory, parentAvailableDisk int64

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := checkParentServerLimitsOnUpdate(tx, server, &parentAvailableCPU, &parentAvailableMemory, &parentAvailableDisk); err != nil {
			return err
		}
		ss := &services.Server{DB: tx}
		return ss.Update(server)
	})
	if response.HandleError(c, err, http.StatusBadRequest) {
		return false
	}

	node, nodeErr := (&services.Node{DB: db}).Get(server.NodeID)
	if nodeErr == nil && node != nil {
		updateParentServerLimitsOnNode(c, db, &services.Node{DB: db}, node, server, parentAvailableCPU, parentAvailableMemory, parentAvailableDisk)
	}
	return true
}

// editPortSettings allows users to manage port metadata (primary port + notes)
// without needing admin-level port editing permissions.
func editPortSettings(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)

	var postBody map[string]interface{}
	if err := c.ShouldBindJSON(&postBody); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	dirty := false
	portChanged := false

	// Primary port: user selects which port is primary (reorders the list)
	if primaryField, has := postBody["primaryPort"]; has && promotePrimaryPort(server, cast.ToUint16(primaryField)) {
		dirty = true
		portChanged = true
	}

	// Port notes: user-defined labels per port
	if notes, hasNotes := parsePortNotes(postBody); hasNotes {
		server.PortNotes = notes
		dirty = true
	}

	if dirty {
		ss := &services.Server{DB: db}
		if err := ss.Update(server); err != nil {
			response.HandleError(c, err, http.StatusInternalServerError)
			return
		}
	}

	// Sync port variables (port, port2, port3...) to the daemon so Docker
	// binds all assigned ports on next start.
	syncPortVarsToDaemon(c, server)

	if portChanged {
		scheduleSubdomainRefresh(server.Identifier)
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// promotePrimaryPort moves an already assigned port to the front of the list
// and makes it the main port; false when nothing had to change.
func promotePrimaryPort(server *models.Server, primary uint16) bool {
	if primary == 0 || server.Ports == nil {
		return false
	}
	i := portIndex(server.Ports, primary)
	if i <= 0 {
		return false
	}
	server.Ports[0], server.Ports[i] = server.Ports[i], server.Ports[0]
	server.Port = primary
	return true
}

// syncPortVarsToDaemon sends the current port list to the daemon so it
// creates/updates port, port2, port3... variables. Best-effort: errors are
// logged but not returned to the client.
func syncPortVarsToDaemon(c *gin.Context, server *models.Server) {
	if len(server.Ports) == 0 {
		return
	}
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}

	node := &server.Node
	daemonPath := daemonServerPrefix + server.Identifier + "/data"

	syncBody := map[string]interface{}{
		"ports":       server.Ports,
		"primaryPort": server.Port,
	}
	bodyBytes, err := json.Marshal(syncBody)
	if err != nil {
		return
	}

	ts, err := services.NewTokenService()
	if err != nil {
		return
	}
	token, err := ts.GenerateRequest(node)
	if err != nil {
		return
	}

	headers := http.Header{
		headerContentType: {contentTypeJSON},
		"Authorization":   {bearerPrefix + token},
	}

	resp, err := ns.CallNode(node, http.MethodPut, daemonPath, io.NopCloser(bytes.NewReader(bodyBytes)), headers)
	if err != nil {
		return
	}
	defer resp.Body.Close()
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

	syncAutomaticBackups(c.Request.Context(), db, server)

	records, err := bs.GetAllForServer(server.Identifier)

	if !response.HandleError(c, err, http.StatusInternalServerError) {
		c.JSON(http.StatusOK, records)
	}
}

// @Summary Gets a specific backup on a server
// @Description Gets a specific backup made on this server
// @Success 200 {object} models.Backup
// @Param id path string true "Server ID"
// @Param backupID path string true "BackupId"
// @Tags Daemon Servers
// @Router /api/servers/{id}/backup/{backupID} [get]
// @Security OAuth2Application[server.backup.view]
func getBackup(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	bs := &services.Backup{DB: db}
	backupID, err := cast.ToUintE(c.Param("backupID"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	records, err := bs.Get(server.Identifier, backupID)
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
		response.HandleError(c, skypanel.ErrFieldRequired("name"), http.StatusBadRequest)
		return
	}

	// keep the path escaped: it is parsed again for the node, so a decoded "#", "?" or "%"
	// in a file name would end up as fragment, query or a broken escape
	resolvedPath := daemonServerPrefix + strings.TrimPrefix(c.Request.URL.EscapedPath(), "/api/servers/")
	if c.Request.URL.RawQuery != "" {
		resolvedPath += "?" + c.Request.URL.RawQuery
	}

	callResponse, err := ns.CallNodeContext(c.Request.Context(), node, c.Request.Method, resolvedPath, c.Request.Body, c.Request.Header)
	defer utils.CloseResponse(callResponse)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// any refusal from the node (bad request, server running, backup in progress...) is
	// passed through; only a 400 used to be, so other errors were reported as success
	if callResponse.StatusCode >= http.StatusMultipleChoices {
		newHeaders := cleanHTTPReturnErrors(callResponse.Header)

		c.DataFromReader(callResponse.StatusCode, callResponse.ContentLength, callResponse.Header.Get(headerContentType), callResponse.Body, newHeaders)
		c.Abort()
		return
	}

	responseData := &skypanel.ServerBackupResponse{}
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
// @Param backupID path string true "Backup ID"
// @Tags Daemon Servers
// @Router /api/servers/{id}/backup/Delete/{backupID} [delete]
// @Security OAuth2Application[server.backup.delete]
func deleteBackup(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}
	bs := &services.Backup{DB: db}
	node := &server.Node

	backupID, err := cast.ToUintE(c.Param("backupID"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	backup, err := bs.Get(server.Identifier, backupID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if backup == nil {
		c.Status(http.StatusNotFound)
		return
	}

	resolvedPath := daemonServerPrefix + server.Identifier + "/backup" + fileNameParam + backup.FileName

	callResponse, err := ns.CallNode(node, "DELETE", resolvedPath, nil, nil)
	defer utils.CloseResponse(callResponse)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// any refusal from the node (bad request, server running, backup in progress...) is
	// passed through; only a 400 used to be, so other errors were reported as success
	if callResponse.StatusCode >= http.StatusMultipleChoices {
		newHeaders := cleanHTTPReturnErrors(callResponse.Header)

		c.DataFromReader(callResponse.StatusCode, callResponse.ContentLength, callResponse.Header.Get(headerContentType), callResponse.Body, newHeaders)
		c.Abort()
		return
	}

	err = bs.Delete(backupID)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Restore backup
// @Description Removes all exisiting files and restores the server to the state of the backup
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param backupID path string true "Backup ID"
// @Tags Daemon Servers
// @Router /api/servers/{id}/backup/Delete/{backupID} [delete]
// @Security OAuth2Application[server.backup.restore]
func restoreBackup(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}
	bs := &services.Backup{DB: db}
	node := &server.Node

	backupID, err := cast.ToUintE(c.Param("backupID"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	backup, err := bs.Get(server.Identifier, backupID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if backup == nil {
		c.Status(http.StatusNotFound)
		return
	}

	resolvedPath := daemonServerPrefix + server.Identifier + "/backup/restore" + fileNameParam + backup.FileName

	callResponse, err := ns.CallNode(node, "POST", resolvedPath, nil, nil)
	defer utils.CloseResponse(callResponse)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// any refusal from the node (bad request, server running, backup in progress...) is
	// passed through; only a 400 used to be, so other errors were reported as success
	if callResponse.StatusCode >= http.StatusMultipleChoices {
		newHeaders := cleanHTTPReturnErrors(callResponse.Header)

		c.DataFromReader(callResponse.StatusCode, callResponse.ContentLength, callResponse.Header.Get(headerContentType), callResponse.Body, newHeaders)
		c.Abort()
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Download backup
// @Description Download a server backup
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param backupID path string true "Backup ID"
// @Tags Daemon Servers
// @Router /api/servers/{id}/backup/Delete/{backupID} [delete]
// @Security OAuth2Application[server.backup.restore]
func downloadBackup(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}
	bs := &services.Backup{DB: db}
	node := &server.Node

	backupID, err := cast.ToUintE(c.Param("backupID"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	backup, err := bs.Get(server.Identifier, backupID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if backup == nil {
		c.Status(http.StatusNotFound)
		return
	}

	resolvedPath := daemonServerPrefix + server.Identifier + "/backup/download" + fileNameParam + backup.FileName

	callResponse, err := ns.CallNode(node, "GET", resolvedPath, nil, nil)
	defer utils.CloseResponse(callResponse)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	newHeaders := cleanHTTPReturnErrors(callResponse.Header)

	c.DataFromReader(callResponse.StatusCode, callResponse.ContentLength, callResponse.Header.Get(headerContentType), callResponse.Body, newHeaders)
}

func getFromData(variables map[string]skypanel.Variable, key string) (result interface{}, exists bool) {
	for k, v := range variables {
		if k == key {
			return v.Value, true
		}
	}
	return nil, false
}

func getFromDataOrDefault(variables map[string]skypanel.Variable, key string, val interface{}) (interface{}, error) {
	res, exists := getFromData(variables, key)

	if exists {
		return utils.Convert(res, val)
	}

	return val, nil
}

// collectPortsFromVariables extracts the primary port (port) and any extra
// ports (port2, port3, ...) from the given variables, in order, dropping any
// zero/empty values.
func collectPortsFromVariables(variables map[string]skypanel.Variable) []uint16 {
	var ports []uint16
	for i := 1; ; i++ {
		key := "port"
		if i > 1 {
			key = fmt.Sprintf("port%d", i)
		}
		val, exists := getFromData(variables, key)
		if !exists {
			break
		}
		p := cast.ToUint16(val)
		if p > 0 {
			ports = append(ports, p)
		}
	}
	return ports
}

// validatePortsAgainstNode ensures the given ports are within the allowed
// range, contain no duplicates, and are not already assigned to another server
// on the same node.
func validatePortsAgainstNode(db *gorm.DB, nodeID uint, excludeIdentifier string, ports []uint16) error {
	used, err := portsTakenByOtherServers(db, nodeID, excludeIdentifier)
	if err != nil {
		return err
	}

	seen := make(map[uint16]bool)
	for _, p := range ports {
		if p == 0 {
			continue
		}
		if p < 1024 {
			return fmt.Errorf("el puerto %d está fuera del rango permitido (1024-65535)", p)
		}
		if seen[p] {
			return fmt.Errorf("el puerto %d está duplicado", p)
		}
		if used[p] {
			return fmt.Errorf("el puerto %d ya está en uso por otro servidor en este nodo", p)
		}
		seen[p] = true
		used[p] = true
	}
	return nil
}

// portsTakenByOtherServers collects every port already assigned to the
// servers on the node, skipping the server being edited.
func portsTakenByOtherServers(db *gorm.DB, nodeID uint, excludeIdentifier string) (map[uint16]bool, error) {
	query := db.Model(&models.Server{})
	if nodeID == 0 {
		query = query.Where("node_id IS NULL")
	} else {
		query = query.Where("node_id = ?", nodeID)
	}

	var serversOnNode []*models.Server
	if err := query.Find(&serversOnNode).Error; err != nil {
		return nil, err
	}

	used := make(map[uint16]bool)
	for _, s := range serversOnNode {
		if s.Identifier == excludeIdentifier {
			continue
		}
		if s.Port > 0 {
			used[s.Port] = true
		}
		for _, p := range s.Ports {
			if p > 0 {
				used[p] = true
			}
		}
	}
	return used, nil
}

func proxyServerRequest(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}

	// keep the path escaped: it is parsed again for the node, so a decoded "#", "?" or "%"
	// in a file name would end up as fragment, query or a broken escape
	resolvedPath := daemonServerPrefix + strings.TrimPrefix(c.Request.URL.EscapedPath(), "/api/servers/")
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

	token, err := ts.GenerateRequest(node)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// switch to our token for auth
	c.Request.Header.Set("Authorization", bearerPrefix+token)

	if c.IsWebsocket() {
		socketPath, ok := websocketPathWithGrants(c, db, user, server, resolvedPath)
		if !ok {
			return
		}
		proxySocketRequest(c, socketPath, ns, node)
	} else {
		proxyHTTPRequest(c, resolvedPath, ns, node)
	}

	c.Abort()
}

// websocketPathWithGrants replaces the query of a websocket path with the
// streams (console, status, stats) the user is allowed to receive. It writes
// the error response itself and returns false when the request must stop.
func websocketPathWithGrants(c *gin.Context, db *gorm.DB, user *models.User, server *models.Server, resolvedPath string) (string, bool) {
	// for websocket, nuke the query params to avoid trying to escalate
	resolvedPath = strings.SplitN(resolvedPath, "?", 2)[0]
	if !strings.HasPrefix(resolvedPath, "/") {
		resolvedPath = "/" + resolvedPath
	}

	permService := &services.Permission{DB: db}
	perms, err := permService.GetForUserAndServer(user.ID, server.Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return "", false
	}

	allScopes := perms.Scopes

	perms, err = permService.GetForUserAndServer(user.ID, "")
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return "", false
	}

	allScopes = append(allScopes, perms.Scopes...)

	// add the params we can grant for this request
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
	return resolvedPath + "?" + strings.Join(params, "&"), true
}

var wsupgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

func proxyHTTPRequest(c *gin.Context, path string, ns *services.Node, node *models.Node) {
	// If it's a local node, check if the server is a ghost (in DB but not on disk)
	if answerLocalGhostServer(c, path, node) {
		return
	}

	callResponse, err := ns.CallNodeContext(c.Request.Context(), node, c.Request.Method, path, c.Request.Body, c.Request.Header)

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	defer utils.CloseResponse(callResponse)

	// Intercept 404 for stats/status/console to avoid errors in console for "ghost" servers (non-local or fallback)
	if callResponse.StatusCode == http.StatusNotFound && c.Request.Method == "GET" && writeIdleServerStub(c, path) {
		return
	}

	newHeaders := cleanHTTPReturnErrors(callResponse.Header)

	c.DataFromReader(callResponse.StatusCode, callResponse.ContentLength, callResponse.Header.Get(headerContentType), callResponse.Body, newHeaders)
}

// answerLocalGhostServer answers for a server on the local node that is in the
// database but not on disk; true when the response has been written.
func answerLocalGhostServer(c *gin.Context, path string, node *models.Node) bool {
	if !node.IsLocal() || servers.GetFromCache(c.Param("serverId")) != nil {
		return false
	}
	if c.Request.Method == "GET" && writeIdleServerStub(c, path) {
		return true
	}
	// For ghost servers, return 200 with an error description instead of a red 404
	c.JSON(http.StatusOK, gin.H{
		"error": "El servidor existe en la base de datos pero sus archivos no se encuentran en este nodo. Por favor, reintenta instalarlo o elimínalo.",
	})
	return true
}

// writeIdleServerStub answers the polling endpoints (stats, status, console)
// of a server the node does not have with a stopped, empty payload; false for
// any other path.
func writeIdleServerStub(c *gin.Context, path string) bool {
	switch {
	case strings.HasSuffix(path, "/stats"):
		c.JSON(http.StatusOK, skypanel.ServerStats{Running: false})
	case strings.HasSuffix(path, statusSuffix):
		c.JSON(http.StatusOK, skypanel.ServerRunning{Running: false, Installing: false})
	case strings.HasSuffix(path, "/console"):
		c.JSON(http.StatusOK, skypanel.ServerLogs{Logs: []byte("")})
	default:
		return false
	}
	return true
}

func cleanHTTPReturnErrors(currentHeaders http.Header) map[string]string {
	// Even though apache isn't going to be in place, we can't set certain headers
	newHeaders := make(map[string]string)
	for k, v := range currentHeaders {
		switch k {
		case "Transfer-Encoding":
		case headerContentType:
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
		serverID := c.Param("serverId")
		// Check if it's a ghost server
		if servers.GetFromCache(serverID) == nil {
			// Upgrade the connection normally to avoid red 404/204 console errors
			conn, err := wsupgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				return
			}
			// Enviar un mensaje de advertencia simulado
			msg := skypanel.ServerLogs{
				Logs: []byte("> Error: Los archivos de este servidor han desaparecido del nodo. No se puede conectar a la consola."),
			}
			data, _ := json.Marshal(msg)
			_ = conn.WriteMessage(websocket.TextMessage, data)

			// Esperar un segundo y cerrar
			time.Sleep(1 * time.Second)
			_ = conn.Close()
			return
		}

		// have gin handle the request again, but send it to daemon instead
		// c.Request.URL.Path = path
		addr, err := url.Parse(path)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		c.Request.URL = addr
		skypanel.Engine.HandleContext(c)
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
		if err := tx.Where(parentServerIDQuery, server.Identifier).Find(&children).Error; err != nil {
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

		ts, tsErr := services.NewTokenService()

		stopServer := func(srvIdentifier string, nodeID uint) {
			if tsErr != nil {
				return
			}
			node, err := ns.Get(nodeID)
			if err != nil {
				return
			}
			// each node only accepts tokens minted for it
			token, err := ts.GenerateRequest(node)
			if err != nil {
				return
			}
			headers := make(http.Header)
			headers.Set("Authorization", bearerPrefix+token)
			stopPath := daemonServerPrefix + srvIdentifier + "/stop"
			resp, callErr := ns.CallNode(node, "POST", stopPath, nil, headers)
			if callErr == nil && resp != nil {
				resp.Body.Close()
			}
		}

		// Stop parent
		stopServer(server.Identifier, server.NodeID)

		// Find children and stop them
		var children []models.Server
		if err := db.Where(parentServerIDQuery, server.Identifier).Find(&children).Error; err == nil {
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

// --- Helpers for Search Servers ---

func parseSearchQueryParams(pageSizeQuery, pageQuery, nodeQuery string) (int, int, int, error) {
	pageSize, err := strconv.Atoi(pageSizeQuery)
	if err != nil || pageSize <= 0 {
		return 0, 0, 0, skypanel.ErrFieldTooSmall("pageSize", 0)
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	page, err := strconv.Atoi(pageQuery)
	if err != nil || page <= 0 {
		return 0, 0, 0, skypanel.ErrFieldTooSmall("page", 0)
	}

	node, err := strconv.Atoi(nodeQuery)
	if err != nil || node < 0 {
		return 0, 0, 0, skypanel.ErrFieldTooSmall("nodeId", 0)
	}

	return pageSize, page, node, nil
}

func getUserScopesForSearch(user *models.User, ps *services.Permission) []*scopes.Scope {
	userScopes := make([]*scopes.Scope, 0)
	perms, err := ps.GetForUser(user.ID)
	if err == nil {
		for _, p := range perms {
			for _, s := range p.Scopes {
				userScopes = scopes.AddScope(userScopes, s)
			}
		}
	}
	if user.RoleID != nil && user.Role.ID != 0 {
		for _, s := range user.Role.Scopes {
			userScopes = scopes.AddScope(userScopes, scopes.GetScope(s))
		}
	}
	return userScopes
}

func processServerSearchResults(results []*models.Server, isAdmin bool, user *models.User, ps *services.Permission, userScopes []*scopes.Scope) []*models.ServerView {
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
	return data
}

// --- Helpers for Delete Server ---

func getImpactedUsersByServerDeletion(db *gorm.DB, identifier string) ([]models.User, error) {
	ps := services.Permission{DB: db}
	users := make([]models.User, 0)
	perms, err := ps.GetForServer(identifier)
	if err != nil {
		return nil, err
	}
	for _, p := range perms {
		exists := false
		for _, u := range users {
			if u.ID == p.User.ID {
				exists = true
				break
			}
		}
		if !exists {
			users = append(users, p.User)
		}
	}
	return users, nil
}

func shutdownServerOnNodeBeforeDeletion(ns *services.Node, node *models.Node, identifier string) error {
	statusRes, err := ns.CallNode(node, "GET", daemonServerPrefix+identifier+statusSuffix, nil, nil)
	if err == nil && statusRes.StatusCode == http.StatusOK {
		var statusData struct {
			Running bool `json:"running"`
		}
		if err := json.NewDecoder(statusRes.Body).Decode(&statusData); err == nil && statusData.Running {
			stopRes, err := ns.CallNode(node, "POST", daemonServerPrefix+identifier+"/stop?wait=true", nil, nil)
			if err != nil {
				logging.Error.Printf("Error stopping server before deletion: %s", err)
				return err
			}
			if stopRes != nil && stopRes.Body != nil {
				stopRes.Body.Close()
			}
		}
		if statusRes != nil && statusRes.Body != nil {
			statusRes.Body.Close()
		}
	}
	return nil
}

func executeServerDeletionOnNode(c *gin.Context, ns *services.Node, node *models.Node, identifier string) error {
	nodeResponse, err := ns.CallNode(node, "DELETE", daemonServerPrefix+identifier, nil, c.Request.Header)
	defer utils.CloseResponse(nodeResponse)

	if response.HandleError(c, err, http.StatusBadGateway) {
		return err
	}

	if nodeResponse.StatusCode != http.StatusNoContent && nodeResponse.StatusCode != http.StatusNotFound {
		resData, err := io.ReadAll(nodeResponse.Body)
		if err != nil {
			logging.Error.Printf(daemonParseErrorFmt, err.Error())
		}
		logging.Error.Printf(daemonUnexpectedFmt, nodeResponse.StatusCode, string(resData))
		c.Header(headerContentType, contentTypeJSON)
		c.Status(nodeResponse.StatusCode)
		_, _ = c.Writer.Write(resData)
		e := c.Error(errors.New("unexpected response from daemon"))
		c.Abort()
		return e
	}
	return nil
}

func updateParentServerLimitsAfterDeletion(db *gorm.DB, ns *services.Node, node *models.Node, server *models.Server) {
	var parent models.Server
	if err := db.Where(serverIdentifierQuery, *server.ParentServerID).First(&parent).Error; err != nil {
		return
	}
	var children []*models.Server
	if err := db.Where(parentServerIDQuery, parent.Identifier).Find(&children).Error; err != nil {
		return
	}

	var usedCPU int
	var usedMemory, usedDisk int64
	for _, child := range children {
		usedCPU += child.TotalCPU
		usedMemory += child.TotalMemory
		usedDisk += child.TotalDisk
	}

	// the admin PUT: the plain POST only changes values the owner may edit, which these are not
	updateBytes, _ := json.Marshal(parentLimitsData(parent.TotalCPU-usedCPU, parent.TotalMemory-usedMemory, parent.TotalDisk-usedDisk))
	parentReqBody := io.NopCloser(bytes.NewReader(updateBytes))
	parentRes, _ := ns.CallNode(node, "PUT", daemonServerPrefix+parent.Identifier+"/data", parentReqBody, nil)
	if parentRes != nil && parentRes.Body != nil {
		parentRes.Body.Close()
	}
}

func sendServerDeletionEmails(users []models.User, server *models.Server) {
	es := services.GetEmailService()
	for _, user := range users {
		err := es.SendEmail(user.Email, "deletedFromServer", map[string]interface{}{
			"Server": server,
		}, true)
		if err != nil {
			logging.Error.Printf("Error sending email: %s", err)
		}
	}
}

// --- Helpers for Create Server ---

func validateAndBuildServerCreation(c *gin.Context, db *gorm.DB, us *services.User, postBody *models.ServerCreation) (*models.Server, []*models.User, error) {
	if err := inheritParentServer(c, db, us, postBody); err != nil {
		return nil, nil, err
	}

	port, err := getFromDataOrDefault(postBody.Variables, "port", uint16(0))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return nil, nil, err
	}

	ip, err := getFromDataOrDefault(postBody.Variables, "ip", "0.0.0.0")
	if response.HandleError(c, err, http.StatusBadRequest) {
		return nil, nil, err
	}

	ports := collectPortsFromVariables(postBody.Variables)
	if len(ports) > 0 {
		if err := validatePortsAgainstNode(db, postBody.NodeID, "", ports); err != nil {
			response.HandleError(c, err, http.StatusBadRequest)
			return nil, nil, err
		}
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
		return nil, nil, errors.New("resources cannot be negative")
	}
	// a sub-server takes its limits out of its parent's plan; 0 would mean "no limit" to
	// Docker, so it could use the whole node while counting as nothing against the parent
	if hasParentServer(&models.Server{ParentServerID: postBody.ParentServerID}) && (totalCPU == 0 || totalMemory == 0 || totalDisk == 0) {
		err := errors.New("a sub-server needs a CPU, memory and disk limit")
		response.HandleError(c, err, http.StatusBadRequest)
		return nil, nil, err
	}

	server := &models.Server{
		Name:           postBody.Name,
		Identifier:     postBody.Identifier,
		NodeID:         postBody.NodeID,
		IP:             cast.ToString(ip),
		Port:           cast.ToUint16(port),
		Ports:          ports,
		Type:           postBody.Type.Type,
		Icon:           postBody.Icon,
		ParentServerID: postBody.ParentServerID,
		TotalCPU:       totalCPU,
		TotalMemory:    totalMemory,
		TotalDisk:      totalDisk,
	}

	users := make([]*models.User, len(postBody.Users))
	for k, v := range postBody.Users {
		user, err := us.Get(v)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return nil, nil, err
		}
		users[k] = user
	}

	return server, users, nil
}

// inheritParentServer puts a sub-server on its parent's node and gives access
// to the parent's users; it does nothing for a top-level server. It writes the
// error response itself.
func inheritParentServer(c *gin.Context, db *gorm.DB, us *services.User, postBody *models.ServerCreation) error {
	if postBody.ParentServerID == nil || *postBody.ParentServerID == "" {
		return nil
	}

	var parent models.Server
	if err := db.Where(serverIdentifierQuery, *postBody.ParentServerID).First(&parent).Error; err != nil {
		response.HandleError(c, errors.New("parent server not found"), http.StatusBadRequest)
		return err
	}

	postBody.NodeID = parent.NodeID

	var parentPerms []models.Permissions
	db.Where(serverIdentifierRefQuery, parent.Identifier).Find(&parentPerms)

	var inheritedUsers []string
	for _, p := range parentPerms {
		if p.UserID == nil {
			continue
		}
		user, err := us.GetByID(*p.UserID)
		if err == nil && user != nil {
			inheritedUsers = append(inheritedUsers, user.Username)
		}
	}
	postBody.Users = inheritedUsers
	return nil
}

// subServerDockerKeys are the docker environment settings a non-admin may choose. Host
// mounts (bindings), hostConfig (privileged mode, capabilities, devices) and the network
// are left out because they give the container access to the host.
var subServerDockerKeys = []string{"image", "portBindings", "containerRoot", "config", "labels"}

// lockEnvironment keeps the server's environment as it is except for the docker settings a
// non-admin may choose (subServerDockerKeys)
func lockEnvironment(current, wanted skypanel.MetadataType) skypanel.MetadataType {
	locked := skypanel.MetadataType{Type: current.Type, Metadata: make(map[string]interface{}, len(current.Metadata))}
	for k, v := range current.Metadata {
		locked.Metadata[k] = v
	}
	if current.Type != "docker" || wanted.Type != current.Type {
		return locked
	}
	for _, k := range subServerDockerKeys {
		if v, ok := wanted.Metadata[k]; ok {
			locked.Metadata[k] = v
		} else {
			delete(locked.Metadata, k)
		}
	}
	return locked
}

// currentDefinition reads the server's definition from its node
func currentDefinition(c *gin.Context, ns *services.Node, server *models.Server) (*skypanel.Server, error) {
	ctx, cancel := nodeCallContext(c)
	defer cancel()
	res, err := ns.CallNodeContext(ctx, &server.Node, "GET", daemonServerPrefix+server.Identifier+"/definition", nil, nil)
	defer utils.CloseResponse(res)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the node answered %d reading the definition", res.StatusCode)
	}
	definition := &skypanel.Server{}
	return definition, json.NewDecoder(res.Body).Decode(definition)
}

// ownerOfNewServer decides who owns a server being created: a sub-server belongs to its
// parent's owner, otherwise the first user it is created for, otherwise its creator unless
// that is an administrator making a server of their own (it then has no owner).
func ownerOfNewServer(db *gorm.DB, creator *models.User, users []*models.User, parentID *string) (*uint, error) {
	if parentID != nil && *parentID != "" {
		var parent models.Server
		if err := db.Select("identifier", "owner_id").First(&parent, "identifier = ?", *parentID).Error; err != nil {
			return nil, err
		}
		if parent.OwnerID != nil {
			return parent.OwnerID, nil
		}
	}
	if len(users) > 0 && users[0] != nil {
		return &users[0].ID, nil
	}
	isAdmin, err := userIsAdmin(db, creator)
	if err != nil || isAdmin {
		return nil, err
	}
	return &creator.ID, nil
}

// restrictSubServerCreation limits users without admin to the Splitter flow: a child of a
// server they already have access to, running in a docker container without host access.
func restrictSubServerCreation(ps *services.Permission, user *models.User, postBody *models.ServerCreation) error {
	if postBody.ParentServerID == nil || *postBody.ParentServerID == "" {
		return errors.New("only administrators can create servers without a parent server")
	}

	// splitting takes resources out of the parent's plan, so it is the owner's call: a
	// sub-user of the parent cannot carve servers of their own out of it
	owner, err := ps.IsServerOwner(user.ID, *postBody.ParentServerID)
	if err != nil {
		return err
	}
	if !owner {
		return errors.New("only the parent server's owner can split it")
	}

	if postBody.Environment.Type != "docker" {
		return errors.New("only docker environments are allowed for sub-servers")
	}

	allowed := make(map[string]interface{}, len(subServerDockerKeys))
	for _, k := range subServerDockerKeys {
		if v, ok := postBody.Environment.Metadata[k]; ok {
			allowed[k] = v
		}
	}
	postBody.Environment.Metadata = allowed

	return nil
}

// nodeCallInTransactionTimeout bounds node calls made while the request's transaction is
// open (create, definition edit): they are quick on a healthy node, and a hung node must not
// keep the transaction, and on SQLite the whole panel, waiting forever
const nodeCallInTransactionTimeout = 60 * time.Second

func nodeCallContext(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), nodeCallInTransactionTimeout)
}

// lockParentServer loads the parent row and locks it until the transaction ends, so two
// children cannot both pass the resource check. GORM writes FOR UPDATE on MySQL and Postgres
// and leaves it out on SQLite, which has no row locks and rejected the raw query.
func lockParentServer(tx *gorm.DB, identifier string, parent *models.Server) error {
	return tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).Where(serverIdentifierQuery, identifier).Find(parent).Error
}

func checkParentServerLimits(tx *gorm.DB, server *models.Server, parentAvailableCPU *int, parentAvailableMemory, parentAvailableDisk *int64) error {
	var parent models.Server
	if err := lockParentServer(tx, *server.ParentServerID, &parent); err != nil {
		return err
	}
	if parent.Identifier == "" {
		return errors.New("parent server not found")
	}

	var children []*models.Server
	if err := tx.Where(parentServerIDQuery, parent.Identifier).Find(&children).Error; err != nil {
		return err
	}

	var usedCPU int
	var usedMemory, usedDisk int64
	for _, child := range children {
		usedCPU += child.TotalCPU
		usedMemory += child.TotalMemory
		usedDisk += child.TotalDisk
	}

	*parentAvailableCPU = parent.TotalCPU - usedCPU - server.TotalCPU
	*parentAvailableMemory = parent.TotalMemory - usedMemory - server.TotalMemory
	*parentAvailableDisk = parent.TotalDisk - usedDisk - server.TotalDisk

	if *parentAvailableCPU < 0 {
		return errors.New("not enough CPU available in parent server")
	}
	if *parentAvailableMemory < 0 {
		return errors.New("not enough memory available in parent server")
	}
	if *parentAvailableDisk < 0 {
		return errors.New("not enough disk available in parent server")
	}
	return nil
}

func checkParentServerLimitsOnUpdate(tx *gorm.DB, server *models.Server, parentAvailableCPU *int, parentAvailableMemory, parentAvailableDisk *int64) error {
	var parent models.Server
	if err := lockParentServer(tx, *server.ParentServerID, &parent); err != nil {
		return err
	}
	if parent.Identifier == "" {
		return errors.New("parent server not found")
	}

	var children []*models.Server
	if err := tx.Where(parentServerIDQuery, parent.Identifier).Find(&children).Error; err != nil {
		return err
	}

	var usedCPU int
	var usedMemory, usedDisk int64
	for _, child := range children {
		if child.Identifier == server.Identifier {
			continue
		}
		usedCPU += child.TotalCPU
		usedMemory += child.TotalMemory
		usedDisk += child.TotalDisk
	}

	*parentAvailableCPU = parent.TotalCPU - usedCPU - server.TotalCPU
	*parentAvailableMemory = parent.TotalMemory - usedMemory - server.TotalMemory
	*parentAvailableDisk = parent.TotalDisk - usedDisk - server.TotalDisk

	if *parentAvailableCPU < 0 {
		return errors.New("not enough CPU available in parent server")
	}
	if *parentAvailableMemory < 0 {
		return errors.New("not enough memory available in parent server")
	}
	if *parentAvailableDisk < 0 {
		return errors.New("not enough disk available in parent server")
	}
	return nil
}

func assignServerPermissions(ps *services.Permission, serverID string, users []*models.User) error {
	for _, v := range users {
		perm, err := ps.GetForUserAndServer(v.ID, serverID)
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

		err = ps.UpdatePermissions(perm)
		if err != nil {
			return err
		}
	}
	return nil
}

func notifyNodeOfServerCreation(c *gin.Context, ns *services.Node, node *models.Node, postBody *models.ServerCreation, server *models.Server) error {
	reader := &bytes.Buffer{}
	if err := json.NewEncoder(reader).Encode(&postBody.Server); err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		return err
	}

	ctx, cancel := nodeCallContext(c)
	defer cancel()
	nodeResponse, err := ns.CallNodeContext(ctx, node, "PUT", daemonServerPrefix+server.Identifier, io.NopCloser(reader), c.Request.Header)
	defer utils.CloseResponse(nodeResponse)

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return err
	}

	if nodeResponse.StatusCode != http.StatusOK {
		resData, err := io.ReadAll(nodeResponse.Body)
		if err != nil {
			logging.Error.Printf(daemonParseErrorFmt, err.Error())
		}
		logging.Error.Printf(daemonUnexpectedFmt, nodeResponse.StatusCode, string(resData))
		c.Header(headerContentType, contentTypeJSON)
		c.Status(nodeResponse.StatusCode)
		_, _ = c.Writer.Write(resData)
		c.Abort()
		return errors.New("unexpected node response")
	}
	return nil
}

func updateParentServerLimitsOnNode(c *gin.Context, db *gorm.DB, ns *services.Node, node *models.Node, server *models.Server, parentAvailableCPU int, parentAvailableMemory int64, parentAvailableDisk int64) {
	// the admin PUT: the plain POST only changes values the owner may edit, which these are not
	updateBytes, _ := json.Marshal(parentLimitsData(parentAvailableCPU, parentAvailableMemory, parentAvailableDisk))
	parentReqBody := io.NopCloser(bytes.NewReader(updateBytes))

	var parent models.Server
	db.Where(serverIdentifierQuery, *server.ParentServerID).First(&parent)
	ctx, cancel := nodeCallContext(c)
	defer cancel()
	parentRes, _ := ns.CallNodeContext(ctx, node, "PUT", daemonServerPrefix+parent.Identifier+"/data", parentReqBody, nil)
	if parentRes != nil && parentRes.Body != nil {
		parentRes.Body.Close()
	}
}

func sendServerCreationEmails(users []*models.User, server *models.Server) {
	es := services.GetEmailService()
	for _, user := range users {
		err := es.SendEmail(user.Email, "addedToServer", map[string]interface{}{
			"Server":        server,
			"RegisterToken": "",
		}, true)
		if err != nil {
			logging.Error.Printf("Error sending email: %s", err)
		}
	}
}
