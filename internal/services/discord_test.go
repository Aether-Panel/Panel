package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
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

	// Initialize config for tests
	config.DiscordWebhook.Set(server.URL, false)

	service := GetDiscordService()
	fields := []DiscordEmbedField{
		{Name: "Test Field", Value: "Test Value", Inline: true},
	}

	err := service.SendWebhookToURL(server.URL, "Test Title", "Test Description", 0xFF0000, fields)
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
