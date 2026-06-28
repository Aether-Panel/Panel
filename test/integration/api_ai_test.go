package tests

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestAIAPI(t *testing.T) {
	session, err := createSessionAdmin()
	if !assert.NoError(t, err) {
		return
	}

	t.Run("NoAPIKey", func(t *testing.T) {
		// Ensure API key is empty
		config.GeminiAPIKey.Set("", false)

		response := CallAPI("POST", "/api/ai/analyze", map[string]interface{}{
			"logs": []string{"Error: Something went wrong"},
		}, session)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "Gemini API Key is not configured")
	})

	t.Run("EmptyLogs", func(t *testing.T) {
		config.GeminiAPIKey.Set("fake-key", false)

		response := CallAPI("POST", "/api/ai/analyze", map[string]interface{}{
			"logs": []string{},
		}, session)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "No logs provided for analysis")
	})
}
