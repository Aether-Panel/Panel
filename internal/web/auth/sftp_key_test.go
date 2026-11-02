package auth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type sftpKeyHarness struct {
	db     *gorm.DB
	router *gin.Engine
	user   *models.User
	node   *models.Node
	other  *models.Node
	key    string
}

// newSFTPKeyHarness is a panel with two remote nodes, a server on the first one and a user who
// may sign in, holds SFTP on that server and has one SSH key
func newSFTPKeyHarness(t *testing.T) *sftpKeyHarness {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sftp_key.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permissions{}, &models.Node{}, &models.Server{}, &models.SSHKey{}))

	h := &sftpKeyHarness{db: db}
	h.node = &models.Node{Name: "remote", PublicHost: "10.0.0.5", PrivateHost: "10.0.0.5", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "node-secret"}
	require.NoError(t, db.Create(h.node).Error)
	h.other = &models.Node{Name: "other", PublicHost: "10.0.0.6", PrivateHost: "10.0.0.6", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "other-secret"}
	require.NoError(t, db.Create(h.other).Error)
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: "srv1", Name: "srv1", Type: "generic", NodeID: h.node.ID}).Error)

	h.user = &models.User{Username: "alice", Email: "alice@example.com", HashedPassword: "x"}
	require.NoError(t, db.Create(h.user).Error)
	ps := &services.Permission{DB: db}
	server := "srv1"
	require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &h.user.ID, Scopes: []*scopes.Scope{scopes.ScopeLogin}}))
	require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &h.user.ID, ServerIdentifier: &server, Scopes: []*scopes.Scope{scopes.ScopeServerSftp}}))

	h.key = newSFTPTestKey(t)
	_, err = (&services.SSHKey{DB: db}).Add(h.user.ID, "laptop", h.key)
	require.NoError(t, err)

	t.Cleanup(func() { services.LoginAttempts.Reset(sftpKeyAttemptKey(h.user.ID)) })
	services.LoginAttempts.Reset(sftpKeyAttemptKey(h.user.ID))

	h.router = gin.New()
	h.router.POST("/auth/sftp-key", func(c *gin.Context) { c.Set("db", db) }, SFTPKeyPost)
	return h
}

func newSFTPTestKey(t *testing.T) string {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return string(ssh.MarshalAuthorizedKey(key))
}

func (h *sftpKeyHarness) check(t *testing.T, secret, username, key string) (int, map[string]interface{}) {
	t.Helper()
	body, _ := json.Marshal(SFTPKeyRequest{Username: username, PublicKey: key})
	req := httptest.NewRequest(http.MethodPost, "/auth/sftp-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	answer := map[string]interface{}{}
	_ = json.Unmarshal(w.Body.Bytes(), &answer)
	return w.Code, answer
}

func TestSFTPKey_GrantsTheServerScopeAndNoSession(t *testing.T) {
	h := newSFTPKeyHarness(t)

	status, answer := h.check(t, "node-secret", "alice@example.com#srv1", h.key)

	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "srv1:"+scopes.ScopeServerSftp.String(), answer["scope"])
	assert.NotContains(t, answer, "access_token", "a key check must never hand out a panel session")
}

func TestSFTPKey_OnlyNodesMayAsk(t *testing.T) {
	h := newSFTPKeyHarness(t)

	for _, secret := range []string{"", "not-a-node"} {
		status, answer := h.check(t, secret, "alice@example.com#srv1", h.key)
		assert.Equal(t, http.StatusUnauthorized, status, secret)
		assert.Equal(t, "invalid_client", answer["error"])
	}
}

func TestSFTPKey_Refusals(t *testing.T) {
	h := newSFTPKeyHarness(t)
	ps := &services.Permission{DB: h.db}

	cases := map[string]struct{ secret, username, key string }{
		"another node":        {"other-secret", "alice@example.com#srv1", h.key},
		"a key not hers":      {"node-secret", "alice@example.com#srv1", newSFTPTestKey(t)},
		"an unknown account":  {"node-secret", "bob@example.com#srv1", h.key},
		"an unknown server":   {"node-secret", "alice@example.com#nope", h.key},
		"a bad username":      {"node-secret", "alice@example.com", h.key},
		"not a key":           {"node-secret", "alice@example.com#srv1", "ssh-ed25519 garbage"},
		"a private key block": {"node-secret", "alice@example.com#srv1", "-----BEGIN OPENSSH PRIVATE KEY-----"},
	}
	for name, c := range cases {
		status, answer := h.check(t, c.secret, c.username, c.key)
		assert.Equal(t, http.StatusBadRequest, status, name)
		assert.Equal(t, "no access", answer["error_description"], name)
		assert.NotContains(t, answer, "scope", name)
	}

	// SFTP on the server is needed...
	server := "srv1"
	require.NoError(t, h.db.Where("user_id = ? AND server_identifier = ?", h.user.ID, server).Delete(&models.Permissions{}).Error)
	require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &h.user.ID, ServerIdentifier: &server, Scopes: []*scopes.Scope{scopes.ScopeServerView}}))
	status, _ := h.check(t, "node-secret", "alice@example.com#srv1", h.key)
	assert.Equal(t, http.StatusBadRequest, status, "without server.sftp")

	// ...and so is the login scope, as for a password
	require.NoError(t, h.db.Where("user_id = ?", h.user.ID).Delete(&models.Permissions{}).Error)
	require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &h.user.ID, ServerIdentifier: &server, Scopes: []*scopes.Scope{scopes.ScopeServerSftp}}))
	status, _ = h.check(t, "node-secret", "alice@example.com#srv1", h.key)
	assert.Equal(t, http.StatusBadRequest, status, "without the login scope")
}

func TestSFTPKey_LimitsFailedAttempts(t *testing.T) {
	h := newSFTPKeyHarness(t)

	for i := 0; i < 10; i++ {
		status, _ := h.check(t, "node-secret", "alice@example.com#srv1", newSFTPTestKey(t))
		require.Equal(t, http.StatusBadRequest, status)
	}

	status, _ := h.check(t, "node-secret", "alice@example.com#srv1", h.key)
	assert.Equal(t, http.StatusTooManyRequests, status, "once the budget is spent even the right key waits")
	assert.True(t, services.LoginAttempts.Allowed(fmt.Sprintf("password:user:%d", h.user.ID)), "key misses do not lock the password login")
}
