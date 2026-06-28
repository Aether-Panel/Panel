package sleep

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	duration, err := cast.ToDurationE(op.OperationArgs["duration"])
	if err != nil {
		return nil, err
	}
	return &Sleep{Duration: duration}, nil
}

func (of OperationFactory) Key() string {
	return "sleep"
}

var Factory OperationFactory
