package tests

import (
	"encoding/json"
	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestOauth(t *testing.T) {
	var clientID string
	name := "Test Client"
	description := "this is a test to make sure things even work"

	session, err := createSessionAdmin()
	if !assert.NoError(t, err) {
		return
	}

	t.Run("CreateClient", func(t *testing.T) {
		response := CallAPI("POST", "/api/self/oauth2", map[string]string{
			"name":        name,
			"description": description,
		}, session)
		if !assert.Equal(t, http.StatusOK, response.Code) {
			return
		}
		var client domain.Client
		err = json.NewDecoder(response.Body).Decode(&client)
		if !assert.NoError(t, err) {
			return
		}
		clientID = client.ClientID
	})

	t.Run("GetClient", func(t *testing.T) {
		response := CallAPI("GET", "/api/self/oauth2", nil, session)
		if !assert.Equal(t, http.StatusOK, response.Code) {
			return
		}

		var clients []domain.Client
		err = json.NewDecoder(response.Body).Decode(&clients)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotEmpty(t, clients) {
			return
		}

		found := false
		for _, v := range clients {
			if v.ClientID == clientID {
				found = true
				if !assert.Equal(t, name, v.Name) {
					return
				}
				if !assert.Equal(t, description, v.Description) {
					return
				}
			}
		}
		assert.True(t, found)
	})
}
