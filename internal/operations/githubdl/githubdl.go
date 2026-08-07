package githubdl

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/cavaliergopher/grab/v3"
)

type GithubDl struct {
	Repository     string
	AssetMatch     string
	OutputVariable string
}

func (op GithubDl) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", op.Repository)

	env.DisplayToConsole(true, "Fetching latest release from %s\n", op.Repository)
	logging.Debug.Printf("Fetching github release from %s", url)

	response, err := skypanel.HTTPGet(url)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	defer utils.CloseResponse(response)

	if response.StatusCode != 200 {
		return skypanel.OperationResult{Error: fmt.Errorf("unexpected status code %d from github api", response.StatusCode)}
	}

	var releaseData struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadUrl string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(response.Body).Decode(&releaseData); err != nil {
		return skypanel.OperationResult{Error: err}
	}

	var downloadUrl string
	for _, asset := range releaseData.Assets {
		if strings.Contains(asset.Name, op.AssetMatch) {
			downloadUrl = asset.BrowserDownloadUrl
			break
		}
	}

	if downloadUrl == "" {
		return skypanel.OperationResult{Error: errors.New("no matching asset found in latest github release")}
	}

	env.DisplayToConsole(true, "Downloading %s\n", downloadUrl)
	_, err = grab.Get(env.GetRootDirectory(), downloadUrl)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	if op.OutputVariable != "" {
		return skypanel.OperationResult{
			VariableOverrides: map[string]interface{}{
				op.OutputVariable: downloadUrl, // you might want to extract just the version here, but let's provide the full url or asset name?
			},
			Error: nil,
		}
	}

	return skypanel.OperationResult{Error: nil}
}
