# Utility Packages

Documentation for the backend's utility and cross-cutting packages.

---

## Templates System (`internal/services/templates.go`)

Template management with support for local templates, Git repositories, and VPS JSON index repositories.

### Template Repository (`models/templaterepo.go`)

```go
type TemplateRepo struct {
    ID       uint   `gorm:"primaryKey"`
    Name     string `gorm:"unique;size:100"`
    URL      string `gorm:"size:500"`      // Git repo URL or VPS JSON index URL
    Branch   string `gorm:"size:100;default:main"`
    PAT      string `gorm:"size:200"`      // Personal Access Token (private repos)
    Username string `gorm:"size:100"`      // Git username
    Password string `gorm:"size:200"`      // Git password / token
    SSHKey   string `gorm:"type:text"`     // SSH private key for auth
    IsLocal  bool   `gorm:"-"`             // Runtime: true for repo ID 0
}
```

**Repo Types:**
| ID | Type | Description |
|----|------|-------------|
| 0 | Local | Templates stored in DB (editable via panel) |
| >0 | Git | Cloned from remote Git repo |
| >0 | VPS JSON | Index URL pointing to `templates.json` |

### Template (`models/template.go`)

```go
type Template struct {
    Name       string `gorm:"primaryKey;size:100"`
    RawValue   string `gorm:"type:mediumtext"`  // Full server definition JSON
    Readme     string `gorm:"type:text"`        // Markdown description
}
```

### TemplateService (`internal/services/templates.go`)

```go
type TemplateService struct {
    DB *gorm.DB
}

func (s *TemplateService) GetRepos() ([]*models.TemplateRepo, error)
func (s *TemplateService) AddRepo(repo *models.TemplateRepo) error
func (s *TemplateService) DeleteRepo(id uint) error
func (s *TemplateService) GetAllFromRepo(repoID uint) ([]*models.Template, error)
func (s *TemplateService) Get(repoID uint, name string) (*models.Template, error)
func (s *TemplateService) Save(template *models.Template) error
func (s *TemplateService) Delete(repoID uint, name string) error

// VPS JSON Index Repositories
func (s *TemplateService) SyncRepo(repo *models.TemplateRepo) error
func (s *TemplateService) getAllFromVps(repo *models.TemplateRepo) ([]*models.Template, error)
func (s *TemplateService) getFromVps(repo *models.TemplateRepo, name string) (*models.Template, error)
func (s *TemplateService) getTemplateFromURL(url string) (*models.Template, error)
```

### VPS JSON Index Format

Remote repository index (`templates.json`):

```json
{
  "templates": [
    {
      "name": "minecraft-java",
      "url": "https://example.com/templates/minecraft-java.json"
    },
    {
      "name": "paper-mc",
      "url": "https://example.com/templates/paper-mc.json"
    }
  ]
}
```

**SyncRepo Flow:**
1. Fetch `templates.json` from `repo.URL`
2. Parse template list
3. For each template: fetch individual JSON from `template.url`
4. Store in DB as `Template` records
4. Cache for 5 minutes

### Private Repository Authentication

Supports multiple auth methods for private Git repos:

| Method | Config Fields |
|--------|---------------|
| HTTPS + PAT | `PAT` (Personal Access Token) |
| HTTPS + Basic | `Username` + `Password` |
| SSH | `SSHKey` (private key) |

### Local Repository (ID = 0)

- Templates stored directly in DB
- Editable via `PUT /api/templates/0/:name`
- No external sync needed

---

## Utility Packages

Documentation for the backend's utility and cross-cutting packages.

---

## files/ — Filesystem and Compression

### FileServer Interface

Defines secure file operations with root jail (path traversal prevention via `openat2` / `openat` + `O_NOFOLLOW`):

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

Concrete implementation: `fileServer` in `filesystem.go`.

- **`NewFileServer(prefix string, uid, gid int) (FileServer, error)`** — Creates a FileServer jailed in `prefix`
- **`OpenFile(path, flags, mode)`** — Uses `openat2` with `RESOLVE_BENEATH` (kernel >=5.6) or falls back to `openat` + `O_NOFOLLOW`. Chowns the file if `uid != -1`
- **`MkdirAll`, `Rename`, `Remove`, `RemoveAll`** — Operate with `unix.Mkdirat`, `unix.Renameat2`, `unix.Unlinkat`

### MergedFS

```go
type MergedFS struct {
    fileSystems []fs.FS
}
```

