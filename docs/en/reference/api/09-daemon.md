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
| `401 {"error": "invalid_token"}` with `WWW-Authenticate: Bearer error="invalid_token"` | Bad signature, expired token, missing expiry or wrong audience (the node logs the reason) |

A token that does not check out usually means the node's `daemon.auth.clientSecret` is not the secret the panel has for that node, or one of the two clocks is off. The panel does not pass such a `401` (or a `403` with `WWW-Authenticate`, from a proxy in front of the node) on to the browser: it answers `502 ErrNodeAuthFailed` and logs the node's name (see [Conventions → Node authentication failures](./01-conventions.md#node-authentication-failures)). Nodes before this version answered `400` here.

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
| GET | `/daemon/servers/status` | Panel token | Running / installing state of every server on the node; the request also carries the panel's notification language | The panel's uptime poller and Discord node report |
| POST | `/daemon/update` | Panel token | Starts an update of this node | `POST /api/settings/update-panel` |
| GET | `/daemon/update-status` | Panel token | State of this node's last update | `GET /api/settings/update-status` |

### Servers

Every route below needs the panel token. `:serverId` must be a server the node has (`404` with no body otherwise), except for `PUT /daemon/server/:serverId`, which creates it.

Unless the last column says otherwise, the panel route is the same path under `/api/servers/:serverId` (for example `POST /daemon/server/:serverId/start` for `POST /api/servers/:serverId/start`), described in [Servers](./03-servers.md) and [Server content](./04-server-content.md). For those the panel passes the query string and body through and returns the node's answer unchanged.

| Method | Path (after `/daemon/server/:serverId`) | What it does | Panel route |
|--------|------------------------------------------|--------------|-------------|
| PUT | *(none)* | Creates the server from a definition | `PUT /api/servers/:serverId`, node transfer, clone |
| DELETE | *(none)* | Deletes the server, its files and its backups (`?keepBackups=true` keeps the backups) | `DELETE /api/servers/:serverId`, node transfer (and its undo) |
| GET | `/definition` | Full definition | Same; also node transfer and clone |
| PUT | `/definition` | Replaces the definition | Same (the panel filters what non-administrators send) |
| GET | `/data` | User-editable variables | Same |
| POST | `/data` | Changes user-editable variables | Same |
| PUT | `/data` | Changes any variable; `ports` / `primaryPort` become `port`, `port2`, ... | Same, plus port settings and sub-server limits |
| GET | `/tasks` | Scheduled tasks | Same |
| GET, PUT, DELETE | `/tasks/:taskID` | One task | Same |
| POST | `/tasks/:taskID/run` | Runs a task now (skipped while the tasks are paused) | Same |
| POST | `/scheduler/pause`, `/scheduler/resume` | Pauses / resumes the server's scheduled tasks | Node transfer |
| POST | `/reload` | Reloads the definition from disk | Same |
| POST | `/start`, `/restart` | Starts / restarts | Same (the panel refuses suspended servers first) |
| POST | `/stop`, `/kill` | Stops / kills | Same |
| POST | `/install` | Runs the template's installation | Same; also right after creation |
| GET, PUT, POST, DELETE | `/file/*filename` | Read, write, move, delete files | Same; also both transfers and the server assistant |
| GET | `/fs/stat` | Describes a file or folder | None: the panel's server assistant ([below](#files)) |
| POST | `/fs/copy` | Copies a file or folder | None: the panel's server assistant |
| GET | `/fs/search` | Searches names or file contents | None: the panel's server assistant |
| POST | `/archive/*filename` | Compresses files | Same; also both transfers |
| POST | `/extract/*filename` | Extracts an archive | Same; also both transfers |
| GET | `/console` | Console buffer | Same |
| POST | `/console` | Sends a command | Same |
| GET, POST | `/flags` | Auto start / restart flags | Same |
| GET | `/stats` | CPU, memory, disk, network | Same |
| GET | `/status` | Running / installing | Same; also uptime checks and node transfer |
| HEAD, GET | `/query` | Game query | Same |
| GET | `/crashes` | Crash reports | Same |
| GET | `/metrics` | Performance history (`range` `1h`, `24h` or `7d`) | Same |
| GET, POST | `/version` | Game versions / change version | Same |
| GET, PUT | `/notifications` | Notification channels | Same |
| POST | `/notifications/test` | Test message | Same |
| GET, DELETE | `/plugins` | Installed plugins or mods / remove one (`?name=`), in `mods/` on Fabric, Forge and NeoForge and `plugins/` otherwise | Same |
| GET | `/plugins/search` | Searches Modrinth for the server's platform (`?q=`) | Same |
| POST | `/plugins/:pluginId` | Installs a plugin or mod (`?loaders=`, `?game_versions=` filter the Modrinth versions) | Same |
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
| GET | `/backup/busy` | Whether a backup, restore point or restore is being written | Node transfer |
| PUT | `/backup/file?fileName=&modTime=` | Receives a backup archive copied from another node | Node transfer |
| DELETE | `/backup/file?fileName=` | Removes this node's copy of a backup, leaving the off-site copy | Node transfer (undo) |
| GET, PUT | `/offsite` | The server's periodic off-site copy setting (`<id>.offsite`) | Same (`/api/servers/:serverId/offsite`); also the node transfer, which copies it to the new node |
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

### `GET /daemon/servers/status`

```json
{ "servers": { "3F9A1C2B7D4E": { "running": true, "installing": false }, "a1b2c3d4": { "running": false, "installing": true } } }
```

The state of every server loaded on the node, keyed by server ID: the same as `GET /daemon/server/:serverId/status`, for all of them in one call. A server whose state cannot be read is left out, so the panel takes it as unknown rather than stopped. The panel asks every remote node for it every 30 seconds to record [uptime](../../general/13-uptime.md), and uses the last answer in the Discord node report. A node older than this route answers `404`, and the panel treats it as unreachable until it is updated.

The panel sends its notification language (`panel.settings.notificationLanguage`) with each poll in the header `X-Aether-Notification-Language: es|en`, and polls at once when that setting is saved with a new value. The node keeps the value in memory and writes its servers' alerts in it, unless the node's own config file or environment sets `panel.settings.notificationLanguage`, which wins. An empty or unknown value is ignored. After a node restart its alerts use `es` (or its own value) until the next poll, at most 30 seconds later. See [Server notifications](../../features/server-notifications.md).

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

Kills the server if it runs, runs the template's uninstall steps, deletes the server folder and everything the node keeps for it (definition, tasks, crash reports, notification settings, periodic off-site copy setting (`<id>.offsite`), metrics, world map and modpack state, file history, its `/tmp` folder). A failed uninstall step is logged and shown on the console, and the deletion goes on. Then the server's backup folder on this node (`<daemon.data.backups.folder>/<serverId>`) is deleted, unless the request has `?keepBackups=true`, which the panel sends when it undoes a node transfer and removes the server from the new node (it takes back only the backup copies it brought, one by one, with `DELETE /backup/file`). After a transfer that succeeded, the old node gets a plain `DELETE`: its backups were copied to the new node. Copies in off-site storage are never touched. `204`.

### Power actions

`start`, `restart`, `stop` and `install` answer `202` and work in the background; with `?wait` they answer `204` once done. `start` and `restart` answer `409` straight away when the server's disk is full (`ErrDiskLimitExceeded`) or one of its host ports is already published by another running container or held by another process (`ErrPortBusy`). `kill` answers `204`.

### Files

The same rules as the [panel's file routes](./04-server-content.md#files), including the `?folder` flag and the query string being read as part of the file name on `GET`, `PUT` and `DELETE`. Both [transfers](./08-external-transfer.md#what-moves) use these routes with the archive `transfer.tar.gz` and `extract/transfer.tar.gz?destination=.`.

Changes made for the server assistant (they apply to every caller of these routes):

- `PUT /file/*filename?create=true` only creates a new file: it opens the file with `O_EXCL`, so nothing already there (file, folder or link) is replaced, and answers `409 ErrFileExists` when something exists. With `create` in the query (any value), the query is **not** read as part of the name, and no previous version is kept in the file history (there is none). Missing parent folders are made and the disk limit applies as for any `PUT` (`409 ErrDiskLimitExceeded`).
- `DELETE /file/*filename` describes the target without following it: a symlink is removed itself (unlinked from its folder) and what it points at is not touched. Before, deleting a link to a folder emptied that folder and then failed to remove the link. A path that does not exist answers `404` with no body; an empty path, or one that resolves to the server folder itself, answers `400 ErrIllegalFileAccess` instead of emptying the server.

The server assistant (see [AI agent toolkit](../../backend/14-ai-toolkit.md)) also uses three routes that have no panel route: the panel calls them only from its `nodeWorkspace`, for the server of the conversation. They work through the toolkit's workspace over the server's confined file server, so paths are checked again on the node (relative to the server folder; no leading `/`, drive letter, `..` or NUL; at most 4096 bytes and 64 elements) and symlinks are not followed out of the folder.

#### `GET /daemon/server/:serverId/fs/stat?path=`

`path` is relative to the server folder; empty (or `.`) is the folder itself. Answer:

```json
{ "name": "server.properties", "path": "server.properties", "isDir": false, "size": 1187, "modTime": "2026-11-02T09:41:00Z" }
```

A symlink that stays inside the server folder is described by what it points at. The server folder answers `name` and `path` `"."`. `size` is `0` for folders.

#### `POST /daemon/server/:serverId/fs/copy`

Body `{"source": "config", "destination": "config-backup"}`. Copies a file or a folder tree; `204`. The destination must not exist and cannot be inside the source; missing parent folders are made. A symlink as source is refused; inside a copied folder, symlinks, devices, sockets and pipes are skipped. Before copying, the node checks the disk (`ErrDiskLimitExceeded`) and, with a disk limit, adds up the regular files to be copied and refuses with `409 ErrDiskLimitExceeded` when they would not fit; afterwards it measures the folder again.

#### `GET /daemon/server/:serverId/fs/search?dir=&query=&content=`

Looks for `query` (case-insensitive, at most 200 characters, not empty) below `dir` (empty is the server folder), breadth first: in file and folder names, or with `content=true` in the lines of text files. Limits: 8 folders deep, 5000 entries looked at, 200 matches, files up to 1 MiB; binary files and the inside of linked folders are skipped.

```json
{ "matches": [ { "path": "logs/latest.log", "line": 214, "text": "[ERROR] Could not load plugin" } ], "truncated": false }
```

A name match has only `path`; a content match also has `line` (from 1) and `text` (the trimmed line, at most 300 characters). `truncated` is `true` when the search stopped at 200 matches.

#### Errors of the `fs/*` routes

They answer `{"error": {"code": "...", "msg": "..."}}` with a message that holds at most a server-relative path. The panel reads the code back to know what happened.

| Status | Code | When |
|---|---|---|
| `400` | `ErrFileInvalidPath` | Empty, too long or too deep path, NUL byte |
| `400` | `ErrFileOutsideServer` | Absolute path, drive letter, `..`, or a path or link that leaves the server folder |
| `400` | `ErrFileNotAFolder`, `ErrFileIsAFolder` | A file where a folder is needed, or the other way round |
| `400` | `ErrFileSymlink`, `ErrFileNotRegular` | A symlink, device, socket or pipe where a regular file or folder is needed |
| `400` | `ErrFileInvalidRequest` | Bad body, empty or too long search text, a destination inside its source |
| `403` | `ErrFileProtected`, `ErrFilePermission` | A protected path (none by default); the operating system refused |
| `404` | `ErrFileNotFound` | Not found |
| `409` | `ErrFileExists` | The destination exists |
| `409` | `ErrDiskLimitExceeded` | The copy does not fit in the server's disk limit |
| `413` | `ErrFileTooLarge` | Over a size limit |
| `507` | `ErrFileNoSpace` | The node's disk is full |
| `500` | `ErrFileOperationFailed` | Anything else (the cause is only logged on the node) |

### Backups

The node knows backups by file name, not by the panel's record ID: `GET /backup` lists the finished archives (newest first), then the backups made by hand whose archive could not be written, with `"location": "failed"` (the panel drops their records when it sees them and calls `DELETE` to clear the mark). When off-site storage is set up but could not be listed, the answer carries the header `X-Backups-Offsite: unavailable` and the body, same shape, holds only the node's copies; the panel then keeps the records of the copies that were in the bucket and does not turn `both` into `local`. A node without a bucket, or that listed it fine, sends no header, as does an older node. `restore`, `download` and `DELETE` take `?fileName=`. `POST /backup/create` answers `{"backupFileName": "..."}` and writes the archive in the background. `restore` answers `400 ErrBackupServerRunning` while the server runs, otherwise `202`. See [Server content → Backups](./04-server-content.md#backups).

A [node transfer](./03-servers.md#post-apiserversserveridtransfer) uses three more routes:

- `GET /backup/busy` answers `{"busy": true}` while a backup, restore point or restore of the server is being written. The panel refuses the transfer meanwhile; a node without this route is not asked.
- `PUT /backup/file?fileName=<name>&modTime=<RFC 3339>` takes the archive as the raw body (`application/octet-stream`) and writes it in the server's backup folder under the same name, keeping its date (`modTime`, optional), which the list order and the retention of automatic backups go by. The name must be a plain `.tar.gz` file name, not hidden. It is written under a hidden name and moved into place once complete, so a copy cut short is never listed. `204`; `409 ErrFileExists` when a backup with that name is already there (the panel leaves it alone); `400` for a bad `modTime`; `500` for a bad name or a write error.
- `DELETE /backup/file?fileName=<name>` removes this node's copy only (a missing file is fine), unlike `DELETE /backup`, which also deletes the off-site copy. `204`.

### `POST /daemon/server/:serverId/scheduler/pause` and `/scheduler/resume`

No body. `pause` keeps the server's scheduled tasks from running until `resume`: runs that fall due meanwhile are skipped (the node logs `Task <id> skipped: the server is being moved to another node`), and so is `POST /tasks/:taskID/run`, which still answers `204`. A task already running is not interrupted. The state is saved as `"paused": true` in the server's `.cron` file, so it survives a restart of the node. Pausing a paused server, or resuming a running one, does nothing. `204`; `500` when the file cannot be written.

A [node transfer](./03-servers.md#post-apiserversserveridtransfer) pauses the tasks on both nodes while it copies the server (and copies them with `GET /tasks` and `PUT /tasks/:taskID`), then resumes them on the node that keeps the server. A node without these routes answers `404` and the transfer fails at that step and is undone.

### `POST /daemon/server/:serverId/clone-from/:sourceId`

Copies the files of server `:sourceId` (on the same node) into `:serverId` in the background; the target counts as installing until the copy ends. Optional body `{"sourceName": "Survival"}` (the name shown on the console, cut to 80 characters). `202`; `404` when the source is not on the node; `400` when both are the same server; `409` when either is running, the target is not empty or a backup is in progress.

### `GET /daemon/server/:serverId/map/view/*path`

Reverse proxy to the world map's own web server (BlueMap or Dynmap). Only a few request headers are passed on, and `Set-Cookie` is removed from the answer. `400` for a path with `..`; `502` when the map's web server does not answer. The browser reaches it only through the panel's signed link, see [Server content → World map](./04-server-content.md#world-map).

### `GET /daemon/server/:serverId/socket`

WebSocket upgrade. The query flags `console`, `stats` and `status` choose what the socket receives. The panel replaces whatever the browser asked for with the streams the caller's scopes allow (`server.console`, `server.stats`, `server.status`, resolved as on every other route) before it opens the socket to the node with its own token.
