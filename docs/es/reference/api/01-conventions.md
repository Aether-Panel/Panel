# Convenciones de la API

Lo que comparten todas las rutas del panel: dónde están, qué aceptan, qué responden y cómo funcionan los errores, la paginación y los límites. Volver a la [referencia de la API](../11-api-reference.md).

## Rutas base

| Prefijo | Qué es | Autenticación |
|---------|--------|---------------|
| `/api/...` | API del panel | Cookie de sesión o token bearer, salvo las rutas públicas indicadas abajo |
| `/api/v1/...` | API de facturación / aprovisionamiento | Clave de API (`ak_...`) |
| `/auth/...` | Inicio de sesión, registro, restablecimiento de contraseña | Pública, salvo `/auth/reauth` |
| `/oauth2/token` | Endpoint de tokens OAuth2 | Credenciales de cliente (formulario) |
| `/daemon/...` | API del nodo, a la que llama el panel | JWT del panel (`/daemon` y `/daemon/features` son públicas) |
| `/swagger/*` | Documentación generada de la API | Administrador con sesión iniciada (`admin`) |

Rutas públicas bajo `/api` (no necesitan sesión):

| Ruta | Por qué es pública |
|------|--------------------|
| `GET /api/config` | La página de inicio de sesión lee la marca y la configuración anti-bots antes de que nadie inicie sesión |
| `/api/extransfer/*` | La llama otro panel; cada llamada va firmada (véase [Transferencia externa](./08-external-transfer.md)) |
| `GET/HEAD /api/map-view/:serverId/:token/*path` | La carga un iframe aislado; la ruta lleva un enlace firmado (véase [Mapa del mundo](./04-server-content.md#mapa-del-mundo)) |
| `GET/HEAD /api/extension-files/:id/*path` | Archivos de las extensiones encendidas, cargados por un iframe aislado (véase [Extensiones](./06-panel-admin.md#extensiones)) |
| `/api/v1/*` | Usa una clave de API en lugar de una sesión |

Las rutas desconocidas bajo `/api/`, `/oauth2/` y `/daemon/` responden `404` sin cuerpo; cualquier otra ruta desconocida devuelve el cliente web.

## Peticiones

- Los cuerpos son JSON (`Content-Type: application/json`) salvo que la ruta indique otra cosa.
- Excepciones: `POST /oauth2/token` recibe un formulario (`application/x-www-form-urlencoded`); `POST /api/servers/:serverId/console` recibe el comando como texto plano; las subidas de archivos reciben el cuerpo en bruto o un campo multipart `file`; `POST /api/templates/upload` recibe el campo multipart `files`; `POST /api/extensions` recibe el campo multipart `file`.
- Identificadores en las rutas: los servidores usan su identificador (letras, dígitos, `-` y `_`, de 1 a 20 caracteres); los usuarios, nodos, roles, copias de seguridad, hosts de bases de datos, productos y claves de API usan identificadores numéricos.
- Los filtros de texto de las rutas de listado (`name`, `username`, `email`) admiten `*` como comodín.

## Respuestas

Las respuestas correctas no llevan envoltorio: la ruta devuelve directamente el objeto o el array. Las rutas que no devuelven nada responden `204 No Content`.

### Formato de error

La mayoría de los errores tienen esta forma:

```json
{
  "error": {
    "code": "ErrFieldRequired",
    "msg": "name is required",
    "metadata": { "field": "name" }
  }
}
```

`code` es estable y está pensado para los programas; `msg` es un texto en inglés para las personas; `metadata` solo aparece con algunos códigos. Los códigos se definen en `pkg/skypanel/errors.go`.

Excepciones a esa forma:

| Caso | Respuesta |
|------|-----------|
| Sin sesión, o una cabecera `Authorization` incorrecta | `401` sin cuerpo y `WWW-Authenticate: Bearer realm=""` |
| Falta un scope | `403` sin cuerpo |
| [Protección de mismo origen](./02-authentication.md#protección-de-mismo-origen) | `403 {"error": "cross-origin request refused"}` |
| Middleware de claves de API | `401 {"error": "Unauthorized - Missing or invalid API Key format"}` o `401 {"error": "Invalid API Key"}` |
| `/oauth2/token` | Estilo OAuth2: `{"error": "invalid_client", "error_description": "..."}` |
| `/api/extransfer/*` y las rutas `extransfer` del servidor | `{"error_code": "...", "message": "...", "retryable": false}` |
| Algunas rutas (IA, validación del identificador del servidor, rangos, algunas comprobaciones) | `{"error": "texto plano"}` |
| Rutas reenviadas al nodo | El código de estado y el cuerpo del nodo, tal cual |

### Errores internos

Un fallo inesperado (base de datos, sistema de archivos, red) responde con su código de estado (normalmente `500`), el código `ErrInternal` y el mensaje «internal error, the details are in the panel log». El motivo real solo se escribe en el registro del panel. Los errores que el panel provoca a propósito conservan su código aunque el estado sea `5xx` (por ejemplo, `ErrDatabaseNotAvailable` o `ErrNodeHasServers`).

Por eso, las rutas cuyo fallo debe poder leer el administrador (pruebas de correo, Discord, Cloudflare y S3, orígenes de modpacks, cambios de subdominio) responden `422` en lugar de `502`, para que el texto llegue al navegador.

### Códigos de estado

| Código | Significado en esta API |
|--------|-------------------------|
| `200` | Correcto, con cuerpo |
| `202` | Aceptado; el trabajo continúa en segundo plano (acciones de encendido sin `?wait`, traslado de nodo, recepción externa, cambio de versión del juego, instalación de modpacks y del mapa del mundo, copia de archivos al clonar) |
| `204` | Hecho, sin cuerpo |
| `400` | Cuerpo, parámetro de consulta o valor no válido; o la plantilla del servidor no permite la acción |
| `401` | No hay una sesión, un token o una clave válidos |
| `403` | Falta un permiso, petición de otro origen rechazada, servidor suspendido, función experimental desactivada (`ErrFeatureDisabled`), análisis de registros con IA desactivado (`ErrAIAnalysisDisabled`), registros cerrados (`ErrInviteSignupsClosed`) |
| `404` | El objeto no existe, o quien llama no puede saber que existe |
| `409` | Conflicto con el estado actual: disco lleno (`ErrDiskLimitExceeded`), puerto ocupado, límite de copias (`ErrBackupLimit`), propietario del servidor (`ErrServerOwner`), host de bases de datos aún en uso (`ErrDatabaseHostInUse`), puertos del servidor ocupados en el nodo de destino de un traslado (`ErrTransferPortsTaken`), subdominio ocupado, otro trabajo en curso |
| `413` | Cuerpo de la tarea demasiado grande |
| `422` | Un servicio externo lo rechazó (servidor de correo, Discord, Cloudflare, S3, Modrinth/CurseForge) |
| `429` | Demasiados intentos (`ErrTooManyAttempts`), cuota de IA, cambio de subdominio o prueba de notificaciones demasiado frecuentes |
| `500` | Error interno (véase arriba) |
| `502` | No se pudo contactar con el nodo o con un servicio externo (algunas rutas) |
| `503` | Transferencia externa: el archivo del origen aún no está listo (`NOT_READY`, reintentable) |

### Códigos de error comunes

| Código | Cuándo |
|--------|--------|
| `ErrFieldRequired`, `ErrFieldLength`, `ErrFieldTooSmall`, `ErrFieldNotEmail`, `ErrFieldMustBePrintable`, `ErrFieldIsInvalidHost` | Validación de un campo del cuerpo o de la consulta (`metadata.field` indica cuál) |
| `ErrInvalidCredentials` | Correo, contraseña o código 2FA incorrectos |
| `ErrTooManyAttempts` | Inicio de sesión limitado (véase [Límites de frecuencia](#límites-de-frecuencia)) |
| `ErrLoginNotPermitted` | La cuenta no tiene el scope `login` |
| `ErrCaptchaFailed` | Falta la comprobación anti-bots o es incorrecta |
| `ErrPasswordRequirements` | Contraseña de menos de 8 bytes o por encima del límite de bcrypt |
| `ErrTokenInvalid` | Token de restablecimiento de contraseña desconocido, ya usado o caducado |
| `ErrInvalidSession`, `ErrSessionExpired` | El paso de 2FA no tiene un inicio de sesión pendiente, o tiene más de 5 minutos |
| `ErrNoPermission` | La acción no está permitida para esta cuenta |
| `ErrUserNotFound`, `ErrServerNotFound` | Usuario o servidor desconocido |
| `ErrNodeInvalid`, `ErrNodeHasServers` | Nodo desconocido; no se puede eliminar un nodo con servidores |
| `ErrServerOwner` | El propietario de un servidor no se puede limitar ni quitar |
| `ErrInviteSignupsClosed` | Invitación a un correo desconocido con los registros cerrados |
| `ErrBackupLimit`, `ErrBackupServerRunning`, `ErrBackupInProgress` | Límite de copias alcanzado; restauración con el servidor en marcha; otra copia en curso |
| `ErrDiskLimitExceeded`, `ErrNodeDiskFull` | Disco del servidor lleno; nodo por debajo de `daemon.minFreeDiskMB` antes de una instalación |
| `ErrFileExists`, `ErrIllegalFileAccess` | Mover sobre una ruta que ya existe; ruta fuera de la carpeta del servidor |
| `ErrFeatureDisabled` | Función experimental desactivada en la configuración |
| `ErrAIAnalysisDisabled` | Análisis de registros con IA desactivado (`panel.settings.hideAiAnalysis`) |
| `ErrAPIKeyNotAllowed`, `ErrAPIKeyPermissions` | La clave de API no tiene esa acción; clave creada sin ninguna acción conocida |
| `ErrSSONotForAdmins` | Se pidió un enlace de acceso para una cuenta privilegiada |
| `ErrDatabaseHostFull`, `ErrDatabaseHostOtherNode`, `ErrDatabaseNameTaken`, `ErrInvalidDatabaseName` | Creación de base de datos rechazada |
| `ErrDatabaseHostInUse` | No se puede eliminar un host de bases de datos que aún tiene bases de datos |
| `ErrTransferPortsTaken` | Traslado de nodo rechazado: algunos puertos del servidor están en uso en el nodo de destino (`metadata.ports` los enumera) |
| `ErrPluginNoCompatibleVersion` | El proyecto de Modrinth no tiene ninguna versión para el cargador y la versión de Minecraft del servidor |
| `ErrSSHKeyInvalid`, `ErrSSHKeyIsPrivate`, `ErrSSHKeyWeak`, `ErrSSHKeyExists`, `ErrSSHKeyLimit`, `ErrSSHKeyName` | Clave SSH rechazada |
| `ErrOtpAlreadyEnabled` | Alta de 2FA con la 2FA ya activada |
| `ErrInternal` | Fallo inesperado; véase el registro del panel |

## Paginación

Las listas que pueden crecer sin límite van paginadas. Hay dos estilos.

**Página y límite.** `GET /api/servers`, `GET /api/users` y `GET /api/activity` admiten `page` (por defecto `1`) y `limit` (por defecto `20`; `10` en `/api/activity`; como máximo `100`). La respuesta incluye un objeto `paging` junto a la lista:

```json
{
  "servers": [ ... ],
  "paging": { "page": 1, "pageSize": 20, "maxSize": 100, "total": 57 }
}
```

`total` es el número de filas que coinciden, sumando todas las páginas. En `GET /api/servers`, un `page` o un `limit` de `0` o menos responde `400 ErrFieldTooSmall`; en las tres rutas, un `limit` mayor que `100` se reduce a `100`.

**Cursor.** `GET /api/servers/:serverId/activity` admite `limit` (por defecto `50`, como máximo `100`) y `before` (el id de una entrada). La respuesta es `{"items": [...], "nextCursor": 123}`; para obtener la página siguiente se pasa `nextCursor` como `before`. En la última página `nextCursor` es `null`.

Las demás listas devuelven todas sus filas de una vez.

## Cabeceras

| Cabecera | Dónde |
|----------|-------|
| `Cache-Control: no-store` | Todas las respuestas de `/api`; `/oauth2/token` envía además `Pragma: no-cache` |
| `X-API-Version` | Respuestas autenticadas de `/api`: la versión del panel |
| `Content-Security-Policy: frame-ancestors 'self'; object-src 'none'; base-uri 'self'` | Todas las respuestas (los archivos de extensiones y el visor del mapa del mundo envían en su lugar una política sandbox) |
| `X-Frame-Options: SAMEORIGIN`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin` | Todas las respuestas |
| `Strict-Transport-Security: max-age=31536000` | Cuando la petición llegó por HTTPS, directamente o con `X-Forwarded-Proto: https` |

### CORS

El panel responde a peticiones de otros orígenes, de cualquier origen, pero nunca con credenciales (no se envía `Access-Control-Allow-Credentials`). Cabeceras de petición permitidas: `Authorization`, `Content-Type`, `Accept`, `Origin`; métodos: `GET`, `POST`, `PUT`, `DELETE`, `OPTIONS`. En la práctica, otro sitio web puede llamar a la API con un token bearer, pero nunca con la cookie de sesión de un visitante. Las peticiones previas (preflight) del navegador las responde la capa CORS; una petición `OPTIONS` normal a una ruta responde `200` con una cabecera `Allow` que enumera sus métodos.

## Límites de frecuencia

Todos los contadores se guardan en memoria y se reinician cuando se reinicia el panel.

| Qué | Límite | Respuesta |
|-----|--------|-----------|
| Intentos de contraseña y 2FA (inicio de sesión en el panel, concesión de contraseña de OAuth2, SFTP) | 10 por cuenta cada 15 minutos; un acierto reinicia el contador | `429 ErrTooManyAttempts` |
| Solicitudes de restablecimiento de contraseña | 5 por correo cada hora | `204` como siempre (el límite no se revela) |
| IA (análisis de registros y chat en conjunto) | 30 por usuario cada 10 minutos | `429 {"error": "too many AI requests, try again in a few minutes"}` |
| Cambios de subdominio | Uno por servidor cada 10 segundos | `429 ErrSubdomainRateLimited` |
| Prueba de notificaciones | Una por servidor cada 10 segundos | `429` |
| Desafío anti-bots integrado | Cada solución sirve una sola vez y caduca a los 10 minutos | `400 ErrCaptchaFailed` |

## Swagger

El panel sirve documentación OpenAPI generada en `/swagger/index.html`. Solo puede abrirla un usuario con sesión iniciada y el scope `admin` (se comprueba la cookie de sesión, así que debe abrirse desde el navegador en el que se ha iniciado sesión). Se genera a partir de anotaciones del código. Los métodos, rutas y scopes de varias rutas de copias de seguridad, tareas, definición y mapa del mundo se corrigieron en las anotaciones, pero los archivos que sirve el panel aún no se han regenerado, así que siguen mostrando los valores antiguos; algunas anotaciones todavía nombran scopes que no existen, como `server.query`. Donde difiera, prevalece esta referencia.
