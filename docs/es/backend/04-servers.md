# Servidores — Entornos, Ciclo de Vida y Scheduler

## Arquitectura

El sistema de servidores se basa en el concepto de **entornos** (`Environment`). Un entorno es una abstracción que define **cómo** se ejecuta un proceso de servidor de juego. Hay dos implementaciones concretas:

## Entornos Soportados

| Entorno | Factory | Descripción |
|---|---|---|
| `host` | `tty.EnvironmentFactory` | Ejecuta directamente en el host (sin contenedor) |
| `tty` | `tty.EnvironmentFactory` | Ejecuta en una pseudoterminal (PTY) en el host |
| `standard` | `tty.EnvironmentFactory` | Sin PTY, redirección de stdout/stderr |
| `docker` | `docker.EnvironmentFactory` | Ejecuta dentro de un contenedor Docker |

### TTY Environment (`internal/servers/tty/`)

- Ejecuta procesos directamente en el sistema operativo host.
- Usa `unshare` para aislamiento de namespaces (USER, mount, cgroup, IPC, UTS).
- Opciones de seguridad: `unshare` se puede deshabilitar con `security.disableUnshare` (config) o `disableUnshare` (por servidor).
- Implementa la interfaz `EnvironmentImpl` con:
  - `ExecuteAsyncImpl` — lanza el proceso con `os/exec` + PTY (si aplica)
  - `KillImpl` — envía señal de terminación
  - `GetStatsImpl` — lee `/proc` para CPU/memoria del proceso
  - `IsRunningImpl` — verifica estado del proceso
  - `GetUIDImpl` / `GetGidImpl` — UID/GID del proceso

### Docker Environment (`internal/servers/docker/`)

- Requiere el socket de Docker en el host.
- Cada servidor se ejecuta en un contenedor separado.
- Directorios: monta el `RootDirectory` del servidor en el contenedor.
- Networking: mapeo de puertos automático.
- Implementa la misma interfaz `EnvironmentImpl`.
- Archivos:
  - `docker.go` — creación y gestión de contenedores
  - `factory.go` — `EnvironmentFactory`
  - `container_mount_source.go` — montaje de bind mounts
  - `imagewriter.go` — descarga de imágenes

## Interfaz Environment

```go
// pkg/skypanel/environment.go
type Environment struct {
    Type            string            // "docker", "tty", "host", "standard"
    RootDirectory   string            // directorio raíz del servidor
    BackupDirectory string
    ConsoleBuffer   *MemoryCache      // buffer circular de logs
    Server          Server            // definición del servidor
    Implementation  EnvironmentImpl   // implementación concreta
    // Trackers pub/sub para WebSocket
    ConsoleTracker  *Tracker
    StatusTracker   *Tracker
    StatsTracker    *Tracker
}
```

```go
type EnvironmentImpl interface {
    ExecuteAsyncImpl(env *Environment, steps ExecutionData) error
    KillImpl(env *Environment) error
    GetStatsImpl(env *Environment) (*ServerStats, error)
    SendCodeImpl(env *Environment, code int) error
    GetUIDImpl(env *Environment) int
    GetGidImpl(env *Environment) int
    IsRunningImpl(env *Environment) (bool, error)
}
```

## Ciclo de Vida del Servidor

```
1. PUT /daemon/server/:serverId
   ├── servers.CreateEnvironment(type, folder, backupFolder, server)
   │   └── envMapping[type].Create() → EnvironmentImpl
   ├── Crea directorios (root, backup, etc.)
   └── Agrega al cache de servidores

2. POST /daemon/server/:serverId/install
   ├── server.Install()
   │   └── Ejecuta operaciones de instalación (descarga, extracción, etc.)
   └── Procesa server.Installation[]

3. POST /daemon/server/:serverId/start
   ├── server.Start()
   ├── Crea Environment si no existe
   ├── Verifica requirements
   └── ExecuteAsyncImpl(steps) → proceso asíncrono

4. POST /daemon/server/:serverId/stop
   ├── server.Stop()
   └── Envía señal de parada (stopCommand o stopCode)

5. POST /daemon/server/:serverId/kill
   ├── server.Kill()
   └── KillImpl() → SIGKILL

6. DELETE /daemon/server/:serverId
   ├── server.Delete()
   ├── Mata procesos
   ├── Elimina archivos
   └── Remueve del cache
```

## Cola de Procesos

El daemon mantiene una **cola FIFO** de operaciones de servidor para evitar sobrecarga:

```go
var queue *list.List        // cola de operaciones
var startQueueTicker *time.Ticker  // procesa cola cada 1s
var statTicker *time.Ticker        // estadísticas cada 5s
var systemStatusTicker *time.Ticker // estado del sistema cada 1min
```

- `InitService()` inicia 4 goroutines: `processQueue()`, `processStats()`, `processSystemStatus()`, `trackUptimeForAllServers()`
- `processStats()` recolecta métricas de todos los servidores cada 5s y notifica via StatsTracker
- `processSystemStatus()` recolecta métricas del sistema cada 1min
- `trackUptimeForAllServers()` registra uptime de servidores

## Scheduler (Tareas Programadas)

Cada servidor tiene un scheduler basado en **gocron** que persiste en `servers/{serverId}.cron`.

```go
type Scheduler struct {
    scheduler       gocron.Scheduler
    serverID        string
    Tasks           map[string]skypanel.Task  // tareas programadas
    Timezone        string                     // zona horaria
    ConcurrentLimit uint                       // límite de concurrencia
    LimitMode       string                     // modo de límite
}
```

- Se carga desde archivo JSON al iniciar el servidor
- Las tareas se definen como operaciones (command, console, etc.)
- Se pueden crear/editar/eliminar via API
- Integración con la cola de procesos

## Server Definition (JSON)

Los servidores se definen mediante un JSON estructurado (`pkg/skypanel/server.go`):

```go
type Server struct {
    Type                    string                    // "generic", "minecraft", etc.
    Identifier              string                    // ID único del servidor
    Display                 string                    // nombre mostrado
    Icon                    string                    // icono
    Variables               map[string]Variable       // variables de configuración
    Groups                  []Group                   // grupos de variables (UI)
    Installation            []ConditionalMetadataType  // pasos de instalación
    Uninstallation          []ConditionalMetadataType  // pasos de desinstalación
    Execution               Execution                 // configuración de ejecución
    Environment             MetadataType              // metadatos del entorno
    SupportedEnvironments   []MetadataType            // entornos soportados
    Requirements            Requirements              // requisitos del sistema
    Stats                   MetadataType              // configuración de estadísticas
    Query                   MetadataType              // configuración de query
    KeepAlive               KeepAlive                 // keepalive del proceso
}
```

## KeepAlive

Mantiene el proceso vivo enviando comandos periódicos:

```go
type KeepAlive struct {
    Frequency string   // intervalo (ej: "5m")
    Command   string   // comando a ejecutar
}
```

## Cache de Servidores

El daemon mantiene un cache en memoria de los servidores activos. Cuando el panel se comunica con el daemon, envía la definición completa del servidor.

## Alerta de Cambio de Estado

El sistema trackea cambios de estado (running ↔ stopped) y puede enviar notificaciones vía Discord webhook.
