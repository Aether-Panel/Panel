package dockerpull

import (
	"context"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/servers/docker"
)

type DockerPull struct {
	ImageName string
}

func (d DockerPull) Run(args SkyPanel.RunOperatorArgs) SkyPanel.OperationResult {
	env := args.Environment
	dockerEnv, ok := env.Implementation.(*docker.Docker)

	if !ok {
		return SkyPanel.OperationResult{Error: SkyPanel.ErrEnvironmentNotSupported}
	}

	err := dockerEnv.PullImage(env, context.Background(), d.ImageName, true)
	return SkyPanel.OperationResult{Error: err}
}
