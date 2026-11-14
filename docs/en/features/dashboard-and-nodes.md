# Dashboard and nodes

The dashboard (sidebar **Home**) is the first page after signing in. Operators get an overview of the whole installation; regular users get the list of their servers. The **Nodes** pages show each machine that runs game servers, its live resources and disks, and warn before a node runs out of disk. This page also covers the background services behind those figures: the node's network history, the CPU sampler, port checks and image prefetching.

## What it does

**Dashboard, operator view** (users with `nodes.view` or `users.info.search`):

- **Status band**: a welcome line, **Active servers** ("N of M"), **Running time (30 days)** (average share of the last 30 days the servers were running) and **Network traffic** (live rate of all nodes).
- **Low-disk alerts** for any node whose servers' disk is nearly full (only with `nodes.view`).
- **Resource indicators**: **Average CPU**, **Average memory** and **Average storage**, as percentages.
- **Panel information**: version, number of servers, users and nodes, and whether **Sign-ups** are open or closed.
- **Network history**: download and upload of every node added up, over 1 h, 6 h or 24 h.
- **Access records**: latest sign-ins and account creations (see [Activity and access log](activity-and-access-log.md)).

**Dashboard, user view** (everyone else): "My Servers", one card per server the user can open.

**Nodes** (sidebar **Nodes**, scope `nodes.view`):

- List of nodes with location and server count; actions **View Details**, **Edit**, **Deploy**, **Delete**.
- Node detail: total servers (online/offline), **Memory Usage**, **CPU Usage** (cores/threads), uptime, system information (hostname, platform, OS, architecture, CPU model, memory), **Disks** and **Servers on this Node**.
- **Deploy Node** dialog: the one-step command that installs the node on another machine, or the steps by hand.

## How it works

### Where the dashboard figures come from

