package dockerpull

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	imageName := cast.ToString(op.OperationArgs["imageName"])
	return &DockerPull{ImageName: imageName}, nil
}

func (of OperationFactory) Key() string {
	return "dockerpull"
}

var Factory OperationFactory
