package mkdir

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/logging"
	"os"
	"path/filepath"
)

type Mkdir struct {
	TargetFile string
}

func (m *Mkdir) Run(args SkyPanel.RunOperatorArgs) SkyPanel.OperationResult {
	env := args.Environment
	logging.Info.Printf("Making directory: %s\n", m.TargetFile)
	env.DisplayToConsole(true, "Creating directory: %s\n", m.TargetFile)
	target := filepath.Join(env.GetRootDirectory(), m.TargetFile)
	err := os.MkdirAll(target, 0755)
	return SkyPanel.OperationResult{Error: err}
}
