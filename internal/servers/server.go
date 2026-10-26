package servers

import (
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/conditions"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/offsite"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gofrs/uuid/v5"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/spf13/cast"
)

// messages for a failed uninstall
const (
	uninstallErrorFmt  = "Error uninstalling server: %s"
	uninstallFailedMsg = "Failed to uninstall server\n"
)

type Server struct {
	skypanel.DaemonServer
	skypanel.Server

	CrashCounter       int                   `json:"-"`
	RunningEnvironment *skypanel.Environment `json:"-"`
	Scheduler          *Scheduler            `json:"-"`
	stopChan           chan bool
	waitForConsole     sync.Locker
	fileServer         files.FileServer
	// backupState is backupIdle, backupRunning or restoreRunning. One atomic value, so two
	// requests cannot both pass the "nothing in progress" check and run at the same time
	backupState atomic.Int32
	// stopRequested is set when someone stops or kills the server, so its exit is not
	// mistaken for a crash (or answered with an automatic restart)
	stopRequested atomic.Bool
	// exitHandled is set once the last exit went through afterExit (a stop someone asked for,
	// a clean exit, or a crash that was reported), so the admin channel's offline alert only
	// covers servers that vanished without any of those
	exitHandled atomic.Bool
	// startedAt is when the current run began, used to forgive old crashes
	startedAt     atomic.Int64
	keepAlive     *time.Ticker
	keepAliveChan chan bool
	// disk is the folder size the disk limit is checked against
	disk diskUsage
}

var queue *list.List
var lock = sync.Mutex{}
var startQueueTicker, statTicker, systemStatusTicker *time.Ticker
var running = false

// Tracking de estado anterior para alertas
var serverStateTracking = make(map[string]*serverState)
var stateTrackingLock = sync.RWMutex{}

type serverState struct {
	wasRunning bool
	lastStats  *skypanel.ServerStats
	lastAlert  map[string]time.Time // Para evitar spam de alertas
	// offlineSince is when the server was seen stopped without a known reason; the admin
	// alert waits a little in case the stop or crash is still being processed
	offlineSince time.Time
}

// offlineAlertDelay gives afterExit time to explain an exit before the admin is told
const offlineAlertDelay = 15 * time.Second

var ErrServerTypeRequired = errors.New("server type is required")
var ErrEnvironmentTypeRequired = errors.New("environment type is required")

func InitService() {
	queue = list.New()
	running = true
	go processQueue()
	go processStats()
	go processPlayers()
	go processSystemStatus()
	// Trackear uptime inicialmente para todos los servidores
	go trackUptimeForAllServers()
}

func StartViaService(p *Server) {
	lock.Lock()
	defer func() {
		lock.Unlock()
	}()

	if running {
		queue.PushBack(p)
	}
}

func ShutdownService() {
	if !running {
		return
	}

	lock.Lock()
	defer func() {
		lock.Unlock()
	}()

	running = false
	SaveAllMetrics()
	// the tickers are created by their goroutines, so a very early shutdown can find them unset
	if startQueueTicker != nil {
		startQueueTicker.Stop()
	}
	if statTicker != nil {
		statTicker.Stop()
	}
	if systemStatusTicker != nil {
		systemStatusTicker.Stop()
	}
}

func processQueue() {
	startQueueTicker = time.NewTicker(time.Second)
	for range startQueueTicker.C {
		lock.Lock()
		next := queue.Front()
		if next != nil {
			queue.Remove(next)
		}
		lock.Unlock()
		if next == nil {
			continue
		}
		program := next.Value.(*Server)
		if run, _ := program.IsRunning(); !run {
			err := program.Start()
			if err != nil {
				logging.Error.Printf("[%s] Error starting server: %s", program.ID(), err)
			}
		}
	}
}

// playerSampleEvery is how often the player count of running servers goes into their history
const playerSampleEvery = time.Minute

func processPlayers() {
	ticker := time.NewTicker(playerSampleEvery)
	for now := range ticker.C {
		if !running {
			ticker.Stop()
			return
		}
		samplePlayers(now)
	}
}

func processStats() {
	statTicker = time.NewTicker(5 * time.Second)
	for range statTicker.C {
		SendStatsForServers()
		// También trackear uptime aunque no haya stats disponibles
		go trackUptimeForAllServers()
	}
}

// uptimeTracking is set while a round of uptime tracking runs, so a slow database does not
// make the next tick overlap it and record a server twice
var uptimeTracking atomic.Bool

// trackUptimeForAllServers is the only place uptime is recorded. It only runs where the
// panel runs: a daemon-only node has no panel database to write to.
func trackUptimeForAllServers() {
	if !config.PanelEnabled.Value() || !uptimeTracking.CompareAndSwap(false, true) {
		return
	}
	defer uptimeTracking.Store(false)

	for _, server := range GetAll() {
		trackUptime(server)
	}
}

func processSystemStatus() {
	systemStatusTicker = time.NewTicker(1 * time.Minute)
	for range systemStatusTicker.C {
		go sendSystemStatusToDiscord()
	}
}

func sendSystemStatusToDiscord() {
	// the report reads the panel database, which a daemon-only node does not have
	if !running || !config.PanelEnabled.Value() {
		return
	}

	ds := services.GetDiscordService()
	if ds == nil {
		return
	}

	allServers := GetAll()
	if len(allServers) == 0 {
		return
	}

	// Obtener información de los nodos y servidores desde la base de datos
	db, err := database.GetConnection()
	if err != nil {
		logging.Error.Printf("Error getting database connection for node status: %s", err)
		return
	}

	ss := &services.Server{DB: db}
	ns := &services.Node{DB: db}
	nodes, err := ns.GetAll()
	if err != nil {
		logging.Error.Printf("Error getting nodes from database: %s", err)
		return
	}

	// Crear un mapa de servidor ID a node ID desde la base de datos
	serverNodeMap := serverNodeIDs(ss, allServers)

	serverInfos := make([]services.ServerInfo, 0, len(allServers))
	nodeServerMap := make(map[uint][]services.ServerInfo)

	for _, server := range allServers {
		serverInfo := systemStatusInfo(server)
		serverInfos = append(serverInfos, serverInfo)

		// Agrupar por nodo
		nodeID := serverNodeMap[server.ID()]
		nodeServerMap[nodeID] = append(nodeServerMap[nodeID], serverInfo)
	}

	// Enviar el reporte del sistema a Discord
	err = ds.SendSystemStatus(serverInfos)
	if err != nil {
		logging.Error.Printf("Error sending system status to Discord: %s", err)
	}

	// Enviar estado de cada nodo a Discord
	for _, node := range nodes {
		sendNodeStatusToDiscord(ds, node, nodeServerMap[node.ID])
	}
}

// serverNodeIDs maps each server to its node from the database; servers missing there,
// or without a node, count as the local node (0)
func serverNodeIDs(ss *services.Server, allServers []*Server) map[string]uint {
	serverNodeMap := make(map[string]uint)
	for _, server := range allServers {
		serverModel, err := ss.Get(server.ID())
		if err != nil {
			// Si no se encuentra en la BD, asumimos nodo local (0)
			serverNodeMap[server.ID()] = 0
			continue
		}
		nodeID := serverModel.NodeID
		if nodeID == 0 || serverModel.RawNodeID == nil {
			nodeID = 0 // Nodo local
		}
		serverNodeMap[server.ID()] = nodeID
	}
	return serverNodeMap
}

