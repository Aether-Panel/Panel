package api

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/middleware"
	"github.com/gin-gonic/gin"
)

const MaxPageSize = 100
const DefaultPageSize = 20

func RegisterRoutes(rg *gin.RouterGroup) {

	rg.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	})

	rg.Use(middleware.ResponseAndRecover)
	rg.Use(middleware.NeedsDatabase)

	publicRg := rg.Group("")

	rg.Use(middleware.AuthMiddleware)
	rg.Use(middleware.AddVersionHeader)

	registerNodes(rg.Group("/nodes"))
	registerServers(rg.Group("/servers"))
	registerDatabases(rg.Group("/servers"))
	registerUsers(rg.Group("/users"))
	registerTemplates(rg.Group("/templates"))
	registerSelf(rg.Group("/self"))
	registerSettings(rg.Group("/settings"))
	registerUserSettings(rg.Group("/userSettings"))
	registerUptime(rg.Group("/uptime"))
	registerRoles(rg.Group("/roles"))
	registerDatabaseHosts(rg.Group("/databasehosts"))
	registerAPIKeys(rg.Group("/settings/apikeys"))
	registerProducts(rg.Group("/provision/products"))
	RegisterAIRoutes(rg.Group("/ai"))

	RegisterExTransferRoutes(publicRg.Group("/extransfer")) // External federated transfers

	v1 := publicRg.Group("/v1")
	v1.Use(middleware.APIKeyAuthMiddleware)
	registerProvision(v1)

	publicRg.GET("/config", panelConfig) // Public — no auth needed for config
}
