package api

import (
	"github.com/SkyPanel/SkyPanel/v3/middleware"
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
	
	RegisterExTransferRoutes(publicRg.Group("/extransfer")) // External federated transfers

	publicRg.GET("/config", panelConfig) // Public — no auth needed for config
}
