package neoforgedl

import (
	"encoding/xml"
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/hashicorp/go-version"
	"path"
	"strings"
)

const InstallerURL = "https://maven.neoforged.net/releases/net/neoforged/neoforge/${version}/neoforge-${version}-installer.jar"
const MetadataURL = "https://maven.neoforged.net/releases/net/neoforged/neoforge/maven-metadata.xml"

type NeoforgeDL struct {
	Version          string
	Filename         string
	MinecraftVersion string
	OutputVariable   string
}

func (op NeoforgeDL) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment
	if op.Version == "" {
		neoVersion, err := getLatestForMCVersion(op.MinecraftVersion)
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
		op.Version = neoVersion
	}

	jarDownload := strings.ReplaceAll(InstallerURL, "${version}", op.Version)

	localFile, err := skypanel.DownloadViaMaven(jarDownload, env)
	defer utils.Close(localFile)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	// copy from the cache
	err = files.WriteFile(localFile, path.Join(env.GetRootDirectory(), op.Filename))
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	return skypanel.OperationResult{VariableOverrides: map[string]interface{}{
		op.OutputVariable: op.Version,
	}}
}

func getLatestForMCVersion(minecraftVersion string) (string, error) {
	response, err := skypanel.HTTPGet(MetadataURL)
	defer utils.CloseResponse(response)
	if err != nil {
		return "", err
	}

	var metadata Metadata
	err = xml.NewDecoder(response.Body).Decode(&metadata)
	if err != nil {
		return "", err
	}
	splitVersion := strings.TrimPrefix(minecraftVersion, "1.")

	var topVersion *version.Version

	for _, v := range metadata.Versions {
		if strings.HasPrefix(v, splitVersion) {
			newVersion, err := version.NewVersion(v)
			if err != nil {
				logging.Debug.Printf("Failed to parse version for Neoforge: %s -> %s", v, err.Error())
				continue
			}
			if topVersion == nil {
				topVersion = newVersion
			} else if newVersion.GreaterThan(topVersion) {
				topVersion = newVersion
			}
		}
	}

	if topVersion == nil {
		return "", errors.New("failed to find neoforgedl version for " + minecraftVersion)
	}

	return topVersion.Original(), nil
}

type Metadata struct {
	Versions []string `xml:"versioning>versions>version"`
	Latest   string   `xml:"latest"`
	Release  string   `xml:"release"`
}
