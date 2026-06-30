package fabricdl

import "github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(_ skypanel.CreateOperation) (skypanel.Operation, error) {
	return &Fabricdl{}, nil
}

func (of OperationFactory) Key() string {
	return "fabricdl"
}

var Factory OperationFactory
