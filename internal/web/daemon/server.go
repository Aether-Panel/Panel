package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers/docker"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron/v2"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/websocket"
	"github.com/spf13/cast"
	"golang.org/x/sys/unix"
)

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

func RegisterServerRoutes(e *gin.RouterGroup) {
	l := e.Group("/server", middleware.ValidateJWT)
	{
		l.PUT("/:serverId", createServer)
		l.DELETE("/:serverId", middleware.ResolveServerNode, deleteServer)
		l.OPTIONS("/:serverId", response.CreateOptions("PUT", "DELETE", "GET"))

		l.GET("/:serverId/definition", middleware.ResolveServerNode, getServerAdmin)
		l.PUT("/:serverId/definition", middleware.ResolveServerNode, editServerAdmin)
		l.OPTIONS("/:serverId/definition", response.CreateOptions("PUT", "DELETE", "GET"))

		l.GET("/:serverId/data", middleware.ResolveServerNode, getServerData)
		l.POST("/:serverId/data", middleware.ResolveServerNode, editServerData)
		l.PUT("/:serverId/data", middleware.ResolveServerNode, editServerDataAdmin)
		l.OPTIONS("/:serverId/data", response.CreateOptions("GET", "POST", "PUT"))

		l.GET("/:serverId/tasks", middleware.ResolveServerNode, getServerTasks)
		l.OPTIONS("/:serverId/tasks", response.CreateOptions("GET"))

		l.GET("/:serverId/tasks/:taskID", middleware.ResolveServerNode, getServerTask)
		l.PUT("/:serverId/tasks/:taskID", middleware.ResolveServerNode, editServerTask)
		l.DELETE("/:serverId/tasks/:taskID", middleware.ResolveServerNode, deleteServerTask)
		l.OPTIONS("/:serverId/tasks/:taskID", response.CreateOptions("GET", "PUT", "DELETE"))

		l.POST("/:serverId/tasks/:taskID/run", middleware.ResolveServerNode, runServerTask)
		l.OPTIONS("/:serverId/tasks/:taskID/run", response.CreateOptions("POST"))

		l.POST("/:serverId/reload", middleware.ResolveServerNode, reloadServer)
		l.OPTIONS("/:serverId/reload", response.CreateOptions("POST"))

		l.POST("/:serverId/start", middleware.ResolveServerNode, startServer)
		l.OPTIONS("/:serverId/start", response.CreateOptions("POST"))

		l.POST("/:serverId/restart", middleware.ResolveServerNode, restartServer)
		l.OPTIONS("/:serverId/restart", response.CreateOptions("POST"))

		l.POST("/:serverId/stop", middleware.ResolveServerNode, stopServer)
		l.OPTIONS("/:serverId/stop", response.CreateOptions("POST"))

		l.POST("/:serverId/kill", middleware.ResolveServerNode, killServer)
		l.OPTIONS("/:serverId/kill", response.CreateOptions("POST"))

		l.POST("/:serverId/install", middleware.ResolveServerNode, installServer)
		l.OPTIONS("/:serverId/install", response.CreateOptions("POST"))

		l.GET("/:serverId/file/*filename", middleware.ResolveServerNode, getFile)
		l.PUT("/:serverId/file/*filename", middleware.ResolveServerNode, putFile)
		l.DELETE("/:serverId/file/*filename", middleware.ResolveServerNode, deleteFile)
		l.POST("/:serverId/file/*filename", middleware.ResolveServerNode, moveFile)
		l.OPTIONS("/:serverId/file/*filename", response.CreateOptions("GET", "PUT", "DELETE", "POST"))

		// the server assistant's file operations that the file manager's routes do not cover
		l.GET("/:serverId/fs/stat", middleware.ResolveServerNode, statServerPath)
		l.POST("/:serverId/fs/copy", middleware.ResolveServerNode, copyServerPath)
		l.GET("/:serverId/fs/search", middleware.ResolveServerNode, searchServerFiles)

		l.GET("/:serverId/console", middleware.ResolveServerNode, getLogs)
		l.POST("/:serverId/console", middleware.ResolveServerNode, postConsole)
		l.OPTIONS("/:serverId/console", response.CreateOptions("GET", "POST"))

		l.GET("/:serverId/flags", middleware.ResolveServerNode, getFlags)
		l.POST("/:serverId/flags", middleware.ResolveServerNode, setFlags)
		l.OPTIONS("/:serverId/flags", response.CreateOptions("GET", "POST"))

		l.GET("/:serverId/stats", middleware.ResolveServerNode, getStats)
		l.OPTIONS("/:serverId/stats", response.CreateOptions("GET"))

		l.GET("/:serverId/status", middleware.ResolveServerNode, getStatus)
		l.OPTIONS("/:serverId/status", response.CreateOptions("GET"))

		l.POST("/:serverId/archive/*filename", middleware.ResolveServerNode, archive)
		l.POST("/:serverId/extract/*filename", middleware.ResolveServerNode, extract)

		l.GET("/:serverId/backup", middleware.ResolveServerNode, listBackups)
		l.POST("/:serverId/backup/create", middleware.ResolveServerNode, createBackup)
		l.DELETE("/:serverId/backup", middleware.ResolveServerNode, deleteBackup)
		l.POST("/:serverId/backup/restore", middleware.ResolveServerNode, restoreBackup)
		l.GET("/:serverId/backup/download", middleware.ResolveServerNode, downloadBackup)

		l.HEAD("/:serverId/query", middleware.ResolveServerNode, canQueryServer)
		l.GET("/:serverId/query", middleware.ResolveServerNode, queryServer)

		l.GET("/:serverId/crashes", middleware.ResolveServerNode, getCrashReports)
		l.GET("/:serverId/metrics", middleware.ResolveServerNode, getMetrics)
		l.GET("/:serverId/version", middleware.ResolveServerNode, getGameVersions)
		l.POST("/:serverId/version", middleware.ResolveServerNode, changeGameVersion)
		l.GET("/:serverId/notifications", middleware.ResolveServerNode, getNotifications)
		l.PUT("/:serverId/notifications", middleware.ResolveServerNode, putNotifications)
		l.POST("/:serverId/notifications/test", middleware.ResolveServerNode, testNotifications)
		l.GET("/:serverId/offsite", middleware.ResolveServerNode, getOffsiteSettings)
		l.PUT("/:serverId/offsite", middleware.ResolveServerNode, putOffsiteSettings)
		l.OPTIONS("/:serverId/offsite", response.CreateOptions("GET", "PUT"))

		l.GET("/:serverId/plugins", middleware.ResolveServerNode, getPlugins)
		l.DELETE("/:serverId/plugins", middleware.ResolveServerNode, deletePlugin)
		l.OPTIONS("/:serverId/plugins", response.CreateOptions("GET", "DELETE"))
		l.GET("/:serverId/plugins/search", middleware.ResolveServerNode, searchPlugins)
		l.OPTIONS("/:serverId/plugins/search", response.CreateOptions("GET"))
		l.POST("/:serverId/plugins/:pluginId", middleware.ResolveServerNode, installPlugin)
		l.OPTIONS("/:serverId/plugins/:pluginId", response.CreateOptions("POST"))

		registerModpackRoutes(l)
		registerGitHubRoutes(l)
		registerWorldMapRoutes(l)
		registerFileHistoryRoutes(l)
		registerCloneRoutes(l)
		registerTransferRoutes(l)

		p := l.Group("/:serverId/socket")
		{
			p.GET("", middleware.ResolveServerNode, cors.New(cors.Config{
				AllowAllOrigins:  true,
				AllowCredentials: true,
			}), openSocket)
			p.Handle("CONNECT", "", func(c *gin.Context) {
				c.Header("Access-Control-Allow-Origin", "*")
				c.Header("Access-Control-Allow-Credentials", "false")
			})
			p.OPTIONS("", response.CreateOptions("GET", "CONNECT"))
		}
	}
}

func getServerFromGin(c *gin.Context) *servers.Server {
	return c.MustGet("program").(*servers.Server)
}

// diskStatus answers a write the disk limit refused with 409, so the caller is told the
// disk is full instead of seeing a server error
func diskStatus(err error, fallback int) int {
	if errors.Is(err, skypanel.ErrDiskLimitExceeded) {
		return http.StatusConflict
	}
	return fallback
}

