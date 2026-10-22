package api

import (
	"crypto/ed25519"
	"sync"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestExTransferKey_SurvivesARestart(t *testing.T) {
	previous := config.ExTransferKey.Value()
	t.Cleanup(func() {
		_ = config.ExTransferKey.Set(previous, false)
		exTransferKeyOnce = sync.Once{}
	})
	_ = config.ExTransferKey.Set("", false)

	exTransferKeyOnce = sync.Once{}
	first := exTransferPublicKey()
	saved := config.ExTransferKey.Value()
	assert.NotEmpty(t, saved, "the key is stored in the config")

	// a restart starts with empty memory but the same config
	exTransferKeyOnce = sync.Once{}
	second := exTransferPublicKey()

	assert.True(t, first.Equal(second), "the public key the other panel stored must still verify our signatures")
	sig := ed25519.Sign(exTransferPrivateKey(), []byte("NONCE"))
	assert.True(t, ed25519.Verify(first, []byte("NONCE"), sig))
}
