package spongedl

import (
	"encoding/json"
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/forgedl"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"net/http"
	"os"
	"path"
	"strings"
)

var SpongeAPIBaseURL = "https://dl-api.spongepowered.org/v2/groups/org.spongepowered/artifacts/"

type SpongeDl struct {
	Recommended      bool
	SpongeType       string
	SpongeVersion    string // version of sponge to download
	MinecraftVersion string // version of minecraft to download
}

type SpongeAPIV2Versions struct {
	Artifacts map[string]interface{} `json:"artifacts"`
}

type SpongeAPIV2Latest struct {
	Assets []SpongeAPIV2Asset `json:"assets"`
	Tags   map[string]string
}

type SpongeAPIV2Asset struct {
	Classifier  string
	DownloadURL string
	Extension   string
}

func (op SpongeDl) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	// first, we need to get the build we need to get, if one isn't specified
	if op.SpongeVersion == "" {
		latest, err := op.latestVersion(env)
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
		op.SpongeVersion = latest
	}

	var key string
	if op.SpongeType == "vanilla" {
		key = ""
	} else {
		key = "universal"
	}

	data, err := op.getSpecificVersion(env, op.SpongeVersion)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	url := findJarURL(data.Assets, key)
	if url == "" {
		err = errors.New("no asset found to download")
		return skypanel.OperationResult{Error: err}
	}

	switch strings.ToLower(op.SpongeType) {
	case "spongeforge":
		err = installSpongeForge(args, data, url)
	case "spongevanilla":
		err = downloadSpongeJar(env, url, path.Join(env.GetRootDirectory(), "server.jar"))
	default:
		err = errors.New("invalid sponge type")
	}
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	return skypanel.OperationResult{Error: nil}
}

// latestVersion asks the Sponge API for the newest version matching the filters.
func (op SpongeDl) latestVersion(env *skypanel.Environment) (string, error) {
	data, err := op.getLatestVersion(env)
	if err != nil {
		return "", err
	}

	if len(data.Artifacts) == 0 {
		env.DisplayToConsole(true, "No matching Sponge versions found")
		return "", errors.New("no valid sponge versions found")
	}

	var version string
	for k := range data.Artifacts {
		version = k
		break
	}
	return version, nil
}

// findJarURL returns the download url of the last jar asset with the given classifier.
func findJarURL(assets []SpongeAPIV2Asset, classifier string) string {
	var url string
	for _, v := range assets {
		if v.Classifier == classifier && v.Extension == "jar" {
			url = v.DownloadURL
		}
	}
	return url
}

// installSpongeForge installs forge first, then drops spongeforge into the mods folder.
func installSpongeForge(args skypanel.RunOperatorArgs, data SpongeAPIV2Latest, url string) error {
	mapping := make(map[string]interface{})
	mapping["minecraftVersion"] = data.Tags["minecraft"]
	mapping["target"] = "forge-installer.jar"
	forgeDlOp, err := forgedl.Factory.Create(skypanel.CreateOperation{OperationArgs: mapping})
	if err != nil {
		return err
	}

	res := forgeDlOp.Run(args)
	if res.Error != nil {
		return res.Error
	}

	err = args.Server.GetFileServer().Mkdir("mods", 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// going to stick the spongeforge rename in, to assist with those modpacks
	env := args.Environment
	return downloadSpongeJar(env, url, path.Join(env.GetRootDirectory(), "mods", "_aspongeforge.jar"))
}

func downloadSpongeJar(env *skypanel.Environment, url, target string) error {
	file, err := skypanel.DownloadViaMaven(url, env)
	defer utils.Close(file)
	if err != nil {
		return err
	}

	return files.WriteFile(file, target)
}

func (op SpongeDl) getLatestVersion(env *skypanel.Environment) (SpongeAPIV2Versions, error) {
	var data SpongeAPIV2Versions

	var params = "?limit=1"
	if op.MinecraftVersion != "" {
		params += "&tags=minecraft:" + op.MinecraftVersion
	}
	if op.Recommended {
		params += "&recommended=true"
	}

	var url = SpongeAPIBaseURL + op.SpongeType + "/versions" + params

	response, err := skypanel.HTTPGet(url)
	if err != nil {
		return data, err
	}
	defer utils.CloseResponse(response)
	if response.StatusCode != http.StatusOK {
		env.DisplayToConsole(true, "Failed to get the Sponge information from %s: %s", url, response.Status)
		return data, errors.New(response.Status)
	}

	err = json.NewDecoder(response.Body).Decode(&data)
	return data, err
}

func (op SpongeDl) getSpecificVersion(env *skypanel.Environment, version string) (SpongeAPIV2Latest, error) {
	var data SpongeAPIV2Latest

	var url = SpongeAPIBaseURL + op.SpongeType + "/versions/" + version

	response, err := skypanel.HTTPGet(url)
	if err != nil {
		return data, err
	}
	defer utils.CloseResponse(response)
	if response.StatusCode != http.StatusOK {
		env.DisplayToConsole(true, "Failed to get the Sponge information from %s: %s", url, response.Status)
		return data, errors.New(response.Status)
	}

	err = json.NewDecoder(response.Body).Decode(&data)
	return data, err
}
