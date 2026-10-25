package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateTelegram(t *testing.T) {
	token := "123456789:AAHf3xQ9rT1abcdefghijklmnopqrstuvw"
	assert.NoError(t, ValidateTelegram("", ""))
	assert.NoError(t, ValidateTelegram(token, "-1001234567890"))
	assert.NoError(t, ValidateTelegram(token, "@mi_canal"))
	assert.ErrorIs(t, ValidateTelegram(token, ""), ErrInvalidTelegram)
	assert.ErrorIs(t, ValidateTelegram("nope", "123"), ErrInvalidTelegram)
	assert.ErrorIs(t, ValidateTelegram(token+"/../x", "123"), ErrInvalidTelegram)
}

func TestValidateWebhook(t *testing.T) {
	assert.NoError(t, ValidateWebhook(""))
	assert.NoError(t, ValidateWebhook("https://hooks.example.com/aether"))
	assert.ErrorIs(t, ValidateWebhook("http://hooks.example.com"), ErrInvalidWebhook)
	assert.ErrorIs(t, ValidateWebhook("https://user:pass@hooks.example.com"), ErrInvalidWebhook)
	assert.ErrorIs(t, ValidateWebhook("ftp://x"), ErrInvalidWebhook)
}

func useLocalClient(t *testing.T, client *http.Client) {
	discordHTTPClient = func() *http.Client { return client }
	t.Cleanup(func() { discordHTTPClient = defaultDiscordHTTPClient })
}

func TestSendWebhookJSON_SignsTheBody(t *testing.T) {
	var gotBody []byte
	var gotSignature string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotSignature = r.Header.Get("X-Aether-Signature")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	useLocalClient(t, server.Client())

	err := SendWebhookJSON(server.URL+"/hook", "s3cret", map[string]string{"event": "crash"})
	if !assert.NoError(t, err) {
		return
	}
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write(gotBody)
	assert.Equal(t, "sha256="+hex.EncodeToString(mac.Sum(nil)), gotSignature)

	var payload map[string]string
	assert.NoError(t, json.Unmarshal(gotBody, &payload))
	assert.Equal(t, "crash", payload["event"])
}

func TestSendWebhookJSON_ReportsRefusals(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer server.Close()
	useLocalClient(t, server.Client())

	err := SendWebhookJSON(server.URL, "", map[string]string{})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "410")
	}
}

func TestSendTelegram_PostsHTMLToTheBot(t *testing.T) {
	var path string
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&payload)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	useLocalClient(t, server.Client())
	previous := telegramAPI
	telegramAPI = server.URL
	defer func() { telegramAPI = previous }()

	token := "123456789:AAHf3xQ9rT1abcdefghijklmnopqrstuvw"
	err := SendTelegram(token, "-100123", "<b>Caída</b> "+TelegramHTML("a < b & c"))
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "/bot"+token+"/sendMessage", path)
	assert.Equal(t, "-100123", payload["chat_id"])
	assert.Equal(t, "HTML", payload["parse_mode"])
	assert.True(t, strings.Contains(payload["text"].(string), "a &lt; b &amp; c"))
}
