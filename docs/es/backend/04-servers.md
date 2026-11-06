# Servidores — Entornos, ciclo de vida y tareas programadas

Esta página describe cómo ejecuta el daemon los servidores de juego (`internal/servers`, `internal/servers/docker`, `internal/servers/tty`). Las rutas se resumen aquí; la lista completa está en la [referencia de la API](../reference/11-api-reference.md).

## Arquitectura

Cada servidor de juego es una definición (`pkg/skypanel.Server`) más un **entorno** (`Environment`) que decide **cómo** se ejecuta su proceso. Al arrancar, el daemon carga en una caché en memoria cada archivo regular `<id>.json` de `daemon.data.servers` (`servers.LoadFromFolder`); las definiciones que son enlaces simbólicos se omiten. La base de datos del panel solo guarda la fila del servidor (nombre, nodo, dueño, puertos, límites); la definición y los archivos viven en el nodo.

El panel nunca toca los archivos de un servidor por sí mismo: las rutas `/api/servers/{id}/...` comprueban los permisos de quien llama y reenvían la petición a `/daemon/server/{id}/...` del nodo con un token que firma el panel (ver [Capa API](./05-api-layer.md#peticiones-del-panel-a-los-nodos)).

## Entornos soportados

`internal/servers/env_loader.go` registra cuatro tipos de entorno:

| Tipo | Factory | Qué se ejecuta |
|---|---|---|
| `host` | `tty.EnvironmentFactory` | Un proceso en el propio nodo, dentro de una pseudoterminal |
| `tty` | `tty.EnvironmentFactory` | Alias de `host` |
| `standard` | `tty.EnvironmentFactory` | Alias de `host` |
| `docker` | `docker.EnvironmentFactory` | Un contenedor Docker por servidor |

Los tres tipos de host comparten implementación, y el tipo del entorno se guarda como `host`. `docker.disallowHost` los rechaza (su lista incluye también `bubblewrap`, que no está registrado como entorno). La imagen Docker y el compose lo activan, así que las instalaciones con Docker solo ejecutan contenedores.

### Entorno host (`internal/servers/tty/`)

- Arranca el proceso con `os/exec` dentro de una pseudoterminal (`creack/pty`). El comando de arranque se divide en argumentos sin pasar por un shell; se rechaza un comando con un NUL o un salto de línea.
- Aísla el proceso con espacios de nombres `unshare` (usuario, montajes, archivos, cgroup, IPC, UTS) y solo monta la carpeta del servidor, los binarios y la caché. `security.disableUnshare` (global) o `disableUnshare` (por servidor) lo desactivan. Ver [unshare](../general/20-security-unshare.md).
- Lee CPU y memoria de `/proc`, como mucho cada 5 segundos.
- No puede reconectarse a un proceso tras reiniciar el daemon.

### Entorno Docker

Archivos: `docker.go` (vida del contenedor), `factory.go`, `container_mount_source.go` (ruta en el host de la carpeta de datos cuando el daemon corre en un contenedor), `imagewriter.go` (salida de la descarga), `user.go` (usuario, capacidades, límite de procesos) y `prefetch.go` (imágenes de ejecución).

Cuando un servidor arranca, `createContainer`:

- Descarga la imagen de la plantilla si el nodo no la tiene y muestra el progreso en la consola. Un arranque se puede cancelar mientras se descarga la imagen.
- Crea un contenedor **con el ID del servidor como nombre**, con la etiqueta `skypanel.server=<id>`, `AutoRemove`, TTY y stdin conectados y plataforma Linux.
- Monta la carpeta del servidor en la raíz del contenedor que indica la plantilla (`/SkyPanel` por defecto), `<id>.tmp` en `/tmp` y `/var/tmp`, la carpeta de binarios en `/var/lib/SkyPanel/binaries` y los montajes propios de la plantilla. Cuando el daemon corre en un contenedor, las rutas del host salen de `docker.root` o del propio montaje de `daemon.data.root` del daemon.
- Elige la red: la de la plantilla; si no indica ninguna (o indica `bridge`), `docker.serversNetwork` cuando esa red existe y, si no, la red del contenedor del panel (`bridge` fuera de Docker). Ver [Conceptos: red Docker](../general/concepts.md).
- Publica los puertos de la plantilla, cada variable de puerto extra (`port2`, `port3`…, TCP y UDP, ver abajo) y, si la plantilla lo usa, el puerto de stdin en `127.0.0.1`.
- Aplica los límites de recursos a partir de las variables del servidor:

| Variable | Ajuste de Docker |
|---|---|
| `cpu` | `NanoCPUs = cpu × 10⁷`: `100` es un núcleo, `200` dos. `0` o vacío significa sin límite. |
| `memory` | Memoria en MB. El swap se fija al mismo valor, así que el contenedor no tiene swap. |
| `io` | `BlkioWeight`, ver [Prioridad de disco](#prioridad-de-disco-io). |

- Aplica el endurecimiento que se describe abajo.

`docker.prefetchImages` descarga las imágenes de ejecución en segundo plano en cada nodo Docker, dos minutos después de arrancar y luego a diario, y borra una copia sustituida cuando ningún contenedor la usa.

#### Usuario y aislamiento del contenedor

Cuando el panel corre como root (la instalación normal con Docker), los juegos **no** corren como root, igual que en Pterodactyl:

- **El juego** corre con `docker.serverUser` (`988:988` por defecto). **La instalación** corre como root, porque sus scripts instalan paquetes. Si la plantilla indica su propio usuario, se respeta. Si el panel no corre como root, todo corre con su usuario.
- **Antes de cada arranque**, el panel da todos los archivos de la carpeta del servidor a ese usuario. El recorrido va carpeta a carpeta mediante descriptores abiertos y nunca sigue enlaces simbólicos. Así se migran solos los servidores creados cuando los juegos corrían como root.
- **La carpeta del servidor** es de ese usuario desde que se crea, y lo que el panel escribe (gestor de archivos, SFTP, copias, restauraciones) nace con él.
- **Capacidades:** el contenedor del juego pierde `setpcap`, `mknod`, `audit_write`, `net_raw`, `dac_override`, `fowner`, `fsetid`, `net_bind_service`, `sys_chroot` y `setfcap` (la lista de Wings) y lleva `no-new-privileges`. Los contenedores que corren como root (instalaciones) solo pierden `setpcap`, `mknod`, `audit_write`, `net_raw` y `sys_chroot` y conservan lo que hace falta para instalar paquetes. Las listas propias de la plantilla se mantienen.
- **Límite de procesos:** cada contenedor tiene `docker.pidsLimit` procesos e hilos (4096 por defecto) salvo que la plantilla fije el suyo, para que una «fork bomb» no tumbe el nodo.
- **`/tmp` y `/var/tmp`** son una carpeta junto a la del servidor (`<id>.tmp`): se vacía al crear cada contenedor (sticky, modo 1777, del usuario del contenedor), cuenta en el límite de disco y se borra con el servidor. No aparece en el gestor de archivos ni en las copias de seguridad.
- **Enlaces simbólicos del juego:** el panel trabaja como root, así que nunca sigue un enlace que haya dejado el juego. Las descargas de instalación, descomprimir, comprimir, las copias de seguridad y el cambio de dueño del arranque abren cada archivo dentro de la carpeta del servidor; un enlace se guarda como enlace y un FIFO no bloquea nada.
- **Lo que el dueño no cambia:** memoria, CPU, disco, IP y puertos los fija un administrador aunque la plantilla los marque como editables. Un sub-servidor solo publica sus propias variables de puerto, sin etiquetas ni un usuario de contenedor elegido por el cliente.

#### Puertos extra (`ExtraPortBindings`)

`ExtraPortBindings()` en `docker.go` convierte las variables de puerto extra en publicaciones de Docker, para que todos los puertos asignados a un servidor sean accesibles, digan lo que digan los `portBindings` de la plantilla:

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

- Se detiene en el primer `portN` que falta y omite los valores vacíos o `0`.
- `createContainer()` los suma a los puertos de la plantilla; la comprobación de conflictos al arrancar (`CheckBindingsConflict`) usa la misma lista.
- El panel escribe esas variables: cuando un administrador guarda la lista de puertos (`PUT /api/servers/{id}/data`) o el dueño elige el puerto principal (`PUT /api/servers/{id}/port-settings`), el daemon recibe `ports` y `primaryPort` y reescribe `port`, `port2`, `port3`…, quitando los que sobran. Ver [Capa API: puertos](./05-api-layer.md#gestión-de-puertos).

#### Prioridad de disco (`io`)

La variable `io` reparte las lecturas y escrituras de disco entre los servidores del mismo nodo. Solo se aplica al entorno Docker, como `BlkioWeight` del contenedor.

| Valor | Efecto |
|-------|--------|
| De 10 a 1000 | Peso del servidor frente a los demás. `500` es un reparto equitativo. |
| Menos de 10 / más de 1000 | Se ajusta a 10 / 1000. |
| `0`, vacío o no numérico | No se fija peso; Docker usa su valor por defecto. |

Docker solo lo aplica si el planificador de E/S del host admite pesos. El valor se lee al crear el contenedor, así que un cambio cuenta desde el siguiente arranque. En el panel aparece como **Prioridad de disco** junto a CPU, memoria y disco (por defecto `500`).

## Interfaz del entorno

```go
// pkg/skypanel/environment.go
type Environment struct {
    Type            string          // "docker" o "host"
    RootDirectory   string          // carpeta del servidor (debe quedar dentro de daemon.data.servers)
    BackupDirectory string          // <backups>/<id>
    ConsoleBuffer   *MemoryCache    // salida reciente de la consola
    ServerID        string
    ConsoleTracker  *Tracker        // pub/sub para el websocket
    StatusTracker   *Tracker
    StatsTracker    *Tracker
    Server          Server          // la definición
    Implementation  EnvironmentImpl // docker o tty
    DiskUsage       func() int64    // tamaño de la carpeta, cacheado
    // ...
}

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

Dos interfaces opcionales, ambas implementadas solo por Docker:

```go
type StartCanceler interface { CancelStartImpl() }                                    // detener un arranque cuya imagen aún se descarga
type Reattacher interface { ReattachImpl(env *Environment, steps ExecutionData) error } // reconectar tras reiniciar el daemon
```

## Ciclo de vida del servidor

| Paso | Ruta del daemon | Qué ocurre |
|---|---|---|
| Crear | `PUT /daemon/server/{id}` | Valida la definición, rechaza un entorno host con `docker.disallowHost`, crea el entorno y las carpetas, guarda `<id>.json` y añade el servidor a la caché. Después el panel lanza la instalación si `panel.servers.autoInstall` está activo, y pide un subdominio si lo está `panel.dns.autoAssign`. |
| Instalar | `POST /daemon/server/{id}/install` | Solo una instalación a la vez por servidor. Se rechaza con `docker.disallowHost` para entornos host, y con `ErrNodeDiskFull` cuando el nodo tiene menos espacio libre que `daemon.minFreeDiskMB`. Detiene el servidor si está en marcha y ejecuta las operaciones `install` de la plantilla (ver [Operaciones](./06-operations.md)). `?wait` espera a que termine. |
| Arrancar | `POST /daemon/server/{id}/start` | Se rechaza mientras el servidor está ocupado (instalando, haciendo o restaurando una copia), para un entorno host con `docker.disallowHost`, con el disco lleno (`409`, ver [Límite de disco](#límite-de-disco)) y con un puerto ocupado (`409 ErrPortBusy`, con el puerto en `metadata.port`). El panel lo rechaza para un servidor suspendido. El proceso arranca de forma asíncrona. |
| Reiniciar | `POST /daemon/server/{id}/restart` | Detener, esperar a que salga, arrancar. |
| Detener | `POST /daemon/server/{id}/stop` | Envía la señal `stopCode` de la plantilla o escribe su `stopCommand` en la consola. |
| Matar | `POST /daemon/server/{id}/kill` | Mata el proceso (`SIGKILL` / kill del contenedor). |
| Redefinir | `PUT /daemon/server/{id}/definition` | Sustituye la definición (mismas comprobaciones de entorno que al crear). |
| Recargar | `POST /daemon/server/{id}/reload` | Vuelve a cargar de disco la definición y las tareas programadas; solo con el servidor inactivo. |
| Borrar | `DELETE /daemon/server/{id}` | Lo mata si está en marcha, detiene el planificador, ejecuta las operaciones de desinstalación de la plantilla (si fallan, se registra en el log y se muestra en la consola, y el borrado continúa), borra el entorno y después elimina `<id>.json`, la carpeta `<id>.tmp`, los archivos asociados (`.cron`, `.crashes`, `.notify`, `.offsite`, `.metrics`, `.worldmap`, `.modpack`) y el historial de archivos. Por último, `servers.DeleteBackups` elimina `<backups>/<id>/`, salvo que la petición lleve `?keepBackups=true`: un traslado entre nodos que se deshace lo usa para quitar el servidor del nodo nuevo sin tocar las copias que ya estaban allí (las copias que trajo se borran una a una). |

Una parada o un kill pedidos nunca cuentan como caída. Cuando el panel se detiene, pide a todos los servidores que se detengan a la vez y da a cada uno hasta un minuto antes de matarlo (`docker-compose.yml` da 90 segundos al contenedor del panel).

## Bucle de servicio

`servers.InitService()` lanza siete goroutines (`internal/servers/server.go`):

| Goroutine | Intervalo | Función |
|---|---|---|
| `processQueue()` | 1 s | Arranca los servidores encolados con `StartViaService` (arranque automático al iniciar, reinicios automáticos), una cola por nodo. |
| `processStats()` | 5 s | Recoge las estadísticas de cada servidor, las publica en el tracker de estadísticas, registra el historial de rendimiento, comprueba el límite de disco y las alertas de recursos, y registra el uptime. |
| `processPlayers()` | 1 min | Registra los jugadores conectados de cada servidor en marcha que responde a consultas. |
| `processSystemStatus()` | 1 min | Envía el resumen del sistema al webhook de sistema de Discord (solo en el panel, si está configurado; enumera los servidores que ejecuta el propio panel y, con más de 25, enumera 24 y una nota), y un informe por nodo al webhook de nodos cuando está definido. Los recuentos de servidores de un nodo remoto salen del sondeo de estado de abajo (o de una llamada directa cuando su última respuesta tiene más de 60 s) y su hardware, de su propio `/daemon/system` (10 s de límite; sin la frecuencia de la CPU); un nodo que no responde recibe un informe rojo «sin respuesta» con el número de servidores que tiene asignados. El nodo del panel se lee de su propia máquina. |
| `processRemoteStatus()` | 30 s | Solo en el panel: pide a cada nodo remoto el estado de todos sus servidores (`GET /daemon/servers/status`, una petición por nodo, 10 s de límite, las rondas nunca se solapan), registra su uptime y guarda la respuesta de cada nodo para el informe de nodos. |
| `reattachRunningServers()` | una vez | Se reconecta a los contenedores que siguieron en marcha durante un reinicio del daemon. |
| `trackUptimeForAllServers()` | una vez (y en cada ronda de estadísticas) | Registra en la base de datos del panel el uptime de los servidores que ejecuta este proceso; no hace nada en un nodo sin panel. Los servidores de los nodos remotos los registra `processRemoteStatus()`. |

Más detalle en [Gestión de procesos y métricas](./13-process-metrics.md).

## Caídas y reinicios automáticos

Lo que ve el dueño se describe en [Informes de caídas](../features/crash-reports.md).

Cuando el proceso termina, `afterExit` (`server.go`) decide qué ha pasado:

- Una salida con el `expectedExitCode` de la plantilla es una salida limpia. Una parada o un kill pedidos tampoco son una caída.
- Cualquier otra cosa es una **caída**: la consola muestra `Server crashed (exit code N)`, se guarda un informe de caída, se envían la notificación de caída del dueño y la alerta de Discord, y el tracker de estado emite un mensaje `crash` (el cliente web lo muestra como aviso emergente).
- Si la plantilla tiene `autoRestartFromCrash`, el servidor vuelve a arrancar mientras las caídas seguidas no lleguen a `daemon.data.crashLimit` (3). Un servidor que funcionó más de 10 minutos antes de caer empieza una cuenta nueva.
- Tras una salida limpia, el servidor solo se reinicia si la plantilla tiene `autoRestartFromGraceful`.
- Las operaciones `post` de la plantilla se ejecutan tras cada salida, con `success` y `exitCode` disponibles.

Un informe de caída (`<id>.crashes`, los 10 más recientes, `GET /api/servers/{id}/crashes`) guarda la hora, el código de salida, las últimas 60 líneas de consola (cada una cortada a 500 caracteres), si el servidor se reinició y el número de intento frente al límite.

Alertas de recursos: una CPU por encima del 80 % de lo que puede usar el servidor o una memoria por encima del 90 % envían una alerta como mucho cada 5 minutos por recurso. Una salida que nadie ha explicado (sin parada, salida limpia ni informe de caída) se avisa como «apagado» pasados 15 segundos.

## Copias de seguridad y puntos de restauración

Guía de uso: [Copias de seguridad](../features/backups.md).

| Tipo | Nombre de archivo | Quién la hace | Se conservan |
|---|---|---|---|
| Manual | `<uuid>.tar.gz` | `POST /api/servers/{id}/backup/create` | Hasta borrarlas; los usuarios están limitados por `panel.backups.maxPerServer` (cuentan las copias que están en el nodo, no las que solo están en el bucket) |
| Automática | `auto-<id-de-la-tarea>-<uuid>.tar.gz` (antes `auto-<uuid>.tar.gz`) | La operación `backup` de una tarea programada | En el nodo, las `keep` más recientes (5 por defecto, 50 como máximo; todas las `auto-` juntas); en el bucket, las `offsiteKeep` más recientes de esa tarea (4 por defecto) |
| Periódica | `offsite-<uuid>.tar.gz` | El trabajador de copias periódicas del nodo | Solo en el bucket: las `periodicKeep` más recientes del servidor (4 por defecto) |
| Punto de restauración | `restore-<motivo>-<uuid>.tar.gz` | El daemon, antes de un paso arriesgado | Las 3 más recientes |

- Las copias se guardan en `daemon.data.backups.folder/<id>/`. El archivo se escribe con un nombre oculto y se mueve a su sitio al terminar, así que una copia a medias nunca se lista, descarga ni restaura. Solo se ejecuta una copia o restauración a la vez por servidor.
- El panel registra una copia manual en cuanto se pide. Si después no se puede escribir su archivo, el nodo deja una marca oculta `.failed-<nombre>`; `GET /daemon/server/{id}/backup` la lista con `location: "failed"`, y la siguiente sincronización del panel borra el registro y llama a `DELETE` en el nodo, que elimina la marca. Las copias programadas no dejan marca (la tarea informa ella misma del fallo).
- Al borrar el servidor se borra su carpeta de copias en el nodo; las copias del bucket externo se quedan. Un traslado entre nodos copia al nodo nuevo, a través del panel, las copias guardadas en el nodo (`local` y `both`) (`PUT /daemon/server/{id}/backup/file`) y después borra el servidor y sus copias del nodo antiguo; las copias externas no se tocan. Ver [Traslados de servidores](../features/server-transfers.md).
- Los **puntos de restauración** se crean antes de instalar o quitar un plugin (como mucho uno cada 30 minutos en una serie de cambios de plugins; solo cuando Modrinth ofrece un archivo compatible o el plugin que se quita existe), instalar un mapa del mundo, cambiar la versión del juego, instalar un modpack y restaurar una copia. No cuentan en la retención de copias automáticas.
- **Restaurar** (`POST /api/servers/{id}/backup/restore/{backupID}`) solo funciona con el servidor detenido e inactivo. Primero se comprueba el archivo, se escribe un punto de restauración (sin él la restauración no sigue), se borran los archivos del servidor y se extrae la copia.
- Con las **copias externas** activas (`daemon.backups.s3.*`), una copia de tarea se sube además al bucket cuando su frecuencia (`offsite`) lo pide, en segundo plano y sin hacer fallar la tarea; las copias manuales y los puntos de restauración no se suben. `StartOffsitePeriodic` arranca con el daemon un único trabajador que cada 10 minutos (el primer recorrido, 5 minutos después de arrancar) recorre los servidores de uno en uno y hace la copia periódica de los que la tienen pendiente (`daemon.backups.s3.periodicHours` o el ajuste `<id>.offsite` del servidor): escribe `offsite-<uuid>.tar.gz`, lo sube, poda el bucket y borra el archivo local, avisa solo si algo falla y, tras un fallo, espera 1, 2, 4… hasta 24 h. Lo deja para otro recorrido si el servidor se instala, se traslada, tiene otra copia o restauración en curso o le toca una tarea dentro de la próxima hora (`Scheduler.TaskDueWithin`). `GET`/`PUT /daemon/server/{id}/offsite` leen y guardan el ajuste propio del servidor. La poda local (`keep`) no toca el bucket. La lista combina ambos sitios (`location`: `local`, `offsite` o `both`), una copia que solo está en el bucket se descarga antes de restaurarla y al borrar se eliminan las dos.

## Historial de archivos

Guía de uso: [Historial de archivos](../features/file-history.md).

Cada vez que se guarda desde el panel un archivo de configuración de texto, el daemon conserva su contenido anterior en `<servers>/<id>.history/` (fuera de la carpeta del servidor, para que el dueño no pueda verlo ni alterarlo desde ahí):

- Solo se guardan formatos de texto (`.properties`, `.yml`, `.yaml`, `.json`, `.json5`, `.toml`, `.conf`, `.cfg`, `.ini`, `.txt`, `.xml`, `.sk`, `.js`, `.ts`, `.lua`, `.mcmeta`, `.cnf`, `.env`), archivos de hasta 1 MB y nada que parezca binario.
- Hasta 20 versiones por archivo y 50 MB por servidor; las más antiguas se van primero.
- Restaurar una versión guarda antes el texto actual como versión, así que la restauración se puede deshacer.

Rutas: `GET /api/servers/{id}/file-history`, `GET .../file-history/{versionId}`, `POST .../file-history/{versionId}/restore`.

## Planificador (tareas programadas)

Guía de uso: [Tareas programadas](../features/scheduled-tasks.md).

Cada servidor tiene un planificador **gocron** guardado en `<servers>/<id>.cron` (escrito de forma atómica, modo `0600`):

```go
type Scheduler struct {
    Tasks           map[string]skypanel.Task `json:"tasks"`
    Timezone        string                   `json:"timezone,omitempty"`
    ConcurrentLimit uint                     `json:"concurrentLimit"`
    LimitMode       string                   `json:"limitMode"`
    Paused          bool                     `json:"paused,omitempty"`
}
```

- El planificador se carga y arranca cuando el daemon carga el servidor, no cuando el servidor arranca, así que las tareas se ejecutan también con el servidor detenido (una tarea `start` puede encenderlo).
- Un servidor admite como máximo 50 tareas (`MaxTasks`); se rechaza añadir una más.
- Las tareas son listas de operaciones. Además de las [operaciones](./06-operations.md) normales, las tareas pueden usar `start`, `stop`, `restart` (solo reinicia un servidor en marcha) y `backup` (copia automática con el argumento `keep`). `start`, `restart` y `backup` esperan hasta 60 minutos a que termine una copia o restauración en curso; `restart`, antes de detener el servidor.
- Las programaciones se ejecutan en la zona horaria de quien las creó (`CRON_TZ=`); el binario incluye la base de datos de zonas horarias.
- **Pausa** (`SetPaused`, `IsPaused` en `internal/servers/scheduler_pause.go`): mientras `Paused` vale true, `runScheduledTask` omite todas las ejecuciones, programadas o desde **Ejecutar ahora**, y registra `Task <id> skipped: the server is being moved to another node`; una tarea que ya se está ejecutando continúa. La marca se guarda en el archivo `.cron`, así que se mantiene si el nodo se reinicia. Solo la activa una transferencia entre nodos, mediante `POST /daemon/server/{id}/scheduler/pause` y `/scheduler/resume`: pausa las tareas en ambos nodos, las copia al nodo nuevo (`GET` de la lista y `PUT` de cada tarea; no se copian `Timezone`, `ConcurrentLimit` ni `LimitMode`) y las reanuda en el nodo que se queda con el servidor. Véase [Transferencias de servidores](../features/server-transfers.md).
- Rutas: `GET /api/servers/{id}/tasks`, `GET/PUT/DELETE .../tasks/{taskId}`, `POST .../tasks/{taskId}/run`.

## Definición del servidor (JSON)

`pkg/skypanel/server.go`:

```go
type Server struct {
    Type                  string                    // "generic", "minecraft-java"...
    Identifier            string                    // ID del servidor
    Display               string                    // nombre visible
    Icon                  string
    Variables             map[string]Variable       // variables de configuración
    Groups                []Group                   // grupos de variables (UI)
    Installation          []ConditionalMetadataType // pasos de instalación
    Uninstallation        []ConditionalMetadataType // pasos de desinstalación
    Execution             Execution                 // comando, parada, reinicios, pasos posteriores
    Environment           MetadataType              // entorno y sus ajustes
    SupportedEnvironments []MetadataType
    Requirements          Requirements
    Stats                 MetadataType
    Query                 MetadataType
    KeepAlive             KeepAlive
}
```

Los identificadores de servidor son seguros para URL y rutas (los nuevos tienen 12 caracteres hexadecimales en mayúsculas; los antiguos de 8 siguen siendo válidos, hasta 20 caracteres), porque se insertan en las URL del daemon y en nombres de archivo.

## KeepAlive

Mantiene activo el proceso enviando periódicamente un comando de consola:

```go
type KeepAlive struct {
    Frequency string // intervalo, p. ej. "5m"
    Command   string // comando a escribir
}
```

Cada ejecución tiene su propio ticker y canal de parada; se detiene cuando el proceso termina y vuelve a empezar tras una reconexión.

## Archivos de cada servidor en el nodo

Además de su carpeta, cada servidor tiene estas entradas en `daemon.data.servers`:

| Ruta | Contenido |
|---|---|
| `<id>/` | Carpeta del servidor |
| `<id>.json` | Definición |
| `<id>.tmp/` | `/tmp` y `/var/tmp` del contenedor Docker |
| `<id>.cron` | Tareas programadas |
| `<id>.crashes` | Informes de caída |
| `<id>.metrics` | Historial de rendimiento |
| `<id>.notify` | Alertas del dueño (canales y eventos), modo `0600` |
| `<id>.history/` | Historial de archivos |
| `<id>.modpack` | Modpack instalado |
| `<id>.worldmap` | Mapa del mundo instalado |

Todas se borran con el servidor. Al clonar solo se copian los archivos del servidor más `.modpack` y `.worldmap`.

**Alertas del dueño** (`GET/PUT /api/servers/{id}/notifications`, `POST .../notifications/test`, permiso `server.flags.edit`): los canales son un webhook de Discord, un bot y un chat de Telegram o un webhook JSON (opcionalmente firmado con un secreto); los eventos son caídas, copias de seguridad, recursos y encendido/apagado. La prueba se puede enviar una vez cada 10 segundos. Ver [Notificaciones del servidor](../features/server-notifications.md).

## Historial de rendimiento

Las muestras de 5 segundos se agrupan en puntos de un minuto (se conservan un día) y de 15 minutos (se conservan una semana): CPU, memoria, red, porcentaje de tiempo en marcha y jugadores. Se guardan en `<id>.metrics` cada 5 minutos y al detenerse el nodo. `GET /api/servers/{id}/metrics` ofrece las ventanas `1h` (pasos de 1 minuto), `24h` (pasos de 5 minutos) y `7d` (pasos de 1 hora); los pasos en los que el nodo estuvo caído se devuelven como `noData` en lugar de dibujarse de un lado a otro. Ver [Estadísticas del servidor](../features/server-statistics.md).

## Límite de disco

El límite es la variable `disk` del servidor, en MB. Sin `disk`, o con `0`, no hay límite. Código: `internal/servers/disk.go`.

### Cómo se mide

- El daemon recorre la carpeta del servidor y suma el tamaño de sus archivos. En Docker también cuenta su `/tmp` (la carpeta `<id>.tmp`).
- La medida se da por buena durante 30 segundos, y las estadísticas del servidor usan esa misma medida, así que la carpeta no se recorre dos veces. Lo que escribe el panel (subidas, SFTP, extracción, plugins) se suma al momento. Lo que escribe el propio juego solo se ve en la siguiente medida.
- Borrar archivos (desde el panel o por SFTP) descarta la medida, así que el espacio liberado cuenta enseguida.

### Qué se bloquea

Un servidor que ya ha usado todo su disco no arranca ni acepta archivos nuevos hasta que se borre algo. La API del daemon responde `409` con el error `ErrDiskLimitExceeded`.

| Operación | Comportamiento |
|-----------|----------------|
| Arrancar / reiniciar | La API lo rechaza con `409` antes de arrancar. `Start()` vuelve a medir la carpeta y, si está llena, escribe en la consola: *The server has used all of its disk space. Delete files to free some before starting it.* |
| Subida de archivo (`PUT` de archivo) | Se rechaza si el disco ya está lleno. Cada byte cuenta: si la subida supera el límite se corta, se borra el archivo parcial y la respuesta es `409`. Al sobrescribir un archivo, su tamaño anterior deja de contar. |
| Escrituras por SFTP | Se rechazan si el disco ya está lleno. Solo cuenta lo que hace crecer el archivo; reescribir bytes que ya tenía es gratis. Más abajo se explica qué pasa con una subida cortada. |
| Extraer un archivo comprimido | Se rechaza si el disco ya está lleno. Cada archivo extraído cuenta: la extracción se detiene en el límite (`409`) y lo ya extraído se queda. |
| Comprimir | Se rechaza si el disco ya está lleno. El archivo cuenta mientras se escribe: si supera el límite se borra y la respuesta es `409`. |
| Instalar un plugin | Se rechaza si el disco ya está lleno. Si la descarga supera el límite, se borra el archivo parcial y la respuesta es `409`. |

### Subidas por SFTP cortadas por el límite

Cuando una escritura por SFTP superaría el límite, falla y no escribe nada. Al cerrar el archivo:

- Si la subida empezó desde un archivo vacío (el cliente lo abrió con truncado, lo normal en una subida), el archivo parcial **se borra**: medio archivo no sirve y mantendría el disco lleno.
- Si era una subida reanudada o que añade al final (sin truncado), el archivo se conserva.

En ambos casos la medida del disco se descarta y se vuelve a tomar.

### Servidor en marcha

Además, cada 5 segundos `processStats()` compara el uso de disco con el límite. Un servidor en marcha que lo supera en más de 1 MB se detiene (`Stop()` y, si falla, `Kill()`) y se escribe un aviso en la consola. En Docker, el tamaño de la carpeta que usan las estadísticas se recalcula como mucho cada 30 segundos.

### Espacio libre del nodo

Aparte del límite de cada servidor, se vigila el disco del propio nodo:

- Se rechaza una instalación cuando el disco de `daemon.data.servers` tiene menos espacio libre que `daemon.minFreeDiskMB` (2048 MB). La consola indica cuánto hay libre y cuánto hace falta. Si no se puede leer el espacio libre, la instalación sigue adelante.
- El nodo informa de su disco como **bajo** cuando queda menos del 10 % o menos de 5 GB libres; entonces el panel avisa a los administradores.

## Reconexión tras reiniciar el daemon

Un contenedor Docker sigue en marcha cuando el daemon se reinicia (una actualización, un fallo). En una instalación en la que panel y daemon corren juntos, reiniciar el panel también reinicia el daemon. Código: `internal/servers/reattach.go`.

Al arrancar, el daemon se reconecta en segundo plano a cada servidor que sigue en marcha (`reattachRunningServers()`):

- La consola vuelve a recibir la salida del contenedor y acepta comandos.
- La salida del proceso pasa por el mismo `afterExit` que un arranque normal (una sola vez): detección de caídas, reinicio automático, notificaciones.
- Si el servidor tiene KeepAlive, se vuelve a iniciar.
- `startedAt` toma la hora de la reconexión. Solo retrasa el momento en que se perdonan las caídas anteriores.
- La consola muestra *Reconnected to the running server after a daemon restart*.

`Stop()`, `Kill()` y enviar un comando también se reconectan antes, por si llegan antes que la pasada en segundo plano. Un cerrojo (`attachMu`) impide que se ejecuten dos reconexiones a la vez.

No hace nada si el servidor está detenido o ya conectado. Un proceso host no se puede recuperar: devuelve `ErrNotImplemented` y no se registra nada. Si la reconexión falla, se registra el error y el servidor sigue como estaba; vuelve a estar accesible cuando se reinicia.

## Subdominio (Cloudflare)

Un servidor puede tener un subdominio en la zona de Cloudflare del administrador (por ejemplo `survival.play.example.com`), configurada con las claves `panel.dns.*` (ver [Configuración](./02-configuration.md#subdominios-de-servidores-dns-de-cloudflare)). Los registros son solo DNS (sin proxy), porque el tráfico de juego no es HTTP:

- Un registro `A`, `AAAA` o `CNAME` con el nombre del subdominio, que apunta al nodo: su IP pública; en el nodo local, `panel.settings.nodeIp` cuando es pública; si no, su nombre de host como `CNAME`, que puede ser de otro dominio (`pablo.example.es` → `usa.example.com`). `panel.dns.target` solo se usa para un nodo sin ninguna de esas direcciones.
- Para servidores de tipo `minecraft` o `minecraft-java` con puerto, un registro `SRV` (`_minecraft._tcp.<subdominio>`) con el puerto principal. Detrás de un `CNAME` apunta al nombre del nodo, nunca al alias.

El servidor guarda el subdominio, los ID de sus registros y el dominio base y la zona en los que se crearon (`subdomain`, `dns_record_ids`, `dns_domain`, `dns_zone_id`). La información del servidor (`ServerInfo`) lleva la dirección completa en `subdomain` y `subdomainSrv` cuando la dirección no necesita el puerto.

Rutas (permiso `server.flags.edit` o `server.data.edit`): `GET/PUT/DELETE /api/servers/{id}/subdomain` y `GET /api/servers/{id}/subdomain/check`. Cada servidor puede cambiar su subdominio una vez cada 10 segundos. Con `panel.dns.autoAssign` activo, un servidor nuevo recibe un nombre derivado del suyo (con `-2`, `-3`… si ya existe, hasta 20 intentos).

El subdominio sigue al servidor. Estos cambios actualizan sus registros:

| Cambio | Ruta |
|--------|------|
| Puerto principal o tipo de servidor | `PUT /api/servers/{id}/definition` |
| Puerto principal (administrador) | `PUT /api/servers/{id}/data` |
| Puerto principal elegido en los ajustes de puertos | `PUT /api/servers/{id}/port-settings` |
| Traslado a otro nodo | transferencia interna entre nodos |
| Host público del nodo | `PUT /api/nodes/{id}` (servidores de ese nodo) |
| IP del nodo maestro o URL base | `POST /api/settings`, `PUT /api/settings/{key}` (servidores del nodo local) |
| Destino DNS | `POST /api/settings`, `PUT /api/settings/{key}` (todos los servidores con subdominio) |

La actualización (`refreshSubdomainRecords` en `internal/web/api/server_dns.go`):

- Se ejecuta en segundo plano, medio segundo después de guardar el cambio.
- Edita los registros del servidor sin recrearlos; el nombre del subdominio no cambia.
- Si el nodo pasa a estar detrás de un nombre de host, el registro `A` se convierte en `CNAME`.
- Crea o quita el registro `SRV` cuando cambia su regla (por ejemplo, el tipo deja de ser Minecraft).
- Solo toca los registros que el servidor tiene guardados.
- Es de mejor esfuerzo: si Cloudflare falla, se registra el error y el cambio del servidor no se deshace.

No hace nada si el servidor no tiene subdominio o Cloudflare no está configurado. Los cambios que afectan a muchos servidores se aplican de uno en uno (`sweepSubdomains`).

Un dominio base o un ID de zona nuevos mueven todos los subdominios (`moveSubdomain`): se crea el mismo nombre bajo el dominio nuevo, se guarda y se eliminan los registros antiguos de la zona en la que se crearon. Si el nombre ya se usa en la zona nueva fuera del panel, ese servidor se queda donde estaba. Antes de guardar ese cambio, los servidores cuyo subdominio es anterior a `dns_domain` reciben los ajustes vigentes (`rememberSubdomainHome`).

Guía para usuarios y administradores: [Subdominios](../features/subdomains.md).

## Otras funciones por servidor

Cada una tiene su propia guía: [Versiones del juego](../features/game-versions.md), [Modpacks y plugins](../features/modpacks-and-plugins.md), [Clonado](../features/cloning.md), [Mapa del mundo](../features/world-map.md) y [Sub-servidores](../features/sub-servers.md).

| Función | Rutas | Notas |
|---|---|---|
| Cambio de versión del juego | `GET/POST /api/servers/{id}/version` | Para plantillas que descargan el juego con `paperdl` o `mojangdl`: detener, punto de restauración, versión nueva, reinstalar, arrancar. `POST` requiere `panel.experimental.versionSwitch`. Las listas de versiones se guardan en caché una hora. |
| Modpacks | `GET /api/servers/{id}/modpacks`, `.../search`, `.../versions`, `POST .../install` | Modrinth (`.mrpack`) o server packs de CurseForge (CurseForge requiere `daemon.curseforge.key`). Se ejecuta en segundo plano: detener, punto de restauración, limpieza, descarga, loader, versión de Java y del juego, arrancar. |
| Clonar | `POST /api/servers/{id}/clone` | Requiere `panel.experimental.serverClone`. Copia los archivos a un servidor nuevo en el mismo nodo; la copia queda marcada como instalándose hasta que termina. Se omiten los enlaces que salen del origen, los dispositivos y los sockets. |
| Mapa del mundo | `GET/DELETE /api/servers/{id}/map`, `POST .../map/install` | BlueMap o Dynmap desde Modrinth para Minecraft Java, mostrado a través del panel con un enlace firmado (`/api/map-view/...`). |
| Jugadores | `GET /api/servers/{id}/query` | Ping de la lista de servidores de Minecraft o A2S de Steam, en caché 10 segundos. |
| Plugins | `GET/DELETE /api/servers/{id}/plugins`, `.../plugins/search`, `POST .../plugins/{pluginId}` | Listar, instalar y borrar usan `mods/` en servidores Fabric, Forge y NeoForge y `plugins/` en el resto (la plataforma se detecta como para el mapa del mundo). La búsqueda en Modrinth se limita a esa plataforma: Paper recibe plugins y mods hechos para Paper, Spigot o Bukkit; Fabric, Forge y NeoForge, mods de servidor para su loader; una plataforma desconocida, todo. Instalar y quitar crean antes un punto de restauración. Un archivo de Modrinth cuyo nombre no es un `.jar` simple responde `502 ErrPluginBadFileName`. |
