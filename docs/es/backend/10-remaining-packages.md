# Paquetes de Utilidades

Documentación de los paquetes utilitarios y transversales del backend.

---

## files/ — Sistema de Archivos y Compresión

### FileServer Interface

Define operaciones seguras de archivos con jail root (path traversal prevention via `openat2` / `openat` + `O_NOFOLLOW`):

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

Implementación concreta: `fileServer` en `filesystem.go`.

- **`NewFileServer(prefix string, uid, gid int) (FileServer, error)`** — Crea un FileServer enjaulado en `prefix`
- **`OpenFile(path, flags, mode)`** — Usa `openat2` con `RESOLVE_BENEATH` (kernel >=5.6) o fallback a `openat` + `O_NOFOLLOW`. Hace chown del archivo si `uid != -1`
- **`MkdirAll`, `Rename`, `Remove`, `RemoveAll`** — Operan con `unix.Mkdirat`, `unix.Renameat2`, `unix.Unlinkat`

### MergedFS

```go
type MergedFS struct {
    fileSystems []fs.FS
}
```

- **`NewMergedFS(systems ...fs.FS) *MergedFS`** — Fusiona múltiples `fs.FS` en uno solo
- **`Open(name)`** — Devuelve el primer `fs.File` exitoso
- **`ReadDir(name)`** — Fusiona directorios, deduplicando por nombre
- **`ReadFile(name)`** — Devuelve la primera lectura exitosa

### Compresión (`compression.go`)

Usa la librería `mholt/archives` para soportar múltiples formatos (ZIP, tar.gz, tar.bz2, etc.).

```go
type ExtractOptions struct {
    FileServer   FileServer
    SourceFile   string
    TargetPath   string
    Filter       string       // glob filter
    SkipRoot     bool         // skip single root directory
    ForcedWalker archives.Extractor
}
```

| Función | Descripción |
|---|---|
| `Extract(fs, sourceFile, targetPath, filter, skipRoot, forcedType)` | Extrae un archivo del FileServer |
| `ExtractFromReader(reader, sourceFile, targetPath, filter, skipRoot, forcedType)` | Extrae desde un `io.ReadSeeker` arbitrario |
| `DetermineIfSingleRoot(ctx, sourceFile, reader)` | Detecta si un archivo tiene un solo directorio raíz |
| `Compress(fs, targetFile, filesToCompress)` | Comprime archivos/directorios, detecta formato por extensión |
| `safeJoin(base, path string)` | Previene path traversal verificando que la ruta resuelta esté bajo base |

### CopyFile / WriteFile (`files.go`)

```go
func CopyFile(src, dest string) error     // copia archivo creando directorios padre
func WriteFile(src io.Reader, dest string) error  // escribe reader a archivo
```

---

## conditions/ — Evaluación de Expresiones CEL

Motor de evaluación de **CEL** (Common Expression Language) de `google/cel-go`. Se usa para condiciones en operaciones de instalación, plantillas, y cualquier lugar con sintaxis `{{ expression }}`.

### Constantes Globales

```go
var GlobalConstantValues = map[string]interface{}{
    "os":   runtime.GOOS,
    "arch": runtime.GOARCH,
}
```

### Funciones

| Función | Descripción |
|---|---|
| `ResolveIf(condition, data, extras)` | Evalúa condición CEL que debe devolver `bool`. Si condición vacía, usa `data["success"]` |
| `Run[T](statement, data, extras)` | Evalúa cualquier expresión CEL y type-assert a `T` |
| `ReplaceInString(str, data, extras)` | Reemplaza `{{ expression }}` en string con el resultado de evaluar cada expresión |

### Variables Disponibles en Expresiones

| Variable | Descripción |
|---|---|
| `os` | `runtime.GOOS` |
| `arch` | `runtime.GOARCH` |
| `success` | Resultado de operación anterior |
| `env` | Tipo de entorno (docker/tty) |
| `serverId` | ID del servidor |
| Cualquier variable del servidor | Definidas en `Server.Variables` |

---

## pkg/skypanel/ — Tipos Compartidos

Paquete central de tipos del dominio. Todos los demás paquetes dependen de él.

### Interfaces Clave

