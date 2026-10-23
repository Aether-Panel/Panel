package api

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/gin-gonic/gin"
)

// registerServerModpacks adds the modpack routes. Looking packs up only needs to see the
// files; installing replaces mods and the loader, so it needs the reinstall permission.
func registerServerModpacks(g *gin.RouterGroup) {
	g.GET("/:serverId/modpacks", middleware.RequiresPermission(scopes.ScopeServerFileView), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/modpacks", response.CreateOptions("GET"))
	g.GET("/:serverId/modpacks/search", middleware.RequiresPermission(scopes.ScopeServerFileView), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/modpacks/search", response.CreateOptions("GET"))
	g.GET("/:serverId/modpacks/versions", middleware.RequiresPermission(scopes.ScopeServerFileView), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/modpacks/versions", response.CreateOptions("GET"))
	g.POST("/:serverId/modpacks/install", middleware.RequiresPermission(scopes.ScopeServerInstall), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/modpacks/install", response.CreateOptions("POST"))
}
