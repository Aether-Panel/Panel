package command

import (
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type Command struct {
	Commands  []string
	Env       map[string]string
	StdIn     skypanel.StdinConsoleConfiguration
	Variables map[string]interface{}
}

func (c Command) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	for _, cmd := range c.Commands {
		logging.Info.Printf("Executing command: %s", cmd)
		env.DisplayToConsole(true, fmt.Sprintf("Executing: %s\n", cmd))
		ch := make(chan error, 1)
		err := env.Execute(skypanel.ExecutionData{
			Command:     cmd,
			Environment: c.Env,
			// exactly one value: a second send blocked the exit handling for good, so the
			// server stayed "installing" and could not be deleted, reinstalled or started
			Callback: func(exitCode int) {
				if exitCode != 0 {
					ch <- fmt.Errorf("failed to run command (exit code %d)", exitCode)
					return
				}
				ch <- nil
			},
			StdInConfig:  c.StdIn,
			Variables:    c.Variables,
			DisableStats: true,
		})
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
		err = <-ch
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
	}

	return skypanel.OperationResult{Error: nil}
}
