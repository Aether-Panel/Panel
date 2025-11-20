package archive

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Key() string {
	return "archive"
}
func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	source := cast.ToStringSlice(op.OperationArgs["source"])
	destination := cast.ToString(op.OperationArgs["destination"])

	return Archive{
		Source:      source,
		Destination: destination,
	}, nil
}

var Factory OperationFactory
