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
| `400` con un cuerpo de error | Firma incorrecta, token caducado, sin caducidad o con una audiencia que no corresponde |

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
| POST | `/daemon/update` | Token del panel | Inicia una actualización de este nodo | `POST /api/settings/update-panel` |
| GET | `/daemon/update-status` | Token del panel | Estado de la última actualización de este nodo | `GET /api/settings/update-status` |

### Servidores

Todas las rutas siguientes necesitan el token del panel. `:serverId` debe ser un servidor que el nodo tenga (si no, `404` sin cuerpo), salvo en `PUT /daemon/server/:serverId`, que lo crea.

Salvo que la última columna diga otra cosa, la ruta del panel es la misma bajo `/api/servers/:serverId` (por ejemplo, `POST /daemon/server/:serverId/start` para `POST /api/servers/:serverId/start`), descrita en [Servidores](./03-servers.md) y [Contenido del servidor](./04-server-content.md). En esos casos el panel pasa tal cual la consulta y el cuerpo y devuelve sin cambios la respuesta del nodo.

| Método | Ruta (tras `/daemon/server/:serverId`) | Qué hace | Ruta del panel |
|--------|----------------------------------------|----------|----------------|
| PUT | *(ninguna)* | Crea el servidor a partir de una definición | `PUT /api/servers/:serverId`, transferencia entre nodos, clonado |
| DELETE | *(ninguna)* | Borra el servidor, sus archivos y sus copias de seguridad (`?keepBackups=true` conserva las copias) | `DELETE /api/servers/:serverId`, transferencia entre nodos |
| GET | `/definition` | Definición completa | La misma; también transferencia entre nodos y clonado |
| PUT | `/definition` | Sustituye la definición | La misma (el panel filtra lo que envían quienes no son administradores) |
| GET | `/data` | Variables editables por el usuario | La misma |
| POST | `/data` | Cambia variables editables por el usuario | La misma |
| PUT | `/data` | Cambia cualquier variable; `ports` / `primaryPort` se convierten en `port`, `port2`, ... | La misma, además de los ajustes de puertos y los límites de subservidores |
| GET | `/tasks` | Tareas programadas | La misma |
| GET, PUT, DELETE | `/tasks/:taskID` | Una tarea | La misma |
| POST | `/tasks/:taskID/run` | Ejecuta una tarea ahora | La misma |
| POST | `/reload` | Recarga la definición desde el disco | La misma |
| POST | `/start`, `/restart` | Inicia / reinicia | La misma (antes, el panel rechaza los servidores suspendidos) |
| POST | `/stop`, `/kill` | Detiene / fuerza la parada | La misma |
| POST | `/install` | Ejecuta la instalación de la plantilla | La misma; también justo después de crear el servidor |
| GET, PUT, POST, DELETE | `/file/*filename` | Leer, escribir, mover y borrar archivos | La misma; también las dos transferencias |
| POST | `/archive/*filename` | Comprime archivos | La misma; también las dos transferencias |
| POST | `/extract/*filename` | Descomprime un archivo | La misma; también las dos transferencias |
| GET | `/console` | Búfer de la consola | La misma |
| POST | `/console` | Envía un comando | La misma |
| GET, POST | `/flags` | Opciones de inicio y reinicio automáticos | La misma |
| GET | `/stats` | CPU, memoria, disco, red | La misma |
| GET | `/status` | En marcha / instalando | La misma; también las comprobaciones de disponibilidad (uptime) |
| HEAD, GET | `/query` | Consulta al juego | La misma |
| GET | `/crashes` | Informes de fallos | La misma |
| GET | `/metrics` | Historial de rendimiento (`range` `1h`, `24h` o `7d`) | La misma |
| GET, POST | `/version` | Versiones del juego / cambio de versión | La misma |
| GET, PUT | `/notifications` | Canales de notificación | La misma |
| POST | `/notifications/test` | Mensaje de prueba | La misma |
| GET, DELETE | `/plugins` | Plugins instalados / quitar uno (`?name=`) | La misma |
| GET | `/plugins/search` | Busca en Modrinth (`?q=`) | La misma |
| POST | `/plugins/:pluginId` | Instala un plugin (`?loaders=` y `?game_versions=` filtran las versiones de Modrinth) | La misma |
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

### `POST /daemon/update`

Inicia una actualización de la máquina en la que se ejecuta el nodo, en un contenedor de vida corta (si falta la imagen de actualización, primero la descarga). Respuesta: `{"message": "Update initiated on this node.", "containerId": "..."}`; `500` cuando no se puede contactar con Docker o el contenedor no arranca. Véase [Actualizaciones del panel](../../features/panel-updates.md).

