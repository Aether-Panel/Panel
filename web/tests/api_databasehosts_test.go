package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/models"
	"github.com/stretchr/testify/assert"
)

func TestDatabaseHostsAPI(t *testing.T) {
	session, err := createSessionAdmin()
	if !assert.NoError(t, err) {
		return
	}

	var hostId uint

	t.Run("CreateDatabaseHost", func(t *testing.T) {
		response := CallAPI("POST", "/api/databasehosts", map[string]interface{}{
			"name":     "TestDBHost",
			"host":     "127.0.0.1",
			"port":     3306,
			"username": "root",
			"password": "password",
		}, session)
		
		if !assert.Equal(t, http.StatusOK, response.Code) {
			return
		}

		var host models.DatabaseHost
		err := json.NewDecoder(response.Body).Decode(&host)
		assert.NoError(t, err)
		hostId = host.ID
		assert.NotZero(t, hostId)
	})

	t.Run("ListDatabaseHosts", func(t *testing.T) {
		response := CallAPI("GET", "/api/databasehosts", nil, session)
		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("DeleteDatabaseHost", func(t *testing.T) {
		response := CallAPI("DELETE", "/api/databasehosts/999", nil, session)
		assert.Equal(t, http.StatusNoContent, response.Code)
	})
}
