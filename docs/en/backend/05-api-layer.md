# API Layer — Routes, Middleware, Authentication

## Main Router (`internal/web/loader.go`)

Everything starts in `RegisterRoutes(e *gin.Engine)`:

```go
// Registration order:
1. Global CORS (AllowOriginFunc: true, AllowCredentials: true)
2. Recovery middleware
3. /swagger/*any → Swagger UI
4. /daemon/*    → daemon.RegisterDaemonRoutes()   [if daemon.enable=true]
5. /api/*       → api.RegisterRoutes()             [if panel.enable=true]
6. /oauth2/*    → oauth2.RegisterRoutes()
7. /auth/*      → auth.RegisterRoutes()
8. Static Frontend (embedded files + SPA fallback)
9. /favicon.ico, /manifest.json
10. NoRoute → SPA handler (index.html)
```

## Middleware Chain

### Global

```go
// Applied to all routes
e.Use(cors.New(corsConfig))
e.Use(func(c *gin.Context) { middleware.Recover(c) })
```

### API (`/api/*`)

```go
rg.Use(func(c *gin.Context) { c.Header("Cache-Control", "no-store"); c.Next() })
rg.Use(middleware.ResponseAndRecover)  // panic recovery + error response
rg.Use(middleware.NeedsDatabase)       // injects *gorm.DB into the context
// === From here on, authentication is required ===
rg.Use(middleware.AuthMiddleware)      // validates session/cookie/Bearer token
rg.Use(middleware.AddVersionHeader)    // adds X-Panel-Version
```

### Auth (`/auth/*`)

```go
rg.Use(middleware.ResponseAndRecover)
// Specific endpoints:
/auth/login          → NeedsDatabase, LoginPost
/auth/logout         → NeedsDatabase, LogoutPost
/auth/otp            → NeedsDatabase, OtpPost
/auth/register       → NeedsDatabase, RegisterPost
/auth/forgot-password → NeedsDatabase, ForgotPasswordPost
/auth/reset-password → NeedsDatabase, ResetPasswordPost
/auth/reauth         → AuthMiddleware, NeedsDatabase, Reauth
/auth/publickey      → (no auth) TokenServiceGetPublicKey
```

### OAuth2 (`/oauth2/*`)

```go
/oauth2/token → setHeaders, recovery, NeedsDatabase, handleTokenRequest
```

### Provision API v1 (`/api/v1/*`)

```go
// API Key Authentication
rg.Use(middleware.APIKeyAuthMiddleware)
/v1/ping, /v1/provision, /v1/terminate, /v1/suspend, /v1/unsuspend
```

## Authentication System

### 1. Cookie Session

1. `POST /auth/login` → validates credentials → if OTP enabled, requires step 2
2. `POST /auth/otp` → validates OTP → creates session → sets `skypanel_auth` cookie
3. `AuthMiddleware` first looks for `Authorization: Bearer <token>` header, then `skypanel_auth` cookie
4. The session is validated against the DB (`sessions` table)

### 2. Bearer Token (OAuth2)

1. `POST /oauth2/token` with `grant_type=client_credentials` or `password`
2. Returns `access_token` (JWT signed with Ed25519)
3. Protected routes accept `Authorization: Bearer <token>`

### 5. Password Recovery

1. `POST /auth/forgot-password` with `{ "email": "..." }` → creates a token (blake2b hash, 30 min) and sends an email with the reset link
2. `POST /auth/reset-password` with `{ "token": "...", "password": "..." }` → consumes the token and changes the password
3. Both return `204` whenever possible to avoid user enumeration; the link uses the real host (`Host`/`X-Forwarded-Host` header)

### 3. API Key

1. Created from Settings in the panel
2. `ak_` prefix in the token
3. Authentication via `X-Api-Key` header or `Authorization: Bearer ak_*`
4. Used by `APIKeyAuthMiddleware` (`/api/v1/*` routes)

### 4. Daemon Auth

The daemon authenticates against the panel with a **JWT signed with Ed25519**. On each request to `/daemon/*`, the daemon generates a request token signed with its private key and sends it as `Authorization: Bearer <token>`; the panel validates it against the node's public key (see `services/token.go`). It does not use OAuth2.

## Scope Checking

The permission system uses **scopes** (granular permissions) which are verified with `RequiresPermission`:

```go
// In route definition:
servers.GET("", middleware.RequiresPermission(scopes.ScopeServerView), handler)

// With logical OR:
servers.POST("/suspend", middleware.RequiresAnyPermission(scopes.ScopeServerAdmin, scopes.ScopeServerEditDefinition), handler)
```

The `RequiresPermission` middleware:
1. Gets the user from the context (injected by `AuthMiddleware`)
2. Looks for user permissions for the specific server (if `ForServer` applies)
3. Evaluates scopes of the user's global role
4. If the scope is `ForServer`, it requires `serverId` to exist in the route
5. Uses `scopes.ContainsScope()` to verify

## OAuth2

### Supported Grant Types

| Grant | Endpoint | Usage |
|---|---|---|
| `client_credentials` | `POST /oauth2/token` | Machine-to-machine (daemon, external API) |
| `password` | `POST /oauth2/token` | Username/password (with optional OTP) |

### Personal OAuth2 Clients

Users can create their own OAuth2 clients from `/api/self/oauth2/*`.

## Daemon Routes

Daemon routes (`/daemon/*`) are called by nodes for tasks that require direct filesystem access:

- `/daemon` → health check
- `/daemon/features` → node capabilities
- `/daemon/system` → system information
- `/daemon/server/*` → CRUD of servers, files, console, backups, etc.

The daemon validates requests using a JWT signed by the panel.

## Swagger

OpenAPI/Swagger documentation is available at `/swagger/index.html`. Swagger files are located in `internal/web/swagger/swagger.json` and served via `gin-swagger`.
