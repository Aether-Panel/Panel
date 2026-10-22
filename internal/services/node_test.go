package services

import (
	"net/http"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCallNode_DoesNotForwardTheUsersCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := skypanel.Engine
	t.Cleanup(func() { skypanel.Engine = previous })

	var seen http.Header
	engine := gin.New()
	engine.GET("/daemon/echo", func(c *gin.Context) {
		seen = c.Request.Header.Clone()
		c.Status(http.StatusOK)
	})
	skypanel.Engine = engine

	browser := http.Header{}
	browser.Set("Cookie", "skypanel_auth=user-session")
	browser.Set("Range", "bytes=0-10")

	ns := &Node{}
	res, err := ns.CallNode(models.LocalNode, "GET", "/daemon/echo", nil, browser)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, http.StatusOK, res.StatusCode)

	assert.Empty(t, seen.Get("Cookie"), "the session cookie must not reach the node")
	assert.Equal(t, "bytes=0-10", seen.Get("Range"), "other headers still pass through")
	assert.Contains(t, seen.Get("Authorization"), "Bearer ", "the node gets the panel's token")
	assert.Equal(t, "skypanel_auth=user-session", browser.Get("Cookie"), "the caller's headers are not modified")
	assert.Empty(t, browser.Get("Authorization"))
}
