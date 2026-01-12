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

	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/logging"
	"github.com/SkyPanel/SkyPanel/v3/middleware"
	"github.com/SkyPanel/SkyPanel/v3/query"
	"github.com/SkyPanel/SkyPanel/v3/response"
	"github.com/SkyPanel/SkyPanel/v3/servers"
	"github.com/SkyPanel/SkyPanel/v3/utils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron/v2"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/websocket"
	"github.com/spf13/cast"
)

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
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

		l.GET("/:serverId/tasks/:taskId", middleware.ResolveServerNode, getServerTask)
		l.PUT("/:serverId/tasks/:taskId", middleware.ResolveServerNode, editServerTask)
		l.DELETE("/:serverId/tasks/:taskId", middleware.ResolveServerNode, deleteServerTask)
		l.OPTIONS("/:serverId/tasks/:taskId", response.CreateOptions("GET", "PUT", "DELETE"))

		l.POST("/:serverId/tasks/:taskId/run", middleware.ResolveServerNode, runServerTask)
		l.OPTIONS("/:serverId/tasks/:taskId/run", response.CreateOptions("POST"))

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
		l.POST("/:serverId/file/*filename", middleware.ResolveServerNode, response.NotImplemented)
		l.OPTIONS("/:serverId/file/*filename", response.CreateOptions("GET", "PUT", "DELETE", "POST"))

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

		l.POST("/:serverId/backup/create", middleware.ResolveServerNode, createBackup)
		l.DELETE("/:serverId/backup", middleware.ResolveServerNode, deleteBackup)
		l.POST("/:serverId/backup/restore", middleware.ResolveServerNode, restoreBackup)
		l.GET("/:serverId/backup/download", middleware.ResolveServerNode, downloadBackup)

		l.HEAD("/:serverId/query", middleware.ResolveServerNode, canQueryServer)
		l.GET("/:serverId/query", middleware.ResolveServerNode, queryServer)

		l.GET("/:serverId/plugins", middleware.ResolveServerNode, getPlugins)
		l.DELETE("/:serverId/plugins", middleware.ResolveServerNode, deletePlugin)
		l.OPTIONS("/:serverId/plugins", response.CreateOptions("GET", "DELETE"))
		l.GET("/:serverId/plugins/search", middleware.ResolveServerNode, searchPlugins)
		l.OPTIONS("/:serverId/plugins/search", response.CreateOptions("GET"))
		l.POST("/:serverId/plugins/:pluginId", middleware.ResolveServerNode, installPlugin)
		l.OPTIONS("/:serverId/plugins/:pluginId", response.CreateOptions("POST"))

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

// @Summary Start server
// @Description Start server
// @Success 202 {object} nil
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Router /api/servers/{id}/start [post]
// @Security OAuth2Application[server.start]
func startServer(c *gin.Context) {
	server := getServerFromGin(c)
	_, wait := c.GetQuery("wait")

	if wait {
		err := server.Start()
		if response.HandleError(c, err, http.StatusInternalServerError) {
		} else {
			c.Status(http.StatusNoContent)
		}
	} else {
		go func() {
			err := server.Start()
			if err != nil {
				logging.Error.Printf("Error starting server %s: %s", server.Id(), err)
			}
		}()
		c.Status(http.StatusAccepted)
	}
}

func doRestart(server *servers.Server) error {
	err := server.Stop()
	if err != nil {
		return err
	}
	err = server.GetEnvironment().WaitForMainProcess()
	if err != nil {
		return err
	}
	return server.Start()
}

