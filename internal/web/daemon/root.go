package daemon

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers/docker"
	"github.com/SkyPanel/SkyPanel/v3/internal/update"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/moby/moby/client"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

func RegisterDaemonRoutes(e *gin.RouterGroup) {
	e.GET("", getStatusGET)
	e.HEAD("", getStatusHEAD)
	e.Handle("OPTIONS", "", response.CreateOptions("GET", "HEAD"))

	e.GET("features", getFeatures)
	e.Handle("OPTIONS", "features", response.CreateOptions("GET"))

	// host details (hostname, disks, CPU) are only for the panel, and each call blocks for a second
	e.GET("system", middleware.ValidateJWT, getSystemInfo)
	e.Handle("OPTIONS", "system", response.CreateOptions("GET"))

	// host ports the containers running on this node publish, for the panel's port checks
	e.GET("ports", middleware.ValidateJWT, getPublishedPorts)
	e.Handle("OPTIONS", "ports", response.CreateOptions("GET"))

	e.POST("update", middleware.ValidateJWT, daemonUpdate)
	e.Handle("OPTIONS", "update", response.CreateOptions("POST"))

	e.GET("update-status", middleware.ValidateJWT, daemonUpdateStatus)
	e.Handle("OPTIONS", "update-status", response.CreateOptions("GET"))

	RegisterServerRoutes(e)
}

