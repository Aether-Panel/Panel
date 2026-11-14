package api

import (
	"crypto/ed25519"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/stretchr/testify/assert"
)

func TestExTransferKey_IsThePanelIdentity(t *testing.T) {
	assert.True(t, exTransferPublicKey().Equal(services.PanelPublicKey()), "transfers and extension events sign as the same panel")
	sig := ed25519.Sign(exTransferPrivateKey(), []byte("NONCE"))
	assert.True(t, ed25519.Verify(services.PanelPublicKey(), []byte("NONCE"), sig))
}
