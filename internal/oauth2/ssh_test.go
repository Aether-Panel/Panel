package oauth2

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestSFTPKeyURL(t *testing.T) {
	for _, c := range []struct{ configured, expected string }{
		// the panel's address, or its full token URL: the key route on the same panel
		{"http://localhost:8080", "http://localhost:8080/auth/sftp-key"},
		{"https://panel.example.com/", "https://panel.example.com/auth/sftp-key"},
		{"http://localhost:8080/oauth2/token", "http://localhost:8080/auth/sftp-key"},
		// a panel served under a sub path keeps it
		{"https://example.com/panel/oauth2/token", "https://example.com/panel/auth/sftp-key"},
		// the query belongs to the token request
		{"http://10.0.0.5:8080?x=1", "http://10.0.0.5:8080/auth/sftp-key"},
	} {
		got, err := sftpKeyURL(c.configured)
		assert.NoError(t, err, c.configured)
		assert.Equal(t, c.expected, got, c.configured)
	}

	for _, bad := range []string{"", "localhost", "ftp://panel.example.com"} {
		_, err := sftpKeyURL(bad)
		assert.Error(t, err, bad)
	}
}

func newTestSSHKey(t *testing.T) ssh.PublicKey {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return key
}

// pointDaemonAt makes the daemon talk to the given panel with the given node secret
func pointDaemonAt(t *testing.T, panelURL, secret string) {
	previousURL, previousSecret := config.AuthURL.Value(), config.ClientSecret.Value()
	_ = config.AuthURL.Set(panelURL, false)
	_ = config.ClientSecret.Set(secret, false)
	t.Cleanup(func() {
		_ = config.AuthURL.Set(previousURL, false)
		_ = config.ClientSecret.Set(previousSecret, false)
	})
}

// A remote node checks a key with the panel, signed in with its own secret, and lets the
// client into the server the panel granted
func TestValidateKey_AsksThePanel(t *testing.T) {
	key := newTestSSHKey(t)
	var gotPath, gotAuth string
	var gotBody map[string]string
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"scope":"srv1:server.sftp"}`))
	}))
	defer panel.Close()
	pointDaemonAt(t, panel.URL, "node-secret")

	perms, err := (&WebSSHAuthorization{}).ValidateKey("alice@example.com#srv1", key)

	require.NoError(t, err)
	assert.Equal(t, "srv1", perms.Extensions["server_id"])
	assert.Equal(t, "/auth/sftp-key", gotPath)
	assert.Equal(t, "Bearer node-secret", gotAuth)
	assert.Equal(t, "alice@example.com#srv1", gotBody["username"])
	offered, _, _, _, err := ssh.ParseAuthorizedKey([]byte(gotBody["publicKey"]))
	require.NoError(t, err)
	assert.Equal(t, key.Marshal(), offered.Marshal())
}

func TestValidateKey_RefusedByThePanel(t *testing.T) {
	for _, answer := range []struct {
		status int
		body   string
	}{
		{http.StatusBadRequest, `{"error":"invalid_request","error_description":"no access"}`},
		{http.StatusUnauthorized, `{"error":"invalid_client"}`},
		{http.StatusTooManyRequests, `{"error":"invalid_request"}`},
		{http.StatusNotFound, `not found`},
		// a 200 must still grant the SFTP scope
		{http.StatusOK, `{"scope":"srv1:server.view"}`},
		{http.StatusOK, `{}`},
	} {
		panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(answer.status)
			_, _ = w.Write([]byte(answer.body))
		}))
		pointDaemonAt(t, panel.URL, "node-secret")

		perms, err := (&WebSSHAuthorization{}).ValidateKey("alice@example.com#srv1", newTestSSHKey(t))

		assert.Error(t, err, "%d %s", answer.status, answer.body)
		assert.Nil(t, perms)
		panel.Close()
	}
}

// the SFTP server offers key logins only when its authorization can check keys, so a
// remote node's authorization must be able to
func TestWebSSHAuthorization_ChecksKeys(t *testing.T) {
	var auth skypanel.SFTPAuthorization = &WebSSHAuthorization{}
	_, ok := auth.(skypanel.SFTPKeyAuthorization)
	assert.True(t, ok)
}

func TestSFTPPermissionsFromAnswer(t *testing.T) {
	perms, err := sftpPermissionsFromAnswer(strings.NewReader(`{"token_type":"Bearer","scope":"abc:server.sftp"}`))
	require.NoError(t, err)
	assert.Equal(t, "abc", perms.Extensions["server_id"])

	_, err = sftpPermissionsFromAnswer(strings.NewReader(`{"error":"invalid_request"}`))
	assert.Error(t, err)
}
