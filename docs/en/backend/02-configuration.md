# Configuration

Aether Panel reads its configuration with **Viper**. Every option is a key declared in `internal/config/entries.go` with its type and built-in default; `internal/config/config.go` finds the file and maps environment variables. This page lists **every key the code declares** (97), grouped by area.

## Where values come from

### The configuration file

The file is looked up in this order (`config.LoadConfigFile`):

1. The `--config <path>` flag of the binary.
2. The `SKYPANEL_CONFIG` environment variable.
3. `/etc/SkyPanel/config.json`, if it exists.
4. `config.json` in the working directory (`--workDir` changes it before the lookup).

Viper picks the format from the file extension; the shipped files (`config.docker.json`, `config.linux.json`) and the installer use JSON. Nested keys are nested objects: `panel.database.dialect` is `{"panel": {"database": {"dialect": "..."}}}`. Keys are case-insensitive.

If the file cannot be read, the binary prints `Error loading config, this may impact features` and carries on with the environment and the built-in defaults.

The Docker image copies `config.docker.json` to `/etc/SkyPanel/config.json`; `docker-compose.yml` mounts `./storage/skypanel-config` there so the file survives updates.

### Environment variables

Every key can be set with an environment variable: the prefix `SKYPANEL_`, then the key in upper case with each `.` replaced by `_`.

| Key | Variable |
|---|---|
| `web.host` | `SKYPANEL_WEB_HOST` |
| `panel.database.url` | `SKYPANEL_PANEL_DATABASE_URL` |
| `docker.disallowHost` | `SKYPANEL_DOCKER_DISALLOWHOST` |
| `panel.web.cookies.sameSite` | `SKYPANEL_PANEL_WEB_COOKIES_SAMESITE` |
| `logRetentionDays` | `SKYPANEL_LOGRETENTIONDAYS` |

Notes:

- An **empty** variable counts as unset (Viper ignores empty variables), so `SKYPANEL_DOCKER_ROOT=` in `docker-compose.yml` leaves the default in place. To clear a list such as `docker.prefetchImages`, use the file (`"prefetchImages": []`).
- A list key (`security.trustedProxies`, `docker.prefetchImages`) set from a variable is split on spaces: `SKYPANEL_SECURITY_TRUSTEDPROXIES="10.0.0.0/8 172.16.0.0/12"`.
- Two variables are read directly by the code and are not configuration keys: `SKYPANEL_CONFIG` (file location, above) and `SKYPANEL_PLATFORM`. The image and compose set `SKYPANEL_PLATFORM=docker`; with it the panel keeps running if the daemon fails to start, and the daemon looks for its own container to translate data paths into host paths (see `docker.root`).

### Precedence

From highest to lowest:

1. A value changed at runtime from the panel Settings (`/api/settings`): applied at once and written to the configuration file.
2. The environment variable.
3. The configuration file.
4. The built-in default in `entries.go`.

Because the panel writes Settings to the file, **after a restart an environment variable wins again** over what was saved from Settings. That is why `docker-compose.yml` does not set options that are meant to be changed in Settings (registration, theme, Turnstile).

### Values the panel generates and saves

| Key | When |
|---|---|
| `panel.sessionKey` | At panel start, if empty: 32 random bytes, hex encoded. It signs the `session` cookie and seeds the local node's secret. |
| `panel.token` | The first time the token service starts on a panel: a random Ed25519 seed, base64 encoded. It signs every panel→daemon request. |
| `panel.exTransferKey` | The first time a federated transfer key is needed. |
| `daemon.data.root` | After the file is read, if empty: the parent folder of `daemon.data.servers`. |
| `panel.license.serverId`, `panel.license.serverIp` | When a license is activated, if empty (hostname and detected address). |

The SFTP host key is a file, not a value: if `daemon.sftp.key` points to a missing file, an Ed25519 key is generated and written there.

### Data folders

