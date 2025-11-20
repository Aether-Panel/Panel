package extract

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Key() string {
	return "extract"
}
func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	source := cast.ToString(op.OperationArgs["source"])
	destination := cast.ToString(op.OperationArgs["destination"])

	return Extract{
		Source:      source,
		Destination: destination,
	}, nil
}

var Factory OperationFactory
