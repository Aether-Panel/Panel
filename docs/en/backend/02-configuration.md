# Configuration

The system uses **Viper** with prefix `SKYPANEL_`. Environment variables take precedence over the configuration file.

## Configuration File

Search paths (in order): `--config` flag > `SKYPANEL_CONFIG` env > `/etc/SkyPanel/config.json` > `./config.json`

Format: JSON (Viper compatible, also YAML/TOML).

## Environment Variables

All nested keys with `.` are converted to `_`. `SKYPANEL_` prefix:

| Variable | Viper Key |
|---|---|
| `SKYPANEL_WEB_HOST` | `web.host` |
| `SKYPANEL_PANEL_DATABASE_DIALECT` | `panel.database.dialect` |
| `SKYPANEL_DAEMON_SFTP_HOST` | `daemon.sftp.host` |

## Complete Configuration Table

### Web

| Key | Type | Default | Description |
|---|---|---|---|
| `web.host` | string | `0.0.0.0:8080` | HTTP server address and port |

### Panel

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.enable` | bool | `true` | Enables panel mode (API, frontend) |
| `panel.database.dialect` | string | `sqlite3` | Dialect: `sqlite3`, `mysql`, `postgresql`, `sqlserver` |
| `panel.database.url` | string | — | Connection string (default according to dialect) |
| `panel.database.log` | bool | `false` | Enables detailed GORM logging (SQL) |
| `panel.web.files` | string | `www` | Path to frontend files (embedded FS override) |
| `panel.web.cookies.secure` | bool | `false` | Cookie Secure flag |
| `panel.web.cookies.httpOnly` | bool | `true` | Cookie HttpOnly flag |
| `panel.web.cookies.domain` | string | — | Cookie Domain |
| `panel.web.cookies.age` | int | `2592000` (30d) | Cookie Max-Age in seconds |
| `panel.web.cookies.sameSite` | string | `Strict` | Cookie SameSite |
| `panel.web.cookies.path` | string | `/` | Cookie Path |
| `panel.email.templateFolder` | string | — | Email template folder |
| `panel.email.provider` | string | — | Email provider (`mailgun`, `smtp`) |
| `panel.email.from` | string | — | From address |
| `panel.email.domain` | string | — | Domain (Mailgun) |
| `panel.email.host` | string | — | SMTP Host |
| `panel.email.key` | string | — | API Key (Mailgun) or password |
| `panel.email.username` | string | — | SMTP Username |
| `panel.email.password` | string | — | SMTP Password |
| `panel.settings.companyName` | string | `SkyPanel` | Company/brand name |
| `panel.settings.defaultTheme` | string | `SkyPanel` | Default theme |
| `panel.settings.themeSettings` | string | `{}` | Theme settings (JSON) |
| `panel.settings.masterUrl` | string | `http://localhost:8080` | Master panel URL |
| `panel.settings.nodeIp` | string | `0.0.0.0` | Node IP |
| `panel.settings.geminiApiKey` | string | — | Google Gemini API Key |
| `panel.settings.hideAiAnalysis` | bool | `false` | Hides AI Analysis functionality |
| `panel.settings.headerDecorations` | bool | `true` | Enables header decorations |
| `panel.notifications.discordWebhook` | string | — | Discord Webhook (notifications) |
| `panel.notifications.discordWebhookSystem` | string | — | Discord Webhook (system) |
| `panel.notifications.discordWebhookNode` | string | — | Discord Webhook (nodes) |
| `panel.notifications.discordWebhookExTransfer` | string | — | Discord Webhook (external transfers) |
| `panel.license.key` | string | — | License key |
| `panel.license.status` | string | `free` | License status |
| `panel.license.serverId` | string | — | Server ID for license |
| `panel.license.serverIp` | string | — | Server IP for license |
| `panel.sessionKey` | string | — | Key for session encryption (auto-generated) |
| `panel.registrationEnabled` | bool | `true` | Allows user registration |
| `panel.token` | string | — | Ed25519 private key (JWT) |
| `panel.turnstile.siteKey` | string | — | Cloudflare Turnstile Site Key |
| `panel.turnstile.secretKey` | string | — | Cloudflare Turnstile Secret Key |
| `panel.turnstile.enabled` | bool | `false` | Enables Cloudflare Turnstile on register/login |

### Token

| Key | Type | Default | Description |
|---|---|---|---|
| `token.public` | string | — | Public URL of the master panel to validate JWTs (node mode) |

### Templates

| Key | Type | Default | Description |
|---|---|---|---|
| `templates.url` | string | — | URL of VPS JSON template repository |

### Daemon

| Key | Type | Default | Description |
|---|---|---|---|
| `daemon.enable` | bool | `true` | Enables daemon mode |
| `daemon.console.buffer` | int | `50` | Console buffer size in memory |
| `daemon.console.forward` | bool | `false` | Forwards the server console to standard output (stdout/journald/Docker logs) |
| `daemon.sftp.host` | string | `0.0.0.0:5657` | SFTP server address |
| `daemon.sftp.key` | string | `sftp.key` | Path to SFTP private key |
| `daemon.sftp.log` | bool | `false` | Detailed SFTP logging |
| `daemon.auth.url` | string | `http://localhost:8080` | Panel URL (legacy, for authentication) |
| `daemon.auth.clientId` | string | — | Client ID (legacy, not used by current JWT auth) |
| `daemon.auth.clientSecret` | string | — | Client Secret (legacy, not used by current JWT auth) |
| `daemon.data.cache` | string | `cache` | Cache folder |
| `daemon.data.servers` | string | `servers` | Servers folder |
| `daemon.data.backups.folder` | string | `backups` | Backups folder |
| `daemon.data.binaries` | string | `binaries` | Binaries folder |
| `daemon.data.crashLimit` | int | `3` | Crash limit before suspending |
| `daemon.data.root` | string | — | Data root (default: parent of servers) |
| `daemon.curseforge.key` | string | — | CurseForge API Key |
| `daemon.depotDownloader.version` | string | `latest` | DepotDownloader version |
| `daemon.depotDownloader.disableLancache` | bool | `false` | Disables Lancache |

### Security

| Key | Type | Default | Description |
|---|---|---|---|
| `security.forceOpenat` | bool | `false` | Forces usage of openat() instead of open() |
| `security.trustedProxies` | []string | `[]` | Trusted proxies |
| `security.trustedProxyHeader` | string | — | Trusted proxy header |
| `security.disableUnshare` | bool | `false` | Disables unshare of namespaces |

### Docker

| Key | Type | Default | Description |
|---|---|---|---|
| `docker.root` | string | — | Docker root path |
| `docker.disallowHost` | bool | `false` | Disables host environment (forces Docker) |

### Logs

| Key | Type | Default | Description |
|---|---|---|---|
| `logs` | string | `logs` | Logs folder |

## Automatically Generated Keys

- `panel.sessionKey` — Automatically generated if not configured (using `securecookie.GenerateRandomKey`)
- `panel.token` — Ed25519 private key for JWT (automatically generated)
- `daemon.data.root` — Calculated as `filepath.Dir(daemon.data.servers)` if empty

## Editable Configuration System

The `GET /api/config` endpoint exposes a filtered version of the configuration (branding, themes, registration). Changes to `panel.settings.*` and `panel.notifications.*` can be made via `POST /api/settings` and are persisted to the configuration file.
