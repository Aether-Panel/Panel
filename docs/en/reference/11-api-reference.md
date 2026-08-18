# API Reference

> **Note:** The project has the names Aether Panel (public) and SkyPanel (code, CLI, Go module v3).

Interactive OpenAPI/Swagger documentation available at `http://localhost:8080/swagger/index.html` when the panel is running.

---

## Table of Contents

- [Authentication](#authentication)
- [Scopes (Permissions)](#scopes-permissions)
- [Response Format](#response-format)
- [Data Types](#data-types)
- [Config](#config)
- [Auth](#auth)
- [OAuth2](#oauth2)
- [Nodes](#nodes)
- [Servers](#servers)
- [Files](#files)
- [Archived Files](#archived-files)
- [Plugins](#plugins)
- [Backups](#backups)
- [Tasks](#tasks)
- [Flags](#flags)
- [Console](#console)
- [Server Databases](#server-databases)
- [Server Users](#server-users)
- [Transfer Between Nodes](#transfer-between-nodes)
- [External Transfer](#external-transfer)
- [AI](#ai)
- [Global Users](#global-users)
- [Self (Own Profile)](#self-own-profile)
- [Settings](#settings)
- [User Settings](#user-settings)
- [API Keys](#api-keys)
- [Roles](#roles)
- [Database Hosts](#database-hosts)
- [Templates](#templates)
- [Provision Products](#provision-products)
- [Provision API v1](#provision-api-v1)
- [Uptime](#uptime)
- [Daemon](#daemon)
- [WebSocket](#websocket)

---

## Authentication

### 1. OAuth2 Client Credentials (External API)

```
POST /oauth2/token
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials&client_id=ID&client_secret=SECRET
```

**Response:**
```json
{
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "server.view server.start"
}
```

Use the token in all requests:
```
Authorization: Bearer eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...
```

### 2. OAuth2 Password Grant (SFTP/SSH)

```
POST /oauth2/token
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer <node-token>

grant_type=password&username=email%23serverId&password=USER_PASSWORD&scope=sftp
```

### 3. API Keys (Provision)

```
X-Api-Key: ak_<key>
```
Or as Bearer:
```
Authorization: Bearer ak_<key>
```

### 4. Panel Auth (Web Sessions)

Use the `/auth/login`, `/auth/otp`, etc. endpoints to obtain session cookies.

---

## Scopes (Permissions)

### Server Scopes (per server)

| Scope | Description |
|-------|-------------|
| `server.view` | View server |
| `server.admin` | Server admin |
| `server.delete` | Delete server |
| `server.definition.edit` | Edit definition |
| `server.definition.view` | View definition |
| `server.data.edit` | Edit server data |
| `server.data.edit.admin` | Edit data (admin) |
| `server.data.view` | View data |
| `server.flags.edit` | Edit flags |
| `server.flags.view` | View flags |
| `server.name.edit` | Change name |
| `server.clients.view` | View OAuth2 clients |
| `server.clients.edit` | Edit clients |
| `server.clients.create` | Create clients |
| `server.clients.delete` | Delete clients |
| `server.users.view` | View server users |
| `server.users.create` | Add users |
| `server.users.edit` | Edit user permissions |
| `server.users.delete` | Delete server users |
| `server.tasks.view` | View tasks |
| `server.tasks.run` | Run task |
| `server.tasks.create` | Create task |
| `server.tasks.delete` | Delete task |
| `server.tasks.edit` | Edit task |
| `server.reload` | Reload server |
| `server.start` | Start server |
| `server.stop` | Stop server |
| `server.kill` | Kill process |
| `server.install` | Run installation |
| `server.files.view` | View files |
| `server.files.edit` | Upload/edit/delete files |
| `server.sftp` | SFTP access |
| `server.console` | View console |
| `server.console.send` | Send commands |
| `server.stats` | View statistics |
| `server.status` | View status |
| `server.backup.view` | View backups |
| `server.backup.create` | Create backup |
| `server.backup.restore` | Restore backup |
| `server.backup.delete` | Delete backup |
| `server.admin.view` | Admin: view |
| `server.admin.install.view` | Admin: view installation |
| `server.admin.install.manage` | Admin: manage installation |
| `server.admin.transfer.view` | Admin: view transfers |
| `server.admin.transfer.manage` | Admin: manage transfers |
| `server.admin.config.view` | Admin: view config |
| `server.admin.config.manage` | Admin: manage config |
| `server.admin.assignments.view` | Admin: view assignments |
| `server.admin.assignments.manage` | Admin: manage assignments |

### Global Scopes

| Scope | Description |
|-------|-------------|
| `admin` | Superadmin |
| `login` | Log in |
| `panel` | Panel access |
| `oauth2.auth` | Validate credentials via OAuth2 |
| `nodes.view` | View nodes |
| `nodes.create` | Create nodes |
| `nodes.edit` | Edit nodes |
| `nodes.delete` | Delete nodes |
| `nodes.deploy` | Get deployment data |
| `self.edit` | Edit own profile |
| `self.clients` | Manage own OAuth2 clients |
| `settings.edit` | Edit global configuration |
| `templates.view` | View templates |
| `templates.local.edit` | Edit local templates |
| `templates.repo.create` | Add repositories |
| `templates.repo.delete` | Delete repositories |
| `users.info.search` | Search users |
| `users.info.view` | View users |
| `users.info.edit` | Create/edit/delete users |
| `users.perms.view` | View user permissions |
| `users.perms.edit` | Edit user permissions |
| `uptime.view` | View uptime statistics |
| `server.create` | Create servers |

---

## Response Format

### Success
```json
{ "data": { ... } }
```
Or directly an array or object depending on the endpoint.

### Error
```json
{
  "error": {
    "code": "ErrFieldRequired",
    "msg": "username: required field is missing"
  }
}
```

Error codes: `ErrFieldRequired`, `ErrFieldLength`, `ErrServerNotFound`, `ErrUserNotFound`, `ErrNodeInvalid`, `ErrDatabaseNotAvailable`, `ErrNoPermission`, `ErrInvalidCredentials`, `ErrUnknownError`. Complete list in `pkg/skypanel/errors.go`.

### HTTP Codes

| Code | Meaning |
|--------|-------------|
| 200 | OK |
| 201 | Created |
| 204 | No Content |
| 202 | Accepted (asynchronous operation) |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 500 | Internal Server Error |

---

## Data Types

### Paging
```json
{ "page": 1, "size": 20, "maxSize": 100, "total": 1 }
```

### Error
```json
{
  "error": { "code": "ErrFieldRequired", "msg": "...", "metadata": {} }
}
```

### Node
```json
{
  "id": 1, "name": "Node-01", "isLocal": true,
  "publicHost": "node1.example.com", "publicPort": 8080,
  "privateHost": "192.168.1.10", "privatePort": 8080,
  "sftpPort": 5657
}
```

### ServerInfo
```json
{
  "id": "abc123", "name": "Minecraft", "node": { "id": 1, "name": "Node" },
  "nodeId": 1, "ip": "192.168.1.100", "port": 25565, "type": "minecraft-java",
  "icon": "minecraft.png", "isGhost": false, "canGetStatus": true,
  "users": [ { "username": "admin", "scopes": ["server.view"] } ]
}
```

### ServerDefinition (Create/Edit)
```json
{
  "name": "My Server", "type": "minecraft-java", "icon": "minecraft.png",
  "environment": { "type": "standard" },
  "install": [ { "type": "mojangdl", "version": "1.20.1" } ],
  "uninstall": [],
  "run": {
    "command": "java -Xmx{{memory}}M -jar server.jar nogui",
    "stop": "stop", "stopCode": 0,
    "pre": [], "post": [],
    "environmentVars": { "KEY": "VALUE" },
    "workingDirectory": "",
    "autostart": false, "autorestart": false, "autorecover": false,
    "expectedExitCode": 0,
    "stdin": { "type": "", "ip": "", "port": "", "password": "" }
  },
  "data": {
    "memory": { "type": "integer", "value": 2048, "required": true, "desc": "Memory MB", "display": "Memory" },
    "port": { "type": "string", "value": "25565", "required": true },
    "version": { "type": "string", "value": "1.20.1", "required": true }
  },
  "groups": [ { "display": "Config", "variables": ["memory", "port"] } ],
  "requirements": { "os": "linux", "arch": "amd64", "binaries": ["java"] },
  "supportedEnvironments": [ { "type": "standard" }, { "type": "docker" } ],
  "keepAlive": { "command": "", "frequency": "" },
  "query": { "type": "" },
  "stats": { "type": "" }
}
```

### ServerData
```json
{
  "data": { "version": { "type": "string", "value": "1.20.1" } },
  "groups": [ { "display": "Config", "variables": ["memory", "port"] } ]
}
```

### ServerRunning (Status)
```json
{ "running": true, "installing": false }
```

### ServerStats
```json
{
  "cpu": 45.2, "memory": 1536000000, "maxMemory": 2147483648,
  "running": true, "storage": 5000000000,
  "jvm": { "heapUsed": 1000000000, "heapTotal": 2000000000, "metaspaceUsed": 50000000, "metaspaceTotal": 100000000 }
}
```

### ServerLogs
```json
{ "logs": ["[10:30:15] [Server thread/INFO]: Starting server"], "epoch": 1705312215 }
```

### ServerFlags
```json
{ "autoStart": false, "autoRestartOnCrash": true, "autoRestartOnGraceful": false }
```

### ServerTask
```json
{
  "name": "Daily backup", "description": "Runs backup every 6h",
  "cronSchedule": "0 */6 * * *",
  "operations": [ { "type": "backup" } ]
}
```

### User
```json
{ "id": 1, "username": "admin", "email": "admin@example.com", "otpActive": false, "roleId": null, "scopes": [] }
```

### Permissions
```json
{ "serverIdentifier": "abc123", "scopes": [ { "value": "server.view", "forServer": true } ] }
```

### Backup
```json
{ "id": 1, "name": "Backup 2024-01-15", "fileName": "backup_abc123.tar.gz", "createdAt": "2024-01-15T10:30:00Z" }
```

### Features
```json
{ "arch": "amd64", "os": "linux", "version": "1.0.0", "environments": ["standard", "docker"], "features": ["docker"] }
```

### SystemInfo
```json
{
  "hostname": "node1", "os": "linux", "platform": "ubuntu", "platformVersion": "22.04",
  "arch": "amd64", "uptime": 123456,
  "cpuModel": "Intel Core", "cpuCores": 8, "cpuThreads": 16, "cpuUsage": 25.5,
  "memoryTotal": 17179869184, "memoryUsed": 8589934592, "memoryFree": 8589934592,
  "disks": [ { "path": "/", "total": 1099511627776, "used": 549755813888, "free": 549755813888, "usedPercent": 50.0 } ],
  "networkBytesSent": 1000000, "networkBytesRecv": 2000000
}
```

### DatabaseHostView
```json
{
  "id": 1, "name": "DB Host 1", "host": "db1.example.com", "port": 3306,
  "username": "skypanel", "node_id": 1, "max_databases": 10,
  "created_at": "...", "updated_at": "..."
}
```

### DatabaseView
```json
{
  "id": 1, "server_id": "abc123", "database_host_id": 1, "database_name": "server_abc123",
  "host": "db1.example.com", "port": 3306, "username": "user_abc123",
  "password": "pass123", "max_connections": 5, "remote_connection": "",
  "host_name": "DB Host 1", "created_at": "...", "updated_at": "..."
}
```

### Template
```json
{
  "id": "minecraft-java", "name": "Minecraft Java Edition", "display": "Minecraft Java",
  "type": "minecraft-java", "icon": "minecraft.png",
  "install": [], "run": { "command": "" }, "data": {},
  "environment": { "type": "standard" }
}
```

### TemplateRepo
```json
{ "id": 1, "name": "official", "url": "https://templates.example.com/templates.json", "branch": "main", "isLocal": false }
```

### Role
```json
{ "id": 1, "name": "Admin", "description": "Full access", "scopes": ["admin"], "createdAt": "...", "updatedAt": "..." }
```

### Client (OAuth2)
```json
{ "client_id": "abc123", "client_secret": "secret...", "name": "My App", "description": "App description" }
```

### PluginInfo
```json
{ "name": "EssentialsX.jar", "version": "2.20.1", "size": 1234567 }
```

### PluginSearchResult
```json
{
  "id": "essentialssx", "name": "EssentialsX", "tag": "essentialsx",
  "version": "2.20.1", "author": "EssentialsX Team",
  "description": "Essential commands...", "iconUrl": "...", "downloads": 1000000
}
```

---

## Config

### `GET /api/config`
No authentication. Returns the panel's public configuration.

```json
{
  "branding": { "name": "SkyPanel" },
  "registrationEnabled": true,
  "themes": { "active": "default", "available": ["alternativeTheme"], "settings": "{}" }
}
```

---

## Auth

### `POST /auth/login`
**Body:**
```json
{ "email": "admin@example.com", "password": "..." }
```
**Response:**
```json
{
  "scopes": [{ "value": "server.view", "forServer": true }],
  "otpNeeded": false,
  "session": "session_token"
}
```
- `otpNeeded: false` → full login; it also sets the `skypanel_auth` cookie.
- `otpNeeded: true` → continue with `/auth/otp` (the current session is stored in the browser cookie).
- `session` exposes the session token for authentication of external apps.

### `POST /auth/otp`
**Body:**
```json
{ "token": "123456" }
```
`token` is the **OTP code** (the user's email and the timestamp come from the cookie session, with a 5-minute expiration).
**Response:** same as `/auth/login` (LoginResponse with `scopes` and `session`).

### `POST /auth/logout`
Closes the current session.

### `POST /auth/register`
Requires `registrationEnabled: true`.
**Body:**
```json
{ "username": "newuser", "email": "new@example.com", "password": "Secure123!" }
```

### `POST /auth/forgot-password`
Requests a reset link for an email. **Response:** always `204` (prevents user enumeration). If the email exists, an email is sent with the link `/reset-password/?token=<token>`.
**Body:**
```json
{ "email": "admin@example.com" }
```

### `POST /auth/reset-password`
Consumes the reset token and sets a new password. **Response:** `204`. **Errors:** `400` if the token is invalid/expired or the password does not meet the requirements.
**Body:**
```json
{ "token": "reset_token", "password": "NewSecure123!" }
```

### `POST /auth/reauth`
Re-authenticates the current session. **Auth:** Bearer.

### `GET /auth/publickey`
Returns the Ed25519 public key in JWK format to validate JWTs.

---

## OAuth2

### `POST /oauth2/token`
**Form (urlencoded):** `grant_type`, `client_id`, `client_secret`, `username`, `password`

See [Authentication](#authentication) for examples.

**Errors:**
```json
{ "error": "invalid_client", "error_description": "Invalid client credentials" }
```

---

## Nodes

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| GET | `/api/nodes` | `nodes.view` | List nodes |
| POST | `/api/nodes` | `nodes.create` | Create node |
| GET | `/api/nodes/:id` | `nodes.view` | Get node |
| PUT | `/api/nodes/:id` | `nodes.edit` | Update node |
| DELETE | `/api/nodes/:id` | `nodes.delete` | Delete node |
| GET | `/api/nodes/:id/features` | `nodes.view` | Node features |
| GET | `/api/nodes/:id/system` | `nodes.view` | System info |
| GET | `/api/nodes/:id/deployment` | `nodes.deploy` | Deployment data |

### `POST /api/nodes`
**Body:**
```json
{
  "name": "Node-02", "publicHost": "node2.example.com", "privateHost": "192.168.1.11",
  "publicPort": 8080, "privatePort": 8080, "sftpPort": 5657
}
```
**Response:** `Node` (includes `id`)

### `GET /api/nodes/:id/deployment`
```json
{ "clientId": ".node_1", "clientSecret": "abc123def456...", "publicKey": "..." }
```

---

## Servers

Most action endpoints use `proxyServerRequest`, which forwards the request to the node's daemon.

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| GET | `/api/servers` | (auth) | List servers |
| GET | `/api/servers/:serverId` | `server.view` | Get server |
| PUT | `/api/servers/:serverId` | `server.create` | Create server |
| DELETE | `/api/servers/:serverId` | `server.delete` | Delete server |
| POST | `/api/servers/:serverId/suspend` | `server.edit.data.admin` | Suspend/activate |
| PUT | `/api/servers/:serverId/name/:name` | `server.name.edit` | Rename |
| GET | `/api/servers/:serverId/definition` | `server.definition.view` | Get definition |
| PUT | `/api/servers/:serverId/definition` | `server.definition.edit` | Edit definition |
| GET | `/api/servers/:serverId/data` | `server.data.view` | Get variables |
| POST | `/api/servers/:serverId/data` | `server.data.edit` | Edit variables |
| PUT | `/api/servers/:serverId/data` | `server.data.edit.admin` | Edit data (admin) |
| POST | `/api/servers/:serverId/transfer` | `server.edit.data.admin` | Transfer to another node |
| GET | `/api/servers/:serverId/status` | `server.status` | Status (running/stopped) |
| GET | `/api/servers/:serverId/stats` | `server.stats` | Statistics |
| GET | `/api/servers/:serverId/console` | `server.console` | Console logs |
| POST | `/api/servers/:serverId/console` | `server.console.send` | Send command |
| GET | `/api/servers/:serverId/flags` | `server.flags.view` | Get flags |
| POST | `/api/servers/:serverId/flags` | `server.flags.edit` | Edit flags |
| POST | `/api/servers/:serverId/start` | `server.start` | Start |
| POST | `/api/servers/:serverId/stop` | `server.stop` | Stop |
| POST | `/api/servers/:serverId/restart` | `server.start`+`server.stop` | Restart |
| POST | `/api/servers/:serverId/kill` | `server.kill` | Kill process |
| POST | `/api/servers/:serverId/install` | `server.install` | Run installation |
| POST | `/api/servers/:serverId/reload` | `server.reload` | Reload configuration |
| HEAD | `/api/servers/:serverId/query` | `server.stats` | Query server |
| GET | `/api/servers/:serverId/query` | `server.stats` | Query server |
| GET | `/api/servers/:serverId/socket` | `server.view` | WebSocket (console/stats) |

### `GET /api/servers`
**Query params:** `name` (filter with `*`), `node` (ID), `username`, `page`, `limit`.

```json
{
  "servers": [ { "identifier": "abc123", "name": "Server", "node": { "id": 1, "name": "Node" }, "ip": "10.0.0.1", "port": 25565, "type": "minecraft-java", "canGetStatus": true } ],
  "metadata": { "paging": { "page": 1, "size": 20, "maxSize": 100, "total": 1 } }
}
```

### `GET /api/servers/:serverId?perms=true`
```json
{
  "server": { "identifier": "abc123", "name": "Server", "icon": "minecraft.png", "node": { "id": 1, "name": "Node" } },
  "perms": { "scopes": [{ "value": "server.view", "forServer": true }] }
}
```

### `PUT /api/servers/:serverId`
Creates a server. **Body:** `ServerDefinition` (see types).
**Response:** `{ "id": "abc123" }`

### `DELETE /api/servers/:serverId?skipNode=true`
`skipNode`: deletes only from the database, not from the node.

### `POST /api/servers/:serverId/suspend`
Suspends or activates the server (toggle).

### `PUT /api/servers/:serverId/name/:name`
Renames the server in the database.

### `GET /api/servers/:serverId/definition`
Returns the server's complete definition.

### `PUT /api/servers/:serverId/definition`
**Body:** `ServerDefinition`.
**Response:** `204`.

### `GET /api/servers/:serverId/data`
```json
{ "data": { "version": { "type": "string", "value": "1.20.1" } }, "groups": [] }
```

### `POST /api/servers/:serverId/data`
Edits server variables. **Body:** `{ "key": "value" }` (flat object).
**Response:** `202`.

### `PUT /api/servers/:serverId/data`
Admin data editing. **Body:** `{ ... }`. **Response:** `202`.

### `POST /api/servers/:serverId/transfer`
**Body:** `{ "nodeId": 2 }`. Transfers the server to another node.
**Response:** `202 "Transfer started"`.

### `GET /api/servers/:serverId/status`
```json
{ "running": true, "installing": false }
```

### `GET /api/servers/:serverId/stats`
```json
{
  "cpu": 45.2, "memory": 1536000000, "maxMemory": 2147483648,
  "running": true, "storage": 5000000000,
  "jvm": { "heapUsed": 1000000000, "heapTotal": 2000000000, "metaspaceUsed": 50000000, "metaspaceTotal": 100000000 }
}
```

### `GET /api/servers/:serverId/console?time=1705312215000`
`time`: epoch in ms to get logs from that moment.
```json
{ "logs": ["[10:30:15] [Server thread/INFO]: Starting server"], "epoch": 1705312215 }
```

### `POST /api/servers/:serverId/console`
**Body:** `"command"` (literal string of the command).
**Response:** `204`.

### `GET /api/servers/:serverId/flags`
```json
{ "autoStart": false, "autoRestartOnCrash": true, "autoRestartOnGraceful": false }
```

### `POST /api/servers/:serverId/flags`
**Body:** `ServerFlags`. **Response:** `204`.

### Lifecycle actions

| Action | Method | Response |
|--------|--------|-----------|
| Start | `POST /api/servers/:serverId/start` | `202` / `204` |
| Stop | `POST /api/servers/:serverId/stop` | `202` / `204` |
| Restart | `POST /api/servers/:serverId/restart` | `202` / `204` |
| Kill | `POST /api/servers/:serverId/kill` | `204` |
| Install | `POST /api/servers/:serverId/install` | `202` / `204` |
| Reload | `POST /api/servers/:serverId/reload` | `204` |

### `HEAD` / `GET /api/servers/:serverId/query`
Queries the game server via the query protocol.

---

## Files

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| GET | `/api/servers/:serverId/file/*filename` | `server.files.view` | List/download file |
| PUT | `/api/servers/:serverId/file/*filename` | `server.files.edit` | Upload file |
| DELETE | `/api/servers/:serverId/file/*filename` | `server.files.edit` | Delete file |
| POST | `/api/servers/:serverId/file/*filename` | `server.files.edit` | Move/copy file |

**List directory:** `GET /api/servers/:serverId/file/`
```json
{
  "files": [
    { "name": "server.jar", "size": 45678901, "modified": "2024-01-15T10:30:00Z", "isFile": true },
    { "name": "world", "size": 0, "modified": "2024-01-15T10:25:00Z", "isFile": false }
  ]
}
```

**Upload:** `PUT /api/servers/:serverId/file/config.yml` with `Content-Type: application/octet-stream`.

---

## Archived Files

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| HEAD | `/api/servers/:serverId/archive/*filename` | `server.files.edit` | Check if it exists |
| POST | `/api/servers/:serverId/archive/*filename` | `server.files.edit` | Create ZIP |
| POST | `/api/servers/:serverId/extract/*filename` | `server.files.edit` | Extract ZIP |

### `POST /api/servers/:serverId/archive/backup.zip`
**Body:** `["file1.txt", "folder/"]` — files to compress.
**Query:** `destination` — destination subdirectory.
**Response:** `204`.

### `POST /api/servers/:serverId/extract/archive.zip`
**Query:** `destination` — directory to extract into (empty = server root).
**Response:** `204`.

---

## Plugins

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/servers/:serverId/plugins` | `server.files.view` |
| DELETE | `/api/servers/:serverId/plugins` | `server.files.edit` |
| GET | `/api/servers/:serverId/plugins/search` | `server.files.view` |
| POST | `/api/servers/:serverId/plugins/:pluginId` | `server.files.edit` |

### `GET /api/servers/:serverId/plugins`
```json
[{ "name": "EssentialsX.jar", "version": "2.20.1", "size": 1234567 }]
```

### `GET /api/servers/:serverId/plugins/search?q=essentials`
```json
[{ "id": "essentialsx", "name": "EssentialsX", "version": "2.20.1", "author": "...", "downloads": 1000000 }]
```

### `DELETE /api/servers/:serverId/plugins?name=EssentialsX.jar`
Deletes the plugin `EssentialsX.jar`.

### `POST /api/servers/:serverId/plugins/:pluginId`
Installs the plugin from SpigotMC (Spigot numeric ID).

---

## Backups

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/servers/:serverId/backup` | `server.backup.view` |
| GET | `/api/servers/:serverId/backup/:backupID` | `server.backup.view` |
| DELETE | `/api/servers/:serverId/backup/:backupID` | `server.backup.delete` |
| POST | `/api/servers/:serverId/backup/create` | `server.backup.create` |
| POST | `/api/servers/:serverId/backup/restore/:backupID` | `server.backup.restore` |
| GET | `/api/servers/:serverId/backup/download/:backupID` | `server.backup.view` |

### `GET /api/servers/:serverId/backup`
```json
[{ "id": 1, "name": "Backup 2024-01-15", "fileName": "backup_abc123.tar.gz", "createdAt": "2024-01-15T10:30:00Z" }]
```

### `POST /api/servers/:serverId/backup/create`
**Response:** `{ "backupFileName": "backup_abc123.tar.gz" }`

---

## Tasks

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/servers/:serverId/tasks` | `server.tasks.view` |
| GET | `/api/servers/:serverId/tasks/:taskId` | `server.tasks.view` |
| PUT | `/api/servers/:serverId/tasks/:taskId` | `server.tasks.edit` |
| DELETE | `/api/servers/:serverId/tasks/:taskId` | `server.tasks.delete` |
| POST | `/api/servers/:serverId/tasks/:taskId/run` | `server.tasks.run` |

### `GET /api/servers/:serverId/tasks`
```json
{
  "tasks": {
    "backup_task": { "name": "Daily backup", "cronSchedule": "0 */6 * * *", "operations": [{ "type": "backup" }] }
  }
}
```

### `PUT /api/servers/:serverId/tasks/:taskId`
**Body:** `ServerTask`. **Response:** `204`.

---

## Server Databases

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/servers/:serverId/databases` | `server.view` |
| POST | `/api/servers/:serverId/databases` | `server.data.edit` |
| DELETE | `/api/servers/:serverId/databases/:id` | `server.data.edit` |

### `POST /api/servers/:serverId/databases`
**Body:** `{ "database_host_id": 1, "database_name": "my_db" }`
**Response:** `DatabaseView` (with generated username/password).

---

## Server Users

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/servers/:serverId/user` | `server.users.view` |
| GET | `/api/servers/:serverId/user/:email` | `server.users.view` |
| PUT | `/api/servers/:serverId/user/:email` | `server.users.edit` |
| DELETE | `/api/servers/:serverId/user/:email` | `server.users.delete` |

### `GET /api/servers/:serverId/user`
```json
[{ "username": "admin", "email": "admin@example.com", "scopes": ["server.view", "server.console"] }]
```

### `PUT /api/servers/:serverId/user/:email`
**Body:** `{ "permissions": { "scopes": ["server.view", "server.console"] } }`
**Response:** `204`.

---

## Transfer Between Nodes

| Method | Path | Scope |
|--------|------|-------|
| POST | `/api/servers/:serverId/transfer` | `server.edit.data.admin` |

**Body:** `{ "nodeId": 2 }`

---

## External Transfer

Public endpoints to migrate servers between panels (no authentication).

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/extransfer/validate` | Validate transfer token |
| POST | `/api/extransfer/consume` | Consume transfer |
| POST | `/api/extransfer/heartbeat` | Heartbeat during transfer |
| POST | `/api/extransfer/confirm` | Confirm transfer |
| GET | `/api/extransfer/download` | Download transfer data |
| POST | `/api/extransfer/cancel` | Cancel transfer |

Also from the server:

| Method | Path | Scope |
|--------|------|-------|
| POST | `/api/servers/:serverId/extransfer/create` | `server.edit.data.admin` |
| POST | `/api/servers/:serverId/extransfer/pull` | `server.edit.data.admin` |
| GET | `/api/servers/:serverId/extransfer/status` | `server.edit.data.admin` |

---

## AI

| Method | Path | Scope |
|--------|------|-------|
| POST | `/api/ai/analyze` | `—` (authenticated) |
| POST | `/api/servers/:serverId/ai/analyze` | `server.console` |

Analyzes server logs using Google GenAI (requires `geminiApiKey` configured).

### `POST /api/ai/analyze`
```json
// Request
{ "logs": ["[ERROR] Connection refused", "[WARN] Memory low"] }

// Response
{
  "summary": "Analysis summary...",
  "rootCauses": ["Root cause 1"],
  "suggestions": ["Suggestion 1", "Suggestion 2"]
}
```

---

## Global Users

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| GET | `/api/users` | `users.info.search` | Search users |
| POST | `/api/users` | `users.info.edit` | Create user |
| GET | `/api/users/:id` | `users.info.view` | Get user |
| POST | `/api/users/:id` | `users.info.edit` | Update user |
| DELETE | `/api/users/:id` | `users.info.edit` | Delete user |
| GET | `/api/users/:id/perms` | `users.perms.view` | Get permissions |
| PUT | `/api/users/:id/perms` | `users.perms.edit` | Update permissions |

### `GET /api/users?username=admin*&email=*@example.com&page=1&limit=25`
```json
{
  "users": [{ "id": 1, "username": "admin", "email": "admin@example.com" }],
  "metadata": { "paging": { "page": 1, "size": 20, "maxSize": 100, "total": 1 } }
}
```

### `POST /api/users`
**Body:** `{ "username": "newuser", "email": "new@example.com", "password": "Secure123!" }`

### `POST /api/users/:id`
**Body:** `{ "username": "newname", "email": "new@example.com", "password": "newpass" }`

### `GET /api/users/:id/perms`
```json
{ "serverIdentifier": "", "scopes": [{ "value": "server.view", "forServer": false }, { "value": "admin", "forServer": false }] }
```

### `PUT /api/users/:id/perms`
**Body:** `{ "scopes": ["admin", "server.view"] }` (array of strings).
**Response:** `204`.

---

## Self (Own Profile)

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| GET | `/api/self` | `login` | Get own profile |
| PUT | `/api/self` | `self.edit` | Update profile |
| GET | `/api/self/otp` | `self.edit` | OTP status |
| POST | `/api/self/otp` | `self.edit` | Start OTP enrollment |
| PUT | `/api/self/otp` | `self.edit` | Validate enrollment |
| POST | `/api/self/otp/recovery` | `self.edit` | Regenerate recovery codes |
| DELETE | `/api/self/otp/:token` | `self.edit` | Disable OTP |
| GET | `/api/self/oauth2` | `self.clients` | List OAuth2 clients |
| POST | `/api/self/oauth2` | `self.clients` | Create client |
| DELETE | `/api/self/oauth2/:clientID` | `self.clients` | Delete client |

---

## Settings

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| GET | `/api/settings` | `settings.edit` | Get configuration |
| POST | `/api/settings` | `settings.edit` | Update multiple values |
| GET | `/api/settings/:key` | `settings.edit` | Get a value |
| PUT | `/api/settings/:key` | `settings.edit` | Update a value |
| POST | `/api/settings/test/email` | `settings.edit` | Send test email |
| POST | `/api/settings/test/discord` | `settings.edit` | Send test Discord notification |
| POST | `/api/settings/license/activate` | `settings.edit` | Activate license |

### `POST /api/settings`
**Body:** `{ "companyName": "My Company", "registrationEnabled": false }`. **Response:** `204`.

### `PUT /api/settings/:key`
**Body:** `{ "value": "new_value" }`. **Response:** `204`.

---

## User Settings

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/userSettings` | `login` |
| PUT | `/api/userSettings/:key` | `login` |

### `PUT /api/userSettings/theme`
**Body:** `{ "value": "dark" }`. **Response:** `204`.

---

## API Keys

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/settings/apikeys` | `admin` |
| POST | `/api/settings/apikeys` | `admin` |
| DELETE | `/api/settings/apikeys/:id` | `admin` |

---

## Roles

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/roles` | `admin` or `users.info.view/edit` |
| POST | `/api/roles` | `admin` |
| GET | `/api/roles/:id` | `admin` |
| POST | `/api/roles/:id` | `admin` |
| DELETE | `/api/roles/:id` | `admin` |

### `POST /api/roles`
**Body:** `{ "name": "Moderator", "description": "Can manage servers", "scopes": ["server.view", "server.start"] }`

---

## Database Hosts

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/databasehosts` | `admin` |
| POST | `/api/databasehosts` | `admin` |
| GET | `/api/databasehosts/:id` | `admin` |
| PUT | `/api/databasehosts/:id` | `admin` |
| DELETE | `/api/databasehosts/:id` | `admin` |

### `POST /api/databasehosts`
**Body:**
```json
{
  "name": "DB Host 1", "host": "db1.example.com", "port": 3306,
  "username": "skypanel", "password": "secret", "max_databases": 10, "node_id": 1
}
```

---

## Templates

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/templates` | `login` |
| POST | `/api/templates` | `templates.repo.create` |
| GET | `/api/templates/:repo` | `login` |
| DELETE | `/api/templates/:repo` | `templates.repo.delete` |
| GET | `/api/templates/:repo/:name` | `login` |
| PUT | `/api/templates/0/:name` | `templates.local.edit` |
| DELETE | `/api/templates/0/:name` | `templates.local.edit` |

`:repo=0` is the local repository. Remote repositories have IDs > 0.

---

## Provision Products

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/provision/products` | `admin` |
| POST | `/api/provision/products` | `admin` |
| PUT | `/api/provision/products/:id` | `admin` |
| DELETE | `/api/provision/products/:id` | `admin` |

---

## Provision API v1

API Key authentication in header. Endpoints for integration with external systems (WHMCS, etc.).

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/ping` | Check connectivity |
| POST | `/api/v1/provision` | Automatically create server |
| POST | `/api/v1/terminate` | Terminate server |
| POST | `/api/v1/suspend` | Suspend server |
| POST | `/api/v1/unsuspend` | Reactivate server |

---

## Uptime

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| GET | `/api/uptime` | `admin` or `uptime.view` | All records |
| GET | `/api/uptime/:serverId` | `server.view` | Records of a server |

**Query params:** `days` (days back), `limit` (number of records).

---

## Daemon

Daemon endpoints for direct communication between nodes and panel. They do not go through a proxy. They use JWT authentication.

| Method | Path | Swagger Scope |
|--------|------|---------------|
| GET | `/daemon` | `none` |
| HEAD | `/daemon` | `none` |
| GET | `/daemon/features` | `none` |
| GET | `/daemon/system` | `none` |
| GET/PUT/DELETE | `/daemon/server/:serverId/...` | (depending on action) |

### Daemon Actions per Server

| Method | Path | Swagger Scope |
|--------|------|---------------|
| PUT | `/daemon/server/:serverId` | — |
| DELETE | `/daemon/server/:serverId` | — |
| GET | `/daemon/server/:serverId/definition` | `server.definition.view` |
| PUT | `/daemon/server/:serverId/definition` | `server.definition.edit` |
| GET | `/daemon/server/:serverId/data` | `server.data.view` |
| POST | `/daemon/server/:serverId/data` | — |
| PUT | `/daemon/server/:serverId/data` | — |
| GET | `/daemon/server/:serverId/tasks` | `server.tasks.view` |
| GET/PUT/DELETE | `/daemon/server/:serverId/tasks/:taskID` | `server.tasks.*` |
| POST | `/daemon/server/:serverId/tasks/:taskID/run` | `server.tasks.run` |
| POST | `/daemon/server/:serverId/reload` | `server.reload` |
| POST | `/daemon/server/:serverId/start` | `server.start` |
| POST | `/daemon/server/:serverId/restart` | `server.start` + `server.stop` |
| POST | `/daemon/server/:serverId/stop` | `server.stop` |
| POST | `/daemon/server/:serverId/kill` | `server.kill` |
| POST | `/daemon/server/:serverId/install` | `server.install` |
| GET | `/daemon/server/:serverId/file/*filename` | `server.files.view` |
| PUT | `/daemon/server/:serverId/file/*filename` | `server.files.edit` |
| DELETE | `/daemon/server/:serverId/file/*filename` | `server.files.edit` |
| GET | `/daemon/server/:serverId/console` | `server.console` |
| POST | `/daemon/server/:serverId/console` | `server.console.send` |
| GET | `/daemon/server/:serverId/flags` | `server.flags.view` |
| POST | `/daemon/server/:serverId/flags` | `server.flags.edit` |
| GET | `/daemon/server/:serverId/stats` | `server.stats` |
| GET | `/daemon/server/:serverId/status` | `server.status` |
| POST | `/daemon/server/:serverId/archive/*filename` | `server.files.edit` |
| POST | `/daemon/server/:serverId/extract/*filename` | `server.files.edit` |
| POST | `/daemon/server/:serverId/backup/create` | `server.backup.create` |
| DELETE | `/daemon/server/:serverId/backup` | `server.backup.delete` |
| POST | `/daemon/server/:serverId/backup/restore` | `server.backup.restore` |
| GET | `/daemon/server/:serverId/backup/download` | `server.backup.restore` |
| HEAD/GET | `/daemon/server/:serverId/query` | `server.stats` |
| GET | `/daemon/server/:serverId/plugins` | — |
| DELETE | `/daemon/server/:serverId/plugins` | — |
| GET | `/daemon/server/:serverId/plugins/search` | — |
| POST | `/daemon/server/:serverId/plugins/:pluginId` | — |
| GET | `/daemon/server/:serverId/socket` | — |

---

## WebSocket

### `GET /api/servers/:serverId/socket`

Connects to the console and real-time statistics.

```javascript
const ws = new WebSocket(`ws://localhost:8080/api/servers/${serverId}/socket`);
```

Authentication is done via the session cookie (`skypanel_auth`); no token is required in the URL.

### Message Types

| Type | Direction | Description |
|------|-----------|-------------|
| `console` | Server → Client | Console line (struct `ServerLogs`) |
| `stat` | Server → Client | Periodic statistics (struct `ServerStats`) |
| `status` | Server → Client | Status change (struct `ServerRunning`) |
| `console` | Client → Server | Send command (data = string) |

### Server Events
```json
{ "type": "console", "data": { "epoch": 1712345678000, "logs": "[10:30:15] [Server thread/INFO]: Starting server" } }
{ "type": "stat", "data": { "cpu": 45.2, "memory": 1536000000, "maxMemory": 2147483648, "storage": 5000000000, "maxStorage": 10737418240, "networkRx": 0, "networkTx": 0, "running": true } }
{ "type": "status", "data": { "running": true, "installing": false } }
```

### Send Command
```json
{ "type": "console", "data": "say Hello World!" }
```

---

## Examples

### cURL
```bash
TOKEN=$(curl -s -X POST http://localhost:8080/oauth2/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=ID" \
  -d "client_secret=SECRET" | jq -r '.access_token')

curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/servers
curl -s -X POST -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/servers/abc123/start
```

### Python
```python
import requests
api = "http://localhost:8080"
r = requests.post(f"{api}/oauth2/token", data={"grant_type": "client_credentials", "client_id": "ID", "client_secret": "SECRET"})
token = r.json()["access_token"]
headers = {"Authorization": f"Bearer {token}"}
servers = requests.get(f"{api}/api/servers", headers=headers).json()
```

### JavaScript
```javascript
const api = axios.create({ baseURL: 'http://localhost:8080' });
const { data } = await api.post('/oauth2/token', new URLSearchParams({ grant_type: 'client_credentials', client_id: 'ID', client_secret: 'SECRET' }));
api.defaults.headers.Authorization = `Bearer ${data.access_token}`;
const servers = (await api.get('/api/servers')).data;
```
