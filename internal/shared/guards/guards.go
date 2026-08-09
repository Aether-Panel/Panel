package guards

import (
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/feature/permission"
	"github.com/SkyPanel/SkyPanel/v3/internal/feature/role"
	"github.com/SkyPanel/SkyPanel/v3/internal/feature/server"
	"github.com/SkyPanel/SkyPanel/v3/internal/feature/user"
	"github.com/SkyPanel/SkyPanel/v3/internal/runtime"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/scopes"
	"github.com/gin-gonic/gin"
)

func RequiresPermission(perm *scopes.Scope) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !checkPermission(c, perm) {
			if !c.IsAborted() {
				c.AbortWithStatus(http.StatusForbidden)
			}
		}
	}
}

func RequiresAnyPermission(perms ...*scopes.Scope) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, v := range perms {
			if checkPermission(c, v) {
				return
			}
		}
		if !c.IsAborted() {
			c.AbortWithStatus(http.StatusForbidden)
		}
	}
}

func checkPermission(c *gin.Context, perm *scopes.Scope) bool {
	actuallyFinished := false
	defer func() {
		if !actuallyFinished && !c.IsAborted() {
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}()

	middleware.NeedsDatabase(c)
	if c.IsAborted() {
		return false
	}

	userGin, exists := c.Get("user")
	if !exists {
		return false
	}
	u, ok := userGin.(*user.User)
	if !ok {
		panic("user not defined")
	}

	serverID := c.Param("serverId")
	if perm.ForServer && serverID == "" {
		return false
	}

	db := middleware.GetDatabase(c)
	ps := &permission.PermissionRepo{DB: db}

	var perms []*permission.Permissions

	p, err := ps.GetForUserAndServer(u.ID, serverID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return false
	}

	perms = append(perms, p)
	if serverID != "" {
		p, err = ps.GetForUserAndServer(u.ID, "")
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return false
		}
		perms = append(perms, p)
	}

	allScopes := make([]*scopes.Scope, 0)

	if u.RoleID != nil {
		var r *role.Role
		if u.Role.ID == *u.RoleID {
			r = &u.Role
		} else {
			rs := &role.RoleRepo{DB: db}
			r, err = rs.Get(*u.RoleID)
		}

		if err == nil && r != nil {
			if u.Role.ID == 0 {
				u.Role = *r
			}
			for _, s := range r.Scopes {
				scopeObj := scopes.GetScope(s)
				allScopes = scopes.AddScope(allScopes, scopeObj)
			}
		}
	}

	for _, p := range perms {
		for _, s := range p.Scopes {
			allScopes = scopes.AddScope(allScopes, s)
		}
	}

	allowed := scopes.ContainsScope(allScopes, perm)
	if allowed {
		c.Set("scopes", allScopes)
		actuallyFinished = true
	} else {
		actuallyFinished = true
	}

	return allowed
}

func ResolveServerPanel(c *gin.Context) {
	serverID := c.Param("serverId")
	if serverID == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	db := middleware.GetDatabase(c)
	ss := &server.ServerRepo{DB: db}
	srv, err := ss.Get(serverID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	} else if srv == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Set("server", srv)
}

func ResolveServerNode(c *gin.Context) {
	serverID := c.Param("serverId")
	if serverID == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	srv := runtime.GetFromCache(serverID)
	if srv == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Set("program", srv)
}
