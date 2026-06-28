package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestRolesAPI(t *testing.T) {
	session, err := createSessionAdmin()
	if !assert.NoError(t, err) {
		return
	}

	var createdRoleId uint

	t.Run("CreateRole", func(t *testing.T) {
		response := CallAPI("POST", "/api/roles", map[string]interface{}{
			"name":        "TestRole",
			"description": "A role for testing",
			"is_default":  false,
			"permissions": []map[string]interface{}{},
		}, session)

		if !assert.Equal(t, http.StatusOK, response.Code) {
			t.Log(response.Body.String())
			return
		}

		var role models.Role
		err := json.NewDecoder(response.Body).Decode(&role)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "TestRole", role.Name)
		assert.NotZero(t, role.ID)
		createdRoleId = role.ID
	})

	t.Run("ListRoles", func(t *testing.T) {
		response := CallAPI("GET", "/api/roles", nil, session)
		if !assert.Equal(t, http.StatusOK, response.Code) {
			return
		}

		var roles []models.Role
		err := json.NewDecoder(response.Body).Decode(&roles)
		if !assert.NoError(t, err) {
			return
		}

		assert.NotEmpty(t, roles)
		found := false
		for _, r := range roles {
			if r.ID == createdRoleId {
				found = true
				break
			}
		}
		assert.True(t, found, "Newly created role should be in the list")
	})

	t.Run("GetRole", func(t *testing.T) {
		response := CallAPI("GET", fmt.Sprintf("/api/roles/%d", createdRoleId), nil, session)
		if !assert.Equal(t, http.StatusOK, response.Code) {
			return
		}

		var role models.Role
		err := json.NewDecoder(response.Body).Decode(&role)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, createdRoleId, role.ID)
		assert.Equal(t, "TestRole", role.Name)
	})

	t.Run("UpdateRole", func(t *testing.T) {
		response := CallAPI("POST", fmt.Sprintf("/api/roles/%d", createdRoleId), map[string]interface{}{
			"name":        "TestRoleUpdated",
			"description": "Updated description",
			"is_default":  false,
		}, session)
		if !assert.Equal(t, http.StatusOK, response.Code) {
			return
		}

		var role models.Role
		err := json.NewDecoder(response.Body).Decode(&role)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "TestRoleUpdated", role.Name)
	})

	t.Run("DeleteRole", func(t *testing.T) {
		response := CallAPI("DELETE", fmt.Sprintf("/api/roles/%d", createdRoleId), nil, session)
		if !assert.Equal(t, http.StatusNoContent, response.Code) {
			t.Log(response.Body.String())
			return
		}

		// Verify deletion
		response2 := CallAPI("GET", fmt.Sprintf("/api/roles/%d", createdRoleId), nil, session)
		assert.Equal(t, http.StatusNotFound, response2.Code)
	})
}
