package servers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers/docker"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/spf13/cast"
)

// The maps write their configuration on their first start, so the panel cannot set it at
// install time. While a map is installed, each run of the server has its configuration
// checked once: the panel waits for the files, sets the web port (and, for BlueMap, the
// resource download the owner agreed to) and reloads the map when something changed.

const (
	mapConfigWait  = 3 * time.Minute
	mapConfigPoll  = 3 * time.Second
	mapReloadPause = 20 * time.Second
	maxMapConfig   = 1 << 20
	mapTargetCache = 30 * time.Second
)

var ErrMapUnreachable = errors.New("the map's web server does not answer yet")

// mapConfigEdit sets one top-level key of a map's configuration file
type mapConfigEdit struct {
	file  string
	key   string
	value string
}

// mapConfigEdits lists what the panel sets for a map; the files are relative to the
// server's folder
func mapConfigEdits(sidecar *worldMapSidecar, port int) []mapConfigEdit {
	portValue := strconv.Itoa(port)
	if sidecar.Kind == MapDynmap {
		file := "plugins/dynmap/configuration.txt"
		if sidecar.Platform != "paper" {
			file = "dynmap/configuration.txt"
		}
		return []mapConfigEdit{{file: file, key: "webserver-port", value: portValue}}
	}
	folder := "plugins/BlueMap/"
	if sidecar.Platform != "paper" {
		folder = "config/bluemap/"
	}
	edits := make([]mapConfigEdit, 0, 2)
	if sidecar.AcceptDownload {
		edits = append(edits, mapConfigEdit{file: folder + "core.conf", key: "accept-download", value: "true"})
	}
	return append(edits, mapConfigEdit{file: folder + "webserver.conf", key: "port", value: portValue})
}

// setConfigValue sets a top-level "key: value" (or "key = value") line, keeping every other
// line and comment as it was. A key that is only there commented out gets the new line right
// below the comment; a missing key is added at the end. changed is false when the value was
// already right.
func setConfigValue(content, key, value string) (result string, changed bool) {
	newline := "\n"
	if strings.Contains(content, "\r\n") {
		newline = "\r\n"
	}
	lines := strings.Split(content, newline)
	active := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `(\s*[:=]\s*)(.*?)\s*$`)
	commented := regexp.MustCompile(`^\s*(#|//)\s*` + regexp.QuoteMeta(key) + `\s*[:=]`)

	for i, line := range lines {
		match := active.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if unquoteConfig(match[2]) == value {
			return content, false
		}
		lines[i] = key + match[1] + value
		return strings.Join(lines, newline), true
	}
	for i, line := range lines {
		if commented.MatchString(line) {
			lines = append(lines[:i+1], append([]string{key + ": " + value}, lines[i+1:]...)...)
			return strings.Join(lines, newline), true
		}
	}
	if content != "" && !strings.HasSuffix(content, newline) {
		content += newline
	}
	return content + key + ": " + value + newline, true
}

func unquoteConfig(v string) string {
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
		return v[1 : len(v)-1]
	}
	return v
}

// ensureMapConfigured checks the map's configuration once per run of the server
func (p *Server) ensureMapConfigured() {
	state := p.worldMapState()
	run := p.startedAt.Load() + 1
	if state.configuredRun.Load() == run {
		return
	}
	if !state.watching.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer state.watching.Store(false)
		if p.watchMapConfig(run) {
			state.configuredRun.Store(run)
		}
	}()
}

// watchMapConfig waits for the map to write its configuration and patches it; false means the
// files did not show up while this run lasted
func (p *Server) watchMapConfig(run int64) bool {
	deadline := time.Now().Add(mapConfigWait)
	for time.Now().Before(deadline) {
		if running, err := p.IsRunning(); err != nil || !running || p.startedAt.Load()+1 != run {
			return false
		}
		sidecar := p.installedWorldMap()
		if sidecar == nil {
			return false
		}
		done, reloaded, err := p.patchMapConfig(sidecar)
		if err != nil {
			p.Log(logging.Error, "Could not configure the world map: %s", err)
			return false
		}
		if done {
			return true
		}
		if reloaded {
			// let the reload finish (and write the files it was missing) before looking again
			time.Sleep(mapReloadPause)
		}
		time.Sleep(mapConfigPoll)
	}
	return false
}

