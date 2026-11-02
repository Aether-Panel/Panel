# Console Input: stdin, RCON, Telnet and RCON over WebSocket

Every running server has one **console input**: the channel the daemon writes commands to. By default that is the process's own stdin (the PTY on the host, the attached stream in Docker). A template can instead route commands over the network, through RCON, Telnet or RCON over WebSocket.

This only changes where **commands go**. Console **output** always comes from the process's stdout/stderr (PTY or `docker attach`), whatever the input type.

Code:

| File | Role |
|---|---|
| `pkg/skypanel/stdin.go` | `StdinConsoleConfiguration` (the `stdin` block of a template) |
| `pkg/skypanel/console.go` | `Console` interface and `NoStartConsole` (plain stdin) |
| `pkg/skypanel/environment.go` | `CreateConsoleStdinProxy()` and `ExecuteInMainProcess()` |
| `internal/connections/rcon.go` | `RCONConnection` |
| `internal/connections/telnet.go` | `TelnetConnection` |
| `internal/connections/rconws.go` | `RCONWSConnection` |

---

## Input types

| `type` | Implementation | Transport | Library |
|---|---|---|---|
| `stdin` (or empty) | `NoStartConsole` | Writes to the process stdin (PTY / Docker attach) | — |
| `rcon` | `RCONConnection` | Source RCON over TCP | `github.com/gorcon/rcon` |
| `telnet` | `TelnetConnection` | Raw TCP | `net` |
| `rconws` | `RCONWSConnection` | JSON over WebSocket, **TLS only** (`wss://`) | `github.com/gorilla/websocket` |

The type is spelled `rconws`. Any other value the switch does not know (for example `rcon-ws`) falls back to plain stdin and logs an error naming the server, the type and the known ones: `Unknown console type "rcon-ws" (known: stdin, rcon, telnet, rconws): commands go to the standard input`.

---

## Configuration

The `stdin` block lives in the server definition, under `run`, or on a single entry of a command list:

```json
{
  "run": {
    "command": "java -jar server.jar nogui",
    "stop": "stop",
    "stdin": {
      "type": "rcon",
      "ip": "127.0.0.1",
      "port": "${rconport}",
      "password": "${rconpassword}"
    }
  }
}
```

```go
type StdinConsoleConfiguration struct {
    Type     string `json:"type,omitempty"`
    IP       string `json:"ip,omitempty"`
    Port     string `json:"port,omitempty"`
    Password string `json:"password,omitempty"`
}
```

| Field | Notes |
|---|---|
| `type` | `stdin`, `rcon`, `telnet` or `rconws`. A `stdin` block without `type` is read as `stdin` (custom `UnmarshalJSON`). |
| `ip` | Address the daemon connects to. Empty, `0.0.0.0` or `::` mean the game's own machine (see below). |
| `port` | A **string**, so it can hold a variable (`"${rconport}"`). |
| `password` | RCON password, Telnet password line or the RCON-WS path segment. |

**Variables.** Before the process starts, `ExecuteAsync()` (and `Reattach()`) replaces `${name}` tokens in `ip`, `port` and `password` with the server's variables (`StdinConsoleConfiguration.Replace` → `utils.ReplaceTokens`). The syntax is `${name}`, not `{{name}}`. There is no automatic port detection: the template must point `port` at the right variable.

**Which block applies.** When `run.command` is a list, the chosen entry's own `stdin` wins; if that entry has none (`type` empty), `run.stdin` is used (`Server.Start()` in `internal/servers/server.go`).

**Where the daemon connects.** The connection is made from the daemon process to `ip:port`. When `ip` is empty, `0.0.0.0` or `::`, the three network consoles resolve it as the game queries do (`query.HostOrLocal`, `internal/query/host.go`): `127.0.0.1` on a plain install, or the container's default gateway when the daemon itself runs in Docker, which reaches the ports game containers publish on the host. A specific address is used as it is. See [Remaining packages](./10-remaining-packages.md#internalquery--game-queries).

