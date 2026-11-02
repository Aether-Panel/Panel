# AI agent toolkit

[Español](README.es.md)

The toolkit lets the AI assistant of a server inspect and change **that server's files** through Gemini function calling. Every tool works on a workspace bound to the server of the conversation: no tool takes a server argument, and every path is relative to the server folder.

Architecture, the function-calling flow, permissions and confirmations are described in [docs/en/backend/14-ai-toolkit.md](../../docs/en/backend/14-ai-toolkit.md).

## Layout

| Path | What it holds |
|------|---------------|
| `toolkit.go` | `Tool`, `Kind` (read, create, modify, destructive), `Summary`, `ExecContext`, `Result`, scope names, shared errors |
| `registry.go` | `Registry`: `Register` (panics on a duplicate name), `Get`, `Available(scopes)` |
| `security/` | `CleanPath` / `CleanDir`, the size and search `Limits`, `ProtectedPaths`, `LooksBinary` |
| `workspace/` | the `Workspace` interface and `NewFileServerWorkspace`, built on the node's confined `files.FileServer` |
| `filesystem/` | the 12 file tools and `Register(r)` |
| `agent/` | the agent loop: rounds, confirmations, limits |
| `gemini/` | the Gemini function-calling client |
| `internal/resolvemode/` | test helper that runs tests with and without openat2 |

## The file tools

| Tool | Kind | Scope | Confirmation |
|------|------|-------|--------------|
| `list_directory`, `read_file`, `file_info`, `search_files` | read | `server.files.view` | no |
| `create_file`, `create_directory`, `copy_path` | create | `server.files.edit` | no (never replace anything) |
| `edit_file`, `write_file`, `move_path`, `rename_path` | modify | `server.files.edit` | yes |
| `delete_path` | destructive | `server.files.edit` | yes |

## Adding a tool

1. Write a type that implements `toolkit.Tool`. In `filesystem/`, embed `base` to get `Name`, `Description`, `Kind`, `Scope` and `Parameters`.
2. Validate the arguments in **both** `Describe` and `Run` (Run may be called alone, or after the file changed). Paths go through `pathArg` / `dirArg`, which use `security.CleanPath` / `CleanDir`.
3. Touch files only through `ec.Workspace`. Return errors that wrap a `toolkit.Err…` value and never contain a host path.
4. Register it in `Tools()` and add tests (happy path, invalid arguments, isolation).

A minimal read tool:

```go
type countLines struct{ base }

func newCountLines() countLines {
	return countLines{base{
		name:        "count_lines",
		description: "Count the lines of a text file of this server.",
		kind:        toolkit.KindRead,
		scope:       toolkit.ScopeFilesView,
		params: []param{
			{"path", "string", "File to count. " + pathRules, true},
		},
	}}
}

func (t countLines) Describe(_ context.Context, _ toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	p, err := pathArg(args, "path")
	if err != nil {
		return toolkit.Summary{}, err
	}
	return toolkit.Summary{Text: "Count the lines of " + p, Paths: []string{p}}, nil
}

func (t countLines) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	p, err := pathArg(args, "path")
	if err != nil {
		return toolkit.Result{}, err
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return toolkit.Result{}, err
	}
	data, err := readText(ctx, ws, p, security.Limits.MaxReadBytes)
	if err != nil {
		return toolkit.Result{}, err
	}
	n := strings.Count(string(data), "\n")
	return toolkit.Result{Output: map[string]any{"path": p, "lines": n}, Summary: fmt.Sprintf("Counted %d lines in %s", n, p)}, nil
}
```

## Security rules

- **One server per workspace.** The server comes from the authenticated route, never from the model. A workspace cannot name another server.
- **Paths are checked twice.** `security.CleanPath` refuses absolute paths, drive letters, any `..` element, NUL bytes, more than 4096 characters and more than 64 folders. The node's file server then opens every path beneath the server folder (openat2 with `RESOLVE_BENEATH`, or one folder at a time without following links), so a symlink that leaves the folder is refused by the kernel.
- **Links are not followed where it matters.** Writes refuse a symlink as target (`O_NOFOLLOW`), creates are atomic (`O_EXCL`), copies skip symlinks, search never walks into them, and deleting a link removes only the link.
- **Limits.** Read, write and edit up to 1 MiB; list 1000 entries; search 8 folders deep, 5000 entries, 200 matches, files up to 1 MiB, queries up to 200 characters. Binary files (NUL in the first 8 KB) are not read, edited or searched.
- **Confirmation.** `modify` and `destructive` calls wait for the user; tools the user has no scope for are not offered to the model.
- **No commands.** No tool runs shell or console commands.
- **User-safe errors.** Errors and results carry server-relative paths only, never host paths.

## Tests

```sh
go test ./tools/toolkit/...
```

The isolation tests give server A's tools every path trick (traversal, absolute paths, backslashes, links to server B and to a host folder) and check that nothing of server B or the host is read, listed, found, copied, changed, moved or deleted, in both resolve modes.
