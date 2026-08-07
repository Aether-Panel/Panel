package docker

import (
	"testing"

	network "github.com/moby/moby/api/types/network"
	"github.com/stretchr/testify/assert"
)

func TestParsePortSpecsDockerClassicFormat(t *testing.T) {
	// Templates use Docker's classic "[IP:]HOST_PORT:CONTAINER_PORT[/PROTO]" syntax
	// WITHOUT the "->" separator. This must produce a real port binding or the
	// container is unreachable from outside ("connection timed out: getsockopt").
	exposed, bindings, err := parsePortSpecs([]string{"0.0.0.0:25565:25565/tcp"})
	assert.NoError(t, err)

	containerPort := network.MustParsePort("25565/tcp")
	assert.Contains(t, exposed, containerPort)
	assert.Len(t, bindings, 1)

	binds := bindings[containerPort]
	if assert.Len(t, binds, 1) {
		assert.Equal(t, "25565", binds[0].HostPort)
		assert.Equal(t, "0.0.0.0", binds[0].HostIP.String())
	}
}

func TestParsePortSpecsDockerClassicFormatWithoutIP(t *testing.T) {
	// "HOST_PORT:CONTAINER_PORT[/PROTO]" without an explicit IP also binds.
	exposed, bindings, err := parsePortSpecs([]string{"25566:25566/udp"})
	assert.NoError(t, err)

	containerPort := network.MustParsePort("25566/udp")
	assert.Contains(t, exposed, containerPort)
	binds := bindings[containerPort]
	if assert.Len(t, binds, 1) {
		assert.Equal(t, "25566", binds[0].HostPort)
		// Zero-value HostIP means Docker binds to all interfaces.
		assert.False(t, binds[0].HostIP.IsValid())
	}
}

func TestParsePortSpecsLegacyArrowFormat(t *testing.T) {
	// The legacy PufferPanel "HOST->CONTAINER" format must keep working.
	exposed, bindings, err := parsePortSpecs([]string{"0.0.0.0:25567->25567/tcp"})
	assert.NoError(t, err)

	containerPort := network.MustParsePort("25567/tcp")
	assert.Contains(t, exposed, containerPort)
	binds := bindings[containerPort]
	if assert.Len(t, binds, 1) {
		assert.Equal(t, "25567", binds[0].HostPort)
		assert.Equal(t, "0.0.0.0", binds[0].HostIP.String())
	}
}

func TestParsePortSpecsExposeOnly(t *testing.T) {
	// A bare container port only exposes it, with no host binding.
	exposed, bindings, err := parsePortSpecs([]string{"25568/tcp"})
	assert.NoError(t, err)

	assert.Contains(t, exposed, network.MustParsePort("25568/tcp"))
	assert.Len(t, bindings, 0)
}

func TestParsePortSpecsMultiple(t *testing.T) {
	exposed, bindings, err := parsePortSpecs([]string{
		"0.0.0.0:25565:25565/tcp",
		"0.0.0.0:8123:8123/udp",
	})
	assert.NoError(t, err)

	assert.Contains(t, exposed, network.MustParsePort("25565/tcp"))
	assert.Contains(t, exposed, network.MustParsePort("8123/udp"))
	assert.Len(t, bindings, 2)
}