Keys of the "data folder" kind accept an absolute path, or a relative one that is resolved against `daemon.data.root`: `daemon.sftp.key`, `daemon.data.cache`, `daemon.data.servers`, `daemon.data.backups.folder`, `daemon.data.binaries` and `panel.extensions.folder`. Other path keys (`logs`, `panel.web.files`, `panel.email.templateFolder`) are relative to the working directory.

## Configuration reference

The **Settings** column marks the keys an administrator can change from the panel (Settings page, `GET/POST /api/settings`, `GET/PUT /api/settings/{key}`, scope `settings.edit`). A dash in **Default** means an empty string.

### General and logging

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `web.host` | string | `0.0.0.0:8080` | | Address and port of the HTTP server (panel, API and `/daemon`). In the Docker install it stays `0.0.0.0:8080` inside the container; the installer decides which host address publishes the port (only `127.0.0.1` when the panel is served through a domain). |
| `logs` | string | `logs` | | Folder of the panel log. The Docker config uses `/var/log/SkyPanel`. |
| `logRetentionDays` | int | `14` | | Days rotated logs are kept; `0` keeps them all. |
| `templates.url` | string | — | | URL of a JSON template index. When set, the template repositories stored in the database are ignored and the panel offers this index (as the `community` repository) plus the local templates. `docker-compose.yml` sets `https://templates.aetherpanel.es/templates.json`. |
| `token.public` | string | — | | Only on a node without the panel (`panel.enable=false`): URL of the panel's public key set, `<panel>/auth/publickey`, used to check the requests the panel signs. The panel warns at start-up when it is empty on such a node. |

The panel writes the current day to `skypanel.log` inside `logs`. When the day changes, or the file passes 100 MB, it is archived as `skypanel-YYYY-MM-DD.log.gz` (`.1`, `.2`… when that day already has one) and a new file starts. `skypanel-*` archives last written more than `logRetentionDays` days ago are deleted. No `logrotate` is needed; if you use one, set `logRetentionDays` to `0` and send `SIGUSR1` so the panel reopens the file.

### Panel

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `panel.enable` | bool | `true` | | Runs the web panel: API, sign-in, OAuth2, web client and database. `false` turns the binary into a node that only serves `/daemon`. |
| `panel.registrationEnabled` | bool | `true` | Yes | Allows public sign-up (`POST /auth/register`). Administrators can still create accounts. |
| `panel.servers.autoInstall` | bool | `true` | | Runs the template's installation right after a server is created (also from billing), so it can be started without pressing Install. |
| `panel.backups.maxPerServer` | int | `10` | | Backups a server's users can keep; creating one more answers `409`. Administrators are not limited. `0` turns the cap off. Backups do not count against the server's disk. |
| `panel.web.files` | string | `www` | | Folder layered over the web client embedded in the binary: a file found here replaces the embedded one. Themes are read from its `theme/` subfolder (`*.tar`). The shipped configs use `/var/www/SkyPanel`. |
| `panel.extensions.folder` | data folder | `extensions` | | Where installed extensions keep their files, one folder per extension. See [Extensions](../reference/15-extensions.md). |

### Database

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `panel.database.dialect` | string | `sqlite3` | | `sqlite3`, `mysql`, `postgresql` or `sqlserver`. |
| `panel.database.url` | string | — | | Connection string. Empty means `file:skypanel.db` for SQLite and `SkyPanel:SkyPanel@/SkyPanel` for MySQL; PostgreSQL and SQL Server need one. |
| `panel.database.log` | bool | `false` | | Logs every SQL statement. |

The parameters added to each connection string are listed in [Database](./03-database.md).

### Session cookies

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `panel.web.cookies.secure` | bool | `false` | | Marks cookies `Secure`. The `skypanel_auth` cookie is already `Secure` whenever the request arrives over HTTPS (directly or with `X-Forwarded-Proto: https`); this forces it on every request. |
| `panel.web.cookies.httpOnly` | bool | `true` | | `HttpOnly` flag of the `session` cookie. |
| `panel.web.cookies.domain` | string | — | | `Domain` of the `session` cookie. |
| `panel.web.cookies.age` | int | `2592000` | | `Max-Age` of the `session` cookie, in seconds (30 days). |
| `panel.web.cookies.sameSite` | string | `Strict` | | `SameSite` of the `session` cookie: `Strict`, `Lax` or `None`; any other value means `Strict`. |
| `panel.web.cookies.path` | string | `/` | | `Path` of the `session` cookie. |

