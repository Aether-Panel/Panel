package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"syscall"

	"github.com/SkyPanel/SkyPanel/v3/internal/utils"

	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cast"
)

type Docker struct {
	ImageName     string               `json:"image"`
	Binds         map[string]string    `json:"bindings,omitempty"`
	Network       string               `json:"networkName,omitempty"`
	Ports         []string             `json:"portBindings,omitempty"`
	ContainerRoot string               `json:"containerRoot,omitempty"`
	HostConfig    container.HostConfig `json:"hostConfig,omitempty"`
	Labels        map[string]string    `json:"labels,omitempty"`
	Config        container.Config     `json:"config,omitempty"`

	connection       client.HijackedResponse
	cli              *client.Client
	downloadingImage bool
	statLocker       sync.Mutex
	lastStats        *skypanel.ServerStats
	lastStatTime     time.Time
	lastNetworkRx    uint64
	lastNetworkTx    uint64
	lastNetTime      time.Time
	// disableStdin        bool
	disableSpecialStats bool

	dirSize     int64
	dirSizeTime time.Time
}

var (
	panelNetworkOnce sync.Once
	panelNetworkName string
)

// detectPanelNetwork returns the Docker network the SkyPanel container itself
// is connected to (e.g. "panel_skypanel-network"). Falls back to "bridge" if
// detection fails (running outside Docker, or no socket available).
func detectPanelNetwork() string {
	panelNetworkOnce.Do(func() {
		hostname, _ := os.Hostname()
		if hostname == "" {
			panelNetworkName = "bridge"
			return
		}
		cli, err := client.New(client.FromEnv)
		if err != nil {
			panelNetworkName = "bridge"
			return
		}
		defer cli.Close()
		info, err := cli.ContainerInspect(context.Background(), hostname, client.ContainerInspectOptions{})
		if err != nil {
			panelNetworkName = "bridge"
			return
		}
		for name := range info.Container.NetworkSettings.Networks {
			panelNetworkName = name
			return
		}
		panelNetworkName = "bridge"
	})
	return panelNetworkName
}

// ResolvedPortBindings resolves template port specs against the given variables.
func ResolvedPortBindings(portSpecs []string, variables map[string]interface{}) []string {
	return utils.ReplaceTokensInArr(portSpecs, variables)
}

// ExtraPortBindings returns host bindings for the extra port variables
// (port2, port3, ...) so additional ports assigned to a server are always
// reachable regardless of the template's own portBindings. Both TCP and UDP
// are exposed since game servers commonly listen on both.
func ExtraPortBindings(variables map[string]interface{}) []string {
	var specs []string
	for i := 2; ; i++ {
		key := fmt.Sprintf("port%d", i)
		val, ok := variables[key]
		if !ok {
			break
		}
		p := cast.ToString(val)
		if p == "" || p == "0" {
			continue
		}
		specs = append(specs,
			"0.0.0.0:"+p+":"+p+"/tcp",
			"0.0.0.0:"+p+":"+p+"/udp",
		)
	}
	return specs
}

