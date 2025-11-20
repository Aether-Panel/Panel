package fabricdl

import (
	"encoding/json"
	"errors"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/utils"
	"path"
)

const FabricMetadataUrl = "https://meta.fabricmc.net/v2/versions/installer"

type Fabricdl struct {
}

type FabricMetadata struct {
	Url string `json:"url"`
}

func (f *Fabricdl) Run(args SkyPanel.RunOperatorArgs) SkyPanel.OperationResult {
	env := args.Environment

	env.DisplayToConsole(true, "Downloading metadata from %s\n", FabricMetadataUrl)
	response, err := SkyPanel.HttpGet(FabricMetadataUrl)
	if err != nil {
		return SkyPanel.OperationResult{Error: err}
	}
	defer utils.Close(response.Body)

	var metadata []FabricMetadata
	err = json.NewDecoder(response.Body).Decode(&metadata)
	if err != nil {
		return SkyPanel.OperationResult{Error: err}
	}
	if len(metadata) == 0 {
		err = errors.New("no metadata available from Fabric, unable to download installer")
		return SkyPanel.OperationResult{Error: err}
	}

	file, err := SkyPanel.DownloadViaMaven(metadata[0].Url, env)
	defer utils.Close(file)
	if err != nil {
		return SkyPanel.OperationResult{Error: err}
	}

	err = files.WriteFile(file, path.Join(env.GetRootDirectory(), "fabric-installer.jar"))
	if err != nil {
		return SkyPanel.OperationResult{Error: err}
	}

	return SkyPanel.OperationResult{Error: nil}
}
