package resolveforgeversion

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	minecraftVersion := cast.ToString(op.OperationArgs["minecraftVersion"])
	version := cast.ToString(op.OperationArgs["version"])
	outputVariable := cast.ToString(op.OperationArgs["outputVariable"])

	if outputVariable == "" {
		outputVariable = "opForgeVersion"
	}

	return ResolveForgeVersion{Version: version, MinecraftVersion: minecraftVersion, OutputVariable: outputVariable}, nil
}

func (of OperationFactory) Key() string {
	return "resolveforgeversion"
}

var Factory OperationFactory
