package scraperdl

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	url := cast.ToString(op.OperationArgs["url"])
	regex := cast.ToString(op.OperationArgs["regex"])
	downloadUrl := cast.ToString(op.OperationArgs["downloadUrl"])
	outputVariable := cast.ToString(op.OperationArgs["outputVariable"])
	return &ScraperDl{URL: url, Regex: regex, DownloadURL: downloadUrl, OutputVariable: outputVariable}, nil
}

func (of OperationFactory) Key() string {
	return "scraperdl"
}

var Factory OperationFactory
