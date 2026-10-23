package servers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/query"
	"github.com/spf13/cast"
)

// One-click world map for Minecraft: Java Edition. BlueMap (3D) or Dynmap (2D) is downloaded
// from Modrinth into plugins/ (Paper) or mods/ (Fabric, Forge, NeoForge); the panel then
// points the map's own web server at a known port and shows it through a reverse proxy. What
// the panel installed is remembered in <ServersFolder>/<id>.worldmap, outside the server's
// own folder.

const (
	MapBlueMap = "bluemap"
	MapDynmap  = "dynmap"

	// default ports of the maps' web servers
	blueMapDefaultPort = 8100
	dynmapDefaultPort  = 8123
)

var (
	ErrMapUnsupported  = errors.New("the world map needs a Paper, Fabric, Forge or NeoForge server")
	ErrMapKind         = errors.New("the map must be bluemap or dynmap")
	ErrMapEULA         = errors.New("installing BlueMap needs consent to download Minecraft's resources (Mojang's EULA)")
	ErrMapInstalling   = errors.New("a map install is already running")
	ErrMapNotInstalled = errors.New("no world map is installed on this server")
	gameVersionPattern = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)
)

// WorldMapStatus is what the map tab needs
type WorldMapStatus struct {
	Supported bool `json:"supported"`
	// Reason says why the map is not supported: "vanilla" or "unknown"
	Reason string `json:"reason"`
	// Platform is "paper", "fabric", "forge" or "neoforge"
	Platform    string `json:"platform,omitempty"`
	GameVersion string `json:"gameVersion,omitempty"`
	// Installed is "bluemap", "dynmap" or empty
	Installed  string `json:"installed"`
	Version    string `json:"version,omitempty"`
	Installing bool   `json:"installing"`
	LastError  string `json:"lastError,omitempty"`
	Running    bool   `json:"running"`
	// Ready is true when the map's web server answers
	Ready bool `json:"ready"`
	// ViewURL is the signed link the panel adds for the viewer
	ViewURL string `json:"viewUrl,omitempty"`
} // @name WorldMapStatus

// worldMapSidecar is what the panel installed, kept next to the server's folder
type worldMapSidecar struct {
	Kind           string    `json:"kind"`
	Version        string    `json:"version"`
	Port           int       `json:"port"`
	AcceptDownload bool      `json:"acceptDownload"`
	InstalledAt    time.Time `json:"installedAt"`
	// File is the jar, relative to the server's folder
	File     string `json:"file"`
	Platform string `json:"platform"`
}

// worldMapState tracks a running install, the last failure and which run of the server has
// had its map configuration checked
type worldMapState struct {
	installing atomic.Bool
	lastError  atomic.Value
	watching   atomic.Bool
	// configuredRun is the startedAt of the run whose configuration was checked
	configuredRun atomic.Int64
	// target caches the address the map answered on
	targetLock sync.Mutex
	target     string
	targetAt   time.Time
}

var worldMapStates sync.Map

func (p *Server) worldMapState() *worldMapState {
	state, _ := worldMapStates.LoadOrStore(p.ID(), &worldMapState{})
	return state.(*worldMapState)
}

func worldMapFile(serverID string) string {
	return filepath.Join(config.ServersFolder.Value(), serverID+".worldmap")
}

func (p *Server) readWorldMap() (*worldMapSidecar, error) {
	safeID, err := validateServerID(p.ID())
	if err != nil {
		return nil, err
	}
	data, err := readLimited(worldMapFile(safeID), 64*1024)
	if err != nil {
		return nil, err
	}
	var sidecar worldMapSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return nil, fmt.Errorf("reading the world map settings: %w", err)
	}
	if sidecar.Kind != MapBlueMap && sidecar.Kind != MapDynmap {
		return nil, ErrMapKind
	}
	return &sidecar, nil
}

func (p *Server) writeWorldMap(sidecar *worldMapSidecar) error {
	safeID, err := validateServerID(p.ID())
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(worldMapFile(safeID), data, 0644)
}

// installedWorldMap is the sidecar when its jar is still there; an owner who deleted the jar
// by hand has no map any more
func (p *Server) installedWorldMap() *worldMapSidecar {
	sidecar, err := p.readWorldMap()
	if err != nil {
		return nil
	}
	if !safeRelativePath(sidecar.File) {
		return nil
	}
	if _, err := p.GetFileServer().Stat(sidecar.File); err != nil {
		return nil
	}
	return sidecar
}

