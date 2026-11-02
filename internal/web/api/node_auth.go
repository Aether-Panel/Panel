package api

import (
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
)

// nodeRefusedPanel reports whether a node's answer is the node refusing the panel itself, not
// an answer about what the user asked for. A node checks only the panel's token (the panel has
// already checked the user's permissions), so a 401, or a 403 that carries an authentication
// challenge, means the two do not trust each other: another secret, a clock that is off, or a
// proxy in front of the node asking for credentials. Any other 403 is the node's own answer.
func nodeRefusedPanel(res *http.Response) bool {
	if res == nil {
		return false
	}
	switch res.StatusCode {
	case http.StatusUnauthorized:
		return true
	case http.StatusForbidden:
		return res.Header.Get(middleware.WWWAuthenticateHeader) != ""
	}
	return false
}

// answerNodeAuthFailure answers 502 with ErrNodeAuthFailed when the node refused the panel's
// credentials, and reports whether it did. Passing the node's 401 on would tell the browser
// that its own session ended, and the panel would send the user to the login page again and
// again while the problem is between the panel and the node.
func answerNodeAuthFailure(c *gin.Context, node *models.Node, res *http.Response) bool {
	if !nodeRefusedPanel(res) {
		return false
	}
	name := ""
	if node != nil {
		name = node.Name
	}
	logging.Error.Printf("Node %q refused the panel's token (status %d): check that its daemon.auth.clientSecret is the node's secret in the panel and that both clocks are right", name, res.StatusCode)
	c.AbortWithStatusJSON(http.StatusBadGateway, &skypanel.ErrorResponse{Error: skypanel.ErrNodeAuthFailed})
	return true
}
