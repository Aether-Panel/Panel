# Permissions reference (scopes)

A **scope** is a named permission, for example `server.start`. Every API route asks for one, and the request goes through only if the user holds it. The full list is in `internal/scopes/scopes.go`: 73 scopes, 23 panel scopes and 50 server scopes.

- **Panel scopes**: apply to the whole panel (`nodes.view`, `settings.edit`…).
- **Server scopes**: apply to one server. The route must carry `:serverId`; without it, the request is refused.

Two scopes cover others: `admin` covers all of them, and `server.admin` covers every server scope on that server. Because `admin` covers everything, it also counts as `login`: an account with `admin` can sign in even without `login`.

When a request lacks the scope, the API answers `403` with no body. A server scope asked for on a route without `:serverId` is always refused.

Billing API keys do not use scopes: they have their own list of actions (see [API keys](#api-keys)). The routes behind each scope are listed in the [API reference](../reference/11-api-reference.md).

## Where permissions come from

On every request the panel gathers the user's permissions from these sources:

| Source | On panel routes | On a server's routes |
|--------|-----------------|----------------------|
| The user's role | Counts | Only its panel scopes count (for example `admin`). Its server scopes **do not count**. |
| The user's global row (permissions with no server) | Counts | Counts |
| Owning the server | — | Gives the owner set (see below) |
| The user's row on that server | — | Counts, only its server scopes (a panel scope such as `admin` in it is ignored) |

If the session comes from an OAuth2 client, the client sets another limit on top: a client tied to a server only acts on that server, and a client with declared scopes never goes beyond them. That limit also applies to the scopes the request keeps for the later checks inside each route (seeing a database password, editing the environment, granting scopes, a task's steps), so those checks never see the user's wider powers.

The server socket (live console, status and statistics) uses the same resolved scopes as every other route: the owner's, the global row, the role and the OAuth2 client's bounds. It gets the console with `server.console`, the status with `server.status` and the statistics with `server.stats`.

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

#### Variables only an administrator changes

Some template variables hold what the customer pays for: the server's limits, its address and its ports. Only someone with `server.data.edit.admin` changes them, **even when the template marks them as editable by the user**:

| Kind | Variables |
|------|-----------|
| Limits and address | `memory`, `cpu`, `io`, `disk`, `swap`, `ip` (any case) |
| Ports | `port`, `port2`, `port3`…, `queryport`, `rconport`, `steamport`, `gameport`, `serverport`, `voiceport`, `webport`, `httpport`, `appport`, any name ending in `_port`, `-port` or `.port`, and any `integer` variable whose name ends in `port` |

Without `server.data.edit.admin`:

- `POST /api/servers/:serverId/data` (the **Settings** tab) silently skips them, like any other variable the template does not open to the user.
- `PUT /api/servers/:serverId/definition` keeps their current value, and drops new variables of that kind.

The owner can still pick which of the ports already assigned is the primary one and label them (`PUT /api/servers/:serverId/port-settings`, with `server.data.edit`): that does not take a new port.

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
- A server's permission row **only holds server scopes**. Panel scopes sent to `PUT /api/servers/:serverId/user/:email` (`admin`, `nodes.view`, `settings.edit`…) are dropped before saving, also when the editor holds `server.admin`. When a user's permissions are gathered, panel scopes found in a server row are ignored as well. Before, someone with `server.admin` on a server could write `admin` into their own row and become a panel administrator.
- Nobody gives more than they hold: the editor can only set or clear the scopes they hold on that server themselves. The other scopes in the row stay as they were. The exceptions are users with `server.admin` (on the server or in their global row) or `admin` in their global row: they can set any scope.
- If the email has no account, the panel creates an invited account (allowed to sign in) and emails it the link to choose its password (`/forgot-password/`). The email must be a plain address (`ana@example.com`, not `Ana <ana@example.com>`). With public sign-ups turned off (`panel.registrationEnabled: false`), only an administrator can invite someone without an account; anyone else gets `403 ErrInviteSignupsClosed`.
- Looking a person up by email on a server (`GET /api/servers/:serverId/user/:email`) answers `404` for an account that is not a member of that server, the same as for an unknown email. So an owner cannot find out which emails have an account on the panel.

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
| Plugins | `server.files.edit`, and the tab turned on for that server |
| Overview, Activity | Always (their data still asks for its own scope) |
| Properties (beta) | Minecraft: Java Edition, `panel.experimental.propertiesEditor` on, and `server.files.edit` |
| Modpacks, Map | Minecraft: Java Edition servers |
| Administration | Any of `server.install`, `server.delete`, `server.data.edit.admin`, `server.definition.edit` or the `server.admin.*` view scopes. An owner sees it because of `server.install` (reinstall); the transfer, configuration, suspension and deletion sections need the administrator's scopes. |

`admin` or `server.admin` show everything. Hiding is only a convenience: the API checks every request anyway.

### Environment and definition

The environment decides what the container may touch on the node (host folders, privileged mode, running on the host). That is why:

- Editing the definition (`PUT /api/servers/:serverId/definition`) asks for `server.definition.edit`, which the owner does not hold.
- Even with `server.definition.edit`, without `server.data.edit.admin` too the environment stays as it is. In a `docker` environment only `image`, `containerRoot` and `config` can change, and `config` loses any `User` key (the game runs as the user the node gives it, see [Security](./security.md#game-server-containers)). Published ports (`portBindings`) stay the template's. The environment type cannot change. The [admin-only variables](#variables-only-an-administrator-changes) keep their value.
- A non-administrator can only create sub-servers: of a server they own, in `docker`, with only `image`, `portBindings`, `containerRoot` and `config` (without its `User`). `bindings`, `hostConfig`, `networkName` and `labels` are dropped. Each published port must be one of the sub-server's own port variables (`${port}:25565`, `0.0.0.0:${port2}:…` or `${ip}:${port}:…`); a fixed port such as `0.0.0.0:25565:25565` is refused, because it could take another customer's port.

### Sub-servers and copies

- A sub-server belongs to the parent's owner (if the parent has no owner, neither does the child). The parent's members join the child with exactly the permissions they had on the parent.
- A copy belongs to the original's owner and copies its users' permission rows as they are.

### Servers that already existed

On upgrade, the `20261026-server-owners` migration names an owner for every server that has none: the member with the most scopes on that server and, on a tie, the one added first. Billing customers hold the full set, so it is usually them.

A server with no members stays without an owner. After upgrading, check in the **Users** tab that each server has the right owner, and change it with **Make owner** if needed.

## Roles

A role gives panel scopes to all its users. Its server scopes do not count on any server: what a user can do on a server comes from owning it or from their row on it.

Roles the panel creates in the database (migration `20260617-default-roles`, adjusted by `20260624-usuario-role-templates-view`, `20260625-usuario-role-add-server-create` and `20260821-fix-usuario-role-scopes`). This is what they hold on a new installation; an administrator can change them later in **Roles**:

| Role | Panel scopes | Server scopes stored (no effect) |
|------|--------------|----------------------------------|
| Administrador | `admin` | — |
| Usuario | `login`, `self.edit`, `self.clients`, `server.create`, `templates.view`, `uptime.view` | `server.view`, `server.status`, `server.stats`, `server.console`, `server.console.send`, `server.start`, `server.stop`, `server.kill`, `server.install`, `server.files.view`, `server.files.edit`, `server.sftp`, `server.data.view`, `server.data.edit`, `server.flags.view`, `server.flags.edit`, `server.definition.view`, `server.definition.edit`, `server.backup.view`, `server.backup.create`, `server.backup.restore`, `server.backup.delete`, `server.clients.view`, `server.clients.create`, `server.clients.edit`, `server.clients.delete`, `server.users.view`, `server.tasks.view`, `server.tasks.run` |

The server scopes of "Usuario" come from earlier versions, when the role gave every member control of their servers; they no longer have any effect on servers. `server.create` lets it split its own servers; creating a server with no parent is for administrators only.

- "Administrador" and "Usuario" cannot be deleted (`cannot delete a default role`). Deleting any other role leaves its users without a role.
- A role's description is stored in English ("Full access to all panel features (super-admin).", "Standard user: …").

Who gets which role:

- Customers created by the billing API (`POST /api/v1/provision`) get "Usuario".
- On upgrade, every user without a role was given "Usuario" (migrations `20260624-usuario-role-templates-view` and `20260625-assign-usuario-role-to-existing-users`).
- Someone who signs up on the sign-up page gets **no role**: only `login` in their global row.
- `SkyPanel user add --admin` puts `login` and `admin` in the user's global row, without a role.
- An administrator can pick a role when creating or editing a user in **Users**. Only administrators create, edit and delete roles; listing them (`GET /api/roles`) also works with `users.info.view` or `users.info.edit`. Only an administrator can change a user's role: the role sent by staff without `admin` is ignored.
- An account created in **Users** with no role holds no scope at all, not even `login`, so it cannot sign in until it gets a role or `login` in its global row (`PUT /api/users/:id/perms`, with `users.perms.edit`). The web page has no editor for the global row.
- A person invited to a server by email gets `login` in their global row and no role (see [Sub-users](#sub-users)).

### Editing other accounts

`users.info.edit` (edit and delete accounts) and `users.perms.edit` (change their permissions) are staff powers. A user who holds them without `admin` **cannot edit, delete or change the permissions of an account that holds a power they lack**. The panel compares the target's global row and role with the editor's scopes, for these scopes: `admin`, `settings.edit`, `nodes.create`, `nodes.edit`, `nodes.delete`, `nodes.deploy`, `templates.local.edit`, `templates.repo.create`, `templates.repo.delete`, `users.info.edit`, `users.perms.edit`, `oauth2.auth` and `server.admin` (in the global row it applies to every server). The answer is `403` ("you cannot modify an account with permissions you do not have"). Otherwise staff with `users.info.edit` alone could set the password of an account that edits the settings and sign in as it.

Also, without `admin`, `users.perms.edit` only sets or clears panel scopes the editor holds; the server scopes of someone's global row stay as they are, since they apply on every server.

Changing a user's password from **Users** (`POST /api/users/:id` with a `password`) ends all of that user's open sessions.

### Taking away sign-in

`login` is checked every time someone comes in, not only when the session is opened. It counts when it is in the user's global row or in their role (or when they hold `admin`):

| Way in | What happens without `login` |
|--------|------------------------------|
| Sign-in page (password, 2FA, sign-in after registering) | Refused with `403 ErrLoginNotPermitted`. |
| Any request with an open session (cookie or `Authorization` header) | The session is ended and the request answers `401`. So removing `login`, or the role that gave it, signs the person out on their next request instead of waiting for the session to expire. |
| SFTP, with password or SSH key | The connection is refused. |
| Sign-in links from billing (`/api/v1/sso`) | The browser is sent to `/login/?sso=denied`. |

A session that belonged to an OAuth2 client that no longer exists is ended the same way.

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
| `server.view` | See the server, its version, its uptime history, its map and the list of databases (without passwords; those need `server.data.edit`). Connect to the socket. | yes |
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
| `server.sftp` | Sign in over SFTP, with the password or an SSH key, on any node. | yes |
| `server.backup.view` | See and download backups. | yes |
| `server.backup.create` | Create backups. Without `admin`, up to `panel.backups.maxPerServer` backups on the node per server (10 by default; `0` turns the cap off; those only in the bucket do not count), then `409 ErrBackupLimit`. | yes |
| `server.backup.restore` | Restore backups. | yes |
| `server.backup.delete` | Delete backups. | yes |

### Settings

| Scope | What it allows | Owner |
|-------|----------------|-------|
| `server.data.view` | See the variables. | yes |
| `server.data.edit` | Edit the variables the template opens to the user (never limits, IP or ports, see [above](#variables-only-an-administrator-changes)), pick the primary port among the assigned ones, port notes, databases (including each one's password) and the subdomain. | yes |
| `server.data.edit.admin` | Admin variables (resources), suspending, transferring between nodes and external transfers. Unlocks the environment when editing the definition. | no |
| `server.flags.view` | See auto start and auto restart. `server.flags.edit` is also enough. | yes |
| `server.flags.edit` | Edit auto start and auto restart, the server's Discord alerts and the subdomain. | yes |
| `server.name.edit` | Rename. | yes |
| `server.definition.view` | See the definition. | yes |
| `server.definition.edit` | Edit the definition (see [Environment and definition](#environment-and-definition)). | no |
| `server.ports.edit` | Nothing checks it: it only appears as a box in the role editor. Ports are changed with `server.data.edit.admin`. | no |
| `server.delete` | Delete the server. | no |

### Tasks

| Scope | What it allows | Owner |
|-------|----------------|-------|
| `server.tasks.view` | See tasks. | yes |
| `server.tasks.run` | Run a task by hand. | yes |
| `server.tasks.create` | Create tasks. Creating and editing use the same route, which accepts either one. Without `admin`, each step also needs the scope of its action (`server.console.send` for a command, `server.start`/`server.stop` for power, `server.backup.create` for a backup); see [Scheduled tasks](../features/scheduled-tasks.md#checks-for-non-administrators). | yes |
| `server.tasks.edit` | Edit tasks. | yes |
| `server.tasks.delete` | Delete tasks. | yes |

### Users and clients

| Scope | What it allows | Owner |
|-------|----------------|-------|
| `server.users.view` | See the members and the server's activity. | yes |
| `server.users.edit` | Add sub-users and change their permissions (server scopes only). | yes |
| `server.users.delete` | Remove sub-users. | yes |
| `server.users.create` | No API route checks it: adding uses `server.users.edit`. | yes |
| `server.clients.view`, `server.clients.create`, `server.clients.edit`, `server.clients.delete` | No API route checks them today. | yes |

### Administration

| Scope | What it allows | Owner |
|-------|----------------|-------|
| `server.admin` | Every server scope on that server. It also allows giving any scope to sub-users. | no |
| `server.admin.view` | No API route checks it; the web uses it to show the **Administration** tab. | no |
| `server.admin.install.view`, `server.admin.transfer.view`, `server.admin.config.view`, `server.admin.assignments.view` | No API route checks them; the web uses them to show that section of the **Administration** tab (`server.admin.config.view` also shows the resources in **Settings**). Acting in each section still needs the real scope: `server.install`, `server.data.edit.admin`, `server.definition.edit`. | no |
| `server.admin.install.manage`, `server.admin.transfer.manage`, `server.admin.config.manage`, `server.admin.assignments.manage` | No API route checks them (`server.admin.config.manage` only shows the resources in **Settings**). | no |

## Panel scopes (23)

| Scope | What it allows |
|-------|----------------|
| `admin` | Everything, including sign-in. Only `admin` manages roles, API keys, extensions, database hosts, products and server owners, opens the API docs (`/swagger`), creates servers without a parent, writes any step into a task and sees every server. |
| `login` | Signing in (see [Taking away sign-in](#taking-away-sign-in)), seeing your own profile, your user settings, the active extensions, game versions and reading templates. |
| `panel` | Nothing checks it. |
| `self.edit` | Editing your own profile and password, 2FA (including recovery codes) and SSH keys. |
| `self.clients` | Creating and deleting your own OAuth2 clients. |
| `oauth2.auth` | Validating users' credentials over OAuth2. |
| `server.create` | Creating servers. Without `admin`, only sub-servers of a server you own (see [Sub-servers](../features/sub-servers.md)). Also cloning, when the experiment is on and you are a member of the original (see [Cloning](../features/cloning.md)), and checking whether a port is free on a node (`POST /api/nodes/:id/ports/check`). |
| `settings.edit` | Editing the panel settings, sending the test email, Discord message, S3 and Cloudflare checks, activating the license, checking for and starting panel updates, and seeing the domain status (see [Panel settings](../features/panel-settings.md) and [Panel updates](../features/panel-updates.md)). |
| `nodes.view` | Seeing nodes, their features, system data and network history. |
| `nodes.create` | Creating nodes. |
| `nodes.edit` | Editing nodes. |
| `nodes.delete` | Deleting nodes. |
| `nodes.deploy` | Seeing a node's deployment data (it includes secrets). |
| `templates.view` | No API route checks it: reading templates asks for `login`. The web uses it to show **Templates** in the menu. |
| `templates.local.edit` | Uploading and editing local templates. |
| `templates.repo.create` | Adding template repositories. |
| `templates.repo.delete` | Deleting template repositories. |
| `users.info.search` | Searching users and seeing the panel's activity (see [Activity and access log](../features/activity-and-access-log.md)). |
| `users.info.view` | Seeing a user and listing roles. |
| `users.info.edit` | Creating, editing and deleting users, except accounts with powers the editor lacks (see [Editing other accounts](#editing-other-accounts)). |
| `users.perms.view` | Seeing a user's permissions. |
| `users.perms.edit` | Changing a user's panel permissions, within the same limit. |
| `uptime.view` | Seeing the uptime history (`GET /api/uptime`). Without `admin`, only of the servers the user is a member of. |
