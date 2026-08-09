package paperdl

import (
	"crypto"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/hashicorp/go-version"
)

var UserAgent = skypanel.Display + " https://github.com/SkyPanel/SkyPanel"

// DefaultProject is used when a template does not specify a PaperMC project.
const DefaultProject = "paper"

type PaperDl struct {
	MinecraftVersion string
	Build            string
	Filename         string
	Project          string
}

func (op PaperDl) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	if op.Project == "" {
		op.Project = DefaultProject
	}

	if op.MinecraftVersion == "latest" {
		logging.Info.Printf("PaperDL got Minecraft version 'latest', looking up latest version supported by %s", op.Project)
		mcVersion, err := getLatestVersion(op.Project)
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
		op.MinecraftVersion = mcVersion
	}

	dlURL, hash, err := op.getDownloadURLAndHash(env)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	dl, err := skypanel.Download(dlURL, hash, crypto.SHA256, true, env)
	defer utils.Close(dl)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	err = files.WriteFile(dl, path.Join(env.GetRootDirectory(), op.Filename))
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	return skypanel.OperationResult{Error: nil}
}

func getLatestVersion(project string) (string, error) {
	u, err := url.Parse(fmt.Sprintf("https://fill.papermc.io/v3/projects/%s/versions", project))
	if err != nil {
		return "", err
	}

	request := &http.Request{
		Method: "GET",
		URL:    u,
		Header: http.Header{},
	}
	request.Header.Add("user-agent", UserAgent)

	response, err := skypanel.HTTP().Do(request)
	defer utils.CloseResponse(response)
	if err != nil {
		return "", err
	}

	var versions PaperVersionsResponse
	err = json.NewDecoder(response.Body).Decode(&versions)
	if err != nil {
		return "", err
	}

	latest, _ := version.NewVersion("0.0")
	for _, v := range versions.Versions {
		if ver, err := version.NewVersion(v.VersionInfo.ID); err == nil && latest.LessThan(ver) {
			latest = ver
		} else if err != nil {
			logging.Info.Printf("failed to parse version '%s', %s", v, err)
		}
	}

	logging.Info.Printf("Latest version supported by %s is %s", project, latest.Original())
	return latest.Original(), nil
}

func (op PaperDl) getDownloadURLAndHash(env *skypanel.Environment) (string, string, error) {
	buildPath := fmt.Sprintf("https://fill.papermc.io/v3/projects/%s/versions/%s/builds/%s", op.Project, op.MinecraftVersion, op.Build)
	path, err := url.Parse(buildPath)
	if err != nil {
		return "", "", err
	}

	request := &http.Request{
		Method: "GET",
		URL:    path,
		Header: http.Header{},
	}
	request.Header.Add("User-Agent", UserAgent)

	response, err := skypanel.HTTP().Do(request)
	defer utils.CloseResponse(response)
	if err != nil {
		return "", "", err
	}

	if response.StatusCode == 404 {
		env.DisplayToConsole(true, "Invalid %s version or build\n", op.Project)
		return "", "", fmt.Errorf("invalid %s version or build", op.Project)
	}

	var build PaperBuild
	err = json.NewDecoder(response.Body).Decode(&build)
	if err != nil {
		return "", "", err
	}

	return build.Downloads.Server.URL, build.Downloads.Server.Checksums.Sha256, nil
}

type PaperVersionInfo struct {
	ID string `json:"id"`
}

type PaperVersion struct {
	VersionInfo PaperVersionInfo `json:"version"`
}

type PaperVersionsResponse struct {
	Versions []PaperVersion `json:"versions"`
}

type PaperChecksums struct {
	Sha256 string `json:"sha256"`
}

type PaperServer struct {
	Checksums PaperChecksums `json:"checksums"`
	URL       string         `json:"url"`
}

type PaperDownload struct {
	Server PaperServer `json:"server:default"`
}

type PaperBuild struct {
	Downloads PaperDownload `json:"downloads"`
}
