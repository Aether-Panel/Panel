package auth

import (
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/gin-gonic/gin"
)

// @Summary Reauthenticate user
// @Description Reauthenticates a user and refreshes their session
// @Produce json
// @Success 200 {object} LoginResponse
// @Failure 401 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Tags Auth
// @Router /auth/reauth [post]
func Reauth(c *gin.Context) {
	// a client-credentials token must never mint a full browser session: that would
	// turn a leaked client secret into whole-account access that also skips 2FA
	if _, isClient := c.Get("client"); isClient {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	user, _ := c.MustGet("user").(*models.User)

	createSession(c, user)

}
