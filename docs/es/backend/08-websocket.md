# WebSocket — Protocolo de Consola en Tiempo Real

El panel utiliza WebSockets para transmitir en tiempo real la consola, estadísticas y estado de los servidores.

## Conexión

```
ws[s]://<host>/api/servers/<serverId>/socket[?console]
```

- `?console` — parámetro opcional (actualmente ignorado, siempre se conecta al canal de consola)
- Autenticación vía cookie de sesión (no requiere token en URL)
- Protocolo: **gorilla/websocket** con mensajes de texto JSON

## Mensajes del Servidor → Cliente

Todas las transmisiones usan la estructura `Transmission` (`pkg/skypanel/message.go`), con campos `data` y `type`. Los tipos son `console`, `stat` y `status`.

### Tipo: `console`
```json
{
  "type": "console",
  "data": {
    "epoch": 1712345678,
    "logs": "[10:30:15] [Server thread/INFO]: Starting server"
  }
}
```
Estructura `ServerLogs` (`pkg/skypanel/httpmodels.go`): `epoch` (timestamp en ms) y `logs` (línea de consola como bytes). El frontend aplica parseo de colores ANSI.

### Tipo: `stat`
```json
{
  "type": "stat",
  "data": {
    "cpu": 45.2,
    "memory": 1536000000,
    "maxMemory": 2147483648,
    "storage": 1024000000,
    "maxStorage": 10737418240,
    "networkRx": 1024,
    "networkTx": 2048,
    "jvm": { "heapUsed": 512, "heapTotal": 1024 },
    "running": true
  }
}
```
Estructura `ServerStats`: CPU como porcentaje, memoria/almacenamiento en bytes, `jvm` opcional (solo si el servidor lo reporta), `running` indica estado. Se envían periódicamente (cada 5s en `processStats()`).

### Tipo: `status`
```json
{
  "type": "status",
  "data": {
    "running": true,
    "installing": false
  }
}
```
Estructura `ServerRunning`: `running` (estado running/stopped) e `installing` (proceso de instalación en curso).

## Mensajes del Cliente → Servidor

### Enviar Comando
```json
{
  "type": "console",
  "data": "say Hello World!"
}
```
Envía un comando a la consola del servidor de juego (se reenvía al stdin del proceso).

## Sistema de Trackers (Pub/Sub Interno)

Los WebSocket usan un sistema de **trackers** definido en `pkg/skypanel/tracker.go`:

```go
type Tracker struct {
    sockets []*Socket   // sockets suscritos
    locker  sync.Mutex
}

type Socket struct {
    conn     *websocket.Conn
    locker   sync.Mutex
    trackers []*Tracker
    closed   bool
    io.WriteCloser
}
```

Hay 3 trackers por servidor, creados en `CreateEnvironment()`:

| Tracker | Descripción |
|---|---|
| `ConsoleTracker` | Líneas de consola |
| `StatusTracker` | Cambios de estado (running/stopped) |
| `StatsTracker` | Estadísticas periódicas (CPU, RAM) |

### Funcionamiento

1. El daemon escribe en los trackers desde:
   - `processStats()` → `StatsTracker`
   - `servers/server.go:` cambios de estado → `StatusTracker`
   - STDOUT del proceso del servidor → `ConsoleTracker`
2. El handler WebSocket (`/api/servers/:serverId/socket`) se suscribe a los 3 trackers
3. Cuando llega un mensaje del cliente, se reenvía al proceso (stdin)
4. `Socket.Serve()` mantiene la conexión viva (ping cada 25s, pong timeout 60s) y desregistra el socket de todos sus trackers al desconectarse

### MemoryCache (Buffer de Consola)

```go
type MemoryCache struct {
    Buffer   []cacheMessage  // msg []byte + time (UnixMicro)
    Capacity int             // en bytes = daemon.console.buffer * 1024 (KB)
    Size     int
    Lock     sync.RWMutex
}
```

- Buffer circular con capacidad en **bytes** (`daemon.console.buffer`, default 50, en KB)
- Evicta las entradas más viejas cuando excede la capacidad
- Cuando un cliente se conecta, recibe primero el buffer histórico (con `ReadFrom(startTime)`)

## Daemon ↔ Panel WebSocket (Proxy)

El WebSocket del cliente se conecta al **panel**, y el panel hace de **proxy** hacia el daemon del nodo:

- El handler `/api/servers/:serverId/socket` detecta el nodo del servidor.
- Si el nodo es local, el panel atiende directamente.
- Si es remoto, `services.Node.OpenSocket()` conecta vía `websocket.Dialer` a `ws[s]://<daemon>:<port>/daemon/server/<serverId>/socket`, autenticando con el JWT de daemon (`Authorization: Bearer <token>` generado por `services/token.go`), y hace puente bidireccional entre el cliente y el daemon.

`daemon.console.forward` NO está relacionado con este proxy: simplemente reenvía la consola del servidor a la salida estándar del daemon (ver `pkg/skypanel/environment.go:CreateWrapper()`).

## RCON WebSocket

Además del WebSocket de consola, existe RCON sobre WebSocket en `internal/connections/rconws.go` para conexiones RCON desde el navegador.
