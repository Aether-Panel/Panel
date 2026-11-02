package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validateJWTWith(authorization string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/daemon/server/srv1/status", nil)
	if authorization != "" {
		c.Request.Header.Set("Authorization", authorization)
	}
	ValidateJWT(c)
	if !c.IsAborted() {
		c.Status(http.StatusNoContent)
	}
	c.Writer.WriteHeaderNow()
	return w
}

// A token that does not check out is a failed authentication like a missing one, so both
// answer 401 with a WWW-Authenticate challenge (it used to be 400)
func TestValidateJWT_InvalidTokenIsUnauthorized(t *testing.T) {
	for _, authorization := range []string{"", "Basic abc", "Bearer not-a-jwt", "Bearer eyJhbGciOiJIUzI1NiJ9.e30.c2lnbmF0dXJl"} {
		w := validateJWTWith(authorization)
		assert.Equal(t, http.StatusUnauthorized, w.Code, authorization)
		assert.Contains(t, w.Header().Get(WWWAuthenticateHeader), "Bearer", authorization)
	}
}

func TestValidateJWT_AcceptsThePanelsToken(t *testing.T) {
	ts, err := services.NewTokenService()
	require.NoError(t, err)
	token, err := ts.GenerateRequest(nil)
	require.NoError(t, err)

	w := validateJWTWith("Bearer " + token)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
