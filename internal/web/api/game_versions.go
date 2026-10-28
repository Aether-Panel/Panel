package api

import (
	"errors"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/gin-gonic/gin"
)

func registerGameVersions(g *gin.RouterGroup) {
	g.Handle("GET", "/:platform", middleware.RequiresPermission(scopes.ScopeLogin), getGameVersionList)
	g.Handle("OPTIONS", "/:platform", response.CreateOptions("GET"))
}

// @Summary Game versions of a platform
// @Description The stable releases of vanilla Minecraft, Paper, Folia, Velocity or Waterfall, newest first. The create form offers them for a template's version variable, so nobody has to type one that does not exist.
// @Success 200 {object} []string
// @Param platform path string true "vanilla, paper, folia, velocity or waterfall"
// @Tags Templates
// @Router /api/game-versions/{platform} [get]
// @Security OAuth2Application[login]
func getGameVersionList(c *gin.Context) {
	versions, err := servers.ListVersions(c.Param("platform"))
	if errors.Is(err, servers.ErrUnknownPlatform) {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	// the lists come from Mojang and PaperMC; when they cannot be reached say so
	if response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	c.JSON(http.StatusOK, versions)
}
