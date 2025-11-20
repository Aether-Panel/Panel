package fabricdl

import "github.com/SkyPanel/SkyPanel/v3"

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	return &Fabricdl{}, nil
}

func (of OperationFactory) Key() string {
	return "fabricdl"
}

var Factory OperationFactory