// systemStatusInfo is one server's line in the system status report
func systemStatusInfo(server *Server) services.ServerInfo {
	isRunning, err := server.IsRunning()
	if err != nil {
		logging.Error.Printf("[%s] Error checking server status for system status: %s", server.ID(), err)
		isRunning = false
	}

	serverName := server.Server.Display
	if serverName == "" {
		serverName = server.ID()
	}

	serverInfo := services.ServerInfo{
		Name:      serverName,
		ID:        server.ID(),
		IsRunning: isRunning,
		CPU:       0.0,
		Memory:    0.0,
	}

	// Obtener estadísticas si el servidor está corriendo
	if isRunning {
		stats, err := server.GetEnvironment().GetStats()
		if err == nil && stats != nil {
			serverInfo.CPU = stats.CPU
			serverInfo.Memory = stats.Memory
		}
	}
	return serverInfo
}

func sendNodeStatusToDiscord(ds *services.DiscordService, node *models.Node, serversForNode []services.ServerInfo) {
	totalServers := len(serversForNode)
	onlineServers := 0
	offlineServers := 0

	for _, info := range serversForNode {
		if info.IsRunning {
			onlineServers++
		} else {
			offlineServers++
		}
	}

	nodeName := node.Name
	if nodeName == "" || node.IsLocal() {
		nodeName = "Nodo Local"
	}

	// Obtener información del sistema
	cpuModel, cpuCores, cpuThreads, cpuGhz := hostCPUSummary()

	// Obtener información de memoria
	memInfo, err := mem.VirtualMemory()
	memTotal := uint64(0)
	memUsed := uint64(0)
	if err == nil {
		memTotal = memInfo.Total
		memUsed = memInfo.Used
	}

	// Sistema operativo - obtener información detallada como neofetch/fastfetch
	osName := getOSName()

	err = ds.SendNodeStatus(totalServers, onlineServers, offlineServers, nodeName, node.PublicHost, node.PublicPort, cpuModel, cpuCores, cpuThreads, cpuGhz, memTotal, memUsed, osName)
	if err != nil {
		logging.Error.Printf("Error sending node status to Discord for node %s: %s", nodeName, err)
	}
}

// hostCPUSummary returns the CPU model, physical cores, threads and GHz of this machine
func hostCPUSummary() (string, int, int, float64) {
	cpuInfo, err := cpu.Info()
	haveInfo := err == nil && len(cpuInfo) > 0
	cpuThreads := int(runtime.NumCPU()) // Núcleos lógicos (hilos) - esto siempre es correcto

	cpuCores := 0
	// Obtener cores físicos leyendo /proc/cpuinfo en Linux
	if runtime.GOOS == "linux" {
		cpuCores = linuxPhysicalCores()
	}

	// Si no se pudo obtener cores físicos, usar cpu.Info como fallback
	if cpuCores == 0 {
		cpuCores = cpuInfoCores(cpuInfo, haveInfo, cpuThreads)
	}

	// Obtener modelo y frecuencia del CPU
	cpuModel := "Desconocido"
	cpuGhz := 0.0
	if haveInfo {
		cpuModel = cpuInfo[0].ModelName
		for _, info := range cpuInfo {
			if info.Mhz > 0 {
				cpuGhz = info.Mhz / 1000.0 // Convertir MHz a GHz
				break
			}
		}
	}
	return cpuModel, cpuCores, cpuThreads, cpuGhz
}

// cpuInfoCores adds up the cores cpu.Info reports, or guesses threads / 2 without it
func cpuInfoCores(cpuInfo []cpu.InfoStat, haveInfo bool, cpuThreads int) int {
	if !haveInfo {
		// Fallback final: usar hilos / 2 (asume hyperthreading)
		cpuCores := cpuThreads / 2
		if cpuCores == 0 {
			cpuCores = 1
		}
		return cpuCores
	}

	// Para sistemas con múltiples CPUs físicos, sumar los cores
	totalCores := int(0)
	for _, info := range cpuInfo {
		totalCores += int(info.Cores)
	}
	if totalCores > 0 {
		return totalCores
	}
	return 0
}

// linuxPhysicalCores counts the physical cores in /proc/cpuinfo, 0 when it cannot tell
func linuxPhysicalCores() int {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0
	}

	// /proc/cpuinfo está organizado en bloques separados por líneas vacías
	// Cada bloque representa un procesador lógico (thread)
	physicalCores := make(map[string]bool)
	coresPerSocket := make(map[string]int)

	// Dividir en bloques (cada bloque es un procesador lógico)
	blocks := strings.Split(string(data), "\n\n")
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		physicalID, coreID := parseCPUInfoBlock(block, coresPerSocket)

		// Si tenemos physical id y core id, crear clave única para core físico
		if physicalID != "" && coreID != "" {
			physicalCores[physicalID+"-"+coreID] = true
		}
	}

	// Si encontramos cores físicos únicos, usarlos
	if len(physicalCores) > 0 {
		return len(physicalCores)
	}

	// Calcular: sumar cores de todos los sockets
	totalCores := 0
	for _, cores := range coresPerSocket {
		totalCores += cores
	}
	if totalCores > 0 {
		return totalCores
	}
	return 0
}

// parseCPUInfoBlock reads one processor block of /proc/cpuinfo, recording its socket's
// "cpu cores" in coresPerSocket
func parseCPUInfoBlock(block string, coresPerSocket map[string]int) (physicalID, coreID string) {
	// Procesar cada línea del bloque
	for _, line := range strings.Split(block, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch key {
		case "physical id":
			physicalID = value
		case "core id":
			coreID = value
		case "cpu cores":
			if physicalID == "" {
				continue
			}
			if val, err := cast.ToIntE(value); err == nil {
				coresPerSocket[physicalID] = val
			}
		}
	}
	return physicalID, coreID
}

// getOSName obtiene el nombre detallado del sistema operativo como neofetch/fastfetch
func getOSName() string {
	goos := runtime.GOOS

	switch goos {
	case "linux":
		return linuxOSName()
	case "windows":
		return "Windows"
	case "darwin":
		return macOSName()
	case "freebsd":
		return "FreeBSD"
	case "openbsd":
		return "OpenBSD"
	case "netbsd":
		return "NetBSD"
	default:
		return goos
	}
}

func linuxOSName() string {
	// Intentar leer /etc/os-release primero (estándar systemd)
	if name := osReleaseName(); name != "" {
		return name
	}

	// Intentar /etc/lsb-release (Ubuntu/Debian)
	if name := lsbReleaseName(); name != "" {
		return name
	}

	// Intentar /etc/redhat-release (RedHat/CentOS/Fedora)
	if data, err := os.ReadFile("/etc/redhat-release"); err == nil {
		return strings.TrimSpace(string(data))
	}

	// Intentar /etc/debian_version (Debian)
	if data, err := os.ReadFile("/etc/debian_version"); err == nil {
		return "Debian " + strings.TrimSpace(string(data))
	}

	// Intentar /etc/arch-release (Arch Linux)
	if _, err := os.Stat("/etc/arch-release"); err == nil {
		return "Arch Linux"
	}

	// Fallback
	return "Linux"
}

