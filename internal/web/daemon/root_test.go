package daemon

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
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
