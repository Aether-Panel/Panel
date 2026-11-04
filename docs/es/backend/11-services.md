# Servicios (`internal/services`)

`internal/services` es la capa de negocio entre los handlers HTTP y la base de datos. La mayoría de los servicios son un struct con un campo `DB *gorm.DB` y métodos que envuelven consultas GORM junto con las reglas que las acompañan (hashes, límites, borrados en cascada). Unos pocos son singletons o funciones sueltas que hablan con servicios externos (Discord, Telegram, Cloudflare, la API de licencias).

Hoy el paquete define **23 tipos de servicio** y **5 módulos auxiliares** (limitación de intentos, enlaces de inicio de sesión, canales de notificación, estado del dominio, transporte al nodo local). La antigua cifra de "19 servicios" y nombres como `UserService` o `SessionService` ya no corresponden al código: los tipos se llaman `User`, `Session`, etc.

---

## Uso de un servicio

Los servicios de base de datos no tienen constructores; los handlers los crean en el momento con la conexión de la petición:

```go
db := middleware.GetDatabase(c)      // la deja middleware.NeedsDatabase
us := &services.User{DB: db}
user, err := us.GetByID(id)
```

`middleware.NeedsDatabase` llama a `database.GetConnection()` y guarda el `*gorm.DB` en la clave `"db"` del contexto de Gin; `middleware.GetDatabase(c)` lo recupera. El código que se ejecuta fuera de una petición (los bucles del daemon, SFTP) llama directamente a `database.GetConnection()`. Si se pasa una transacción (`tx`) como `DB`, el servicio trabaja dentro de ella; así obtienen atomicidad `Permission.SetServerOwner` y quienes llaman a `Server.Delete`.

---

## Resumen

| # | Tipo | Archivo | Clase | Responsabilidad |
|---|---|---|---|---|
| 1 | `Session` | `session.go` | BD | Tokens de sesión: crear, validar (caducidad deslizante), caducar |
| 2 | `User` | `user.go` | BD | Cuentas, inicio de sesión, 2FA, restablecimiento de contraseña, búsqueda |
| 3 | `Server` | `server.go` | BD | Filas de servidores: buscar, obtener, crear, actualizar, borrado en cascada |
| 4 | `Node` | `node.go`, `node_local.go` | BD | Nodos, dirección del nodo local, llamadas y proxy WebSocket a los daemons |
| 5 | `Permission` | `permission.go` | BD | Filas de permisos, propiedad, comprobación de scopes |
| 6 | `Role` | `role.go` | BD | CRUD de roles; protege los roles predeterminados |
| 7 | `APIKeyService` | `apikey.go` | BD | Claves de API de administración (`ak_...`) |
| 8 | `Backup` | `backup.go` | BD | Filas de metadatos de copias de seguridad |
| 9 | `Database` | `database.go` | BD + MySQL | Bases de datos y usuarios MySQL por servidor |
| 10 | `DatabaseHost` | `databasehost.go` | BD | CRUD de hosts de bases de datos |
| 11 | `Template` | `templates.go` | BD + Git/HTTP | Repositorios de plantillas y plantillas |
| 12 | `OAuth2` | `oauth2.go` | BD | Filas de clientes OAuth2 |
| 13 | `UserSettings` | `usersettings.go` | BD | Ajustes clave/valor por usuario |
| 14 | `Activity` | `activity.go` | BD | Registro de accesos al panel, retención de 90 días |
| 15 | `ServerActivity` | `server_activity.go` | BD | Registro de actividad por servidor, filtros, paginación por cursor |
| 16 | `Uptime` | `uptime.go` | BD | Periodos en línea/apagado por servidor |
| 17 | `SSHKey` | `sshkey.go` | BD | Claves públicas SFTP de los usuarios |
| 18 | `DatabaseSFTPAuthorization` | `sftp.go` | BD | Inicio de sesión SFTP con contraseña o clave |
| 19 | `TokenService` (interfaz) | `token.go` | Criptografía | JWT EdDSA para las llamadas panel → daemon, JWKS |
| 20 | `EmailSender` (interfaz) | `email.go` | Singleton | Plantillas y envío de correo |
| 21 | `DiscordService` | `discord.go` | Singleton | Embeds para webhooks de Discord |
| 22 | `LicenseService` | `license.go` | Singleton | API externa de licencias |
| 23 | `Cloudflare` | `cloudflare.go` | Cliente de API | Registros DNS de una zona de Cloudflare |

Módulos auxiliares:

| Módulo | Archivo | Responsabilidad |
|---|---|---|
| `AttemptLimiter` (`LoginAttempts`, `PasswordResetRequests`) | `login_attempts.go` | Limitación de intentos en memoria por cuenta |
| `SSOLinks` | `sso.go` | Enlaces de inicio de sesión de un solo uso para sistemas de facturación |
| Telegram / webhook JSON | `channels.go` | Validación y envío para las notificaciones del propietario y las extensiones |
| Textos de las notificaciones | `notification_text.go` | Textos que se envían fuera de las páginas del panel, en español e inglés |
| `ReadDomainStatus` | `domain_status.go` | Lee lo que hizo el servicio de dominio del instalador |
| `serveLocal` | `node_local.go` | Ejecuta en el propio proceso una "llamada al nodo" dirigida al nodo del panel |