// patchMapConfig applies the edits to the files that exist (a map may write some only after
// a reload); done is false while some are still missing
func (p *Server) patchMapConfig(sidecar *worldMapSidecar) (done, reloaded bool, err error) {
	port := sidecar.Port
	if port <= 0 {
		port = mapPort(p.DataToMap(), sidecar.Kind)
	}
	edits := mapConfigEdits(sidecar, port)
	fs := p.GetFileServer()
	contents, missing, err := readMapConfigFiles(fs, edits)
	if err != nil {
		return false, false, err
	}

	changedFiles := make(map[string]bool)
	portChanged := false
	for _, edit := range edits {
		if missing[edit.file] {
			continue
		}
		updated, changed := setConfigValue(contents[edit.file], edit.key, edit.value)
		if !changed {
			continue
		}
		contents[edit.file] = updated
		changedFiles[edit.file] = true
		if edit.key != "accept-download" {
			portChanged = true
		}
	}
	if err := writeMapConfigFiles(fs, contents, changedFiles); err != nil {
		return false, false, err
	}
	if sidecar.Port != port {
		sidecar.Port = port
		_ = p.writeWorldMap(sidecar)
	}
	if len(changedFiles) == 0 {
		return len(missing) == 0, false, nil
	}

	p.ForgetWorldMapTarget()
	name := mapDisplayName(sidecar.Kind)
	p.RunningEnvironment.DisplayToConsole(true, "%s configured (web map on port %d); reloading it\n", name, port)
	switch {
	case sidecar.Kind == MapBlueMap:
		err = p.Execute("bluemap reload")
	case portChanged:
		err = p.Execute("dynmap reload")
	}
	if err != nil {
		p.Log(logging.Error, "Could not reload the world map: %s", err)
	}
	return len(missing) == 0, true, nil
}

// readMapConfigFiles loads every file the edits touch; the ones that do not exist yet are
// reported in missing
func readMapConfigFiles(fs files.FileServer, edits []mapConfigEdit) (contents map[string]string, missing map[string]bool, err error) {
	contents = make(map[string]string)
	missing = make(map[string]bool)
	for _, edit := range edits {
		if _, ok := contents[edit.file]; ok || missing[edit.file] {
			continue
		}
		file, err := fs.OpenFile(edit.file, os.O_RDONLY, 0644)
		if errors.Is(err, os.ErrNotExist) {
			missing[edit.file] = true
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		data, err := io.ReadAll(io.LimitReader(file, maxMapConfig))
		utils.Close(file)
		if err != nil {
			return nil, nil, err
		}
		contents[edit.file] = string(data)
	}
	return contents, missing, nil
}

// writeMapConfigFiles saves the files the edits changed
func writeMapConfigFiles(fs files.FileServer, contents map[string]string, changedFiles map[string]bool) error {
	for file := range changedFiles {
		out, err := fs.OpenFile(file, os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, contents[file])
		closeErr := out.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return fmt.Errorf("writing %s: %w", file, err)
		}
	}
	return nil
}

// mapCandidates are the addresses the map's web server may answer on, most likely first
func (p *Server) mapCandidates(ctx context.Context, port int) []string {
	portText := strconv.Itoa(port)
	env := p.GetEnvironment()
	if env != nil {
		if d, ok := env.Implementation.(*docker.Docker); ok {
			return p.containerMapCandidates(ctx, d, env.ServerID, port)
		}
	}
	ip := strings.TrimSpace(cast.ToString(p.DataToMap()["ip"]))
	if ip == "" || ip == "0.0.0.0" || ip == "::" || ip == "[::]" {
		ip = "127.0.0.1"
	}
	return []string{net.JoinHostPort(strings.Trim(ip, "[]"), portText)}
}

