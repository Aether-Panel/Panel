package servers

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/hashicorp/go-version"
)

// One-click modpacks. The panel installs a whole pack from Modrinth (.mrpack) or CurseForge
// (server pack) in the background: stop, restore point, clean what a pack replaces, download,
// install the loader, set the Java and game version, start again. A small sidecar file next
// to the server's definition remembers which pack is installed, so the version picker can
// hide (changing the game version under a pack breaks it).

// Modpack sources
const (
	ModpackSourceModrinth   = "modrinth"
	ModpackSourceCurseForge = "curseforge"
)

// Install steps, reported as short keys the UI translates
const (
	ModpackStepPreparing     = "preparing"
	ModpackStepStopping      = "stopping"
	ModpackStepRestorePoint  = "restorePoint"
	ModpackStepCleaning      = "cleaning"
	ModpackStepDownloadPack  = "downloadPack"
	ModpackStepDownloadFiles = "downloadFiles"
	ModpackStepExtracting    = "extracting"
	ModpackStepLoader        = "loader"
	ModpackStepCurseForge    = "curseforge"
	ModpackStepStarting      = "starting"
	ModpackStepDone          = "done"
	ModpackStepFailed        = "failed"
)

var (
	ErrModpackInstalling    = errors.New("a modpack install is already running")
	ErrModpackServerBusy    = errors.New("a backup or restore is running; try again when it finishes")
	ErrModpackBadRequest    = errors.New("invalid modpack request")
	ErrCurseForgeKeyMissing = errors.New("CurseForge needs an API key; an administrator can add it in the panel settings")
	ErrModpackUnsupported   = errors.New("this modpack version uses a loader the panel cannot install (Fabric, Forge and NeoForge are supported)")
	errModpackNoLauncher    = errors.New("the pack was installed but no server.jar was produced, so the server could not start")
)

// UpstreamError is a refusal or failure from Modrinth, CurseForge or a download host; the
// daemon answers 422 with its message, so the owner can read what went wrong
type UpstreamError struct {
	Site string
	Err  error
}

func (e *UpstreamError) Error() string { return e.Site + ": " + e.Err.Error() }
func (e *UpstreamError) Unwrap() error { return e.Err }

func upstream(site string, err error) error {
	if err == nil {
		return nil
	}
	var already *UpstreamError
	if errors.As(err, &already) {
		return err
	}
	return &UpstreamError{Site: site, Err: err}
}

// InstalledModpack describes the pack on a server; it is kept in <ServersFolder>/<id>.modpack
type InstalledModpack struct {
	Source      string    `json:"source"`
	ProjectID   string    `json:"projectId"`
	VersionID   string    `json:"versionId"`
	Title       string    `json:"title"`
	VersionName string    `json:"versionName"`
	IconURL     string    `json:"iconUrl"`
	Loader      string    `json:"loader"`
	GameVersion string    `json:"gameVersion"`
	InstalledAt time.Time `json:"installedAt"`
	// Launcher and LauncherSHA1 identify the jar the pack starts with; if it changes (a
	// backup was restored, the server was reinstalled) the pack is no longer considered
	// installed
	Launcher     string `json:"launcher,omitempty"`
	LauncherSHA1 string `json:"launcherSha1,omitempty"`
} // @name InstalledModpack

// ModpackStatus is what the modpacks tab shows
type ModpackStatus struct {
	Installed  *InstalledModpack `json:"installed"`
	Installing bool              `json:"installing"`
	// Step is one of the ModpackStep keys; Current and Total count files when known
	Step      string `json:"step"`
	Current   int    `json:"current"`
	Total     int    `json:"total"`
	LastError string `json:"lastError"`
	// Target is the pack being installed (or the last one tried)
	Target *InstalledModpack `json:"target,omitempty"`
	// CurseForge is true when the node has a CurseForge API key
	CurseForge bool            `json:"curseforge"`
	Platform   ModpackPlatform `json:"platform"`
	// Memory is the server's memory in MB, for the "this pack needs more" hint
	Memory int `json:"memory"`
} // @name ModpackStatus

// ModpackPlatform is the loader and game version the server runs today
type ModpackPlatform struct {
	Loader      string `json:"loader"`
	GameVersion string `json:"gameVersion"`
} // @name ModpackPlatform

// ModpackInstallRequest is the body of an install
type ModpackInstallRequest struct {
	Source     string `json:"source"`
	ProjectID  string `json:"projectId"`
	VersionID  string `json:"versionId"`
	FreshWorld bool   `json:"freshWorld"`
	Label      string `json:"label"`
} // @name ModpackInstallRequest

