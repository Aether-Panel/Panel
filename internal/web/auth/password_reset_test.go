package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func resetRequestFrom(host string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/auth/forgot-password", nil)
	c.Request.Host = host
	c.Request.Header.Set("X-Forwarded-Host", host)
	return c
}

func TestPanelBaseURL_IgnoresRequestHostWhenConfigured(t *testing.T) {
	_ = config.MasterURL.Set("https://panel.example.com/", false)
	t.Cleanup(func() { _ = config.MasterURL.Set("", false) })

	assert.Equal(t, "https://panel.example.com", panelBaseURL(resetRequestFrom("evil.example")))
}
