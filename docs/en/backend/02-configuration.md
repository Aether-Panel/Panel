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
| `web.host` | string | `0.0.0:8080` | HTTP server address and port |
| `web.files` | string | `/var/www/SkyPanel` | Path to frontend static files (embedded FS override) |

### Panel

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.enable` | bool | `true` | Enables panel mode (API, frontend) |
| `panel.database.dialect` | string | `sqlite3` | Dialect: `sqlite3`, `mysql`, `postgresql`, `sqlserver` |
| `panel.database.url` | string | — | Connection string (default according to dialect) |
| `panel.database.log` | bool | `false` | Enables detailed GORM logging (SQL) |
| `panel.web.files` | string | `/var/www/SkyPanel` | Path to frontend files (embedded FS override) |
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
| `panel.settings.branding` | object | `{}` | Custom branding JSON |
| `panel.settings.turnstileSiteKey` | string | — | Turnstile site key (alias) |
| `panel.settings.turnstileSecretKey` | string | — | Turnstile secret key (alias) |
| `panel.settings.turnstileEnabled` | bool | `false` | Turnstile enabled (alias) |
| `panel.settings.licenseKey` | string | — | License key (alias) |
| `panel.settings.sentryDSN` | string | — | Sentry DSN for error tracking |
| `panel.curseforgeKey` | string | — | CurseForge API key (internal) |
| `panel.frontendPath` | string | `/var/www/SkyPanel` | Frontend files path (internal) |

### Panel Database

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.database.dialect` | string | `sqlite3` | Dialect: `sqlite3`, `mysql`, `postgresql`, `sqlserver` |
| `panel.database.url` | string | `skypanel.db` | Connection string |
| `panel.database.log` | bool | `false` | Enable GORM SQL logging |

### Panel Web Cookies

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.web.cookies.secure` | bool | `false` | Cookie Secure flag (HTTPS only) |
| `panel.web.cookies.httpOnly` | bool | `true` | Cookie HttpOnly flag |
| `panel.web.cookies.domain` | string | — | Cookie Domain |
| `panel.web.cookies.age` | int | `2592000` (30d) | Cookie Max-Age in seconds |
| `panel.web.cookies.sameSite` | string | `Strict` | Cookie SameSite policy |
| `panel.web.cookies.path` | string | `/` | Cookie Path |

### Panel Email

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.email.templateFolder` | string | — | Custom email template folder |
| `panel.email.provider` | string | — | Email provider (`smtp`, `sendgrid`, `mailjet`, `mailgun`, `debug`) |
| `panel.email.from` | string | — | From address |
| `panel.email.domain` | string | — | Domain (Mailgun) |
| `panel.email.host` | string | — | SMTP Host |
| `panel.email.key` | string | — | API Key (Mailgun/SendGrid) or password |
| `panel.email.username` | string | — | SMTP Username |
| `panel.email.password` | string | — | SMTP Password |

### Panel Settings

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.settings.companyName` | string | `SkyPanel` | Company/brand name |
| `panel.settings.defaultTheme` | string | `SkyPanel` | Default UI theme |
| `panel.settings.themeSettings` | string | `{}` | Theme settings (JSON) |
| `panel.settings.masterUrl` | string | `http://localhost:8080` | Master panel URL |
| `panel.settings.nodeIp` | string | `0.0.0.0` | Local node IP |
| `panel.settings.geminiApiKey` | string | — | Google GenAI API Key |
| `panel.settings.hideAiAnalysis` | bool | `false` | Hides AI Analysis functionality |
| `panel.settings.headerDecorations` | bool | `true` | Enables header decorations |
| `panel.settings.branding` | object | `{}` | Custom branding JSON |
| `panel.settings.turnstileEnabled` | bool | `false` | Enable Cloudflare Turnstile |
| `panel.settings.turnstileSiteKey` | string | — | Turnstile site key |
| `panel.settings.turnstileSecretKey` | string | — | Turnstile secret key |
| `panel.settings.licenseKey` | string | — | License key |
| `panel.settings.sentryDSN` | string | — | Sentry DSN for error tracking |

### Panel Notifications

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.notifications.discordWebhook` | string | — | Discord webhook (general alerts) |
| `panel.notifications.discordWebhookSystem` | string | — | Discord webhook (system status) |
| `panel.notifications.discordWebhookNode` | string | — | Discord webhook (node alerts) |
| `panel.notifications.discordWebhookExTransfer` | string | — | Discord webhook (external transfers) |

### Panel License

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.license.key` | string | — | License key |
| `panel.license.status` | string | `free` | License status (`free`, `pro`, `enterprise`) |
| `panel.license.serverId` | string | — | Licensed server ID |
| `panel.license.serverIp` | string | — | Licensed server IP |

