package command

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	cmds := cast.ToStringSlice(op.OperationArgs["commands"])

	var stdIn skypanel.StdinConsoleConfiguration
	if field, exists := op.OperationArgs["stdin"]; exists {
		err := utils.UnmarshalTo(field, stdIn)
		if err != nil {
			return nil, err
		}
	}

	return Command{Commands: cmds, Env: op.EnvironmentVariables, StdIn: stdIn, Variables: op.DataMap}, nil
}

func (of OperationFactory) Key() string {
	return "command"
}

var Factory OperationFactory
