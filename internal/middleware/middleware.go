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
	// fail-safe in the event something pukes, we don't end up accidentally giving rights to something they should not
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

	// we now have a user and they are allowed to access something, let's confirm they have server access
	serverID := c.Param("serverId")
	if perm.ForServer && serverID == "" {
		return false
	}

	db := GetDatabase(c)
	ps := &services.Permission{DB: db}

	var perms []*models.Permissions

	p, err := ps.GetForUserAndServer(user.ID, serverID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return false
	}

	perms = append(perms, p)
	// a stored permission row for this server is what grants the user access to it
	serverMember := serverID != "" && p.ID != 0
	if serverID != "" {
		// if we had a server, also grab global scopes
		p, err = ps.GetForUserAndServer(user.ID, "")
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return false
		}
		perms = append(perms, p)
	}

	// Check role-based permissions first (Global Roles)
	var role *models.Role
	if user.RoleID != nil {
		// Use preloaded role if available and correct
		if user.Role.ID == *user.RoleID {
			role = &user.Role
		} else {
			rs := &services.Role{DB: db}
			role, err = rs.Get(*user.RoleID)
		}

		if err == nil && role != nil {
			// Update the user object with the fetched role so subsequent handlers see it
			if user.Role.ID == 0 {
				user.Role = *role
			}
		} else {
			role = nil
		}
	}

	allScopes := collectScopes(role, perms, serverID, serverMember)

	allowed := scopes.ContainsScope(allScopes, perm)
	if allowed {
		c.Set("scopes", allScopes)
		actuallyFinished = true
	} else {
		actuallyFinished = true // We finished the check, but it was denied
	}

	return allowed
}

// collectScopes merges the scopes from the user's role and permission rows. A role applies
// panel-wide, so its server scopes only count on servers the user was given access to;
// otherwise every user with the role could operate every server on the panel.
func collectScopes(role *models.Role, perms []*models.Permissions, serverID string, serverMember bool) []*scopes.Scope {
	allScopes := make([]*scopes.Scope, 0)

	if role != nil {
		for _, s := range role.Scopes {
			scopeObj := scopes.GetScope(s)
			if scopeObj.ForServer && serverID != "" && !serverMember {
				continue
			}
			allScopes = scopes.AddScope(allScopes, scopeObj)
		}
	}

	for _, p := range perms {
		for _, s := range p.Scopes {
			allScopes = scopes.AddScope(allScopes, s)
		}
	}

	return allScopes
}

func GetToken(c *gin.Context) string {
	// use header first, because we set that a lot
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

	cookie, err := c.Cookie("skypanel_auth")
	if errors.Is(err, http.ErrNoCookie) {
		return ""
	}
	return strings.TrimSpace(cookie)
}

func ResolveServerPanel(c *gin.Context) {
	serverID := c.Param("serverId")
	if serverID == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	db := GetDatabase(c)
	ss := &services.Server{DB: db}
	server, err := ss.Get(serverID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	} else if server == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Set("server", server)
}

func ResolveServerNode(c *gin.Context) {
	serverID := c.Param("serverId")
	if serverID == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	server := servers.GetFromCache(serverID)
	if server == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Set("program", server)
}
