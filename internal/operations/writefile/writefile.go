package writefile

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"os"
)

type WriteFile struct {
	TargetFile string
	Text       string
}

func (c WriteFile) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment
	fs := args.Server.GetFileServer()

	logging.Info.Printf("Writing data to file: %s", c.TargetFile)
	env.DisplayToConsole(true, "Writing some data to file: %s\n", c.TargetFile)

	file, err := fs.OpenFile(c.TargetFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	defer utils.Close(file)

	_, err = file.Write([]byte(c.Text))
	return skypanel.OperationResult{Error: err}
}
