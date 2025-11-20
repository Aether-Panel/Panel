package stdin

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	message := cast.ToString(op.OperationArgs["command"])
	return &Stdin{Command: message}, nil
}

func (of OperationFactory) Key() string {
	return "stdin"
}

var Factory OperationFactory
