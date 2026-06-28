package auth

import (
	"errors"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
)

// @Summary User Logout
// @Description Invalidates the current user session
// @Produce json
// @Success 204 {string} string ""
// @Tags Auth
// @Router /auth/logout [post]
func LogoutPost(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ss := services.Session{DB: db}

	var err error
	cookie := c.Query("token")
	if cookie == "" {
		cookie, err = c.Cookie("puffer_auth")
		if errors.Is(err, http.ErrNoCookie) {
			c.Status(http.StatusNoContent)
			return
		}
	}

	_ = ss.Expire(cookie)

	secure := false
	if c.Request.TLS != nil {
		secure = true
	}

	c.SetCookie("puffer_auth", "", 0, "/", "", secure, true)
	c.SetCookie("puffer_auth_expires", "", 0, "/", "", secure, false)
	c.Status(http.StatusNoContent)
}
