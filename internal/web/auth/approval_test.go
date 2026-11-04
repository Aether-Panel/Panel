package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// approvalRouter is the registration and sign-in routes on a fresh database, without captcha
func approvalRouter(t *testing.T, approval bool) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "auth.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permissions{}, &models.Session{}, &models.ActivityLog{}))

	settings := map[*config.BoolEntry]bool{
		&config.RegistrationEnabled: true, &config.RegistrationApproval: approval,
		&config.CaptchaBuiltin: false, &config.TurnstileEnabled: false,
	}
	for entry, value := range settings {
		previous := entry.Value()
		_ = entry.Set(value, false)
		t.Cleanup(func() { _ = entry.Set(previous, false) })
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db) })
	r.POST("/auth/register", RegisterPost)
	r.POST("/auth/login", LoginPost)
	return r, db
}

func postJSON(r http.Handler, path string, body interface{}) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

var newAccount = map[string]string{"username": "newcomer", "email": "new@example.com", "password": "Very-Strong-Pass-123"}

func TestRegister_WithApprovalTheAccountWaits(t *testing.T) {
	r, db := approvalRouter(t, true)

	w := postJSON(r, "/auth/register", newAccount)
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	assert.JSONEq(t, `{"pending":true}`, w.Body.String())
	assert.Empty(t, w.Result().Cookies(), "no session for an account that waits")

	var user models.User
	require.NoError(t, db.First(&user, "email = ?", "new@example.com").Error)
	assert.True(t, user.PendingApproval)
	allowed, err := (&services.Permission{DB: db}).HasPermission(user.ID, "", scopes.ScopeLogin)
	require.NoError(t, err)
	assert.False(t, allowed, "it cannot sign in, by any route, until approved")

	w = postJSON(r, "/auth/login", map[string]string{"email": "new@example.com", "password": "Very-Strong-Pass-123"})
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ErrAccountPending")

	w = postJSON(r, "/auth/login", map[string]string{"email": "new@example.com", "password": "wrong-password-1"})
	assert.NotContains(t, w.Body.String(), "ErrAccountPending", "only the right password learns the account waits")
}

func TestRegister_WithoutApprovalSignsIn(t *testing.T) {
	r, db := approvalRouter(t, false)

	w := postJSON(r, "/auth/register", newAccount)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var user models.User
	require.NoError(t, db.First(&user, "email = ?", "new@example.com").Error)
	assert.False(t, user.PendingApproval)
}
