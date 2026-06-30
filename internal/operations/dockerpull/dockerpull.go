package dockerpull

import (
	"context"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers/docker"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type DockerPull struct {
	ImageName string
}

func (d DockerPull) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment
	dockerEnv, ok := env.Implementation.(*docker.Docker)

	if !ok {
		return skypanel.OperationResult{Error: skypanel.ErrEnvironmentNotSupported}
	}

	err := dockerEnv.PullImage(context.Background(), env, d.ImageName, true)
	return skypanel.OperationResult{Error: err}
}
