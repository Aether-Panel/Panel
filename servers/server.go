package servers

import (
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/conditions"
	"github.com/SkyPanel/SkyPanel/v3/config"
	"github.com/SkyPanel/SkyPanel/v3/database"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/logging"
	"github.com/SkyPanel/SkyPanel/v3/services"
	"github.com/SkyPanel/SkyPanel/v3/utils"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/gofrs/uuid/v5"
	"github.com/mholt/archiver/v3"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/spf13/cast"
)

type Server struct {
	SkyPanel.DaemonServer
	SkyPanel.Server

	CrashCounter       int                   `json:"-"`
	RunningEnvironment *SkyPanel.Environment `json:"-"`
	Scheduler          *Scheduler            `json:"-"`
	stopChan           chan bool
	waitForConsole     sync.Locker
	fileServer         files.FileServer
	backingUp          bool
	restoring          bool
	keepAlive          *time.Ticker
	keepAliveChan      chan bool
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
	lastStats  *SkyPanel.ServerStats
	lastAlert  map[string]time.Time // Para evitar spam de alertas
}

var ErrServerTypeRequired = errors.New("server type is required")
var ErrEnvironmentTypeRequired = errors.New("environment type is required")

func init() {
	archiver.DefaultZip.OverwriteExisting = true
	archiver.DefaultTarGz.OverwriteExisting = true
}

func InitService() {
	queue = list.New()
	running = true
	go processQueue()
	go processStats()
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
	startQueueTicker.Stop()
	statTicker.Stop()
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
				logging.Error.Printf("[%s] Error starting server: %s", program.Id(), err)
			}
		}
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

func trackUptimeForAllServers() {
	allServers := GetAll()
	for _, server := range allServers {
		go trackUptime(server)
	}
}

func processSystemStatus() {
	systemStatusTicker = time.NewTicker(1 * time.Minute)
	for range systemStatusTicker.C {
		go sendSystemStatusToDiscord()
	}
}

