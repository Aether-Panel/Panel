# RCON, Telnet & RCON-WS Connections

Aether Panel supports three protocols for server console stdin. The panel auto-selects the appropriate protocol based on server configuration.

---

## Protocol Overview

| Protocol | Type | Use Case | Implementation |
|----------|------|----------|----------------|
| **RCON** | TCP | Minecraft (Spigot/Paper), Source engine | `gorcon/rcon` |
| **Telnet** | TCP | Legacy servers, custom protocols | Raw TCP |
| **RCON-WS** | WebSocket | Browser-based RCON, some panels | `gorilla/websocket` |

---

## Configuration (Server Definition)

In `ServerDefinition.Execution.StdinConsoleConfiguration`:

```json
{
  "stdin": {
    "type": "rcon",           // "rcon" | "telnet" | "rcon-ws"
    "ip": "127.0.0.1",        // Host to connect to
    "port": 25575,            // Port (RCON default: 25575)
    "password": "secret"      // RCON password
  }
}
```

**Defaults by Environment:**
- Docker: auto-detects from container port mappings
- Host/TTY: Uses server's primary IP + RCON port variable

---

## Auto-Selection Logic

`internal/servers/environment.go:CreateConsoleStdinProxy()`:

```go
func CreateConsoleStdinProxy(stdInConfig StdinConsoleConfiguration) (ConsoleStdinProxy, error) {
    switch stdInConfig.Type {
    case "rcon":
        return NewRCONConnection(ip, port, password)
    case "telnet":
        return NewTelnetConnection(ip, port)
    case "rcon-ws":
        return NewRCONWSConnection(ip, port, password)
    default:
        // Auto-detect: try RCON first, fallback to Telnet
        if rconConn, err := NewRCONConnection(ip, port, password); err == nil {
            return rconConn
        }
        return NewTelnetConnection(ip, port)
    }
}
```

---

## RCON Connection (`internal/connections/rcon.go`)

Standard Source RCON protocol via `gorcon/rcon`.

```go
type RCONConnection struct {
    conn *rcon.Conn
    ip   string
    port int
    pass string
}

func NewRCONConnection(ip string, port int, password string) (*RCONConnection, error)
func (r *RCONConnection) Start() error
func (r *RCONConnection) Write(data []byte) (int, error)
func (r *RCONConnection) Close() error
```

**Protocol Details:**
- TCP connection to `ip:port`
- Authentication: `password` sent on connect
- Commands: `rconCmd("command")` via `gorcon`
- Reconnection: Auto-reconnect on disconnect (exponential backoff)
- Timeout: 5s connection, 10s command

**Error Handling:**
- Invalid password → `ErrAuthFailed`
- Connection refused → `ErrConnectionRefused`
- Timeout → `ErrTimeout`

---

## Telnet Connection (`internal/connections/telnet.go`)

Raw TCP with optional password and keepalive.

```go
type TelnetConnection struct {
    conn   net.Conn
    ip     string
    port   int
    pass   string
    alive  bool
}

func NewTelnetConnection(ip string, port int, password string) (*TelnetConnection, error)
func (t *TelnetConnection) Start() error
func (t *TelnetConnection) Write(data []byte) (int, error)
func (t *TelnetConnection) Close() error
```

**Features:**
- Raw TCP connection (no protocol framing)
- Optional password on connect (sent as first line if configured)
- Keepalive: TCP keepalive + application-level heartbeat (every 30s)
- Line buffering: `\n` or `\r\n` terminated
- Echo control: Can disable server echo

**Configuration:**
```json
{
  "stdin": {
    "type": "telnet",
    "ip": "127.0.0.1",
    "port": 23,
    "password": "optional_password"
  }
}
```

---

## RCON-WS Connection (`internal/connections/rconws.go`)

RCON over WebSocket for browser-based clients.

```go
type RCONWSConnection struct {
    conn *websocket.Conn
    ip   string
    port int
    pass string
}

func NewRCONWSConnection(ip string, port int, password string) (*RCONWSConnection, error)
func (r *RCONWSConnection) Start() error
func (r *RCONWSConnection) Write(data []byte) (int, error)
func (r *RCONWSConnection) Close() error
```

**Protocol:**
- WebSocket connection to `ws://ip:port/password`
- Messages: JSON with incremental `id`
```json
{"id": 1, "command": "say Hello"}
```
- Response: Same `id` with output
```json
{"id": 1, "response": "[Server] Hello"}
```
- Auto-reconnect on disconnect
- Heartbeat: ping every 30s

**Use Cases:**
- Browser-based server consoles
- Panels that expose RCON via WebSocket
- Cross-origin RCON access

---

## Console Stdin Proxy Interface

All three implementations satisfy this interface:

```go
type ConsoleStdinProxy interface {
    Start() error
    Write(data []byte) (int, error)
    Close() error
}
```

Used in `Environment.ExecuteAsyncImpl()`:
```go
stdinProxy, err := CreateConsoleStdinProxy(server.Execution.Stdin)
if err != nil {
    return err
}
defer stdinProxy.Close()

// Send command to process stdin
stdinProxy.Write([]byte("say Hello World\n"))
```

---

## Server Definition Example

```json
{
  "execution": {
    "command": "java -Xmx{{memory}}M -jar server.jar nogui",
    "stop": "stop",
    "stdin": {
      "type": "rcon",
      "ip": "{{ip}}",
      "port": {{rconPort}},
      "password": "{{rconPassword}}"
    }
  }
}
```

**Variable Replacement:**
- `{{ip}}` → Server IP
- `{{rconPort}}` → Resolved `port2` or variable `rconPort`
- `{{rconPassword}}` → Variable `rconPassword`

---

## Error Handling

| Error | Cause | Resolution |
|-------|-------|------------|
| `ErrAuthFailed` | Wrong RCON password | Check server.properties `rcon.password` |
| `ErrConnectionRefused` | Port closed/firewall | Check server running, port open |
| `ErrTimeout` | Server unresponsive | Increase timeout, check server load |
| `ErrProtocolMismatch` | Wrong protocol | Match server's enabled protocol |

---

## Integration with Server Execution

In `server.go:ExecuteAsyncImpl()`:

```go
// 1. Create stdin proxy from server definition
stdinProxy, _ := CreateConsoleStdinProxy(env.Server.Execution.Stdin)

// 2. Start proxy (connects to server)
stdinProxy.Start()

// 3. Execute main command + pre/post operations
// ...

// 4. On command send (WebSocket console):
stdinProxy.Write([]byte(command + "\n"))

// 5. On server stop:
stdinProxy.Close()
```

---

## Testing Connections

**CLI Test:**
```bash
# RCON test
rcon-cli -H 127.0.0.1 -P 25575 -p password "say Test"

# Telnet test
telnet 127.0.0.1 23

# RCON-WS test (requires WS client)
wscat -c "ws://127.0.0.1:25575/password"
```

**Panel Test:**
- Server → Console tab → Send command → Check response
- Check logs: `[INFO] RCON connected to 127.0.0.1:25575`

---

## Security Considerations

| Protocol | Encryption | Auth | Notes |
|----------|------------|------|-------|
| RCON | None | Password | Use localhost/VPN |
| Telnet | None | Optional | Never use on public net |
| RCON-WS | WSS (TLS) | Token in URL | Use WSS in production |

**Best Practices:**
- Bind RCON/Telnet to localhost only (`127.0.0.1`)
- Use strong passwords (32+ chars)
- Restrict with firewall (Docker network isolation)
- Rotate passwords periodically