---

## 1. `Session` (`session.go`)

| Método | Descripción |
|---|---|
| `CreateForUser(user) (string, error)` | Nueva sesión para un usuario |
| `CreateForClient(client) (string, error)` | Nueva sesión para un cliente OAuth2 (registra también al propietario del cliente como `UserID`) |
| `Validate(token) (*models.Session, error)` | Busca una sesión viva y la renueva |
| `ValidateNode(token) (*models.Node, error)` | Busca el nodo cuyo `Secret` coincide con el token (primero el nodo local) |
| `Expire(token) error` | Borra la sesión; que no exista no es un error |
| `ExpireAllForUser(userID, keepToken) error` | Borra todas las sesiones del usuario salvo `keepToken` (se usa tras cambiar la contraseña) |
| `PruneExpired(now) error` | Borra todas las sesiones caducadas a fecha de `now` |
| `HashToken(source) (string, error)` | Función del paquete: SHA-256 en hexadecimal |

- El token que se entrega es un UUID v4; solo se guarda su hash SHA-256.
- `SessionLength = time.Hour`. La caducidad es deslizante: `Validate` la lleva a ahora + 1 h cuando queda menos de la mitad (30 min), así que hay como mucho una escritura por cada media hora de actividad.
- `Validate` precarga `Client`, `User.Permissions`, `User.Role` y `Server`, e ignora las filas sin usuario ni cliente.
- Las filas caducadas las borra `PruneExpired(now)`, que `CreateForUser`/`CreateForClient` ejecutan tras guardar la nueva sesión como mucho una vez por hora (`sessionPruneEvery`, compartido por todas las peticiones). Si la limpieza falla solo se registra en el log; nunca hace fallar el inicio de sesión.

## 2. `User` (`user.go`)

| Método | Descripción |
|---|---|
| `Get(username)`, `GetByID(id)`, `GetByEmail(email)` | Búsquedas; un valor vacío devuelve `ErrRecordNotFound` en lugar de la primera fila |
| `ValidateLogin(email, password) (user, otpNeeded, err)` | Comprueba la contraseña con limitación de intentos; `otpNeeded` si el 2FA está activo |
| `ValidOtp(email, token) (user, err)` | Código TOTP o código de recuperación (que se consume) |
| `IsValidCredentials(user, password) bool` | Comparación bcrypt |
| `Create`, `Update` | Crear / guardar sin más |
| `Delete(user)` | Transacción: borra permisos, clientes OAuth2, sesiones, códigos de recuperación, restablecimientos pendientes, claves SSH y preferencias (`user_settings`) del usuario, pone a `NULL` el `owner_id` de sus servidores y borra el usuario |
| `ChangePassword(username, newPass)` | Vuelve a generar el hash y guarda |
| `CreatePasswordResetToken(email) (token, accountEmail, err)` | Un restablecimiento por usuario (se borran los anteriores), válido 30 minutos; devuelve la dirección guardada a la que enviar |
| `ConsumePasswordResetToken(token) (*models.User, error)` | Comprueba la caducidad y borra el restablecimiento |
| `GetOtpStatus(userID)` | Si el 2FA está activo |
| `StartOtpEnroll(userID) (secret, qrDataURI, err)` | Genera un secreto TOTP que queda *pendiente*; se niega con `ErrOtpAlreadyEnabled` si el 2FA ya está activo |
| `ValidateOtpEnroll(userID, token) ([]string, error)` | Confirma el secreto pendiente, activa el 2FA y devuelve 10 códigos de recuperación nuevos |
| `RegenerateOtpRecoveryCodes(userID)` | Sustituye los códigos de recuperación |
| `DisableOtp(userID)` | Borra secretos y códigos de recuperación |
| `Search(usernameFilter, emailFilter, pageSize, page)` | `*` funciona como comodín; precarga `Role` y `Permissions` |
| `IsSecurePassword(password) error` | Solo comprueba la longitud: de 8 a 72 **bytes** (el límite de bcrypt) |

Detalles verificados en el código:

- Contraseñas: bcrypt con `bcrypt.DefaultCost` (`models.User.SetPassword`).
- Limitación: `LoginAttempts` (10 intentos / 15 min) con la clave `password:user:<id>` (o el email en minúsculas si no hay cuenta); los códigos 2FA usan una clave `otp:` aparte. Un intento correcto reinicia el contador. Con un email desconocido se hace igualmente una comparación bcrypt para que el tiempo de respuesta no revele qué emails existen.
- TOTP con `github.com/pquerna/otp` y sus valores por defecto, emisor `panel.settings.companyName`, código QR como data URI PNG de 256×256.
- Códigos de recuperación: 10 códigos de 12 caracteres alfanuméricos, guardados como hash BLAKE2b-256. Tokens de restablecimiento: `crypto/rand.Text()`, guardados como BLAKE2b-256.

## 3. `Server` (`server.go`)