func sendSystemStatusToDiscord() {
	if !running {
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
	serverNodeMap := make(map[string]uint)
	for _, server := range allServers {
		serverModel, err := ss.Get(server.Id())
		if err != nil {
			// Si no se encuentra en la BD, asumimos nodo local (0)
			serverNodeMap[server.Id()] = 0
			continue
		}
		nodeID := serverModel.NodeID
		if nodeID == 0 || serverModel.RawNodeID == nil {
			nodeID = 0 // Nodo local
		}
		serverNodeMap[server.Id()] = nodeID
	}

	serverInfos := make([]services.ServerInfo, 0, len(allServers))
	nodeServerMap := make(map[uint][]services.ServerInfo)

	for _, server := range allServers {
		isRunning, err := server.IsRunning()
		if err != nil {
			logging.Error.Printf("[%s] Error checking server status for system status: %s", server.Id(), err)
			isRunning = false
		}

		serverName := server.Server.Display
		if serverName == "" {
			serverName = server.Id()
		}

		serverInfo := services.ServerInfo{
			Name:      serverName,
			ID:        server.Id(),
			IsRunning: isRunning,
			CPU:       0.0,
			Memory:    0.0,
		}

		// Obtener estadísticas si el servidor está corriendo
		if isRunning {
			stats, err := server.GetEnvironment().GetStats()
			if err == nil && stats != nil {
				serverInfo.CPU = stats.Cpu
				serverInfo.Memory = stats.Memory
			}
		}

		serverInfos = append(serverInfos, serverInfo)

		// Agrupar por nodo
		nodeID := serverNodeMap[server.Id()]
		if nodeServerMap[nodeID] == nil {
			nodeServerMap[nodeID] = make([]services.ServerInfo, 0)
		}
		nodeServerMap[nodeID] = append(nodeServerMap[nodeID], serverInfo)
	}

	// Enviar el reporte del sistema a Discord
	err = ds.SendSystemStatus(serverInfos)
	if err != nil {
		logging.Error.Printf("Error sending system status to Discord: %s", err)
	}

	// Enviar estado de cada nodo a Discord
	for _, node := range nodes {
		serversForNode := nodeServerMap[node.ID]
		if serversForNode == nil {
			serversForNode = make([]services.ServerInfo, 0)
		}

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
		cpuInfo, err := cpu.Info()
		cpuModel := "Desconocido"
		cpuCores := 0
		cpuThreads := int(runtime.NumCPU()) // Núcleos lógicos (hilos) - esto siempre es correcto
		cpuGhz := 0.0

		// Obtener cores físicos leyendo /proc/cpuinfo en Linux
		if runtime.GOOS == "linux" {
			if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
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

					var physicalID, coreID string

					// Procesar cada línea del bloque
					lines := strings.Split(block, "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if strings.Contains(line, ":") {
							parts := strings.SplitN(line, ":", 2)
							if len(parts) != 2 {
								continue
							}
							key := strings.TrimSpace(parts[0])
							value := strings.TrimSpace(parts[1])

							switch key {
							case "physical id":
								physicalID = value
							case "core id":
								coreID = value
							case "cpu cores":
								if physicalID != "" {
									if val, err := cast.ToIntE(value); err == nil {
										coresPerSocket[physicalID] = val
									}
								}
							}
						}
					}

					// Si tenemos physical id y core id, crear clave única para core físico
					if physicalID != "" && coreID != "" {
						physicalCores[physicalID+"-"+coreID] = true
					}
				}

				// Si encontramos cores físicos únicos, usarlos
				if len(physicalCores) > 0 {
					cpuCores = len(physicalCores)
				} else if len(coresPerSocket) > 0 {
					// Calcular: sumar cores de todos los sockets
					totalCores := 0
					for _, cores := range coresPerSocket {
						totalCores += cores
					}
					if totalCores > 0 {
						cpuCores = totalCores
					}
				}
			}
		}

		// Si no se pudo obtener cores físicos, usar cpu.Info como fallback
		if cpuCores == 0 {
			if err == nil && len(cpuInfo) > 0 {
				cpuModel = cpuInfo[0].ModelName
				// Para sistemas con múltiples CPUs físicos, sumar los cores
				totalCores := int(0)
				for _, info := range cpuInfo {
					totalCores += int(info.Cores)
					if cpuGhz == 0 && info.Mhz > 0 {
						cpuGhz = info.Mhz / 1000.0 // Convertir MHz a GHz
					}
				}
				if totalCores > 0 {
					cpuCores = totalCores
				}
			} else {
				// Fallback final: usar hilos / 2 (asume hyperthreading)
				cpuCores = cpuThreads / 2
				if cpuCores == 0 {
					cpuCores = 1
				}
			}
		}

		// Obtener modelo y frecuencia del CPU si no se obtuvo antes
		if err == nil && len(cpuInfo) > 0 {
			if cpuModel == "Desconocido" {
				cpuModel = cpuInfo[0].ModelName
			}
			if cpuGhz == 0 {
				for _, info := range cpuInfo {
					if info.Mhz > 0 {
						cpuGhz = info.Mhz / 1000.0 // Convertir MHz a GHz
						break
					}
				}
			}
		}

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
}

