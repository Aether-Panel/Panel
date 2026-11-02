# Capa API — Rutas, middleware y autenticación

La capa HTTP vive en `internal/web` (rutas) e `internal/middleware`. Un único motor Gin sirve en `web.host` la API del panel, los endpoints de inicio de sesión, OAuth2, la API del daemon y el cliente web. La lista endpoint por endpoint está en la [referencia de la API](../reference/11-api-reference.md); los permisos (scopes) se detallan en la [referencia de permisos](../general/scopes-reference.md).

## Preparación del motor (`cmd/panel/run.go`)

Antes de registrar ninguna ruta:

1. `gin.Recovery()` y un registro de accesos cuyas líneas omiten secretos (`middleware.RedactedLogFormatter`: el token de `/api/map-view/<servidor>/<token>/...` y los valores de consulta `token`, `code`, `access_token` y `refresh_token` se sustituyen por `***`).
2. Proxies de confianza: ninguno por defecto; `security.trustedProxies` y `security.trustedProxyHeader` lo cambian (ver [Configuración](./02-configuration.md#seguridad)).
3. Con el panel activo, se añade el almacén de la cookie `session` (`panel.web.cookies.*`).

El servidor HTTP fija un `ReadHeaderTimeout` de 10 segundos y, al apagarse, deja terminar las peticiones en curso durante hasta 10 segundos.

## Router principal (`internal/web/loader.go`)

`RegisterRoutes(e *gin.Engine)` registra, en este orden:

```text
1. CORS                      (todas las rutas)
2. SecurityHeaders           (todas las rutas)
3. SameOriginGuard           (todas las rutas)
4. Recover                   (todas las rutas)
5. /daemon/*                 si daemon.enable
6. si panel.enable:
   /swagger/*any             solo administradores con sesión (permiso admin)
   /api/*                    API del panel
   /oauth2/*                 endpoint de tokens OAuth2
   /auth/*                   inicio de sesión, registro, restablecer contraseña, SSO, clave pública
   cliente web estático      build incluido, superpuesto por panel.web.files
   /favicon.ico, /manifest.json, /favicon.png
   NoRoute                   reserva SPA (index.html)
```

El cliente web es el build de Astro incluido en el binario (`client/frontend/dist`). Una carpeta se redirige a su URL con barra final y se sirve desde su `index.html`. Las rutas desconocidas bajo `/api/`, `/oauth2/` y `/daemon/` responden `404`; un recurso del build que no existe (`/_astro/...`, `.js`, `.css`, fuentes…) también responde `404` con `Cache-Control: no-store`, para que una página abierta antes de una actualización se recargue en lugar de ejecutar HTML como script. Todo lo demás recibe `index.html` sin caché.

### CORS

```go
corsConfig := cors.DefaultConfig()
corsConfig.AllowOriginFunc = func(string) bool { return true }
corsConfig.AllowCredentials = false
corsConfig.AddAllowHeaders("Authorization", "Content-Type", "Accept", "Origin")
corsConfig.AddAllowMethods("GET", "POST", "PUT", "DELETE", "OPTIONS")
```

Cualquier sitio puede llamar a la API **con un token bearer**, pero nunca con las cookies del usuario: no se permiten credenciales entre orígenes. El propio panel es del mismo origen y no necesita CORS. Las rutas del websocket de consola añaden su propio manejador CORS.

### `SecurityHeaders`

Todas las respuestas llevan:

| Cabecera | Valor |
|---|---|
| `Content-Security-Policy` | `frame-ancestors 'self'; object-src 'none'; base-uri 'self'` |
| `X-Frame-Options` | `SAMEORIGIN` |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `same-origin` |
| `Strict-Transport-Security` | `max-age=31536000`, solo cuando la petición llegó por HTTPS (TLS o `X-Forwarded-Proto: https`) |

Los manejadores que sirven páginas no confiables (archivos de extensiones, mapas del mundo) sustituyen la política por su propio sandbox.

### `SameOriginGuard`

Rechaza (`403 {"error": "cross-origin request refused"}`) una petición que:

1. lleva una cookie del panel (`skypanel_auth` o `session`), **y**
2. cambia estado (cualquier método distinto de `GET`, `HEAD` y `OPTIONS`, o un upgrade a websocket), **y**
3. no la han iniciado las propias páginas del panel.

El origen se decide con `Sec-Fetch-Site` cuando el navegador la envía (pasan `same-origin` y `none`). Si no, la cabecera `Origin` debe coincidir con el host de la petición, con `X-Forwarded-Host` o con el host de `panel.settings.masterUrl`; una petición sin ninguna de las dos no viene de una página de navegador y pasa.

Existe porque `SameSite=Lax` no basta aquí: un «sitio» es el host sin el puerto, así que la página web de un servidor de juego en `http://IP:25566` es el mismo sitio que el panel en `http://IP:8080` y, sin esto, podría enviar formularios o abrir un websocket de consola con las cookies del usuario. Las peticiones sin cookies (tokens bearer, nodos, clientes OAuth2) no se ven afectadas.

## Middleware

| Middleware | Archivo | Función |
|---|---|---|
| `SecurityHeaders`, `SameOriginGuard` | `security.go` | Ver arriba. |
| `Recover`, `ResponseAndRecover` | `middleware.go` | Convierten un pánico en un `500` (el segundo también escribe la respuesta de error). |
| `NeedsDatabase`, `HasTransaction` | `database.go` | Ponen en el contexto la conexión GORM (o una transacción). |
| `AuthMiddleware` | `auth.go` | Autenticación por sesión (cookie o token bearer). |
| `RequiresPermission`, `RequiresAnyPermission` | `middleware.go` | Comprobación de permisos. |
| `ResolveServerPanel` / `ResolveServerNode` | `middleware.go` | Cargan el servidor de la base de datos del panel / de la caché del daemon, o responden `404`. |
| `APIKeyAuthMiddleware`, `RequiresAPIKeyPermission` | `api_key_auth.go` | Claves de API de la API de facturación. |
| `ValidateJWT` | `decrypt.go` | Comprueba el token firmado por el panel en las rutas `/daemon`. |
| `AddVersionHeader` | `version.go` | Añade `X-API-Version: <versión del panel>`. |
| `RecordActivity` | `activity.go` | Escribe los eventos de inicio de sesión en el registro de actividad (de mejor esfuerzo). |

### Cadena de `/api/*` (`internal/web/api/loader.go`)

```go
rg.Use(func(c *gin.Context) { c.Header("Cache-Control", "no-store"); c.Next() })
rg.Use(middleware.ResponseAndRecover)
rg.Use(middleware.NeedsDatabase)
publicRg := rg.Group("")              // sin autenticación por debajo de este grupo
rg.Use(middleware.AuthMiddleware)
rg.Use(middleware.AddVersionHeader)
```

Grupos públicos (sin sesión): `GET /api/config`, `/api/extransfer/*` (transferencias federadas, autenticadas con sus propios tokens de transferencia), `/api/map-view/*` (mapa del mundo tras un enlace firmado), `/api/extension-files/*` (páginas de extensiones, en un marco con sandbox) y `/api/v1/*` (API de facturación, claves de API).

### Grupos de rutas

| Prefijo | Archivo | Permisos principales |
|---|---|---|
| `/api/servers` | `servers.go` y `server_*.go`, `databases.go` | `server.*` por servidor; crear requiere `server.create` |
| `/api/nodes` | `nodes.go`, `node_ports.go`, `node_network.go` | `nodes.view`, `nodes.create`, `nodes.edit`, `nodes.delete`, `nodes.deploy` |
| `/api/users` | `users.go` | `users.info.*`, `users.perms.*` |
| `/api/self` | `self.go` | `login`, `self.edit`, `self.clients` (perfil, 2FA, clientes OAuth2 personales, claves SFTP) |
| `/api/roles` | `roles.go` | `admin` (el listado también con permisos de usuarios) |
| `/api/templates` | `templates.go` | `login` para leer, `templates.*` para cambiar |
| `/api/game-versions` | `game_versions.go` | `login` |
| `/api/settings` | `settings.go` | `settings.edit` |
| `/api/settings/apikeys` | `apikeys.go` | `admin` |
| `/api/userSettings` | `usersettings.go` | `login` |
| `/api/activity` | `activity.go` | `users.info.search` |
| `/api/uptime` | `uptime.go` | `admin` o `uptime.view`; por servidor `server.view` |
| `/api/databasehosts` | `databasehosts.go` | `admin` |
| `/api/provision/products` | `products.go` | `admin` |
| `/api/extensions` | `extensions.go` | `admin` (`/active` con `login`) |
| `/api/v1` | `provision.go` | Clave de API (ver abajo) |

La mayoría de las rutas `/api/servers/{id}/...` son una comprobación de permiso más `proxyServerRequest`, que reenvía la petición al nodo (ver [Peticiones del panel a los nodos](#peticiones-del-panel-a-los-nodos)).

### `/auth/*` (`internal/web/auth/loader.go`)

| Ruta | Middleware | Función |
|---|---|---|
| `POST /auth/login` | `NeedsDatabase` | Paso de la contraseña. Abre la sesión, o responde `{"otpNeeded": true}` y guarda el inicio de sesión pendiente en la cookie `session` durante 5 minutos. |
| `POST /auth/otp` | `NeedsDatabase` | Paso del segundo factor (TOTP o código de recuperación). |
| `POST /auth/register` | `NeedsDatabase` | Registro público, si `panel.registrationEnabled`. |
| `POST /auth/forgot-password` | `NeedsDatabase` | Envía un enlace de restablecimiento. |
| `POST /auth/reset-password` | `NeedsDatabase` | Consume el token de restablecimiento y fija la nueva contraseña. |
| `POST /auth/reauth` | `AuthMiddleware`, `NeedsDatabase` | Renueva la sesión actual. |
| `POST /auth/logout` | `NeedsDatabase` | Cierra la sesión actual. |
| `GET /auth/sso` | `NeedsDatabase` | Consume un enlace de un solo uso creado por la API de facturación (`POST /api/v1/sso`) e inicia la sesión del usuario. |
| `GET /auth/challenge` | — | Reto de prueba de trabajo del captcha propio. |
| `GET /auth/publickey` | — | Conjunto de claves públicas del panel (JWKS) que usan los nodos. |

El inicio de sesión, el registro y el restablecimiento de contraseña comprueban antes el captcha: Turnstile si está activado y configurado y, si no, la prueba de trabajo propia cuando `panel.captcha.builtin` está activo.

### Endpoint de tokens (`/oauth2/*`)

```go
POST    /oauth2/token   → setHeaders (no-store), recovery, NeedsDatabase, handleTokenRequest
OPTIONS /oauth2/token
```

## Autenticación

### Sesión (cookie o bearer)

1. `POST /auth/login` (y `/auth/otp` cuando hay 2FA) crea una fila en `sessions` y devuelve el token en el cuerpo (`session`) y en la cookie `skypanel_auth`.
2. `AuthMiddleware` lee primero `Authorization: Bearer <token>` y, si no hay, la cookie `skypanel_auth`. Las rutas bajo `/auth/`, `/error/` y `/api/config` se saltan, salvo `/auth/reauth` y `/auth/logout`.
3. El token se busca por su hash SHA-256 y no debe haber caducado. Las sesiones duran **una hora, deslizante**: la actividad renueva la caducidad en la base de datos, y una petición con cookie renueva también la cookie.
4. En cada petición el usuario debe seguir teniendo el permiso `login` (en sus permisos globales o en su rol); quitárselo cierra las sesiones ya abiertas. También se cierra la sesión de un cliente OAuth2 borrado.

La cookie `skypanel_auth` es `HttpOnly`, `SameSite=Lax`, de ruta `/`, y `Secure` cuando la petición llegó por HTTPS (directamente o con `X-Forwarded-Proto: https`) o está activo `panel.web.cookies.secure`. A propósito no es `Secure` en HTTP sin cifrar: los navegadores descartan ahí las cookies `Secure`, y la dirección por defecto del instalador es `http://IP:8080`.

### OAuth2

`POST /oauth2/token` (formulario) admite dos tipos de concesión:

| Concesión | Quién | Resultado |
|---|---|---|
| `client_credentials` | Clientes OAuth2 personales o de servidor (`client_id`, `client_secret`, creados en `/api/self/oauth2`) | Un token de sesión (`access_token`, `Bearer`, una hora) con los permisos que tiene el usuario del cliente. Un cliente nunca recibe más de aquello para lo que se registró, y un cliente de servidor solo llega a su propio servidor. |
| `password` | Solo nodos, para comprobar contraseñas SFTP | Requiere `Authorization: Bearer <secreto del nodo>`. El usuario es `email#idServidor`; el servidor debe estar en ese nodo y el usuario debe tener `server.sftp` sobre él. La respuesta solo lleva el permiso `<idServidor>:server.sftp`, nunca un token utilizable, así que una contraseña sola no puede saltarse el 2FA de la cuenta. |

Los secretos de cliente se guardan como hash bcrypt.

### Claves de API (API de facturación)

- Las crea un administrador en Configuración (`/api/settings/apikeys`). Empiezan por `ak_`; solo se guardan un hash SHA-256 y un prefijo de 8 caracteres.
- Se envían como `X-Api-Key: ak_...` o `Authorization: Bearer ak_...`; solo se aceptan en `/api/v1/*`.
- Cada clave se puede limitar a acciones: `create`, `terminate`, `suspend` (también reactivar), `change-package` y `sso`. Una clave sin acciones, o con la antigua `provision`, puede hacerlo todo.

| Ruta | Acción |
|---|---|
| `GET /api/v1/ping` | ninguna |
| `POST /api/v1/provision` | `create` |
| `POST /api/v1/terminate` | `terminate` |
| `POST /api/v1/suspend`, `POST /api/v1/unsuspend` | `suspend` |
| `POST /api/v1/change-package` | `change-package` |
| `POST /api/v1/sso` | `sso` |

### Recuperación de contraseña

1. `POST /auth/forgot-password` con `{"email": "..."}` crea un token (guardado como hash BLAKE2b-256, válido 30 minutos) y envía un enlace a la dirección guardada en la cuenta.
2. `POST /auth/reset-password` con `{"token": "...", "password": "..."}` lo consume y cambia la contraseña.
3. Ambas responden `204` siempre que pueden, para que no se puedan enumerar cuentas. El enlace usa `panel.settings.masterUrl` cuando tiene valor.

### Peticiones del panel a los nodos

Cada llamada del panel a `/daemon/*` de un nodo (incluidas las rutas `/api/servers/{id}/...` reenviadas y el websocket de consola) lleva `Authorization: Bearer <JWT>`:

- Lo firma el panel con EdDSA (clave Ed25519 de `panel.token`), con ID de clave `SkyPanel`.
- Es válido 15 minutos, con un minuto de margen para diferencias de reloj.
- Su audiencia es un hash SHA-256 derivado del secreto del nodo de destino, así que un token capturado de camino a un nodo no se puede reutilizar contra otro.

`ValidateJWT` en el nodo solo acepta tokens EdDSA con caducidad y esa audiencia (el secreto del nodo local o, en un nodo remoto, `daemon.auth.clientSecret`). La clave de verificación es el propio conjunto de claves del panel cuando el panel corre en el mismo binario, o el JWKS descargado de `token.public` en un nodo remoto. Rutas del daemon sin autenticación: `GET/HEAD /daemon` (estado) y `GET /daemon/features`.

El nodo solo llama al panel para comprobar contraseñas SFTP (la concesión `password` de arriba). Ver [Configuración: nodo remoto](./02-configuration.md#ejemplo-nodo-remoto) y [Seguridad](./09-security.md).

## Comprobación de permisos

```go
g.Handle("GET", "/:serverId", middleware.RequiresPermission(scopes.ScopeServerView), middleware.ResolveServerPanel, getServer)
g.GET("/:serverId/flags", middleware.RequiresAnyPermission(scopes.ScopeServerViewFlags, scopes.ScopeServerEditFlags), ...)
```

`RequiresPermission` (`checkPermission`):

1. Necesita un usuario en el contexto (lo pone `AuthMiddleware`); si no, `403`.
2. Un permiso de servidor necesita `:serverId` en la ruta.
3. Una sesión que viene de un cliente OAuth2 se limita antes a los permisos y al servidor del propio cliente.
4. Reúne los permisos: en un servidor, la fila de permisos de ese servidor, la fila global que haya fijado un administrador y los **permisos de dueño** cuando el usuario es el dueño del servidor. Los permisos de servidor de un rol nunca cuentan en un servidor (darían a cada subusuario los poderes del dueño); fuera de un servidor cuentan los permisos del rol y la fila global.
5. Limita la lista reunida al cliente OAuth2, si la sesión tiene uno (`boundByClient`), y la guarda en la petición como `scopes`. Los controladores leen esa lista (`callerScopes`) para sus propias segundas comprobaciones (contraseñas de bases de datos, variables de entorno bloqueadas, permisos que un usuario puede conceder, permisos de los pasos de las tareas) y para elegir los flujos del WebSocket, así que esas comprobaciones ven exactamente lo mismo que la comprobación de la ruta.
6. Comprueba el permiso con `scopes.ContainsScope`: `admin` cubre todos los permisos y `server.admin` todos los de servidor. Cualquier fallo inesperado responde `500`, nunca da acceso.

## Límites de frecuencia

| Qué | Límite | Clave |
|---|---|---|
| Intentos de contraseña (inicio de sesión del panel, OAuth2 `password`, SFTP) | 10 cada 15 minutos | cuenta |
| Códigos del segundo factor | 10 cada 15 minutos | cuenta |
| Solicitudes de restablecimiento de contraseña | 5 por hora | email |
| Peticiones de IA (análisis de registros y asistente juntos) | 30 cada 10 minutos | usuario |
| Cambios de subdominio | 1 cada 10 segundos | servidor |
| Pruebas de notificaciones | 1 cada 10 segundos | servidor |

Los limitadores usan la cuenta como clave en lugar de la IP, así que funcionan igual detrás de un proxy inverso; viven en memoria y se reinician al reiniciar el panel. Demasiados intentos responden `429`.

## Gestión de puertos

### `PUT /api/servers/{id}/data` (administrador)

Guarda la lista completa de puertos y el puerto principal. Requiere `server.data.edit.admin`.

```json
{ "ports": [25565, 25575, 25585], "primaryPort": 25565 }
```

- `primaryPort` debe ser uno de los puertos; se coloca el primero.
- Cada puerto se comprueba contra el nodo: rango 1024–65535, sin duplicados, que no sea uno de los puertos del propio panel en ese nodo y que no esté asignado a otro servidor allí.
- Primero se actualiza el nodo; si lo rechaza o está caído, el panel no guarda puertos ni límites que el servidor no tiene. El daemon reescribe `port`, `port2`, `port3`… (quitando los que sobran).
- Actualiza `server.Port` y `server.Ports` en la base de datos del panel y refresca los registros del subdominio cuando cambia el puerto principal.

### `PUT /api/servers/{id}/port-settings` (dueño)

Cambia el puerto principal entre los asignados y las etiquetas de los puertos, sin tocar la lista. Requiere `server.data.edit`.

```json
{ "primaryPort": 25565, "portNotes": { "25575": "RCON", "25585": "Voz" } }
// → { "success": true }
```

- Coloca `primaryPort` el primero en `server.Ports` y guarda `server.PortNotes`.
- Envía la lista actual al nodo para mantener sincronizadas `port`/`port2`/`port3`… y refresca el subdominio cuando cambia el puerto principal.

### `POST /api/nodes/{id}/ports/check`

Lo usa el asistente de nuevo servidor antes de crear nada. Requiere `server.create`.

```json
{ "ports": { "port": 25565, "port2": 25566 }, "exclude": "<servidor que se edita>" }
```

Para cada variable responde si el puerto está ocupado, por qué (`range` por debajo de 1024, `panel` para los puertos del propio panel en ese nodo, `server` para otro servidor, `container` para otro contenedor en marcha), un puerto libre alternativo y, solo para administradores, quién lo ocupa.

## Swagger

La documentación OpenAPI se genera a partir de los comentarios de los manejadores (`internal/web/swagger/swagger.json` y `.yaml`) y se sirve en `/swagger/index.html` mediante `gin-swagger`, solo a usuarios con sesión y permiso `admin`, porque describe todas las rutas. Los archivos se regeneran con `swag` a partir de las anotaciones; los actuales son anteriores a las últimas correcciones de anotaciones (varias rutas de copias de seguridad, tareas, definición y mapa del mundo), así que conviene regenerarlos antes de fiarse de esas entradas.
