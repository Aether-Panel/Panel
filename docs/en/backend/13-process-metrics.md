# Process Management and Metrics

How the daemon runs its background loops, starts and restarts servers, collects live statistics, keeps a performance history per server, and measures the node itself (CPU, disks, network history).

Related pages: server lifecycle, scheduler, keep-alive and disk limit in [Servers](./04-servers.md); the WebSocket `stat` messages in [WebSocket](./08-websocket.md); uptime as a feature in [Uptime](../general/13-uptime.md). User-level guides: [Server statistics](../features/server-statistics.md), [Crash reports](../features/crash-reports.md), [Dashboard and nodes](../features/dashboard-and-nodes.md).

---

## Background loops

`servers.InitService()` (`internal/servers/server.go`) is called by `cmd/panel/run.go` after the servers are loaded. It starts:

| Goroutine | Interval | What it does |
|---|---|---|
| `processQueue()` | 1 s | Starts the next server waiting in the start queue |
| `processStats()` | 5 s | `SendStatsForServers()` (live stats, history, alerts, disk limit) and `trackUptimeForAllServers()` |
| `processPlayers()` | 1 min | `samplePlayers()`: player counts into the history |
| `processSystemStatus()` | 1 min | `sendSystemStatusToDiscord()` |
| `processRemoteStatus()` | 30 s | `pollRemoteNodes()`: state of every remote node's servers, recorded as uptime ([below](#uptime-tracking)) |
| `reattachRunningServers()` | once | Reconnects to Docker containers that kept running across a daemon restart ([Servers](./04-servers.md#reconnecting-after-a-daemon-restart)) |
| `trackUptimeForAllServers()` | once (then every 5 s from `processStats`) | Uptime periods |

Also started by `run.go`, outside `InitService`:

| Function | Interval | What it does |
|---|---|---|
| `daemon.StartNetworkHistory()` (`internal/web/daemon/network_history.go`) | 10 s | Node network history |
| CPU sampler (`internal/web/daemon/root.go`) | 2 s windows | Started lazily by the first `/daemon/system` call |

`servers.ShutdownService()` stops the tickers and writes every performance history to disk (`SaveAllMetrics()`).

---

## Start queue

```go
var queue *list.List   // *Server values waiting to start
```

- `StartViaService(p)` appends a server. It is used for **autostart** servers when the daemon starts (`run.go`) and for **automatic restarts** after an exit (`afterExit`). Start requests from the panel call `Server.Start()` directly and do not go through the queue.
- `processQueue()` takes one server per second and calls `Start()` if it is not already running; errors are logged.
- There is no concurrency limit or priority: it is a plain FIFO drained at one start per second.

---

## Exit handling and crash reports

When the process exits, the environment calls `Server.afterExit(exitCode)`:

1. Stops the keep-alive sender.
2. Classifies the exit: **graceful** when `exitCode == run.expectedExitCode`; **requested** when someone pressed stop/kill (`stopRequested`); **crashed** otherwise.
3. The crash counter resets when the exit was not a crash or the server had run for more than 10 minutes (`crashCounterResetAfter`).
4. Automatic restart after a crash: only if `run.autorecover` is true and the counter is below `daemon.data.crashLimit` (default `3`).
5. On a crash: console line `Server crashed (exit code N)`, a crash report is saved, the owner's notifications fire, and an alert goes to the admin Discord webhook.
6. Runs the `run.post` operations with `success` and `exitCode` in the data map.
7. Queues a restart (`StartViaService`) after a crash with auto-restart, or after a graceful, unrequested exit when `run.autorestart` is true.

Crash reports (`internal/servers/crash.go`) are stored in `<daemon.data.servers>/<id>.crashes`, newest first, at most 10 per server. Each keeps the last 60 non-empty console lines (each cut at 500 characters), exit code, whether it restarted, and attempt/limit. A `crash` message (`{id, exitCode, restarted}`) is also sent on the server's status WebSocket. How owners see them: [Crash reports](../features/crash-reports.md).

---

## Live statistics

### `ServerStats` (`pkg/skypanel/httpmodels.go`)

```go
type ServerStats struct {
    CPU        float64         `json:"cpu"`        // percent of ONE core (200 = two cores busy)
    MaxCPU     float64         `json:"maxCpu"`     // what the server may use, same unit
    Memory     float64         `json:"memory"`     // bytes
    MaxMemory  float64         `json:"maxMemory"`  // bytes
    Disk       float64         `json:"storage"`    // bytes used by the server folder
    MaxStorage float64         `json:"maxStorage"` // bytes, 0 = no limit
    NetworkRx  float64         `json:"networkRx"`  // KiB/s
    NetworkTx  float64         `json:"networkTx"`  // KiB/s
    Jvm        *utils.JvmStats `json:"jvm,omitempty"`
    Running    bool            `json:"running"`
}
```

**CPU share.** `stats.CPUPercent()` turns `CPU` into a 0–100 share of what the server may use: `CPU / MaxCPU * 100`, clamped. `MaxCPU` is the server's `cpu` variable (percent of one core, so `200` = two cores) or, without a limit, every core of the node × 100. Charts, cards, alerts and the history all use this one number, so a two-core server at full load reads 100%. When `MaxCPU` is 0 the raw `CPU` is returned.

### How each environment measures

| | Docker (`internal/servers/docker/docker.go`) | Host: `tty`, `host`, `standard` (`internal/servers/tty/tty.go`) |
|---|---|---|
| Source | `ContainerStats` with the previous sample | gopsutil on the main process |
| CPU | Docker's formula: `cpuDelta / systemDelta × onlineCPUs × 100`; 0 when the counters go backwards (first sample of a new container) | `process.Percent(1s)` |
| Memory | Usage **minus inactive page cache** (`inactive_file`, cgroup v2, or `total_inactive_file`, v1), as `docker stats` shows it | Process RSS |
| MaxMemory | Container memory limit | Total RAM of the node |
| Network | Sum of the container's interfaces | Node-wide counters (`psnet.IOCounters`), not just the server |
| Disk | Server folder size, refreshed at most every 30 s (the server's own disk measure when available) | Same |
| Cache | One `ContainerStats` call answers every caller for **4 s** (`statsCacheFor`); the time is taken before the call | One reading every **5 s** |
| JVM | `jcmd` inside the container when the template's `stats.type` is `jcmd` | `jcmd` on the process |

Network rates are KiB/s since the previous sample. A counter that went backwards (a new container, an interface reset) starts a new count instead of producing a huge wrapped rate. A stopped server reports zeros (`Running: false`) and resets the network baseline.

### The 5-second stats pass

`SendStatsForServers()` runs for every server in parallel:

1. `GetStats()` (from the cache above when fresh).
2. Writes a `stat` message to the server's stats tracker (the WebSocket).
3. `recordMetrics()` adds the sample to the performance history.
4. `checkServerAlerts()` (in its own goroutine): power changes and high resource use.
5. `checkDiskLimit()` (in its own goroutine): stops a running server whose folder exceeds `MaxStorage` + 1 MB, see [Servers: disk limit](./04-servers.md#disk-limit).

The same numbers are served on demand by `GET /api/servers/:id/stats` (scope `server.stats`), proxied to the daemon's `GET /daemon/server/:id/stats`.

---

## Alerts

`checkServerAlerts()` keeps one state per server in memory (`serverStateTracking`). The first sample after the daemon starts only records the state.

| Alert | Condition | Repeat |
|---|---|---|
| Server online | Stopped → running | On each change |
| Server offline | Running → stopped **without** a requested stop, a clean exit or a reported crash, still unexplained after 15 s (`offlineAlertDelay`) | On each change |
| High CPU | `CPUPercent() > 80` | At most every 5 min while it stays high |
| High memory | `Memory / MaxMemory × 100 > 90` (if `MaxMemory` is 0, `Memory` itself when it is ≤ 100) | At most every 5 min while it stays high |

Where they go:
- The admin channel: `panel.notifications.discordWebhook` through `DiscordService` ([Services](./11-services.md#21-discordservice-discordgo)).
- The owner's channels (Discord, Telegram, JSON webhook) when they enabled `power` or `resources` ([Server notifications](../features/server-notifications.md)). Power and resource events are also published to subscribed extensions.

The thresholds are constants in code; there is no setting for them.

---

## Uptime tracking

`trackUptimeForAllServers()` calls `services.Uptime.TrackStatus(id, isRunning)` for every server. It only runs where the panel runs (`panel.enable`): a daemon-only node has no panel database. An atomic flag stops two rounds from overlapping on a slow database. Storage and statistics: [Services: Uptime](./11-services.md#16-uptime-uptimego). Endpoints: `GET /api/uptime` (scope `admin` or `uptime.view`) and `GET /api/uptime/:serverId` (scope `server.view`).

`trackUptimeForAllServers()` only sees the servers the panel's own process runs. Servers on **remote nodes** are recorded by `processRemoteStatus()` (`internal/servers/remote_status.go`), also only where the panel runs:

- Every 30 seconds it asks every remote node, all at once, for the state of all its servers in one request: `GET /daemon/servers/status` (panel JWT), which answers `{"servers": {"<id>": {"running": bool, "installing": bool}}}` for every server loaded on the node and leaves out a server whose state cannot be read. Each request has a 10 s timeout, and a new round never starts while one is running.
- The answers go to `services.Uptime.RecordNodeStatuses` for the servers the panel places on that node; an installing server counts as not running.
- A node that does not answer records nothing: its servers' state is unknown, not stopped. An open period not confirmed for more than 3 minutes is closed at its last confirmation, so the outage counts as neither up nor down; a new period starts when the node answers again. The panel log says once when a node stops answering the poll and once when it answers again.
- A node on an older version answers `404` and is treated as unreachable: update it to have its servers' uptime recorded.
- The last answer of each node is kept for the Discord node report ([below](#discord-status-reports)).

---

## Performance history (`internal/servers/metrics.go`)

Every stats sample and every player count is folded into two series per server (the charts built on it: [Server statistics](../features/server-statistics.md)):

| Series | Bucket | Kept |
|---|---|---|
| `minutes` | 1 minute | 24 h (1,440 points) |
| `quarter` | 15 minutes | 7 days (672 points) |

### Points

```go
type MetricPoint struct {
    Time        int64   `json:"t"`          // bucket start, Unix seconds
    CPU         float64 `json:"cpu"`        // CPUPercent(), 0-100
    Memory      float64 `json:"memory"`     // bytes
    MaxMemory   float64 `json:"maxMemory"`
    NetRx       float64 `json:"rx"`         // KiB/s
    NetTx       float64 `json:"tx"`
    Online      float64 `json:"online"`     // share of samples taken while running, 0-1
    NoData      bool    `json:"noData,omitempty"`
    Players     float64 `json:"players"`    // average online
    PlayersPeak int     `json:"playersPeak"`
    MaxPlayers  int     `json:"maxPlayers"`
    HasPlayers  bool    `json:"hasPlayers"` // false when no query answered in the bucket
}
```

- CPU, memory and network are averaged over the samples taken **while the server ran**, so an hour spent half stopped does not halve them. `Online` is running samples / all samples.
- A bucket with only player counts counts as `Online = 1` (a game answering queries is running).
- Players: `samplePlayers()` asks each running server once a minute (`Server.QueryGame()`: Minecraft list ping or Steam A2S, answers cached 10 s). Steam bots are not counted as players. The all-time peak is kept as `recordPlayers` / `recordAt`.

### Storage

- File `<daemon.data.servers>/<id>.metrics` (JSON, mode 0600), read the first time the server's history is needed.
- Written at most every 5 minutes while samples arrive, and for every server on shutdown. A crash of the daemon can lose up to 5 minutes.
- Deleted with the server (with `.cron`, `.crashes`, `.notify` and `.worldmap`, `server_loader.go`).

### Ranges and resampling

`GET /api/servers/:id/metrics?range=1h|24h|7d` (scope `server.stats`, default `24h`), proxied to the daemon:

| `range` | Window | Step | Source series |
|---|---|---|---|
| `1h` | 1 hour | 1 min | `minutes` |
| `24h` | 24 hours | 5 min | `minutes` |
| `7d` | 7 days | 1 hour | `quarter` |

`Server.Metrics(window, step, now)`:
1. Takes the source series plus the bucket still being filled.
2. `resample()` averages points into steps, weighting CPU, memory and network by each point's `Online` share (the same rule as a bucket). Points before the window and `NoData` points are skipped.
3. `fillGaps()` inserts `{"t": ..., "noData": true}` for every step between the first point and now that has nothing: the node was down, so the server was too. The step still running is never marked. Charts leave those steps blank and count them as offline instead of drawing a line across them.

Response:

```json
{
  "range": "24h",
  "step": 300,
  "points": [ { "t": 1761984000, "cpu": 12.5, "memory": 1610612736, "maxMemory": 4294967296,
                "rx": 3.2, "tx": 8.9, "online": 1, "players": 2.4, "playersPeak": 4,
                "maxPlayers": 20, "hasPlayers": true } ],
  "record": { "players": 17, "at": 1761400000 }
}
```

An unknown range answers 400 `range must be 1h, 24h or 7d`.

---

## Node metrics

### System information

`GET /api/nodes/:id/system` (scope `nodes.view`) → `GET /daemon/system` (JWT only, `internal/web/daemon/root.go`). Returns hostname, OS, platform and version, architecture, uptime, CPU model / physical cores / threads, `cpuUsage`, memory total/used/free/available, the list of disks, the disk holding the servers (`serversDisk`) and total network bytes sent/received.

- **CPU sampler**: a background goroutine measures node CPU over 2-second windows (`cpu.Percent(2s)`) and keeps the latest value, so `/system` answers at once. It starts on the first call; that first call returns a 250 ms reading. Before this, every call blocked for a second, and the dashboard, node pages and server wizard poll it every few seconds.
- **Disks**: every partition whose mount point is a directory, deduplicated by device + size + usage (inside a container every bind mount looks like a partition of the same disk). The label preferred is `/`, then the mount holding the servers folder, then the shortest path.

### Network history (`internal/web/daemon/network_history.go`)

Each node keeps its own traffic history, so the dashboard no longer depends on how long a browser tab stayed open. Dashboard view: [Dashboard and nodes](../features/dashboard-and-nodes.md).

| Item | Value |
|---|---|
| Sampling | Every 10 s, total bytes of the **physical** interfaces |
| Resolution | One point per minute: KB/s received/sent, averaged over the seconds actually measured |
| Kept | 24 hours |
| File | `network.history` in the parent folder of `daemon.data.servers`, saved every 5 min (written to `.tmp`, then renamed) |
| Gaps | A gap longer than 5 min between two readings (host suspended) is not spread over the minute; counters that go backwards only start a new reading |

**Physical interfaces only.** Loopback, VLAN sub-interfaces (`eth0.10`, names with `@`) and virtual prefixes (`docker`, `br-`, `veth`, `virbr`, `vnet`, `cni`, `flannel`, `cali`, `vxlan`, `kube`, `lxc`, `podman`, `tun`, `tap`, `wg`, `tailscale`, `zt`, `dummy`, `bond`, `ifb`) are excluded, so container traffic is not counted two or three times.

**`daemon.hostProc`.** In a container the daemon's own `/proc/net/dev` only shows the container's traffic. When `daemon.hostProc` is set (env `SKYPANEL_DAEMON_HOSTPROC`), the counters are read from `<hostProc>/1/net/dev`, i.e. the host's. `docker-compose.yml` mounts `/proc:/host/proc:ro` and sets `SKYPANEL_DAEMON_HOSTPROC=/host/proc`. Without the file, gopsutil's counters are used. The same reading feeds the `networkBytesSent/Recv` of `/daemon/system`.

**Endpoints:**

| Endpoint | Auth | Description |
|---|---|---|
| `GET /daemon/network/history?range=1h\|6h\|24h` | Panel JWT | This node's history |
| `GET /api/nodes/network-history?range=1h\|6h\|24h` | Scope `nodes.view` | Every node's history added up step by step |

| `range` | Step |
|---|---|
| `1h` (default) | 1 min |
| `6h` | 5 min |
| `24h` | 15 min |

A step the node did not measure has no point. The panel endpoint asks all nodes in parallel (10 s timeout each) and returns:

```json
{ "step": 60, "points": [ { "t": 1761984000, "rx": 120.5, "tx": 88.1 } ], "nodes": 2, "unavailable": 0 }
```

`unavailable` counts nodes that did not answer (down, or an older daemon without the endpoint). An unknown range answers 400 `range must be 1h, 6h or 24h`.

---

## Discord status reports

`sendSystemStatusToDiscord()` runs **every minute**, only where the panel runs, and only does something when webhooks are configured:

- `panel.notifications.discordWebhookSystem`: one embed with online/offline counts, averages and a line per server. It covers the servers the panel's own process runs; with none, nothing is sent.
- `panel.notifications.discordWebhookNode`: one embed per node with its server counts, host, CPU model/cores/threads/GHz, RAM and OS.

For the panel's own node, the counts are its servers and the CPU, RAM and OS figures are read on the panel's machine (`hostCPUSummary`, `mem.VirtualMemory`, `getOSName`). For a remote node, the counts come from the status poller's last answer (or a direct call to `/daemon/servers/status` when that answer is older than 60 seconds, which is not recorded as uptime), and the hardware from the node's own `GET /daemon/system` (10 s timeout; the daemon does not send the CPU frequency, so that field is left out). A remote node that does not answer gets a red "no answer" report with the number of servers placed on it.

The texts follow `panel.settings.notificationLanguage` ([Services: notification texts](./11-services.md#notification-texts-notification_textgo)).

---

## Endpoint summary

| Endpoint | Scope | Data |
|---|---|---|
| `GET /api/servers/:id/stats` | `server.stats` | Current `ServerStats` |
| `GET /api/servers/:id/metrics?range=` | `server.stats` | Performance history |
| `GET /api/uptime` | `admin` or `uptime.view` | Uptime of every server |
| `GET /api/uptime/:serverId` | `server.view` | Uptime of one server |
| `GET /api/nodes/:id/system` | `nodes.view` | Node system information |
| `GET /api/nodes/network-history?range=` | `nodes.view` | Network history of all nodes |
| `GET /daemon/network/history?range=` | panel JWT | Network history of one node |
| `GET /daemon/servers/status` | panel JWT | Running/installing state of every server on one node |

---

## Troubleshooting

| Symptom | Cause / check |
|---|---|
| CPU stuck near 100% on a multi-core server | Expected when the server uses all of its `cpu` allowance: CPU is a share of the limit, not of the node |
| Memory looks lower than `free` on the host | Docker memory excludes the inactive page cache, as `docker stats` does |
| Host-environment servers all show the same network rate | Host environments read node-wide counters |
| Gaps in the 24 h or 7 d chart | The node (daemon) was down; those steps are `noData` |
| Node network chart flat at zero inside Docker | `daemon.hostProc` not set or `/proc` not mounted; the container only sees its own traffic |
| `unavailable` > 0 in the network history | A node is down or runs a daemon without `/daemon/network/history` |
| No uptime data on a daemon-only node | Expected: uptime is written in the panel's database only; the panel polls remote nodes for theirs |
| No uptime for the servers of a remote node | The node runs an older daemon without `/daemon/servers/status` (update it), or it does not answer the panel: the panel log shows `Node <name> did not answer the server status poll` |
| Too many Discord status messages | The system/node webhooks post every minute; leave them empty to turn them off |
