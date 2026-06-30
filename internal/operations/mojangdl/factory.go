package mojangdl

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	version := cast.ToString(op.OperationArgs["version"])
	target := cast.ToString(op.OperationArgs["target"])

	return MojangDl{Version: version, Target: target}, nil
}

func (of OperationFactory) Key() string {
	return "mojangdl"
}

var Factory OperationFactory
