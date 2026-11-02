# Daemon

The routes every node serves under `/daemon`. The panel calls them on the node that runs a server: to forward most `/api/servers/:serverId/...` requests, to create, move and delete servers, and to read the node's state. They are **panel-to-node** routes, not meant for users, scripts or billing systems: those use the [panel API](../11-api-reference.md), which checks permissions and then calls the node. Back to the [API reference](../11-api-reference.md).

Code: `internal/web/daemon/*.go` (routes), `internal/middleware/decrypt.go` (token check), `internal/services/node.go` and `internal/services/token.go` (the panel's side).

## Where the node answers

- The daemon routes are registered when `daemon.enable` is on (the default) and are served by the same web server as the rest of the process (`web.host`, default `0.0.0.0:8080`). The panel reaches a node at its `privateHost:privatePort`.
- On a combined install (panel and daemon in one process, the "local" node) the panel runs the request in-process, without going through the network. It still signs a token, and the node checks it the same way.
- To know whether a node speaks HTTPS, the panel sends `OPTIONS /daemon` over HTTPS and then HTTP. A node once seen on HTTPS is pinned to it and never contacted over plain HTTP again.
- When the panel forwards a browser request it removes the `Cookie` header, so a user's session never reaches a node.

## Authentication

| Routes | Auth |
|--------|------|
| `GET`, `HEAD`, `OPTIONS /daemon`, `GET /daemon/features` | None |
| Everything else | `Authorization: Bearer <token>` signed by the panel |

The token is a JWT the panel signs with its Ed25519 key (`EdDSA`, key id `SkyPanel`) for each call. It expires after 15 minutes (one minute of clock leeway) and its audience is derived from the secret of the node it is sent to, so a token captured on its way to one node does not work on another. A node with only the daemon (`panel.enable=false`) reads the panel's public keys from the JWKS at `token.public` (the panel's [`GET /auth/publickey`](./02-authentication.md#get-authpublickey)). Details: [Security → Panel and node tokens](../../backend/09-security.md#panel-and-node-tokens).

The token carries **no user and no scopes**: any valid panel token may call any route on that node. Permissions are checked on the panel route before the panel calls the node (see [Permissions (scopes)](./02-authentication.md#permissions-scopes)).

| Answer | When |
|--------|------|
| `401 {"error": "invalid_request"}` with `WWW-Authenticate: Bearer` | No `Authorization` header, or it is not `Bearer <token>` |
| `400` with an error body | Bad signature, expired token, missing expiry or wrong audience |

## Routes

### Node

| Method | Path | Auth | What it does | Called for |
|--------|------|------|--------------|------------|
| GET | `/daemon` | None | `{"message": "daemon is running"}` | Liveness checks |
| HEAD | `/daemon` | None | `204` | Liveness checks |
| OPTIONS | `/daemon` | None | Allowed methods | The panel's HTTPS detection |
| GET | `/daemon/features` | None | OS, architecture, version, environments, Docker | [`GET /api/nodes/:id/features`](./06-panel-admin.md#get-apinodesidfeatures) |
| GET | `/daemon/system` | Panel token | Host, CPU, memory, disks, network counters | [`GET /api/nodes/:id/system`](./06-panel-admin.md#get-apinodesidsystem) |
| GET | `/daemon/ports` | Panel token | Host ports published by the node's containers | [`POST /api/nodes/:id/ports/check`](./06-panel-admin.md#post-apinodesidportscheck) |
| GET | `/daemon/network/history` | Panel token | The node's traffic over the last hour, 6 hours or day | [`GET /api/nodes/network-history`](./06-panel-admin.md#get-apinodesnetwork-historyrange1h) |
| POST | `/daemon/update` | Panel token | Starts an update of this node | `POST /api/settings/update-panel` |
| GET | `/daemon/update-status` | Panel token | State of this node's last update | `GET /api/settings/update-status` |

### Servers

Every route below needs the panel token. `:serverId` must be a server the node has (`404` with no body otherwise), except for `PUT /daemon/server/:serverId`, which creates it.

Unless the last column says otherwise, the panel route is the same path under `/api/servers/:serverId` (for example `POST /daemon/server/:serverId/start` for `POST /api/servers/:serverId/start`), described in [Servers](./03-servers.md) and [Server content](./04-server-content.md). For those the panel passes the query string and body through and returns the node's answer unchanged.

| Method | Path (after `/daemon/server/:serverId`) | What it does | Panel route |
|--------|------------------------------------------|--------------|-------------|
| PUT | *(none)* | Creates the server from a definition | `PUT /api/servers/:serverId`, node transfer, clone |
| DELETE | *(none)* | Deletes the server, its files and its backups (`?keepBackups=true` keeps the backups) | `DELETE /api/servers/:serverId`, node transfer |
| GET | `/definition` | Full definition | Same; also node transfer and clone |
| PUT | `/definition` | Replaces the definition | Same (the panel filters what non-administrators send) |
| GET | `/data` | User-editable variables | Same |
| POST | `/data` | Changes user-editable variables | Same |
| PUT | `/data` | Changes any variable; `ports` / `primaryPort` become `port`, `port2`, ... | Same, plus port settings and sub-server limits |
| GET | `/tasks` | Scheduled tasks | Same |
| GET, PUT, DELETE | `/tasks/:taskID` | One task | Same |
| POST | `/tasks/:taskID/run` | Runs a task now | Same |
| POST | `/reload` | Reloads the definition from disk | Same |
| POST | `/start`, `/restart` | Starts / restarts | Same (the panel refuses suspended servers first) |
| POST | `/stop`, `/kill` | Stops / kills | Same |
| POST | `/install` | Runs the template's installation | Same; also right after creation |
| GET, PUT, POST, DELETE | `/file/*filename` | Read, write, move, delete files | Same; also both transfers |
| POST | `/archive/*filename` | Compresses files | Same; also both transfers |
| POST | `/extract/*filename` | Extracts an archive | Same; also both transfers |
| GET | `/console` | Console buffer | Same |
| POST | `/console` | Sends a command | Same |
| GET, POST | `/flags` | Auto start / restart flags | Same |
| GET | `/stats` | CPU, memory, disk, network | Same |
| GET | `/status` | Running / installing | Same; also uptime checks |
| HEAD, GET | `/query` | Game query | Same |
| GET | `/crashes` | Crash reports | Same |
| GET | `/metrics` | Performance history (`range` `1h`, `24h` or `7d`) | Same |
| GET, POST | `/version` | Game versions / change version | Same |
| GET, PUT | `/notifications` | Notification channels | Same |
| POST | `/notifications/test` | Test message | Same |
| GET, DELETE | `/plugins` | Installed plugins / remove one (`?name=`) | Same |
| GET | `/plugins/search` | Searches Modrinth (`?q=`) | Same |
| POST | `/plugins/:pluginId` | Installs a plugin (`?loaders=`, `?game_versions=` filter the Modrinth versions) | Same |
| GET | `/modpacks`, `/modpacks/search`, `/modpacks/versions` | Modpack status and search | Same |
| POST | `/modpacks/install` | Installs a modpack | Same |
| GET | `/file-history` | Versions of a file (`?path=`) | Same |
| GET | `/file-history/:versionId` | Text of one version | Same |
| POST | `/file-history/:versionId/restore` | Restores a version | Same |
| GET | `/map` | World map status | `GET /api/servers/:serverId/map` (the panel adds the viewer link) |
| DELETE | `/map` | Removes the map plugin | Same |
| POST | `/map/install` | Installs BlueMap or Dynmap | Same |
| GET, HEAD | `/map/view/*path` | Reverse proxy to the map's web server | `/api/map-view/:serverId/:token/*path` |
| GET | `/backup` | Backup archives on the node | Backup list sync |
| POST | `/backup/create` | Starts a backup | `POST /api/servers/:serverId/backup/create` |
| POST | `/backup/restore?fileName=` | Restores a backup | `POST /api/servers/:serverId/backup/restore/:backupID` |
| GET | `/backup/download?fileName=` | Streams a backup | `GET /api/servers/:serverId/backup/download/:backupID` |
| DELETE | `/backup?fileName=` | Deletes a backup archive | `DELETE /api/servers/:serverId/backup/:backupID` |
| POST | `/clone-from/:sourceId` | Copies another server's files into this one | [`POST /api/servers/:serverId/clone`](./03-servers.md#post-apiserversserveridclone) |
| GET | `/socket` | WebSocket: console, stats, status | [`GET /api/servers/:serverId/socket`](./03-servers.md#websocket) |

## Node routes

### `GET /daemon/features`

```json
{ "features": ["docker"], "environments": ["docker"], "os": "linux", "arch": "amd64", "version": "3.x.y" }
```

`features` holds `docker` when the node can reach Docker within a second. With `docker.disallowHost` on, the host environments (`host`, `tty`, `standard`, `bubblewrap`) are left out of `environments`.

### `GET /daemon/system`

The object the panel returns from [`GET /api/nodes/:id/system`](./06-panel-admin.md#get-apinodesidsystem). The CPU usage is sampled in the background every two seconds, so the call answers at once. `networkBytesSent` and `networkBytesRecv` add up the physical interfaces (loopback, Docker bridges, `veth`, VPN and other virtual interfaces and VLANs are left out); in a container they are the host's when `daemon.hostProc` points to the host's `/proc`.

### `GET /daemon/ports`

```json
{ "ports": { "8080": "PANEL_CONTAINER_NAME", "25565": "SERVER_CONTAINER_NAME" } }
```

Host ports published by the containers running on the node, each with the container's name. Empty when Docker cannot be reached.

### `GET /daemon/network/history?range=1h`

`range` is `1h` (default), `6h` or `24h`; anything else answers `400 {"error": "range must be 1h, 6h or 24h"}`.

```json
{ "step": 60, "points": [ { "t": 1793576400, "rx": 0.49, "tx": 0.37 } ] }
```

The node reads its traffic counters every 10 seconds and keeps one point a minute for 24 hours, in the file `network.history` next to the servers folder (saved every 5 minutes, so it survives restarts). The answer averages those minutes into steps of 1, 5 or 15 minutes. `t` is the step's start in Unix seconds; `rx` and `tx` are KB/s received and sent. Steps the node did not measure (it was down) have no point. The panel adds the nodes up in [`GET /api/nodes/network-history`](./06-panel-admin.md#get-apinodesnetwork-historyrange1h); a node too old to have this route counts as unavailable there.

### `POST /daemon/update`

Starts an update of the machine the node runs on, in a short-lived container (the update image is pulled first when missing). Answer: `{"message": "Update initiated on this node.", "containerId": "..."}`; `500` when Docker cannot be reached or the container cannot start. See [Panel updates](../../features/panel-updates.md).

### `GET /daemon/update-status`

```json
{ "running": false, "containerId": "...", "startedAt": "2026-11-02T10:00:00Z", "finishedAt": "2026-11-02T10:03:12Z", "exitCode": 0, "log": "...", "error": "" }
```

The state of the last update started on this node: whether it is running, its exit code and the end of its log.

## Server routes

Only the differences from the panel routes they back are described here.

### `PUT /daemon/server/:serverId`

Body: a server definition. If the node already has a server with that identifier, it is deleted first (with its files). The environment must be allowed (`400` for a host environment with `docker.disallowHost`) and the template's requirements met (`400`). Answer: `{"id": "<serverId>"}`. It does not install the server; the panel calls `/install` afterwards when `panel.servers.autoInstall` is on.

### `DELETE /daemon/server/:serverId`

Kills the server if it runs, runs the template's uninstall steps, deletes the server folder and everything the node keeps for it (definition, tasks, crash reports, notification settings, metrics, world map and modpack state, file history, its `/tmp` folder). A failed uninstall step is logged and shown on the console, and the deletion goes on. Then the server's backup folder on this node (`<daemon.data.backups.folder>/<serverId>`) is deleted, unless the request has `?keepBackups=true`, which the panel sends when it moves the server to another node; copies in off-site storage are not touched. `204`.

### Power actions

`start`, `restart`, `stop` and `install` answer `202` and work in the background; with `?wait` they answer `204` once done. `start` and `restart` answer `409` straight away when the server's disk is full or one of its ports is already bound by another running container. `kill` answers `204`.

### Files

The same rules as the [panel's file routes](./04-server-content.md#files), including the `?folder` flag and the query string being read as part of the file name on `GET`, `PUT` and `DELETE`. Both [transfers](./08-external-transfer.md#what-moves) use these routes with the archive `transfer.tar.gz` and `extract/transfer.tar.gz?destination=.`.

### Backups

The node knows backups by file name, not by the panel's record ID: `GET /backup` lists the finished archives (newest first), then the backups made by hand whose archive could not be written, with `"location": "failed"` (the panel drops their records when it sees them and calls `DELETE` to clear the mark). `restore`, `download` and `DELETE` take `?fileName=`. `POST /backup/create` answers `{"backupFileName": "..."}` and writes the archive in the background. `restore` answers `400 ErrBackupServerRunning` while the server runs, otherwise `202`. See [Server content → Backups](./04-server-content.md#backups).

### `POST /daemon/server/:serverId/clone-from/:sourceId`

Copies the files of server `:sourceId` (on the same node) into `:serverId` in the background; the target counts as installing until the copy ends. Optional body `{"sourceName": "Survival"}` (the name shown on the console, cut to 80 characters). `202`; `404` when the source is not on the node; `400` when both are the same server; `409` when either is running, the target is not empty or a backup is in progress.

### `GET /daemon/server/:serverId/map/view/*path`

Reverse proxy to the world map's own web server (BlueMap or Dynmap). Only a few request headers are passed on, and `Set-Cookie` is removed from the answer. `400` for a path with `..`; `502` when the map's web server does not answer. The browser reaches it only through the panel's signed link, see [Server content → World map](./04-server-content.md#world-map).

### `GET /daemon/server/:serverId/socket`

WebSocket upgrade. The query flags `console`, `stats` and `status` choose what the socket receives. The panel replaces whatever the browser asked for with the streams the caller's scopes allow (`server.console`, `server.stats`, `server.status`, resolved as on every other route) before it opens the socket to the node with its own token.
