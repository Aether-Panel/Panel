package daemon

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/config"
	"github.com/SkyPanel/SkyPanel/v3/response"
	"github.com/SkyPanel/SkyPanel/v3/servers"
	"github.com/SkyPanel/SkyPanel/v3/utils"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
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

	e.GET("system", getSystemInfo)
	e.Handle("OPTIONS", "system", response.CreateOptions("GET"))

	RegisterServerRoutes(e)
}

// @Summary Check daemon status
// @Description Check to see if the daemon is online or not
// @Success 200 {object} SkyPanel.DaemonRunning
// @Router /daemon [get]
// @Security OAuth2Application[none]
func getStatusGET(c *gin.Context) {
	c.JSON(http.StatusOK, &SkyPanel.DaemonRunning{Message: "daemon is running"})
}

// @Summary Check daemon status
// @Description Check to see if the daemon is online or not
// @Success 204 {object} nil
// @Router /daemon [head]
// @Security OAuth2Application[none]
func getStatusHEAD(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// @Summary Get features of the node
// @Description Gets the features that the node supports, like it's OS and environments
// @Success 200 {object} Features
// @Router /daemon/features [get]
// @Security OAuth2Application[none]
func getFeatures(c *gin.Context) {
	features := make([]string, 0)

	envs := servers.GetSupportedEnvironments()

	if testDocker() {
		features = append(features, "docker")
	}

	if config.DockerDisallowHost.Value() {
		envs = utils.Remove(envs, "host")
		envs = utils.Remove(envs, "tty")
		envs = utils.Remove(envs, "standard")
		envs = utils.Remove(envs, "bubblewrap")
	}

	c.JSON(http.StatusOK, Features{Features: features, Environments: envs, OS: runtime.GOOS, Arch: runtime.GOARCH, Version: SkyPanel.Version})
}

func testDocker() bool {
	d, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = d.Ping(ctx)
	return err == nil
}

type Features struct {
	Features     []string `json:"features"`
	Environments []string `json:"environments"`
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
	Version      string   `json:"version"`
} //@name Features

type DiskInfo struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"usedPercent"`
} //@name DiskInfo

type SystemInfo struct {
	Hostname         string      `json:"hostname"`
	OS               string      `json:"os"`
	Platform         string      `json:"platform"`
	PlatformVer      string      `json:"platformVersion"`
	Arch             string      `json:"arch"`
	CPUModel         string      `json:"cpuModel"`
	CPUCores         int         `json:"cpuCores"`
	CPUThreads       int         `json:"cpuThreads"`
	CPUUsage         float64     `json:"cpuUsage"`
	MemoryTotal      uint64      `json:"memoryTotal"`
	MemoryUsed       uint64      `json:"memoryUsed"`
	MemoryFree       uint64      `json:"memoryFree"`
	Disks            []*DiskInfo `json:"disks"`
	Uptime           uint64      `json:"uptime"`
	NetworkBytesSent uint64      `json:"networkBytesSent"`
	NetworkBytesRecv uint64      `json:"networkBytesRecv"`
} //@name SystemInfo

// @Summary Get system information
// @Description Gets detailed system information including CPU, memory, and disk
// @Success 200 {object} SystemInfo
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
	}

	// Get CPU usage (percentage)
	if cpuPercent, err := cpu.Percent(time.Second, false); err == nil && len(cpuPercent) > 0 {
		sysInfo.CPUUsage = cpuPercent[0]
	}

	// Get disk info
	if partitions, err := disk.Partitions(false); err == nil {
		for _, partition := range partitions {
			if usage, err := disk.Usage(partition.Mountpoint); err == nil {
				diskInfo := &DiskInfo{
					Path:        partition.Mountpoint,
					Total:       usage.Total,
					Used:        usage.Used,
					Free:        usage.Free,
					UsedPercent: usage.UsedPercent,
				}
				sysInfo.Disks = append(sysInfo.Disks, diskInfo)
			}
		}
	}

	// Get network statistics
	if netStats, err := net.IOCounters(false); err == nil && len(netStats) > 0 {
		// Sum all network interfaces
		var totalBytesSent uint64 = 0
		var totalBytesRecv uint64 = 0
		for _, stat := range netStats {
			totalBytesSent += stat.BytesSent
			totalBytesRecv += stat.BytesRecv
		}
		sysInfo.NetworkBytesSent = totalBytesSent
		sysInfo.NetworkBytesRecv = totalBytesRecv
	}

	c.JSON(http.StatusOK, sysInfo)
}
