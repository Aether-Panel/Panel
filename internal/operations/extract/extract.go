package extract

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type Extract struct {
	Source      string
	Destination string
}

func (op Extract) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	err := args.Server.Extract(op.Source, op.Destination)
	return skypanel.OperationResult{Error: err}
}
