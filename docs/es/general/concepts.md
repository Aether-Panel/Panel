# Core Concepts

Aether Panel is a server management platform with a two-component architecture: the Panel (REST API + web frontend) and the Daemon (agent that runs the servers). This page explains the fundamental concepts to understand how the system works.

## Architecture: Panel & Daemon

The system consists of two processes that can run together or separately:

- **panel**: Panel — REST API server built with Gin and GORM. Manages users, nodes, servers, templates, and permissions. Serves the web frontend (Astro + React) and exposes the public API on port 8080.
- **daemon**: Daemon — Agent that runs on each machine where application servers run. Manages the process lifecycle (start, stop, restart), the SFTP file system, WebSocket console, and Docker container execution. Runs on port 8080 with SFTP on port 5657.
- **comms**: Communication — If the node is local, the Panel calls the Daemon directly through the Gin router without going over the network (httptest.ResponseRecorder). If the node is remote, the Panel communicates over HTTP authenticated with Ed25519 JWT.
## Nodes

A node represents a physical or virtual machine where application servers run.

- **local**: Local Node — It is the same machine where the Panel runs. Identified by ID 0 and uses the IPs configured in the MasterUrl variable. Its secret is auto-generated as a UUID.
- **remote**: Remote Node — Any other machine registered in the Panel. On registration it receives a client_id (format .node_{ID}) and a client_secret used to authenticate via OAuth2.
- **deploy**: Deployment — The GET /api/nodes/:id/deployment endpoint returns the credentials needed to configure a remote node.
- **features**: Features — Each node reports its capabilities (Docker, filesystems, OS, architecture) via GET /api/nodes/:id/features.
## Servers

A server is an application instance (Minecraft, Discord bot, web, etc.) running on a node.

- **definition**: Definition — Each server has a JSON definition (server.json) that includes configuration variables, execution commands, install/uninstall files, environment variables, requirements, and file groups.
- **lifecycle**: Lifecycle — Create  Install (download, extract assets)  Start (pre-commands + main command execution)  Stop / Kill  Uninstall  Destroy. Each stage runs operations defined in the template.
- **environments**: Execution Environments — TTY (direct process on the host using PTY) for standard/host/tty server types, and Docker (isolated container) for docker server types. The environment is selected based on the server type.
- **scheduler**: Scheduler — Each server has a gocron-based scheduler that runs scheduled tasks (cron jobs) such as automatic backups, restarts, etc.
- **stats**: Statistics — The Daemon collects real-time metrics (CPU, RAM, disk, network) that are sent to WebSocket clients.
- **backups**: Backups — The server can create, restore, and delete compressed backups of its file directory.
## Users & Authentication

The authentication system supports multiple mechanisms for different use cases:

- **sessions**: Sessions — On login (POST /auth/login with email + password), a UUID token is generated, stored as a SHA-256 hash in the database with a 1-hour expiry. The token is returned as a skypanel_auth cookie and in the JSON body.
- **bearer**: Bearer Token — The AuthMiddleware checks the Authorization: Bearer <token> header first, falling back to the skypanel_auth cookie.
- **jwt**: Ed25519 JWT — The Daemon uses Ed25519-signed JWT tokens to authenticate Panel requests. The public key is exposed at GET /auth/publickey in JWKS format.
- **oauth2**: OAuth2 — Endpoint /oauth2/token supporting client_credentials (for nodepanel authentication) and password (for SFTP authentication). Tokens are valid for 1 hour.
- **twofactor**: 2FA — The panel supports two-factor authentication (OTP) as an additional security layer on login.
## Permissions & Scopes

The permission system is based on granular scopes (74 in total: 50 server-specific + 24 global) that control every possible action in the panel.

- **scopes**: Scopes — Each permission is a string like server.start, nodes.view, users.edit, etc. Scopes can be global or server-specific (ForServer: true).
- **admin**: Hierarchy — The admin scope grants all permissions. At the server level, server.admin grants all scopes for that server.
- **roles**: Roles — Roles group multiple scopes under a name (e.g. "Admin", "Moderator") and are assigned to users to simplify permission management.
- **checking**: Verification — The RequiresPermission and RequiresAnyPermission middleware load the user's permissions (global + server-specific + role) and check whether they contain the required scope.
- **serverscopes**: Server scope examples — server.start, server.stop, server.kill, server.console, server.files.view, server.files.edit, server.sftp, server.backup.create, server.stats, server.status, server.users.view, server.users.edit.
## SFTP

The Daemon includes an integrated SFTP server for server file access.

- **port**: Port — Runs on port 5657 by default (configurable via daemon.sftp.host), running as a standalone SSH server using the github.com/pkg/sftp package and an Ed25519 host key.
- **auth**: Authentication — The username format is email#serverId. When the Panel is enabled, it validates against the database (DatabaseSFTPAuthorization). When the Daemon is standalone, it calls the /oauth2/token endpoint with grant_type=password (WebSSHAuthorization).
- **isolation**: Isolation — Each SFTP connection is isolated to the specific server's directory. Files from other servers or from the system cannot be accessed.
## WebSocket Console

