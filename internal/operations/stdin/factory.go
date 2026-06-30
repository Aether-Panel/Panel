package stdin

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	message := cast.ToString(op.OperationArgs["command"])
	return &Stdin{Command: message}, nil
}

func (of OperationFactory) Key() string {
	return "stdin"
}

var Factory OperationFactory