// containerMapCandidates are the addresses of the server's container, plus the host's when the
// map uses an extra published port
func (p *Server) containerMapCandidates(ctx context.Context, d *docker.Docker, containerName string, port int) []string {
	portText := strconv.Itoa(port)
	ips, gateways, err := d.ContainerAddresses(ctx, containerName)
	if err != nil {
		p.Log(logging.Debug, "Could not inspect the container for the world map: %s", err)
	}
	candidates := make([]string, 0, len(ips)+len(gateways)+1)
	for _, ip := range ips {
		candidates = append(candidates, net.JoinHostPort(ip, portText))
	}
	// an extra port is also published on the host
	if port != blueMapDefaultPort && port != dynmapDefaultPort {
		candidates = append(candidates, net.JoinHostPort("127.0.0.1", portText))
		for _, gateway := range gateways {
			candidates = append(candidates, net.JoinHostPort(gateway, portText))
		}
	}
	return candidates
}

// WorldMapTarget is the host:port the map's web server answers on
func (p *Server) WorldMapTarget(ctx context.Context) (string, error) {
	sidecar := p.installedWorldMap()
	if sidecar == nil {
		return "", ErrMapNotInstalled
	}
	state := p.worldMapState()
	state.targetLock.Lock()
	cached, at := state.target, state.targetAt
	state.targetLock.Unlock()
	if cached != "" && time.Since(at) < mapTargetCache {
		return cached, nil
	}

	port := sidecar.Port
	if port <= 0 {
		port = mapPort(p.DataToMap(), sidecar.Kind)
	}
	candidates := p.mapCandidates(ctx, port)
	if len(candidates) == 0 {
		return "", ErrMapUnreachable
	}
	target := firstReachable(ctx, candidates, 1500*time.Millisecond)
	if target == "" {
		return "", ErrMapUnreachable
	}
	state.targetLock.Lock()
	state.target, state.targetAt = target, time.Now()
	state.targetLock.Unlock()
	return target, nil
}

// ForgetWorldMapTarget drops the cached address, so the next request looks for the map again
func (p *Server) ForgetWorldMapTarget() {
	state := p.worldMapState()
	state.targetLock.Lock()
	state.target = ""
	state.targetLock.Unlock()
}

// firstReachable dials every candidate at once and returns the first, in the given order of
// preference, that accepts a connection
func firstReachable(ctx context.Context, candidates []string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	results := make([]chan bool, len(candidates))
	for i, address := range candidates {
		results[i] = make(chan bool, 1)
		go func(address string, result chan<- bool) {
			var dialer net.Dialer
			conn, err := dialer.DialContext(ctx, "tcp", address)
			if err == nil {
				_ = conn.Close()
			}
			result <- err == nil
		}(address, results[i])
	}
	for i, result := range results {
		if <-result {
			return candidates[i]
		}
	}
	return ""
}

// worldMapReady is true when the map's web server answers its front page
func (p *Server) worldMapReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	target, err := p.WorldMapTarget(ctx)
	if err != nil {
		return false
	}
	// the map's own web server only speaks plain HTTP, and the request never leaves the node
	probe := url.URL{Scheme: "http", Host: target, Path: "/"}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probe.String(), nil)
	if err != nil {
		return false
	}
	res, err := mapProbeClient.Do(req)
	defer utils.CloseResponse(res)
	if err != nil {
		p.ForgetWorldMapTarget()
		return false
	}
	return res.StatusCode < http.StatusInternalServerError
}

var mapProbeClient = &http.Client{
	Timeout: 3 * time.Second,
	// the front page is enough; following a redirect elsewhere is not needed
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}
