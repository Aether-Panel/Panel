package servers

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

var (
	// mapModrinthAPI is a variable so tests can point it at a local server
	mapModrinthAPI = "https://api.modrinth.com/v2"
	// mapDownloadHosts are the hosts a map jar may be downloaded from
	mapDownloadHosts = map[string]bool{"cdn.modrinth.com": true}
	// mapLoaderFallbacks are the Modrinth loaders tried for each platform, best first
	mapLoaderFallbacks = map[string][][]string{
		"paper":    {{"paper"}, {"spigot", "bukkit"}, {"folia"}},
		"fabric":   {{"fabric"}},
		"forge":    {{"forge"}},
		"neoforge": {{"neoforge"}},
	}
)

const (
	maxMapJarSize          = 128 << 20
	maxMapModrinthResponse = 8 << 20
)

type mapModrinthFile struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Primary  bool   `json:"primary"`
	Size     int64  `json:"size"`
	Hashes   struct {
		SHA512 string `json:"sha512"`
	} `json:"hashes"`
}

type mapModrinthVersion struct {
	VersionNumber string            `json:"version_number"`
	VersionType   string            `json:"version_type"`
	Files         []mapModrinthFile `json:"files"`
}

// findMapRelease finds the newest build of a map for the platform and Minecraft version,
// preferring stable releases
func findMapRelease(kind, platform, gameVersion string) (mapModrinthFile, string, error) {
	groups, ok := mapLoaderFallbacks[platform]
	if !ok {
		return mapModrinthFile{}, "", ErrMapUnsupported
	}
	for _, loaders := range groups {
		versions, err := mapModrinthVersions(kind, loaders, gameVersion)
		if err != nil {
			return mapModrinthFile{}, "", err
		}
		if file, version, ok := pickMapRelease(versions); ok {
			return file, version, nil
		}
	}
	return mapModrinthFile{}, "", fmt.Errorf("%s has no build for %s %s yet", mapDisplayName(kind), platform, gameVersion)
}

func mapModrinthVersions(project string, loaders []string, gameVersion string) ([]mapModrinthVersion, error) {
	loadersJSON, _ := json.Marshal(loaders)
	versionsJSON, _ := json.Marshal([]string{gameVersion})
	query := url.Values{}
	query.Set("loaders", string(loadersJSON))
	query.Set("game_versions", string(versionsJSON))
	endpoint := fmt.Sprintf("%s/project/%s/version?%s", mapModrinthAPI, url.PathEscape(project), query.Encode())

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", skypanel.Display+" https://github.com/SkyPanel/SkyPanel")
	client := *skypanel.HTTP()
	client.Timeout = 30 * time.Second
	res, err := client.Do(req)
	defer utils.CloseResponse(res)
	if err != nil {
		return nil, fmt.Errorf("could not reach Modrinth: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the Modrinth API answered %d", res.StatusCode)
	}
	var versions []mapModrinthVersion
	if err := json.NewDecoder(io.LimitReader(res.Body, maxMapModrinthResponse)).Decode(&versions); err != nil {
		return nil, fmt.Errorf("reading Modrinth's answer: %w", err)
	}
	return versions, nil
}

// pickMapRelease takes the first (newest) release, or the newest beta when there is no
// release, and its primary jar
func pickMapRelease(versions []mapModrinthVersion) (mapModrinthFile, string, bool) {
	for _, stableOnly := range []bool{true, false} {
		for _, v := range versions {
			if stableOnly && v.VersionType != "release" {
				continue
			}
			if v.VersionType == "alpha" {
				continue
			}
			if file, ok := primaryJar(v.Files); ok {
				return file, v.VersionNumber, true
			}
		}
	}
	return mapModrinthFile{}, "", false
}

func primaryJar(files []mapModrinthFile) (mapModrinthFile, bool) {
	var found *mapModrinthFile
	for i := range files {
		if !validJarName(files[i].Filename) {
			continue
		}
		if files[i].Primary {
			return files[i], true
		}
		if found == nil {
			found = &files[i]
		}
	}
	if found == nil {
		return mapModrinthFile{}, false
	}
	return *found, true
}

func validJarName(name string) bool {
	return name != "" && name == filepath.Base(name) && !strings.ContainsAny(name, `/\`) &&
		strings.HasSuffix(strings.ToLower(name), ".jar") && filepath.IsLocal(name)
}

// downloadMapJar downloads the jar next to its target, checks Modrinth's hash and only then
// moves it into place, so a broken download never reaches the plugins folder
func (p *Server) downloadMapJar(file mapModrinthFile, target string) error {
	if err := checkMapDownloadURL(file.URL); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, file.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", skypanel.Display+" https://github.com/SkyPanel/SkyPanel")
	client := *skypanel.HTTP()
	client.Timeout = 5 * time.Minute
	res, err := client.Do(req)
	defer utils.CloseResponse(res)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", file.Filename, err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: the server answered %d", file.Filename, res.StatusCode)
	}

	fs := p.GetFileServer()
	partial := target + ".part"
	out, err := fs.OpenFile(partial, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	hash := sha512.New()
	written, err := io.Copy(io.MultiWriter(out, hash), io.LimitReader(res.Body, maxMapJarSize+1))
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil && written > maxMapJarSize {
		err = errors.New("the file is larger than expected")
	}
	if err == nil && file.Hashes.SHA512 != "" && !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), file.Hashes.SHA512) {
		err = errors.New("the file does not match Modrinth's checksum")
	}
	if err != nil {
		_ = fs.Remove(partial)
		return fmt.Errorf("downloading %s: %w", file.Filename, err)
	}
	if err := fs.Rename(partial, target); err != nil {
		_ = fs.Remove(partial)
		return err
	}
	return nil
}

func checkMapDownloadURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	api, _ := url.Parse(mapModrinthAPI)
	trusted := u.Scheme == "https" && mapDownloadHosts[u.Hostname()]
	// tests serve both from the same local server
	if api != nil && u.Host == api.Host && u.Scheme == api.Scheme {
		trusted = true
	}
	if !trusted {
		return fmt.Errorf("refusing to download from %s", u.Host)
	}
	return nil
}
