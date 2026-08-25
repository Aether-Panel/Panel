# Servers — Environments, Lifecycle, and Scheduler

## Architecture

The server system is based on the concept of **environments** (`Environment`). An environment is an abstraction that defines **how** a game server process is executed. There are two concrete implementations:

## Supported Environments

| Environment | Factory | Description |
|---|---|---|
| `host` | `tty.EnvironmentFactory` | Runs directly on the host (without container) |
| `tty` | `tty.EnvironmentFactory` | Runs in a pseudo-terminal (PTY) on the host |
| `standard` | `tty.EnvironmentFactory` | No PTY, stdout/stderr redirection |
| `docker` | `docker.EnvironmentFactory` | Runs inside a Docker container |

### TTY Environment (`internal/servers/tty/`)

- Runs processes directly on the host operating system.
- Uses `unshare` for namespace isolation (USER, mount, cgroup, IPC, UTS).
- Security options: `unshare` can be disabled with `security.disableUnshare` (config) or `disableUnshare` (per server).
- Implements the `EnvironmentImpl` interface with:
  - `ExecuteAsyncImpl` — launches the process with `os/exec` + PTY (if applicable)
  - `KillImpl` — sends termination signal
  - `GetStatsImpl` — reads `/proc` for process CPU/memory
  - `IsRunningImpl` — verifies process status
  - `GetUIDImpl` / `GetGidImpl` — process UID/GID

### Docker Environment (`internal/servers/docker/`)

- Requires the Docker socket on the host.
- Each server runs in a separate container.
- Directories: mounts the server's `RootDirectory` into the container.
- Networking: automatic port mapping.
- **Default network**: auto-detection of `skypanel-network` (see [Concepts: Docker Network](../general/concepts.md#docker-network-auto-detection-skypanel-network)).
- Implements the same `EnvironmentImpl` interface.
- Files:
  - `docker.go` — container creation and management
  - `factory.go` — `EnvironmentFactory`
  - `container_mount_source.go` — bind mount handling
  - `imagewriter.go` — image download

## ExtraPortBindings — Extra Ports in Containers

The `ExtraPortBindings()` function in `docker.go` automatically generates Docker bindings for all extra ports (`port2`, `port3`, ...):

```go
func ExtraPortBindings(variables map[string]interface{}) []string {
    var specs []string
    for i := 2; ; i++ {
        key := fmt.Sprintf("port%d", i)
        val, ok := variables[key]
        if !ok { break }
        p := cast.ToString(val)
        if p == "" || p == "0" { continue }
        specs = append(specs,
            "0.0.0.0:"+p+":"+p+"/tcp",
            "0.0.0.0:"+p+":"+p+"/udp",
        )
    }
    return specs
}
```

- Searches for `port2`, `port3`... in `ExecutionData.Variables`.
- Generates `0.0.0.0:PORT:PORT/tcp` and `/udp` bindings for each.
- Invoked in `createContainer()` alongside template ports.
- `checkPortConflicts()` also uses it for pre-start validation.

## Environment Interface

```go
// pkg/skypanel/environment.go
type Environment struct {
    Type            string            // "docker", "tty", "host", "standard"
    RootDirectory   string            // server root directory
    BackupDirectory string
    ConsoleBuffer   *MemoryCache      // circular log buffer
    Server          Server            // server definition
    Implementation  EnvironmentImpl   // concrete implementation
    // Trackers pub/sub for WebSocket
    ConsoleTracker  *Tracker
    StatusTracker   *Tracker
    StatsTracker    *Tracker
}
```

```go
type EnvironmentImpl interface {
    ExecuteAsyncImpl(env *Environment, steps ExecutionData) error
    KillImpl(env *Environment) error
    GetStatsImpl(env *Environment) (*ServerStats, error)
    SendCodeImpl(env *Environment, code int) error
    GetUIDImpl(env *Environment) int
    GetGidImpl(env *Environment) int
    IsRunningImpl(env *Environment) (bool, error)
}
```

## Server Lifecycle

```
1. PUT /daemon/server/:serverId
   ├── servers.CreateEnvironment(type, folder, backupFolder, server)
   │   └── envMapping[type].Create() → EnvironmentImpl
   ├── Creates directories (root, backup, etc.)
   └── Adds to server cache

2. POST /daemon/server/:serverId/install
   ├── server.Install()
   │   └── Executes installation operations (download, extraction, etc.)
   └── Processes server.Installation[]

3. POST /daemon/server/:serverId/start
   ├── server.Start()
   ├── Creates Environment if it doesn't exist
   ├── Verifies requirements
   └── ExecuteAsyncImpl(steps) → asynchronous process

4. POST /daemon/server/:serverId/stop
   ├── server.Stop()
   └── Sends stop signal (stopCommand or stopCode)

5. POST /daemon/server/:serverId/kill
   ├── server.Kill()
   └── KillImpl() → SIGKILL

6. DELETE /daemon/server/:serverId
   ├── server.Delete()
   ├── Kills processes
   ├── Deletes files
   └── Removes from cache
```

## Process Queue

The daemon maintains a **FIFO queue** of server operations to prevent overload:

```go
var queue *list.List        // operations queue
var startQueueTicker *time.Ticker  // processes the queue every 1s
var statTicker *time.Ticker        // statistics every 5s
var systemStatusTicker *time.Ticker // system status every 1min
```

- `InitService()` starts 4 goroutines: `processQueue()`, `processStats()`, `processSystemStatus()`, `trackUptimeForAllServers()`
- `processStats()` collects metrics from all servers every 5s and notifies via StatsTracker
- `processSystemStatus()` collects system metrics every 1min
- `trackUptimeForAllServers()` records server uptime

## Scheduler (Scheduled Tasks)

Each server has a scheduler based on **gocron** that persists in `servers/{serverId}.cron`.

```go
type Scheduler struct {
    scheduler       gocron.Scheduler
    serverID        string
    Tasks           map[string]skypanel.Task  // scheduled tasks
    Timezone        string                     // timezone
    ConcurrentLimit uint                       // concurrency limit
    LimitMode       string                     // limit mode
}
```

- Loaded from JSON file when the server starts
- Tasks are defined as operations (command, console, etc.)
- Can be created/edited/deleted via API
- Integration with the process queue

## Server Definition (JSON)

Servers are defined using a structured JSON (`pkg/skypanel/server.go`):

```go
type Server struct {
    Type                    string                    // "generic", "minecraft", etc.
    Identifier              string                    // unique server ID
    Display                 string                    // display name
    Icon                    string                    // icon
    Variables               map[string]Variable       // configuration variables
    Groups                  []Group                   // variable groups (UI)
    Installation            []ConditionalMetadataType  // installation steps
    Uninstallation          []ConditionalMetadataType  // uninstallation steps
    Execution               Execution                 // execution configuration
    Environment             MetadataType              // environment metadata
    SupportedEnvironments   []MetadataType            // supported environments
    Requirements            Requirements              // system requirements
    Stats                   MetadataType              // statistics configuration
    Query                   MetadataType              // query configuration
    KeepAlive               KeepAlive                 // process keepalive
}
```

## KeepAlive

Keeps the process alive by sending periodic commands:

```go
type KeepAlive struct {
    Frequency string   // interval (e.g., "5m")
    Command   string   // command to execute
}
```

## Server Cache

The daemon maintains an in-memory cache of active servers. When the panel communicates with the daemon, it sends the complete server definition.

## Status Change Alert

The system tracks status changes (running ↔ stopped) and can send notifications via Discord webhook.
