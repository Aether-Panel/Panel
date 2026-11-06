# Tareas programadas

Las tareas programadas automatizan el trabajo rutinario de un servidor: reinicios diarios, copias de seguridad automáticas con retención, avisos a los jugadores antes de un reinicio o cualquier comando de consola a una hora fija. Las tareas se ejecutan solas en el nodo, tenga alguien el panel abierto o no.

La estructura interna del planificador (el tipo `Scheduler` y su archivo) se describe en [Servidores: planificador](../backend/04-servers.md). Esta página describe la función tal como la ve un administrador o un usuario del servidor.

## Qué hace

- Cada servidor tiene su propia lista de tareas, con un máximo de **50 tareas por servidor**.
- Una tarea tiene nombre, una descripción opcional, una programación y una lista ordenada de **pasos**. Los pasos se ejecutan uno tras otro; si un paso falla, se omite el resto de la tarea.
- Una tarea puede ejecutarse según una programación (cron) o no tener programación ("solo a mano") y ejecutarse únicamente cuando alguien pulsa **Ejecutar ahora**.
- Pasos disponibles para cualquier usuario con los permisos adecuados:

| Paso (`type`) | Etiqueta en la interfaz | Argumentos | Qué hace |
|---|---|---|---|
| `stdin` | Enviar comando | `command` | Envía una línea a la consola del servidor, como si se escribiera. |
| `sleep` | Esperar | `duration` (por ejemplo `5m`) | Pausa la tarea. |
| `start` | Iniciar | — | Inicia el servidor si no está ya encendido. |
| `stop` | Detener | — | Detiene el servidor y espera hasta 5 minutos a que termine. |
| `restart` | Reiniciar | — | Detiene e inicia el servidor, **solo si está encendido**. Un servidor que su propietario apagó no se vuelve a encender. |
| `backup` | Copia de seguridad | `keep` (1–50, por defecto 5), `offsite` (`never`, `always`, `weekend`, `weekly`, `biweekly`, `monthly`; `weekly` en un paso nuevo, `always` si falta), `offsiteKeep` (1–50, por defecto 4) | Hace una copia automática y borra del nodo las copias automáticas más antiguas que superen `keep`. Las copias hechas a mano nunca se borran. Según `offsite` (**Subir al S3**), sube además la copia al bucket y deja allí las `offsiteKeep` más recientes de la tarea; véase [Copias de seguridad](backups.md#subida-de-las-copias-de-tareas). |

- Los administradores (quienes tienen el permiso `admin`) también pueden usar cualquier operación del [motor de operaciones](../backend/06-operations.md) (comandos de shell, descargas, escritura de archivos...) y condiciones `if`. El resto de usuarios no, porque una tarea se ejecuta en el nodo sin supervisión.

## Cómo funciona

- Las tareas de un servidor se guardan en el nodo en `<daemon.data.servers>/<serverId>.cron` (JSON, permisos `0600`). El nodo las carga al cargar el servidor, por lo que se disparan aunque el servidor de juego esté detenido.
- La planificación usa gocron. Las programaciones son expresiones cron con un campo de segundos opcional, un prefijo `CRON_TZ=` o `TZ=` opcional y descriptores como `@daily`. Una tarea con la programación vacía se registra con fecha de ejecución en el año 9999, de modo que solo se ejecuta con **Ejecutar ahora**.
- El planificador de cada servidor ejecuta como máximo **5 tareas a la vez**; las ejecuciones adicionales esperan a que quede un hueco (`concurrentLimit: 5`, `limitMode: "wait"` en el archivo `.cron`). Estos valores no se exponen en el panel ni en la API.
- Mientras un servidor se [traslada a otro nodo](server-transfers.md), sus tareas quedan **pausadas** en ambos nodos (`"paused": true` en el archivo `.cron`, que se mantiene si el nodo se reinicia): las ejecuciones que tocan y **Ejecutar ahora** se omiten, y el nodo registra `Task <id> skipped: the server is being moved to another node`. Una tarea que ya se está ejecutando no se interrumpe. La transferencia copia las tareas al nodo nuevo y las reanuda en el nodo que se queda con el servidor; si la transferencia se deshace, se reanudan en el nodo antiguo.
- Al ejecutarse una tarea, la consola muestra `Running task <nombre>` y, al terminar, `Task <nombre> finished`. Si un paso falla, la consola muestra `Failed to setup tasks` seguido del error.
- Las copias automáticas se escriben como `auto-<id-de-la-tarea>-<uuid>.tar.gz` en la carpeta de copias del servidor (las de versiones anteriores, `auto-<uuid>.tar.gz`). El planificador pone el id de la tarea en cada paso de copia al ejecutarla, también con **Ejecutar ahora**, sin leerlo de los argumentos guardados. El panel las añade a la lista de **Copias de seguridad** (con el nombre "Automatic backup") la próxima vez que se carga la lista, y quita las entradas que la retención ha borrado. Borrar una tarea no borra sus copias del nodo ni del bucket. Véase [Copias de seguridad](backups.md).
- El panel siempre escribe una programación de cinco campos precedida de la zona horaria del navegador de quien crea la tarea, por ejemplo `CRON_TZ=Europe/Madrid 0 5 * * *`. Así, "05:00" significa las 05:00 en el lugar donde vive quien creó la tarea, sea cual sea la zona horaria del nodo.

### Comprobaciones para usuarios que no son administradores

Cuando quien llama no tiene `admin`, el panel valida la tarea antes de enviarla al nodo:

| Regla | Límite |
|---|---|
| Tipos de paso permitidos | `stdin`, `sleep`, `start`, `stop`, `restart`, `backup` |
| Pasos por tarea | De 1 a 10 |
| Longitud del nombre / de la descripción | 64 / 256 caracteres |
| Comando (`stdin`) | una sola línea, hasta 512 caracteres |
| Espera (`sleep`) | mayor que 0, hasta 1 hora |
| `keep` (`backup`) | De 1 a 50 |
| `offsite` / `offsiteKeep` (`backup`) | `never`, `always`, `weekend`, `weekly`, `biweekly` o `monthly` / de 1 a 50 |
| Programación | cinco campos (minuto, hora, día del mes, mes, día de la semana) o `@yearly`, `@annually`, `@monthly`, `@weekly`, `@daily`, `@midnight`, `@hourly`, opcionalmente tras `CRON_TZ=`/`TZ=`. Sin campo de segundos ni `@every`, de modo que una tarea se ejecuta como mucho una vez por minuto. |
| Tamaño del cuerpo | 64 KiB |
| Condiciones (`if`) | no permitidas |

Además, cada paso exige el permiso que necesita el botón equivalente: `stdin` necesita `server.console.send`, `start` necesita `server.start`, `stop` necesita `server.stop`, `restart` necesita ambos y `backup` necesita `server.backup.create`.

## Uso

### Desde el panel

Abra el servidor y seleccione la pestaña **Tareas** (visible para usuarios con `server.tasks.view`).

- Si aún no hay tareas, la pestaña ofrece tres **Plantillas**: **Reinicio diario** (todos los días a las 05:00), **Copia de seguridad diaria** (todos los días a las 04:00, guarda las 7 últimas y sube una cada semana al S3) y **Aviso y reinicio** (envía `say El servidor se reiniciará en 5 minutos`, espera 5 minutos y reinicia).
- **Nueva tarea** abre el editor:
  - **Nombre**.
  - **Cuándo** → **Frecuencia**: **Todos los días** (una **Hora**), **Días de la semana** (una hora y uno o más **Días**), **Cada pocas horas** (cada 1, 2, 3, 4, 6, 8 o 12 h, en punto), **Personalizada (cron)** (una **Expresión cron** de cinco campos, por ejemplo `30 4 * * 1`) o **Solo a mano**. El editor indica qué zona horaria usan las horas.
  - **Qué hace**: **Añadir paso**, reordenar con las flechas, quitar. La lista solo ofrece los pasos que el usuario puede usar. **Esperar** ofrece 1, 5, 10, 15, 30 o 60 minutos. **Copia de seguridad** tiene **Guardar las últimas** (1–50), **Subir al S3** (**Nunca**, **Siempre**, **Cada fin de semana**, **Cada semana**, **Cada 15 días** o **Cada mes**; un paso nuevo trae **Cada semana**) y, salvo con **Nunca**, **Conservar en S3** (1–50, 4 por defecto).
  - **Guardar tarea**.
- Cada tarjeta de tarea muestra su programación, la **Próxima** ejecución (solo para programaciones sencillas en la zona horaria de quien la ve), sus pasos y los botones **Ejecutar ahora**, **Editar** y **Eliminar**. Un paso que el usuario no puede editar aparece como "Paso de administrador: `<tipo>`".
- **Ejecutar ahora** lanza la tarea de inmediato; su salida puede seguirse en la consola.

### Mediante la API

Todas las rutas son relativas a `/api/servers/:serverId`. Los detalles completos de peticiones y respuestas están en la [referencia de la API](../reference/11-api-reference.md).

| Método | Ruta | Permiso | Propósito |
|---|---|---|---|
| `GET` | `/tasks` | `server.tasks.view` | Lista las tareas con sus pasos (`{"tasks": {"<id>": Task}}`). |
| `GET` | `/tasks/:taskId` | `server.tasks.view` | Devuelve una tarea. `404` si no existe. |
| `PUT` | `/tasks/:taskId` | `server.tasks.create` o `server.tasks.edit` | Crea la tarea o sustituye la que tiene ese id. `204` si todo va bien. |
| `DELETE` | `/tasks/:taskId` | `server.tasks.delete` | Elimina la tarea. `404` si no existe. |
| `POST` | `/tasks/:taskId/run` | `server.tasks.run` | Ejecuta la tarea ahora. Devuelve `204` al momento; la tarea sigue en segundo plano. |

El id de la tarea lo elige el cliente: de 1 a 64 letras, dígitos, `-` o `_`. El panel genera ids como `task-lq3x9k2a`.

Ejemplo de cuerpo para `PUT /api/servers/:serverId/tasks/nightly-backup`:

```json
{
  "name": "Copia nocturna",
  "cronSchedule": "CRON_TZ=Europe/Madrid 0 4 * * *",
  "operations": [
    { "type": "stdin", "command": "save-all" },
    { "type": "sleep", "duration": "1m" },
    { "type": "backup", "keep": 7, "offsite": "weekly", "offsiteKeep": 4 }
  ]
}
```

Guardar, eliminar y ejecutar una tarea queda anotado en el [registro de actividad](activity-and-access-log.md) del servidor (`task.save`, `task.delete`, `task.run`). Los pasos que ejecuta una tarea cuando se dispara sola según su programación no se anotan ahí.

## Requisitos y configuración

- Ninguna clave de configuración activa o desactiva la función; siempre está disponible.
- Los archivos `.cron` están en la carpeta indicada por `daemon.data.servers` (por defecto `servers` dentro de la raíz de datos del daemon; variable de entorno `SKYPANEL_DAEMON_DATA_SERVERS`). Véase [Configuración](../backend/02-configuration.md).
- La zona horaria de `CRON_TZ=` debe existir en la base de datos de zonas horarias del nodo; si no, el nodo rechaza la programación.

## Límites

- 50 tareas por servidor; 10 pasos por tarea para quienes no son administradores.
- Quienes no son administradores no pueden programar con una frecuencia mayor que una vez por minuto.
- `stop` y `restart` dan al juego 5 minutos para terminar; pasado ese tiempo, el paso falla y se omite el resto de la tarea.
- Solo puede hacerse una copia de seguridad por servidor a la vez. Un paso `backup` que empieza mientras se hace otra copia falla.
- La **Próxima** ejecución solo la calcula el navegador para programaciones diarias, semanales y por horas en la zona horaria de quien la ve. Las expresiones cron personalizadas solo las evalúa el nodo.
- Las ejecuciones que tocaban mientras el nodo estaba apagado, o mientras las tareas estaban pausadas por una transferencia entre nodos, no se recuperan después.
- Una transferencia entre nodos copia las tareas, pero no la zona horaria ni los ajustes de concurrencia del archivo `.cron`; el nodo nuevo usa sus valores por defecto.

## Problemas conocidos y soluciones

| Mensaje | Causa y solución |
|---|---|
| `invalid task: the task id may only use letters, digits, - and _` | El id de la URL no tiene entre 1 y 64 caracteres de `A-Z a-z 0-9 - _`. Use un id válido. |
| `invalid task: the task needs a name` | El campo `name` está vacío. |
| `invalid task: a server can have at most 50 tasks` | Elimine las tareas que ya no necesite. |
| `invalid task: <error de cron>` | El nodo no pudo interpretar `cronSchedule`. Revise el número de campos y el nombre de la zona de `CRON_TZ=`. |
| `the schedule must have five fields: minute, hour, day of month, month and day of week` | Quienes no son administradores no pueden usar campo de segundos ni `@every`. Use cinco campos o un descriptor permitido. |
| `only administrators can use "<tipo>" steps` / `only administrators can use conditions` | El tipo de paso o la condición `if` están reservados a administradores. |
| `you need the <permiso> permission for "<tipo>" steps` | Solicite ese permiso al propietario del servidor o a un administrador. |
| `a task needs between 1 and 10 steps` | Añada al menos un paso o divida las tareas largas. |
| `a command must be a single line of at most 512 characters` | Quite los saltos de línea o acorte el comando. |
| `a wait step can last up to one hour` | Use una duración de `sleep` mayor que 0 y de hasta `1h`. |
| `keep must be between 1 and 50` | Ajuste el `keep` del paso de copia. |
| `invalid off-site frequency "…"` / `off-site keep must be between 1 and 50` | Use para `offsite` uno de `never`, `always`, `weekend`, `weekly`, `biweekly` o `monthly`, y para `offsiteKeep` un número de 1 a 50. |
| Consola: `Off-site copy skipped: …` | No se pudo listar el bucket para decidir si la copia sube. La copia está en el nodo y la tarea sigue; véase [Copias de seguridad](backups.md#subida-de-las-copias-de-tareas). |
| `the task is too large` (413) | El cuerpo de la petición supera 64 KiB. |
| Consola: `Failed to setup tasks` + `backup in progress` | Había otra copia en curso. Separe en el tiempo las tareas de copia. |
| Consola: `Server is not running, skipping the restart` | Es lo esperado: `restart` no enciende un servidor detenido. Use `start` si es lo que se busca. |
| La pestaña indica "No tienes permiso para ver las tareas de este servidor." | El usuario no tiene `server.tasks.view` en este servidor. |

## Véase también

- [Servidores: planificador](../backend/04-servers.md)
- [Motor de operaciones](../backend/06-operations.md)
- [Copias de seguridad](backups.md)
- [Registro de actividad y registros de acceso](activity-and-access-log.md)
- [Referencia de permisos](../general/scopes-reference.md)
- [Referencia de la API](../reference/11-api-reference.md)
