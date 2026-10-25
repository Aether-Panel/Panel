package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// consoleCommandMax caps how much of a console command the activity log keeps
const consoleCommandMax = 200

// activityBodyPeek caps how much of a request body is read to describe it; the rest is left
// untouched for the handler
const activityBodyPeek = 64 * 1024

// serverActivityRoutes maps a mutating route below /:serverId to its action key. Routes left
// out fall back to "server.change"; routes in serverActivityIgnored are never recorded.
var serverActivityRoutes = map[string]string{
	"PUT ":                                  "server.create",
	"DELETE ":                               "server.delete",
	"POST /suspend":                         "server.suspend",
	"PUT /name/:name":                       "server.rename",
	"POST /start":                           "server.start",
	"POST /stop":                            "server.stop",
	"POST /restart":                         "server.restart",
	"POST /kill":                            "server.kill",
	"POST /install":                         "server.install",
	"POST /reload":                          "server.reload",
	"POST /transfer":                        "server.transfer",
	"POST /console":                         "console.command",
	"PUT /file/*filename":                   "file.write",
	"POST /file/*filename":                  "file.rename",
	"DELETE /file/*filename":                "file.delete",
	"POST /archive/*filename":               "file.archive",
	"POST /extract/*filename":               "file.extract",
	"POST /plugins/:pluginId":               "plugin.install",
	"DELETE /plugins":                       "plugin.delete",
	"POST /backup/create":                   "backup.create",
	"POST /backup/restore/:backupID":        "backup.restore",
	"DELETE /backup/:backupID":              "backup.delete",
	"PUT /user/:email":                      "user.edit",
	"DELETE /user/:email":                   "user.remove",
	"POST /data":                            "settings.data",
	"PUT /data":                             "settings.data",
	"PUT /port-settings":                    "settings.ports",
	"PUT /definition":                       "settings.definition",
	"POST /flags":                           "settings.flags",
	"PUT /notifications":                    "settings.notifications",
	"PUT /tasks/:taskId":                    "task.save",
	"DELETE /tasks/:taskId":                 "task.delete",
	"POST /tasks/:taskId/run":               "task.run",
	"POST /extransfer/create":               "extransfer.create",
	"POST /extransfer/pull":                 "extransfer.pull",
	"POST /extransfer/cancel":               "extransfer.cancel",
	"POST /version":                         "server.version",
	"POST /modpacks/install":                "modpack.install",
	"POST /map/install":                     "map.install",
	"DELETE /map":                           "map.remove",
	"POST /file-history/:versionId/restore": "file.restore",
	"POST /clone":                           "server.clone",
}

// serverActivityIgnored are POSTs that change nothing on the server
var serverActivityIgnored = map[string]bool{
	"POST /ai/analyze":         true,
	"POST /ai/chat":            true,
	"POST /notifications/test": true,
}

// serverActivityWrite runs an activity insert; tests swap it for a synchronous call
var serverActivityWrite = func(fn func()) { go fn() }

// activityCapture is what has to be read before the handler runs, because the handler
// consumes the body or removes the row the entry names
type activityCapture struct {
	command    string
	taskName   string
	version    string
	backupName string
	userExists bool
	// label is the readable name a request carries for the log (a modpack, a map, a clone)
	label string
}

// recordServerActivity writes successful changes made below /:serverId to that server's
// activity log. It never fails the request: errors are only logged.
func recordServerActivity(c *gin.Context) {
	route, serverID, ok := serverActivityRoute(c)
	if !ok {
		c.Next()
		return
	}

	capture := captureServerActivity(c, route)

	c.Next()

	if c.Writer.Status() >= http.StatusBadRequest {
		return
	}

	db := serverActivityDB(c)
	if db == nil {
		return
	}
	sa := &services.ServerActivity{DB: db}

	action, detail := describeServerActivity(c, route, capture)
	if action == "server.delete" {
		// the server is gone, and so is the log of what happened to it
		serverActivityWrite(func() {
			if err := sa.DeleteForServer(serverID); err != nil {
				logging.Error.Printf("Failed to clear the activity of deleted server %s: %s", serverID, err.Error())
			}
		})
		return
	}

	entry := &models.ServerActivity{
		ServerID: serverID,
		Action:   action,
		Detail:   detail,
		IP:       c.ClientIP(),
	}
	if v, exists := c.Get("user"); exists {
		if user, isUser := v.(*models.User); isUser && user != nil {
			id := user.ID
			entry.UserID = &id
			entry.Username = user.Username
		}
	}

	serverActivityWrite(func() {
		if err := sa.Record(entry); err != nil {
			logging.Error.Printf("Failed to record activity %s on server %s: %s", action, serverID, err.Error())
		}
	})
}

