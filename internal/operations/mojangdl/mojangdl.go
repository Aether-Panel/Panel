package mojangdl

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

const VersionJSONURL = "https://launchermeta.mojang.com/mc/game/version_manifest.json"

type MojangDl struct {
	Version string
	Target  string
}

func (op MojangDl) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	response, err := skypanel.HTTPGet(VersionJSONURL)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	var data LauncherJSON
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	err = response.Body.Close()
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	var targetVersion string
	switch op.Version {
	case "release":
		targetVersion = data.Latest.Release
	case "latest":
		targetVersion = data.Latest.Release
	case "snapshot":
		targetVersion = data.Latest.Snapshot
	default:
		targetVersion = op.Version
	}

	for _, version := range data.Versions {
		if version.ID == targetVersion {
			logging.Info.Printf("Version %s json located, downloading from %s", version.ID, version.URL)
			env.DisplayToConsole(true, fmt.Sprintf("Version %s json located, downloading from %s\n", version.ID, version.URL))
			//now, get the version json for this one...
			err = downloadServerFromJSON(version.URL, op.Target, env)
			return skypanel.OperationResult{Error: err}
		}
	}

	env.DisplayToConsole(true, "Could not locate version "+targetVersion+"\n")
	err = errors.New("Version not located: " + op.Version)
	return skypanel.OperationResult{Error: err}
}

func downloadServerFromJSON(url, target string, env *skypanel.Environment) error {
	response, err := skypanel.HTTPGet(url)
	defer utils.CloseResponse(response)
	if err != nil {
		return err
	}

	var data VersionJSON
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		return err
	}
	err = response.Body.Close()
	if err != nil {
		return err
	}

	serverBlock := data.Downloads["server"]

	logging.Info.Printf("Version jar located, downloading from %s", serverBlock.URL)
	env.DisplayToConsole(true, fmt.Sprintf("Version jar located, downloading from %s\n", serverBlock.URL))

	return skypanel.DownloadFile(serverBlock.URL, target, env)
}

type LauncherJSON struct {
	Versions []LauncherVersion `json:"versions"`
	Latest   Latest            `json:"latest"`
}

type Latest struct {
	Release  string `json:"release"`
	Snapshot string `json:"snapshot"`
}

type LauncherVersion struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Type string `json:"type"`
}

type VersionJSON struct {
	Downloads map[string]DownloadType `json:"downloads"`
}

type DownloadType struct {
	Sha1 string `json:"sha1"`
	Size uint64 `json:"size"`
	URL  string `json:"url"`
}