// modpackPlan is a checked request with what the job needs to run it
type modpackPlan struct {
	request ModpackInstallRequest
	pack    InstalledModpack
	// Modrinth: the .mrpack to download
	packURL  string
	packSize int64
	hashes   map[string]string
	// CurseForge: the file to install (client file or server pack)
	fileID string
}

// modpackJob tracks a running install and its last result
type modpackJob struct {
	running atomic.Bool
	mu      sync.Mutex
	step    string
	current int
	total   int
	lastErr string
	target  *InstalledModpack
}

var modpackJobs sync.Map

func (p *Server) modpackState() *modpackJob {
	state, _ := modpackJobs.LoadOrStore(p.ID(), &modpackJob{})
	return state.(*modpackJob)
}

func (j *modpackJob) setStep(step string, current, total int) {
	j.mu.Lock()
	j.step, j.current, j.total = step, current, total
	j.mu.Unlock()
}

// idPattern accepts Modrinth ids and slugs and CurseForge numeric ids
var (
	idPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	numericPattern = regexp.MustCompile(`^[0-9]{1,12}$`)
)

func validateModpackRequest(req ModpackInstallRequest) error {
	switch req.Source {
	case ModpackSourceModrinth:
		if !idPattern.MatchString(req.ProjectID) || !idPattern.MatchString(req.VersionID) {
			return fmt.Errorf("%w: unknown project or version", ErrModpackBadRequest)
		}
	case ModpackSourceCurseForge:
		if !numericPattern.MatchString(req.ProjectID) || !numericPattern.MatchString(req.VersionID) {
			return fmt.Errorf("%w: unknown project or file", ErrModpackBadRequest)
		}
		if config.CurseForgeKey.Value() == "" {
			return ErrCurseForgeKeyMissing
		}
	default:
		return fmt.Errorf("%w: source must be modrinth or curseforge", ErrModpackBadRequest)
	}
	return nil
}

// ModpackStatus describes the tab: what is installed, what is running and what the server is
func (p *Server) ModpackStatus() ModpackStatus {
	state := p.modpackState()
	status := ModpackStatus{
		Installed:  p.InstalledModpack(),
		Installing: state.running.Load(),
		CurseForge: config.CurseForgeKey.Value() != "",
	}
	state.mu.Lock()
	status.Step, status.Current, status.Total = state.step, state.current, state.total
	status.LastError = state.lastErr
	if state.target != nil {
		target := *state.target
		status.Target = &target
	}
	state.mu.Unlock()

	if status.Installed != nil {
		status.Platform = ModpackPlatform{Loader: status.Installed.Loader, GameVersion: status.Installed.GameVersion}
	} else {
		status.Platform = ModpackPlatform{Loader: p.variableString("modlauncher"), GameVersion: p.variableString("version")}
	}
	if memory, ok := p.Variables["memory"]; ok {
		fmt.Sscan(fmt.Sprint(memory.Value), &status.Memory) //nolint:errcheck // 0 when not a number
	}
	return status
}

func (p *Server) variableString(name string) string {
	v, ok := p.Variables[name]
	if !ok || v.Value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v.Value))
}

// modpackLocked is true while a pack is installed or being installed; the game version is
// then the pack's, so the version picker must not change it
func (p *Server) modpackLocked() bool {
	if p.ID() == "" {
		return false
	}
	return p.modpackState().running.Load() || p.InstalledModpack() != nil
}

func modpackFile(serverID string) (string, bool) {
	id, err := validateServerID(serverID)
	if err != nil || id == "" {
		return "", false
	}
	return filepath.Join(config.ServersFolder.Value(), id+".modpack"), true
}

// InstalledModpack reads the sidecar; nil when no pack is installed or the server no longer
// starts with the jar the pack installed
func (p *Server) InstalledModpack() *InstalledModpack {
	file, ok := modpackFile(p.ID())
	if !ok {
		return nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var pack InstalledModpack
	if json.Unmarshal(data, &pack) != nil || pack.Source == "" {
		return nil
	}
	if pack.Launcher != "" && p.RunningEnvironment != nil {
		sum, err := sha1File(filepath.Join(p.RunningEnvironment.GetRootDirectory(), pack.Launcher))
		if err != nil || sum != pack.LauncherSHA1 {
			return nil
		}
	}
	return &pack
}

func (p *Server) saveInstalledModpack(pack InstalledModpack) error {
	file, ok := modpackFile(p.ID())
	if !ok {
		return errors.New("invalid server id")
	}
	data, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0600)
}

