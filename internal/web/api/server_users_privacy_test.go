package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func serverUsersContext(ps *services.Permission, email string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/servers/parent01/user/"+email, nil)
	c.Params = gin.Params{{Key: "email", Value: email}}
	c.Set("db", ps.DB)
	c.Set("server", &models.Server{Identifier: "parent01"})
	getServerUsers(c)
	return w
}

func TestGetServerUsers_SomeoneWithoutAccessLooksUnknown(t *testing.T) {
	ps, _ := setupSubServerTest(t)
	stranger := &models.User{Username: "stranger", Email: "stranger@example.com", HashedPassword: "x"}
	if !assert.NoError(t, ps.DB.Create(stranger).Error) {
		return
	}

	assert.Equal(t, http.StatusNotFound, serverUsersContext(ps, "nobody@example.com").Code)
	w := serverUsersContext(ps, "stranger@example.com")
	assert.Equal(t, http.StatusNotFound, w.Code, "an account with no access answers like an unknown email")
	assert.NotContains(t, w.Body.String(), "stranger")

	assert.Equal(t, http.StatusOK, serverUsersContext(ps, "owner@example.com").Code, "members are still found")
}

func TestFindOrInviteServerUser_SignupsClosed(t *testing.T) {
	ps, _ := setupSubServerTest(t)
	t.Cleanup(func() { _ = config.RegistrationEnabled.Set(true, false) })
	_ = config.RegistrationEnabled.Set(false, false)
	us := &services.User{DB: ps.DB}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/", nil)

	user, _, ok := findOrInviteServerUser(c, us, "new@example.com", "")
	assert.False(t, ok)
	assert.Nil(t, user)
	assert.Equal(t, http.StatusForbidden, w.Code, "an owner cannot open accounts while sign-ups are closed")

	var count int64
	ps.DB.Model(&models.User{}).Where("email = ?", "new@example.com").Count(&count)
	assert.Zero(t, count)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/", nil)
	existing, _, ok := findOrInviteServerUser(c, us, "owner@example.com", "")
	assert.True(t, ok, "people who already have an account can still be added")
	assert.NotNil(t, existing)
}
