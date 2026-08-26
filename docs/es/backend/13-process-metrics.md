# Process Management & Metrics

Complete reference for Aether Panel's process management, metrics collection, and background services.

---

## Process Queue

The daemon maintains a FIFO queue for server operations to prevent system overload.

### Structure (`internal/servers/server.go`)

```go
var (
    queue           *list.List        // FIFO operations queue
    startQueueTicker *time.Ticker     // Processes queue every 1s
    statTicker      *time.Ticker      // Collects metrics every 5s
    systemStatusTicker *time.Ticker   // System metrics every 1min
)
```

**Note:** No `trackUptimeTicker` variable - uptime tracking is done inside `processStats()`.

### Queue Processing (`processQueue()`)

```go
func processQueue() {
    for {
        select {
        case <-startQueueTicker.C:
            // Process next operation if slot available
            if len(running) < maxConcurrent {
                op := queue.Front()
                if op != nil {
                    queue.Remove(op)
                    go executeOperation(op.Value)
                }
            }
        }
    }
}
```

### Concurrency Control

- **Limit**: Configurable per-scheduler via `ConcurrentLimit` (default 1)
- **Modes**: 
  - `wait` - queue operations until slot available
  - `skip` - skip if at limit
- **Integration**: Scheduler respects limit when scheduling tasks

### Operations Queued

| Operation | Description |
|-----------|-------------|
| Install | Server installation (download, extract, configure) |
| Start | Server startup sequence |
| Stop | Graceful shutdown |
| Restart | Stop + Start |
| Kill | Force terminate |
| Transfer | Move server to another node |

---

## Metrics Collection (`processStats()`)

Collects and broadcasts server metrics every 5 seconds.

### Flow

```go
func processStats() {
    for range statTicker.C {
        for _, server := range servers {
            // 1. Get server stats
            stats, err := env.GetStats()
            
            // 2. Broadcast via WebSocket (StatsTracker)
            env.StatsTracker.Broadcast(ServerStatsMessage(stats))
            
            // 2. Update uptime
            uptimeService.TrackStatus(serverID, stats.Running)
            
            // 3. Check disk limit
            checkDiskLimit(server, stats.Storage)
            
            // 4. Check resource alerts
            checkServerAlerts(server, stats)
        }
    }
}
```

### ServerStats Structure

```go
type ServerStats struct {
    CPU        float64       // CPU percentage
    Memory     int64         // Used bytes
    MaxMemory  int64         // Limit bytes
    Disk       int64         // Used bytes
    MaxDisk    int64         // Limit bytes
    NetworkRx  int64         // Bytes received
    NetworkTx  int64         // Bytes sent
    JVM        *JvmStats     // Java heap/metaspace (if applicable)
    Running    bool          // Server state
}
```

**Note:** No `MaxMemory` field - uses `MaxMemory` from config for limits. No `NetworkRx`/`NetworkTx` in current struct - network stats may be nil.

### JVM Stats (Java Servers)

```go
type JvmStats struct {
    HeapUsed      int64
    HeapTotal     int64
    MetaspaceUsed int64
    MetaspaceTotal int64
}
```

**Parsing:** `internal/utils/jvm.go:ParseJCMDResponse()` - parses `jcmd PID VM.native_memory` output

---

## Resource Alerts (`checkServerAlerts()`)

Monitors CPU, RAM, Disk thresholds and sends Discord notifications.

### Thresholds (Hardcoded in Code)

| Resource | Warning | Critical (Auto-stop) |
|----------|---------|---------------------|
| CPU | 80% | - |
| RAM | 90% | - |
| Disk | 95% | 100% (auto-stop) |

**Note:** Only CPU and RAM have warning thresholds. Disk has auto-stop at 100%.

### Deduplication

```go
var serverStateTracking = make(map[string]*serverState)
type serverState struct {
    wasRunning   bool
    cpuAlerted   bool
    memoryAlerted bool
    diskAlerted  bool
}
```

