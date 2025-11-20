package docker

import (
	"github.com/SkyPanel/SkyPanel/v3"
)

type EnvironmentFactory struct {
	SkyPanel.EnvironmentFactory
}

func (ef EnvironmentFactory) Create() SkyPanel.EnvironmentImpl {
	return &Docker{
		ImageName: "SkyPanel/generic",
		Network:   "host",
		Ports:     make([]string, 0),
		Binds:     make(map[string]string),
		Labels:    make(map[string]string),
	}
}

func (ef EnvironmentFactory) Key() string {
	return "docker"
}
