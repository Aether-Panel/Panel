# Conditions (CEL)

Templates and tasks can make a step depend on a condition, written in the `if` field. Aether Panel evaluates these conditions with the [Common Expression Language](https://github.com/google/cel-spec) (CEL), through the `google/cel-go` library. CEL is a small expression language: it has no loops, no assignments and no access to files or the network beyond the functions the panel adds.

The operation types themselves (`download`, `javadl`, `writefile`...) and the `${variable}` substitution in their arguments are described in [Operations engine](../backend/06-operations.md).

## Where conditions are evaluated

| Place | Field | What happens |
|-------|-------|--------------|
| Install steps | `install[].if` | The step runs only if the condition is true. |
| Uninstall steps | `uninstall[].if` | Same. |
| Pre- and post-start steps | `run.pre[].if`, `run.post[].if` | Same. |
| Start command | `run.command[].if` | When `run.command` is a list, the first entry whose condition is true is used. If none matches, the entry without `if` is used. |
| Scheduled tasks | `operations[].if` | Same as install steps. Only administrators can save a task step with a condition; for other users the API answers `only administrators can use conditions`. |
| Version changes | `install[].if` | When the panel changes a server's version, it uses the conditions to find which download step applies to that server (for example, which launcher of the Minecraft template). |
| World map detection | `install[].if` | The install steps whose condition holds tell the panel which platform the server runs. |

A condition must return a boolean. Any other result is an error (`invalid return type, expected bool, got ...`).

What happens on an error:

- In install, pre/post and task steps, an evaluation error stops the whole process and the error is shown in the server console.
- In uninstall steps, an error stops the remaining steps and is logged and shown in the console, but the server is still deleted (its environment is removed).
- In the start command, an error stops the start (`Failed to start server`).
- In version changes and world map detection, an error counts as "false" and the step is ignored.

## Available data

| Name | Type | Value |
|------|------|-------|
| `os` | string | The operating system of the daemon process (Go's `runtime.GOOS`): `linux`, `windows`... |
| `arch` | string | The architecture of the daemon process (`runtime.GOARCH`): `amd64`, `arm64`... |
| `env` | string | The server environment type, for example `"docker"` or `"host"`. It is a string, not a map of environment variables. |
| `serverId` | string | The server identifier. |
| `success` | bool | In step lists, `true`: a failed step stops the list, so later steps never see `false`. In `run.post`, whether the server stopped gracefully (`false` after a crash). |
| `exitCode` | int | Only in `run.post`: the exit code of the server's process. |
| Each server variable | any | The variable's current value, by its name (`javaversion`, `modlauncher`, `memory`...). |

Notes:

- Server variables are referenced by name (`javaversion != ""`). You can also write `${name}` inside an `if`, as in the rest of the step: before evaluating, the panel replaces it safely with the value. Inside a string the value is escaped (`in_path("java${javaversion}")` becomes `in_path("java17")`); outside a string `${name}` becomes the variable itself. The value is never read as CEL code. A `${...}` with an unknown name is left as it is.
- The `${...}` tokens of a step (in its `if` and its arguments) are resolved right when that step runs, so they see variables an earlier step changed (for example `githubdl`'s `outputVariable` or the version a resolve step finds).
- Every variable is declared with a dynamic type, so comparisons are checked at evaluation time. A variable of type number is compared with a number, a text variable with a string.
- An empty `if` means the step always runs, in `run.post` after a crash too.
- `os` and `arch` describe where the daemon runs, not the inside of a Docker container.

## Functions added by the panel

They are registered in `internal/servers/operation_functions.go` (`CreateFunctions`).

### `file_exists(path)`

```cel
file_exists("server.properties")
```

Returns `true` if the path exists inside the server's folder. The path is relative to the server's root; a path that leaves the root (`../`) always returns `false`. It uses `os.Stat`, so a symbolic link counts if its target exists.

### `in_path(command)`

```cel
in_path("java")
```

Returns `true` if the command is found in the `PATH` (`exec.LookPath`), including a match in the current directory. The search is done on the daemon's own `PATH`, not inside the server's container.

### `is_server_running()`

```cel
is_server_running()
```

Takes no arguments. Returns `true` if **this** server's process is running. Any error while asking counts as `false`.

## Standard CEL

All of CEL's standard syntax works. A few examples:

```cel
javaversion != ""
modlauncher == "fabric"
modlauncher in ["forge", "neoforge"]
os != "windows" && arch == "amd64"
!file_exists("server.properties")
```

Strings can use double or single quotes (`loader == 'vanilla'` works).

## Examples

Install Java only if the template sets a version, and write `server.properties` only the first time:

```json
"install": [
  { "if": "javaversion != \"\"", "type": "javadl", "version": "${javaversion}" },
  { "type": "mojangdl", "version": "${version}", "target": "server.jar" },
  {
    "if": "!file_exists(\"server.properties\")",
    "type": "writefile",
    "target": "server.properties",
    "text": "server-port=${port}\n"
  }
]
```

One download step per launcher; only the one that matches the server runs:

```json
"install": [
  { "if": "modlauncher == \"\"", "type": "mojangdl", "version": "${version}", "target": "server.jar" },
  { "if": "modlauncher == \"fabric\"", "type": "fabricdl", "version": "${version}" }
]
```

A start command with a fallback:

```json
"run": {
  "command": [
    { "if": "in_path(\"java21\")", "command": "java21 -Xmx${memory}M -jar server.jar" },
    { "command": "java -Xmx${memory}M -jar server.jar" }
  ]
}
```

A post-execution step that only runs after a crash (`success` and `exitCode` only exist in `run.post` conditions, not in `${...}` arguments):

```json
"run": {
  "post": [
    { "if": "!success && exitCode != 0", "type": "writefile", "target": "last-crash.txt", "text": "The server crashed\n" }
  ]
}
```

## Common errors

| Message (start) | Cause |
|-----------------|-------|
| `undeclared reference to 'x'` | The condition uses a name that is neither a server variable nor one of the names above. |
| `invalid return type, expected bool` | The expression does not return true or false. |

Syntax errors and operators used with the wrong types (for example, comparing a number with a string) also fail; the message comes from `cel-go` and points at the position in the expression.

## For developers

The `conditions` package (`conditions/executor.go`) holds the engine:

| Function | What it does |
|----------|--------------|
| `ResolveIf(condition, data, extras)` | Replaces the `${name}` tokens of the condition (`SubstituteVariables`, in `conditions/variables.go`), evaluates it and returns a boolean. If the condition is empty and `data` has `success`, it evaluates `success` (step lists do not call `ResolveIf` for a step without `if`: they always run it). |
| `Run[T](statement, data, extras)` | Evaluates an expression and returns a value of type `T`. Each key of `data` becomes a CEL variable of dynamic type, plus the constants `os` and `arch`. |

`Server.RunCondition` (`internal/servers/server.go`) builds the data (`env`, `serverId`, the server variables and any extra values) and adds the functions of `CreateFunctions`. A new function is added there, as another `cel.Function(...)` with its overload and binding.
