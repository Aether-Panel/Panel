package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestUptime_GatusProxyMock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	router.GET("/api/uptime/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "UP", 
			"components": []string{"Node1", "Node2"},
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/uptime/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "UP")
}
