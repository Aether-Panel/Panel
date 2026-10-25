package middleware

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/stretchr/testify/assert"
)

func TestClientAllows_AccountClientKeepsUserAccess(t *testing.T) {
	client := &models.Client{}

	assert.True(t, clientAllows(client, scopes.ScopeSelfEdit, ""))
	assert.True(t, clientAllows(client, scopes.ScopeServerConsole, "abc123"))
}

func TestClientAllows_ServerClientOnlyReachesItsServer(t *testing.T) {
	serverID := "abc123"
	client := &models.Client{ServerID: &serverID}

	assert.True(t, clientAllows(client, scopes.ScopeServerConsole, "abc123"))
	assert.False(t, clientAllows(client, scopes.ScopeServerConsole, "other99"))
}

func TestClientAllows_ServerClientCannotUseAccountScopes(t *testing.T) {
	serverID := "abc123"
	client := &models.Client{ServerID: &serverID}

	assert.False(t, clientAllows(client, scopes.ScopeSelfEdit, ""))
	assert.False(t, clientAllows(client, scopes.ScopeSelfClients, ""))
	assert.False(t, clientAllows(client, scopes.ScopeNodesDeploy, ""))
	// identity lookup stays available so integrations can see who they act as
	assert.True(t, clientAllows(client, scopes.ScopeLogin, ""))
}

func TestClientAllows_DeclaredScopesNarrowTheToken(t *testing.T) {
	client := &models.Client{Scopes: "server.console server.view"}

	assert.True(t, clientAllows(client, scopes.ScopeServerConsole, "abc123"))
	assert.False(t, clientAllows(client, scopes.ScopeServerStop, "abc123"))
	assert.False(t, clientAllows(client, scopes.ScopeSelfEdit, ""))
}
