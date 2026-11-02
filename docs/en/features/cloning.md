# Cloning servers

Cloning creates a copy of an existing server on the same node, so changes (a new version, a modpack, a different configuration) can be tried on the copy without touching the original. It is an **experimental** feature: it is off until an administrator turns it on.

## What it does

The copy gets:

- The original's server definition (template, version, settings and variables, execution and environment).
- The same resource limits (CPU, memory and disk).
- The same owner, and a permission row for every user of the original with the same scopes.
- New ports for every port variable, chosen so that they do not collide with any server on the node.
- Optionally (on by default), all the original's files and world, plus the side files that describe an installed modpack or world map (`<id>.modpack`, `<id>.worldmap`).

The copy does **not** get:

- The original's address and port (new ones are assigned).
- The subdomain.
- Backups.
- Scheduled tasks, alerts, metrics, crash reports, file history or activity log.
- The billing link and the suspension state.
- OAuth2 clients of the original (SFTP accounts, API clients): only user permissions are copied.

## How it works

1. The panel validates the name (1 to 40 characters, printable ASCII only, the same rule as server names).
2. It checks who is asking (see [Limits](#limits)) and reads the original's definition from its node.
3. It generates a new server ID and assigns ports. For each port variable (`port`, `port2`…, `queryport`, `rconport`, `steamport`, `gameport`, `serverport`, `voiceport`, `webport`, `httpport`, `appport`, and any key ending in `_port`, `-port` or `.port`) it looks for the first free port in the 200 ports above the original's (25565 → 25566), and otherwise any free port on the node. Variables that shared a port keep sharing the new one. Ports written as plain numbers in the Docker `portBindings` and the original's port labels are moved to the new ports too.
4. It saves the copy with the original's users and, for a sub-server, checks that the parent still has room for it (see [Sub-servers](sub-servers.md)).
5. It creates the server on the node. If the copy is a sub-server, the parent's limits on the node are reduced accordingly.
6. With **Copy files and world** on, the panel asks the node to copy the files (`POST /daemon/server/<new>/clone-from/<original>`). The node copies in the background:
   - The new server counts as *installing* until the copy ends, so it cannot be started half copied.
   - The copy's console shows progress (`Copying files from …`, `Copied N files (X MB) so far…`) and the final total.
   - Regular files, folders and symlinks that stay inside the server folder are copied with their modes and times; symlinks that leave the folder, devices, sockets and pipes are skipped; files that disappear mid-copy (rotated logs) are ignored.
   - Afterwards, `server.properties` is rewritten so that lines whose key contains `port` and whose value was one of the original's ports point to the copy's ports.
7. If creating the server on the node or starting the copy fails, the panel removes the half-made copy from the node and the database.

The original's activity log records the action as `server.clone`, with the copy's name.

## Using it

### In the panel

1. An administrator turns on **Settings › General › Experimental features › Clone server**.
2. Open the server and go to **Administration › Clone** (the tab is marked as beta).
3. Enter the **Name of the copy** (it defaults to the original's name followed by " (copy)").
4. Choose whether to **Copy files and world**. If it is off, the copy starts empty with the same template and must be installed from **Administration**.
5. Click **Clone server** and confirm.

When it finishes, the panel shows the **New port** and an **Open the copy** button. If the files are being copied, follow the progress on the copy's console and start it once the copy ends.

If the original is running, the panel warns that the world may be copied mid-save. For a clean copy, stop the server first.

### Via the API

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| POST | `/api/servers/:serverId/clone` | `server.create` | Creates the copy. Answers `{"id": "<new id>", "port": <new main port>}`. |
| POST | `/daemon/server/:serverId/clone-from/:sourceId` | node (panel token) | Internal: copies the files of `sourceId` into the new, empty `serverId`. Answers `202`. |

Request body of `POST /api/servers/:serverId/clone`:

```json
{
  "name": "My server (copy)",
  "copyFiles": true,
  "label": "My server (copy)"
}
```

`copyFiles` defaults to `true` when omitted. `label` is the name the activity log shows on the original. See the [API reference](../reference/11-api-reference.md) for the common error format.

## Requirements and configuration

| Key | Default | Environment variable | Meaning |
|-----|---------|----------------------|---------|
| `panel.experimental.serverClone` | `false` | `SKYPANEL_PANEL_EXPERIMENTAL_SERVERCLONE` | Turns cloning on. While it is off, the route answers `403` with `ErrFeatureDisabled` and the tab is hidden. Turning it off deletes nothing. |

The user needs the panel-wide scope `server.create` (the default "Usuario" role includes it) or `admin`. See [Configuration](../backend/02-configuration.md) and the [scopes reference](../general/scopes-reference.md).

## Limits

- The copy is always created on the **same node** as the original. To move it elsewhere afterwards, use a [node transfer](server-transfers.md).
- **Without `admin`**, a user can only clone a **sub-server** of a server they own: creating a server without a parent is for administrators. Cloning a top-level server as a non-administrator answers `403` ("only administrators can create servers without a parent server").
- A non-administrator must have access to the original, and the original must not be suspended.
- A copy of a sub-server is a sub-server of the same parent and takes its CPU, memory and disk out of the parent's plan.
- The panel does not check the node's total capacity for a top-level copy: it simply gets the same limits as the original.
- The file copy needs a target with no files. It is a plain copy of the folder, not a consistent snapshot: copying a running server can catch files mid-write.
- The node must have enough free disk for a second copy of the files.

## Troubleshooting

| Message or status | Cause | Fix |
|-------------------|-------|-----|
| `403` `this experimental feature is turned off in the panel settings` | `panel.experimental.serverClone` is off. | Turn on **Clone server** in **Settings › General**. |
| `403` `you can only copy servers you have access to` | The user is not a member of the original. | Use an account with access, or an administrator. |
| `403` `a suspended server cannot be copied` | The original is suspended. | Unsuspend it first (administrator). |
| `403` `only administrators can create servers without a parent server` / `only the parent server's owner can split it` | A non-administrator tried to clone a top-level server, or a sub-server whose parent they do not own. | Ask an administrator to make the copy. |
| `400` `the copy needs a name` / `the name can be at most 40 characters long` / `the name can only use letters without accents, digits, spaces and common symbols` | Invalid name. | Use 1–40 printable ASCII characters. |
| `409` `there is no free port left on this node for the copy` | No free port could be found. | Free ports on the node or delete unused servers. |
| `409` `the node of this server is not reachable: …` / `the server's files are not on its node, so it cannot be copied` | The node is offline, or the server is a "ghost" (in the panel but not on the node). | Bring the node back online; check the server exists on the node. |
| `422` `not enough CPU/memory/disk available in parent server` | Copying a sub-server whose parent has no room left. | Free resources in the parent (shrink or delete other sub-servers). |
| Copy's console: `Copying the files failed: …` | The background copy stopped (disk full, permission error). The server stays without the remaining files. | Fix the cause, delete the copy and clone again. |
| Copy's console: `Could not update the ports in server.properties: …` | The file could not be read or rewritten (for example larger than 1 MB). | Edit `server-port` and the other port lines of `server.properties` by hand. |

## See also

- [Sub-servers (Splitter)](sub-servers.md)
- [Server transfers](server-transfers.md)
- [Scopes reference: owner and copies](../general/scopes-reference.md)
- [API reference](../reference/11-api-reference.md)
- [Configuration](../backend/02-configuration.md)
