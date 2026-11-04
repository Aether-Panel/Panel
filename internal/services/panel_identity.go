package services

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"sync"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
)

// The panel's identity: an ed25519 key generated once and kept in the config file
// (panel.exTransferKey, the name it had when only federated transfers used it). It signs what
// the panel sends to other systems — transfer requests to other panels, extension events — and
// its public half is published at /.well-known/aether-panel.json, so a receiver can check that
// something really came from the panel at that address.

// PanelSignatureHeader carries the panel's ed25519 signature of an extension event's body
const PanelSignatureHeader = "X-Aether-Panel-Signature"

var (
	panelKeyOnce sync.Once
	panelKey     ed25519.PrivateKey
)

// PanelIdentityKey is the panel's signing key; it is created on first use and survives restarts
func PanelIdentityKey() ed25519.PrivateKey {
	panelKeyOnce.Do(func() {
		if seed, err := base64.StdEncoding.DecodeString(config.ExTransferKey.Value()); err == nil && len(seed) == ed25519.SeedSize {
			panelKey = ed25519.NewKeyFromSeed(seed)
			return
		}
		seed := make([]byte, ed25519.SeedSize)
		if _, err := rand.Read(seed); err != nil {
			panic(err)
		}
		if err := config.ExTransferKey.Set(base64.StdEncoding.EncodeToString(seed), true); err != nil {
			logging.Error.Printf("Could not save the panel's identity key, it will change on restart: %s", err)
		}
		panelKey = ed25519.NewKeyFromSeed(seed)
	})
	return panelKey
}

// PanelPublicKey is the public half of PanelIdentityKey
func PanelPublicKey() ed25519.PublicKey {
	return PanelIdentityKey().Public().(ed25519.PublicKey)
}

// PanelURL is the panel's address as set in Settings (panel.settings.masterUrl), without a
// trailing slash
func PanelURL() string {
	return strings.TrimRight(strings.TrimSpace(config.MasterURL.Value()), "/")
}

// PanelSignature is the header value that signs body with the panel's identity:
// "ed25519=<base64 signature>"
func PanelSignature(body []byte) string {
	return "ed25519=" + base64.StdEncoding.EncodeToString(ed25519.Sign(PanelIdentityKey(), body))
}
