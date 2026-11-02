package download

import (
	"io"
	"net"

	"github.com/SkyPanel/SkyPanel/v3/files"
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
		err := utils.DownloadFile(file, func(ip net.IP) {
			env.DisplayToConsole(true, "Warning: downloading from private network address %s\n", ip)
		}, saveTo(env.GetRootDirectory()))
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
	}
	return skypanel.OperationResult{Error: nil}
}

// saveTo writes a finished download into the server folder at root
func saveTo(root string) func(string, io.Reader) error {
	return func(name string, content io.Reader) error {
		return files.WriteFile(content, root, name)
	}
}
