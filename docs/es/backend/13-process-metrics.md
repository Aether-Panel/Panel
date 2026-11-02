# Gestión de procesos y métricas

Cómo ejecuta el daemon sus bucles en segundo plano, arranca y reinicia servidores, recoge estadísticas en vivo, guarda un historial de rendimiento por servidor y mide el propio nodo (CPU, discos, historial de red).

Páginas relacionadas: ciclo de vida, scheduler, keep-alive y límite de disco en [Servidores](./04-servers.md); los mensajes `stat` del WebSocket en [WebSocket](./08-websocket.md); el uptime como funcionalidad en [Uptime](../general/13-uptime.md). Guías de uso: [Estadísticas del servidor](../features/server-statistics.md), [Informes de caída](../features/crash-reports.md), [Dashboard y nodos](../features/dashboard-and-nodes.md).

---

## Bucles en segundo plano

`servers.InitService()` (`internal/servers/server.go`) lo llama `cmd/panel/run.go` después de cargar los servidores. Arranca:

| Goroutine | Intervalo | Qué hace |
|---|---|---|
| `processQueue()` | 1 s | Arranca el siguiente servidor que espera en la cola de arranque |
| `processStats()` | 5 s | `SendStatsForServers()` (estadísticas en vivo, historial, alertas, límite de disco) y `trackUptimeForAllServers()` |
| `processPlayers()` | 1 min | `samplePlayers()`: número de jugadores al historial |
| `processSystemStatus()` | 1 min | `sendSystemStatusToDiscord()` |
| `reattachRunningServers()` | una vez | Vuelve a conectar con los contenedores Docker que siguieron en marcha durante un reinicio del daemon ([Servidores](./04-servers.md)) |
| `trackUptimeForAllServers()` | una vez (después cada 5 s desde `processStats`) | Periodos de uptime |

`run.go` arranca además, fuera de `InitService`:

| Función | Intervalo | Qué hace |
|---|---|---|
| `daemon.StartNetworkHistory()` (`internal/web/daemon/network_history.go`) | 10 s | Historial de red del nodo |
| Muestreador de CPU (`internal/web/daemon/root.go`) | ventanas de 2 s | Lo arranca, de forma diferida, la primera llamada a `/daemon/system` |

`servers.ShutdownService()` detiene los tickers y escribe en disco todos los historiales de rendimiento (`SaveAllMetrics()`).

---

## Cola de arranque

```go
var queue *list.List   // valores *Server que esperan para arrancar
```

- `StartViaService(p)` añade un servidor. Se usa para los servidores con **autoarranque** al iniciar el daemon (`run.go`) y para los **reinicios automáticos** tras una salida (`afterExit`). Las peticiones de arranque desde el panel llaman directamente a `Server.Start()` y no pasan por la cola.
- `processQueue()` toma un servidor por segundo y llama a `Start()` si no está ya en marcha; los errores se registran en el log.
- No hay límite de concurrencia ni prioridades: es una FIFO simple que se vacía a un arranque por segundo.

---

## Gestión de la salida e informes de caída

Cuando el proceso termina, el entorno llama a `Server.afterExit(exitCode)`:

1. Detiene el envío de keep-alive.
2. Clasifica la salida: **limpia** cuando `exitCode == run.expectedExitCode`; **solicitada** cuando alguien pulsó detener/kill (`stopRequested`); **caída** en cualquier otro caso.
3. El contador de caídas se reinicia si la salida no fue una caída o si el servidor llevaba más de 10 minutos en marcha (`crashCounterResetAfter`).
4. Reinicio automático tras una caída: solo si `run.autorecover` es true y el contador está por debajo de `daemon.data.crashLimit` (por defecto `3`).
5. En una caída: línea de consola `Server crashed (exit code N)`, se guarda un informe de caída, saltan las notificaciones del propietario y se envía una alerta al webhook de Discord de administración.
6. Ejecuta las operaciones de `run.post` con `success` y `exitCode` en el mapa de datos.
7. Encola un reinicio (`StartViaService`) tras una caída con reinicio automático, o tras una salida limpia no solicitada cuando `run.autorestart` es true.

