package githubdl

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	repository := cast.ToString(op.OperationArgs["repository"])
	assetMatch := cast.ToString(op.OperationArgs["assetMatch"])
	outputVariable := cast.ToString(op.OperationArgs["outputVariable"])
	return &GithubDl{Repository: repository, AssetMatch: assetMatch, OutputVariable: outputVariable}, nil
}

func (of OperationFactory) Key() string {
	return "githubdl"
}

var Factory OperationFactory
