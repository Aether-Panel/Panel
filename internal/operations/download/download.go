package download

import (
	"net"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type Download struct {
	Files []string
}

func (d Download) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	for _, file := range d.Files {
		logging.Info.Printf("Download file from %s to %s", file, env.GetRootDirectory())
		env.DisplayToConsole(true, "Downloading file %s\n", file)
		err := utils.DownloadFile(env.GetRootDirectory(), file, func(ip net.IP) {
			env.DisplayToConsole(true, "Warning: downloading from private network address %s\n", ip)
		})
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
	}
	return skypanel.OperationResult{Error: nil}
}