Los informes de caída (`internal/servers/crash.go`) se guardan en `<daemon.data.servers>/<id>.crashes`, los más recientes primero, como máximo 10 por servidor. Cada uno guarda las últimas 60 líneas no vacías de la consola (cada una recortada a 500 caracteres), el código de salida, si se reinició y el intento/límite. También se envía un mensaje `crash` (`{id, exitCode, restarted}`) por el WebSocket de estado del servidor. Cómo los ve el propietario: [Informes de caída](../features/crash-reports.md).

---

## Estadísticas en vivo

### `ServerStats` (`pkg/skypanel/httpmodels.go`)

```go
type ServerStats struct {
    CPU        float64         `json:"cpu"`        // porcentaje de UN núcleo (200 = dos núcleos ocupados)
    MaxCPU     float64         `json:"maxCpu"`     // lo que puede usar el servidor, misma unidad
    Memory     float64         `json:"memory"`     // bytes
    MaxMemory  float64         `json:"maxMemory"`  // bytes
    Disk       float64         `json:"storage"`    // bytes que ocupa la carpeta del servidor
    MaxStorage float64         `json:"maxStorage"` // bytes, 0 = sin límite
    NetworkRx  float64         `json:"networkRx"`  // KiB/s
    NetworkTx  float64         `json:"networkTx"`  // KiB/s
    Jvm        *utils.JvmStats `json:"jvm,omitempty"`
    Running    bool            `json:"running"`
}
```

**Cuota de CPU.** `stats.CPUPercent()` convierte `CPU` en una cuota de 0 a 100 sobre lo que el servidor puede usar: `CPU / MaxCPU * 100`, acotado. `MaxCPU` es la variable `cpu` del servidor (porcentaje de un núcleo, así que `200` = dos núcleos) o, sin límite, todos los núcleos del nodo × 100. Gráficas, tarjetas, alertas e historial usan ese mismo número, de modo que un servidor de dos núcleos a plena carga marca 100 %. Si `MaxCPU` es 0 se devuelve el `CPU` sin convertir.

### Cómo mide cada entorno

| | Docker (`internal/servers/docker/docker.go`) | Host: `tty`, `host`, `standard` (`internal/servers/tty/tty.go`) |
|---|---|---|
| Origen | `ContainerStats` con la muestra anterior | gopsutil sobre el proceso principal |
| CPU | Fórmula de Docker: `cpuDelta / systemDelta × onlineCPUs × 100`; 0 si los contadores retroceden (primera muestra de un contenedor nuevo) | `process.Percent(1s)` |
| Memoria | Uso **menos la caché de páginas inactiva** (`inactive_file` en cgroup v2 o `total_inactive_file` en v1), como lo muestra `docker stats` | RSS del proceso |
| MaxMemory | Límite de memoria del contenedor | RAM total del nodo |
| Red | Suma de las interfaces del contenedor | Contadores de todo el nodo (`psnet.IOCounters`), no solo del servidor |
| Disco | Tamaño de la carpeta del servidor, actualizado como mucho cada 30 s (la medida de disco propia del servidor cuando existe) | Igual |
| Caché | Una llamada a `ContainerStats` responde a todos durante **4 s** (`statsCacheFor`); la hora se toma antes de la llamada | Una lectura cada **5 s** |
| JVM | `jcmd` dentro del contenedor cuando el `stats.type` de la plantilla es `jcmd` | `jcmd` sobre el proceso |

Las tasas de red son KiB/s desde la muestra anterior. Un contador que retrocede (un contenedor nuevo, una interfaz reiniciada) empieza una cuenta nueva en lugar de producir una tasa enorme por desbordamiento. Un servidor detenido informa de ceros (`Running: false`) y reinicia la referencia de red.

### La pasada de estadísticas de 5 segundos

`SendStatsForServers()` se ejecuta en paralelo para cada servidor:

