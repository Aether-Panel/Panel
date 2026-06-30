package auth

import (
	"context"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
)

// @Summary Get public JWKS
// @Description Gets the public JSON Web Key Set
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Auth
// @Router /auth/publickey [get]
func TokenServiceGetPublicKey(c *gin.Context) {
	ts, err := services.NewTokenService()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	rawJWKS, err := ts.GetTokenStore().JSONPublic(context.Background())
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, rawJWKS)
}
