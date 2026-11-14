package api

import (
	"net/http"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestApproveUser_LetsTheAccountSignIn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "approve.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permissions{}))
	user := &models.User{Username: "newcomer", Email: "new@example.com", HashedPassword: "x", PendingApproval: true}
	require.NoError(t, db.Create(user).Error)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db) })
	r.POST("/users/:id/approve", approveUser)

	w := doJSON(r, http.MethodPost, "/users/"+itoa(user.ID)+"/approve", nil)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	var stored models.User
	require.NoError(t, db.First(&stored, user.ID).Error)
	assert.False(t, stored.PendingApproval)
	allowed, err := (&services.Permission{DB: db}).HasPermission(user.ID, "", scopes.ScopeLogin)
	require.NoError(t, err)
	assert.True(t, allowed)

	w = doJSON(r, http.MethodPost, "/users/"+itoa(user.ID)+"/approve", nil)
	assert.Equal(t, http.StatusNoContent, w.Code, "approving twice does nothing")
	w = doJSON(r, http.MethodPost, "/users/999/approve", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func itoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