// ForgetModpack removes the sidecar, e.g. when the server is deleted
func ForgetModpack(serverID string) {
	if file, ok := modpackFile(serverID); ok {
		_ = os.Remove(file)
	}
	modpackJobs.Delete(serverID)
}

func sha1File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer utils.Close(f)
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// javaForMinecraft is the Java the eclipse-temurin image (and javadl on host) should use for
// a game version: 1.20.5+ needs 21, 1.17 to 1.20.4 run on 17, older Forge packs need 8.
// The year-numbered releases (26.1 onwards) need Java 25.
func javaForMinecraft(mc string) string {
	v, err := version.NewVersion(strings.TrimSpace(mc))
	if err != nil {
		return "21"
	}
	segments := v.Segments()
	switch {
	case len(segments) > 0 && segments[0] >= 26:
		return "25"
	case !v.LessThan(version.Must(version.NewVersion("1.20.5"))):
		return "21"
	case !v.LessThan(version.Must(version.NewVersion("1.17"))):
		return "17"
	default:
		return "8"
	}
}

// InstallModpack checks the request, looks the pack up and runs the install in the
// background; the console and ModpackStatus show each step
func (p *Server) InstallModpack(req ModpackInstallRequest) error {
	req.Source = strings.ToLower(strings.TrimSpace(req.Source))
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.VersionID = strings.TrimSpace(req.VersionID)
	if err := validateModpackRequest(req); err != nil {
		return err
	}
	if p.IsBackingUp() || p.IsRestoring() {
		return ErrModpackServerBusy
	}
	if p.versionState().running.Load() {
		return ErrVersionChanging
	}
	state := p.modpackState()
	if !state.running.CompareAndSwap(false, true) {
		return ErrModpackInstalling
	}

	plan, err := resolveModpackPlan(req)
	if err != nil {
		state.running.Store(false)
		return err
	}

	state.mu.Lock()
	state.step, state.current, state.total, state.lastErr = ModpackStepPreparing, 0, 0, ""
	target := plan.pack
	state.target = &target
	state.mu.Unlock()

	go func() {
		defer state.running.Store(false)
		if err := p.runModpackInstall(plan, state); err != nil {
			state.mu.Lock()
			state.lastErr = err.Error()
			state.step = ModpackStepFailed
			state.mu.Unlock()
			p.Log(logging.Error, "Modpack install failed: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "Modpack install failed: %s\n", err)
		}
	}()
	return nil
}

func resolveModpackPlan(req ModpackInstallRequest) (modpackPlan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if req.Source == ModpackSourceCurseForge {
		return resolveCurseForgePlan(ctx, req)
	}
	return resolveModrinthPlan(ctx, req)
}

// modpackVariables are the server variables the install changes, so a failure can put them back
var modpackVariables = []string{"version", "javaversion", "projectId", "fileId"}

func (p *Server) runModpackInstall(plan modpackPlan, state *modpackJob) error {
	env := p.RunningEnvironment
	pack := plan.pack
	env.DisplayToConsole(true, "Installing the modpack %s (%s)\n", pack.Title, pack.VersionName)

	state.setStep(ModpackStepStopping, 0, 0)
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

	// nobody may start the server while its files are replaced
	env.SetInstalling(true)
	// without a way back the install does not happen
	state.setStep(ModpackStepRestorePoint, 0, 0)
	if err := p.CreateRestorePoint(RestoreReasonModpack); err != nil {
		env.SetInstalling(false)
		return fmt.Errorf("could not create a restore point, nothing was changed: %w", err)
	}

	installErr := p.installModpackFiles(plan, state)
	env.SetInstalling(false)
	if installErr != nil {
		return fmt.Errorf("%w (a restore point was saved before the install; it is in Backups)", installErr)
	}

	env.DisplayToConsole(true, "Modpack %s installed\n", pack.Title)
	if wasRunning {
		state.setStep(ModpackStepStarting, 0, 0)
		if err := p.Start(); err != nil {
			return fmt.Errorf("the modpack was installed but the server did not start: %w", err)
		}
	}
	state.setStep(ModpackStepDone, 0, 0)
	return nil
}

// installModpackFiles does the part of the install that changes files and variables; on a
// failure the variables are put back so the server still describes what it was
func (p *Server) installModpackFiles(plan modpackPlan, state *modpackJob) error {
	env := p.RunningEnvironment
	fs := p.GetFileServer()
	root := env.GetRootDirectory()
	pack := plan.pack

	previous := make(map[string]interface{})
	for _, name := range modpackVariables {
		if v, ok := p.Variables[name]; ok {
			previous[name] = v.Value
		}
	}
	restoreVariables := func() {
		for name, value := range previous {
			v := p.Variables[name]
			v.Value = value
			p.Variables[name] = v
		}
		_ = p.Save()
	}

	state.setStep(ModpackStepCleaning, 0, 0)
	env.DisplayToConsole(true, "Removing the files the pack replaces\n")
	kept, err := readKeptFiles(fs)
	if err != nil {
		return err
	}
	if err := cleanForModpack(fs, root, plan.request.FreshWorld); err != nil {
		return fmt.Errorf("could not clean the server folder: %w", err)
	}

	// the installer and the server run on the Java this game version needs (the docker
	// image is eclipse-temurin:${javaversion}), so it is set before anything runs
	p.setVariableIfPresent("version", pack.GameVersion)
	p.setVariableIfPresent("javaversion", javaForMinecraft(pack.GameVersion))
	if plan.request.Source == ModpackSourceCurseForge {
		// the CurseForge template reinstalls the same pack from these
		p.setVariableIfPresent("projectId", pack.ProjectID)
		p.setVariableIfPresent("fileId", plan.fileID)
	}
	if err := p.Save(); err != nil {
		restoreVariables()
		return err
	}

	var installErr error
	if plan.request.Source == ModpackSourceCurseForge {
		state.setStep(ModpackStepCurseForge, 0, 0)
		installErr = p.installCurseForgePack(plan)
	} else {
		installErr = p.installModrinthPack(plan, &pack, state)
	}
	if installErr == nil {
		installErr = restoreKeptFiles(fs, kept, plan.request.FreshWorld)
	}
	if installErr == nil {
		installErr = p.applyTemplateFiles()
	}
	if installErr != nil {
		restoreVariables()
		return installErr
	}

	launcher := ""
	for _, name := range []string{"fabric-server-launch.jar", "server.jar"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			launcher = name
			break
		}
	}
	if launcher == "" {
		restoreVariables()
		return errModpackNoLauncher
	}
	pack.Launcher = launcher
	pack.LauncherSHA1, _ = sha1File(filepath.Join(root, launcher))
	pack.InstalledAt = time.Now()
	if err := p.saveInstalledModpack(pack); err != nil {
		p.Log(logging.Error, "Could not save the installed modpack: %s", err)
	}
	return nil
}