### Panel Session / Security

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.sessionKey` | string | (auto) | Session encryption key (auto-generated) |
| `panel.registrationEnabled` | bool | `true` | Allow public registration |
| `panel.token` | string | (auto) | Ed25519 private key for JWT (auto-generated) |
| `panel.turnstile.siteKey` | string | — | Turnstile site key |
| `panel.turnstile.secretKey` | string | — | Turnstile secret key |
| `panel.turnstile.enabled` | bool | `false` | Enable Cloudflare Turnstile |

### Panel Misc

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.curseforgeKey` | string | — | CurseForge API key (internal) |
| `panel.frontendPath` | string | `/var/www/SkyPanel` | Frontend files path (internal) |

### Web Root

| Key | Type | Default | Description |
|---|---|---|---|
| `web.root` | string | `www` | Web root path (legacy) |
| `web.files` | string | `/var/www/SkyPanel` | Frontend static files path |

### Panel Settings

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.settings.companyName` | string | `SkyPanel` | Company/brand name |
| `panel.settings.defaultTheme` | string | `SkyPanel` | Default UI theme |
| `panel.settings.registrationEnabled` | bool | `true` | Allow public registration |
| `panel.settings.companyName` | string | `SkyPanel` | Company name |
| `panel.settings.defaultTheme` | string | `SkyPanel` | Default theme |
| `panel.settings.branding` | object | `{}` | Custom branding JSON |
| `panel.settings.turnstileEnabled` | bool | `false` | Enable Cloudflare Turnstile |
| `panel.settings.turnstileSiteKey` | string | — | Turnstile site key |
| `panel.settings.turnstileSecretKey` | string | — | Turnstile secret key |
| `panel.settings.licenseKey` | string | — | License key |
| `panel.settings.sentryDSN` | string | — | Sentry DSN |
| `panel.settings.geminiApiKey` | string | — | Google GenAI API key |
| `panel.settings.hideAiAnalysis` | bool | `false` | Hide AI Analysis button |
| `panel.settings.headerDecorations` | bool | `true` | Header decorations |
| `panel.settings.masterUrl` | string | `http://localhost:8080` | Master panel URL |
| `panel.settings.nodeIp` | string | `0.0.0.0` | Local node IP |

### Panel Notifications

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.notifications.discordWebhook` | string | — | Discord webhook (general) |
| `panel.notifications.discordWebhookSystem` | string | — | Discord webhook (system alerts) |
| `panel.notifications.discordWebhookNode` | string | — | Discord webhook (node alerts) |
| `panel.notifications.discordWebhookExTransfer` | string | — | Discord webhook (external transfers) |

### Panel License

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.license.key` | string | — | License key |
| `panel.license.status` | string | `free` | License status |
| `panel.license.serverId` | string | — | Licensed server ID |
| `panel.license.serverIp` | string | — | Licensed server IP |

### Panel Session / Security

| Key | Type | Default | Description |
|---|---|---|---|
| `panel.sessionKey` | string | (auto) | Session encryption key (auto-generated) |
| `panel.registrationEnabled` | bool | `true` | Public registration |
| `panel.token` | string | (auto) | Ed25519 private key for JWT (auto) |
| `panel.turnstile.siteKey` | string | — | Turnstile site key |
| `panel.turnstile.secretKey` | string | — | Turnstile secret key |
| `panel.turnstile.enabled` | bool | `false` | Enable Turnstile |

### Token (Remote Daemon)

| Key | Type | Default | Description |
|---|---|---|---|
| `token.public` | string | — | Master panel URL for JWT validation (node mode) |

### Templates

| Key | Type | Default | Description |
|---|---|---|---|
| `templates.url` | string | `https://templates.aetherpanel.es/templates.json` | Template repository index URL |

### Daemon

