# CEL Engine & Custom Functions

Aether Panel uses **Common Expression Language (CEL)** for conditional operation execution and dynamic configuration evaluation.

---

## Overview

- **Library**: `google/cel-go`
- **Use Cases**: Operation conditions, variable validation, dynamic config
- **Syntax**: CEL spec compliant with custom functions

---

## CEL Basics

### Syntax
```cel
// Comparisons
variable > 10
name == "admin"
status in ["running", "stopped"]

// Logical
a && b || c
!condition

// String operations
name.contains("admin")
name.startsWith("server_")
name.endsWith(".jar")

// List/map operations
size(list) > 0
key in map
map[key] == "value"

// Math
memory / 1024 / 1024 > 1024
```

### In Operations
```json
{
  "type": "download",
  "url": "https://example.com/file.jar",
  "if": "!file_exists('{{rootDir}}/server.jar') && variables.auto_update == true"
}
```

---

## Global Constants

| Constant | Type | Description |
|----------|------|-------------|
| `os` | string | Runtime OS: `linux`, `windows` |
| `arch` | string | Architecture: `amd64`, `arm64` |

---

## Available Variables

| Variable | Source | Type |
|----------|--------|------|
| `success` | Previous operation | bool |
| `env` | Environment variables | map[string]string |
| `serverId` | Current server | string |
| `nodeId` | Current node | string |
| All server variables | Server definition | various |

---

## Custom CEL Functions

Registered in `internal/servers/operation_functions.go:initCELFunctions()`:

### 1. `file_exists(path)`

```cel
file_exists("/path/to/file")
```

**Returns:** `bool` - true if file exists and is accessible

**Implementation:** `os.Stat(path)` - checks existence and readability

---

### 2. `in_path(command)`

```cel
in_path("java")
in_path("/usr/bin/java")
```

**Returns:** `bool` - true if command found in PATH

**Implementation:** `exec.LookPath(command)`

---

### 3. `is_server_running()`

```cel
is_server_running()
```

**Returns:** `bool` - true if any server is currently running (checks all servers)

**Implementation:** Checks server cache for running processes

**Note:** Takes **no arguments** - checks if ANY server is running.

---

## CEL in Operations

### Condition Evaluation (`internal/servers/operation_functions.go`)

```go
func evaluateCondition(condition string, variables map[string]interface{}) (bool, error) {
    if condition == "" {
        return true, nil
    }
    env, _ := cel.NewEnv(
        cel.Variable("success", cel.BoolType),
        cel.Variable("env", cel.MapType(cel.StringType, cel.StringType)),
        // ... all variables
        cel.Function("file_exists", fileExists),
        cel.Function("in_path", inPath),
        cel.Function("is_server_running", isServerRunning),
        // ...
    )
    // ...
}
```

### Variable Binding

All variables from server definition + runtime are bound:

```go
variables := map[string]interface{}{
    "success":     previousResult.Success,
    "env":         getEnvVars(),
    "serverId":    server.Identifier,
    "nodeId":      server.NodeID,
    // All server variables
    "memory":      server.Variables["memory"].Value,
    "port":        server.Variables["port"].Value,
    // ...
}
```

---

## Use Cases

### 1. Conditional Installation Steps

```json
{
  "type": "javadl",
  "version": "17",
  "if": "!in_path('java') || env_var('JAVA_HOME') == ''"
}
```

### 2. Conditional Config Updates

```json
{
  "type": "alterfile",
  "file": "server.properties",
  "operations": [
    { "type": "replace", "from": "max-players=20", "to": "max-players=100" }
  ],
  "if": "variables.maxPlayers > 20"
}
```

### 3. Platform-Specific Operations

```json
{
  "type": "download",
  "url": "https://example.com/linux_binary",
  "if": "os == 'linux' && arch == 'amd64'"
}
```

### 4. Skip if Already Done

```json
{
  "type": "download",
  "url": "https://example.com/server.jar",
  "if": "!file_exists('{{rootDir}}/server.jar')"
}
```

### 5. Dependency Checks

```json
{
  "type": "fabricdl",
  "if": "is_server_running() == false"
}
```

---

## Error Handling

| Error | Cause | Resolution |
|-------|-------|------------|
| `undefined variable` | Variable not in scope | Check variable name, add to binding |
| `type mismatch` | Wrong type for operator | Cast or use correct operator |
| `no such overload` | Function args wrong | Check function signature |
| `parse error` | Syntax error | Check CEL syntax |

---

## Debugging CEL

### Enable Debug Logging

```json
{
  "panel": {
    "settings": {
      "celDebug": true
    }
  }
}
```

### Test Expressions

```go
// In Go REPL or test
env, _ := cel.NewEnv(cel.Variable("x", cel.IntType))
ast, iss := env.Compile("x > 10")
prog, _ := env.Program(ast)
result, _, _ := prog.Eval(map[string]interface{}{"x": 15})
fmt.Println(result.Value()) // true
```

---

## Extending CEL Functions

Add custom functions in `initCELFunctions()`:

```go
func initCELFunctions() {
    cel.Function("my_custom_func",
        cel.Overload("my_custom_func_string", []*cel.Type{cel.StringType}, cel.BoolType),
        cel.FunctionBinding(func(args ...ref.Value) ref.Value {
            s := args[0].Value().(string)
            return ref.OfBool(strings.HasPrefix(s, "custom_"))
        }),
    )
}
```

---

## Security

- **No arbitrary code execution**: CEL is not Turing-complete
- **Sandboxed**: No filesystem/network access except via explicit functions
- **Resource limits**: Max expression size, max recursion depth
- **Audit logging**: All evaluated expressions logged at debug level