package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// A session minted from an OAuth2 client credential must never be upgraded to a
// full browser session: that would turn a leaked client secret into complete
// account access that also skips the account's 2FA.
func TestReauth_RejectsClientSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/auth/reauth", nil)

	c.Set("user", &models.User{ID: 1, Username: "victim", Email: "victim@example.com"})
	c.Set("client", &models.Client{ID: 1, ClientID: "client-id", UserID: 1})

	Reauth(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
