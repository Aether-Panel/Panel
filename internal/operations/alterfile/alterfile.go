package alterfile

import (
	"bytes"
	"io"
	"os"
	"regexp"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type AlterFile struct {
	TargetFile string
	Search     string
	Replace    string
	Regex      bool
}

func (c AlterFile) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment
	// all I/O goes through the confined file server so the target can neither point
	// outside the server root nor follow a symlink out of it
	fs := args.Server.GetFileServer()

	logging.Info.Printf("Changing data in file: %s", c.TargetFile)
	env.DisplayToConsole(true, "Changing some data in file: %s\n ", c.TargetFile)

	file, err := fs.OpenFile(c.TargetFile, os.O_RDONLY, 0644)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	data, err := io.ReadAll(file)
	utils.Close(file)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	var out []byte
	if c.Regex {
		regex, err := regexp.Compile("(?m)" + c.Search)
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
		out = regex.ReplaceAllLiteral(data, []byte(c.Replace))
	} else {
		out = bytes.ReplaceAll(data, []byte(c.Search), []byte(c.Replace))
	}

	file, err = fs.OpenFile(c.TargetFile, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	defer utils.Close(file)

	_, err = file.Write(out)
	return skypanel.OperationResult{Error: err}
}