func readLimited(file string, limit int64) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}

// safeRelativePath accepts a slash path that stays inside the server's folder
func safeRelativePath(name string) bool {
	if name == "" || strings.Contains(name, "\\") || path.IsAbs(name) {
		return false
	}
	return filepath.IsLocal(filepath.FromSlash(name)) && path.Clean(name) == name
}

// modpackInfo reads the loader and game version another feature leaves in
// <ServersFolder>/<id>.modpack, when a modpack was installed
func (p *Server) modpackInfo() (loader, gameVersion string) {
	safeID, err := validateServerID(p.ID())
	if err != nil {
		return "", ""
	}
	data, err := readLimited(filepath.Join(config.ServersFolder.Value(), safeID+".modpack"), 256*1024)
	if err != nil {
		return "", ""
	}
	var info map[string]interface{}
	if json.Unmarshal(data, &info) != nil {
		return "", ""
	}
	return cast.ToString(info["loader"]), cast.ToString(info["gameVersion"])
}

// mapPlatformFor turns a loader or launcher name into the platform the maps are built for
func mapPlatformFor(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "paper", "spigot", "bukkit", "purpur", "pufferfish", "folia":
		return "paper"
	case "fabric", "quilt":
		return "fabric"
	case "forge":
		return "forge"
	case "neoforge":
		return "neoforge"
	}
	return ""
}

// detectMapPlatform decides which build of a map the server can load. It looks at, in order,
// the modpack's loader, the template's "modlauncher" variable (launcher is nil when the
// template has none) and the install steps that download the game (steps holds their types,
// paperdl as "paperdl:<project>"). reason is "vanilla" or "unknown" when no map fits.
func detectMapPlatform(modpackLoader string, launcher *string, steps []string) (platform, reason string) {
	if platform := mapPlatformFor(modpackLoader); platform != "" {
		return platform, ""
	}
	if launcher != nil {
		value := strings.ToLower(strings.TrimSpace(*launcher))
		if platform := mapPlatformFor(value); platform != "" {
			return platform, ""
		}
		if value != "" && value != "vanilla" {
			// magma, mohist, a modpack without a known loader...
			return "", "unknown"
		}
	}
	vanilla := false
	for _, step := range steps {
		kind, project, _ := strings.Cut(step, ":")
		switch kind {
		case "paperdl":
			if project == "" {
				project = "paper"
			}
			if mapPlatformFor(project) == "paper" {
				return "paper", ""
			}
			// Velocity and Waterfall are proxies without a world
			return "", "unknown"
		case "fabricdl":
			return "fabric", ""
		case "forgedl":
			return "forge", ""
		case "neoforgedl":
			return "neoforge", ""
		case "mojangdl":
			vanilla = true
		}
	}
	if vanilla || (launcher != nil && strings.TrimSpace(*launcher) == "") {
		return "", "vanilla"
	}
	return "", "unknown"
}

// mapPlatform reads what detectMapPlatform needs from the server
func (p *Server) mapPlatform() (platform, reason, modpackVersion string) {
	if p.Type.Type != "" && p.Type.Type != "minecraft-java" {
		return "", "unknown", ""
	}
	loader, modpackVersion := p.modpackInfo()
	var launcher *string
	if v, ok := p.Variables["modlauncher"]; ok {
		value := cast.ToString(v.Value)
		launcher = &value
	}
	steps := make([]string, 0)
	for _, op := range p.Installation {
		if op.If != "" {
			active, err := p.RunCondition(op.If, nil)
			if err != nil || !active {
				continue
			}
		}
		step := op.Type
		if op.Type == "paperdl" {
			project := ""
			if raw, ok := op.Metadata["project"]; ok && raw != nil {
				project = strings.TrimSpace(cast.ToString(raw))
				if match := tokenVariable.FindStringSubmatch(project); match != nil {
					project = cast.ToString(p.Variables[match[1]].Value)
				}
			}
			step += ":" + strings.ToLower(project)
		}
		steps = append(steps, step)
	}
	platform, reason = detectMapPlatform(loader, launcher, steps)
	return platform, reason, modpackVersion
}

