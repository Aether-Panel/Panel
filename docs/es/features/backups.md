# Copias de seguridad

Aether Panel guarda copias completas de los archivos de un servidor para poder descargarlas o recuperarlas tras un error, un plugin defectuoso o la pérdida de un disco. Existen cuatro tipos de copia: las **copias de seguridad** creadas a mano, las **copias automáticas** creadas por una tarea programada, las **copias periódicas** que el nodo hace de cada servidor directamente en el bucket y los **puntos de restauración** que el propio panel toma justo antes de un cambio arriesgado. Opcionalmente, un bucket compatible con S3 guarda copias fuera del nodo (**copias externas**): la copia periódica de cada servidor y las copias de las tareas que lo piden. Las copias hechas a mano se quedan solo en el nodo.

## Qué hace

- **Crear** una copia completa de la carpeta del servidor en un archivo `.tar.gz`.
- **Listar** todas las copias con su tamaño, tipo (manual, automática, periódica, punto de restauración) y ubicación (en el nodo, en el almacenamiento externo o en ambos).
- **Descargar** una copia al equipo local.
- **Restaurar** una copia: se borran todos los archivos actuales del servidor y se extrae el archivo en su lugar.
- **Eliminar** una copia (del nodo y, si las copias externas están activas, también del bucket).
- **Copias automáticas** mediante una tarea programada, con su propia retención (conservar las *N* más recientes) y una frecuencia de subida al bucket (nunca, siempre, cada fin de semana, cada semana, cada 15 días o cada mes).
- **Puntos de restauración** antes de instalar o quitar plugins, cambiar la versión del juego, instalar modpacks, instalar un visor de mapa y restaurar copias, para poder deshacer ese paso.
- **Copias externas** en AWS S3, Backblaze B2, Cloudflare R2, Wasabi, MinIO o cualquier servicio compatible con S3, con una **copia periódica** de cada servidor (cada 12 h a 30 días) aunque no tenga tareas, y una retención propia en el bucket.

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