// osReleaseName reads /etc/os-release, empty when it is missing or names nothing
func osReleaseName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	var prettyName, name, version string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "PRETTY_NAME="):
			val := strings.TrimPrefix(line, "PRETTY_NAME=")
			// Remover comillas simples o dobles
			prettyName = strings.Trim(val, "\"'")
		case strings.HasPrefix(line, "NAME=") && name == "":
			val := strings.TrimPrefix(line, "NAME=")
			// Remover comillas simples o dobles
			name = strings.Trim(val, "\"'")
		case strings.HasPrefix(line, "VERSION=") && version == "":
			val := strings.TrimPrefix(line, "VERSION=")
			// Remover comillas simples o dobles
			version = strings.Trim(val, "\"'")
		}
	}
	if prettyName != "" {
		return prettyName
	}
	if name != "" && version != "" {
		return name + " " + version
	}
	return name
}

// lsbReleaseName reads /etc/lsb-release, empty when it is missing or names nothing
func lsbReleaseName() string {
	data, err := os.ReadFile("/etc/lsb-release")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	var dist, release string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "DISTRIB_ID=") {
			dist = strings.TrimPrefix(line, "DISTRIB_ID=")
		} else if strings.HasPrefix(line, "DISTRIB_RELEASE=") {
			release = strings.TrimPrefix(line, "DISTRIB_RELEASE=")
		}
	}
	if dist != "" && release != "" {
		return dist + " " + release
	}
	return dist
}

func macOSName() string {
	// Intentar obtener la versión de macOS
	data, err := os.ReadFile("/System/Library/CoreServices/SystemVersion.plist")
	if err != nil {
		return "macOS"
	}
	content := string(data)
	// Buscar ProductVersion
	if idx := strings.Index(content, "<key>ProductVersion</key>"); idx != -1 {
		if idx2 := strings.Index(content[idx:], "<string>"); idx2 != -1 {
			start := idx + idx2 + 8
			if idx3 := strings.Index(content[start:], "</string>"); idx3 != -1 {
				version := strings.TrimSpace(content[start : start+idx3])
				return "macOS " + version
			}
		}
	}
	return "macOS"
}

func SendStatsForServers() {
	var wg sync.WaitGroup
	for _, v := range GetAll() {
		wg.Add(1)
		go func(p *Server) {
			defer wg.Done()
			stats, err := p.GetEnvironment().GetStats()
			if err != nil {
				return
			}

			_ = p.GetEnvironment().GetStatsTracker().WriteMessage(skypanel.Transmission{
				Message: stats,
				Type:    skypanel.MessageTypeStats,
			})

			p.recordMetrics(stats, time.Now())

			// Monitorear para alertas
			go checkServerAlerts(p, stats)

			// Monitorear límite de disco
			go checkDiskLimit(p, stats)
		}(v)
	}
	wg.Wait()
}

func trackUptime(server *Server) {
	isRunning, err := server.IsRunning()
	if err != nil {
		return
	}

	db, err := database.GetConnection()
	if err != nil {
		return
	}

	us := &services.Uptime{DB: db}
	err = us.TrackStatus(server.ID(), isRunning)
	if err != nil {
		logging.Error.Printf("[%s] Error tracking uptime: %s", server.ID(), err)
	}
}

func checkDiskLimit(server *Server, stats *skypanel.ServerStats) {
	if stats == nil || stats.MaxStorage <= 0 {
		return
	}

	// Si el disco excede el límite (con un margen de 1MB para evitar falsos positivos)
	if stats.Disk > stats.MaxStorage+(1024*1024) {
		isRunning, _ := server.IsRunning()
		if isRunning {
			logging.Info.Printf("[%s] Server exceeded disk limit (%.2f MB > %.2f MB). Stopping server...", server.ID(), stats.Disk/1024/1024, stats.MaxStorage/1024/1024)
			server.RunningEnvironment.DisplayToConsole(true, fmt.Sprintf("\n[CRÍTICO] El servidor ha superado el límite de disco asignado (%.2f MB / %.2f MB).\n[CRÍTICO] Deteniendo el servidor por seguridad...\n", stats.Disk/1024/1024, stats.MaxStorage/1024/1024))

			// Detener el servidor
			err := server.Stop()
			if err != nil {
				logging.Error.Printf("[%s] Failed to stop server after exceeding disk limit: %s", server.ID(), err)
				_ = server.Kill() // Forzar kill si falla el stop
			}
		}
	}
}

func checkServerAlerts(server *Server, stats *skypanel.ServerStats) {
	stateTrackingLock.Lock()
	defer stateTrackingLock.Unlock()

	serverID := server.ID()
	isRunning, _ := server.IsRunning()

	// Obtener o crear estado anterior
	state, exists := serverStateTracking[serverID]
	if !exists {
		state = &serverState{
			wasRunning: isRunning,
			lastStats:  stats,
			lastAlert:  make(map[string]time.Time),
		}
		serverStateTracking[serverID] = state
		return // Primera vez, no enviar alertas
	}

	serverName := server.Server.Display
	if serverName == "" {
		serverName = serverID
	}

	// Verificar cambio de estado online/offline
	trackPowerChange(server, state, isRunning, serverName)

	// Verificar uso alto de recursos (solo si está online)
	if isRunning && stats != nil {
		checkResourceAlerts(server, state, stats, serverName)
	}

	// Actualizar stats anteriores
	state.lastStats = stats
}

// trackPowerChange alerts when the server comes online and, once offlineAlertDelay passes,
// when it went offline without a stop, clean exit or reported crash. Callers hold stateTrackingLock.
func trackPowerChange(server *Server, state *serverState, isRunning bool, serverName string) {
	serverID := server.ID()
	if state.wasRunning != isRunning {
		server.notifyPower(isRunning)
		ds := services.GetDiscordService()
		if isRunning {
			_ = ds.SendServerOnlineAlert(serverName, serverID)
		} else {
			// stops someone asked for are not news for the admin channel and crashes are
			// reported as they happen, so only an unexplained exit is worth an alert
			state.offlineSince = time.Now()
		}
		state.wasRunning = isRunning
	}
	if isRunning || server.exitHandled.Load() || server.stopRequested.Load() {
		state.offlineSince = time.Time{}
	} else if !state.offlineSince.IsZero() && time.Since(state.offlineSince) > offlineAlertDelay {
		_ = services.GetDiscordService().SendServerOfflineAlert(serverName, serverID)
		state.offlineSince = time.Time{}
	}
}