| Interfaz | Archivo | Métodos |
|---|---|---|
| `EnvironmentImpl` | `environment.go` | `ExecuteAsyncImpl`, `KillImpl`, `GetStatsImpl`, `SendCodeImpl`, `GetUIDImpl`, `GetGidImpl`, `IsRunningImpl` |
| `EnvironmentFactory` | `environmentfactory.go` | `Create() EnvironmentImpl`, `Key() string` |
| `Operation` | `operation.go` | `Run(args RunOperatorArgs) OperationResult` |
| `OperationFactory` | `operation.go` | `Create(CreateOperation) (Operation, error)`, `Key() string` |
| `Console` | `console.go` | `io.WriteCloser`, `Start()` |
| `DaemonServer` | `server.go` | `GetFileServer()`, `Extract()`, `ArchiveItems()`, `DataToMap()` |
| `SFTPAuthorization` | `authorization.go` | `Validate(username, password) (*ssh.Permissions, error)` |

### Trackers (Sistema Pub/Sub para WebSocket)

```go
type Tracker struct {
    sockets []*Socket
    locker  sync.Mutex
}
```

| Función | Descripción |
|---|---|
| `CreateTracker()` | Crea tracker vacío |
| `Register(conn *Socket)` | Agrega socket al tracker |
| `WriteMessage(msg Transmission)` | Envía mensaje JSON a todos los sockets |
| `Write(source []byte)` | Envía bytes a todos los sockets |

```go
type Socket struct {
    conn   *websocket.Conn
    locker sync.Mutex
    io.WriteCloser
}
```

| Función | Descripción |
|---|---|
| `Create(ws)` | Crea socket desde conexión WebSocket |
| `WriteMessage(msg Transmission)` | Envía mensaje tipado |
| `Write(data []byte)` | Envía datos raw |
| `WriteJSON(data interface{})` | Envía JSON |
| `Close()` | Cierra conexión |

### MemoryCache (Buffer Circular de Consola)

```go
type MemoryCache struct {
    Buffer   []cacheMessage
    Capacity int    // capacidad en KB (config: daemon.console.buffer)
    Size     int
    Lock     sync.RWMutex
}
```

| Método | Descripción |
|---|---|
| `CreateCache()` | Crea con capacidad de `daemon.console.buffer` KB |
| `Read()` | Devuelve todo el buffer |
| `ReadFrom(startTime)` | Devuelve datos desde timestamp |
| `Write(b)` | Agrega datos, evicta los más viejos si excede capacidad |

### Errores Compartidos

Error tipado con código y metadata:

```go
type Error struct {
    Message string
    Code    string
    Meta    map[string]interface{}
}
```

| Función | Descripción |
|---|---|
| `CreateError(msg, code)` | Crea error tipado |
| `FromError(err)` | Convierte cualquier error a `*Error` |

Errores predefinidos (selección):
- `ErrInvalidCredentials`, `ErrServerOffline`, `ErrNoPermission`
- `ErrIllegalFileAccess`, `ErrBackupInProgress`, `ErrDockerNotSupported`
- `ErrDatabaseNotAvailable`, `ErrNotImplemented`, `ErrNoTemplate`, `ErrNodeInvalid`
- `ErrInvalidSession`, `ErrSessionExpired`, `ErrUserNotFound`, `ErrServerNotFound`

Fábricas parametrizadas:
- `ErrSettingNotConfigured(name)`, `ErrNoTemplate(template)`, `ErrServiceInvalidProvider(service, provider)`
- `ErrFieldTooLarge(field, value)`, `ErrFieldTooSmall(field, value)`, `ErrFieldNotBetween(field, min, max)`
- `ErrFieldEqual(f1, f2)`, `ErrFieldNotEqual(f1, f2)`, `ErrFieldNotEmail(field)`, `ErrFieldLength(field, min, max)`
- `ErrFactoryError(op, err)`, `ErrUnsupportedOS(actual, expected)`, `ErrUnsupportedArch(actual, expected)`
- `ErrMissingBinary(binary)`, `ErrPathNotAbs(path)`, `ErrCurseForgeDistribution(projectId)`, `ErrCurseForgeFile(projectId, fileID)`

### Descargas

```go
func DownloadFile(url, fileName string, env *Environment) error
func DownloadFileToCache(url, fileName string) error
func Download(downloadURL, hash, algorithm, cache, env) (io.ReadCloser, error)
func DownloadHash(hashURL, algorithm) (string, error)
func DownloadViaMaven(downloadURL, env) (io.ReadCloser, error)
func HTTP() *http.Client
func HTTPGet(requestURL) (*http.Response, error)
func HTTPExtract(requestURL, directory, archiveType) error
```

### FileList

