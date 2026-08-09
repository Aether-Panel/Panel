package tty

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type EnvironmentFactory struct {
	skypanel.EnvironmentFactory
}

func (ef EnvironmentFactory) Create() skypanel.EnvironmentImpl {
	return &tty{}
}

func (ef EnvironmentFactory) Key() string {
	return "host"
}