// @Summary Start server
// @Description Start server
// @Success 202 {object} nil
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/start [post]
// @Security OAuth2Application[server.start]
func startServer(c *gin.Context) {
	server := getServerFromGin(c)
	_, wait := c.GetQuery("wait")

	// a full disk and port conflicts are answered here, instead of a 202 that later
	// fails asynchronously on the docker daemon
	if err := server.CheckDiskSpace(false); response.HandleError(c, err, http.StatusConflict) {
		return
	}
	if err := checkPortConflicts(server); err != nil {
		response.HandleError(c, err, http.StatusConflict)
		return
	}

	if wait {
		err := server.Start()
		if !response.HandleError(c, err, http.StatusInternalServerError) {
			c.Status(http.StatusNoContent)
		}
	} else {
		go func() {
			err := server.Start()
			if err != nil {
				logging.Error.Printf("Error starting server %s: %s", server.ID(), err)
			}
		}()
		c.Status(http.StatusAccepted)
	}
}

// checkPortConflicts inspects the server's docker port bindings and returns an
// error if any of them are already in use by another running container.
func checkPortConflicts(server *servers.Server) error {
	env := server.GetEnvironment()
	if env == nil || env.Implementation == nil {
		return nil
	}

	d, ok := env.Implementation.(*docker.Docker)
	if !ok {
		return nil
	}

	variables := server.DataToMap()
	resolved := docker.ResolvedPortBindings(d.Ports, variables)
	resolved = append(resolved, docker.ExtraPortBindings(variables)...)
	return docker.CheckBindingsConflict(resolved, variables, server.ID())
}

func doRestart(server *servers.Server) error {
	return server.Restart()
}

// @Summary Restart server
// @Description Restart server
// @Success 202 {object} nil
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/restart [post]
// @Security OAuth2Application[server.start, server.stop]
func restartServer(c *gin.Context) {
	server := getServerFromGin(c)
	_, wait := c.GetQuery("wait")

	// a full disk and port conflicts are answered here, instead of a 202 that later
	// fails asynchronously on the docker daemon
	if err := server.CheckDiskSpace(false); response.HandleError(c, err, http.StatusConflict) {
		return
	}
	if err := checkPortConflicts(server); err != nil {
		response.HandleError(c, err, http.StatusConflict)
		return
	}

	if wait {
		err := doRestart(server)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}

		c.Status(http.StatusNoContent)
	} else {
		go func() {
			err := doRestart(server)
			if err != nil {
				logging.Error.Printf("Error restarting server %s: %s", server.ID(), err)
			}
		}()
		c.Status(http.StatusAccepted)
	}
}

// @Summary Stop server
// @Description Stop server
// @Success 202 {object} nil
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/stop [post]
// @Security OAuth2Application[server.stop]
func stopServer(c *gin.Context) {
	server := getServerFromGin(c)

	_, wait := c.GetQuery("wait")

	err := server.Stop()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if wait {
		err = server.GetEnvironment().WaitForMainProcess()
		if !response.HandleError(c, err, http.StatusInternalServerError) {
			c.Status(http.StatusNoContent)
		}
	} else {
		c.Status(http.StatusAccepted)
	}
}

// @Summary Kill server
// @Description Kill server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/kill [post]
// @Security OAuth2Application[server.kill]
func killServer(c *gin.Context) {
	server := getServerFromGin(c)

	err := server.Kill()
	if !response.HandleError(c, err, http.StatusInternalServerError) {
		c.Status(http.StatusNoContent)
	}
}

// Already declared in panel routing
func createServer(c *gin.Context) {
	serverID := c.Param("serverId")
	if serverID == "" {
		id, err := uuid.NewV4()
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		serverID = id.String()
	}
	prg := servers.GetFromCache(serverID)

	if prg != nil {
		// Un servidor ya existente en el nodo (p.ej. por una transferencia previa
		// que no limpió el destino) bloquea el re-creado con un 409. Lo eliminamos
		// para que la operación sea idempotente y el transfer round-trip funcione.
		if err := servers.Delete(serverID); err != nil {
			response.HandleError(c, err, http.StatusInternalServerError)
			return
		}
	}

	prg = servers.CreateProgram()
	err := json.NewDecoder(c.Request.Body).Decode(prg)
	if err != nil {
		logging.Error.Printf("Error decoding JSON body: %s", err)
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	prg.Identifier = serverID

	if err = checkEnvironmentAllowed(prg.Environment.Type); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	err = prg.Requirements.Test(prg.Server)
	if err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	if prg, err = servers.Create(prg); err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		if prg != nil {
			_ = servers.Delete(prg.ID())
		}
		return
	}

	c.JSON(http.StatusOK, &skypanel.ServerIDResponse{ID: serverID})
}

// Already declared in panel routing
func deleteServer(c *gin.Context) {
	server := getServerFromGin(c)

	if running, err := server.IsRunning(); running {
		if err = server.Kill(); response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		_ = server.RunningEnvironment.WaitForMainProcess()
	} else if err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	err := servers.Delete(server.ID())
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	// its backups go too, unless the panel asks to keep them: a node transfer that is undone
	// removes only the backup copies it brought, one by one
	if c.Query("keepBackups") != "true" {
		if err := servers.DeleteBackups(server.ID()); err != nil {
			logging.Error.Printf("Error removing the backups of server %s: %s", server.ID(), err)
		}
	}
	c.Status(http.StatusNoContent)
}

// @Summary Install server
// @Description Install server
// @Success 202 {object} nil
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/install [post]
// @Security OAuth2Application[server.install]
func installServer(c *gin.Context) {
	server := getServerFromGin(c)

	_, wait := c.GetQuery("wait")

	if wait {
		err := server.Install()
		if !response.HandleError(c, err, http.StatusInternalServerError) {
			c.Status(http.StatusNoContent)
		}
	} else {
		go func(p *servers.Server) {
			_ = p.Install()
		}(server)

		c.Status(http.StatusAccepted)
	}
}

// Not documented in swagger as overridden on frontend
func editServerData(c *gin.Context) {
	server := getServerFromGin(c)

	var data map[string]interface{}
	err := json.NewDecoder(c.Request.Body).Decode(&data)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	err = server.EditData(data, false)
	if !response.HandleError(c, err, http.StatusInternalServerError) {
		c.Status(http.StatusNoContent)
	}
}

// Not documented in swagger as overridden on frontend
func editServerDataAdmin(c *gin.Context) {
	server := getServerFromGin(c)

	var data map[string]interface{}
	err := json.NewDecoder(c.Request.Body).Decode(&data)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// Extra ports: the panel sends "ports" (full assigned list) and
	// "primaryPort". Mirror them into the port/port2/... variables so the
	// container binds all of them, and remove stale extra-port variables.
	if portsField, hasPorts := data["ports"]; hasPorts {
		mirrorPortsIntoData(server, data, portsField)
	}

	err = server.EditData(data, true)
	if !response.HandleError(c, err, http.StatusInternalServerError) {
		c.Status(http.StatusNoContent)
	}
}

// mirrorPortsIntoData replaces the panel's "ports" and "primaryPort" fields
// with the port/port2/... variables, primary port first.
func mirrorPortsIntoData(server *servers.Server, data map[string]interface{}, portsField interface{}) {
	ports := parseValidPorts(portsField)

	var primary uint16
	if rawPrimary, ok := data["primaryPort"]; ok {
		primary = cast.ToUint16(rawPrimary)
	} else if len(ports) > 0 {
		primary = ports[0]
	}

	if primary > 0 {
		movePortFirst(ports, primary)
	}

	if len(ports) > 0 {
		setPortVariables(server, data, ports)
	}
	delete(data, "ports")
	delete(data, "primaryPort")
}

// parseValidPorts keeps the entries of the "ports" array that are valid,
// non-zero ports.
func parseValidPorts(portsField interface{}) []uint16 {
	rawList, ok := portsField.([]interface{})
	if !ok {
		return nil
	}
	var ports []uint16
	for _, raw := range rawList {
		if p, perr := cast.ToUint16E(raw); perr == nil && p > 0 {
			ports = append(ports, p)
		}
	}
	return ports
}

// movePortFirst swaps the given port into the first slot when it is listed.
func movePortFirst(ports []uint16, port uint16) {
	for i, p := range ports {
		if p == port {
			ports[0], ports[i] = ports[i], ports[0]
			return
		}
	}
}