- Only alerts on threshold **crossing** (not every 5s tick)
- No explicit cooldown - tracks `wasRunning` and alerted flags per server

### Discord Integration

```go
func checkServerAlerts(server *Server, stats *skypanel.ServerStats) {
    // CPU
    if stats.CPU > 80 && !state.cpuAlerted {
        discord.SendResourceAlert(server.Name, server.Identifier, "CPU", stats.CPU, 80)
        state.cpuAlerted = true
    }
    if stats.CPU <= 80 {
        state.cpuAlerted = false
    }
    // ... RAM (90%), Disk (95%) similar
}
```

---

## Disk Enforcement (`checkDiskLimit()`)

Enforces per-server disk limits with auto-stop at 100%.

```go
func checkDiskLimit(server *Server, usedBytes int64) {
    limit := server.TotalDisk // bytes, 0 = unlimited
    if limit == 0 {
        return
    }
    
    if stats.MaxDisk > 0 {
        // Check against max disk from stats
        if stats.Disk > stats.MaxDisk+1024*1024 { // 1MB buffer
            server.Stop()
        }
    }
}
```

**Logic:** Auto-stops when `stats.Disk > stats.MaxDisk + 1MB` (no percentage-based warning in current code).

---

## System Status Broadcast (`sendSystemStatusToDiscord()`)

Hourly system-wide metrics broadcast via Discord webhook.

### Triggered By

```go
func processSystemStatus() {
    for range systemStatusTicker.C {
        sendSystemStatusToDiscord()
    }
}
```

### SystemInfo Structure

```go
type SystemInfo struct {
    Hostname      string
    OS            string
    Platform      string
    PlatformVersion string
    Arch          string
    Uptime        uint64      // seconds
    CPUModel      string
    CPUCores      int
    CPUThreads    int
    CPUUsage      float64
    MemoryTotal   uint64
    MemoryUsed    uint64
    MemoryFree    uint64
    Disks         []DiskInfo
    NetworkRx     uint64
    NetworkTx     uint64
}
```

### Discord Embed

- Total servers, online/offline counts
- Average CPU/RAM across online servers
- Per-server fields (max 25 per Discord limit)

---

## Scheduler / Cron

Per-server gocron-based scheduler persisted in JSON.

### Scheduler Struct

```go
type Scheduler struct {
    scheduler       gocron.Scheduler
    serverID        string
    Tasks           map[string]skypanel.Task
    Timezone        string
    ConcurrentLimit uint
    LimitMode       string  // "wait" | "skip"
}
```

### Task Definition

```go
type Task struct {
    Name        string
    Description string
    CronSchedule string    // "0 */6 * * *"
    Operations  []Operation
}
```

### Features

- Persisted in `{serverID}.cron` JSON file
- Loaded on server creation/start
- CRUD via API: `GET/PUT/DELETE /api/servers/:id/tasks/:taskId`
- Manual run: `POST /api/servers/:id/tasks/:taskId/run`
- Integration with process queue for concurrency control

---

## KeepAlive

Prevents server process from idling out by sending periodic commands.

### Configuration (Server Definition)

```json
{
  "keepAlive": {
    "frequency": "5m",    // Parsed by time.ParseDuration
    "command": "say alive"
  }
}
```

### Implementation (in `server.go` lines 789-813)

```go
func (s *Server) startKeepAlive() {
    if s.Execution.KeepAlive.Frequency == "" || s.Execution.KeepAlive.Command == "" {
        return
    }
    d, _ := time.ParseDuration(s.Execution.KeepAlive.Frequency)
    ticker := time.NewTicker(d)
    go func() {
        for range ticker.C {
            if s.IsRunning() {
                s.ExecuteInMainProcess(s.Execution.KeepAlive.Command)
            }
        }
    }()
}
```

### Lifecycle

