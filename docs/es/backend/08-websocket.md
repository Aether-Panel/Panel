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

### Tipo: `console`
```json
{
  "type": "console",
  "data": "[10:30:15] [Server thread/INFO]: Starting server"
}
```
Línea de consola del servidor de juego. El frontend aplica parseo de colores ANSI.

### Tipo: `stats`
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
Estadísticas periódicas (cada ~15 segundos). CPU como porcentaje, memoria en bytes.

### Tipo: `status`
```json
{
  "type": "status",
  "data": {
    "running": true
  }
}
```
Cambio de estado del servidor (running/stopped/installing).

## Mensajes del Cliente → Servidor

### Enviar Comando
```json
{
  "type": "console",
  "data": "say Hello World!"
}
```
Envía un comando a la consola del servidor de juego.

## Sistema de Trackers (Pub/Sub Interno)

Los WebSocket usan un sistema de **trackers** definido en `pkg/skypanel/tracker.go`:

```go
type Tracker struct {
    listeners map[string]chan interface{}  // canales por topic
    lock      sync.RWMutex
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

### MemoryCache (Buffer de Consola)

```go
type MemoryCache struct {
    entries []interface{}
    max     int
}
```

- Buffer circular de tamaño configurable (`daemon.console.buffer`, default 50)
- Almacena las últimas N líneas de consola
- Cuando un cliente se conecta, recibe primero el buffer histórico

## Daemon ↔ Panel WebSocket

El daemon también puede conectar vía WebSocket al panel para reenviar consola (habilitado con `daemon.console.forward`). Esto permite que el panel central tenga acceso a la consola aunque el daemon esté en un nodo separado.

## RCON WebSocket

Además del WebSocket de consola, existe RCON sobre WebSocket en `internal/connections/rconws.go` para conexiones RCON desde el navegador.
