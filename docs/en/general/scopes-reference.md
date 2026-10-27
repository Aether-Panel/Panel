# Permissions reference (scopes)

A **scope** is a named permission, for example `server.start`. Every API route asks for one, and the request goes through only if the user holds it. The full list is in `internal/scopes/scopes.go`: 73 scopes, 23 panel scopes and 50 server scopes.

- **Panel scopes**: apply to the whole panel (`nodes.view`, `settings.edit`…).
- **Server scopes**: apply to one server. The route must carry `:serverId`; without it, the request is refused.

Two scopes cover others: `admin` covers all of them, and `server.admin` covers every server scope on that server.

Billing API keys do not use scopes: they have their own list of actions (see [API keys](#api-keys)).

## Where permissions come from

On every request the panel gathers the user's permissions from these sources:

| Source | On panel routes | On a server's routes |
|--------|-----------------|----------------------|
| The user's role | Counts | Only its panel scopes count (for example `admin`). Its server scopes **do not count**. |
| The user's global row (permissions with no server) | Counts | Counts |
| Owning the server | — | Gives the owner set (see below) |
| The user's row on that server | — | Counts |

If the session comes from an OAuth2 client, the client sets another limit on top: a client tied to a server only acts on that server, and a client with declared scopes never goes beyond them.

## Owner and sub-users

Every server has an **owner** (`servers.owner_id`): the customer it belongs to. Everyone else with access is a **sub-user**.

### Who owns a new server

| How it is created | Owner |
|-------------------|-------|
| From billing (`POST /api/v1/provision`) | The customer it was bought for |
| Sub-server (Splitter) | The parent server's owner |
| Copy of a server (clone) | The original's owner |
| An administrator creates it for other users | The first user in the list |
| An administrator creates it with no users | Nobody: it is the administrator's own server |

The owner is always a member of the server too (they have a permission row on it), so the server shows up in their list.

### What the owner can do

The owner holds the `scopes.OwnerScopes` set on their server, whatever their row says:

| Group | Scopes |
|-------|--------|
| View | `server.view`, `server.status`, `server.stats` |
| Console | `server.console`, `server.console.send` |
| Power and installation | `server.start`, `server.stop`, `server.kill`, `server.reload`, `server.install` |
| Files | `server.files.view`, `server.files.edit`, `server.sftp` |
| Backups | `server.backup.view`, `server.backup.create`, `server.backup.restore`, `server.backup.delete` |
| Settings | `server.data.view`, `server.data.edit`, `server.flags.view`, `server.flags.edit`, `server.name.edit`, `server.definition.view` |
| Tasks | `server.tasks.view`, `server.tasks.run`, `server.tasks.create`, `server.tasks.edit`, `server.tasks.delete` |
| Users and clients | `server.users.view`, `server.users.create`, `server.users.edit`, `server.users.delete`, `server.clients.view`, `server.clients.create`, `server.clients.edit`, `server.clients.delete` |

Also, only the owner (or an administrator) can **split** the server with Splitter: splitting takes resources out of their plan.

What is left out is the administrator's call:

- Deleting the server (`server.delete`).
- Resources and admin variables, suspending and transferring (`server.data.edit.admin`).
- Editing the definition and the environment (`server.definition.edit`).
- Ports (`server.ports.edit`) and the admin section (`server.admin.*`).

The owner cannot be limited or removed from the server: the API answers `409 ErrServerOwner`.

### Changing the owner

Administrators only. In the server's **Users** tab, each sub-user has a **Make owner** button (only administrators see it). Through the API:

```
PUT /api/servers/:serverId/owner/:email
```

Requires `admin`. If the user was not a member, they become one with `server.view`. The previous owner stays as a sub-user with the permissions their row already had. The owner cannot give the server away on their own.

### Sub-users

A sub-user holds **exactly** the scopes of their row on that server. Their role's server scopes do not count: before, the "Usuario" role gave every member full control, even when the owner had only given them "view".

- Whoever holds `server.users.edit` on the server adds and edits sub-users (the owner holds it). Both use the same route: `PUT /api/servers/:serverId/user/:email`.
- Whoever holds `server.users.delete` removes sub-users.
- Nobody gives more than they hold: the editor can only set or clear the scopes they hold on that server themselves. The other scopes in the row stay as they were. The exceptions are users with `server.admin` (on the server or in their global row) or `admin` in their global row: they can set any scope.
- If the email has no account, the panel creates an invited account (allowed to sign in) and emails it the link to choose its password (`/forgot-password/`).

### The permission editor

In the server's **Users** tab, the owner sees the list of members. The owner comes first, with the **Owner** badge and no buttons. Each sub-user has **Permissions** (edit), **Revoke** and, for administrators, **Make owner**.

When adding or editing, permissions are ticked by group. Each group has a checkbox that ticks or clears the whole group:

| Group | Scopes |
|-------|--------|
| Console | `server.console`, `server.console.send`, `server.stats`, `server.status` |
| Power and installation | `server.start`, `server.stop`, `server.kill`, `server.reload`, `server.install` |
| Files & SFTP | `server.files.view`, `server.files.edit`, `server.sftp` |
| Backups | `server.backup.view`, `server.backup.create`, `server.backup.restore`, `server.backup.delete` |
| Settings | `server.data.view`, `server.data.edit`, `server.flags.view`, `server.flags.edit`, `server.name.edit`, `server.definition.view` |
| Scheduled tasks | `server.tasks.view`, `server.tasks.run`, `server.tasks.create`, `server.tasks.edit`, `server.tasks.delete` |
| Users & Clients | `server.users.view`, `server.users.create`, `server.users.edit`, `server.users.delete`, `server.clients.view`, `server.clients.create`, `server.clients.edit`, `server.clients.delete` |

These are the same scopes the owner holds. A new sub-user starts with `server.console`, `server.stats`, `server.status`, `server.start` and `server.stop`. The web always adds `server.view`: without it the server is not in their list.

### What each person sees on the web

The server page asks for `GET /api/servers/:serverId?perms`. The answer carries the scopes the user holds on that server and `owner: true` if they are the owner. With that, the web only shows what the user can use:

| Tab or action | Shown with |
|---------------|------------|
| Console | `server.console` |
| Typing in the console | `server.console.send` (without it, the console is read-only) |
| Start, Restart, Stop, Kill | `server.start`, `server.start` + `server.stop`, `server.stop`, `server.kill` |
| Files | `server.files.view` |
| Settings | `server.data.view` or `server.flags.view` |
| Users | `server.users.view` |
| Database | `server.data.edit` |
| Backups | `server.backup.view` |
| Tasks | `server.tasks.view` |
| SFTP | `server.sftp` |
| External transfer | `server.data.edit.admin` |
| Splitter | Being the owner or `server.data.edit.admin` |
| Plugins | `server.files.edit` |

`admin` or `server.admin` show everything. Hiding is only a convenience: the API checks every request anyway.

### Environment and definition

The environment decides what the container may touch on the node (host folders, privileged mode, running on the host). That is why:

- Editing the definition (`PUT /api/servers/:serverId/definition`) asks for `server.definition.edit`, which the owner does not hold.
- Even with `server.definition.edit`, without `server.data.edit.admin` too the environment stays as it is. In a `docker` environment only `image`, `portBindings`, `containerRoot`, `config` and `labels` can change. The environment type cannot change.
- A non-administrator can only create sub-servers: of a server they own, in `docker` and with those same keys. `bindings`, `hostConfig` and `networkName` are dropped.

### Sub-servers and copies

- A sub-server belongs to the parent's owner (if the parent has no owner, neither does the child). The parent's members join the child with exactly the permissions they had on the parent.
- A copy belongs to the original's owner and copies its users' permission rows as they are.

### Servers that already existed

On upgrade, the `20261026-server-owners` migration names an owner for every server that has none: the member with the most scopes on that server and, on a tie, the one added first. Billing customers hold the full set, so it is usually them.

A server with no members stays without an owner. After upgrading, check in the **Users** tab that each server has the right owner, and change it with **Make owner** if needed.

## Roles

A role gives panel scopes to all its users. Its server scopes do not count on any server: what a user can do on a server comes from owning it or from their row on it.

Roles the panel creates:

| Role | Panel scopes |
|------|--------------|
| Administrador | `admin` |
| Usuario | `login`, `self.edit`, `self.clients`, `server.create`, `templates.view`, `uptime.view` |

The "Usuario" role still stores server scopes from earlier versions; they no longer have any effect on servers. `server.create` lets it split its own servers; creating a server with no parent is for administrators only.

## API keys

API keys are for billing systems (WHMCS, Paymenter…). They only work on `/api/v1` and each key only does the actions it was given.

An administrator creates them in **Settings → API keys**. When creating one, tick **What it can do**; all actions start ticked. The full key (`ak_...`) is shown only once; the panel stores only its hash. To change a key's actions, create a new one and revoke the old one: a key cannot be edited.

The key goes in the `X-Api-Key: ak_...` header or in `Authorization: Bearer ak_...`.

| Action | Routes | In Settings |
|--------|--------|-------------|
| `create` | `POST /api/v1/provision` | Create servers |
| `terminate` | `POST /api/v1/terminate` | Terminate |
| `suspend` | `POST /api/v1/suspend`, `POST /api/v1/unsuspend` | Suspend and unsuspend |
| `change-package` | `POST /api/v1/change-package` | Change plan |
| `sso` | `POST /api/v1/sso` | Sign-in links |

- `GET /api/v1/ping` needs no action.
- A route the key does not hold answers `403 ErrAPIKeyNotAllowed`.
- Keys created before this hold `provision` (or no list). That means every action, so existing integrations keep working. Settings shows them as **Every action**.

Through the API (requires `admin`):

```
POST /api/settings/apikeys
{ "name": "WHMCS", "permissions": ["create", "suspend"] }
```

Without `permissions`, the key gets `provision` (every action). Unknown or repeated names are dropped. If no known name is left, it answers `400 ErrAPIKeyPermissions`.

Limits on `sso` even when the key holds it:

- The link only goes to a server the customer can open (`server.view`).
- It is refused for accounts with panel powers, directly or through their role: `admin`, `settings.edit`, `oauth2.auth` and the scopes that change nodes, templates, users or their permissions. That way a leaked key cannot open an administrator's account.

## Server scopes (50)

The **Owner** column says whether it is in the owner set.

### View and power

| Scope | What it allows | Owner |
|-------|----------------|-------|
| `server.view` | See the server, its version, its map and the list of databases. Connect to the socket. | yes |
| `server.status` | See whether it is running. | yes |
| `server.stats` | Statistics, metrics and `query`. | yes |
| `server.console` | See the console and the crash history, and use the AI assistant. | yes |
| `server.console.send` | Send commands to the console. | yes |
| `server.start` | Start. Restart asks for `server.start` and `server.stop`. | yes |
| `server.stop` | Stop. | yes |
| `server.kill` | Force stop. | yes |
| `server.reload` | Reload the configuration. | yes |
| `server.install` | Install or reinstall, install modpacks and switch versions. | yes |

### Files and backups

| Scope | What it allows | Owner |
|-------|----------------|-------|
| `server.files.view` | List and read files, their history, plugins and modpacks. | yes |
| `server.files.edit` | Upload, edit, delete, compress and extract files; restore versions; install or remove plugins and the map. | yes |
| `server.sftp` | Sign in over SFTP. | yes |
| `server.backup.view` | See and download backups. | yes |
| `server.backup.create` | Create backups. | yes |
| `server.backup.restore` | Restore backups. | yes |
| `server.backup.delete` | Delete backups. | yes |

### Settings

| Scope | What it allows | Owner |
|-------|----------------|-------|
| `server.data.view` | See the variables. | yes |
| `server.data.edit` | Edit the user variables, the primary port and port notes, databases and the subdomain. | yes |
| `server.data.edit.admin` | Admin variables (resources), suspending, transferring between nodes and external transfers. Unlocks the environment when editing the definition. | no |
| `server.flags.view` | See auto start and auto restart. `server.flags.edit` is also enough. | yes |
| `server.flags.edit` | Edit auto start and auto restart, the server's Discord alerts and the subdomain. | yes |
| `server.name.edit` | Rename. | yes |
| `server.definition.view` | See the definition. | yes |
| `server.definition.edit` | Edit the definition (see [Environment and definition](#environment-and-definition)). | no |
| `server.ports.edit` | No API route checks it; the web uses it. | no |
| `server.delete` | Delete the server. | no |

### Tasks

| Scope | What it allows | Owner |
|-------|----------------|-------|
| `server.tasks.view` | See tasks. | yes |
| `server.tasks.run` | Run a task by hand. | yes |
| `server.tasks.create` | Create tasks. Creating and editing use the same route, which accepts either one. | yes |
| `server.tasks.edit` | Edit tasks. | yes |
| `server.tasks.delete` | Delete tasks. | yes |

### Users and clients

| Scope | What it allows | Owner |
|-------|----------------|-------|
| `server.users.view` | See the members and the server's activity. | yes |
| `server.users.edit` | Add sub-users and change their permissions. | yes |
| `server.users.delete` | Remove sub-users. | yes |
| `server.users.create` | No API route checks it: adding uses `server.users.edit`. | yes |
| `server.clients.view`, `server.clients.create`, `server.clients.edit`, `server.clients.delete` | No API route checks them today. | yes |

### Administration

| Scope | What it allows | Owner |
|-------|----------------|-------|
| `server.admin` | Every server scope on that server. It also allows giving any scope to sub-users. | no |
| `server.admin.view` | No API route checks it; the web uses it for the Admin tab. | no |
| `server.admin.install.view`, `server.admin.install.manage` | Same as above. | no |
| `server.admin.transfer.view`, `server.admin.transfer.manage` | Same as above. | no |
| `server.admin.config.view`, `server.admin.config.manage` | Same as above. | no |
| `server.admin.assignments.view`, `server.admin.assignments.manage` | Same as above. | no |

## Panel scopes (23)

| Scope | What it allows |
|-------|----------------|
| `admin` | Everything. Managing roles, API keys, extensions, database hosts, products and server owners. |
| `login` | Signing in, seeing your own profile and reading templates. |
| `panel` | No API route checks it. |
| `self.edit` | Editing your own profile, 2FA and SSH keys. |
| `self.clients` | Creating and deleting your own OAuth2 clients. |
| `oauth2.auth` | Validating users' credentials over OAuth2. |
| `server.create` | Creating servers. Without `admin`, only sub-servers of a server you own. Also cloning (experimental). |
| `settings.edit` | Editing the panel settings and activating the license. |
| `nodes.view` | Seeing nodes. |
| `nodes.create` | Creating nodes. |
| `nodes.edit` | Editing nodes. |
| `nodes.delete` | Deleting nodes. |
| `nodes.deploy` | Seeing a node's deployment data (it includes secrets). |
| `templates.view` | No API route checks it: reading templates asks for `login`. |
| `templates.local.edit` | Uploading and editing local templates. |
| `templates.repo.create` | Adding template repositories. |
| `templates.repo.delete` | Deleting template repositories. |
| `users.info.search` | Searching users and seeing the panel's activity. |
| `users.info.view` | Seeing a user. |
| `users.info.edit` | Creating, editing and deleting users. |
| `users.perms.view` | Seeing a user's permissions. |
| `users.perms.edit` | Changing a user's permissions. |
| `uptime.view` | Seeing the uptime history. |
