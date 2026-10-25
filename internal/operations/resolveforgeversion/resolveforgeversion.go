/*
 Copyright 2019 Padduck, LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

 	http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package resolveforgeversion

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/hashicorp/go-version"
)

type ResolveForgeVersion struct {
	Version          string
	MinecraftVersion string
	OutputVariable   string
}

func (op ResolveForgeVersion) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment
	fs := args.Server.GetFileServer()

	// if a specific version wasn't specified, we have to dig around through the files....
	if op.Version == "" {
		dir := filepath.Join("libraries", "net", "minecraftforge", "forge")
		folders, err := fs.ReadDir(dir)
		if os.IsNotExist(err) {
			return skypanel.OperationResult{VariableOverrides: map[string]interface{}{
				op.OutputVariable: op.Version,
			}}
		}
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}

		op.Version = latestForgeVersion(fs, dir, folders, op.MinecraftVersion)
	}

	if op.Version != "" {
		env.DisplayToConsole(true, "Resolved Forge Version: %s", op.Version)
	}

	return skypanel.OperationResult{VariableOverrides: map[string]interface{}{
		op.OutputVariable: op.Version,
	}}
}

// latestForgeVersion picks a version folder that has a unix_args.txt, so it is supported.
func latestForgeVersion(fileServer files.FileServer, dir string, folders []fs.DirEntry, mcVersion string) string {
	var current string
	var ver *version.Version
	for _, v := range folders {
		// look for folders
		if !v.IsDir() {
			continue
		}
		folderName := v.Name()
		// look for the unix file to accurately confirm this to be supported
		desiredFile := filepath.Join(dir, folderName, "unix_args.txt")
		if _, err := fileServer.Stat(desiredFile); err != nil {
			continue
		}
		current, ver = pickVersion(current, ver, folderName, mcVersion)
	}
	return current
}

// pickVersion keeps the first folder found, then only looks at folders for the wanted
// Minecraft version.
func pickVersion(current string, ver *version.Version, folderName, mcVersion string) (string, *version.Version) {
	switch {
	case current == "":
		current = folderName
		ver, _ = version.NewVersion(current)
	case !strings.HasPrefix(folderName, mcVersion):
		// we need a different version of MC
	case ver != nil:
		// time to check to see if this a newer version
		if ver2, _ := version.NewVersion(current); ver2 != nil && ver.LessThan(ver2) {
			current = folderName
			ver = ver2
		}
	}
	return current, ver
}