func (d *Docker) ExecuteAsyncImpl(environment *skypanel.Environment, steps skypanel.ExecutionData) error {
	if d.downloadingImage {
		return skypanel.ErrImageDownloading
	}

	var err error
	var dockerClient *client.Client
	dockerClient, err = d.getClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	// TODO: This logic may not work anymore, it's complicated to use an existing container with install/uninstall
	exists, err := doesContainerExist(ctx, dockerClient, environment.ServerID)
	if err != nil {
		return err
	}

	if exists {
		return errors.New("docker container already exists")
	}

	err = d.createContainer(ctx, environment, steps)
	if err != nil {
		return err
	}

	d.disableSpecialStats = steps.DisableStats
	// d.disableStdin = steps.DisableStdin

	cfg := client.ContainerAttachOptions{
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		Stream: true,
	}

	attachResult, err := dockerClient.ContainerAttach(ctx, environment.ServerID, cfg)
	if err != nil {
		return err
	}
	d.connection = attachResult.HijackedResponse

	environment.Wait.Add(1)

	go func() {
		defer d.connection.Close()
		_, _ = io.Copy(environment.Wrapper, d.connection.Reader)
	}()

	// if !d.disableStdin {
	//	environment.CreateConsoleStdinProxy(steps.StdInConfig, d.connection.Conn)
	//}
	environment.CreateConsoleStdinProxy(steps.StdInConfig, d.connection.Conn)

	environment.Console.Start()

	go d.handleClose(environment, dockerClient, steps.Callback)

	startOpts := client.ContainerStartOptions{}

	_ = environment.StatusTracker.WriteMessage(skypanel.Transmission{
		Message: skypanel.ServerRunning{
			Running:    true,
			Installing: environment.IsInstalling(),
		},
		Type: skypanel.MessageTypeStatus,
	})

	environment.DisplayToConsole(true, "Starting container\n")
	_, err = dockerClient.ContainerStart(ctx, environment.ServerID, startOpts)
	if err != nil {
		return err
	}

	return err
}

func (d *Docker) KillImpl(environment *skypanel.Environment) error {
	running, err := environment.IsRunning()
	if err != nil {
		return err
	}

	if !running {
		return nil
	}

	dockerClient, err := d.getClient()
	if err != nil {
		return err
	}
	_, err = dockerClient.ContainerKill(context.Background(), environment.ServerID, client.ContainerKillOptions{Signal: "SIGKILL"})
	return err
}

func (d *Docker) IsRunningImpl(environment *skypanel.Environment) (bool, error) {
	dockerClient, err := d.getClient()
	if err != nil {
		return false, err
	}

	ctx := context.Background()

	exists, err := doesContainerExist(ctx, dockerClient, environment.ServerID)
	if !exists {
		return false, err
	}

	inspectResult, err := dockerClient.ContainerInspect(ctx, environment.ServerID, client.ContainerInspectOptions{})
	if err != nil {
		return false, err
	}
	return inspectResult.Container.State.Running, nil
}