The Daemon provides a real-time console via WebSocket at GET /daemon/server/:serverId/socket.

- **streams**: Three streaming channels — console (live process output), stat (CPU, RAM, disk, network at regular intervals), status (server state: online/offline/installing).
- **tracker**: Tracker — Each Environment has three Trackers (ConsoleTracker, StatsTracker, StatusTracker) that register Sockets and broadcast JSON messages in {message, type} format.
- **proxy**: Proxy — The Panel proxies WebSocket connections: if the node is local, it rewrites the URL internally; if remote, it creates a bidirectional proxy.
## Templates

Templates are JSON blueprints that define how to deploy and run a server.

- **structure**: Structure — A template contains configuration variables, install/uninstall commands, execution configuration, environment variables, system requirements, environment type (standard, docker, tty, host), and file groups.
- **storage**: Storage — Templates are stored in the database. Repository 0 is local. Remote repositories (git URLs) can be added and synced via SyncRepo() by cloning the repo and parsing the JSONs.
- **usage**: Usage — When creating a server, a template is selected as a blueprint and specific variables can be overridden.
## Database Hosts

Database Hosts allow managing external MySQL databases for the servers.

- **model**: Model — Each Database Host has a name, host, port (default 3306), username and password, maximum database limit, and an optional associated node.
- **api**: Full CRUD API at /api/databasehosts to create, read, update, and delete database hosts.
## External Transfer (Federated Transfer)

Allows migrating servers between independent Aether Panel instances (cross-panel).

- **protocol**: Protocol — Uses Ed25519 to sign requests, HMAC-SHA256 to hash tokens with the AETHER_FEDERATED_SALT_v1 salt, nonces for challenge/response, and sessions expire after 15 minutes.
- **states**: Transfer states — CREATED  VALIDATED  MIGRATING  CONSUMED / COMPLETED / FAILED / CANCELLED.
- **endpoints**: Endpoints — /api/servers/:id/extransfer/create (origin), /api/extransfer/validate (destination), /api/extransfer/consume, /api/extransfer/heartbeat, /api/extransfer/confirm, /api/extransfer/cancel, /api/servers/:id/extransfer/pull.
## CLI (Command Line Interface)

The Aether Panel binary includes a Cobra-based CLI with the following commands:

- **run**: run — Starts the Panel and/or Daemon according to configuration. Hidden command not shown in help.
- **version**: version — Shows the panel version.
- **user**: user add / user edit — Manages users from terminal. Allows creating users with name, email, password, and admin option, and editing existing users (change email, password, admin, remove 2FA).
- **db**: db upgrade / db migrate — Manages the database (`upgrade` runs schema migrations; `migrate` is an experimental stub).
- **runservice**: runService — Same as run but with systemd notify support (NOTIFY_SOCKET).

## Port Management

Aether Panel allows assigning multiple ports to a server, not just the primary port. This is critical for servers requiring additional ports (e.g., Minecraft + plugins using separate ports for query, RCON, voice, etc.).

### Key Features

- **Primary Port**: The first port in the list is the "primary port" used by the panel for default connections.
- **Extra Ports**: Additional ports (port2, port3, etc.) can be assigned and are automatically bound to the Docker container.
- **Port Notes**: Each port can have a custom label/description to identify its purpose (e.g., "RCON", "Voice Chat", "Query").
- **Primary Selection**: Users can choose which port is primary by reordering the list.

### Technical Flow

1. Panel stores ordered list in `server.Ports` (array) and primary in `server.Port`.
2. On save (`PUT /api/servers/:id/data`), panel sends list to Daemon which converts to `port`, `port2`, `port3`... variables.
3. Daemon creates container with `ExtraPortBindings()` binding all extra ports (TCP and UDP).
4. `PUT /api/servers/:id/port-settings` manages only metadata (notes, primary) without touching the full list.
5. Daemon sync is automatic: changes in `port-settings` also propagate the port list.

### Permissions

| Action | Required Scope |
|--------|----------------|
| View assigned ports | `server.data.view` |
| Edit full list (admin) | `server.data.edit.admin` |
| Change primary / notes (user) | `server.data.view` (via `port-settings`) |

> **Important**: Extra ports only work in Docker environment. In TTY/Host environment, port binding is the server process responsibility.

## Docker Network: Auto-Detection `skypanel-network`

For Docker container servers to connect to MySQL databases and other internal services, Aether Panel automatically detects the Panel's Docker network and connects all servers to that same network.

### Problem Solved

Previously, server containers were created in Docker's default `bridge` network, where they couldn't resolve hostnames like `mysql` (MySQL service runs in `skypanel-network`). Typical error:
```
Communications link failure... database address 'mysql:3306' accessible
```

