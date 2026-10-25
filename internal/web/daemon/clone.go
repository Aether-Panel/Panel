package daemon

import (
	"errors"
	"net/http"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
)

// maxCloneSourceName bounds the original's name shown on the copy's console
const maxCloneSourceName = 80

// registerCloneRoutes adds the route that copies another server's files into this one
func registerCloneRoutes(l *gin.RouterGroup) {
	l.POST("/:serverId/clone-from/:sourceId", middleware.ResolveServerNode, cloneFromServer)
	l.OPTIONS("/:serverId/clone-from/:sourceId", response.CreateOptions("POST"))
}

// @Summary Copy another server's files
// @Description Copies the files of a server on this node into this (new, empty) server in the background. The server counts as installing until the copy ends; the console shows the progress.
// @Success 202 {object} nil
// @Param id path string true "Server ID"
// @Param sourceId path string true "ID of the server to copy from"
// @Tags Daemon Server
// @Router /daemon/server/{id}/clone-from/{sourceId} [post]
func cloneFromServer(c *gin.Context) {
	target := getServerFromGin(c)
	source := servers.GetFromCache(c.Param("sourceId"))
	if source == nil {
		response.HandleError(c, errors.New("the server to copy from is not on this node"), http.StatusNotFound)
		return
	}

	// the body is optional: it only carries the name shown on the console
	var body struct {
		SourceName string `json:"sourceName"`
	}
	if c.Request.ContentLength != 0 {
		_ = c.ShouldBindJSON(&body)
	}
	name := strings.TrimSpace(body.SourceName)
	if r := []rune(name); len(r) > maxCloneSourceName {
		name = string(r[:maxCloneSourceName])
	}

	err := target.CloneFrom(source, name)
	switch {
	case errors.Is(err, servers.ErrCloneSameServer):
		response.HandleError(c, err, http.StatusBadRequest)
	case errors.Is(err, servers.ErrCloneRunning), errors.Is(err, servers.ErrCloneTargetNotEmpty),
		errors.Is(err, skypanel.ErrServerRunning), errors.Is(err, skypanel.ErrBackupInProgress):
		response.HandleError(c, err, http.StatusConflict)
	case err != nil:
		response.HandleError(c, err, http.StatusInternalServerError)
	default:
		c.Status(http.StatusAccepted)
	}
}
