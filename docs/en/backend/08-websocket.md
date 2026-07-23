# WebSocket — Real-time Console Protocol

The panel uses WebSockets to transmit the console, statistics, and server status in real-time.

## Connection

```
ws[s]://<host>/api/servers/<serverId>/socket[?console]
```

- `?console` — optional parameter (currently ignored, always connects to the console channel)
- Authentication via session cookie (does not require token in URL)
- Protocol: **gorilla/websocket** with JSON text messages

## Server → Client Messages

### Type: `console`
```json
{
  "type": "console",
  "data": "[10:30:15] [Server thread/INFO]: Starting server"
}
```
Game server console line. The frontend applies ANSI color parsing.

### Type: `stats`
```json
{
  "type": "stats",
  "data": {
    "cpu": 45.2,
    "memory": 1536000000,
    "memoryTotal": 2147483648
  }
}
```
Periodic statistics (every ~15 seconds). CPU as percentage, memory in bytes.

### Type: `status`
```json
{
  "type": "status",
  "data": {
    "running": true
  }
}
```
Server status change (running/stopped/installing).

## Client → Server Messages

### Send Command
```json
{
  "type": "console",
  "data": "say Hello World!"
}
```
Sends a command to the game server console.

## Tracker System (Internal Pub/Sub)

WebSockets use a **tracker** system defined in `pkg/skypanel/tracker.go`:

```go
type Tracker struct {
    listeners map[string]chan interface{}  // channels by topic
    lock      sync.RWMutex
}
```

There are 3 trackers per server, created in `CreateEnvironment()`:

| Tracker | Description |
|---|---|
| `ConsoleTracker` | Console lines |
| `StatusTracker` | Status changes (running/stopped) |
| `StatsTracker` | Periodic statistics (CPU, RAM) |

### How it Works

1. The daemon writes to the trackers from:
   - `processStats()` → `StatsTracker`
   - `servers/server.go:` status changes → `StatusTracker`
   - STDOUT of the server process → `ConsoleTracker`
2. The WebSocket handler (`/api/servers/:serverId/socket`) subscribes to all 3 trackers
3. When a message arrives from the client, it is forwarded to the process (stdin)

### MemoryCache (Console Buffer)

```go
type MemoryCache struct {
    entries []interface{}
    max     int
}
```

- Circular buffer of configurable size (`daemon.console.buffer`, default 50)
- Stores the last N console lines
- When a client connects, it first receives the historical buffer

## Daemon ↔ Panel WebSocket

The daemon can also connect via WebSocket to the panel to forward the console (enabled with `daemon.console.forward`). This allows the central panel to access the console even if the daemon is on a separate node.

## RCON WebSocket

In addition to the console WebSocket, there is RCON over WebSocket in `internal/connections/rconws.go` for RCON connections from the browser.