| Tipo | Nombre de archivo | Nombre mostrado en la lista | ¿Va al bucket? |
|------|-------------------|-----------------------------|----------------|
| Copia manual | `<uuid>.tar.gz` | El nombre que escribió el usuario | No |
| Copia automática (tarea programada) | `auto-<id-de-la-tarea>-<uuid>.tar.gz` | «Copia automática», con el distintivo «Automática» | Según la frecuencia de la tarea ([Subida de las copias de tareas](#subida-de-las-copias-de-tareas)) |
| Copia periódica | `offsite-<uuid>.tar.gz` | «Copia periódica (S3)», con el distintivo «Periódica» | Siempre, y solo allí ([Copia periódica](#copia-periódica)) |
| Punto de restauración | `restore-<motivo>-<uuid>.tar.gz` (`plugin`, `version`, `modpack`, `restore`) | Distintivo «Punto de restauración» y el motivo | No |

Las copias de tareas hechas con versiones anteriores se llaman `auto-<uuid>.tar.gz`, sin el id de la tarea. Siguen siendo copias automáticas (se listan y se podan en el nodo como las demás), pero ninguna tarea las reconoce como suyas en el bucket, así que la retención del bucket no las borra: se quedan allí hasta que alguien las borre.

La retención de las copias automáticas solo borra archivos `auto-` del nodo, la de cada serie del bucket solo borra sus propias copias y la limpieza de puntos de restauración solo borra archivos `restore-`: una copia creada a mano nunca se elimina automáticamente.

### Creación

1. El panel comprueba el límite por servidor (`panel.backups.maxPerServer`), salvo que quien llama sea administrador (permiso `admin`). Las copias periódicas no cuentan para ese límite.
2. El nodo rechaza la petición si ya hay otra copia o restauración de ese servidor en curso (`backup in progress`): solo se ejecuta una a la vez por servidor.
3. El nodo responde de inmediato con el nombre del archivo y escribe el archivo comprimido en segundo plano. El panel guarda el registro en ese momento.
4. El archivo se escribe con un nombre oculto (`.partial-<nombre>`) y se renombra al terminar, de modo que una copia a medio escribir nunca se lista, descarga ni restaura. Los archivos se recorren y abren por debajo de la raíz del servidor, así que un enlace simbólico colocado durante la copia no puede introducir archivos del host. Todas las entradas se guardan bajo una carpeta con el ID del servidor.
5. La consola muestra `Backing up server` y después `Backup complete` o `Backup failed`. Se envían el aviso global de copias de Discord ([Discord](../general/16-discord.md)) y la notificación de copias del servidor. Si el archivo de una copia manual no se pudo escribir, el nodo la marca como fallida (un archivo oculto `.failed-<nombre>`) y el panel borra su registro la próxima vez que se abre la lista; así no queda una entrada sin archivo detrás.
6. Una copia manual se queda solo en el nodo: no se sube al bucket aunque las copias externas estén activas. Las versiones anteriores sí subían las copias manuales; las que ya están en el bucket se quedan allí y se listan, descargan, restauran y borran como siempre. Para tener copias fuera del nodo están la [copia periódica](#copia-periódica) y la [subida de las copias de tareas](#subida-de-las-copias-de-tareas).

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

Una tarea programada puede incluir un paso **Copia de seguridad** con estos valores:

| Valor | Campo en el panel | Rango | Por defecto | Para qué |
|-------|-------------------|-------|-------------|----------|
| `keep` | **Guardar las últimas** | 1-50 | 5 | Copias automáticas que se quedan en el nodo |
| `offsite` | **Subir al S3** | `never`, `always`, `weekend`, `weekly`, `biweekly`, `monthly` | `weekly` en un paso nuevo; `always` en uno guardado antes de que existiera | Qué ejecuciones suben además su copia al bucket ([Subida de las copias de tareas](#subida-de-las-copias-de-tareas)) |
| `offsiteKeep` | **Conservar en S3** (oculto con **Nunca**) | 1-50 | 4 | Copias de esa tarea que se quedan en el bucket |

El paso espera a que el archivo termine, para que el siguiente paso de la tarea lo encuentre completo, y después borra del nodo las copias `auto-` más antiguas que excedan `keep` (todas las copias de tareas del servidor cuentan juntas). Esa poda solo borra el archivo del nodo: las copias que la tarea subió al bucket se quedan allí, con su propia retención. La plantilla **Copia de seguridad diaria** crea una copia cada día a las 04:00, guarda las 7 últimas en el nodo y sube una cada semana (conserva 4 en el bucket). Crear una tarea con un paso de copia requiere el permiso `server.backup.create`. Las tareas se describen en [Tareas programadas](./scheduled-tasks.md).

Cada vez que se abre la lista de copias, el panel compara sus registros con los archivos del nodo y del bucket: añade las copias automáticas, las periódicas y los puntos de restauración creados por el nodo, quita los que ha eliminado la retención y las copias manuales que el nodo marca como fallidas (y le pide que borre la marca), y actualiza tamaños y ubicaciones. Una copia de tarea que la retención borró del nodo pero sigue en el bucket se queda en la lista como **Solo externa**.

### Al borrar o transferir un servidor

- **Borrar un servidor** borra también sus copias de seguridad en el nodo (la carpeta `<daemon.data.backups.folder>/<id-servidor>/`), su ajuste de copia periódica (`<id>.offsite`) y sus registros en el panel. Las copias externas del bucket **no** se borran, tampoco las periódicas.
- **Borrar una tarea** no borra sus copias: las del nodo siguen hasta que la retención de otra tarea o una persona las borre, y las del bucket se quedan allí y siguen en la lista (distintivo **Externa** o **Solo externa**), donde se pueden borrar a mano.
- **Transferir un servidor a otro nodo** ([Transferencias](./server-transfers.md)) copia a través del panel al nodo nuevo las copias guardadas en el nodo antiguo (ubicación `local` o `both`, incluidos los puntos de restauración), con su nombre y su fecha, y después las borra del nodo antiguo junto con el servidor. Los registros siguen al servidor. Las copias externas se quedan donde están: el bucket es el mismo para todos los nodos. Si la transferencia falla, se quitan las copias que hizo en el nodo nuevo y el nodo antiguo conserva las suyas.
- La transferencia también copia el ajuste de copia periódica propio del servidor (`GET` y `PUT /daemon/server/:id/offsite`), igual que copia sus tareas. Si el servidor sigue el valor por defecto, no hay nada que copiar. Un nodo antiguo que no conoce la ruta (responde `404`) se trata como «sin ajuste propio»; cualquier otro fallo al leerlo o al guardarlo en el nodo nuevo hace fallar la transferencia, que se deshace. Mientras dura la transferencia, la copia periódica de ese servidor no se hace en ninguno de los dos nodos.

### Copias externas

Cuando las copias externas están activas y completas (endpoint, bucket, access key y secret key rellenados):

- Al bucket van dos series de copias, en `<prefijo>/<id-del-servidor>/<archivo>`: las de las tareas a las que les toca subir (la copia local se mantiene) y la copia periódica de cada servidor (que solo queda en el bucket). Las copias manuales y los puntos de restauración no se suben.
- La lista combina ambos sitios: `local`, `offsite` (solo en el bucket) o `both` (en ambos).
- **Eliminar** una copia la borra del nodo y del bucket.
- **Descargar** una copia que solo existe en el bucket la transmite directamente desde él.
- **Restaurar** una copia que solo existe en el bucket la descarga primero al nodo.
- Si el bucket no se puede listar (clave errónea, red), se muestra la lista local sin cambios y el error queda en el registro.

Tiempos máximos: 2 horas por subida o descarga, 15 segundos para listar en la lista de copias, 1 minuto para listar el bucket antes de decidir una subida o una poda y 1 minuto para borrar.

#### Subida de las copias de tareas

Cada paso **Copia de seguridad** decide con `offsite` (**Subir al S3**) qué ejecuciones suben su copia al bucket. La decisión se toma después de escribir la copia en el nodo, mirando la copia más reciente **de esa tarea** en el bucket (la fecha del objeto, es decir, cuando terminó de subirse) y la hora actual en la zona horaria del planificador del servidor (la de sus tareas; sin zona, la del nodo):

| Valor | Panel | Sube si… |
|-------|-------|----------|
| `never` | Nunca | Nunca. |
| `always` | Siempre | Siempre. |
| `weekend` | Cada fin de semana | Hoy es sábado o domingo **y** la tarea no tiene ninguna copia en el bucket desde el sábado a las 00:00 de este fin de semana. |
| `weekly` | Cada semana | La tarea no tiene ninguna copia en el bucket desde el lunes a las 00:00 de esta semana. |
| `biweekly` | Cada 15 días | La última copia de la tarea en el bucket tiene 15 días menos 2 horas o más (o no hay ninguna). Las 2 horas de margen evitan que una tarea diaria a la misma hora se salte un día porque la subida anterior terminó unos minutos más tarde. |
| `monthly` | Cada mes | La tarea no tiene ninguna copia en el bucket desde el día 1 de este mes a las 00:00. |

- Un paso guardado antes de que existiera esta opción, sin `offsite`, vale **Siempre**, para que ninguna tarea deje de subir sin que nadie lo haya pedido. Un paso nuevo creado desde el panel trae **Cada semana**.
- El resto de ejecuciones guardan la copia solo en el nodo.
- La subida se hace en segundo plano (consola: `Uploading backup to off-site storage` y luego `Backup uploaded to off-site storage` u `Off-site upload failed: …`), así que la tarea sigue con sus pasos sin esperarla. Un fallo en la subida no hace fallar la tarea: la copia ya está en el nodo, y la siguiente ejecución vuelve a decidir.
- Mientras la subida anterior de la misma tarea no ha terminado, la siguiente ejecución no sube (todavía no la vería en el bucket y subiría dos veces); su copia se queda en el nodo.
- Si el bucket no se puede listar, la ejecución no sube nada y la consola muestra `Off-site copy skipped: …`; la tarea no falla.
- Las copias de una tarea se reconocen por el id de la tarea en el nombre (`auto-<id-de-la-tarea>-<uuid>.tar.gz`). El id lo pone siempre el planificador al ejecutar la tarea, nunca los argumentos guardados, para que nadie pueda nombrar otra tarea y podar sus copias.

#### Copia periódica

Cada servidor puede recibir una copia completa que va directa al bucket cada cierto tiempo, tenga tareas o no. La hace el daemon del nodo que aloja el servidor:

- **Intervalo**: `daemon.backups.s3.periodicHours`, uno de `0`, `12`, `24`, `48`, `72`, `168` (7 días), `336` (14 días) o `720` (30 días). `0`, el valor por defecto, la desactiva, así que nada cambia hasta que un administrador elige un intervalo. Las copias a conservar en el bucket son `daemon.backups.s3.periodicKeep` (de 1 a 50, 4 por defecto).
- **Ajuste por servidor**: cada servidor puede usar el valor por defecto, desactivar su copia o elegir otro intervalo y otro número de copias. Se guarda en el nodo, en `<daemon.data.servers>/<id>.offsite` (JSON, permisos `0600`): `{"periodicHours": null, "periodicKeep": null}`, donde `null` significa «por defecto». Sin el archivo, el servidor sigue los valores por defecto. Un archivo editado a mano con un intervalo que no está en la lista desactiva la copia, y un número de copias fuera de 1-50 usa 4.
- **Un único trabajador por nodo**: cada 10 minutos recorre los servidores de uno en uno, para que nunca hagan todos la copia a la vez. El primer recorrido empieza 5 minutos después de arrancar el nodo.
- **Cuándo hace la copia de un servidor**: las copias externas están activas y completas en ese nodo, el intervalo efectivo es mayor que 0 y la última copia periódica de ese servidor en el bucket es más antigua que el intervalo. El intervalo cuenta desde que la copia anterior terminó de subirse, y como el trabajador pasa cada 10 minutos (y espera a cada copia antes de pasar al siguiente servidor), es un mínimo, no una hora exacta.
- **Cuándo la deja para otro recorrido**: el servidor se está instalando o trasladando a otro nodo, o tiene otra copia o restauración en curso. Un servidor de juego encendido no es motivo: la copia se hace en caliente, como una manual, y nunca lo detiene.
- **La copia y su subida**: antes de escribir nada, el nodo comprueba que el bucket responde. Después escribe `offsite-<uuid>.tar.gz` en la carpeta de copias, lo sube, borra del bucket las copias periódicas de ese servidor que pasen de las que se conservan (de la más antigua a la más reciente) y borra el archivo del nodo. El archivo local se borra también si la subida falla, para que los reintentos no llenen el disco. Al arrancar, el nodo borra los archivos `offsite-` que queden en las carpetas de copias (por ejemplo, uno que un reinicio dejó a medio subir): las copias periódicas que cuentan son las del bucket.
- **Avisos**: una copia periódica que va bien no avisa (solo la consola muestra `Backing up server`, `Backup complete` y la subida). Si falla la copia, la subida o el bucket no responde, se avisa como un fallo de copia al propietario del servidor ([Notificaciones del servidor](./server-notifications.md)) y al aviso global de copias de Discord, y la consola muestra el motivo (por ejemplo `Periodic off-site backup skipped, the bucket is not available: …`).
- **Reintentos**: tras un fallo, ese servidor espera 1 h, 2 h, 4 h… hasta un máximo de 24 h antes de volver a intentarlo; una copia que sube bien reinicia la espera. Así, un bucket caído un día entero no fabrica copias que luego se tiran.
- **La última copia** se lee del bucket la primera vez que el trabajador pasa por el servidor después de arrancar el nodo, y después el nodo la recuerda en memoria. La tarjeta **Copia al S3** del servidor muestra esa fecha.
- La lista de copias muestra las periódicas como **Copia periódica (S3)**, con los distintivos **Periódica** y **Solo externa**. Se descargan, restauran y borran como cualquier otra copia, y no cuentan para el límite `panel.backups.maxPerServer`.

#### Retención en el bucket

- Cada tarea conserva en el bucket sus `offsiteKeep` copias más recientes; al terminar bien una subida, se borran las más antiguas **de esa tarea**.
- La copia periódica conserva las `periodicKeep` más recientes de cada servidor; al terminar bien una subida, se borran las periódicas más antiguas de ese servidor.
- Cada serie (cada tarea, y la copia periódica de cada servidor) solo cuenta y borra sus propias copias: nunca las de otra serie, las manuales que se subieron con versiones anteriores ni las `auto-<uuid>` antiguas sin id de tarea.
- La poda de las copias del nodo (`keep`) ya no borra nada del bucket. **Eliminar** una copia desde el panel sí la borra de los dos sitios.

## Uso

### Desde el panel

Pestaña **Servidor › Copias de seguridad**:

- **Crear copia de seguridad**: escribir un **Nombre** y confirmar. La copia aparece en la lista al instante; su tamaño se rellena cuando el archivo termina de escribirse (recargar la lista).
- Cada fila muestra nombre, fecha, tamaño y distintivos: **Automática**, **Periódica** (copia periódica), **Punto de restauración** (con el motivo, p. ej. «Antes de un cambio de plugins»), **Externa** (también en el bucket) o **Solo externa** (la copia local ya no existe).
- **Restaurar** pide confirmación («Se sobrescribirán todos los archivos actuales»). Antes hay que detener el servidor.
- **Descargar** abre la descarga del archivo en una pestaña nueva.
- **Eliminar** pide confirmación.
- La cabecera muestra **Espacio usado por copias**, la copia de mayor tamaño y cuántas están también en externo.

Encima de la lista, la tarjeta **Copia al S3** muestra la copia periódica de ese servidor:

- **Cada cuánto**: **Por defecto (…)** (el valor del panel, entre paréntesis cuando se conoce), **Desactivada**, 12 h, 24 h, 48 h, 72 h, 7 días, 14 días o 30 días.
- **Conservar en S3**: de 1 a 50; vacío o el botón **Por defecto** vuelve al valor del panel.
- **Guardar** aplica los cambios. Cambiarlo requiere `server.backup.create`; con solo `server.backup.view` la tarjeta se puede ver pero no cambiar.
- Debajo del título, la fecha de la **Última copia periódica** o «Todavía no hay ninguna copia periódica.».
- Si el nodo del servidor no tiene el S3 configurado, la tarjeta muestra «Este nodo no tiene S3 configurado.» y desactiva los controles. Si el nodo no responde, muestra un aviso con **Reintentar**. Sin `server.backup.view`, o con un nodo de una versión anterior sin la copia periódica, la tarjeta no aparece.

Las copias automáticas se configuran en la pestaña **Tareas** (paso **Copia de seguridad**, plantilla **Copia de seguridad diaria**): junto a **Guardar las últimas** están **Subir al S3** (Nunca, Siempre, Cada fin de semana, Cada semana, Cada 15 días, Cada mes) y, salvo con **Nunca**, **Conservar en S3**. Con una frecuencia que no es **Nunca** ni **Siempre**, una línea recuerda que las demás ejecuciones guardan la copia solo en el servidor.

El almacenamiento externo lo configura un administrador en **Configuración › Copias externas**:

1. Opcionalmente, pulsar un proveedor (Backblaze B2, Cloudflare R2, AWS S3, Wasabi) para rellenar endpoint y región.
2. Rellenar **Endpoint**, **Nombre del bucket**, **Región** (algunos proveedores la exigen; en R2, `auto`) y **Carpeta dentro del bucket** (por defecto `aether-backups`).
3. Rellenar **Access key** y **Secret key** de una clave con permiso para leer, escribir y borrar en ese bucket.
4. Activar **Conexión sin cifrar (http)** solo para un MinIO en la red propia.
5. Pulsar **Probar conexión**: el panel escribe y borra un pequeño objeto en el bucket con los valores que hay en pantalla, estén guardados o no.
6. Activar **Subir las copias al bucket**.
7. En **Copia periódica de cada servidor**, elegir el **Intervalo** por defecto (**Desactivada**, 12 h, 24 h, 48 h, 72 h, 7 días, 14 días o 30 días) y las **Copias a conservar en S3** (de 1 a 50). Cada servidor puede cambiarlo en su pestaña **Copias de seguridad**.
8. Guardar.

### Mediante la API

| Método | Ruta | Permiso | Uso |
|--------|------|---------|-----|
| `GET` | `/api/servers/:serverId/backup` | `server.backup.view` | Listar copias (antes sincroniza copias automáticas y puntos de restauración) |
| `GET` | `/api/servers/:serverId/backup/:backupID` | `server.backup.view` | Obtener el registro de una copia |
| `POST` | `/api/servers/:serverId/backup/create?name=<nombre>` | `server.backup.create` | Iniciar una copia; `204` al empezar |
| `POST` | `/api/servers/:serverId/backup/restore/:backupID` | `server.backup.restore` | Iniciar una restauración; `204` al empezar |
| `GET` | `/api/servers/:serverId/backup/download/:backupID` | `server.backup.view` | Descargar el archivo |
| `DELETE` | `/api/servers/:serverId/backup/:backupID` | `server.backup.delete` | Eliminar la copia (nodo y bucket) |
| `GET` | `/api/servers/:serverId/offsite` | `server.backup.view` | Leer la copia periódica del servidor (la responde su nodo) |
| `PUT` | `/api/servers/:serverId/offsite` | `server.backup.create` | Cambiar la copia periódica del servidor; `204` |
| `POST` | `/api/settings/test/s3` | `settings.edit` | Probar la configuración S3; los valores del cuerpo sustituyen a los guardados (una secret key `********` usa la guardada) |

`GET /api/servers/:serverId/offsite` responde `{"periodicHours": null, "periodicKeep": 6, "effectiveHours": 24, "effectiveKeep": 6, "configured": true, "lastPeriodic": "2026-11-05T04:12:00Z"}`: el ajuste propio del servidor (`null` = por defecto), los valores que usa al final, si su nodo tiene el S3 configurado y la fecha de la última copia periódica que conoce el nodo (`null` si no conoce ninguna). `PUT` recibe `{"periodicHours": 48, "periodicKeep": null}`; un intervalo que no está en la lista responde `400 ErrOffsitePeriodicHours` y un número de copias fuera de 1-50, `400 ErrOffsiteKeep`. Los valores por defecto se cambian con `POST /api/settings` (claves `daemon.backups.s3.periodicHours` y `daemon.backups.s3.periodicKeep`, con las mismas comprobaciones; véase [Configuración del panel](./panel-settings.md)).

Un registro de copia tiene esta forma: `{"id": 12, "name": "antes de actualizar", "fileName": "…tar.gz", "fileSize": 104857600, "location": "both", "createdAt": "…"}`. Los errores del nodo (servidor en marcha, copia en curso…) se devuelven con el código de estado del nodo. Detalles completos: [Referencia de la API — Backups](../reference/api/04-server-content.md#copias-de-seguridad). Permisos: [Referencia de permisos](../general/scopes-reference.md).

## Requisitos y configuración

| Clave | Valor por defecto | Variable de entorno | Descripción |
|-------|-------------------|---------------------|-------------|
| `panel.backups.maxPerServer` | `10` | `SKYPANEL_PANEL_BACKUPS_MAXPERSERVER` | Copias que pueden conservar los usuarios de un servidor; los administradores no tienen límite; `0` = sin límite. No se puede editar desde la página de Configuración |
| `daemon.data.backups.folder` | `backups` | `SKYPANEL_DAEMON_DATA_BACKUPS_FOLDER` | Carpeta de copias; las rutas relativas se resuelven bajo `daemon.data.root` |
| `daemon.backups.s3.enabled` | `false` | `SKYPANEL_DAEMON_BACKUPS_S3_ENABLED` | Activar el bucket: la copia periódica y las copias de las tareas a las que les toca subir |
| `daemon.backups.s3.endpoint` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_ENDPOINT` | `host`, `host:puerto` o una URL completa (su esquema decide http/https) |
| `daemon.backups.s3.region` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_REGION` | Región, si el proveedor la exige |
| `daemon.backups.s3.bucket` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_BUCKET` | Nombre del bucket (debe existir) |
| `daemon.backups.s3.accessKey` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_ACCESSKEY` | Access key |
| `daemon.backups.s3.secretKey` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_SECRETKEY` | Secret key |
| `daemon.backups.s3.prefix` | `aether-backups` | `SKYPANEL_DAEMON_BACKUPS_S3_PREFIX` | Carpeta dentro del bucket |
| `daemon.backups.s3.insecure` | `false` | `SKYPANEL_DAEMON_BACKUPS_S3_INSECURE` | http sin cifrar (MinIO en red local) |
| `daemon.backups.s3.periodicHours` | `0` | `SKYPANEL_DAEMON_BACKUPS_S3_PERIODICHOURS` | Intervalo por defecto de la copia periódica, en horas: `0` (desactivada), `12`, `24`, `48`, `72`, `168`, `336` o `720` |
| `daemon.backups.s3.periodicKeep` | `4` | `SKYPANEL_DAEMON_BACKUPS_S3_PERIODICKEEP` | Copias periódicas que se conservan en el bucket por servidor, de 1 a 50 |

Las copias externas solo están activas cuando `enabled` vale `true` **y** están definidos endpoint, bucket, access key y secret key. El ajuste propio de cada servidor (`<daemon.data.servers>/<id>.offsite`) tiene prioridad sobre `periodicHours` y `periodicKeep`. La lista completa y la correspondencia entre variables de entorno y claves están en [Configuración](../backend/02-configuration.md).

**Nodos remotos:** las subidas, listados, borrados y la copia periódica los ejecuta el daemon del nodo que aloja el servidor, que lee las claves `daemon.backups.s3.*` (también `periodicHours` y `periodicKeep`) de su propia configuración. La página **Configuración › Copias externas** escribe el `config.json` del panel, por lo que se aplica a los servidores del nodo local del panel. En un nodo remoto hay que definir las mismas claves en su `config.json` o en sus variables de entorno; si no, sus servidores no tienen copia periódica ni suben las copias de sus tareas, y su tarjeta **Copia al S3** dice «Este nodo no tiene S3 configurado.». El ajuste propio de un servidor sí funciona en cualquier nodo, porque se guarda en el nodo que lo aloja, pero el valor «por defecto» que sigue es el de ese nodo. El botón **Probar conexión** prueba desde el host del panel.

## Límites

- Una copia o restauración a la vez por servidor.
- El límite cuenta los registros de copia del servidor: copias manuales, automáticas (también las que solo quedan en el bucket) y puntos de restauración. Las copias periódicas no cuentan: las impone el administrador y, si contaran, un `periodicKeep` alto podría impedir a los usuarios hacer copias a mano.
- Los nombres de copia deben ser ASCII imprimible (el registro se valida con `printascii`); se rechazan letras acentuadas, la «ñ» o emojis.
- La restauración siempre sustituye **todos** los archivos del servidor; no hay restauración parcial. Para recuperar un solo archivo, descargar la copia y subir ese archivo con el gestor de archivos o por SFTP.
- Puntos de restauración: se conservan 3, solo en el nodo.
- Retención de copias automáticas: `keep` entre 1 y 50 en el nodo, `offsiteKeep` entre 1 y 50 en el bucket. Copia periódica: `periodicKeep` entre 1 y 50.
- Las copias manuales no se suben al bucket, y no hay botón para subir una a mano.
- La copia periódica la hace un solo trabajador por nodo, servidor a servidor: en un nodo con muchos servidores grandes, un recorrido puede durar horas y las copias llegan más tarde de lo que marca el intervalo.
- Clonar un servidor no copia sus copias de seguridad ([Clonar servidores](./cloning.md)).
- Al eliminar un servidor o una tarea, sus copias del bucket externo se quedan allí; hay que borrarlas a mano si ya no se necesitan.
- Una transferencia entre nodos copia todas las copias de seguridad a través del panel, así que tarda más y necesita espacio para ellas en el nodo nuevo. Ambos nodos deben tener una versión actual: el nodo nuevo necesita la ruta `/daemon/server/:id/backup/file` para recibirlas, y los dos necesitan las rutas para pausar las tareas que usa la transferencia (véase [Transferencias de servidores](./server-transfers.md#requisitos-y-configuración)). Si el servidor tiene un ajuste propio de copia periódica, el nodo nuevo también necesita la ruta `/daemon/server/:id/offsite`; sin ella la transferencia falla y se deshace.
- La configuración S3 guardada en **Configuración › Copias externas**, incluida la copia periódica por defecto, solo se aplica al nodo propio del panel; cada nodo remoto necesita la suya (véase [Nodos remotos](#requisitos-y-configuración)).

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
| Consola: `Off-site upload failed: …` tras una copia de tarea | Error de red o credenciales erróneas | La copia local está bien; corregir la configuración del nodo. La siguiente ejecución que toque vuelve a subir |
| Consola: `Off-site copy skipped: …` tras una copia de tarea | No se pudo listar el bucket para decidir la subida | La copia local está bien y la tarea no ha fallado; corregir la configuración o esperar a que el bucket responda |
| Una tarea con **Cada semana** (o fin de semana, 15 días, mes) no ha subido su copia | Ya hay una copia de esa tarea en el bucket dentro del periodo, o la subida anterior de la tarea seguía en curso | Es lo esperado: esa ejecución guarda la copia solo en el nodo |
| Una copia manual ya no aparece como **Externa** | Las copias manuales ya no se suben al bucket | Usar la copia periódica o una tarea con **Subir al S3** |
| Consola: `Periodic off-site backup skipped, the bucket is not available: …` y aviso de copia fallida | El bucket no responde o no existe | Corregir la configuración S3 del nodo; el nodo vuelve a intentarlo tras 1 h, 2 h, 4 h… (hasta 24 h) |
| Registro del nodo: `Error reading the off-site settings, no periodic copy until it is fixed: …` | El archivo `<id>.offsite` del servidor está dañado. Mientras tanto, ese servidor no hace copia periódica, y la tarjeta **Copia al S3** no puede leer el ajuste (solo muestra el error de carga), así que no sirve para corregirlo | Borrar el archivo `<daemon.data.servers>/<id>.offsite` en el nodo: el servidor vuelve a los valores por defecto y se puede ajustar otra vez desde la tarjeta. O enviar el ajuste directamente con `PUT /api/servers/:serverId/offsite`, que sobrescribe el archivo sin leer el anterior |
| «Este nodo no tiene S3 configurado.» en la tarjeta **Copia al S3** | El nodo del servidor no tiene las claves `daemon.backups.s3.*` completas o `enabled` vale `false` (lo habitual en un nodo remoto) | Definir las claves en la configuración de ese nodo (véase [Nodos remotos](#requisitos-y-configuración)) |
| «No se pudo leer la copia al S3 de este servidor: el nodo no ha respondido.» | El nodo no respondió, o el archivo `<id>.offsite` del servidor está dañado (el nodo responde con un error; **Reintentar** no lo arregla) | Comprobar que el nodo está en línea y pulsar **Reintentar**. Si sigue fallando y el registro del nodo muestra `Error reading the off-site settings`, véase la fila anterior |
| «La copia periódica externa solo puede hacerse cada 12, 24, 48, 72, 168, 336 o 720 horas, o desactivarse (0).» (`400`, `ErrOffsitePeriodicHours`) | Se envió un intervalo que no está en la lista | Usar uno de esos valores |
| «Las copias externas que se conservan deben ser entre 1 y 50.» (`400`, `ErrOffsiteKeep`) | Número de copias fuera de 1-50 | Usar un valor entre 1 y 50 |
| La transferencia falla con `reading its off-site settings: …` o `copying its off-site settings: …` | No se pudo leer el ajuste de copia periódica en el nodo antiguo o guardarlo en el nuevo (por ejemplo, un nodo nuevo sin la ruta `/daemon/server/:id/offsite`) | Actualizar el nodo y volver a intentarlo; la transferencia se ha deshecho |

## Véase también

- [Tareas programadas](./scheduled-tasks.md) — copias automáticas y su subida al S3
- [Configuración del panel](./panel-settings.md) — pestaña **Copias externas**
- [Historial de archivos](./file-history.md) — deshacer ediciones de archivos de configuración sin restaurar todo
- [Notificaciones del servidor](./server-notifications.md) — avisos de copias por servidor
- [Archivos y compresión](../general/19-files-compression.md)
- [Discord](../general/16-discord.md) — avisos globales de copias
- [Configuración](../backend/02-configuration.md)
- [Referencia de la API](../reference/api/04-server-content.md#copias-de-seguridad)
