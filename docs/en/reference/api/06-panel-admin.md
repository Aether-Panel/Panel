# Panel administration

Nodes, panel settings, API keys, templates, game versions, database hosts, extensions and uptime. Back to the [API reference](../11-api-reference.md). Provisioning products are in [Provisioning / billing](./07-provisioning.md#products).

## Routes

| Method | Path | Scope | What it does |
|--------|------|-------|--------------|
| GET | `/api/nodes` | `nodes.view` | Lists nodes |
| POST | `/api/nodes` | `nodes.create` | Adds a node |
| GET | `/api/nodes/network-history` | `nodes.view` | Traffic of all nodes added up |
| GET | `/api/nodes/:id` | `nodes.view` | One node |
| PUT | `/api/nodes/:id` | `nodes.edit` | Updates a node |
| DELETE | `/api/nodes/:id` | `nodes.delete` | Deletes a node without servers |
| GET | `/api/nodes/:id/features` | `nodes.view` | OS, architecture, environments, Docker |
| GET | `/api/nodes/:id/system` | `nodes.view` | CPU, memory, disks, network counters |
| POST | `/api/nodes/:id/ports/check` | `server.create` | Which ports are taken, with free alternatives |
| GET | `/api/nodes/:id/deployment` | `nodes.deploy` | Credentials to deploy the node's daemon |
| POST | `/api/nodes/:id/join` | `nodes.deploy` | One-time join code and the command that installs the node |
| POST | `/api/node-join` | Public | Trades a join code for the node's credentials |
| GET | `/api/settings` | `settings.edit` | Every editable setting |
| POST | `/api/settings` | `settings.edit` | Saves several settings |
| GET | `/api/settings/:key` | `settings.edit` | One setting |
| PUT | `/api/settings/:key` | `settings.edit` | Saves one setting |
| POST | `/api/settings/test/email` | `settings.edit` | Sends a test email |
| POST | `/api/settings/test/discord` | `settings.edit` | Posts to every configured Discord webhook |
| POST | `/api/settings/test/s3` | `settings.edit` | Checks the off-site backup storage |
| POST | `/api/settings/test/cloudflare` | `settings.edit` | Checks the Cloudflare token, zone and domain |
| POST | `/api/settings/license/activate` | `settings.edit` | Activates a license key |
| POST | `/api/settings/update-panel` | `settings.edit` | Starts an update of the panel and every node |
| GET | `/api/settings/update-check` | `settings.edit` | Whether a newer version is available |
| GET | `/api/settings/update-status` | `settings.edit` | Progress of the last update on each node |
| GET | `/api/settings/domain-status` | `settings.edit` | What the installer did with the panel domain |
| GET | `/api/settings/node-sync` | `settings.edit` | Whether every remote node has the panel's node settings |
| GET | `/api/settings/apikeys` | `admin` | Lists billing API keys |
| POST | `/api/settings/apikeys` | `admin` | Creates one |
| DELETE | `/api/settings/apikeys/:id` | `admin` | Deletes one |
| GET | `/api/templates` | `login` | Template repositories |
| POST | `/api/templates` | `templates.repo.create` | Adds a repository |
| POST | `/api/templates/upload` | `templates.local.edit` | Uploads local templates (`.json` files) |
| GET | `/api/templates/:repo` | `login` | Templates of a repository |
| DELETE | `/api/templates/:repo` | `templates.repo.delete` | Removes a repository |
| GET | `/api/templates/:repo/:name` | `login` | One template |
| PUT | `/api/templates/0/:name` | `templates.local.edit` | Creates or replaces a local template |
| DELETE | `/api/templates/0/:name` | `templates.local.edit` | Deletes a local template |
| GET | `/api/game-versions/:platform` | `login` | Stable releases of a Minecraft platform |
| GET | `/api/databasehosts` | `admin` | Lists MySQL hosts |
| POST | `/api/databasehosts` | `admin` | Adds a host |
| POST | `/api/databasehosts/test` | `admin` | Tests a connection |
| GET | `/api/databasehosts/:id` | `admin` | One host |
| PUT | `/api/databasehosts/:id` | `admin` | Updates a host |
| DELETE | `/api/databasehosts/:id` | `admin` | Deletes a host |
| GET | `/api/extensions` | `admin` | Installed extensions |
| POST | `/api/extensions` | `admin` | Installs or updates an extension (zip) |
| GET | `/api/extensions/active` | `login` | Pages of the switched-on extensions |
| PUT | `/api/extensions/:id` | `admin` | Switches on/off, changes the webhook |
| DELETE | `/api/extensions/:id` | `admin` | Uninstalls |
| POST | `/api/extensions/:id/test` | `admin` | Sends a test event |
| GET, HEAD | `/api/extension-files/:id/*path` | Public | Files of a switched-on extension |
| GET | `/api/uptime` | any of `admin`, `uptime.view` | Uptime of every server you can see |
| GET | `/api/uptime/:serverId` | `server.view` | Uptime of one server |

## Nodes

Details of the dashboard and node pages: [Dashboard and nodes](../../features/dashboard-and-nodes.md).

### The node object

```json
{ "id": 1, "name": "node-1", "publicHost": "node1.example.com", "privateHost": "10.0.0.5", "publicPort": 8080, "privatePort": 8080, "sftpPort": 5657, "isLocal": false }
```

`isLocal` marks the node that runs inside the panel's own process.

### `POST /api/nodes`

Body: the node object without `id`. On creation every field is required: `name` (printable ASCII that needs no URL escaping, so no spaces), `publicHost` (IP or domain), `privateHost`, and the three ports (1 to 65535); `sftpPort` cannot equal `publicPort` or `privatePort`. The panel generates the node's secret. Answers the node object. Validation errors are `400` with `ErrFieldRequired`, `ErrFieldIsInvalidHost`, `ErrFieldHasURICharacters`, `ErrFieldNotBetween` or `ErrFieldEqual`.

### `PUT /api/nodes/:id`

Same body; empty fields and `0` ports keep their value. `204`.

### `DELETE /api/nodes/:id`

`204`. A node that still has servers answers `500` with code `ErrNodeHasServers`; the panel's own node cannot be deleted (`500`).

### `GET /api/nodes/:id/deployment`

```json
{ "clientId": ".node_2", "clientSecret": "4f9c...", "publicKey": "" }
```

`clientId` and `clientSecret` go into the node's `daemon.auth.clientId` and `daemon.auth.clientSecret`. `publicKey` is always empty: the node reads the panel's keys from `GET /auth/publickey`.

### `POST /api/nodes/:id/join`

```json
{ "code": "k3v7…", "expiresAt": "2026-11-05T05:10:00Z", "panelUrl": "https://panel.example.com",
  "command": "curl -fsSL https://raw.githubusercontent.com/Aether-Panel/Panel/main/tools/install/install.sh | sudo bash -s -- --node --panel https://panel.example.com --join k3v7…" }
```

The code (32 characters) works once and for an hour, and a new code for the same node cancels the previous one. The panel keeps only its hash, in memory: restarting it cancels pending codes. `panelUrl` is the base URL from Settings; when it is not set or is local (`localhost`), the answer is `400 ErrNodeJoinURL`. The panel's own node answers `400 ErrNodeJoinLocal`.

### `POST /api/node-join`

Public: the node's installer calls it with `{ "code": "…" }`.

```json
{ "nodeId": 2, "name": "eu-2", "clientId": ".node_2", "clientSecret": "4f9c…", "port": 8080, "sftpPort": 5657 }
```

`port` is the node's private port (the one the daemon listens on). A code that does not exist, was used or has expired answers `404 ErrNodeJoinCode`; more than 10 tries per address in 15 minutes, `429 ErrTooManyAttempts`.

### `GET /api/nodes/:id/features`

```json
{ "features": ["docker"], "environments": ["docker"], "os": "linux", "arch": "amd64", "version": "3.x.y" }
```

`features` holds `docker` when the node can reach Docker. With `docker.disallowHost` on, the host environments (`host`, `tty`, `standard`, `bubblewrap`) are left out of `environments`.

### `GET /api/nodes/:id/system`

```json
{
  "hostname": "node1", "os": "linux", "platform": "ubuntu", "platformVersion": "24.04", "arch": "amd64",
  "cpuModel": "AMD EPYC", "cpuCores": 4, "cpuThreads": 8, "cpuUsage": 12.5,
  "memoryTotal": 17179869184, "memoryUsed": 6442450944, "memoryFree": 2147483648, "memoryAvailable": 10737418240,
  "disks": [ { "path": "/", "total": 214748364800, "used": 53687091200, "free": 161061273600, "usedPercent": 25 } ],
  "serversDisk": { "path": "servers", "total": 214748364800, "used": 53687091200, "free": 161061273600, "usedPercent": 25, "low": false },
  "uptime": 864000, "networkBytesSent": 123456789, "networkBytesRecv": 987654321
}
```

`cpuUsage` is sampled in the background every two seconds. Each physical disk is listed once. `serversDisk` is the disk holding the servers' folders; `low` is true under 10% or 5 GB free, and the panel warns administrators then. `networkBytes*` add up the physical interfaces; in a container they are the host's when `daemon.hostProc` is set.

### `POST /api/nodes/:id/ports/check`

Used by the new-server form before it creates anything.

```json
{ "ports": { "port": 25565, "queryport": 25565 }, "exclude": "" }
```

`ports` maps each port variable of the form to the port it asks for; `exclude` is a server being edited, whose own ports count as free.

```json
{ "ports": { "port": { "busy": true, "reason": "server", "by": "Survival", "suggestion": 25566 }, "queryport": { "busy": false } } }
```

`reason` is `range` (below 1024), `panel` (the node's own ports and the panel's containers), `server` or `container`. `by` names the holder and is only sent to administrators. `suggestion` is the next port nobody holds and the form does not ask for. Busy ports come from the panel's records and from the containers the node reports running.

### `GET /api/nodes/network-history?range=1h`

`range` is `1h`, `6h` or `24h` (a point every 1, 5 or 15 minutes; anything else answers `400`). Each node keeps a day of traffic on disk and answers `GET /daemon/network/history`; the panel adds the nodes up step by step.

```json
{ "step": 60, "points": [ { "t": 1793576400, "rx": 0.49, "tx": 0.37 } ], "nodes": 2, "unavailable": 0 }
```

`rx` and `tx` are KB/s averaged over the step. A step no node measured has no point. `unavailable` counts the nodes that did not answer (down, or too old to keep a history).

## Settings

What each setting does: [Panel settings](../../features/panel-settings.md) and [Configuration](../../backend/02-configuration.md).

### `GET /api/settings`

An object with every editable key and its current value, plus `panel.settings.masterNodeIp` (kept in the database, not in the config file).

Secrets are never sent back: a saved secret reads as `********` and an unset one as `""`, so a client can tell whether it is set. The masked keys are `panel.email.key`, `panel.email.password`, `panel.settings.geminiApiKey`, `daemon.curseforge.key`, the four `panel.notifications.discordWebhook*` keys, `panel.license.key`, `daemon.backups.s3.secretKey`, `panel.dns.cloudflare.token` and `panel.turnstile.secretKey`. The same masking applies to `GET /api/settings/:key`.

Editable keys:

| Group | Keys |
|-------|------|
| Branding | `panel.settings.companyName`, `panel.settings.defaultTheme`, `panel.settings.themeSettings` |
| Addresses | `panel.settings.masterUrl` (saved without a trailing slash and with `https://` in front when only the host name arrives, `panel.example.com`), `panel.settings.nodeIp`, `panel.settings.masterNodeIp` |
| Sign-up and anti-bot | `panel.registrationEnabled`, `panel.registrationApproval`, `panel.turnstile.enabled`, `panel.turnstile.siteKey`, `panel.turnstile.secretKey`, `panel.captcha.builtin` |
| Email | `panel.email.provider`, `panel.email.from`, `panel.email.host`, `panel.email.username`, `panel.email.password`, `panel.email.domain`, `panel.email.key` |
| Notifications | `panel.settings.notificationLanguage` (`es` or `en`, default `es`: the language of Discord, Telegram and webhook alerts, the panel's Discord reports and extension events), `panel.notifications.discordWebhook`, `panel.notifications.discordWebhookSystem`, `panel.notifications.discordWebhookNode`, `panel.notifications.discordWebhookExTransfer` |
| AI and integrations | `panel.settings.geminiApiKey`, `panel.settings.hideAiAnalysis`, `daemon.curseforge.key` |
| Experimental | `panel.experimental.versionSwitch`, `panel.experimental.propertiesEditor`, `panel.experimental.serverClone` |
| Off-site backups | `daemon.backups.s3.enabled`, `daemon.backups.s3.endpoint`, `daemon.backups.s3.region`, `daemon.backups.s3.bucket`, `daemon.backups.s3.accessKey`, `daemon.backups.s3.secretKey`, `daemon.backups.s3.prefix`, `daemon.backups.s3.insecure`, `daemon.backups.s3.periodicHours` (`0`, `12`, `24`, `48`, `72`, `168`, `336` or `720`; any other value: `400 ErrOffsitePeriodicHours`), `daemon.backups.s3.periodicKeep` (1 to 50; any other value: `400 ErrOffsiteKeep`) |
| Subdomains | `panel.dns.cloudflare.token`, `panel.dns.cloudflare.zoneId`, `panel.dns.baseDomain`, `panel.dns.target`, `panel.dns.autoAssign` |
| GitHub | `panel.github.clientId` |
| License | `panel.license.key`, `panel.license.status`, `panel.license.serverId`, `panel.license.serverIp` |

Other configuration keys (for example `panel.backups.maxPerServer`) are only set in the config file or environment.

### `POST /api/settings`

An object of keys and values: `{"panel.settings.companyName": "My Hosting", "panel.registrationEnabled": false}`. Every value is checked first (Discord webhooks must be valid Discord webhook URLs, `panel.settings.notificationLanguage` must be `es` or `en`: `400`, with `ErrNotificationLanguage` for the latter, otherwise), so a bad value saves nothing. Text values are trimmed; unknown keys are ignored. A secret sent as `********` (the mask, unchanged) is dropped and keeps its saved value. `204`. When the save touches a node setting (`daemon.backups.s3.*` or `daemon.curseforge.key`), the panel then sends them to every remote node in the background, without making the answer wait (see [`GET /api/settings/node-sync`](#get-apisettingsnode-sync)).

### `GET /api/settings/:key` and `PUT /api/settings/:key`

`GET` answers `{"value": ...}` (secrets masked as above), or `204` for a key that is not editable. `PUT` takes `{"value": ...}` and answers `204`; for a secret key, a value of `********` changes nothing. A node setting saved this way also reaches the remote nodes, as with `POST /api/settings`.

### Tests

The test routes take the values typed in the form, not saved yet, as overrides (same key names); missing ones use the saved values, and so does a secret sent back as `********`.

| Route | Overrides | Answer |
|-------|-----------|--------|
| `POST /test/email` | `panel.email.*` | `204`, sent to the caller's email; `422` with the mail server's reason |
| `POST /test/discord` | The four `panel.notifications.discordWebhook*` keys | `204`; `400 ErrDiscordNoWebhooks` when none is configured, `400` when one is invalid; `422 ErrDiscordTestFailed` naming the webhooks the message did not reach in `metadata.webhooks` (`alerts`, `reports`, `status`, `transfers`, which the settings page shows with the names of its sections). The message is written in the notification language |
| `POST /test/s3` | `daemon.backups.s3.*` | `204`, after writing and deleting a small object; `400` when not configured; `422` with the storage's answer |
| `POST /test/cloudflare` | `panel.dns.cloudflare.token`, `panel.dns.cloudflare.zoneId`, `panel.dns.baseDomain` | `200 {"zone": ...}` after creating and removing a TXT record; `400` when not configured; `422` with Cloudflare's answer |

### `POST /api/settings/license/activate`

`{"key": "XXXX-XXXX-XXXX-XXXX"}` (16 characters without dashes). Verifies and binds the key with the license server and saves it. `200 {"success": true, "type": "...", "license": {...}, "permissions": ..., "bound": true, "message": "License activated successfully"}`; `400` with `{"success": false, "message": "..."}` when it is not valid. Details: [License](../../general/14-license.md).

### Updates

- `POST /api/settings/update-panel` starts an update on the panel's host and on every node (`/daemon/update`). `200 {"message": "Update initiated on the panel and all nodes.", "nodes": [{"name": "node-1", "local": true}, {"name": "node-2", "error": "node returned status 500"}]}`.
- `GET /api/settings/update-check` compares the running commit with the tip of its branch on GitHub: `{"current": "<sha>", "latest": "<sha>", "version": "3.x.y", "branch": "main", "updateAvailable": true, "behindBy": 3}`. `reason` (`github_unreachable`, `github_rate_limited`, `github_error`) explains a failed check; `verification` (`pending` or `failed`) holds back an update whose CI checks have not passed.
- `GET /api/settings/update-status`: `{"nodes": [{"name", "address", "local", "online", "running", "containerId", "startedAt", "finishedAt", "exitCode", "log", "error"}]}`.

Details: [Panel updates](../../features/panel-updates.md).

### `GET /api/settings/domain-status`

What the installer's domain service did with the base URL: `{"state": "...", "url": "...", "domain": "...", "https": true, "proxied": false, "serverIp": "...", "message": "...", "updatedAt": "..."}`. `proxied` means the domain does not point at this server (as with Cloudflare's proxy), so game ports and SFTP use `serverIp`.

### `GET /api/settings/node-sync`

Whether every remote node has the panel's node settings: the off-site backups (`daemon.backups.s3.*`, the periodic copy's defaults included) and the CurseForge key (`daemon.curseforge.key`). The panel sends them to each remote node itself (`PUT /daemon/settings`, see [Daemon](./09-daemon.md#put-daemonsettings)) after a save that changes one of them and every 10 minutes, the first time a minute after it starts. The panel's own node shares its `config.json` and is not listed.

```json
[
  { "id": 2, "name": "node-2", "state": "synced", "checkedAt": "2026-11-06T10:00:00Z" },
  { "id": 3, "name": "node-3", "state": "failed", "error": "the off-site copies to keep must be between 1 and 50", "checkedAt": "2026-11-06T10:00:01Z" }
]
```

One item per remote node, in id order; `[]` without remote nodes. Never includes a setting's value.

| `state` | Meaning |
|---------|---------|
| `synced` | The node has the same values as the panel |
| `pending` | Not checked yet (`checkedAt` is `null`), or the node did not answer or could not save; the next pass tries again by itself |
| `outdated` | The node runs a version without `/daemon/settings` (it answered `404`) and needs updating |
| `failed` | The node refused the panel (wrong secret, clocks off) or the settings; `error` says why |

## API keys

Keys for the billing routes (`/api/v1`). How they are used and limited: [Provisioning / billing](./07-provisioning.md#authentication).

- `GET /api/settings/apikeys`: `[{"id": 1, "name": "WHMCS", "prefix": "ak_a1b2c", "permissions": ["create", "suspend"], "created_at": "..."}]`.
- `POST /api/settings/apikeys`: `{"name": "WHMCS", "permissions": ["create", "terminate", "suspend", "change-package", "sso"]}`. `name` is required. Repeated or unknown permissions are ignored; a list with no known permission answers `400 ErrAPIKeyPermissions`; no list at all gives the key every action (`provision`). Answers `{"token": "ak_...", "key": {...}}`; the token is only shown here.
- `DELETE /api/settings/apikeys/:id`: `204`.

## Templates

Templates are server definitions. Repository `0` is the local one, kept in the panel's database. When `templates.url` is set, the list is that index (shown as repository `1`, "community") plus the local one, and the database repositories are not used. Details: [Templates](../../features/templates.md).

Reading templates only needs `login`, because the new-server and Splitter forms read them; `templates.view` only shows the templates page in the web client.

| Route | Notes |
|-------|-------|
| `GET /api/templates` | `[{"id": 0, "name": "Local", "url": "", "branch": "", "isLocal": true}]` |
| `POST /api/templates` | `{"name": "mine", "url": "https://github.com/me/templates.git", "branch": "main"}`, plus optional credentials for a private repository: `pat`, `username`, `password`, `sshKey` (see below). Answers the repository, never its credentials; `400 ErrFieldRequired` without `name` |
| `GET /api/templates/:repo` | The repository's templates |
| `GET /api/templates/:repo/:name` | One template: the definition plus `name` and `readme`; `404` when unknown |
| `DELETE /api/templates/:repo` | `204` |
| `PUT /api/templates/0/:name` | Body: a definition. Creates or replaces the local template. `204` |
| `DELETE /api/templates/0/:name` | `204`; `404` when unknown |
| `POST /api/templates/upload` | Multipart field `files`, one or more `.json` files of up to 2 MB; the template name is the file name (`minecraft.json` → `minecraft`). Answers the created templates; `400 ErrInvalidTemplate` for a file that is not JSON or has neither install steps nor a run command |

Private repositories: over HTTPS, `pat` (a personal access token, sent with the user `username`, or `git` when empty) or `username` + `password`; over SSH (`git@host:owner/repo.git` or `ssh://...`), `sshKey` holds the private key, `password` its passphrase, and the user is `username`, else the one in the URL, else `git`. For SSH the host must be in the `known_hosts` of the account that runs the panel. The web client has no form for adding repositories yet. Internals: [Services → Template](../../backend/11-services.md#11-template-templatesgo).

### `GET /api/game-versions/:platform`

The stable releases of `vanilla`, `paper`, `folia`, `velocity` or `waterfall`, newest first: `["1.21.4", "1.21.3", ...]`. `400` for another platform; `502` when Mojang or PaperMC cannot be reached.

## Database hosts

MySQL servers the panel creates server databases on. Details: [Database hosts](../../features/database-hosts.md).

```json
{ "id": 1, "name": "Main MySQL", "host": "db1.example.com", "port": 3306, "username": "panel", "max_databases": 50, "node_id": null, "created_at": "...", "updated_at": "..." }
```

The password is never returned. `max_databases` `0` means no limit; `node_id` ties the host to one node's servers (`null`: every node).

- `POST /api/databasehosts`: the object plus `password`; `name`, `host`, `port`, `username` and `password` are required. Answers the host.
- `PUT /api/databasehosts/:id`: same fields; an empty `password` keeps the current one. `204`.
- `POST /api/databasehosts/test`: `{"host": "...", "port": 3306, "username": "...", "password": "..."}` (port defaults to 3306). `200 {"status": "ok"}` or `400` with the connection error.
- `DELETE /api/databasehosts/:id`: `204`; `409 ErrDatabaseHostInUse` while the host still has databases (delete them first).

## Extensions

Extension packages, their pages and events are described in [Extensions](../15-extensions.md).

```json
{
  "id": "hello", "name": "Hello", "version": "1.0.0", "author": "...", "description": "...",
  "enabled": false, "webhookUrl": "https://...", "secret": "...", "installedAt": "...", "updatedAt": "...",
  "pages": [ { "...": "...", "url": "/api/extension-files/hello/index.html" } ],
  "events": ["server.created"],
  "manifestWebhook": "https://..."
}
```

- `GET /api/extensions` lists them with their pages, events and webhook secret.
- `POST /api/extensions` takes the zip in the multipart field `file`. Installing an id already present updates it; a new extension starts switched off. A bad package answers `400 {"error": {"code": "ErrExtensionInvalid", "msg": "..."}}`.
- `GET /api/extensions/active`: `[{"id", "name", "pages"}]` of the switched-on extensions; administration pages are only listed for administrators.
- `PUT /api/extensions/:id`: `{"enabled": true, "webhookUrl": "https://..."}` (both optional). Answers the extension; `400` for an invalid webhook; `404`.
- `DELETE /api/extensions/:id`: `204`; `404`.
- `POST /api/extensions/:id/test`: posts a signed `test` event; `204`, or `400` with `ErrExtensionNoWebhook` or `ErrExtensionWebhook`.
- `GET /api/extension-files/:id/*path` is public and only serves switched-on extensions, always with a sandbox Content-Security-Policy.

## Uptime

Details: [Uptime](../../general/13-uptime.md).

Servers on the panel's own node are recorded by the panel every few seconds; servers on remote nodes by a poller that asks each node every 30 seconds (`GET /daemon/servers/status`). While a remote node does not answer nothing is recorded for it, and after 3 minutes without an answer its open periods are closed at their last confirmation, so the outage counts as neither uptime nor downtime. Periods that overlap the window are counted only for the part inside it, and the running period up to now.

### `GET /api/uptime?days=30`

`days` defaults to 30, at most 365. An object keyed by server id; users without `admin` only get their own servers.

```json
{
  "3F9A1C2B7D4E": { "serverName": "Survival", "nodeName": "LocalNode", "uptime": 2505600, "downtime": 86400, "uptimePercent": 96.67, "currentStatus": true, "currentStartTime": "..." }
}
```

`currentStatus` is asked of each node (5 seconds at most per node); servers without history get an entry built from their current state.

### `GET /api/uptime/:serverId?days=30&limit=50`

```json
{
  "serverId": "3F9A1C2B7D4E", "serverName": "Survival", "nodeName": "LocalNode",
  "currentStatus": true, "currentStartTime": "...", "currentUptime": 3600,
  "uptimeSeconds": 2505600, "downtimeSeconds": 86400, "totalSeconds": 2595600, "uptimePercent": 96.67,
  "period": { "days": 30, "since": "...", "until": "..." },
  "history": [ ... ]
}
```

`limit` is the number of history entries, default 50, at most 200. `totalSeconds` is `uptimeSeconds + downtimeSeconds`; both already include the running period, which `currentUptime` shows on its own.
