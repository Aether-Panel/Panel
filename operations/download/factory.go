package download

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	files := cast.ToStringSlice(op.OperationArgs["files"])
	return &Download{Files: files}, nil
}

func (of OperationFactory) Key() string {
	return "download"
}

var Factory OperationFactory
