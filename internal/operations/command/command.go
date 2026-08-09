package command

import (
	"errors"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/logging"
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
			Callback: func(exitCode int) {
				if exitCode != 0 {
					ch <- errors.New("failed to run command")
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