// checkResourceAlerts alerts on high CPU or memory at most every 5 minutes per resource.
// Callers hold stateTrackingLock.
func checkResourceAlerts(server *Server, state *serverState, stats *skypanel.ServerStats, serverName string) {
	now := time.Now()

	// CPU > 80%
	if stats.CPU > 80.0 {
		sendResourceAlert(server, state, now, serverName, resourceAlert{key: "cpu_high", resource: "CPU", value: stats.CPU, limit: 80.0})
	}

	// RAM > 90%
	memory := memoryPercent(stats)
	if memory > 90.0 {
		sendResourceAlert(server, state, now, serverName, resourceAlert{key: "memory_high", resource: "Memoria", value: memory, limit: 90.0})
	}

	// Limpiar alertas antiguas si el recurso ya no está alto
	if stats.CPU <= 80.0 {
		delete(state.lastAlert, "cpu_high")
	}
	if memory <= 90.0 {
		delete(state.lastAlert, "memory_high")
	}
}

// resourceAlert is one resource over its limit: which alert it is, what is high and by how much
type resourceAlert struct {
	key, resource string
	value, limit  float64
}

func sendResourceAlert(server *Server, state *serverState, now time.Time, serverName string, alert resourceAlert) {
	lastAlert, hasAlerted := state.lastAlert[alert.key]
	if hasAlerted && now.Sub(lastAlert) <= 5*time.Minute {
		return
	}
	ds := services.GetDiscordService()
	_ = ds.SendResourceAlert(serverName, server.ID(), alert.resource, alert.value, alert.limit)
	server.notifyResource(alert.resource, alert.value, alert.limit)
	state.lastAlert[alert.key] = now
}

// crashCounterResetAfter is how long a server must run before a crash counts as the
// first of a new series
const crashCounterResetAfter = 10 * time.Minute

func (p *Server) ranLongerThan(d time.Duration) bool {
	started := p.startedAt.Load()
	return started > 0 && time.Since(time.Unix(0, started)) > d
}

// memoryPercent is memory use as a percentage. Docker reports bytes with a limit; other
// environments may already report a percentage (0-100) and no limit.
func memoryPercent(stats *skypanel.ServerStats) float64 {
	if stats.MaxMemory > 0 {
		return stats.Memory / stats.MaxMemory * 100
	}
	if stats.Memory <= 100 {
		return stats.Memory
	}
	return 0
}

type FileData struct {
	Contents      io.ReadCloser
	ContentLength int64
	FileList      []skypanel.FileDesc
	Name          string
}

func (p *Server) DataToMap() map[string]interface{} {
	var result = p.Server.DataToMap()
	result["rootDir"] = p.RunningEnvironment.GetRootDirectory()
	result["core:os"] = runtime.GOOS
	result["core:arch"] = runtime.GOARCH

	return result
}

func CreateProgram() *Server {
	p := &Server{
		Server: skypanel.Server{
			Execution: skypanel.Execution{
				AutoStart:               false,
				AutoRestartFromCrash:    false,
				AutoRestartFromGraceful: false,
				PreExecution:            make([]skypanel.ConditionalMetadataType, 0),
				PostExecution:           make([]skypanel.ConditionalMetadataType, 0),
				EnvironmentVariables:    make(map[string]string),
			},
			Type:           skypanel.Type{Type: "standard"},
			Variables:      make(map[string]skypanel.Variable),
			Display:        "Unknown server",
			Installation:   make([]skypanel.ConditionalMetadataType, 0),
			Uninstallation: make([]skypanel.ConditionalMetadataType, 0),
			Groups:         make([]skypanel.Group, 0),
		},
	}
	p.stopChan = make(chan bool, 1)
	p.waitForConsole = &sync.Mutex{}
	p.keepAliveChan = make(chan bool)
	return p
}

// Start Starts the program.
// This includes starting the environment if it is not running.
func (p *Server) Start() error {
	if err := p.IsIdle(); err != nil {
		return err
	}
	// like Pterodactyl, a server that already filled its disk does not boot until files are deleted
	if err := p.CheckDiskSpace(true); err != nil {
		p.RunningEnvironment.DisplayToConsole(true, "The server has used all of its disk space. Delete files to free some before starting it.\n")
		return err
	}

	p.stopRequested.Store(false)
	p.startedAt.Store(time.Now().UnixNano())
	p.exitHandled.Store(false)
	p.Log(logging.Info, "Starting server %s", p.ID())
	p.RunningEnvironment.DisplayToConsole(true, "Starting server\n")

	process, err := GenerateProcess(p.Execution.PreExecution, p.RunningEnvironment, p.DataToMap(), p.Execution.EnvironmentVariables)
	if err != nil {
		p.Log(logging.Error, "Error generating pre-execution steps: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Error running pre execute\n")
		return err
	}

	err = process.Run(p)
	if err != nil {
		p.Log(logging.Error, "Error running pre-execution steps: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Error running pre execute\n")
		return err
	}

	command, err := p.resolveCommand()
	if err != nil {
		return err
	}

	if command.StdIn.Type == "" {
		command.StdIn = p.Execution.Stdin
	}

	data := p.DataToMap()

	commandLine := utils.ReplaceTokens(command.Command, data)
	err = p.RunningEnvironment.ExecuteAsync(skypanel.ExecutionData{
		Command:     commandLine,
		Environment: utils.ReplaceTokensInMap(p.Execution.EnvironmentVariables, data),
		Variables:   p.DataToMap(),
		Callback:    p.afterExit,
		StdInConfig: command.StdIn,
	})

	if err != nil {
		p.Log(logging.Error, "error starting server %s: %s", p.ID(), err)
		p.RunningEnvironment.DisplayToConsole(true, " Failed to start server\n")
		return err
	}

	// a world map writes its config on first start; fix the port and download consent then,
	// whether or not someone has the map tab open
	if p.installedWorldMap() != nil {
		p.ensureMapConfigured()
	}

	// keepalive!
	if p.KeepAlive.Frequency != "" && p.KeepAlive.Command != "" {
		p.startKeepAlive()
	}

	return err
}

// resolveCommand picks the command to run: a plain string, or the first entry of the list
// whose condition holds, falling back to the entry without a condition
func (p *Server) resolveCommand() (skypanel.Command, error) {
	if c, ok := p.Execution.Command.(string); ok {
		return skypanel.Command{Command: c}, nil
	}

	// we have a list
	var possibleCommands []skypanel.Command
	err := utils.UnmarshalTo(p.Execution.Command, &possibleCommands)
	if err != nil {
		return skypanel.Command{}, err
	}

	var defaultCommand skypanel.Command
	for _, v := range possibleCommands {
		if v.If == "" {
			defaultCommand = v
			break
		}
	}

	command, err := p.firstMatchingCommand(possibleCommands)
	if err != nil {
		return skypanel.Command{}, err
	}

	// if no command, use default
	if command.Command == "" {
		command = defaultCommand
	}
	return command, nil
}

// firstMatchingCommand returns the first conditional command whose condition holds
func (p *Server) firstMatchingCommand(possibleCommands []skypanel.Command) (skypanel.Command, error) {
	for _, v := range possibleCommands {
		// now... we see which command to use
		if v.If == "" {
			continue
		}
		useThis, err := p.RunCondition(v.If, nil)
		if err != nil {
			p.Log(logging.Error, "error starting server %s: %s", p.ID(), err)
			p.RunningEnvironment.DisplayToConsole(true, " Failed to start server\n")
			return skypanel.Command{}, err
		}
		if useThis {
			return v, nil
		}
	}
	return skypanel.Command{}, nil
}

