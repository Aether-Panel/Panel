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

An environment can also implement `Reattacher`, to connect again to a process that kept running while the daemon was down. Only Docker implements it (see [Reconnecting after a daemon restart](#reconnecting-after-a-daemon-restart)):

```go
type Reattacher interface {
    ReattachImpl(env *Environment, steps ExecutionData) error
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
   ├── Disk full → 409 ErrDiskLimitExceeded (see Disk limit)
   ├── Port conflict → 409
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

- `InitService()` starts 6 goroutines: `processQueue()`, `processStats()`, `processPlayers()`, `processSystemStatus()`, `reattachRunningServers()`, `trackUptimeForAllServers()`
- `processStats()` collects metrics from all servers every 5s and notifies via StatsTracker
- `processPlayers()` stores the player count of running servers every minute
- `processSystemStatus()` collects system metrics every 1min
- `reattachRunningServers()` reconnects to servers that kept running across a daemon restart
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

## Disk limit

The limit is the server's `disk` variable, in MB. Without `disk`, or with `0`, there is no limit. Code: `internal/servers/disk.go`.

### How it is measured

- The daemon walks the server folder and adds up the size of its files.
- The measure is trusted for 15 seconds. What the panel writes (uploads, SFTP, extraction, plugins) is added at once. What the game itself writes is only seen at the next measure.
- Deleting files (from the panel or over SFTP) drops the measure, so freed space counts right away.

### What is blocked

A server that has already used all of its disk does not start and accepts no new files until something is deleted. The daemon API answers `409` with the error `ErrDiskLimitExceeded`.

| Operation | Behaviour |
|-----------|-----------|
| Start / restart | The API refuses it with `409` before starting. `Start()` measures the folder again and, when it is full, writes to the console: *The server has used all of its disk space. Delete files to free some before starting it.* |
| File upload (file `PUT`) | Refused when the disk is already full. Every byte counts: when the upload goes over the limit it is cut short, the partial file is deleted and the answer is `409`. When a file is overwritten, its old size stops counting. |
| SFTP writes | Refused when the disk is already full. Only what makes the file grow counts; rewriting bytes it already had is free. See below for an upload cut short. |
| Extracting an archive | Refused when the disk is already full. Every extracted file counts: extraction stops at the limit (`409`) and the files already extracted stay. |
| Compressing | Refused when the disk is already full. The archive counts as it is written: if it goes over the limit it is deleted and the answer is `409`. |
| Installing a plugin | Refused when the disk is already full. When the download goes over the limit, the partial file is deleted and the answer is `409`. |

### SFTP uploads cut short by the limit

When an SFTP write would go over the limit, it fails and writes nothing. When the file is closed:

- If the upload started from an empty file (the client opened it with truncate, the usual case for an upload), the partial file **is deleted**: half a file is useless and would keep the disk full.
- If it was a resumed or appending upload (no truncate), the file is kept.

In both cases the disk measure is dropped and taken again.

### Running server

On top of that, every 5 seconds `processStats()` compares disk usage with the limit. A running server more than 1 MB over it is stopped (`Stop()`, then `Kill()` if that fails) and a warning is written to the console. On Docker the folder size used by the stats is recalculated at most every 30 seconds.

## Disk priority (`io`)

The `io` variable shares disk reads and writes between the servers on the same node. It only applies to the Docker environment, as the container's `BlkioWeight`.

| Value | Effect |
|-------|--------|
| 10 to 1000 | The server's weight against the others. `500` is an even share. |
| Below 10 / above 1000 | Clamped to 10 / 1000. |
| `0`, empty or not a number | No weight is set; Docker uses its default. |

Docker only applies it when the host's I/O scheduler supports weights. The value is read when the container is created, so a change counts from the next start. In the panel it shows as **Disk priority** next to CPU, memory and disk (default `500`).

## Reconnecting after a daemon restart

A Docker container keeps running when the daemon restarts (an update, a crash). On an install where panel and daemon run together, restarting the panel also restarts the daemon. Code: `internal/servers/reattach.go`.

At boot, the daemon reconnects in the background to every server that is still running (`reattachRunningServers()`):

- The console receives the container's output again and accepts commands.
- The process exit goes through the same `afterExit` as a normal start (once): crash detection, auto-restart, notifications.
- If the server has KeepAlive, it is started again.
- `startedAt` takes the time of the reconnection. It only delays when earlier crashes are forgiven.
- The console shows *Reconnected to the running server after a daemon restart*.

`Stop()`, `Kill()` and sending a command also reconnect first, in case they arrive before the background pass. A lock (`attachMu`) keeps two reconnections from running at once.

It does nothing when the server is stopped or already connected. A host process (`tty`, `host`, `standard`) cannot be reached again: it returns `ErrNotImplemented` and nothing is logged. When the reconnection fails, the error is logged and the server stays as it was; it is reachable again once it is restarted.

## Subdomain (Cloudflare)

A server can have a subdomain in the administrator's Cloudflare zone (for example `survival.play.example.com`). The records are DNS-only (not proxied), because game traffic is not HTTP:

- An `A`, `AAAA` or `CNAME` record with the subdomain's name, pointing at the node's address.
- For servers of type `minecraft` or `minecraft-java` with a port, an `SRV` record (`_minecraft._tcp.<subdomain>`) carrying the main port.

The subdomain follows the server. These changes update its records:

| Change | Route |
|--------|-------|
| Main port or server type | `PUT /api/servers/:serverId/definition` |
| Main port (administrator) | `PUT /api/servers/:serverId/data` |
| Main port picked in the port settings | `PUT /api/servers/:serverId/port-settings` |
| Move to another node | internal transfer between nodes |

The update (`refreshSubdomainRecords` in `internal/web/api/server_dns.go`):

- Runs in the background, half a second after the change is saved.
- Edits the server's records in place; the subdomain name does not change.
- If the node is now behind a hostname, the `A` record becomes a `CNAME`.
- Creates or removes the `SRV` record when its rule changes (for example, the type is no longer Minecraft).
- Only touches the records the server has stored.
- Is best effort: if Cloudflare fails, the error is logged and the server change is not undone.

It does nothing when the server has no subdomain or Cloudflare is not configured.
