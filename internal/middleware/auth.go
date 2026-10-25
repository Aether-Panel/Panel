package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var noLogin = []string{"/auth/", "/error/", "/api/config"}
var overrideRequireLogin = []string{"/auth/reauth", "/auth/logout"}

const WWWAuthenticateHeader = "WWW-Authenticate"
const WWWAuthenticateHeaderContents = "Bearer realm=\"\""

func AuthMiddleware(c *gin.Context) {
	if skipsLogin(c.Request.URL.Path) {
		return
	}

	db, err := database.GetConnection()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	ss := services.Session{DB: db}

	token, fromCookie, ok := readAuthToken(c)
	if !ok {
		return
	}

	// pull user from the session
	sess, err := ss.Validate(token)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		abortUnauthorized(c)
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
	// services.Session. Skip when the token came from an Authorization header
	// (background/API clients manage their own token lifetime).
	if fromCookie {
		maxAge := int(services.SessionLength / time.Second)
		SetAuthCookie(c, token, maxAge)
	}
}

// skipsLogin reports whether the path is public, unless it is one of the overrides
func skipsLogin(path string) bool {
	for _, v := range noLogin {
		if strings.HasPrefix(path, v) {
			// and now we see if it's actually one we override
			skip := false
			for _, o := range overrideRequireLogin {
				if o == path {
					skip = true
					break
				}
			}
			if !skip {
				return true
			}
		}
	}
	return false
}

// readAuthToken pulls the token from the Authorization header, falling back to the cookie.
// ok is false when the request has already been aborted.
func readAuthToken(c *gin.Context) (token string, fromCookie bool, ok bool) {
	var err error

	// order of priority, use auth headers first
	// check for token Auth header
	authHeader := c.Request.Header.Get("Authorization")
	authHeader = strings.TrimSpace(authHeader)

	if authHeader == "" {
		token, err = c.Cookie("skypanel_auth")
		fromCookie = true

		if errors.Is(err, http.ErrNoCookie) || token == "" {
			abortUnauthorized(c)
			return "", fromCookie, false
		}
	} else {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 {
			abortUnauthorized(c)
			return "", fromCookie, false
		}

		if parts[0] != "Bearer" || parts[1] == "" {
			abortUnauthorized(c)
			return "", fromCookie, false
		}

		token = parts[1]
	}

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return "", fromCookie, false
	}
	return token, fromCookie, true
}

func abortUnauthorized(c *gin.Context) {
	c.Header(WWWAuthenticateHeader, WWWAuthenticateHeaderContents)
	c.AbortWithStatus(http.StatusUnauthorized)
}

// SetAuthCookie writes the session cookie. SameSite=Lax keeps browsers from attaching it
// to requests started by other sites, on top of HttpOnly and Secure.
func SetAuthCookie(c *gin.Context, value string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("skypanel_auth", value, maxAge, "/", "", true, true)
}
