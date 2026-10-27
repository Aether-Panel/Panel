package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeAPIKeyPermissions(t *testing.T) {
	got, err := normalizeAPIKeyPermissions([]string{"create", " sso ", "create", "everything", "suspend"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"create", "sso", "suspend"}, got)

	got, err = normalizeAPIKeyPermissions(nil)
	assert.NoError(t, err)
	assert.Equal(t, []string{models.APIKeyAll}, got, "no list: every action, as keys always had")
	_, err = normalizeAPIKeyPermissions([]string{"unknown"})
	assert.ErrorIs(t, err, skypanel.ErrAPIKeyPermissions, "a list of nothing known is not turned into an all-powerful key")
}

func TestRequiresAPIKeyPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	call := func(key *models.APIKey, action string) int {
		r := gin.New()
		r.POST("/x", func(c *gin.Context) {
			if key != nil {
				c.Set("api_key", key)
			}
		}, middleware.RequiresAPIKeyPermission(action), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", nil))
		return w.Code
	}

	limited := &models.APIKey{Permissions: []string{models.APIKeySuspend}}
	assert.Equal(t, http.StatusNoContent, call(limited, models.APIKeySuspend))
	assert.Equal(t, http.StatusForbidden, call(limited, models.APIKeyTerminate))
	assert.Equal(t, http.StatusNoContent, call(&models.APIKey{Permissions: []string{models.APIKeyAll}}, models.APIKeySSO))
	assert.Equal(t, http.StatusForbidden, call(nil, models.APIKeyCreate), "no key, no access")
}
