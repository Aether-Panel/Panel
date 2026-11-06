# Contenido del servidor

Rutas bajo `/api/servers/:serverId` que actúan sobre lo que contiene un servidor: archivos, archivos comprimidos, historial de archivos, plugins, modpacks, mapa del mundo, copias de seguridad, tareas programadas y bases de datos MySQL. Volver a la [referencia de la API](../11-api-reference.md).

Las rutas marcadas con «nodo» se reenvían al nodo del servidor (`/daemon/server/:serverId/...`): el panel comprueba el scope y devuelve sin cambios el código de estado y el cuerpo del nodo. Las rutas dentro del servidor (`*filename`, `path`, `destination`) son relativas a la carpeta del servidor; se rechaza todo lo que saldría de ella (`..`, rutas absolutas, enlaces simbólicos que apuntan fuera).

## Rutas

| Método | Ruta | Scope | Qué hace |
|--------|------|-------|----------|
| GET | `/file/*filename` | `server.files.view` | Descarga un archivo o lista una carpeta (nodo) |
| PUT | `/file/*filename` | `server.files.edit` | Sube o sobrescribe un archivo; `?folder` crea una carpeta (nodo) |
| POST | `/file/*filename?destination=` | `server.files.edit` | Cambia el nombre o mueve (nodo) |
| DELETE | `/file/*filename` | `server.files.edit` | Elimina un archivo o una carpeta entera (nodo) |
| POST | `/archive/*filename` | `server.files.edit` | Comprime archivos en un archivo comprimido (nodo) |
| HEAD | `/archive/*filename` | `server.files.edit` | Registrada, pero el nodo no tiene una ruta equivalente: responde `404` |
| POST | `/extract/*filename` | `server.files.edit` | Extrae un archivo comprimido (nodo) |
| GET | `/file-history?path=` | `server.files.view` | Versiones anteriores de un archivo (nodo) |
| GET | `/file-history/:versionId?path=` | `server.files.view` | Texto de una versión (nodo) |
| POST | `/file-history/:versionId/restore?path=` | `server.files.edit` | Restaura una versión (nodo) |
| GET | `/plugins` | `server.files.view` | Plugins o mods `.jar` instalados (nodo) |
| GET | `/plugins/search?q=` | `server.files.view` | Busca en Modrinth para la plataforma del servidor (nodo) |
| POST | `/plugins/:pluginId` | `server.files.edit` | Instala un proyecto de Modrinth (nodo) |
| DELETE | `/plugins?name=` | `server.files.edit` | Quita un plugin o un mod (nodo) |
| GET | `/modpacks` | `server.files.view` | Modpack instalado y progreso de la instalación (nodo) |
| GET | `/modpacks/search` | `server.files.view` | Busca en Modrinth o CurseForge (nodo) |
| GET | `/modpacks/versions` | `server.files.view` | Versiones de un modpack (nodo) |
| POST | `/modpacks/install` | `server.install` | Instala un modpack (nodo) |
| GET | `/map` | `server.view` | Estado del mapa del mundo y enlace al visor |
| POST | `/map/install` | `server.files.edit` | Instala BlueMap o Dynmap (nodo) |
| DELETE | `/map` | `server.files.edit` | Quita el plugin del mapa (nodo) |
| GET | `/backup` | `server.backup.view` | Lista las copias de seguridad |
| GET | `/backup/:backupID` | `server.backup.view` | Una copia de seguridad |
| POST | `/backup/create?name=` | `server.backup.create` | Crea una copia de seguridad |
| POST | `/backup/restore/:backupID` | `server.backup.restore` | Restaura una copia de seguridad |
| GET | `/backup/download/:backupID` | `server.backup.view` | Descarga el archivo |
| DELETE | `/backup/:backupID` | `server.backup.delete` | Elimina una copia de seguridad |
| GET | `/offsite` | `server.backup.view` | Copia periódica al S3 del servidor (nodo) |
| PUT | `/offsite` | `server.backup.create` | Cambia la copia periódica al S3 del servidor (nodo) |
| GET | `/tasks` | `server.tasks.view` | Lista las tareas programadas (nodo) |
| GET | `/tasks/:taskId` | `server.tasks.view` | Una tarea (nodo) |
| PUT | `/tasks/:taskId` | cualquiera de `server.tasks.create`, `server.tasks.edit` | Crea o sustituye una tarea |
| DELETE | `/tasks/:taskId` | `server.tasks.delete` | Elimina una tarea (nodo) |
| POST | `/tasks/:taskId/run` | `server.tasks.run` | Ejecuta una tarea ahora (nodo) |
| GET | `/databases` | `server.view` | Las bases de datos del servidor; las contraseñas solo con `server.data.edit` |
| POST | `/databases` | `server.data.edit` | Crea una base de datos |
| DELETE | `/databases/:id` | `server.data.edit` | Elimina una base de datos |
| GET | `/databasehosts` | `server.data.edit` | Hosts en los que el servidor puede crear bases de datos |

