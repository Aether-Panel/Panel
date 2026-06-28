package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/models"
	"github.com/stretchr/testify/assert"
)

func TestSelfAPI(t *testing.T) {
	session, err := createSessionAdmin()
	if !assert.NoError(t, err) {
		return
	}

	t.Run("GetProfile", func(t *testing.T) {
		response := CallAPI("GET", "/api/self", nil, session)
		if !assert.Equal(t, http.StatusOK, response.Code) {
			return
		}

		var user models.UserView
		err := json.NewDecoder(response.Body).Decode(&user)
		assert.NoError(t, err)
		assert.Equal(t, loginAdminUser.Email, user.Email)
	})

	t.Run("UpdateProfile", func(t *testing.T) {
		response := CallAPI("PUT", "/api/self", map[string]interface{}{
			"username": loginAdminUser.Username,
			"email":    loginAdminUser.Email,
			"password": loginAdminUserPassword,
		}, session)
		if !assert.Equal(t, http.StatusNoContent, response.Code) {
			return
		}
	})
}
