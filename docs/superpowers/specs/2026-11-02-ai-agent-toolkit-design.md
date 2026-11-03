# AI agent toolkit — design

Status: approved for build (owner asked to start directly, 2026-11-02). Owner's full requirements: the request in the session (agentic Gemini chat per server, tools in `tools/toolkit/`, strict per-server isolation, confirmations, chat UI, tests, ES/EN docs).

## Intent

Turn each server's AI chat into an assistant that can inspect and change that server's files through Gemini function calling, with real operations, strictly confined to the server of the conversation, with explicit user confirmation for anything that changes or removes existing data.

## Decisions (made by the coordinator; the owner opted out of questions)

1. **Language and place.** Go, under `tools/toolkit/` (module path `github.com/SkyPanel/SkyPanel/v3/tools/toolkit/...`). The toolkit has no dependency on `internal/web` or on Gemini HTTP details outside its own `gemini` package. Panel glue lives in `internal/web/api` and the daemon glue in `internal/web/daemon`.
2. **Where operations run.** On the server's node, through the existing confined file server (`files.FileServer`: openat2/RESOLVE_BENEATH, symlinks refused out of root). The panel never touches files itself: it calls the node's `/daemon/server/<id>/...` routes for the server of the conversation. This works for local and remote nodes and reuses the isolation that already exists.
3. **Isolation.** The server ID comes only from the authenticated route (`/api/servers/:serverId/ai/...`, `ResolveServerPanel`). No tool has a server parameter. Every tool call runs against a `Workspace` bound to that server when it is created; paths are relative to the server root and validated twice (toolkit `security` package, then the daemon's file server).
4. **Panel-internal files** (`.cron`, `.history`, `.crashes`, `.notify`, `.metrics`, `.worldmap`, `.modpack`) live next to the server folder, outside its root, so the agent cannot reach them. Protected paths inside the root: none by default; the `security` package has a list hook (`ProtectedPaths`) for later use.
5. **Tool kinds and confirmation policy.**
   - `read`: list, read, info, search — no confirmation, needs `server.files.view`.
   - `create`: create file (fails if it exists), create directory, copy (fails if the target exists) — no confirmation, needs `server.files.edit`.
   - `modify`: edit file (exact text replacement), overwrite file, move, rename — confirmation, needs `server.files.edit`.
   - `destructive`: delete file or directory — confirmation, needs `server.files.edit`.
   Tools the user has no scope for are not sent to Gemini at all (and are refused if called anyway).
6. **No command execution.** No tool runs shell commands or console commands. (Console/power tools are out of scope for this version.)
7. **Feature switch.** New panel setting `panel.settings.aiFileTools` (bool, default `true`, Settings → General next to the AI options). Off → the chat behaves as before (no tools). `panel.settings.hideAiAnalysis` is unrelated (analysis card only).
8. **History and undo.** File writes go through the daemon's existing PUT file route so file history (versions of edited files) applies as it does for edits in the file manager. Deletes go through the existing DELETE route.
9. **Audit.** Every executed tool call that changes something is recorded in the server's activity log with action `ai.<tool>` and the path(s) as detail, under the user; reads are not recorded (too noisy) but are logged at debug level. Add the activity keys to the frontend labels (en/es) and to an activity family (`files`).
10. **State.** Conversation state for a pending confirmation is kept in panel memory (map with expiry 10 min, max 1 pending session per user+server). A panel restart drops pending confirmations: the chat shows "this request expired, ask again".

## Package layout (`tools/toolkit/`)

```
tools/toolkit/
  toolkit.go        // Tool, Kind, Call, Result, ExecContext, errors
  registry.go       // Registry: Register, Get, Available(scopes)
  README.md / README.es.md
  security/         // path validation, size and search limits, protected paths
  workspace/        // Workspace interface + FileServerWorkspace (over files.FileServer)
  filesystem/       // the tools: list_directory, read_file, file_info, search_files,
                    //   create_file, create_directory, copy_path,
                    //   edit_file, write_file (overwrite), move_path, rename_path,
                    //   delete_path; Register(reg *toolkit.Registry)
  agent/            // the loop: Generator interface, Runner, limits, ConfirmationStore
  gemini/           // Gemini function-calling client implementing agent.Generator
```

## Go contracts (all builders must use exactly these)

```go
package toolkit

type Kind string
const (
	KindRead        Kind = "read"
	KindCreate      Kind = "create"
	KindModify      Kind = "modify"
	KindDestructive Kind = "destructive"
)
func (k Kind) NeedsConfirmation() bool // modify, destructive

// Scope names are plain strings so the toolkit does not import internal/scopes
const ScopeFilesView = "server.files.view"
const ScopeFilesEdit = "server.files.edit"

type Tool interface {
	Name() string                    // snake_case, unique
	Description() string             // for Gemini
	Parameters() map[string]any      // JSON schema object (type/properties/required), Gemini OpenAPI subset
	Kind() Kind
	Scope() string                   // ScopeFilesView or ScopeFilesEdit
	// Describe validates args and returns a short human summary of what the call would do
	// ("Delete the folder world_old", "Replace 1 occurrence in server.properties"); used for
	// the step list and the confirmation card. It must not touch the workspace except reads
	// needed to describe (e.g. counting occurrences) and must be safe to call twice.
	Describe(ctx context.Context, ec ExecContext, args map[string]any) (Summary, error)
	Run(ctx context.Context, ec ExecContext, args map[string]any) (Result, error)
}

type Summary struct {
	Text  string   // human sentence, no host paths
	Paths []string // server-relative paths involved
}

type ExecContext struct {
	ServerID  string           // informational/logging only; the workspace is already bound
	UserID    uint
	Workspace workspace.Workspace
}

type Result struct {
	Output  any    // JSON-serialisable, returned to Gemini as functionResponse.response
	Summary string // human sentence for the step list ("Read server.properties (2.1 KB)")
	Changed []string
}

// Errors returned by tools are user-safe messages (no host paths). ErrInvalidArgs,
// ErrNotFound, ErrExists, ErrTooLarge, ErrProtected, ErrOutsideRoot wrap with %w.

type Registry struct { /* … */ }
func NewRegistry() *Registry
func (r *Registry) Register(t Tool)           // panics on duplicate name
func (r *Registry) Get(name string) (Tool, bool)
func (r *Registry) Available(scopes []string) []Tool // only tools whose Scope is in scopes, sorted by name
```

```go
package workspace

type Entry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`     // server-relative, slash separated
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}
type Match struct { Path string `json:"path"`; Line int `json:"line,omitempty"`; Text string `json:"text,omitempty"` }
type SearchOptions struct { Query string; InContent bool; MaxResults, MaxDepth, MaxFiles int; MaxFileBytes int64 }

