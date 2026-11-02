# Servers

Routes under `/api/servers`: the server list, creating and deleting servers, power actions, console, stats, settings, members, activity, subdomain, clone, node transfer, AI and the WebSocket. Files, backups, tasks and databases are in [Server content](./04-server-content.md). Back to the [API reference](../11-api-reference.md).

Most routes below `/api/servers/:serverId/` are forwarded to the daemon of the server's node (`/daemon/server/:serverId/...`, see [Daemon](./09-daemon.md)): the panel checks the scope, then passes the node's status and body back unchanged (except when the node refuses the panel's own token: `502 ErrNodeAuthFailed`, see [Conventions](./01-conventions.md#node-authentication-failures)). Those are marked "node" in the tables. Every successful change below `/:serverId` is written to the server's [activity log](#get-apiserversserveridactivity).

## Routes

### Servers

| Method | Path | Scope | What it does |
|--------|------|-------|--------------|
| GET | `/api/servers` | Signed in | Lists the servers you can see (paged) |
| GET | `/api/servers/:serverId` | `server.view` | One server, optionally with your permissions on it |
| PUT | `/api/servers/:serverId` | `server.create` | Creates a server with that identifier |
| DELETE | `/api/servers/:serverId` | `server.delete` | Deletes a server |
| POST | `/api/servers/:serverId/suspend` | `server.data.edit.admin` | Suspends or lifts the suspension (toggle) |
| PUT | `/api/servers/:serverId/name/:name` | `server.name.edit` | Renames |
| POST | `/api/servers/:serverId/clone` | `server.create` (experimental) | Copies the server on the same node |
| POST | `/api/servers/:serverId/transfer` | `server.data.edit.admin` | Moves the server to another node |

### Definition, variables and ports

| Method | Path | Scope | What it does |
|--------|------|-------|--------------|
| GET | `/api/servers/:serverId/definition` | `server.definition.view` | Full definition (node) |
| PUT | `/api/servers/:serverId/definition` | `server.definition.edit` | Replaces the definition |
| GET | `/api/servers/:serverId/data` | `server.data.view` | User-editable variables (node) |
| POST | `/api/servers/:serverId/data` | `server.data.edit` | Changes user-editable variables (node) |
| PUT | `/api/servers/:serverId/data` | `server.data.edit.admin` | Changes any variable, limits, address and port list |
| PUT | `/api/servers/:serverId/port-settings` | `server.data.edit` | Primary port and port labels |
| GET | `/api/servers/:serverId/flags` | any of `server.flags.view`, `server.flags.edit` | Auto start / restart flags (node) |
| POST | `/api/servers/:serverId/flags` | `server.flags.edit` | Changes the flags (node) |

### Power and state

| Method | Path | Scope | What it does |
|--------|------|-------|--------------|
| POST | `/api/servers/:serverId/start` | `server.start` | Starts (node) |
| POST | `/api/servers/:serverId/stop` | `server.stop` | Stops (node) |
| POST | `/api/servers/:serverId/restart` | `server.start` and `server.stop` | Restarts (node) |
| POST | `/api/servers/:serverId/kill` | `server.kill` | Kills the process (node) |
| POST | `/api/servers/:serverId/install` | `server.install` | Runs the template's installation (node) |
| POST | `/api/servers/:serverId/reload` | `server.reload` | Reloads the definition from disk (node) |
| GET | `/api/servers/:serverId/status` | `server.status` | Running / installing (node) |
| GET | `/api/servers/:serverId/stats` | `server.stats` | CPU, memory, disk, network (node) |
| HEAD | `/api/servers/:serverId/query` | `server.stats` | Whether the game can be queried (node) |
| GET | `/api/servers/:serverId/query` | `server.stats` | Players and game info (node) |
| GET | `/api/servers/:serverId/metrics` | `server.stats` | Performance history (node) |
| GET | `/api/servers/:serverId/console` | `server.console` | Console buffer (node) |
| POST | `/api/servers/:serverId/console` | `server.console.send` | Sends a command (node) |
| GET | `/api/servers/:serverId/crashes` | `server.console` | Latest crash reports (node) |
| GET | `/api/servers/:serverId/socket` | `server.view` | WebSocket: console, stats, status |

### Game version and notifications

| Method | Path | Scope | What it does |
|--------|------|-------|--------------|
| GET | `/api/servers/:serverId/version` | `server.view` | Current game version and the ones available (node) |
| POST | `/api/servers/:serverId/version` | `server.install` (experimental) | Changes the game version (node) |
| GET | `/api/servers/:serverId/notifications` | `server.flags.edit` | The server's own notification channels (node) |
| PUT | `/api/servers/:serverId/notifications` | `server.flags.edit` | Saves them (node) |
| POST | `/api/servers/:serverId/notifications/test` | `server.flags.edit` | Sends a test message (node) |

### Members, activity and subdomain

| Method | Path | Scope | What it does |
|--------|------|-------|--------------|
| GET | `/api/servers/:serverId/user` | `server.users.view` | Members and their scopes |
| GET | `/api/servers/:serverId/user/:email` | `server.users.view` | One member |
| PUT | `/api/servers/:serverId/user/:email` | `server.users.edit` | Adds or edits a member (invites unknown emails) |
| DELETE | `/api/servers/:serverId/user/:email` | `server.users.delete` | Removes a member |
| PUT | `/api/servers/:serverId/owner/:email` | `admin` | Gives the server to another user |
| GET | `/api/servers/:serverId/activity` | `server.users.view` | Activity log (cursor paged) |
| GET | `/api/servers/:serverId/subdomain` | any of `server.flags.edit`, `server.data.edit` | Subdomain and its DNS records |
| GET | `/api/servers/:serverId/subdomain/check` | any of `server.flags.edit`, `server.data.edit` | Whether a name is free |
| PUT | `/api/servers/:serverId/subdomain` | any of `server.flags.edit`, `server.data.edit` | Sets the subdomain |
| DELETE | `/api/servers/:serverId/subdomain` | any of `server.flags.edit`, `server.data.edit` | Removes it |

### AI

| Method | Path | Scope | What it does |
|--------|------|-------|--------------|
| POST | `/api/servers/:serverId/ai/analyze` | `server.console` | Explains console lines |
| POST | `/api/servers/:serverId/ai/chat` | `server.console` | Chat with the server assistant |
| POST | `/api/servers/:serverId/ai/chat/confirm` | `server.console` | Approves or declines the changes the assistant is waiting on |

External transfer routes (`/extransfer/*`) are in [External transfer](./08-external-transfer.md); the uptime of a server (`/api/uptime/:serverId`) is in [Panel administration](./06-panel-admin.md#uptime).

## The server object

`GET /api/servers` and `GET /api/servers/:serverId` describe a server like this:

```json
{
  "id": "3F9A1C2B7D4E",
  "name": "Survival",
  "nodeId": 1,
  "node": { "id": 1, "name": "node-1", "publicHost": "node1.example.com", "publicPort": 8080, "sftpPort": 5657, "isLocal": false },
  "ip": "0.0.0.0",
  "port": 25565,
  "ports": [25565, 25566],
  "portNotes": { "25566": "RCON" },
  "type": "minecraft-java",
  "icon": "",
  "canGetStatus": true,
  "parent_server_id": "1A2B3C4D5E6F",
  "total_cpu": 200,
  "total_memory": 4096,
  "total_disk": 20480,
  "suspended": false
}
```

| Field | Meaning |
|-------|---------|
| `id` | Identifier used in every path |
| `node` | The node; `privateHost` and `privatePort` are only included for administrators |
| `ports`, `portNotes` | Every port assigned to the server (primary first) and the labels set on them |
| `canGetStatus` | Only in the list: whether you may read its status |
| `isGhost` | Only when true: the server is in the database but not on its (local) node |
| `parent_server_id` | Set on sub-servers (Splitter) |
| `total_cpu`, `total_memory`, `total_disk` | Limits in % of a core, MB and MB; `0` or missing means none |

## Server routes

### `GET /api/servers`

| Query | Default | Meaning |
|-------|---------|---------|
| `name` | `*` | Name filter, `*` is a wildcard |
| `node` | `0` | Node ID, `0` for all |
| `username` | you | Only administrators may list another user's servers; anyone else asking for a different user gets an empty list |
| `page`, `limit` | `1`, `20` | See [Pagination](./01-conventions.md#pagination) |

```json
{ "servers": [ { "id": "3F9A1C2B7D4E", "name": "Survival", "...": "..." } ], "paging": { "page": 1, "pageSize": 20, "maxSize": 100, "total": 1 } }
```

### `GET /api/servers/:serverId`

Add `?perms` to receive your effective permissions on the server too:

```json
{
  "server": { "id": "3F9A1C2B7D4E", "name": "Survival", "...": "..." },
  "permissions": { "serverIdentifier": "3F9A1C2B7D4E", "owner": true, "scopes": ["server.view", "server.start", "..."] }
}
```

Without `?perms`, `permissions` is `null`.

### `PUT /api/servers/:serverId`

Creates a server whose identifier is the path value (letters, digits, `-` and `_`, 1 to 20 characters, not starting with `-` or `_`; otherwise `400`). The web client generates 12 uppercase hexadecimal characters. The body is a server definition (usually a template) plus these fields:

| Field | Meaning |
|-------|---------|
| `name` | Display name; defaults to the identifier |
| `node` | Node ID (ignored for sub-servers, which use the parent's node) |
| `users` | Usernames to add as members with the standard member scopes; the first one becomes the owner |
| `parent_server_id` | Creates a sub-server (Splitter) inside that server |
| `type`, `icon`, `data`, `install`, `run`, `environment`, ... | The definition, see [Servers](../../backend/04-servers.md) |

```json
{
  "name": "Survival",
  "node": 1,
  "users": ["alice"],
  "type": "minecraft-java",
  "environment": { "type": "docker", "image": "ghcr.io/aether-panel/generic:latest" },
  "data": { "port": { "type": "integer", "value": 25565 }, "memory": { "type": "integer", "value": 4096 } },
  "install": [ { "type": "mojangdl", "version": "1.21.1", "target": "server.jar" } ],
  "run": { "command": "java -Xmx${memory}M -jar server.jar nogui", "stop": "stop" }
}
```

In practice the definition is copied from a template (`GET /api/templates/:repo/:name`) and its variable values are filled in.

Answer: `200 {"id": "3F9A1C2B7D4E"}`.

What the panel checks and does:

- The `port`, `port2`, ... variables must be 1024 or higher, unique, not one of the node's own ports and not used by another server on the node; other port variables (query, RCON, any integer variable whose name ends in `port`) are checked the same way. Failures answer `400` with `ErrPortOutOfRange`, `ErrPortDuplicated`, `ErrPortUsedByPanel` or `ErrPortTaken`; `metadata.port` is the number, followed by the variable name for the other port variables (for example `27015 (queryport)`).
- `cpu`, `memory` and `disk` variables become the server's limits; negative values answer `400`. A sub-server needs all three above zero and they must fit in what the parent has left.
- Users without `admin` can only create sub-servers: `parent_server_id` must be a server they own, the environment must be `docker`, and only `image`, `containerRoot`, `config` (without its user) and the template's `portBindings` are kept from it. Anything else answers `403`. See [Sub-servers](../../features/sub-servers.md).
- Owner: the first listed user; for a sub-server the parent's owner; for a non-administrator creator, themselves; an administrator creating without users owns nothing.
- The node gets the definition (`400` when it refuses the environment, for example a host environment with `docker.disallowHost`). With `panel.servers.autoInstall` on (the default) the installation starts right away in the background.
- Members get an email, and an automatic subdomain is assigned when enabled.

Errors: `400 ErrNodeInvalid` for an unknown node, `403` for the sub-server rules, the node's status and body when it refuses, `502 ErrNodeAuthFailed` when the node does not accept the panel's token.

### `DELETE /api/servers/:serverId`

Stops the server, deletes it on the node together with the backup archives kept there (copies already uploaded to off-site storage stay there) and removes its permissions, backup records, OAuth2 clients, sessions, uptime history and DNS records. Its MySQL databases and their users are dropped from their hosts. A server with sub-servers answers `400` until they are deleted. `?skipNode` deletes it from the panel only, without contacting the node. Answer: `204`; members are notified by email (template `deletedServer`).

### `POST /api/servers/:serverId/suspend`

Toggles the suspension of the server and its sub-servers. Suspending also stops them. The answer is the server object with the new `suspended` value. A suspended server cannot be started or restarted (`403`) or cloned by a non-administrator.

### `PUT /api/servers/:serverId/name/:name`

Renames the server in the panel. `204`.

### `POST /api/servers/:serverId/clone`

Experimental: answers `403 ErrFeatureDisabled` unless `panel.experimental.serverClone` is on. Creates a copy on the same node with the same template, settings, limits, owner and members, and new free ports (looked for just above each original port first).

```json
{ "name": "Survival copy", "copyFiles": true, "label": "Survival copy" }
```

`name` is required, at most 40 printable ASCII characters. `copyFiles` (default `true`) also copies the files in the background. `label` is the name shown in the original's activity log. Answer: `200 {"id": "<new id>", "port": 25567}`.

Non-administrators must be members of the original, the original must not be suspended (`403` otherwise) and the sub-server rules above apply. `409` when no port is free, the node does not have the server's files or the copy cannot start; `422` when a sub-server copy does not fit in the parent. Details: [Cloning](../../features/cloning.md).

### `POST /api/servers/:serverId/transfer`

```json
{ "nodeId": 2 }
```

Moves the server to another node of this panel in the background, together with its [sub-servers](../../features/sub-servers.md) (a sub-server already on the target node stays where it is) and the backups their old node keeps. Answer: `202 {"msg": "Transfer started"}`.

Checked before answering:

| Error | When |
|-------|------|
| `400 ErrTransferSubServer` | The server is a sub-server: it stays on its parent's node and moves with the parent |
| `400` | The target is the server's own node |
| `404` | Unknown node |
| `409 ErrTransferRunning` | The server or one of its sub-servers is already being transferred, or is in a transfer recorded in `node_transfers` that has not ended |
| `409 ErrTransferPortsTaken` | A port (primary or extra) of the server or of a sub-server is taken on the target node: by its panel, daemon or SFTP port, by a server already there or by a port its containers publish (`metadata.ports` lists them) |
| `409 ErrTransferBackupRunning` | The old node is writing a backup, restore point or restore of one of them (asked through [`GET /daemon/server/:id/backup/busy`](./09-daemon.md); a node without that route is not asked) |

The transfer is recorded in the `node_transfers` table before the answer and updated at each step. Then, in the background:

1. **Prepare**, server by server: pause its scheduled tasks on the old node (`POST /daemon/server/:id/scheduler/pause`), copy the definition to the new node, pause the tasks there and copy each task (`GET /tasks`, then `PUT /tasks/:taskId` on the daemon), stop the server on the old node (killing it if it does not stop), archive its files, stream the archive to the new node and extract it, and copy each backup whose archive is on the old node (`local` or `both`) through the panel into the new node, keeping its name and date.
2. **Commit**: one database transaction points every record at the new node and marks the transfer record committed. Only `node_id` is written, so a change made to the server meanwhile is kept.
3. **Clean up**: delete the servers from the old node, with the backup archives there. Copies in off-site storage are not touched; the bucket is the same wherever the server runs.
4. **Start**: resume the tasks on the new node (`POST /daemon/server/:id/scheduler/resume`) and start there the servers that were running before. The transfer record is then deleted.

When a step fails, up to and including the commit, everything is undone, newest first: the backup copies and the servers are removed from the new node, the archives from the old one, the tasks are resumed on the old node and a server that was running is started again there. The servers stay where they were. The failure is written to the panel log and to the activity log of each server as `server.transferFailed` (step and reason). A server that does not start on the new node after the commit stays there and gets `server.transferStartFailed`. Its subdomain follows it to the new node's address.

When the panel starts, it ends the transfers a stop cut short, in one pass with no retry: one not committed is undone as above (activity `server.transferFailed`, detail `the panel restarted while <step>: the transfer was undone`), one committed is finished (old copies deleted, tasks resumed and servers started on the new node). See [Recovery after a panel restart](../../features/server-transfers.md#recovery-after-a-panel-restart).

Limits: recovery after a restart is a single pass, and a node unreachable at that moment keeps what was left on it; both daemons need the scheduler routes (`/scheduler/pause`, `/scheduler/resume`), so with an older daemon the transfer fails while pausing the tasks and is undone; a task already running is not interrupted, and the time zone and concurrency settings of the tasks are not copied; only one level of sub-servers is moved. Details: [Server transfers](../../features/server-transfers.md).

## Definition, variables and ports

### `GET /api/servers/:serverId/definition`

The full server definition as the node stores it (`type`, `display`, `icon`, `data`, `groups`, `install`, `uninstall`, `run`, `environment`, `supportedEnvironments`, `requirements`, `stats`, `query`, `keepAlive`). The format is described in [Servers](../../backend/04-servers.md).

### `PUT /api/servers/:serverId/definition`

Body: a definition plus optional `name`. The server must be idle on the node. Without `server.data.edit.admin` the panel keeps the current environment (except the Docker `image`, `containerRoot` and `config` without its user) and the current values of locked variables, so customers cannot change their limits, address or host access. What each setting does: [Server settings](../../features/server-settings.md). The `port`, `ip`, `name`, `type` and `icon` are copied to the panel. Answer: `204`, or the node's error unchanged.

### `GET /api/servers/:serverId/data`

The variables marked user-editable, with their groups:

```json
{
  "data": { "motd": { "type": "string", "value": "Welcome", "display": "MOTD", "required": false, "userEdit": true } },
  "groups": [ { "display": "General", "description": "", "variables": ["motd"], "order": 0 } ]
}
```

### `POST /api/servers/:serverId/data`

Flat object of new values: `{"motd": "Hello"}`. Only user-editable variables change. Answer: `204` from the node.

### `PUT /api/servers/:serverId/data`

The administrator's variant. Accepts any variable plus these fields, which the panel also applies to its own record:

| Field | Meaning |
|-------|---------|
| `cpu`, `memory`, `disk` | Limits (`%`, MB, MB); checked against the parent for sub-servers and against the sub-servers' share for parents |
| `ip`, `port` | Address |
| `ports` | Full port list; the node turns it into the `port`, `port2`, ... variables and drops the ones no longer listed |
| `primaryPort` | Which of `ports` is the main one (must be in the list) |
| `portNotes` | Labels per port, e.g. `{"25575": "RCON"}` |

The node is called first; if it refuses, nothing is saved. Answer: `204`.

### `PUT /api/servers/:serverId/port-settings`

For members who may edit variables: changes which assigned port is primary and the labels, without touching the port list.

```json
{ "primaryPort": 25566, "portNotes": { "25565": "Game", "25566": "Query" } }
```

Answer: `200 {"success": true}`.

### Flags

`GET /api/servers/:serverId/flags`:

```json
{ "autoStart": false, "autoRestartOnCrash": true, "autoRestartOnGraceful": false }
```

`POST` takes the same object; fields left out are not changed. `204`.

## Power and state

### Power actions

`POST /start`, `/stop`, `/restart`, `/kill`, `/install`, `/reload`. Without a query, `start`, `stop`, `restart` and `install` answer `202` and run in the background; add `?wait` to get `204` once done. `kill` and `reload` answer `204`.

| Error | When |
|-------|------|
| `403` | Starting or restarting a suspended server |
| `409 ErrDiskLimitExceeded` | Starting or restarting with the disk limit reached |
| `409 ErrPortBusy` | A host port of the server is already published by another running container or held by another process on the node (`metadata.port`) |

An install refused because the node is below `daemon.minFreeDiskMB` fails with `ErrNodeDiskFull` (shown on the console; returned as an error with `?wait`).

### `GET /api/servers/:serverId/status`

`{"running": true, "installing": false}`. While installing, `running` is `false`.

### `GET /api/servers/:serverId/stats`

```json
{ "cpu": 45.2, "maxCpu": 200, "memory": 1536000000, "maxMemory": 4294967296, "storage": 5000000000, "maxStorage": 21474836480, "networkRx": 1200, "networkTx": 800, "running": true, "jvm": { "...": "..." } }
```

`cpu` is in percent of one core (200 = two cores busy); `maxCpu` is what the server may use in the same unit (its CPU limit, or every core of the node). The panel shows `cpu / maxCpu`. `memory` leaves out the inactive page cache, like `docker stats`. `jvm` only appears for Java servers that report it.

For a server the node does not have, `status`, `stats` and `console` answer an empty stopped state instead of `404`.

### `GET /api/servers/:serverId/query`

`HEAD` answers `202` when the template's query type is `minecraft`, `source` or `steam`, else `204`. `GET` answers `200` with the game's answer (players, map, version...) or `204` when it does not answer.

### `GET /api/servers/:serverId/metrics?range=24h`

`range` is `1h`, `24h` (default) or `7d`; anything else answers `400`.

```json
{
  "range": "24h",
  "step": 900,
  "points": [ { "t": 1793576400, "cpu": 12.5, "memory": 1.2e9, "maxMemory": 4.2e9, "rx": 3.1, "tx": 2.4, "online": 1, "players": 3.5, "playersPeak": 6, "maxPlayers": 20, "hasPlayers": true } ],
  "record": { "players": 14, "at": "2026-10-30T21:14:00Z" }
}
```

`online` is the share of samples taken while running; `noData: true` marks a step with nothing recorded. Details: [Server statistics](../../features/server-statistics.md).

### `GET /api/servers/:serverId/console`

```json
{ "epoch": 1793576400123456, "logs": "W1NlcnZlciB0aHJlYWQvSU5GT10..." }
```

`logs` is the console output as **base64** (decode it to get the text, ANSI codes included). `epoch` is in microseconds; pass it back as `?time=` to get only what was printed after it. A `time` that is not a number answers `400 ErrInvalidUnixTime`.

### `POST /api/servers/:serverId/console`

The body is the command as plain text (not JSON). `204`. A command to a server whose console the node lost after a daemon restart fails with `ErrConsoleDetached`.

### `GET /api/servers/:serverId/crashes`

The latest crash reports, newest first:

```json
[ { "id": "...", "time": "2026-10-31T10:00:00Z", "exitCode": 1, "lines": ["..."], "restarted": true, "attempt": 1, "limit": 3 } ]
```

Details: [Crash reports](../../features/crash-reports.md).

## Game version and notifications

### `GET /api/servers/:serverId/version`

```json
{ "platform": "paper", "software": "paper", "variable": "version", "current": "1.21.1", "versions": ["1.21.4", "1.21.3"], "changing": false }
```

`204` when the template does not let the panel change the version; `502` when the version lists (PaperMC, Mojang) cannot be reached.

### `POST /api/servers/:serverId/version`

Experimental (`panel.experimental.versionSwitch`, else `403 ErrFeatureDisabled`). Body `{"version": "1.21.4"}`. Stops the server, saves a restore point, reinstalls with the new version and starts it again if it was running. `202`. `400` for an unknown version or a template without version switching; `409` while a change is running. Details: [Game versions](../../features/game-versions.md).

### `GET /api/servers/:serverId/notifications`

```json
{
  "discordWebhook": "", "telegramBotToken": "", "telegramChatId": "", "webhookUrl": "", "webhookSecret": "",
  "crash": true, "backup": true, "resources": true, "power": false
}
```

`PUT` saves the same object (`400` for an invalid Discord webhook, Telegram data or webhook URL; `204` on success). `POST /notifications/test` posts a sample to every configured channel and answers `200` with each channel's result (`"ok"` or the reason), `400` when none is configured, `429` within 10 seconds of the previous test. Details: [Server notifications](../../features/server-notifications.md).

## Members

Every server has an owner and may have sub-users. The owner holds the owner's set of scopes whatever their row says and cannot be limited or removed here; only an administrator can change the owner. Rules: [Permissions reference](../../general/scopes-reference.md).

### `GET /api/servers/:serverId/user`

```json
[ { "username": "alice", "email": "alice@example.com", "scopes": ["server.view", "server.console"], "owner": true } ]
```

`GET /user/:email` answers the same list with one entry. An email that is not a member answers `404`, like an unknown one.

### `PUT /api/servers/:serverId/user/:email`

```json
{ "scopes": ["server.view", "server.console", "server.start"] }
```

Only server scopes are kept. A caller without `server.admin` or `admin` can only grant or remove scopes they hold themselves. An unknown email gets an invited account (random password, `login` scope) and an email to choose a password. `204`.

| Error | When |
|-------|------|
| `409 ErrServerOwner` | The email is the owner's |
| `403 ErrInviteSignupsClosed` | The email has no account, sign-ups are closed and the caller is not an administrator |
| `400 ErrFieldNotEmail` | The email is not valid |

### `DELETE /api/servers/:serverId/user/:email`

`204`; the user is notified by email. `404 ErrUserNotFound`, `409 ErrServerOwner`.

### `PUT /api/servers/:serverId/owner/:email`

Makes that user the owner (and a member). The previous owner stays as a sub-user with the scopes their row already had. `204`; `404` for an unknown email.

## Activity log

### `GET /api/servers/:serverId/activity`

| Query | Meaning |
|-------|---------|
| `limit` | Entries per page, default 50, at most 100 |
| `before` | Only entries older than this id |
| `type` | A family (`power`, `console`, `files`, `backups`, `users`, `settings`, `tasks`) or an action prefix such as `file`, `backup.create` or `ai.edit_file` (lowercase letters, one dot, underscores after the dot) |

```json
{ "items": [ { "id": 812, "action": "console.command", "detail": "say hi", "username": "alice", "userId": 3, "createdAt": "2026-11-01T09:12:00Z" } ], "nextCursor": 790 }
```

`ip` is only included for administrators. Actions include `server.start`, `file.write`, `file.upload`, `backup.create`, `user.add`, `task.save`, `settings.notifications`, the node transfer's `server.transfer`, `server.transferFailed` and `server.transferStartFailed` (in the `settings` family), and the assistant's file changes `ai.create_file`, `ai.create_directory`, `ai.copy_path`, `ai.edit_file`, `ai.write_file`, `ai.move_path`, `ai.rename_path` and `ai.delete_path` (in the `files` family, detail: the path or `source → destination`); settings changes are recorded without their values. Details: [Activity and access log](../../features/activity-and-access-log.md).

## Subdomain

Needs the Cloudflare settings (`panel.dns.cloudflare.token`, `panel.dns.cloudflare.zoneId`, `panel.dns.baseDomain`). Details: [Subdomains](../../features/subdomains.md).

### `GET /api/servers/:serverId/subdomain`

```json
{
  "enabled": true, "domain": "play.example.com", "subdomain": "survival", "address": "survival.play.example.com",
  "srv": true, "port": 25565,
  "records": [ { "type": "A", "name": "survival.play.example.com", "content": "203.0.113.5" } ]
}
```

### `GET /api/servers/:serverId/subdomain/check?name=survival`

`{"available": true, "reason": ""}`; `reason` is `invalid`, `reserved`, `taken` or `current`.

### `PUT /api/servers/:serverId/subdomain`

`{"subdomain": "survival"}` (3 to 32 lowercase letters, digits and inner hyphens). Creates the records and removes the old ones; answers the subdomain object.

| Error | When |
|-------|------|
| `400 ErrSubdomainDisabled` | Subdomains are not configured |
| `400 ErrSubdomainInvalid`, `ErrSubdomainReserved` | Name not allowed |
| `409 ErrSubdomainTaken` | Another server uses it |
| `422` | Cloudflare refused, or the node has no public address (`ErrSubdomainNoTarget`) |
| `429 ErrSubdomainRateLimited` | Changed less than 10 seconds ago |

### `DELETE /api/servers/:serverId/subdomain`

Deletes the records. `204`; `422` when Cloudflare refuses (the name is kept so it can be retried).

## AI

The three routes need `panel.settings.geminiApiKey` (otherwise `400 {"error": "Gemini API Key is not configured. Please add it in Settings."}`) and share a quota of 30 requests per user every 10 minutes (`429`). Details: [AI assistant](../../features/ai-assistant.md) and [AI assistant: working with files](../../features/ai-agent.md).

### `POST /api/servers/:serverId/ai/analyze`

```json
{ "logs": ["[ERROR] ...", "..."], "language": "en", "exitCode": 1 }
```

`logs` is required (the newest 200 lines, up to 32 KB, are sent); `language` is `es` or `en`; `exitCode` marks the lines as coming from a crash. Answer:

```json
{ "summary": "...", "rootCauses": ["..."], "suggestions": ["..."] }
```

`500 {"error": "..."}` when the AI service fails. While `panel.settings.hideAiAnalysis` is on, the route answers `403 ErrAIAnalysisDisabled` for every caller that has the scope, before the Gemini key and the quota are checked (the chat below is not affected); `GET /api/config` exposes the switch as `hideAiAnalysis`.

### `POST /api/servers/:serverId/ai/chat`

```json
{ "messages": [ { "role": "user", "content": "Why does my server lag?" } ], "language": "en" }
```

1 to 20 messages of 1 to 2000 characters, `role` `user` or `assistant`, the last one from the user. The assistant sees the server's status, players, last day of performance, crashes and recent console. Answer: `{"reply": "...", "sources": ["status", "console"]}`; `502` when the AI service fails.

**Agent mode.** When `panel.settings.aiFileTools` is on (the default), the request has a signed-in user and that user holds `server.files.view` on the server, the assistant can also use file tools on this server (and only this one): reading tools with `server.files.view`, creating, changing, moving and deleting tools with `server.files.edit`. Reads and creations run during the request; changes to existing data stop for confirmation. The answer then is:

```json
{
  "reply": "I will remove the old world folder.",
  "sources": ["status", "console"],
  "steps": [
    { "id": "local-1-1", "tool": "list_directory", "kind": "read", "summary": "Listed the server folder (12 entries)", "paths": ["."], "status": "done" }
  ],
  "pending": {
    "token": "9f2c…",
    "actions": [
      { "id": "local-2-1", "tool": "delete_path", "kind": "destructive", "summary": "Delete the folder world_old and everything in it", "paths": ["world_old"], "status": "pending" }
    ],
    "expiresAt": "2026-11-02T10:10:00Z"
  }
}
```

| Field | Meaning |
|-------|---------|
| `steps` | The tool calls of this request, in order. `kind` is `read`, `create`, `modify` or `destructive` (empty for a refused call); `status` is `done`, `failed`, `declined` or `refused` (not run: unknown tool, missing scope or a limit); `error` explains a failure or refusal; `paths` are relative to the server folder |
| `pending` | `null`, or the calls waiting for the user (`status` `pending`), the single-use `token` and `expiresAt` (10 minutes) |

When `pending` is set, `reply` is the model's explanation of those calls and may be empty. In agent mode a failed task answers `502 {"error": "...", "steps": [...], "pending": null}` with the steps that ran before it failed. The changes made are recorded in the [activity log](#activity-log) as `ai.<tool>`. Tools, limits and isolation: [AI agent toolkit](../../backend/14-ai-toolkit.md).

### `POST /api/servers/:serverId/ai/chat/confirm`

Answers a pending confirmation. Scope `server.console`; counts against the same quota.

```json
{ "token": "9f2c…", "approved": ["local-2-1"], "language": "en" }
```

`approved` holds the IDs of the pending actions to run (at most 64); the others are declined, and an empty list declines them all. The token and each ID have at most 128 characters. Each approved action is checked again (scopes and files as they are now) before it runs, then the assistant continues. The answer has the same shape as the agent-mode chat (with `sources: []`) and may carry a new `pending`.

| Error | When |
|-------|------|
| `400` | No Gemini key; invalid JSON; token missing or longer than 128 characters; more than 64 IDs or an empty or too long ID |
| `403 ErrNoPermission` | No signed-in user, or the caller no longer has `server.files.view` on the server. Without `server.files.edit`, approved changes are not run and come back as `refused` steps |
| `409 ErrAIConfirmationInvalid` | The token is unknown, expired, already used or replaced by a newer request of the same user on the same server; it belongs to another user or server; an approved ID was not pending; or `panel.settings.aiFileTools` was turned off meanwhile. Pending confirmations are kept in memory, so a panel restart also invalidates them |
| `429` | Quota |
| `502` | The assistant failed while continuing: `{"error": "...", "steps": [...], "pending": null}` |

A token sent by another user or for another server is refused without being used up. An answer naming an ID that was not pending uses the token up.

## WebSocket

### `GET /api/servers/:serverId/socket`

Opens a WebSocket that streams the server's console, stats and status. Scope: `server.view`. Browser connections use the session cookie and must be same-origin (see the [same-origin guard](./02-authentication.md#same-origin-guard)); other clients send `Authorization: Bearer <token>`.

```javascript
const ws = new WebSocket(`wss://panel.example.com/api/servers/${serverId}/socket`);
```

The query string is ignored: the panel subscribes the connection to the streams the caller's scopes on the server allow (`console` with `server.console`, `status` with `server.status`, `stats` with `server.stats`). Those are the scopes the permission check resolved for this request, as on every other route: the owner's scopes, the server and panel-wide rows, the role's panel-wide scopes, and an OAuth2 client's bounds.

Messages are JSON `{"type": ..., "data": ...}`:

| `type` | `data` |
|--------|--------|
| `console` | `{"epoch": 0, "logs": "<base64>"}`: new console output, base64 like `GET /console` |
| `stat` | A stats object like `GET /stats` |
| `status` | `{"running": true, "installing": false}` |
| `crash` | `{"id": "...", "exitCode": 1, "restarted": true}`: sent on the status stream when the server dies on its own |

Messages the client sends are read only to keep the connection alive and are otherwise ignored: commands go through `POST /console`. The server pings every 25 seconds and drops a peer silent for 60 seconds. Internals: [WebSocket](../../backend/08-websocket.md).
