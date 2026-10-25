package daemon

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/gin-gonic/gin"
)

// registerModpackRoutes adds the modpack search and install routes
func registerModpackRoutes(l *gin.RouterGroup) {
	l.GET("/:serverId/modpacks", middleware.ResolveServerNode, getModpackStatus)
	l.OPTIONS("/:serverId/modpacks", response.CreateOptions("GET"))
	l.GET("/:serverId/modpacks/search", middleware.ResolveServerNode, searchModpacks)
	l.OPTIONS("/:serverId/modpacks/search", response.CreateOptions("GET"))
	l.GET("/:serverId/modpacks/versions", middleware.ResolveServerNode, getModpackVersions)
	l.OPTIONS("/:serverId/modpacks/versions", response.CreateOptions("GET"))
	l.POST("/:serverId/modpacks/install", middleware.ResolveServerNode, installModpack)
	l.OPTIONS("/:serverId/modpacks/install", response.CreateOptions("POST"))
}

// modpackErrorStatus picks the answer for a modpack error: the owner's mistakes are 400, a
// busy server 409, and refusals from Modrinth or CurseForge 422 (the panel shows the text of
// 4xx answers, not of 5xx)
func modpackErrorStatus(err error) int {
	var upstream *servers.UpstreamError
	switch {
	case errors.Is(err, servers.ErrModpackBadRequest), errors.Is(err, servers.ErrCurseForgeKeyMissing), errors.Is(err, servers.ErrModpackUnsupported):
		return http.StatusBadRequest
	case errors.Is(err, servers.ErrModpackInstalling), errors.Is(err, servers.ErrModpackServerBusy), errors.Is(err, servers.ErrVersionChanging):
		return http.StatusConflict
	case errors.As(err, &upstream):
		return http.StatusUnprocessableEntity
	}
	return http.StatusInternalServerError
}

// @Summary Get modpack status
// @Description The installed modpack, the install that is running (step and file counts) and whether CurseForge can be used
// @Success 200 {object} servers.ModpackStatus
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/modpacks [get]
// @Security OAuth2Application[server.files.view]
func getModpackStatus(c *gin.Context) {
	server := getServerFromGin(c)
	c.JSON(http.StatusOK, server.ModpackStatus())
}

// @Summary Search modpacks
// @Description Searches Modrinth or CurseForge for Minecraft modpacks the panel can install; an empty query lists the most popular ones
// @Success 200 {object} servers.ModpackSearchResult
// @Param id path string true "Server ID"
// @Param source query string true "modrinth or curseforge"
// @Param q query string false "Search text"
// @Param offset query int false "Results to skip"
// @Tags Daemon Server
// @Router /api/servers/{id}/modpacks/search [get]
// @Security OAuth2Application[server.files.view]
func searchModpacks(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	query := c.Query("q")
	if len(query) > 100 {
		query = query[:100]
	}
	result, err := servers.SearchModpacks(strings.ToLower(c.Query("source")), query, offset)
	if response.HandleError(c, err, modpackErrorStatus(err)) {
		return
	}
	c.JSON(http.StatusOK, result)
}

// @Summary List modpack versions
// @Description The versions of a modpack the panel can install, newest first
// @Success 200 {array} servers.ModpackVersion
// @Param id path string true "Server ID"
// @Param source query string true "modrinth or curseforge"
// @Param project query string true "Project id"
// @Tags Daemon Server
// @Router /api/servers/{id}/modpacks/versions [get]
// @Security OAuth2Application[server.files.view]
func getModpackVersions(c *gin.Context) {
	versions, err := servers.ModpackVersions(strings.ToLower(c.Query("source")), strings.TrimSpace(c.Query("project")))
	if response.HandleError(c, err, modpackErrorStatus(err)) {
		return
	}
	c.JSON(http.StatusOK, versions)
}

// @Summary Install a modpack
// @Description Stops the server, saves a restore point, replaces mods and configuration with the pack's, installs its loader, sets the Java and game version and starts the server again if it was running. Runs in the background; the console and the status show each step.
// @Success 202 {object} nil
// @Param id path string true "Server ID"
// @Param body body servers.ModpackInstallRequest true "Pack to install"
// @Tags Daemon Server
// @Router /api/servers/{id}/modpacks/install [post]
// @Security OAuth2Application[server.install]
func installModpack(c *gin.Context) {
	server := getServerFromGin(c)

	var body servers.ModpackInstallRequest
	if response.HandleError(c, c.ShouldBindJSON(&body), http.StatusBadRequest) {
		return
	}
	if err := server.InstallModpack(body); err != nil {
		response.HandleError(c, err, modpackErrorStatus(err))
		return
	}
	c.Status(http.StatusAccepted)
}
