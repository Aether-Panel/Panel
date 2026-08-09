package download

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/cavaliergopher/grab/v3"
)

type Download struct {
	Files []string
}

func (d Download) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	for _, file := range d.Files {
		logging.Info.Printf("Download file from %s to %s", file, env.GetRootDirectory())
		env.DisplayToConsole(true, "Downloading file %s\n", file)
		_, err := grab.Get(env.GetRootDirectory(), file)
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
	}
	return skypanel.OperationResult{Error: nil}
}
