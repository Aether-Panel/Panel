# API Layer — Routes, Middleware, Authentication

The HTTP layer lives in `internal/web` (routes) and `internal/middleware`. One Gin engine serves the panel API, the sign-in endpoints, OAuth2, the daemon API and the web client on `web.host`. The endpoint-by-endpoint list is in the [API reference](../reference/11-api-reference.md); scopes are listed in the [scopes reference](../general/scopes-reference.md).

## Engine set-up (`cmd/panel/run.go`)

Before any route is registered:

1. `gin.Recovery()` and an access log whose lines leave out secrets (`middleware.RedactedLogFormatter`: the token in `/api/map-view/<server>/<token>/...` and the `token`, `code`, `access_token` and `refresh_token` query values are replaced with `***`).
2. Trusted proxies: none by default; `security.trustedProxies` and `security.trustedProxyHeader` change that (see [Configuration](./02-configuration.md#security)).
3. When the panel is enabled, the `session` cookie store (`panel.web.cookies.*`) is added.

The HTTP server sets a 10-second `ReadHeaderTimeout` and, on shutdown, lets requests in progress finish for up to 10 seconds.

## Main router (`internal/web/loader.go`)

`RegisterRoutes(e *gin.Engine)` registers, in this order:

```text
1. CORS                      (every route)
2. SecurityHeaders           (every route)
3. SameOriginGuard           (every route)
4. Recover                   (every route)
5. /daemon/*                 if daemon.enable
6. if panel.enable:
   /swagger/*any             signed-in administrators only (scope admin)
   /api/*                    panel API
   /oauth2/*                 OAuth2 token endpoint
   /auth/*                   sign-in, sign-up, password reset, SSO, public key
   static web client         embedded build, overlaid by panel.web.files
   /favicon.ico, /manifest.json, /favicon.png
   NoRoute                   SPA fallback (index.html)
```

The web client is the Astro build embedded in the binary (`client/frontend/dist`). A directory is redirected to its URL with a trailing slash and served from its `index.html`. Unknown paths under `/api/`, `/oauth2/` and `/daemon/` answer `404`; a missing build asset (`/_astro/...`, `.js`, `.css`, fonts…) also answers `404` with `Cache-Control: no-store`, so a page opened before an update reloads instead of running HTML as a script. Everything else gets `index.html` with caching disabled.

### CORS

```go
corsConfig := cors.DefaultConfig()
corsConfig.AllowOriginFunc = func(string) bool { return true }
corsConfig.AllowCredentials = false
corsConfig.AddAllowHeaders("Authorization", "Content-Type", "Accept", "Origin")
corsConfig.AddAllowMethods("GET", "POST", "PUT", "DELETE", "OPTIONS")
```

Any site may call the API **with a bearer token**, but never with the user's cookies: credentials are not allowed cross-origin. The panel itself is same-origin and does not need CORS. The console websocket routes add their own CORS handler.

### `SecurityHeaders`

Every answer carries:

| Header | Value |
|---|---|
| `Content-Security-Policy` | `frame-ancestors 'self'; object-src 'none'; base-uri 'self'` |
| `X-Frame-Options` | `SAMEORIGIN` |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `same-origin` |
| `Strict-Transport-Security` | `max-age=31536000`, only when the request came over HTTPS (TLS or `X-Forwarded-Proto: https`) |

Handlers that serve untrusted pages (extension files, world maps) replace the policy with their own sandbox.

### `SameOriginGuard`

Refuses (`403 {"error": "cross-origin request refused"}`) a request that:

1. carries a panel cookie (`skypanel_auth` or `session`), **and**
2. changes state (any method other than `GET`, `HEAD` and `OPTIONS`, or a websocket upgrade), **and**
3. was not started by the panel's own pages.

Origin is decided with `Sec-Fetch-Site` when the browser sends it (`same-origin` or `none` pass). Otherwise the `Origin` header must match the request host, `X-Forwarded-Host` or the host of `panel.settings.masterUrl`; a request with neither header did not come from a browser page and passes.

It exists because `SameSite=Lax` is not enough here: a "site" is the host without the port, so a game server's web page on `http://IP:25566` is the same site as the panel on `http://IP:8080` and could otherwise post forms or open a console websocket with the user's cookies. Requests without cookies (bearer tokens, nodes, OAuth2 clients) are not affected.

## Middleware

| Middleware | File | Role |
|---|---|---|
| `SecurityHeaders`, `SameOriginGuard` | `security.go` | See above. |
| `Recover`, `ResponseAndRecover` | `middleware.go` | Turn a panic into a `500` (the second one also writes the error response). |
| `NeedsDatabase`, `HasTransaction` | `database.go` | Put the GORM connection (or a transaction) in the context. |
| `AuthMiddleware` | `auth.go` | Session authentication (cookie or bearer token). |
| `RequiresPermission`, `RequiresAnyPermission` | `middleware.go` | Scope checks. |
| `ResolveServerPanel` / `ResolveServerNode` | `middleware.go` | Load the server from the panel database / from the daemon cache, or answer `404`. |
| `APIKeyAuthMiddleware`, `RequiresAPIKeyPermission` | `api_key_auth.go` | API keys of the billing API. |
| `ValidateJWT` | `decrypt.go` | Checks the panel-signed token on `/daemon` routes. |
| `AddVersionHeader` | `version.go` | Adds `X-API-Version: <panel version>`. |
| `RecordActivity` | `activity.go` | Writes sign-in events to the activity feed (best effort). |

### `/api/*` chain (`internal/web/api/loader.go`)

```go
rg.Use(func(c *gin.Context) { c.Header("Cache-Control", "no-store"); c.Next() })
rg.Use(middleware.ResponseAndRecover)
rg.Use(middleware.NeedsDatabase)
publicRg := rg.Group("")              // no authentication below this group
rg.Use(middleware.AuthMiddleware)
rg.Use(middleware.AddVersionHeader)
```

Public groups (no session): `GET /api/config`, `/api/extransfer/*` (federated transfers, authenticated with their own transfer tokens), `/api/map-view/*` (world map behind a signed link), `/api/extension-files/*` (extension pages, framed sandboxed) and `/api/v1/*` (billing API, API keys).

### Route groups

| Prefix | File | Main scopes |
|---|---|---|
| `/api/servers` | `servers.go` and `server_*.go`, `databases.go` | `server.*` per server; create needs `server.create` |
| `/api/nodes` | `nodes.go`, `node_ports.go`, `node_network.go` | `nodes.view`, `nodes.create`, `nodes.edit`, `nodes.delete`, `nodes.deploy` |
| `/api/users` | `users.go` | `users.info.*`, `users.perms.*` |
| `/api/self` | `self.go` | `login`, `self.edit`, `self.clients` (profile, 2FA, personal OAuth2 clients, SFTP keys) |
| `/api/roles` | `roles.go` | `admin` (listing also with user scopes) |
| `/api/templates` | `templates.go` | `login` to read, `templates.*` to change |
| `/api/game-versions` | `game_versions.go` | `login` |
| `/api/settings` | `settings.go` | `settings.edit` |
| `/api/settings/apikeys` | `apikeys.go` | `admin` |
| `/api/userSettings` | `usersettings.go` | `login` |
| `/api/activity` | `activity.go` | `users.info.search` |
| `/api/uptime` | `uptime.go` | `admin` or `uptime.view`; per server `server.view` |
| `/api/databasehosts` | `databasehosts.go` | `admin` |
| `/api/provision/products` | `products.go` | `admin` |
| `/api/extensions` | `extensions.go` | `admin` (`/active` with `login`) |
| `/api/v1` | `provision.go` | API key (see below) |

Most `/api/servers/{id}/...` routes are a scope check plus `proxyServerRequest`, which forwards the request to the node (see [Panel to node requests](#panel-to-node-requests)).

### `/auth/*` (`internal/web/auth/loader.go`)

| Route | Middleware | Purpose |
|---|---|---|
| `POST /auth/login` | `NeedsDatabase` | Password step. Opens the session, or answers `{"otpNeeded": true}` and keeps the pending sign-in in the `session` cookie for 5 minutes. |
| `POST /auth/otp` | `NeedsDatabase` | Two-factor step (TOTP or recovery code). |
| `POST /auth/register` | `NeedsDatabase` | Public sign-up, if `panel.registrationEnabled`. |
| `POST /auth/forgot-password` | `NeedsDatabase` | Sends a reset link. |
| `POST /auth/reset-password` | `NeedsDatabase` | Consumes the reset token and sets the new password. |
| `POST /auth/reauth` | `AuthMiddleware`, `NeedsDatabase` | Refreshes the current session. |
| `POST /auth/logout` | `NeedsDatabase` | Ends the current session. |
| `GET /auth/sso` | `NeedsDatabase` | Consumes a one-time link created by the billing API (`POST /api/v1/sso`) and signs the user in. |
| `GET /auth/challenge` | — | Proof-of-work challenge of the built-in captcha. |
| `GET /auth/publickey` | — | The panel's public key set (JWKS) used by nodes. |

Sign-in, sign-up and password reset check the captcha first: Turnstile when it is enabled and configured, otherwise the built-in proof of work when `panel.captcha.builtin` is on.

### Token endpoint (`/oauth2/*`)

```go
POST    /oauth2/token   → setHeaders (no-store), recovery, NeedsDatabase, handleTokenRequest
OPTIONS /oauth2/token
```

## Authentication

### Session (cookie or bearer)

1. `POST /auth/login` (and `/auth/otp` when 2FA is on) creates a row in `sessions` and returns the token in the body (`session`) and in the `skypanel_auth` cookie.
2. `AuthMiddleware` reads `Authorization: Bearer <token>` first and falls back to the `skypanel_auth` cookie. Paths under `/auth/`, `/error/` and `/api/config` are skipped, except `/auth/reauth` and `/auth/logout`.
3. The token is looked up by its SHA-256 hash and must not be expired. Sessions last **one hour, sliding**: activity renews the expiry in the database, and a cookie-based request also renews the cookie.
4. On every request the user must still hold the `login` scope (in their panel-wide permissions or role); taking it away ends sessions already open. A session of a deleted OAuth2 client is also ended.

The `skypanel_auth` cookie is `HttpOnly`, `SameSite=Lax`, path `/`, and `Secure` when the request came over HTTPS (directly or with `X-Forwarded-Proto: https`) or `panel.web.cookies.secure` is on. It is not `Secure` on plain HTTP on purpose: browsers drop `Secure` cookies there, and the installer's default address is `http://IP:8080`.

### OAuth2

`POST /oauth2/token` (form-encoded) supports two grants:

| Grant | Who | Result |
|---|---|---|
| `client_credentials` | Personal or server OAuth2 clients (`client_id`, `client_secret`, created in `/api/self/oauth2`) | A session token (`access_token`, `Bearer`, one hour) with the scopes the client's user holds. A client never gets more than it was registered for, and a server client only reaches its own server. |
| `password` | Nodes only, to check SFTP passwords | Requires `Authorization: Bearer <node secret>`. Username is `email#serverId`; the server must be on that node and the user must hold `server.sftp` on it. The answer only carries the scope `<serverId>:server.sftp`, never a usable token, so a password alone cannot bypass the account's 2FA. |

Client secrets are stored as bcrypt hashes.

### API keys (billing API)

- Created by an administrator in Settings (`/api/settings/apikeys`). Keys start with `ak_`; only a SHA-256 hash and an 8-character prefix are stored.
- Sent as `X-Api-Key: ak_...` or `Authorization: Bearer ak_...`; only accepted on `/api/v1/*`.
- Each key can be limited to actions: `create`, `terminate`, `suspend` (also unsuspend), `change-package` and `sso`. A key with no actions, or with the legacy `provision`, may do everything.

| Route | Action |
|---|---|
| `GET /api/v1/ping` | none |
| `POST /api/v1/provision` | `create` |
| `POST /api/v1/terminate` | `terminate` |
| `POST /api/v1/suspend`, `POST /api/v1/unsuspend` | `suspend` |
| `POST /api/v1/change-package` | `change-package` |
| `POST /api/v1/sso` | `sso` |

### Password recovery

1. `POST /auth/forgot-password` with `{"email": "..."}` creates a token (stored as a BLAKE2b-256 hash, valid 30 minutes) and emails a link to the address stored on the account.
2. `POST /auth/reset-password` with `{"token": "...", "password": "..."}` consumes it and changes the password.
3. Both answer `204` whenever possible so accounts cannot be enumerated. The link uses `panel.settings.masterUrl` when it is set.

### Panel to node requests

Every call the panel makes to a node's `/daemon/*` (including the proxied `/api/servers/{id}/...` routes and the console websocket) carries `Authorization: Bearer <JWT>`:

- Signed by the panel with EdDSA (Ed25519 key from `panel.token`), key ID `SkyPanel`.
- Valid for 15 minutes, with one minute of leeway for clock differences.
- Its audience is a SHA-256 hash derived from the target node's secret, so a token captured on its way to one node cannot be replayed against another.

`ValidateJWT` on the node accepts only EdDSA tokens with an expiry and that audience (the local node's secret, or `daemon.auth.clientSecret` on a remote node). The verification key is the panel's own key set when the panel runs on the same binary, or the JWKS fetched from `token.public` on a remote node. Unauthenticated daemon routes: `GET/HEAD /daemon` (status) and `GET /daemon/features`.

The node calls the panel only to check SFTP passwords (the `password` grant above). See [Configuration: remote node](./02-configuration.md#example-remote-node) and [Security](./09-security.md).

## Scope checks

```go
g.Handle("GET", "/:serverId", middleware.RequiresPermission(scopes.ScopeServerView), middleware.ResolveServerPanel, getServer)
g.GET("/:serverId/flags", middleware.RequiresAnyPermission(scopes.ScopeServerViewFlags, scopes.ScopeServerEditFlags), ...)
```

`RequiresPermission` (`checkPermission`):

1. Needs a user in the context (set by `AuthMiddleware`); otherwise `403`.
2. A server scope needs `:serverId` in the route.
3. A session from an OAuth2 client is first bounded by the client's own scopes and server.
4. Collects the scopes: on a server, that server's permission row, the panel-wide row an administrator set, and the **owner scopes** when the user owns the server. A role's server scopes never count on a server (they would give every sub-user the owner's powers); off a server, the role's scopes and the panel-wide row count.
5. Bounds the collected list by the OAuth2 client, when the session has one (`boundByClient`), and stores it on the request as `scopes`. Handlers read that list (`callerScopes`) for their own second checks (database passwords, locked environment variables, scopes a user may grant, task step scopes) and to pick the WebSocket streams, so those checks see exactly what the route check saw.
6. Checks the scope with `scopes.ContainsScope`: `admin` covers every scope and `server.admin` every server scope. Any unexpected failure answers `500`, never access.

## Rate limits

| What | Limit | Key |
|---|---|---|
| Password attempts (panel sign-in, OAuth2 `password`, SFTP) | 10 per 15 minutes | account |
| Two-factor codes | 10 per 15 minutes | account |
| Password reset requests | 5 per hour | email |
| AI requests (log analysis and assistant together) | 30 per 10 minutes | user |
| Subdomain changes | 1 per 10 seconds | server |
| Notification tests | 1 per 10 seconds | server |

The limiters are keyed by account rather than by IP, so they behave the same behind a reverse proxy; they live in memory and reset when the panel restarts. Too many attempts answer `429`.

## Port management

### `PUT /api/servers/{id}/data` (administrator)

Saves the full port list and the main port. Requires `server.data.edit.admin`.

```json
{ "ports": [25565, 25575, 25585], "primaryPort": 25565 }
```

- `primaryPort` must be one of the ports; it is moved to the front.
- Every port is checked against the node: range 1024–65535, no duplicates, not one of the panel's own ports on that node, not assigned to another server there.
- The node is updated first; if it refuses or is down, the panel does not save ports or limits the server does not have. The daemon rewrites `port`, `port2`, `port3`… (removing stale ones).
- Updates `server.Port` and `server.Ports` in the panel database and refreshes the subdomain records when the main port changed.

### `PUT /api/servers/{id}/port-settings` (owner)

Changes the main port among the assigned ones and the port labels, without touching the list. Requires `server.data.edit`.

```json
{ "primaryPort": 25565, "portNotes": { "25575": "RCON", "25585": "Voice" } }
// → { "success": true }
```

- Moves `primaryPort` to the front of `server.Ports` and saves `server.PortNotes`.
- Sends the current list to the node so `port`/`port2`/`port3`… stay in sync, and refreshes the subdomain when the main port changed.

### `POST /api/nodes/{id}/ports/check`

Used by the new-server wizard before anything is created. Requires `server.create`.

```json
{ "ports": { "port": 25565, "port2": 25566 }, "exclude": "<server being edited>" }
```

For each variable it answers whether the port is busy, why (`range` below 1024, `panel` for the panel's own ports on that node, `server` for another server, `container` for another running container), a free port to use instead and, for administrators only, who holds it.

## Swagger

The OpenAPI documentation is generated from the handler comments (`internal/web/swagger/swagger.json` and `.yaml`) and served at `/swagger/index.html` through `gin-swagger`, only to signed-in users with the `admin` scope, since it maps every route. The files are regenerated with `swag` from the annotations; the current ones predate the latest annotation fixes (several backup, task, definition and world map routes), so regenerate them before relying on those entries.
