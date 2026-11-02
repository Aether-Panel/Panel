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

package resolveneoforgeversion

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/hashicorp/go-version"
)

// librariesDir is where the NeoForge installer leaves one folder per installed version
var librariesDir = filepath.Join("libraries", "net", "neoforged", "neoforge")

type ResolveNeoForgeVersion struct {
	Version          string
	MinecraftVersion string
	OutputVariable   string
}

func (op ResolveNeoForgeVersion) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	// if a specific version wasn't specified, we have to dig around through the files....
	if op.Version == "" {
		// through the server's file server, like every other operation: the folders are the
		// game's, and a symlink left there must not lead outside the server
		if args.Server == nil || args.Server.GetFileServer() == nil {
			return skypanel.OperationResult{Error: errors.New("no file server for this server")}
		}
		fsys := args.Server.GetFileServer()
		folders, err := fsys.ReadDir(librariesDir)
		if errors.Is(err, fs.ErrNotExist) {
			return skypanel.OperationResult{VariableOverrides: map[string]interface{}{
				op.OutputVariable: op.Version,
			}}
		}
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}

		op.Version = latestNeoForgeVersion(fsys, folders, op.MinecraftVersion)
	}

	if op.Version != "" {
		args.Environment.DisplayToConsole(true, "Resolved NeoForge Version: %s", op.Version)
	}

	return skypanel.OperationResult{VariableOverrides: map[string]interface{}{
		op.OutputVariable: op.Version,
	}}
}

// latestNeoForgeVersion picks the newest version folder for the wanted Minecraft version that
// has a unix_args.txt, so it is supported. When none is for that version, the first
// supported folder found is used, as before.
func latestNeoForgeVersion(fsys files.FileServer, folders []fs.DirEntry, mcVersion string) string {
	prefix := neoForgePrefix(mcVersion)
	var first, best string
	var bestVersion *version.Version
	for _, v := range folders {
		if !v.IsDir() {
			continue
		}
		name := v.Name()
		// look for the unix file to accurately confirm this to be supported
		if _, err := fsys.Stat(filepath.Join(librariesDir, name, "unix_args.txt")); err != nil {
			continue
		}
		if first == "" {
			first = name
		}
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if ver, err := version.NewVersion(name); err == nil && (bestVersion == nil || bestVersion.LessThan(ver)) {
			best, bestVersion = name, ver
		}
	}
	if best != "" {
		return best
	}
	return first
}

// neoForgePrefix is how the NeoForge versions for a Minecraft version start: NeoForge drops
// the "1." (21.4.150 is for 1.21.4, 21.0.x for 1.21). Comparing with the Minecraft version
// itself matched no folder, so the newest one was never looked for.
func neoForgePrefix(mcVersion string) string {
	if mcVersion == "" {
		return ""
	}
	trimmed := strings.TrimPrefix(mcVersion, "1.")
	if !strings.Contains(trimmed, ".") {
		trimmed += ".0"
	}
	return trimmed + "."
}
