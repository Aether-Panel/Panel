package auth

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type turnstileVerifyResponse struct {
	Success bool `json:"success"`
}

// verifyTurnstile validates a Cloudflare Turnstile token against Cloudflare's
// siteverify endpoint. If Turnstile is disabled or no secret key is configured,
// validation is skipped so the panel keeps working without it. Any other failure,
// Cloudflare unreachable included, refuses the request with ErrCaptchaFailed.
func verifyTurnstile(token, remoteIP string) error {
	if !config.TurnstileEnabled.Value() {
		return nil
	}

	secret := config.TurnstileSecretKey.Value()
	if secret == "" {
		return nil
	}

	if token == "" {
		return skypanel.ErrCaptchaFailed
	}

	params := url.Values{}
	params.Set("secret", secret)
	params.Set("response", token)
	if remoteIP != "" {
		params.Set("remoteip", remoteIP)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.PostForm(turnstileVerifyURL, params)
	if err != nil {
		logging.Error.Printf("Could not reach Cloudflare Turnstile: %s", err)
		return skypanel.ErrCaptchaFailed
	}
	defer resp.Body.Close()

	var result turnstileVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || !result.Success {
		return skypanel.ErrCaptchaFailed
	}
	return nil
}
