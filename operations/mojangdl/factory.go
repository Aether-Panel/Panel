package mojangdl

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	version := cast.ToString(op.OperationArgs["version"])
	target := cast.ToString(op.OperationArgs["target"])

	return MojangDl{Version: version, Target: target}, nil
}

func (of OperationFactory) Key() string {
	return "mojangdl"
}

var Factory OperationFactory
