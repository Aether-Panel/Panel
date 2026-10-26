package auth

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

func setTurnstile(t *testing.T, enabled bool, secret string) {
	prevEnabled, prevSecret := config.TurnstileEnabled.Value(), config.TurnstileSecretKey.Value()
	_ = config.TurnstileEnabled.Set(enabled, false)
	_ = config.TurnstileSecretKey.Set(secret, false)
	t.Cleanup(func() {
		_ = config.TurnstileEnabled.Set(prevEnabled, false)
		_ = config.TurnstileSecretKey.Set(prevSecret, false)
	})
}

func TestVerifyTurnstile_SkippedWhenOffOrWithoutSecret(t *testing.T) {
	setTurnstile(t, false, "secret")
	assert.NoError(t, verifyTurnstile("", "203.0.113.1"))

	setTurnstile(t, true, "")
	assert.NoError(t, verifyTurnstile("", "203.0.113.1"))
}

func TestVerifyTurnstile_RefusesAMissingToken(t *testing.T) {
	setTurnstile(t, true, "secret")
	assert.ErrorIs(t, verifyTurnstile("", "203.0.113.1"), skypanel.ErrCaptchaFailed)
}
