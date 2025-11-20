package archive

import (
	"github.com/SkyPanel/SkyPanel/v3"
)

type Archive struct {
	Source      []string
	Destination string
}

func (op Archive) Run(args SkyPanel.RunOperatorArgs) SkyPanel.OperationResult {
	err := args.Server.ArchiveItems(op.Source, op.Destination)
	return SkyPanel.OperationResult{Error: err}
}
