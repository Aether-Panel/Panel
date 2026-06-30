package middleware

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/oauth2"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func ValidateJWT(c *gin.Context) {
	ts, err := services.NewTokenService()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	authHeader := c.GetHeader("Authorization")

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.Header("WWW-Authenticate", "Bearer")
		c.AbortWithStatusJSON(http.StatusUnauthorized, &oauth2.ErrorResponse{Error: "invalid_request"})
		return
	}
	token := parts[1]

	err = ts.ValidateRequest(token)
	// if decryption failed, the request wasn't valid
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
}