1. `GetStats()` (desde la caché anterior si está fresca).
2. Escribe un mensaje `stat` en el tracker de estadísticas del servidor (el WebSocket).
3. `recordMetrics()` añade la muestra al historial de rendimiento.
4. `checkServerAlerts()` (en su propia goroutine): cambios de encendido y uso alto de recursos.
5. `checkDiskLimit()` (en su propia goroutine): detiene un servidor en marcha cuya carpeta supera `MaxStorage` + 1 MB; véase [Servidores: límite de disco](./04-servers.md#límite-de-disco).

Los mismos números se sirven bajo demanda en `GET /api/servers/:id/stats` (scope `server.stats`), que se redirige al `GET /daemon/server/:id/stats` del daemon.

---

## Alertas

`checkServerAlerts()` mantiene un estado por servidor en memoria (`serverStateTracking`). La primera muestra tras arrancar el daemon solo registra el estado.

| Alerta | Condición | Repetición |
|---|---|---|
| Servidor en línea | Detenido → en marcha | En cada cambio |
| Servidor apagado | En marcha → detenido **sin** parada solicitada, salida limpia ni caída notificada, y sigue sin explicación tras 15 s (`offlineAlertDelay`) | En cada cambio |
| CPU alta | `CPUPercent() > 80` | Como mucho cada 5 min mientras siga alta |
| Memoria alta | `Memory / MaxMemory × 100 > 90` (si `MaxMemory` es 0, el propio `Memory` cuando es ≤ 100) | Como mucho cada 5 min mientras siga alta |

Adónde van:
- Canal de administración: `panel.notifications.discordWebhook` mediante `DiscordService` ([Servicios](./11-services.md#21-discordservice-discordgo)).
- Canales del propietario (Discord, Telegram, webhook JSON) si activó `power` o `resources` ([Notificaciones de servidor](../features/server-notifications.md)). Los eventos de encendido y de recursos se publican también a las extensiones suscritas.

Los umbrales son constantes en el código; no hay ningún ajuste para cambiarlos.

---

## Seguimiento del uptime

`trackUptimeForAllServers()` llama a `services.Uptime.TrackStatus(id, isRunning)` para cada servidor. Solo se ejecuta donde corre el panel (`panel.enable`): un nodo solo daemon no tiene base de datos del panel. Un indicador atómico evita que dos rondas se solapen si la base de datos va lenta. Almacenamiento y estadísticas: [Servicios: Uptime](./11-services.md#16-uptime-uptimego). Endpoints: `GET /api/uptime` (scope `admin` o `uptime.view`) y `GET /api/uptime/:serverId` (scope `server.view`).

---

## Historial de rendimiento (`internal/servers/metrics.go`)

Cada muestra de estadísticas y cada recuento de jugadores se incorpora a dos series por servidor (las gráficas que se construyen con ellas: [Estadísticas del servidor](../features/server-statistics.md)):

| Serie | Cubeta | Se conserva |
|---|---|---|
| `minutes` | 1 minuto | 24 h (1.440 puntos) |
| `quarter` | 15 minutos | 7 días (672 puntos) |

### Puntos

```go
type MetricPoint struct {
    Time        int64   `json:"t"`          // inicio de la cubeta, segundos Unix
    CPU         float64 `json:"cpu"`        // CPUPercent(), 0-100
    Memory      float64 `json:"memory"`     // bytes
    MaxMemory   float64 `json:"maxMemory"`
    NetRx       float64 `json:"rx"`         // KiB/s
    NetTx       float64 `json:"tx"`
    Online      float64 `json:"online"`     // fracción de muestras tomadas en marcha, 0-1
    NoData      bool    `json:"noData,omitempty"`
    Players     float64 `json:"players"`    // media de jugadores conectados
    PlayersPeak int     `json:"playersPeak"`
    MaxPlayers  int     `json:"maxPlayers"`
    HasPlayers  bool    `json:"hasPlayers"` // false si ninguna consulta respondió en la cubeta
}
```

- CPU, memoria y red se promedian sobre las muestras tomadas **mientras el servidor estaba en marcha**, para que una hora con la mitad del tiempo detenido no las reduzca a la mitad. `Online` es muestras en marcha / todas las muestras.
- Una cubeta que solo tiene recuentos de jugadores cuenta como `Online = 1` (un juego que responde consultas está en marcha).
- Jugadores: `samplePlayers()` pregunta a cada servidor en marcha una vez por minuto (`Server.QueryGame()`: ping de lista de Minecraft o A2S de Steam, respuestas cacheadas 10 s). Los bots de Steam no cuentan como jugadores. El máximo histórico se guarda como `recordPlayers` / `recordAt`.

### Almacenamiento

- Archivo `<daemon.data.servers>/<id>.metrics` (JSON, permisos 0600), que se lee la primera vez que se necesita el historial del servidor.
- Se escribe como mucho cada 5 minutos mientras llegan muestras, y para todos los servidores al apagar. Si el daemon cae de golpe se pueden perder hasta 5 minutos.
- Se borra junto con el servidor (con `.cron`, `.crashes`, `.notify` y `.worldmap`, `server_loader.go`).

### Rangos y remuestreo

`GET /api/servers/:id/metrics?range=1h|24h|7d` (scope `server.stats`, por defecto `24h`), redirigido al daemon:

| `range` | Ventana | Paso | Serie de origen |
|---|---|---|---|
| `1h` | 1 hora | 1 min | `minutes` |
| `24h` | 24 horas | 5 min | `minutes` |
| `7d` | 7 días | 1 hora | `quarter` |

`Server.Metrics(window, step, now)`:
1. Toma la serie de origen más la cubeta que se está llenando.
2. `resample()` promedia los puntos por pasos, ponderando CPU, memoria y red por la fracción `Online` de cada punto (la misma regla que una cubeta). Se omiten los puntos anteriores a la ventana y los `NoData`.
3. `fillGaps()` inserta `{"t": ..., "noData": true}` en cada paso sin datos entre el primer punto y ahora: el nodo estaba caído, así que el servidor también. El paso en curso nunca se marca. Las gráficas dejan esos pasos en blanco y los cuentan como apagados en lugar de trazar una línea por encima.

Respuesta:

```json
{
  "range": "24h",
  "step": 300,
  "points": [ { "t": 1761984000, "cpu": 12.5, "memory": 1610612736, "maxMemory": 4294967296,
                "rx": 3.2, "tx": 8.9, "online": 1, "players": 2.4, "playersPeak": 4,
                "maxPlayers": 20, "hasPlayers": true } ],
  "record": { "players": 17, "at": 1761400000 }
}
```

Un rango desconocido responde 400 `range must be 1h, 24h or 7d`.

---

## Métricas del nodo

### Información del sistema

`GET /api/nodes/:id/system` (scope `nodes.view`) → `GET /daemon/system` (solo con JWT, `internal/web/daemon/root.go`). Devuelve nombre de host, sistema operativo, plataforma y versión, arquitectura, tiempo encendido, modelo de CPU / núcleos físicos / hilos, `cpuUsage`, memoria total/usada/libre/disponible, la lista de discos, el disco que contiene los servidores (`serversDisk`) y el total de bytes de red enviados/recibidos.

- **Muestreador de CPU**: una goroutine en segundo plano mide la CPU del nodo en ventanas de 2 segundos (`cpu.Percent(2s)`) y guarda el último valor, así `/system` responde al instante. Arranca con la primera llamada; esa primera llamada devuelve una lectura de 250 ms. Antes, cada llamada se bloqueaba un segundo, y el dashboard, las páginas de nodos y el asistente de servidores la consultan cada pocos segundos.
- **Discos**: cada partición cuyo punto de montaje es un directorio, sin repetir dispositivo + tamaño + uso (dentro de un contenedor cada montaje bind parece una partición del mismo disco). La etiqueta preferida es `/`, después el montaje que contiene la carpeta de servidores y después la ruta más corta.

### Historial de red (`internal/web/daemon/network_history.go`)

Cada nodo guarda su propio historial de tráfico, de modo que el dashboard ya no depende de cuánto tiempo estuvo abierta una pestaña del navegador. Vista del dashboard: [Dashboard y nodos](../features/dashboard-and-nodes.md).

| Elemento | Valor |
|---|---|
| Muestreo | Cada 10 s, bytes totales de las interfaces **físicas** |
| Resolución | Un punto por minuto: KB/s recibidos/enviados, promediados sobre los segundos realmente medidos |
| Se conserva | 24 horas |
| Archivo | `network.history` en la carpeta padre de `daemon.data.servers`, guardado cada 5 min (se escribe en `.tmp` y luego se renombra) |
| Huecos | Un hueco de más de 5 min entre dos lecturas (host suspendido) no se reparte en el minuto; los contadores que retroceden solo inician una lectura nueva |

**Solo interfaces físicas.** Se excluyen loopback, las subinterfaces VLAN (`eth0.10`, nombres con `@`) y los prefijos virtuales (`docker`, `br-`, `veth`, `virbr`, `vnet`, `cni`, `flannel`, `cali`, `vxlan`, `kube`, `lxc`, `podman`, `tun`, `tap`, `wg`, `tailscale`, `zt`, `dummy`, `bond`, `ifb`), para no contar dos o tres veces el tráfico de los contenedores.

**`daemon.hostProc`.** Dentro de un contenedor, el `/proc/net/dev` del daemon solo muestra el tráfico del contenedor. Si `daemon.hostProc` tiene valor (variable `SKYPANEL_DAEMON_HOSTPROC`), los contadores se leen de `<hostProc>/1/net/dev`, es decir, los del host. `docker-compose.yml` monta `/proc:/host/proc:ro` y define `SKYPANEL_DAEMON_HOSTPROC=/host/proc`. Si el archivo no existe, se usan los contadores de gopsutil. La misma lectura alimenta `networkBytesSent/Recv` de `/daemon/system`.

**Endpoints:**

| Endpoint | Autenticación | Descripción |
|---|---|---|
| `GET /daemon/network/history?range=1h\|6h\|24h` | JWT del panel | Historial de este nodo |
| `GET /api/nodes/network-history?range=1h\|6h\|24h` | Scope `nodes.view` | Historial de todos los nodos sumado paso a paso |

| `range` | Paso |
|---|---|
| `1h` (por defecto) | 1 min |
| `6h` | 5 min |
| `24h` | 15 min |

Un paso que el nodo no midió no tiene punto. El endpoint del panel consulta todos los nodos en paralelo (10 s de límite cada uno) y devuelve:

```json
{ "step": 60, "points": [ { "t": 1761984000, "rx": 120.5, "tx": 88.1 } ], "nodes": 2, "unavailable": 0 }
```

`unavailable` cuenta los nodos que no respondieron (caídos o con un daemon antiguo sin el endpoint). Un rango desconocido responde 400 `range must be 1h, 6h or 24h`.

---

## Informes de estado en Discord

`sendSystemStatusToDiscord()` se ejecuta **cada minuto**, solo donde corre el panel, y solo hace algo si hay webhooks configurados:

- `panel.notifications.discordWebhookSystem`: un embed con los recuentos en línea/apagados, medias y una línea por servidor.
- `panel.notifications.discordWebhookNode`: un embed por nodo con sus recuentos de servidores, host, modelo/núcleos/hilos/GHz de CPU, RAM y sistema operativo.

Las cifras de CPU, RAM y sistema operativo del informe de nodo se leen en la máquina donde corre el panel (`hostCPUSummary`, `mem.VirtualMemory`, `getOSName`), para todos los nodos, incluidos los remotos.

---

## Resumen de endpoints

| Endpoint | Scope | Datos |
|---|---|---|
| `GET /api/servers/:id/stats` | `server.stats` | `ServerStats` actual |
| `GET /api/servers/:id/metrics?range=` | `server.stats` | Historial de rendimiento |
| `GET /api/uptime` | `admin` o `uptime.view` | Uptime de todos los servidores |
| `GET /api/uptime/:serverId` | `server.view` | Uptime de un servidor |
| `GET /api/nodes/:id/system` | `nodes.view` | Información del sistema del nodo |
| `GET /api/nodes/network-history?range=` | `nodes.view` | Historial de red de todos los nodos |
| `GET /daemon/network/history?range=` | JWT del panel | Historial de red de un nodo |

---

## Solución de problemas

| Síntoma | Causa / comprobación |
|---|---|
| CPU casi al 100 % en un servidor de varios núcleos | Es lo esperado cuando el servidor usa toda su asignación `cpu`: la CPU es una cuota del límite, no del nodo |
| La memoria parece menor que la de `free` en el host | La memoria de Docker excluye la caché de páginas inactiva, igual que `docker stats` |
| Todos los servidores de entorno host muestran la misma tasa de red | Los entornos host leen contadores de todo el nodo |
| Huecos en la gráfica de 24 h o 7 d | El nodo (daemon) estaba caído; esos pasos son `noData` |
| Gráfica de red del nodo plana a cero dentro de Docker | `daemon.hostProc` sin definir o `/proc` sin montar; el contenedor solo ve su propio tráfico |
| `unavailable` > 0 en el historial de red | Un nodo está caído o tiene un daemon sin `/daemon/network/history` |
| Sin datos de uptime en un nodo solo daemon | Es lo esperado: el uptime solo lo escribe el nodo del panel |
| Demasiados mensajes de estado en Discord | Los webhooks de sistema/nodo publican cada minuto; déjelos vacíos para desactivarlos |
