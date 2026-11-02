# Server settings

The **Settings** tab of a server gathers what defines it day to day: its name, its resource limits, its ports, what happens when it stops or the node restarts, and the variables of the template it was installed from. This page explains each section, who can change it and what the panel does when you save.

The cards at the bottom of the same tab have their own pages: the server address and automatic subdomain ([Subdomains](./subdomains.md)), the per-server notifications ([Server notifications](./server-notifications.md)) and, when the experimental feature is on, the game version switch ([Game versions](./game-versions.md)).

## What it does

The tab is split into sections. Each one only shows when you hold the permission it needs:

| Section | Shows | Who sees it | Who can change it |
|---|---|---|---|
| **General Information** | The server name and its type (read only) | Everyone with the tab | `server.name.edit` |
| **Resource Limits** (`Admins only` badge) | CPU, memory, disk and disk priority | Administrators (`admin`, `server.admin`, `server.admin.config.view` or `server.admin.config.manage`) | `server.data.edit.admin` |
| **Ports** | Assigned ports, the primary one and a note per port | `server.data.view` | Ports: `server.data.edit.admin`. Notes: `server.data.edit` |
| **Auto Start Conditions** | Start on Boot, Restart on Crash, Restart on Stop | Everyone with the tab | `server.flags.edit` |
| **Plugin Settings** | Enable Plugins Tab (Minecraft Java only) | Everyone with the tab | Anyone (it is a browser preference, see below) |
| Template groups (one section per group) | The template's variables, grouped as the template defines them | Everyone with the tab | `server.data.edit` (variables marked as not user-editable need `server.data.edit.admin`) |
| **Variables** | Template variables that do not belong to any group | Everyone with the tab | Same as above |
| **Server Metadata** | Installation details (read only) | `admin` or `server.admin` | — |

The tab itself appears when you hold `server.data.view` or `server.flags.view` on the server. The owner of a server holds everything in this table except `server.data.edit.admin` and the administrator scopes, so owners rename the server, edit variables, port notes and auto-start conditions, but not limits or ports (see [Owner and sub-users](../general/scopes-reference.md)).

When something changes, a bar appears at the bottom with **You have unsaved changes**, **Discard** and **Save Changes**. Nothing is sent until you press **Save Changes**.

## How it works

Pressing **Save Changes** sends several requests in this order:

1. **Variables**: the user-editable variables go to `POST /api/servers/:id/data`. Variables the template marks with `userEdit: false` (CPU, memory, disk and similar) are sent separately to `PUT /api/servers/:id/data` when you hold `server.data.edit.admin`; the non-admin route skips them.
2. **Auto-start conditions** go to `POST /api/servers/:id/flags`.
3. **Name**: if it changed, `PUT /api/servers/:id/name/:name`.
4. **Minecraft Java only**: if `motd`, `ip` or `port` were among the variables sent, the panel rewrites those keys (`motd`, `server-ip`, `server-port`) in `server.properties`. If the file does not exist yet it is created with just those keys; any other read error cancels the save so the file is never overwritten.
5. **Ports** (administrators): `PUT /api/servers/:id/data` with `{ "ports": [...], "primaryPort": n }`.
6. **Port notes**, only when they changed: `PUT /api/servers/:id/port-settings` with the whole `portNotes` map (it replaces the stored one).

The server definition itself is never sent from this tab: administrators edit it in the **Administration** tab.

### Resource limits

- **Allocated CPU** (`%`): 100% is one processor thread.
- **RAM Memory** (MB): hard container memory limit.
- **Disk Space** (MB): when it fills up, the server stops and does not start or accept new files until space is freed. How it is measured and what is blocked is in [Servers: Disk limit](../backend/04-servers.md).
- **Disk priority** (`io`): share of disk reads and writes against the node's other servers, from 10 to 1000; 500 is an even share. Details in [Servers: Disk priority](../backend/04-servers.md).

The panel stores the new limits only after the node accepts them, so the page never shows limits the container does not have. Negative values are rejected. For servers split with Splitter, a sub-server must fit in what its parent has left, and a parent cannot go below what its sub-servers already use (see [Sub-servers](./sub-servers.md)).

### Ports

The first port in the list is the **Primary** one: it is the game port and the address the panel shows. The others are extra ports bound to the same container; the node exposes them as the template variables `port`, `port2`, `port3`… (see [ExtraPortBindings](../backend/04-servers.md)). Each port can carry a short **Note** (for example "Game", "RCON", "Query") that only serves as a label.

Rules checked when saving:

- At least one port; every port between 1024 and 65535; no duplicates (checked by the page and by the API).
- The API also refuses a port the node itself listens on (its web and SFTP ports) and a port already assigned to another server on the same node.
- Changing **which** port is primary requires the server to be stopped (checked by the page). Adding or removing extra ports is allowed while it runs; the change takes effect on the next restart, and the panel says so after saving.
- When the primary port changes, the server's subdomain record is refreshed (see [Subdomains](./subdomains.md)).

