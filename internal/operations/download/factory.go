package download

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	files := cast.ToStringSlice(op.OperationArgs["files"])
	return &Download{Files: files}, nil
}

func (of OperationFactory) Key() string {
	return "download"
}

var Factory OperationFactory
