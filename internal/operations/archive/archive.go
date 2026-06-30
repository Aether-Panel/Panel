package archive

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type Archive struct {
	Source      []string
	Destination string
}

func (op Archive) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	err := args.Server.ArchiveItems(op.Source, op.Destination)
	return skypanel.OperationResult{Error: err}
}