// startKeepAlive sends the keep-alive command on its interval until the server exits
func (p *Server) startKeepAlive() {
	dur, err := time.ParseDuration(p.KeepAlive.Frequency)
	if err != nil {
		p.RunningEnvironment.DisplayToConsole(true, " Failed to enable keep-alive: %s", err)
		return
	}
	if p.keepAlive == nil {
		p.keepAlive = time.NewTicker(dur)
	} else {
		p.keepAlive.Reset(dur)
	}

	if p.keepAlive != nil {
		go func() {
			for {
				select {
				case <-p.keepAliveChan:
					return
				case <-p.keepAlive.C:
					_ = p.RunningEnvironment.ExecuteInMainProcess(p.KeepAlive.Command)
				}
			}
		}()
	}
}

// Stop Stops the program.
// This will also stop the environment it is ran in.
func (p *Server) Stop() error {
	var err error
	if r, err := p.IsRunning(); !r || err != nil {
		return err
	}

	p.stopRequested.Store(true)
	p.Log(logging.Info, "Stopping server %s", p.ID())
	if p.Execution.StopCode != 0 {
		err = p.RunningEnvironment.SendCode(p.Execution.StopCode)
	} else {
		err = p.RunningEnvironment.ExecuteInMainProcess(p.Execution.StopCommand)
	}
	if err != nil {
		p.Log(logging.Error, "Error stopping server: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to stop server\n")
	} else {
		p.RunningEnvironment.DisplayToConsole(true, "Server was told to stop\n")
	}
	return err
}

// Kill Kills the program.
// This will also stop the environment it is ran in.
func (p *Server) Kill() (err error) {
	p.stopRequested.Store(true)
	p.Log(logging.Info, "Killing server %s", p.ID())
	err = p.RunningEnvironment.Kill()
	if err != nil {
		p.Log(logging.Error, "Error killing server: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to kill server\n")
	} else {
		p.RunningEnvironment.DisplayToConsole(true, "Server killed\n")
	}
	return
}

// Create Creates any files needed for the program.
// This includes creating the environment.
func (p *Server) Create() (err error) {
	p.Log(logging.Info, "Creating server %s", p.ID())
	p.RunningEnvironment.DisplayToConsole(true, "Allocating server\n")
	err = p.RunningEnvironment.Create()
	if err != nil {
		p.Log(logging.Error, "Error creating server: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to create server\n")
	} else {
		p.RunningEnvironment.DisplayToConsole(true, "Server allocated\n")
	}

	return
}

// Destroy Destroys the server.
// This will delete the server, environment, and any files related to it.
func (p *Server) Destroy() (err error) {
	if err := p.IsIdle(); err != nil {
		return err
	}

	p.Log(logging.Info, "Destroying server %s", p.ID())

	p.Log(logging.Debug, "Stopping scheduler")
	if p.Scheduler != nil && p.Scheduler.IsRunning() {
		p.Scheduler.Stop()
	}

	p.Log(logging.Debug, "Starting uninstall processes")
	process, err := GenerateProcess(p.Uninstallation, p.RunningEnvironment, p.DataToMap(), p.Execution.EnvironmentVariables)
	if err != nil {
		p.Log(logging.Error, uninstallErrorFmt, err)
		p.RunningEnvironment.DisplayToConsole(true, uninstallFailedMsg)
		return
	}

	err = process.Run(p)
	if err != nil {
		p.Log(logging.Error, uninstallErrorFmt, err)
		p.RunningEnvironment.DisplayToConsole(true, uninstallFailedMsg)
		return
	}

	p.Log(logging.Debug, "Deleting environment")
	err = p.RunningEnvironment.Delete()
	if err != nil {
		p.Log(logging.Error, uninstallErrorFmt, err)
		p.RunningEnvironment.DisplayToConsole(true, uninstallFailedMsg)
	}

	return
}

