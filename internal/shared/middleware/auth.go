package middleware

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/shared/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var noLogin = []string{"/auth/", "/error/", "/api/config"}
var overrideRequireLogin = []string{"/auth/reauth", "/auth/logout"}

const WWWAuthenticateHeader = "WWW-Authenticate"
const WWWAuthenticateHeaderContents = "Bearer realm=\"\""

func AuthMiddleware(c *gin.Context) {
	for _, v := range noLogin {
		if strings.HasPrefix(c.Request.URL.Path, v) {
			// and now we see if it's actually one we override
			skip := false
			for _, o := range overrideRequireLogin {
				if o == c.Request.URL.Path {
					skip = true
					break
				}
			}
			if !skip {
				return
			}
		}
	}

	db, err := database.GetConnection()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	ss := session.SessionRepo{DB: db}

	var token string
	fromCookie := false

	// order of priority, use auth headers first
	// check for token Auth header
	authHeader := c.Request.Header.Get("Authorization")
	authHeader = strings.TrimSpace(authHeader)

	if authHeader == "" {
		token, err = c.Cookie("puffer_auth")
		fromCookie = true

		if errors.Is(err, http.ErrNoCookie) || token == "" {
			c.Header(WWWAuthenticateHeader, WWWAuthenticateHeaderContents)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	} else {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 {
			c.Header(WWWAuthenticateHeader, WWWAuthenticateHeaderContents)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if parts[0] != "Bearer" || parts[1] == "" {
			c.Header(WWWAuthenticateHeader, WWWAuthenticateHeaderContents)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		token = parts[1]
	}

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// pull user from the session
	sess, err := ss.Validate(token)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.Header(WWWAuthenticateHeader, WWWAuthenticateHeaderContents)
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if sess.UserID != nil {
		c.Set("user", &sess.User)
	}
	if sess.ClientID != nil {
		c.Set("client", &sess.Client)
	}

	// Sliding session: refresh the browser cookie on activity so it stays alive
	// as long as the user is active. The DB expiration is renewed in
	// session.SessionRepo. Skip when the token came from an Authorization header
	// (background/API clients manage their own token lifetime).
	if fromCookie {
		maxAge := int(services.SessionLength / time.Second)
		c.SetCookie("puffer_auth", token, maxAge, "/", "", isSecureRequest(c), true)
	}
}

// isSecureRequest reports whether the request reached the panel over a secure
// connection (direct TLS or a trusted reverse proxy declaring HTTPS). Mirrors
// the logic used when the auth cookie is first set at login so the cookie's
// Secure flag stays consistent on every sliding refresh.
func isSecureRequest(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}

	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil || !isTrustedProxy(ip) {
		return false
	}

	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

// isTrustedProxy reports whether ip matches one of the configured trusted
// reverse proxy addresses or CIDRs (security.trustedProxies).
func isTrustedProxy(ip net.IP) bool {
	for _, entry := range config.SecurityTrustedProxies.Value() {
		if _, cidr, err := net.ParseCIDR(entry); err == nil {
			if cidr.Contains(ip) {
				return true
			}
		} else if trusted := net.ParseIP(entry); trusted != nil {
			if trusted.Equal(ip) {
				return true
			}
		}
	}
	return false
}
