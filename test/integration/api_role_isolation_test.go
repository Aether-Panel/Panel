package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// createRoleUser creates a user that only has the default "Usuario" role, like users
// registered or provisioned on a real panel
func createRoleUser(t *testing.T, db *gorm.DB, name string) (*models.User, string) {
	var role models.Role
	if !assert.NoError(t, db.Where("name = ?", "Usuario").First(&role).Error) {
		t.FailNow()
	}

	user := &models.User{Username: name, Email: name + "@example.com", RoleID: &role.ID}
	if !assert.NoError(t, user.SetPassword(name+"-password")) || !assert.NoError(t, db.Create(user).Error) {
		t.FailNow()
	}

	session, err := createSession(db, user)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return user, session
}

func TestRoleUsersOnlyReachTheirOwnServers(t *testing.T) {
	db, err := database.GetConnection()
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	owner, ownerSession := createRoleUser(t, db, "roleiso-owner")
	_, otherSession := createRoleUser(t, db, "roleiso-other")

	serverID := "roleiso-srv"
	if !assert.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: serverID, Name: serverID, Type: "generic"}).Error) {
		t.FailNow()
	}
	ownerPerms := &models.Permissions{
		UserID:           &owner.ID,
		ServerIdentifier: &serverID,
		Scopes:           []*scopes.Scope{scopes.ScopeServerView},
	}
	if !assert.NoError(t, db.Omit(clause.Associations).Create(ownerPerms).Error) {
		t.FailNow()
	}

	t.Run("OwnerCanOpenTheirServer", func(t *testing.T) {
		res := CallAPI("GET", "/api/servers/"+serverID, nil, ownerSession)
		assert.Equal(t, http.StatusOK, res.Code)
	})

	t.Run("OtherUserCannotOpenIt", func(t *testing.T) {
		res := CallAPI("GET", "/api/servers/"+serverID, nil, otherSession)
		assert.Equal(t, http.StatusForbidden, res.Code)
	})

	t.Run("OtherUserCannotUseTheConsole", func(t *testing.T) {
		res := CallAPI("POST", "/api/servers/"+serverID+"/console", "say hi", otherSession)
		assert.Equal(t, http.StatusForbidden, res.Code)
	})

	t.Run("UptimeOnlyListsOwnServers", func(t *testing.T) {
		res := CallAPI("GET", "/api/uptime", nil, otherSession)
		if !assert.Equal(t, http.StatusOK, res.Code) {
			return
		}
		var stats map[string]interface{}
		assert.NoError(t, json.NewDecoder(res.Body).Decode(&stats))
		assert.NotContains(t, stats, serverID)
	})

	t.Run("CannotCreateTopLevelServer", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        "roleiso-new",
			"node":        0,
			"environment": map[string]interface{}{"type": "host"},
		}
		res := CallAPI("PUT", "/api/servers/roleiso-new", body, otherSession)
		assert.Equal(t, http.StatusForbidden, res.Code)
	})

	t.Run("CannotCreateSubServerOfSomeoneElse", func(t *testing.T) {
		body := map[string]interface{}{
			"name":             "roleiso-child",
			"environment":      map[string]interface{}{"type": "docker", "image": "alpine"},
			"parent_server_id": serverID,
		}
		res := CallAPI("PUT", "/api/servers/roleiso-child", body, otherSession)
		assert.Equal(t, http.StatusForbidden, res.Code)
	})

	t.Run("CannotCreateHostSubServerOfOwnServer", func(t *testing.T) {
		body := map[string]interface{}{
			"name":             "roleiso-child",
			"environment":      map[string]interface{}{"type": "host"},
			"parent_server_id": serverID,
		}
		res := CallAPI("PUT", "/api/servers/roleiso-child", body, ownerSession)
		assert.Equal(t, http.StatusForbidden, res.Code)
	})
}
