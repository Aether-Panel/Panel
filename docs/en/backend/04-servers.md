# Servers — Environments, Lifecycle and Scheduler

This page describes how the daemon (`internal/servers`, `internal/servers/docker`, `internal/servers/tty`) runs game servers. The routes are summarised here; the full list is in the [API reference](../reference/11-api-reference.md).

## Architecture

Every game server is a definition (`pkg/skypanel.Server`) plus an **environment** (`Environment`) that decides **how** its process runs. At start-up the daemon loads every regular `<id>.json` file of `daemon.data.servers` into an in-memory cache (`servers.LoadFromFolder`); symlinked definitions are skipped. The panel database only holds the server's row (name, node, owner, ports, limits); the definition and the files live on the node.

The panel never touches a server's files itself: `/api/servers/{id}/...` routes check the caller's permissions and forward the request to the node's `/daemon/server/{id}/...` with a token the panel signs (see [API layer](./05-api-layer.md#panel-to-node-requests)).

## Supported environments

`internal/servers/env_loader.go` registers four environment types:

| Type | Factory | What runs |
|---|---|---|
| `host` | `tty.EnvironmentFactory` | A process on the node itself, in a pseudo-terminal |
| `tty` | `tty.EnvironmentFactory` | Alias of `host` |
| `standard` | `tty.EnvironmentFactory` | Alias of `host` |
| `docker` | `docker.EnvironmentFactory` | A Docker container per server |

The three host types share the same implementation, and the environment's type is stored as `host`. `docker.disallowHost` refuses them (it also lists `bubblewrap`, which is not registered as an environment). The Docker image and compose turn it on, so Docker installs only run containers.

### Host environment (`internal/servers/tty/`)

- Starts the process with `os/exec` in a pseudo-terminal (`creack/pty`). The start command is split into arguments without a shell; a command with a NUL or a line break is refused.
- Isolates the process with `unshare` namespaces (user, mount, files, cgroup, IPC, UTS), mounting only the server folder, the binaries and the cache. `security.disableUnshare` (global) or `disableUnshare` (per server) turns that off. See [unshare](../general/20-security-unshare.md).
- Reads CPU and memory from `/proc`, at most every 5 seconds.
- Cannot reconnect to a process after a daemon restart.

### Docker environment

Files: `docker.go` (container life), `factory.go`, `container_mount_source.go` (host path of the data folder when the daemon runs in a container), `imagewriter.go` (pull output), `user.go` (user, capabilities, process limit) and `prefetch.go` (runtime images).

When a server starts, `createContainer`:

- Pulls the template's image if the node does not have it, showing the progress on the console. A start can be cancelled while the image downloads.
- Creates a container **named after the server ID**, labelled `skypanel.server=<id>`, with `AutoRemove`, TTY and stdin attached, and the Linux platform.
- Mounts the server folder at the template's container root (`/SkyPanel` by default), `<id>.tmp` at `/tmp` and `/var/tmp`, the binaries folder at `/var/lib/SkyPanel/binaries`, and the template's own binds. When the daemon runs in a container, the host paths come from `docker.root` or from the daemon's own mount of `daemon.data.root`.
- Picks the network: the template's network; if it names none (or `bridge`), `docker.serversNetwork` when that network exists, otherwise the network of the panel container (`bridge` outside Docker). See [Concepts: Docker network](../general/concepts.md#docker-network-auto-detection-skypanel-network).
- Publishes the template's port bindings, every extra port variable (`port2`, `port3`…, TCP and UDP, see below), and the stdin port, if the template uses one, on `127.0.0.1`.
- Applies the resource limits from the server variables:

