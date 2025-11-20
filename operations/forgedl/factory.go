package forgedl

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	minecraftVersion := cast.ToString(op.OperationArgs["minecraftVersion"])
	version := cast.ToString(op.OperationArgs["version"])
	filename := cast.ToString(op.OperationArgs["target"])
	outputVariable := cast.ToString(op.OperationArgs["outputVariable"])

	if version == "" && minecraftVersion == "" {
		return nil, errors.New("missing version and minecraftVersion")
	}

	if outputVariable == "" {
		outputVariable = "opForgeVersion"
	}

	return ForgeDl{Version: version, Filename: filename, MinecraftVersion: minecraftVersion, OutputVariable: outputVariable}, nil
}

func (of OperationFactory) Key() string {
	return "forgedl"
}

var Factory OperationFactory
