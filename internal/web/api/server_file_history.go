package api

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/gin-gonic/gin"
)

// registerServerFileHistory adds the file version history routes; the node keeps the versions,
// so every call goes straight to it
func registerServerFileHistory(g *gin.RouterGroup) {
	g.GET("/:serverId/file-history", middleware.RequiresPermission(scopes.ScopeServerFileView), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/file-history", response.CreateOptions("GET"))

	g.GET("/:serverId/file-history/:versionId", middleware.RequiresPermission(scopes.ScopeServerFileView), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/file-history/:versionId", response.CreateOptions("GET"))

	g.POST("/:serverId/file-history/:versionId/restore", middleware.RequiresPermission(scopes.ScopeServerFileEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/file-history/:versionId/restore", response.CreateOptions("POST"))
}
