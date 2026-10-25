package services

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestDiscordService_SendWebhookToURL(t *testing.T) {
	// Start a local HTTP server
	var receivedPayload DiscordWebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		err := json.NewDecoder(req.Body).Decode(&receivedPayload)
		assert.NoError(t, err)
		rw.Write([]byte(`OK`))
	}))
	defer server.Close()

	// a real webhook must be a Discord URL reached through the restricted client; this test
	// posts to a local server, so it first checks that is refused and then lets it through
	err := GetDiscordService().SendWebhookToURL(server.URL, "Test Title", "", 0, nil)
	assert.ErrorIs(t, err, ErrInvalidDiscordWebhook)
	checkDiscordWebhook = func(string) error { return nil }
	discordHTTPClient = func() *http.Client { return http.DefaultClient }
	t.Cleanup(func() {
		checkDiscordWebhook = ValidateDiscordWebhook
		discordHTTPClient = utils.NewRestrictedHTTPClient
	})

	// Initialize config for tests
	config.DiscordWebhook.Set(server.URL, false)

	service := GetDiscordService()
	fields := []DiscordEmbedField{
		{Name: "Test Field", Value: "Test Value", Inline: true},
	}

	err = service.SendWebhookToURL(server.URL, "Test Title", "Test Description", 0xFF0000, fields)
	assert.NoError(t, err)

	// Verify payload
	assert.Len(t, receivedPayload.Embeds, 1)
	assert.Equal(t, "Test Title", receivedPayload.Embeds[0].Title)
	assert.Equal(t, "Test Description", receivedPayload.Embeds[0].Description)
	assert.Equal(t, 0xFF0000, receivedPayload.Embeds[0].Color)
	assert.Len(t, receivedPayload.Embeds[0].Fields, 1)
	assert.Equal(t, "Test Field", receivedPayload.Embeds[0].Fields[0].Name)
}

/* este archivo interactua con servicios externos como los de discord y hace pruebas de que todo funcione correctamente, este archivo se coloco aqui por que en el archivo main.go se inicializa el servicio de discord como un servicio global */

func TestValidateDiscordWebhook(t *testing.T) {
	for _, ok := range []string{
		"",
		"https://discord.com/api/webhooks/123/abc_DEF-9",
		"https://discordapp.com/api/webhooks/123/abc",
		"https://ptb.discord.com/api/webhooks/123/abc",
		"https://canary.discord.com/api/webhooks/123/abc",
	} {
		if err := ValidateDiscordWebhook(ok); err != nil {
			t.Errorf("%q should be accepted: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"http://discord.com/api/webhooks/123/abc",
		"https://discord.com.attacker.example/api/webhooks/123/abc",
		"https://evil.example/https://discord.com/api/webhooks/1/a",
		"https://discord.com/api/webhooks/123/abc?wait=true&x=http://127.0.0.1",
		"https://127.0.0.1/api/webhooks/1/a",
		"https://discord.com/api/webhooks/notanumber/abc",
	} {
		if err := ValidateDiscordWebhook(bad); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

func TestPostDiscordPayloadRefusesNonDiscordURLs(t *testing.T) {
	// the system status report must not be a way around the webhook check either
	var hit bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := postDiscordPayload(server.URL+"/api/webhooks/1/token", DiscordWebhookPayload{Content: "status"})
	if !errors.Is(err, ErrInvalidDiscordWebhook) {
		t.Fatalf("got %v, want ErrInvalidDiscordWebhook", err)
	}
	if hit {
		t.Fatal("the payload was posted to a URL that is not a Discord webhook")
	}
	if err := postDiscordPayload("", DiscordWebhookPayload{}); err != nil {
		t.Fatalf("an empty webhook means notifications are off, got %v", err)
	}
}
