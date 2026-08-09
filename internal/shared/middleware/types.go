package middleware

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/scopes"
	"github.com/gin-gonic/gin"
)

type RouteDeps struct {
	RequiresPermission    func(*scopes.Scope) gin.HandlerFunc
	RequiresAnyPermission func(...*scopes.Scope) gin.HandlerFunc
	ResolveServerPanel    gin.HandlerFunc
	ResolveServerNode     gin.HandlerFunc
	HasTransaction        gin.HandlerFunc
}
