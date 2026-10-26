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

// defaultServerNetwork picks the network for containers whose template does not choose one.
// docker.serversNetwork keeps game servers off the panel's network, so code uploaded to a
// server cannot reach the panel or other services there; the database joins both networks.
// Without it, or when that network does not exist, servers join the panel's network as before.
func (d *Docker) defaultServerNetwork(ctx context.Context) string {
	if name := config.DockerServersNetwork.Value(); name != "" {
		if _, err := d.cli.NetworkInspect(ctx, name, client.NetworkInspectOptions{}); err == nil {
			return name
		}
		logging.Error.Printf("Docker network %q from docker.serversNetwork does not exist, using the panel's network", name)
	}
	return detectPanelNetwork()
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
	// a leftover container is never reused; wait for the previous run's to be removed
	exists, err := waitForContainerRemoval(ctx, dockerClient, environment.ServerID)
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
		return stoppedStats(environment), nil
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

	rxRate, txRate := d.networkRates(data)
	d.refreshDirSize(environment)

	stats := &skypanel.ServerStats{
		Memory:     float64(data.MemoryStats.Usage),
		MaxMemory:  float64(data.MemoryStats.Limit),
		CPU:        calculateCPUPercent(data),
		Disk:       float64(d.dirSize),
		MaxStorage: maxStorageBytes(environment),
		NetworkRx:  rxRate,
		NetworkTx:  txRate,
		Running:    true,
	}

	// for java, we can get some extra data from the jcmd command
	if !d.disableSpecialStats && environment.Server.Stats.Type == "jcmd" {
		stats.Jvm = jcmdStats(dockerClient, environment)
		if stats.Jvm == nil {
			stats.Jvm = &utils.JvmStats{}
		}
	}

	d.lastStats = stats
	d.lastStatTime = time.Now()

	return stats, nil
}

// stoppedStats is what a server that is not running reports.
func stoppedStats(environment *skypanel.Environment) *skypanel.ServerStats {
	stats := &skypanel.ServerStats{
		CPU:    0,
		Memory: 0,
	}

	if environment.Server.Stats.Type == "jcmd" {
		stats.Jvm = &utils.JvmStats{}
	}

	return stats
}

// networkRates returns the rx/tx rate in KiB/s since the previous sample and stores
// this sample for the next call. Callers must hold statLocker.
func (d *Docker) networkRates(data *container.StatsResponse) (float64, float64) {
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
	return rxRate, txRate
}

// refreshDirSize walks the server folder at most every 30 seconds, it is slow on big servers.
// Callers must hold statLocker.
func (d *Docker) refreshDirSize(environment *skypanel.Environment) {
	if !d.disableSpecialStats && (d.dirSizeTime.IsZero() || time.Since(d.dirSizeTime) > 30*time.Second) {
		d.dirSize = getDirSize(environment.GetRootDirectory())
		d.dirSizeTime = time.Now()
	}
}

// maxStorageBytes converts the server's "disk" variable (MB) to bytes, 0 when unset.
func maxStorageBytes(environment *skypanel.Environment) float64 {
	if diskVar, ok := environment.Server.Variables["disk"]; ok {
		if limit, err := cast.ToInt64E(diskVar.Value); err == nil && limit > 0 {
			return float64(limit) * 1024 * 1024
		}
	}
	return 0
}

// jcmdStats runs jcmd inside the container to read the JVM heap, nil when it cannot.
func jcmdStats(dockerClient *client.Client, environment *skypanel.Environment) *utils.JvmStats {
	cmd, _ := environment.Server.Stats.Metadata["cmd"].(string)
	if cmd == "" {
		cmd = "jcmd"
	}

	r, e := dockerClient.ExecCreate(context.Background(), environment.ServerID, client.ExecCreateOptions{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          []string{cmd, "1", "GC.heap_info"},
	})
	if e != nil {
		return nil
	}

	rw, e := dockerClient.ExecAttach(context.Background(), r.ID, client.ExecAttachOptions{
		TTY: false,
	})
	if e != nil {
		logging.Error.Printf("Could not exec JCMD: %s", e.Error())
		return nil
	}
	defer func(z client.HijackedResponse) {
		z.Close()
	}(rw.HijackedResponse)

	jcmdData, err := io.ReadAll(rw.Reader)
	if err != nil {
		logging.Error.Printf("Could not get result of JCMD: %s", err.Error())
	}

	return utils.ParseJCMDResponse(jcmdData)
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
		return firstBusyHostPort(hostPorts, func(host uint16) bool { return host == 0 })
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

	excludedPorts, err := containerPortConflicts(existingContainers.Items, hostPorts, excludedID)
	if err != nil {
		return err
	}

	// Also detect host ports held by non-Docker processes (e.g. another daemon
	// or a random service), which never show up in the container list. Ports
	// already bound by the excluded (restarting) server must be skipped.
	return firstBusyHostPort(hostPorts, func(host uint16) bool { return excludedPorts[host] })
}

