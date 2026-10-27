package tests

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/clause"
)

// The node's SFTP check sends the user's email and password. It must learn whether SFTP is
// allowed, but must not receive a panel session: that would turn a password into full
// account access, skipping the account's 2FA.
func TestSFTPPasswordGrantDoesNotCreateASession(t *testing.T) {
	db, err := database.GetConnection()
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	user := &models.User{Username: "sftpgrant", Email: "sftpgrant@example.com", OtpActive: true}
	if !assert.NoError(t, user.SetPassword("sftpgrant-password")) || !assert.NoError(t, db.Create(user).Error) {
		t.FailNow()
	}
	serverID := "sftpgrant-srv"
	if !assert.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: serverID, Name: serverID, Type: "generic"}).Error) {
		t.FailNow()
	}
	perms := &models.Permissions{UserID: &user.ID, ServerIdentifier: &serverID, Scopes: []*scopes.Scope{scopes.ScopeServerSftp}}
	if !assert.NoError(t, db.Omit(clause.Associations).Create(perms).Error) {
		t.FailNow()
	}
	signIn := &models.Permissions{UserID: &user.ID, Scopes: []*scopes.Scope{scopes.ScopeLogin}}
	if !assert.NoError(t, db.Omit(clause.Associations).Create(signIn).Error) {
		t.FailNow()
	}

	var before int64
	db.Model(&models.Session{}).Where("user_id = ?", user.ID).Count(&before)

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("username", user.Email+"#"+serverID)
	form.Set("password", "sftpgrant-password")
	request, _ := http.NewRequest("POST", "/oauth2/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Bearer "+models.LocalNode.Secret)
	res := serve(request)

	if !assert.Equal(t, http.StatusOK, res.Code, res.Body.String()) {
		return
	}
	var body map[string]interface{}
	assert.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, serverID+":"+scopes.ScopeServerSftp.String(), body["scope"], "the node still learns SFTP is allowed")
	assert.Empty(t, body["access_token"], "no panel token is handed out")

	var after int64
	db.Model(&models.Session{}).Where("user_id = ?", user.ID).Count(&after)
	assert.Equal(t, before, after, "no panel session is created")
}

// Taking away the login permission locks the account out of SFTP too, not only the web
func TestSFTPPasswordGrantRefusesAnAccountThatMayNotSignIn(t *testing.T) {
	db, err := database.GetConnection()
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	user := &models.User{Username: "sftpnologin", Email: "sftpnologin@example.com"}
	if !assert.NoError(t, user.SetPassword("sftpnologin-password")) || !assert.NoError(t, db.Create(user).Error) {
		t.FailNow()
	}
	serverID := "sftpnologin-srv"
	if !assert.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: serverID, Name: serverID, Type: "generic"}).Error) {
		t.FailNow()
	}
	perms := &models.Permissions{UserID: &user.ID, ServerIdentifier: &serverID, Scopes: []*scopes.Scope{scopes.ScopeServerSftp}}
	if !assert.NoError(t, db.Omit(clause.Associations).Create(perms).Error) {
		t.FailNow()
	}

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("username", user.Email+"#"+serverID)
	form.Set("password", "sftpnologin-password")
	request, _ := http.NewRequest("POST", "/oauth2/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Bearer "+models.LocalNode.Secret)
	res := serve(request)

	assert.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
}
