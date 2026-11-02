# Paquetes restantes

Referencia de los paquetes Go que no tienen una página de backend propia: los tipos compartidos de `pkg/skypanel`, los paquetes `files` y `conditions` de la raíz del repositorio y los paquetes pequeños de `internal/` (consultas a juegos, correo, extensiones, copias externas, autoactualización, logs, utilidades HTTP y utilidades generales).

---

## Dónde se documenta cada paquete

| Paquete | Página |
|---|---|
| `cmd/panel` | [CLI](./07-cli.md) |
| `internal/config` | [Configuración](./02-configuration.md) |
| `internal/database`, `internal/models` | [Base de datos](./03-database.md) |
| `internal/servers`, `internal/servers/docker`, `internal/servers/tty` | [Servidores](./04-servers.md), [Gestión de procesos y métricas](./13-process-metrics.md) |
| `internal/web` (`api`, `auth`, `daemon`, `oauth2`, `swagger`), `internal/middleware`, `internal/scopes`, `internal/oauth2` | [Capa de API](./05-api-layer.md) |
| `internal/operations/*` | [Operaciones](./06-operations.md) |
| Trackers de WebSocket (`pkg/skypanel/tracker.go`) | [WebSocket](./08-websocket.md) |
| `internal/sftp`, JWT, claves de API | [Seguridad](./09-security.md) |
| `internal/services` | [Servicios](./11-services.md) |
| `internal/connections` | [Entrada de consola: RCON, Telnet, RCON-WS](./12-rcon-telnet.md) |
| `files`, `conditions`, `pkg/skypanel`, `internal/query`, `internal/email`, `internal/extensions`, `internal/offsite`, `internal/update`, `internal/logging`, `internal/response`, `internal/utils`, `internal/sys`, `internal/groups`, `internal/systemd` | Esta página |

Cosas que **no** son paquetes independientes, aunque a veces se describan así:

