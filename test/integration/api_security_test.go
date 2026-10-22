package tests

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/clause"
)

// a user manager: may edit users but is not an administrator
func createUserManager(t *testing.T) (*models.User, string) {
	db, err := database.GetConnection()
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	user := &models.User{Username: "secmanager", Email: "secmanager@example.com"}
	if !assert.NoError(t, user.SetPassword("secmanager-password")) || !assert.NoError(t, db.Create(user).Error) {
		t.FailNow()
	}
	perms := &models.Permissions{
		UserID: &user.ID,
		Scopes: []*scopes.Scope{scopes.ScopeLogin, scopes.ScopeUserInfoView, scopes.ScopeUserInfoEdit, scopes.ScopeUserPermsEdit},
	}
	if !assert.NoError(t, db.Omit(clause.Associations).Create(perms).Error) {
		t.FailNow()
	}

	session, err := createSession(db, user)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return user, session
}

func TestUserManagerCannotBecomeOrTouchAdmins(t *testing.T) {
	db, err := database.GetConnection()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	manager, session := createUserManager(t)

	var adminRole models.Role
	if !assert.NoError(t, db.Where("name = ?", "Administrador").First(&adminRole).Error) {
		t.FailNow()
	}

	t.Run("CannotGiveItselfTheAdminRole", func(t *testing.T) {
		res := CallAPI("POST", "/api/users/"+strconv.Itoa(int(manager.ID)), map[string]interface{}{"roleId": adminRole.ID}, session)
		assert.Equal(t, http.StatusNoContent, res.Code)

		var stored models.User
		assert.NoError(t, db.First(&stored, manager.ID).Error)
		assert.Nil(t, stored.RoleID, "the role must not change without admin rights")
	})

	t.Run("CannotChangeAnAdminsEmail", func(t *testing.T) {
		res := CallAPI("POST", "/api/users/"+strconv.Itoa(int(loginAdminUser.ID)), map[string]interface{}{"email": "attacker@example.com"}, session)
		assert.Equal(t, http.StatusForbidden, res.Code)

		var stored models.User
		assert.NoError(t, db.First(&stored, loginAdminUser.ID).Error)
		assert.Equal(t, loginAdminUser.Email, stored.Email)
	})

	t.Run("CannotDeleteAnAdmin", func(t *testing.T) {
		res := CallAPI("DELETE", "/api/users/"+strconv.Itoa(int(loginAdminUser.ID)), nil, session)
		assert.Equal(t, http.StatusForbidden, res.Code)
	})

	t.Run("AdminCanStillAssignRoles", func(t *testing.T) {
		admin, err := createSessionAdmin()
		if !assert.NoError(t, err) {
			return
		}
		res := CallAPI("POST", "/api/users/"+strconv.Itoa(int(manager.ID)), map[string]interface{}{"roleId": adminRole.ID}, admin)
		assert.Equal(t, http.StatusNoContent, res.Code)

		var stored models.User
		assert.NoError(t, db.First(&stored, manager.ID).Error)
		if assert.NotNil(t, stored.RoleID) {
			assert.Equal(t, adminRole.ID, *stored.RoleID)
		}

		// put it back so other tests keep a non-admin manager
		assert.NoError(t, db.Exec("UPDATE users SET role_id = NULL WHERE id = ?", manager.ID).Error)
	})
}

func TestAdminPasswordChangeEndsSessions(t *testing.T) {
	db, err := database.GetConnection()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	user := &models.User{Username: "secsessions", Email: "secsessions@example.com"}
	if !assert.NoError(t, user.SetPassword("secsessions-password")) || !assert.NoError(t, db.Create(user).Error) {
		t.FailNow()
	}
	ss := &services.Session{DB: db}
	oldSession, err := ss.CreateForUser(user)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	admin, err := createSessionAdmin()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	res := CallAPI("POST", "/api/users/"+strconv.Itoa(int(user.ID)), map[string]interface{}{"password": "a-brand-new-password-123"}, admin)
	assert.Equal(t, http.StatusNoContent, res.Code)

	_, err = ss.Validate(oldSession)
	assert.Error(t, err, "sessions opened with the old password must stop working")
}

func TestSessionCookieAndCORS(t *testing.T) {
	t.Run("LoginCookieIsSameSiteLax", func(t *testing.T) {
		res := CallAPI("POST", "/auth/login", map[string]string{"email": loginAdminUser.Email, "password": loginAdminUserPassword}, "")
		if !assert.Equal(t, http.StatusOK, res.Code) {
			return
		}
		cookie := res.Header().Get("Set-Cookie")
		assert.Contains(t, cookie, "skypanel_auth=")
		assert.Contains(t, cookie, "SameSite=Lax")
		assert.Contains(t, cookie, "HttpOnly")
	})

	t.Run("OtherSitesDoNotGetCredentialedCORS", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", "/api/self", nil)
		req.Header.Set("Origin", "https://evil.example")
		req.Header.Set("Access-Control-Request-Method", "GET")
		res := serve(req)
		assert.NotEqual(t, "true", res.Header().Get("Access-Control-Allow-Credentials"))
	})
}

func TestDaemonSystemNeedsPanelToken(t *testing.T) {
	res := CallAPI("GET", "/daemon/system", nil, "")
	assert.True(t, res.Code == http.StatusUnauthorized || res.Code == http.StatusForbidden, "got %d", res.Code)
	assert.False(t, strings.Contains(res.Body.String(), "hostname"))
}

func serve(req *http.Request) *httptest.ResponseRecorder {
	writer := httptest.NewRecorder()
	skypanel.Engine.ServeHTTP(writer, req)
	return writer
}
