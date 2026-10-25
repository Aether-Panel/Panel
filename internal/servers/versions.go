package servers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/hashicorp/go-version"
)

// One-click game version changes. A template that downloads the game with paperdl (Paper,
// Folia, Velocity, Waterfall) or mojangdl (vanilla) takes the version from a server variable;
// changing that variable and reinstalling is the whole switch. The panel does it in one go:
// stop, restore point, new version, reinstall, start again.

// VersionInfo is what the version picker needs
type VersionInfo struct {
	// Platform is "paper", "folia", "velocity", "waterfall" or "vanilla"
	Platform string `json:"platform"`
	// Variable is the server variable that holds the version
	Variable string `json:"variable"`
	Current  string `json:"current"`
	// Versions are the stable releases, newest first
	Versions []string `json:"versions"`
	// Changing is true while a change runs; LastError holds the last failure, if any
	Changing  bool   `json:"changing"`
	LastError string `json:"lastError,omitempty"`
} // @name VersionInfo

var (
	ErrNoVersionSwitch  = errors.New("this server's template does not let the panel change its game version")
	ErrUnknownVersion   = errors.New("that version is not available for this server")
	ErrVersionChanging  = errors.New("a version change is already running")
	tokenVariable       = regexp.MustCompile(`^\$\{(\w+)\}$`)
	stableVersion       = regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`)
	versionCache        = make(map[string]cachedVersions)
	versionCacheLock    sync.Mutex
	versionListEndpoint = map[string]string{
		"vanilla": "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json",
		"paper":   "https://fill.papermc.io/v3/projects/%s/versions",
	}
)

type cachedVersions struct {
	versions []string
	fetched  time.Time
}

const versionCacheFor = time.Hour

// versionTarget finds the install step that downloads the game and the variable it reads
func (p *Server) versionTarget() (platform, variable string, ok bool) {
	for _, op := range p.Installation {
		var arg, plat string
		switch op.Type {
		case "paperdl":
			arg = fmt.Sprint(op.Metadata["minecraftVersion"])
			plat = strings.ToLower(strings.TrimSpace(utils.ReplaceTokens(fmt.Sprint(op.Metadata["project"]), p.Server.DataToMap())))
			if plat == "" || plat == "<nil>" {
				plat = "paper"
			}
		case "mojangdl":
			arg = fmt.Sprint(op.Metadata["version"])
			plat = "vanilla"
		default:
			continue
		}
		match := tokenVariable.FindStringSubmatch(strings.TrimSpace(arg))
		if match == nil {
			continue
		}
		if _, exists := p.Variables[match[1]]; !exists {
			continue
		}
		return plat, match[1], true
	}
	return "", "", false
}

// versionChange tracks a running change and the last failure
type versionChange struct {
	running   atomic.Bool
	lastError atomic.Value
}

var versionChanges sync.Map

func (p *Server) versionState() *versionChange {
	state, _ := versionChanges.LoadOrStore(p.ID(), &versionChange{})
	return state.(*versionChange)
}

// GameVersions describes the version picker for this server
func (p *Server) GameVersions() (VersionInfo, error) {
	platform, variable, ok := p.versionTarget()
	// a modpack fixes the game version: changing it would break the pack
	if !ok || p.modpackLocked() {
		return VersionInfo{}, ErrNoVersionSwitch
	}
	versions, err := availableVersions(platform)
	if err != nil {
		return VersionInfo{}, err
	}
	state := p.versionState()
	info := VersionInfo{
		Platform: platform,
		Variable: variable,
		Current:  fmt.Sprint(p.Variables[variable].Value),
		Versions: versions,
		Changing: state.running.Load(),
	}
	if last, ok := state.lastError.Load().(string); ok {
		info.LastError = last
	}
	return info, nil
}

// ChangeVersion checks the request and runs the change in the background; the console shows
// each step
func (p *Server) ChangeVersion(target string) error {
	platform, variable, ok := p.versionTarget()
	if !ok || p.modpackLocked() {
		return ErrNoVersionSwitch
	}
	versions, err := availableVersions(platform)
	if err != nil {
		return err
	}
	known := target == "latest"
	for _, v := range versions {
		known = known || v == target
	}
	if !known {
		return ErrUnknownVersion
	}
	if p.IsBackingUp() || p.IsRestoring() {
		return errBackupBusy
	}
	state := p.versionState()
	if !state.running.CompareAndSwap(false, true) {
		return ErrVersionChanging
	}
	state.lastError.Store("")

	go func() {
		defer state.running.Store(false)
		if err := p.runVersionChange(variable, target); err != nil {
			state.lastError.Store(err.Error())
			p.Log(logging.Error, "Version change failed: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "Version change failed: %s\n", err)
		}
	}()
	return nil
}

func (p *Server) runVersionChange(variable, target string) error {
	previous := p.Variables[variable].Value
	p.RunningEnvironment.DisplayToConsole(true, "Changing the game version from %v to %s\n", previous, target)

	wasRunning, err := p.IsRunning()
	if err != nil {
		return err
	}
	if wasRunning {
		if err := p.Stop(); err != nil {
			return err
		}
		if err := p.GetEnvironment().WaitForMainProcessFor(stopTimeout); err != nil {
			return err
		}
	}

	// without a way back the change does not happen
	if err := p.CreateRestorePoint(RestoreReasonVersion); err != nil {
		return fmt.Errorf("could not create a restore point: %w", err)
	}

	variableValue := p.Variables[variable]
	variableValue.Value = target
	p.Variables[variable] = variableValue
	// a build pinned for the old version does not exist for the new one
	buildVariable, previousBuild := p.buildVariable(), interface{}(nil)
	if buildVariable != "" {
		build := p.Variables[buildVariable]
		previousBuild = build.Value
		build.Value = "latest"
		p.Variables[buildVariable] = build
	}
	if err := p.Save(); err != nil {
		return err
	}

	if err := p.Install(); err != nil {
		// put the variable back, so the server still describes what is installed
		variableValue.Value = previous
		p.Variables[variable] = variableValue
		if buildVariable != "" {
			build := p.Variables[buildVariable]
			build.Value = previousBuild
			p.Variables[buildVariable] = build
		}
		_ = p.Save()
		return fmt.Errorf("the new version could not be installed (a restore point was saved): %w", err)
	}

	p.RunningEnvironment.DisplayToConsole(true, "Game version changed to %s\n", target)
	if wasRunning {
		return p.Start()
	}
	return nil
}

// availableVersions lists the stable releases of a platform, newest first, cached for an hour
func availableVersions(platform string) ([]string, error) {
	versionCacheLock.Lock()
	cached, ok := versionCache[platform]
	versionCacheLock.Unlock()
	if ok && time.Since(cached.fetched) < versionCacheFor {
		return cached.versions, nil
	}

	var versions []string
	var err error
	if platform == "vanilla" {
		versions, err = fetchVanillaVersions()
	} else {
		versions, err = fetchPaperVersions(platform)
	}
	if err != nil {
		return nil, err
	}
	if len(versions) > 80 {
		versions = versions[:80]
	}

	versionCacheLock.Lock()
	versionCache[platform] = cachedVersions{versions: versions, fetched: time.Now()}
	versionCacheLock.Unlock()
	return versions, nil
}

func getJSON(url string, target interface{}) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", skypanel.Display+" https://github.com/SkyPanel/SkyPanel")
	res, err := skypanel.HTTP().Do(req)
	defer utils.CloseResponse(res)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %d", url, res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func fetchVanillaVersions() ([]string, error) {
	var manifest struct {
		Versions []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"versions"`
	}
	if err := getJSON(versionListEndpoint["vanilla"], &manifest); err != nil {
		return nil, err
	}
	versions := make([]string, 0)
	for _, v := range manifest.Versions {
		if v.Type == "release" {
			versions = append(versions, v.ID)
		}
	}
	// the manifest is already newest first
	return versions, nil
}

