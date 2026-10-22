package middleware

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/stretchr/testify/assert"
)

func userRole() *models.Role {
	return &models.Role{
		ID:     2,
		Name:   "Usuario",
		Scopes: []string{"login", "server.create", "server.view", "server.console.send", "server.files.edit"},
	}
}

func TestCollectScopes_RoleServerScopesNeedServerAccess(t *testing.T) {
	serverPerms := &models.Permissions{}
	globalPerms := &models.Permissions{}

	got := collectScopes(userRole(), []*models.Permissions{serverPerms, globalPerms}, "someone-else", false)

	assert.False(t, scopes.ContainsScope(got, scopes.ScopeServerSendCommand), "role must not unlock a server the user has no access to")
	assert.False(t, scopes.ContainsScope(got, scopes.ScopeServerFileEdit))
	assert.False(t, scopes.ContainsScope(got, scopes.ScopeServerView))
	assert.True(t, scopes.ContainsScope(got, scopes.ScopeLogin), "panel-wide role scopes still apply")
	assert.True(t, scopes.ContainsScope(got, scopes.ScopeServerCreate))
}

func TestCollectScopes_RoleServerScopesApplyToOwnServer(t *testing.T) {
	serverPerms := &models.Permissions{ID: 10, Scopes: []*scopes.Scope{scopes.ScopeServerView}}
	globalPerms := &models.Permissions{}

	got := collectScopes(userRole(), []*models.Permissions{serverPerms, globalPerms}, "mine", true)

	assert.True(t, scopes.ContainsScope(got, scopes.ScopeServerSendCommand))
	assert.True(t, scopes.ContainsScope(got, scopes.ScopeServerFileEdit))
}

func TestCollectScopes_AdminRoleKeepsAccessToEveryServer(t *testing.T) {
	admin := &models.Role{ID: 1, Name: "Administrador", Scopes: []string{"admin"}}

	got := collectScopes(admin, []*models.Permissions{{}, {}}, "someone-else", false)

	assert.True(t, scopes.ContainsScope(got, scopes.ScopeServerSendCommand))
}

func TestCollectScopes_GlobalPermissionRowStillGrantsServerScopes(t *testing.T) {
	globalPerms := &models.Permissions{ID: 3, Scopes: []*scopes.Scope{scopes.ScopeServerConsole}}

	got := collectScopes(nil, []*models.Permissions{{}, globalPerms}, "any", false)

	assert.True(t, scopes.ContainsScope(got, scopes.ScopeServerConsole))
}
