# Copias de seguridad

Aether Panel guarda copias completas de los archivos de un servidor para poder descargarlas o recuperarlas tras un error, un plugin defectuoso o la pérdida de un disco. Existen tres tipos de copia: las **copias de seguridad** creadas a mano, las **copias automáticas** creadas por una tarea programada y los **puntos de restauración** que el propio panel toma justo antes de un cambio arriesgado. Opcionalmente, cada copia terminada se sube también a un bucket compatible con S3 (**copias externas**).

## Qué hace

- **Crear** una copia completa de la carpeta del servidor en un archivo `.tar.gz`.
- **Listar** todas las copias con su tamaño, tipo (manual, automática, punto de restauración) y ubicación (en el nodo, en el almacenamiento externo o en ambos).
- **Descargar** una copia al equipo local.
- **Restaurar** una copia: se borran todos los archivos actuales del servidor y se extrae el archivo en su lugar.
- **Eliminar** una copia (del nodo y, si las copias externas están activas, también del bucket).
- **Copias automáticas** mediante una tarea programada, con su propia retención (conservar las *N* más recientes).
- **Puntos de restauración** antes de instalar o quitar plugins, cambiar la versión del juego, instalar modpacks, instalar un visor de mapa y restaurar copias, para poder deshacer ese paso.
- **Copias externas** en AWS S3, Backblaze B2, Cloudflare R2, Wasabi, MinIO o cualquier servicio compatible con S3.

## Cómo funciona

### Dónde se guardan

Las copias las crea y almacena el daemon del nodo que aloja el servidor, fuera de la carpeta del servidor:

```
<daemon.data.root>/<daemon.data.backups.folder>/<id-del-servidor>/
```

En la instalación con Docker (`daemon.data.root` = `/var/lib/SkyPanel`, carpeta `backups`) la ruta es `/var/lib/SkyPanel/backups/<id-del-servidor>/` dentro del contenedor, que `docker-compose.yml` monta en `./storage/skypanel-data/backups/<id-del-servidor>/` del host.

