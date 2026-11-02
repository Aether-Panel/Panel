# Core Concepts

This page explains the pieces Aether Panel is made of and how they fit together: nodes, servers, templates, sub-servers, users and permissions, SFTP, database hosts, transfers and the Docker networks. Each section links to the page with the details.

## Panel and daemon

Aether Panel is a single program (the `SkyPanel` binary) with two roles that can run together or apart:

| Role | What it does | Turned on by |
|------|--------------|--------------|
| **Panel** | The web interface (Astro + React) and the REST API (`/api`, `/auth`, `/oauth2`). Keeps users, permissions, nodes, servers and templates in the database. | `panel.enable` (`true`) |
| **Daemon** | Runs the game servers on its machine: Docker containers, files, console, statistics, backups, scheduled tasks and the SFTP server. Its API is under `/daemon`. | `daemon.enable` (`true`) |

Both listen on the same HTTP address (`web.host`, `0.0.0.0:8080`); SFTP listens on `daemon.sftp.host` (`0.0.0.0:5657`).

The official install runs in Docker: `docker-compose.yml` starts the `skypanel` container (panel and daemon), `skypanel-mysql` (MariaDB 10.11, the panel's database) and `skypanel-adminer` (Adminer, on `127.0.0.1:8081` only). The daemon creates the game server containers through the host's Docker socket. See the [Installation guide](../setup/01-installation.md).

The panel's database can be SQLite (`panel.database.dialect: sqlite3`, the binary's default), MySQL/MariaDB (`mysql`, what the Docker install uses) or PostgreSQL (`postgresql`).

## Nodes

A **node** is a machine that runs game servers, that is, a machine with a daemon.

- **Local node**: the daemon inside the panel's own container. It is always there (ID `0`, named `LocalNode`), is not stored in the database and cannot be edited. Its address is taken from the panel's Base URL (`panel.settings.masterUrl`) or from the server's IP when the domain is behind Cloudflare's proxy. When the panel talks to it, the call is served in-process, without going over the network.
- **Remote nodes**: other machines running the same image with the panel turned off (`SKYPANEL_PANEL_ENABLE=false`) and `token.public` pointing at the panel's `/auth/publickey`. An administrator registers them in **Nodes** (`nodes.create`) with a public and private host, the daemon port (8080) and the SFTP port (5657). `GET /api/nodes/:id/deployment` (`nodes.deploy`) gives the credentials the node uses towards the panel: client ID `.node_<id>` and its secret, which go in `daemon.auth.clientId` and `daemon.auth.clientSecret` (with `daemon.auth.url` set to the panel's address).

How they talk:

- **Panel → node**: HTTP requests with a JWT signed by the panel (Ed25519, valid 15 minutes, bound to that node). The node checks it against the panel's public key and answers `401` (`invalid_token`) when it does not check out. A node seen speaking TLS once is never called over plain HTTP again. Every 30 seconds the panel also asks each remote node for the state of its servers (`GET /daemon/servers/status`) to record their [uptime](./13-uptime.md).
- **Node → panel**: the node signs in with its client credentials (`/oauth2/token`, `client_credentials`) and checks SFTP logins against the panel: passwords with the `password` grant, SSH keys with `POST /auth/sftp-key` signed with the node's secret.
- **Browser ↔ node**: consoles and statistics go through the panel, which proxies the websocket to the node.

Each node reports what it supports (`GET /api/nodes/:id/features`) and its system information (`GET /api/nodes/:id/system`). The dashboard shows each node's CPU, memory, disk and network history. Nodes also:

- download the runtime images in `docker.prefetchImages` in the background and keep them updated, so the first server that uses one does not wait;
- refuse an install when the servers folder would have less than `daemon.minFreeDiskMB` (2048 MB) free;
- answer the port check of the new-server wizard (`POST /api/nodes/:id/ports/check`).

More in [Dashboard and nodes](../features/dashboard-and-nodes.md).

## Servers

A **server** is one game or application instance running on a node.

