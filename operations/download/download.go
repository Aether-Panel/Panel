package download

import (
	"github.com/cavaliergopher/grab/v3"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/logging"
)

type Download struct {
	Files []string
}

func (d Download) Run(args SkyPanel.RunOperatorArgs) SkyPanel.OperationResult {
	env := args.Environment

	for _, file := range d.Files {
		logging.Info.Printf("Download file from %s to %s", file, env.GetRootDirectory())
		env.DisplayToConsole(true, "Downloading file %s\n", file)
		_, err := grab.Get(env.GetRootDirectory(), file)
		if err != nil {
			return SkyPanel.OperationResult{Error: err}
		}
	}
	return SkyPanel.OperationResult{Error: nil}
}
