# Remaining Packages

Reference for the Go packages that do not have a backend page of their own: the shared `pkg/skypanel` types, the `files` and `conditions` packages at the repository root, and the smaller `internal/` packages (queries, email, extensions, off-site backups, self-update, logging, HTTP helpers, utilities).

---

## Where each package is documented

| Package | Page |
|---|---|
| `cmd/panel` | [CLI](./07-cli.md) |
| `internal/config` | [Configuration](./02-configuration.md) |
| `internal/database`, `internal/models` | [Database](./03-database.md) |
| `internal/servers`, `internal/servers/docker`, `internal/servers/tty` | [Servers](./04-servers.md), [Process management & metrics](./13-process-metrics.md) |
| `internal/web` (`api`, `auth`, `daemon`, `oauth2`, `swagger`), `internal/middleware`, `internal/scopes`, `internal/oauth2` | [API layer](./05-api-layer.md) |
| `internal/operations/*` | [Operations](./06-operations.md) |
| WebSocket trackers (`pkg/skypanel/tracker.go`) | [WebSocket](./08-websocket.md) |
| `internal/sftp`, JWT, API keys | [Security](./09-security.md) |
| `internal/services` | [Services](./11-services.md) |
| `internal/connections` | [Console input: RCON, Telnet, RCON-WS](./12-rcon-telnet.md) |
| `files`, `conditions`, `pkg/skypanel`, `internal/query`, `internal/email`, `internal/extensions`, `internal/offsite`, `internal/update`, `internal/logging`, `internal/response`, `internal/utils`, `internal/sys`, `internal/groups`, `internal/systemd` | This page |

Things that are **not** separate packages, although they are sometimes described as such:

