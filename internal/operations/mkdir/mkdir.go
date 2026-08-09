package mkdir

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"os"
	"path/filepath"
	"strings"
)

type Mkdir struct {
	TargetFile string
}

func (m *Mkdir) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment
	logging.Info.Printf("Making directory: %s\n", m.TargetFile)
	env.DisplayToConsole(true, "Creating directory: %s\n", m.TargetFile)
	cleanRoot := filepath.Clean(env.GetRootDirectory())
	target := filepath.Join(cleanRoot, m.TargetFile)
	cleanTarget := filepath.Clean(target)
	if !strings.HasPrefix(cleanTarget, cleanRoot+string(filepath.Separator)) && cleanTarget != cleanRoot {
		return skypanel.OperationResult{Error: os.ErrPermission}
	}
	err := os.MkdirAll(cleanTarget, 0755)
	return skypanel.OperationResult{Error: err}
}
