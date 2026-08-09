package paperdl

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	minecraftVersion := cast.ToString(op.OperationArgs["minecraftVersion"])
	build := cast.ToString(op.OperationArgs["build"])
	filename := cast.ToString(op.OperationArgs["target"])
	project := cast.ToString(op.OperationArgs["project"])

	if minecraftVersion == "" {
		return nil, errors.New("missing minecraftVersion")
	}

	if build == "" {
		return nil, errors.New("missing build")
	}

	return PaperDl{MinecraftVersion: minecraftVersion, Build: build, Filename: filename, Project: project}, nil
}

func (of OperationFactory) Key() string {
	return "paperdl"
}

var Factory OperationFactory