// serverActivityRoute returns the "METHOD /suffix" key of a mutating request below
// /:serverId and the server it is about
func serverActivityRoute(c *gin.Context) (string, string, bool) {
	switch c.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
	default:
		return "", "", false
	}

	full := c.FullPath()
	const marker = "/:serverId"
	idx := strings.Index(full, marker)
	if idx < 0 {
		return "", "", false
	}
	suffix := full[idx+len(marker):]
	if suffix != "" && !strings.HasPrefix(suffix, "/") {
		return "", "", false
	}

	serverID := c.Param("serverId")
	if serverID == "" || len(serverID) > 20 {
		return "", "", false
	}

	route := c.Request.Method + " " + suffix
	if serverActivityIgnored[route] {
		return "", "", false
	}
	return route, serverID, true
}

func captureServerActivity(c *gin.Context, route string) activityCapture {
	var capture activityCapture
	db := middleware.GetDatabase(c)
	serverID := c.Param("serverId")

	switch route {
	case "POST /console":
		body := peekRequestBody(c, activityBodyPeek)
		capture.command = services.TruncateRunes(strings.TrimSpace(string(body)), consoleCommandMax)
	case "PUT /tasks/:taskId":
		body := peekRequestBody(c, activityBodyPeek)
		var task struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(body, &task) == nil {
			capture.taskName = strings.TrimSpace(task.Name)
		}
	case "POST /version":
		var body struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(peekRequestBody(c, activityBodyPeek), &body) == nil {
			capture.version = services.TruncateRunes(strings.TrimSpace(body.Version), 40)
		}
	case "POST /modpacks/install", "POST /map/install", "POST /clone":
		var body struct {
			Label string `json:"label"`
		}
		if json.Unmarshal(peekRequestBody(c, activityBodyPeek), &body) == nil {
			capture.label = services.TruncateRunes(strings.TrimSpace(body.Label), 80)
		}
	case "POST /backup/restore/:backupID", "DELETE /backup/:backupID":
		if db == nil {
			break
		}
		if id, err := strconv.ParseUint(c.Param("backupID"), 10, 64); err == nil {
			var backup models.Backup
			if db.Select("name").Where("id = ? AND server_id = ?", id, serverID).Limit(1).Find(&backup).Error == nil {
				capture.backupName = backup.Name
			}
		}
	case "PUT /user/:email":
		if db == nil {
			break
		}
		var count int64
		err := db.Model(&models.Permissions{}).
			Joins("JOIN users ON users.id = permissions.user_id").
			Where("users.email = ? AND permissions.server_identifier = ?", c.Param("email"), serverID).
			Count(&count).Error
		capture.userExists = err == nil && count > 0
	}

	return capture
}

// peekRequestBody reads up to limit bytes of the body and puts them back in front of what is
// left, so the handler still sees the whole request
func peekRequestBody(c *gin.Context, limit int64) []byte {
	if c.Request.Body == nil {
		return nil
	}
	original := c.Request.Body
	body, err := io.ReadAll(io.LimitReader(original, limit))
	c.Request.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(body), original), original}
	if err != nil {
		return nil
	}
	return body
}

