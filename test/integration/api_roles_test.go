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

	var createdRoleID uint

	t.Run("CreateRole", func(t *testing.T) {
		createdRoleID = testCreateRole(t, session, createdRoleID)
	})

	t.Run("ListRoles", func(t *testing.T) {
		testListRoles(t, session, createdRoleID)
	})

	t.Run("GetRole", func(t *testing.T) {
		testGetRole(t, session, createdRoleID)
	})

	t.Run("UpdateRole", func(t *testing.T) {
		testUpdateRole(t, session, createdRoleID)
	})

	t.Run("DeleteRole", func(t *testing.T) {
		testDeleteRole(t, session, createdRoleID)
	})
}

// testCreateRole returns the new role id, or the previous one when creation fails
func testCreateRole(t *testing.T, session string, previousID uint) uint {
	response := CallAPI("POST", "/api/roles", map[string]interface{}{
		"name":        "TestRole",
		"description": "A role for testing",
		"is_default":  false,
		"permissions": []map[string]interface{}{},
	}, session)

	if !assert.Equal(t, http.StatusOK, response.Code) {
		t.Log(response.Body.String())
		return previousID
	}

	var role models.Role
	err := json.NewDecoder(response.Body).Decode(&role)
	if !assert.NoError(t, err) {
		return previousID
	}

	assert.Equal(t, "TestRole", role.Name)
	assert.NotZero(t, role.ID)
	return role.ID
}

func testListRoles(t *testing.T, session string, createdRoleID uint) {
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
		if r.ID == createdRoleID {
			found = true
			break
		}
	}
	assert.True(t, found, "Newly created role should be in the list")
}

func testGetRole(t *testing.T, session string, createdRoleID uint) {
	response := CallAPI("GET", fmt.Sprintf("/api/roles/%d", createdRoleID), nil, session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return
	}

	var role models.Role
	err := json.NewDecoder(response.Body).Decode(&role)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, createdRoleID, role.ID)
	assert.Equal(t, "TestRole", role.Name)
}

func testUpdateRole(t *testing.T, session string, createdRoleID uint) {
	response := CallAPI("POST", fmt.Sprintf("/api/roles/%d", createdRoleID), map[string]interface{}{
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
}

func testDeleteRole(t *testing.T, session string, createdRoleID uint) {
	response := CallAPI("DELETE", fmt.Sprintf("/api/roles/%d", createdRoleID), nil, session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		t.Log(response.Body.String())
		return
	}

	// Verify deletion
	response2 := CallAPI("GET", fmt.Sprintf("/api/roles/%d", createdRoleID), nil, session)
	assert.Equal(t, http.StatusNotFound, response2.Code)
}
