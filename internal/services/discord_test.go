package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
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
	realPattern := discordWebhookPattern
	discordWebhookPattern = regexp.MustCompile(`^` + regexp.QuoteMeta(server.URL) + `$`)
	discordHTTPClient = func() *http.Client { return http.DefaultClient }
	t.Cleanup(func() {
		discordWebhookPattern = realPattern
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

// The Discord tests live in this package because main.go starts the Discord service as a
// global, so they exercise the same instance the panel uses.

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

// Discord rejects an embed with more than 25 fields; above that the report used to send 26
func TestSystemStatusFields_StayWithinDiscordsLimit(t *testing.T) {
	servers := func(n int) []ServerInfo {
		list := make([]ServerInfo, n)
		for i := range list {
			list[i] = ServerInfo{Name: fmt.Sprintf("srv%d", i)}
		}
		return list
	}

	assert.Len(t, systemStatusFields(servers(25)), 25, "25 servers fit, no note")
	assert.Equal(t, "srv24", systemStatusFields(servers(25))[24].Name)

	fields := systemStatusFields(servers(40))
	assert.Len(t, fields, discordMaxEmbedFields)
	assert.Equal(t, "Nota", fields[24].Name)
	assert.Equal(t, "Mostrando 24 de 40 servidores", fields[24].Value)
}

// The node report said "🟢 Activo" for a node that did not answer; now it says so instead
func TestNodeUnreachableReport(t *testing.T) {
	description, fields := nodeUnreachableReport(3, "node2", "203.0.113.5", 8080)

	assert.Contains(t, description, "**Nodo:** node2")
	assert.Contains(t, description, "**Total de Servidores:** 3")
	assert.Contains(t, description, "no responde")
	if assert.Len(t, fields, 3) {
		assert.Equal(t, DiscordEmbedField{Name: "Estado", Value: "🔴 Sin respuesta", Inline: true}, fields[0])
		assert.Equal(t, "3", fields[1].Value)
		assert.Equal(t, "203.0.113.5:8080", fields[2].Value)
	}
}

// With the notification language set to English the same report is written in English
func TestNodeUnreachableReport_English(t *testing.T) {
	_ = config.NotificationLanguage.Set("en", false)
	t.Cleanup(func() { _ = config.NotificationLanguage.Set("es", false) })

	description, fields := nodeUnreachableReport(3, "node2", "", 0)

	assert.Contains(t, description, "**Node:** node2")
	assert.Contains(t, description, "**Total servers:** 3")
	assert.Contains(t, description, "does not answer")
	if assert.Len(t, fields, 2) {
		assert.Equal(t, DiscordEmbedField{Name: "Status", Value: "🔴 No answer", Inline: true}, fields[0])
	}
}