// @Summary Update this node
// @Description Starts an update of the host this node runs on by spawning an ephemeral container.
// @Success 200 {object} nil
// @Tags Daemon Root
// @Router /daemon/update [post]
// @Security OAuth2Application[none]
func daemonUpdate(c *gin.Context) {
	containerID, err := update.Trigger()
	if err != nil {
		logging.Error.Printf("Failed to trigger node update: %v", err)
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	logging.Info.Printf("Node update container started with ID %s", containerID)
	c.JSON(http.StatusOK, gin.H{"message": "Update initiated on this node.", "containerId": containerID})
}

// @Summary Get node update status
// @Description Returns the update status, exit code and log tail of this node.
// @Success 200 {object} update.State
// @Tags Daemon Root
// @Router /daemon/update-status [get]
// @Security OAuth2Application[none]
func daemonUpdateStatus(c *gin.Context) {
	c.JSON(http.StatusOK, update.Status())
}

// @Summary Check daemon status
// @Description Check to see if the daemon is online or not
// @Success 200 {object} skypanel.DaemonRunning
// @Tags Daemon Root
// @Router /daemon [get]
// @Security OAuth2Application[none]
func getStatusGET(c *gin.Context) {
	c.JSON(http.StatusOK, &skypanel.DaemonRunning{Message: "daemon is running"})
}

// @Summary Check daemon status
// @Description Check to see if the daemon is online or not
// @Success 204 {object} nil
// @Tags Daemon Root
// @Router /daemon [head]
// @Security OAuth2Application[none]
func getStatusHEAD(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// @Summary Get features of the node
// @Description Gets the features that the node supports, like it's OS and environments
// @Success 200 {object} Features
// @Tags Daemon Root
// @Router /daemon/features [get]
// @Security OAuth2Application[none]
func getFeatures(c *gin.Context) {
	features := make([]string, 0)

	envs := servers.GetSupportedEnvironments()

	if testDocker() {
		features = append(features, "docker")
	}

	if config.DockerDisallowHost.Value() {
		for _, env := range hostEnvironments {
			envs = utils.Remove(envs, env)
		}
	}

	c.JSON(http.StatusOK, Features{Features: features, Environments: envs, OS: runtime.GOOS, Arch: runtime.GOARCH, Version: skypanel.Version})
}

// hostEnvironments run server processes directly on the node instead of in a container
var hostEnvironments = []string{"host", "tty", "standard", "bubblewrap"}

// checkEnvironmentAllowed rejects environments that docker.disallowHost turns off, so the
// setting is enforced when a server is created or redefined, not only hidden from the list
func checkEnvironmentAllowed(envType string) error {
	if !config.DockerDisallowHost.Value() {
		return nil
	}
	for _, env := range hostEnvironments {
		if envType == env {
			return fmt.Errorf("environment %q is disabled on this node (docker.disallowHost)", envType)
		}
	}
	return nil
}

func testDocker() bool {
	d, err := client.New(client.FromEnv)
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = d.Ping(ctx, client.PingOptions{})
	return err == nil
}

type Features struct {
	Features     []string `json:"features"`
	Environments []string `json:"environments"`
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
	Version      string   `json:"version"`
} // @name Features

type DiskInfo struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"usedPercent"`
} // @name DiskInfo

type SystemInfo struct {
	Hostname    string  `json:"hostname"`
	OS          string  `json:"os"`
	Platform    string  `json:"platform"`
	PlatformVer string  `json:"platformVersion"`
	Arch        string  `json:"arch"`
	CPUModel    string  `json:"cpuModel"`
	CPUCores    int     `json:"cpuCores"`
	CPUThreads  int     `json:"cpuThreads"`
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryTotal uint64  `json:"memoryTotal"`
	MemoryUsed  uint64  `json:"memoryUsed"`
	MemoryFree  uint64  `json:"memoryFree"`
	// MemoryAvailable is what can still be given out without swapping (free plus caches)
	MemoryAvailable uint64      `json:"memoryAvailable"`
	Disks           []*DiskInfo `json:"disks"`
	// ServersDisk is the disk that holds the servers' folders, whichever partition it is
	ServersDisk      *servers.NodeDisk `json:"serversDisk,omitempty"`
	Uptime           uint64            `json:"uptime"`
	NetworkBytesSent uint64            `json:"networkBytesSent"`
	NetworkBytesRecv uint64            `json:"networkBytesRecv"`
} // @name SystemInfo

// @Summary Get system information
// @Description Gets detailed system information including CPU, memory, and disk
// @Success 200 {object} SystemInfo
// @Tags Daemon Root
// @Router /daemon/system [get]
// @Security OAuth2Application[none]
func getSystemInfo(c *gin.Context) {
	sysInfo := &SystemInfo{
		OS:    runtime.GOOS,
		Arch:  runtime.GOARCH,
		Disks: make([]*DiskInfo, 0),
	}

	// Get hostname
	if hostInfo, err := host.Info(); err == nil {
		sysInfo.Hostname = hostInfo.Hostname
		sysInfo.Platform = hostInfo.Platform
		sysInfo.PlatformVer = hostInfo.PlatformVersion
		sysInfo.Uptime = hostInfo.Uptime
	}

	// Get CPU info
	if cpuInfo, err := cpu.Info(); err == nil && len(cpuInfo) > 0 {
		sysInfo.CPUModel = cpuInfo[0].ModelName
		sysInfo.CPUCores = int(cpuInfo[0].Cores)
	}

	// Get logical CPU count (threads)
	if logicalCount, err := cpu.Counts(true); err == nil {
		sysInfo.CPUThreads = logicalCount
	}

	// Get physical CPU count (cores)
	if physicalCount, err := cpu.Counts(false); err == nil && physicalCount > 0 {
		sysInfo.CPUCores = physicalCount
	}

	// Get memory info
	if memInfo, err := mem.VirtualMemory(); err == nil {
		sysInfo.MemoryTotal = memInfo.Total
		sysInfo.MemoryUsed = memInfo.Used
		sysInfo.MemoryFree = memInfo.Free
		sysInfo.MemoryAvailable = memInfo.Available
	}

	sysInfo.CPUUsage = currentCPUUsage()

	// Get disk info
	sysInfo.Disks = append(sysInfo.Disks, collectDiskInfo()...)
	if serversDisk, err := servers.NodeDiskUsage(); err == nil {
		sysInfo.ServersDisk = serversDisk
	}

	// Get network statistics
	sysInfo.NetworkBytesSent, sysInfo.NetworkBytesRecv = totalNetworkBytes()

	c.JSON(http.StatusOK, sysInfo)
}

// getPublishedPorts lists the host ports published by the containers running on the node, each
// with the container's name: the panel's own (panel, SFTP, database, Adminer) and the servers'.
func getPublishedPorts(c *gin.Context) {
	ports, err := docker.PublishedHostPorts(c.Request.Context())
	if err != nil {
		// no Docker on this node: nothing published, the panel still checks its own records
		ports = map[uint16]string{}
	}
	c.JSON(http.StatusOK, gin.H{"ports": ports})
}

// The node's CPU usage is sampled in the background over two-second windows, so /system answers
// at once with the latest reading. Measuring it inside the request held every call for a second,
// and the dashboard, the nodes pages and the new-server wizard ask for it every few seconds.
var cpuSampler struct {
	once  sync.Once
	ready atomic.Bool
	value atomic.Uint64 // math.Float64bits of the percentage
}

func currentCPUUsage() float64 {
	cpuSampler.once.Do(func() { go sampleCPU() })
	if !cpuSampler.ready.Load() {
		// first call since the daemon started: a short reading instead of none
		if percent, err := cpu.Percent(250*time.Millisecond, false); err == nil && len(percent) > 0 {
			return percent[0]
		}
		return 0
	}
	return math.Float64frombits(cpuSampler.value.Load())
}

func sampleCPU() {
	defer skypanel.Recover()
	for {
		percent, err := cpu.Percent(2*time.Second, false)
		if err != nil || len(percent) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}
		cpuSampler.value.Store(math.Float64bits(percent[0]))
		cpuSampler.ready.Store(true)
	}
}

// collectDiskInfo reports the usage of every mounted partition that can be read
func collectDiskInfo() []*DiskInfo {
	disks := make([]*DiskInfo, 0)
	partitions, err := disk.Partitions(false)
	if err != nil {
		return disks
	}
	for _, partition := range partitions {
		if usage, err := disk.Usage(partition.Mountpoint); err == nil {
			diskInfo := &DiskInfo{
				Path:        partition.Mountpoint,
				Total:       usage.Total,
				Used:        usage.Used,
				Free:        usage.Free,
				UsedPercent: usage.UsedPercent,
			}
			disks = append(disks, diskInfo)
		}
	}
	return disks
}

// totalNetworkBytes sums the bytes sent and received over all network
// interfaces; zero when the counters cannot be read
func totalNetworkBytes() (sent, recv uint64) {
	netStats, err := net.IOCounters(false)
	if err != nil {
		return 0, 0
	}
	for _, stat := range netStats {
		sent += stat.BytesSent
		recv += stat.BytesRecv
	}
	return sent, recv
}
