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

package forgedl

import (
	"encoding/json"
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"path"
	"strings"
)

const InstallerURL = "https://maven.minecraftforge.net/net/minecraftforge/forge/${version}/forge-${version}-installer.jar"
const PromoURL = "https://files.minecraftforge.net/net/minecraftforge/forge/promotions_slim.json"

type ForgeDl struct {
	Version          string
	Filename         string
	MinecraftVersion string
	OutputVariable   string
}

func (op ForgeDl) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	if op.Version == "" {
		version, err := getLatestForMCVersion(op.MinecraftVersion)
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
		op.Version = op.MinecraftVersion + "-" + version
	}

	jarDownload := strings.ReplaceAll(InstallerURL, "${version}", op.Version)

	localFile, err := skypanel.DownloadViaMaven(jarDownload, env)
	defer utils.Close(localFile)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	//copy from the cache
	err = files.WriteFile(localFile, path.Join(env.GetRootDirectory(), op.Filename))
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	return skypanel.OperationResult{VariableOverrides: map[string]interface{}{
		op.OutputVariable: op.Version,
	}}
}

func getLatestForMCVersion(minecraftVersion string) (string, error) {
	response, err := skypanel.HttpGet(PromoURL)
	defer utils.CloseResponse(response)
	if err != nil {
		return "", err
	}

	var promos ForgePromos
	err = json.NewDecoder(response.Body).Decode(&promos)
	if err != nil {
		return "", err
	}
	version := promos.VersionMap[minecraftVersion+"-latest"]
	if version == "" {
		return "", errors.New("no forge available for mc version")
	}
	return version, nil
}

type ForgePromos struct {
	Homepage   string            `json:"homepage"`
	VersionMap map[string]string `json:"promos"`
}
