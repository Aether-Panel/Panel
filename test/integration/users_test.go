package tests

import (
	"encoding/json"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestUserApi(t *testing.T) {
	session, err := createSessionAdmin()
	if !assert.NoError(t, err) {
		return
	}

	t.Run("TestUserWithPerms", func(t *testing.T) {
		testUserPerms(t, session, loginNoServerViewUser.ID, false)
	})

	var newUserID uint

	t.Run("CreateUser", func(t *testing.T) {
		newUserID = testCreateUser(t, session, newUserID)
	})

	t.Run("TestEmptyUserPerms", func(t *testing.T) {
		testUserPerms(t, session, newUserID, true)
	})

	t.Run("TestSetEmptyPerms", func(t *testing.T) {
		testSetEmptyUserPerms(t, session, newUserID)
	})

	t.Run("TestDelete", func(t *testing.T) {
		url := fmt.Sprintf("/api/users/%d", newUserID)
		response := CallAPI("DELETE", url, nil, session)
		if !assert.Equal(t, http.StatusNoContent, response.Code) {
			return
		}
	})
}

// testUserPerms fetches a user's perms and checks whether its scopes are empty
func testUserPerms(t *testing.T, session string, userID uint, expectEmpty bool) {
	url := fmt.Sprintf("/api/users/%d/perms", userID)
	response := CallAPI("GET", url, nil, session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return
	}

	perms := &models.PermissionView{}
	err := json.NewDecoder(response.Body).Decode(perms)
	if !assert.NoError(t, err) {
		return
	}

	if expectEmpty {
		assert.Empty(t, perms.Scopes)
	} else {
		assert.NotEmpty(t, perms.Scopes)
	}
}

// testCreateUser returns the new user id, or the previous one when creation fails
func testCreateUser(t *testing.T, session string, previousID uint) uint {
	response := CallAPI("POST", "/api/users", map[string]string{
		"username": "apicreateduser",
		"email":    "none@example.com",
		"password": "testing123!",
	}, session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return previousID
	}

	user := &models.UserView{}
	err := json.NewDecoder(response.Body).Decode(user)
	if !assert.NoError(t, err) {
		return previousID
	}
	return user.ID
}

func testSetEmptyUserPerms(t *testing.T, session string, userID uint) {
	url := fmt.Sprintf("/api/users/%d/perms", userID)
	response := CallAPI("PUT", url, map[string]interface{}{
		"scopes": []string{},
	}, session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	response = CallAPI("GET", url, nil, session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return
	}

	perms := &models.PermissionView{}
	err := json.NewDecoder(response.Body).Decode(perms)
	if !assert.NoError(t, err) {
		return
	}

	assert.Empty(t, perms.Scopes)
}
