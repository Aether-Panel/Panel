package api

import (
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
)

// nodeRefusedPanel reports whether a node's answer is the node refusing the panel itself, not
// an answer about what the user asked for (see services.NodeRefusedPanel, which the node
// settings sync shares).
func nodeRefusedPanel(res *http.Response) bool {
	return services.NodeRefusedPanel(res)
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
	logging.Error.Printf("Node %q refused the panel's token (status %d): check that it reaches the panel's address to read its key (token.public), that its daemon.auth.clientSecret is the node's secret in the panel and that both clocks are right", name, res.StatusCode)
	c.AbortWithStatusJSON(http.StatusBadGateway, &skypanel.ErrorResponse{Error: skypanel.ErrNodeAuthFailed})
	return true
}