- **`NewMergedFS(systems ...fs.FS) *MergedFS`** — Merges multiple `fs.FS` into a single one
- **`Open(name)`** — Returns the first successful `fs.File`
- **`ReadDir(name)`** — Merges directories, deduplicating by name
- **`ReadFile(name)`** — Returns the first successful read

### Compression (`compression.go`)

Uses the `mholt/archives` library to support multiple formats (ZIP, tar.gz, tar.bz2, etc.).

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

| Function | Description |
|---|---|
| `Extract(fs, sourceFile, targetPath, filter, skipRoot, forcedType)` | Extracts a file from the FileServer |
| `ExtractFromReader(reader, sourceFile, targetPath, filter, skipRoot, forcedType)` | Extracts from an arbitrary `io.ReadSeeker` |
| `DetermineIfSingleRoot(ctx, sourceFile, reader)` | Detects if a file has a single root directory |
| `Compress(fs, targetFile, filesToCompress)` | Compresses files/directories, detects format by extension |
| `safeJoin(base, path string)` | Prevents path traversal by verifying that the resolved path is under base |

### CopyFile / WriteFile (`files.go`)

```go
func CopyFile(src, dest string) error     // copies file creating parent directories
func WriteFile(src io.Reader, dest string) error  // writes reader to file
```

---

## conditions/ — CEL Expression Evaluation

Evaluation engine for **CEL** (Common Expression Language) from `google/cel-go`. Used for conditions in installation operations, templates, and anywhere with `{{ expression }}` syntax.

### Global Constants

```go
var GlobalConstantValues = map[string]interface{}{
    "os":   runtime.GOOS,
    "arch": runtime.GOARCH,
}
```

### Functions

| Function | Description |
|---|---|
| `ResolveIf(condition, data, extras)` | Evaluates CEL condition which must return `bool`. If condition is empty, uses `data["success"]` |
| `Run[T](statement, data, extras)` | Evaluates any CEL expression and type-asserts to `T` |
| `ReplaceInString(str, data, extras)` | Replaces `{{ expression }}` in string with the result of evaluating each expression |

### Variables Available in Expressions

| Variable | Description |
|---|---|
| `os` | `runtime.GOOS` |
| `arch` | `runtime.GOARCH` |
| `success` | Result of previous operation |
| `env` | Environment type (docker/tty) |
| `serverId` | Server ID |
| Any server variable | Defined in `Server.Variables` |

---

## pkg/skypanel/ — Shared Types

Core domain types package. All other packages depend on it.

### Key Interfaces

| Interface | File | Methods |
|---|---|---|
| `EnvironmentImpl` | `environment.go` | `ExecuteAsyncImpl`, `KillImpl`, `GetStatsImpl`, `SendCodeImpl`, `GetUIDImpl`, `GetGidImpl`, `IsRunningImpl` |
| `EnvironmentFactory` | `environmentfactory.go` | `Create() EnvironmentImpl`, `Key() string` |
| `Operation` | `operation.go` | `Run(args RunOperatorArgs) OperationResult` |
| `OperationFactory` | `operation.go` | `Create(CreateOperation) (Operation, error)`, `Key() string` |
| `Console` | `console.go` | `io.WriteCloser`, `Start()` |
| `DaemonServer` | `server.go` | `GetFileServer()`, `Extract()`, `ArchiveItems()`, `DataToMap()` |
| `SFTPAuthorization` | `authorization.go` | `Validate(username, password) (*ssh.Permissions, error)` |

### Trackers (Pub/Sub System for WebSocket)

```go
type Tracker struct {
    sockets []*Socket
    locker  sync.Mutex
}
```

| Function | Description |
|---|---|
| `CreateTracker()` | Creates an empty tracker |
| `Register(conn *Socket)` | Adds socket to the tracker |
| `WriteMessage(msg Transmission)` | Sends JSON message to all sockets |
| `Write(source []byte)` | Sends bytes to all sockets |

```go
type Socket struct {
    conn   *websocket.Conn
    locker sync.Mutex
    io.WriteCloser
}
```

| Function | Description |
|---|---|
| `Create(ws)` | Creates socket from WebSocket connection |
| `WriteMessage(msg Transmission)` | Sends typed message |
| `Write(data []byte)` | Sends raw data |
| `WriteJSON(data interface{})` | Sends JSON |
| `Close()` | Closes connection |

### MemoryCache (Console Circular Buffer)

```go
type MemoryCache struct {
    Buffer   []cacheMessage
    Capacity int    // capacity in KB (config: daemon.console.buffer)
    Size     int
    Lock     sync.RWMutex
}
```