---

## How the input is created

`Environment.CreateConsoleStdinProxy(config, base)` picks the implementation and stores it in `Environment.Console`:

```go
func (e *Environment) CreateConsoleStdinProxy(config StdinConsoleConfiguration, base io.WriteCloser) {
    switch config.Type {
    case "telnet":
        e.Console = &connections.TelnetConnection{IP: config.IP, Port: config.Port, Password: config.Password}
    case "rcon":
        e.Console = &connections.RCONConnection{IP: config.IP, Port: config.Port, Password: config.Password}
    case "rconws":
        e.Console = &connections.RCONWSConnection{IP: config.IP, Port: config.Port, Password: config.Password}
    case "", "stdin":
        e.Console = &NoStartConsole{Base: base}
    default:
        logging.Error.Printf("[%s] Unknown console type %q (known: stdin, rcon, telnet, rconws): ...", e.ServerID, config.Type)
        e.Console = &NoStartConsole{Base: base}
    }
}
```

It is called by each environment right after the process is up:

| Environment | Call site | `base` (plain stdin) |
|---|---|---|
| Host (`tty`, `host`, `standard`) | `internal/servers/tty/tty.go` `ExecuteAsyncImpl` | the PTY from `pty.Start` |
| Docker | `internal/servers/docker/docker.go` `ExecuteAsyncImpl` | the hijacked `ContainerAttach` connection |
| Docker, after a daemon restart | `docker.go` `ReattachImpl` | the new attach connection |

Then `environment.Console.Start()` runs. On process exit the environment calls `environment.Console.Close()`.

```go
type Console interface {
    io.WriteCloser
    Start()
}
```

`NoStartConsole.Start()` does nothing; `Write` and `Close` go straight to the base stream.

---

## What writes to the console

All commands go through `Environment.ExecuteInMainProcess(cmd)`, which appends `\n` and writes to `Environment.Console`:

| Source | Code |
|---|---|
| Console box in the panel: `POST /api/servers/:id/console` (scope `server.console.send`) | `daemon/server.go postConsole` → `Server.Execute` |
| Stop button, when the template has no `stopCode` | `Server.Stop` → `ExecuteInMainProcess(run.stop)` |
| Keep-alive command | `Server.startKeepAlive` |
| `stdin` operation (scheduled tasks, install steps) | `internal/operations/stdin` |

Errors returned:

| Error | When |
|---|---|
| `ErrServerOffline` | The server is not running |
| `ErrConsoleDetached` | The server is still running from before a daemon restart and no console is attached yet ("kill and start the server to reconnect"). `Execute`, `Stop` and `Kill` call `Reattach()` first, which reconnects Docker servers. |
| `"rcon not available"`, `"telnet not available"`, `"rconws not available"` | The network connection is not ready, even after waiting one second |

The WebSocket that streams the console to the browser does not accept commands: incoming frames are only read to keep the connection alive (`Socket.Serve`).

---

## RCON (`internal/connections/rcon.go`)

```go
type RCONConnection struct {
    io.WriteCloser
    IP, Port, Password string
    Reconnect          bool
    // connection, ready, closer, lock: internal
}
```