// setPortVariables writes the ports as port, port2, port3... and removes the
// variables of ports that are no longer assigned.
func setPortVariables(server *servers.Server, data map[string]interface{}, ports []uint16) {
	data["port"] = ports[0]
	for i := 1; i < len(ports); i++ {
		data[fmt.Sprintf("port%d", i+1)] = ports[i]
	}
	// Remove variables beyond the new list so dropped ports unbind.
	for i := len(ports) + 1; ; i++ {
		key := fmt.Sprintf("port%d", i)
		if _, exists := server.Variables[key]; !exists {
			break
		}
		delete(server.Variables, key)
	}
}

// @Summary Get server tasks
// @Description Get server tasks
// @Success 200 {object} skypanel.ServerTasks
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/tasks [get]
// @Security OAuth2Application[server.tasks.view]
func getServerTasks(c *gin.Context) {
	server := getServerFromGin(c)

	tasks := server.Scheduler.GetTasks()
	result := skypanel.ServerTasks{
		Tasks: make(map[string]skypanel.ServerTask, len(tasks)),
	}

	// the steps are included so the panel can describe and edit each task without a request per task
	for k, v := range tasks {
		result.Tasks[k] = skypanel.ServerTask{Task: v}
	}

	c.JSON(http.StatusOK, result)
}

// @Summary Get server task
// @Description Get server task by id
// @Success 200 {object} skypanel.ServerTask
// @Param id path string true "Server ID"
// @Param taskID path string true "Task ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/tasks/{taskID} [get]
// @Security OAuth2Application[server.tasks.view]
func getServerTask(c *gin.Context) {
	server := getServerFromGin(c)

	task, ok := server.Scheduler.GetTask(c.Param("taskID"))
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, &skypanel.ServerTask{Task: task})
}

