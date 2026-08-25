# Operations Engine

Aether Panel uses a JSON-based operation engine for executing installation, update, and maintenance tasks. Operations are composable, conditionally executed, and support variable interpolation.

---

## Overview

Operations are the building blocks for:
- **Installation**: Downloading, extracting, configuring game servers
- **Updates**: Patching, version switching, backup before update
- **Maintenance**: Cleanup, optimization, validation
- **Custom workflows**: User-defined operation sequences

---

## Operation Structure

Each operation is a JSON object with a `type` and type-specific parameters:

```json
{
  "type": "command",
  "command": "java -version",
  "if": "variables.java_installed == true"
}
```

**Common Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Operation type (required) |
| `if` | string | CEL condition (optional) |
| `description` | string | Human-readable description |
| `ignoreError` | bool | Continue on failure |

---

## Variable Interpolation

All string fields support `{{variable}}` interpolation:

```json
{
  "type": "download",
  "url": "https://example.com/{{version}}/server.jar",
  "destination": "{{rootDir}}/server.jar"
}
```

**Available Variables:**
- All server variables (`memory`, `port`, `version`, etc.)
- `rootDir`: Server root directory
- `backupDir`: Backup directory
- `os`, `arch`: Runtime OS/architecture
- `success`: Previous operation success (boolean)
- `env`: All environment variables

---

## Condition Engine (CEL)

Conditions use **Common Expression Language (CEL)**:

```json
"if": "variables.java_installed == true && os == 'linux'"
```

**Global Constants:**
| Constant | Type | Description |
|----------|------|-------------|
| `os` | string | Runtime OS (`linux`, `windows`) |
| `arch` | string | Architecture (`amd64`, `arm64`) |

**Available Variables:**
| Variable | Source |
|----------|--------|
| `success` | Previous operation result |
| `env` | All environment variables |
| `serverId` | Current server identifier |
| `nodeId` | Current node ID |
| All server variables | From server definition |

**Custom CEL Functions:**
| Function | Signature | Description |
|----------|-----------|-------------|
| `file_exists(path)` | `string -> bool` | Checks if file exists |
| `in_path(cmd)` | `string -> bool` | Checks if command in PATH |
| `is_server_running(id)` | `string -> bool` | Checks if server is running |

---

## Operation Registration (`internal/servers/operation_functions.go`)

26 operations registered via `init()` in `commandMapping`:

```go
var commandMapping = map[string]OperationFactory{
    "alterfile": AlterFileFactory,
    "archive": ArchiveFactory,
    "command": CommandFactory,
    "console": ConsoleFactory,
    "curseforge": CurseForgeFactory,
    "dockerpull": DockerPullFactory,
    "download": DownloadFactory,
    "extract": ExtractFactory,
    "fabricdl": FabricDLFactory,
    "forgedl": ForgeDLFactory,
    "githubdl": GitHubDLFactory,
    "javadl": JavaDLFactory,
    "mkdir": MkdirFactory,
    "mojangdl": MojangDLFactory,
    "move": MoveFactory,
    "neoforgedl": NeoForgeDLFactory,
    "nodejsdl": NodeJSDLFactory,
    "paperdl": PaperDLFactory,
    "resolveforgeversion": ResolveForgeVersionFactory,
    "resolveneoforgeversion": ResolveNeoForgeVersionFactory,
    "scraperdl": ScraperDLFactory,
    "sleep": SleepFactory,
    "spongedl": SpongeDLFactory,
    "stdin": StdinFactory,
    "steamgamedl": SteamGameDLFactory,
    "writefile": WriteFileFactory,
}
```

---

## Operation Process (`internal/servers/operation_process.go`)

```go
type OperationProcess struct {
    operations []Operation
    server     *Server
    variables  map[string]interface{}
}

func (op *OperationProcess) Run(ctx context.Context) error {
    for _, op := range operations {
        // 1. Evaluate condition
        if !evaluateCondition(op.If, variables) {
            continue // Skip
        }
        
        // 2. Resolve variables in parameters
        resolved := resolveVariables(op, variables)
        
        // 3. Execute operation
        result := factory.Create(resolved).Run(ctx, server)
        
        // 4. Update variables
        variables["success"] = result.Success
        if !result.Success && !op.IgnoreError {
            return result.Error
        }
    }
    return nil
}
```