- **`Start()`**: sets `Reconnect = true`, resolves `IP` with `query.HostOrLocal` (see [Where the daemon connects](#configuration)) and starts the reconnect loop in a goroutine.
- **Reconnect loop**: `rcon.Dial("ip:port", password)`. The first attempt is immediate, every later one waits **5 seconds**. A connection stays open until a write fails or `Close()` is called; then the loop dials again. Dial errors are only logged at debug level.
- **`Write(p)`**: if not connected, waits one second and checks again. Runs `conn.Execute(string(p))`. The server's reply is **only written to the debug log** (`RCON Response: ...`), not to the console. A failed command drops the connection so the loop opens a new one.
- **`Close()`**: stops the loop, wakes it and closes the connection. A second close of the same connection is not an error.
- State is guarded by a mutex (covered by `internal/connections/rcon_test.go`).

Timeouts are those of `gorcon/rcon` defaults; the panel does not set its own.

---

## Telnet (`internal/connections/telnet.go`)

```go
type TelnetConnection struct {
    io.WriteCloser
    IP, Port, Password string
    Reconnect          bool
}
```

- **`Start()`** resolves `IP` like RCON does.
- **Reconnect loop**: `net.DialTCP` to `ip:port`, retry every 5 seconds after the first attempt. Enables TCP keep-alive.
- After connecting it waits **one second** (for a password prompt) and, if `password` is set, sends it followed by `\n`; if that write fails, the socket is closed and the loop dials again.
- Incoming data is read and **discarded**; the loop only uses reads to notice a closed connection, then dials again.
- **`Write(p)`** writes the raw bytes to the socket (one-second wait if not ready).
- No application-level heartbeat, line buffering or echo control.
- State is guarded by a mutex, as with RCON: a socket that breaks is closed before the next dial, a connection opened while `Close()` runs is closed at once instead of being kept, and closing twice is not an error (covered by `internal/connections/telnet_test.go`).

---

## RCON over WebSocket (`internal/connections/rconws.go`)

```go
type RCONWSConnection struct {
    io.WriteCloser
    IP, Port, Password string
    Reconnect          bool
}
```

- **Reconnect loop**: dials `wss://<ip>:<port>/<password>` (`rconwsURL`) with the default gorilla dialer, retry every 5 seconds. The password is the URL path and is escaped (`url.PathEscape`), so a `/`, `?`, `#` or `%` in it reaches the game as typed. The scheme is always `wss://`, since the template's console settings have no way to ask for plain `ws://`: a server that only speaks `ws://` (Rust's WebRCON usually does) cannot be reached.
- **`Start()`** resolves `IP` like RCON does.
- Incoming messages are read and discarded.
- State is guarded by a mutex and writes are serialised (a WebSocket takes one writer at a time). As with Telnet, a broken socket is closed before the next dial, a connection opened while `Close()` runs is closed at once, and closing twice is not an error (covered by `internal/connections/rconws_test.go`).
- **`Write(p)`** sends one JSON frame per command, with an identifier that increases on every write:

```json
{ "Message": "say Hello\n", "Identifier": 1, "Stacktrace": "" }
```

(The trailing `\n` is the one `ExecuteInMainProcess` appends.)

---

## Template examples

Minecraft with RCON (the game must have `enable-rcon=true` and the same password and port):

```json
"run": {
  "command": "java -Xmx${memory}M -jar server.jar nogui",
  "stop": "stop",
  "stdin": { "type": "rcon", "port": "${rconport}", "password": "${rconpassword}" }
}
```

A server with a Telnet admin console:

```json
"stdin": { "type": "telnet", "ip": "127.0.0.1", "port": "${telnetport}", "password": "${telnetpassword}" }
```

The variable names (`rconport`, `rconpassword`, ...) are whatever the template defines; the panel gives them no special meaning.

---

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Commands fail with `rcon not available` | RCON is not enabled in the game, wrong port/password, or the game listens on an address the daemon cannot reach. Turn on debug logging to see `Error connecting to RCON`. |
| Command accepted but nothing appears in the console | Expected with `rcon`: the reply only goes to the debug log. The game's own output still appears if it prints to stdout. |
| `rconws` never connects | The target only offers `ws://`; this client always uses `wss://`. |
| `type` set but commands still go to stdin | The type is misspelled (`rcon-ws` instead of `rconws`); the daemon log shows `Unknown console type`. |
| `ErrConsoleDetached` | The daemon restarted while a host (non-Docker) server kept running; kill and start it. |

## Security notes

- RCON and Telnet are unencrypted. Keep them on loopback or a private network and do not publish their ports.
- The `password` is part of the server definition; anyone who can read the definition can read it.
- With `rconws` the password travels in the URL path (encrypted by TLS, but visible to the remote server's logs).
