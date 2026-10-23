package services

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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

func useEngine(t *testing.T, register func(e *gin.Engine)) {
	gin.SetMode(gin.TestMode)
	previous := skypanel.Engine
	t.Cleanup(func() { skypanel.Engine = previous })
	engine := gin.New()
	register(engine)
	skypanel.Engine = engine
}

func TestCallNode_LocalSmallResponseIsComplete(t *testing.T) {
	useEngine(t, func(e *gin.Engine) {
		e.GET("/daemon/small", func(c *gin.Context) {
			c.Header("X-Test", "yes")
			c.JSON(http.StatusCreated, gin.H{"ok": true})
		})
		e.DELETE("/daemon/empty", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	})
	ns := &Node{}

	res, err := ns.CallNode(models.LocalNode, "GET", "/daemon/small", nil, nil)
	if !assert.NoError(t, err) {
		return
	}
	body, _ := io.ReadAll(res.Body)
	assert.Equal(t, http.StatusCreated, res.StatusCode)
	assert.Equal(t, "yes", res.Header.Get("X-Test"))
	assert.JSONEq(t, `{"ok":true}`, string(body))

	res, err = ns.CallNode(models.LocalNode, "DELETE", "/daemon/empty", nil, nil)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusNoContent, res.StatusCode)
	}
}

func TestCallNode_LocalLargeResponseIsStreamed(t *testing.T) {
	release := make(chan struct{})
	chunk := bytes.Repeat([]byte("x"), 256*1024)
	useEngine(t, func(e *gin.Engine) {
		e.GET("/daemon/big", func(c *gin.Context) {
			c.Status(http.StatusOK)
			for i := 0; i < 8; i++ { // 2 MB, over the in-memory limit
				_, _ = c.Writer.Write(chunk)
			}
			<-release // the handler is still running when the caller gets the response
			_, _ = c.Writer.Write([]byte("end"))
		})
	})

	done := make(chan *http.Response, 1)
	go func() {
		res, _ := (&Node{}).CallNode(models.LocalNode, "GET", "/daemon/big", nil, nil)
		done <- res
	}()

	var res *http.Response
	select {
	case res = <-done:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("the response waited for the whole body, so it was buffered in memory")
	}
	close(release)

	body, err := io.ReadAll(res.Body)
	assert.NoError(t, err)
	assert.Equal(t, 8*len(chunk)+3, len(body))
	assert.True(t, strings.HasSuffix(string(body), "end"))
}

func TestCallNode_LocalStreamStopsWhenCallerCloses(t *testing.T) {
	finished := make(chan struct{})
	chunk := bytes.Repeat([]byte("x"), 512*1024)
	useEngine(t, func(e *gin.Engine) {
		e.GET("/daemon/endless", func(c *gin.Context) {
			defer close(finished)
			for {
				if _, err := c.Writer.Write(chunk); err != nil {
					return
				}
			}
		})
	})

	res, err := (&Node{}).CallNode(models.LocalNode, "GET", "/daemon/endless", nil, nil)
	if !assert.NoError(t, err) {
		return
	}
	_ = res.Body.Close()

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler kept running after the caller closed the body")
	}
}
