package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

func AddVersionHeader(c *gin.Context) {
	c.Header("X-API-Version", SkyPanel.Version)
}
