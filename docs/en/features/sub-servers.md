# Sub-servers (Splitter)

A sub-server is a server carved out of another server's resources. The owner of a server with, say, 8 GB of memory can split it into a lobby and two game servers that share those 8 GB, without asking an administrator for a new plan. In the panel this feature is called **Splitter**.

## What it does

- Creates child servers whose CPU, memory and disk are taken out of the parent's plan.
- Keeps the parent's own container limited to what its sub-servers leave free.
- Shows, for the parent, the total resources, how much the sub-servers use and how much is free, plus a card per sub-server with its status and limits.
- Gives the sub-servers the parent's node, owner and members.

## How it works

**Data model.** A sub-server is a normal server with `parent_server_id` set to the parent's identifier (`servers.parent_server_id`). The panel stores each server's whole plan in `TotalCPU`, `TotalMemory` and `TotalDisk`.

**Creation.** When a server is created with a parent (`PUT /api/servers/:serverId` with `parent_server_id`):

1. The sub-server is placed on the **parent's node**, whatever node the request names.
2. Its users are the parent's members, and each keeps **exactly the scopes they have on the parent** (a sub-user who can only watch the console on the parent gets only that on the child).
3. Its owner is the parent's owner. If the parent has no owner, neither does the child.
4. It must have a CPU, memory and disk limit greater than 0 (0 would mean "no limit" to Docker).
5. In one transaction that locks the parent row, the panel adds up the limits of the existing sub-servers and refuses the new one if the parent would go below zero in any resource.
6. The server is created on the node and, if `panel.servers.autoInstall` is on, its installation starts.
7. The parent's limits on the node are lowered to `plan − sum of sub-servers` (never below 1, because 0 means unlimited to Docker). The panel keeps the parent's full plan in its database.

**Non-administrators.** Users without `admin` can only create servers through this flow:

- The request must name a parent, and the user must be the parent's **owner** (sub-users of the parent cannot split it).
- The environment must be `docker`. Only the keys `image`, `portBindings`, `containerRoot` and `config` are kept; host mounts (`bindings`), `hostConfig`, the network and labels are dropped, and the container user is removed from `config`.
- Every published port binding must use one of the server's own port variables (for example `0.0.0.0:${port}:${port}`).

**Changing limits.** When an administrator changes resources (`PUT /api/servers/:serverId/data`):

- For a sub-server, the new limits must fit in what the parent has left; the parent's node limits are then recalculated.
- For a parent, the new plan cannot be lower than what its sub-servers already hold, and the limits sent to its node are what the sub-servers leave free.

**Deleting.** Deleting a sub-server gives its resources back to the parent (the parent's node limits are recalculated). A parent with sub-servers cannot be deleted.

**Suspending.** Suspending or unsuspending a parent does the same to all its sub-servers; suspending also stops them.

## Using it

### In the panel

1. Open the parent server and go to the **Splitter** tab. It is shown to the server's owner and to users with `server.data.edit.admin` (and administrators).
2. The **Server split** card shows **Total CPU**, **Total memory** and **Total disk**, with **Used** and **Free** for each.
3. Click **Create sub-server**. The creation wizard opens with the parent fixed: the node, environment and users are inherited (**Inherited environment**). Choose a name, a template and the sub-server's CPU, memory and disk.
4. The new sub-server appears under **Active sub-servers**. Click a card to open it.

If the server is itself a sub-server, the tab only says: *This server is already a sub-server. Only main servers can be split.*

### Via the API

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| PUT | `/api/servers/:serverId` | `server.create` (non-admins: owner of the parent) | Creates a server; with `parent_server_id` it is a sub-server. |
| PUT | `/api/servers/:serverId/data` | `server.data.edit.admin` | Changes limits; checked against the parent or the sub-servers. |
| DELETE | `/api/servers/:serverId` | `server.delete` | Deletes; refused for a parent that still has sub-servers. |
| GET | `/api/servers` | session (lists the servers the user can see) | Lists servers; each item carries `parent_server_id` when it is a sub-server. |

Minimal body for a sub-server (the rest is the template definition, as for any server):

```json
{
  "name": "Lobby",
  "parent_server_id": "a1b2c3d4",
  "environment": { "type": "docker", "image": "…" },
  "data": {
    "cpu":    { "value": 100 },
    "memory": { "value": 2048 },
    "disk":   { "value": 10240 }
  }
}
```

See the [API reference](../reference/11-api-reference.md) for the full server definition and response format.

## Requirements and configuration

- The parent must have real limits (CPU, memory and disk). A parent with a 0 (unlimited) limit has nothing to split.
- Non-administrators need the panel-wide scope `server.create`. The default "Usuario" role includes it; see the [scopes reference](../general/scopes-reference.md).
- `panel.servers.autoInstall` (default `true`, `SKYPANEL_PANEL_SERVERS_AUTOINSTALL`) decides whether a new sub-server installs straight away. See [Configuration](../backend/02-configuration.md).

There are no settings specific to sub-servers.

## Limits

- One level only in the interface: a sub-server cannot be split again from the **Splitter** tab.
- A sub-server always lives on its parent's node. A [node transfer](server-transfers.md) of the parent moves its sub-servers along with it, with their files, backups and scheduled tasks; a sub-server cannot be transferred on its own (`400` `ErrTransferSubServer`).
- CPU is a percentage (100 = one core), memory and disk are in MB, the same units as any server.
- The parent's database plan is not reduced; only the limits applied on the node are. Usage graphs of the parent therefore compare against the node limits.
- Customers cannot choose host mounts, privileged mode, networks, labels or the container user for a sub-server.

## Troubleshooting

| Message | Cause | Fix |
|---------|-------|-----|
| `not enough CPU available in parent server` (also memory or disk) | The new or resized sub-server does not fit in what the parent has left. | Lower the sub-server's limits, or shrink or delete other sub-servers. |
| `a sub-server needs a CPU, memory and disk limit` | A limit was 0. | Give every resource a value greater than 0. |
| `only the parent server's owner can split it` | A sub-user of the parent tried to split it. | The owner (or an administrator) must create it. |
| `only administrators can create servers without a parent server` | A non-administrator called the create route without `parent_server_id`. | Use the **Splitter** tab, or ask an administrator. |
| `only docker environments are allowed for sub-servers` | The environment was not `docker`. | Use a Docker template. |
| `sub-servers can only publish their port variables, not "…"` / `… is not a port variable of the server` | A port binding uses a fixed port or an unknown variable. | Use bindings based on the server's port variables. |
| `parent server not found` | `parent_server_id` does not exist. | Check the parent's identifier. |
| `the sub-servers of this server already use more than that` | An administrator tried to shrink a parent below what its sub-servers hold. | Shrink or delete sub-servers first. |
| `cannot delete server because it has active child servers` | Deleting a parent that still has sub-servers. | Delete its sub-servers first. |
| `403` with no message when clicking **Create sub-server** | The user lacks `server.create`. | Give it directly or through a role. |

## See also

- [Scopes reference: owner, sub-users and sub-servers](../general/scopes-reference.md)
- [Cloning servers](cloning.md) (a copy of a sub-server is a sub-server of the same parent)
- [Servers backend: container user and isolation](../backend/04-servers.md)
- [API reference](../reference/11-api-reference.md)
