# Seguridad, SFTP, JWT y API Keys

## SFTP Server

El panel incluye un servidor SFTP integrado (no requiere OpenSSH). Corre en el puerto configurado (`daemon.sftp.host`, default `0.0.0.0:5657`).

### Autenticación

La autenticación SFTP se realiza contra el panel:
1. El cliente SFTP se conecta al daemon
2. El daemon valida las credenciales contra el panel via API (`/daemon/sftp/auth`)
3. Se usa autenticación por contraseña (username/password del usuario del panel)
4. El daemon verifica que el usuario tenga el scope `server.sftp`

### Server Key

- Ruta: `daemon.sftp.key` (default: `<data>/sftp.key`)
- Se genera automáticamente si no existe
- Formato: clave privada SSH

### Implementación

Archivos: `internal/sftp/server.go` y `internal/sftp/requestprefix.go`

- `server.go` — inicializa y corre el servidor SSH/SFTP
- `requestprefix.go` — maneja el prefijo de ruta para aislar a cada servidor

### Flujo de Conexión SFTP

1. Cliente se conecta al puerto SFTP
2. Servidor SSH inicia (con la server key)
3. Autenticación password contra el panel
4. Verificación de scope `server.sftp`
5. Acceso restringido al directorio del servidor

## JWT (JSON Web Tokens)

Usado para autenticación daemon↔panel y OAuth2.

### Algoritmo

- **Ed25519** (clave pública/privada)
- Librería: `golang-jwt/jwt/v5`

### Claves

- `panel.token`: clave privada Ed25519 (se genera automáticamente si no existe)
- El endpoint `GET /auth/publickey` expone la clave pública para validación externa
- `TokenPublicURL` permite configurar URL pública de la clave

### Claims

```go
// JWT contiene:
{
  "sub": "serverId o clientId",
  "iat": timestamp,
  "exp": timestamp,
  "scopes": ["admin", "server.console"],
  "type": "client_credentials" | "password"
}
```

### Servicio de Token (`internal/services/token.go`)

| Método | Descripción |
|---|---|
| `Sign(claims)` | Firma y devuelve JWT |
| `Validate(tokenString)` | Valida y parsea JWT |
| `GetPublicKey()` | Devuelve la clave pública Ed25519 |

## API Keys

### Formato

Prefijo: `ak_` + token generado con `securecookie.GenerateRandomKey(32)` → hex.

### Almacenamiento

Tabla `api_keys` con los campos:
- `id` — autoincremental
- `user_id` — propietario
- `token` — hash del token (para validación)
- `scopes` — JSON array de scopes permitidos
- `memo` — descripción visible
- `created_at`, `updated_at`

### Autenticación

El middleware `APIKeyAuthMiddleware` busca en orden:
1. Header `X-Api-Key`
2. Header `Authorization: Bearer <token>` (solo si empieza con `ak_`)

Usado por las rutas de Provisioning API (`/api/v1/*`).

## Unshare (Aislamiento de Namespaces)

Los entornos TTY usan `unshare` para aislar el proceso del servidor:

- PID namespace (procesos separados)
- Mount namespace (sistema de archivos aislado)
- Network namespace (red aislada)
- UTS namespace (hostname separado)
- IPC namespace

Configurable con:
- `security.disableUnshare` (bool, default false) — deshabilita completamente el unshare
- `security.forceOpenat` (bool, default false) — fuerza openat() para operaciones de archivos

## Trusted Proxies

Para cuando el panel corre detrás de un reverse proxy (Nginx, Caddy, Cloudflare):

```json
{
  "security": {
    "trustedProxies": ["10.0.0.0/8", "172.16.0.0/12"],
    "trustedProxyHeader": "X-Forwarded-For"
  }
}
```

## Cookies de Sesión

| Propiedad | Config | Default |
|---|---|---|
| Secure | `panel.web.cookies.secure` | `false` |
| HttpOnly | `panel.web.cookies.httpOnly` | `true` |
| SameSite | `panel.web.cookies.sameSite` | `Strict` |
| Max-Age | `panel.web.cookies.age` | 30 días |
| Path | `panel.web.cookies.path` | `/` |
| Domain | `panel.web.cookies.domain` | — |

## CORS

Configuración global en `RegisterRoutes()`:

```go
corsConfig.AllowOriginFunc = func(_ string) bool { return true }
corsConfig.AllowCredentials = true
corsConfig.AddAllowHeaders("Authorization", "Content-Type", "Accept", "Origin")
corsConfig.AddAllowMethods("GET", "POST", "PUT", "DELETE", "OPTIONS")
```

## Validación de Path (Antipath Traversal)

El sistema usa `path.Clean()` y `fs.ValidPath()` delante del `fs.FS` embebido para prevenir path traversal en el frontend estático.

Para el daemon, los handlers de archivos usan `strings.TrimPrefix(ruta, "/")` y verifican que la ruta no contenga `..`.

## Notificaciones de Seguridad

El sistema puede enviar alertas vía Discord webhook cuando hay cambios de estado críticos:

- `panel.notifications.discordWebhook` — notificaciones generales
- `panel.notifications.discordWebhookSystem` — alertas del sistema
- `panel.notifications.discordWebhookNode` — eventos de nodos
