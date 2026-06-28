package mkdir

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	target := cast.ToString(op.OperationArgs["target"])
	return &Mkdir{TargetFile: target}, nil
}

func (of OperationFactory) Key() string {
	return "mkdir"
}

var Factory OperationFactory