// getOSName obtiene el nombre detallado del sistema operativo como neofetch/fastfetch
func getOSName() string {
	goos := runtime.GOOS

	switch goos {
	case "linux":
		// Intentar leer /etc/os-release primero (estándar systemd)
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			lines := strings.Split(string(data), "\n")
			var prettyName, name, version string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					val := strings.TrimPrefix(line, "PRETTY_NAME=")
					// Remover comillas simples o dobles
					prettyName = strings.Trim(val, "\"'")
				} else if strings.HasPrefix(line, "NAME=") && name == "" {
					val := strings.TrimPrefix(line, "NAME=")
					// Remover comillas simples o dobles
					name = strings.Trim(val, "\"'")
				} else if strings.HasPrefix(line, "VERSION=") && version == "" {
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
			if name != "" {
				return name
			}
		}

		// Intentar /etc/lsb-release (Ubuntu/Debian)
		if data, err := os.ReadFile("/etc/lsb-release"); err == nil {
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
			if dist != "" {
				return dist
			}
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
	case "windows":
		return "Windows"
	case "darwin":
		// Intentar obtener la versión de macOS
		if data, err := os.ReadFile("/System/Library/CoreServices/SystemVersion.plist"); err == nil {
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
		}
		return "macOS"
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

func SendStatsForServers() {
	var wg sync.WaitGroup
	for _, v := range allServers {
		wg.Add(1)
		go func(p *Server) {
			defer wg.Done()
			stats, err := p.GetEnvironment().GetStats()
			if err != nil {
				return
			}

			_ = p.GetEnvironment().GetStatsTracker().WriteMessage(SkyPanel.Transmission{
				Message: stats,
				Type:    SkyPanel.MessageTypeStats,
			})

			// Monitorear para alertas
			go checkServerAlerts(p, stats)

			// Trackear uptime/downtime
			go trackUptime(p)
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
	err = us.TrackStatus(server.Id(), isRunning)
	if err != nil {
		logging.Error.Printf("[%s] Error tracking uptime: %s", server.Id(), err)
	}
}

func checkServerAlerts(server *Server, stats *SkyPanel.ServerStats) {
	stateTrackingLock.Lock()
	defer stateTrackingLock.Unlock()

	serverID := server.Id()
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
	if state.wasRunning != isRunning {
		ds := services.GetDiscordService()
		if isRunning {
			_ = ds.SendServerOnlineAlert(serverName, serverID)
		} else {
			_ = ds.SendServerOfflineAlert(serverName, serverID)
		}
		state.wasRunning = isRunning
	}

	// Verificar uso alto de recursos (solo si está online)
	if isRunning && stats != nil {
		now := time.Now()

		// CPU > 80%
		if stats.Cpu > 80.0 {
			alertKey := "cpu_high"
			lastAlert, hasAlerted := state.lastAlert[alertKey]
			if !hasAlerted || now.Sub(lastAlert) > 5*time.Minute {
				ds := services.GetDiscordService()
				_ = ds.SendResourceAlert(serverName, serverID, "CPU", stats.Cpu, 80.0)
				state.lastAlert[alertKey] = now
			}
		}

		// RAM > 90%
		// El campo Memory puede estar en bytes o porcentaje dependiendo del entorno
		// Para Docker generalmente es porcentaje (0-100), para TTY puede ser bytes
		// Si Memory > 100, probablemente está en bytes y no podemos calcular porcentaje fácilmente
		// Solo alertar si Memory está en formato de porcentaje (0-100)
		if stats.Memory > 90.0 && stats.Memory <= 100.0 {
			// Es porcentaje
			alertKey := "memory_high"
			lastAlert, hasAlerted := state.lastAlert[alertKey]
			if !hasAlerted || now.Sub(lastAlert) > 5*time.Minute {
				ds := services.GetDiscordService()
				_ = ds.SendResourceAlert(serverName, serverID, "Memoria", stats.Memory, 90.0)
				state.lastAlert[alertKey] = now
			}
		}

		// Limpiar alertas antiguas si el recurso ya no está alto
		if stats.Cpu <= 80.0 {
			delete(state.lastAlert, "cpu_high")
		}
		if stats.Memory <= 90.0 {
			delete(state.lastAlert, "memory_high")
		}
	}

	// Actualizar stats anteriores
	state.lastStats = stats
}

type FileData struct {
	Contents      io.ReadCloser
	ContentLength int64
	FileList      []SkyPanel.FileDesc
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
		Server: SkyPanel.Server{
			Execution: SkyPanel.Execution{
				AutoStart:               false,
				AutoRestartFromCrash:    false,
				AutoRestartFromGraceful: false,
				PreExecution:            make([]SkyPanel.ConditionalMetadataType, 0),
				PostExecution:           make([]SkyPanel.ConditionalMetadataType, 0),
				EnvironmentVariables:    make(map[string]string),
			},
			Type:           SkyPanel.Type{Type: "standard"},
			Variables:      make(map[string]SkyPanel.Variable),
			Display:        "Unknown server",
			Installation:   make([]SkyPanel.ConditionalMetadataType, 0),
			Uninstallation: make([]SkyPanel.ConditionalMetadataType, 0),
			Groups:         make([]SkyPanel.Group, 0),
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

	p.Log(logging.Info, "Starting server %s", p.Id())
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

	var command SkyPanel.Command

	if c, ok := p.Execution.Command.(string); ok {
		command = SkyPanel.Command{Command: c}
	} else {
		//we have a list
		var possibleCommands []SkyPanel.Command
		err = utils.UnmarshalTo(p.Execution.Command, &possibleCommands)
		if err != nil {
			return err
		}

		var defaultCommand SkyPanel.Command
		var commandToRun SkyPanel.Command
		for _, v := range possibleCommands {
			if v.If == "" {
				defaultCommand = v
				break
			}
		}

		for _, v := range possibleCommands {
			//now... we see which command to use
			if v.If == "" {
				continue
			}
			useThis, err := p.RunCondition(v.If, nil)
			if err != nil {
				p.Log(logging.Error, "error starting server %s: %s", p.Id(), err)
				p.RunningEnvironment.DisplayToConsole(true, " Failed to start server\n")
				return err
			}
			if useThis {
				commandToRun = v
				break
			}
		}

		command = commandToRun

		//if no command, use default
		if command.Command == "" {
			command = defaultCommand
		}
	}

	if command.StdIn.Type == "" {
		command.StdIn = p.Execution.Stdin
	}

	data := p.DataToMap()

	commandLine := utils.ReplaceTokens(command.Command, data)
	err = p.RunningEnvironment.ExecuteAsync(SkyPanel.ExecutionData{
		Command:     commandLine,
		Environment: utils.ReplaceTokensInMap(p.Execution.EnvironmentVariables, data),
		Variables:   p.DataToMap(),
		Callback:    p.afterExit,
		StdInConfig: command.StdIn,
	})

	if err != nil {
		p.Log(logging.Error, "error starting server %s: %s", p.Id(), err)
		p.RunningEnvironment.DisplayToConsole(true, " Failed to start server\n")
		return err
	}

	//keepalive!
	if p.KeepAlive.Frequency != "" && p.KeepAlive.Command != "" {
		dur, err := time.ParseDuration(p.KeepAlive.Frequency)
		if err != nil {
			p.RunningEnvironment.DisplayToConsole(true, " Failed to enable keep-alive: %s", err)
			return nil
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

	return err
}

// Stop Stops the program.
// This will also stop the environment it is ran in.
func (p *Server) Stop() error {
	var err error
	if r, err := p.IsRunning(); !r || err != nil {
		return err
	}

	p.Log(logging.Info, "Stopping server %s", p.Id())
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
	p.Log(logging.Info, "Killing server %s", p.Id())
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
	p.Log(logging.Info, "Creating server %s", p.Id())
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

	p.Log(logging.Info, "Destroying server %s", p.Id())

	p.Log(logging.Debug, "Stopping scheduler")
	if p.Scheduler != nil && p.Scheduler.IsRunning() {
		p.Scheduler.Stop()
	}

	p.Log(logging.Debug, "Starting uninstall processes")
	process, err := GenerateProcess(p.Uninstallation, p.RunningEnvironment, p.DataToMap(), p.Execution.EnvironmentVariables)
	if err != nil {
		p.Log(logging.Error, "Error uninstalling server: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to uninstall server\n")
		return
	}

	err = process.Run(p)
	if err != nil {
		p.Log(logging.Error, "Error uninstalling server: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to uninstall server\n")
		return
	}

	p.Log(logging.Debug, "Deleting environment")
	err = p.RunningEnvironment.Delete()
	if err != nil {
		p.Log(logging.Error, "Error uninstalling server: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to uninstall server\n")
	}

	return
}

func (p *Server) Install() error {
	if err := p.IsIdle(); err != nil {
		return err
	}

	p.GetEnvironment().SetInstalling(true)
	defer p.GetEnvironment().SetInstalling(false)

	p.Log(logging.Info, "Installing server %s", p.Id())
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

func (p *Server) SetEnvironment(environment *SkyPanel.Environment) (err error) {
	p.RunningEnvironment = environment
	return
}

func (p *Server) Id() string {
	return p.Identifier
}

func (p *Server) GetEnvironment() *SkyPanel.Environment {
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
	p.Log(logging.Info, "Saving server %s", p.Id())

	file := filepath.Join(config.ServersFolder.Value(), p.Id()+".json")

	if err = p.valid(); err != nil {
		p.Log(logging.Error, "Server %s contained invalid data, this server is.... broken", p.Identifier)
		//we can't even reload from disk....
		//so, puke back, and for now we'll handle it later
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
		var elem SkyPanel.Variable

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

func (p *Server) GetData() map[string]SkyPanel.Variable {
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
	if graceful {
		p.CrashCounter = 0
	}

	mapping := p.DataToMap()
	mapping["success"] = graceful
	mapping["exitCode"] = exitCode

	processes, err := GenerateProcess(p.Execution.PostExecution, p.RunningEnvironment, mapping, p.Execution.EnvironmentVariables)
	if err != nil {
		p.Log(logging.Error, "Error running post processing for server %s: %s", p.Id(), err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to run post-execution steps\n")
		return
	}
	p.RunningEnvironment.DisplayToConsole(true, "Running post-execution steps\n")
	p.Log(logging.Info, "Running post execution steps: %s", p.Id())

	err = processes.Run(p)
	if err != nil {
		p.Log(logging.Error, "Error running post processing for server: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to run post-execution steps\n")
		return
	}

	if graceful && p.Execution.AutoRestartFromGraceful {
		StartViaService(p)
	} else if !graceful && p.Execution.AutoRestartFromCrash && p.CrashCounter < config.CrashLimit.Value() {
		p.CrashCounter++
		StartViaService(p)
	}
}

func (p *Server) GetItem(name string) (*FileData, error) {
	info, err := p.GetFileServer().Stat(name)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		fileList, _ := p.GetFileServer().ReadDir(name)
		var fileNames []SkyPanel.FileDesc
		offset := 0
		if name == "" || name == "." || name == "/" {
			fileNames = make([]SkyPanel.FileDesc, len(fileList))
		} else {
			fileNames = make([]SkyPanel.FileDesc, len(fileList)+1)
			fileNames[0] = SkyPanel.FileDesc{
				Name: "..",
				File: false,
			}
			offset = 1
		}

		//validate any symlinks are valid

		for i, file := range fileList {
			newFile := SkyPanel.FileDesc{
				Name: file.Name(),
				File: !file.IsDir(),
			}

			if !file.IsDir() && file.Type()&os.ModeSymlink == 0 {
				infoData, err := p.GetFileServer().Stat(filepath.Join(name, file.Name()))
				if err != nil {
					continue
				}
				newFile.Size = infoData.Size()
				newFile.Modified = infoData.ModTime().Unix()
				newFile.Extension = filepath.Ext(file.Name())
			}

			fileNames[i+offset] = newFile
		}

		return &FileData{FileList: fileNames}, nil
	} else {
		file, err := p.GetFileServer().Open(name)
		if err != nil {
			return nil, err
		}
		return &FileData{Contents: file, ContentLength: info.Size(), Name: info.Name()}, nil
	}
}

func (p *Server) ArchiveItems(sourceFiles []string, destination string) error {
	// This may technically error out in other cases
	if _, err := os.Stat(destination); err != nil && !os.IsNotExist(err) {
		return SkyPanel.ErrFileExists
	}
	return files.Compress(p.GetFileServer(), destination, sourceFiles)
}

func (p *Server) Extract(source, destination string) error {
	return files.Extract(p.GetFileServer(), source, destination, "*", false, nil)
}

func (p *Server) StartBackup() (string, error) {
	if err := p.IsIdle(); err != nil {
		return "", err
	}

	p.backingUp = true
	c := make(chan bool)
	serverName := p.Server.Display
	if serverName == "" {
		serverName = p.Id()
	}
	go func(d chan bool) {
		r := <-d
		p.backingUp = false
		if r {
			p.RunningEnvironment.DisplayToConsole(true, "Backup complete")
			// Enviar alerta de backup exitoso
			ds := services.GetDiscordService()
			_ = ds.SendBackupAlert(serverName, p.Id(), "Completado exitosamente", true)
		} else {
			p.RunningEnvironment.DisplayToConsole(true, "Backup failed")
			// Enviar alerta de backup fallido
			ds := services.GetDiscordService()
			_ = ds.SendBackupAlert(serverName, p.Id(), "Falló durante la creación", false)
		}
	}(c)

	p.RunningEnvironment.DisplayToConsole(true, "Backing up server")
	backupDirectory := p.GetBackupDirectory()

	_, err := os.Stat(backupDirectory)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(backupDirectory, 0755)
		if err != nil && !os.IsExist(err) {
			c <- false
			return "", err
		}
	}

	backupId, err := uuid.NewV4()
	if err != nil {
		c <- false
		return "", err
	}
	backupFileName := backupId.String() + ".tar.gz"
	backupFile := path.Join(backupDirectory, backupFileName)

	go func(file string, d chan bool) {
		defer func() {
			d <- true
		}()
		sourceFiles := []string{filepath.Join(p.GetFileServer().Prefix())}

		err = files.Compress(nil, file, sourceFiles)
		if err != nil {
			p.Log(logging.Error, "Error creating backup file: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "Failed to create backup file")
		}
	}(backupFile, c)

	return backupFileName, nil
}

func (p *Server) DeleteBackup(fileName string) error {
	backupDirectory := p.GetBackupDirectory()
	if backupDirectory == "" {
		return SkyPanel.ErrSettingNotConfigured("backupDirectory")
	}

	backupFile := path.Join(backupDirectory, fileName)

	err := os.Remove(backupFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (p *Server) StartRestore(fileName string) error {
	if err := p.IsIdle(); err != nil {
		return err
	}

	p.restoring = true
	c := make(chan bool)
	go func(d chan bool) {
		r := <-d
		p.restoring = false
		if r {
			p.RunningEnvironment.DisplayToConsole(true, "Restore complete")
		} else {
			p.RunningEnvironment.DisplayToConsole(true, "Restore failed")
		}
	}(c)

	p.RunningEnvironment.DisplayToConsole(true, "Restoring server")

	backupFile := filepath.Join(p.GetBackupDirectory(), fileName)

	_, err := os.Stat(backupFile)
	if err != nil && !os.IsNotExist(err) {
		c <- false
		return err
	}

	go func(source string, d chan bool) {
		defer func() {
			d <- true
		}()

		//Check if any files exist, as remove all errors if its empty
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

		err = files.Extract(nil, source, p.GetFileServer().Prefix(), "*", true, nil)
		if err != nil {
			p.Log(logging.Error, "Error restoring files: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "Failed to restore files: %s", err)
		}
	}(backupFile, c)

	return nil
}

func (p *Server) GetBackup(fileName string) (*FileData, error) {
	backupFile := filepath.Join(p.GetBackupDirectory(), fileName)

	info, err := os.Stat(backupFile)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return &FileData{ContentLength: info.Size(), Name: info.Name()}, nil
}

func (p *Server) GetBackupFile(fileName string) (*FileData, error) {
	backupFile := filepath.Join(p.GetBackupDirectory(), fileName)

	file, err := os.Open(backupFile)
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
	//we need a type at least, this is a safe check
	if p.Type.Type == "" {
		return ErrServerTypeRequired
	}

	if p.Environment.Type == "" {
		return ErrEnvironmentTypeRequired
	}

	return nil
}

func (p *Server) Log(l *log.Logger, format string, obj ...interface{}) {
	msg := fmt.Sprintf("[%s] ", p.Id()) + format
	l.Printf(msg, obj...)
}

func (p *Server) RunCondition(condition string, extraData map[string]interface{}) (bool, error) {
	data := map[string]interface{}{
		conditions.VariableEnv:      p.RunningEnvironment.Type,
		conditions.VariableServerId: p.Id(),
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
	return p.backingUp
}

func (p *Server) IsRestoring() bool {
	return p.restoring
}

func (p *Server) IsIdle() error {
	if p.IsRestoring() || p.IsBackingUp() {
		return SkyPanel.ErrBackupInProgress
	}

	r, _ := p.GetEnvironment().IsRunning()
	if r {
		return SkyPanel.ErrServerRunning
	}

	if p.GetEnvironment().IsInstalling() {
		return SkyPanel.ErrServerRunning
	}

	return nil
}

func (p *Server) GetBackupDirectory() string {
	return filepath.Join(config.BackupsFolder.Value(), p.Id())
}
