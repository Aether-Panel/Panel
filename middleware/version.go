package middleware

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
)

func AddVersionHeader(c *gin.Context) {
	c.Header("X-API-Version", SkyPanel.Version)
}
