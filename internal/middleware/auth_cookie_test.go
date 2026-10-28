package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func authCookieFor(t *testing.T, prepare func(*http.Request)) *http.Cookie {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	prepare(c.Request)

	SetAuthCookie(c, "token", 3600)

	cookies := w.Result().Cookies()
	if !assert.Len(t, cookies, 1) {
		t.FailNow()
	}
	return cookies[0]
}

func TestSetAuthCookie_PlainHTTPIsNotSecure(t *testing.T) {
	cookie := authCookieFor(t, func(*http.Request) {})

	assert.False(t, cookie.Secure, "a browser drops a Secure cookie on http://IP:8080 and sign-in loops")
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetAuthCookie_SecureOverHTTPS(t *testing.T) {
	assert.True(t, authCookieFor(t, func(r *http.Request) { r.TLS = &tls.ConnectionState{} }).Secure)
}

func TestSetAuthCookie_SecureBehindATLSProxy(t *testing.T) {
	assert.True(t, authCookieFor(t, func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "https") }).Secure)
}
