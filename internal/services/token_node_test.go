package services

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPanelPublicKeyURL_ComesFromThePanelAddress(t *testing.T) {
	previousToken, previousAuth := config.TokenPublicURL.Value(), config.AuthURL.Value()
	t.Cleanup(func() {
		_ = config.TokenPublicURL.Set(previousToken, false)
		_ = config.AuthURL.Set(previousAuth, false)
	})

	require.NoError(t, config.TokenPublicURL.Set("", false))
	for auth, want := range map[string]string{
		"https://panel.example.com/oauth2/token": "https://panel.example.com/auth/publickey",
		"https://panel.example.com/":             "https://panel.example.com/auth/publickey",
		"":                                       "",
	} {
		require.NoError(t, config.AuthURL.Set(auth, false))
		assert.Equal(t, want, PanelPublicKeyURL(), auth)
	}

	require.NoError(t, config.TokenPublicURL.Set("https://other.example.com/auth/publickey", false))
	assert.Equal(t, "https://other.example.com/auth/publickey", PanelPublicKeyURL(), "token.public wins when it is set")
}

// A node that started while the panel was unreachable used to refuse the panel's requests
// for five minutes; it now reads the key as soon as the panel answers.
func TestPanelKeyfunc_RecoversAsSoonAsThePanelAnswers(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	jwk, err := jwkset.NewJWKFromKey(public, jwkset.JWKOptions{Metadata: jwkset.JWKMetadataOptions{KID: keyID}})
	require.NoError(t, err)
	set, err := json.Marshal(map[string]any{"keys": []any{jwk.Marshal()}})
	require.NoError(t, err)

	var up atomic.Bool
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !up.Load() {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(set)
	}))
	t.Cleanup(panel.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	kf, err := newPanelKeyfunc(ctx, panel.URL, 50*time.Millisecond)
	require.NoError(t, err, "a panel that does not answer yet does not stop the node from starting")

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{"sub": "panel"})
	token.Header[jwkset.HeaderKID] = keyID
	signed, err := token.SignedString(private)
	require.NoError(t, err)

	_, err = jwt.Parse(signed, kf.Keyfunc)
	require.Error(t, err, "no key yet")

	up.Store(true)
	assert.Eventually(t, func() bool {
		_, err := jwt.Parse(signed, kf.Keyfunc)
		return err == nil
	}, 3*time.Second, 100*time.Millisecond, "the key is read again within the retry, not five minutes later")
}

// A node that cannot reach the panel refuses its requests at once. It used to hold each one
// until its next try to read the key (15 seconds), so the nodes page waited that long for the
// node's metrics on every refresh.
func TestPanelKeyfunc_RefusesAtOnceWhileThePanelIsUnreachable(t *testing.T) {
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(panel.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	kf, err := newPanelKeyfunc(ctx, panel.URL, 10*time.Second)
	require.NoError(t, err)

	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{"sub": "panel"})
	token.Header[jwkset.HeaderKID] = keyID
	signed, err := token.SignedString(private)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		start := time.Now()
		_, err = jwt.Parse(signed, kf.Keyfunc)
		require.Error(t, err, "no key to check the token with")
		assert.Less(t, time.Since(start), 2*time.Second, "request %d waited for the next retry", i)
	}
}