### Implemented Solution

1. **Auto-detection**: On startup, Panel inspects its own container via Docker API and detects its network (e.g., `panel_skypanel-network`).
2. **Automatic default**: If a template doesn't specify `networkName`, the detected network is used.
3. **Result**: All servers are created in `skypanel-network` and can resolve `mysql:3306` directly.

### Configuration

No additional configuration needed. Works automatically with `docker compose up`. For native installation (without Docker), network doesn't apply and host IP is used.

> **Note**: Servers created before this change must be recreated to join the correct network.

## Updated Roles and Permissions (Migration `20260821-fix-usuario-role-scopes`)

Fixed scope assignment for "Usuario" role to remove improper admin access and grant correct definition/flags permissions.

### Changes

| Action | Before | After |
|--------|--------|-------|
| `server.admin.config.view` | ✅ Granted | ❌ Revoked |
| `server.admin.config.manage` | ✅ Granted | ❌ Revoked |
| `server.data.edit.admin` | ✅ Granted | ❌ Revoked |
| `server.definition.view` | ❌ Missing | ✅ Granted |
| `server.definition.edit` | ❌ Missing | ✅ Granted |
| `server.flags.view` | ❌ Missing | ✅ Granted |
| `server.flags.edit` | ❌ Missing | ✅ Granted |

### Result

- **Admin**: Full access to everything (config, limits, metadata, admin tab, ports CRUD).
- **User**: View/edit Groups, Variables, Plugins, Auto-start, Ports (view + primary + notes, no number CRUD), **NO** Resource Limits, Metadata, Admin tab.

## Monaco Editor — Pin to v0.44.0

`monaco-editor` 0.56+ introduced `package.json` `exports` restrictions preventing CSS imports via `?inline` with Vite 8/Rolldown. Version `^0.44.0` is pinned until upstream fix.

```json
"monaco-editor": "^0.44.0"
```

CSS is injected inline and codicon font served from jsDelivr CDN pointing to version 0.44.0.

---

## Multi-Node Deployment

Aether Panel supports managing servers across multiple physical/virtual machines (nodes).

### Architecture

- **Local Node (ID 0)**: Runs the Panel + Daemon in the same process. Uses direct Gin router calls instead of HTTP.
- **Remote Nodes**: Separate machines running only the Daemon. Registered in Panel DB with `.node_{ID}` client ID + secret.

### Communication Flow

```
Panel (local) ──HTTP+JWT──▶ Remote Daemon
                     ▲
                     │
              WebSocket Proxy
```

- **Panel → Daemon**: HTTP with Ed25519 JWT (generated by `TokenService`).
- **Daemon → Panel**: WebSocket proxy for console/stats/status (bridged by Panel).
- **Authentication**: OAuth2 `client_credentials` flow for initial token, then JWT for all `/daemon/*` calls.

### Remote Daemon Configuration

```json
{
  "daemon": {
    "enable": true,
    "auth": {
      "url": "http://master-panel:8080",
      "clientId": ".node_1",
      "clientSecret": "remote-node-secret"
    },
    "token": {
      "public": "http://master-panel:8080/auth/publickey"
    },
    "sftp": {
      "host": "0.0.0.0:5657"
    }
  },
  "web": {
    "host": "0.0.0.0:8080"
  }
}
```

- `daemon.auth.url`: Master panel URL for OAuth2 token endpoint.
- `daemon.auth.clientId`: `.node_{ID}` format.
- `daemon.auth.clientSecret`: Shared secret from deployment endpoint.
- `daemon.token.public`: Master panel JWKS endpoint for JWT validation.

### Deployment Data

`GET /api/nodes/:id/deployment` returns:
```json
{
  "clientId": ".node_1",
  "clientSecret": "random_36_char_string",
  "publicKey": "ed25519_public_key_base64"
}
```

### WebSocket Proxy

For remote nodes, the Panel bridges client WebSocket ↔ Daemon WebSocket:
1. Client connects to `ws://panel/api/servers/:id/socket`
2. Panel dials `ws://remote-daemon:8080/daemon/server/:id/socket`
3. Injects `Authorization: Bearer <JWT>` header
4. Bidirectional copy with ping/pong keepalive

### Docker Network Auto-Detection

`detectPanelNetwork()` in `internal/servers/docker/docker.go`:
- Inspects Panel container via Docker API at startup
- Finds network name (e.g., `panel_skypanel-network`)
- Uses as default for server containers (if template doesn't specify `networkName`)
- Enables `mysql:3306` resolution from server containers

### Local Node Detection

`internal/models/node.go:51-75` - `IsLocal()` fallback chain:
1. ID == 0 (legacy)
2. MasterURL hostname matches node publicHost
3. Node IP matches panel's detected public IP
4. Node IP matches any local interface IP

### Panel Update Propagation

`POST /api/settings/update-panel` triggers update on all connected nodes via their daemon endpoints.