These keys configure the `session` cookie store, which only carries the sign-in between the password step and the two-factor step (it expires after 5 minutes). The cookie that holds the panel session, `skypanel_auth`, is always `HttpOnly`, `SameSite=Lax` and path `/`; only `panel.web.cookies.secure` affects it. See [API layer](./05-api-layer.md#authentication).

### Panel secrets

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `panel.sessionKey` | string | (generated) | | Key of the `session` cookie store; also seeds the stable secret of the local node. Changing it signs out the half-finished two-factor sign-ins and changes that secret. |
| `panel.token` | string | (generated) | | Ed25519 seed (base64) the panel signs its requests to the nodes with. Its public half is served at `/auth/publickey`. After a change, remote nodes refuse the panel's requests until they load the new public key set. |
| `panel.exTransferKey` | string | (generated) | | Seed of the key that signs federated (panel-to-panel) transfer requests. It is kept so a restart during a transfer does not invalidate it. |

### Branding and general settings

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `panel.settings.companyName` | string | `SkyPanel` | Yes | Name shown by the web client, in the web manifest, in emails and as the issuer of two-factor codes. |
| `panel.settings.defaultTheme` | string | `SkyPanel` | Yes | Active theme returned by `GET /api/config`. |
| `panel.settings.themeSettings` | string | `{}` | Yes | Theme settings (a JSON string) returned by `GET /api/config`. |
| `panel.settings.masterUrl` | string | `http://localhost:8080` | Yes | Public URL of the panel (the "Base URL"). Used for password-reset links and email templates, as an allowed origin by the cross-origin guard, and as the address of the local node (host and port; without a port, 80 or 443 by scheme). On a node without the panel it is the panel's URL. |
| `panel.settings.nodeIp` | string | `0.0.0.0` | Yes | Address of this machine. A node registered with this address counts as the local node, and subdomains of servers on the local node point to it when it is a public address. |
| `panel.settings.geminiApiKey` | string | — | Yes | Google Gemini key for the AI log analysis and assistant. Without it those endpoints fail. |
| `panel.settings.hideAiAnalysis` | bool | `false` | Yes | Turns the AI log analysis off for every user: the web client hides the console card and the crash report's "Explain with AI" button (it reads the switch from `GET /api/config`, field `hideAiAnalysis`), and `POST /api/servers/{id}/ai/analyze` answers `403 ErrAIAnalysisDisabled`. The AI chat is not affected. |

`panel.settings.masterNodeIp` also appears in the Settings API, but it is not a configuration key: it is saved in the database (`panel_settings` table, key `master_node_ip`). When set, it is the address players and SFTP clients get for the local node, instead of the host of `panel.settings.masterUrl`.

### Sign-in protection

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `panel.captcha.builtin` | bool | `true` | Yes | Built-in anti-bot check on sign-in, sign-up and password reset: the browser solves a small proof of work (`GET /auth/challenge`), with no outside service. |
| `panel.turnstile.enabled` | bool | `false` | Yes | Uses Cloudflare Turnstile instead of the built-in check. It only takes over when the site and secret keys are also set. |
| `panel.turnstile.siteKey` | string | — | Yes | Turnstile site key (public, returned by `GET /api/config`). |
| `panel.turnstile.secretKey` | string | — | Yes | Turnstile secret key. |

See [Turnstile](../general/17-turnstile.md).

### Email

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `panel.email.provider` | string | — | Yes | `smtp`, `mailgun`, `mailjet`, `sendgrid` or `debug`. Empty disables email. |
| `panel.email.from` | string | — | Yes | Sender address (all providers). |
| `panel.email.host` | string | — | Yes | SMTP server (`smtp`). |
| `panel.email.username` | string | — | Yes | SMTP user (`smtp`). |
| `panel.email.password` | string | — | Yes | SMTP password (`smtp`; spaces are removed, so app passwords can be pasted as shown). |
| `panel.email.domain` | string | — | Yes | Mailgun sending domain (`mailgun`), or the API public key (`mailjet`). |
| `panel.email.key` | string | — | Yes | API key (`mailgun`, `sendgrid`), or the API private key (`mailjet`). |
| `panel.email.templateFolder` | string | — | | Folder layered over the embedded email templates. |

`POST /api/settings/test/email` sends a test message with the values in the request body without saving them. See [Email](../general/15-email.md).

### Notifications

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `panel.notifications.discordWebhook` | string | — | Yes | Discord webhook for server alerts (crash, offline/online, resources, backups). |
| `panel.notifications.discordWebhookSystem` | string | — | Yes | Discord webhook for the periodic system summary. |
| `panel.notifications.discordWebhookNode` | string | — | Yes | Discord webhook for the node status. While it is empty the node report is skipped and remote nodes are not asked for their hardware. |
| `panel.notifications.discordWebhookExTransfer` | string | — | Yes | Discord webhook for federated transfers. |
| `panel.settings.notificationLanguage` | string | `es` | Yes | Language (`es` or `en`) of what the panel sends outside its own pages: the panel's Discord alerts and reports, each owner's Discord, Telegram and JSON webhook alerts and their test, the title and message of extension events, and the external transfer reports. Any other value is refused when saving (`ErrNotificationLanguage`). In the web client: Settings → General → Notifications. Environment variable `SKYPANEL_PANEL_SETTINGS_NOTIFICATIONLANGUAGE`. The panel sends its value to every remote node in the `X-Aether-Notification-Language` header of the status poll (every 30 seconds, and at once when the setting is saved with a new value). On a node the order is: a value set in that node's own config file or environment, then the panel's value, then `es`; the panel's value is kept in memory, so after a node restart it is `es` (or the node's own) until the next poll. The panel's pages follow each user's own language. |

