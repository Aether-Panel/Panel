package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type turnstileVerifyResponse struct {
	Success bool `json:"success"`
}

// verifyTurnstile validates a Cloudflare Turnstile token against Cloudflare's
// siteverify endpoint. If Turnstile is disabled or no secret key is configured,
// validation is skipped so the panel keeps working without it.
func verifyTurnstile(token string) error {
	if !config.TurnstileEnabled.Value() {
		return nil
	}

	secret := config.TurnstileSecretKey.Value()
	if secret == "" {
		return nil
	}

	params := url.Values{}
	params.Set("secret", secret)
	params.Set("response", token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.PostForm(turnstileVerifyURL, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result turnstileVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Success {
		return errors.New("turnstile verification failed")
	}
	return nil
}
