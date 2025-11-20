package command

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/utils"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	cmds := cast.ToStringSlice(op.OperationArgs["commands"])

	var stdIn SkyPanel.StdinConsoleConfiguration
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
