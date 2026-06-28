package auth

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/gin-gonic/gin"
)

// @Summary Reauthenticate user
// @Description Reauthenticates a user and refreshes their session
// @Produce json
// @Success 200 {object} LoginResponse
// @Failure 401 {object} skypanel.ErrorResponse
// @Tags Auth
// @Router /auth/reauth [post]
func Reauth(c *gin.Context) {
	user, _ := c.MustGet("user").(*models.User)

	createSession(c, user)

}
