package console

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	message := cast.ToString(op.OperationArgs["message"])
	return &Console{Text: message}, nil
}

func (of OperationFactory) Key() string {
	return "console"
}

var Factory OperationFactory
