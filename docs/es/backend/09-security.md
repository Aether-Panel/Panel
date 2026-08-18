# Seguridad, SFTP, JWT y API Keys

## SFTP Server

El panel incluye un servidor SFTP integrado (no requiere OpenSSH). Corre en el puerto configurado (`daemon.sftp.host`, default `0.0.0.0:5657`).

### Autenticación

La autenticación SFTP se realiza contra el panel:
1. El cliente SFTP se conecta al daemon con username/password
2. El daemon valida las credenciales contra el panel vía el endpoint OAuth2 (`grant_type=password`, `scope=sftp`) — ver `internal/oauth2/ssh.go:validateSSH`
3. El panel devuelve los scopes concedidos en formato `<serverId>:<scope>`; el daemon otorga acceso solo si incluye `server.sftp`
4. Modo local (`DatabaseSFTPAuthorization`, `internal/services/sftp.go`): el username usa formato `<email>#<serverId>` y verifica el scope `server.sftp` directamente contra la BD

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
// El token de daemon (GenerateRequest) usa un JWT firmado con Ed25519.
// Header: alg=EdDSA, kid="SkyPanel"
// Claims: jwt.MapClaims (sin claims personalizados por defecto)
// El panel valida firma + expiración; la identidad del nodo se resuelve
// por el endpoint /daemon/* solicitado (el daemon se autentica con su
// token request firmado con la clave privada del nodo).
```

### Servicio de Token (`internal/services/token.go`)

| Método | Descripción |
|---|---|
| `GenerateRequest()` | Firma un JWT request del daemon (header `kid=SkyPanel`, método EdDSA/Ed25519) |
| `ValidateRequest(token)` | Valida la firma y expiración del JWT contra la clave pública |
| `GetKeyFunc()` | Devuelve `jwt.Keyfunc` (resuelve la clave JWK) |
| `GetTokenStore()` | Devuelve el almacén JWKS (`jwkset.Storage`) |

La clave privada se obtiene de `config.PrivateKey` (base64, 32 bytes seed) o se genera aleatoriamente si no existe. El KID es fijo (`SkyPanel`).

## API Keys

### Formato

Token: `ak_<5 hex>_<43 hex>` (ej. `ak_a1b2c3_...`). Generado con 24 bytes aleatorios (`crypto/rand`) en `services/apikey.go:GenerateKey`.

### Almacenamiento

Tabla `api_keys` con los campos:
- `id` — autoincremental
- `name` — nombre descriptivo
- `prefix` — primeros 8 caracteres (`ak_` + 5 hex), usado para lookup rápido
- `hashed_key` — hash SHA-256 del token completo (hex), para validación
- `permissions` — JSON array de permisos/scopes permitidos
- `created_at`

Solo se guarda el hash; el token completo no se almacena en BD.

### Autenticación

El middleware `APIKeyAuthMiddleware` busca en orden:
1. Header `X-Api-Key`
2. Header `Authorization: Bearer <token>` (solo si empieza con `ak_`)

Usado por las rutas de Provisioning API (`/api/v1/*`).

## Unshare (Aislamiento de Namespaces)

Los entornos TTY usan `unshare` para aislar el proceso del servidor:

- User namespace (CLONE_NEWUSER)
- Mount namespace (CLONE_NEWNS)
- Cgroup namespace (CLONE_NEWCGROUP)
- UTS namespace (hostname separado)
- IPC namespace (CLONE_NEWIPC)
- CLONE_FILES (compartir descriptores de archivo)

Configurable con:
- `security.disableUnshare` (bool, default false) — deshabilita el unshare globalmente
- `disableUnshare` (bool, por servidor) — deshabilita el unshare para un servidor concreto
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
