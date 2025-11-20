package move

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/logging"
	"os"
	"path/filepath"
)

type Move struct {
	SourceFile string
	TargetFile string
}

func (m Move) Run(args SkyPanel.RunOperatorArgs) SkyPanel.OperationResult {
	env := args.Environment
	fs := args.Server.GetFileServer()

	result, valid := resolve(fs, m.SourceFile, m.TargetFile)
	if !valid {
		return SkyPanel.OperationResult{Error: nil}
	}

	for k, v := range result {
		logging.Info.Printf("Moving file from %s to %s", k, v)
		env.DisplayToConsole(true, "Moving file from %s to %s\n", k, v)
		err := fs.Rename(k, v)
		if err != nil {
			return SkyPanel.OperationResult{Error: err}
		}
	}
	return SkyPanel.OperationResult{Error: nil}
}

func resolve(fs files.FileServer, source string, target string) (result map[string]string, valid bool) {
	result = make(map[string]string)
	sourceFiles, _ := fs.Glob(source)
	info, err := fs.Stat(target)

	if err != nil {
		if os.IsNotExist(err) && len(sourceFiles) > 1 {
			logging.Error.Printf("Target folder does not exist")
			valid = false
			return
		} else if !os.IsNotExist(err) {
			valid = false
			logging.Error.Printf("Error reading target file: %s", err)
			return
		}
	} else if info.IsDir() && len(sourceFiles) > 1 {
		logging.Error.Printf("Cannot move multiple files to single file target")
		valid = false
		return
	}

	if info != nil && info.IsDir() {
		for _, v := range sourceFiles {
			_, fileName := filepath.Split(v)
			result[v] = filepath.Join(target, fileName)
		}
	} else {
		for _, v := range sourceFiles {
			result[v] = target
		}
	}
	valid = true
	return
}