func (d *Docker) GetStatsImpl(environment *skypanel.Environment) (*skypanel.ServerStats, error) {
	running, err := environment.IsRunning()
	if err != nil {
		return nil, err
	}

	if !running {
		stats := &skypanel.ServerStats{
			CPU:    0,
			Memory: 0,
		}

		if environment.Server.Stats.Type == "jcmd" {
			stats.Jvm = &utils.JvmStats{}
		}

		return stats, nil
	}

	d.statLocker.Lock()
	defer d.statLocker.Unlock()

	// only fetch stats once every 5 seconds, to avoid excessive spam
	if d.lastStatTime.Add(5 * time.Second).After(time.Now()) {
		return d.lastStats, nil
	}

	dockerClient, err := d.getClient()

	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	res, err := dockerClient.ContainerStats(ctx, environment.ServerID, client.ContainerStatsOptions{
		IncludePreviousSample: true,
	})
	defer func() {
		if res.Body != nil {
			utils.Close(res.Body)
		}
	}()
	if err != nil {
		return nil, err
	}

	data := &container.StatsResponse{}
	err = json.NewDecoder(res.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	// for java, we can get some extra data from the jcmd command
	// as such, we'll see if we can

	var totalRx, totalTx uint64
	for _, netStats := range data.Networks {
		totalRx += netStats.RxBytes
		totalTx += netStats.TxBytes
	}

	now := time.Now()
	var rxRate, txRate float64
	if !d.lastNetTime.IsZero() {
		elapsed := now.Sub(d.lastNetTime).Seconds()
		if elapsed > 0 {
			rxRate = (float64(totalRx-d.lastNetworkRx) / elapsed) / 1024
			txRate = (float64(totalTx-d.lastNetworkTx) / elapsed) / 1024
		}
	}
	d.lastNetworkRx = totalRx
	d.lastNetworkTx = totalTx
	d.lastNetTime = now

	if !d.disableSpecialStats && (d.dirSizeTime.IsZero() || time.Since(d.dirSizeTime) > 30*time.Second) {
		d.dirSize = getDirSize(environment.GetRootDirectory())
		d.dirSizeTime = time.Now()
	}

	var maxStorage float64
	if diskVar, ok := environment.Server.Variables["disk"]; ok {
		if limit, err := cast.ToInt64E(diskVar.Value); err == nil && limit > 0 {
			maxStorage = float64(limit) * 1024 * 1024
		}
	}

	stats := &skypanel.ServerStats{
		Memory:     float64(data.MemoryStats.Usage),
		MaxMemory:  float64(data.MemoryStats.Limit),
		CPU:        calculateCPUPercent(data),
		Disk:       float64(d.dirSize),
		MaxStorage: maxStorage,
		NetworkRx:  rxRate,
		NetworkTx:  txRate,
		Running:    true,
	}

	if !d.disableSpecialStats && environment.Server.Stats.Type == "jcmd" {
		cmd, _ := environment.Server.Stats.Metadata["cmd"].(string)
		if cmd == "" {
			cmd = "jcmd"
		}

		r, e := dockerClient.ExecCreate(context.Background(), environment.ServerID, client.ExecCreateOptions{
			AttachStderr: true,
			AttachStdout: true,
			Cmd:          []string{cmd, "1", "GC.heap_info"},
		})

		if e == nil {
			rw, e := dockerClient.ExecAttach(context.Background(), r.ID, client.ExecAttachOptions{
				TTY: false,
			})
			if e != nil {
				logging.Error.Printf("Could not exec JCMD: %s", e.Error())
			} else {
				defer func(z client.HijackedResponse) {
					z.Close()
				}(rw.HijackedResponse)

				jcmdData, err := io.ReadAll(rw.Reader)
				if err != nil {
					logging.Error.Printf("Could not get result of JCMD: %s", err.Error())
				}

				stats.Jvm = utils.ParseJCMDResponse(jcmdData)
			}
		}
		if stats.Jvm == nil {
			stats.Jvm = &utils.JvmStats{}
		}
	}

	d.lastStats = stats
	d.lastStatTime = time.Now()

	return stats, nil
}

func (d *Docker) getClient() (*client.Client, error) {
	var err error
	if d.cli == nil {
		d.cli, err = client.New(client.FromEnv)
	}
	return d.cli, err
}

// CheckBindingsConflict returns an error if any host port referenced in the
// given port specs (after token replacement) is already bound by another
// running container on this node or by a host process. The container
// identified by excludedID (e.g. the server being restarted) is ignored.
func CheckBindingsConflict(portSpecs []string, variables map[string]interface{}, excludedID string) error {
	hostPorts, err := parseHostPorts(portSpecs, variables)
	if err != nil || len(hostPorts) == 0 {
		// If we can't resolve the specs, let Docker report the real error on start.
		return nil
	}

	dockerClient, err := client.New(client.FromEnv)
	if err != nil {
		// Without docker access we can't list containers, so probe the host for
		// any port that is already bound by a non-Docker process.
		for host := range hostPorts {
			if host == 0 {
				continue
			}
			if !hostPortFree(host) {
				return portConflictError(host)
			}
		}
		return nil
	}

	opts := client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters),
	}
	opts.Filters.Add("status", "running")

	existingContainers, err := dockerClient.ContainerList(context.Background(), opts)
	if err != nil {
		logging.Debug.Printf("CheckBindingsConflict: could not list containers: %s", err)
		return nil
	}

	excludedPorts := make(map[uint16]bool)
	for _, cont := range existingContainers.Items {
		names := cont.Names
		if slices.Contains(names, "/"+excludedID) {
			for _, p := range cont.Ports {
				if p.PublicPort != 0 {
					excludedPorts[p.PublicPort] = true
				}
			}
			continue
		}
		for _, p := range cont.Ports {
			host := p.PublicPort
			if host == 0 {
				continue
			}
			if _, conflict := hostPorts[host]; conflict {
				return portConflictError(host)
			}
		}
	}

	// Also detect host ports held by non-Docker processes (e.g. another daemon
	// or a random service), which never show up in the container list. Ports
	// already bound by the excluded (restarting) server must be skipped.
	for host := range hostPorts {
		if excludedPorts[host] {
			continue
		}
		if !hostPortFree(host) {
			return portConflictError(host)
		}
	}

	return nil
}

