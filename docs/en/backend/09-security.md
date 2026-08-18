# Security, SFTP, JWT and API Keys

## SFTP Server

The panel includes an integrated SFTP server (does not require OpenSSH). It runs on the configured port (`daemon.sftp.host`, default `0.0.0.0:5657`).

### Authentication

SFTP authentication is performed against the panel:
1. The SFTP client connects to the daemon with username/password
2. The daemon validates the credentials against the panel via the OAuth2 endpoint (`grant_type=password`, `scope=sftp`) — see `internal/oauth2/ssh.go:validateSSH`
3. The panel returns the granted scopes in `<serverId>:<scope>` format; the daemon grants access only if it includes `server.sftp`
4. Local mode (`DatabaseSFTPAuthorization`, `internal/services/sftp.go`): the username uses the `<email>#<serverId>` format and verifies the `server.sftp` scope directly against the DB

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
// The daemon token (GenerateRequest) uses a JWT signed with Ed25519.
// Header: alg=EdDSA, kid="SkyPanel"
// Claims: jwt.MapClaims (no custom claims by default)
// The panel validates signature + expiration; the node identity is resolved
// by the requested /daemon/* endpoint (the daemon authenticates with its
// request token signed with the node's private key).
```

### Token Service (`internal/services/token.go`)

| Method | Description |
|---|---|
| `GenerateRequest()` | Signs a daemon request JWT (header `kid=SkyPanel`, EdDSA/Ed25519 method) |
| `ValidateRequest(token)` | Validates the JWT signature and expiration against the public key |
| `GetKeyFunc()` | Returns `jwt.Keyfunc` (resolves the JWK key) |
| `GetTokenStore()` | Returns the JWKS store (`jwkset.Storage`) |

The private key is obtained from `config.PrivateKey` (base64, 32 byte seed) or randomly generated if it does not exist. The KID is fixed (`SkyPanel`).

## API Keys

### Format

Token: `ak_<5 hex>_<43 hex>` (e.g. `ak_a1b2c3_...`). Generated with 24 random bytes (`crypto/rand`) in `services/apikey.go:GenerateKey`.

### Storage

`api_keys` table with the fields:
- `id` — auto-increment
- `name` — descriptive name
- `prefix` — first 8 characters (`ak_` + 5 hex), used for fast lookup
- `hashed_key` — SHA-256 hash of the full token (hex), for validation
- `permissions` — JSON array of allowed permissions/scopes
- `created_at`

Only the hash is stored; the full token is not stored in the DB.

### Authentication

The `APIKeyAuthMiddleware` middleware searches in order:
1. `X-Api-Key` Header
2. `Authorization: Bearer <token>` Header (only if it starts with `ak_`)

Used by Provisioning API routes (`/api/v1/*`).

## Unshare (Namespace Isolation)

TTY environments use `unshare` to isolate the server process:

- User namespace (CLONE_NEWUSER)
- Mount namespace (CLONE_NEWNS)
- Cgroup namespace (CLONE_NEWCGROUP)
- UTS namespace (separate hostname)
- IPC namespace (CLONE_NEWIPC)
- CLONE_FILES (share file descriptors)

Configurable with:
- `security.disableUnshare` (bool, default false) — disables unshare globally
- `disableUnshare` (bool, per server) — disables unshare for a specific server
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
