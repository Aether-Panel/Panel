package tty

import (
	"github.com/SkyPanel/SkyPanel/v3"
)

type EnvironmentFactory struct {
	SkyPanel.EnvironmentFactory
}

func (ef EnvironmentFactory) Create() SkyPanel.EnvironmentImpl {
	return &tty{}
}

func (ef EnvironmentFactory) Key() string {
	return "host"
}