// stableGameVersion keeps "1.21.4"-like versions and drops "latest", snapshots and blanks
func stableGameVersion(v string) string {
	v = strings.TrimSpace(v)
	if stableVersion.MatchString(v) {
		return v
	}
	return ""
}

// mapGameVersion works out the Minecraft version: the modpack's, the server variable's, what
// the running server reports, or the newest release when the variable says "latest"
func (p *Server) mapGameVersion(modpackVersion string) string {
	if v := stableGameVersion(modpackVersion); v != "" {
		return v
	}
	variable := "version"
	if _, v, ok := p.versionTarget(); ok {
		variable = v
	}
	if v := stableGameVersion(cast.ToString(p.Variables[variable].Value)); v != "" {
		return v
	}
	if result, ok := p.QueryGame(); ok {
		if mc, found := result["minecraft"].(query.MinecraftResponse); found {
			if v := stableGameVersion(gameVersionPattern.FindString(mc.Version)); v != "" {
				return v
			}
		}
	}
	if versions, err := availableVersions("vanilla"); err == nil && len(versions) > 0 {
		return versions[0]
	}
	return ""
}

// WorldMapStatus describes the map tab; it also makes sure the map's configuration was
// checked for the current run of the server
func (p *Server) WorldMapStatus() WorldMapStatus {
	state := p.worldMapState()
	status := WorldMapStatus{Installing: state.installing.Load()}
	if last, ok := state.lastError.Load().(string); ok {
		status.LastError = last
	}
	platform, reason, modpackVersion := p.mapPlatform()
	status.Platform = platform
	status.Reason = reason
	status.Supported = platform != ""
	if status.Supported {
		status.GameVersion = p.mapGameVersion(modpackVersion)
	}
	status.Running, _ = p.IsRunning()

	sidecar := p.installedWorldMap()
	if sidecar == nil {
		return status
	}
	status.Installed = sidecar.Kind
	status.Version = sidecar.Version
	if status.Running && !status.Installing {
		p.ensureMapConfigured()
		status.Ready = p.worldMapReady()
	}
	return status
}

