package tests

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/clause"
	"net/http"
	"testing"
)

func TestAIAPI(t *testing.T) {
	session, err := createSessionAdmin()
	if !assert.NoError(t, err) {
		return
	}

	// log analysis is only offered per server, to users who can read its console
	db, err := database.GetConnection()
	if !assert.NoError(t, err) {
		return
	}
	serverID := "aitest-srv"
	if !assert.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: serverID, Name: serverID, Type: "generic"}).Error) {
		return
	}

	t.Run("NoAPIKey", func(t *testing.T) {
		// Ensure API key is empty
		config.GeminiAPIKey.Set("", false)

		response := CallAPI("POST", "/api/servers/"+serverID+"/ai/analyze", map[string]interface{}{
			"logs": []string{"Error: Something went wrong"},
		}, session)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "Gemini API Key is not configured")
	})

	t.Run("EmptyLogs", func(t *testing.T) {
		config.GeminiAPIKey.Set("fake-key", false)

		response := CallAPI("POST", "/api/servers/"+serverID+"/ai/analyze", map[string]interface{}{
			"logs": []string{},
		}, session)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "No logs provided for analysis")
	})
}
