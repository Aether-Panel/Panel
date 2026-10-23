package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Telegram and plain JSON webhooks, next to Discord, so server owners can get their alerts
// where they already are. Every request goes through the client that refuses private
// addresses: these URLs come from users.

var (
	ErrInvalidTelegram = errors.New("the Telegram bot token or chat id is not valid")
	ErrInvalidWebhook  = errors.New("the webhook must be an https:// URL")
)

var (
	telegramTokenPattern = regexp.MustCompile(`^\d{5,15}:[\w-]{30,}$`)
	telegramChatPattern  = regexp.MustCompile(`^(-?\d{1,20}|@[A-Za-z][\w]{4,31})$`)
)

// telegramAPI is swapped in tests, which talk to a local server
var telegramAPI = "https://api.telegram.org"

// ValidateTelegram accepts both empty (Telegram off), or a bot token with a chat id
func ValidateTelegram(token, chat string) error {
	if token == "" && chat == "" {
		return nil
	}
	if !telegramTokenPattern.MatchString(token) || !telegramChatPattern.MatchString(chat) {
		return ErrInvalidTelegram
	}
	return nil
}

// ValidateWebhook accepts an empty URL (off) or an https URL with a host
func ValidateWebhook(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ErrInvalidWebhook
	}
	return nil
}

// SendTelegram posts an HTML-formatted message through a bot
func SendTelegram(token, chat, text string) error {
	if err := ValidateTelegram(token, chat); err != nil || token == "" {
		return err
	}
	body, err := json.Marshal(map[string]interface{}{
		"chat_id":                  chat,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	})
	if err != nil {
		return err
	}
	return postJSON(fmt.Sprintf("%s/bot%s/sendMessage", telegramAPI, token), body, nil, "telegram")
}

// SendWebhookJSON posts a JSON event. With a secret, the body is signed in the
// X-Aether-Signature header (sha256=<hex HMAC>), so the receiver can check it came from us.
func SendWebhookJSON(webhookURL, secret string, payload interface{}) error {
	if err := ValidateWebhook(webhookURL); err != nil || webhookURL == "" {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	headers := map[string]string{"User-Agent": "AetherPanel-Webhook/1"}
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		headers["X-Aether-Signature"] = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}
	return postJSON(webhookURL, body, headers, "webhook")
}

func postJSON(target string, body []byte, headers map[string]string, what string) error {
	req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := *discordHTTPClient()
	client.Timeout = 10 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		// the error text can hold the URL, and a Telegram URL holds the bot token
		return fmt.Errorf("%s: could not reach the server", what)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s answered with status %d", what, resp.StatusCode)
	}
	return nil
}

// TelegramHTML escapes text for Telegram's HTML mode
func TelegramHTML(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}