### `GET /daemon/update-status`

```json
{ "running": false, "containerId": "...", "startedAt": "2026-11-02T10:00:00Z", "finishedAt": "2026-11-02T10:03:12Z", "exitCode": 0, "log": "...", "error": "" }
```

El estado de la última actualización iniciada en este nodo: si sigue en marcha, su código de salida y el final de su registro.

## Rutas de servidor

Aquí solo se describe en qué se diferencian de las rutas del panel a las que dan servicio.

### `PUT /daemon/server/:serverId`

Cuerpo: una definición de servidor. Si el nodo ya tiene un servidor con ese identificador, primero lo borra (con sus archivos). El entorno debe estar permitido (`400` para un entorno del propio equipo con `docker.disallowHost`) y deben cumplirse los requisitos de la plantilla (`400`). Respuesta: `{"id": "<serverId>"}`. No instala el servidor; el panel llama después a `/install` cuando `panel.servers.autoInstall` está activado.

### `DELETE /daemon/server/:serverId`

Fuerza la parada del servidor si está en marcha, ejecuta los pasos de desinstalación de la plantilla y borra la carpeta del servidor y todo lo que el nodo guarda de él (definición, tareas, informes de fallos, ajustes de notificación, métricas, estado del mapa del mundo y del modpack, historial de archivos y su carpeta `/tmp`). Si falla un paso de desinstalación, se registra en el log y se muestra en la consola, y el borrado continúa. Después se borra la carpeta de copias de seguridad del servidor en este nodo (`<daemon.data.backups.folder>/<serverId>`), salvo que la petición lleve `?keepBackups=true`, que el panel envía cuando traslada el servidor a otro nodo; las copias del almacenamiento externo no se tocan. `204`.

### Acciones de encendido

`start`, `restart`, `stop` e `install` responden `202` y trabajan en segundo plano; con `?wait` responden `204` al terminar. `start` y `restart` responden `409` de inmediato cuando el disco del servidor está lleno o alguno de sus puertos ya lo ocupa otro contenedor en marcha. `kill` responde `204`.

### Archivos

Las mismas reglas que las [rutas de archivos del panel](./04-server-content.md#archivos), incluida la opción `?folder` y que la cadena de consulta se lea como parte del nombre del archivo en `GET`, `PUT` y `DELETE`. Las dos [transferencias](./08-external-transfer.md#qué-se-traslada) usan estas rutas con el archivo `transfer.tar.gz` y `extract/transfer.tar.gz?destination=.`.

### Copias de seguridad

El nodo identifica las copias por el nombre del archivo, no por el ID del registro del panel: `GET /backup` lista los archivos terminados (los más recientes primero) y, después, las copias hechas a mano cuyo archivo no se pudo escribir, con `"location": "failed"` (el panel elimina sus registros al verlas y llama a `DELETE` para borrar la marca). `restore`, `download` y `DELETE` reciben `?fileName=`. `POST /backup/create` responde `{"backupFileName": "..."}` y escribe el archivo en segundo plano. `restore` responde `400 ErrBackupServerRunning` mientras el servidor está en marcha y, si no, `202`. Véase [Contenido del servidor → Copias de seguridad](./04-server-content.md#copias-de-seguridad).

### `POST /daemon/server/:serverId/clone-from/:sourceId`

Copia en segundo plano los archivos del servidor `:sourceId` (del mismo nodo) en `:serverId`; el destino cuenta como "instalando" hasta que termina la copia. Cuerpo opcional `{"sourceName": "Survival"}` (el nombre que se muestra en la consola, cortado a 80 caracteres). `202`; `404` cuando el origen no está en el nodo; `400` cuando ambos son el mismo servidor; `409` cuando alguno está en marcha, el destino no está vacío o hay una copia de seguridad en curso.

### `GET /daemon/server/:serverId/map/view/*path`

Proxy inverso al servidor web propio del mapa del mundo (BlueMap o Dynmap). Solo se reenvían unas pocas cabeceras de la petición y se quita `Set-Cookie` de la respuesta. `400` para una ruta con `..`; `502` cuando el servidor web del mapa no responde. El navegador solo llega a él mediante el enlace firmado del panel; véase [Contenido del servidor → Mapa del mundo](./04-server-content.md#mapa-del-mundo).

### `GET /daemon/server/:serverId/socket`

Apertura de WebSocket. Las opciones de consulta `console`, `stats` y `status` eligen qué recibe el socket. El panel sustituye lo que haya pedido el navegador por los flujos que permiten los scopes de quien llama (`server.console`, `server.stats`, `server.status`, resueltos como en cualquier otra ruta) antes de abrir el socket con el nodo usando su propio token.
