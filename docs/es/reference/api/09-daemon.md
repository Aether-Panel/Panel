# Daemon

Las rutas que sirve cada nodo bajo `/daemon`. El panel las llama en el nodo donde se ejecuta un servidor: para reenviar la mayoría de las peticiones `/api/servers/:serverId/...`, para crear, mover y borrar servidores y para leer el estado del nodo. Son rutas **del panel al nodo**, no pensadas para usuarios, scripts ni sistemas de facturación: estos usan la [API del panel](../11-api-reference.md), que comprueba los permisos y después llama al nodo. Volver a la [referencia de la API](../11-api-reference.md).

Código: `internal/web/daemon/*.go` (rutas), `internal/middleware/decrypt.go` (comprobación del token), `internal/services/node.go` e `internal/services/token.go` (el lado del panel).

## Dónde responde el nodo

- Las rutas del daemon se registran cuando `daemon.enable` está activado (lo está por defecto) y las sirve el mismo servidor web que el resto del proceso (`web.host`, por defecto `0.0.0.0:8080`). El panel llega a un nodo en su `privateHost:privatePort`.
- En una instalación combinada (panel y daemon en un mismo proceso, el nodo "local"), el panel ejecuta la petición dentro del propio proceso, sin pasar por la red. Aun así firma un token, y el nodo lo comprueba igual.
- Para saber si un nodo habla HTTPS, el panel envía `OPTIONS /daemon` por HTTPS y después por HTTP. Un nodo que se ha visto alguna vez con HTTPS queda fijado a él y nunca se vuelve a contactar por HTTP sin cifrar.
- Cuando el panel reenvía una petición del navegador, quita la cabecera `Cookie`, de modo que la sesión de un usuario nunca llega a un nodo.

## Autenticación

| Rutas | Autenticación |
|-------|---------------|
| `GET`, `HEAD`, `OPTIONS /daemon`, `GET /daemon/features` | Ninguna |
| Todas las demás | `Authorization: Bearer <token>` firmado por el panel |

