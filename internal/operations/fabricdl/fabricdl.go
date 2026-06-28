package fabricdl

import (
	"encoding/json"
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"path"
)

const FabricMetadataURL = "https://meta.fabricmc.net/v2/versions/installer"

type Fabricdl struct {
}

type FabricMetadata struct {
	URL string `json:"url"`
}

func (f *Fabricdl) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	env.DisplayToConsole(true, "Downloading metadata from %s\n", FabricMetadataURL)
	response, err := skypanel.HTTPGet(FabricMetadataURL)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	defer utils.Close(response.Body)

	var metadata []FabricMetadata
	err = json.NewDecoder(response.Body).Decode(&metadata)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	if len(metadata) == 0 {
		err = errors.New("no metadata available from Fabric, unable to download installer")
		return skypanel.OperationResult{Error: err}
	}

	file, err := skypanel.DownloadViaMaven(metadata[0].URL, env)
	defer utils.Close(file)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	err = files.WriteFile(file, path.Join(env.GetRootDirectory(), "fabric-installer.jar"))
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	return skypanel.OperationResult{Error: nil}
}
