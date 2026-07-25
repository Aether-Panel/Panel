# Operations System

Operations are atomic units of work that are executed as part of a server's lifecycle (installation, uninstallation, scheduled tasks, etc.).

## Interfaces

All operations implement two interfaces in `pkg/skypanel/operation.go`:

```go
type Operation interface {
    Run(args RunOperatorArgs) OperationResult
}

type OperationFactory interface {
    Create(CreateOperation) (Operation, error)
    Key() string  // unique operation identifier
}

type RunOperatorArgs struct {
    Environment *Environment   // execution environment
    Server      DaemonServer   // server (for file access)
}

type OperationResult struct {
    Error             error
    VariableOverrides map[string]interface{}  // variables to update post-execution
}
```

## Operation Registration

Factories are registered via `init()` in each operation package. The global map is located in `internal/servers/operation_functions.go`.

## Available Operations (25)

| Key | Package | Description |
|---|---|---|
| `alterfile` | `operations/alterfile/` | Modifies files (sed-like, regex) |
| `archive` | `operations/archive/` | Compresses files/folders into ZIP |
| `command` | `operations/command/` | Executes an arbitrary command |
| `console` | `operations/console/` | Sends a command to the server console |
| `curseforge` | `operations/curseforge/` | Downloads CurseForge modpacks |
| `dockerpull` | `operations/dockerpull/` | Downloads Docker images |
| `download` | `operations/download/` | Downloads files via HTTP/HTTPS |
| `extract` | `operations/extract/` | Extracts compressed files (ZIP, tar.gz) |
| `fabricdl` | `operations/fabricdl/` | Downloads Fabric Loader for Minecraft |
| `forgedl` | `operations/forgedl/\` | Downloads Minecraft Forge |
| `javadl` | `operations/javadl/\` | Downloads Java Runtime |
| `mkdir` | `operations/mkdir/\` | Creates directories |
| `mojangdl` | `operations/mojangdl/\` | Downloads Minecraft server from Mojang |
| `move` | `operations/move/\` | Moves or renames files |
| `neoforgedl` | `operations/neoforgedl/\` | Downloads NeoForge |
| `nodejsdl` | `operations/nodejsdl/\` | Downloads Node.js |
| `paperdl` | `operations/paperdl/\` | Downloads PaperMC |
| `resolveforgeversion` | `operations/resolveforgeversion/\` | Resolves Forge version |
| `resolveneoforgeversion` | `operations/resolveneoforgeversion/\` | Resolves NeoForge version |
| `sleep` | `operations/sleep/\` | Pauses for a specified duration |
| `spongedl` | `operations/spongedl/\` | Downloads Sponge |
| `stdin` | `operations/stdin/\` | Sends text to process stdin |
| `steamgamedl` | `operations/steamgamedl/\` | Downloads Steam games (DepotDownloader) |
| `writefile` | `operations/writefile/\` | Writes content to a file |

## Conditions (CEL)

Operations can have conditions that determine whether they should be executed:

```go
type ConditionalMetadataType struct {
    MetadataType
    If string // CEL expression
}
```

Conditions are evaluated using `google/cel-go`. For example:

```json
{
  "if": "variables.server_jar == 'paper'",
  "command": "java -jar paper.jar"
}
```

## Execution Flow

1. Operations are defined in the `install` or `uninstall` section of the Server Definition
2. Also in scheduled tasks of the scheduler
3. `internal/servers/operation_process.go:` processes the queue:
   - For each operation in the queue, it retrieves the factory from the global map
   - Creates the operation with `CreateOperation` (args + environment variables)
   - Executes `Run(args)` with the current environment and server
   - If there are `VariableOverrides`, it updates the server variables

## Downloads

Download operations use `cavaliergopher/grab/v3` for HTTP downloads with resumption, progress, and timeouts. Download configuration is located in `pkg/skypanel/download.go`.

## Downloaders (Tests)

`internal/operations/downloaders_test.go` contains tests to verify downloaders for Minecraft Paper, Forge, Fabric, NeoForge, and CurseForge.