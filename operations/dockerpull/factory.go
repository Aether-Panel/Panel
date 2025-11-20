package dockerpull

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	imageName := cast.ToString(op.OperationArgs["imageName"])
	return &DockerPull{ImageName: imageName}, nil
}

func (of OperationFactory) Key() string {
	return "dockerpull"
}

var Factory OperationFactory
