package command

import (
	"errors"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/logging"
)

type Command struct {
	Commands  []string
	Env       map[string]string
	StdIn     SkyPanel.StdinConsoleConfiguration
	Variables map[string]interface{}
}

func (c Command) Run(args SkyPanel.RunOperatorArgs) SkyPanel.OperationResult {
	env := args.Environment

	for _, cmd := range c.Commands {
		logging.Info.Printf("Executing command: %s", cmd)
		env.DisplayToConsole(true, fmt.Sprintf("Executing: %s\n", cmd))
		ch := make(chan error, 1)
		err := env.Execute(SkyPanel.ExecutionData{
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
			return SkyPanel.OperationResult{Error: err}
		}
		err = <-ch
		if err != nil {
			return SkyPanel.OperationResult{Error: err}
		}
	}

	return SkyPanel.OperationResult{Error: nil}
}