Todas las rutas de la tabla empiezan por `/api/servers/:serverId`. El visor público del mapa del mundo, `/api/map-view/...`, se describe en [Mapa del mundo](#mapa-del-mundo).

## Archivos

### `GET /api/servers/:serverId/file/*filename`

Para una carpeta, un array JSON: en las subcarpetas aparece primero `..`, después las carpetas y después los archivos, por orden alfabético.

```json
[
  { "name": "..", "isFile": false },
  { "name": "world", "isFile": false },
  { "name": "server.properties", "isFile": true, "size": 1452, "modifyTime": 1793576400, "extension": ".properties" }
]
```

`modifyTime` está en segundos Unix. Para un archivo, el contenido en bruto como `application/octet-stream` con `Content-Disposition: attachment`. `404` si no existe; `400 ErrIllegalFileAccess` para una ruta fuera del servidor.

No añada una cadena de consulta a las lecturas de archivos: todo lo que va después de `?` (salvo `folder`) se toma como parte del nombre del archivo.

### `PUT /api/servers/:serverId/file/*filename`

Escribe en el archivo el cuerpo de la petición, o el campo multipart `file` cuando la petición es `multipart/form-data`. Las carpetas superiores que falten se crean. El texto anterior del archivo se conserva en el [historial de archivos](#historial-de-archivos). `204`.

- `?folder`: crea en su lugar la carpeta (y las superiores); `204`.
- `409 ErrDiskLimitExceeded`: el disco del servidor está lleno, o la subida lo llenó (se borra el archivo parcial).

### `POST /api/servers/:serverId/file/*filename?destination=nueva/ruta`

Cambia el nombre de un archivo o carpeta, o lo mueve; la extensión puede cambiar. `204`. `400` si falta `destination` o queda fuera del servidor, `400 ErrFileExists` si ya existe algo en esa ruta.

### `DELETE /api/servers/:serverId/file/*filename`

Elimina un archivo, o una carpeta con todo su contenido. `204`. El espacio liberado cuenta al instante, así que un servidor detenido por disco lleno puede volver a arrancar. No hay ninguna ruta que borre varias rutas a la vez: **Eliminar selección** del gestor de archivos llama a esta ruta una vez por elemento, uno tras otro, y se detiene en el primer fallo.

### `POST /api/servers/:serverId/archive/*filename`

Cuerpo: las rutas que se van a comprimir, por ejemplo `["world", "server.properties"]`. `*filename` es el archivo comprimido que se crea; su extensión determina el formato (véase [Archivos del servidor y archivos comprimidos](../../general/19-files-compression.md)). `204`. El gestor de archivos envía una ruta desde **Comprimir** en una fila, y todas las rutas seleccionadas desde **Comprimir selección**.

| Error | Cuándo |
|-------|--------|
| `400` | Lista vacía, una ruta fuera del servidor, un archivo que no existe o un enlace que apunta fuera del servidor |
| `409 ErrFileExists` | El archivo comprimido ya existe |
| `409 ErrDiskLimitExceeded` | Se alcanzó el límite de disco |

### `POST /api/servers/:serverId/extract/*filename`

Extrae el archivo comprimido. `?destination=` es la carpeta de destino (vacía: la carpeta del servidor); `?skipRoot` descarta la carpeta superior del archivo comprimido. `204`; `400` para rutas fuera del servidor; `409 ErrDiskLimitExceeded`.

### Historial de archivos

Cuando se sobrescribe un archivo desde el panel, se conserva su texto anterior. Detalles y límites: [Historial de archivos](../../features/file-history.md).

- `GET /file-history?path=config.yml` lista las versiones, de la más reciente a la más antigua: `[{"id": "lzx4k2-a1b2c3", "savedAt": 1793576400123, "size": 812}]` (`savedAt` en milisegundos Unix).
- `GET /file-history/:versionId?path=config.yml` responde el texto de esa versión (`text/plain`).
- `POST /file-history/:versionId/restore?path=config.yml` la vuelve a escribir; antes, el texto actual se guarda como una versión más, de modo que la restauración se puede deshacer. `204`.

`400` para una ruta no válida, `404` para una versión desconocida.

## Plugins

Los plugins y los mods proceden de Modrinth y solo existen en servidores de Minecraft: Java Edition (`type` `minecraft-java`): en cualquier otro, las cuatro rutas responden `400 ErrPluginsNotMinecraft` y el panel no muestra la pestaña Plugins. Detalles: [Modpacks y plugins](../../features/modpacks-and-plugins.md).

El nodo deduce la plataforma del servidor igual que para el [mapa del mundo](#mapa-del-mundo) (el cargador del modpack, la variable `modlauncher` y, por último, los pasos de instalación): `paper`, `fabric`, `forge`, `neoforge` o desconocida (vanilla, proxies, otros juegos, lanzadores desconocidos). La carpeta depende de ella: **`mods/`** en Fabric, Forge y NeoForge, **`plugins/`** en los demás casos. Listar, instalar y quitar usan esa carpeta.

### `GET /api/servers/:serverId/plugins`

Los archivos `.jar` de la carpeta del servidor (`mods/` o `plugins/`, véase arriba); una lista vacía si la carpeta no existe.

```json
[ { "name": "EssentialsX-2.20.1.jar", "version": "2.20.1", "size": 1234567 } ]
```

`version` se deduce de la parte del nombre que sigue al último `-`, sin `.jar`; queda vacía si el nombre no contiene ningún `-`.

### `GET /api/servers/:serverId/plugins/search?q=essentials`

Hasta 20 proyectos de Modrinth adecuados para la plataforma del servidor:

| Plataforma | Qué se busca |
|------------|--------------|
| Paper | Plugins y mods con una versión para Paper, Spigot o Bukkit |
| Fabric, Forge, NeoForge | Mods para ese cargador que funcionan en un servidor (`server_side` obligatorio u opcional; se excluyen los mods solo de cliente) |
| Desconocida | Plugins y mods de cualquier cargador |

```json
[ { "id": "hXiIvTyT", "name": "EssentialsX", "tag": "", "description": "...", "author": "...", "iconUrl": "https://...", "downloads": 1000000, "version": "2.20.1" } ]
```

`400` sin `q`; `500` cuando Modrinth no responde.

### `POST /api/servers/:serverId/plugins/:pluginId`

`:pluginId` es el id del proyecto en Modrinth. Consulta opcional: `loaders` (cargadores de Modrinth separados por comas, como `paper,spigot,bukkit`) y `game_versions` (versiones de Minecraft separadas por comas, como `1.21.4`); solo se conservan letras minúsculas, dígitos, `.`, `-` y `_`, hasta 32 caracteres por valor. Elige el archivo principal de la versión más reciente que cumpla esos filtros (cualquier versión si no se envían), comprueba que su nombre sea un nombre de archivo `.jar` simple (sin carpetas ni `..`) y que haya espacio en disco, y después guarda un punto de restauración (una copia de seguridad, como máximo una cada 30 minutos para los cambios de plugins), descarga el archivo en `mods/` o `plugins/` (véase arriba) y lo indica en la consola. Una instalación rechazada no deja ningún punto de restauración. El cliente web envía los cargadores que funcionan en la plataforma del servidor y su versión del juego, leídos de `GET /map`. `204`. `404 ErrPluginNoCompatibleVersion` cuando se enviaron filtros y ninguna versión los cumple; `404` cuando no se encuentran el proyecto, sus versiones o sus archivos; `502 ErrPluginBadFileName` cuando el archivo que indica Modrinth no tiene un nombre `.jar` simple; `409 ErrDiskLimitExceeded`.

### `DELETE /api/servers/:serverId/plugins?name=EssentialsX.jar`

Quita `<name>` de `mods/` o `plugins/` (véase arriba; se añade `.jar` si falta). El punto de restauración se guarda una vez encontrado el archivo, así que un nombre equivocado no deja ninguno. `204`; `400` sin `name`; `404` si el archivo no existe.

## Modpacks

| Ruta | Notas |
|------|-------|
| `GET /modpacks` | `{"installed": {...}, "installing": false, "step": "", "current": 0, "total": 0, "lastError": "", "curseforge": true, "platform": {"loader": "fabric", "gameVersion": "1.21.1"}, "memory": 4096}`; `curseforge` indica si el nodo tiene una clave de API de CurseForge |
| `GET /modpacks/search?source=modrinth&q=&offset=0` | `source` es `modrinth` o `curseforge`; con `q` vacío se listan los packs más populares |
| `GET /modpacks/versions?source=modrinth&project=<id>` | Las versiones instalables del pack, de la más reciente a la más antigua |
| `POST /modpacks/install` | Cuerpo `{"source": "modrinth", "projectId": "...", "versionId": "...", "freshWorld": false, "label": "Pack name"}`; responde `202` y se ejecuta en segundo plano |

La instalación detiene el servidor, guarda un punto de restauración, sustituye los mods y la configuración, instala el loader, ajusta la versión de Java y del juego y vuelve a arrancar el servidor si estaba en marcha. Errores: `400` por una petición incorrecta, por falta de la clave de CurseForge o por un pack no compatible; `409` mientras hay otra instalación o un cambio de versión en curso, o el servidor está ocupado; `422` cuando Modrinth o CurseForge lo rechazan.

## Mapa del mundo

BlueMap o Dynmap, renderizado por el servidor de juego y mostrado en el panel mediante un iframe aislado. Detalles: [Mapa del mundo](../../features/world-map.md).

### `GET /api/servers/:serverId/map`

```json
{
  "supported": true, "reason": "", "platform": "paper", "gameVersion": "1.21.1",
  "installed": "bluemap", "version": "5.4", "installing": false, "running": true, "ready": true,
  "viewUrl": "/api/map-view/3F9A1C2B7D4E/<token>/"
}
```

`reason` explica por qué un servidor no es compatible (`vanilla` o `unknown`); `ready` indica si responde el servidor web del mapa. `viewUrl` solo está presente cuando hay un mapa instalado.

### `POST /api/servers/:serverId/map/install`

`{"kind": "bluemap", "acceptDownload": true}` (`kind` es `bluemap` o `dynmap`). Descarga el plugin o mod desde Modrinth tras guardar un punto de restauración y reinicia el servidor si está en marcha. `202`. `400` para un tipo desconocido, una descarga no aceptada o un servidor no compatible; `409` en los demás casos (ya se está instalando, servidor ocupado...).

### `DELETE /api/servers/:serverId/map`

Quita el jar del mapa; el mapa renderizado se queda en el disco. `204`; `404` si no hay ningún mapa instalado; `409` durante la instalación.

### `GET /api/map-view/:serverId/:token/*path`

Pública: la autorización es el token de la ruta. Es un HMAC firmado por el panel, válido entre 12 y 13 horas e igual durante toda una hora, y deja de funcionar cuando se reinicia el panel. El panel hace de proxy hacia el servidor web propio del mapa y sirve todas las respuestas con una Content-Security-Policy sandbox, de modo que los scripts del mapa no pueden acceder al origen, las cookies ni el almacenamiento del panel.

## Copias de seguridad

El panel guarda un registro por copia; el nodo guarda el archivo, que acompaña al servidor cuando se [traslada a otro nodo](./03-servers.md#post-apiserversserveridtransfer). Detalles (retención, copias externas, puntos de restauración): [Copias de seguridad](../../features/backups.md).

### `GET /api/servers/:serverId/backup`

```json
[ { "id": 12, "name": "before update", "fileName": "....tar.gz", "fileSize": 104857600, "location": "both", "createdAt": "2026-10-31T09:00:00Z" } ]
```

`location` es `local`, `offsite` o `both`. Antes de responder, el panel pone la lista al día con el nodo: se añaden las copias hechas por tareas programadas y las copias periódicas (`offsite-…`, con el nombre `Periodic backup (S3)`), desaparecen las que eliminó la retención, se quita una copia hecha a mano cuyo archivo no se pudo escribir (el nodo la indica como `failed`) y se completan los tamaños cuando el archivo ya está escrito (en la medida de lo posible; si no se puede contactar con el nodo, se muestra la lista guardada).

`GET /backup/:backupID` responde un único registro.

### `POST /api/servers/:serverId/backup/create?name=before+update`

Inicia una copia de seguridad; el archivo se escribe en segundo plano y la consola avisa cuando termina. La copia se queda solo en el nodo: las copias manuales no se suben al bucket. `204`. El registro se crea en el acto; si después falla el archivo, el siguiente listado lo quita.

| Error | Cuándo |
|-------|--------|
| `400 ErrFieldRequired` | Falta `name` |
| `409 ErrBackupLimit` | El servidor ya tiene `panel.backups.maxPerServer` copias en el nodo (por defecto 10; `0` desactiva el límite). Las que solo están en el bucket no cuentan (las periódicas una vez subidas y las de tareas que la retención ya borró del nodo); una periódica que aún está en el nodo sí. Los administradores no tienen límite |
| El error del nodo | Por ejemplo, otra copia o restauración en curso |

Las copias guardan la carpeta del servidor: los archivos se abren por debajo de ella, los enlaces simbólicos se guardan como enlaces y nunca se siguen, y las tuberías (pipes) se omiten.

### `POST /api/servers/:serverId/backup/restore/:backupID`

Borra los archivos del servidor y extrae la copia en su lugar. El servidor debe estar detenido (`400 ErrBackupServerRunning` del nodo). `204`; `404` para una copia desconocida.

### `GET /api/servers/:serverId/backup/download/:backupID`

Transmite el archivo (`application/octet-stream`, `Content-Disposition: attachment`). `404` para una copia desconocida.

### `DELETE /api/servers/:serverId/backup/:backupID`

Elimina el archivo en el nodo (y en el bucket, si está allí) y el registro. `204`; `404` para una copia desconocida; el error del nodo cuando este lo rechaza.

### `GET /api/servers/:serverId/offsite`

La [copia periódica](../../features/backups.md#copia-periódica) del servidor, tal como la ve su nodo:

```json
{ "periodicHours": null, "periodicKeep": 6, "effectiveHours": 24, "effectiveKeep": 6, "configured": true, "lastPeriodic": "2026-11-05T04:12:00Z" }
```

`periodicHours` y `periodicKeep` son el ajuste propio del servidor (`null` = el valor por defecto del nodo); `effectiveHours` y `effectiveKeep`, lo que usa al final (`0` horas = sin copia periódica); `configured`, si el nodo tiene el S3 configurado; `lastPeriodic`, la fecha de la última copia periódica que conoce el nodo (`null` si ninguna). Un nodo de una versión anterior responde `404`.

### `PUT /api/servers/:serverId/offsite`

```json
{ "periodicHours": 48, "periodicKeep": null }
```

Guarda el ajuste propio del servidor; `null` vuelve al valor por defecto y `periodicHours: 0` desactiva la copia periódica de ese servidor. `204`.

| Error | Cuándo |
|-------|--------|
| `400 ErrOffsitePeriodicHours` | `periodicHours` no es `0`, `12`, `24`, `48`, `72`, `168`, `336` ni `720` |
| `400 ErrOffsiteKeep` | `periodicKeep` no está entre 1 y 50 |

## Tareas

Las tareas programadas se ejecutan en el nodo, aunque nadie tenga la sesión iniciada. Detalles: [Tareas programadas](../../features/scheduled-tasks.md).

Una tarea:

```json
{
  "name": "Nightly restart",
  "description": "Warn, wait, restart",
  "cronSchedule": "0 4 * * *",
  "operations": [
    { "type": "stdin", "command": "say Restarting in 1 minute" },
    { "type": "sleep", "duration": "1m" },
    { "type": "restart" }
  ]
}
```

`GET /tasks` responde `{"tasks": {"<taskId>": { ...tarea... }}}`; `GET /tasks/:taskId`, una tarea (`404` si es desconocida).

### `PUT /api/servers/:serverId/tasks/:taskId`

Crea la tarea o sustituye la que tiene ese id. `204`. Reglas para todos (las comprueba el nodo; en otro caso, `400`): el id tiene de 1 a 64 letras, dígitos, `-` o `_`; `name` es obligatorio; `cronSchedule` debe poder interpretarse (vacío significa que la tarea solo se ejecuta a mano); como máximo 50 tareas por servidor.

Quien llama sin `admin` tiene reglas más estrictas (`400` con el motivo; `413` para un cuerpo de más de 64 KB):

| Regla | Límite |
|-------|--------|
| Pasos | De 1 a 10, sin condiciones `if` |
| `stdin` | `command`, una línea de 512 caracteres como máximo; necesita `server.console.send` |
| `sleep` | `duration` de hasta una hora |
| `start`, `stop`, `restart` | Necesitan `server.start`, `server.stop` o ambos |
| `backup` | `keep` opcional (de 1 a 50, por defecto 5), `offsite` opcional (`never`, `always`, `weekend`, `weekly`, `biweekly` o `monthly`; si falta, `always`) y `offsiteKeep` opcional (de 1 a 50, por defecto 4); necesita `server.backup.create` |
| Programación | Cinco campos (sin segundos) o `@yearly`, `@annually`, `@monthly`, `@weekly`, `@daily`, `@midnight`, `@hourly`; con un prefijo opcional `CRON_TZ=` o `TZ=` |
| `name`, `description` | Como máximo 64 y 256 caracteres |

Los administradores pueden usar cualquier operación (comandos, descargas, escritura de archivos...).

`DELETE /tasks/:taskId` responde `204` o `404`; `POST /tasks/:taskId/run` la ejecuta en ese momento, `204` o `404`.

## Bases de datos

Bases de datos MySQL en los hosts que ha registrado un administrador (véase [Hosts de bases de datos](./06-panel-admin.md#hosts-de-bases-de-datos)).

### `GET /api/servers/:serverId/databases`

```json
[
  {
    "id": 4, "server_id": "3F9A1C2B7D4E", "database_host_id": 1, "database_name": "s_3F9A1C2B7D4E_main",
    "username": "db_k3x9q2m7ab", "password": "<generated>", "remote_connection": "db1.example.com:3306", "max_connections": 0,
    "host": "db1.example.com", "port": 3306, "host_name": "Main MySQL",
    "created_at": "2026-10-31T09:00:00Z", "updated_at": "2026-10-31T09:00:00Z"
  }
]
```

La lista solo necesita `server.view`, pero `password` solo se incluye cuando quien llama tiene `server.data.edit` en el servidor (el scope que crea y elimina bases de datos); en otro caso el campo se omite y el cliente web indica que la contraseña solo la ven los usuarios que pueden gestionar bases de datos. En una sesión de un cliente OAuth2 cuentan también los límites del cliente.

### `GET /api/servers/:serverId/databasehosts`

Los hosts que puede usar este servidor: los que sirven a su nodo (o a todos los nodos) y aún tienen hueco por debajo de su `max_databases`. Solo `[{"id": 1, "name": "Main MySQL"}]`, nunca direcciones ni credenciales.

### `POST /api/servers/:serverId/databases`

`{"database_host_id": 1, "database_name": "main"}`. La base de datos se llama `s_<serverId>_<name>`; el usuario (`db_` seguido de 10 caracteres aleatorios) y su contraseña de 16 caracteres se generan automáticamente. Responde el objeto de la base de datos. Si falla un paso posterior a `CREATE DATABASE`, la base de datos y el usuario se vuelven a eliminar para poder reintentar con el mismo nombre.

| Error | Cuándo |
|-------|--------|
| `400 ErrInvalidDatabaseName` | El nombre usa algo distinto de letras, dígitos, `_` y `-`, o `s_<serverId>_<name>` supera los 64 caracteres |
| `400 ErrDatabaseNameTaken` | El nombre ya existe en ese host |
| `400 ErrDatabaseHostFull` | El host alcanzó `max_databases` |
| `400 ErrDatabaseHostOtherNode` | El host sirve a otro nodo |

### `DELETE /api/servers/:serverId/databases/:id`

Elimina la base de datos y su usuario. `204`; `403` cuando la base de datos pertenece a otro servidor. Al eliminar un servidor se eliminan todas sus bases de datos.
