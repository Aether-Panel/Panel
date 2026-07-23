# Security, SFTP, JWT and API Keys

## SFTP Server

The panel includes an integrated SFTP server (does not require OpenSSH). It runs on the configured port (`daemon.sftp.host`, default `0.0.0.0:5657`).

### Authentication

SFTP authentication is performed against the panel:
1. The SFTP client connects to the daemon
2. The daemon validates the credentials against the panel via API (`/daemon/sftp/auth`)
3. Password authentication is used (panel user's username/password)
4. The daemon verifies that the user has the `server.sftp` scope

### Server Key

- Path: `daemon.sftp.key` (default: `<data>/sftp.key`)
- Automatically generated if it does not exist
- Format: SSH private key

### Implementation

Files: `internal/sftp/server.go` and `internal/sftp/requestprefix.go`

- `server.go` — initializes and runs the SSH/SFTP server
- `requestprefix.go` — handles the path prefix to isolate each server

### SFTP Connection Flow

1. Client connects to the SFTP port
2. SSH server starts (with the server key)
3. Password authentication against the panel
4. `server.sftp` scope verification
5. Access restricted to the server's directory

## JWT (JSON Web Tokens)

Used for daemon↔panel and OAuth2 authentication.

### Algorithm

- **Ed25519** (public/private key)
- Library: `golang-jwt/jwt/v5`

### Keys

- `panel.token`: Ed25519 private key (automatically generated if it does not exist)
- The `GET /auth/publickey` endpoint exposes the public key for external validation
- `TokenPublicURL` allows configuring the key's public URL

### Claims

```go
// JWT contains:
{
  "sub": "serverId o clientId",
  "iat": timestamp,
  "exp": timestamp,
  "scopes": ["admin", "server.console"],
  "type": "client_credentials" | "password"
}
```

### Token Service (`internal/services/token.go`)

| Method | Description |
|---|---|
| `Sign(claims)` | Signs and returns JWT |
| `Validate(tokenString)` | Validates and parses JWT |
| `GetPublicKey()` | Returns the Ed25519 public key |

## API Keys

### Format

Prefix: `ak_` + token generated with `securecookie.GenerateRandomKey(32)` → hex.

### Storage

`api_keys` table with the fields:
- `id` — auto-increment
- `user_id` — owner
- `token` — token hash (for validation)
- `scopes` — JSON array of allowed scopes
- `memo` — visible description
- `created_at`, `updated_at`

### Authentication

The `APIKeyAuthMiddleware` middleware searches in order:
1. `X-Api-Key` Header
2. `Authorization: Bearer <token>` Header (only if it starts with `ak_`)

Used by Provisioning API routes (`/api/v1/*`).

## Unshare (Namespace Isolation)

TTY environments use `unshare` to isolate the server process:

- PID namespace (separate processes)
- Mount namespace (isolated file system)
- Network namespace (isolated network)
- UTS namespace (separate hostname)
- IPC namespace

Configurable with:
- `security.disableUnshare` (bool, default false) — completely disables unshare
- `security.forceOpenat` (bool, default false) — forces openat() for file operations

## Trusted Proxies

For when the panel runs behind a reverse proxy (Nginx, Caddy, Cloudflare):

```json
{
  "security": {
    "trustedProxies": ["10.0.0.0/8", "172.16.0.0/12"],
    "trustedProxyHeader": "X-Forwarded-For"
  }
}
```

## Session Cookies

| Property | Config | Default |
|---|---|---|
| Secure | `panel.web.cookies.secure` | `false` |
| HttpOnly | `panel.web.cookies.httpOnly` | `true` |
| SameSite | `panel.web.cookies.sameSite` | `Strict` |
| Max-Age | `panel.web.cookies.age` | 30 days |
| Path | `panel.web.cookies.path` | `/` |
| Domain | `panel.web.cookies.domain` | — |

## CORS

Global configuration in `RegisterRoutes()`:

```go
corsConfig.AllowOriginFunc = func(_ string) bool { return true }
corsConfig.AllowCredentials = true
corsConfig.AddAllowHeaders("Authorization", "Content-Type", "Accept", "Origin")
corsConfig.AddAllowMethods("GET", "POST", "PUT", "DELETE", "OPTIONS")
```

## Path Validation (Antipath Traversal)

The system uses `path.Clean()` and `fs.ValidPath()` in front of the embedded `fs.FS` to prevent path traversal in the static frontend.

For the daemon, file handlers use `strings.TrimPrefix(path, "/")` and verify that the path does not contain `..`.

## Security Notifications

The system can send alerts via Discord webhook when critical status changes occur:

- `panel.notifications.discordWebhook` — general notifications
- `panel.notifications.discordWebhookSystem` — system alerts
- `panel.notifications.discordWebhookNode` — node events