---

## 26 Operation Types Reference

### 1. `alterfile` - Modify File Content

```json
{
  "type": "alterfile",
  "file": "server.properties",
  "operations": [
    { "type": "replace", "from": "max-players=20", "to": "max-players={{maxPlayers}}" },
    { "type": "append", "content": "motd=Welcome to {{serverName}}!" }
  ]
}
```

**Operations:** `replace`, `append`, `prepend`, `regex_replace`, `delete_line`

---

### 2. `archive` - Create Archive

```json
{
  "type": "archive",
  "source": "world",
  "destination": "backups/world_{{timestamp}}.zip",
  "format": "zip"  // zip, tar, tar.gz, tar.bz2
}
```

---

### 3. `command` - Execute Shell Command

```json
{
  "type": "command",
  "command": "chmod +x {{rootDir}}/start.sh",
  "workingDirectory": "{{rootDir}}",
  "environment": { "JAVA_HOME": "/usr/lib/jvm/java-17" },
  "timeout": 300
}
```

---

### 4. `console` - Send Console Command

```json
{
  "type": "console",
  "command": "save-all",
  "waitFor": "Saved the game"
}
```

---

### 5. `curseforge` - Download from CurseForge

```json
{
  "type": "curseforge",
  "projectId": 12345,
  "fileId": 67890,
  "destination": "{{rootDir}}/mods/mod.jar",
  "apiKey": "{{curseforgeApiKey}}"
}
```

---

### 6. `dockerpull` - Pull Docker Image

```json
{
  "type": "dockerpull",
  "image": "eclipse-temurin:17-jdk",
  "platform": "linux/amd64"
}
```

---

### 7. `download` - HTTP Download

```json
{
  "type": "download",
  "url": "https://example.com/file.jar",
  "destination": "{{rootDir}}/server.jar",
  "headers": { "Authorization": "Bearer {{token}}" },
  "checksum": "sha256:abc123...",
  "progress": true
}
```

---

### 8. `extract` - Extract Archive

```json
{
  "type": "extract",
  "source": "server.zip",
  "destination": "{{rootDir}}",
  "skipRoot": true,
  "overwrite": true
}
```

---

### 9. `fabricdl` - Download Fabric Loader

```json
{
  "type": "fabricdl",
  "minecraftVersion": "1.20.1",
  "loaderVersion": "0.14.21",
  "destination": "{{rootDir}}/fabric.jar"
}
```

---

### 9. `forgedl` - Download Forge

```json
{
  "type": "forgedl",
  "minecraftVersion": "1.20.1",
  "forgeVersion": "47.2.0",
  "destination": "{{rootDir}}/forge.jar"
}
```

---

### 10. `githubdl` - Download from GitHub Releases

```json
{
  "type": "githubdl",
  "repo": "PaperMC/Paper",
  "tag": "1.20.1",
  "assetPattern": "paper-*.jar",
  "destination": "{{rootDir}}/paper.jar",
  "token": "{{githubToken}}"
}
```

---

### 11. `javadl` - Download Java

```json
{
  "type": "javadl",
  "version": "17",
  "distribution": "temurin",
  "destination": "{{rootDir}}/java",
  "addToPath": true
}
```

---

### 12. `mkdir` - Create Directory

```json
{
  "type": "mkdir",
  "path": "{{rootDir}}/plugins",
  "permissions": "0755"
}
```

---

### 12. `mojangdl` - Download Mojang Server

```json
{
  "type": "mojangdl",
  "version": "1.20.1",
  "destination": "{{rootDir}}/server.jar"
}
```

---

### 13. `move` - Move/Rename Files

