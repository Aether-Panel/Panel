package docker

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type EnvironmentFactory struct {
	skypanel.EnvironmentFactory
}

func (ef EnvironmentFactory) Create() skypanel.EnvironmentImpl {
	return &Docker{
		// Docker needs image references in lowercase: the old default "SkyPanel/generic" was
		// read as a private registry and failed to resolve
		ImageName: "ghcr.io/aether-panel/generic",
		// "bridge" maps the ports (0.0.0.0:PORT->PORT/tcp); "host" shares the host's network
		// and breaks that mapping on a VPS
		Network: "bridge",
		Ports:   make([]string, 0),
		Binds:   make(map[string]string),
		Labels:  make(map[string]string),
	}
}

func (ef EnvironmentFactory) Key() string {
	return "docker"
}
