package services

import (
	"crypto/ed25519"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPanelSignedWebhook_CanBeCheckedWithThePublicKey(t *testing.T) {
	var body []byte
	var signature string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		signature = r.Header.Get(PanelSignatureHeader)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	useLocalClient(t, srv.Client())

	require.NoError(t, SendPanelSignedWebhookJSON(srv.URL+"/hook", "secret", map[string]string{"event": "server.started"}))
	require.True(t, strings.HasPrefix(signature, "ed25519="), signature)
	sig, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(signature, "ed25519="))
	require.NoError(t, err)
	assert.True(t, ed25519.Verify(PanelPublicKey(), body, sig), "the receiver checks the body with the published key")
	assert.False(t, ed25519.Verify(PanelPublicKey(), append(body, ' '), sig), "a changed body fails")
}

func TestPanelIdentityKey_SurvivesARestart(t *testing.T) {
	previous := config.ExTransferKey.Value()
	t.Cleanup(func() {
		_ = config.ExTransferKey.Set(previous, false)
		panelKeyOnce = sync.Once{}
	})
	_ = config.ExTransferKey.Set("", false)

	panelKeyOnce = sync.Once{}
	first := PanelPublicKey()
	assert.NotEmpty(t, config.ExTransferKey.Value(), "the key is stored in the config")

	// a restart starts with empty memory but the same config
	panelKeyOnce = sync.Once{}
	assert.True(t, first.Equal(PanelPublicKey()), "receivers that pinned the key must still verify the panel")
	assert.Len(t, first, ed25519.PublicKeySize)
}

func TestNormalizePanelURL_IsAlwaysAFullAddress(t *testing.T) {
	cases := [][2]string{
		{"test.aetherpanel.es", "https://test.aetherpanel.es"},
		{"https://panel.example.com/", "https://panel.example.com"},
		{"http://10.0.0.5:8080", "http://10.0.0.5:8080"},
		{"panel.example.com:8443/", "https://panel.example.com:8443"},
		{"", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c[1], NormalizePanelURL(c[0]), c[0])
	}
	assert.Equal(t, "https://panel.example.com", NormalizePanelURL("  panel.example.com/  "), "spaces around it are dropped")
}