Las copias **no cuentan para el límite de disco del servidor**. Por eso se limita el número de copias que pueden conservar los usuarios de un servidor (véase [Requisitos y configuración](#requisitos-y-configuración)).

La base de datos del panel guarda un registro por copia (nombre, nombre de archivo, tamaño, ubicación y fecha). El archivo en sí solo existe en el nodo (y en el bucket).

### Nombres de archivo

| Tipo | Nombre de archivo | Nombre mostrado en la lista |
|------|-------------------|-----------------------------|
| Copia manual | `<uuid>.tar.gz` | El nombre que escribió el usuario |
| Copia automática (tarea programada) | `auto-<uuid>.tar.gz` | Distintivo «Automática» |
| Punto de restauración | `restore-<motivo>-<uuid>.tar.gz` (`plugin`, `version`, `modpack`, `restore`) | Distintivo «Punto de restauración» y el motivo |

La retención de las copias automáticas solo borra archivos `auto-`, y la limpieza de puntos de restauración solo borra archivos `restore-`: una copia creada a mano nunca se elimina automáticamente.

### Creación

1. El panel comprueba el límite por servidor (`panel.backups.maxPerServer`), salvo que quien llama sea administrador (permiso `admin`).
2. El nodo rechaza la petición si ya hay otra copia o restauración de ese servidor en curso (`backup in progress`): solo se ejecuta una a la vez por servidor.
3. El nodo responde de inmediato con el nombre del archivo y escribe el archivo comprimido en segundo plano. El panel guarda el registro en ese momento.
4. El archivo se escribe con un nombre oculto (`.partial-<nombre>`) y se renombra al terminar, de modo que una copia a medio escribir nunca se lista, descarga ni restaura. Los archivos se recorren y abren por debajo de la raíz del servidor, así que un enlace simbólico colocado durante la copia no puede introducir archivos del host. Todas las entradas se guardan bajo una carpeta con el ID del servidor.
5. La consola muestra `Backing up server` y después `Backup complete` o `Backup failed`. Se envían el aviso global de copias de Discord ([Discord](../general/16-discord.md)) y la notificación de copias del servidor. Si el archivo de una copia manual no se pudo escribir, el nodo la marca como fallida (un archivo oculto `.failed-<nombre>`) y el panel borra su registro la próxima vez que se abre la lista; así no queda una entrada sin archivo detrás.
6. Si las copias externas están activas, el archivo terminado se sube después (consola: `Uploading backup to off-site storage` y luego `Backup uploaded to off-site storage` u `Off-site upload failed: …`). Un fallo en la subida no invalida la copia, que existe en el nodo.

**No** es necesario detener el servidor para crear una copia: el archivo refleja los ficheros tal como están en disco en ese momento. Un juego que escribe el mundo de forma continua puede dejar esos ficheros a medio cambiar; para una copia totalmente coherente conviene detener el servidor o hacer que el juego guarde antes.

### Restauración

La restauración es destructiva y se ejecuta en segundo plano:

1. El servidor debe estar **detenido** y no estar instalándose, y no puede haber otra copia o restauración en curso.
2. Si la copia local ya no existe pero el bucket la conserva, primero se descarga (consola: `Downloading backup from off-site storage`). Si la descarga falla, el servidor queda intacto.
3. Se lee y comprueba el archivo completo **antes de borrar nada**. Un archivo ilegible, o con una entrada que la extracción rechazaría, detiene la restauración con `Backup file cannot be read: …`.
4. Se escribe un punto de restauración (`restore-restore-…`) con los archivos actuales. Si no se puede escribir, la restauración no continúa.
5. Se borran todos los archivos del servidor y se extrae la copia.
6. La consola muestra `Restoring server` y después `Restore complete` o `Restore failed`.

### Puntos de restauración

Los puntos de restauración son copias rápidas que se toman justo antes de un paso arriesgado:

| Motivo | Cuándo | Si no se puede crear el punto |
|--------|--------|-------------------------------|
| `plugin` | Instalar o quitar un plugin, instalar un visor de mapa | El paso continúa sin punto de restauración |
| `version` | Cambiar la versión del juego | El cambio se cancela |
| `modpack` | Instalar un modpack | La instalación se cancela y no se modifica nada |
| `restore` | Restaurar una copia | La restauración se cancela |

- Solo se conservan los **3 más recientes**; los anteriores se eliminan.
- En los plugins no se crea un punto nuevo si ya existe uno de hace menos de **30 minutos**, para que una serie de instalaciones no genere uno por plugin. Una instalación lo crea solo cuando Modrinth tiene una versión compatible con un nombre de archivo válido, justo antes de la descarga; una eliminación, solo cuando encuentra el archivo. Una instalación rechazada o un nombre equivocado no dejan ningún punto de restauración.
- Los puntos de restauración se guardan solo en el nodo; no se suben al almacenamiento externo.
- Aparecen en la lista de copias y se restauran, descargan y eliminan como cualquier otra copia.

### Copias automáticas (tareas programadas)

Una tarea programada puede incluir un paso **Copia de seguridad** con un valor `keep` (de 1 a 50, 5 por defecto). El paso espera a que el archivo termine, para que el siguiente paso de la tarea lo encuentre completo, y después borra las copias `auto-` más antiguas que excedan `keep`. La plantilla **Copia de seguridad diaria** crea una copia cada día a las 04:00 y guarda las 7 últimas. Crear una tarea con un paso de copia requiere el permiso `server.backup.create`. Las tareas se describen en [Tareas programadas](./scheduled-tasks.md).

Cada vez que se abre la lista de copias, el panel compara sus registros con los archivos del nodo: añade las copias automáticas y los puntos de restauración creados por el nodo, quita los que ha eliminado la retención y las copias manuales que el nodo marca como fallidas (y le pide que borre la marca), y actualiza tamaños y ubicaciones.

### Al borrar o transferir un servidor

- **Borrar un servidor** borra también sus copias de seguridad en el nodo (la carpeta `<daemon.data.backups.folder>/<id-servidor>/`) y sus registros en el panel. Las copias externas del bucket **no** se borran.
- **Transferir un servidor a otro nodo** ([Transferencias](./server-transfers.md)) copia a través del panel al nodo nuevo las copias guardadas en el nodo antiguo (ubicación `local` o `both`, incluidos los puntos de restauración), con su nombre y su fecha, y después las borra del nodo antiguo junto con el servidor. Los registros siguen al servidor. Las copias externas se quedan donde están: el bucket es el mismo para todos los nodos. Si la transferencia falla, se quitan las copias que hizo en el nodo nuevo y el nodo antiguo conserva las suyas.

### Copias externas

Cuando las copias externas están activas y completas (endpoint, bucket, access key y secret key rellenados):

- Cada copia terminada (manual o automática) se sube a `<prefijo>/<id-del-servidor>/<archivo>` en el bucket. La copia local se mantiene: el bucket es una segunda copia.
- La lista combina ambos sitios: `local`, `offsite` (solo en el bucket) o `both` (en ambos).
- **Eliminar** una copia la borra del nodo y del bucket.
- **Descargar** una copia que solo existe en el bucket la transmite directamente desde él.
- **Restaurar** una copia que solo existe en el bucket la descarga primero al nodo.
- Si el bucket no se puede listar (clave errónea, red), se muestra la lista local sin cambios y el error queda en el registro.

Tiempos máximos: 2 horas por subida o descarga, 15 segundos para listar y 1 minuto para borrar.

## Uso

### Desde el panel

Pestaña **Servidor › Copias de seguridad**:

- **Crear copia de seguridad**: escribir un **Nombre** y confirmar. La copia aparece en la lista al instante; su tamaño se rellena cuando el archivo termina de escribirse (recargar la lista).
- Cada fila muestra nombre, fecha, tamaño y distintivos: **Automática**, **Punto de restauración** (con el motivo, p. ej. «Antes de un cambio de plugins»), **Externa** (también en el bucket) o **Solo externa** (la copia local ya no existe).
- **Restaurar** pide confirmación («Se sobrescribirán todos los archivos actuales»). Antes hay que detener el servidor.
- **Descargar** abre la descarga del archivo en una pestaña nueva.
- **Eliminar** pide confirmación.
- La cabecera muestra **Espacio usado por copias**, la copia de mayor tamaño y cuántas están también en externo.

Las copias automáticas se configuran en la pestaña **Tareas** (paso **Copia de seguridad**, plantilla **Copia de seguridad diaria**).

El almacenamiento externo lo configura un administrador en **Configuración › Copias externas**:

1. Opcionalmente, pulsar un proveedor (Backblaze B2, Cloudflare R2, AWS S3, Wasabi) para rellenar endpoint y región.
2. Rellenar **Endpoint**, **Nombre del bucket**, **Región** (algunos proveedores la exigen; en R2, `auto`) y **Carpeta dentro del bucket** (por defecto `aether-backups`).
3. Rellenar **Access key** y **Secret key** de una clave con permiso para leer, escribir y borrar en ese bucket.
4. Activar **Conexión sin cifrar (http)** solo para un MinIO en la red propia.
5. Pulsar **Probar conexión**: el panel escribe y borra un pequeño objeto en el bucket con los valores que hay en pantalla, estén guardados o no.
6. Activar **Subir las copias al bucket** y guardar.

### Mediante la API

| Método | Ruta | Permiso | Uso |
|--------|------|---------|-----|
| `GET` | `/api/servers/:serverId/backup` | `server.backup.view` | Listar copias (antes sincroniza copias automáticas y puntos de restauración) |
| `GET` | `/api/servers/:serverId/backup/:backupID` | `server.backup.view` | Obtener el registro de una copia |
| `POST` | `/api/servers/:serverId/backup/create?name=<nombre>` | `server.backup.create` | Iniciar una copia; `204` al empezar |
| `POST` | `/api/servers/:serverId/backup/restore/:backupID` | `server.backup.restore` | Iniciar una restauración; `204` al empezar |
| `GET` | `/api/servers/:serverId/backup/download/:backupID` | `server.backup.view` | Descargar el archivo |
| `DELETE` | `/api/servers/:serverId/backup/:backupID` | `server.backup.delete` | Eliminar la copia (nodo y bucket) |
| `POST` | `/api/settings/test/s3` | `settings.edit` | Probar la configuración S3; los valores del cuerpo sustituyen a los guardados (una secret key `********` usa la guardada) |

Un registro de copia tiene esta forma: `{"id": 12, "name": "antes de actualizar", "fileName": "…tar.gz", "fileSize": 104857600, "location": "both", "createdAt": "…"}`. Los errores del nodo (servidor en marcha, copia en curso…) se devuelven con el código de estado del nodo. Detalles completos: [Referencia de la API — Backups](../reference/api/04-server-content.md#copias-de-seguridad). Permisos: [Referencia de permisos](../general/scopes-reference.md).

## Requisitos y configuración

| Clave | Valor por defecto | Variable de entorno | Descripción |
|-------|-------------------|---------------------|-------------|
| `panel.backups.maxPerServer` | `10` | `SKYPANEL_PANEL_BACKUPS_MAXPERSERVER` | Copias que pueden conservar los usuarios de un servidor; los administradores no tienen límite; `0` = sin límite. No se puede editar desde la página de Configuración |
| `daemon.data.backups.folder` | `backups` | `SKYPANEL_DAEMON_DATA_BACKUPS_FOLDER` | Carpeta de copias; las rutas relativas se resuelven bajo `daemon.data.root` |
| `daemon.backups.s3.enabled` | `false` | `SKYPANEL_DAEMON_BACKUPS_S3_ENABLED` | Subir las copias terminadas al bucket |
| `daemon.backups.s3.endpoint` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_ENDPOINT` | `host`, `host:puerto` o una URL completa (su esquema decide http/https) |
| `daemon.backups.s3.region` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_REGION` | Región, si el proveedor la exige |
| `daemon.backups.s3.bucket` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_BUCKET` | Nombre del bucket (debe existir) |
| `daemon.backups.s3.accessKey` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_ACCESSKEY` | Access key |
| `daemon.backups.s3.secretKey` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_SECRETKEY` | Secret key |
| `daemon.backups.s3.prefix` | `aether-backups` | `SKYPANEL_DAEMON_BACKUPS_S3_PREFIX` | Carpeta dentro del bucket |
| `daemon.backups.s3.insecure` | `false` | `SKYPANEL_DAEMON_BACKUPS_S3_INSECURE` | http sin cifrar (MinIO en red local) |

Las copias externas solo están activas cuando `enabled` vale `true` **y** están definidos endpoint, bucket, access key y secret key. La lista completa y la correspondencia entre variables de entorno y claves están en [Configuración](../backend/02-configuration.md).

**Nodos remotos:** las subidas, listados y borrados los ejecuta el daemon del nodo que aloja el servidor, que lee las claves `daemon.backups.s3.*` de su propia configuración. La página **Configuración › Copias externas** escribe el `config.json` del panel, por lo que se aplica a los servidores del nodo local del panel. En un nodo remoto hay que definir las mismas claves en su `config.json` o en sus variables de entorno. El botón **Probar conexión** prueba desde el host del panel.

## Límites

- Una copia o restauración a la vez por servidor.
- El límite cuenta todos los registros de copia del servidor: copias manuales, automáticas y puntos de restauración.
- Los nombres de copia deben ser ASCII imprimible (el registro se valida con `printascii`); se rechazan letras acentuadas, la «ñ» o emojis.
- La restauración siempre sustituye **todos** los archivos del servidor; no hay restauración parcial. Para recuperar un solo archivo, descargar la copia y subir ese archivo con el gestor de archivos o por SFTP.
- Puntos de restauración: se conservan 3, solo en el nodo.
- Retención de copias automáticas: `keep` entre 1 y 50.
- Clonar un servidor no copia sus copias de seguridad ([Clonar servidores](./cloning.md)).
- Al eliminar un servidor, sus copias del bucket externo se quedan allí; hay que borrarlas a mano si ya no se necesitan.
- Una transferencia entre nodos copia todas las copias de seguridad a través del panel, así que tarda más y necesita espacio para ellas en el nodo nuevo. Ambos nodos deben tener una versión actual: el nodo nuevo necesita la ruta `/daemon/server/:id/backup/file` para recibirlas, y los dos necesitan las rutas para pausar las tareas que usa la transferencia (véase [Transferencias de servidores](./server-transfers.md#requisitos-y-configuración)).
- La configuración S3 guardada en **Configuración › Copias externas** solo se aplica al nodo propio del panel; cada nodo remoto necesita la suya (véase [Nodos remotos](#requisitos-y-configuración)).

## Problemas conocidos y soluciones

| Mensaje / síntoma | Causa | Solución |
|-------------------|-------|----------|
| «Este servidor ya tiene todas las copias de seguridad que puede guardar. Borra una antigua antes de crear otra.» (`409`, `ErrBackupLimit`) | Se alcanzó `panel.backups.maxPerServer` | Borrar copias antiguas (incluidas automáticas o puntos de restauración) o subir el límite en la configuración |
| `backup in progress` (`ErrBackupInProgress`) | Hay otra copia, restauración o punto de restauración del servidor en curso | Esperar a `Backup complete` / `Restore complete` en la consola |
| `cannot backup server, is running` (`ErrBackupServerRunning`) o `server running` al restaurar | El servidor está en marcha o instalándose | Detener el servidor y repetir la restauración |
| `name is required` al crear | Nombre vacío | Dar un nombre a la copia |
| Una copia manual desaparece de la lista poco después de crearla | El archivo no se pudo escribir (la consola mostró `Failed to create backup file` / `Backup failed`); el panel quita el registro al sincronizar | Revisar el espacio libre del nodo y crear la copia de nuevo |
| `Backup file cannot be read: …` al restaurar | Archivo dañado o incompleto | No se ha borrado nada; elegir otra copia |
| `Could not create a restore point: …` | Falta espacio en el disco de copias del nodo o error de escritura | Liberar espacio en el nodo; sin punto de restauración no se ejecutan restauraciones ni cambios de versión |
| «No se pudieron cargar las copias» en la pestaña | El nodo no respondió | Comprobar que el nodo está en línea ([Solución de problemas](../setup/troubleshooting.md)) |
| Prueba: `off-site backups are not configured` (`400`) | Falta endpoint, bucket, access key o secret key | Rellenar los cuatro |
| Prueba: `the bucket "…" does not exist` | Nombre de bucket o región incorrectos | Crear el bucket o corregir el nombre |
| Prueba: `could not write to the bucket: …` / `could write but not delete in the bucket: …` | La clave no tiene permiso de escritura o de borrado | Dar a la clave permisos de lectura, escritura y borrado en el bucket |
| Consola: `Off-site upload failed: …` | Error de red, credenciales erróneas o el nodo no tiene configuración S3 (nodo remoto) | La copia local está bien; corregir la configuración del nodo y crear una copia nueva |

## Véase también

- [Tareas programadas](./scheduled-tasks.md) — copias automáticas
- [Historial de archivos](./file-history.md) — deshacer ediciones de archivos de configuración sin restaurar todo
- [Notificaciones del servidor](./server-notifications.md) — avisos de copias por servidor
- [Archivos y compresión](../general/19-files-compression.md)
- [Discord](../general/16-discord.md) — avisos globales de copias
- [Configuración](../backend/02-configuration.md)
- [Referencia de la API](../reference/api/04-server-content.md#copias-de-seguridad)