// @Summary Run server task
// @Description Run a specific task
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param taskID path string true "Task ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/tasks/{taskID}/run [post]
// @Security OAuth2Application[server.tasks.run]
func runServerTask(c *gin.Context) {
	server := getServerFromGin(c)

	taskID := c.Param("taskID")

	err := server.Scheduler.RunTask(taskID)
	if errors.Is(err, gocron.ErrJobNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Edit server task
// @Description Edit server task by id
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param taskID path string true "Task ID"
// @Param task body skypanel.Task true "Task definition"
// @Tags Daemon Server
// @Router /daemon/server/{id}/tasks/{taskID} [put]
// @Security OAuth2Application[server.tasks.edit]
func editServerTask(c *gin.Context) {
	server := getServerFromGin(c)

	taskID := c.Param("taskID")

	var task skypanel.Task
	err := c.ShouldBindJSON(&task)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	err = server.Scheduler.SetTask(taskID, task)
	if errors.Is(err, servers.ErrInvalidTask) {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	if !response.HandleError(c, err, http.StatusInternalServerError) {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Delete server task
// @Description Delete server task by id
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param taskID path string true "Task ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/tasks/{taskID} [delete]
// @Security OAuth2Application[server.tasks.delete]
func deleteServerTask(c *gin.Context) {
	server := getServerFromGin(c)

	taskID := c.Param("taskID")

	err := server.Scheduler.RemoveTask(taskID)
	if errors.Is(err, gocron.ErrJobNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if !response.HandleError(c, err, http.StatusInternalServerError) {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Reload server
// @Description Reload server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/reload [post]
// @Security OAuth2Application[server.reload]
func reloadServer(c *gin.Context) {
	server := getServerFromGin(c)

	err := servers.Reload(server.ID())
	if !response.HandleError(c, err, http.StatusInternalServerError) {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Get server data
// @Description Get server variables
// @Success 200 {object} skypanel.ServerData
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/data [get]
// @Security OAuth2Application[server.data.view]
func getServerData(c *gin.Context) {
	server := getServerFromGin(c)

	data := server.GetData()

	var replacement = make(map[string]skypanel.Variable)
	for k, v := range data {
		if v.UserEditable {
			replacement[k] = v
		}
	}

	c.JSON(http.StatusOK, &skypanel.ServerData{Variables: replacement, Groups: server.Groups})
}

// @Summary Get server definition
// @Description Get server definition
// @Success 200 {object} skypanel.Server
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/definition [get]
// @Security OAuth2Application[server.definition.view]
func getServerAdmin(c *gin.Context) {
	server := getServerFromGin(c)

	c.JSON(http.StatusOK, &server.Server)
}

// @Summary Edit server definition
// @Description Updates the server with a new definition
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param server body skypanel.Server true "New definition"
// @Tags Daemon Server
// @Router /daemon/server/{id}/definition [put]
// @Security OAuth2Application[server.definition.edit]
func editServerAdmin(c *gin.Context) {
	prg := getServerFromGin(c)
	server := &prg.Server

	if err := prg.IsIdle(); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	replacement := &skypanel.Server{}
	err := c.BindJSON(replacement)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if err = checkEnvironmentAllowed(replacement.Environment.Type); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// backup, just in case we break
	backup := &skypanel.Server{}
	backup.CopyFrom(server)

	// copy from request
	server.CopyFrom(replacement)

	err = servers.Save(prg.ID())
	if response.HandleError(c, err, http.StatusInternalServerError) {
		// REVERT!!!!!!!
		server.CopyFrom(backup)
		return
	}

	err = servers.Reload(prg.ID())

	if response.HandleError(c, err, http.StatusInternalServerError) {
		// attempt to revert... but no promise this works
		server.CopyFrom(backup)
		_ = servers.Reload(prg.ID())
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Get file/folder
// @Description Gets a specific file or a list of files in a folder. This will either return
// @Description a) A raw file if the path points to a valid file
// @Description or b) An array of files for the folder contents
// @Success 200 {object} nil
// @Param id path string true "Server ID"
// @Param filepath path string true "File path"
// @Tags Daemon Server
// @Router /api/servers/{id}/file/{filepath} [get]
// @Security OAuth2Application[server.files.view]
func getFile(c *gin.Context) {
	server := getServerFromGin(c)

	targetPath := getFullFilename(c)

	data, err := server.GetItem(targetPath)
	defer func() {
		if data != nil {
			utils.Close(data.Contents)
		}
	}()

	if err != nil {
		// nolint:gocritic
		if os.IsNotExist(err) {
			c.AbortWithStatus(http.StatusNotFound)
		} else if errors.Is(err, skypanel.ErrIllegalFileAccess) {
			response.HandleError(c, err, http.StatusBadRequest)
		} else {
			response.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	// nolint:gocritic
	if data.FileList != nil {
		c.JSON(http.StatusOK, data.FileList)
	} else if data.Contents != nil {
		fileName := filepath.Base(data.Name)

		extraHeaders := map[string]string{
			"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, fileName),
		}

		// discard the built-in response, we cannot use this one at all
		c.DataFromReader(http.StatusOK, data.ContentLength, "application/octet-stream", data.Contents, extraHeaders)
	} else {
		// uhhhhhhhhhhhhh
		response.HandleError(c, errors.New("no file content or file list"), http.StatusInternalServerError)
	}
}

// @Summary Edit file
// @Description Adds or edit a file, replacing the contents with the provided body.
// @Description With create=true it only creates a new file and answers 409 when something is already there.
// @Success 204 {object} nil
// @Failure 409 {object} skypanel.ErrorResponse "create=true and the file exists, or the disk limit is reached"
// @Param id path string true "Server ID"
// @Param filepath path string true "File path"
// @Param create query bool false "Only create a new file, never replace one"
// @Param file formData file true "File contents"
// @Accept multipart/form-data
// @Tags Daemon Server
// @Router /api/servers/{id}/file/{filepath} [put]
// @Security OAuth2Application[server.files.edit]
func putFile(c *gin.Context) {
	server := getServerFromGin(c)

	targetPath, create := putFileTarget(c)
	if targetPath == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if _, mkFolder := c.GetQuery("folder"); mkFolder {
		makeFolder(c, server, targetPath)
		return
	}

	sourceFile, ok := uploadSource(c)
	if !ok {
		return
	}

	// os.O_CREATE does not create missing parent directories, so ensure they exist first
	parentDir := filepath.Dir(targetPath)
	if err := server.GetFileServer().MkdirAll(parentDir, 0755); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if err := server.CheckDiskSpace(false); response.HandleError(c, err, http.StatusConflict) {
		return
	}

	file, ok := openPutTarget(c, server, targetPath, create)
	if !ok {
		return
	}
	defer utils.Close(file)

	_, err := io.Copy(server.DiskWriter(file), sourceFile)
	if errors.Is(err, skypanel.ErrDiskLimitExceeded) {
		// a half-written upload is useless and would keep the disk full
		utils.Close(file)
		_ = server.GetFileServer().Remove(targetPath)
		server.ForgetDiskUsage()
		response.HandleError(c, err, http.StatusConflict)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

// putFileTarget is the file a PUT writes and whether it may only create it. Only
// "create=true" turns creating on: a stray "create" or "create=false" keeps the normal write.
func putFileTarget(c *gin.Context) (string, bool) {
	if c.Query("create") == "true" {
		// getFullFilename keeps a query as part of the name, for old links to names with a
		// "?"; a create request escapes its name, so its query is only the flag
		return c.Param("filename"), true
	}
	return getFullFilename(c), false
}

// makeFolder answers a PUT with ?folder: the folder and its parents are made
func makeFolder(c *gin.Context, server *servers.Server, targetPath string) {
	err := server.GetFileServer().MkdirAll(targetPath, 0755)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

// uploadSource is what a PUT writes: the "file" part of a multipart form, or the raw body
func uploadSource(c *gin.Context) (io.ReadCloser, bool) {
	if t, _, _ := mime.ParseMediaType(c.Request.Header.Get("Content-Type")); t != "multipart/form-data" {
		return c.Request.Body, true
	}
	sourceFile, _, err := c.Request.FormFile("file")
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return nil, false
	}
	return sourceFile, true
}

// openPutTarget opens the file a PUT writes. false means the answer was already sent.
func openPutTarget(c *gin.Context, server *servers.Server, targetPath string, create bool) (*os.File, bool) {
	flags := os.O_CREATE | os.O_TRUNC | os.O_RDWR
	if create {
		// O_EXCL makes the check and the creation one step: two writers cannot both create the
		// file, and nothing already there (a file, a folder or a link) is replaced
		flags = os.O_CREATE | os.O_EXCL | os.O_WRONLY
	} else {
		// keep the text being replaced, so a config edit can be compared and undone
		recordFileVersion(server, targetPath)

		// the old content is about to be truncated, so it stops counting against the disk limit
		if fi, err := server.GetFileServer().Stat(targetPath); err == nil && !fi.IsDir() {
			server.ReleaseDisk(fi.Size())
		}
	}

	file, err := server.GetFileServer().OpenFile(targetPath, flags, 0644)
	if create && os.IsExist(err) {
		response.HandleError(c, skypanel.ErrFileExists, http.StatusConflict)
		return nil, false
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return nil, false
	}
	return file, true
}

// @Summary Delete file
// @Description Deletes a file or folder.
// @Description WARNING: This is a recursive operation, specifying a folder will delete all children.
// @Description A symlink is removed itself; what it points at is not touched.
// @Success 204 {object} nil
// @Failure 404 {object} nil
// @Param id path string true "Server ID"
// @Param filepath path string true "File path"
// @Tags Daemon Server
// @Router /api/servers/{id}/file/{filepath} [delete]
// @Security OAuth2Application[server.files.edit]
func deleteFile(c *gin.Context) {
	server := getServerFromGin(c)

	targetPath := getFullFilename(c)
	// the server folder itself is never removed through this route: an empty path would
	// otherwise empty the whole server
	if filepath.Clean("/"+targetPath) == "/" {
		response.HandleError(c, skypanel.ErrIllegalFileAccess, http.StatusBadRequest)
		return
	}

	// described without following it: following a symlink to a folder would empty that folder
	// and then fail to remove the link itself
	fi, err := lstatFile(server.GetFileServer(), targetPath)
	if os.IsNotExist(err) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		// only the link goes; what it points at is not touched
		err = removeLink(server.GetFileServer(), targetPath)
	case fi.IsDir():
		err = server.GetFileServer().RemoveAll(targetPath)
	default:
		err = server.GetFileServer().Remove(targetPath)
	}
	// freed space counts at once, so a server stopped for a full disk can start again
	server.ForgetDiskUsage()

	if !response.HandleError(c, err, http.StatusInternalServerError) {
		c.Status(http.StatusNoContent)
	}
}

// lstatFile describes name without following it when it is a symlink. The way there is still
// resolved beneath the server folder, as every file server path is.
func lstatFile(fsys files.FileServer, name string) (os.FileInfo, error) {
	f, err := fsys.OpenFile(name, unix.O_PATH|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer utils.Close(f)
	return f.Stat()
}

// removeLink removes the symlink name itself. The file server's Remove opens what a link
// points at, so the link is unlinked from its folder, which is opened beneath the root.
func removeLink(fsys files.FileServer, name string) error {
	name = filepath.Clean(name)
	folder, err := fsys.OpenFile(filepath.Dir(name), os.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer utils.Close(folder)
	return unix.Unlinkat(int(folder.Fd()), filepath.Base(name), 0)
}

// @Summary Rename or move a file/folder
// @Description Renames or moves a file/folder to a new path, allowing the name
// @Description and extension to be changed (e.g. config.json -> config.yaml).
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param filepath path string true "Current file path"
// @Param destination query string true "New file path"
// @Tags Daemon Server
// @Router /api/servers/{id}/file/{filepath} [post]
// @Security OAuth2Application[server.files.edit]
func moveFile(c *gin.Context) {
	server := getServerFromGin(c)

	source := strings.TrimPrefix(c.Param("filename"), "/")
	destination := c.Query("destination")

	if !filepath.IsLocal(source) {
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid source path: %s", source))
		return
	}
	if destination == "" {
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("destination path is required"))
		return
	}
	if !filepath.IsLocal(destination) {
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid destination path: %s", destination))
		return
	}

	fs := server.GetFileServer()

	if _, err := fs.Stat(source); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// Do not silently overwrite an existing file/folder
	if _, err := fs.Stat(destination); err == nil {
		response.HandleError(c, skypanel.ErrFileExists, http.StatusBadRequest)
		return
	}

	if err := fs.Rename(source, destination); !response.HandleError(c, err, http.StatusInternalServerError) {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Send command
// @Description Sends a command to the server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param command body string true "Command"
// @Tags Daemon Server
// @Router /api/servers/{id}/console [post]
// @Security OAuth2Application[server.console.send]
func postConsole(c *gin.Context) {
	server := getServerFromGin(c)

	d, err := io.ReadAll(c.Request.Body)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	cmd, err := cast.ToStringE(d)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	err = server.Execute(cmd)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)

}

// @Summary Get stats
// @Description Gets the CPU and memory usage of the server
// @Success 200 {object} skypanel.ServerStats
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/stats [get]
// @Security OAuth2Application[server.stats]
func getStats(c *gin.Context) {
	server := getServerFromGin(c)

	results, err := server.GetEnvironment().GetStats()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, results)
}

// @Summary Get logs
// @Description Get the console logs for the server
// @Success 200 {object} skypanel.ServerLogs
// @Param id path string true "Server ID"
// @Param time query int64 false "Epoch time in MS to get from"
// @Tags Daemon Server
// @Router /api/servers/{id}/console [get]
// @Security OAuth2Application[server.console]
func getLogs(c *gin.Context) {
	server := getServerFromGin(c)

	time := c.DefaultQuery("time", "0")

	castedTime, ok := cast.ToInt64E(time)
	if ok != nil {
		response.HandleError(c, skypanel.ErrInvalidUnixTime, http.StatusBadRequest)
		return
	}

	console, epoch := server.GetEnvironment().GetConsoleFrom(castedTime)

	c.JSON(http.StatusOK, &skypanel.ServerLogs{
		Epoch: epoch,
		Logs:  console,
	})
}

// @Summary Get status
// @Description Get the server's status (is it running)
// @Success 200 {object} skypanel.ServerRunning
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/status [get]
// @Security OAuth2Application[server.status]
func getStatus(c *gin.Context) {
	server := getServerFromGin(c)

	installing := server.GetEnvironment().IsInstalling()

	if installing {
		c.JSON(http.StatusOK, &skypanel.ServerRunning{Installing: installing})
		return
	}

	running, err := server.IsRunning()

	if !response.HandleError(c, err, http.StatusInternalServerError) {
		c.JSON(http.StatusOK, &skypanel.ServerRunning{Running: running})
	}
}

// @Summary Create archive
// @Description Creates an archive of files or folders
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param files body []string true "Files to archive"
// @Param filename path string true "Archive name"
// @Tags Daemon Server
// @Router /api/servers/{id}/archive/{filename} [post]
// @Security OAuth2Application[server.files.edit]
func archive(c *gin.Context) {
	server := getServerFromGin(c)
	var items []string

	if err := c.BindJSON(&items); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if len(items) == 0 {
		c.Status(http.StatusBadRequest)
		return
	}

	destination := strings.TrimPrefix(c.Param("filename"), "/")

	for _, f := range items {
		if !filepath.IsLocal(f) {
			_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid file path: %s", f))
			return
		}
	}
	if !filepath.IsLocal(destination) {
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid destination path: %s", destination))
		return
	}

	err := server.ArchiveItems(items, destination)
	status := diskStatus(err, http.StatusInternalServerError)
	if errors.Is(err, files.ErrPathTraversal) || errors.Is(err, os.ErrNotExist) {
		status = http.StatusBadRequest // a file that is not there, or a link out of the server
	}
	if errors.Is(err, skypanel.ErrFileExists) {
		status = http.StatusConflict // the archive name is taken: the user picks another one
	}
	if !response.HandleError(c, err, status) {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Extract archive
// @Description Extracts an archive to the server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param filename path string true "Target file to extract"
// @Param destination query string true "Path to place files"
// @Tags Daemon Server
// @Router /api/servers/{id}/extract/{filename} [post]
// @Security OAuth2Application[server.files.edit]
func extract(c *gin.Context) {
	server := getServerFromGin(c)

	targetPath := strings.TrimPrefix(c.Param("filename"), "/")
	destination := c.Query("destination")

	if !filepath.IsLocal(targetPath) {
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid file path: %s", targetPath))
		return
	}
	if destination != "" && !filepath.IsLocal(destination) {
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid destination path: %s", destination))
		return
	}

	_, skipRoot := c.GetQuery("skipRoot")

	var err error
	if skipRoot {
		if err = server.CheckDiskSpace(false); err == nil {
			err = files.ExtractLimited(server.GetFileServer(), targetPath, destination, "*", true, server.DiskWriter)
			server.ForgetDiskUsage()
		}
	} else {
		err = server.Extract(targetPath, destination)
	}

	if !response.HandleError(c, err, diskStatus(err, http.StatusInternalServerError)) {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Create backup
// @Description Creates a full backup of the server
// @Success 200 {object} skypanel.ServerBackupResponse
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /daemon/server/{id}/backup/create [post]
// @Security OAuth2Application[server.backup.create]
func createBackup(c *gin.Context) {
	server := getServerFromGin(c)
	/*isRunning, err := server.IsRunning()
	if err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	} else if isRunning {
		response.HandleError(c, skypanel.ErrBackupServerRunning, http.StatusBadRequest)
		return
	}*/

	id, err := server.StartBackup()

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, &skypanel.ServerBackupResponse{BackupFileName: id})
}

// @Summary List backup files
// @Description Lists the finished backup archives in the server's backup folder, newest first,
// @Description then the backups made by hand whose archive could not be written, with location "failed"
// @Success 200 {object} []skypanel.BackupFile
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /daemon/server/{id}/backup [get]
// @Security OAuth2Application[server.backup.view]
func listBackups(c *gin.Context) {
	server := getServerFromGin(c)

	backups, err := server.ListBackups()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	// the panel recorded those when they were asked for, and drops them when it sees them here
	failed, err := server.FailedBackups()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	for _, name := range failed {
		backups = append(backups, skypanel.BackupFile{FileName: name, Location: servers.BackupFailedLocation})
	}
	c.JSON(http.StatusOK, backups)
}

// @Summary Delete backup
// @Description Delete a backup of the server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param fileName query string true "File Name"
// @Tags Daemon Server
// @Router /daemon/server/{id}/backup [delete]
// @Security OAuth2Application[server.backup.delete]
func deleteBackup(c *gin.Context) {
	server := getServerFromGin(c)
	fileName := c.Query("fileName")

	err := server.DeleteBackup(fileName)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Restore backup
// @Description Restore a full backup of the server
// @Success 202 {object} nil
// @Param id path string true "Server ID"
// @Param fileName query string true "File Name"
// @Tags Daemon Server
// @Router /daemon/server/{id}/backup/restore [post]
// @Security OAuth2Application[server.backup.restore]
func restoreBackup(c *gin.Context) {
	server := getServerFromGin(c)
	fileName := c.Query("fileName")
	isRunning, err := server.IsRunning()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	} else if isRunning {
		response.HandleError(c, skypanel.ErrBackupServerRunning, http.StatusBadRequest)
		return
	}

	err = server.StartRestore(fileName)

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusAccepted)
}

// @Summary Download backup
// @Description Download a backup of the server
// @Produce octet-stream
// @Success 200 {file} binary "Backup archive"
// @Param id path string true "Server ID"
// @Param fileName query string true "File Name"
// @Tags Daemon Server
// @Router /daemon/server/{id}/backup/download [get]
// @Security OAuth2Application[server.backup.view]
func downloadBackup(c *gin.Context) {
	server := getServerFromGin(c)
	fileName := c.Query("fileName")

	data, err := server.GetBackupFile(fileName)
	defer func() {
		if data != nil {
			utils.Close(data.Contents)
		}
	}()
	if !response.HandleError(c, err, http.StatusInternalServerError) {
		fileName := filepath.Base(data.Name)

		extraHeaders := map[string]string{
			"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, fileName),
		}

		// discard the built-in response, we cannot use this one at all
		c.DataFromReader(http.StatusOK, data.ContentLength, "application/octet-stream", data.Contents, extraHeaders)
	}
}

// @Summary Get flags
// @Description Get the management flags for a server
// @Success 200 {object} skypanel.ServerFlags
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/flags [get]
// @Security OAuth2Application[server.flags.view]
func getFlags(c *gin.Context) {
	server := getServerFromGin(c)

	c.JSON(http.StatusOK, &skypanel.ServerFlags{
		AutoStart:             &server.Execution.AutoStart,
		AutoRestartOnCrash:    &server.Execution.AutoRestartFromCrash,
		AutoRestartOnGraceful: &server.Execution.AutoRestartFromGraceful,
	})
}

// @Summary Set flags
// @Description Sets management flags for a server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param flags body skypanel.ServerFlags true "Flags to change"
// @Tags Daemon Server
// @Router /api/servers/{id}/flags [post]
// @Security OAuth2Application[server.flags.edit]
func setFlags(c *gin.Context) {
	server := getServerFromGin(c)

	var req skypanel.ServerFlags
	err := c.BindJSON(&req)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if req.AutoRestartOnCrash != nil {
		server.Execution.AutoRestartFromCrash = *req.AutoRestartOnCrash
	}
	if req.AutoRestartOnGraceful != nil {
		server.Execution.AutoRestartFromGraceful = *req.AutoRestartOnGraceful
	}
	if req.AutoStart != nil {
		server.Execution.AutoStart = *req.AutoStart
	}
	err = server.Save()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Determine if the server supports query protocol
// @Description Returns a 202 if the server can be queried, otherwise 204
// @Success 202 {object} nil
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/query [head]
// @Security OAuth2Application[server.query]
func canQueryServer(c *gin.Context) {
	server := getServerFromGin(c)

	switch server.Query.Type {
	case "minecraft", "source", "steam":
		c.Status(http.StatusAccepted)
	default:
		c.Status(http.StatusNoContent)
	}
}

// @Summary Queries the server for game-specific stats
// @Description Queries the server using the server's protocol to gather information such as players.
// @Description Minecraft servers answer the server list ping; Steam servers (query type "source",
// @Description or no query type at all) answer A2S.
// @Success 200 {object} interface{}
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/query [get]
// @Security OAuth2Application[server.query]
func queryServer(c *gin.Context) {
	server := getServerFromGin(c)

	result, ok := server.QueryGame()
	if !ok {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, result)
}

func openSocket(c *gin.Context) {
	server := getServerFromGin(c)

	conn, err := wsupgrader.Upgrade(c.Writer, c.Request, nil)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	socket := skypanel.Create(conn)

	if _, exists := c.GetQuery("console"); exists {
		server.GetEnvironment().AddConsoleListener(socket)
	}

	if _, exists := c.GetQuery("stats"); exists {
		server.GetEnvironment().AddStatsListener(socket)
	}

	if _, exists := c.GetQuery("status"); exists {
		server.GetEnvironment().AddStatusListener(socket)
	}

	// Serve keeps the connection alive (pings/pongs), detects dead peers,
	// and unregisters the socket from every tracker once it drops. It must
	// run in the background or idle connections get closed by proxies.
	go socket.Serve()
}

func getFullFilename(c *gin.Context) string {
	filename := c.Param("filename")

	if _, exists := c.GetQuery("folder"); exists {
		return filename
	}

	if c.Request.URL.RawQuery != "" {
		filename = filename + "?" + c.Request.URL.RawQuery
	}
	return filename
}

// Plugin management functions

type PluginInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Size    int64  `json:"size"`
}

type PluginSearchResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Tag         string `json:"tag"`
	Description string `json:"description"`
	Author      string `json:"author"`
	IconURL     string `json:"iconUrl,omitempty"`
	Downloads   int    `json:"downloads"`
	Version     string `json:"version,omitempty"`
}

// @Summary Get installed plugins
// @Description Lists the jars in mods/ on Fabric, Forge and NeoForge servers and in plugins/ otherwise
// @Success 200 {array} PluginInfo
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /daemon/server/{id}/plugins [get]
// errPluginsNotMinecraft: plugins and mods come from Modrinth and only run on Minecraft: Java
// Edition, so any other server (Arma, Node.js…) has none to list or install
var errPluginsNotMinecraft = skypanel.CreateError("plugins and mods are only for Minecraft: Java Edition servers", "ErrPluginsNotMinecraft")

// pluginServer is the request's server when it can have plugins; otherwise it answers 400
func pluginServer(c *gin.Context) (*servers.Server, bool) {
	server := getServerFromGin(c)
	if server.Type.Type != "minecraft-java" {
		c.AbortWithStatusJSON(http.StatusBadRequest, &skypanel.ErrorResponse{Error: errPluginsNotMinecraft})
		return nil, false
	}
	return server, true
}

func getPlugins(c *gin.Context) {
	server, ok := pluginServer(c)
	if !ok {
		return
	}

	pluginsDir := pluginFolder(server.PluginPlatform())
	_, err := server.GetFileServer().Stat(pluginsDir)
	if err != nil {
		// Si no existe la carpeta plugins, crear una lista vacía
		c.JSON(http.StatusOK, []PluginInfo{})
		return
	}

	entries, err := server.GetFileServer().ReadDir(pluginsDir)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	plugins := make([]PluginInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Solo archivos .jar
		if !strings.HasSuffix(strings.ToLower(name), ".jar") {
			continue
		}

		fullPath := filepath.Join(pluginsDir, name)
		info, err := server.GetFileServer().Stat(fullPath)
		if err != nil {
			continue
		}

		// Extraer versión del nombre si es posible
		version := ""
		nameWithoutExt := strings.TrimSuffix(name, ".jar")
		if strings.Contains(nameWithoutExt, "-") {
			parts := strings.Split(nameWithoutExt, "-")
			if len(parts) > 1 {
				version = parts[len(parts)-1]
			}
		}

		plugins = append(plugins, PluginInfo{
			Name:    name, // Devolver el nombre completo del archivo (con .jar) para eliminación precisa
			Version: version,
			Size:    info.Size(),
		})
	}

	c.JSON(http.StatusOK, plugins)
}

// pluginLoaders are the Modrinth loaders whose builds run on each platform. Paper and its
// forks also load Spigot and Bukkit plugins. The plugins tab asks the install for the same
// loaders, so what the search shows can be installed.
var pluginLoaders = map[string][]string{
	"paper":    {"paper", "spigot", "bukkit"},
	"fabric":   {"fabric"},
	"forge":    {"forge"},
	"neoforge": {"neoforge"},
}

// pluginFolder is where the server loads its jars from: Fabric, Forge and NeoForge read mods/
// (as the world map install does), Paper and anything the panel cannot tell read plugins/
func pluginFolder(platform string) string {
	switch platform {
	case "fabric", "forge", "neoforge":
		return "mods"
	}
	return "plugins"
}

// pluginSearchFacets limits a Modrinth search to what the server can load: projects with a
// build for one of its loaders, and on a modded server only mods that run on a server (not
// client-only ones). Modrinth's v2 API lists plugins as project_type "mod" too, so for Paper
// the loaders are what keep mods out. An unknown platform lists plugins and mods of any
// loader, as before.
func pluginSearchFacets(platform string) string {
	loaders, ok := pluginLoaders[platform]
	if !ok {
		return `[["project_type:mod","project_type:plugin"]]`
	}
	categories := make([]string, 0, len(loaders))
	for _, loader := range loaders {
		categories = append(categories, "categories:"+loader)
	}
	facets := [][]string{{"project_type:mod", "project_type:plugin"}, categories}
	if pluginFolder(platform) == "mods" {
		facets[0] = []string{"project_type:mod"}
		facets = append(facets, []string{"server_side:required", "server_side:optional"})
	}
	encoded, _ := json.Marshal(facets)
	return string(encoded)
}

// @Summary Search plugins
// @Description Searches Modrinth for plugins or mods that fit the server's platform (Paper plugins, or Fabric, Forge or NeoForge mods); anything when the platform is unknown
// @Success 200 {array} PluginSearchResult
// @Param id path string true "Server ID"
// @Param q query string true "Search query"
// @Tags Daemon Server
// @Router /daemon/server/{id}/plugins/search [get]
func searchPlugins(c *gin.Context) {
	server, ok := pluginServer(c)
	if !ok {
		return
	}

	queryParam := c.Query("q")
	if queryParam == "" {
		response.HandleError(c, errors.New("search query is required"), http.StatusBadRequest)
		return
	}

	// Buscar usando Modrinth API
	u, _ := url.Parse(pluginModrinthAPI + "/search")
	q := u.Query()
	q.Set("query", queryParam)
	q.Set("limit", "20")
	q.Set("facets", pluginSearchFacets(server.PluginPlatform()))
	u.RawQuery = q.Encode()

	modrinthURL := u.String()

	sanitizedQuery := strings.ReplaceAll(queryParam, "\n", "")
	sanitizedQuery = strings.ReplaceAll(sanitizedQuery, "\r", "")

	logging.Debug.Printf("Searching plugins on Modrinth with query: '%s' -> URL: %s", sanitizedQuery, modrinthURL)

	client := &http.Client{}
	req, err := http.NewRequest("GET", modrinthURL, nil)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	req.Header.Set("User-Agent", "PufferPanel/SkyPanel Plugin Manager")

	resp, err := client.Do(req)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		logging.Error.Printf("Error fetching from Modrinth API: %v", err)
		return
	}
	defer utils.CloseResponse(resp)

	if resp.StatusCode != http.StatusOK {
		logging.Error.Printf("Modrinth API returned status %d", resp.StatusCode)
		response.HandleError(c, fmt.Errorf("modrinth API returned status %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}

	var searchResponse struct {
		Hits []map[string]interface{} `json:"hits"`
	}

	err = json.NewDecoder(resp.Body).Decode(&searchResponse)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	results := make([]PluginSearchResult, 0)
	for _, hit := range searchResponse.Hits {
		id, _ := cast.ToStringE(hit["project_id"])
		title, _ := cast.ToStringE(hit["title"])
		author, _ := cast.ToStringE(hit["author"])
		description, _ := cast.ToStringE(hit["description"])
		iconURL, _ := cast.ToStringE(hit["icon_url"])
		downloads, _ := cast.ToIntE(hit["downloads"])
		latestVersion, _ := cast.ToStringE(hit["latest_version"])

		results = append(results, PluginSearchResult{
			ID:          id,
			Name:        title,
			Description: description,
			Author:      author,
			IconURL:     iconURL,
			Downloads:   downloads,
			Version:     latestVersion,
		})
	}

	c.JSON(http.StatusOK, results)
}

// pluginModrinthAPI is a variable so tests can point it at a local server
var pluginModrinthAPI = "https://api.modrinth.com/v2"

// errNoCompatiblePluginVersion is returned when the project has versions, but none for the
// loader and Minecraft version the panel asked for
var errNoCompatiblePluginVersion = skypanel.CreateError("this project has no version for the server's loader and Minecraft version", "ErrPluginNoCompatibleVersion")

// errBadPluginFileName is returned when the file Modrinth names is not a plain .jar name: the
// name becomes the path the jar is written to, so a folder or ".." in it is never followed
var errBadPluginFileName = skypanel.CreateError("the file Modrinth offers for this project is not a .jar with a plain file name", "ErrPluginBadFileName")

// @Summary Install plugin
// @Description Downloads and installs the newest version of a Modrinth project, limited to the given loaders and game versions when they are set, into mods/ on Fabric, Forge and NeoForge servers and plugins/ otherwise
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param pluginId path string true "Modrinth project ID"
// @Param loaders query string false "Comma-separated Modrinth loaders, such as paper,spigot,bukkit"
// @Param game_versions query string false "Comma-separated Minecraft versions, such as 1.21.4"
// @Tags Daemon Server
// @Router /daemon/server/{id}/plugins/{pluginId} [post]
func installPlugin(c *gin.Context) {
	server, ok := pluginServer(c)
	if !ok {
		return
	}

	projectID := c.Param("pluginId")
	if projectID == "" {
		response.HandleError(c, errors.New("project ID is required"), http.StatusBadRequest)
		return
	}

	loaders := pluginFilter(c.Query("loaders"))
	gameVersions := pluginFilter(c.Query("game_versions"))
	downloadURL, fileName, ok := fetchLatestModrinthFile(c, projectID, loaders, gameVersions)
	if !ok {
		return
	}

	if err := server.CheckDiskSpace(false); response.HandleError(c, err, http.StatusConflict) {
		return
	}

	// a plugin can break the server; a restore point lets the owner go back (a busy backup
	// only means this install goes ahead without one). It is made only once Modrinth has a
	// file that fits, so a refused install leaves none behind, and before the download
	// starts, so the download does not sit idle while the archive is written.
	if err := server.CreateRestorePoint(servers.RestoreReasonPlugin); err != nil {
		logging.Info.Printf("[%s] No restore point before installing a plugin: %s", server.ID(), err)
	}

	// Descargar el plugin
	pluginResp, err := http.Get(downloadURL)
	if err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	}
	defer pluginResp.Body.Close()

	if pluginResp.StatusCode != http.StatusOK {
		response.HandleError(c, fmt.Errorf("failed to download plugin from %s", downloadURL), http.StatusInternalServerError)
		return
	}

	// Crear la carpeta si no existe: mods/ en Fabric, Forge y NeoForge, plugins/ en el resto
	pluginsDir := pluginFolder(server.PluginPlatform())
	_, err = server.GetFileServer().Stat(pluginsDir)
	if err != nil && os.IsNotExist(err) {
		err = server.GetFileServer().Mkdir(pluginsDir, 0755)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	}

	pluginPath := filepath.Join(pluginsDir, fileName)

	file, err := server.GetFileServer().OpenFile(pluginPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	defer utils.Close(file)

	_, err = io.Copy(server.DiskWriter(file), pluginResp.Body)
	if errors.Is(err, skypanel.ErrDiskLimitExceeded) {
		utils.Close(file)
		_ = server.GetFileServer().Remove(pluginPath)
		server.ForgetDiskUsage()
	}
	if response.HandleError(c, err, diskStatus(err, http.StatusInternalServerError)) {
		return
	}

	if env := server.GetEnvironment(); env != nil {
		env.DisplayToConsole(true, fmt.Sprintf("Plugin/Mod %s installed successfully as %s\n", projectID, fileName))
	}
	c.Status(http.StatusNoContent)
}

// pluginFilter splits a comma-separated loaders or game_versions value. Only plain names
// and version numbers are kept, so nothing else ends up in the query sent to Modrinth.
func pluginFilter(raw string) []string {
	values := make([]string, 0)
	for _, v := range strings.Split(raw, ",") {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" || len(v) > 32 {
			continue
		}
		valid := true
		for _, r := range v {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '.' && r != '-' && r != '_' {
				valid = false
				break
			}
		}
		if valid {
			values = append(values, v)
		}
	}
	return values
}

// modrinthVersionsURL is the list of a project's versions, newest first. Modrinth keeps
// only the versions made for one of the loaders and one of the game versions when they are
// given (as JSON arrays), so the server gets a jar that runs on it.
func modrinthVersionsURL(projectID string, loaders, gameVersions []string) string {
	query := url.Values{}
	if len(loaders) > 0 {
		encoded, _ := json.Marshal(loaders)
		query.Set("loaders", string(encoded))
	}
	if len(gameVersions) > 0 {
		encoded, _ := json.Marshal(gameVersions)
		query.Set("game_versions", string(encoded))
	}
	versionURL := fmt.Sprintf("%s/project/%s/version", pluginModrinthAPI, url.PathEscape(projectID))
	if len(query) > 0 {
		versionURL += "?" + query.Encode()
	}
	return versionURL
}

// fetchLatestModrinthFile looks up the newest version of a Modrinth project for the given
// loaders and game versions (any, when empty) and returns the download URL and name of its
// primary file (or the first one). It writes the error response itself and returns false
// when the request must stop.
func fetchLatestModrinthFile(c *gin.Context, projectID string, loaders, gameVersions []string) (string, string, bool) {
	versionURL := modrinthVersionsURL(projectID, loaders, gameVersions)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", versionURL, nil)
	req.Header.Set("User-Agent", "PufferPanel/SkyPanel Plugin Manager")

	resp, err := client.Do(req)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return "", "", false
	}
	defer utils.CloseResponse(resp)

	if resp.StatusCode != http.StatusOK {
		response.HandleError(c, fmt.Errorf("project or versions not found on Modrinth"), http.StatusNotFound)
		return "", "", false
	}

	var versions []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Files []struct {
			URL      string `json:"url"`
			FileName string `json:"filename"`
			Primary  bool   `json:"primary"`
		} `json:"files"`
		VersionNumber string `json:"version_number"`
	}

	err = json.NewDecoder(resp.Body).Decode(&versions)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return "", "", false
	}

	if len(versions) == 0 {
		err = fmt.Errorf("no versions found for this project")
		if len(loaders) > 0 || len(gameVersions) > 0 {
			err = errNoCompatiblePluginVersion
		}
		response.HandleError(c, err, http.StatusNotFound)
		return "", "", false
	}

	// Seleccionar la versión más reciente (primera en la lista)
	latestVersion := versions[0]
	if len(latestVersion.Files) == 0 {
		response.HandleError(c, fmt.Errorf("no files found in the latest version"), http.StatusNotFound)
		return "", "", false
	}

	// Buscar el archivo primario o el primero
	var downloadURL string
	var fileName string
	for _, file := range latestVersion.Files {
		if file.Primary || downloadURL == "" {
			downloadURL = file.URL
			fileName = file.FileName
		}
	}
	if !servers.ValidJarName(fileName) {
		response.HandleError(c, errBadPluginFileName, http.StatusBadGateway)
		return "", "", false
	}
	return downloadURL, fileName, true
}

// @Summary Delete plugin
// @Description Deletes a jar from mods/ on Fabric, Forge and NeoForge servers and from plugins/ otherwise
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param name query string true "Plugin file name"
// @Tags Daemon Server
// @Router /daemon/server/{id}/plugins [delete]
func deletePlugin(c *gin.Context) {
	server, ok := pluginServer(c)
	if !ok {
		return
	}

	pluginName := c.Query("name")

	// Prevent path traversal
	pluginName = filepath.Clean(filepath.Base(pluginName))
	// Prevent log injection
	pluginName = strings.ReplaceAll(pluginName, "\n", "")
	pluginName = strings.ReplaceAll(pluginName, "\r", "")

	logging.Debug.Printf("deletePlugin called with query param 'name' = '%s'", pluginName)

	if pluginName == "" || pluginName == "." || pluginName == "/" {
		logging.Error.Printf("deletePlugin: plugin name is required")
		response.HandleError(c, errors.New("plugin name is required (use ?name=plugin.jar)"), http.StatusBadRequest)
		return
	}

	// Asegurar que es un archivo .jar
	originalName := pluginName
	if !strings.HasSuffix(strings.ToLower(pluginName), ".jar") {
		pluginName += ".jar"
		logging.Debug.Printf("deletePlugin: added .jar extension: '%s' -> '%s'", originalName, pluginName)
	}

	// the same folder the list came from: mods/ on Fabric, Forge and NeoForge
	pluginPath := filepath.Join(pluginFolder(server.PluginPlatform()), pluginName)
	logging.Debug.Printf("deletePlugin: checking if file exists at path: '%s'", pluginPath)

	_, err := server.GetFileServer().Stat(pluginPath)
	if err != nil {
		logging.Error.Printf("deletePlugin: file not found at path '%s': %v", pluginPath, err)
		response.HandleError(c, err, http.StatusNotFound)
		return
	}

	// only once there is a file to remove, so a wrong name leaves no restore point behind
	if err := server.CreateRestorePoint(servers.RestoreReasonPlugin); err != nil {
		logging.Info.Printf("[%s] No restore point before removing a plugin: %s", server.ID(), err)
	}

	logging.Debug.Printf("deletePlugin: file found, attempting to remove: '%s'", pluginPath)
	err = server.GetFileServer().Remove(pluginPath)
	if err != nil {
		logging.Error.Printf("deletePlugin: error removing file '%s': %v", pluginPath, err)
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	logging.Debug.Printf("deletePlugin: successfully removed plugin '%s'", pluginPath)
	if env := server.GetEnvironment(); env != nil {
		env.DisplayToConsole(true, fmt.Sprintf("Plugin %s deleted successfully\n", pluginName))
	}
	c.Status(http.StatusNoContent)
}

// @Summary Get crash reports
// @Description The server's latest crash reports, newest first, each with the console lines before the crash
// @Success 200 {object} []servers.CrashReport
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/crashes [get]
// @Security OAuth2Application[server.console]
func getCrashReports(c *gin.Context) {
	server := getServerFromGin(c)

	reports, err := server.CrashReports()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, reports)
}

// @Summary Get notification settings
// @Description The server's own Discord notifications
// @Success 200 {object} servers.Notifications
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/notifications [get]
// @Security OAuth2Application[server.flags.edit]
func getNotifications(c *gin.Context) {
	server := getServerFromGin(c)

	n, err := server.Notifications()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, n)
}

// @Summary Save notification settings
// @Description Saves the server's Discord webhook and which events are sent to it
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param notifications body servers.Notifications true "Notification settings"
// @Tags Daemon Server
// @Router /api/servers/{id}/notifications [put]
// @Security OAuth2Application[server.flags.edit]
func putNotifications(c *gin.Context) {
	server := getServerFromGin(c)

	var n servers.Notifications
	if response.HandleError(c, c.ShouldBindJSON(&n), http.StatusBadRequest) {
		return
	}
	err := server.SetNotifications(n)
	if errors.Is(err, services.ErrInvalidDiscordWebhook) || errors.Is(err, services.ErrInvalidTelegram) || errors.Is(err, services.ErrInvalidWebhook) {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	if !response.HandleError(c, err, http.StatusInternalServerError) {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Get the periodic off-site copy settings
// @Description The server's own periodic off-site copy setting (null follows the panel's), what it ends up using, whether the bucket is set up, and when its last periodic copy was made (null when none is known)
// @Success 200 {object} servers.OffsiteStatus
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/offsite [get]
// @Security OAuth2Application[server.backup.view]
func getOffsiteSettings(c *gin.Context) {
	server := getServerFromGin(c)

	status, err := server.OffsiteStatus()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, status)
}

// @Summary Save the periodic off-site copy settings
// @Description Sets how often the server gets a periodic copy in the bucket (0 = never) and how many of those copies stay there; null goes back to the panel's setting
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse "An interval the panel does not offer, or copies to keep out of 1-50"
// @Param id path string true "Server ID"
// @Param settings body servers.OffsiteSettings true "Periodic off-site copy settings"
// @Tags Daemon Server
// @Router /api/servers/{id}/offsite [put]
// @Security OAuth2Application[server.backup.create]
func putOffsiteSettings(c *gin.Context) {
	server := getServerFromGin(c)

	var settings servers.OffsiteSettings
	if response.HandleError(c, c.ShouldBindJSON(&settings), http.StatusBadRequest) {
		return
	}
	err := server.SetOffsiteSettings(settings)
	if errors.Is(err, skypanel.ErrOffsitePeriodicHours) || errors.Is(err, skypanel.ErrOffsiteKeep) {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	if !response.HandleError(c, err, http.StatusInternalServerError) {
		c.Status(http.StatusNoContent)
	}
}

// testNotificationsLast keeps the test button from being used to flood a channel
var (
	testNotificationsLast     = make(map[string]time.Time)
	testNotificationsLastLock sync.Mutex
)

// @Summary Send a test notification
// @Description Posts a sample message to every channel the server has, and reports each one ("ok" or why it failed)
// @Success 200 {object} map[string]string
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/notifications/test [post]
// @Security OAuth2Application[server.flags.edit]
func testNotifications(c *gin.Context) {
	server := getServerFromGin(c)

	testNotificationsLastLock.Lock()
	last := testNotificationsLast[server.ID()]
	if time.Since(last) < 10*time.Second {
		testNotificationsLastLock.Unlock()
		response.HandleError(c, errors.New("wait a few seconds before sending another test"), http.StatusTooManyRequests)
		return
	}
	testNotificationsLast[server.ID()] = time.Now()
	testNotificationsLastLock.Unlock()

	results, err := server.SendTestNotification()
	if errors.Is(err, servers.ErrNoNotificationChannel) {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	// a channel refusing the message (deleted webhook, wrong token) is the owner's to fix; the
	// page shows which one
	c.JSON(http.StatusOK, results)
}

// @Summary Get performance history
// @Description CPU, memory, network and uptime over the last hour, day or week, averaged per step
// @Success 200 {object} map[string]interface{}
// @Param id path string true "Server ID"
// @Param range query string false "1h, 24h (default) or 7d"
// @Tags Daemon Server
// @Router /api/servers/{id}/metrics [get]
// @Security OAuth2Application[server.stats]
func getMetrics(c *gin.Context) {
	server := getServerFromGin(c)

	name := c.DefaultQuery("range", "24h")
	window, ok := servers.MetricRanges[name]
	if !ok {
		response.HandleError(c, errors.New("range must be 1h, 24h or 7d"), http.StatusBadRequest)
		return
	}
	record, recordAt := server.PlayerRecord()
	c.JSON(http.StatusOK, gin.H{
		"range":  name,
		"step":   int64(window.Step / time.Second),
		"points": server.Metrics(window.Window, window.Step, time.Now()),
		"record": gin.H{"players": record, "at": recordAt},
	})
}

// @Summary Get game versions
// @Description The server's game version and the stable releases it can switch to (Paper, Folia, Velocity, Waterfall, vanilla)
// @Success 200 {object} servers.VersionInfo
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/version [get]
// @Security OAuth2Application[server.view]
func getGameVersions(c *gin.Context) {
	server := getServerFromGin(c)

	info, err := server.GameVersions()
	if errors.Is(err, servers.ErrNoVersionSwitch) {
		c.Status(http.StatusNoContent)
		return
	}
	// the version lists come from PaperMC and Mojang; when they cannot be reached say so
	if response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	c.JSON(http.StatusOK, info)
}

// @Summary Change game version
// @Description Stops the server, saves a restore point, sets the new version, reinstalls and starts it again if it was running. Runs in the background; the console shows each step.
// @Success 202 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/version [post]
// @Security OAuth2Application[server.install]
func changeGameVersion(c *gin.Context) {
	server := getServerFromGin(c)

	var body struct {
		Version string `json:"version" binding:"required"`
	}
	if response.HandleError(c, c.ShouldBindJSON(&body), http.StatusBadRequest) {
		return
	}
	err := server.ChangeVersion(strings.TrimSpace(body.Version))
	switch {
	case errors.Is(err, servers.ErrNoVersionSwitch), errors.Is(err, servers.ErrUnknownVersion):
		response.HandleError(c, err, http.StatusBadRequest)
	case errors.Is(err, servers.ErrVersionChanging):
		response.HandleError(c, err, http.StatusConflict)
	case err != nil:
		response.HandleError(c, err, http.StatusConflict)
	default:
		c.Status(http.StatusAccepted)
	}
}
