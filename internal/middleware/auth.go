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

	ss := services.Session{DB: db}

	var token string
	fromCookie := false

	// order of priority, use auth headers first
	// check for token Auth header
	authHeader := c.Request.Header.Get("Authorization")
	authHeader = strings.TrimSpace(authHeader)

	if authHeader == "" {
		token, err = c.Cookie("skypanel_auth")
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
	// services.Session. Skip when the token came from an Authorization header
	// (background/API clients manage their own token lifetime).
	if fromCookie {
		maxAge := int(services.SessionLength / time.Second)
		c.SetCookie("skypanel_auth", token, maxAge, "/", "", true, true)
	}
}
