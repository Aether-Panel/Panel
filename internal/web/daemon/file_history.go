package daemon

import (
	"errors"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/gin-gonic/gin"
)

// registerFileHistoryRoutes adds the file version history routes
func registerFileHistoryRoutes(l *gin.RouterGroup) {
	l.GET("/:serverId/file-history", middleware.ResolveServerNode, getFileHistory)
	l.OPTIONS("/:serverId/file-history", response.CreateOptions("GET"))
	l.GET("/:serverId/file-history/:versionId", middleware.ResolveServerNode, getFileHistoryVersion)
	l.OPTIONS("/:serverId/file-history/:versionId", response.CreateOptions("GET"))
	l.POST("/:serverId/file-history/:versionId/restore", middleware.ResolveServerNode, restoreFileHistoryVersion)
	l.OPTIONS("/:serverId/file-history/:versionId/restore", response.CreateOptions("POST"))
}

// recordFileVersion keeps the current content of a file that is about to be overwritten
func recordFileVersion(server *servers.Server, path string) {
	server.RecordFileVersion(path)
}

// fileHistoryStatus maps history errors to the status the panel shows
func fileHistoryStatus(err error) int {
	switch {
	case errors.Is(err, servers.ErrFileHistoryPath):
		return http.StatusBadRequest
	case errors.Is(err, servers.ErrFileVersionNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// @Summary List file versions
// @Description Lists the stored previous versions of a file, newest first
// @Success 200 {array} servers.FileVersion
// @Param id path string true "Server ID"
// @Param path query string true "File path, relative to the server folder"
// @Tags Daemon Server
// @Router /api/servers/{id}/file-history [get]
// @Security OAuth2Application[server.files.view]
func getFileHistory(c *gin.Context) {
	server := getServerFromGin(c)

	versions, err := server.FileVersions(c.Query("path"))
	if err != nil {
		response.HandleError(c, err, fileHistoryStatus(err))
		return
	}
	c.JSON(http.StatusOK, versions)
}

// @Summary Get a file version
// @Description Returns the text of one stored version of a file
// @Success 200 {string} string
// @Param id path string true "Server ID"
// @Param versionId path string true "Version ID"
// @Param path query string true "File path, relative to the server folder"
// @Tags Daemon Server
// @Router /api/servers/{id}/file-history/{versionId} [get]
// @Security OAuth2Application[server.files.view]
func getFileHistoryVersion(c *gin.Context) {
	server := getServerFromGin(c)

	data, err := server.FileVersionContent(c.Query("path"), c.Param("versionId"))
	if err != nil {
		response.HandleError(c, err, fileHistoryStatus(err))
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", data)
}

// @Summary Restore a file version
// @Description Writes a stored version back to the file. The current text is kept as a version
// @Description first, so the restore can be undone.
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param versionId path string true "Version ID"
// @Param path query string true "File path, relative to the server folder"
// @Tags Daemon Server
// @Router /api/servers/{id}/file-history/{versionId}/restore [post]
// @Security OAuth2Application[server.files.edit]
func restoreFileHistoryVersion(c *gin.Context) {
	server := getServerFromGin(c)

	err := server.RestoreFileVersion(c.Query("path"), c.Param("versionId"))
	if err != nil {
		response.HandleError(c, err, fileHistoryStatus(err))
		return
	}
	c.Status(http.StatusNoContent)
}
