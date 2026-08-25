# Capa API — Rutas, Middleware, Autenticación

## Router Principal (`internal/web/loader.go`)

Todo arranca en `RegisterRoutes(e *gin.Engine)`:

```go
// Orden de registro:
1. CORS global (AllowOriginFunc: true, AllowCredentials: true)
2. Recovery middleware
3. /swagger/*any → Swagger UI
4. /daemon/*    → daemon.RegisterDaemonRoutes()   [si daemon.enable=true]
5. /api/*       → api.RegisterRoutes()             [si panel.enable=true]
6. /oauth2/*    → oauth2.RegisterRoutes()
7. /auth/*      → auth.RegisterRoutes()
8. Frontend estático (archivos embebidos + SPA fallback)
9. /favicon.ico, /manifest.json
10. NoRoute → SPA handler (index.html)
```

## Cadena de Middleware

### Global

```go
// Aplicado a todas las rutas
e.Use(cors.New(corsConfig))
e.Use(func(c *gin.Context) { middleware.Recover(c) })
```

### API (`/api/*`)

```go
rg.Use(func(c *gin.Context) { c.Header("Cache-Control", "no-store"); c.Next() })
rg.Use(middleware.ResponseAndRecover)  // panic recovery + error response
rg.Use(middleware.NeedsDatabase)       // inyecta *gorm.DB en el contexto
// === A partir de aquí requiere autenticación ===
rg.Use(middleware.AuthMiddleware)      // valida sesión/cookie/Bearer token
rg.Use(middleware.AddVersionHeader)    // agrega X-Panel-Version
```

### Auth (`/auth/*`)

```go
rg.Use(middleware.ResponseAndRecover)
// Endpoints específicos:
/auth/login          → NeedsDatabase, LoginPost
/auth/logout         → NeedsDatabase, LogoutPost
/auth/otp            → NeedsDatabase, OtpPost
/auth/register       → NeedsDatabase, RegisterPost
/auth/forgot-password → NeedsDatabase, ForgotPasswordPost
/auth/reset-password → NeedsDatabase, ResetPasswordPost
/auth/reauth         → AuthMiddleware, NeedsDatabase, Reauth
/auth/publickey      → (sin auth) TokenServiceGetPublicKey
```

### OAuth2 (`/oauth2/*`)

```go
/oauth2/token → setHeaders, recovery, NeedsDatabase, handleTokenRequest
```

### Provision API v1 (`/api/v1/*`)

```go
// Autenticación por API Key
rg.Use(middleware.APIKeyAuthMiddleware)
/v1/ping, /v1/provision, /v1/terminate, /v1/suspend, /v1/unsuspend
```

## Sistema de Autenticación

### 1. Sesión por Cookie

1. `POST /auth/login` → valida credenciales → si OTP habilitado, requiere paso 2
2. `POST /auth/otp` → valida OTP → crea sesión → setea cookie `skypanel_auth`
3. `AuthMiddleware` busca primero header `Authorization: Bearer <token>`, luego cookie `skypanel_auth`
4. La sesión se valida contra la BD (tabla `sessions`)

### 2. Bearer Token (OAuth2)

1. `POST /oauth2/token` con `grant_type=client_credentials` o `password`
2. Devuelve `access_token` (JWT firmado con Ed25519)
3. Las rutas protegidas aceptan `Authorization: Bearer <token>`

### 5. Recuperación de Contraseña

1. `POST /auth/forgot-password` con `{ "email": "..." }` → crea token (hash blake2b, 30 min) y envía email con enlace de reset
2. `POST /auth/reset-password` con `{ "token": "...", "password": "..." }` → consume el token y cambia la contraseña
3. Ambos responden `204` siempre que sea posible para evitar enumeración de usuarios; el enlace usa el host real (header `Host`/`X-Forwarded-Host`)

### 3. API Key

1. Creadas desde Settings en el panel
2. Prefijo `ak_` en el token
3. Autenticación via header `X-Api-Key` o `Authorization: Bearer ak_*`
4. Usado por `APIKeyAuthMiddleware` (rutas `/api/v1/*`)

### 4. Daemon Auth

El daemon se autentica contra el panel con un **JWT firmado con Ed25519**. En cada petición a `/daemon/*`, el daemon genera un token request firmado con su clave privada y lo envía como `Authorization: Bearer <token>`; el panel lo valida contra la clave pública del nodo (ver `services/token.go`). No usa OAuth2.

## Scope Checking

El sistema de permisos usa **scopes** (permisos granulares) que se verifican con `RequiresPermission`:

```go
// En definición de ruta:
servers.GET("", middleware.RequiresPermission(scopes.ScopeServerView), handler)

// Con OR lógico:
servers.POST("/suspend", middleware.RequiresAnyPermission(scopes.ScopeServerAdmin, scopes.ScopeServerEditDefinition), handler)
```

El middleware `RequiresPermission`:
1. Obtiene el usuario del contexto (inyectado por `AuthMiddleware`)
2. Busca permisos del usuario para el servidor específico (si aplica `ForServer`)
3. Evalúa scopes del rol global del usuario
4. Si el scope es `ForServer`, requiere que exista `serverId` en la ruta
5. Usa `scopes.ContainsScope()` para verificar

## OAuth2

### Grant Types Soportados

| Grant | Endpoint | Uso |
|---|---|---|
| `client_credentials` | `POST /oauth2/token` | Máquina a máquina (daemon, API externa) |
| `password` | `POST /oauth2/token` | Usuario/contraseña (con OTP opcional) |

### Personal OAuth2 Clients

Los usuarios pueden crear sus propios OAuth2 clients desde `/api/self/oauth2/*`.

## Daemon Routes

Las rutas del daemon (`/daemon/*`) son llamadas por los nodos para tareas que requieren acceso directo al sistema de archivos:

- `/daemon` → health check
- `/daemon/features` → capacidades del nodo
- `/daemon/system` → información del sistema
- `/daemon/server/*` → CRUD de servidores, archivos, consola, backups, etc.

El daemon valida las peticiones usando JWT firmado por el panel.

## Swagger

La documentación OpenAPI/Swagger está disponible en `/swagger/index.html`. Los archivos Swagger están en `internal/web/swagger/swagger.json` y se sirven mediante `gin-swagger`.

## Endpoints de Puertos (Port Management)

### `PUT /api/servers/:id/data` (Admin)
Guarda la lista completa de puertos y primario. Requiere `server.data.edit.admin` o `admin`.
```json
// Request
{ "ports": [25565, 25575, 25585], "primaryPort": 25565 }
// Response: 204 No Content
```
- Valida contra nodo (puertos en uso, rango 1024-65535).
- Actualiza `server.Port`, `server.Ports` en Panel DB.
- **Proxy a Daemon**: envía `ports` + `primaryPort` → Daemon crea variables `port`, `port2`, `port3`...

### `PUT /api/servers/:id/port-settings` (Usuario)
Gestiona metadatos de puertos sin tocar la lista completa. Requiere `server.data.view`.
```json
// Request
{ "primaryPort": 25565, "portNotes": { "25575": "RCON", "25585": "Voice" } }
// Response: { "success": true }
```
- Reordena `server.Ports` poniendo el primario primero.
- Guarda `server.PortNotes` (map string->string).
- **Sincroniza con Daemon**: envía la lista completa actualizada para mantener `port`/`port2`/`port3` en sync.

### `PUT /api/servers/:id/port-settings` (Daemon side)
El Daemon recibe la sincronización en `PUT /daemon/server/:id/data` y ejecuta la misma lógica de conversión `ports` → `port`/`port2`/...
