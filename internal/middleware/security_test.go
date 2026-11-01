package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func guardedRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders, SameOriginGuard)
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.POST("/api/users", ok)
	r.GET("/api/servers/x/socket", ok)
	return r
}

func TestSameOriginGuard(t *testing.T) {
	cookie := &http.Cookie{Name: "skypanel_auth", Value: "token"}
	cases := map[string]struct {
		method  string
		path    string
		cookie  bool
		headers map[string]string
		want    int
	}{
		"form posted from a game server on the same IP": {http.MethodPost, "/api/users", true,
			map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "http://203.0.113.5:25566"}, http.StatusForbidden},
		"form posted from another site": {http.MethodPost, "/api/users", true,
			map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}, http.StatusForbidden},
		"older browser, Origin of another port": {http.MethodPost, "/api/users", true,
			map[string]string{"Origin": "http://203.0.113.5:25566"}, http.StatusForbidden},
		"sandboxed frame": {http.MethodPost, "/api/users", true,
			map[string]string{"Origin": "null"}, http.StatusForbidden},
		"websocket opened by another site": {http.MethodGet, "/api/servers/x/socket", true,
			map[string]string{"Upgrade": "websocket", "Sec-Fetch-Site": "same-site"}, http.StatusForbidden},
		"the panel itself": {http.MethodPost, "/api/users", true,
			map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://203.0.113.5:8080"}, http.StatusOK},
		"the panel's own websocket": {http.MethodGet, "/api/servers/x/socket", true,
			map[string]string{"Upgrade": "websocket", "Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		"older browser, same origin": {http.MethodPost, "/api/users", true,
			map[string]string{"Origin": "http://203.0.113.5:8080"}, http.StatusOK},
		"API client without cookies": {http.MethodPost, "/api/users", false,
			map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://tool.example"}, http.StatusOK},
		"script or curl with the cookie": {http.MethodPost, "/api/users", true, nil, http.StatusOK},
	}
	r := guardedRouter()
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://203.0.113.5:8080"+tc.path, nil)
			if tc.cookie {
				req.AddCookie(cookie)
			}
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tc.want, w.Code)
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	r := guardedRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	h := w.Header()
	assert.Contains(t, h.Get("Content-Security-Policy"), "frame-ancestors 'self'")
	assert.Equal(t, "SAMEORIGIN", h.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
	assert.Empty(t, h.Get("Strict-Transport-Security"), "no HSTS over plain HTTP")

	req = httptest.NewRequest(http.MethodPost, "/api/users", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.NotEmpty(t, w.Header().Get("Strict-Transport-Security"))
}

func TestRedactedLogFormatter(t *testing.T) {
	line := RedactedLogFormatter(gin.LogFormatterParams{
		Path:   "/api/map-view/abc123/eyJhbGciOi.secret/index.html",
		Method: http.MethodGet,
	})
	assert.NotContains(t, line, "eyJhbGciOi")
	assert.Contains(t, line, "/api/map-view/abc123/***/index.html")

	line = RedactedLogFormatter(gin.LogFormatterParams{Path: "/auth/sso?token=s3cr3t&next=/servers", Method: http.MethodGet})
	assert.NotContains(t, line, "s3cr3t")
	assert.Contains(t, line, "next=/servers")
}