func portConflictError(host uint16) error {
	return fmt.Errorf(
		"el puerto %d ya está en uso por otro servidor o proceso activo en este nodo. Detén el servicio que lo ocupa o cambia el puerto del servidor antes de iniciarlo",
		host,
	)
}

// hostPortFree reports whether a TCP listener can be bound to the given port
// on the host. The daemon itself may hold the port (SocketOverlay), so only
// "address already in use" failures are treated as occupied.
func hostPortFree(port uint16) bool {
	l, err := net.Listen("tcp", net.JoinHostPort("", cast.ToString(port)))
	if err != nil {
		return !errors.Is(err, syscall.EADDRINUSE)
	}
	_ = l.Close()
	return true
}

func parseHostPorts(portSpecs []string, variables map[string]interface{}) (map[uint16]bool, error) {
	resolved := utils.ReplaceTokensInArr(portSpecs, variables)
	_, bindings, err := parsePortSpecs(resolved)
	if err != nil {
		return nil, err
	}

	hostPorts := make(map[uint16]bool)
	for _, list := range bindings {
		for _, binding := range list {
			if binding.HostPort == "" {
				continue
			}
			if port, err := cast.ToUint16E(binding.HostPort); err == nil {
				hostPorts[port] = true
			}
		}
	}
	return hostPorts, nil
}

func doesContainerExist(ctx context.Context, dockerClient *client.Client, id string) (bool, error) {
	opts := client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters),
	}

	opts.Filters.Add("name", id)

	existingContainers, err := dockerClient.ContainerList(ctx, opts)
	if err != nil {
		return false, err
	}

	for _, v := range existingContainers.Items {
		if slices.Contains(v.Names, "/"+id) {
			return true, nil
		}
	}

	return false, nil
}

func (d *Docker) PullImage(ctx context.Context, environment *skypanel.Environment, imageName string, force bool) error {
	if d.downloadingImage {
		return skypanel.ErrImageDownloading
	}

	if !force {
		exists := false

		parts := strings.SplitN(imageName, ":", 2)
		if len(parts) != 2 {
			imageName += ":latest"
		}

		opts := client.ImageListOptions{
			All:     true,
			Filters: make(client.Filters),
		}
		opts.Filters.Add("reference", imageName)
		listResult, err := d.cli.ImageList(ctx, opts)

		if err != nil {
			return err
		}

		for _, v := range listResult.Items {
			for _, z := range v.RepoTags {
				if z == imageName {
					exists = true
					break
				}
			}
			if exists {
				break
			}
		}

		environment.Log(logging.Debug, "Does image %v exist? %v", imageName, exists)

		if exists {
			return nil
		}
	}

	op := client.ImagePullOptions{}

	environment.Log(logging.Debug, "Downloading image %v", imageName)
	environment.DisplayToConsole(true, "Downloading image for container, please wait\n")

	d.downloadingImage = true
	defer func() {
		d.downloadingImage = false
	}()

	r, err := d.cli.ImagePull(ctx, imageName, op)
	defer utils.Close(r)
	if err != nil {
		return err
	}

	w := &ImageWriter{Parent: environment.ConsoleTracker}
	_, err = io.Copy(w, r)

	if err != nil {
		return err
	}

	environment.Log(logging.Debug, "Downloaded image %v", imageName)
	environment.DisplayToConsole(true, "Downloaded image for container\n")
	return err
}

