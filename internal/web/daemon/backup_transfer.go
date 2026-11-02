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

// registerBackupTransferRoutes adds the calls a node transfer uses to bring a server's backups
// along: the panel copies each backup from the old node into the new one, and takes the
// copies back out when the transfer is undone
func registerBackupTransferRoutes(l *gin.RouterGroup) {
	l.PUT("/:serverId/backup/file", middleware.ResolveServerNode, receiveBackup)
	l.DELETE("/:serverId/backup/file", middleware.ResolveServerNode, removeLocalBackup)
	l.OPTIONS("/:serverId/backup/file", response.CreateOptions("PUT", "DELETE"))
	l.GET("/:serverId/backup/busy", middleware.ResolveServerNode, backupBusy)
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
