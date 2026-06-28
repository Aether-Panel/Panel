package middleware

import (
	"errors"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/gin-gonic/gin"
)

func ResponseAndRecover(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			if _, ok := err.(error); !ok {
				err = errors.New(utils.ToString(err))
			}
			response.HandleError(c, err.(error), http.StatusInternalServerError)

			logging.Error.Printf("Error handling route\n%+v\n%s", err, debug.Stack())
			c.Abort()
		}
	}()

	c.Next()
}

func Recover(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			logging.Error.Printf("Error handling route\n%+v\n%s", err, debug.Stack())
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}()

	c.Next()
}

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
	//fail-safe in the event something pukes, we don't end up accidentally giving rights to something they should not
	actuallyFinished := false
	defer func() {
		if !actuallyFinished && !c.IsAborted() {
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}()

	NeedsDatabase(c)
	if c.IsAborted() {
		return false
	}

	userGin, exists := c.Get("user")
	if !exists {
		return false
	}
	user, ok := userGin.(*models.User)
	if !ok {
		panic("user not defined")
	}

	//we now have a user and they are allowed to access something, let's confirm they have server access
	serverId := c.Param("serverId")
	if perm.ForServer && serverId == "" {
		return false
	}

	db := GetDatabase(c)
	ps := &services.Permission{DB: db}

	var perms []*models.Permissions

	p, err := ps.GetForUserAndServer(user.ID, serverId)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return false
	}

	perms = append(perms, p)
	if serverId != "" {
		//if we had a server, also grab global scopes
		p, err = ps.GetForUserAndServer(user.ID, "")
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return false
		}
		perms = append(perms, p)
	}

	allScopes := make([]*scopes.Scope, 0)

	// Check role-based permissions first (Global Roles)
	if user.RoleId != nil {
		// Use preloaded role if available and correct
		var role *models.Role
		if user.Role.ID == *user.RoleId {
			role = &user.Role
		} else {
			rs := &services.Role{DB: db}
			role, err = rs.Get(*user.RoleId)
		}

		if err == nil && role != nil {
			// Update the user object with the fetched role so subsequent handlers see it
			if user.Role.ID == 0 {
				user.Role = *role
			}
			for _, s := range role.Scopes {
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
		actuallyFinished = true // We finished the check, but it was denied
	}

	return allowed
}

func GetToken(c *gin.Context) string {
	//use header first, because we set that a lot
	authHeader := c.Request.Header.Get("Authorization")

	if authHeader != "" {
		authHeader = strings.TrimSpace(authHeader)
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 {
			return ""
		}

		if parts[0] != "Bearer" || parts[1] == "" {
			return ""
		}

		return parts[1]
	}

	cookie, err := c.Cookie("puffer_auth")
	if errors.Is(err, http.ErrNoCookie) {
		return ""
	}
	return strings.TrimSpace(cookie)
}

func ResolveServerPanel(c *gin.Context) {
	serverId := c.Param("serverId")
	if serverId == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	db := GetDatabase(c)
	ss := &services.Server{DB: db}
	server, err := ss.Get(serverId)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	} else if server == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Set("server", server)
}

func ResolveServerNode(c *gin.Context) {
	serverId := c.Param("serverId")
	if serverId == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	server := servers.GetFromCache(serverId)
	if server == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Set("program", server)
}