### Auto-start conditions

| Switch | Field (`/flags`) | What the node does |
|---|---|---|
| **Start on Boot** | `autoStart` | Queues the server to start when the daemon starts (for example after the node reboots or the service restarts). Queued servers start one per second. |
| **Restart on Crash** | `autoRestartOnCrash` | Starts the server again after a crash, up to `daemon.data.crashLimit` times in a row (default `3`). The count resets when the server ran more than 10 minutes before crashing. A stop or kill requested from the panel is never a crash. |
| **Restart on Stop** | `autoRestartOnGraceful` | Starts the server again when the process ends **by itself** with its expected exit code (for example someone typed `stop` in the game). A stop or kill from the panel does not trigger it. |

Crashes are also recorded and reported; see [Crash reports](./crash-reports.md).

### Plugin tab switch

**Enable Plugins Tab** only shows or hides the **Plugins** tab of that server. It is saved in the browser (`localStorage`), not on the server, so it applies only to the browser where it was changed and does not affect other users.

## Using it

### In the panel

1. Open the server and go to **Settings**.
2. Change what you need. Fields you cannot edit are not shown, or are shown as plain text (for example the port numbers for a non-administrator).
3. Press **Save Changes** in the bottom bar, or **Discard** to go back to the saved values.

To add a port (administrators): **Ports → Add port**, type the number and save. To remove one, use the **×** button on its row. The primary port cannot be removed while the server runs.

### Via the API

| Method and path | Scope | Purpose |
|---|---|---|
| `GET /api/servers/:id/data` | `server.data.view` | Read the variables |
| `POST /api/servers/:id/data` | `server.data.edit` | Save user-editable variables |
| `PUT /api/servers/:id/data` | `server.data.edit.admin` | Save admin variables, `cpu`/`memory`/`disk`, `ip`/`port`, `ports` + `primaryPort`, `portNotes` |
| `PUT /api/servers/:id/port-settings` | `server.data.edit` | Change the primary port among the assigned ones and/or replace `portNotes` |
| `GET /api/servers/:id/flags` | `server.flags.view` or `server.flags.edit` | Read the auto-start conditions |
| `POST /api/servers/:id/flags` | `server.flags.edit` | Save `autoStart`, `autoRestartOnCrash`, `autoRestartOnGraceful` |
| `PUT /api/servers/:id/name/:name` | `server.name.edit` | Rename the server |
| `GET /api/servers/:id/definition` | `server.definition.view` | Read the definition (groups, variable metadata) |

Example, admin port change (port `25566` becomes primary, `25575` stays as an extra port):

```bash
curl -X PUT https://panel.example.com/api/servers/abc123/data \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"ports": [25566, 25575], "primaryPort": 25566, "portNotes": {"25566": "Game", "25575": "RCON"}}'
```

Request and response details are in the [API reference](../reference/11-api-reference.md).

## Requirements and configuration

| Key | Default | Environment variable | Effect |
|---|---|---|---|
| `daemon.data.crashLimit` | `3` | `SKYPANEL_DAEMON_DATA_CRASHLIMIT` | Automatic restarts in a row after crashes |

The full list of keys is in [Configuration](../backend/02-configuration.md).

## Limits

- The "primary port only while stopped" rule is checked by the page, not by the API.
- **Enable Plugins Tab** is per browser and does not hide the tab for anyone else.
- Variables marked `internal` in the template are never shown.
- The page reads its data once when it opens; changes made meanwhile by someone else are overwritten by a later save from this page.

## Troubleshooting

| Message | Cause | Fix |
|---|---|---|
| `The server must be stopped to change the primary port.` | The server is running | Stop it, change the primary port and save |
| `Port N is invalid (must be between 1024 and 65535).` / `Port N is already assigned to this server.` | Page-side validation | Use a port in range, without repeating it |
| `el puerto N lo usa el propio panel en este nodo` | The port is the node's web or SFTP port | Choose another port |
| `el puerto N ya está en uso por otro servidor en este nodo` | Another server on the node has it | Choose a free port. `POST /api/nodes/:id/ports/check` (used by the create-server wizard) tells which ports are free on a node; see [Dashboard and nodes](./dashboard-and-nodes.md) |
| `primaryPort must be one of the assigned ports` | API call with a `primaryPort` outside `ports` | Include it in `ports` |
| `the sub-servers of this server already use more than that` | Lowering a parent below what its sub-servers hold | Shrink or remove sub-servers first |
| `resources cannot be negative` | Negative CPU, memory or disk | Send zero or a positive number |
| `Failed to save settings` | One of the requests failed (node down, missing permission) | Check the node is online and that you hold the scope of the section you changed |

The API messages about ports are currently returned in Spanish, whatever the panel language.

## See also

- [Server properties](./server-properties.md): `server.properties` as a form (Minecraft Java).
- [Servers: environments, lifecycle and scheduler](../backend/04-servers.md)
- [Scopes reference](../general/scopes-reference.md)
- [Crash reports](./crash-reports.md)