| Feature | Where the code is |
|---|---|
| AI assistant | `internal/web/api/ai.go`, `ai_chat.go` (see [AI assistant](../features/ai-assistant.md)) |
| Per-server notifications | `internal/servers/notifications.go` and `internal/services/channels.go` (see [Server notifications](../features/server-notifications.md)) |
| Uptime | `internal/services/uptime.go` (see [Services](./11-services.md#16-uptime-uptimego)) |
| Template repositories | `internal/services/templates.go` (see [Services](./11-services.md#11-template-templatesgo)) |
| Docker volume folders | `storage/` at the repository root, not `internal/storage` (see [below](#storage--docker-volume-folders)) |

---

## `files/` — File access and archives

Secure access to a server's folder and archive handling. Detailed behaviour (limits, symlinks, formats): [Files and compression](../general/19-files-compression.md).

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

`NewFileServer(prefix, uid, gid)` returns one jailed in `prefix`. Every path must be local (`filepath.IsLocal`). Files are opened with `openat2(RESOLVE_BENEATH)` when the kernel supports it; otherwise each directory is walked with `openat(O_NOFOLLOW|O_PATH)`. Opens add `O_NONBLOCK` so a named pipe cannot hang a request. Newly created files are chowned to `uid:gid` when `uid != -1` (best effort).

**openat2 support** is decided at startup by `utils.DetermineKernelSupport()`: it tries the syscall the way the file server uses it (opening `/` with `RESOLVE_BENEATH`). Success turns `openat2` on; `ENOSYS` (kernel without it), `EPERM` (a seccomp filter, such as an older Docker profile, blocks it) or `EOPNOTSUPP` turn it off; any other error falls back to looking for `do_sys_openat2` in `/proc/kallsyms`. When it is off the daemon **refuses to start** ("Cowardly not starting"), unless `security.forceOpenat` is true, which skips the check and forces the `openat` fallback.

### Writing into a server folder (`files.go`)

| Function | Description |
|---|---|
| `CreateIn(root, dest)` | Creates or truncates `dest` inside `root` through a `FileServer`, never following a symlink at the target, creating missing folders |
| `CopyFile(src, root, dest)` | Copies a panel file (cache) into the server folder |
| `WriteFile(src io.Reader, root, dest)` | Writes a reader into the server folder |

### Archives (`compression.go`)

Built on `github.com/mholt/archives`.

| Function | Description |
|---|---|
| `Extract(fs, source, target, filter, skipRoot, forcedType)` | Extracts an archive from the file server |
| `ExtractLimited(fs, source, target, filter, skipRoot, limit)` | Same, with a `WriteLimit` (used to enforce the disk limit) |
| `ExtractReader(fs, reader, source, target, filter, skipRoot)` | From an `io.ReadSeeker` into a file server |
| `ExtractFromReader(reader, source, target, filter, skipRoot, forcedType)` | From a reader to a plain folder |
| `DetermineIfSingleRoot(ctx, source, reader)` | Whether everything sits under one top folder |
| `CheckArchive(reader, source, skipRoot)` | Validates entries before extracting |
| `Compress(fs, target, files)` / `CompressLimited(...)` | Creates an archive (format from the extension) |
| `CompressServer(fs, target)` | Archives a whole server folder (backups) |
| `Readlink(fs, name)` | Reads a link beneath the root |

Entries that would land outside the target (`ErrPathTraversal`) are refused; symlinks are recreated only when they stay beneath the root.

### `MergedFS` (`mergedfs.go`)

`NewMergedFS(systems ...fs.FS)` layers several `fs.FS`: `Open` and `ReadFile` return the first hit, `ReadDir` merges entries by name. Used to overlay `panel.email.templateFolder` on the embedded email templates.

---

## `conditions/` — CEL expressions

Evaluates [CEL](https://github.com/google/cel-go) expressions for `if` conditions in templates and operations. Full guide: [Conditions (CEL)](../general/18-conditions-cel.md) and [Operations](./06-operations.md).

| Function | Description |
|---|---|
| `ResolveIf(condition, data, extras)` | Replaces the `${name}` tokens with `SubstituteVariables`, then runs the expression, which must return `bool` |
| `Run[T](statement, data, extras)` | Runs any expression and type-asserts the result to `T` |
| `SubstituteVariables(condition, data)` | Replaces each `${name}` whose name is in `data` or is `os`/`arch`: escaped inside string literals (a raw literal is closed and joined with an escaped string), the bare variable name outside them (a quoted value when the name is not a valid CEL identifier). Unknown names and bytes literals are left as they are (`variables.go`) |

- Every key of `data` becomes a CEL variable (type `any`), plus the constants `os` (`runtime.GOOS`) and `arch` (`runtime.GOARCH`).
- An empty statement becomes `success` when `data` has a `success` key.
- Variable name constants (`consts.go`): `os`, `arch`, `success`, `env`, `serverId`. Which of them are actually present depends on the caller's data map.

---

## `pkg/skypanel/` — Shared types

The domain types shared by the panel, the daemon and the operations.

### Interfaces

| Interface | File | Methods |
|---|---|---|
| `EnvironmentImpl` | `environment.go` | `ExecuteAsyncImpl`, `KillImpl`, `GetStatsImpl`, `SendCodeImpl`, `GetUIDImpl`, `GetGidImpl`, `IsRunningImpl` |
| `StartCanceler` | `environment.go` | `CancelStartImpl()`: stop a start whose image is still downloading |
| `Reattacher` | `environment.go` | `ReattachImpl(env, steps)`: reconnect to a process from before a daemon restart (Docker only) |
| `EnvironmentFactory` | `environmentfactory.go` | `Create() EnvironmentImpl`, `Key() string` |
| `Operation` / `OperationFactory` | `operation.go` | `Run(RunOperatorArgs) OperationResult`; `Create(CreateOperation)`, `Key()` |
| `Console` | `console.go` | `io.WriteCloser` + `Start()` ([Console input](./12-rcon-telnet.md)) |
| `DaemonServer` | `server.go` | `GetFileServer()`, `Extract()`, `ArchiveItems()`, `DataToMap()` |
| `SFTPAuthorization` / `SFTPKeyAuthorization` | `authorization.go` | `Validate(user, password)`; `ValidateKey(user, key)` |

`CreateOperation.Arg(names...)` returns the first argument the template set among several spellings (`appId` / `appID`).

### Main structs

| Type | File | Purpose |
|---|---|---|
| `Server`, `Execution`, `Command`, `Group`, `KeepAlive` | `server.go` | Server definition (JSON keys `data`, `install`, `uninstall`, `run`, `environment`, ...) |
| `Environment`, `ExecutionData` | `environment.go` | Runtime state of a server and the data for one start |
| `Variable`, `VariableOption` | `variable.go` | A template variable; `value` is converted to the declared type on load |
| `MetadataType`, `ConditionalMetadataType` | `typewithmetadata.go` | `{type, ...}` objects (operations, environments) with an optional `if` |
| `Requirements` | `requirements.go` | `os`, `arch`, `binaries` a template needs; `Test(server)` checks them |
| `Task` | `task.go` | Scheduled task: `name`, `cronSchedule`, `description`, `operations` |
| `ServerStats`, `ServerLogs`, `ServerRunning`, `ErrorResponse`, ... | `httpmodels.go` | API payloads ([metrics](./13-process-metrics.md#live-statistics)) |
| `Transmission`, `ServerCrash` | `message.go` | WebSocket messages; types `console`, `stat`, `status`, `crash` |
| `FileList`, `FileDesc` | `filelist.go` | File manager listings |
| `StdinConsoleConfiguration` | `stdin.go` | Console input settings |

### Console buffer (`cache.go`)

`MemoryCache` keeps the recent console output of a server in memory. `CreateCache()` sizes it from `daemon.console.buffer` (KB, default `50`; values ≤ 0 also mean 50). `Write` appends and evicts the oldest messages past the capacity; `Read()` returns everything with the last timestamp; `ReadFrom(t)` returns what was written after `t` (used by `GET /api/servers/:id/console?time=`).

### Errors (`errors.go`)

```go
type Error struct {
    Message string                 `json:"msg,omitempty"`
    Code    string                 `json:"code,omitempty"`
    Meta    map[string]interface{} `json:"metadata,omitempty"`
}
type ErrorResponse struct { Error *Error `json:"error"` }
```

- About a hundred predefined errors created with `CreateError(msg, code)`; the code is the variable name (`ErrServerOffline`, `ErrTooManyAttempts`, `ErrConsoleDetached`, `ErrDiskLimitExceeded`, `ErrNodeDiskFull`, ...).
- Parameterised ones are functions: `ErrSettingNotConfigured(name)`, `ErrNoTemplate(name)`, `ErrServiceInvalidProvider(service, provider)`, `ErrFieldRequired`, `ErrFieldLength(field, min, max)`, `ErrUnsupportedOS`, `ErrMissingBinary`, `ErrCurseForgeStatus`, and others. Messages use `${key}` placeholders filled from `Meta`.
- `CreateErrMissingScope(scope)`, `FromError(err)` (wraps a plain error as code `ErrGeneric`), `GenerateValidationMessage(err)` (validator errors) and `Recover()` (logs a panic with its stack instead of crashing the process).

### Downloads and HTTP (`download.go`, `client.go`)

| Function | Description |
|---|---|
| `DownloadFile(url, fileName, env)` | Downloads into the server folder through `files.CreateIn` (never through a symlink) |
| `DownloadFileToCache(url, fileName)` | Downloads into a panel path |
| `Download(url, hash, algorithm, cache, env)` | Reader with optional hash check and caching in `daemon.data.cache`, at `<cache>/<address without http:// or https://>` (`cache/maven.example.com/path/file.jar`); an address that would land outside the cache folder (a `..` in it) is downloaded without caching |
| `DownloadHash(hashURL, algorithm)` | Fetches a published checksum |
| `DownloadViaMaven(url, env)` | Maven artifact download |
| `HTTP()`, `HTTPGet(url)`, `HTTPExtract(url, dir, type)` | Shared client, GET, and download-and-extract |

Downloads whose URL comes from a template use `utils.NewDownloadHTTPClient` (see [utils](#internalutils--general-utilities)).

### Other

- `Engine` (`engine.go`): the global `*gin.Engine`, used by `services.serveLocal` to run node calls in-process.
- Version (`version.go`): `Version` (default `nightly`) and `Hash` (default `unknown`), set with `-ldflags`; `Display` = `"SkyPanel <version> (<hash>)"`.

---

## `internal/query/` — Game queries

Asks running game servers for their players. Used by `Server.QueryGame()` (`internal/servers/players.go`), which caches answers for 10 seconds and feeds the server cards and the player history.

| Function | Protocol | Result |
|---|---|---|
| `Minecraft(ip, port)` | Server list ping 1.7+ (`github.com/dreamscached/minequery/v2`, `Ping17`) | `MinecraftResponse{numPlayers, maxPlayers, version, players}` |
| `Source(ip, port)` | Steam A2S over UDP (A2S_INFO, then A2S_PLAYER), challenge supported, 1.5 s timeout, single-packet answers only | `SourceResponse{name, map, game, numPlayers, maxPlayers, bots, players}` |

**Address** (`host.go`, exported as `HostOrLocal` for the RCON, Telnet and RCON-over-WebSocket consoles too): an empty IP, `0.0.0.0` or `::` means "this machine". On a plain install that is `127.0.0.1`. When the daemon runs inside a Docker container (`/.dockerenv` exists), it uses the container's default gateway from `/proc/net/route`, which reaches the ports game containers publish on the host.

Which protocol is used depends on the template's `query.type`: `minecraft`, `source`/`steam`, or empty (A2S is tried, and after a failure not retried for 2 minutes). The port is the template's query port variable (`queryport`, `query_port`, `queryPort`, `steamqueryport`, `steam_query_port`) or the game `port`.

---

## `internal/email/` — Email providers

```go
type Provider interface {
    Send(to, subject, body string) error
}
func GetProvider(name string) Provider   // case-insensitive, nil if unknown
```

Providers register themselves in `init()`:

| Name | File | Library | Settings it needs |
|---|---|---|---|
| `smtp` | `smtp.go` | `github.com/wneessen/go-mail` | `panel.email.from`, `panel.email.host`, optional `username`/`password`; implicit TLS (SSL port), auth auto-discovered; spaces stripped from the password |
| `sendgrid` | `sendgrid.go` | `sendgrid-go` | `from`, `key` |
| `mailgun` | `mailgun.go` | `mailgun-go` | `domain`, `from`, `key` |
| `mailjet` | `mailjet.go` | `mailjet-apiv3-go` | `domain` (the public API key), `from`, `key` (the secret key) |
| `debug` | `debug.go` | — | Only prints the email to the debug log |

A missing setting returns `ErrSettingNotConfigured(<key>)`. Rendering and templates are in `services.EmailSender` ([Services](./11-services.md#20-emailsender-emailgo)); configuration in [Email](../general/15-email.md).

---

## `internal/extensions/` — Panel extensions

Administrator add-ons: a zip with `extension.json` at its root and HTML pages. Pages are shown in a sandboxed frame; events are posted to the extension's webhook. Packaging guide: [Extensions](../reference/15-extensions.md).

| Item | Value |
|---|---|
| Files | `<panel.extensions.folder>/<id>` (default `extensions` under the data folder) |
| Package limits | 20 MB zip, 500 files, 50 MB unpacked, manifest ≤ 64 KB |
| Manifest | `id` (2–40 of `a-z0-9-`), `name` (≤ 60), `version` (≤ 20), `author`, `description` (≤ 300), up to 10 `pages` (`id`, `title` ≤ 40, `placement` `server` or `admin`, `path` to an `.html` in the package), `webhook` (https), `events`; unknown fields are rejected |
| Events | `server.created`, `server.deleted`, `server.started`, `server.stopped`, `server.crashed`, `server.backup`, `server.resources` |
| Install | `Install(db, zip, size)`: a new extension starts **disabled**; reinstalling an id updates it and keeps its switch, secret and an admin-edited webhook |
| Delivery | `Publish(event, data)` runs in the background; body = data + `event`, `extension`, `timestamp`; signed with the extension's secret (`X-Aether-Signature`, see [channels](./11-services.md#notification-channels-channelsgo)); retries after 5 s and 30 s; at most 32 deliveries in flight, extra events are dropped and logged |
| Other | `Uninstall(db, id)`, `Reload(db)` (after any change), `SendTest(ext)` (a `test` event), `ServerData(id, name)` |

`server.created` / `server.deleted` are published by the API (`internal/web/api/servers.go`, `provision.go`); the others come from `internal/servers/notifications.go` through `servers.EventSink`.

---

## `internal/offsite/` — Off-site backups

Takes backups to an S3-compatible bucket (AWS S3, Backblaze B2, Cloudflare R2, Wasabi, MinIO) with `github.com/minio/minio-go/v7`: each server's periodic copy and the backups of the tasks whose turn it is to upload. Manual backups are not uploaded. User guide: [Backups](../features/backups.md).

| Setting | Default | Notes |
|---|---|---|
| `daemon.backups.s3.enabled` | `false` | |
| `daemon.backups.s3.endpoint` | `""` | `host`, `host:port` or a full URL (its scheme then decides https) |
| `daemon.backups.s3.region` | `""` | |
| `daemon.backups.s3.bucket` | `""` | Must already exist |
| `daemon.backups.s3.accessKey` / `secretKey` | `""` | |
| `daemon.backups.s3.prefix` | `aether-backups` | Objects are `<prefix>/<serverId>/<file>.tar.gz` |
| `daemon.backups.s3.insecure` | `false` | Plain http, for a MinIO on the LAN |
| `daemon.backups.s3.periodicHours` | `0` | Default interval of the periodic copy (`0`, `12`, `24`, `48`, `72`, `168`, `336` or `720`; `0` = off) |
| `daemon.backups.s3.periodicKeep` | `4` | Periodic copies kept per server (1-50) |

`FromSettings()` returns the config and whether it is enabled **and** complete (endpoint, bucket, both keys). It reads the keys on every call, so a change takes effect without a restart; on a remote node they come from the panel, which sends them with `PUT /daemon/settings` ([Daemon API](../reference/api/09-daemon.md#put-daemonsettings)).

| Function | Used for |
|---|---|
| `Upload` | `Server.uploadOffsite` (2 h timeout): after the backup of a task whose turn it is to upload (in the background; the local copy stays and a failure does not fail the task) and in the periodic copy (the local file is deleted afterwards) |
| `List` | Backup list: entries are marked `local`, `offsite` or `both` (15 s timeout; on error the local list is shown). Also decides whether a task uploads its backup, finds the last periodic copy and prunes each series (1 min timeout) |
| `Reachable` | Before writing a periodic copy: checks the bucket answers and exists, without writing anything |
| `Download` | Restoring a backup only the bucket has (downloaded to `<file>.download`, then renamed) |
| `Open` | Downloading a backup straight from the bucket |
| `Delete` | Deleting a backup from the panel also deletes the object; it also prunes a task's copies beyond `offsiteKeep` and the periodic ones beyond `periodicKeep`. Local task pruning (`keep`) does not use it |
| `Test` | `POST /api/settings/test/s3` (scope `settings.edit`): checks the bucket exists, writes a small object under `<prefix>/.aether-test/` and deletes it |

---

## `internal/update/` — Self-update

Runs the host's update script from inside the panel. User guide: [Panel updates](../features/panel-updates.md).

- `Trigger()`: pulls `alpine:latest` if missing, creates a container with host networking and `/` mounted at `/host`, and runs `chroot /host bash -c "cd /opt/skypanel && chmod +x tools/panelUpdate/panelUpdate.sh && ./tools/panelUpdate/panelUpdate.sh"`. The path `/opt/skypanel` is fixed in code.
- A background monitor polls the container every 2 s, keeps the last 8,000 characters of its log, records the exit code and removes the container.
- `Status()` returns `{running, containerId, startedAt, finishedAt, exitCode, log, error}`; `RecordFailure(msg)` marks an update that could not start.

Entry points (`internal/web/api/settings.go`, scope `settings.edit`):

| Endpoint | Description |
|---|---|
| `POST /api/settings/update-panel` | Triggers the update on the panel's own node and calls `POST /daemon/update` on every remote node; answers with one result per node |
| `GET /api/settings/update-status` | `Status()` of the local update |
| `GET /api/settings/update-check` | Compares the deployed commit with the latest commit of the branch the panel was installed from (`main` when it cannot be read) |

On each node the daemon exposes `POST /daemon/update` and `GET /daemon/update-status` (panel JWT).

---

## `internal/logging/` — Logging

```go
var Error  = log.New(os.Stderr, "[ERROR] ", log.LstdFlags)
var Debug  = log.New(os.Stdout, "[DEBUG] ", log.LstdFlags)
var Info   = log.New(os.Stdout, "[INFO] ", log.LstdFlags)
var Server = log.New(os.Stdout, "[SERVER] ", log.LstdFlags)
var OriginalStdOut, OriginalStdErr = os.Stdout, os.Stderr
```

- `Initialize(useFiles)`: with files, creates the `logs` folder (default `logs`) and a `Rotator`; recreates `Error`, `Info` and `Debug` writing to both the file and the console; redirects `log`, `os.Stdout` and `os.Stderr` through pipes into `Info` / `Error`. `Server` is not redirected to the file.
- `Close()` closes the log file.

**Rotator** (`rotator.go`): the current day goes to `skypanel.log`. When the day changes, or the file reaches 100 MB, it is renamed `skypanel-<YYYY-MM-DD>.log` (a free suffix is used if taken) and gzip-compressed in the background. Archives last modified more than `logRetentionDays` days ago (default `14`, `0` keeps them forever) are deleted; only `skypanel-*` files are touched. A file left over from an earlier day is archived at startup. `SIGUSR1` reopens `skypanel.log` for external logrotate setups.

**MultiWriter** (`multi.go`): like `io.MultiWriter`, but skips nil writers and flattens nested ones.

`daemon.console.forward` (`Environment.CreateWrapper`) also copies every server's console to the daemon's original stdout.

---

## `internal/response/` — HTTP helpers

| Function | Description |
|---|---|
| `HandleError(c, err, status) bool` | Returns `false` when `err` is nil. Otherwise logs the error and aborts: `gorm.ErrRecordNotFound` → 404 with no body; a status ≥ 500 with a non-panel error → `{"error": ErrInternal}` (details only in the log); anything else → `{"error": {"msg", "code", "metadata"}}` |
| `CreateOptions(methods...)` | Handler for `OPTIONS` routes: `Access-Control-Allow-Origin: *`, the allowed methods plus `OPTIONS`, allowed headers `authorization, origin, content-type, accept`, status 200 |
| `NotImplemented(c)` | 501 with `ErrNotImplemented` |

---

## `internal/utils/` — General utilities

### Outbound requests

| Function | Description |
|---|---|
| `NewRestrictedHTTPClient()` | Refuses private, loopback, link-local, CGNAT, benchmarking and unspecified addresses **at connect time** (after DNS, so DNS rebinding is covered); 10 s dial timeout. Used for Discord, Telegram and JSON webhooks. |
| `ValidateExternalURL(url)` | http/https only, no `localhost`, every resolved IP public |
| `NewDownloadHTTPClient(onPrivate)` | For template downloads: always refuses loopback, link-local, multicast and unspecified; private LAN addresses are allowed unless `security.blockPrivateDownloads` (default `true`) |
| `DownloadFile(url, onPrivate, save)` / `DownloadWith(client, url, save)` | Download with grab into a temporary folder under `daemon.data.cache`, then hand the file to `save` (never written straight into a server folder). `DownloadFile` uses `NewDownloadHTTPClient` (used by `download`, `scraperdl` and `githubdl`); `DownloadWith(nil, ...)` uses grab's default client, which has no address guard |

### Templates and arguments

| Function | Description |
|---|---|
| `ReplaceTokens(msg, mapping)` | Replaces `${key}` with `fmt.Sprint(value)` |
| `ReplaceTokensInArr`, `ReplaceTokensInMap` | Same for slices and maps |
| `SplitArguments(source)` / `MergeArguments(args)` | Split a command line respecting quotes / join it back |

### Data helpers

| Function | Description |
|---|---|
| `GetStringOrDefault`, `GetBooleanOrDefault`, `GetMapOrNull`, `GetObjectArrayOrNull`, `GetStringArrayOrNull` | Typed reads from `map[string]interface{}` |
| `Convert(val, target)` | Converts `val` to the type of `target` |
| `ToString(v)`, `UnmarshalTo(source, target)` | String conversion; copy through a JSON round-trip (`target` must be a pointer) |
| `Intersection[T](a, b)` | Elements of `a` also in `b`, in the order of `a` |
| `Remove[T](a, b)` | `a` without occurrences of `b` |
| `GenerateRandomString(n)` | `n` random bytes, base64 URL encoded |
| `CompareWildcard(source, match)`, `WildCardToRegexp(pattern)` | `*` wildcard matching (empty or `*` matches everything; used by the archive extraction filter) |
| `GetDirSize(path)` | Recursive folder size |
| `Close(closer)`, `CloseResponse(resp)` | Close ignoring errors and panics |

### System

| Function | Description |
|---|---|
| `DetermineKernelSupport()`, `UseOpenat2()` | openat2 detection, see [files](#files--file-access-and-archives) |
| `ParseJCMDResponse(data) *JvmStats` | Parses `jcmd` output into heap and metaspace usage |

---

## `internal/sys/` — Syscall helpers

`SyscallMode(os.FileMode) uint32` converts permission bits plus setuid, setgid and sticky to the raw mode used by `openat2`, `openat` and `mkdirat` in `files/filesystem.go`.

---

## `internal/groups/` — Group check

`IsUserIn(groups...) bool` says whether the current OS user belongs to one of the groups or to `root`. `SkyPanelGroup = "SkyPanel"`. Only `skypanel user edit` uses it (`cmd/panel/user.go`); the same check in `main.go` is commented out.

---

## `internal/systemd/` — Native packaging files

Not a Go package and not used by the Docker installer:

| File | Content |
|---|---|
| `servicefiles/skypanel.service` | `Type=notify` unit running `/usr/sbin/SkyPanel runService --config /etc/SkyPanel/config.json` as user/group `SkyPanel` |
| `servicefiles/apparmor.conf` | Unconfined AppArmor profile for `/usr/sbin/SkyPanel` |
| `debian/templates` | Debconf templates |

`cmd/panel/runservice.go` notifies systemd when `NOTIFY_SOCKET` is set.

---

## `storage/` — Docker volume folders

At the repository root (there is no `internal/storage`). `docker-compose.yml` mounts:

| Folder | Mounted at | Service |
|---|---|---|
| `storage/mysql-data` | `/var/lib/mysql` | MariaDB |
| `storage/skypanel-config` | `/etc/SkyPanel` | Panel configuration |
| `storage/skypanel-data` | `/var/lib/SkyPanel` | Servers, backups, cache, history files |
| `storage/skypanel-logs` | `/var/log/SkyPanel` | Panel logs |

See [Docker](../docker/README.docker.md).