Only `https://discord.com/api/webhooks/...` URLs (or an empty value) are accepted when saving. See [Discord](../general/16-discord.md). Per-server alerts set by each owner are not configuration: they live next to the server (see [Servers](./04-servers.md#per-server-files-on-the-node)).

### License

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `panel.license.key` | string | — | Yes | License key, saved by `POST /api/settings/license/activate`. |
| `panel.license.status` | string | `free` | Yes | License type returned by the license server on activation. |
| `panel.license.serverId` | string | — | Yes | Server identifier sent when binding the license (hostname by default). |
| `panel.license.serverIp` | string | — | Yes | Server address sent when binding the license. |

See [License](../general/14-license.md).

### Experimental features

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `panel.experimental.versionSwitch` | bool | `false` | Yes | One-click game version change. While off, `POST /api/servers/{id}/version` answers `403`. |
| `panel.experimental.propertiesEditor` | bool | `false` | Yes | Shows the form editor for `server.properties` in the web client (flag only, returned by `GET /api/config`). |
| `panel.experimental.serverClone` | bool | `false` | Yes | Server cloning. While off, `POST /api/servers/{id}/clone` answers `403`. |

### Server subdomains (Cloudflare DNS)

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `panel.dns.cloudflare.token` | string | — | Yes | Cloudflare API token; it only needs Zone → DNS → Edit on the zone. |
| `panel.dns.cloudflare.zoneId` | string | — | Yes | Zone ID. |
| `panel.dns.baseDomain` | string | — | Yes | Domain servers get names under (`survival` → `survival.<baseDomain>`). Token, zone and base domain are all needed. |
| `panel.dns.target` | string | — | Yes | Where the records point (IP or hostname) when a node is registered by hostname or a private address. Empty uses each node's address. |
| `panel.dns.autoAssign` | bool | `false` | Yes | Gives every new server a subdomain derived from its name (`-2`, `-3`… on a clash). |

See [Subdomain (Cloudflare)](./04-servers.md#subdomain-cloudflare) and [Subdomains](../features/subdomains.md).

### Daemon

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `daemon.enable` | bool | `true` | | Runs the daemon: `/daemon` routes, SFTP and the game servers of this machine. |
| `daemon.data.root` | string | (generated) | | Base folder for the relative data folders below. The shipped configs use `/var/lib/SkyPanel`. |
| `daemon.data.servers` | data folder | `servers` | | Server folders and their side files (`<id>.json`, `<id>.cron`…). |
| `daemon.data.backups.folder` | data folder | `backups` | | Backups, one subfolder per server. |
| `daemon.data.cache` | data folder | `cache` | | Download cache. |
| `daemon.data.binaries` | data folder | `binaries` | | Tools the panel downloads (Java, DepotDownloader…). Added to the daemon's `PATH` and mounted in containers at `/var/lib/SkyPanel/binaries`. |
| `daemon.data.crashLimit` | int | `3` | | Automatic restarts after crashes in a row (see [crash handling](./04-servers.md#crashes-and-automatic-restarts)). |
| `daemon.minFreeDiskMB` | int | `2048` | | Free space (MB) the disk holding the servers must keep: an install that would start with less is refused. `0` turns the check off. |
| `daemon.hostProc` | string | — | | Where the host's `/proc` is mounted when the daemon runs in a container (`docker-compose.yml` mounts it read-only at `/host/proc`). The node's network traffic is read there; without it only the panel container's own traffic is seen. |
| `daemon.sftp.host` | string | `0.0.0.0:5657` | | SFTP listen address. Its port is also the SFTP port reported for the local node. |
| `daemon.sftp.key` | data folder | `sftp.key` | | SFTP host key file (generated if missing). |
| `daemon.sftp.log` | bool | `false` | | Logs every SFTP request. |
| `daemon.auth.url` | string | `http://localhost:8080` | | Only on a node without the panel: the panel's OAuth2 token endpoint, `<panel>/oauth2/token`, used to check SFTP passwords. A URL without a path (such as the default, or `https://panel.example.com`) gets `/oauth2/token` appended; a URL with a path is used as it is. SSH key logins are checked at `/auth/sftp-key` on the same address (a path in front of `/oauth2/token` is kept), so for them the value must be an absolute `http(s)` URL. |
| `daemon.auth.clientId` | string | — | | Node client ID (`.node_<id>`). Declared and set by the deployment snippet, but not read by the current code. |
| `daemon.auth.clientSecret` | string | — | | Node secret, from `GET /api/nodes/{id}/deployment`. Sent to the panel when checking SFTP passwords and SSH keys, and used to recognise the tokens the panel signs for this node. |
| `daemon.console.buffer` | int | `50` | | Declared but not read by the current code. |
| `daemon.console.forward` | bool | `false` | | Declared but not read by the panel (only the developer tools in `tools/` set it). |
| `daemon.curseforge.key` | string | (build value) | Yes | CurseForge API key for CurseForge modpacks and downloads. The default can be injected at build time (Dockerfile build argument `curseforgeKey`); it is empty otherwise. On a remote node it is set in that node's own configuration. |
| `daemon.depotDownloader.version` | string | `latest` | | DepotDownloader version used for Steam downloads. |
| `daemon.depotDownloader.disableLancache` | bool | `false` | | Leaves out `-use-lancache` when running DepotDownloader. |

### Off-site backups (S3)

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `daemon.backups.s3.enabled` | bool | `false` | Yes | Copies every finished backup to an S3-compatible bucket (AWS S3, Backblaze B2, Cloudflare R2, Wasabi, MinIO…). The local copy stays. |
| `daemon.backups.s3.endpoint` | string | — | Yes | Endpoint: `s3.example.com`, `host:port` or a full URL. |
| `daemon.backups.s3.region` | string | — | Yes | Region. |
| `daemon.backups.s3.bucket` | string | — | Yes | Bucket. |
| `daemon.backups.s3.accessKey` | string | — | Yes | Access key. |
| `daemon.backups.s3.secretKey` | string | — | Yes | Secret key. |
| `daemon.backups.s3.prefix` | string | `aether-backups` | Yes | Prefix of the objects in the bucket. |
| `daemon.backups.s3.insecure` | bool | `false` | Yes | Plain HTTP to the endpoint (for a MinIO on the local network). |

The copy is made only when it is enabled and endpoint, bucket, access key and secret key are all set. These are daemon keys: Settings changes them on the machine running the panel (its local node); a remote node reads them from its own configuration. `POST /api/settings/test/s3` checks that the bucket exists, then writes and deletes a small object (reading is not tried, so a key that may write but not read still passes). See [Backups](../features/backups.md).

### Docker

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `docker.disallowHost` | bool | `false` | | Refuses the environments that run on the host (`host`, `tty`, `standard`, `bubblewrap`). Checked when a server is created, redefined, installed and started, and those environments are left out of `/daemon/features`. The image and compose set it to `true`. |
| `docker.root` | string | — | | Host path of the daemon's data folder, used to build bind mounts when the daemon itself runs in a container. When empty and `SKYPANEL_PLATFORM` is set, the daemon finds its own container and reads the mount of `daemon.data.root`. |
| `docker.serversNetwork` | string | — | | Docker network for game containers whose template names none. If the network does not exist, the panel's own network is used. Compose sets `skypanel-servers`. |
| `docker.serverUser` | string | `988:988` | | `uid:gid` game servers run as when the panel runs as root; installs stay root, and a template's own user wins. `0:0` keeps games as root. A value that is not numeric falls back to `988:988`. |
| `docker.pidsLimit` | int | `4096` | | Processes and threads per server container; a template's own limit wins. `0` removes the cap. |
| `docker.prefetchImages` | []string | `ghcr.io/aether-panel/steamcmd:latest`, `ghcr.io/aether-panel/ubuntu:latest`, `ghcr.io/aether-panel/generic:latest` | | Runtime images pulled in the background on every Docker node, two minutes after start and then daily; a replaced copy is removed if no container uses it. An empty list turns it off. |

See [Servers: Docker environment](./04-servers.md#docker-environment).

### Security

| Key | Type | Default | Settings | Description |
|---|---|---|---|---|
| `security.trustedProxies` | []string | `[]` | | Proxies (IPs or CIDRs) whose forwarding headers are trusted for the client address. No proxy is trusted by default. |
| `security.trustedProxyHeader` | string | — | | Header a trusted platform sets with the client address (Gin's `TrustedPlatform`, for example `CF-Connecting-IP`). |
| `security.blockPrivateDownloads` | bool | `true` | | Refuses template downloads from private network addresses (Docker networks, the LAN). Loopback, link-local and multicast addresses are always refused. Turn it off only for mirrors on the LAN. |
| `security.forceOpenat` | bool | `false` | | Skips the start-up check for `openat2` and uses plain `openat`. Without it, a daemon that cannot use `openat2` (the kernel lacks it, or a seccomp profile blocks it) refuses to start. |
| `security.disableUnshare` | bool | `false` | | Runs host-environment servers without `unshare` namespaces (each server can also set `disableUnshare`). See [unshare](../general/20-security-unshare.md). |

## Settings API

| Route | Description |
|---|---|
| `GET /api/settings` | Every key marked **Settings** above, plus `panel.settings.masterNodeIp`. Secrets (passwords, API keys, tokens, webhooks, the license key) are answered as `********` when set; sending that mask back to any route below keeps the saved value. |
| `POST /api/settings` | Changes several keys at once (`{"key": value, ...}`). Everything is validated first, so a bad value saves nothing. Unknown keys are ignored. |
| `GET /api/settings/{key}` / `PUT /api/settings/{key}` | Reads or changes one key (`{"value": ...}`). `GET` answers `204` for a key that is not editable. |
| `POST /api/settings/test/email`, `/test/discord`, `/test/s3`, `/test/cloudflare` | Checks the email, Discord, S3 and Cloudflare settings. |
| `POST /api/settings/license/activate` | Verifies and binds a license key. |
| `GET /api/config` | Public: name, theme, registration, captcha, `hideAiAnalysis` and experimental flags and version, for the web client. |

All but `/api/config` need `settings.edit`. Strings are trimmed before saving and converted to the key's type. The Settings page itself is described in [Panel settings](../features/panel-settings.md).

## Example: Docker install

`docker-compose.yml` sets these variables for the panel container (the database values come from `.env`):

```yaml
environment:
  - GIN_MODE=release
  - SKYPANEL_PLATFORM=docker
  - SKYPANEL_DOCKER_ROOT=
  - SKYPANEL_DOCKER_DISALLOWHOST=true
  - SKYPANEL_DAEMON_HOSTPROC=/host/proc
  - SKYPANEL_DOCKER_SERVERSNETWORK=skypanel-servers
  - SKYPANEL_WEB_HOST=0.0.0.0:8080
  - SKYPANEL_PANEL_DATABASE_DIALECT=mysql
  - SKYPANEL_PANEL_DATABASE_URL=${DB_USER:-skypanel}:${DB_PASSWORD:-skypanel_secret}@tcp(mysql:3306)/${DB_DATABASE:-skypanel}?charset=utf8&parseTime=true
  - SKYPANEL_TEMPLATES_URL=https://templates.aetherpanel.es/templates.json
```

and the image ships this `/etc/SkyPanel/config.json` (`config.docker.json`):

```json
{
  "logs": "/var/log/SkyPanel",
  "panel": {
    "enable": true,
    "database": {
      "dialect": "mysql",
      "url": "skypanel:CHANGE_ME@tcp(mysql:3306)/skypanel?charset=utf8mb4&parseTime=true"
    },
    "web": { "files": "/var/www/SkyPanel" }
  },
  "daemon": { "data": { "root": "/var/lib/SkyPanel" } },
  "docker": { "disallowHost": true }
}
```

The environment variables win over the file, so the compose database URL is the one used.

## Example: remote node

A node runs the same image with the panel turned off. The Nodes page of the panel shows a compose snippet like this one; the client secret is the node's secret, returned by `GET /api/nodes/{id}/deployment` (scope `nodes.deploy`):

```yaml
environment:
  - GIN_MODE=release
  - SKYPANEL_PLATFORM=docker
  - SKYPANEL_WEB_HOST=0.0.0.0:8080
  - SKYPANEL_PANEL_ENABLE=false
  - SKYPANEL_PANEL_SETTINGS_MASTERURL=https://panel.example.com
  - SKYPANEL_TOKEN_PUBLIC=https://panel.example.com/auth/publickey
  - SKYPANEL_DAEMON_AUTH_URL=https://panel.example.com/oauth2/token
  - SKYPANEL_DAEMON_AUTH_CLIENTID=.node_2
  - SKYPANEL_DAEMON_AUTH_CLIENTSECRET=<node secret>
  - SKYPANEL_DAEMON_SFTP_HOST=0.0.0.0:5657
```

The same in `config.json`:

```json
{
  "panel": {
    "enable": false,
    "settings": { "masterUrl": "https://panel.example.com" }
  },
  "token": { "public": "https://panel.example.com/auth/publickey" },
  "daemon": {
    "auth": {
      "url": "https://panel.example.com/oauth2/token",
      "clientId": ".node_2",
      "clientSecret": "<node secret>"
    },
    "sftp": { "host": "0.0.0.0:5657" }
  },
  "web": { "host": "0.0.0.0:8080" }
}
```

How the two sides authenticate:

- **Panel → node.** For every call to `/daemon/*` the panel signs a short-lived JWT (EdDSA with `panel.token`, 15 minutes, audience derived from the node secret). The node checks it against the key set at `token.public` and against `daemon.auth.clientSecret`.
- **Node → panel.** When someone signs in to SFTP with a password, the node posts it to `daemon.auth.url` (`grant_type=password`, username `email#serverId`) with `Authorization: Bearer <clientSecret>`. With an SSH key, it posts the username and the offered public key to the panel's `/auth/sftp-key` with the same header. Either way the panel only confirms the `server.sftp` scope for that server; it does not open a session.

See [Security](./09-security.md) for the details of tokens and SFTP.