```go
type ServerSearch struct {
    Username string
    NodeID   uint
    NodeName string   // "LocalNode" = servidores con node_id NULL
    Name     string   // comodín *
    PageSize uint
    Page     uint
}
```

| Método | Descripción |
|---|---|
| `Search(ServerSearch) ([]*models.Server, total, err)` | Filtra por nodo (id o nombre), por las filas de permisos de un usuario (`Username`) y por nombre; ordena por nombre y precarga las asociaciones |
| `Get(identifier)` | Por el identificador público |
| `GetByInternalID(internalID)` | Por el UUIDv7 interno inmutable |
| `Create(model)` | Genera un identificador de 12 caracteres (hasta 5 intentos ante colisiones) y el UUIDv7 interno si faltan |
| `Update(model)` | Guarda sin asociaciones y sin tocar `node_id` |
| `Delete(identifier)` | Borra permisos, clientes OAuth2, copias, las bases de datos MySQL del servidor (véase `Database.DropAllForServer`) y sus filas, filas de uptime, sesiones ligadas al servidor y por último el servidor. No usa transacción propia: quien llama la aporta. |

`Search` no comprueba por sí mismo los permisos de quien llama; los handlers deciden qué pasarle.

## 4. `Node` (`node.go`, `node_local.go`)

| Método / función | Descripción |
|---|---|
| `GetAll()` | Nodos de la base de datos más `models.LocalNode` cuando `panel.enable` es true |
| `Get(id)` | El nodo local para su id; si no, una fila de la base de datos |
| `Create`, `Update`, `Delete` | `Update` no hace nada con el nodo local; `Delete` rechaza el nodo local y los nodos que aún tienen servidores (`ErrNodeHasServers`) |
| `CallNode(node, method, path, body, headers)` / `CallNodeContext(ctx, ...)` | Llamada HTTP al daemon del nodo con un JWT nuevo ligado a ese nodo |
| `OpenSocket(node, path, w, r)` | Actualiza la conexión del navegador y la redirige al WebSocket del daemon (`ws`/`wss`) |
| `SyncNodeToConfig()` | Se ejecuta en `init`: host/puerto del nodo local desde `panel.settings.masterUrl`, puerto SFTP desde `daemon.sftp.host` |
| `KeepLocalNodeHostInSync(db)` | Se llama al arrancar; actualiza la dirección del nodo local en ese momento y cada 15 s |

**Dirección del nodo local** (`syncLocalNodeHost`): el ajuste del panel `master_node_ip` si existe; si no, cuando el dominio de la Base URL está tras un proxy (nube naranja de Cloudflare, leído de `domain-status.json`), la IP pública del servidor; si no, el host de la Base URL.

**Llamadas a un nodo** (`CallNodeContext`):
- Se copian las cabeceras del navegador y se quita `Cookie`: los nodos se autentican con el token del panel, nunca con la sesión del usuario.
- `Authorization: Bearer <JWT>` de `TokenService.GenerateRequest(node)`.
- Nodo local: la petición se atiende dentro del proceso con `serveLocal` (las rutas del daemon en `skypanel.Engine`). Las respuestas de hasta 1 MiB se guardan en memoria; las mayores (descargas, copias) se transmiten por una tubería.
- Nodo remoto: `http(s)://PrivateHost:PrivatePort/<ruta>`. El esquema lo decide `doesDaemonUseSSL`: una petición `OPTIONS /daemon` por https y después por http (10 s de límite), cacheada 5 minutos. Un nodo al que se ha visto hablar TLS queda fijado (`nodes.ssl = true` en la base de datos) y nunca se rebaja a http.

## 5. `Permission` (`permission.go`)

| Método | Descripción |
|---|---|
| `GetForUser(id)`, `GetForServer(serverID)`, `GetForClient(id)` | Filas de un usuario, servidor o cliente OAuth2 |
| `GetForUserAndServer(userID, serverID)` | Una fila (global si `serverID` está vacío); si no existe devuelve una fila vacía sin guardar |
| `GetForClientAndServer(id, serverID)` | `FirstOrCreate` para un cliente OAuth2 |
| `IsServerOwner(userID, serverID)` | Comprueba `servers.owner_id` |
| `SetServerOwner(serverID, userID)` | Transacción: el nuevo propietario recibe una fila con `server.view` si no tenía; el anterior conserva la suya |
| `HasPermission(userID, serverID, scope)` | Véase abajo |
| `MaySignIn(userID)` | `HasPermission(userID, "", login)`; lo comprueban el inicio de sesión web, OAuth2 y SFTP |
| `UpdatePermissions(perms)` | Guarda la fila (las filas nunca se borran al quedarse vacías) |
| `Remove(perms)` | Borra la fila |

Orden de `HasPermission`: (1) en scopes de servidor, el propietario tiene todo lo que hay en `scopes.OwnerScopes`; (2) la fila del usuario en ese servidor o su fila global; (3) su rol, donde cada scope del rol cuenta en todo el panel pero los scopes **de servidor** del rol no se aplican en un servidor concreto (la misma regla que `middleware.collectScopes`). Reglas completas: [Capa de API](./05-api-layer.md) y [referencia de scopes](../general/scopes-reference.md).

