package writefile

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	text := cast.ToString(op.OperationArgs["text"])
	target := cast.ToString(op.OperationArgs["target"])
	return WriteFile{TargetFile: target, Text: text}, nil
}

func (of OperationFactory) Key() string {
	return "writefile"
}

var Factory OperationFactory
