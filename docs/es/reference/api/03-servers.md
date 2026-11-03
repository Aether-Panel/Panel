# Servidores

Rutas bajo `/api/servers`: la lista de servidores, la creación y eliminación de servidores, las acciones de encendido, la consola, las estadísticas, los ajustes, los miembros, la actividad, el subdominio, la clonación, el traslado de nodo, la IA y el WebSocket. Los archivos, las copias de seguridad, las tareas y las bases de datos están en [Contenido del servidor](./04-server-content.md). Volver a la [referencia de la API](../11-api-reference.md).

La mayoría de las rutas bajo `/api/servers/:serverId/` se reenvían al daemon del nodo del servidor (`/daemon/server/:serverId/...`, véase [Daemon](./09-daemon.md)): el panel comprueba el scope y devuelve sin cambios el código de estado y el cuerpo del nodo (salvo cuando el nodo rechaza el token del propio panel: `502 ErrNodeAuthFailed`, véase [Convenciones](./01-conventions.md#fallos-de-autenticación-con-el-nodo)). En las tablas se marcan con «nodo». Todo cambio correcto bajo `/:serverId` se anota en el [registro de actividad](#get-apiserversserveridactivity) del servidor.

## Rutas

### Servidores

| Método | Ruta | Scope | Qué hace |
|--------|------|-------|----------|
| GET | `/api/servers` | Sesión iniciada | Lista los servidores visibles para quien llama (paginada) |
| GET | `/api/servers/:serverId` | `server.view` | Un servidor, opcionalmente con los permisos de quien llama sobre él |
| PUT | `/api/servers/:serverId` | `server.create` | Crea un servidor con ese identificador |
| DELETE | `/api/servers/:serverId` | `server.delete` | Elimina un servidor |
| POST | `/api/servers/:serverId/suspend` | `server.data.edit.admin` | Suspende o levanta la suspensión (alterna) |
| PUT | `/api/servers/:serverId/name/:name` | `server.name.edit` | Cambia el nombre |
| POST | `/api/servers/:serverId/clone` | `server.create` (experimental) | Copia el servidor en el mismo nodo |
| POST | `/api/servers/:serverId/transfer` | `server.data.edit.admin` | Traslada el servidor a otro nodo |

### Definición, variables y puertos

| Método | Ruta | Scope | Qué hace |
|--------|------|-------|----------|
| GET | `/api/servers/:serverId/definition` | `server.definition.view` | Definición completa (nodo) |
| PUT | `/api/servers/:serverId/definition` | `server.definition.edit` | Sustituye la definición |
| GET | `/api/servers/:serverId/data` | `server.data.view` | Variables editables por el usuario (nodo) |
| POST | `/api/servers/:serverId/data` | `server.data.edit` | Cambia las variables editables por el usuario (nodo) |
| PUT | `/api/servers/:serverId/data` | `server.data.edit.admin` | Cambia cualquier variable, los límites, la dirección y la lista de puertos |
| PUT | `/api/servers/:serverId/port-settings` | `server.data.edit` | Puerto principal y etiquetas de los puertos |
| GET | `/api/servers/:serverId/flags` | cualquiera de `server.flags.view`, `server.flags.edit` | Opciones de arranque y reinicio automáticos (nodo) |
| POST | `/api/servers/:serverId/flags` | `server.flags.edit` | Cambia esas opciones (nodo) |

### Encendido y estado

| Método | Ruta | Scope | Qué hace |
|--------|------|-------|----------|
| POST | `/api/servers/:serverId/start` | `server.start` | Arranca (nodo) |
| POST | `/api/servers/:serverId/stop` | `server.stop` | Detiene (nodo) |
| POST | `/api/servers/:serverId/restart` | `server.start` y `server.stop` | Reinicia (nodo) |
| POST | `/api/servers/:serverId/kill` | `server.kill` | Mata el proceso (nodo) |
| POST | `/api/servers/:serverId/install` | `server.install` | Ejecuta la instalación de la plantilla (nodo) |
| POST | `/api/servers/:serverId/reload` | `server.reload` | Vuelve a cargar la definición desde el disco (nodo) |
| GET | `/api/servers/:serverId/status` | `server.status` | En marcha / instalando (nodo) |
| GET | `/api/servers/:serverId/stats` | `server.stats` | CPU, memoria, disco, red (nodo) |
| HEAD | `/api/servers/:serverId/query` | `server.stats` | Si se puede consultar el juego (nodo) |
| GET | `/api/servers/:serverId/query` | `server.stats` | Jugadores e información del juego (nodo) |
| GET | `/api/servers/:serverId/metrics` | `server.stats` | Historial de rendimiento (nodo) |
| GET | `/api/servers/:serverId/console` | `server.console` | Búfer de la consola (nodo) |
| POST | `/api/servers/:serverId/console` | `server.console.send` | Envía un comando (nodo) |
| GET | `/api/servers/:serverId/crashes` | `server.console` | Últimos informes de caída (nodo) |
| GET | `/api/servers/:serverId/socket` | `server.view` | WebSocket: consola, estadísticas, estado |

### Versión del juego y notificaciones

| Método | Ruta | Scope | Qué hace |
|--------|------|-------|----------|
| GET | `/api/servers/:serverId/version` | `server.view` | Versión actual del juego y las disponibles (nodo) |
| POST | `/api/servers/:serverId/version` | `server.install` (experimental) | Cambia la versión del juego (nodo) |
| GET | `/api/servers/:serverId/notifications` | `server.flags.edit` | Los canales de notificación propios del servidor (nodo) |
| PUT | `/api/servers/:serverId/notifications` | `server.flags.edit` | Los guarda (nodo) |
| POST | `/api/servers/:serverId/notifications/test` | `server.flags.edit` | Envía un mensaje de prueba (nodo) |

### Miembros, actividad y subdominio

| Método | Ruta | Scope | Qué hace |
|--------|------|-------|----------|
| GET | `/api/servers/:serverId/user` | `server.users.view` | Miembros y sus scopes |
| GET | `/api/servers/:serverId/user/:email` | `server.users.view` | Un miembro |
| PUT | `/api/servers/:serverId/user/:email` | `server.users.edit` | Añade o edita un miembro (invita a los correos desconocidos) |
| DELETE | `/api/servers/:serverId/user/:email` | `server.users.delete` | Quita un miembro |
| PUT | `/api/servers/:serverId/owner/:email` | `admin` | Entrega el servidor a otro usuario |
| GET | `/api/servers/:serverId/activity` | `server.users.view` | Registro de actividad (paginado con cursor) |
| GET | `/api/servers/:serverId/subdomain` | cualquiera de `server.flags.edit`, `server.data.edit` | Subdominio y sus registros DNS |
| GET | `/api/servers/:serverId/subdomain/check` | cualquiera de `server.flags.edit`, `server.data.edit` | Si un nombre está libre |
| PUT | `/api/servers/:serverId/subdomain` | cualquiera de `server.flags.edit`, `server.data.edit` | Establece el subdominio |
| DELETE | `/api/servers/:serverId/subdomain` | cualquiera de `server.flags.edit`, `server.data.edit` | Lo quita |

### IA

| Método | Ruta | Scope | Qué hace |
|--------|------|-------|----------|
| POST | `/api/servers/:serverId/ai/analyze` | `server.console` | Explica líneas de la consola |
| POST | `/api/servers/:serverId/ai/chat` | `server.console` | Chat con el asistente del servidor |
| POST | `/api/servers/:serverId/ai/chat/confirm` | `server.console` | Aprueba o rechaza los cambios que espera el asistente |

Las rutas de transferencia externa (`/extransfer/*`) están en [Transferencia externa](./08-external-transfer.md); la disponibilidad de un servidor (`/api/uptime/:serverId`), en [Administración del panel](./06-panel-admin.md#disponibilidad).

## El objeto servidor

`GET /api/servers` y `GET /api/servers/:serverId` describen un servidor así:

```json
{
  "id": "3F9A1C2B7D4E",
  "name": "Survival",
  "nodeId": 1,
  "node": { "id": 1, "name": "node-1", "publicHost": "node1.example.com", "publicPort": 8080, "sftpPort": 5657, "isLocal": false },
  "ip": "0.0.0.0",
  "port": 25565,
  "ports": [25565, 25566],
  "portNotes": { "25566": "RCON" },
  "type": "minecraft-java",
  "icon": "",
  "canGetStatus": true,
  "parent_server_id": "1A2B3C4D5E6F",
  "total_cpu": 200,
  "total_memory": 4096,
  "total_disk": 20480,
  "suspended": false
}
```

| Campo | Significado |
|-------|-------------|
| `id` | Identificador que se usa en todas las rutas |
| `node` | El nodo; `privateHost` y `privatePort` solo se incluyen para los administradores |
| `ports`, `portNotes` | Todos los puertos asignados al servidor (el principal primero) y las etiquetas que tienen |
| `canGetStatus` | Solo en la lista: si quien llama puede leer su estado |
| `isGhost` | Solo cuando es verdadero: el servidor está en la base de datos pero no en su nodo (local) |
| `parent_server_id` | Presente en los subservidores (Splitter) |
| `total_cpu`, `total_memory`, `total_disk` | Límites en % de un núcleo, MB y MB; `0` o ausente significa sin límite |

## Rutas del servidor

### `GET /api/servers`

| Consulta | Por defecto | Significado |
|----------|-------------|-------------|
| `name` | `*` | Filtro por nombre; `*` es un comodín |
| `node` | `0` | ID del nodo; `0` para todos |
| `username` | quien llama | Solo los administradores pueden listar los servidores de otro usuario; cualquier otra cuenta que pida otro usuario recibe una lista vacía |
| `page`, `limit` | `1`, `20` | Véase [Paginación](./01-conventions.md#paginación) |

```json
{ "servers": [ { "id": "3F9A1C2B7D4E", "name": "Survival", "...": "..." } ], "paging": { "page": 1, "pageSize": 20, "maxSize": 100, "total": 1 } }
```

### `GET /api/servers/:serverId`

Con `?perms` se reciben también los permisos efectivos de quien llama sobre el servidor:

```json
{
  "server": { "id": "3F9A1C2B7D4E", "name": "Survival", "...": "..." },
  "permissions": { "serverIdentifier": "3F9A1C2B7D4E", "owner": true, "scopes": ["server.view", "server.start", "..."] }
}
```

Sin `?perms`, `permissions` es `null`.

### `PUT /api/servers/:serverId`

Crea un servidor cuyo identificador es el valor de la ruta (letras, dígitos, `-` y `_`, de 1 a 20 caracteres, sin empezar por `-` ni por `_`; en otro caso, `400`). El cliente web genera 12 caracteres hexadecimales en mayúsculas. El cuerpo es una definición de servidor (normalmente una plantilla) más estos campos:

| Campo | Significado |
|-------|-------------|
| `name` | Nombre visible; por defecto, el identificador |
| `node` | ID del nodo (se ignora en los subservidores, que usan el nodo del padre) |
| `users` | Nombres de usuario que se añaden como miembros con los scopes estándar de miembro; el primero pasa a ser el propietario |
| `parent_server_id` | Crea un subservidor (Splitter) dentro de ese servidor |
| `type`, `icon`, `data`, `install`, `run`, `environment`, ... | La definición; véase [Servidores](../../backend/04-servers.md) |

```json
{
  "name": "Survival",
  "node": 1,
  "users": ["alice"],
  "type": "minecraft-java",
  "environment": { "type": "docker", "image": "ghcr.io/aether-panel/generic:latest" },
  "data": { "port": { "type": "integer", "value": 25565 }, "memory": { "type": "integer", "value": 4096 } },
  "install": [ { "type": "mojangdl", "version": "1.21.1", "target": "server.jar" } ],
  "run": { "command": "java -Xmx${memory}M -jar server.jar nogui", "stop": "stop" }
}
```

En la práctica, la definición se copia de una plantilla (`GET /api/templates/:repo/:name`) y se rellenan los valores de sus variables.

Respuesta: `200 {"id": "3F9A1C2B7D4E"}`.

Qué comprueba y qué hace el panel:

- Las variables `port`, `port2`, ... deben ser 1024 o mayores, únicas, distintas de los puertos propios del nodo y no estar en uso por otro servidor del nodo; las demás variables de puerto (query, RCON, cualquier variable entera cuyo nombre termine en `port`) se comprueban igual. Los fallos responden `400` con `ErrPortOutOfRange`, `ErrPortDuplicated`, `ErrPortUsedByPanel` o `ErrPortTaken`; `metadata.port` es el número, seguido del nombre de la variable en las demás variables de puerto (por ejemplo, `27015 (queryport)`).
- Las variables `cpu`, `memory` y `disk` pasan a ser los límites del servidor; los valores negativos responden `400`. Un subservidor necesita los tres por encima de cero, y deben caber en lo que le queda libre al padre.
- Los usuarios sin `admin` solo pueden crear subservidores: `parent_server_id` debe ser un servidor suyo, el entorno debe ser `docker` y de él solo se conservan `image`, `containerRoot`, `config` (sin su usuario) y los `portBindings` de la plantilla. Cualquier otra cosa responde `403`. Véase [Subservidores](../../features/sub-servers.md).
- Propietario: el primer usuario de la lista; en un subservidor, el propietario del padre; si quien crea no es administrador, él mismo; un administrador que crea sin usuarios no es propietario de nada.
- El nodo recibe la definición (`400` si rechaza el entorno, por ejemplo un entorno host con `docker.disallowHost`). Con `panel.servers.autoInstall` activado (valor por defecto), la instalación empieza enseguida en segundo plano.
- Los miembros reciben un correo y, si está activado, se asigna un subdominio automático.

Errores: `400 ErrNodeInvalid` para un nodo desconocido, `403` por las reglas de los subservidores, el código de estado y el cuerpo del nodo cuando este lo rechaza, y `502 ErrNodeAuthFailed` cuando el nodo no acepta el token del panel.

### `DELETE /api/servers/:serverId`

Detiene el servidor, lo elimina en el nodo (junto con los archivos de sus copias de seguridad guardados en ese nodo; las copias ya subidas al almacenamiento externo se quedan allí) y borra sus permisos, registros de copias de seguridad, clientes OAuth2, sesiones, historial de disponibilidad y registros DNS. Sus bases de datos MySQL y los usuarios de estas se eliminan de sus hosts. Un servidor con subservidores responde `400` hasta que se eliminen. `?skipNode` lo elimina solo del panel, sin contactar con el nodo. Respuesta: `204`; los miembros reciben un aviso por correo (plantilla `deletedServer`).

### `POST /api/servers/:serverId/suspend`

Alterna la suspensión del servidor y de sus subservidores. Al suspender, también se detienen. La respuesta es el objeto servidor con el nuevo valor de `suspended`. Un servidor suspendido no se puede arrancar ni reiniciar (`403`), ni puede clonarlo quien no sea administrador.

### `PUT /api/servers/:serverId/name/:name`

Cambia el nombre del servidor en el panel. `204`.

### `POST /api/servers/:serverId/clone`

Experimental: responde `403 ErrFeatureDisabled` salvo que `panel.experimental.serverClone` esté activado. Crea una copia en el mismo nodo con la misma plantilla, ajustes, límites, propietario y miembros, y con puertos libres nuevos (se buscan primero justo por encima de cada puerto original).

```json
{ "name": "Survival copy", "copyFiles": true, "label": "Survival copy" }
```

`name` es obligatorio, con un máximo de 40 caracteres ASCII imprimibles. `copyFiles` (por defecto `true`) copia también los archivos en segundo plano. `label` es el nombre que aparece en el registro de actividad del original. Respuesta: `200 {"id": "<nuevo id>", "port": 25567}`.

Quien no sea administrador debe ser miembro del original, el original no debe estar suspendido (en otro caso, `403`) y se aplican las reglas de subservidores de arriba. `409` cuando no hay ningún puerto libre, el nodo no tiene los archivos del servidor o la copia no puede empezar; `422` cuando la copia de un subservidor no cabe en el padre. Detalles: [Clonar servidores](../../features/cloning.md).

### `POST /api/servers/:serverId/transfer`

```json
{ "nodeId": 2 }
```

Traslada el servidor a otro nodo de este panel en segundo plano, junto con sus [subservidores](../../features/sub-servers.md) (un subservidor que ya esté en el nodo de destino se queda donde está) y las copias de seguridad que guarda su nodo antiguo. Respuesta: `202 {"msg": "Transfer started"}`.

Se comprueba antes de responder:

| Error | Cuándo |
|-------|--------|
| `400 ErrTransferSubServer` | El servidor es un subservidor: se queda en el nodo de su servidor padre y se traslada con él |
| `400` | El destino es el propio nodo del servidor |
| `404` | Nodo desconocido |
| `409 ErrTransferRunning` | El servidor o uno de sus subservidores ya se está trasladando, o figura en un traslado registrado en `node_transfers` que no ha terminado |
| `409 ErrTransferPortsTaken` | Un puerto (principal o adicional) del servidor o de un subservidor está ocupado en el nodo de destino: por sus puertos del panel, del daemon o SFTP, por un servidor que ya está allí o por un puerto que publican sus contenedores (`metadata.ports` los enumera) |
| `409 ErrTransferBackupRunning` | El nodo antiguo está escribiendo una copia de seguridad, un punto de restauración o una restauración de alguno de ellos (se pregunta con [`GET /daemon/server/:id/backup/busy`](./09-daemon.md); a un nodo sin esa ruta no se le pregunta) |

El traslado se registra en la tabla `node_transfers` antes de responder y se actualiza en cada paso. Después, en segundo plano:

1. **Preparación**, servidor por servidor: se pausan sus tareas programadas en el nodo antiguo (`POST /daemon/server/:id/scheduler/pause`), se copia la definición al nodo nuevo, se pausan allí las tareas y se copia cada tarea (`GET /tasks` y después `PUT /tasks/:taskId` en el daemon), se detiene el servidor en el nodo antiguo (se fuerza su cierre si no se detiene), se empaquetan sus archivos, el archivo se envía al nodo nuevo y se extrae, y cada copia de seguridad cuyo archivo está en el nodo antiguo (`local` o `both`) se copia a través del panel al nodo nuevo, con su nombre y su fecha.
2. **Confirmación**: una única transacción de la base de datos apunta todos los registros al nodo nuevo y marca como confirmado el registro del traslado. Solo se escribe `node_id`, así que se conservan los cambios hechos en el servidor mientras tanto.
3. **Limpieza**: se eliminan los servidores del nodo antiguo, con los archivos de copias de seguridad de allí. Las copias del almacenamiento externo no se tocan; el bucket es el mismo sea cual sea el nodo.
4. **Arranque**: se reanudan las tareas en el nodo nuevo (`POST /daemon/server/:id/scheduler/resume`) y se arrancan allí los servidores que estaban en marcha antes. Después se borra el registro del traslado.

Si falla un paso hasta la confirmación incluida, todo se deshace, del más reciente al más antiguo: se eliminan del nodo nuevo las copias de seguridad y los servidores, del nodo antiguo los archivos empaquetados, se reanudan las tareas en el nodo antiguo y un servidor que estaba en marcha se vuelve a arrancar allí. Los servidores se quedan donde estaban. El fallo se escribe en el registro del panel y en el registro de actividad de cada servidor como `server.transferFailed` (paso y motivo). Un servidor que no arranca en el nodo nuevo después de la confirmación se queda allí y recibe `server.transferStartFailed`. Su subdominio pasa a apuntar a la dirección del nodo nuevo.

Al arrancar, el panel cierra los traslados que una parada dejó a medias, en una sola pasada y sin reintentos: uno sin confirmar se deshace como se describe arriba (actividad `server.transferFailed`, detalle `the panel restarted while <paso>: the transfer was undone`) y uno confirmado se completa (se borran las copias antiguas, se reanudan las tareas y se arrancan los servidores en el nodo nuevo). Véase [Recuperación tras un reinicio del panel](../../features/server-transfers.md#recuperación-tras-un-reinicio-del-panel).

Límites: la recuperación tras un reinicio es una sola pasada, y un nodo que no responda en ese momento conserva lo que quedó en él; ambos daemons necesitan las rutas de tareas (`/scheduler/pause`, `/scheduler/resume`), así que con un daemon más antiguo el traslado falla al pausar las tareas y se deshace; una tarea que ya se esté ejecutando no se interrumpe, y no se copian la zona horaria ni el límite de concurrencia de las tareas; solo se traslada un nivel de subservidores. Detalles: [Transferencias de servidores](../../features/server-transfers.md).

## Definición, variables y puertos

### `GET /api/servers/:serverId/definition`

La definición completa del servidor tal como la guarda el nodo (`type`, `display`, `icon`, `data`, `groups`, `install`, `uninstall`, `run`, `environment`, `supportedEnvironments`, `requirements`, `stats`, `query`, `keepAlive`). El formato se describe en [Servidores](../../backend/04-servers.md).

### `PUT /api/servers/:serverId/definition`

Cuerpo: una definición y, opcionalmente, `name`. El servidor debe estar inactivo en el nodo. Sin `server.data.edit.admin`, el panel conserva el entorno actual (salvo la `image` de Docker, `containerRoot` y `config` sin su usuario) y los valores actuales de las variables bloqueadas, de modo que los clientes no pueden cambiar sus límites, su dirección ni su acceso al host. Qué hace cada ajuste: [Configuración del servidor](../../features/server-settings.md). `port`, `ip`, `name`, `type` e `icon` se copian al panel. Respuesta: `204`, o el error del nodo sin cambios.

### `GET /api/servers/:serverId/data`

Las variables marcadas como editables por el usuario, con sus grupos:

```json
{
  "data": { "motd": { "type": "string", "value": "Welcome", "display": "MOTD", "required": false, "userEdit": true } },
  "groups": [ { "display": "General", "description": "", "variables": ["motd"], "order": 0 } ]
}
```

### `POST /api/servers/:serverId/data`

Objeto plano con los valores nuevos: `{"motd": "Hello"}`. Solo cambian las variables editables por el usuario. Respuesta: `204` del nodo.

### `PUT /api/servers/:serverId/data`

La variante del administrador. Acepta cualquier variable y además estos campos, que el panel aplica también a su propio registro:

| Campo | Significado |
|-------|-------------|
| `cpu`, `memory`, `disk` | Límites (`%`, MB, MB); en los subservidores se comprueban frente al padre, y en los padres frente a lo asignado a sus subservidores |
| `ip`, `port` | Dirección |
| `ports` | Lista completa de puertos; el nodo la convierte en las variables `port`, `port2`, ... y quita las que ya no figuran |
| `primaryPort` | Cuál de los `ports` es el principal (debe estar en la lista) |
| `portNotes` | Etiquetas por puerto, por ejemplo `{"25575": "RCON"}` |

Primero se llama al nodo; si lo rechaza, no se guarda nada. Respuesta: `204`.

### `PUT /api/servers/:serverId/port-settings`

Para los miembros que pueden editar variables: cambia cuál de los puertos asignados es el principal y las etiquetas, sin tocar la lista de puertos.

```json
{ "primaryPort": 25566, "portNotes": { "25565": "Game", "25566": "Query" } }
```

Respuesta: `200 {"success": true}`.

### Opciones de arranque (flags)

`GET /api/servers/:serverId/flags`:

```json
{ "autoStart": false, "autoRestartOnCrash": true, "autoRestartOnGraceful": false }
```

`POST` recibe el mismo objeto; los campos que se omiten no cambian. `204`.

## Encendido y estado

### Acciones de encendido

`POST /start`, `/stop`, `/restart`, `/kill`, `/install`, `/reload`. Sin parámetros de consulta, `start`, `stop`, `restart` e `install` responden `202` y se ejecutan en segundo plano; con `?wait` responden `204` cuando terminan. `kill` y `reload` responden `204`.

| Error | Cuándo |
|-------|--------|
| `403` | Arrancar o reiniciar un servidor suspendido |
| `409 ErrDiskLimitExceeded` | Arrancar o reiniciar con el límite de disco alcanzado |
| `409 ErrPortBusy` | Otro contenedor en marcha ya publica un puerto de host del servidor, u otro proceso del nodo lo ocupa (`metadata.port`) |

Una instalación rechazada porque el nodo está por debajo de `daemon.minFreeDiskMB` falla con `ErrNodeDiskFull` (se muestra en la consola; con `?wait` se devuelve como error).

### `GET /api/servers/:serverId/status`

`{"running": true, "installing": false}`. Durante la instalación, `running` es `false`.

### `GET /api/servers/:serverId/stats`

```json
{ "cpu": 45.2, "maxCpu": 200, "memory": 1536000000, "maxMemory": 4294967296, "storage": 5000000000, "maxStorage": 21474836480, "networkRx": 1200, "networkTx": 800, "running": true, "jvm": { "...": "..." } }
```

`cpu` está en porcentaje de un núcleo (200 = dos núcleos ocupados); `maxCpu` es lo que puede usar el servidor en la misma unidad (su límite de CPU, o todos los núcleos del nodo). El panel muestra `cpu / maxCpu`. `memory` excluye la caché de páginas inactiva, como `docker stats`. `jvm` solo aparece en los servidores Java que la informan.

Para un servidor que el nodo no tiene, `status`, `stats` y `console` responden un estado vacío y detenido en lugar de `404`.

### `GET /api/servers/:serverId/query`

`HEAD` responde `202` cuando el tipo de consulta de la plantilla es `minecraft`, `source` o `steam`, y `204` en otro caso. `GET` responde `200` con la respuesta del juego (jugadores, mapa, versión...) o `204` cuando el juego no responde.

### `GET /api/servers/:serverId/metrics?range=24h`

`range` es `1h`, `24h` (por defecto) o `7d`; cualquier otro valor responde `400`.

```json
{
  "range": "24h",
  "step": 900,
  "points": [ { "t": 1793576400, "cpu": 12.5, "memory": 1.2e9, "maxMemory": 4.2e9, "rx": 3.1, "tx": 2.4, "online": 1, "players": 3.5, "playersPeak": 6, "maxPlayers": 20, "hasPlayers": true } ],
  "record": { "players": 14, "at": "2026-10-30T21:14:00Z" }
}
```

`online` es la proporción de muestras tomadas con el servidor en marcha; `noData: true` marca un intervalo sin datos registrados. Detalles: [Resumen y estadísticas del servidor](../../features/server-statistics.md).

### `GET /api/servers/:serverId/console`

```json
{ "epoch": 1793576400123456, "logs": "W1NlcnZlciB0aHJlYWQvSU5GT10..." }
```

`logs` es la salida de la consola en **base64** (al decodificarla se obtiene el texto, con los códigos ANSI incluidos). `epoch` está en microsegundos; si se devuelve como `?time=`, solo se obtiene lo que se imprimió después. Un `time` que no sea un número responde `400 ErrInvalidUnixTime`.

### `POST /api/servers/:serverId/console`

El cuerpo es el comando en texto plano (no JSON). `204`. Un comando a un servidor cuya consola perdió el nodo tras reiniciar el daemon falla con `ErrConsoleDetached`.

### `GET /api/servers/:serverId/crashes`

Los últimos informes de caída, del más reciente al más antiguo:

```json
[ { "id": "...", "time": "2026-10-31T10:00:00Z", "exitCode": 1, "lines": ["..."], "restarted": true, "attempt": 1, "limit": 3 } ]
```

Detalles: [Detección de caídas e informes de caída](../../features/crash-reports.md).

## Versión del juego y notificaciones

### `GET /api/servers/:serverId/version`

```json
{ "platform": "paper", "software": "paper", "variable": "version", "current": "1.21.1", "versions": ["1.21.4", "1.21.3"], "changing": false }
```

`204` cuando la plantilla no permite que el panel cambie la versión; `502` cuando no se pueden obtener las listas de versiones (PaperMC, Mojang).

### `POST /api/servers/:serverId/version`

Experimental (`panel.experimental.versionSwitch`; en otro caso, `403 ErrFeatureDisabled`). Cuerpo `{"version": "1.21.4"}`. Detiene el servidor, guarda un punto de restauración, reinstala con la nueva versión y vuelve a arrancarlo si estaba en marcha. `202`. `400` para una versión desconocida o una plantilla sin cambio de versión; `409` mientras hay un cambio en curso. Detalles: [Versión del juego](../../features/game-versions.md).

### `GET /api/servers/:serverId/notifications`

```json
{
  "discordWebhook": "", "telegramBotToken": "", "telegramChatId": "", "webhookUrl": "", "webhookSecret": "",
  "crash": true, "backup": true, "resources": true, "power": false
}
```

`PUT` guarda el mismo objeto (`400` para un webhook de Discord, unos datos de Telegram o una URL de webhook no válidos; `204` si todo va bien). `POST /notifications/test` envía un ejemplo a cada canal configurado y responde `200` con el resultado de cada canal (`"ok"` o el motivo), `400` si no hay ninguno configurado y `429` si han pasado menos de 10 segundos desde la prueba anterior. Detalles: [Notificaciones del servidor](../../features/server-notifications.md).

## Miembros

Cada servidor tiene un propietario y puede tener subusuarios. El propietario tiene el conjunto de scopes del propietario, diga lo que diga su fila, y aquí no se le puede limitar ni quitar; solo un administrador puede cambiar el propietario. Reglas: [Referencia de permisos](../../general/scopes-reference.md).

### `GET /api/servers/:serverId/user`

```json
[ { "username": "alice", "email": "alice@example.com", "scopes": ["server.view", "server.console"], "owner": true } ]
```

`GET /user/:email` responde la misma lista con una sola entrada. Un correo que no es miembro responde `404`, igual que uno desconocido.

### `PUT /api/servers/:serverId/user/:email`

```json
{ "scopes": ["server.view", "server.console", "server.start"] }
```

Solo se conservan los scopes de servidor. Quien llama sin `server.admin` ni `admin` solo puede conceder o quitar scopes que tenga él mismo. Un correo desconocido recibe una cuenta invitada (contraseña aleatoria, scope `login`) y un correo para elegir contraseña. `204`.

| Error | Cuándo |
|-------|--------|
| `409 ErrServerOwner` | El correo es el del propietario |
| `403 ErrInviteSignupsClosed` | El correo no tiene cuenta, los registros están cerrados y quien llama no es administrador |
| `400 ErrFieldNotEmail` | El correo no es válido |

### `DELETE /api/servers/:serverId/user/:email`

`204`; el usuario recibe un aviso por correo. `404 ErrUserNotFound`, `409 ErrServerOwner`.

### `PUT /api/servers/:serverId/owner/:email`

Convierte a ese usuario en propietario (y en miembro). El propietario anterior sigue como subusuario con los scopes que ya tenía su fila. `204`; `404` para un correo desconocido.

## Registro de actividad

### `GET /api/servers/:serverId/activity`

| Consulta | Significado |
|----------|-------------|
| `limit` | Entradas por página; por defecto 50, como máximo 100 |
| `before` | Solo entradas anteriores a este id |
| `type` | Una familia (`power`, `console`, `files`, `backups`, `users`, `settings`, `tasks`) o un prefijo de acción como `file`, `backup.create` o `ai.edit_file` (letras minúsculas, un punto y guiones bajos después del punto) |

```json
{ "items": [ { "id": 812, "action": "console.command", "detail": "say hi", "username": "alice", "userId": 3, "createdAt": "2026-11-01T09:12:00Z" } ], "nextCursor": 790 }
```

`ip` solo se incluye para los administradores. Entre las acciones están `server.start`, `file.write`, `file.upload`, `backup.create`, `user.add`, `task.save`, `settings.notifications`, las del traslado de nodo `server.transfer`, `server.transferFailed` y `server.transferStartFailed` (en la familia `settings`) y los cambios de archivos del asistente `ai.create_file`, `ai.create_directory`, `ai.copy_path`, `ai.edit_file`, `ai.write_file`, `ai.move_path`, `ai.rename_path` y `ai.delete_path` (en la familia `files`; detalle: la ruta u `origen → destino`); los cambios de ajustes se anotan sin sus valores. Detalles: [Registro de actividad y registros de acceso](../../features/activity-and-access-log.md).

## Subdominio

Necesita la configuración de Cloudflare (`panel.dns.cloudflare.token`, `panel.dns.cloudflare.zoneId`, `panel.dns.baseDomain`). Detalles: [Subdominios automáticos (Cloudflare)](../../features/subdomains.md).

### `GET /api/servers/:serverId/subdomain`

```json
{
  "enabled": true, "domain": "play.example.com", "subdomain": "survival", "address": "survival.play.example.com",
  "srv": true, "port": 25565,
  "records": [ { "type": "A", "name": "survival.play.example.com", "content": "203.0.113.5" } ]
}
```

### `GET /api/servers/:serverId/subdomain/check?name=survival`

`{"available": true, "reason": ""}`; `reason` es `invalid`, `reserved`, `taken` o `current`.

### `PUT /api/servers/:serverId/subdomain`

`{"subdomain": "survival"}` (de 3 a 32 letras minúsculas, dígitos y guiones intermedios). Crea los registros y quita los antiguos; responde el objeto del subdominio.

| Error | Cuándo |
|-------|--------|
| `400 ErrSubdomainDisabled` | Los subdominios no están configurados |
| `400 ErrSubdomainInvalid`, `ErrSubdomainReserved` | Nombre no permitido |
| `409 ErrSubdomainTaken` | Lo usa otro servidor |
| `422` | Cloudflare lo rechazó, o el nodo no tiene dirección pública (`ErrSubdomainNoTarget`) |
| `429 ErrSubdomainRateLimited` | Se cambió hace menos de 10 segundos |

### `DELETE /api/servers/:serverId/subdomain`

Elimina los registros. `204`; `422` cuando Cloudflare lo rechaza (el nombre se conserva para poder reintentarlo).

## IA

Las tres rutas necesitan `panel.settings.geminiApiKey` (en otro caso, `400 {"error": "Gemini API Key is not configured. Please add it in Settings."}`) y comparten una cuota de 30 peticiones por usuario cada 10 minutos (`429`). Detalles: [Asistente de IA y análisis de registros](../../features/ai-assistant.md) y [Asistente de IA: trabajar con archivos](../../features/ai-agent.md).

### `POST /api/servers/:serverId/ai/analyze`

```json
{ "logs": ["[ERROR] ...", "..."], "language": "en", "exitCode": 1 }
```

`logs` es obligatorio (se envían las 200 líneas más recientes, hasta 32 KB); `language` es `es` o `en`; `exitCode` indica que las líneas proceden de una caída. Respuesta:

```json
{ "summary": "...", "rootCauses": ["..."], "suggestions": ["..."] }
```

`500 {"error": "..."}` cuando falla el servicio de IA. Mientras `panel.settings.hideAiAnalysis` está activado, la ruta responde `403 ErrAIAnalysisDisabled` a cualquier usuario que tenga el scope, antes de comprobar la clave de Gemini y la cuota (el chat de abajo no se ve afectado); `GET /api/config` expone el interruptor como `hideAiAnalysis`.

### `POST /api/servers/:serverId/ai/chat`

```json
{ "messages": [ { "role": "user", "content": "Why does my server lag?" } ], "language": "en" }
```

De 1 a 20 mensajes de 1 a 2000 caracteres, con `role` `user` o `assistant`, y el último del usuario. El asistente ve el estado del servidor, los jugadores, el rendimiento del último día, las caídas y la consola reciente. Respuesta: `{"reply": "...", "sources": ["status", "console"]}`. Cuando falla el servicio de IA: `422` (no `502`: un proxy como Cloudflare lo sustituiría por su propia página y el motivo no llegaría al navegador; véase [Convenciones](./01-conventions.md#errores-internos)) con `{"error": "...", "reason": "..."}`, donde `reason` es `keyRejected`, `quota`, `unavailable`, `unreachable`, `rejected`, `unreadable`, `empty`, `tooLong`, `blocked`, `malformed`, `timeout` o `failed` (sin clave es `400` con `reason` `notConfigured`). La web muestra un texto para cada `reason`; la causa exacta queda en el log del panel.

**Modo agente.** Cuando `panel.settings.aiFileTools` está activado (por defecto), la petición tiene un usuario con sesión y ese usuario tiene `server.files.view` en el servidor, el asistente también puede usar herramientas de archivos en este servidor (y solo en este): las de lectura con `server.files.view` y las de crear, cambiar, mover y borrar con `server.files.edit`. Las lecturas y las creaciones se ejecutan durante la petición; los cambios de datos existentes se detienen a la espera de confirmación. La respuesta es entonces:

```json
{
  "reply": "Voy a eliminar la carpeta del mundo antiguo.",
  "sources": ["status", "console"],
  "steps": [
    { "id": "local-1-1", "tool": "list_directory", "kind": "read", "summary": "Listed the server folder (12 entries)", "paths": ["."], "status": "done" }
  ],
  "pending": {
    "token": "9f2c…",
    "actions": [
      { "id": "local-2-1", "tool": "delete_path", "kind": "destructive", "summary": "Delete the folder world_old and everything in it", "paths": ["world_old"], "status": "pending" }
    ],
    "expiresAt": "2026-11-02T10:10:00Z"
  }
}
```

| Campo | Significado |
|-------|-------------|
| `steps` | Las llamadas a herramientas de esta petición, en orden. `kind` es `read`, `create`, `modify` o `destructive` (vacío en una llamada rechazada); `status` es `done`, `failed`, `declined` o `refused` (no se ejecutó: herramienta desconocida, falta un scope o un límite); `error` explica un fallo o un rechazo; `paths` son relativas a la carpeta del servidor |
| `pending` | `null`, o las llamadas que esperan al usuario (`status` `pending`), el `token` de un solo uso y `expiresAt` (10 minutos) |

Cuando hay `pending`, `reply` es la explicación del modelo sobre esas llamadas y puede estar vacía. En modo agente, una tarea fallida responde `422` `{"error": "...", "reason": "...", "steps": [...], "pending": null}` con los pasos que se ejecutaron antes del fallo. Los cambios hechos se anotan en el [registro de actividad](#registro-de-actividad) como `ai.<herramienta>`. Herramientas, límites y aislamiento: [Kit de herramientas del agente de IA](../../backend/14-ai-toolkit.md).

### `POST /api/servers/:serverId/ai/chat/confirm`

Responde a una confirmación pendiente. Scope `server.console`; cuenta para la misma cuota.

```json
{ "token": "9f2c…", "approved": ["local-2-1"], "language": "es" }
```

`approved` contiene los IDs de las acciones pendientes que se ejecutan (como mucho 64); las demás se rechazan, y una lista vacía las rechaza todas. El token y cada ID tienen como mucho 128 caracteres. Cada acción aprobada se vuelve a comprobar (scopes y archivos tal como están en ese momento) antes de ejecutarse, y después el asistente continúa. La respuesta tiene la misma forma que la del chat en modo agente (con `sources: []`) y puede traer un `pending` nuevo.

| Error | Cuándo |
|-------|--------|
| `400` | No hay clave de Gemini; JSON no válido; falta el token o supera 128 caracteres; más de 64 IDs o un ID vacío o demasiado largo |
| `403 ErrNoPermission` | No hay usuario con sesión, o quien llama ya no tiene `server.files.view` en el servidor. Sin `server.files.edit`, los cambios aprobados no se ejecutan y vuelven como pasos `refused` |
| `409 ErrAIConfirmationInvalid` | El token es desconocido, ha caducado, ya se usó o lo sustituyó una petición más reciente del mismo usuario en el mismo servidor; pertenece a otro usuario o servidor; un ID aprobado no estaba pendiente; o se desactivó entretanto `panel.settings.aiFileTools`. Las confirmaciones pendientes se guardan en memoria, así que un reinicio del panel también las invalida |
| `429` | Cuota |
| `422` | El asistente falló al continuar: `{"error": "...", "reason": "...", "steps": [...], "pending": null}` |

Un token enviado por otro usuario o para otro servidor se rechaza sin gastarse. Una respuesta que nombra un ID que no estaba pendiente gasta el token.

## WebSocket

### `GET /api/servers/:serverId/socket`

Abre un WebSocket que transmite la consola, las estadísticas y el estado del servidor. Scope: `server.view`. Las conexiones del navegador usan la cookie de sesión y deben ser del mismo origen (véase la [protección de mismo origen](./02-authentication.md#protección-de-mismo-origen)); los demás clientes envían `Authorization: Bearer <token>`.

```javascript
const ws = new WebSocket(`wss://panel.example.com/api/servers/${serverId}/socket`);
```

La cadena de consulta se ignora: el panel suscribe la conexión a los flujos que permiten los scopes de quien llama en el servidor (`console` con `server.console`, `status` con `server.status`, `stats` con `server.stats`). Son los scopes que resolvió la comprobación de permisos de esta petición, como en cualquier otra ruta: los del propietario, las filas del servidor y globales, los scopes globales del rol y los límites de un cliente OAuth2.

Los mensajes son JSON `{"type": ..., "data": ...}`:

| `type` | `data` |
|--------|--------|
| `console` | `{"epoch": 0, "logs": "<base64>"}`: salida nueva de la consola, en base64 como `GET /console` |
| `stat` | Un objeto de estadísticas como el de `GET /stats` |
| `status` | `{"running": true, "installing": false}` |
| `crash` | `{"id": "...", "exitCode": 1, "restarted": true}`: se envía por el flujo de estado cuando el servidor se cae por sí solo |

Los mensajes que envía el cliente solo se leen para mantener viva la conexión y por lo demás se ignoran: los comandos van por `POST /console`. El servidor envía un ping cada 25 segundos y corta a un interlocutor que lleve 60 segundos en silencio. Detalles internos: [WebSocket](../../backend/08-websocket.md).