- Started in `ExecuteAsyncImpl()` after process starts
- Stopped in `afterExit()` callback
- Uses `ExecuteInMainProcess()` to write to process stdin

---

## Uptime Tracking Integration

Called from `processStats()` every 5 seconds.

```go
func (s *ServerService) processStats() {
    // ... get stats ...
    
    // Track uptime
    if uptimeService := services.NewUptimeService(); uptimeService != nil {
        uptimeService.TrackStatus(server.Identifier, running)
    }
}
```

**UptimeService** (`internal/services/uptime.go`):
- Creates/closes `UptimeStatus` records on state change
- Tracks `start_time`, `end_time`, `duration` (seconds)
- Calculates uptime percentage including active record

---

## Process Lifecycle (`server.go`)

### Start Sequence

```go
func (s *Server) Start() error {
    // 1. Create environment if needed
    // 2. Verify requirements (OS, arch, binaries)
    // 3. Execute pre-commands (PreExecution operations)
    // 4. ExecuteAsyncImpl() -> process start
    // 4a. Create stdin proxy (RCON/Telnet/RCON-WS)
    // 4b. Start process with PTY or pipes
    // 4c. Start KeepAlive goroutine
    // 4d. Set up afterExit callback
    // 5. Register with StatsTracker, ConsoleTracker, StatusTracker
}
```

### Stop Sequence

```go
func (s *Server) Stop() error {
    // 1. Send stop command (configurable: "stop", "quit", etc.)
    // 2. Wait for graceful shutdown (configurable timeout)
    // 3. If timeout: Kill() -> SIGKILL
    // 4. Cleanup: stop trackers, close stdin proxy
}
```

### afterExit Callback

```go
func (s *Server) afterExit(err error) {
    // 1. Update crash counter
    // 2. Auto-restart logic:
    //    - CrashLimit (default 3) reached -> suspend
    //    - autoRestartOnCrash -> restart
    //    - autoRestartOnGraceful -> restart
    // 3. Update uptime
    // 4. Broadcast status change
}
```

---

## Server Creation Flow

```go
func CreateServer(def ServerDefinition) (*Server, error) {
    // 1. Validate definition (Type, Environment.Type required)
    // 2. Assign identifier (UUID)
    // 3. Create directories (root, backup, logs)
    // 4. Save server.json
    // 5. Initialize scheduler (load .cron file)
    // 6. Add to server cache
    // 7. If autostart: queue Start operation
}
```

---

## Port Management

Port management is handled in API handlers, not in server.go:

- `internal/web/api/servers.go:editPortSettings()` - handles primary port + notes
- `internal/web/api/servers.go:editServerDataAdmin()` - handles full port list + proxy to daemon
- Daemon creates `port`, `port2`, `port3` variables from `ports` array

**Note:** The functions `collectPortsFromVariables`, `validatePortsAgainstNode`, `syncPortVarsToDaemon` documented in previous version **do not exist in server.go** - they are in `internal/web/api/servers.go`.

---

## Monitoring Endpoints

| Endpoint | Scope | Description |
|----------|-------|-------------|
| `GET /api/servers/:id/stats` | `server.stats` | Current ServerStats |
| `GET /api/uptime/:id` | `server.view` | Uptime stats |
| `GET /api/uptime` | `admin` | All servers uptime |
| `GET /api/nodes/:id/system` | `nodes.view` | SystemInfo (CPU, RAM, disks, network) |
| `GET /api/nodes/:id/features` | `nodes.view` | Node capabilities (Docker, etc.) |

---

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| Metrics not updating | `statTicker` stopped | Check `InitService()` called |
| Disk enforcement not working | `TotalDisk = 0` | Set `server.TotalDisk` in config |
| Alerts not firing | Discord webhook not configured | Add `DiscordWebhook` in config |
| Scheduler not running | `InitService()` not called | Ensure daemon started |
| KeepAlive not sending | Frequency parse error | Use `5m`, `30s` format (time.ParseDuration) |