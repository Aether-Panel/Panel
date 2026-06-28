package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestProvision_CheckAvailableNodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Dummy endpoint for provisioning check
	router.GET("/api/provision", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success", "available_nodes": 3, "recommended_node": "node-1"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/provision", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "recommended_node")
}