// @Summary Restart server
// @Description Restart server
// @Success 202 {object} nil
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Router /api/servers/{id}/restart [post]
// @Security OAuth2Application[server.start, server.stop]
func restartServer(c *gin.Context) {
	server := getServerFromGin(c)
	_, wait := c.GetQuery("wait")

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
				logging.Error.Printf("Error restarting server %s: %s", server.Id(), err)
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
		if response.HandleError(c, err, http.StatusInternalServerError) {
		} else {
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
// @Router /api/servers/{id}/kill [post]
// @Security OAuth2Application[server.kill]
func killServer(c *gin.Context) {
	server := getServerFromGin(c)

	err := server.Kill()
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

// Already declared in panel routing
func createServer(c *gin.Context) {
	serverId := c.Param("serverId")
	if serverId == "" {
		id, err := uuid.NewV4()
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		serverId = id.String()
	}
	prg := servers.GetFromCache(serverId)

	if prg != nil {
		response.HandleError(c, SkyPanel.ErrServerAlreadyExists, http.StatusConflict)
		return
	}

	prg = servers.CreateProgram()
	err := json.NewDecoder(c.Request.Body).Decode(prg)
	if err != nil {
		logging.Error.Printf("Error decoding JSON body: %s", err)
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	prg.Identifier = serverId

	err = prg.Requirements.Test(prg.Server)
	if err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	if prg, err = servers.Create(prg); err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		if prg != nil {
			_ = servers.Delete(prg.Id())
		}
		return
	}

	c.JSON(http.StatusOK, &SkyPanel.ServerIdResponse{Id: serverId})
}

// Already declared in panel routing
func deleteServer(c *gin.Context) {
	server := getServerFromGin(c)

	if running, err := server.IsRunning(); running || err != nil {
		if response.HandleError(c, err, http.StatusInternalServerError) {
		} else {
			response.HandleError(c, SkyPanel.ErrServerRunning, http.StatusNotAcceptable)
		}
		return
	}

	err := servers.Delete(server.Id())
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Install server
// @Description Install server
// @Success 202 {object} nil
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Router /api/servers/{id}/install [post]
// @Security OAuth2Application[server.install]
func installServer(c *gin.Context) {
	server := getServerFromGin(c)

	_, wait := c.GetQuery("wait")

	if wait {
		err := server.Install()
		if response.HandleError(c, err, http.StatusInternalServerError) {
		} else {
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
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
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

	err = server.EditData(data, true)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Get server tasks
// @Description Get server tasks
// @Success 200 {object} SkyPanel.ServerTasks
// @Param id path string true "Server ID"
// @Router /api/servers/{id}/tasks [get]
// @Security OAuth2Application[server.tasks.view]
func getServerTasks(c *gin.Context) {
	server := getServerFromGin(c)

	result := SkyPanel.ServerTasks{
		Tasks: make(map[string]SkyPanel.ServerTask),
	}

	for k, v := range server.Scheduler.Tasks {
		result.Tasks[k] = SkyPanel.ServerTask{
			Task: SkyPanel.Task{
				Name:         v.Name,
				CronSchedule: v.CronSchedule,
				Description:  v.Description,
			},
			//IsRunning: server.Scheduler.IsTaskRunning(k),
		}
	}

	c.JSON(http.StatusOK, result)
}

// @Summary Get server task
// @Description Get server task by id
// @Success 200 {object} SkyPanel.ServerTask
// @Param id path string true "Server ID"
// @Param taskId path string true "Task ID"
// @Router /api/servers/{id}/tasks/{taskId} [get]
// @Security OAuth2Application[server.tasks.view]
func getServerTask(c *gin.Context) {
	server := getServerFromGin(c)

	var result *SkyPanel.ServerTask

	for _, v := range server.Scheduler.Tasks {
		result = &SkyPanel.ServerTask{
			Task: v,
			//IsRunning: server.Scheduler.IsTaskRunning(k),
		}
	}

	if result != nil {
		c.JSON(http.StatusOK, result)
	} else {
		c.Status(http.StatusNotFound)
	}
}

// @Summary Run server task
// @Description Run a specific task
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param taskId path string true "Task ID"
// @Router /api/servers/{id}/tasks/{taskId}/run [post]
// @Security OAuth2Application[server.tasks.run]
func runServerTask(c *gin.Context) {
	server := getServerFromGin(c)

	taskId := c.Param("taskId")

	err := server.Scheduler.RunTask(taskId)
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
// @Param taskId path string true "Task ID"
// @Param task body SkyPanel.Task true "Task definition"
// @Router /api/servers/{id}/tasks/{taskId} [put]
// @Security OAuth2Application[server.tasks.edit]
func editServerTask(c *gin.Context) {
	server := getServerFromGin(c)

	taskId := c.Param("taskId")

	var task SkyPanel.Task
	err := c.ShouldBindJSON(&task)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	err = server.Scheduler.RemoveTask(taskId)
	if errors.Is(err, gocron.ErrJobNotFound) {
		err = nil
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = server.Scheduler.AddTask(taskId, task)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Delete server task
// @Description Delete server task by id
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param taskId path string true "Task ID"
// @Router /api/servers/{id}/tasks/{taskId} [delete]
// @Security OAuth2Application[server.tasks.delete]
func deleteServerTask(c *gin.Context) {
	server := getServerFromGin(c)

	taskId := c.Param("taskId")

	err := server.Scheduler.RemoveTask(taskId)
	if errors.Is(err, gocron.ErrJobNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Reload server
// @Description Reload server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Router /api/servers/{id}/reload [post]
// @Security OAuth2Application[server.reload]
func reloadServer(c *gin.Context) {
	server := getServerFromGin(c)

	err := servers.Reload(server.Id())
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Get server data
// @Description Get server variables
// @Success 200 {object} SkyPanel.ServerData
// @Param id path string true "Server ID"
// @Router /api/servers/{id}/data [get]
// @Security OAuth2Application[server.data.view]
func getServerData(c *gin.Context) {
	server := getServerFromGin(c)

	data := server.GetData()

	var replacement = make(map[string]SkyPanel.Variable)
	for k, v := range data {
		if v.UserEditable {
			replacement[k] = v
		}
	}

	c.JSON(http.StatusOK, &SkyPanel.ServerData{Variables: replacement, Groups: server.Groups})
}

// @Summary Get server definition
// @Description Get server definition
// @Success 200 {object} SkyPanel.Server
// @Param id path string true "Server ID"
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
// @Param server body SkyPanel.Server true "New definition"
// @Router /api/servers/{id}/definition [post]
// @Security OAuth2Application[server.definition.edit]
func editServerAdmin(c *gin.Context) {
	prg := getServerFromGin(c)
	server := &prg.Server

	if err := prg.IsIdle(); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	replacement := &SkyPanel.Server{}
	err := c.BindJSON(replacement)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	//backup, just in case we break
	backup := &SkyPanel.Server{}
	backup.CopyFrom(server)

	//copy from request
	server.CopyFrom(replacement)

	err = servers.Save(prg.Id())
	if response.HandleError(c, err, http.StatusInternalServerError) {
		//REVERT!!!!!!!
		server.CopyFrom(backup)
		return
	}

	err = servers.Reload(prg.Id())

	if response.HandleError(c, err, http.StatusInternalServerError) {
		//attempt to revert... but no promise this works
		server.CopyFrom(backup)
		_ = servers.Reload(prg.Id())
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
		if os.IsNotExist(err) {
			c.AbortWithStatus(http.StatusNotFound)
		} else if errors.Is(err, SkyPanel.ErrIllegalFileAccess) {
			response.HandleError(c, err, http.StatusBadRequest)
		} else {
			response.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	if data.FileList != nil {
		c.JSON(http.StatusOK, data.FileList)
	} else if data.Contents != nil {
		fileName := filepath.Base(data.Name)

		extraHeaders := map[string]string{
			"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, fileName),
		}

		//discard the built-in response, we cannot use this one at all
		c.DataFromReader(http.StatusOK, data.ContentLength, "application/octet-stream", data.Contents, extraHeaders)
	} else {
		//uhhhhhhhhhhhhh
		response.HandleError(c, errors.New("no file content or file list"), http.StatusInternalServerError)
	}
}

// @Summary Edit file
// @Description Adds or edit a file, replacing the contents with the provided body
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param filepath path string true "File path"
// @Param file formData file true "File contents"
// @Accept multipart/form-data
// @Router /api/servers/{id}/file/{filepath} [put]
// @Security OAuth2Application[server.files.edit]
func putFile(c *gin.Context) {
	server := getServerFromGin(c)

	targetPath := getFullFilename(c)

	if targetPath == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var err error

	_, mkFolder := c.GetQuery("folder")
	if mkFolder {
		err = server.GetFileServer().MkdirAll(targetPath, 0755)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		c.Status(http.StatusNoContent)
		return
	}

	var sourceFile io.ReadCloser

	v := c.Request.Header.Get("Content-Type")
	if t, _, _ := mime.ParseMediaType(v); t == "multipart/form-data" {
		sourceFile, _, err = c.Request.FormFile("file")
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	} else {
		sourceFile = c.Request.Body
	}

	file, err := server.GetFileServer().OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	defer utils.Close(file)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	_, err = io.Copy(file, sourceFile)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Delete file
// @Description Deletes a file or folder.
// @Description WARNING: This is a recursive operation, specifying a folder will delete all children
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param filepath path string true "File path"
// @Router /api/servers/{id}/file/{filepath} [delete]
// @Security OAuth2Application[server.files.edit]
func deleteFile(c *gin.Context) {
	server := getServerFromGin(c)

	targetPath := getFullFilename(c)

	fi, err := server.GetFileServer().Stat(targetPath)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if fi.IsDir() {
		err = server.GetFileServer().RemoveAll(targetPath)
	} else {
		err = server.GetFileServer().Remove(targetPath)
	}

	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Send command
// @Description Sends a command to the server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param command body string true "Command"
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
// @Success 200 {object} SkyPanel.ServerStats
// @Param id path string true "Server ID"
// @Router /api/servers/{id}/stats [get]
// @Security OAuth2Application[server.stats]
func getStats(c *gin.Context) {
	server := getServerFromGin(c)

	results, err := server.GetEnvironment().GetStats()
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.JSON(http.StatusOK, results)
	}
}

// @Summary Get logs
// @Description Get the console logs for the server
// @Success 200 {object} SkyPanel.ServerLogs
// @Param id path string true "Server ID"
// @Param time query int64 false "Epoch time in MS to get from"
// @Router /api/servers/{id}/console [get]
// @Security OAuth2Application[server.console]
func getLogs(c *gin.Context) {
	server := getServerFromGin(c)

	time := c.DefaultQuery("time", "0")

	castedTime, ok := cast.ToInt64E(time)
	if ok != nil {
		response.HandleError(c, SkyPanel.ErrInvalidUnixTime, http.StatusBadRequest)
		return
	}

	console, epoch := server.GetEnvironment().GetConsoleFrom(castedTime)

	c.JSON(http.StatusOK, &SkyPanel.ServerLogs{
		Epoch: epoch,
		Logs:  console,
	})
}

// @Summary Get status
// @Description Get the server's status (is it running)
// @Success 200 {object} SkyPanel.ServerRunning
// @Param id path string true "Server ID"
// @Router /api/servers/{id}/status [get]
// @Security OAuth2Application[server.status]
func getStatus(c *gin.Context) {
	server := getServerFromGin(c)

	installing := server.GetEnvironment().IsInstalling()

	if installing {
		c.JSON(http.StatusOK, &SkyPanel.ServerRunning{Installing: installing})
		return
	}

	running, err := server.IsRunning()

	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.JSON(http.StatusOK, &SkyPanel.ServerRunning{Running: running})
	}
}

// @Summary Create archive
// @Description Creates an archive of files or folders
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param files body []string true "Files to archive"
// @Param filename path string true "Archive name"
// @Router /api/servers/{id}/archive/{filename} [post]
// @Security OAuth2Application[server.files.edit]
func archive(c *gin.Context) {
	server := getServerFromGin(c)
	var files []string

	if err := c.BindJSON(&files); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if len(files) == 0 {
		c.Status(http.StatusBadRequest)
		return
	}
	destination := c.Param("filename")

	err := server.ArchiveItems(files, destination)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Extract archive
// @Description Extracts an archive to the server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param filename path string true "Target file to extract"
// @Param destination query string true "Path to place files"
// @Router /api/servers/{id}/extract/{filename} [post]
// @Security OAuth2Application[server.files.edit]
func extract(c *gin.Context) {
	server := getServerFromGin(c)

	targetPath := c.Param("filename")
	destination := c.Query("destination")

	err := server.Extract(targetPath, destination)
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		c.Status(http.StatusNoContent)
	}
}

// @Summary Create backup
// @Description Creates a full backup of the server
// @Success 200 {object} SkyPanel.ServerBackupResponse
// @Param id path string true "Server ID"
// @Router /api/servers/{id}/backup/create [post]
// @Security OAuth2Application[server.backup.create]
func createBackup(c *gin.Context) {
	server := getServerFromGin(c)
	isRunning, err := server.IsRunning()
	if err != nil {
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	} else if isRunning {
		response.HandleError(c, SkyPanel.ErrBackupServerRunning, http.StatusBadRequest)
		return
	}

	id, err := server.StartBackup()

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, &SkyPanel.ServerBackupResponse{BackupFileName: id})
}

// @Summary Delete backup
// @Description Delete a backup of the server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param fileName query string true "File Name"
// @Router /api/servers/{id}/backup/delete [post]
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
// @Router /api/servers/{id}/backup/restore [post]
// @Security OAuth2Application[server.backup.restore]
func restoreBackup(c *gin.Context) {
	server := getServerFromGin(c)
	fileName := c.Query("fileName")
	isRunning, err := server.IsRunning()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	} else if isRunning {
		response.HandleError(c, SkyPanel.ErrBackupServerRunning, http.StatusBadRequest)
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
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param fileName query string true "File Name"
// @Router /api/servers/{id}/backup/restore [post]
// @Security OAuth2Application[server.backup.restore]
func downloadBackup(c *gin.Context) {
	server := getServerFromGin(c)
	fileName := c.Query("fileName")

	data, err := server.GetBackupFile(fileName)
	defer func() {
		if data != nil {
			utils.Close(data.Contents)
		}
	}()
	if response.HandleError(c, err, http.StatusInternalServerError) {
	} else {
		fileName := filepath.Base(data.Name)

		extraHeaders := map[string]string{
			"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, fileName),
		}

		//discard the built-in response, we cannot use this one at all
		c.DataFromReader(http.StatusOK, data.ContentLength, "application/octet-stream", data.Contents, extraHeaders)
	}
}

// @Summary Get flags
// @Description Get the management flags for a server
// @Success 200 {object} SkyPanel.ServerFlags
// @Param id path string true "Server ID"
// @Router /api/servers/{id}/flags [get]
// @Security OAuth2Application[server.flags.view]
func getFlags(c *gin.Context) {
	server := getServerFromGin(c)

	c.JSON(http.StatusOK, &SkyPanel.ServerFlags{
		AutoStart:             &server.Execution.AutoStart,
		AutoRestartOnCrash:    &server.Execution.AutoRestartFromCrash,
		AutoRestartOnGraceful: &server.Execution.AutoRestartFromGraceful,
	})
}

// @Summary Set flags
// @Description Sets management flags for a server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param flags body SkyPanel.ServerFlags true "Flags to change"
// @Router /api/servers/{id}/flags [post]
// @Security OAuth2Application[server.flags.edit]
func setFlags(c *gin.Context) {
	server := getServerFromGin(c)

	var req SkyPanel.ServerFlags
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
// @Router /api/servers/{id}/query [head]
// @Security OAuth2Application[server.query]
func canQueryServer(c *gin.Context) {
	server := getServerFromGin(c)

	switch server.Query.Type {
	case "minecraft":
		c.Status(http.StatusAccepted)
	default:
		c.Status(http.StatusNoContent)
	}
}

// @Summary Queries the server for game-specific stats
// @Description Queries the server using the server's protocol to gather information such as players.
// @Success 200 {object} interface{}
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Router /api/servers/{id}/query [get]
// @Security OAuth2Application[server.query]
func queryServer(c *gin.Context) {
	server := getServerFromGin(c)

	if running, err := server.IsRunning(); err != nil || !running {
		c.Status(http.StatusNoContent)
		return
	}

	result := make(map[string]interface{})

	switch server.Query.Type {
	case "minecraft":
		ip := cast.ToString(server.DataToMap()["ip"])
		if ip == "" {
			c.Status(http.StatusNoContent)
			return
		}

		port := cast.ToInt(server.DataToMap()["port"])
		if port <= 0 {
			c.Status(http.StatusNoContent)
			return
		}

		res, err := query.Minecraft(ip, port)
		if err != nil {
			c.Status(http.StatusNoContent)
			return
		}
		result["minecraft"] = res
	default:
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

	socket := SkyPanel.Create(conn)

	if _, exists := c.GetQuery("console"); exists {
		server.GetEnvironment().AddConsoleListener(socket)
	}

	if _, exists := c.GetQuery("stats"); exists {
		server.GetEnvironment().AddStatsListener(socket)
	}

	if _, exists := c.GetQuery("status"); exists {
		server.GetEnvironment().AddStatusListener(socket)
	}
}

func getFullFilename(c *gin.Context) string {
	filename := c.Param("filename")

	if k := c.Query("folder"); k != "" {
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
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Tag         string `json:"tag"`
	Description string `json:"description"`
	Author      string `json:"author"`
	IconURL     string `json:"iconUrl,omitempty"`
	Downloads   int    `json:"downloads"`
	Version     string `json:"version,omitempty"`
}

// @Summary Get installed plugins
// @Description Gets list of plugins installed in the server
// @Success 200 {array} PluginInfo
// @Param id path string true "Server ID"
// @Router /daemon/server/{id}/plugins [get]
func getPlugins(c *gin.Context) {
	server := getServerFromGin(c)

	pluginsDir := "plugins"
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

// @Summary Search plugins
// @Description Searches for plugins using Spigot API
// @Success 200 {array} PluginSearchResult
// @Param id path string true "Server ID"
// @Param q query string true "Search query"
// @Router /daemon/server/{id}/plugins/search [get]
func searchPlugins(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		response.HandleError(c, errors.New("search query is required"), http.StatusBadRequest)
		return
	}

	// Buscar usando Spigot API
	// url.PathEscape codifica correctamente espacios para usar en la ruta del URL
	// Esto maneja correctamente búsquedas con espacios como "terra form"
	encodedQuery := url.PathEscape(query)
	spigotURL := fmt.Sprintf("https://api.spiget.org/v2/search/resources/%s?field=name&size=20", encodedQuery)

	logging.Debug.Printf("Searching plugins with query: '%s' -> encoded: '%s' -> URL: %s", query, encodedQuery, spigotURL)

	resp, err := http.Get(spigotURL)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		logging.Error.Printf("Error fetching from Spigot API: %v", err)
		return
	}
	defer utils.CloseResponse(resp)

	if resp.StatusCode != http.StatusOK {
		logging.Error.Printf("Spigot API returned status %d", resp.StatusCode)
		response.HandleError(c, fmt.Errorf("spigot API returned status %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.Error.Printf("Error reading Spigot API response: %v", err)
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	var resources []map[string]interface{}

	err = json.Unmarshal(body, &resources)
	if err != nil {
		bodyPreview := string(body)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "..."
		}
		logging.Error.Printf("Error decoding Spigot API response: %v\nResponse body (first 500 chars): %s", err, bodyPreview)
		response.HandleError(c, fmt.Errorf("failed to parse Spigot API response: %v", err), http.StatusInternalServerError)
		return
	}

	results := make([]PluginSearchResult, 0)
	for _, res := range resources {
		// Extraer campos de forma segura
		id, _ := cast.ToIntE(res["id"])
		name, _ := cast.ToStringE(res["name"])
		tag, _ := cast.ToStringE(res["tag"])

		// Extraer autor
		authorName := "Unknown"
		if author, ok := res["author"].(map[string]interface{}); ok {
			if name, ok := author["name"].(string); ok {
				authorName = name
			}
		}

		// Extraer descargas
		downloads := 0
		if download, ok := res["download"].(map[string]interface{}); ok {
			if dls, ok := download["downloads"].(float64); ok {
				downloads = int(dls)
			}
		}

		// Extraer versión
		version := ""
		if ver, ok := res["version"].(map[string]interface{}); ok {
			if verName, ok := ver["name"].(string); ok {
				version = verName
			}
		}

		// Extraer icono
		iconURL, _ := cast.ToStringE(res["iconUrl"])

		// Usar tag como descripción por defecto
		description := tag
		if tag == "" {
			description = "No description available"
		}

		results = append(results, PluginSearchResult{
			ID:          id,
			Name:        name,
			Tag:         tag,
			Description: description,
			Author:      authorName,
			IconURL:     iconURL,
			Downloads:   downloads,
			Version:     version,
		})
	}

	c.JSON(http.StatusOK, results)
}

// @Summary Install plugin
// @Description Downloads and installs a plugin from Spigot
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param pluginId path int true "Plugin ID from Spigot"
// @Router /daemon/server/{id}/plugins/{pluginId} [post]
func installPlugin(c *gin.Context) {
	server := getServerFromGin(c)

	pluginID := c.Param("pluginId")
	if pluginID == "" {
		response.HandleError(c, errors.New("plugin ID is required"), http.StatusBadRequest)
		return
	}

	// Obtener información del plugin
	infoURL := fmt.Sprintf("https://api.spiget.org/v2/resources/%s", pluginID)
	resp, err := http.Get(infoURL)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	defer utils.CloseResponse(resp)

	if resp.StatusCode != http.StatusOK {
		response.HandleError(c, fmt.Errorf("plugin not found"), http.StatusNotFound)
		return
	}

	var pluginInfo struct {
		Name    string `json:"name"`
		Version struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"version"`
	}

	err = json.NewDecoder(resp.Body).Decode(&pluginInfo)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// Descargar el plugin
	downloadURL := fmt.Sprintf("https://api.spiget.org/v2/resources/%s/download", pluginID)
	pluginResp, err := http.Get(downloadURL)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	defer utils.CloseResponse(pluginResp)

	if pluginResp.StatusCode != http.StatusOK {
		response.HandleError(c, fmt.Errorf("failed to download plugin"), http.StatusInternalServerError)
		return
	}

	// Crear carpeta plugins si no existe
	pluginsDir := "plugins"
	_, err = server.GetFileServer().Stat(pluginsDir)
	if err != nil && os.IsNotExist(err) {
		err = server.GetFileServer().Mkdir(pluginsDir, 0755)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	}

	// Guardar el plugin
	safeName := strings.ReplaceAll(pluginInfo.Name, " ", "_")
	safeName = strings.ReplaceAll(safeName, "/", "_")
	pluginFileName := fmt.Sprintf("%s-%s.jar", safeName, pluginInfo.Version.Name)
	pluginPath := filepath.Join(pluginsDir, pluginFileName)

	file, err := server.GetFileServer().OpenFile(pluginPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	defer utils.Close(file)

	_, err = io.Copy(file, pluginResp.Body)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if env := server.GetEnvironment(); env != nil {
		env.DisplayToConsole(true, fmt.Sprintf("Plugin %s installed successfully\n", pluginInfo.Name))
	}
	c.Status(http.StatusNoContent)
}

// @Summary Delete plugin
// @Description Deletes a plugin from the server
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param name query string true "Plugin file name"
// @Router /daemon/server/{id}/plugins [delete]
func deletePlugin(c *gin.Context) {
	server := getServerFromGin(c)

	pluginName := c.Query("name")
	logging.Debug.Printf("deletePlugin called with query param 'name' = '%s'", pluginName)

	if pluginName == "" {
		logging.Error.Printf("deletePlugin: plugin name is required")
		response.HandleError(c, errors.New("plugin name is required (use ?name=plugin.jar)"), http.StatusBadRequest)
		return
	}

	// Asegurar que es un archivo .jar
	originalName := pluginName
	if !strings.HasSuffix(strings.ToLower(pluginName), ".jar") {
		pluginName = pluginName + ".jar"
		logging.Debug.Printf("deletePlugin: added .jar extension: '%s' -> '%s'", originalName, pluginName)
	}

	pluginPath := filepath.Join("plugins", pluginName)
	logging.Debug.Printf("deletePlugin: checking if file exists at path: '%s'", pluginPath)

	_, err := server.GetFileServer().Stat(pluginPath)
	if err != nil {
		logging.Error.Printf("deletePlugin: file not found at path '%s': %v", pluginPath, err)
		response.HandleError(c, err, http.StatusNotFound)
		return
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
