package javadl

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cast"
	"strconv"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Key() string {
	return "javadl"
}
func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	v := op.OperationArgs["version"]

	version, err := cast.ToIntE(v)
	if err != nil {
		return nil, err
	}

	return JavaDl{Version: strconv.Itoa(version)}, nil
}

var Factory OperationFactory