- **Identifier**: letters, digits, `-` and `_`, 1 to 20 characters, not starting with `-` or `_`. The wizard builds it from the name plus a random suffix; the name itself is free text.
- **Definition**: the node keeps a JSON definition per server with its variables, install steps, run command and environment. It starts as a copy of the template.
- **Environment**: Docker. The Docker install turns off the host environments with `docker.disallowHost`, and the wizard only offers Docker.
- **Resources**: CPU (percent of one core, so 200 is two cores), memory (MB), disk (MB) and disk I/O priority, stored as the `cpu`, `memory`, `disk` and `io` variables. Only an administrator changes them.
- **Ports**: a primary port and extra ports (`port2`, `port3`…), each published over TCP and UDP. The owner can choose which assigned port is the primary one and label them; only an administrator assigns ports.
- **Lifecycle**: create → install (runs the template's install steps; automatic right after creation while `panel.servers.autoInstall` is on) → start / stop / restart / kill → reinstall → delete. A server that crashes is restarted according to its auto-restart setting, and the crash is recorded.
- **Owner and sub-users**: every server has an owner (the customer it belongs to) and may have sub-users with exactly the permissions the owner gives them.

Inside the server page: console, overview with statistics, file manager, settings, users, database, backups, scheduled tasks, activity, SFTP, and depending on the game and permissions plugins, modpacks, the world map, sub-servers (Splitter), migration to another panel and the administration tab. See [Panel usage](./usages.md) for a tour and the [feature guides](../features/README.md) for each part.

## Templates

A **template** is a JSON blueprint for a kind of server: its variables (with type, default, options and whether the user may edit them), the install steps, the run command, the Docker image and the ports.

Templates come from repositories:

| Repository | What it is |
|------------|------------|
| **Local** (ID `0`) | Templates stored in the panel's database. Uploaded and edited with `templates.local.edit`. |
| **Community** | When `templates.url` is set, a single repository read from that JSON index. The Docker install sets it to `https://templates.aetherpanel.es/templates.json` (`SKYPANEL_TEMPLATES_URL`), and the repositories stored in the database are then ignored. |
| Git repositories | Without `templates.url`, repositories added with `templates.repo.create` are cloned and read from disk. |

Any signed-in user with `login` can read templates; the **Templates** menu entry needs `templates.view`. When a server is created, the chosen template is copied into the server's definition with the values typed in the wizard; editing the template later does not change existing servers. More in [Templates](../features/templates.md).

## Sub-servers (Splitter)

The **Splitter** tab splits a server into smaller ones. A sub-server:

- takes its CPU, memory and disk out of the parent: the parent must have that much left, and the parent's own limits on the node shrink to what remains;
- runs on the parent's node, in Docker, with the parent's owner as its owner; the parent's members join it with the same permissions they have on the parent;
- can only be created by the parent's owner or an administrator (the owner needs the panel scope `server.create`, which the "Usuario" role includes);
- when created by a non-administrator, may only publish its own port variables and cannot set host folders, `hostConfig`, the network, labels or a container user.

Details in [Sub-servers](../features/sub-servers.md) and in the [Scopes reference](./scopes-reference.md#environment-and-definition).

## Users, roles and permissions

- A **user** signs in with email and password (plus 2FA if turned on). Accounts are created by an administrator, by public sign-up (if `panel.registrationEnabled` is on), by the billing API, or by inviting an email to a server.
- A **scope** is a named permission (`server.start`, `nodes.view`…). There are 73: 23 panel scopes and 50 server scopes. `admin` covers all of them; `server.admin` covers every server scope on one server.
- A **role** gives panel scopes to its users. The panel creates "Administrador" (`admin`) and "Usuario" (`login`, `self.edit`, `self.clients`, `server.create`, `templates.view`, `uptime.view`). A role's server scopes never apply on servers.
- On a server, a user's power comes from being its **owner** (a fixed set, `scopes.OwnerScopes`) or from their **permission row** on that server, which only holds server scopes.

Everything about scopes, owners, sub-users, roles and billing API keys is in the [Scopes reference](./scopes-reference.md). Sign-in, sessions, 2FA and SSH keys are in [Security](./security.md).

## Authentication

| Method | Used by |
|--------|---------|
| Session cookie `skypanel_auth` | The web interface. Sessions last 1 hour of inactivity and are renewed while active. |
| `Authorization: Bearer <token>` | API clients. The same session token, or an OAuth2 token. |
| OAuth2 (`/oauth2/token`) | `client_credentials` for OAuth2 clients and nodes, `password` for SFTP password checks from remote nodes. Users create their own clients with `self.clients`. |
| Node secret (`/auth/sftp-key`) | SFTP key checks from remote nodes: `Authorization: Bearer <node secret>`. |
| JWT (Ed25519) | Panel → node requests. Public key at `/auth/publickey`. |
| Billing API keys (`ak_...`) | `/api/v1` only (provisioning, suspension, sign-in links). |

See [Security](./security.md#sign-in-and-sessions).

## SFTP

Each daemon runs an SFTP server (port 5657 by default, `daemon.sftp.host`).

- The username is `email#serverID`; the server's **SFTP** tab shows the full address.
- Sign-in uses the panel password or an SSH key added in the account settings (keys work on every node; a remote node asks the panel to check them).
- The user needs `server.sftp` on that server, or to be its owner, and the panel-wide `login` permission. Both are checked on every connection.
- A session only sees that server's folder. Uploads count against the server's disk limit.

## Database hosts

A **database host** is a MySQL or MariaDB server where the panel creates databases for game servers.

- Administrators manage them in **Database hosts** (`/api/databasehosts`, requires `admin`): name, host, port (3306), a user allowed to create users and grant privileges, an optional maximum number of databases (`0` is no limit) and an optional node (then only servers on that node can use it).
- In a server's **Database** tab, whoever holds `server.data.edit` creates a database on one of the hosts the server may use. It is named `s_<serverId>_<name>`, gets its own user with a random name and password, and that user is granted all privileges on that database only.
- Deleting the server drops its databases and users from the host.
- Game servers reach the panel's own MariaDB as `mysql:3306`, because it is on their Docker network (see below).

More in [Database hosts](../features/database-hosts.md).

## Transfers

- **Between nodes of the same panel**: an administrator (`server.data.edit.admin`) moves a server to another node from the **Administration** tab (`POST /api/servers/:serverId/transfer`). Its sub-servers, its scheduled tasks and the backups kept on the old node go with it; if a step fails, or the panel restarts before the move is committed, the transfer is undone and the server stays where it was.
- **Between panels (federated transfer)**: moves a server from one Aether Panel installation to another (the **Migration** tab, `server.data.edit.admin`). The panels sign their requests with Ed25519 keys (`panel.exTransferKey`), tokens are hashed with HMAC-SHA256, and a transfer session is valid for 15 minutes; while the data moves, the destination's heartbeats (every 20 seconds) keep it alive, for up to 6 hours. States: `CREATED` → `VALIDATED` → `MIGRATING` → `CONSUMED` / `COMPLETED`, or `FAILED` / `CANCELLED`. The origin's routes are `/api/servers/:serverId/extransfer/{create,pull,status,cancel}`; the routes the other panel calls are `/api/extransfer/{validate,consume,heartbeat,confirm,download,cancel}`.

More in [Server transfers](../features/server-transfers.md).

<a id="docker-network-auto-detection-skypanel-network"></a>

## Docker networks

The Docker install uses two networks:

| Network | Who is on it |
|---------|--------------|
| `skypanel-network` | The panel, MariaDB and Adminer. |
| `skypanel-servers` | Game server containers, MariaDB and the panel. |

- Game servers go to `skypanel-servers` (`docker.serversNetwork`, set by `docker-compose.yml`). They reach MariaDB as `mysql:3306`, but not the panel's own network. The panel is also on `skypanel-servers` so it can reach game servers from inside (world map, player list).
- A template can name its own network (`networkName`); only administrators can set it.
- If `docker.serversNetwork` is empty, or that network does not exist, servers join the network the panel's container is on (detected at start-up), as older versions did.

<a id="panel-update-propagation"></a>

## Panel updates

**Settings → Updates** checks GitHub for a newer version of the branch the panel was installed from, and only offers it once GitHub's checks on it have passed. Updating (`POST /api/settings/update-panel`, `settings.edit`) starts a temporary container on the host that runs the update script for the local panel, and asks every remote node to update itself (`POST /daemon/update`). The installer does the same from the command line with `--update`. See [Panel updates](../features/panel-updates.md).

## Command line

The `SkyPanel` binary also has a CLI (`user add`, `user edit`, `db upgrade`, `version`…). In the Docker install run it with `docker exec -it skypanel /SkyPanel/bin/SkyPanel …`. See the [CLI commands reference](../reference/14-cli-commands.md).
