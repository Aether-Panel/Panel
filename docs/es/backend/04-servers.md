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
- **Red por defecto**: detección automática de `skypanel-network` (ver [Conceptos: Red Docker](./../../general/concepts.md#red-docker-auto-deteccion-skypanel-network)).
- Implementa la misma interfaz `EnvironmentImpl`.
- Archivos:
  - `docker.go` — creación y gestión de contenedores
  - `factory.go` — `EnvironmentFactory`
  - `container_mount_source.go` — montaje de bind mounts
  - `imagewriter.go` — descarga de imágenes

## ExtraPortBindings — Puertos Extra en Contenedores

La función `ExtraPortBindings()` en `docker.go` genera automáticamente los bindings Docker para todos los puertos extra (`port2`, `port3`, ...):

```go
func ExtraPortBindings(variables map[string]interface{}) []string {
    var specs []string
    for i := 2; ; i++ {
        key := fmt.Sprintf("port%d", i)
        val, ok := variables[key]
        if !ok { break }
        p := cast.ToString(val)
        if p == "" || p == "0" { continue }
        specs = append(specs,
            "0.0.0.0:"+p+":"+p+"/tcp",
            "0.0.0.0:"+p+":"+p+"/udp",
        )
    }
    return specs
}
```

- Busca variables `port2`, `port3`... en `ExecutionData.Variables`.
- Genera bindings `0.0.0.0:PUERTO:PUERTO/tcp` y `/udp` para cada uno.
- Se invoca en `createContainer()` junto con los puertos del template.
- `checkPortConflicts()` también lo usa para validar antes de iniciar.

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

Un entorno puede implementar además `Reattacher`, para volver a conectarse a un proceso que siguió en marcha mientras el daemon estaba parado. Solo lo implementa Docker (ver [Reconexión tras reiniciar el daemon](#reconexión-tras-reiniciar-el-daemon)):

```go
type Reattacher interface {
    ReattachImpl(env *Environment, steps ExecutionData) error
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
   ├── Disco lleno → 409 ErrDiskLimitExceeded (ver Límite de disco)
   ├── Conflicto de puertos → 409
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

- `InitService()` inicia 6 goroutines: `processQueue()`, `processStats()`, `processPlayers()`, `processSystemStatus()`, `reattachRunningServers()`, `trackUptimeForAllServers()`
- `processStats()` recolecta métricas de todos los servidores cada 5s y notifica via StatsTracker
- `processPlayers()` guarda cada minuto el número de jugadores de los servidores en marcha
- `processSystemStatus()` recolecta métricas del sistema cada 1min
- `reattachRunningServers()` se reconecta a los servidores que siguieron en marcha durante un reinicio del daemon
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

## Límite de disco

El límite es la variable `disk` del servidor, en MB. Sin `disk`, o con `0`, no hay límite. Código: `internal/servers/disk.go`.

### Cómo se mide

- El daemon recorre la carpeta del servidor y suma el tamaño de sus archivos.
- La medida vale 15 segundos. Lo que escribe el panel (subidas, SFTP, extracción, plugins) se suma al momento. Lo que escribe el propio juego solo se ve en la siguiente medida.
- Al borrar archivos (desde el panel o por SFTP) la medida se descarta, así que el espacio liberado cuenta enseguida.

### Qué se bloquea

Un servidor que ya ha usado todo su disco no arranca y no acepta archivos nuevos hasta que se borre algo. La API del daemon contesta `409` con el error `ErrDiskLimitExceeded`.

| Operación | Comportamiento |
|-----------|----------------|
| Arrancar / reiniciar | La API lo rechaza con `409` antes de arrancar. `Start()` vuelve a medir la carpeta y, si está llena, escribe en la consola: *The server has used all of its disk space. Delete files to free some before starting it.* |
| Subir un archivo (`PUT` de archivo) | Se rechaza si el disco ya está lleno. Cada byte cuenta: si la subida pasa del límite, se corta, el archivo a medias se borra y la respuesta es `409`. Al sobrescribir un archivo, su tamaño anterior deja de contar. |
| Escribir por SFTP | Se rechaza si el disco ya está lleno. Solo cuenta lo que hace crecer el archivo; reescribir bytes que ya tenía no cuenta. Ver abajo qué pasa con una subida cortada. |
| Extraer un archivo comprimido | Se rechaza si el disco ya está lleno. Cada archivo extraído cuenta: la extracción se detiene al llegar al límite (`409`) y los archivos ya extraídos se quedan. |
| Comprimir | Se rechaza si el disco ya está lleno. El archivo comprimido cuenta mientras se escribe: si pasa del límite, se borra y la respuesta es `409`. |
| Instalar un plugin | Se rechaza si el disco ya está lleno. Si la descarga pasa del límite, el archivo a medias se borra y la respuesta es `409`. |

### Subidas por SFTP cortadas por el límite

Cuando una escritura por SFTP pasaría del límite, falla y no escribe nada. Al cerrar el archivo:

- Si la subida empezó desde un archivo vacío (el cliente abrió con truncado, lo normal al subir un archivo), el archivo a medias **se borra**: medio archivo no sirve y mantendría el disco lleno.
- Si era una subida reanudada o que añadía al final (sin truncado), el archivo se conserva.

En los dos casos la medida de disco se descarta y se vuelve a medir.

### Servidor en marcha

Además, cada 5 segundos `processStats()` compara el uso de disco con el límite. Si un servidor en marcha lo supera en más de 1 MB, se para (`Stop()`, y `Kill()` si falla) y se avisa en la consola. En Docker el tamaño de la carpeta que usan las estadísticas se recalcula como mucho cada 30 segundos.

## Prioridad de disco (`io`)

La variable `io` reparte la lectura y escritura de disco entre los servidores del mismo nodo. Solo se aplica en el entorno Docker, como `BlkioWeight` del contenedor.

| Valor | Efecto |
|-------|--------|
| 10 a 1000 | Peso del servidor frente a los demás. `500` es un reparto igual. |
| Menor que 10 / mayor que 1000 | Se ajusta a 10 / 1000. |
| `0`, vacío o no numérico | No se fija peso; Docker usa su valor por defecto. |

Docker solo lo aplica si el planificador de E/S del host admite pesos. El valor se lee al crear el contenedor, así que un cambio cuenta desde el siguiente arranque. En el panel aparece como **Prioridad de disco** junto a CPU, memoria y disco (por defecto `500`).

## Reconexión tras reiniciar el daemon

Un contenedor Docker sigue en marcha aunque el daemon se reinicie (una actualización, un fallo). En una instalación con panel y daemon juntos, reiniciar el panel también reinicia el daemon. Código: `internal/servers/reattach.go`.

Al arrancar, el daemon se reconecta en segundo plano a cada servidor que sigue en marcha (`reattachRunningServers()`):

- La consola vuelve a recibir la salida del contenedor y acepta comandos.
- La salida del proceso pasa por el mismo `afterExit` que un arranque normal (una sola vez): detección de caídas, reinicio automático, avisos.
- Si el servidor tiene KeepAlive, se vuelve a activar.
- `startedAt` toma la hora de la reconexión. Solo retrasa el momento en que se olvidan caídas anteriores.
- En la consola aparece *Reconnected to the running server after a daemon restart*.

`Stop()`, `Kill()` y enviar un comando también se reconectan primero, por si llegan antes que la pasada en segundo plano. Un candado (`attachMu`) evita que dos reconexiones se hagan a la vez.

No hace nada si el servidor está parado o ya conectado. Un proceso del host (`tty`, `host`, `standard`) no se puede recuperar: devuelve `ErrNotImplemented` y no se registra nada. Si la reconexión falla, se registra el error y el servidor queda como estaba; vuelve a estar accesible cuando se reinicia.

## Subdominio (Cloudflare)

Un servidor puede tener un subdominio en la zona de Cloudflare del administrador (por ejemplo `survival.play.example.com`). Los registros son solo DNS (sin proxy), porque el tráfico de juego no es HTTP:

- Un registro `A`, `AAAA` o `CNAME` con el nombre del subdominio, que apunta a la dirección del nodo.
- Para servidores de tipo `minecraft` o `minecraft-java` con puerto, un registro `SRV` (`_minecraft._tcp.<subdominio>`) con el puerto principal.

El subdominio sigue al servidor. Estos cambios actualizan sus registros:

| Cambio | Ruta |
|--------|------|
| Puerto principal o tipo del servidor | `PUT /api/servers/:serverId/definition` |
| Puerto principal (administrador) | `PUT /api/servers/:serverId/data` |
| Puerto principal elegido en los ajustes de puertos | `PUT /api/servers/:serverId/port-settings` |
| Traslado a otro nodo | transferencia interna entre nodos |

La actualización (`refreshSubdomainRecords` en `internal/web/api/server_dns.go`):

- Corre en segundo plano, medio segundo después de guardar el cambio.
- Edita los registros del servidor en su sitio; el nombre del subdominio no cambia.
- Si el nodo pasa a estar detrás de un nombre, el registro `A` pasa a `CNAME`.
- Crea o borra el registro `SRV` cuando cambia la regla (por ejemplo, el tipo deja de ser Minecraft).
- Solo toca los registros que el servidor tiene guardados.
- Es de mejor esfuerzo: si Cloudflare falla, se registra el error y el cambio del servidor no se deshace.

No hace nada si el servidor no tiene subdominio o si Cloudflare no está configurado.