```go
type FileList struct {
    CurrentPath string     `json:"path"`
    Error       string     `json:"error,omitempty"`
    URL         string     `json:"url,omitempty"`
    FileList    []FileDesc `json:"files,omitempty"`
    Contents    []byte     `json:"contents,omitempty"`
    Filename    string     `json:"name,omitempty"`
}

type FileDesc struct {
    Name      string `json:"name"`
    Modified  int64  `json:"modifyTime,omitempty"`
    Size      int64  `json:"size,omitempty"`
    File      bool   `json:"isFile"`
    Extension string `json:"extension,omitempty"`
}
```

### Variable

```go
type Variable struct {
    Type
    Value        interface{}        // auto-convertido a int/bool según type
    Display      string
    Description  string
    Required     bool
    Internal     bool
    UserEditable bool
    Options      []VariableOption
}
```

### Version

```go
var Hash string      // git commit hash (ldflags)
var Version string   // versión semver (ldflags)
var Display string   // "SkyPanel v3.x.x (hash)"
```

---

## internal/connections/ — Proxis de Conexión a Consola

Tres tipos de conexión que implementan `io.WriteCloser` + `Start()` para actuar como stdin de servidores:

### RCON

```go
type RCONConnection struct {
    io.WriteCloser
    IP, Port, Password string
    Reconnect          bool
}
```

- **`Start()`** — Inicia loop de reconexión automática
- **`Write(p []byte)`** — Ejecuta comando via `gorcon/rcon`
- IP/port/password vienen de la configuración del servidor (`StdinConsoleConfiguration`)

### RCON sobre WebSocket

```go
type RCONWSConnection struct {
    io.WriteCloser
    IP, Port, Password string
    Reconnect          bool
}
```

- **`Start()`** — Conecta vía WebSocket a `ws://IP:Port/Password`
- **`Write(p []byte)`** — Envía mensaje JSON con identificador incremental

### Telnet

```go
type TelnetConnection struct {
    io.WriteCloser
    IP, Port, Password string
    Reconnect          bool
}
```

- **`Start()`** — Conecta TCP, envía password, mantiene keepalive
- **`Write(p []byte)`** — Escribe al socket TCP

Seleccionadas automáticamente por `Environment.CreateConsoleStdinProxy()` según `StdinConsoleConfiguration.Type`.

---

## internal/query/ — Consulta de Servidores Minecraft

```go
type MinecraftResponse struct {
    NumPlayers int      `json:"numPlayers"`
    MaxPlayers int      `json:"maxPlayers"`
    Version    string   `json:"version"`
    Players    []string `json:"players"`
}

func Minecraft(ip string, port int) (MinecraftResponse, error)
```

- Usa `dreamscached/minequery/v2` con protocolo `Ping17` (Minecraft 1.7+)
- Default IP: `127.0.0.1`

---

## internal/email/ — Proveedores de Email

### Interface

```go
type Provider interface {
    Send(to, subject, body string) error
}
```

### Proveedores Implementados

| Nombre | Archivo | Dependencia |
|---|---|---|
| `smtp` | `smtp.go` | `wneessen/go-mail` |
| `sendgrid` | `sendgrid.go` | `sendgrid/sendgrid-go` |
| `mailjet` | `mailjet.go` | `mailjet/mailjet-apiv3-go` |
| `mailgun` | `mailgun.go` | `mailgun/mailgun-go` |
| `debug` | `debug.go` | logging solamente |

Se registran via `init()`. Se obtienen con:

```go
func GetProvider(provider string) Provider
```

Configuración usada: `EmailProvider`, `EmailFrom`, `EmailDomain`, `EmailHost`, `EmailKey`, `EmailUsername`, `EmailPassword`.

---

## internal/logging/ — Sistema de Logging

### Loggers

```go
var Error *log.Logger   // [ERROR] tag
var Debug *log.Logger   // [DEBUG] tag
var Info  *log.Logger   // [INFO] tag
var Server *log.Logger  // [SERVER] tag
```

### Funciones

| Función | Descripción |
|---|---|
| `Initialize(useFiles bool)` | Crea rotator, tee a archivo+consola, redirige os.Stdout/Stderr |
| `Close()` | Cierra el archivo de log |

### Rotator (`rotator.go`)

Archivo `skypanel.log` en `LogsFolder` con rotación mediante señal `SIGUSR1`:

```go
type Rotator struct {
    io.WriteCloser
}
func (r *Rotator) Rotate(newBackend)     // swap atómico del backend
func (r *Rotator) StartRotation(dir)     // goroutine que escucha SIGUSR1
```

