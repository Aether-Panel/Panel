package auth

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
)

// The reset link is never built from request headers (Host, X-Forwarded-Host), which the
// requester controls: it always uses the configured panel URL.
func TestPanelBaseURL_UsesTheConfiguredURL(t *testing.T) {
	previous := config.MasterURL.Value()
	_ = config.MasterURL.Set("https://panel.example.com/", false)
	t.Cleanup(func() { _ = config.MasterURL.Set(previous, false) })

	assert.Equal(t, "https://panel.example.com", panelBaseURL())
}