func (d *Docker) createContainer(ctx context.Context, environment *skypanel.Environment, data skypanel.ExecutionData) error {
	environment.Log(logging.Debug, "Creating container")
	containerRoot := d.ContainerRoot
	if containerRoot == "" {
		containerRoot = "/SkyPanel"
	}

	if runtime.GOOS != "windows" {
		if !filepath.IsAbs(containerRoot) {
			return skypanel.ErrPathNotAbs(containerRoot)
		}
	}

	imageName := utils.ReplaceTokens(d.ImageName, data.Variables)

	err := d.PullImage(ctx, environment, imageName, false)

	if err != nil {
		return err
	}

	cmd, args := utils.SplitArguments(data.Command)

	var cmdSlice []string
	if data.Command != "" {
		cmdSlice = append(cmdSlice, cmd)
	}
	cmdSlice = append(cmdSlice, args...)

	environment.Log(logging.Debug, "Container command: %s\n", cmdSlice)

	labels := map[string]string{
		"skypanel.server": environment.ServerID,
	}

	for k, v := range d.Labels {
		labels[utils.ReplaceTokens(k, data.Variables)] = utils.ReplaceTokens(v, data.Variables)
	}

	c := d.Config
	containerConfig := &c

	// these we need to override
	containerConfig.AttachStderr = true
	containerConfig.AttachStdin = true
	containerConfig.AttachStdout = true
	containerConfig.Tty = true
	containerConfig.OpenStdin = true
	containerConfig.NetworkDisabled = false
	containerConfig.Labels = labels

	// default if it wasn't overridden
	if containerConfig.Image == "" {
		containerConfig.Image = imageName
	}

	if containerConfig.WorkingDir == "" {
		containerConfig.WorkingDir = containerRoot
	}

	// append anything the container config added
	var envVars = make(map[string]string)

	for _, v := range containerConfig.Env {
		key, value, valid := strings.Cut(v, "=")
		if !valid {
			continue
		}
		if strings.HasPrefix(key, "SKYPANEL_") {
			continue
		}
		envVars[key] = value
	}
	envVars["HOME"] = containerRoot
	envVars["TERM"] = "xterm-256color"

	for k, v := range data.Environment {
		envVars[k] = v
	}

	containerConfig.Env = make([]string, 0)
	for k, v := range envVars {
		containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%s", k, utils.ReplaceTokens(v, data.Variables)))
	}

	if len(containerConfig.Entrypoint) == 0 && len(cmdSlice) > 0 {
		containerConfig.Entrypoint = cmdSlice
	}

	if containerConfig.User == "" && runtime.GOOS != "windows" {
		containerConfig.User = fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	}

	var dir string
	if containerMountSource != "" {
		dir = filepath.Join(containerMountSource, "servers", environment.ServerID)
	} else {
		dir = environment.GetRootDirectory()
	}

	// convert root dir to a full path, so we can bind it
	if !filepath.IsAbs(dir) {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return err
		}
	}

	bindDirs := []string{convertToBind(dir) + ":" + containerRoot}

	binaryFolder := config.BinariesFolder.Value()
	if containerMountSource != "" {
		binaryFolder = filepath.Join(containerMountSource, "binaries")
	} else if !filepath.IsAbs(binaryFolder) {
		var ef error
		binaryFolder, ef = filepath.Abs(binaryFolder)
		if ef != nil {
			logging.Error.Printf("Failed to resolve binary folder to absolute path: %s", ef)
			binaryFolder = ""
		}
	}
	if binaryFolder != "" {
		bindDirs = append(bindDirs, convertToBind(binaryFolder)+":"+"/var/lib/SkyPanel/binaries")
	}

	for k, v := range d.Binds {
		bindDirs = append(bindDirs, convertToBind(k)+":"+v)
	}

	baseConfig := d.HostConfig

	hostConfig := &baseConfig
	hostConfig.AutoRemove = true
	if hostConfig.NetworkMode == "" {
		networkName := utils.ReplaceTokens(d.Network, data.Variables)
		if networkName == "" || networkName == "bridge" {
			networkName = detectPanelNetwork()
		}
		hostConfig.NetworkMode = container.NetworkMode(networkName)
	}

	hostConfig.Binds = append(hostConfig.Binds, bindDirs...)

	var exposedPorts network.PortSet
	portSpecs := utils.ReplaceTokensInArr(d.Ports, data.Variables)
	portSpecs = append(portSpecs, ExtraPortBindings(data.Variables)...)
	exposedPorts, hostConfig.PortBindings, err = parsePortSpecs(portSpecs)
	if err != nil {
		return err
	}

	if hostConfig.PortBindings == nil {
		hostConfig.PortBindings = network.PortMap{}
	}

	if data.StdInConfig.Port != "" {
		p := network.MustParsePort(data.StdInConfig.Port + "/tcp")
		if _, exists := hostConfig.PortBindings[p]; !exists {
			hostIP := netip.Addr{}
			if ip, ipErr := netip.ParseAddr("127.0.0.1"); ipErr == nil {
				hostIP = ip
			}
			// we have a port defined for stdin, we need to also export it
			hostConfig.PortBindings[p] = []network.PortBinding{{
				HostIP: hostIP, HostPort: data.StdInConfig.Port,
			}}
		}
	}

	containerConfig.ExposedPorts = exposedPorts
	if containerConfig.ExposedPorts == nil {
		containerConfig.ExposedPorts = network.PortSet{}
	}

	for k := range hostConfig.PortBindings {
		containerConfig.ExposedPorts[k] = struct{}{}
	}

	// Apply CPU limit from variables
	if cpuVal, ok := data.Variables["cpu"]; ok {
		if cpu, err := cast.ToIntE(cpuVal); err == nil && cpu > 0 {
			hostConfig.NanoCPUs = int64(cpu) * 10000000
		}
	}

	// Apply Memory limit from variables (MB to Bytes)
	if memVal, ok := data.Variables["memory"]; ok {
		if mem, err := cast.ToInt64E(memVal); err == nil && mem > 0 {
			hostConfig.Memory = mem * 1024 * 1024
			hostConfig.MemorySwap = mem * 1024 * 1024 // Equal to Memory to disable swap
		}
	}

	networkConfig := &network.NetworkingConfig{}

	// for now, default to linux across the board. This resolves problems that Windows has when you use it and docker
	_, err = d.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:           containerConfig,
		HostConfig:       hostConfig,
		NetworkingConfig: networkConfig,
		Platform:         &v1.Platform{OS: "linux"},
		Name:             environment.ServerID,
	})
	return err
}

