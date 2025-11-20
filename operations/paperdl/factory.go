package paperdl

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
	build := cast.ToString(op.OperationArgs["build"])
	filename := cast.ToString(op.OperationArgs["target"])

	if minecraftVersion == "" {
		return nil, errors.New("missing minecraftVersion")
	}

	if build == "" {
		return nil, errors.New("missing build")
	}

	return PaperDl{MinecraftVersion: minecraftVersion, Build: build, Filename: filename}, nil
}

func (of OperationFactory) Key() string {
	return "paperdl"
}

var Factory OperationFactory
