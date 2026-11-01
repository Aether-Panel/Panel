package githubdl

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type GithubDl struct {
	Repository     string
	AssetMatch     string
	OutputVariable string
}

var (
	githubAPIBase = "https://api.github.com"
)

func (op GithubDl) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBase, op.Repository)

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
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(response.Body).Decode(&releaseData); err != nil {
		return skypanel.OperationResult{Error: err}
	}

	var downloadURL string
	for _, asset := range releaseData.Assets {
		if strings.Contains(asset.Name, op.AssetMatch) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return skypanel.OperationResult{Error: errors.New("no matching asset found in latest github release")}
	}

	env.DisplayToConsole(true, "Downloading %s\n", downloadURL)
	// saved under the URL's name, the one the output variable hands to the template
	localName := path.Base(downloadURL)
	err = utils.DownloadWith(nil, downloadURL, func(_ string, content io.Reader) error {
		return files.WriteFile(content, env.GetRootDirectory(), localName)
	})
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	if op.OutputVariable != "" {
		// The asset was just saved to the server root directory by grab using
		// the basename of the URL. Templates reference this variable as a local
		// file path (e.g. in `extract source` / `move source` / `rm`), so we must
		// expose the on-disk filename rather than the download URL.
		return skypanel.OperationResult{
			VariableOverrides: map[string]interface{}{
				op.OutputVariable: localName,
			},
			Error: nil,
		}
	}

	return skypanel.OperationResult{Error: nil}
}
