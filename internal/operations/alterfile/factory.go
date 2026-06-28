package alterfile

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	file := cast.ToString(op.OperationArgs["file"])
	search := cast.ToString(op.OperationArgs["search"])
	replace := cast.ToString(op.OperationArgs["replace"])
	regex := cast.ToBool(op.OperationArgs["regex"])
	return AlterFile{TargetFile: file, Search: search, Replace: replace, Regex: regex}, nil
}

func (of OperationFactory) Key() string {
	return "alterfile"
}

var Factory OperationFactory
