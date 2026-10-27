package api

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
)

// The Turnstile switch and keys are edited in Settings → General; they were missing from the
// editable entries, so saving them was silently ignored and Turnstile could never be turned on.
func TestTurnstileSettingsAreEditable(t *testing.T) {
	enabled, site, secret := config.TurnstileEnabled.Value(), config.TurnstileSiteKey.Value(), config.TurnstileSecretKey.Value()
	t.Cleanup(func() {
		_ = config.TurnstileEnabled.Set(enabled, false)
		_ = config.TurnstileSiteKey.Set(site, false)
		_ = config.TurnstileSecretKey.Set(secret, false)
	})

	editable := map[string]bool{}
	for _, e := range editableStringEntries {
		editable[e.Key()] = true
	}
	for _, e := range editableBoolEntries {
		editable[e.Key()] = true
	}
	for _, key := range []string{config.TurnstileEnabled.Key(), config.TurnstileSiteKey.Key(), config.TurnstileSecretKey.Key()} {
		assert.True(t, editable[key], key)
	}
}