| Variable | Docker setting |
|---|---|
| `cpu` | `NanoCPUs = cpu × 10⁷`: `100` is one core, `200` two. `0` or empty means no limit. |
| `memory` | Memory in MB. Swap is set to the same value, so the container gets no swap. |
| `io` | `BlkioWeight`, see [Disk priority](#disk-priority-io). |

- Runs the hardening below.

`docker.prefetchImages` pulls the runtime images in the background on every Docker node, two minutes after start and then daily, and removes a replaced copy when no container uses it.

#### Container user and isolation

When the panel runs as root (the usual Docker install), game servers do **not** run as root, as in Pterodactyl:

- **The game** runs as `docker.serverUser` (`988:988` by default). **The installation** runs as root, since its scripts install packages. A template's own user is kept. A panel that does not run as root runs everything as itself.
- **Before every start** the panel gives every file in the server folder to that user. The walk goes folder by folder through open descriptors and never follows symlinks, so servers created while games ran as root move over on their own.
- **The server folder** belongs to that user from its creation, and what the panel writes (file manager, SFTP, copies, restores) is created with it.
- **Capabilities:** the game container drops `setpcap`, `mknod`, `audit_write`, `net_raw`, `dac_override`, `fowner`, `fsetid`, `net_bind_service`, `sys_chroot` and `setfcap` (Wings' list) and gets `no-new-privileges`. Containers running as root (installs) only drop `setpcap`, `mknod`, `audit_write`, `net_raw` and `sys_chroot`, keeping what package installs need. The template's own lists are kept.
- **Process limit:** each container gets `docker.pidsLimit` processes and threads (4096 by default) unless the template sets its own, so a fork bomb cannot take the node down.
- **`/tmp` and `/var/tmp`** are a folder next to the server's (`<id>.tmp`): emptied when each container is created (sticky, mode 1777, owned by the container user), counted against the disk limit and removed with the server. It is not in the file manager or the backups.
- **Symlinks the game leaves:** the panel works as root, so it never follows a link the game made. Install downloads, extracting, compressing, backups and the ownership change before each start open every file beneath the server folder; a link is stored as a link and a named pipe blocks nothing.
- **What the owner cannot change:** memory, CPU, disk, IP and ports are set by an administrator even when the template marks them editable. A sub-server only publishes its own port variables, with no labels and no container user chosen by the customer.

#### Extra ports (`ExtraPortBindings`)

`ExtraPortBindings()` in `docker.go` turns the extra port variables into Docker bindings, so every port assigned to a server is reachable whatever the template's own `portBindings` say:

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

- It stops at the first missing `portN` and skips empty or `0` values.
- `createContainer()` adds them to the template ports; the start-time conflict check (`CheckBindingsConflict`) uses the same list.
- The panel writes those variables: when an administrator saves the port list (`PUT /api/servers/{id}/data`) or the owner picks the main port (`PUT /api/servers/{id}/port-settings`), the daemon receives `ports` and `primaryPort` and rewrites `port`, `port2`, `port3`…, removing stale ones. See [API layer: ports](./05-api-layer.md#port-management).

#### Disk priority (`io`)

The `io` variable shares disk reads and writes between the servers on the same node. It only applies to the Docker environment, as the container's `BlkioWeight`.

| Value | Effect |
|-------|--------|
| 10 to 1000 | The server's weight against the others. `500` is an even share. |
| Below 10 / above 1000 | Clamped to 10 / 1000. |
| `0`, empty or not a number | No weight is set; Docker uses its default. |

Docker only applies it when the host's I/O scheduler supports weights. The value is read when the container is created, so a change counts from the next start. In the panel it shows as **Disk priority** next to CPU, memory and disk (default `500`).

## Environment interface

```go
// pkg/skypanel/environment.go
type Environment struct {
    Type            string          // "docker" or "host"
    RootDirectory   string          // server folder (must stay inside daemon.data.servers)
    BackupDirectory string          // <backups>/<id>
    ConsoleBuffer   *MemoryCache    // recent console output
    ServerID        string
    ConsoleTracker  *Tracker        // pub/sub for the websocket
    StatusTracker   *Tracker
    StatsTracker    *Tracker
    Server          Server          // the definition
    Implementation  EnvironmentImpl // docker or tty
    DiskUsage       func() int64    // the server's cached folder size
    // ...
}

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

Two optional interfaces, both implemented by Docker only:

```go
type StartCanceler interface { CancelStartImpl() }                                    // stop a start whose image is still downloading
type Reattacher interface { ReattachImpl(env *Environment, steps ExecutionData) error } // reconnect after a daemon restart
```

## Server lifecycle

| Step | Daemon route | What happens |
|---|---|---|
| Create | `PUT /daemon/server/{id}` | Validates the definition, refuses a host environment under `docker.disallowHost`, creates the environment and folders, saves `<id>.json` and adds the server to the cache. The panel then starts the installation itself when `panel.servers.autoInstall` is on, and requests a subdomain when `panel.dns.autoAssign` is on. |
| Install | `POST /daemon/server/{id}/install` | Only one install at a time per server. Refused under `docker.disallowHost` for host environments, and with `ErrNodeDiskFull` when the node has less free space than `daemon.minFreeDiskMB`. Stops the server if it runs, then runs the template's `install` operations (see [Operations](./06-operations.md)). `?wait` waits for it to finish. |
| Start | `POST /daemon/server/{id}/start` | Refused while the server is busy (installing, backing up, restoring), for a host environment under `docker.disallowHost`, when the disk is full (`409`, see [Disk limit](#disk-limit)) and when a port is taken (`409 ErrPortBusy`, with the port in `metadata.port`). The panel refuses it for a suspended server. The process starts asynchronously. |
| Restart | `POST /daemon/server/{id}/restart` | Stop, wait for the exit, start. |
| Stop | `POST /daemon/server/{id}/stop` | Sends the template's `stopCode` signal, or types its `stopCommand` on the console. |
| Kill | `POST /daemon/server/{id}/kill` | Kills the process (`SIGKILL` / container kill). |
| Redefine | `PUT /daemon/server/{id}/definition` | Replaces the definition (same environment checks as create). |
| Reload | `POST /daemon/server/{id}/reload` | Loads the definition and the scheduled tasks again from disk; only while the server is idle. |
| Delete | `DELETE /daemon/server/{id}` | Kills it if it runs, stops the scheduler, runs the template's uninstall operations (a failure is logged and shown on the console, and the deletion goes on), deletes the environment, then removes `<id>.json`, the `<id>.tmp` folder, the side files (`.cron`, `.crashes`, `.notify`, `.metrics`, `.worldmap`, `.modpack`) and the file history. Last, `servers.DeleteBackups` removes `<backups>/<id>/`, unless the request has `?keepBackups=true`: a node transfer that is undone uses it to remove the server from the new node without touching backups that were already there (the copies it brought are removed one by one). |

A stop or kill someone asked for is never treated as a crash. When the panel stops, it asks every server to stop at the same time and gives each up to one minute before killing it (`docker-compose.yml` gives the panel container 90 seconds).

## Service loop

`servers.InitService()` starts seven goroutines (`internal/servers/server.go`):

| Goroutine | Interval | Job |
|---|---|---|
| `processQueue()` | 1 s | Starts the servers queued with `StartViaService` (auto-start at boot, automatic restarts), one queue for the node. |
| `processStats()` | 5 s | Collects every server's stats, publishes them on the stats tracker, records the performance history, checks the disk limit and the resource alerts, and tracks uptime. |
| `processPlayers()` | 1 min | Records the player count of every running server that answers queries. |
| `processSystemStatus()` | 1 min | Sends the system summary to the Discord system webhook (panel only, if configured; it lists the servers the panel runs itself, and above 25 servers it lists 24 and a note), and one report per node to the node webhook when that one is set. A remote node's server counts come from the status poller below (or a direct call when its last answer is older than 60 s) and its hardware from its own `/daemon/system` (10 s timeout; no CPU frequency); a node that does not answer gets a red "no answer" report with the number of servers placed on it. The panel's node is read from its own machine. |
| `processRemoteStatus()` | 30 s | Panel only: asks every remote node for the state of all its servers (`GET /daemon/servers/status`, one request per node, 10 s timeout, rounds never overlap), records their uptime and keeps each node's answer for the node report. |
| `reattachRunningServers()` | once | Reconnects to the containers that kept running across a daemon restart. |
| `trackUptimeForAllServers()` | once (then every stats round) | Records the uptime of the servers this process runs in the panel database; it does nothing on a node without the panel. Remote nodes' servers are recorded by `processRemoteStatus()`. |

More detail in [Process management and metrics](./13-process-metrics.md).

## Crashes and automatic restarts

What the owner sees is described in [Crash reports](../features/crash-reports.md).

When the process exits, `afterExit` (`server.go`) decides what happened:

- An exit with the template's `expectedExitCode` is a clean exit. A stop or kill someone asked for is not a crash either.
- Anything else is a **crash**: the console shows `Server crashed (exit code N)`, a crash report is saved, the owner's crash notification and the Discord alert are sent, and the status tracker emits a `crash` message (the web client shows it as a toast).
- If the template has `autoRestartFromCrash`, the server is started again while the number of crashes in a row is below `daemon.data.crashLimit` (3). A server that ran for more than 10 minutes before crashing starts a new count.
- After a clean exit the server is restarted only if the template has `autoRestartFromGraceful`.
- The template's `post` operations run after every exit, with `success` and `exitCode` available to them.

A crash report (`<id>.crashes`, the newest 10, `GET /api/servers/{id}/crashes`) keeps the time, the exit code, the last 60 console lines (each cut at 500 characters), whether the server was restarted, and the attempt number against the limit.

Resource alerts: CPU above 80 % of what the server may use or memory above 90 % sends an alert at most every 5 minutes per resource. An exit nobody explained (no stop, clean exit or crash report) is reported as "offline" after 15 seconds.

## Backups and restore points

User guide: [Backups](../features/backups.md).

| Kind | File name | Made by | Kept |
|---|---|---|---|
| Manual | `<uuid>.tar.gz` | `POST /api/servers/{id}/backup/create` | Until deleted; users are capped by `panel.backups.maxPerServer` |
| Automatic | `auto-<uuid>.tar.gz` | The `backup` operation of a scheduled task | The newest `keep` (5 by default, 50 at most); only `auto-` files are pruned |
| Restore point | `restore-<reason>-<uuid>.tar.gz` | The daemon, before a risky step | The newest 3 |

- Backups live in `daemon.data.backups.folder/<id>/`. The archive is written under a hidden name and moved into place when complete, so a half-written backup is never listed, downloaded or restored. Only one backup or restore runs at a time per server.
- The panel records a manual backup as soon as it is asked for. If its archive then cannot be written, the node leaves a hidden `.failed-<name>` mark; `GET /daemon/server/{id}/backup` lists it with `location: "failed"`, and the panel's next sync deletes the record and calls `DELETE` on the node, which removes the mark. Scheduled backups leave no mark (the task reports its own failure).
- Deleting the server deletes its backup folder on the node; copies in the off-site bucket stay. A node transfer copies the backups kept on the node (`local` and `both`) to the new node through the panel (`PUT /daemon/server/{id}/backup/file`), then deletes the server and its backups from the old node; off-site copies are not touched. See [Server transfers](../features/server-transfers.md).
- **Restore points** are taken before installing or removing a plugin (at most one every 30 minutes for a run of plugin changes; only once Modrinth offered a compatible file, or the plugin to remove exists), installing a world map, changing the game version, installing a modpack, and restoring a backup. They do not count against the automatic-backup retention.
- **Restore** (`POST /api/servers/{id}/backup/restore/{backupID}`) only runs on a stopped, idle server. The archive is checked first, a restore point is written (without it the restore does not go ahead), the server's files are removed and the archive is extracted.
- With **off-site backups** on (`daemon.backups.s3.*`), every finished backup is also uploaded to the bucket; a failed upload is reported on the console but the local backup stays. The list merges both places (`location`: `local`, `offsite` or `both`), a backup only in the bucket is downloaded before it is restored, and deleting removes both copies.

## File history

User guide: [File history](../features/file-history.md).

Each time a text configuration file is saved from the panel, the daemon keeps its previous content in `<servers>/<id>.history/` (outside the server folder, so the owner cannot see or alter it there):

- Only text formats are kept (`.properties`, `.yml`, `.yaml`, `.json`, `.json5`, `.toml`, `.conf`, `.cfg`, `.ini`, `.txt`, `.xml`, `.sk`, `.js`, `.ts`, `.lua`, `.mcmeta`, `.cnf`, `.env`), files up to 1 MB, and nothing that looks binary.
- Up to 20 versions per file and 50 MB per server; the oldest go first.
- Restoring a version saves the current text as a version first, so the restore can be undone.

Routes: `GET /api/servers/{id}/file-history`, `GET .../file-history/{versionId}`, `POST .../file-history/{versionId}/restore`.

## Scheduler (scheduled tasks)

User guide: [Scheduled tasks](../features/scheduled-tasks.md).

Each server has a **gocron** scheduler stored in `<servers>/<id>.cron` (written atomically, mode `0600`):

```go
type Scheduler struct {
    Tasks           map[string]skypanel.Task `json:"tasks"`
    Timezone        string                   `json:"timezone,omitempty"`
    ConcurrentLimit uint                     `json:"concurrentLimit"`
    LimitMode       string                   `json:"limitMode"`
    Paused          bool                     `json:"paused,omitempty"`
}
```

- The scheduler is loaded and started when the daemon loads the server, not when the server starts, so tasks also run while the server is stopped (a `start` task can switch it on).
- A server holds at most 50 tasks (`MaxTasks`); adding one more is refused.
- Tasks are lists of operations. Besides the normal [operations](./06-operations.md), tasks can use `start`, `stop`, `restart` (only restarts a running server) and `backup` (automatic backup with a `keep` argument).
- Schedules run in the time zone of the person who created them (`CRON_TZ=`); the binary embeds the zone database.
- **Pause** (`SetPaused`, `IsPaused` in `internal/servers/scheduler_pause.go`): while `Paused` is true, `runScheduledTask` skips every run, scheduled or from **Run now**, and logs `Task <id> skipped: the server is being moved to another node`; a task already running goes on. The flag is saved in the `.cron` file, so it survives a node restart. Only a node transfer sets it, through `POST /daemon/server/{id}/scheduler/pause` and `/scheduler/resume`: it pauses the tasks on both nodes, copies them to the new node (`GET` the list, `PUT` each task; `Timezone`, `ConcurrentLimit` and `LimitMode` are not copied) and resumes them on the node that keeps the server. See [Server transfers](../features/server-transfers.md).
- Routes: `GET /api/servers/{id}/tasks`, `GET/PUT/DELETE .../tasks/{taskId}`, `POST .../tasks/{taskId}/run`.

## Server definition (JSON)

`pkg/skypanel/server.go`:

```go
type Server struct {
    Type                  string                    // "generic", "minecraft-java"...
    Identifier            string                    // server ID
    Display               string                    // display name
    Icon                  string
    Variables             map[string]Variable       // configuration variables
    Groups                []Group                   // variable groups (UI)
    Installation          []ConditionalMetadataType // install steps
    Uninstallation        []ConditionalMetadataType // uninstall steps
    Execution             Execution                 // command, stop, restarts, post steps
    Environment           MetadataType              // environment and its settings
    SupportedEnvironments []MetadataType
    Requirements          Requirements
    Stats                 MetadataType
    Query                 MetadataType
    KeepAlive             KeepAlive
}
```

Server identifiers are URL- and path-safe (new ones are 12 uppercase hexadecimal characters; legacy 8-character ones are still valid, up to 20 characters), because they are embedded in daemon URLs and file names.

## KeepAlive

Keeps the process busy by sending a console command periodically:

```go
type KeepAlive struct {
    Frequency string // interval, e.g. "5m"
    Command   string // command to type
}
```

Each run has its own ticker and stop channel; it stops when the process exits and starts again after a reconnection.

## Per-server files on the node

Besides its folder, each server has these entries in `daemon.data.servers`:

| Path | Content |
|---|---|
| `<id>/` | The server folder |
| `<id>.json` | Definition |
| `<id>.tmp/` | `/tmp` and `/var/tmp` of the Docker container |
| `<id>.cron` | Scheduled tasks |
| `<id>.crashes` | Crash reports |
| `<id>.metrics` | Performance history |
| `<id>.notify` | The owner's alerts (channels and events), mode `0600` |
| `<id>.history/` | File history |
| `<id>.modpack` | Installed modpack |
| `<id>.worldmap` | Installed world map |

All of them are removed with the server. Cloning copies only the server files plus `.modpack` and `.worldmap`.

**Owner alerts** (`GET/PUT /api/servers/{id}/notifications`, `POST .../notifications/test`, scope `server.flags.edit`): channels are a Discord webhook, a Telegram bot and chat, or a JSON webhook (optionally signed with a secret); events are crashes, backups, resources and power. The test can be sent once every 10 seconds. See [Server notifications](../features/server-notifications.md).

## Performance history

The 5-second samples are folded into one-minute points (kept for a day) and 15-minute points (kept for a week): CPU, memory, network, share of time online and players. They are saved to `<id>.metrics` every 5 minutes and when the node stops. `GET /api/servers/{id}/metrics` offers the windows `1h` (1-minute steps), `24h` (5-minute steps) and `7d` (1-hour steps); steps when the node was down are returned as `noData` instead of being drawn across. See [Server statistics](../features/server-statistics.md).

## Disk limit

The limit is the server's `disk` variable, in MB. Without `disk`, or with `0`, there is no limit. Code: `internal/servers/disk.go`.

### How it is measured

- The daemon walks the server folder and adds up the size of its files. On Docker its `/tmp` (the `<id>.tmp` folder) counts too.
- The measure is trusted for 30 seconds, and the server's stats use that same measure, so the folder is not walked twice. What the panel writes (uploads, SFTP, extraction, plugins) is added at once. What the game itself writes is only seen at the next measure.
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

### Node free space

Separately from each server's limit, the node's own disk is watched:

- An install is refused when the disk holding `daemon.data.servers` has less free space than `daemon.minFreeDiskMB` (2048 MB). The console says how much is free and how much is needed. If the free space cannot be read, the install goes ahead.
- The node reports its disk as **low** when less than 10 % or 5 GB is free; the panel then warns the administrators.

## Reconnecting after a daemon restart

A Docker container keeps running when the daemon restarts (an update, a crash). On an install where panel and daemon run together, restarting the panel also restarts the daemon. Code: `internal/servers/reattach.go`.

At boot, the daemon reconnects in the background to every server that is still running (`reattachRunningServers()`):

- The console receives the container's output again and accepts commands.
- The process exit goes through the same `afterExit` as a normal start (once): crash detection, auto-restart, notifications.
- If the server has KeepAlive, it is started again.
- `startedAt` takes the time of the reconnection. It only delays when earlier crashes are forgiven.
- The console shows *Reconnected to the running server after a daemon restart*.

`Stop()`, `Kill()` and sending a command also reconnect first, in case they arrive before the background pass. A lock (`attachMu`) keeps two reconnections from running at once.

It does nothing when the server is stopped or already connected. A host process cannot be reached again: it returns `ErrNotImplemented` and nothing is logged. When the reconnection fails, the error is logged and the server stays as it was; it is reachable again once it is restarted.

## Subdomain (Cloudflare)

A server can have a subdomain in the administrator's Cloudflare zone (for example `survival.play.example.com`), configured with the `panel.dns.*` keys (see [Configuration](./02-configuration.md#server-subdomains-cloudflare-dns)). The records are DNS-only (not proxied), because game traffic is not HTTP:

- An `A`, `AAAA` or `CNAME` record with the subdomain's name, pointing at the node: its public IP; for the local node, `panel.settings.nodeIp` when it is public; otherwise its hostname as a `CNAME`, which may be under another domain (`pablo.example.es` → `usa.example.com`). `panel.dns.target` is only used for a node with none of those.
- For servers of type `minecraft` or `minecraft-java` with a port, an `SRV` record (`_minecraft._tcp.<subdomain>`) carrying the main port. Behind a `CNAME` it targets the node's name, never the alias.

The server stores the subdomain, its record IDs and the base domain and zone they were made in (`subdomain`, `dns_record_ids`, `dns_domain`, `dns_zone_id`). The server information (`ServerInfo`) carries the full address as `subdomain` and `subdomainSrv` when the address needs no port.

Routes (scope `server.flags.edit` or `server.data.edit`): `GET/PUT/DELETE /api/servers/{id}/subdomain` and `GET /api/servers/{id}/subdomain/check`. Each server can change its subdomain once every 10 seconds. With `panel.dns.autoAssign` on, a new server gets a name derived from its own (with `-2`, `-3`… on a clash, up to 20 tries).

The subdomain follows the server. These changes update its records:

| Change | Route |
|--------|-------|
| Main port or server type | `PUT /api/servers/{id}/definition` |
| Main port (administrator) | `PUT /api/servers/{id}/data` |
| Main port picked in the port settings | `PUT /api/servers/{id}/port-settings` |
| Move to another node | internal transfer between nodes |
| Node's public host | `PUT /api/nodes/{id}` (that node's servers) |
| Master node IP or Base URL | `POST /api/settings`, `PUT /api/settings/{key}` (local node's servers) |
| DNS target | `POST /api/settings`, `PUT /api/settings/{key}` (every server with a subdomain) |

The update (`refreshSubdomainRecords` in `internal/web/api/server_dns.go`):

- Runs in the background, half a second after the change is saved.
- Edits the server's records in place; the subdomain name does not change.
- If the node is now behind a hostname, the `A` record becomes a `CNAME`.
- Creates or removes the `SRV` record when its rule changes (for example, the type is no longer Minecraft).
- Only touches the records the server has stored.
- Is best effort: if Cloudflare fails, the error is logged and the server change is not undone.

It does nothing when the server has no subdomain or Cloudflare is not configured. Changes that touch many servers run one server at a time (`sweepSubdomains`).

A new base domain or zone ID moves every subdomain (`moveSubdomain`): the same name is created under the new domain, saved, and the old records are removed from the zone they were made in. A name already used in the new zone outside the panel leaves that server where it was. Before such a save, servers whose subdomain predates `dns_domain` get the current settings written in (`rememberSubdomainHome`).

User and administrator guide: [Subdomains](../features/subdomains.md).

## Other per-server features

Each has its own guide: [Game versions](../features/game-versions.md), [Modpacks and plugins](../features/modpacks-and-plugins.md), [Cloning](../features/cloning.md), [World map](../features/world-map.md) and [Sub-servers](../features/sub-servers.md).

| Feature | Routes | Notes |
|---|---|---|
| Game version change | `GET/POST /api/servers/{id}/version` | For templates that download the game with `paperdl` or `mojangdl`: stop, restore point, new version, reinstall, start. `POST` needs `panel.experimental.versionSwitch`. Release lists are cached for an hour. |
| Modpacks | `GET /api/servers/{id}/modpacks`, `.../search`, `.../versions`, `POST .../install` | Modrinth (`.mrpack`) or CurseForge server packs (CurseForge needs `daemon.curseforge.key`). Runs in the background: stop, restore point, clean, download, loader, Java and game version, start. |
| Clone | `POST /api/servers/{id}/clone` | Needs `panel.experimental.serverClone`. Copies the files into a new server on the same node; the copy is marked as installing until it finishes. Symlinks that leave the source, devices and sockets are skipped. |
| World map | `GET/DELETE /api/servers/{id}/map`, `POST .../map/install` | BlueMap or Dynmap from Modrinth for Minecraft Java, shown through the panel with a signed link (`/api/map-view/...`). |
| Players | `GET /api/servers/{id}/query` | Minecraft server list ping or Steam A2S, cached for 10 seconds. |
| Plugins | `GET/DELETE /api/servers/{id}/plugins`, `.../plugins/search`, `POST .../plugins/{pluginId}` | List, install and delete use `mods/` on Fabric, Forge and NeoForge servers and `plugins/` otherwise (platform detected as for the world map). The Modrinth search is limited to that platform: Paper gets plugins and mods built for Paper, Spigot or Bukkit; Fabric, Forge and NeoForge get server-side mods for their loader; an unknown platform gets everything. Install and removal take a restore point first. A Modrinth file that is not a plain `.jar` name answers `502 ErrPluginBadFileName`. |
