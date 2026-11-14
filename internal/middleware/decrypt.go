package middleware

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
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
		c.Header(WWWAuthenticateHeader, "Bearer")
		c.AbortWithStatusJSON(http.StatusUnauthorized, &oauth2.ErrorResponse{Error: "invalid_request"})
		return
	}
	token := parts[1]

	// a token that does not check out (another panel's key, minted for another node, expired
	// by a clock that is off) is a failed sign-in like a missing one: 401, not 400. The panel
	// reports a node's 401 as its own ErrNodeAuthFailed, so it does not look like the user's
	// session ended.
	if err = ts.ValidateRequest(token); err != nil {
		logging.Error.Printf("Refused a request with an invalid panel token: %s", err)
		c.Header(WWWAuthenticateHeader, `Bearer error="invalid_token"`)
		c.AbortWithStatusJSON(http.StatusUnauthorized, &oauth2.ErrorResponse{Error: "invalid_token"})
		return
	}
}
