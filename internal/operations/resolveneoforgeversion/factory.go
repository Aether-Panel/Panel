package resolveneoforgeversion

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	minecraftVersion := cast.ToString(op.OperationArgs["minecraftVersion"])
	version := cast.ToString(op.OperationArgs["version"])
	outputVariable := cast.ToString(op.OperationArgs["outputVariable"])

	if outputVariable == "" {
		outputVariable = "opNeoForgeVersion"
	}

	return ResolveNeoForgeVersion{Version: version, MinecraftVersion: minecraftVersion, OutputVariable: outputVariable}, nil
}

func (of OperationFactory) Key() string {
	return "resolveneoforgeversion"
}

var Factory OperationFactory