| Method | Description |
|---|---|
| `CreateCache()` | Creates with capacity of `daemon.console.buffer` KB |
| `Read()` | Returns the entire buffer |
| `ReadFrom(startTime)` | Returns data from timestamp |
| `Write(b)` | Adds data, evicts oldest if capacity exceeded |

### Shared Errors

Typed error with code and metadata:

```go
type Error struct {
    Message string
    Code    string
    Meta    map[string]interface{}
}
```

| Function | Description |
|---|---|
| `CreateError(msg, code)` | Creates typed error |
| `FromError(err)` | Converts any error to `*Error` |

Predefined errors (selection):
- `ErrInvalidCredentials`, `ErrServerOffline`, `ErrNoPermission`
- `ErrIllegalFileAccess`, `ErrBackupInProgress`, `ErrDockerNotSupported`
- `ErrDatabaseNotAvailable`, `ErrNotImplemented`, `ErrNoTemplate`, `ErrNodeInvalid`
- `ErrInvalidSession`, `ErrSessionExpired`, `ErrUserNotFound`, `ErrServerNotFound`

Parameterized factories:
- `ErrSettingNotConfigured(name)`, `ErrNoTemplate(template)`, `ErrServiceInvalidProvider(service, provider)`
- `ErrFieldTooLarge(field, value)`, `ErrFieldTooSmall(field, value)`, `ErrFieldNotBetween(field, min, max)`
- `ErrFieldEqual(f1, f2)`, `ErrFieldNotEqual(f1, f2)`, `ErrFieldNotEmail(field)`, `ErrFieldLength(field, min, max)`
- `ErrFactoryError(op, err)`, `ErrUnsupportedOS(actual, expected)`, `ErrUnsupportedArch(actual, expected)`
- `ErrMissingBinary(binary)`, `ErrPathNotAbs(path)`, `ErrCurseForgeDistribution(projectId)`, `ErrCurseForgeFile(projectId, fileID)`

### Downloads

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
    Value        interface{}        // auto-converted to int/bool based on type
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
var Version string   // semver version (ldflags)
var Display string   // "SkyPanel v3.x.x (hash)"
```

---

## internal/connections/ — Console Connection Proxies

Three connection types implementing `io.WriteCloser` + `Start()` to act as server stdin:

### RCON

```go
type RCONConnection struct {
    io.WriteCloser
    IP, Port, Password string
    Reconnect          bool
}
```

- **`Start()`** — Starts automatic reconnection loop
- **`Write(p []byte)`** — Executes command via `gorcon/rcon`
- IP/port/password come from the server configuration (`StdinConsoleConfiguration`)

### RCON over WebSocket

```go
type RCONWSConnection struct {
    io.WriteCloser
    IP, Port, Password string
    Reconnect          bool
}
```

- **`Start()`** — Connects via WebSocket to `ws://IP:Port/Password`
- **`Write(p []byte)`** — Sends JSON message with incremental identifier

### Telnet

```go
type TelnetConnection struct {
    io.WriteCloser
    IP, Port, Password string
    Reconnect          bool
}
```

- **`Start()`** — Connects TCP, sends password, maintains keepalive
- **`Write(p []byte)`** — Writes to the TCP socket

Automatically selected by `Environment.CreateConsoleStdinProxy()` based on `StdinConsoleConfiguration.Type`.

---

## internal/query/ — Minecraft Server Query

```go
type MinecraftResponse struct {
    NumPlayers int      `json:"numPlayers"`
    MaxPlayers int      `json:"maxPlayers"`
    Version    string   `json:"version"`
    Players    []string `json:"players"`
}

func Minecraft(ip string, port int) (MinecraftResponse, error)
```

- Uses `dreamscached/minequery/v2` with `Ping17` protocol (Minecraft 1.7+)
- Default IP: `127.0.0.1`

---

## internal/email/ — Email Providers

### Interface

```go
type Provider interface {
    Send(to, subject, body string) error
}
```

### Implemented Providers

| Name | File | Dependency |
|---|---|---|
| `smtp` | `smtp.go` | `wneessen/go-mail` |
| `sendgrid` | `sendgrid.go` | `sendgrid/sendgrid-go` |
| `mailjet` | `mailjet.go` | `mailjet/mailjet-apiv3-go` |
| `mailgun` | `mailgun.go` | `mailgun/mailgun-go` |
| `debug` | `debug.go` | logging only |

Registered via `init()`. Obtained with:

```go
func GetProvider(provider string) Provider
```

Configuration used: `EmailProvider`, `EmailFrom`, `EmailDomain`, `EmailHost`, `EmailKey`, `EmailUsername`, `EmailPassword`.

