package auth

import (
	"net/http"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
)

// @Summary Sign in with a billing system link
// @Description Opens a session from a one-time link the billing system asked for with
// @Description POST /api/v1/sso, then sends the browser on to the panel. A used, expired or
// @Description unknown link goes to the login page instead, and so does one for an account
// @Description that may not sign in.
// @Param token query string true "One-time token"
// @Success 303 {object} nil
// @Tags Auth
// @Router /auth/sso [get]
func SSOGet(c *gin.Context) {
	// the token is in the address; keep it out of the Referer of whatever loads next
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Cache-Control", "no-store")

	userID, redirect, ok := services.SSOLinks.Consume(c.Query("token"))
	if !ok {
		c.Redirect(http.StatusSeeOther, "/login/?sso=expired")
		return
	}

	db := middleware.GetDatabase(c)
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.Redirect(http.StatusSeeOther, "/login/?sso=expired")
		return
	}

	// openSession would refuse too, but with a JSON error the browser shows as a bare page
	ps := &services.Permission{DB: db}
	if allowed, err := ps.MaySignIn(user.ID); err != nil || !allowed {
		c.Redirect(http.StatusSeeOther, "/login/?sso=denied")
		return
	}

	if _, ok := openSession(c, &user, models.ActivityLoginSSO); !ok {
		return
	}
	c.Redirect(http.StatusSeeOther, safeRedirect(redirect))
}

// safeRedirect keeps the destination inside the panel: only a path is accepted
func safeRedirect(path string) string {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.Contains(path, "\\") {
		return "/"
	}
	return path
}