### MultiWriter (`multi.go`)

```go
func MultiWriter(writers ...io.Writer) io.Writer  // ignora nils, aplasta anidados
```

---

## internal/response/ — Helpers HTTP

```go
func NotImplemented(c *gin.Context)                                            // 501
func CreateOptions(options ...string) gin.HandlerFunc                          // middleware CORS OPTIONS
func HandleError(c *gin.Context, err error, statusCode int) bool               // error response estandarizado
```

- `HandleError` retorna `true` si el error no es nil (para early return en handlers)
- 404 automático para `gorm.ErrRecordNotFound`
- Response JSON con formato `{ "error": "mensaje", "code": "..." }`

---

## internal/utils/ — Utilidades Generales

### Seguridad de Red (SSRF Protection)

```go
func ValidateExternalURL(rawURL string) error                         // valida URL no apunte a IP privada
func isPrivateIP(ip net.IP) bool                                      // detecta rangos privados
func NewRestrictedHTTPClient() *http.Client                           // cliente HTTP que bloquea IPs internas
```

### Strings y Tokens

```go
func GenerateRandomString(n int) (string, error)                      // cadena aleatoria base64 URL
func ToString(v interface{}) string                                   // conversión a string
func UnmarshalTo(source, target interface{}) error                    // deep copy via JSON round-trip
```

### Slice Operations

```go
func Union[T comparable](a, b []T) []T                                // intersección
func Remove[T comparable](a []T, b T) []T                             // remover ocurrencias
```

### Argumentos y Tokens

```go
func ReplaceTokens(msg string, mapping map[string]interface{}) string           // reemplaza ${key}
func ReplaceTokensInArr(msg []string, mapping map[string]interface{}) []string
func ReplaceTokensInMap(msg map[string]string, mapping map[string]interface{}) map[string]string
func SplitArguments(source string) (cmd string, arguments []string)             // split respetando comillas
func MergeArguments(arguments []string) string                                  // re-join con quoting
```

### JVM Stats

```go
type JvmStats struct {
    HeapUsed, HeapTotal, MetaspaceUsed, MetaspaceTotal int64
}
func ParseJCMDResponse(data []byte) *JvmStats          // parsea output de jcmd
```

### Kernel Support

```go
func DetermineKernelSupport()       // detecta soporte de openat2
func UseOpenat2() bool              // openat2 disponible?
```

### Filesystem

```go
func GetDirSize(path string) (int64, error)   // tamaño recursivo de directorio
```

### Safe Close

```go
func Close(closer io.Closer)                    // close con recovery
func CloseResponse(response *http.Response)     // close de body HTTP
```

### Type Conversion

```go
func Convert(val interface{}, target interface{}) (interface{}, error)  // conversión genérica
```

### Data Extraction

```go
func GetStringOrDefault(data map[string]interface{}, key, def string) string
func GetBooleanOrDefault(data map[string]interface{}, key string, def bool) bool
func GetMapOrNull(data map[string]interface{}, key string) map[string]interface{}
func GetObjectArrayOrNull(data map[string]interface{}, key string) []interface{}
func GetStringArrayOrNull(data map[string]interface{}, key string) []string
```

### Wildcard Matching

```go
func CompareWildcard(source, match string) bool     // matching con *
func WildCardToRegexp(pattern string) string         // wildcard → regex
```

---

## internal/sys/ — Syscalls

```go
func SyscallMode(i os.FileMode) (o uint32)    // os.FileMode → syscall mode bits (setuid/setgid/sticky)
```

---

## internal/groups/ — Verificación de Grupos

```go
const SkyPanelGroup = "SkyPanel"
func IsUserIn(groups ...string) bool          // verifica si el usuario actual pertenece a algún grupo (incluye root)
```

---

## internal/systemd/ — Archivos de Despliegue

No es un paquete Go. Contiene:

| Archivo | Propósito |
|---|---|
| `servicefiles/skypanel.service` | Template de unit systemd |
| `servicefiles/apparmor.conf` | Configuración AppArmor |
| `debian/templates` | Templates de Debconf para empaquetado Debian |

---

## internal/storage/ — Datos de Volumen Docker

No es un paquete Go. Directorios usados como volúmenes Docker:

| Directorio | Propósito |
|---|---|
| `mysql-data/` | Archivos de base de datos MariaDB |
| `skypanel-config/` | Configuración del panel |
| `skypanel-logs/` | Logs del panel |