// describeServerActivity turns a successful request into an action key and its detail
func describeServerActivity(c *gin.Context, route string, capture activityCapture) (string, string) {
	action, known := serverActivityRoutes[route]
	if !known {
		return "server.change", route
	}

	filename := strings.TrimPrefix(c.Param("filename"), "/")

	switch action {
	case "server.rename":
		return action, c.Param("name")
	case "server.suspend":
		return suspendActivity(c, action), ""
	case "console.command":
		return action, capture.command
	case "server.version":
		return action, capture.version
	case "modpack.install", "map.install", "server.clone":
		return action, capture.label
	case "file.restore":
		return action, c.Query("path")
	case "file.write":
		if _, folder := c.GetQuery("folder"); folder {
			return "file.mkdir", filename
		}
		if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
			return "file.upload", filename
		}
		return action, filename
	case "file.rename":
		if dest := c.Query("destination"); dest != "" {
			return action, filename + " → " + dest
		}
		return action, filename
	case "file.delete", "file.archive", "file.extract":
		return action, filename
	case "plugin.install":
		return action, c.Param("pluginId")
	case "plugin.delete":
		return action, c.Query("name")
	case "backup.create":
		return action, c.Query("name")
	case "backup.restore", "backup.delete":
		if capture.backupName != "" {
			return action, capture.backupName
		}
		return action, "#" + c.Param("backupID")
	case "user.edit":
		if !capture.userExists {
			return "user.add", c.Param("email")
		}
		return action, c.Param("email")
	case "user.remove":
		return action, c.Param("email")
	case "task.save":
		if capture.taskName != "" {
			return action, capture.taskName
		}
		return action, c.Param("taskId")
	case "task.delete", "task.run":
		return action, c.Param("taskId")
	}

	// settings and other changes: their bodies may hold secrets (webhooks, passwords), so
	// only the fact that they changed is kept
	return action, ""
}

// suspendActivity tells a suspension from a lift by the state of the request's server
func suspendActivity(c *gin.Context, action string) string {
	if v, ok := c.Get("server"); ok {
		if server, isServer := v.(*models.Server); isServer && server != nil && !server.Suspended {
			return "server.unsuspend"
		}
	}
	return action
}

// serverActivityDB is the connection to write with once the handler is done; a request-wide
// transaction is already closed by then, so the plain connection behind it is used
func serverActivityDB(c *gin.Context) *gorm.DB {
	if v, ok := c.Get("noTransactionDb"); ok {
		if db, isDB := v.(*gorm.DB); isDB && db != nil {
			return db
		}
	}
	return middleware.GetDatabase(c)
}

// registerServerActivity adds the activity log routes
func registerServerActivity(g *gin.RouterGroup) {
	g.GET("/:serverId/activity", middleware.RequiresPermission(scopes.ScopeServerUserView), middleware.ResolveServerPanel, getServerActivity)
	g.OPTIONS("/:serverId/activity", response.CreateOptions("GET"))
}

// @Summary Get a server's activity
// @Description Who changed what on the server, newest first. The IP is only shown to administrators.
// @Success 200 {object} models.ServerActivityResponse
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path string true "Server Identifier"
// @Param limit query int false "Entries per page (max 100)"
// @Param before query int false "Only entries older than this id"
// @Param type query string false "Family (power, console, files, backups, users, settings, tasks) or action prefix"
// @Tags Servers
// @Router /api/servers/{id}/activity [get]
// @Security OAuth2Application[server.users.view]
func getServerActivity(c *gin.Context) {
	server := getServerFromGin(c)

	limit := 50
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}

	var before uint64
	if raw := c.Query("before"); raw != "" {
		// the cursor is an id, which fits in this platform's uint
		parsed, err := strconv.ParseUint(raw, 10, strconv.IntSize)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid cursor"})
			return
		}
		before = parsed
	}

	sa := &services.ServerActivity{DB: middleware.GetDatabase(c)}
	entries, next, err := sa.List(server.Identifier, limit, uint(before), c.Query("type"))
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	withIP := callerIsAdmin(c)
	items := make([]*models.ServerActivityView, len(entries))
	for i, entry := range entries {
		items[i] = models.FromServerActivity(entry, withIP)
	}

	c.JSON(http.StatusOK, &models.ServerActivityResponse{Items: items, NextCursor: next})
}
