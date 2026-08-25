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
    trackUptimeTicker *time.Ticker   // Uptime tracking every 5s
)
```

### Queue Processing (`processQueue()`)

```go
func processQueue() {
    for {
        select {
        case <-startQueueTicker.C:
            // Process next operation if under concurrency limit
            if runningOps < concurrentLimit {
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

- **Limit**: Configurable per-scheduler (`ConcurrentLimit`, default 1)
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
            
            // 3. Update uptime
            uptimeService.TrackStatus(serverID, stats.Running)
            
            // 4. Check disk limit
            checkDiskLimit(server, stats.Storage)
            
            // 5. Check resource alerts
            checkServerAlerts(server, stats.CPU, stats.Memory)
        }
    }
}
```

### ServerStats Structure

```go
type ServerStats struct {
    CPU            float64       // CPU percentage
    Memory         int64         // Used bytes
    MaxMemory      int64         // Limit bytes
    Storage        int64         // Used bytes
    MaxStorage     int64         // Limit bytes
    NetworkRx      int64         // Bytes received
    NetworkTx      int64         // Bytes sent
    JVM            *JVMStats     // Java heap/metaspace (if applicable)
    Running        bool          // Server state
}
```

### JVM Stats (Java Servers)

```go
type JVMStats struct {
    HeapUsed      int64
    HeapTotal     int64
    MetaspaceUsed int64
    MetaspaceTotal int64
}
```

**Parsing**: `internal/utils/jvm.go:ParseJCMDResponse()` - parses `jcmd PID VM.native_memory` output

---

## Resource Alerts (`checkServerAlerts()`)

Monitors CPU, RAM, Disk thresholds and sends Discord notifications.

### Thresholds (Hardcoded)

| Resource | Warning Threshold | Critical Threshold |
|----------|------------------|-------------------|
| CPU | 80% | 95% |
| RAM | 90% | 98% |
| Disk | 90% | 95% |

### Deduplication

```go
type ServerAlertState struct {
    LastCPUAlert    time.Time
    LastRAMAlert    time.Time
    LastDiskAlert   time.Time
    WasRunning      bool
}
```

- Only alerts on threshold **crossing** (not every 5s tick)
- 5-minute cooldown between same-resource alerts
- Resets when usage drops below threshold

### Discord Integration

```go
func checkServerAlerts(server *Server, cpu, mem float64, storage int64) {
    if cpu > 80 && time.Since(state.LastCPUAlert) > 5*time.Minute {
        discord.SendResourceAlert(server.Name, server.Identifier, "CPU", cpu, 80)
        state.LastCPUAlert = time.Now()
    }
    // ... RAM, Disk similar
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
    
    percent := float64(usedBytes) / float64(limit) * 100
    
    if percent >= 95 && percent < 100 {
        // Warning (Discord + log)
        discord.SendResourceAlert(s.Name, s.Identifier, "Disk", percent, 95)
    } else if percent >= 100 {
        // AUTO-STOP
        s.Stop()
        discord.SendAlert("Disk Full", fmt.Sprintf("Server %s stopped: disk at %.1f%%", s.Name, percent))
    }
}
```

---

## System Status Broadcast (`sendSystemStatusToDiscord()`)

Hourly system-wide metrics broadcast via Discord webhook.

### Triggered By

```go
func processSystemStatus() {
    for range systemStatusTicker.C {
        info := getSystemInfo()
        sendSystemStatusToDiscord(info)
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
- Node resource summary

---

## Scheduler / Cron (`internal/servers/scheduler.go`)

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
- Integrates with process queue for concurrency control

### Supported Operations

Same as installation operations (command, console, download, etc.)

---

## KeepAlive (`internal/servers/keepalive.go`)

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

### Implementation

```go
func (k *KeepAlive) Start(env *Environment) {
    ticker := time.NewTicker(k.Frequency)
    go func() {
        for range ticker.C {
            if env.IsRunning() {
                env.ExecuteInMainProcess(k.Command)
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
    uptimeService.TrackStatus(server.Identifier, stats.Running)
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

### Parent/Child Server Limits

```go
func updateParentServerLimitsOnNode(...) {
    // When server created with parent_server_id:
    // 1. Update parent's available CPU/Memory/Disk
    // 2. Child servers inherit parent's remaining resources
}
```

---

## Port Management

```go
func collectPortsFromVariables(vars map[string]interface{}) []uint16 {
    // Collects port, port2, port3... from variables
}

func validatePortsAgainstNode(db, nodeID, serverID, ports []uint16) error {
    // Checks against other servers on same node
    // Range: 1024-65535
    // No duplicates
}

func syncPortVarsToDaemon(c *gin.Context, server *models.Server) {
    // Sends port, port2, port3... to daemon
}
```

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