package daemon

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCheckEnvironmentAllowed(t *testing.T) {
	previous := config.DockerDisallowHost.Value()
	t.Cleanup(func() { _ = config.DockerDisallowHost.Set(previous, false) })

	_ = config.DockerDisallowHost.Set(false, false)
	assert.NoError(t, checkEnvironmentAllowed("host"), "host environments stay available unless disallowHost is set")

	_ = config.DockerDisallowHost.Set(true, false)
	for _, env := range []string{"host", "tty", "standard", "bubblewrap"} {
		assert.Error(t, checkEnvironmentAllowed(env), "%q must be rejected with disallowHost", env)
	}
	assert.NoError(t, checkEnvironmentAllowed("docker"))
}

// The panel's uptime poller reads an empty object, not null, from a node without servers
func TestServerStatuses_NoServers(t *testing.T) {
	statuses := serverStatuses(nil)

	assert.NotNil(t, statuses)
	assert.Empty(t, statuses)
}

// The panel's status poll carries its notification language, which this node then uses
func TestGetServersStatus_TakesThePanelsNotificationLanguage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { services.SetPanelNotificationLanguage(services.DefaultNotificationLanguage) })

	poll := func(language string) int {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/daemon/servers/status", nil)
		if language != "" {
			c.Request.Header.Set(services.NotificationLanguageHeader, language)
		}
		getServersStatus(c)
		return w.Code
	}

	assert.Equal(t, http.StatusOK, poll("en"))
	assert.Equal(t, "en", services.PanelNotificationLanguage())

	assert.Equal(t, http.StatusOK, poll(""), "an older panel sends no language")
	assert.Equal(t, "en", services.PanelNotificationLanguage(), "and the node keeps the last one")

	poll("fr")
	assert.Equal(t, "en", services.PanelNotificationLanguage(), "an unknown language is ignored")
}