## 6. `Role` (`role.go`)

`Get(id)`, `GetByName(name)`, `List()` (ordenado por nombre), `Create`, `Update`, `Delete(id)`.

`Delete` rechaza los roles llamados `admin`, `Administrador` y `Usuario` (los predeterminados que crean las migraciones de `internal/database/upgrade.go`), pone `role_id = NULL` a los usuarios del rol y lo borra, todo en una transacción.

## 7. `APIKeyService` (`apikey.go`)

| Método | Descripción |
|---|---|
| `GenerateKey(name, permissions) (token, *models.APIKey, err)` | 24 bytes aleatorios → `ak_<5 hex>_<43 hex>`; guarda el prefijo de 8 caracteres y el SHA-256 del token completo |
| `ValidateKey(token)` | Comprueba el formato y busca prefijo + hash |
| `GetAll()`, `Delete(id)` | Listar y borrar |

El token en claro solo se devuelve una vez, al crearlo. Véase [Seguridad](./09-security.md).

## 8. `Backup` (`backup.go`)

`GetAllForServer(serverID)`, `Get(serverID, id)`, `Create`, `Update` (sin asociaciones), `Delete(id)`. Los **archivos** de copia están en el nodo (y opcionalmente en un bucket S3, véase [offsite](./10-remaining-packages.md#internaloffsite--copias-externas)); este servicio solo maneja las filas. Funcionalidad: [Copias de seguridad](../features/backups.md).

## 9. `Database` (`database.go`)

| Método | Descripción |
|---|---|
| `GetAllForServer(serverID)`, `Get(id)` | Filas con su `DatabaseHost` precargado |
| `UsableHosts(serverID)` | Hosts que el servidor puede usar: ligados a su nodo (o a ninguno) y por debajo de `MaxDatabases` |
| `Create(database)` | Comprueba el host, construye el nombre, crea base de datos y usuario en MySQL y guarda la fila |
| `Delete(id)` | Elimina el usuario y la base de datos de MySQL y después la fila |
| `DropAllForServer(serverID)` | Elimina, en la medida de lo posible, todas las bases de datos de un servidor; los fallos se registran en el log, no se devuelven |

Reglas:
- Nombre físico `s_<serverId>_<nombre>`, solo `[a-zA-Z0-9_-]`, como máximo 64 caracteres (`ErrInvalidDatabaseName`); único por host (`ErrDatabaseNameTaken`).
- Límites del host: `ErrDatabaseHostFull` (`MaxDatabases`, 0 = sin límite) y `ErrDatabaseHostOtherNode`.
- Usuario generado `db_` + 10 caracteres alfanuméricos en minúscula, contraseña de 16 caracteres; `RemoteConnection` = `host:puerto`.
- Sentencias MySQL: `CREATE DATABASE` **sin** `IF NOT EXISTS` (utf8mb4 / utf8mb4_unicode_ci), `CREATE USER ... @'%'`, `GRANT ALL PRIVILEGES ON bd.*`, `FLUSH PRIVILEGES`. Si falla algún paso cuando la base de datos ya existe, se eliminan de nuevo el usuario y la base de datos. Límites de 5 s para conectar, leer y escribir. Funcionalidad: [Hosts de bases de datos](../features/database-hosts.md).

## 10. `DatabaseHost` (`databasehost.go`)

CRUD: `GetAll`, `Get`, `Create`, `Update`, `Delete`. `Delete` cuenta primero las bases de datos del host y devuelve `skypanel.ErrDatabaseHostInUse` mientras quede alguna (la API responde `409`): quitar el host borraría en cascada sus filas y dejaría las bases de datos y sus usuarios en el servidor MySQL sin nada en el panel para eliminarlos.

## 11. `Template` (`templates.go`)

| Método | Descripción |
|---|---|
| `GetLocalRepoID()` | Siempre `0` |
| `GetRepos()` | Repos de la base de datos más el repo sintético `Local` (id 0). Si `templates.url` tiene valor, devuelve un repo sintético `community` (id 1, rama `vps`) más `Local` e ignora los repos de la base de datos. |
| `GetAllFromRepo(repoID)` | Resúmenes de plantillas (nombre, display, tipo, entorno, entornos soportados, requisitos) |
| `Get(repoID, name)` | Plantilla completa, con `README.md` en los repos Git |
| `Save(template)`, `Delete(name)` | Plantillas locales (base de datos) |
| `AddRepo(repo)` | Rechaza nombres repetidos (`ErrRepoExists`) |
| `DeleteRepo(id)` | Borra la fila del repo |

Orígenes:
- **Local (id 0)**: filas de la tabla `templates`.
- **Repos Git**: se clonan con go-git en `<daemon.data.cache>/template-repos/<id>`, una sola rama, y se actualizan con un pull cuando la copia tiene más de 5 minutos. Cada carpeta de primer nivel puede contener archivos `<nombre>.json` (se omite `data.json`) y un `README.md`.
- **Índice JSON (`templates.url`)**: un objeto que asocia nombres de plantilla a `{ "url": "..." }`; las URL relativas se resuelven respecto a la URL del índice. Las descargas usan 10 s de límite y cabeceras de navegador. El índice, y la lista completa de plantillas leídas de él, se guardan en memoria 5 minutos (`vpsCacheTTL`) para esa URL; un `templates.url` cambiado nunca se sirve desde la caché anterior, y una lista en la que falló la descarga de alguna plantilla no se guarda, así que la siguiente petición lo vuelve a intentar.

```json
{
  "minecraft-java": { "url": "minecraft-java.json" },
  "paper": { "url": "https://example.com/templates/paper.json" }
}
```

**Repositorios privados.** `TemplateRepo` tiene las columnas `pat`, `username`, `password` y `sshKey`, que `POST /api/templates` acepta y nunca devuelve. `repoAuth` las convierte en credenciales de go-git para `cloneRepo` y `pullRepo`:

| Dirección | Credenciales usadas |
|---|---|
| `https://` / `http://` | `pat` como contraseña de autenticación básica HTTP (usuario `username`, o `git` si está vacío); si no, `username` + `password`; ninguna si todo está vacío |
| SSH (`git@host:...`, `ssh://`) | `sshKey` (clave privada PEM; `password` la desbloquea si está cifrada), usuario `username`, si no el usuario de la URL, si no `git`. La clave del host debe estar en el `known_hosts` de la cuenta que ejecuta el panel. Sin `sshKey`, ninguna credencial |

Los errores que devuelve `repoAuth` nombran el repositorio y el tipo de credencial que no se pudo usar, nunca las credenciales ni la dirección. El cliente web aún no tiene formulario para repositorios; se añaden mediante la API. Funcionalidad: [Plantillas](../features/templates.md).

## 12. `OAuth2` (`oauth2.go`)

`Get(clientID)` (por la cadena del client id), `GetForUser(userID)`, `Create`, `Update`, `Delete(clientID)`. `Delete` borra también, en una transacción, las sesiones abiertas con las credenciales del cliente. Flujos OAuth2: [Capa de API](./05-api-layer.md#oauth2).

## 13. `UserSettings` (`usersettings.go`)

`GetAllForUser(userID)` devuelve un mapa `models.UserSettingsView`; `Update(setting)` crea o guarda la fila `(usuario, clave)`.

## 14. `Activity` (`activity.go`)

`Record(event, userID, username, email, ip, userAgent)` añade un evento de acceso; `List(limit, page, event)` devuelve los más recientes primero, con el total. `ActivityRetention` = 90 días; las filas más antiguas se borran cada vez que se lee la lista. Funcionalidad: [Actividad y registro de accesos](../features/activity-and-access-log.md).

## 15. `ServerActivity` (`server_activity.go`)

| Método | Descripción |
|---|---|
| `Record(entry)` | Recorta detalle/usuario/IP, guarda y, como mucho una vez por hora, purga entradas antiguas y las de servidores borrados |
| `Prune(now)` | Retención (90 días) y limpieza de huérfanas |
| `DeleteForServer(serverID)` | Borra el registro de un servidor |
| `List(serverID, limit, before, filter)` | Más recientes primero; `limit` por defecto 50, máximo 100; `before` es el cursor por id; devuelve el siguiente cursor o nil |
| `TruncateRunes(text, limit)` | Función auxiliar del paquete, corta con `…` |

Filtros: un nombre de familia (`power`, `console`, `files`, `backups`, `users`, `settings`, `tasks`, véase `ServerActivityFamilies`) o un prefijo de acción en minúsculas como `file.` o `backup.restore`. Cualquier otra cosa no devuelve nada. La familia `settings` incluye `server.transferFailed` y `server.transferStartFailed`, que registra un traslado entre nodos después de responder a la petición, con el paso y el motivo como detalle (`internal/web/api/transfer_logic.go`).

## 16. `Uptime` (`uptime.go`)

| Método | Descripción |
|---|---|
| `TrackStatus(serverID, isRunning)` | Abre un periodo, lo cierra (con `duration`) al cambiar el estado o actualiza `updated_at`. Omite los servidores que no están en la base de datos. |
| `GetUptimeStats(serverID, since)` | Segundos en línea/apagado entre `since` y ahora, periodo en curso incluido, y porcentaje (100 si no hay datos) |
| `GetRecentHistory(serverID, limit)` | Últimos periodos |
| `GetAllServerUptime(since)` | Mapa por servidor con `uptime`, `downtime`, `uptimePercent`, `currentStatus`, `currentStartTime` |
| `RecordNodeStatuses(serverIDs, reported, staleAfter)` | Registra lo que un nodo remoto informó de los servidores que el panel tiene en él (ver abajo) |

El bucle de estadísticas del daemon llama a `TrackStatus` cada 5 segundos para cada servidor que ejecuta el propio proceso del panel, solo donde se ejecuta el panel ([Gestión de procesos y métricas](./13-process-metrics.md#seguimiento-del-uptime)). Las estadísticas leen todos los periodos que se solapan con la ventana (aún abiertos, o terminados después de `since`) y cuentan solo la parte que cae dentro: un periodo que empezó antes de `since` se recorta a `since`, y el periodo abierto llega hasta ahora. El periodo en curso se cuenta una sola vez: `GET /api/uptime/:serverId` no lo vuelve a sumar a `totalSeconds`. Descripción funcional: [Uptime](../general/13-uptime.md).

`RecordNodeStatuses` lo llama cada 30 segundos, para cada nodo remoto, el sondeo de estado de nodos remotos del panel ([Gestión de procesos y métricas](./13-process-metrics.md#seguimiento-del-uptime)). `reported` contiene el estado de cada servidor por el que respondió el nodo, y es `nil` cuando el nodo no respondió. Desconocido no es apagado: un servidor que el nodo no incluyó, o todos los de un nodo que no respondió, no registran nada. Antes de registrar, cierra los periodos abiertos de esos servidores que nadie ha confirmado durante `staleAfter` (3 minutos), en su última confirmación (`updated_at`), de modo que una caída larga no cuenta ni como encendido ni como apagado (queda un hueco entre periodos); cuando el nodo vuelve a responder empieza un periodo nuevo.

## 17. `SSHKey` (`sshkey.go`)

| Método | Descripción |
|---|---|
| `GetForUser(userID)` | Más recientes primero |
| `Add(userID, name, line)` | Una línea de `authorized_keys`; el nombre por defecto es el comentario de la clave (1–100 caracteres) |
| `Delete(userID, id)` | Solo claves del propio usuario |
| `FindForLogin(userID, offered)` | Búsqueda por huella y después comparación de la clave completa |
| `ParseSSHPublicKey(line)` | Función del paquete que usa `Add` |

Límites: 25 claves por usuario (`ErrSSHKeyLimit`), sin duplicados por usuario (`ErrSSHKeyExists`). Se aceptan: Ed25519, ECDSA P-256/384/521, `sk-` Ed25519/ECDSA y RSA de 2048 bits o más. Se rechazan: claves privadas, certificados, DSA y claves con opciones (`ErrSSHKeyIsPrivate`, `ErrSSHKeyWeak`, `ErrSSHKeyInvalid`).

## 18. `DatabaseSFTPAuthorization` (`sftp.go`)

Implementa `skypanel.SFTPAuthorization` y `skypanel.SFTPKeyAuthorization`. El usuario es `email#serverId`.

- `Validate(username, password)`: el mismo cupo de `LoginAttempts` que el inicio de sesión web; comprobación bcrypt.
- `ValidateKey(username, key)`: clave de la lista `SSHKey` del usuario. Cada clave ofrecida para una cuenta existente consume un intento del cupo `sshkey:user:<id>` de `LoginAttempts` (el que usa `/auth/sftp-key` para los nodos remotos, separado del de contraseñas); superado el cupo, el inicio de sesión falla con `ErrTooManyAttempts`, y un inicio de sesión con clave correcto lo reinicia.
- En ambos casos se exige después `MaySignIn` y el scope `server.sftp` en ese servidor, y se devuelve un `ssh.Permissions` con la extensión `server_id`. Cualquier fallo responde con el mismo `incorrect username or password`.

Véase [Seguridad: SFTP](./09-security.md).

## 19. `TokenService` (`token.go`)

```go
type TokenService interface {
    GetKeyFunc() jwt.Keyfunc
    GetTokenStore() jwkset.Storage
    GenerateRequest(node *models.Node) (string, error)
    ValidateRequest(string) error
}
```

- `NewTokenService()` mantiene estado para todo el proceso. En un panel (`panel.enable`), la clave Ed25519 sale de la semilla de `panel.token`; si está vacía, se genera una semilla y se guarda. La clave pública se publica mediante `GetTokenStore()` (la sirve `internal/web/auth/jwks.go`). En un nodo solo daemon, las claves se descargan de `token.public`.
- `GenerateRequest(node)`: JWT EdDSA, `kid` `SkyPanel`, claims `iat`, `exp` (+15 min) y `aud` = SHA-256 de `"skypanel-daemon-aud:" + secreto del nodo`, de modo que un token solo sirve en el nodo para el que se emitió.
- `ValidateRequest(token)` (lo usa `middleware.ValidateJWT`): solo EdDSA, `exp` obligatorio, 1 min de margen, y `aud` debe coincidir con el secreto del nodo local o con `daemon.auth.clientSecret`.

## 20. `EmailSender` (`email.go`)

- `LoadEmailService()` (al arrancar) lee `emails.json` y los cuerpos HTML de `assets/email` (embebidos), combinados con `panel.email.templateFolder` si tiene valor (los archivos de esa carpeta tienen prioridad). Asuntos y cuerpos son `html/template` de Go.
- `GetEmailService().SendEmail(to, template, data, async)`: necesita `panel.email.provider` (`ErrEmailNotConfigured`), añade `COMPANY_NAME`, `MASTER_URL` y `HOST_NAME` (el host de `panel.settings.masterUrl`, o el nombre de la empresa si la URL no tiene host) a `data`, renderiza y envía con el proveedor de `internal/email`. Con `async` el envío va en una goroutine y los errores solo se registran en el log.
- Plantillas incluidas: `accountCreation`, `passwordReset`, `login`, `passwordChanged`, `emailChanged`, `otpEnabled`, `recoveryCodesRegenerated`, `otpDisabled`, `oauthCreated`, `oauthDeleted`, `addedToServer`, `deletedServer`, `removedFromServer`, `test`.

Configuración: [Correo](../general/15-email.md).

## 21. `DiscordService` (`discord.go`)

Singleton obtenido con `GetDiscordService()`.

| Método | Webhook | Notas |
|---|---|---|
| `SendWebhook(title, desc, color, fields)` | `panel.notifications.discordWebhook` | |
| `SendWebhookToURL(url, ...)` | el indicado | URL vacía = se omite sin aviso |
| `SendEmbedToURL(url, embed)` | el indicado | Añade marca de tiempo y pie "Aether Panel" |
| `SendAlert(title, desc, fields)` | principal | Rojo |
| `SendServerOnlineAlert`, `SendServerOfflineAlert` | principal | |
| `SendResourceAlert(name, id, resource, value, threshold)` | principal | |
| `SendBackupAlert(name, id, status, ok)` | principal | |
| `SendSystemStatus(servers []ServerInfo)` | `panel.notifications.discordWebhookSystem` | Un campo por servidor; con más de 25 servidores (el límite de campos de Discord), los 24 primeros y una nota con cuántos quedan fuera; los recuentos de la descripción los cubren todos |
| `SendNodeStatus(...)` | `panel.notifications.discordWebhookNode` | Los campos vacíos o a cero se omiten |
| `SendNodeUnreachable(total, name, host, port)` | `panel.notifications.discordWebhookNode` | Informe rojo «sin respuesta» de un nodo remoto que no respondió: solo el número de servidores que tiene asignados, porque su estado y su máquina son desconocidos |
| `ValidateDiscordWebhook(url)` | — | Función del paquete |

Cada envío exige una URL real de webhook de Discord (`https://discord.com/api/webhooks/...`, también `ptb.`, `canary.` y `discordapp.com`) y pasa por `utils.NewRestrictedHTTPClient`, que rechaza direcciones privadas. Los textos de los embeds salen del catálogo de notificaciones, en el idioma de `panel.settings.notificationLanguage` (ver [Textos de las notificaciones](#textos-de-las-notificaciones-notification_textgo)).

`POST /api/settings/test/discord` (`internal/web/api/settings.go`) envía un mensaje de prueba a cada webhook configurado e indica a cuáles no llegó: `422 ErrDiscordTestFailed` con `metadata.webhooks`, que enumera `alerts`, `reports`, `status` o `transfers` (el cliente web muestra los nombres de sus propias secciones); sin ningún webhook configurado responde `400 ErrDiscordNoWebhooks`.

El cuarto webhook, `panel.notifications.discordWebhookExTransfer`, lo usa el código de transferencias externas de `internal/web/api`. Más información: [Discord](../general/16-discord.md).

## 22. `LicenseService` (`license.go`)

Singleton obtenido con `GetLicenseService()`; límite HTTP de 30 s.

- `VerifyLicense(key)`: `GET https://prueba.skyhostingcloud.com/api/public/licenses/verify?licenseKey=...`, con la clave codificada para URL.
- `BindLicense(key, serverID, serverIP)`: `POST .../verify` con `{licenseKey, serverId, serverIp}`.
- `GetLicenseType(resp)`: `free` (no válida o plan `personal`), `pro` (`professional` o plan desconocido), `enterprise`.
- `ExtractPermissions(resp)`: solo `hasPlugins` (planes `professional`/`enterprise`).

La URL base es una constante del código y apunta a un host de pruebas (`prueba.skyhostingcloud.com`); no se puede configurar. Más información: [Licencia](../general/14-license.md).

## 23. `Cloudflare` (`cloudflare.go`)

`NewCloudflare(token, zoneID)` crea un cliente para una zona (`https://api.cloudflare.com/client/v4`, límite de 15 s). Métodos: `Zone`, `ListRecords(name)`, `CreateRecord`, `UpdateRecord(id, ...)`, `DeleteRecord(id)` (un registro que ya no existe cuenta como borrado) y `Verify(domain)` (lee la zona, comprueba que el dominio pertenece a ella y crea y borra un TXT `_aether-panel-test.<dominio>`). El token solo se envía como cabecera Bearer y se elimina de los mensajes de error; los ids de zona y de registro deben tener 32 caracteres hexadecimales. Los errores son códigos de error del panel que traduce el cliente web: `ErrCloudflareNotConfigured`, `ErrCloudflareZoneID`, `ErrCloudflareRecordID`, `ErrCloudflareDomainNotInZone`, `ErrCloudflareUnreachable`, `ErrCloudflareBadAnswer`, y los rechazos de la propia Cloudflare como `ErrCloudflareRefused` o `ErrCloudflareRefusedCode`, con su mensaje (y su código de error) en los metadatos. Lo usa `internal/web/api/server_dns.go` para los subdominios de servidor ([Servidores: subdominio](./04-servers.md#subdominio-cloudflare), [Subdominios](../features/subdomains.md)).

---

## Módulos auxiliares

### `AttemptLimiter` (`login_attempts.go`)

Contador en memoria con ventana deslizante, por cuenta y no por IP (funciona igual tras un proxy inverso); se reinicia con el proceso. `Take(key)` comprueba y cuenta de forma atómica; también hay `Allowed`, `Record` y `Reset`. Las claves caducadas se purgan cuando se superan las 10.000.

| Instancia | Límite | Lo usa |
|---|---|---|
| `LoginAttempts` | 10 cada 15 min | Inicio de sesión del panel, códigos 2FA, grant de contraseña de OAuth2, contraseñas SFTP; claves SSH (nodo propio y `/auth/sftp-key`) con una clave `sshkey:` aparte |
| `PasswordResetRequests` | 5 por hora | `internal/web/auth/password_reset.go` |

### `SSOLinks` (`sso.go`)

Enlaces de inicio de sesión de un solo uso para sistemas de facturación (WHMCS, Paymenter): `Create(userID, redirect)` devuelve un token aleatorio de 32 bytes (solo se guarda su SHA-256, en memoria); `Consume(token)` funciona una vez y dentro de `SSOLinkLifetime` (1 minuto). Los crea la API de aprovisionamiento (`internal/web/api/provision.go`) y los consume `internal/web/auth/sso.go`. Funcionalidad: [Aprovisionamiento y SSO](../features/provisioning-and-sso.md).

### Canales de notificación (`channels.go`)

- `ValidateTelegram(token, chat)`, `SendTelegram(token, chat, text)` (modo HTML), `TelegramHTML(text)`.
- `ValidateWebhook(url)` (solo https, sin credenciales en la URL) y `SendWebhookJSON(url, secret, payload)`: con secreto, el cuerpo se firma en `X-Aether-Signature: sha256=<HMAC-SHA256 en hex>`; `User-Agent: AetherPanel-Webhook/1`.
- Ambos usan el cliente HTTP restringido y un límite de 10 s; los mensajes de error nunca incluyen la URL (una URL de Telegram contiene el token del bot).

Los usan las notificaciones por servidor ([Notificaciones de servidor](../features/server-notifications.md)) y las extensiones ([Extensiones](../reference/15-extensions.md)).

### Textos de las notificaciones (`notification_text.go`)

Todos los textos que el panel envía fuera de sus propias páginas están en un único catálogo, `notificationTexts`, con una versión en español (`es`) y otra en inglés (`en`) de cada clave; una prueba comprueba que ambas tienen las mismas claves y los mismos verbos de formato en el mismo orden. Abarca las alertas e informes de Discord del panel (caídas, apagado/encendido, recursos, copias de seguridad, informes del sistema y de nodos, la prueba de Discord), las alertas de Discord, Telegram y webhook JSON de cada dueño y su prueba, el título y el mensaje de los eventos de las extensiones, y los informes y textos de los pasos de las transferencias externas.

| Función | Descripción |
|---|---|
| `NotificationLanguage()` | El idioma de este proceso: `panel.settings.notificationLanguage` si está definido en su propio archivo de configuración o entorno (`IsSet`) y el catálogo lo conoce; si no, el idioma que envió el panel por última vez (`PanelNotificationLanguage()`); si no, el valor de la clave; si no, `es` (`DefaultNotificationLanguage`) |
| `SetPanelNotificationLanguage(lang)` / `PanelNotificationLanguage()` | Guardan / leen el idioma que envió el panel en la cabecera `X-Aether-Notification-Language` (`NotificationLanguageHeader`) de `GET /daemon/servers/status`. Se guarda en memoria (`atomic.Value`); los valores vacíos o desconocidos se ignoran, así que un panel antiguo que no envía ninguno no cambia nada |
| `NotificationText(key, args...)` | El texto en ese idioma, completado con `fmt` |
| `NotificationTextIn(lang, key, args...)` | Lo mismo en un idioma dado: una clave que falta en él usa la inglesa, y una clave que no existe en ninguno se devuelve tal cual |
| `IsNotificationLanguage(lang)` | Lo usa la API de ajustes para rechazar otros valores (`ErrNotificationLanguage`) |

El panel envía `NotificationLanguage()` con cada sondeo de estado de un nodo remoto (`statusPollHeaders` en `internal/servers/remote_status.go`), y `setSetting` / `setSettings` lanzan una ronda de sondeo al momento (`servers.PollRemoteNodesNow`) cuando un guardado lo cambia, así que un nodo remoto sigue al panel en segundos, salvo que su propia configuración defina la clave. En el JSON del webhook de una alerta de recursos, el campo `resource` mantiene `CPU` o `Memoria` sea cual sea el idioma, por compatibilidad. La API de estado de las transferencias externas y el motivo que se envía al panel de origen usan siempre el texto en inglés, y el cliente web traduce él mismo los códigos de los pasos.

### `ReadDomainStatus` (`domain_status.go`)

Lee `domain-status.json`, junto al `config.json` del panel, que escribe el servicio de dominio del instalador (`tools/install/install.sh --apply-panel-url`). `State` vale `working`, `ok`, `error` o `none` (el archivo no existe). Se expone en `GET /api/settings/domain-status` (scope `settings.edit`) y se usa para la dirección del nodo local.