// applyTemplateFiles runs the template's own file-writing install steps (eula.txt, and a first
// server.properties with the server's address and port when there is none), which a pack does
// not ship, then makes sure the game listens on the port the panel gave the server
func (p *Server) applyTemplateFiles() error {
	steps := p.Installation[:0:0]
	for _, op := range p.Installation {
		if op.Type == "writefile" {
			steps = append(steps, op)
		}
	}
	if len(steps) > 0 {
		process, err := GenerateProcess(steps, p.RunningEnvironment, p.DataToMap(), p.Execution.EnvironmentVariables)
		if err != nil {
			return err
		}
		if err := process.Run(p); err != nil {
			return fmt.Errorf("writing the template's files: %w", err)
		}
	}

	port := fmt.Sprint(p.DataToMap()["port"])
	if port == "" || port == "0" || port == "<nil>" {
		return nil
	}
	fs := p.GetFileServer()
	properties, err := readSmallFile(fs, "server.properties")
	if err != nil {
		// no server.properties: the game writes one with its defaults
		return nil
	}
	if current, ok := readProperty(properties, "server-port"); ok && strings.TrimSpace(current) == port {
		return nil
	}
	merged := mergeProperties(properties, []byte("server-port="+port+"\n"), []string{"server-port"})
	_, err = writeFileFS(fs, "server.properties", bytes.NewReader(merged), keptFileLimit)
	return err
}

func (p *Server) setVariableIfPresent(name, value string) {
	v, ok := p.Variables[name]
	if !ok || value == "" {
		return
	}
	v.Value = value
	if v.Type.Type == "integer" {
		var n int64
		if _, err := fmt.Sscan(value, &n); err != nil {
			return
		}
		v.Value = n
	}
	p.Variables[name] = v
}