| Figure | Source |
|--------|--------|
| Active servers | Server list (`/api/servers`) with status from `/api/uptime` |
| Running time (30 days) | Average of each server's `uptimePercent` from `/api/uptime` (needs `admin` or `uptime.view`; shows nothing otherwise). A server its owner stopped counts as not running, so a low figure does not mean a fault |
| Average CPU / memory / storage | Average across nodes of `GET /api/nodes/:id/system` (CPU usage, used/total memory, used percent of the servers' disk). If no node reports, the average of the online servers is used instead. The hint under each value says which ("average of N nodes" / "average of N servers"); with a single node it names it ("node eu-1", or "local node (this machine)") |
| Network traffic (live) | Difference between two readings of each node's cumulative byte counters (`networkBytesRecv`/`networkBytesSent`), summed across nodes. Each node stamps its reading (`sampledAt`), so the rate divides by the time that passed on the node, not by when each answer arrived. The first reading after opening the page only sets the baseline |
| Network history | `GET /api/nodes/network-history?range=1h\|6h\|24h` |
| Panel information | `/api/config` (version, registration) and counts the user may see (users with `users.info.search`, nodes with `nodes.view`) |

The dashboard polls the nodes every 5 seconds and the network history every 60 seconds; neither polls while the browser tab is hidden.

### Node system information and the CPU sampler

`GET /api/nodes/:id/system` proxies `GET /daemon/system` on the node. The daemon reports host details, memory (including `memoryAvailable`, free plus caches), every disk once (bind mounts of the same disk inside a container are merged and named after `/` or the servers' folder), the **servers' disk** (`serversDisk`) and cumulative network counters.

CPU usage is not measured inside the request: a background sampler reads it over 2-second windows and `/system` answers at once with the latest value. The very first call after the daemon starts does a short 250 ms reading.

The node detail page polls `/system` every 3 seconds while visible.

### Network history

Each daemon samples its network counters every 10 seconds, adds them up per minute, keeps 24 hours of one-minute points and saves them every 5 minutes to `network.history` in its data folder (`daemon.data.root`), so history survives restarts. Only physical interfaces are counted: loopback, VLAN sub-interfaces and virtual interfaces (`docker*`, `br-*`, `veth*`, `virbr*`, `cni*`, `flannel*`, `cali*`, `tun*`, `tap*`, `wg*`, `tailscale*`, `bond*` and similar) are skipped to avoid counting the same traffic twice. A gap of more than 5 minutes between readings (host suspended) is not spread over the minute.

When the daemon runs in a container its own `/proc` only shows the container's traffic. With `daemon.hostProc` set (docker-compose mounts the host `/proc` read-only at `/host/proc` and sets `SKYPANEL_DAEMON_HOSTPROC=/host/proc`) the daemon reads the host's counters from `<hostProc>/1/net/dev`.

`GET /daemon/network/history?range=` returns the points averaged to the range's step: 1 minute for `1h`, 5 minutes for `6h`, 15 minutes for `24h`. The panel's `GET /api/nodes/network-history` asks every node in parallel (10-second timeout each), adds the points step by step and reports `nodes` (asked) and `unavailable` (did not answer, for example an older node or one that is down). A step with no data from any node is drawn as a gap, not a line.

### Low-disk alert

The daemon marks `serversDisk.low = true` when the disk holding the servers' folder has **less than 5 GB or less than 10 %** free. The panel then shows "The node "X" is running out of disk" on the dashboard, on the node list and on that node's page.

Separately, `daemon.minFreeDiskMB` (default 2048) is enforced on installs: if the servers' disk has less free space than that when an install starts, the install is refused with `ErrNodeDiskFull` and the server console shows "The node only has X GB free, and an install needs at least Y GB. Free some space on the node and install again." See [Servers](../backend/04-servers.md).

### Port checks in the new-server wizard

While a user types ports in the new-server wizard, it calls `POST /api/nodes/:id/ports/check` 400 ms after the last change. For each requested port the panel answers whether it is busy and why:

| Reason | Meaning |
|--------|---------|
| `range` | Below 1024 |
| `panel` | The node's own public, private or SFTP port, or a container whose name starts with `skypanel` (panel, database, Adminer) |
| `server` | Main or extra port of another server on that node (from the panel database or a published container port) |
| `container` | Any other container on the node publishing that host port (from `GET /daemon/ports`) |

A busy port comes with a `suggestion`: the next port after it (and at least 1024) that is free and not requested elsewhere in the same form; the wizard offers it as "Use N". Who holds the port (`by`) is only returned to administrators (`admin` scope). A node that cannot answer `/daemon/ports` (no Docker, older version) leaves the check to the panel's records. The request may carry `exclude` with a server identifier whose own ports are ignored.

### Server status on remote nodes

A remote node's daemon runs without the panel, so the panel asks it for the state of its servers: every 30 seconds it calls `GET /daemon/servers/status` on every remote node at once (one request per node, 10-second timeout; a round never overlaps the previous one). Only the process that runs the panel does this. The answers are recorded as uptime in the same way as the panel's own servers (see [Uptime tracking](../general/13-uptime.md)), so **Running time (30 days)** and the uptime figures cover servers on remote nodes too. A node that does not answer records nothing; a period that has not been confirmed for more than 3 minutes is closed at its last confirmation, so an outage of the node counts as neither up nor down, and a new period starts when the node answers again. A node running an older daemon without that route (it answers `404`) is treated as not answering.

The same answers feed the node report sent to the Discord node webhook (`panel.notifications.discordWebhookNode`): a remote node shows its real number of servers online and offline (from the last poll, or from a direct call when that poll is more than 60 seconds old), and a node that does not answer gets a red report with the status **No answer** and the number of servers the panel places there. See [Discord webhooks](../general/16-discord.md).

### Image prefetch

On Docker nodes the daemon downloads the images in `docker.prefetchImages` 2 minutes after start and then once a day (30-minute limit per run). If a pull replaced an older copy, the old image is removed when no container uses it. This avoids a long first start for a server using one of those images.

## Using it

### In the panel

- **Home**: open the dashboard; choose **1 h**, **6 h** or **24 h** on **Network history**. A low-disk alert links to the node.
- **Nodes → Add Node**: enter name, public host, public port and SFTP port; tick the option for a different private host/port if the panel reaches the node through another address.
- **Nodes → node → Deploy Node**: **Generate command** gives a line to paste as `root` on the new machine, which installs it as the node and connects it (the code works once and for an hour; see [Installation → secondary node](../setup/01-installation.md#method-3-secondary-node-daemon-only)). **Show the manual setup** shows the `docker-compose.yml` with the node's real credentials. Needs `nodes.deploy` because it reveals the node's secret.
- **Edit Node / Delete Node** on the node page. The local node cannot be edited ("Cannot be edited"); its displayed host comes from the panel's master URL in Settings.

### Via the API

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| `GET` | `/api/nodes` | `nodes.view` | List nodes |
| `POST` | `/api/nodes` | `nodes.create` | Create a node |
| `GET` / `PUT` / `DELETE` | `/api/nodes/:id` | `nodes.view` / `nodes.edit` / `nodes.delete` | Read, edit, delete a node |
| `GET` | `/api/nodes/:id/features` | `nodes.view` | Environments, Docker support, OS, architecture, version |
| `GET` | `/api/nodes/:id/system` | `nodes.view` | Live system information |
| `GET` | `/api/nodes/network-history?range=1h\|6h\|24h` | `nodes.view` | Traffic of all nodes added up |
| `POST` | `/api/nodes/:id/ports/check` | `server.create` | Which ports are taken, why, and a free alternative |
| `GET` | `/api/nodes/:id/deployment` | `nodes.deploy` | Client ID and secret for a remote daemon |
| `POST` | `/api/nodes/:id/join` | `nodes.deploy` | One-time join code (one hour) and the command that installs the node |
| `POST` | `/api/node-join` | Public (the code) | The installer trades the code for the node's credentials and ports |
| `GET` | `/daemon/system`, `/daemon/ports`, `/daemon/network/history` | panel JWT | Node-side sources used by the panel |

Example port check:

```json
POST /api/nodes/3/ports/check
{ "ports": { "SERVER_PORT": 25565, "extra:0": 8080 }, "exclude": "" }

{ "ports": { "SERVER_PORT": { "busy": true, "reason": "server", "by": "Survival", "suggestion": 25566 },
             "extra:0": { "busy": false } } }
```

Full details are in the [API reference](../reference/11-api-reference.md).

## Requirements and configuration

| Key | Default | Environment variable | Effect |
|-----|---------|----------------------|--------|
| `daemon.minFreeDiskMB` | `2048` | `SKYPANEL_DAEMON_MINFREEDISKMB` | Free MB the servers' disk must keep for an install to start; `0` disables the check |
| `daemon.hostProc` | `""` | `SKYPANEL_DAEMON_HOSTPROC` | Where the host `/proc` is mounted, to measure the node's real traffic from a container (`/host/proc` in docker-compose) |
| `docker.prefetchImages` | `ghcr.io/aether-panel/steamcmd:latest`, `ghcr.io/aether-panel/ubuntu:latest`, `ghcr.io/aether-panel/generic:latest` | `SKYPANEL_DOCKER_PREFETCHIMAGES` (space-separated) | Images kept downloaded and current on Docker nodes; an empty list turns it off |

The low-disk thresholds (5 GB / 10 %), the sampling intervals and the 24-hour history length are fixed in the code. See [Configuration](../backend/02-configuration.md) for every key.

## Limits

- The live network rate needs two polls (about 5 seconds) before it shows a value.
- Network history starts empty on a new node and fills over the following minutes; it covers at most 24 hours.
- A node running an older daemon without `/daemon/network/history` counts as "not answering" in the history footer.
- Without `daemon.hostProc` in a containerised daemon, the traffic shown is only the panel container's own, not the game servers'.
- Port checks are hints: they do not reserve ports.
- Servers on a node running an older daemon (without `/daemon/servers/status`) get no uptime data until the node is updated.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Network history shows "No history yet" | The node only just started recording | Wait a few minutes |
| History footer says "N not answering" | A node is offline or runs an older version | Check the node; update it ([Panel updates](panel-updates.md)) |
| "Could not load the history" | The panel did not answer `/api/nodes/network-history` | It retries every minute; check the panel log |
| Traffic near zero while servers are busy | Containerised daemon without `daemon.hostProc` | Mount `/proc:/host/proc:ro` and set `SKYPANEL_DAEMON_HOSTPROC=/host/proc` |
| Running time (30 days) shows no value | The user lacks `admin`/`uptime.view`, or no uptime data yet | Grant `uptime.view` or wait for data |
| Node page: "This node can't be reached…" | The daemon is down or unreachable from the panel | Start the daemon; check host, ports and firewall |
| "The node refused the panel's credentials…" (`502`, `ErrNodeAuthFailed`) on a server of a remote node | The node refused the panel's token: its `daemon.auth.clientSecret` is not the node's secret in the panel, or one of the two clocks is wrong. The panel log names the node | Copy the secret again from **Deploy Node** into the node's configuration and restart it; synchronise both clocks (NTP). See [Troubleshooting](../setup/troubleshooting.md) |
| Discord node report: **No answer** | The node did not answer the status poll (down, unreachable, or an older daemon) | Check the node; update it if it is old |
| "The node "X" is running out of disk" | Less than 5 GB or 10 % free on the servers' disk | Free space or grow the disk |
| Install fails with `ErrNodeDiskFull` | Free space below `daemon.minFreeDiskMB` | Free space, or lower the key (`0` disables it) |
| Daemon log: `Could not download the runtime image …` | Registry unreachable during prefetch | Check network access to `ghcr.io`; the next daily run retries |
| Daemon log: `Could not save the node's network history` | Data folder not writable | Check permissions of `daemon.data.root` |

## See also

- [Core concepts: multi-node deployment](../general/concepts.md)
- [Servers: disk limit and environments](../backend/04-servers.md)
- [Activity and access log](activity-and-access-log.md)
- [Uptime tracking](../general/13-uptime.md)
- [Panel updates](panel-updates.md)
- [Configuration](../backend/02-configuration.md)
- [API reference](../reference/11-api-reference.md)
