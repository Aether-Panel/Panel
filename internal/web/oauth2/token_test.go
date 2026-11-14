package oauth2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const tokenTestSecret = "client-secret"

type tokenHarness struct {
	db     *gorm.DB
	router *gin.Engine
	user   *models.User
}

// newTokenHarness is a user with the given panel-wide scopes who may view and use the console
// of server srv1
func newTokenHarness(t *testing.T, global ...*scopes.Scope) *tokenHarness {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "token.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permissions{}, &models.Client{}, &models.Session{}, &models.Node{}, &models.Server{}))

	require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: "srv1", Name: "srv1", Type: "generic"}).Error)
	user := &models.User{Username: "alice", Email: "alice@example.com", HashedPassword: "x"}
	require.NoError(t, db.Create(user).Error)

	ps := &services.Permission{DB: db}
	server := "srv1"
	require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &user.ID, Scopes: global}))
	require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &user.ID, ServerIdentifier: &server, Scopes: []*scopes.Scope{scopes.ScopeServerView, scopes.ScopeServerConsole}}))

	r := gin.New()
	r.POST("/oauth2/token", func(c *gin.Context) { c.Set("db", db) }, handleTokenRequest)
	return &tokenHarness{db: db, router: r, user: user}
}

func (h *tokenHarness) addClient(t *testing.T, id string, serverID *string, limits string) {
	t.Helper()
	client := &models.Client{ClientID: id, UserID: h.user.ID, ServerID: serverID, Scopes: limits}
	require.NoError(t, client.SetClientSecret(tokenTestSecret))
	require.NoError(t, h.db.Omit(clause.Associations).Create(client).Error)
}

func (h *tokenHarness) token(t *testing.T, clientID string) (int, map[string]interface{}) {
	t.Helper()
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {clientID}, "client_secret": {tokenTestSecret}}
	req := httptest.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	answer := map[string]interface{}{}
	_ = json.Unmarshal(w.Body.Bytes(), &answer)
	return w.Code, answer
}

func (h *tokenHarness) sessions(t *testing.T) int64 {
	var n int64
	require.NoError(t, h.db.Model(&models.Session{}).Count(&n).Error)
	return n
}

// A client limited to one scope on one server reports that scope, not its owner's panel-wide ones
func TestClientCredentials_ServerClientReportsItsOwnScopes(t *testing.T) {
	h := newTokenHarness(t, scopes.ScopeLogin, scopes.ScopeAdmin)
	server := "srv1"
	h.addClient(t, "limited", &server, "server.view")

	status, answer := h.token(t, "limited")

	require.Equal(t, http.StatusOK, status, answer)
	assert.Equal(t, "server.view", answer["scope"])
	assert.NotEmpty(t, answer["access_token"])
}

// Without declared scopes, a server client reports what its owner may do on that server, and login
func TestClientCredentials_ServerClientWithoutLimits(t *testing.T) {
	h := newTokenHarness(t, scopes.ScopeLogin)
	server := "srv1"
	h.addClient(t, "server-wide", &server, "")

	status, answer := h.token(t, "server-wide")

	require.Equal(t, http.StatusOK, status, answer)
	assert.ElementsMatch(t, []string{"login", "server.view", "server.console"}, strings.Fields(answer["scope"].(string)))
}

// A personal client still reports its owner's panel-wide scopes
func TestClientCredentials_PersonalClientUnchanged(t *testing.T) {
	h := newTokenHarness(t, scopes.ScopeLogin, scopes.ScopeAdmin)
	h.addClient(t, "personal", nil, "")

	status, answer := h.token(t, "personal")

	require.Equal(t, http.StatusOK, status, answer)
	assert.Equal(t, "login admin", answer["scope"])
}

// The owner needs the panel-wide login scope, for a server client as for a personal one, and
// a refused request opens no session
func TestClientCredentials_NeedsTheLoginScope(t *testing.T) {
	h := newTokenHarness(t) // admin would imply login
	server := "srv1"
	h.addClient(t, "personal", nil, "")
	h.addClient(t, "limited", &server, "server.view")

	for _, id := range []string{"personal", "limited"} {
		status, _ := h.token(t, id)
		assert.Equal(t, http.StatusForbidden, status, id)
	}
	assert.Zero(t, h.sessions(t), "no session for a refused client")
}
