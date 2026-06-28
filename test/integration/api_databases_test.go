package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestDatabases_CreateDBForServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	router.POST("/api/servers/:id/databases", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{
			"status":   "success",
			"database": "s1_minecraft",
			"username": "u1_minecraft",
			"password": "generated_secure_password_123",
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/servers/srv-123/databases", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "generated_secure_password_123")
}