```json
{
  "type": "move",
  "source": "{{rootDir}}/old_file.txt",
  "destination": "{{rootDir}}/new_file.txt",
  "overwrite": true
}
```

---

### 14. `neoforgedl` - Download NeoForge

```json
{
  "type": "neoforgedl",
  "minecraftVersion": "1.20.1",
  "neoForgeVersion": "20.1.0",
  "destination": "{{rootDir}}/neoforge.jar"
}
```

---

### 15. `nodejsdl` - Download Node.js

```json
{
  "type": "nodejsdl",
  "version": "20",
  "destination": "{{rootDir}}/node",
  "addToPath": true
}
```

---

### 15. `paperdl` - Download Paper

```json
{
  "type": "paperdl",
  "version": "1.20.1",
  "build": "latest",
  "destination": "{{rootDir}}/paper.jar"
}
```

---

### 16. `resolveforgeversion` - Resolve Forge Version

```json
{
  "type": "resolveforgeversion",
  "minecraftVersion": "1.20.1",
  "outputVariable": "forgeVersion"
}
```

---

### 17. `resolveneoforgeversion` - Resolve NeoForge Version

```json
{
  "type": "resolveneoforgeversion",
  "minecraftVersion": "1.20.1",
  "outputVariable": "neoForgeVersion"
}
```

---

### 18. `scraperdl` - Generic HTML Scraper Download

```json
{
  "type": "scraperdl",
  "url": "https://example.com/downloads",
  "selector": "a.download-link",
  "pattern": "server-*.jar",
  "destination": "{{rootDir}}/server.jar"
}
```

---

### 19. `sleep` - Delay Execution

```json
{
  "type": "sleep",
  "duration": "5s"  // Parsed by time.ParseDuration
}
```

---

### 20. `spongedl` - Download Sponge

```json
{
  "type": "spongedl",
  "minecraftVersion": "1.12.2",
  "destination": "{{rootDir}}/sponge.jar"
}
```

---

### 21. `stdin` - Send to Process Stdin

```json
{
  "type": "stdin",
  "command": "save-all",
  "expect": "Saved the game",
  "timeout": 30
}
```

---

### 22. `steamgamedl` - Download Steam Game

```json
{
  "type": "steamgamedl",
  "appId": 232370,
  "beta": "public",
  "destination": "{{rootDir}}",
  "username": "{{steamUser}}",
  "password": "{{steamPass}}"
}
```

---

### 23. `writefile` - Write File Content

```json
{
  "type": "writefile",
  "file": "config.yml",
  "content": |
    server-port: {{port}}
    max-players: {{maxPlayers}}
    motd: "Welcome to {{serverName}}"
  }
}
```

---

## Installation/Uninstallation in Server Definition

```json
{
  "installation": [
    { "type": "mkdir", "path": "{{rootDir}}" },
    { "type": "download", "url": "...", "destination": "{{rootDir}}/server.jar" },
    { "type": "writefile", "file": "eula.txt", "content": "eula=true" },
    { "type": "command", "command": "java -jar server.jar --generate-config" }
  ],
  "uninstallation": [
    { "type": "command", "command": "rm -rf {{rootDir}}/*" }
  ]
}
```

---

## Conditional Execution

```json
{
  "type": "download",
  "url": "...",
  "if": "!file_exists('{{rootDir}}/server.jar')"
}
```

**Best Practices:**
1. Use `if` to skip already-completed steps
2. Use `ignoreError: true` for optional steps
3. Chain operations with `success` variable
4. Use `description` for UI clarity

---

## Custom Operations

To add a custom operation:

1. Create factory in `internal/servers/operation_functions.go`
2. Register in `commandMapping`
3. Implement `OperationFactory` interface

```go
type MyCustomOperation struct {
    Param1 string `json:"param1"`
}

func (o *MyCustomOperation) Run(ctx context.Context, server *Server) OperationResult {
    // Implementation
    return OperationResult{Success: true}
}

func init() {
    commandMapping["mycustom"] = func() Operation { return &MyCustomOperation{} }
}
```