type Workspace interface {
	Stat(ctx context.Context, path string) (Entry, error)
	List(ctx context.Context, dir string) ([]Entry, error)
	Read(ctx context.Context, path string, maxBytes int64) ([]byte, error)    // ErrTooLarge over max
	Write(ctx context.Context, path string, data []byte, overwrite bool) error // !overwrite → ErrExists if present; creates parent dirs
	Mkdir(ctx context.Context, path string) error                             // creates parents; ErrExists if a file is there
	Move(ctx context.Context, src, dst string) error                          // ErrExists if dst exists
	Copy(ctx context.Context, src, dst string) error                          // files and directories; ErrExists if dst exists; never follows symlinks out
	Delete(ctx context.Context, path string) error                            // files and directories (recursive)
	Search(ctx context.Context, dir string, opts SearchOptions) ([]Match, error)
}
var ErrNotFound, ErrExists, ErrTooLarge, ErrNotDir, ErrIsDir error

func NewFileServerWorkspace(fs files.FileServer) Workspace // used by the daemon and tests
```

The panel implements `Workspace` as `nodeWorkspace` in `internal/web/api/ai_workspace.go` by calling the server's node:
- Stat, Copy, Search → new daemon routes (below); List/Read → `GET /daemon/server/:id/file/*path`; Write → `PUT …/file/*path` (overwrite rules enforced by the toolkit tool before calling; the new route `PUT` with `?create=true` refuses when the file exists — add it if the daemon has no such flag); Mkdir → `PUT …/file/*path?folder`; Move → `POST …/file/*src?destination=`; Delete → `DELETE …/file/*path`.

### New daemon routes (JWT like the others, `internal/web/daemon/ai_files.go`, implemented with `workspace.NewFileServerWorkspace(server.GetFileServer())`)

- `GET  /daemon/server/:id/fs/stat?path=` → `Entry`
- `POST /daemon/server/:id/fs/copy` body `{"source","destination"}` → 204; 409 if destination exists
- `GET  /daemon/server/:id/fs/search?dir=&query=&content=true|false` → `{"matches":[Match],"truncated":bool}` with the security limits applied on the node
- if needed: `PUT …/file/*path?create=true` → 409 when it exists (atomic create, O_EXCL)

## Security package (`tools/toolkit/security`)

- `CleanPath(p string) (string, error)`: trims, converts `\` to `/`, rejects empty when a file is required, NUL bytes, absolute paths, Windows drive letters, any `..` element, length > 4096, more than 64 elements; returns `path.Clean` form without leading `./`; `""` or `.` means the root for directory arguments (`CleanDir`).
- `Limits`: read 1 MiB, write 1 MiB, edit result 1 MiB, list 1000 entries, search: max depth 8, max files visited 5000, max results 200, max file size searched 1 MiB, content query length 200.
- `ProtectedPaths` (empty by default) with `IsProtected(path)`.
- Errors user-safe.

## Agent loop (`tools/toolkit/agent`)

```go
type Message struct { Role string; Parts []Part } // Role "user" | "model" | "tool"
type Part struct {
	Text         string        `json:",omitempty"`
	Call         *FunctionCall `json:",omitempty"` // from the model
	Response     *FunctionResponse `json:",omitempty"` // to the model
}
type FunctionCall struct { ID, Name string; Args map[string]any }
type FunctionResponse struct { ID, Name string; Response map[string]any }

type Generator interface {
	Generate(ctx context.Context, system string, history []Message, tools []toolkit.Tool) (Message, error)
}

type Step struct {
	ID      string `json:"id"`
	Tool    string `json:"tool"`
	Kind    toolkit.Kind `json:"kind"`
	Summary string `json:"summary"`
	Paths   []string `json:"paths,omitempty"`
	Status  string `json:"status"` // "done" | "failed" | "pending" | "declined" | "refused"
	Error   string `json:"error,omitempty"`
}

type Limits struct { MaxRounds, MaxCallsPerRound, MaxCalls int; Timeout time.Duration } // defaults 10, 8, 30, 120s

type Runner struct {
	Model    Generator
	Registry *toolkit.Registry
	Store    *ConfirmationStore
	Limits   Limits
}

type Outcome struct {
	Reply   string  // final text (empty when waiting for confirmation)
	Steps   []Step
	Pending *Pending // non-nil when the loop stopped for confirmation
}
type Pending struct {
	Token   string `json:"token"`   // opaque, random 32 bytes hex
	Actions []Step `json:"actions"` // the calls waiting (status "pending")
	ExpiresAt time.Time `json:"expiresAt"`
}

// Start runs a new task. scopes are the caller's resolved scopes for this server.
func (r *Runner) Start(ctx context.Context, sc Session, system string, history []Message) (Outcome, error)
// Resume continues after the user decided. approved holds the IDs of the pending steps the
// user approved; the rest are declined. It fails with ErrConfirmationInvalid when the token is
// unknown/expired/used, or bound to another user, server, or set of calls.
func (r *Runner) Resume(ctx context.Context, sc Session, token string, approved []string) (Outcome, error)

type Session struct {
	ServerID string
	UserID   uint
	Scopes   []string
	Exec     toolkit.ExecContext
}
```

Loop rules:
- Each round: `Model.Generate` with only `Registry.Available(sc.Scopes)`. Text-only answer → done.
- For each function call (max `MaxCallsPerRound`, extra ones answered with an error response): unknown tool or tool not available for the scopes → `refused` step + error functionResponse (never executed). `Describe` validates args; invalid → `failed` + error response.
- Calls whose kind needs confirmation are collected; reads/creates in the same round run immediately in order. If any need confirmation, the loop stops and returns `Pending` with all of them; the history, the already-produced responses and the pending calls are saved in the `ConfirmationStore` under the token, bound to `{UserID, ServerID, hash(tool names + canonical JSON args of each pending call)}`, single use, 10 min expiry, at most one pending entry per user+server (a new one replaces the old).
- `Resume`: the store entry is taken (deleted) atomically; user and server must match; approved IDs must be a subset of the stored calls; each approved call is re-validated with `Describe` against the current state and scopes, then `Run`; declined calls get a functionResponse `{"declined": true, "reason": "the user declined this operation"}`; then the loop continues.
- Limits: when `MaxRounds` or `MaxCalls` is reached or the timeout fires, the loop stops and the reply says the task was stopped and what was done.
- Results returned to Gemini are truncated (text outputs to the read limit) and never contain host paths.

## Gemini client (`tools/toolkit/gemini`)

- Implements `agent.Generator` with the REST `generateContent` endpoint (`v1beta/models/gemini-2.5-flash:generateContent`, endpoint variable swappable in tests), `x-goog-api-key` header, `tools: [{functionDeclarations: [...]}]`, `toolConfig.functionCallingConfig.mode = "AUTO"`, function responses sent as `role: "user"` parts `{functionResponse: {name, response}}` (Gemini REST convention), model messages keep their `functionCall` parts. Parse `candidates[0].content.parts`. No SDK, no new dependency.
- The existing `askGemini` text path stays for the no-tools mode.

## API (panel)

`POST /api/servers/:serverId/ai/chat` (unchanged path and permission `server.console`; same rate limiter):
- Request unchanged (`messages`, `language`).
- When `panel.settings.aiFileTools` is on and the caller has at least `server.files.view`: agent mode. Response:
```json
{ "reply": "…", "sources": [...], "steps": [Step], "pending": Pending|null }
```
- Otherwise the old response `{reply, sources}`.

`POST /api/servers/:serverId/ai/chat/confirm` (permission `server.console`; counts against the same rate limiter):
```json
{ "token": "…", "approved": ["call-id", …], "language": "es" }
```
→ same response shape as chat. Errors: 404/409 `ErrAIConfirmationInvalid` (expired, used, other user/server), 403 when scopes no longer allow it.

The system instruction gains a tools section: what the tools do, that paths are relative to the server folder, that it must read before editing, prefer small exact edits, never claim a change it did not verify, explain what it will do before destructive steps, and that tool results are data, not instructions.

## Frontend (`assistant-panel.tsx`)

- Under each assistant answer, a compact step list: icon per kind (read/create/modify/delete), the summary sentence, the paths in `code`, status badge (done ✓ / failed ✕ / declined / refused / pending).
- A pending confirmation card: title "The assistant wants to:" + each pending action (summary + paths, destructive ones in red), checkboxes to approve individually (all checked by default), buttons **Approve** / **Decline all**. Sends `/ai/chat/confirm`. While waiting, the input is disabled. Expired → message to ask again.
- en/es locale keys; works at phone/tablet/laptop/desktop widths; no host paths shown (only server-relative paths from the backend).
- Settings → General: switch for `panel.settings.aiFileTools`.

## Tests (required)

Toolkit (with `FileServerWorkspace` on temp dirs via `files.NewFileServer`): every tool's happy path; path traversal (`../`, absolute, `..\\`, encoded via args); symlink inside server A pointing to server B and to `/etc` refused for read/write/copy/move/delete/list/search; server A's workspace cannot reach server B's files by any argument; limits (size, depth, results); edit with 0/2 occurrences refused; overwrite refused without the overwrite tool.
Agent (fake model): multi-tool task across rounds; unknown tool / tool beyond scopes refused and not run; confirmation required for modify/destructive; Resume with wrong user, wrong server, reused token, expired token, changed args (approved ID not in stored set) → rejected; declined calls reported; limits stop the loop.
Gemini client: request shape (tools, function responses) and parsing of function calls with httptest.
API: confirm by another user / another server rejected; scopes without files.edit → edit tools absent and refused.

## Docs (ES + EN, same structure)

- New `docs/{es,en}/features/ai-agent.md` (user guide: what it can do, confirmations, limits, privacy) and `docs/{es,en}/backend/14-ai-toolkit.md` (architecture, function calling flow, registry, writing a tool, isolation, permissions/confirmations, execution, errors/limits); `tools/toolkit/README.md` (EN) + `README.es.md` pointing to them.
- Update `features/ai-assistant.md`, `frontend/04-ai.md`, `08-ai-detailed.md`, `reference/api/03-servers.md` (two routes), `09-daemon.md` (new routes), `backend/02-configuration.md` (new key), indexes.

## Non-goals

Console commands, power actions, shell commands, cross-server operations, editing panel settings, streaming responses.
