package stdin

import "github.com/SkyPanel/SkyPanel/v3"

type Stdin struct {
	Command string
}

func (d Stdin) Run(args SkyPanel.RunOperatorArgs) SkyPanel.OperationResult {
	env := args.Environment

	running, err := env.IsRunning()
	if err != nil {
		return SkyPanel.OperationResult{Error: err}
	} else if !running {
		return SkyPanel.OperationResult{Error: SkyPanel.ErrServerOffline}
	}

	err = env.ExecuteInMainProcess(d.Command)
	return SkyPanel.OperationResult{Error: err}
}
