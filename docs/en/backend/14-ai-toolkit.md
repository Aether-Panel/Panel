# AI Agent Toolkit

How the server assistant works with a server's files: the toolkit in `tools/toolkit/`, the agent loop that drives Gemini function calling, how tools are registered and offered, how every call is confined to the server of the conversation, how confirmations work, and what is recorded.

Related pages: the user guide [AI assistant: working with files](../features/ai-agent.md); the chat itself, its context and the log analysis in [AI assistant](../features/ai-assistant.md) and [AI in detail](../frontend/08-ai-detailed.md); the routes in [Servers → AI](../reference/api/03-servers.md#ai) and [Daemon → Files](../reference/api/09-daemon.md#files); the file server's confinement in [Security → File access](./09-security.md#file-access). The package overview for developers is [`tools/toolkit/README.md`](../../../tools/toolkit/README.md).

---

## Overview

| Package | Path | Role |
|---|---|---|
| `toolkit` | `tools/toolkit/toolkit.go`, `registry.go` | The `Tool` interface, the kinds (`read`, `create`, `modify`, `destructive`), `Summary`, `ExecContext`, `Result`, the scope names and the `Registry` |
| `security` | `tools/toolkit/security/` | Path cleaning (`CleanPath`, `CleanDir`), the size and search `Limits`, the `ProtectedPaths` hook, `LooksBinary` |
| `workspace` | `tools/toolkit/workspace/` | The `Workspace` interface (one server's files) and `NewFileServerWorkspace`, built on the node's confined `files.FileServer` |
| `filesystem` | `tools/toolkit/filesystem/` | The 12 file tools and `Register(r)` |
| `agent` | `tools/toolkit/agent/` | The loop: `Runner`, `Session`, `Step`, `Limits`, `ConfirmationStore` |
| `gemini` | `tools/toolkit/gemini/` | A Gemini REST client with function calling that implements `agent.Generator` (no SDK) |
| Panel glue | `internal/web/api/ai_agent.go`, `ai_workspace.go`, `ai_chat.go` | Agent mode of `POST /ai/chat`, `POST /ai/chat/confirm`, the `nodeWorkspace` that reaches the server through its node, the activity entries |
| Node glue | `internal/web/daemon/ai_files.go`, `server.go` | The routes `fs/stat`, `fs/copy`, `fs/search`, `PUT file?create=true` and the hardened `DELETE file` |

The toolkit does not import `internal/web` and knows nothing of Gemini outside the `gemini` package. Scope names are plain strings (`server.files.view`, `server.files.edit`), so it does not import `internal/scopes` either.

## Request flow

```
Browser (assistant-panel.tsx)
   │  POST /api/servers/:serverId/ai/chat           { messages, language }
   │  POST /api/servers/:serverId/ai/chat/confirm   { token, approved, language }
   ▼
Panel  ── server.console, ResolveServerPanel, rate limit (30 / 10 min per user)
   │      agent mode? panel.settings.aiFileTools on + signed-in user + server.files.view
   │      agent.Runner.Start / Resume
   │         ├─ gemini.Client.Generate ──► generativelanguage.googleapis.com (gemini-2.5-flash)
   │         └─ Tool.Describe / Tool.Run ──► nodeWorkspace (bound to the route's server)
   ▼
Node   /daemon/server/<id>/file/*, /fs/stat, /fs/copy, /fs/search   (panel token)
   │      workspace.NewFileServerWorkspace(server.GetFileServer()) or the file manager's handlers
   ▼
files.FileServer: openat2 + RESOLVE_BENEATH under the server folder
```

1. `chatWithAssistant` (`ai_chat.go`) checks the Gemini key, validates the conversation (1 to 20 messages of 1 to 2000 characters, the last from the user), applies the shared AI rate limit and gathers the server context (status, players, performance, crashes, console) exactly as in plain chat.
2. `aiAgentScopes` decides the mode. Agent mode needs all of: `panel.settings.aiFileTools` on (default `true`), a signed-in user on the request (an OAuth2 client without a user gets plain chat), and `server.files.view` on the server. Otherwise the request goes to the plain text chat (`askGemini`) and answers `{reply, sources}` as before.
3. In agent mode the panel builds an `agent.Session` from the request only: `ServerID` is the identifier of the server the route resolved, `UserID` the signed-in user, `Scopes` the caller's file scopes on that server, `Exec.Workspace` a `nodeWorkspace` for that server, `Language` the request's language.
4. `Runner.Start` runs the loop (below) with the system instruction (`agentInstruction`: the plain assistant instruction plus the tool rules) and the conversation as text messages.
5. The answer is `{reply, sources, steps, pending}`. Changes that ran are written to the server's activity log first.

## The agent loop

`agent.Runner` (`tools/toolkit/agent/runner.go`) holds a `Model` (an `agent.Generator`), a `Registry`, a `ConfirmationStore` and `Limits`. The panel keeps one registry and one confirmation store per process (`aiRunner`, created once with `sync.Once`) and gives each request a copy of the runner with its own `gemini.Client`, because the API key is read on every request and an administrator can change it at any time.

### Rounds

Each round:

1. If the task's context is done (time limit) or `MaxRounds` / `MaxCalls` is reached, the loop stops (see [Limits of a task](#limits-of-a-task)).
2. `Model.Generate(ctx, system, history, tools)` is called with **only** `Registry.Available(session.Scopes)`.
3. The answer is stored in the history as a `model` message. Every function call gets an ID: the model's own when it sent a unique one, otherwise a generated `local-<round>-<n>` (`GeneratedIDPrefix`). The arguments are deep-copied through JSON so a tool cannot change what is stored.
4. An answer with no function call ends the task: its text is the reply.
5. Otherwise the calls of the round are handled in order:
   - beyond `MaxCallsPerRound`: `refused` step and an error response (never run);
   - once `MaxCalls` is reached: `refused` step and an error response;
   - a name that is not among the offered tools: `refused` step and the error response `unknown tool "…"` or `the tool "…" is not allowed for this user` (never run);
   - otherwise `Describe` validates the arguments; an error gives a `failed` step;
   - a call whose kind needs confirmation (`modify`, `destructive`) becomes `pending` and is not run;
   - any other call (`read`, `create`) runs at once with `Run`: `done` or `failed`.
6. If no call is pending, the responses go back to the model as one `tool` message and the next round starts. If any call is pending, the loop stops and returns them (see [Confirmations](#confirmations)); the responses of the calls that already ran in that round are kept for later.

A panic in `Describe` or `Run` is caught and becomes a `failed` step (`the operation failed unexpectedly`).

### Function calls and responses

`gemini.Client` (`tools/toolkit/gemini/client.go`) posts to `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent` (`DefaultEndpoint`, replaceable in tests) with the key in the `x-goog-api-key` header:

- `systemInstruction`: the system text;
- `contents`: the history. `user` and `model` messages keep their roles; the model's function calls stay as `functionCall` parts; the answers to them (role `tool` in the agent) are sent as role `user` with `functionResponse` parts `{id, name, response}`, in call order. IDs the runner generated (`local-…`) are not sent;
- `tools: [{functionDeclarations: [{name, description, parameters}]}]` built from the offered tools, and `toolConfig.functionCallingConfig.mode = "AUTO"`;
- `generationConfig`: temperature 0.4, `maxOutputTokens` (8192 for the agent, 4096 for the plain chat) and `thinkingConfig.thinkingBudget` (1024 for the agent, 512 for the plain chat). Gemini 2.5 counts its thinking inside the output tokens, so without that cap thinking could leave no room for the answer or for a `write_file` with a whole file;
- the model's thought signatures (`thoughtSignature`) go back unchanged on the parts that carried them (`agent.Part.Signature`), so the model keeps its reasoning between rounds.

The client reads `candidates[0].content.parts` (at most 4 MiB of answer): `functionCall` parts become calls, text parts become text, and thinking summaries (`thought`) are dropped. Finish reasons `SAFETY`, `RECITATION`, `BLOCKLIST`, `PROHIBITED_CONTENT`, `SPII` and `IMAGE_SAFETY` give `ErrBlocked`; `MALFORMED_FUNCTION_CALL` and `UNEXPECTED_TOOL_CALL` give `ErrMalformedCall`. A candidate with no text and no call gives `ErrEmpty`, or `ErrTooLong` when its finish reason is `MAX_TOKENS`. Without the key it fails with `ErrNotConfigured`; a transport failure gives `ErrUnreachable`. A status other than 200 gives a `*gemini.ServiceError` (`CheckResponse`), whose `Kind` is:

| Kind | When |
|---|---|
| `ErrKeyRejected` | `401`, `403`, a `API_KEY_*` reason, or a message about the API key (an invalid, restricted or revoked key, or the API not enabled) |
| `ErrQuota` | `429` (`RESOURCE_EXHAUSTED`) |
| `ErrUnavailable` | `500` and above (overloaded or down) |
| `ErrRejected` | any other status (for example `400` for a request Gemini does not accept, `404` for an unknown model) |

Its `Error()` is the kind's safe text; `Detail` holds Google's status, reason and up to 300 characters of its message, for the panel's log only, and `RetryAfter` the `retryDelay` Google asked for.

`Generate` makes up to three attempts. It tries again after 1 s and then 3 s (`Client.Backoff`) when the service is busy (`ErrUnavailable`) or unreachable, and after a `429` only when Google asks to wait 10 s or less (a per-minute limit); a daily limit is reported at once. The wait ends early when the task's context ends. The HTTP client has a 90 s timeout as a backstop.

What goes back to the model as a function response:

- a tool error: `{"error": "<message>"}`;
- a result: the tool's `Output` as a JSON object (a non-object is wrapped as `{"result": …}`; no output gives `{"ok": true, "summary": "…"}`);
- a declined call: `{"declined": true, "reason": "the user declined this operation"}`.

Every string in a response, and every step error, is cut to 64 KiB (`maxOutputString`) with `[truncated]` appended.

### Limits of a task

`agent.DefaultLimits()` (a zero field in `Runner.Limits` takes the default; the panel uses the defaults):

| Limit | Default | When reached |
|---|---|---|
| `MaxRounds` | 10 | The loop stops and the reply is written by the runner |
| `MaxCallsPerRound` | 8 | Extra calls of that answer are `refused` |
| `MaxCalls` | 30 | Further calls are `refused`; the next round stops the task |
| `Timeout` | 120 s | Per `Start` or `Resume` call; the task stops |

A stop because of rounds, calls or time does not fail the request: the reply says why (`I stopped this task because it needed too many steps (limit: 10 rounds).`, `… too many operations (limit: 30).`, `… it took too long.`) and lists the steps done so far, or says nothing was changed. These texts are translated by the runner (`messages.go`, English and Spanish). A request cancelled by the client ends with the context error instead.

## The registry and scopes

`toolkit.Registry` (`registry.go`) maps names to tools. It is filled once at start-up and read concurrently:

- `Register(t)` panics on a nil tool, an empty name or a duplicate name: two tools with one name would make a call ambiguous, so it is a programming error.
- `Get(name)` returns a tool.
- `Available(scopes)` returns the tools whose `Scope()` is in `scopes`, sorted by name.

The panel maps the caller's resolved scopes on the server (`callerScopes`, already bounded by an OAuth2 client's) to the toolkit's names: `server.files.view` → `toolkit.ScopeFilesView`, `server.files.edit` → `toolkit.ScopeFilesEdit`. `scopes.ContainsScope` is used, so administrators and holders of `server.admin` count as having both. They are read on every request, so a permission taken away applies to the next chat or confirmation.

A tool the user has no scope for is **not declared to Gemini at all**; if the model calls it anyway, the call is refused and never run.

### The file tools

`filesystem.Register(reg)` adds all 12. The node route is the one the panel's `nodeWorkspace` calls on the server's node (`/daemon/server/<id>/…`).

| Tool | Kind | Scope | What it does | Node routes |
|---|---|---|---|---|
| `list_directory` | read | `server.files.view` | Lists a folder, folders first; at most 1000 entries (`truncated` says if there were more) | `GET file/<dir>` |
| `read_file` | read | `server.files.view` | Returns a text file exactly as stored, with `size`, `totalLines`; `offset` / `limit` choose a range of lines | `GET file/<path>` |
| `file_info` | read | `server.files.view` | Type, size and modification time, without reading | `GET fs/stat` |
| `search_files` | read | `server.files.view` | Names containing `query`, or with `in_content` lines of text files; case-insensitive | `GET fs/search` |
| `create_file` | create | `server.files.edit` | New text file; fails if anything exists there; makes missing parent folders | `GET fs/stat`, `PUT file/<path>?create=true` |
| `create_directory` | create | `server.files.edit` | Folder and parents; succeeds unchanged if it exists; fails if a file is in the way | `GET fs/stat`, `PUT file/<path>?folder` |
| `copy_path` | create | `server.files.edit` | Copies a file or folder tree to a path that must not exist; symlinks inside are skipped | `GET fs/stat`, `POST fs/copy` |
| `edit_file` | modify | `server.files.edit` | Replaces `old_text` with `new_text`; `old_text` must appear exactly `expected_occurrences` times (default 1) | `GET file/<path>`, `PUT file/<path>` |
| `write_file` | modify | `server.files.edit` | Replaces the whole content of an existing file | `GET fs/stat`, `PUT file/<path>` |
| `move_path` | modify | `server.files.edit` | Moves (and may rename) to a path that must not exist; makes missing parents | `GET fs/stat`, `PUT file/<parent>?folder`, `POST file/<src>?destination=<dst>` |
| `rename_path` | modify | `server.files.edit` | New name in the same folder (`new_name` is a name, not a path) | as `move_path` |
| `delete_path` | destructive | `server.files.edit` | Deletes a file, or a folder with everything in it; the server folder itself is refused | `GET fs/stat`, `DELETE file/<path>` |

Details worth knowing:

- `read_file` returns the text with its line endings, so it can be used as `old_text`. Binary files (a NUL byte in the first 8 KiB) are refused, as are files over 1 MiB.
- `edit_file` re-reads the file in `Run`, so it applies to the file as it is then. When `old_text` uses `\n` but the file uses `\r\n`, it matches the Windows line endings. 0 matches, a count different from `expected_occurrences`, an empty `old_text`, `new_text` equal to `old_text`, or a result over 1 MiB are refused.
- `create_file`, `write_file`: content up to 1 MiB.
- Copy and move refuse the same source and destination and a destination inside the source.

## Writing a tool

A tool implements `toolkit.Tool`:

```go
type Tool interface {
	Name() string                // snake_case, unique in a Registry
	Description() string         // for the model
	Parameters() map[string]any  // JSON schema object: type, properties, required
	Kind() Kind                  // KindRead, KindCreate, KindModify or KindDestructive
	Scope() string               // ScopeFilesView or ScopeFilesEdit
	Describe(ctx context.Context, ec ExecContext, args map[string]any) (Summary, error)
	Run(ctx context.Context, ec ExecContext, args map[string]any) (Result, error)
}
```

Rules the loop relies on:

- **Validate in both `Describe` and `Run`.** `Describe` runs before a call is shown or run and again when a confirmed call is resumed; `Run` may get arguments that are valid but describe a file that changed since.
- **`Describe` must not change anything.** It may read (for example, to count occurrences) and must be safe to call twice. `Summary.Text` is the sentence shown in the step list and the confirmation card; `Summary.Paths` are the server-relative paths involved.
- **Touch files only through `ec.Workspace`**, which is already bound to the server. Never take a server argument.
- **Clean every path** with `security.CleanPath` (a file or folder inside the server folder) or `security.CleanDir` (where `""` or `.` means the server folder), and refuse protected paths (`security.IsProtected` for reads, `security.TouchesProtected` for changes).
- **Errors are shown to the user and the model**: wrap a `toolkit.Err…` value with `%w` and never put a host path in them. The workspace errors already follow this rule.
- **Pick the kind honestly.** `create` must never replace anything; anything that changes or removes existing data is `modify` or `destructive`, which is what makes the user confirm it.

A minimal read tool, written outside the `filesystem` package:

```go
package mytools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
)

// CountLines counts the lines of a text file of the server of the conversation
type CountLines struct{}

func (CountLines) Name() string        { return "count_lines" }
func (CountLines) Description() string { return "Count the lines of a text file of this server." }
func (CountLines) Kind() toolkit.Kind  { return toolkit.KindRead }
func (CountLines) Scope() string       { return toolkit.ScopeFilesView }

func (CountLines) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File relative to the server folder, for example server.properties."},
		},
		"required": []string{"path"},
	}
}

func (CountLines) path(args map[string]any) (string, error) {
	raw, _ := args["path"].(string)
	p, err := security.CleanPath(raw)
	if err != nil {
		return "", fmt.Errorf("%w: path: %w", toolkit.ErrInvalidArgs, err)
	}
	if security.IsProtected(p) {
		return "", fmt.Errorf("%w: %s", toolkit.ErrProtected, p)
	}
	return p, nil
}

func (t CountLines) Describe(_ context.Context, _ toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	p, err := t.path(args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	return toolkit.Summary{Text: "Count the lines of " + p, Paths: []string{p}}, nil
}

func (t CountLines) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	p, err := t.path(args)
	if err != nil {
		return toolkit.Result{}, err
	}
	if ec.Workspace == nil {
		return toolkit.Result{}, errors.New("the files of this server are not available")
	}
	data, err := ec.Workspace.Read(ctx, p, security.Limits.MaxReadBytes)
	if err != nil {
		return toolkit.Result{}, err
	}
	n := strings.Count(string(data), "\n")
	return toolkit.Result{
		Output:  map[string]any{"path": p, "lines": n},
		Summary: fmt.Sprintf("Counted %d lines in %s", n, p),
	}, nil
}
```

Registering it:

- **A file tool** belongs in `tools/toolkit/filesystem`: add its constructor to `Tools()`. There the `base` type supplies `Name`, `Description`, `Kind`, `Scope` and `Parameters` from a list of `param`s, and the helpers `pathArg`, `dirArg`, `readText`, `checkReadable`, `checkChangeable` and `mustNotExist` do the common checks. [`tools/toolkit/README.md`](../../../tools/toolkit/README.md) has the same example written that way.
- **Any other tool** is registered where the panel fills the registry, in `aiRunner` (`internal/web/api/ai_agent.go`): `registry.Register(mytools.CountLines{})` next to `filesystem.Register(registry)`.

A tool of kind `create`, `modify` or `destructive` is recorded in the activity log as `ai.<name>`: add the phrase under `servers.activity.actions.ai.<name>` in `client/frontend/src/lib/locales/en.json` and `es.json` (an unknown action shows a generic phrase). Add tests for the happy path, invalid arguments and isolation (the existing ones use `NewFileServerWorkspace` on temporary folders and run in both resolve modes with `internal/resolvemode`).

## Isolation

### The server comes from the route

The server is the one `ResolveServerPanel` resolved for `/api/servers/:serverId/ai/…`, after `server.console` was checked on it. `aiSession` builds the workspace for that server (`newNodeWorkspace(nodeService, server)`), whose URL prefix is `/daemon/server/<escaped identifier>` on that server's node. No tool has a server parameter and nothing the model writes can change the prefix: every path is a server-relative path appended to it.

### Paths

Every path is checked by `security` before anything is called, in the tool, again in `nodeWorkspace`, and a third time on the node by the workspace of the `fs/*` routes (the file manager's routes rely on the file server alone):

- `CleanPath` / `CleanDir` trim spaces, turn `\` into `/`, and refuse: a NUL byte, more than 4096 bytes, a Windows drive letter (`C:`), a leading `/`, any `..` element, more than 64 elements. They return the `path.Clean` form without a leading `./`.
- `CleanPath` refuses the server folder itself (`""`, `.`), so a tool that needs a file or folder (`delete_path`, `create_file`, …) can never name the root. `delete_path` says so explicitly: `the server folder itself cannot be deleted`.
- `nodeWorkspace.filePath` escapes each element with `url.PathEscape`, so `#`, `?` or `%` in a name stay part of the name.

### On the node: the confined file server

On the node every operation goes through the server's `files.FileServer`, which opens each path beneath the server folder with `openat2` and `RESOLVE_BENEATH` (or, with `security.forceOpenat`, one folder at a time with `O_NOFOLLOW`): a path or a symlink that would leave the folder is refused by the kernel, not by string checks. See [Security → File access](./09-security.md#file-access).

The panel's internal files of a server (`.cron`, `.history`, `.crashes`, `.notify`, `.metrics`, `.worldmap`, `.modpack`) live next to the server folder, outside its root, so no tool can reach them. A path that leaves the root comes back as `the path must stay inside the server folder`.

### Symlinks

- Reading and listing never follow a link out of the server folder: a link that leaves it, or is broken, is listed as itself and cannot be opened. `fs/stat` describes a link that stays inside by what it points at.
- `Write` refuses an existing symlink as target, and opens with `O_NOFOLLOW` (a link put in place after the check is refused, not written through). A create uses `O_EXCL`, so it never replaces anything.
- `Copy` refuses a symlink as source; inside a copied folder, symlinks, devices, sockets and pipes are skipped.
- Search never walks into a linked folder (a link's own name can match).
- `Delete` of a symlink removes the link itself (`unlinkat` on its folder); what it points at is not touched. The node's `DELETE file` route does the same since this feature: before, deleting a link to a folder emptied that folder and then failed to remove the link.
- `Move` renames the entry itself; a link is moved as a link.

### Protected paths

`security.ProtectedPaths` is a list of server-relative paths that no tool may read or change (`IsProtected` for reads and listings, `TouchesProtected` for changes, which also covers a folder holding a protected path). It is **empty by default**: the panel's own files are already outside the server folder. It is a hook for code (set once at start-up, before any tool runs); no configuration key fills it. Protected entries are left out of listings and search results, and refused with `this path is protected and cannot be used`.

## Permissions and confirmations

### Who may use agent mode

| Requirement | Where it is checked |
|---|---|
| `server.console` on the server | Route middleware of `/ai/chat` and `/ai/chat/confirm` |
| A Gemini key (`panel.settings.geminiApiKey`) | Both handlers (`400` without it) |
| `panel.settings.aiFileTools` on | `aiAgentScopes` (chat: plain mode when off); confirm: `409 ErrAIConfirmationInvalid` when off |
| A signed-in user | Chat: plain mode without one; confirm: `403 ErrNoPermission` |
| `server.files.view` | Chat: plain mode without it; confirm: `403 ErrNoPermission` |
| `server.files.edit` | Per tool: without it the create, modify and delete tools are not offered |

### Kinds

| Kind | Tools | Confirmation |
|---|---|---|
| `read` | `list_directory`, `read_file`, `file_info`, `search_files` | No |
| `create` | `create_file`, `create_directory`, `copy_path` | No: they never replace anything |
| `modify` | `edit_file`, `write_file`, `move_path`, `rename_path` | Yes |
| `destructive` | `delete_path` | Yes |

`Kind.NeedsConfirmation()` is true for `modify` and `destructive`.

### Confirmations

When a round has pending calls, `round` stores a `confirmationEntry` in the `ConfirmationStore` (`tools/toolkit/agent/store.go`) and returns `Outcome.Pending`:

```json
{ "token": "<64 hex characters>", "actions": [ { "id": "…", "tool": "delete_path", "kind": "destructive", "summary": "Delete the folder world_old and everything in it", "paths": ["world_old"], "status": "pending" } ], "expiresAt": "2026-11-02T10:10:00Z" }
```

The entry holds the system text, the history up to the model message with the calls, the responses of the calls that already ran in that round, the pending calls and the round and call counters. The reply of that answer is the model's own text next to the calls (it may be empty).

| Rule | Implementation |
|---|---|
| Token | 32 random bytes (`crypto/rand`), hex |
| Bound to | The user ID and the server identifier, and each pending call is stored with the SHA-256 of its tool name and canonical JSON arguments (`callHash`) |
| Lifetime | 10 minutes (`ConfirmationTTL`) |
| One per user and server | A new pending entry replaces the previous one of the same user on the same server |
| Single use | `take` removes the entry before anything is checked or run |
| Capacity | At most 1000 entries in memory (`MaxPendingConfirmations`); when full, the one expiring first is dropped. Expired entries are swept when a new one is stored |
| Storage | Panel memory only: a panel restart drops every pending confirmation |

### Answering a confirmation

`POST /api/servers/:serverId/ai/chat/confirm` with `{token, approved, language}` (`approved`: at most 64 IDs; the token and each ID at most 128 characters) calls `Runner.Resume`:

1. `take(token, user, server)`: an unknown or expired token, or one bound to another user or server, gives `ErrConfirmationInvalid`. A mismatching user or server **leaves the entry in place**, so someone else holding the token cannot burn the owner's confirmation; an expired one is removed.
2. Each stored call's hash is checked again, and every approved ID must be one of the stored calls; otherwise `ErrConfirmationInvalid`. The entry is already used up at this point, so a bad answer cannot be retried with the same token.
3. Declined calls get the declined response. Approved calls are handled again from the start against the **current** scopes and files: the tool must still be offered for the user's scopes (otherwise `refused`), `Describe` validates again (a file that changed may now fail), and then `Run` executes.
4. The responses go back to the model in their original order and the loop continues, with the same round and call counters and a new 120 s timeout. It can stop again for another confirmation.

The panel answers `ErrConfirmationInvalid` with `409 ErrAIConfirmationInvalid` (message `this request expired, was already answered or belongs to someone else: ask again`). Declining everything is an empty `approved` list.

## Execution and audit

- The panel never touches server files itself. `nodeWorkspace` (`ai_workspace.go`) sends each operation to the server's node with `services.Node.CallNodeContext` and the panel's signed token (in-process for the local node). The table in [The file tools](#the-file-tools) lists the routes.
- Overwrites (`edit_file`, `write_file`) use the file manager's `PUT file` route, so the node keeps the previous text in the [file history](../features/file-history.md#when-a-version-is-recorded) under the same rules as an edit in the File Manager. Creates (`?create=true`), copies, moves and deletes do not record versions.
- Disk limits apply: `PUT file` checks the disk and counts the bytes written; `fs/copy` measures what would be copied and refuses with `ErrDiskLimitExceeded` when it would not fit, then resets the cached disk usage.
- **Activity.** After each `Start` or `Resume`, `recordAgentActivity` writes one entry per `done` step of kind `create`, `modify` or `destructive` to the server's activity log, under the user and their IP: action `ai.<tool>` (`ai.create_file`, `ai.create_directory`, `ai.copy_path`, `ai.edit_file`, `ai.write_file`, `ai.move_path`, `ai.rename_path`, `ai.delete_path`), detail the paths joined with ` → `. Reads are not recorded; they go to the debug log. Steps that ran before a task failed are recorded too. The activity middleware ignores `POST /ai/chat` and `POST /ai/chat/confirm`, so these entries are the only ones. They belong to the `files` family. See [Activity log](../features/activity-and-access-log.md).

## Errors

Tool and workspace errors are user-safe: they carry at most a server-relative path, never a host path. The workspace sentinels (`workspace.ErrNotFound`, `ErrExists`, `ErrTooLarge`, `ErrNotDir`, `ErrIsDir`, `ErrSymlink`, `ErrNotRegular`, `ErrInvalid`, `ErrPermission`, `ErrNoSpace`, `ErrFailed`, and the security package's `ErrInvalidPath`, `ErrOutsideRoot`, `ErrProtected`) are matched with `errors.Is` across both layers.

- On the node, `fs/*` routes answer each sentinel with a status and code (`FileErrorStatus` in `internal/web/daemon/ai_files.go`, listed in [Daemon → Files](../reference/api/09-daemon.md#files)); an unknown error is answered as `ErrFileOperationFailed` without its text and logged.
- On the panel, `answerError` maps the code back (`FileErrorForCode`). For the file manager's routes, which do not send these codes, it goes by status: `404` → not found, `409` → exists, `413` → too large, `400` → outside the server folder, `403` → permission, anything else → failed; their text is not passed on. `ErrDiskLimitExceeded` becomes `ErrNoSpace` with the panel's disk message.
- A node that cannot be reached gives `could not reach the node that hosts this server`; a node that refuses the panel's token gives `the node that hosts this server refused the panel's credentials; an administrator has to check the node` (details in the panel log).
- A failed tool call does not fail the request: the step is `failed` with its error, and the model gets the error to act on.
- A failed task (model error, cancelled request) answers `422 {"error": "…", "reason": "…", "steps": […], "pending": null}`: not `502` or `504`, which a proxy in front of the panel (Cloudflare) replaces with its own page, so the reason would never reach the browser. `classifyAIFailure` (`ai_agent.go`) turns the error into a stable `reason` code (`keyRejected`, `quota`, `unavailable`, `unreachable`, `rejected`, `unreadable`, `empty`, `tooLong`, `blocked`, `malformed`, `notConfigured`, `timeout` or `failed`) and an English text that is safe to show, and writes the cause to the panel log: `The AI service did not accept the assistant's request (status 429 RESOURCE_EXHAUSTED: …)` for a `ServiceError`, the error itself for the rest. The plain chat answers the same `{error, reason}` through the same function.

## Limits

Toolkit limits (`security.Limits`, fixed in code):

| Limit | Value |
|---|---|
| `MaxReadBytes` (largest file `read_file` returns) | 1 MiB |
| `MaxWriteBytes` (largest content of `create_file`, `write_file`) | 1 MiB |
| `MaxEditBytes` (largest file `edit_file` works on, before and after) | 1 MiB |
| `MaxListEntries` | 1000 |
| `MaxSearchDepth` | 8 folders below the start |
| `MaxSearchFiles` (entries looked at) | 5000 |
| `MaxSearchResults` | 200 |
| `MaxSearchFileBytes` (larger files are not searched) | 1 MiB |
| `MaxQueryLength` | 200 characters |
| `MaxMatchText` (characters of a matching line) | 300 |
| `SniffBytes` (checked for a NUL byte to call a file binary) | 8 KiB |
| Path | 4096 bytes, 64 elements |
| `rename_path` `new_name` | 255 bytes, no `/` or `\` |

Agent, model and API limits:

| Limit | Value |
|---|---|
| Rounds / calls per answer / calls per task | 10 / 8 / 30 (`agent.DefaultLimits`) |
| Time per `Start` or `Resume` | 120 s |
| Strings returned to the model | 64 KiB each |
| Gemini answer | 8192 output tokens of which up to 1024 thinking (plain chat: 4096 and 512), temperature 0.4, at most 4 MiB read |
| Gemini attempts | 3 per model call, 1 s and 3 s apart, only while busy or unreachable (or a `429` asking for 10 s or less) |
| Pending confirmations | 10 min, one per user and server, 1000 in memory |
| Confirmation request | 64 approved IDs; token and IDs up to 128 characters |
| Rate limit | 30 AI requests per user every 10 minutes, shared with the plain chat and the log analysis; a chat or a confirmation counts once, however many Gemini calls its task makes |
| Node answers read by the panel | Listings 16 MiB, other JSON 4 MiB, error bodies 64 KiB |

## What the assistant cannot do

- Run anything: there is no tool for shell commands, console commands, power actions or installs. The system instruction says so too.
- Reach another server: the workspace is bound to the route's server, and paths are relative to its folder.
- Delete or replace the server folder itself, or leave it through `..`, absolute paths or symlinks.
- Reach the panel's per-server files (`.cron`, `.history`, …), which live outside the server folder.
- Read or edit binary files or files over 1 MiB.
- Change panel settings, users or permissions.

## Known limitations

- **Pending confirmations live in panel memory.** A panel restart drops them, and so does replacing one with a newer request of the same user on the same server. Answering then gives `409`; the user has to ask again.
- **No cancel route.** Clearing the conversation in the browser only removes the card there; the token stays valid on the panel until it expires or is replaced.
- **The conversation sent back is text only.** The browser sends the previous messages' text; the steps and the file contents the model read in an earlier answer are not part of the next request, so the model reads files again when it needs them.
- **Step summaries and tool errors are in English.** The sentences come from the tools (they also go to the model and the logs); only the runner's stop messages are translated. The chat shows other languages a translated phrase built from the tool name and its paths (`servers.assistant.tools.<tool>` in the locales), and the server folder as `/`.
- **Large files.** Files over 1 MiB cannot be read or edited, and content search skips them, although the read error suggests searching them.
- **Written files are small in practice.** The model's answer is limited to 8192 output tokens, so the content it can write in one call is far below the 1 MiB limit; a larger file ends with `ErrTooLong` and the user is told to ask for it in parts.
- **Not atomic across steps.** A move checks that the destination is free and then moves; a confirmed call re-validates, but the files can still change between checks.
- **Nodes must run the same version.** A node without `fs/stat`, `fs/copy` and `fs/search` answers `404` to them: `file_info`, `search_files` and `copy_path` fail, and the existence checks of the other tools read "not found", so file operations misbehave. Update the nodes with the panel.
- **Requests without a user** (OAuth2 clients) always get the plain chat.

## Tests

| Package | What is covered |
|---|---|
| `tools/toolkit/security` | Path cleaning (traversal, absolute, drive letters, backslashes, NUL, length, depth), protected paths |
| `tools/toolkit/workspace` | `NewFileServerWorkspace` on temporary folders: every operation, no writing through a link, search not following links, search limits, protected paths, a cancelled request |
| `tools/toolkit/filesystem` | Every tool's happy path, invalid arguments, edits with 0 or 2 occurrences, overwrite only through `write_file`, isolation between two servers, in both resolve modes (`internal/resolvemode`) |
| `tools/toolkit/agent` | Multi-round tasks, unknown and out-of-scope tools refused, confirmations, `Resume` with another user or server, a reused or expired token, an unknown approved ID, declined calls, limits |
| `tools/toolkit/gemini` | Request shape (tools, function responses, IDs, thinking budget, thought signatures), parsing, error classification and retries with `httptest` |
| `tools/toolkit` | Registry |
| `internal/web/api` (`ai_agent_test.go`, `ai_workspace_test.go`) | Round trip through a node, confirmation bound to user and server, declined actions, no edit permission, permissions checked again on confirm, plain chat when off, safe model errors, `nodeWorkspace` paths, links and failures |
| `internal/web/daemon` (`ai_files_test.go`) | `fs/stat`, `fs/copy` (with the disk limit), `fs/search`, `PUT ?create=true`, `DELETE` of a symlink and of the server folder |

```sh
go test ./tools/toolkit/... ./internal/web/api/... ./internal/web/daemon/...
```