func fetchPaperVersions(project string) ([]string, error) {
	var res struct {
		Versions []struct {
			Version struct {
				ID string `json:"id"`
			} `json:"version"`
		} `json:"versions"`
	}
	if err := getJSON(fmt.Sprintf(versionListEndpoint["paper"], project), &res); err != nil {
		return nil, err
	}
	versions := make([]string, 0, len(res.Versions))
	for _, v := range res.Versions {
		if stableVersion.MatchString(v.Version.ID) {
			versions = append(versions, v.Version.ID)
		}
	}
	return sortVersionsDesc(versions), nil
}

func sortVersionsDesc(versions []string) []string {
	sort.SliceStable(versions, func(i, j int) bool {
		a, errA := version.NewVersion(versions[i])
		b, errB := version.NewVersion(versions[j])
		if errA != nil || errB != nil {
			return versions[i] > versions[j]
		}
		return a.GreaterThan(b)
	})
	return versions
}

// buildVariable is the server variable a paperdl step reads its build number from, if any
func (p *Server) buildVariable() string {
	for _, op := range p.Installation {
		if op.Type != "paperdl" {
			continue
		}
		match := tokenVariable.FindStringSubmatch(strings.TrimSpace(fmt.Sprint(op.Metadata["build"])))
		if match == nil {
			continue
		}
		if _, exists := p.Variables[match[1]]; exists {
			return match[1]
		}
	}
	return ""
}
