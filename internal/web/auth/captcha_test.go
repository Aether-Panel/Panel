package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

// solve does what the browser does: try every number until the hash matches
func solve(t *testing.T, ch *Challenge) solution {
	for n := 0; n <= ch.MaxNumber; n++ {
		if hashChallenge(ch.Salt, n) == ch.Challenge {
			return solution{Algorithm: ch.Algorithm, Challenge: ch.Challenge, Number: n, Salt: ch.Salt, Signature: ch.Signature}
		}
	}
	t.Fatal("challenge has no solution")
	return solution{}
}

func encode(s solution) string {
	raw, _ := json.Marshal(s)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestChallenge_ASolutionWorksOnce(t *testing.T) {
	now := time.Now()
	ch, err := newChallenge(now)
	if !assert.NoError(t, err) {
		return
	}
	token := encode(solve(t, ch))

	assert.NoError(t, verifyChallenge(token, now))
	assert.ErrorIs(t, verifyChallenge(token, now), skypanel.ErrCaptchaFailed, "a solution cannot be replayed")
}

func TestChallenge_RefusesWrongAnswers(t *testing.T) {
	now := time.Now()
	ch, err := newChallenge(now)
	if !assert.NoError(t, err) {
		return
	}
	good := solve(t, ch)

	wrongNumber := good
	wrongNumber.Number = (good.Number + 1) % (ch.MaxNumber + 1)
	forged := good
	forged.Salt = "aaaa?expires=9999999999"
	forged.Challenge = hashChallenge(forged.Salt, 1)
	forged.Number = 1

	for name, token := range map[string]string{
		"empty":                                  "",
		"not base64":                             "%%%",
		"wrong number":                           encode(wrongNumber),
		"own challenge, not signed by the panel": encode(forged),
	} {
		assert.ErrorIs(t, verifyChallenge(token, now), skypanel.ErrCaptchaFailed, name)
	}
	assert.ErrorIs(t, verifyChallenge(encode(good), now.Add(captchaLifetime+time.Minute)), skypanel.ErrCaptchaFailed, "expired")
}

func TestCaptchaProvider(t *testing.T) {
	builtin, enabled, site, secret := config.CaptchaBuiltin.Value(), config.TurnstileEnabled.Value(), config.TurnstileSiteKey.Value(), config.TurnstileSecretKey.Value()
	t.Cleanup(func() {
		_ = config.CaptchaBuiltin.Set(builtin, false)
		_ = config.TurnstileEnabled.Set(enabled, false)
		_ = config.TurnstileSiteKey.Set(site, false)
		_ = config.TurnstileSecretKey.Set(secret, false)
	})

	_ = config.CaptchaBuiltin.Set(true, false)
	_ = config.TurnstileEnabled.Set(true, false)
	_ = config.TurnstileSiteKey.Set("", false)
	_ = config.TurnstileSecretKey.Set("", false)
	assert.Equal(t, CaptchaBuiltin, CaptchaProvider(), "Turnstile without its keys falls back to the built-in check")

	_ = config.TurnstileSiteKey.Set("site", false)
	_ = config.TurnstileSecretKey.Set("secret", false)
	assert.Equal(t, CaptchaTurnstile, CaptchaProvider())

	_ = config.TurnstileEnabled.Set(false, false)
	_ = config.CaptchaBuiltin.Set(false, false)
	assert.Equal(t, "", CaptchaProvider())
}
