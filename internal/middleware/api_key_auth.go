package middleware

import (
	"net/http"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
)

func APIKeyAuthMiddleware(c *gin.Context) {
	token := c.Request.Header.Get("X-Api-Key")
	if token == "" {
		authHeader := strings.TrimSpace(c.Request.Header.Get("Authorization"))
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if token == "" || !strings.HasPrefix(token, "ak_") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized - Missing or invalid API Key format"})
		return
	}

	db, err := database.GetConnection()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	keyService := &services.APIKeyService{DB: db}
	apiKey, err := keyService.ValidateKey(token)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API Key"})
		return
	}

	// Set the api key in context so handlers can access it and check permissions
	c.Set("api_key", apiKey)
	c.Next()
}