func (p *Server) Install() error {
	if err := p.IsIdle(); err != nil {
		return err
	}

	p.GetEnvironment().SetInstalling(true)
	defer p.GetEnvironment().SetInstalling(false)

	p.Log(logging.Info, "Installing server %s", p.ID())
	r, err := p.IsRunning()
	if err != nil {
		p.Log(logging.Error, "Error checking server status: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Error on checking to see if server is running\n")
		return err
	}

	if r {
		err = p.Stop()
	}

	if err != nil {
		p.Log(logging.Error, "Error stopping server: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to stop server\n")
		return err
	}

	p.RunningEnvironment.DisplayToConsole(true, "Installing server\n")

	err = os.MkdirAll(p.RunningEnvironment.GetRootDirectory(), 0755)
	if err != nil && !os.IsExist(err) {
		p.Log(logging.Error, "Error creating server directory: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to create server directory\n")
		return err
	}

	if len(p.Installation) > 0 {
		var process OperationProcess

		data := p.DataToMap()
		process, err = GenerateProcess(p.Installation, p.RunningEnvironment, data, p.Execution.EnvironmentVariables)
		if err != nil {
			p.Log(logging.Error, "Error installing server: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "Failed to install server\n")
			return err
		}

		err = process.Run(p)
		if err != nil {
			p.Log(logging.Error, "Error installing server: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "Failed to install server\n")
			return err
		}
	}

	p.RunningEnvironment.DisplayToConsole(true, "Server installed\n")
	return nil
}

func (p *Server) IsRunning() (bool, error) {
	return p.RunningEnvironment.IsRunning()
}

func (p *Server) Execute(command string) (err error) {
	err = p.RunningEnvironment.ExecuteInMainProcess(command)
	return
}

func (p *Server) SetEnvironment(environment *skypanel.Environment) (err error) {
	p.RunningEnvironment = environment
	return
}

func (p *Server) ID() string {
	return p.Identifier
}

func (p *Server) GetEnvironment() *skypanel.Environment {
	return p.RunningEnvironment
}

func (p *Server) SetAutoStart(isAutoStart bool) (err error) {
	p.Execution.AutoStart = isAutoStart
	return
}

func (p *Server) IsAutoStart() (isAutoStart bool) {
	isAutoStart = p.Execution.AutoStart
	return
}

func (p *Server) Save() (err error) {
	p.Log(logging.Info, "Saving server %s", p.ID())

	safeID, err := validateServerID(p.ID())
	if err != nil {
		return
	}
	file := filepath.Join(config.ServersFolder.Value(), safeID+".json")

	if err = p.valid(); err != nil {
		p.Log(logging.Error, "Server %s contained invalid data, this server is.... broken", p.Identifier)
		// we can't even reload from disk....
		// so, puke back, and for now we'll handle it later
		return err
	}

	var data []byte
	data, err = json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}

	err = os.WriteFile(file, data, 0664)
	return
}

func (p *Server) EditData(data map[string]interface{}, asAdmin bool) (err error) {
	for k, v := range data {
		var elem skypanel.Variable

		if _, ok := p.Variables[k]; ok {
			elem = p.Variables[k]
		}
		if !asAdmin && !elem.UserEditable {
			continue
		}

		elem.Value = v

		p.Variables[k] = elem
	}

	err = p.Save()
	return
}

func (p *Server) GetData() map[string]skypanel.Variable {
	return p.Variables
}

func (p *Server) GetNetwork() string {
	data := p.GetData()
	ip := "0.0.0.0"
	port := "0"

	if ipData, ok := data["ip"]; ok {
		ip = cast.ToString(ipData.Value)
	}

	if portData, ok := data["port"]; ok {
		port = cast.ToString(portData.Value)
	}

	return ip + ":" + port
}

func (p *Server) afterExit(exitCode int) {
	if p.keepAlive != nil {
		p.keepAlive.Stop()
		p.keepAliveChan <- true
	}

	graceful := exitCode == p.Execution.ExpectedExitCode
	// a stop or kill someone asked for is not a crash, whatever exit code it ends with
	requested := p.stopRequested.Swap(false)
	crashed := !graceful && !requested
	p.exitHandled.Store(true)
	// a server that ran fine for a while before crashing starts a fresh count, so crashes a
	// week apart do not use up its automatic restarts
	if !crashed || p.ranLongerThan(crashCounterResetAfter) {
		p.CrashCounter = 0
	}
	autoRestart := crashed && p.Execution.AutoRestartFromCrash && p.CrashCounter < config.CrashLimit.Value()
	if autoRestart {
		p.CrashCounter++
	}
	if crashed {
		p.RunningEnvironment.DisplayToConsole(true, "Server crashed (exit code %d)\n", exitCode)
		report := p.recordCrash(exitCode, autoRestart)
		p.notifyCrash(report)
		restartText := "No"
		if autoRestart {
			restartText = "Sí"
		}
		name, id := p.displayName(), p.ID()
		go func() {
			_ = services.GetDiscordService().SendAlert("💥 Caída del servidor",
				fmt.Sprintf("**%s** se ha caído (código de salida %d).", name, exitCode),
				[]services.DiscordEmbedField{
					{Name: "Servidor", Value: name, Inline: true},
					{Name: "ID", Value: id, Inline: true},
					{Name: "Reinicio automático", Value: restartText, Inline: true},
				})
		}()
	}

	mapping := p.DataToMap()
	mapping["success"] = graceful
	mapping["exitCode"] = exitCode

	processes, err := GenerateProcess(p.Execution.PostExecution, p.RunningEnvironment, mapping, p.Execution.EnvironmentVariables)
	if err != nil {
		p.Log(logging.Error, "Error running post processing for server %s: %s", p.ID(), err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to run post-execution steps\n")
		return
	}
	p.RunningEnvironment.DisplayToConsole(true, "Running post-execution steps\n")
	p.Log(logging.Info, "Running post execution steps: %s", p.ID())

	err = processes.Run(p)
	if err != nil {
		p.Log(logging.Error, "Error running post processing for server: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to run post-execution steps\n")
		return
	}

	if (graceful && !requested && p.Execution.AutoRestartFromGraceful) || autoRestart {
		StartViaService(p)
	}
}

func (p *Server) GetItem(name string) (*FileData, error) {
	info, err := p.GetFileServer().Stat(name)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return &FileData{FileList: p.listDirectory(name)}, nil
	}
	file, err := p.GetFileServer().Open(name)
	if err != nil {
		return nil, err
	}
	return &FileData{Contents: file, ContentLength: info.Size(), Name: info.Name()}, nil
}

// listDirectory lists a folder for the file manager: ".." first (outside the root), then
// folders, then files, alphabetically
func (p *Server) listDirectory(name string) []skypanel.FileDesc {
	fileList, _ := p.GetFileServer().ReadDir(name)
	var fileNames []skypanel.FileDesc
	offset := 0
	if name == "" || name == "." || name == "/" {
		fileNames = make([]skypanel.FileDesc, len(fileList))
	} else {
		fileNames = make([]skypanel.FileDesc, len(fileList)+1)
		fileNames[0] = skypanel.FileDesc{
			Name: "..",
			File: false,
		}
		offset = 1
	}

	// validate any symlinks are valid

	for i, file := range fileList {
		newFile, ok := p.describeFile(name, file)
		if !ok {
			continue
		}
		fileNames[i+offset] = newFile
	}
	// Ordenar (carpetas primero, luego archivos, y alfabéticamente)
	sort.Slice(fileNames, func(i, j int) bool {
		return fileDescLess(fileNames[i], fileNames[j])
	})
	return fileNames
}

// describeFile builds the listing entry for one file; ok is false when a regular file
// cannot be stat'ed, which leaves its slot empty as before
func (p *Server) describeFile(dir string, file fs.DirEntry) (skypanel.FileDesc, bool) {
	newFile := skypanel.FileDesc{
		Name: file.Name(),
		File: !file.IsDir(),
	}

	if !file.IsDir() && file.Type()&os.ModeSymlink == 0 {
		infoData, err := p.GetFileServer().Stat(filepath.Join(dir, file.Name()))
		if err != nil {
			return newFile, false
		}
		newFile.Size = infoData.Size()
		newFile.Modified = infoData.ModTime().Unix()
		newFile.Extension = filepath.Ext(file.Name())
	}
	return newFile, true
}

func fileDescLess(a, b skypanel.FileDesc) bool {
	// ".." siempre de primero
	if a.Name == ".." {
		return true
	}
	if b.Name == ".." {
		return false
	}

	// Si uno es carpeta y el otro archivo, la carpeta va primero
	if a.File != b.File {
		return !a.File
	}

	// Si ambos son del mismo tipo, orden alfabético sin distinguir mayúsculas
	return strings.ToLower(a.Name) < strings.ToLower(b.Name)
}

func (p *Server) ArchiveItems(sourceFiles []string, destination string) error {
	// This may technically error out in other cases
	if _, err := os.Stat(destination); err != nil && !os.IsNotExist(err) {
		return skypanel.ErrFileExists
	}
	if err := p.CheckDiskSpace(false); err != nil {
		return err
	}
	defer p.ForgetDiskUsage()
	return files.Compress(p.GetFileServer(), destination, sourceFiles)
}

func (p *Server) Extract(source, destination string) error {
	if err := p.CheckDiskSpace(false); err != nil {
		return err
	}
	defer p.ForgetDiskUsage()
	return files.Extract(p.GetFileServer(), source, destination, "*", false, nil)
}

const (
	backupIdle int32 = iota
	backupRunning
	restoreRunning
)

// AutoBackupPrefix starts the file name of every backup made by a scheduled task, so
// retention only ever removes those and never a backup somebody made by hand
const AutoBackupPrefix = "auto-"

// StartBackup begins a backup and returns its file name straight away; the archive is
// written in the background and the console reports when it is done
func (p *Server) StartBackup() (string, error) {
	fileName, file, err := p.beginBackup("")
	if err != nil {
		return "", err
	}
	go p.finishBackup(file)
	return fileName, nil
}

// RunAutoBackup backs the server up for a scheduled task. It waits for the archive, so the
// task's next step sees it finished, then keeps only the newest keep automatic backups.
func (p *Server) RunAutoBackup(keep int) (string, error) {
	fileName, file, err := p.beginBackup(AutoBackupPrefix)
	if err != nil {
		return "", err
	}
	if !p.finishBackup(file) {
		return "", errors.New("backup failed")
	}
	if err := p.pruneAutoBackups(keep); err != nil {
		p.Log(logging.Error, "Error removing old automatic backups: %s", err)
	}
	return fileName, nil
}

func (p *Server) beginBackup(prefix string) (fileName, file string, err error) {
	if !p.backupState.CompareAndSwap(backupIdle, backupRunning) {
		return "", "", skypanel.ErrBackupInProgress
	}

	p.RunningEnvironment.DisplayToConsole(true, "Backing up server")
	backupDirectory := p.GetBackupDirectory()
	if err = os.MkdirAll(backupDirectory, 0755); err != nil {
		p.endBackup(false)
		return "", "", err
	}

	backupID, err := uuid.NewV4()
	if err != nil {
		p.endBackup(false)
		return "", "", err
	}
	fileName = prefix + backupID.String() + ".tar.gz"
	return fileName, path.Join(backupDirectory, fileName), nil
}

// finishBackup writes the archive, copies it off-site when that is on, and reports the result
func (p *Server) finishBackup(file string) bool {
	err := p.writeArchive(file)
	if err != nil {
		p.Log(logging.Error, "Error creating backup file: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to create backup file")
	} else {
		p.uploadOffsite(file)
	}
	p.endBackup(err == nil)
	return err == nil
}

// writeArchive compresses the server's files under a hidden name and moves the archive into
// place once it is complete, so a half-written backup is never listed, downloaded or
// restored. The hidden name keeps the .tar.gz ending because the format is picked from it.
func (p *Server) writeArchive(file string) error {
	partial := filepath.Join(filepath.Dir(file), ".partial-"+filepath.Base(file))
	err := files.Compress(nil, partial, []string{p.GetFileServer().Prefix()})
	if err == nil {
		err = os.Rename(partial, file)
	}
	if err != nil {
		_ = os.Remove(partial)
	}
	return err
}

// offsiteTimeout bounds one transfer to or from the bucket
const offsiteTimeout = 2 * time.Hour

// uploadOffsite copies a finished backup to the bucket when off-site backups are on. The
// local copy stays: the bucket is for when the node's disk is gone. A failed upload is
// reported but does not fail the backup, which exists locally.
func (p *Server) uploadOffsite(file string) {
	cfg, ok := offsite.FromSettings()
	if !ok {
		return
	}
	p.RunningEnvironment.DisplayToConsole(true, "Uploading backup to off-site storage")
	ctx, cancel := context.WithTimeout(context.Background(), offsiteTimeout)
	defer cancel()
	if err := offsite.Upload(ctx, cfg, p.ID(), filepath.Base(file), file); err != nil {
		p.Log(logging.Error, "Error uploading backup off-site: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Off-site upload failed: %s", err)
		return
	}
	p.RunningEnvironment.DisplayToConsole(true, "Backup uploaded to off-site storage")
}

func (p *Server) endBackup(success bool) {
	p.backupState.Store(backupIdle)

	serverName := p.Server.Display
	if serverName == "" {
		serverName = p.ID()
	}
	p.notifyBackup(success)
	ds := services.GetDiscordService()
	if success {
		p.RunningEnvironment.DisplayToConsole(true, "Backup complete")
		_ = ds.SendBackupAlert(serverName, p.ID(), "Completado exitosamente", true)
	} else {
		p.RunningEnvironment.DisplayToConsole(true, "Backup failed")
		_ = ds.SendBackupAlert(serverName, p.ID(), "Falló durante la creación", false)
	}
}

// ListBackups returns the finished backups in the server's backup folder, newest first
func (p *Server) ListBackups() ([]skypanel.BackupFile, error) {
	entries, err := os.ReadDir(p.GetBackupDirectory())
	if os.IsNotExist(err) {
		return []skypanel.BackupFile{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := make([]skypanel.BackupFile, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, skypanel.BackupFile{FileName: name, Size: info.Size(), ModTime: info.ModTime(), Location: "local"})
	}
	result = p.mergeOffsite(result)
	sort.Slice(result, func(i, j int) bool { return result[i].ModTime.After(result[j].ModTime) })
	return result, nil
}

// mergeOffsite adds the backups only the bucket still has and marks the ones in both places.
// If the bucket cannot be listed the local list is returned as it is.
func (p *Server) mergeOffsite(local []skypanel.BackupFile) []skypanel.BackupFile {
	cfg, ok := offsite.FromSettings()
	if !ok {
		return local
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	remote, err := offsite.List(ctx, cfg, p.ID())
	if err != nil {
		p.Log(logging.Error, "Error listing off-site backups: %s", err)
		return local
	}
	index := make(map[string]int, len(local))
	for i, b := range local {
		index[b.FileName] = i
	}
	for _, object := range remote {
		if i, ok := index[object.FileName]; ok {
			local[i].Location = "both"
			continue
		}
		local = append(local, skypanel.BackupFile{FileName: object.FileName, Size: object.Size, ModTime: object.ModTime, Location: "offsite"})
	}
	return local
}

// pruneAutoBackups deletes the oldest automatic backups beyond the newest keep
func (p *Server) pruneAutoBackups(keep int) error {
	if keep < 1 {
		return nil
	}
	backups, err := p.ListBackups()
	if err != nil {
		return err
	}
	kept := 0
	for _, b := range backups {
		if !strings.HasPrefix(b.FileName, AutoBackupPrefix) {
			continue
		}
		kept++
		if kept > keep {
			if err := p.DeleteBackup(b.FileName); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Server) DeleteBackup(fileName string) error {
	if !filepath.IsLocal(fileName) {
		return fmt.Errorf("invalid backup file name: %s", fileName)
	}
	backupDirectory := p.GetBackupDirectory()
	if backupDirectory == "" {
		return skypanel.ErrSettingNotConfigured("backupDirectory")
	}

	backupFile := path.Join(backupDirectory, path.Base(fileName))

	err := os.Remove(backupFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if cfg, ok := offsite.FromSettings(); ok {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := offsite.Delete(ctx, cfg, p.ID(), fileName); err != nil {
			p.Log(logging.Error, "Error deleting off-site backup %s: %s", fileName, err)
		}
	}

	return nil
}

// locateBackup finds the backup to restore and whether it must first be fetched from off-site
// storage. A missing backup must stop the restore: it deletes every server file before
// extracting, so going on would leave the server empty
func (p *Server) locateBackup(fileName string) (backupFile string, fetchOffsite bool, err error) {
	backupFile = filepath.Join(p.GetBackupDirectory(), path.Base(fileName))
	if _, err = os.Stat(backupFile); err != nil {
		if !os.IsNotExist(err) {
			return "", false, err
		}
		// the local copy is gone (a new disk, say): the bucket may still have it
		cfg, ok := offsite.FromSettings()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		exists := ok && offsite.Exists(ctx, cfg, p.ID(), fileName)
		cancel()
		if !exists {
			return "", false, skypanel.ErrFileNotFound
		}
		return backupFile, true, nil
	}
	return backupFile, false, nil
}

func (p *Server) StartRestore(fileName string) error {
	if !filepath.IsLocal(fileName) {
		return fmt.Errorf("invalid backup file name: %s", fileName)
	}
	// never restore under a running server or another backup/restore
	if err := p.IsIdle(); err != nil {
		return err
	}

	backupFile, fetchOffsite, err := p.locateBackup(fileName)
	if err != nil {
		return err
	}

	if !p.backupState.CompareAndSwap(backupIdle, restoreRunning) {
		return skypanel.ErrBackupInProgress
	}

	c := make(chan bool)
	go func(d chan bool) {
		r := <-d
		p.backupState.Store(backupIdle)
		if r {
			p.RunningEnvironment.DisplayToConsole(true, "Restore complete")
		} else {
			p.RunningEnvironment.DisplayToConsole(true, "Restore failed")
		}
	}(c)

	p.RunningEnvironment.DisplayToConsole(true, "Restoring server")

	go func(source string, d chan bool) {
		success := false
		defer func() {
			d <- success
		}()

		if fetchOffsite {
			// fetched before anything is touched, so a failed download leaves the server as it was
			p.RunningEnvironment.DisplayToConsole(true, "Downloading backup from off-site storage")
			cfg, _ := offsite.FromSettings()
			ctx, cancel := context.WithTimeout(context.Background(), offsiteTimeout)
			err := os.MkdirAll(filepath.Dir(source), 0755)
			if err == nil {
				err = offsite.Download(ctx, cfg, p.ID(), filepath.Base(source), source)
			}
			cancel()
			if err != nil {
				p.Log(logging.Error, "Error downloading off-site backup: %s", err)
				p.RunningEnvironment.DisplayToConsole(true, "Off-site download failed: %s", err)
				return
			}
		}

		archive, err := os.Open(source)
		if err != nil {
			p.Log(logging.Error, "Error opening backup: %s", err)
			return
		}
		defer utils.Close(archive)

		// read the whole archive before deleting anything: an unreadable backup must fail
		// here, not after the server's files are gone
		if _, err = files.DetermineIfSingleRoot(context.Background(), source, archive); err != nil {
			p.Log(logging.Error, "Error reading backup: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "Backup file cannot be read: %s", err)
			return
		}

		// keep the files about to be replaced, so the restore itself can be undone; without
		// that safety net the restore does not go ahead
		if err = p.writeRestorePoint(RestoreReasonRestore); err != nil {
			return
		}
		if _, err = archive.Seek(0, io.SeekStart); err != nil {
			p.Log(logging.Error, "Error reading backup: %s", err)
			return
		}

		// Check if any files exist, as remove all errors if its empty
		existingFiles, err := p.GetFileServer().Glob("*")
		if err != nil {
			p.Log(logging.Error, "Error globbing files: %s", err)
			return
		}

		for _, existingFile := range existingFiles {
			file, err := p.GetFileServer().Stat(existingFile)
			if err != nil {
				p.Log(logging.Error, "Error deleting files: %s", err)
				return
			}

			if file.IsDir() {
				err = p.GetFileServer().RemoveAll(existingFile)
			} else {
				err = p.GetFileServer().Remove(existingFile)
			}

			if err != nil {
				p.Log(logging.Error, "Error deleting files: %s", err)
				return
			}
		}

		// Extract needs the archive inside the file server; backups live outside it, so the
		// opened archive is extracted through the server's file server instead
		err = files.ExtractReader(p.GetFileServer(), archive, source, "", "*", true)
		if err != nil {
			p.Log(logging.Error, "Error restoring files: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "Failed to restore files: %s", err)
			return
		}
		success = true
	}(backupFile, c)

	return nil
}

func (p *Server) GetBackup(fileName string) (*FileData, error) {
	if !filepath.IsLocal(fileName) {
		return nil, fmt.Errorf("invalid backup file name: %s", fileName)
	}
	backupFile := filepath.Join(p.GetBackupDirectory(), path.Base(fileName))

	info, err := os.Stat(backupFile)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return &FileData{ContentLength: info.Size(), Name: info.Name()}, nil
}

func (p *Server) GetBackupFile(fileName string) (*FileData, error) {
	if !filepath.IsLocal(fileName) {
		return nil, fmt.Errorf("invalid backup file name: %s", fileName)
	}
	backupFile := filepath.Join(p.GetBackupDirectory(), path.Base(fileName))

	file, err := os.Open(backupFile)
	if os.IsNotExist(err) {
		if cfg, ok := offsite.FromSettings(); ok {
			// streamed straight from the bucket; the caller closes it
			reader, size, remoteErr := offsite.Open(context.Background(), cfg, p.ID(), fileName)
			if remoteErr == nil {
				return &FileData{Contents: reader, ContentLength: size, Name: path.Base(fileName)}, nil
			}
		}
	}
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	return &FileData{Contents: file, ContentLength: info.Size(), Name: info.Name()}, nil
}

func (p *Server) valid() error {
	// we need a type at least, this is a safe check
	if p.Type.Type == "" {
		return ErrServerTypeRequired
	}

	if p.Environment.Type == "" {
		return ErrEnvironmentTypeRequired
	}

	return nil
}

func (p *Server) Log(l *log.Logger, format string, obj ...interface{}) {
	msg := fmt.Sprintf("[%s] ", p.ID()) + format
	l.Printf(msg, obj...)
}

func (p *Server) RunCondition(condition string, extraData map[string]interface{}) (bool, error) {
	data := map[string]interface{}{
		conditions.VariableEnv:      p.RunningEnvironment.Type,
		conditions.VariableServerID: p.ID(),
	}

	for k, v := range extraData {
		data[k] = v
	}

	if p.Variables != nil {
		for k, v := range p.Variables {
			data[k] = v.Value
		}
	}

	return conditions.ResolveIf(condition, data, CreateFunctions(p.GetEnvironment()))
}

func (p *Server) GetFileServer() files.FileServer {
	return p.fileServer
}

func (p *Server) SetFileServer(fs files.FileServer) {
	p.fileServer = fs
}

func (p *Server) IsBackingUp() bool {
	return p.backupState.Load() == backupRunning
}

func (p *Server) IsRestoring() bool {
	return p.backupState.Load() == restoreRunning
}

func (p *Server) IsIdle() error {
	if p.IsRestoring() || p.IsBackingUp() {
		return skypanel.ErrBackupInProgress
	}

	r, _ := p.GetEnvironment().IsRunning()
	if r {
		return skypanel.ErrServerRunning
	}

	if p.GetEnvironment().IsInstalling() {
		return skypanel.ErrServerRunning
	}

	return nil
}

func (p *Server) GetBackupDirectory() string {
	return filepath.Join(config.BackupsFolder.Value(), p.ID())
}
