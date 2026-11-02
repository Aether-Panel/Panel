package daemon

import (
	"errors"
	"net/http"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
)

// registerTransferRoutes adds the calls a node transfer uses besides the usual ones: the panel
// copies each backup from the old node into the new one (and takes the copies back out when
// the transfer is undone), and pauses the server's tasks on both nodes while it moves
func registerTransferRoutes(l *gin.RouterGroup) {
	l.PUT("/:serverId/backup/file", middleware.ResolveServerNode, receiveBackup)
	l.DELETE("/:serverId/backup/file", middleware.ResolveServerNode, removeLocalBackup)
	l.OPTIONS("/:serverId/backup/file", response.CreateOptions("PUT", "DELETE"))
	l.GET("/:serverId/backup/busy", middleware.ResolveServerNode, backupBusy)
	l.POST("/:serverId/scheduler/pause", middleware.ResolveServerNode, pauseScheduler)
	l.POST("/:serverId/scheduler/resume", middleware.ResolveServerNode, resumeScheduler)
}

// @Summary Pause the server's tasks
// @Description Keeps the server's scheduled tasks from running until they are resumed, also across a restart of the node. A node transfer pauses them while it moves the server.
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /daemon/server/{id}/scheduler/pause [post]
func pauseScheduler(c *gin.Context) {
	setSchedulerPaused(c, true)
}

// @Summary Resume the server's tasks
// @Description Lets the server's scheduled tasks run again after a pause
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /daemon/server/{id}/scheduler/resume [post]
func resumeScheduler(c *gin.Context) {
	setSchedulerPaused(c, false)
}

func setSchedulerPaused(c *gin.Context, paused bool) {
	server := getServerFromGin(c)
	if server.Scheduler == nil {
		response.HandleError(c, errors.New("the server has no scheduler"), http.StatusInternalServerError)
		return
	}
	if response.HandleError(c, server.Scheduler.SetPaused(paused), http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Is a backup running
// @Description Whether a backup, a restore point or a restore of the server is being written right now. A node transfer is refused meanwhile: the backup would not be finished when its folder is moved.
// @Success 200 {object} map[string]bool
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /daemon/server/{id}/backup/busy [get]
func backupBusy(c *gin.Context) {
	server := getServerFromGin(c)
	c.JSON(http.StatusOK, gin.H{"busy": server.IsBackingUp() || server.IsRestoring()})
}

// @Summary Receive a backup
// @Description Writes a backup copied from the server's old node during a node transfer; the body is the archive. A backup with that name already on this node is left alone (409).
// @Accept octet-stream
// @Success 204 {object} nil
// @Failure 409 {object} skypanel.ErrorResponse "The backup is already on this node"
// @Param id path string true "Server ID"
// @Param fileName query string true "File Name"
// @Param modTime query string false "When the backup was made (RFC 3339)"
// @Tags Daemon Server
// @Router /daemon/server/{id}/backup/file [put]
func receiveBackup(c *gin.Context) {
	server := getServerFromGin(c)

	var modTime time.Time
	if raw := c.Query("modTime"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if response.HandleError(c, err, http.StatusBadRequest) {
			return
		}
		modTime = parsed
	}

	err := server.ReceiveBackup(c.Query("fileName"), modTime, c.Request.Body)
	if errors.Is(err, skypanel.ErrFileExists) {
		response.HandleError(c, err, http.StatusConflict)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Remove a backup copy
// @Description Deletes this node's copy of a backup and leaves its off-site copy, for a node transfer that is undone
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param fileName query string true "File Name"
// @Tags Daemon Server
// @Router /daemon/server/{id}/backup/file [delete]
func removeLocalBackup(c *gin.Context) {
	server := getServerFromGin(c)

	if response.HandleError(c, server.RemoveLocalBackup(c.Query("fileName")), http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}
