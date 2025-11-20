package extract

import (
	"github.com/SkyPanel/SkyPanel/v3"
)

type Extract struct {
	Source      string
	Destination string
}

func (op Extract) Run(args SkyPanel.RunOperatorArgs) SkyPanel.OperationResult {
	err := args.Server.Extract(op.Source, op.Destination)
	return SkyPanel.OperationResult{Error: err}
}