func (d *Docker) SendCodeImpl(environment *skypanel.Environment, code int) error {
	running, err := environment.IsRunning()

	if err != nil || !running {
		return err
	}

	dockerClient, err := d.getClient()

	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = dockerClient.ContainerKill(ctx, environment.ServerID, client.ContainerKillOptions{Signal: cast.ToString(code)})
	return err
}

func (d *Docker) GetUIDImpl(_ *skypanel.Environment) int {
	user := d.Config.User
	if user == "" {
		return -1
	}
	return cast.ToInt(strings.Split(user, ":")[0])
}

func (d *Docker) GetGidImpl(_ *skypanel.Environment) int {
	user := d.Config.User
	if user == "" {
		return -1
	}
	return cast.ToInt(strings.Split(user, ":")[1])
}

func (d *Docker) handleClose(environment *skypanel.Environment, dockerClient *client.Client, callback func(int)) {
	exitCode := -1
	waitResult := dockerClient.ContainerWait(context.Background(), environment.ServerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionRemoved,
	})

	select {
	case chanErr := <-waitResult.Error:
		{
			exitCode = -999
			environment.Log(logging.Error, "Error from error channel: %s\n", chanErr.Error())
		}
	case info := <-waitResult.Result:
		{
			exitCode = cast.ToInt(info.StatusCode)
			if info.Error != nil {
				environment.Log(logging.Error, "Error from info channel: %s\n", info.Error.Message)
			}
		}
	}

	environment.LastExitCode = exitCode

	environment.Wait.Done()

	_ = environment.StatusTracker.WriteMessage(skypanel.Transmission{
		Message: skypanel.ServerRunning{
			Running:    false,
			Installing: environment.IsInstalling(),
		},
		Type: skypanel.MessageTypeStatus,
	})

	_ = environment.Console.Close()
	d.disableSpecialStats = false

	if callback != nil {
		callback(exitCode)
	}
}

