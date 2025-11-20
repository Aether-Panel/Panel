package mkdir

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	target := cast.ToString(op.OperationArgs["target"])
	return &Mkdir{TargetFile: target}, nil
}

func (of OperationFactory) Key() string {
	return "mkdir"
}

var Factory OperationFactory