El token es un JWT que el panel firma con su clave Ed25519 (`EdDSA`, identificador de clave `SkyPanel`) para cada llamada. Caduca a los 15 minutos (con un minuto de margen para desfases de reloj) y su audiencia se deriva del secreto del nodo al que se envía, así que un token capturado de camino a un nodo no sirve en otro. Un nodo que solo ejecuta el daemon (`panel.enable=false`) lee las claves públicas del panel del JWKS de `token.public` (el [`GET /auth/publickey`](./02-authentication.md#get-authpublickey) del panel). Detalles: [Seguridad → Tokens entre el panel y los nodos](../../backend/09-security.md#tokens-entre-el-panel-y-los-nodos).

El token **no lleva usuario ni permisos**: cualquier token válido del panel puede llamar a cualquier ruta de ese nodo. Los permisos se comprueban en la ruta del panel antes de que este llame al nodo (véase [Permisos (scopes)](./02-authentication.md#permisos-scopes)).

| Respuesta | Cuándo |
|-----------|--------|
| `401 {"error": "invalid_request"}` con `WWW-Authenticate: Bearer` | Sin cabecera `Authorization`, o no tiene la forma `Bearer <token>` |
| `401 {"error": "invalid_token"}` con `WWW-Authenticate: Bearer error="invalid_token"` | Firma incorrecta, token caducado, sin caducidad o con una audiencia que no corresponde (el nodo registra el motivo) |

Un token que no supera la comprobación suele indicar que el `daemon.auth.clientSecret` del nodo no es el secreto que el panel tiene para ese nodo, o que el reloj de una de las dos máquinas está desajustado. El panel no pasa ese `401` al navegador (ni un `403` con `WWW-Authenticate`, de un proxy delante del nodo): responde `502 ErrNodeAuthFailed` y registra el nombre del nodo (véase [Convenciones → Fallos de autenticación con el nodo](./01-conventions.md#fallos-de-autenticación-con-el-nodo)). Los nodos anteriores a esta versión respondían `400` en este caso.

## Rutas

### Nodo

| Método | Ruta | Autenticación | Qué hace | Lo usa |
|--------|------|---------------|----------|--------|
| GET | `/daemon` | Ninguna | `{"message": "daemon is running"}` | Comprobaciones de disponibilidad |
| HEAD | `/daemon` | Ninguna | `204` | Comprobaciones de disponibilidad |
| OPTIONS | `/daemon` | Ninguna | Métodos permitidos | La detección de HTTPS del panel |
| GET | `/daemon/features` | Ninguna | Sistema operativo, arquitectura, versión, entornos, Docker | [`GET /api/nodes/:id/features`](./06-panel-admin.md#get-apinodesidfeatures) |
| GET | `/daemon/system` | Token del panel | Equipo, CPU, memoria, discos, contadores de red | [`GET /api/nodes/:id/system`](./06-panel-admin.md#get-apinodesidsystem) |
| GET | `/daemon/ports` | Token del panel | Puertos del equipo publicados por los contenedores del nodo | [`POST /api/nodes/:id/ports/check`](./06-panel-admin.md#post-apinodesidportscheck) |
| GET | `/daemon/network/history` | Token del panel | Tráfico del nodo en la última hora, 6 horas o día | [`GET /api/nodes/network-history`](./06-panel-admin.md#get-apinodesnetwork-historyrange1h) |
| GET | `/daemon/servers/status` | Token del panel | Estado (en marcha / instalando) de todos los servidores del nodo; la petición también lleva el idioma de las notificaciones del panel | El sondeo de disponibilidad del panel y el informe de nodos de Discord |
| POST | `/daemon/update` | Token del panel | Inicia una actualización de este nodo | `POST /api/settings/update-panel` |
| GET | `/daemon/update-status` | Token del panel | Estado de la última actualización de este nodo | `GET /api/settings/update-status` |
| GET | `/daemon/settings` | Token del panel | Claves y hash de los ajustes de nodo que reparte el panel (nunca sus valores) | El reparto de ajustes de nodo del panel ([`GET /api/settings/node-sync`](./06-panel-admin.md#get-apisettingsnode-sync)) |
| PUT | `/daemon/settings` | Token del panel | Guarda y aplica esos ajustes sin reiniciar | El reparto de ajustes de nodo del panel |

### Servidores

Todas las rutas siguientes necesitan el token del panel. `:serverId` debe ser un servidor que el nodo tenga (si no, `404` sin cuerpo), salvo en `PUT /daemon/server/:serverId`, que lo crea.

Salvo que la última columna diga otra cosa, la ruta del panel es la misma bajo `/api/servers/:serverId` (por ejemplo, `POST /daemon/server/:serverId/start` para `POST /api/servers/:serverId/start`), descrita en [Servidores](./03-servers.md) y [Contenido del servidor](./04-server-content.md). En esos casos el panel pasa tal cual la consulta y el cuerpo y devuelve sin cambios la respuesta del nodo.

| Método | Ruta (tras `/daemon/server/:serverId`) | Qué hace | Ruta del panel |
|--------|----------------------------------------|----------|----------------|
| PUT | *(ninguna)* | Crea el servidor a partir de una definición | `PUT /api/servers/:serverId`, transferencia entre nodos, clonado |
| DELETE | *(ninguna)* | Borra el servidor, sus archivos y sus copias de seguridad (`?keepBackups=true` conserva las copias) | `DELETE /api/servers/:serverId`, transferencia entre nodos (y su deshacer) |
| GET | `/definition` | Definición completa | La misma; también transferencia entre nodos y clonado |
| PUT | `/definition` | Sustituye la definición | La misma (el panel filtra lo que envían quienes no son administradores) |
| GET | `/data` | Variables editables por el usuario | La misma |
| POST | `/data` | Cambia variables editables por el usuario | La misma |
| PUT | `/data` | Cambia cualquier variable; `ports` / `primaryPort` se convierten en `port`, `port2`, ... | La misma, además de los ajustes de puertos y los límites de subservidores |
| GET | `/tasks` | Tareas programadas | La misma |
| GET, PUT, DELETE | `/tasks/:taskID` | Una tarea | La misma |
| POST | `/tasks/:taskID/run` | Ejecuta una tarea ahora (se omite mientras las tareas están pausadas) | La misma |
| POST | `/scheduler/pause`, `/scheduler/resume` | Pausa / reanuda las tareas programadas del servidor | Transferencia entre nodos |
| POST | `/reload` | Recarga la definición desde el disco | La misma |
| POST | `/start`, `/restart` | Inicia / reinicia | La misma (antes, el panel rechaza los servidores suspendidos) |
| POST | `/stop`, `/kill` | Detiene / fuerza la parada | La misma |
| POST | `/install` | Ejecuta la instalación de la plantilla | La misma; también justo después de crear el servidor |
| GET, PUT, POST, DELETE | `/file/*filename` | Leer, escribir, mover y borrar archivos | La misma; también las dos transferencias y el asistente del servidor |
| GET | `/fs/stat` | Describe un archivo o una carpeta | Ninguna: el asistente del servidor del panel ([abajo](#archivos)) |
| POST | `/fs/copy` | Copia un archivo o una carpeta | Ninguna: el asistente del servidor del panel |
| GET | `/fs/search` | Busca en nombres o en el contenido de los archivos | Ninguna: el asistente del servidor del panel |
| POST | `/archive/*filename` | Comprime archivos | La misma; también las dos transferencias |
| POST | `/extract/*filename` | Descomprime un archivo | La misma; también las dos transferencias |
| GET | `/console` | Búfer de la consola | La misma |
| POST | `/console` | Envía un comando | La misma |
| GET, POST | `/flags` | Opciones de inicio y reinicio automáticos | La misma |
| GET | `/stats` | CPU, memoria, disco, red | La misma |
| GET | `/status` | En marcha / instalando | La misma; también las comprobaciones de disponibilidad (uptime) y la transferencia entre nodos |
| HEAD, GET | `/query` | Consulta al juego | La misma |
| GET | `/crashes` | Informes de fallos | La misma |
| GET | `/metrics` | Historial de rendimiento (`range` `1h`, `24h` o `7d`) | La misma |
| GET, POST | `/version` | Versiones del juego / cambio de versión | La misma |
| GET, PUT | `/notifications` | Canales de notificación | La misma |
| POST | `/notifications/test` | Mensaje de prueba | La misma |
| GET, DELETE | `/plugins` | Plugins o mods instalados / quitar uno (`?name=`), en `mods/` en Fabric, Forge y NeoForge y en `plugins/` en los demás casos | La misma |
| GET | `/plugins/search` | Busca en Modrinth para la plataforma del servidor (`?q=`) | La misma |
| POST | `/plugins/:pluginId` | Instala un plugin o un mod (`?loaders=` y `?game_versions=` filtran las versiones de Modrinth) | La misma |
| GET | `/modpacks`, `/modpacks/search`, `/modpacks/versions` | Estado y búsqueda de modpacks | La misma |
| POST | `/modpacks/install` | Instala un modpack | La misma |
| GET | `/file-history` | Versiones de un archivo (`?path=`) | La misma |
| GET | `/file-history/:versionId` | Texto de una versión | La misma |
| POST | `/file-history/:versionId/restore` | Restaura una versión | La misma |
| GET | `/map` | Estado del mapa del mundo | `GET /api/servers/:serverId/map` (el panel añade el enlace del visor) |
| DELETE | `/map` | Quita el plugin del mapa | La misma |
| POST | `/map/install` | Instala BlueMap o Dynmap | La misma |
| GET, HEAD | `/map/view/*path` | Proxy inverso al servidor web del mapa | `/api/map-view/:serverId/:token/*path` |
| GET | `/backup` | Archivos de copia de seguridad del nodo | Sincronización de la lista de copias |
| POST | `/backup/create` | Inicia una copia de seguridad | `POST /api/servers/:serverId/backup/create` |
| POST | `/backup/restore?fileName=` | Restaura una copia | `POST /api/servers/:serverId/backup/restore/:backupID` |
| GET | `/backup/download?fileName=` | Envía una copia en flujo continuo | `GET /api/servers/:serverId/backup/download/:backupID` |
| DELETE | `/backup?fileName=` | Borra el archivo de una copia | `DELETE /api/servers/:serverId/backup/:backupID` |
| GET | `/backup/busy` | Si se está escribiendo una copia, un punto de restauración o una restauración | Transferencia entre nodos |
| PUT | `/backup/file?fileName=&modTime=` | Recibe el archivo de una copia copiado desde otro nodo | Transferencia entre nodos |
| DELETE | `/backup/file?fileName=` | Quita la copia de este nodo y deja la del almacenamiento externo | Transferencia entre nodos (deshacer) |
| GET, PUT | `/offsite` | Ajuste de copia periódica al S3 del servidor (`<id>.offsite`) | La misma (`/api/servers/:serverId/offsite`); también la transferencia entre nodos, que lo copia al nodo nuevo |
| POST | `/clone-from/:sourceId` | Copia en este servidor los archivos de otro | [`POST /api/servers/:serverId/clone`](./03-servers.md#post-apiserversserveridclone) |
| GET | `/socket` | WebSocket: consola, estadísticas, estado | [`GET /api/servers/:serverId/socket`](./03-servers.md#websocket) |

## Rutas del nodo

### `GET /daemon/features`

```json
{ "features": ["docker"], "environments": ["docker"], "os": "linux", "arch": "amd64", "version": "3.x.y" }
```

`features` incluye `docker` cuando el nodo consigue contactar con Docker en menos de un segundo. Con `docker.disallowHost` activado, los entornos del propio equipo (`host`, `tty`, `standard`, `bubblewrap`) no aparecen en `environments`.

### `GET /daemon/system`

El objeto que devuelve el panel en [`GET /api/nodes/:id/system`](./06-panel-admin.md#get-apinodesidsystem). El uso de CPU se mide en segundo plano cada dos segundos, así que la llamada responde al momento. `networkBytesSent` y `networkBytesRecv` suman las interfaces físicas (se excluyen loopback, los puentes de Docker, `veth`, las VPN y demás interfaces virtuales y las VLAN); dentro de un contenedor son los del equipo anfitrión cuando `daemon.hostProc` apunta a su `/proc`.

### `GET /daemon/ports`

```json
{ "ports": { "8080": "PANEL_CONTAINER_NAME", "25565": "SERVER_CONTAINER_NAME" } }
```

Puertos del equipo publicados por los contenedores en marcha en el nodo, cada uno con el nombre del contenedor. Vacío cuando no se puede contactar con Docker.

### `GET /daemon/network/history?range=1h`

`range` es `1h` (por defecto), `6h` o `24h`; cualquier otro valor responde `400 {"error": "range must be 1h, 6h or 24h"}`.

```json
{ "step": 60, "points": [ { "t": 1793576400, "rx": 0.49, "tx": 0.37 } ] }
```

El nodo lee sus contadores de tráfico cada 10 segundos y guarda un punto por minuto durante 24 horas en el archivo `network.history`, junto a la carpeta de servidores (se guarda cada 5 minutos, así que sobrevive a los reinicios). La respuesta promedia esos minutos en pasos de 1, 5 o 15 minutos. `t` es el inicio del paso en segundos Unix; `rx` y `tx` son los KB/s recibidos y enviados. Los pasos que el nodo no midió (estaba apagado) no tienen punto. El panel suma los nodos en [`GET /api/nodes/network-history`](./06-panel-admin.md#get-apinodesnetwork-historyrange1h); allí, un nodo demasiado antiguo para tener esta ruta cuenta como no disponible.

### `GET /daemon/servers/status`

```json
{ "servers": { "3F9A1C2B7D4E": { "running": true, "installing": false }, "a1b2c3d4": { "running": false, "installing": true } } }
```

El estado de cada servidor cargado en el nodo, por ID de servidor: lo mismo que `GET /daemon/server/:serverId/status`, para todos a la vez. Un servidor cuyo estado no se puede leer se omite, para que el panel lo tome como desconocido y no como detenido. El panel se lo pide a cada nodo remoto cada 30 segundos para registrar la [disponibilidad](../../general/13-uptime.md), y usa la última respuesta en el informe de nodos de Discord. Un nodo anterior a esta ruta responde `404`, y el panel lo trata como inalcanzable hasta que se actualice.

El panel envía en cada sondeo su idioma de las notificaciones (`panel.settings.notificationLanguage`) en la cabecera `X-Aether-Notification-Language: es|en`, y sondea al momento cuando ese ajuste se guarda con un valor nuevo. El nodo guarda el valor en memoria y redacta en él las alertas de sus servidores, salvo que el archivo de configuración o el entorno del propio nodo definan `panel.settings.notificationLanguage`, que tiene prioridad. Un valor vacío o desconocido se ignora. Tras reiniciarse el nodo, sus alertas usan `es` (o su propio valor) hasta el siguiente sondeo, como mucho 30 segundos después. Véase [Notificaciones del servidor](../../features/server-notifications.md).

### `POST /daemon/update`

Inicia una actualización de la máquina en la que se ejecuta el nodo, en un contenedor de vida corta (si falta la imagen de actualización, primero la descarga). Respuesta: `{"message": "Update initiated on this node.", "containerId": "..."}`; `500` cuando no se puede contactar con Docker o el contenedor no arranca. Véase [Actualizaciones del panel](../../features/panel-updates.md).

### `GET /daemon/update-status`

```json
{ "running": false, "containerId": "...", "startedAt": "2026-11-02T10:00:00Z", "finishedAt": "2026-11-02T10:03:12Z", "exitCode": 0, "log": "...", "error": "" }
```

El estado de la última actualización iniciada en este nodo: si sigue en marcha, su código de salida y el final de su registro.

### `GET /daemon/settings`

```json
{ "hash": "9b1c…e4", "keys": ["daemon.backups.s3.enabled", "daemon.backups.s3.endpoint", "…", "daemon.curseforge.key"] }
```

Los ajustes de nodo que el panel reparte a todos los nodos, es decir, las copias externas (`daemon.backups.s3.*`: `enabled`, `endpoint`, `region`, `bucket`, `accessKey`, `secretKey`, `prefix`, `insecure`, `periodicHours`, `periodicKeep`) y la clave de CurseForge (`daemon.curseforge.key`). `keys` enumera las que conoce este nodo y `hash` es el sha256 (hex) del JSON canónico de sus valores actuales. Los valores nunca se devuelven, porque incluyen secretos. El panel calcula el hash de sus propios valores para las claves que conocen los dos y los compara, así que un panel y un nodo de versiones distintas coinciden igualmente en los ajustes que comparten; si los hashes no coinciden, envía `PUT /daemon/settings`.

Autenticación: el token del panel para este nodo, como el resto de rutas de aquí, así que solo el panel puede leer o cambiar estos ajustes. Un nodo anterior a esta ruta responde `404`, y la página de configuración lo muestra como sin actualizar.

### `PUT /daemon/settings`

```json
{ "daemon.backups.s3.enabled": true, "daemon.backups.s3.bucket": "mis-copias", "daemon.backups.s3.periodicHours": 24, "daemon.curseforge.key": "…" }
```

El nodo comprueba primero todo el cuerpo y después guarda cada valor en su `config.json` y lo aplica sin reiniciar (los ajustes de S3 se leen en cada uso y la copia periódica lee sus valores por defecto en cada vuelta). Responde `204`.

- El cuerpo lleva **cualquier subconjunto** de las claves anteriores: una clave que no venga se queda como está, así que un panel antiguo que envía menos claves sigue funcionando.
- Una clave que este nodo no conoce (de un panel más nuevo) se ignora.
- Una clave conocida con un valor de tipo incorrecto (`400 ErrNodeSettingInvalid`, que nombra la clave, nunca el valor), un `periodicHours` que el panel no ofrece (`400 ErrOffsitePeriodicHours`) o un `periodicKeep` fuera de 1-50 (`400 ErrOffsiteKeep`) rechaza todo el cuerpo: **no se aplica nada**.
- Un cuerpo sin ninguna clave conocida no cambia nada y también responde `204`.

El panel la llama después de un guardado en **Configuración** que cambie alguna de estas claves y en su pasada cada 10 minutos; los valores que envía sobrescriben lo que el nodo tuviera puesto a mano. Véase [Copias de seguridad → Requisitos y configuración](../../features/backups.md#requisitos-y-configuración).

## Rutas de servidor

Aquí solo se describe en qué se diferencian de las rutas del panel a las que dan servicio.

### `PUT /daemon/server/:serverId`

Cuerpo: una definición de servidor. Si el nodo ya tiene un servidor con ese identificador, primero lo borra (con sus archivos). El entorno debe estar permitido (`400` para un entorno del propio equipo con `docker.disallowHost`) y deben cumplirse los requisitos de la plantilla (`400`). Respuesta: `{"id": "<serverId>"}`. No instala el servidor; el panel llama después a `/install` cuando `panel.servers.autoInstall` está activado.

### `DELETE /daemon/server/:serverId`

Fuerza la parada del servidor si está en marcha, ejecuta los pasos de desinstalación de la plantilla y borra la carpeta del servidor y todo lo que el nodo guarda de él (definición, tareas, informes de fallos, ajustes de notificación, ajuste de copia periódica (`<id>.offsite`), métricas, estado del mapa del mundo y del modpack, historial de archivos y su carpeta `/tmp`). Si falla un paso de desinstalación, se registra en el log y se muestra en la consola, y el borrado continúa. Después se borra la carpeta de copias de seguridad del servidor en este nodo (`<daemon.data.backups.folder>/<serverId>`), salvo que la petición lleve `?keepBackups=true`, que el panel envía cuando deshace una transferencia entre nodos y quita el servidor del nodo nuevo (solo retira, una a una con `DELETE /backup/file`, las copias que llevó). Tras una transferencia correcta, el nodo antiguo recibe un `DELETE` normal: sus copias ya se copiaron al nodo nuevo. Las copias del almacenamiento externo nunca se tocan. `204`.

### Acciones de encendido

`start`, `restart`, `stop` e `install` responden `202` y trabajan en segundo plano; con `?wait` responden `204` al terminar. `start` y `restart` responden `409` de inmediato cuando el disco del servidor está lleno (`ErrDiskLimitExceeded`) o alguno de sus puertos de host ya lo publica otro contenedor en marcha o lo ocupa otro proceso (`ErrPortBusy`). `kill` responde `204`.

### Archivos

Las mismas reglas que las [rutas de archivos del panel](./04-server-content.md#archivos), incluida la opción `?folder` y que la cadena de consulta se lea como parte del nombre del archivo en `GET`, `PUT` y `DELETE`. Las dos [transferencias](./08-external-transfer.md#qué-se-traslada) usan estas rutas con el archivo `transfer.tar.gz` y `extract/transfer.tar.gz?destination=.`.

Cambios hechos para el asistente del servidor (se aplican a cualquiera que llame a estas rutas):

- `PUT /file/*filename?create=true` solo crea un archivo nuevo: lo abre con `O_EXCL`, así que no sustituye nada de lo que ya haya (archivo, carpeta o enlace), y responde `409 ErrFileExists` cuando ya existe algo. Con `create` en la consulta (con cualquier valor), la consulta **no** se lee como parte del nombre, y no se guarda ninguna versión anterior en el historial de archivos (no la hay). Se crean las carpetas superiores que falten y se aplica el límite de disco como en cualquier `PUT` (`409 ErrDiskLimitExceeded`).
- `DELETE /file/*filename` describe el destino sin seguirlo: un enlace simbólico se elimina a sí mismo (se desvincula de su carpeta) y aquello a lo que apunta no se toca. Antes, borrar un enlace a una carpeta vaciaba esa carpeta y luego no conseguía eliminar el enlace. Una ruta que no existe responde `404` sin cuerpo; una ruta vacía, o que se resuelve en la propia carpeta del servidor, responde `400 ErrIllegalFileAccess` en lugar de vaciar el servidor.

El asistente del servidor (véase [Kit de herramientas del agente de IA](../../backend/14-ai-toolkit.md)) usa además tres rutas que no tienen ruta en el panel: el panel solo las llama desde su `nodeWorkspace`, para el servidor de la conversación. Funcionan mediante el workspace del kit sobre el servidor de archivos confinado del servidor, así que las rutas se vuelven a comprobar en el nodo (relativas a la carpeta del servidor; sin `/` inicial, letra de unidad, `..` ni NUL; como mucho 4096 bytes y 64 elementos) y los enlaces simbólicos no se siguen fuera de la carpeta.

#### `GET /daemon/server/:serverId/fs/stat?path=`

`path` es relativa a la carpeta del servidor; vacía (o `.`) es la propia carpeta. Respuesta:

```json
{ "name": "server.properties", "path": "server.properties", "isDir": false, "size": 1187, "modTime": "2026-11-02T09:41:00Z" }
```

Un enlace simbólico que queda dentro de la carpeta del servidor se describe por aquello a lo que apunta. La carpeta del servidor responde `name` y `path` `"."`. `size` es `0` en las carpetas.

#### `POST /daemon/server/:serverId/fs/copy`

Cuerpo `{"source": "config", "destination": "config-backup"}`. Copia un archivo o un árbol de carpetas; `204`. El destino no debe existir ni estar dentro del origen; se crean las carpetas superiores que falten. Un enlace simbólico como origen se rechaza; dentro de una carpeta copiada se omiten los enlaces simbólicos, dispositivos, sockets y tuberías. Antes de copiar, el nodo comprueba el disco (`ErrDiskLimitExceeded`) y, si hay límite de disco, suma los archivos normales que se copiarían y lo rechaza con `409 ErrDiskLimitExceeded` si no caben; después vuelve a medir la carpeta.

#### `GET /daemon/server/:serverId/fs/search?dir=&query=&content=`

Busca `query` (sin distinguir mayúsculas, como mucho 200 caracteres, no vacía) por debajo de `dir` (vacía es la carpeta del servidor), por niveles: en los nombres de archivos y carpetas o, con `content=true`, en las líneas de los archivos de texto. Límites: 8 carpetas de profundidad, 5000 entradas revisadas, 200 coincidencias, archivos de hasta 1 MiB; se omiten los archivos binarios y el interior de las carpetas enlazadas.

```json
{ "matches": [ { "path": "logs/latest.log", "line": 214, "text": "[ERROR] Could not load plugin" } ], "truncated": false }
```

Una coincidencia de nombre solo tiene `path`; una de contenido tiene además `line` (desde 1) y `text` (la línea sin espacios en los extremos, como mucho 300 caracteres). `truncated` es `true` cuando la búsqueda se detuvo en 200 coincidencias.

#### Errores de las rutas `fs/*`

Responden `{"error": {"code": "...", "msg": "..."}}` con un mensaje que contiene como mucho una ruta relativa al servidor. El panel vuelve a leer el código para saber qué ocurrió.

| Estado | Código | Cuándo |
|---|---|---|
| `400` | `ErrFileInvalidPath` | Ruta vacía, demasiado larga o demasiado profunda, byte NUL |
| `400` | `ErrFileOutsideServer` | Ruta absoluta, letra de unidad, `..`, o una ruta o enlace que sale de la carpeta del servidor |
| `400` | `ErrFileNotAFolder`, `ErrFileIsAFolder` | Un archivo donde hace falta una carpeta, o al revés |
| `400` | `ErrFileSymlink`, `ErrFileNotRegular` | Un enlace simbólico, dispositivo, socket o tubería donde hace falta un archivo normal o una carpeta |
| `400` | `ErrFileInvalidRequest` | Cuerpo incorrecto, texto de búsqueda vacío o demasiado largo, un destino dentro de su origen |
| `403` | `ErrFileProtected`, `ErrFilePermission` | Una ruta protegida (ninguna por defecto); el sistema operativo lo rechazó |
| `404` | `ErrFileNotFound` | No existe |
| `409` | `ErrFileExists` | El destino ya existe |
| `409` | `ErrDiskLimitExceeded` | La copia no cabe en el límite de disco del servidor |
| `413` | `ErrFileTooLarge` | Supera un límite de tamaño |
| `507` | `ErrFileNoSpace` | El disco del nodo está lleno |
| `500` | `ErrFileOperationFailed` | Cualquier otro caso (la causa solo se anota en el log del nodo) |

### Copias de seguridad

El nodo identifica las copias por el nombre del archivo, no por el ID del registro del panel: `GET /backup` lista los archivos terminados (los más recientes primero) y, después, las copias hechas a mano cuyo archivo no se pudo escribir, con `"location": "failed"` (el panel elimina sus registros al verlas y llama a `DELETE` para borrar la marca). Si el almacenamiento externo está configurado pero no se pudo listar, la respuesta lleva la cabecera `X-Backups-Offsite: unavailable` y el cuerpo, con la misma forma, solo trae las copias del nodo; el panel no borra entonces los registros de las copias que estaban en el bucket ni pasa a `local` las que estaban en `both`. Un nodo sin bucket, o que lo pudo listar, no envía la cabecera, igual que un nodo antiguo. `restore`, `download` y `DELETE` reciben `?fileName=`. `POST /backup/create` responde `{"backupFileName": "..."}` y escribe el archivo en segundo plano. `restore` responde `400 ErrBackupServerRunning` mientras el servidor está en marcha y, si no, `202`. Véase [Contenido del servidor → Copias de seguridad](./04-server-content.md#copias-de-seguridad).

Una [transferencia entre nodos](./03-servers.md#post-apiserversserveridtransfer) usa tres rutas más:

- `GET /backup/busy` responde `{"busy": true}` mientras se escribe una copia, un punto de restauración o una restauración del servidor. Entretanto, el panel rechaza la transferencia; a un nodo sin esta ruta no se le pregunta.
- `PUT /backup/file?fileName=<nombre>&modTime=<RFC 3339>` recibe el archivo como cuerpo sin procesar (`application/octet-stream`) y lo escribe en la carpeta de copias del servidor con el mismo nombre, conservando su fecha (`modTime`, opcional), por la que se ordenan las copias y se aplica la retención de las automáticas. El nombre debe ser un nombre de archivo `.tar.gz` simple y no oculto. Se escribe con un nombre oculto y se mueve a su sitio al terminar, así que una copia interrumpida nunca aparece en la lista. `204`; `409 ErrFileExists` si ya hay una copia con ese nombre (el panel la deja como está); `400` si `modTime` no es válido; `500` si el nombre no es válido o falla la escritura.
- `DELETE /backup/file?fileName=<nombre>` quita solo la copia de este nodo (que el archivo no exista no es un error), a diferencia de `DELETE /backup`, que también borra la copia externa. `204`.

### `POST /daemon/server/:serverId/scheduler/pause` y `/scheduler/resume`

Sin cuerpo. `pause` impide que las tareas programadas del servidor se ejecuten hasta `resume`: las ejecuciones que tocan entretanto se omiten (el nodo anota `Task <id> skipped: the server is being moved to another node`), igual que `POST /tasks/:taskID/run`, que aun así responde `204`. Una tarea que ya se está ejecutando no se interrumpe. El estado se guarda como `"paused": true` en el archivo `.cron` del servidor, así que se mantiene si el nodo se reinicia. Pausar un servidor ya pausado, o reanudar uno que no lo está, no hace nada. `204`; `500` si no se puede escribir el archivo.

Una [transferencia entre nodos](./03-servers.md#post-apiserversserveridtransfer) pausa las tareas en ambos nodos mientras copia el servidor (y las copia con `GET /tasks` y `PUT /tasks/:taskID`), y después las reanuda en el nodo que se queda con el servidor. Un nodo sin estas rutas responde `404` y la transferencia falla en ese paso y se deshace.

Mientras el servidor se recarga o se borra, su planificador está parado: `PUT` y `DELETE /tasks/:taskID` y `POST /tasks/:taskID/run` responden entonces `409 ErrSchedulerStopped`, no guardan nada (el archivo `.cron` del servidor queda como está) y se pueden repetir.

### `POST /daemon/server/:serverId/clone-from/:sourceId`

Copia en segundo plano los archivos del servidor `:sourceId` (del mismo nodo) en `:serverId`; el destino cuenta como "instalando" hasta que termina la copia. Cuerpo opcional `{"sourceName": "Survival"}` (el nombre que se muestra en la consola, cortado a 80 caracteres). `202`; `404` cuando el origen no está en el nodo; `400` cuando ambos son el mismo servidor; `409` cuando alguno está en marcha, el destino no está vacío o hay una copia de seguridad en curso.

### `GET /daemon/server/:serverId/map/view/*path`

Proxy inverso al servidor web propio del mapa del mundo (BlueMap o Dynmap). Solo se reenvían unas pocas cabeceras de la petición y se quita `Set-Cookie` de la respuesta. `400` para una ruta con `..`; `502` cuando el servidor web del mapa no responde. El navegador solo llega a él mediante el enlace firmado del panel; véase [Contenido del servidor → Mapa del mundo](./04-server-content.md#mapa-del-mundo).

### `GET /daemon/server/:serverId/socket`

Apertura de WebSocket. Las opciones de consulta `console`, `stats` y `status` eligen qué recibe el socket. El panel sustituye lo que haya pedido el navegador por los flujos que permiten los scopes de quien llama (`server.console`, `server.stats`, `server.status`, resueltos como en cualquier otra ruta) antes de abrir el socket con el nodo usando su propio token.