func calculateCPUPercent(v *container.StatsResponse) float64 {
	// this math is from https://docs.docker.com/reference/api/engine/version/v1.45/#tag/Container/operation/ContainerStats
	cpuDelta := v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage
	systemCPUDelta := v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage
	numCpus := int(v.CPUStats.OnlineCPUs)
	if numCpus == 0 {
		numCpus = len(v.CPUStats.CPUUsage.PercpuUsage)
	}
	return (float64(cpuDelta) / float64(systemCPUDelta)) * float64(numCpus) * 100.0
}

func getDirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func convertToBind(source string) string {
	fullPath, err := filepath.Abs(source)
	if err != nil {
		panic(err)
	}

	fullPath = strings.ReplaceAll(fullPath, "\\", "/")
	fullPath = strings.ReplaceAll(fullPath, ":", "")
	// lowercase first character as that's the drive
	fullPath = strings.ToLower(string(fullPath[0])) + fullPath[1:]
	fullPath = "/" + fullPath
	return fullPath
}

// parsePortSpecs parses a slice of docker port spec strings into the moby
// network.PortSet and network.PortMap types used by container.HostConfig.
func parsePortSpecs(specs []string) (network.PortSet, network.PortMap, error) {
	exposed := network.PortSet{}
	bindings := network.PortMap{}

	for _, spec := range specs {
		for _, part := range strings.Split(spec, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			var proto string
			host := ""
			parts := strings.SplitN(part, "->", 2)
			if len(parts) == 2 {
				host = parts[0]
				part = parts[1]
			}

			if idx := strings.LastIndex(part, "/"); idx != -1 {
				proto = part[idx+1:]
				part = part[:idx]
				if proto == "" {
					proto = "tcp"
				}
			} else {
				proto = "tcp"
			}

			// The container port is everything after the final ":".
			if idx := strings.LastIndex(part, ":"); idx != -1 {
				// Without an explicit "HOST->CONTAINER" separator, interpret the
				// leading part as the host binding using Docker's classic
				// "[IP:]HOST_PORT:CONTAINER_PORT[/PROTO]" syntax (e.g.
				// "0.0.0.0:25565:25565/tcp" from the templates). This keeps
				// both the legacy PufferPanel format and the template format working.
				if len(parts) != 2 {
					host = part[:idx]
				}
				part = part[idx+1:]
			}

			containerPort, err := network.ParsePort(part + "/" + proto)
			if err != nil {
				return nil, nil, err
			}
			exposed[containerPort] = struct{}{}

			if host == "" {
				continue
			}

			hostPort := host
			if idx := strings.LastIndex(host, ":"); idx != -1 {
				hostPort = host[idx+1:]
			}

			hostIP := netip.Addr{}
			if idx := strings.LastIndex(host, ":"); idx != -1 {
				if ip, ipErr := netip.ParseAddr(host[:idx]); ipErr == nil {
					hostIP = ip
				}
			}

			bindings[containerPort] = append(bindings[containerPort], network.PortBinding{
				HostIP:   hostIP,
				HostPort: hostPort,
			})
		}
	}

	return exposed, bindings, nil
}