// InstallWorldMap checks the request and installs the map in the background; the console
// shows each step
func (p *Server) InstallWorldMap(kind string, acceptDownload bool) error {
	if kind != MapBlueMap && kind != MapDynmap {
		return ErrMapKind
	}
	if kind == MapBlueMap && !acceptDownload {
		return ErrMapEULA
	}
	platform, _, modpackVersion := p.mapPlatform()
	if platform == "" {
		return ErrMapUnsupported
	}
	if p.IsBackingUp() || p.IsRestoring() {
		return errBackupBusy
	}
	state := p.worldMapState()
	if !state.installing.CompareAndSwap(false, true) {
		return ErrMapInstalling
	}
	state.lastError.Store("")

	go func() {
		defer state.installing.Store(false)
		if err := p.runWorldMapInstall(kind, platform, modpackVersion, acceptDownload); err != nil {
			state.lastError.Store(err.Error())
			p.Log(logging.Error, "World map install failed: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "World map install failed: %s\n", err)
		}
	}()
	return nil
}

func mapDisplayName(kind string) string {
	if kind == MapDynmap {
		return "Dynmap"
	}
	return "BlueMap"
}

func (p *Server) runWorldMapInstall(kind, platform, modpackVersion string, acceptDownload bool) error {
	name := mapDisplayName(kind)
	p.RunningEnvironment.DisplayToConsole(true, "Installing the %s world map\n", name)

	gameVersion := p.mapGameVersion(modpackVersion)
	if gameVersion == "" {
		return errors.New("could not tell which Minecraft version this server runs")
	}

	// a plugin can break the server; a busy backup only means this goes ahead without one
	if err := p.CreateRestorePoint(RestoreReasonPlugin); err != nil {
		p.Log(logging.Info, "No restore point before installing a world map: %s", err)
	}

	p.RunningEnvironment.DisplayToConsole(true, "Looking for %s for %s %s on Modrinth\n", name, platform, gameVersion)
	file, version, err := findMapRelease(kind, platform, gameVersion)
	if err != nil {
		return err
	}

	folder := "plugins"
	if platform != "paper" {
		folder = "mods"
	}
	if err := p.GetFileServer().MkdirAll(folder, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", folder, err)
	}
	// one map at a time: an older copy of either map would load next to the new one
	previous, _ := p.readWorldMap()
	p.removeMapJars(folder, previous)

	target := folder + "/" + file.Filename
	p.RunningEnvironment.DisplayToConsole(true, "Downloading %s\n", file.Filename)
	if err := p.downloadMapJar(file, target); err != nil {
		return err
	}

	sidecar := &worldMapSidecar{
		Kind:           kind,
		Version:        version,
		Port:           mapPort(p.DataToMap(), kind),
		AcceptDownload: acceptDownload,
		InstalledAt:    time.Now().UTC(),
		File:           target,
		Platform:       platform,
	}
	if err := p.writeWorldMap(sidecar); err != nil {
		return fmt.Errorf("saving the world map settings: %w", err)
	}
	p.worldMapState().configuredRun.Store(0)

	running, err := p.IsRunning()
	if err != nil {
		return err
	}
	if !running {
		p.RunningEnvironment.DisplayToConsole(true, "%s %s installed; the map starts with the server\n", name, version)
		return nil
	}

	p.RunningEnvironment.DisplayToConsole(true, "%s %s installed; restarting the server so it loads\n", name, version)
	if err := p.Stop(); err != nil {
		return err
	}
	if err := p.GetEnvironment().WaitForMainProcessFor(stopTimeout); err != nil {
		return err
	}
	if err := p.Start(); err != nil {
		return err
	}
	p.ensureMapConfigured()
	return nil
}

// removeMapJars deletes the jar the panel installed before and any other BlueMap or Dynmap
// jar in the folder
func (p *Server) removeMapJars(folder string, previous *worldMapSidecar) {
	fs := p.GetFileServer()
	if previous != nil && safeRelativePath(previous.File) {
		if err := fs.Remove(previous.File); err == nil {
			p.RunningEnvironment.DisplayToConsole(true, "Removed %s\n", previous.File)
		}
	}
	entries, err := fs.ReadDir(folder)
	if err != nil {
		return
	}
	for _, entry := range entries {
		lower := strings.ToLower(entry.Name())
		if entry.IsDir() || !strings.HasSuffix(lower, ".jar") {
			continue
		}
		if strings.HasPrefix(lower, "bluemap") || strings.HasPrefix(lower, "dynmap") {
			if err := fs.Remove(folder + "/" + entry.Name()); err == nil {
				p.RunningEnvironment.DisplayToConsole(true, "Removed %s/%s\n", folder, entry.Name())
			}
		}
	}
}

// RemoveWorldMap deletes the map's jar and the panel's record of it; the rendered map stays,
// so installing it again does not start from zero
func (p *Server) RemoveWorldMap() error {
	if p.worldMapState().installing.Load() {
		return ErrMapInstalling
	}
	sidecar, err := p.readWorldMap()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrMapNotInstalled
		}
		return err
	}
	if safeRelativePath(sidecar.File) {
		if err := p.GetFileServer().Remove(sidecar.File); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", sidecar.File, err)
		}
	}
	safeID, err := validateServerID(p.ID())
	if err != nil {
		return err
	}
	if err := os.Remove(worldMapFile(safeID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	state := p.worldMapState()
	state.configuredRun.Store(0)
	state.lastError.Store("")
	state.targetLock.Lock()
	state.target = ""
	state.targetLock.Unlock()
	if env := p.GetEnvironment(); env != nil {
		env.DisplayToConsole(true, "%s removed; it unloads on the next restart\n", mapDisplayName(sidecar.Kind))
	}
	return nil
}

// mapPort is where the map's web server listens: the first extra port of the server, so it is
// reachable from outside too, or the map's default
func mapPort(variables map[string]interface{}, kind string) int {
	game := cast.ToInt(variables["port"])
	for i := 2; i < 100; i++ {
		value, ok := variables[fmt.Sprintf("port%d", i)]
		if !ok {
			break
		}
		if port := cast.ToInt(value); port > 0 && port < 65536 && port != game {
			return port
		}
	}
	if kind == MapDynmap {
		return dynmapDefaultPort
	}
	return blueMapDefaultPort
}
