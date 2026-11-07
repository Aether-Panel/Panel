package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func (h *sftpKeyHarness) recheck(t *testing.T, secret, username string) (int, map[string]interface{}) {
	t.Helper()
	return h.recheckKey(t, secret, username, "")
}

// recheckKey checks a connection that signed in with the SSH key of that fingerprint
func (h *sftpKeyHarness) recheckKey(t *testing.T, secret, username, fingerprint string) (int, map[string]interface{}) {
	t.Helper()
	body, _ := json.Marshal(SFTPCheckRequest{Username: username, KeyFingerprint: fingerprint})
	req := httptest.NewRequest(http.MethodPost, "/auth/sftp-check", bytes.NewReader(body))
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

func newSFTPCheckHarness(t *testing.T) *sftpKeyHarness {
	h := newSFTPKeyHarness(t)
	h.router.POST("/auth/sftp-check", func(c *gin.Context) { c.Set("db", h.db) }, SFTPCheckPost)
	return h
}

func TestSFTPCheck_StillAllowed(t *testing.T) {
	h := newSFTPCheckHarness(t)

	status, answer := h.recheck(t, "node-secret", "alice@example.com#srv1")

	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "srv1:"+scopes.ScopeServerSftp.String(), answer["scope"])
	assert.NotContains(t, answer, "access_token")
}

func TestSFTPCheck_OnlyNodesMayAsk(t *testing.T) {
	h := newSFTPCheckHarness(t)

	for _, secret := range []string{"", "not-a-node"} {
		status, answer := h.recheck(t, secret, "alice@example.com#srv1")
		assert.Equal(t, http.StatusUnauthorized, status, secret)
		assert.Equal(t, "invalid_client", answer["error"])
	}
}

// every lost access answers the one 400 the node closes the connection on
func TestSFTPCheck_LostAccess(t *testing.T) {
	server := "srv1"
	cases := map[string]func(h *sftpKeyHarness) (secret, username string){
		"another node":   func(*sftpKeyHarness) (string, string) { return "other-secret", "alice@example.com#srv1" },
		"a bad username": func(*sftpKeyHarness) (string, string) { return "node-secret", "alice@example.com" },
		"the server is gone": func(h *sftpKeyHarness) (string, string) {
			require.NoError(t, h.db.Delete(&models.Server{Identifier: server}).Error)
			return "node-secret", "alice@example.com#srv1"
		},
		"the user is deleted": func(h *sftpKeyHarness) (string, string) {
			require.NoError(t, h.db.Delete(&models.User{}, h.user.ID).Error)
			return "node-secret", "alice@example.com#srv1"
		},
		"the SFTP scope is taken away": func(h *sftpKeyHarness) (string, string) {
			require.NoError(t, h.db.Where("user_id = ? AND server_identifier = ?", h.user.ID, server).Delete(&models.Permissions{}).Error)
			ps := &services.Permission{DB: h.db}
			require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &h.user.ID, ServerIdentifier: &server, Scopes: []*scopes.Scope{scopes.ScopeServerView}}))
			return "node-secret", "alice@example.com#srv1"
		},
		"the login scope is taken away": func(h *sftpKeyHarness) (string, string) {
			require.NoError(t, h.db.Where("user_id = ? AND server_identifier IS NULL", h.user.ID).Delete(&models.Permissions{}).Error)
			return "node-secret", "alice@example.com#srv1"
		},
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			h := newSFTPCheckHarness(t)
			secret, username := prepare(h)

			status, answer := h.recheck(t, secret, username)

			assert.Equal(t, http.StatusBadRequest, status)
			assert.Equal(t, "no access", answer["error_description"])
			assert.NotContains(t, answer, "scope")
		})
	}
}

// a database error is not a refusal: the node keeps the connection on anything but a 400
func TestSFTPCheck_ADatabaseErrorIsNotANo(t *testing.T) {
	h := newSFTPCheckHarness(t)
	node := h.node
	sqlDB, err := h.db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = checkSFTPAccess(h.db, node, "alice@example.com#srv1", "")

	require.Error(t, err)
	assert.NotErrorIs(t, err, errSFTPNoAccess)
}

// the check spends nothing of the attempt budgets: the node already checked the password or key
func TestSFTPCheck_CountsNoAttempts(t *testing.T) {
	h := newSFTPCheckHarness(t)
	server := "srv1"
	require.NoError(t, h.db.Where("user_id = ? AND server_identifier = ?", h.user.ID, server).Delete(&models.Permissions{}).Error)

	for i := 0; i < 20; i++ {
		status, _ := h.recheck(t, "node-secret", "alice@example.com#srv1")
		require.Equal(t, http.StatusBadRequest, status)
	}
	assert.True(t, services.LoginAttempts.Allowed(sftpKeyAttemptKey(h.user.ID)))
}

// A connection opened with an SSH key ends when the key is removed from the account; one
// opened with the password does not depend on the keys
func TestSFTPCheck_TheKeyMustStillBeThere(t *testing.T) {
	h := newSFTPCheckHarness(t)
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(h.key))
	require.NoError(t, err)
	fingerprint := ssh.FingerprintSHA256(key)

	status, _ := h.recheckKey(t, "node-secret", "alice@example.com#srv1", fingerprint)
	assert.Equal(t, http.StatusOK, status, "the key is still on the account")

	status, answer := h.recheckKey(t, "node-secret", "alice@example.com#srv1", ssh.FingerprintSHA256(mustParseKey(t, newSFTPTestKey(t))))
	assert.Equal(t, http.StatusBadRequest, status, "a key that is not hers")
	assert.Equal(t, "no access", answer["error_description"])

	require.NoError(t, h.db.Where("user_id = ?", h.user.ID).Delete(&models.SSHKey{}).Error)

	status, answer = h.recheckKey(t, "node-secret", "alice@example.com#srv1", fingerprint)
	assert.Equal(t, http.StatusBadRequest, status, "the key was removed")
	assert.Equal(t, "no access", answer["error_description"])

	status, _ = h.recheck(t, "node-secret", "alice@example.com#srv1")
	assert.Equal(t, http.StatusOK, status, "a password connection does not depend on the keys")
}

func mustParseKey(t *testing.T, line string) ssh.PublicKey {
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	require.NoError(t, err)
	return key
}
