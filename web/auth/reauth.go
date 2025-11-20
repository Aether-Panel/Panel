package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/SkyPanel/SkyPanel/v3/models"
)

func Reauth(c *gin.Context) {
	user, _ := c.MustGet("user").(*models.User)

	createSession(c, user)

}
