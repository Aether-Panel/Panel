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
	"gorm.io/gorm"
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

	// a session minted from an OAuth2 client credential never grants more than the
	// client was registered for; the owning user's permissions still apply on top
	if permitted, finished := clientPermits(c, perm, serverID); !permitted {
		actuallyFinished = finished
		return false
	}

	db := GetDatabase(c)
	ps := &services.Permission{DB: db}

	perms, ok := loadUserPermissions(c, ps, user.ID, serverID)
	if !ok {
		return false
	}

	owner, err := ps.IsServerOwner(user.ID, serverID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return false
	}

	// Check role-based permissions first (Global Roles)
	role := resolveUserRole(db, user)

	allScopes := collectScopes(role, perms, serverID, owner)

	allowed := scopes.ContainsScope(allScopes, perm)
	if allowed {
		c.Set("scopes", allScopes)
		actuallyFinished = true
	} else {
		actuallyFinished = true // We finished the check, but it was denied
	}

	return allowed
}

// clientPermits applies the OAuth2 client bounds when the session came from a client.
// finished is false when the client is malformed, so the fail-safe still aborts.
func clientPermits(c *gin.Context, perm *scopes.Scope, serverID string) (permitted bool, finished bool) {
	clientGin, hasClient := c.Get("client")
	if !hasClient {
		return true, false
	}
	client, ok := clientGin.(*models.Client)
	if !ok || client == nil {
		return false, false
	}
	if !clientAllows(client, perm, serverID) {
		return false, true
	}
	return true, false
}

// loadUserPermissions fetches the server permissions plus the global ones when a server is set.
// ok is false when the request has already been aborted.
func loadUserPermissions(c *gin.Context, ps *services.Permission, userID uint, serverID string) (perms []*models.Permissions, ok bool) {
	p, err := ps.GetForUserAndServer(userID, serverID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return nil, false
	}

	perms = append(perms, p)
	if serverID != "" {
		// if we had a server, also grab global scopes
		p, err = ps.GetForUserAndServer(userID, "")
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return nil, false
		}
		perms = append(perms, p)
	}
	return perms, true
}

// resolveUserRole returns the user's global role, or nil when there is none or it cannot be loaded
func resolveUserRole(db *gorm.DB, user *models.User) *models.Role {
	if user.RoleID == nil {
		return nil
	}

	var role *models.Role
	var err error
	// Use preloaded role if available and correct
	if user.Role.ID == *user.RoleID {
		role = &user.Role
	} else {
		rs := &services.Role{DB: db}
		role, err = rs.Get(*user.RoleID)
	}

	if err != nil || role == nil {
		return nil
	}
	// Update the user object with the fetched role so subsequent handlers see it
	if user.Role.ID == 0 {
		user.Role = *role
	}
	return role
}

// clientAllows bounds what an OAuth2 client token may reach. A server-scoped client
// only operates its own server (identity lookups via the login scope aside) and a
// client with declared scopes never exceeds them.
func clientAllows(client *models.Client, perm *scopes.Scope, serverID string) bool {
	if client.ServerID != nil && *client.ServerID != "" && !perm.Is(scopes.ScopeLogin) {
		if !perm.ForServer || serverID != *client.ServerID {
			return false
		}
	}

	if client.Scopes != "" {
		allowed := make([]*scopes.Scope, 0)
		for _, s := range strings.Fields(client.Scopes) {
			allowed = scopes.AddScope(allowed, scopes.GetScope(s))
		}
		if !scopes.ContainsScope(allowed, perm) {
			return false
		}
	}

	return true
}

// collectScopes merges the scopes from the user's role and permission rows. On a server, what
// the user may do is that server's row (and the panel-wide row an administrator set), plus
// OwnerScopes for its owner. A role's server scopes never count there: they would give every
// sub-user the owner's powers, whatever the owner chose for them.
func collectScopes(role *models.Role, perms []*models.Permissions, serverID string, owner bool) []*scopes.Scope {
	allScopes := make([]*scopes.Scope, 0)

	if role != nil {
		for _, s := range role.Scopes {
			scopeObj := scopes.GetScope(s)
			if scopeObj.ForServer && serverID != "" {
				continue
			}
			allScopes = scopes.AddScope(allScopes, scopeObj)
		}
	}

	if owner && serverID != "" {
		for _, s := range scopes.OwnerScopes {
			allScopes = scopes.AddScope(allScopes, s)
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