// containerPortConflicts fails when a running container other than excludedID publishes
// one of hostPorts, and returns the ports the excluded container itself publishes.
func containerPortConflicts(containers []container.Summary, hostPorts map[uint16]bool, excludedID string) (map[uint16]bool, error) {
	excludedPorts := make(map[uint16]bool)
	for _, cont := range containers {
		if slices.Contains(cont.Names, "/"+excludedID) {
			for _, host := range publishedPorts(cont) {
				excludedPorts[host] = true
			}
			continue
		}
		for _, host := range publishedPorts(cont) {
			if _, conflict := hostPorts[host]; conflict {
				return nil, portConflictError(host)
			}
		}
	}
	return excludedPorts, nil
}

// publishedPorts lists the host ports a container publishes
func publishedPorts(cont container.Summary) []uint16 {
	hosts := make([]uint16, 0, len(cont.Ports))
	for _, p := range cont.Ports {
		if p.PublicPort != 0 {
			hosts = append(hosts, p.PublicPort)
		}
	}
	return hosts
}

// firstBusyHostPort probes every host port not skipped and reports the first one in use.
func firstBusyHostPort(hostPorts map[uint16]bool, skip func(uint16) bool) error {
	for host := range hostPorts {
		if skip(host) {
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

// containerRemovalWait is how long a start waits for the previous run's container to go
// away. Containers are created with AutoRemove and Docker can still be removing one when
// the panel hears the game exited, so restarts failed with "docker container already exists".
const containerRemovalWait = 15 * time.Second

func waitForContainerRemoval(ctx context.Context, dockerClient *client.Client, id string) (bool, error) {
	deadline := time.Now().Add(containerRemovalWait)
	for {
		exists, err := doesContainerExist(ctx, dockerClient, id)
		if err != nil || !exists || time.Now().After(deadline) {
			return exists, err
		}
		time.Sleep(250 * time.Millisecond)
	}
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
		parts := strings.SplitN(imageName, ":", 2)
		if len(parts) != 2 {
			imageName += ":latest"
		}

		exists, err := d.imageExists(ctx, imageName)
		if err != nil {
			return err
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

// imageExists reports whether an image tagged exactly imageName is already local.
func (d *Docker) imageExists(ctx context.Context, imageName string) (bool, error) {
	opts := client.ImageListOptions{
		All:     true,
		Filters: make(client.Filters),
	}
	opts.Filters.Add("reference", imageName)
	listResult, err := d.cli.ImageList(ctx, opts)
	if err != nil {
		return false, err
	}

	for _, v := range listResult.Items {
		if slices.Contains(v.RepoTags, imageName) {
			return true, nil
		}
	}
	return false, nil
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

	cmdSlice := commandSlice(data.Command)

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
	containerConfig.Env = containerEnv(containerConfig.Env, containerRoot, data)

	if len(containerConfig.Entrypoint) == 0 && len(cmdSlice) > 0 {
		containerConfig.Entrypoint = cmdSlice
	}

	if containerConfig.User == "" && runtime.GOOS != "windows" {
		containerConfig.User = fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	}

	bindDirs, err := d.serverBinds(environment, containerRoot)
	if err != nil {
		return err
	}

	baseConfig := d.HostConfig

	hostConfig := &baseConfig
	hostConfig.AutoRemove = true
	if hostConfig.NetworkMode == "" {
		hostConfig.NetworkMode = container.NetworkMode(d.networkName(ctx, data.Variables))
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
		addStdinPortBinding(hostConfig.PortBindings, data.StdInConfig.Port)
	}

	containerConfig.ExposedPorts = exposedPorts
	if containerConfig.ExposedPorts == nil {
		containerConfig.ExposedPorts = network.PortSet{}
	}

	for k := range hostConfig.PortBindings {
		containerConfig.ExposedPorts[k] = struct{}{}
	}

	applyResourceLimits(hostConfig, data.Variables)

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

// commandSlice splits the template command into the container entrypoint.
func commandSlice(command string) []string {
	cmd, args := utils.SplitArguments(command)

	var cmdSlice []string
	if command != "" {
		cmdSlice = append(cmdSlice, cmd)
	}
	return append(cmdSlice, args...)
}

// containerEnv merges the image config env (minus SKYPANEL_ keys), the panel defaults
// and the execution env, replacing tokens in the values.
func containerEnv(configEnv []string, containerRoot string, data skypanel.ExecutionData) []string {
	var envVars = make(map[string]string)

	for _, v := range configEnv {
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

	env := make([]string, 0)
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, utils.ReplaceTokens(v, data.Variables)))
	}
	return env
}

// serverBinds returns the volume binds: the server folder, the shared binaries folder
// and the template's own binds.
func (d *Docker) serverBinds(environment *skypanel.Environment, containerRoot string) ([]string, error) {
	var dir string
	if containerMountSource != "" {
		dir = filepath.Join(containerMountSource, "servers", environment.ServerID)
	} else {
		dir = environment.GetRootDirectory()
	}

	// convert root dir to a full path, so we can bind it
	if !filepath.IsAbs(dir) {
		var err error
		dir, err = filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
	}

	bindDirs := []string{convertToBind(dir) + ":" + containerRoot}

	if binaryFolder := binariesBindSource(); binaryFolder != "" {
		bindDirs = append(bindDirs, convertToBind(binaryFolder)+":"+"/var/lib/SkyPanel/binaries")
	}

	for k, v := range d.Binds {
		bindDirs = append(bindDirs, convertToBind(k)+":"+v)
	}
	return bindDirs, nil
}

// binariesBindSource is the host path of the binaries folder, empty when it cannot be resolved.
func binariesBindSource() string {
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
	return binaryFolder
}

// networkName resolves the template network, falling back to the default servers network.
func (d *Docker) networkName(ctx context.Context, variables map[string]interface{}) string {
	networkName := utils.ReplaceTokens(d.Network, variables)
	if networkName == "" || networkName == "bridge" {
		networkName = d.defaultServerNetwork(ctx)
	}
	return networkName
}

// addStdinPortBinding publishes the stdin port on localhost unless the template already binds it.
func addStdinPortBinding(bindings network.PortMap, port string) {
	p := network.MustParsePort(port + "/tcp")
	if _, exists := bindings[p]; exists {
		return
	}
	hostIP := netip.Addr{}
	if ip, ipErr := netip.ParseAddr("127.0.0.1"); ipErr == nil {
		hostIP = ip
	}
	// we have a port defined for stdin, we need to also export it
	bindings[p] = []network.PortBinding{{
		HostIP: hostIP, HostPort: port,
	}}
}

// applyResourceLimits sets the CPU, memory and disk I/O limits from the server variables.
func applyResourceLimits(hostConfig *container.HostConfig, variables map[string]interface{}) {
	// Disk I/O share against the node's other servers, 10 to 1000 like Pterodactyl (500 is
	// an even share). Docker applies it where the host's I/O scheduler supports weights.
	if ioVal, ok := variables["io"]; ok {
		if weight, err := cast.ToUint16E(ioVal); err == nil && weight > 0 {
			hostConfig.BlkioWeight = min(max(weight, 10), 1000)
		}
	}

	// Apply CPU limit from variables
	if cpuVal, ok := variables["cpu"]; ok {
		if cpu, err := cast.ToIntE(cpuVal); err == nil && cpu > 0 {
			hostConfig.NanoCPUs = int64(cpu) * 10000000
		}
	}

	// Apply Memory limit from variables (MB to Bytes)
	if memVal, ok := variables["memory"]; ok {
		if mem, err := cast.ToInt64E(memVal); err == nil && mem > 0 {
			hostConfig.Memory = mem * 1024 * 1024
			hostConfig.MemorySwap = mem * 1024 * 1024 // Equal to Memory to disable swap
		}
	}
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

			containerPort, host, err := parsePortSpec(part)
			if err != nil {
				return nil, nil, err
			}
			exposed[containerPort] = struct{}{}

			if host == "" {
				continue
			}

			bindings[containerPort] = append(bindings[containerPort], hostPortBinding(host))
		}
	}

	return exposed, bindings, nil
}

// parsePortSpec parses one "[HOST->]CONTAINER[/PROTO]" or "[IP:]HOST_PORT:CONTAINER_PORT[/PROTO]"
// spec into the container port and the host part ("" when not published).
func parsePortSpec(part string) (network.Port, string, error) {
	host := ""
	parts := strings.SplitN(part, "->", 2)
	if len(parts) == 2 {
		host = parts[0]
		part = parts[1]
	}

	proto := "tcp"
	if idx := strings.LastIndex(part, "/"); idx != -1 {
		if p := part[idx+1:]; p != "" {
			proto = p
		}
		part = part[:idx]
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
	return containerPort, host, err
}

// hostPortBinding turns "[IP:]PORT" into a binding; an unparsable IP binds all interfaces.
func hostPortBinding(host string) network.PortBinding {
	hostPort := host
	hostIP := netip.Addr{}
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		hostPort = host[idx+1:]
		if ip, ipErr := netip.ParseAddr(host[:idx]); ipErr == nil {
			hostIP = ip
		}
	}

	return network.PortBinding{
		HostIP:   hostIP,
		HostPort: hostPort,
	}
}
