package mkdir

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type Mkdir struct {
	TargetFile string
}

func (m *Mkdir) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment
	logging.Info.Printf("Making directory: %s\n", m.TargetFile)
	env.DisplayToConsole(true, "Creating directory: %s\n", m.TargetFile)
	// the confined file server refuses paths and symlinks leaving the server root
	err := args.Server.GetFileServer().MkdirAll(m.TargetFile, 0755)
	return skypanel.OperationResult{Error: err}
}
