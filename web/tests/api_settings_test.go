package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/models"
	"github.com/stretchr/testify/assert"
)

func TestSettingsAPI(t *testing.T) {
	session, err := createSessionAdmin()
	if !assert.NoError(t, err) {
		return
	}

	t.Run("GetSettings", func(t *testing.T) {
		response := CallAPI("GET", "/api/settings", nil, session)
		if !assert.Equal(t, http.StatusOK, response.Code) {
			return
		}

		var settings map[string]interface{}
		err := json.NewDecoder(response.Body).Decode(&settings)
		assert.NoError(t, err)
		assert.NotEmpty(t, settings)
	})

	t.Run("UpdateSetting", func(t *testing.T) {
		key := "panel.settings.companyName"
		response := CallAPI("PUT", fmt.Sprintf("/api/settings/%s", key), map[string]interface{}{
			"value": "TestCompany",
		}, session)
		if !assert.Equal(t, http.StatusNoContent, response.Code) {
			t.Log(response.Body.String())
			return
		}

		// Verify
		response2 := CallAPI("GET", fmt.Sprintf("/api/settings/%s", key), nil, session)
		assert.Equal(t, http.StatusOK, response2.Code)
		var setting models.Setting
		json.NewDecoder(response2.Body).Decode(&setting)
		assert.Equal(t, "TestCompany", setting.Value)
	})

	t.Run("UpdateMultipleSettings", func(t *testing.T) {
		response := CallAPI("POST", "/api/settings", map[string]interface{}{
			"panel.settings.companyName": "TestCompany2",
			"panel.settings.registrationEnabled": true,
		}, session)
		if !assert.Equal(t, http.StatusNoContent, response.Code) {
			return
		}
	})
}
