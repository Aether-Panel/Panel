# Core Concepts

Aether Panel is a server management platform with a two-component architecture: the Panel (REST API + web frontend) and the Daemon (agent that runs game servers). This page explains the fundamental concepts to understand how the system works.

## Architecture: Panel & Daemon

The system consists of two processes that can run together or separately:

- **panel**: Panel — REST API server built with Gin and GORM. Manages users, nodes, servers, templates, and permissions. Serves the web frontend (Next.js + React) and exposes the public API on port 8080.
- **daemon**: Daemon — Agent that runs on each machine hosting application servers. Manages the process lifecycle (start, stop, restart), the SFTP file system, WebSocket console, and Docker container execution. Runs on port 8080 with SFTP on port 5657.
- **comms**: Communication — If the node is local, the Panel calls the Daemon directly through the Gin router without network (httptest.ResponseRecorder). If remote, communication happens over HTTP authenticated with Ed25519 JWT.
## Nodes

A node represents a physical or virtual machine where application servers run.

- **local**: Local Node — The same machine running the Panel. Identified by ID 0 and uses IPs from the MasterUrl config. Its secret is auto-generated as a UUID.
- **remote**: Remote Node — Any other machine registered in the Panel. On registration it receives a client_id (format .node_{ID}) and client_secret used for OAuth2 authentication.
- **deploy**: Deployment — The endpoint GET /api/nodes/:id/deployment returns the credentials needed to configure a remote node.
- **features**: Features — Each node reports its capabilities (Docker, filesystems, OS, architecture) via GET /api/nodes/:id/features.
## Servers

A server is an application instance (Minecraft, Discord bot, web app, etc.) running on a node.

- **definition**: Definition — Each server has a JSON definition (server.json) including configuration variables, execution commands, install/uninstall files, environment variables, requirements, and file groups.
- **lifecycle**: Lifecycle — Create  Install (download, extract assets)  Start (pre-commands + main command execution)  Stop / Kill  Uninstall  Destroy. Each stage runs operations defined in the template.
- **environments**: Execution Environments — TTY (direct host process using PTY) for standard/host/tty server types, and Docker (isolated container) for docker server types. The environment is selected based on the server type.
- **scheduler**: Scheduler — Each server has a gocron-based scheduler that runs cron jobs such as automatic backups, restarts, etc.
- **stats**: Statistics — The Daemon collects real-time metrics (CPU, RAM, disk, network) sent to WebSocket clients.
- **backups**: Backups — The server can create, restore, and delete compressed backups of its file directory.
## Users & Authentication

The authentication system supports multiple mechanisms for different use cases:

- **sessions**: Sessions — On login (POST /auth/login with email + password), a UUID token is generated, stored as a SHA-256 hash in the database with a 1-hour expiry. The token is returned as a skypanel_auth cookie and in the JSON body.
- **bearer**: Bearer Token — The AuthMiddleware checks the Authorization: Bearer <token> header first, falling back to the skypanel_auth cookie.
- **jwt**: Ed25519 JWT — The Daemon uses Ed25519-signed JWT tokens to authenticate Panel requests. The public key is exposed at GET /auth/publickey in JWKS format.
- **oauth2**: OAuth2 — Endpoint /oauth2/token supporting client_credentials (for nodepanel auth) and password (for SFTP auth). Tokens are valid for 1 hour.
- **twofactor**: 2FA — The panel supports two-factor authentication (OTP) as an additional security layer on login.
## Permissions & Scopes

The permission system is based on granular scopes (~60 total) controlling every possible action in the panel.

- **scopes**: Scopes — Each permission is a string like server.start, nodes.view, users.edit, etc. Scopes can be global or server-specific (ForServer: true).
- **admin**: Hierarchy — The admin scope grants all permissions. At the server level, server.admin grants all scopes for that specific server.
- **roles**: Roles — Roles group multiple scopes under a name (e.g. "Admin", "Moderator") and are assigned to users to simplify permission management.
- **checking**: Verification — The RequiresPermission and RequiresAnyPermission middleware load the user's permissions (global + server-specific + role) and check if they contain the required scope.
- **serverscopes**: Server scope examples — server.start, server.stop, server.kill, server.console, server.files.view, server.files.edit, server.sftp, server.backup.create, server.stats, server.status, server.users.view, server.users.edit.
## SFTP

The Daemon includes an integrated SFTP server for server file access.

- **port**: Port — Runs on port 5657 by default (configurable via daemon.sftp.host), running as a standalone SSH server using github.com/pkg/sftp with an Ed25519 host key.
- **auth**: Authentication — Username format is email#serverId. When the Panel is enabled, it validates against the database (DatabaseSFTPAuthorization). When the Daemon is standalone, it calls /oauth2/token with grant_type=password (WebSSHAuthorization).
- **isolation**: Isolation — Each SFTP connection is isolated to the specific server's directory. Access to other servers' files or the system is blocked.
## WebSocket Console

The Daemon provides a real-time console via WebSocket at GET /daemon/:serverId/socket.

- **streams**: Three streaming channels — console (live process output), stats (CPU, RAM, disk, network at regular intervals), status (server state: online/offline/installing).
- **tracker**: Tracker — Each Environment has three Trackers (ConsoleTracker, StatsTracker, StatusTracker) that register Sockets and broadcast JSON messages in {message, type} format.
- **proxy**: Proxy — The Panel proxies WebSocket connections: if the node is local, it rewrites the URL internally; if remote, it creates a bidirectional proxy.
## Templates

Templates are JSON blueprints defining how to deploy and run a server.

- **structure**: Structure — A template contains configuration variables, install/uninstall commands, execution configuration, environment variables, system requirements, environment type (standard, docker, tty, host), and file groups.
- **storage**: Storage — Templates are stored in the database. Repository 0 is local. Remote repositories (git URLs) can be added and synced via SyncRepo() by cloning the repo and parsing JSON files.
- **usage**: Usage — When creating a server, a template is selected as a blueprint and specific variables can be overridden.
## Database Hosts

Database Hosts allow managing external MySQL databases for servers.

- **model**: Model — Each Database Host has a name, host, port (default 3306), username and password, maximum database limit, and optional node association.
- **api**: Full CRUD API at /api/databasehosts for creating, reading, updating, and deleting database hosts.
## External Transfer (Federated)

Enables migrating servers between independent Aether Panel instances (cross-panel).

- **protocol**: Protocol — Uses Ed25519 for signing requests, HMAC-SHA256 for token hashing with the AETHER_FEDERATED_SALT_v1 salt, nonces for challenge/response, and sessions expire after 15 minutes.
- **states**: Transfer states — CREATED  VALIDATED  MIGRATING  CONSUMED / COMPLETED / FAILED / CANCELLED.
- **endpoints**: Endpoints — /api/servers/:id/extransfer/create (origin), /api/extransfer/validate (destination), /api/extransfer/consume, /api/extransfer/heartbeat, /api/extransfer/confirm, /api/extransfer/cancel, /api/servers/:id/extransfer/pull.
## CLI (Command Line Interface)

The Aether Panel binary includes a Cobra-based CLI with the following commands:

- **run**: run — Starts the Panel and/or Daemon based on configuration. Hidden command not shown in help.
- **version**: version — Displays the panel version.
- **user**: user add / user edit — Manages users from the terminal. Create users with name, email, password, and admin option; edit existing users (change email, password, admin status, remove 2FA).
- **db**: db upgrade / db migrate — Manages the database (`upgrade` runs schema migrations; `migrate` is an experimental stub).
- **runservice**: runService — Same as run but with systemd notify support (NOTIFY_SOCKET).