| Funcionalidad | Dónde está el código |
|---|---|
| Asistente de IA | `internal/web/api/ai.go`, `ai_chat.go` (véase [Asistente de IA](../features/ai-assistant.md)) |
| Notificaciones por servidor | `internal/servers/notifications.go` e `internal/services/channels.go` (véase [Notificaciones de servidor](../features/server-notifications.md)) |
| Uptime | `internal/services/uptime.go` (véase [Servicios](./11-services.md#16-uptime-uptimego)) |
| Repositorios de plantillas | `internal/services/templates.go` (véase [Servicios](./11-services.md#11-template-templatesgo)) |
| Carpetas de volúmenes Docker | `storage/` en la raíz del repositorio, no `internal/storage` (véase [más abajo](#storage--carpetas-de-volúmenes-docker)) |

---

## `files/` — Acceso a archivos y archivos comprimidos

Acceso seguro a la carpeta de un servidor y manejo de archivos comprimidos. Comportamiento detallado (límites, enlaces simbólicos, formatos): [Archivos y compresión](../general/19-files-compression.md).

### `FileServer` (`filesystem.go`)

```go
type FileServer interface {
    fs.FS
    fs.ReadDirFS
    fs.StatFS
    Prefix() string
    Stat(name string) (fs.FileInfo, error)
    Mkdir(path string, mode os.FileMode) error
    MkdirAll(path string, mode os.FileMode) error
    OpenFile(path string, flags int, mode os.FileMode) (*os.File, error)
    Remove(path string) error
    Rename(source, target string) error
    RemoveAll(path string) error
    Glob(pattern string) ([]string, error)
    Symlink(oldname, newname string) error
    Close() error
}
```

`NewFileServer(prefix, uid, gid)` devuelve uno encerrado en `prefix`. Todas las rutas deben ser locales (`filepath.IsLocal`). Los archivos se abren con `openat2(RESOLVE_BENEATH)` cuando el kernel lo admite; si no, se recorre cada directorio con `openat(O_NOFOLLOW|O_PATH)`. Las aperturas añaden `O_NONBLOCK` para que una tubería con nombre no bloquee la petición. Los archivos nuevos se asignan a `uid:gid` cuando `uid != -1` (sin garantía).

**El soporte de openat2** se decide al arrancar con `utils.DetermineKernelSupport()`: prueba la llamada al sistema como la usa el servidor de archivos (abre `/` con `RESOLVE_BENEATH`). Si funciona, `openat2` se activa; `ENOSYS` (kernel sin ella), `EPERM` (un filtro seccomp, como un perfil de Docker antiguo, la bloquea) o `EOPNOTSUPP` la desactivan; cualquier otro error recurre a buscar `do_sys_openat2` en `/proc/kallsyms`. Si queda desactivada, el daemon **se niega a arrancar** ("Cowardly not starting"), salvo que `security.forceOpenat` sea true, lo que omite la comprobación y fuerza la alternativa con `openat`.

### Escritura en la carpeta de un servidor (`files.go`)

| Función | Descripción |
|---|---|
| `CreateIn(root, dest)` | Crea o vacía `dest` dentro de `root` mediante un `FileServer`, sin seguir nunca un enlace simbólico en el destino y creando las carpetas que falten |
| `CopyFile(src, root, dest)` | Copia un archivo del panel (caché) a la carpeta del servidor |
| `WriteFile(src io.Reader, root, dest)` | Escribe un reader en la carpeta del servidor |

### Archivos comprimidos (`compression.go`)

Basado en `github.com/mholt/archives`.

| Función | Descripción |
|---|---|
| `Extract(fs, source, target, filter, skipRoot, forcedType)` | Extrae un archivo comprimido del file server |
| `ExtractLimited(fs, source, target, filter, skipRoot, limit)` | Igual, con un `WriteLimit` (para aplicar el límite de disco) |
| `ExtractReader(fs, reader, source, target, filter, skipRoot)` | Desde un `io.ReadSeeker` hacia un file server |
| `ExtractFromReader(reader, source, target, filter, skipRoot, forcedType)` | Desde un reader a una carpeta normal |
| `DetermineIfSingleRoot(ctx, source, reader)` | Si todo cuelga de una única carpeta superior |
| `CheckArchive(reader, source, skipRoot)` | Valida las entradas antes de extraer |
| `Compress(fs, target, files)` / `CompressLimited(...)` | Crea un archivo comprimido (formato según la extensión) |
| `CompressServer(fs, target)` | Comprime la carpeta completa de un servidor (copias de seguridad) |
| `Readlink(fs, name)` | Lee un enlace dentro de la raíz |

Se rechazan las entradas que acabarían fuera del destino (`ErrPathTraversal`); los enlaces simbólicos solo se recrean si se quedan dentro de la raíz.

### `MergedFS` (`mergedfs.go`)

`NewMergedFS(systems ...fs.FS)` superpone varios `fs.FS`: `Open` y `ReadFile` devuelven la primera coincidencia y `ReadDir` combina las entradas por nombre. Se usa para superponer `panel.email.templateFolder` a las plantillas de correo embebidas.

---

## `conditions/` — Expresiones CEL

Evalúa expresiones [CEL](https://github.com/google/cel-go) para las condiciones `if` de plantillas y operaciones. Guía completa: [Condiciones (CEL)](../general/18-conditions-cel.md) y [Operaciones](./06-operations.md).

| Función | Descripción |
|---|---|
| `ResolveIf(condition, data, extras)` | Sustituye los marcadores `${nombre}` con `SubstituteVariables` y después ejecuta la expresión, que debe devolver `bool` |
| `Run[T](statement, data, extras)` | Ejecuta cualquier expresión y convierte el resultado a `T` |
| `SubstituteVariables(condition, data)` | Sustituye cada `${nombre}` cuyo nombre esté en `data` o sea `os`/`arch`: escapado dentro de los literales de texto (un literal raw se cierra y se une con un texto escapado), y por el nombre de la variable fuera de ellos (un valor entre comillas si el nombre no es un identificador CEL válido). Los nombres desconocidos y los literales de bytes se dejan como están (`variables.go`) |

- Cada clave de `data` se convierte en una variable CEL (tipo `any`), además de las constantes `os` (`runtime.GOOS`) y `arch` (`runtime.GOARCH`).
- Una expresión vacía pasa a ser `success` si `data` tiene la clave `success`.
- Constantes de nombres de variable (`consts.go`): `os`, `arch`, `success`, `env`, `serverId`. Cuáles están presentes depende del mapa de datos de quien llama.

---

## `pkg/skypanel/` — Tipos compartidos

Los tipos de dominio que comparten el panel, el daemon y las operaciones.

### Interfaces

| Interfaz | Archivo | Métodos |
|---|---|---|
| `EnvironmentImpl` | `environment.go` | `ExecuteAsyncImpl`, `KillImpl`, `GetStatsImpl`, `SendCodeImpl`, `GetUIDImpl`, `GetGidImpl`, `IsRunningImpl` |
| `StartCanceler` | `environment.go` | `CancelStartImpl()`: detiene un arranque cuya imagen aún se está descargando |
| `Reattacher` | `environment.go` | `ReattachImpl(env, steps)`: vuelve a conectar con un proceso de antes de reiniciar el daemon (solo Docker) |
| `EnvironmentFactory` | `environmentfactory.go` | `Create() EnvironmentImpl`, `Key() string` |
| `Operation` / `OperationFactory` | `operation.go` | `Run(RunOperatorArgs) OperationResult`; `Create(CreateOperation)`, `Key()` |
| `Console` | `console.go` | `io.WriteCloser` + `Start()` ([Entrada de consola](./12-rcon-telnet.md)) |
| `DaemonServer` | `server.go` | `GetFileServer()`, `Extract()`, `ArchiveItems()`, `DataToMap()` |
| `SFTPAuthorization` / `SFTPKeyAuthorization` | `authorization.go` | `Validate(user, password)`; `ValidateKey(user, key)` |

`CreateOperation.Arg(names...)` devuelve el primer argumento que haya definido la plantilla entre varias grafías (`appId` / `appID`).

### Structs principales

| Tipo | Archivo | Función |
|---|---|---|
| `Server`, `Execution`, `Command`, `Group`, `KeepAlive` | `server.go` | Definición del servidor (claves JSON `data`, `install`, `uninstall`, `run`, `environment`, ...) |
| `Environment`, `ExecutionData` | `environment.go` | Estado en ejecución de un servidor y datos de un arranque |
| `Variable`, `VariableOption` | `variable.go` | Variable de plantilla; `value` se convierte al tipo declarado al cargarla |
| `MetadataType`, `ConditionalMetadataType` | `typewithmetadata.go` | Objetos `{type, ...}` (operaciones, entornos) con un `if` opcional |
| `Requirements` | `requirements.go` | `os`, `arch` y `binaries` que necesita una plantilla; `Test(server)` los comprueba |
| `Task` | `task.go` | Tarea programada: `name`, `cronSchedule`, `description`, `operations` |
| `ServerStats`, `ServerLogs`, `ServerRunning`, `ErrorResponse`, ... | `httpmodels.go` | Cargas de la API ([métricas](./13-process-metrics.md#estadísticas-en-vivo)) |
| `Transmission`, `ServerCrash` | `message.go` | Mensajes de WebSocket; tipos `console`, `stat`, `status`, `crash` |
| `FileList`, `FileDesc` | `filelist.go` | Listados del gestor de archivos |
| `StdinConsoleConfiguration` | `stdin.go` | Ajustes de la entrada de consola |

### Búfer de consola (`cache.go`)

`MemoryCache` guarda en memoria la salida reciente de la consola de un servidor. `CreateCache()` toma el tamaño de `daemon.console.buffer` (KB, por defecto `50`; un valor ≤ 0 también equivale a 50). `Write` añade y expulsa los mensajes más antiguos cuando se supera la capacidad; `Read()` devuelve todo junto con la última marca de tiempo; `ReadFrom(t)` devuelve lo escrito después de `t` (lo usa `GET /api/servers/:id/console?time=`).

### Errores (`errors.go`)

```go
type Error struct {
    Message string                 `json:"msg,omitempty"`
    Code    string                 `json:"code,omitempty"`
    Meta    map[string]interface{} `json:"metadata,omitempty"`
}
type ErrorResponse struct { Error *Error `json:"error"` }
```

- Un centenar de errores predefinidos creados con `CreateError(msg, code)`; el código es el nombre de la variable (`ErrServerOffline`, `ErrTooManyAttempts`, `ErrConsoleDetached`, `ErrDiskLimitExceeded`, `ErrNodeDiskFull`, ...).
- Los parametrizados son funciones: `ErrSettingNotConfigured(name)`, `ErrNoTemplate(name)`, `ErrServiceInvalidProvider(service, provider)`, `ErrFieldRequired`, `ErrFieldLength(field, min, max)`, `ErrUnsupportedOS`, `ErrMissingBinary`, `ErrCurseForgeStatus`, entre otros. Los mensajes usan marcadores `${clave}` que se rellenan desde `Meta`.
- `CreateErrMissingScope(scope)`, `FromError(err)` (envuelve un error normal con el código `ErrGeneric`), `GenerateValidationMessage(err)` (errores del validador) y `Recover()` (registra un panic con su traza en lugar de tumbar el proceso).

### Descargas y HTTP (`download.go`, `client.go`)

| Función | Descripción |
|---|---|
| `DownloadFile(url, fileName, env)` | Descarga en la carpeta del servidor mediante `files.CreateIn` (nunca a través de un enlace simbólico) |
| `DownloadFileToCache(url, fileName)` | Descarga en una ruta del panel |
| `Download(url, hash, algorithm, cache, env)` | Reader con comprobación de hash opcional y caché en `daemon.data.cache`, en `<cache>/<dirección sin http:// ni https://>` (`cache/maven.example.com/ruta/archivo.jar`); una dirección que caería fuera de la carpeta de caché (con un `..`) se descarga sin caché |
| `DownloadHash(hashURL, algorithm)` | Obtiene una suma de verificación publicada |
| `DownloadViaMaven(url, env)` | Descarga de un artefacto Maven |
| `HTTP()`, `HTTPGet(url)`, `HTTPExtract(url, dir, type)` | Cliente compartido, GET y descarga con extracción |

Las descargas cuya URL procede de una plantilla usan `utils.NewDownloadHTTPClient` (véase [utils](#internalutils--utilidades-generales)).

### Otros

- `Engine` (`engine.go`): el `*gin.Engine` global, que usa `services.serveLocal` para ejecutar las llamadas al nodo dentro del proceso.
- Versión (`version.go`): `Version` (por defecto `nightly`) y `Hash` (por defecto `unknown`), definidos con `-ldflags`; `Display` = `"SkyPanel <versión> (<hash>)"`.

---

## `internal/query/` — Consultas a juegos

Pregunta a los servidores de juego en marcha por sus jugadores. Lo usa `Server.QueryGame()` (`internal/servers/players.go`), que cachea las respuestas 10 segundos y alimenta las tarjetas de servidor y el historial de jugadores.

| Función | Protocolo | Resultado |
|---|---|---|
| `Minecraft(ip, port)` | Ping de lista de servidores 1.7+ (`github.com/dreamscached/minequery/v2`, `Ping17`) | `MinecraftResponse{numPlayers, maxPlayers, version, players}` |
| `Source(ip, port)` | A2S de Steam por UDP (A2S_INFO y después A2S_PLAYER), con soporte de challenge, límite de 1,5 s, solo respuestas de un paquete | `SourceResponse{name, map, game, numPlayers, maxPlayers, bots, players}` |

**Dirección** (`host.go`, exportada como `HostOrLocal` también para las consolas RCON, Telnet y RCON sobre WebSocket): una IP vacía, `0.0.0.0` o `::` significa "esta máquina". En una instalación normal es `127.0.0.1`. Cuando el daemon corre dentro de un contenedor Docker (existe `/.dockerenv`), usa la puerta de enlace por defecto del contenedor según `/proc/net/route`, que llega a los puertos que los contenedores de juego publican en el host.

El protocolo depende del `query.type` de la plantilla: `minecraft`, `source`/`steam` o vacío (se prueba A2S y, tras un fallo, no se reintenta durante 2 minutos). El puerto es la variable de puerto de consulta de la plantilla (`queryport`, `query_port`, `queryPort`, `steamqueryport`, `steam_query_port`) o el `port` del juego.

---

## `internal/email/` — Proveedores de correo

```go
type Provider interface {
    Send(to, subject, body string) error
}
func GetProvider(name string) Provider   // sin distinguir mayúsculas, nil si no existe
```

Los proveedores se registran en `init()`:

| Nombre | Archivo | Biblioteca | Ajustes que necesita |
|---|---|---|---|
| `smtp` | `smtp.go` | `github.com/wneessen/go-mail` | `panel.email.from`, `panel.email.host`, `username`/`password` opcionales; TLS implícito (puerto SSL), autenticación detectada automáticamente; se eliminan los espacios de la contraseña |
| `sendgrid` | `sendgrid.go` | `sendgrid-go` | `from`, `key` |
| `mailgun` | `mailgun.go` | `mailgun-go` | `domain`, `from`, `key` |
| `mailjet` | `mailjet.go` | `mailjet-apiv3-go` | `domain` (la clave pública de la API), `from`, `key` (la clave secreta) |
| `debug` | `debug.go` | — | Solo imprime el correo en el log de debug |

Un ajuste que falte devuelve `ErrSettingNotConfigured(<clave>)`. El renderizado y las plantillas están en `services.EmailService` ([Servicios](./11-services.md#20-emailservice-emailgo)); la configuración, en [Correo](../general/15-email.md).

---

## `internal/extensions/` — Extensiones del panel

Complementos de administración: un zip con `extension.json` en la raíz y páginas HTML. Las páginas se muestran en un marco aislado (sandbox); los eventos se envían al webhook de la extensión. Guía de empaquetado: [Extensiones](../reference/15-extensions.md).

| Elemento | Valor |
|---|---|
| Archivos | `<panel.extensions.folder>/<id>` (por defecto `extensions` dentro de la carpeta de datos) |
| Límites del paquete | Zip de 20 MB, 500 archivos, 50 MB descomprimido, manifiesto ≤ 64 KB |
| Manifiesto | `id` (2–40 caracteres de `a-z0-9-`), `name` (≤ 60), `version` (≤ 20), `author`, `description` (≤ 300), hasta 10 `pages` (`id`, `title` ≤ 40, `placement` `server` o `admin`, `path` a un `.html` del paquete), `webhook` (https), `events`; se rechazan los campos desconocidos |
| Eventos | `server.created`, `server.deleted`, `server.started`, `server.stopped`, `server.crashed`, `server.backup`, `server.resources` |
| Instalación | `Install(db, zip, size)`: una extensión nueva empieza **desactivada**; reinstalar un id lo actualiza y conserva su interruptor, su secreto y un webhook que haya editado el administrador |
| Entrega | `Publish(event, data)` se ejecuta en segundo plano; cuerpo = data + `event`, `extension`, `timestamp`; firmado con el secreto de la extensión (`X-Aether-Signature`, véase [canales](./11-services.md#canales-de-notificación-channelsgo)); reintentos a los 5 s y a los 30 s; como mucho 32 entregas en curso, los eventos sobrantes se descartan y se registran en el log |
| Otros | `Uninstall(db, id)`, `Reload(db)` (tras cualquier cambio), `SendTest(ext)` (un evento `test`), `ServerData(id, name)` |

`server.created` / `server.deleted` los publica la API (`internal/web/api/servers.go`, `provision.go`); el resto procede de `internal/servers/notifications.go` a través de `servers.EventSink`.

---

## `internal/offsite/` — Copias externas

Copia cada copia de seguridad terminada en un bucket compatible con S3 (AWS S3, Backblaze B2, Cloudflare R2, Wasabi, MinIO) mediante `github.com/minio/minio-go/v7`. Guía de uso: [Copias de seguridad](../features/backups.md).

| Ajuste | Por defecto | Notas |
|---|---|---|
| `daemon.backups.s3.enabled` | `false` | |
| `daemon.backups.s3.endpoint` | `""` | `host`, `host:puerto` o una URL completa (su esquema decide entonces si es https) |
| `daemon.backups.s3.region` | `""` | |
| `daemon.backups.s3.bucket` | `""` | Debe existir previamente |
| `daemon.backups.s3.accessKey` / `secretKey` | `""` | |
| `daemon.backups.s3.prefix` | `aether-backups` | Los objetos son `<prefijo>/<serverId>/<archivo>.tar.gz` |
| `daemon.backups.s3.insecure` | `false` | http sin TLS, para un MinIO en la red local |

`FromSettings()` devuelve la configuración y si está activada **y** completa (endpoint, bucket y ambas claves).

| Función | Para qué se usa |
|---|---|
| `Upload` | Tras cada copia (`Server.uploadOffsite`, límite de 2 h). La copia local se mantiene; un fallo de subida se muestra en la consola pero no hace fallar la copia. |
| `List` | Lista de copias: las entradas se marcan como `local`, `offsite` o `both` (límite de 15 s; si falla se muestra la lista local) |
| `Exists`, `Download` | Restaurar una copia que solo está en el bucket (se descarga como `<archivo>.download` y después se renombra) |
| `Open` | Descargar una copia directamente desde el bucket |
| `Delete` | Al borrar una copia también se borra el objeto |
| `Test` | `POST /api/settings/test/s3` (scope `settings.edit`): comprueba que el bucket existe, escribe un objeto pequeño bajo `<prefijo>/.aether-test/` y lo borra |

---

## `internal/update/` — Autoactualización

Ejecuta el script de actualización del host desde dentro del panel. Guía de uso: [Actualizaciones del panel](../features/panel-updates.md).

- `Trigger()`: descarga `alpine:latest` si falta, crea un contenedor con red del host y `/` montado en `/host`, y ejecuta `chroot /host bash -c "cd /opt/skypanel && chmod +x tools/panelUpdate/panelUpdate.sh && ./tools/panelUpdate/panelUpdate.sh"`. La ruta `/opt/skypanel` está fijada en el código.
- Un monitor en segundo plano consulta el contenedor cada 2 s, guarda los últimos 8.000 caracteres de su log, registra el código de salida y elimina el contenedor.
- `Status()` devuelve `{running, containerId, startedAt, finishedAt, exitCode, log, error}`; `RecordFailure(msg)` marca una actualización que no pudo arrancar.

Puntos de entrada (`internal/web/api/settings.go`, scope `settings.edit`):

| Endpoint | Descripción |
|---|---|
| `POST /api/settings/update-panel` | Lanza la actualización en el nodo del propio panel y llama a `POST /daemon/update` en cada nodo remoto; responde con un resultado por nodo |
| `GET /api/settings/update-status` | `Status()` de la actualización local |
| `GET /api/settings/update-check` | Compara el commit desplegado con el último commit de la rama desde la que se instaló el panel (`main` si no se puede leer) |

En cada nodo, el daemon expone `POST /daemon/update` y `GET /daemon/update-status` (JWT del panel).

---

## `internal/logging/` — Logs

```go
var Error  = log.New(os.Stderr, "[ERROR] ", log.LstdFlags)
var Debug  = log.New(os.Stdout, "[DEBUG] ", log.LstdFlags)
var Info   = log.New(os.Stdout, "[INFO] ", log.LstdFlags)
var Server = log.New(os.Stdout, "[SERVER] ", log.LstdFlags)
var OriginalStdOut, OriginalStdErr = os.Stdout, os.Stderr
```

- `Initialize(useFiles)`: con archivos, crea la carpeta `logs` (por defecto `logs`) y un `Rotator`; vuelve a crear `Error`, `Info` y `Debug` para que escriban a la vez en el archivo y en la consola; redirige `log`, `os.Stdout` y `os.Stderr` mediante tuberías hacia `Info` / `Error`. `Server` no se redirige al archivo.
- `Close()` cierra el archivo de log.

**Rotator** (`rotator.go`): el día en curso va a `skypanel.log`. Cuando cambia el día, o el archivo llega a 100 MB, se renombra a `skypanel-<AAAA-MM-DD>.log` (con un sufijo libre si ya existe) y se comprime con gzip en segundo plano. Se borran los archivos modificados por última vez hace más de `logRetentionDays` días (por defecto `14`; `0` los conserva siempre); solo se tocan los archivos `skypanel-*`. Un archivo que quede de un día anterior se archiva al arrancar. `SIGUSR1` vuelve a abrir `skypanel.log` para instalaciones con logrotate externo.

**MultiWriter** (`multi.go`): como `io.MultiWriter`, pero omite los writers nil y aplana los anidados.

`daemon.console.forward` (`Environment.CreateWrapper`) copia además la consola de cada servidor en el stdout original del daemon.

---

## `internal/response/` — Utilidades HTTP

| Función | Descripción |
|---|---|
| `HandleError(c, err, status) bool` | Devuelve `false` si `err` es nil. Si no, registra el error y aborta: `gorm.ErrRecordNotFound` → 404 sin cuerpo; un estado ≥ 500 con un error que no es del panel → `{"error": ErrInternal}` (los detalles solo en el log); cualquier otro caso → `{"error": {"msg", "code", "metadata"}}` |
| `CreateOptions(methods...)` | Handler para las rutas `OPTIONS`: `Access-Control-Allow-Origin: *`, los métodos permitidos más `OPTIONS`, cabeceras permitidas `authorization, origin, content-type, accept`, estado 200 |
| `NotImplemented(c)` | 501 con `ErrNotImplemented` |

---

## `internal/utils/` — Utilidades generales

### Peticiones salientes

| Función | Descripción |
|---|---|
| `NewRestrictedHTTPClient()` | Rechaza direcciones privadas, loopback, link-local, CGNAT, de pruebas de rendimiento y no especificadas **al conectar** (después del DNS, así que cubre el DNS rebinding); límite de conexión de 10 s. Se usa para Discord, Telegram y los webhooks JSON. |
| `ValidateExternalURL(url)` | Solo http/https, sin `localhost`, todas las IP resueltas públicas |
| `NewDownloadHTTPClient(onPrivate)` | Para descargas de plantillas: siempre rechaza loopback, link-local, multicast y no especificadas; las direcciones de red local privadas se permiten salvo que `security.blockPrivateDownloads` (por defecto `true`) esté activo |
| `DownloadFile(url, onPrivate, save)` / `DownloadWith(client, url, save)` | Descarga con grab en una carpeta temporal bajo `daemon.data.cache` y entrega el archivo a `save` (nunca se escribe directamente en la carpeta de un servidor). `DownloadFile` usa `NewDownloadHTTPClient` (lo usan `download`, `scraperdl` y `githubdl`); `DownloadWith(nil, ...)` usa el cliente predeterminado de grab, que no tiene protección de direcciones |

### Plantillas y argumentos

| Función | Descripción |
|---|---|
| `ReplaceTokens(msg, mapping)` | Sustituye `${clave}` por `fmt.Sprint(valor)` |
| `ReplaceTokensInArr`, `ReplaceTokensInMap` | Lo mismo para slices y mapas |
| `SplitArguments(source)` / `MergeArguments(args)` | Divide una línea de comandos respetando las comillas / la vuelve a unir |

### Utilidades de datos

| Función | Descripción |
|---|---|
| `GetStringOrDefault`, `GetBooleanOrDefault`, `GetMapOrNull`, `GetObjectArrayOrNull`, `GetStringArrayOrNull` | Lecturas tipadas de `map[string]interface{}` |
| `Convert(val, target)` | Convierte `val` al tipo de `target` |
| `ToString(v)`, `UnmarshalTo(source, target)` | Conversión a cadena; copia mediante un viaje de ida y vuelta por JSON (`target` debe ser un puntero) |
| `Intersection[T](a, b)` | Elementos de `a` que también están en `b`, en el orden de `a` |
| `Remove[T](a, b)` | `a` sin las apariciones de `b` |
| `GenerateRandomString(n)` | `n` bytes aleatorios en base64 URL |
| `CompareWildcard(source, match)`, `WildCardToRegexp(pattern)` | Coincidencia con comodín `*` (vacío o `*` coincide con todo; lo usa el filtro de extracción de archivos comprimidos) |
| `GetDirSize(path)` | Tamaño recursivo de una carpeta |
| `Close(closer)`, `CloseResponse(resp)` | Cierre que ignora errores y panics |

### Sistema

| Función | Descripción |
|---|---|
| `DetermineKernelSupport()`, `UseOpenat2()` | Detección de openat2, véase [files](#files--acceso-a-archivos-y-archivos-comprimidos) |
| `ParseJCMDResponse(data) *JvmStats` | Interpreta la salida de `jcmd` en uso de heap y metaspace |

---

## `internal/sys/` — Utilidades de syscalls

`SyscallMode(os.FileMode) uint32` convierte los bits de permisos, más setuid, setgid y sticky, en el modo crudo que usan `openat2`, `openat` y `mkdirat` en `files/filesystem.go`.

---

## `internal/groups/` — Comprobación de grupo

`IsUserIn(groups...) bool` indica si el usuario actual del sistema pertenece a alguno de los grupos o a `root`. `SkyPanelGroup = "SkyPanel"`. Solo lo usa `skypanel user edit` (`cmd/panel/user.go`); la misma comprobación en `main.go` está comentada.

---

## `internal/systemd/` — Archivos de empaquetado nativo

No es un paquete Go y el instalador de Docker no lo usa:

| Archivo | Contenido |
|---|---|
| `servicefiles/skypanel.service` | Unidad `Type=notify` que ejecuta `/usr/sbin/SkyPanel runService --config /etc/SkyPanel/config.json` con el usuario/grupo `SkyPanel` |
| `servicefiles/apparmor.conf` | Perfil AppArmor sin restricciones para `/usr/sbin/SkyPanel` |
| `debian/templates` | Plantillas de debconf |

`cmd/panel/runservice.go` avisa a systemd cuando existe `NOTIFY_SOCKET`.

---

## `storage/` — Carpetas de volúmenes Docker

Está en la raíz del repositorio (no existe `internal/storage`). `docker-compose.yml` monta:

| Carpeta | Montada en | Servicio |
|---|---|---|
| `storage/mysql-data` | `/var/lib/mysql` | MariaDB |
| `storage/skypanel-config` | `/etc/SkyPanel` | Configuración del panel |
| `storage/skypanel-data` | `/var/lib/SkyPanel` | Servidores, copias, caché, archivos de historial |
| `storage/skypanel-logs` | `/var/log/SkyPanel` | Logs del panel |

Véase [Docker](../docker/README.docker.md).