| Key | Type | Default | Description |
|---|---|---|---|
| `daemon.enable` | bool | `true` | Enable daemon mode |
| `daemon.console.buffer` | int | `50` | Console buffer lines |
| `daemon.console.forward` | bool | `false` | Forward console to stdout/journald |
| `daemon.sftp.host` | string | `0.0.0.0:5657` | SFTP listen address |
| `daemon.sftp.key` | string | `sftp.key` | SFTP private key path |
| `daemon.sftp.log` | bool | `false` | Verbose SFTP logging |
| `daemon.sftp.disable` | bool | `false` | Disable SFTP server |
| `daemon.auth.url` | string | `http://localhost:8080` | Panel URL for OAuth2 |
| `daemon.auth.clientId` | string | `.node_1` | OAuth2 client ID |
| `daemon.auth.clientSecret` | string | (generated) | OAuth2 client secret |
| `daemon.token.public` | string | `http://localhost:8080/auth/publickey` | Panel JWKS endpoint |
| `daemon.data.cache` | string | `cache` | Cache folder |
| `daemon.data.servers` | string | `servers` | Servers folder |
| `daemon.data.backups.folder` | string | `backups` | Backups folder |
| `daemon.data.binaries` | string | `binaries` | Binaries folder |
| `daemon.data.crashLimit` | int | `3` | Crash limit before suspend |
| `daemon.data.root` | string | (auto) | Data root (parent of servers) |
| `daemon.curseforge.key` | string | — | CurseForge API key |
| `daemon.depotDownloader.version` | string | `latest` | DepotDownloader version |
| `daemon.depotDownloader.disableLancache` | bool | `false` | Disable Lancache |
| `daemon.data.root` | string | `/var/lib/SkyPanel` | Data root directory |
| `daemon.data.binaries` | string | `/var/lib/SkyPanel/binaries` | Binaries cache |
| `daemon.data.cache` | string | `/var/lib/SkyPanel/cache` | Template cache |
| `daemon.data.templates` | string | `/var/lib/SkyPanel/templates` | Local templates |
| `daemon.data.images` | string | `/var/lib/SkyPanel/images` | Docker images |
| `daemon.data.backups` | string | `/var/lib/SkyPanel/backups` | Backups storage |
| `daemon.data.scripts` | string | `/var/lib/SkyPanel/scripts` | Custom scripts |

### Daemon Auth (Remote Daemon OAuth2)

| Key | Type | Default | Description |
|---|---|---|---|
| `daemon.auth.url` | string | `http://localhost:8080` | Panel URL for token endpoint |
| `daemon.auth.clientId` | string | `.node_1` | OAuth2 client ID |
| `daemon.auth.clientSecret` | string | (generated) | OAuth2 client secret |

### Daemon Token (JWT Validation)

| Key | Type | Default | Description |
|---|---|---|---|
| `daemon.token.public` | string | `http://localhost:8080/auth/publickey` | Panel JWKS endpoint |

### Security

| Key | Type | Default | Description |
|---|---|---|---|
| `security.forceOpenat` | bool | `false` | Force openat() syscall |
| `security.trustedProxies` | []string | `[]` | Trusted proxy CIDRs |
| `security.trustedProxyHeader` | string | `X-Forwarded-For` | Proxy header for client IP |
| `security.disableUnshare` | bool | `false` | Disable unshare isolation |

### Docker

| Key | Type | Default | Description |
|---|---|---|---|
| `docker.root` | string | — | Docker root path |
| `docker.disallowHost` | bool | `false` | Disable host environment (force Docker) |

### Node

| Key | Type | Default | Description |
|---|---|---|---|
| `node.ip` | string | (auto) | Local node public IP |
| `node.port` | int | `8080` | Local node port |
| `node.masterUrl` | string | (auto) | Master panel URL |

### Logs

| Key | Type | Default | Description |
|---|---|---|---|
| `logs` | string | `logs` | Logs folder |
| `logs.level` | string | `info` | Log level: `debug`/`info`/`warn`/`error` |
| `logs.format` | string | `text` | Format: `text`/`json` |
| `logs.output` | string | `stdout` | Output: `stdout`/`file`/`both` |

### Web

| Key | Type | Default | Description |
|---|---|---|---|
| `web.host` | string | `0.0.0.0:8080` | HTTP listen address |
| `web.files` | string | `/var/www/SkyPanel` | Frontend static files path |

### Templates (Legacy)

| Key | Type | Default | Description |
|---|---|---|---|
| `templates.url` | string | `https://templates.aetherpanel.es/templates.json` | Template index URL |

## Automatically Generated Keys

- `panel.sessionKey` — Auto-generated using `securecookie.GenerateRandomKey`
- `panel.token` — Ed25519 private key for JWT (auto-generated)
- `daemon.data.root` — Calculated as `filepath.Dir(daemon.data.servers)` if empty
- `daemon.sftp.key` — Ed25519 key generated if missing

## Editable Configuration System

The `GET /api/config` endpoint exposes a filtered version of the configuration (branding, themes, registration). Changes to `panel.settings.*` and `panel.notifications.*` can be made via `POST /api/settings` and are persisted to the configuration file.

## Remote Daemon Configuration Example

For a remote node connecting to a master panel:

```json
{
  "daemon": {
    "enable": true,
    "auth": {
      "url": "http://master-panel:8080",
      "clientId": ".node_1",
      "clientSecret": "remote-node-secret"
    },
    "token": {
      "public": "http://master-panel:8080/auth/publickey"
    },
    "sftp": {
      "host": "0.0.0.0:5657"
    }
  },
  "web": {
    "host": "0.0.0.0:8080"
  }
}
```

The remote daemon authenticates to the panel via OAuth2 (`/oauth2/token` with `client_credentials`), then uses the returned JWT for all `/daemon/*` calls.