---

## internal/logging/ — Logging System

### Loggers

```go
var Error *log.Logger   // [ERROR] tag
var Debug *log.Logger   // [DEBUG] tag
var Info  *log.Logger   // [INFO] tag
var Server *log.Logger  // [SERVER] tag
```

### Functions

| Function | Description |
|---|---|
| `Initialize(useFiles bool)` | Creates rotator, tees to file+console, redirects os.Stdout/Stderr |
| `Close()` | Closes the log file |

### Rotator (`rotator.go`)

File `skypanel.log` in `LogsFolder` with rotation via `SIGUSR1` signal:

```go
type Rotator struct {
    io.WriteCloser
}
func (r *Rotator) Rotate(newBackend)     // atomic swap of the backend
func (r *Rotator) StartRotation(dir)     // goroutine that listens for SIGUSR1
```

### MultiWriter (`multi.go`)

```go
func MultiWriter(writers ...io.Writer) io.Writer  // ignores nils, flattens nested
```

---

## internal/response/ — HTTP Helpers

```go
func NotImplemented(c *gin.Context)                                            // 501
func CreateOptions(options ...string) gin.HandlerFunc                          // CORS OPTIONS middleware
func HandleError(c *gin.Context, err error, statusCode int) bool               // standardized error response
```

- `HandleError` returns `true` if the error is not nil (for early return in handlers)
- Automatic 404 for `gorm.ErrRecordNotFound`
- JSON response with format `{ "error": "message", "code": "..." }`

---

## internal/utils/ — General Utilities

### Network Security (SSRF Protection)

```go
func ValidateExternalURL(rawURL string) error                         // validates URL does not point to private IP
func isPrivateIP(ip net.IP) bool                                      // detects private ranges
func NewRestrictedHTTPClient() *http.Client                           // HTTP client that blocks internal IPs
```

### Strings and Tokens

```go
func GenerateRandomString(n int) (string, error)                      // base64 URL random string
func ToString(v interface{}) string                                   // conversion to string
func UnmarshalTo(source, target interface{}) error                    // deep copy via JSON round-trip
```

### Slice Operations

```go
func Union[T comparable](a, b []T) []T                                // intersection
func Remove[T comparable](a []T, b T) []T                             // remove occurrences
```

### Arguments and Tokens

```go
func ReplaceTokens(msg string, mapping map[string]interface{}) string           // replaces ${key}
func ReplaceTokensInArr(msg []string, mapping map[string]interface{}) []string
func ReplaceTokensInMap(msg map[string]string, mapping map[string]interface{}) map[string]string
func SplitArguments(source string) (cmd string, arguments []string)             // split respecting quotes
func MergeArguments(arguments []string) string                                  // re-join with quoting
```

### JVM Stats

```go
type JvmStats struct {
    HeapUsed, HeapTotal, MetaspaceUsed, MetaspaceTotal int64
}
func ParseJCMDResponse(data []byte) *JvmStats          // parses jcmd output
```

### Kernel Support

```go
func DetermineKernelSupport()       // detects openat2 support
func UseOpenat2() bool              // openat2 available?
```

### Filesystem

```go
func GetDirSize(path string) (int64, error)   // recursive directory size
```

### Safe Close

```go
func Close(closer io.Closer)                    // close with recovery
func CloseResponse(response *http.Response)     // close of HTTP body
```

### Type Conversion

```go
func Convert(val interface{}, target interface{}) (interface{}, error)  // generic conversion
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
func CompareWildcard(source, match string) bool     // matching with *
func WildCardToRegexp(pattern string) string         // wildcard → regex
```

---

## internal/sys/ — Syscalls

```go
func SyscallMode(i os.FileMode) (o uint32)    // os.FileMode → syscall mode bits (setuid/setgid/sticky)
```

---

## internal/groups/ — Group Verification

```go
const SkyPanelGroup = "SkyPanel"
func IsUserIn(groups ...string) bool          // verifies if the current user belongs to any group (includes root)
```

---

## internal/systemd/ — Deployment Files

Not a Go package. Contains:

| File | Purpose |
|---|---|
| `servicefiles/skypanel.service` | systemd unit template |
| `servicefiles/apparmor.conf` | AppArmor configuration |
| `debian/templates` | Debconf templates for Debian packaging |

---

## internal/storage/ — Docker Volume Data

Not a Go package. Directories used as Docker volumes:

| Directory | Purpose |
|---|---|
| `mysql-data/` | MariaDB database files |
| `skypanel-config/` | Panel configuration |
| `skypanel-logs/` | Panel logs |
