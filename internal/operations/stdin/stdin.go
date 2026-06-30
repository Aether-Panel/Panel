package stdin

import "github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"

type Stdin struct {
	Command string
}

func (d Stdin) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	running, err := env.IsRunning()
	if err != nil {
		return skypanel.OperationResult{Error: err}
	} else if !running {
		return skypanel.OperationResult{Error: skypanel.ErrServerOffline}
	}

	err = env.ExecuteInMainProcess(d.Command)
	return skypanel.OperationResult{Error: err}
}
