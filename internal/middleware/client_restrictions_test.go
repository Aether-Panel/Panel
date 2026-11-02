package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/gin-gonic/gin"
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

func TestBoundByClient_KeepsUserScopesWithoutClient(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	granted := []*scopes.Scope{scopes.ScopeServerView, scopes.ScopeServerEditData}

	assert.Equal(t, granted, boundByClient(c, granted, "abc123"))
}

func TestBoundByClient_DropsScopesTheClientWasNotGiven(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("client", &models.Client{Scopes: "server.view"})
	granted := []*scopes.Scope{scopes.ScopeServerView, scopes.ScopeServerEditData}

	bounded := boundByClient(c, granted, "abc123")

	assert.True(t, scopes.ContainsScope(bounded, scopes.ScopeServerView))
	assert.False(t, scopes.ContainsScope(bounded, scopes.ScopeServerEditData))
}
