# Registro de actividad y registros de acceso

Aether Panel mantiene dos registros de auditoría independientes:

- El **registro de actividad del servidor**: quién cambió qué en un servidor concreto y cuándo (encendidos y apagados, comandos de consola, archivos, copias de seguridad, usuarios, ajustes, tareas...).
- Los **registros de acceso**: inicios de sesión, intentos fallidos y altas de cuenta de todo el panel, que se muestran en el panel de control del administrador.

Ambos se guardan en la base de datos del panel y se conservan **90 días**.

## Qué hace

### Registro de actividad del servidor

- Anota cada cambio **correcto** hecho mediante la API del panel bajo `/api/servers/:serverId`: inicio y parada, comandos de consola, escritura, subida, cambio de nombre y borrado de archivos, compresión y extracción, instalación de plugins, modpacks y mapas, copias de seguridad, usuarios del servidor, ajustes, tareas programadas, migraciones, cambios de versión, restauraciones del historial de archivos y clonaciones.
- Cada entrada guarda la clave de la acción (por ejemplo `server.start`, `file.write`, `backup.restore`), un detalle breve (nombre de archivo, comando, nombre de la copia, nombre de la tarea, correo del usuario afectado...), quién lo hizo, la IP del cliente y la hora.
- Agrupa las entradas en familias para filtrarlas: **Energía**, **Consola**, **Archivos**, **Copias**, **Usuarios**, **Ajustes** y **Tareas**.

### Registros de acceso

- Anota estos eventos:

| Evento | Cuándo |
|---|---|
| `login` | Un inicio de sesión correcto (contraseña, o contraseña y código 2FA). |
| `login_failed` | Una contraseña o un código 2FA rechazados para una cuenta **existente**. Las direcciones de correo desconocidas no se anotan, para que el formulario público no sirva para llenar la tabla. |
| `register` | Se creó una cuenta, por autorregistro o por un operador mediante la API de usuarios. |
| `login_sso` | Un inicio de sesión mediante un enlace SSO de un solo uso del sistema de facturación. |

- Cada evento guarda el nombre de usuario y el correo (copiados, para que el registro sobreviva a cambios de nombre y borrados), la IP y el agente de usuario del navegador. La API solo devuelve un resumen breve del agente de usuario, como `Chrome · Windows`, `Safari · iOS` o `CLI`.

## Cómo funciona

### Registro de actividad del servidor

- Un middleware en todas las rutas `/api/servers` revisa las peticiones que modifican algo (`POST`, `PUT`, `DELETE`, `PATCH`). Cuando la petición termina, si el código de respuesta es menor que 400, escribe la entrada en segundo plano. Las peticiones fallidas nunca se anotan, y un error al escribir la entrada nunca hace fallar la petición.
- `POST /ai/analyze`, `POST /ai/chat` y `POST /notifications/test` se ignoran porque no cambian nada. Una ruta de modificación sin clave de acción propia se anota como `server.change`, con la ruta como detalle.
- Los detalles se recortan: los comandos de consola a 200 caracteres y cualquier detalle a 300. Los cambios de ajustes, datos, indicadores, notificaciones, puertos y definición del servidor solo anotan **que** cambiaron, nunca el cuerpo, porque puede contener secretos (webhooks, contraseñas).
- Los cambios hechos fuera de la API del panel no se anotan: transferencias por SFTP y lo que hacen las tareas programadas cuando se disparan solas según su programación (guardar, eliminar y ejecutar una tarea a mano *sí* se anotan).
- Una transferencia entre nodos se ejecuta en segundo plano después de responder a la petición, así que el propio panel anota su resultado: `server.transferFailed` cuando un paso falló y la transferencia se deshizo, y `server.transferStartFailed` cuando el servidor se trasladó pero no se pudo iniciar en el nodo nuevo. El detalle indica en qué paso y por qué, y la página lo muestra como código (véase [Transferencias de servidores](server-transfers.md)).
- Cuando se elimina un servidor desde el panel, su registro completo se elimina con él. La retención se aplica como mucho una vez por hora mientras se escriben entradas nuevas: borra las entradas de más de 90 días y las de servidores que ya no existen.

### Registros de acceso

- Los manejadores de inicio de sesión, registro y SSO anotan el evento en el momento. Las filas de más de 90 días se borran cada vez que se lee la lista.
- Los tipos de evento `logout` y `password_reset` existen en el modelo de datos, pero de momento no se anotan.

## Uso

### Desde el panel

**Actividad del servidor.** Abra el servidor y seleccione la pestaña **Actividad**. La pestaña se muestra a todos los usuarios del servidor, pero la lista exige `server.users.view` en ese servidor; sin él, la pestaña muestra "No tienes acceso a la actividad" y "Pide al propietario del servidor permiso para ver los usuarios."

- Las entradas se agrupan por día (**Hoy**, **Ayer** y después fechas), de la más reciente a la más antigua, de 50 en 50; **Cargar más** trae las anteriores y **Actualizar** recarga la lista.
- **Filtrar por tipo**: **Todo**, **Energía**, **Consola**, **Archivos**, **Copias**, **Usuarios**, **Ajustes**, **Tareas**.
- La dirección IP de cada entrada solo se muestra a los administradores (quienes tienen el permiso `admin`).

**Registros de acceso.** El panel de control del administrador (visible para usuarios con `nodes.view` o `users.info.search`) incluye la tarjeta **Registros de acceso** con los 6 últimos eventos: quién, cuándo (tiempo relativo), el evento (**Inicio de sesión**, **Intento fallido**, **Acceso desde facturación**, **Cuenta creada**) y la IP. La tarjeta se actualiza una vez por minuto. Para leerla hace falta `users.info.search`; un usuario del panel de control que solo tiene `nodes.view` no consulta los registros, y la tarjeta muestra entonces el texto de lista vacía ("Todavía no hay accesos registrados...").

### Mediante la API

Los detalles completos están en la [referencia de la API](../reference/11-api-reference.md).

| Método | Ruta | Permiso | Propósito |
|---|---|---|---|
| `GET` | `/api/servers/:serverId/activity` | `server.users.view` | Una página de la actividad del servidor, de la más reciente a la más antigua. |
| `GET` | `/api/activity` | `users.info.search` | Una página de registros de acceso, del más reciente al más antiguo. |

Parámetros de consulta de `GET /api/servers/:serverId/activity`:

| Parámetro | Por defecto | Descripción |
|---|---|---|
| `limit` | `50` | Entradas por página, de 1 a 100 (los valores mayores se limitan a 100). |
| `before` | — | Solo entradas con un id menor que este (el cursor de la página anterior). |
| `type` | todas | Una familia (`power`, `console`, `files`, `backups`, `users`, `settings`, `tasks`), `all` o un prefijo de acción como `file` o `backup.restore` (letras minúsculas con un punto como máximo). Cualquier otro valor no devuelve entradas. |

Respuesta:

```json
{
  "items": [
    { "id": 812, "action": "file.write", "detail": "server.properties", "username": "alice", "userId": 3, "ip": "203.0.113.7", "createdAt": "2026-10-30T18:22:41Z" }
  ],
  "nextCursor": 763
}
```

`ip` solo aparece para administradores. `nextCursor` es `null` en la última página.

Parámetros de consulta de `GET /api/activity`: `event` (`login`, `login_failed`, `register` o `login_sso`), `page` (por defecto 1) y `limit` (por defecto 10, máximo 100). La respuesta es `{"activity": [...], "metadata": {"paging": {...}}}`; cada elemento tiene `id`, `event`, `username`, `email`, `ip`, `userAgent` (resumen) y `createdAt`.

### Claves de acción

| Familia | Claves de acción |
|---|---|
| Energía | `server.start`, `server.stop`, `server.restart`, `server.kill`, `server.install`, `server.reload` |
| Consola | `console.command` |
| Archivos | `file.write`, `file.upload`, `file.mkdir`, `file.rename`, `file.delete`, `file.archive`, `file.extract`, `file.restore`, `plugin.install`, `plugin.delete`, `modpack.install`, `map.install`, `map.remove` |
| Copias | `backup.create`, `backup.restore`, `backup.delete` |
| Usuarios | `user.add`, `user.edit`, `user.remove` |
| Ajustes | `settings.data`, `settings.ports`, `settings.definition`, `settings.flags`, `settings.notifications`, `server.create`, `server.rename`, `server.suspend`, `server.unsuspend`, `server.transfer`, `server.transferFailed`, `server.transferStartFailed`, `server.change`, `server.clone`, `extransfer.create`, `extransfer.pull`, `extransfer.cancel` |
| Tareas | `task.save`, `task.delete`, `task.run` |

`server.version` (cambio de versión del juego) se anota, pero no pertenece a ninguna familia: solo aparece en **Todo** o con `type=server`.

## Requisitos y configuración

- Los dos registros están siempre activos. Ninguna clave de configuración los activa, desactiva ni cambia su retención; el plazo de 90 días está fijado en el código.
- Usan la base de datos del panel configurada con `panel.database.*`. Véase [Configuración](../backend/02-configuration.md).

## Límites

- La retención es de 90 días en ambos registros.
- Una página de actividad del servidor contiene como máximo 100 entradas; una de registros de acceso, como máximo 100.
- Solo se anotan las peticiones hechas mediante la API del panel. La actividad por SFTP y las ejecuciones desatendidas de tareas no aparecen en el registro de actividad del servidor.
- Los registros de acceso no incluyen cierres de sesión ni restablecimientos de contraseña.
- Las peticiones que fallan (código 400 o superior) no se anotan, de modo que los intentos denegados de cambiar un servidor no aparecen en su registro de actividad.

## Problemas conocidos y soluciones

| Síntoma o mensaje | Causa y solución |
|---|---|
| `invalid limit` (400) | `limit` no es un entero positivo. |
| `invalid cursor` (400) | `before` no es un entero no negativo. Pase el valor de `nextCursor` de la página anterior. |
| La pestaña **Actividad** indica "No tienes acceso a la actividad" | El usuario no tiene `server.users.view` en ese servidor. El propietario puede concederlo en la pestaña **Usuarios**. |
| Un filtro con `type` personalizado no devuelve nada | El valor no es una familia ni un prefijo en minúsculas como `file` o `backup.restore`. |
| Falta un cambio hecho por SFTP | Es lo esperado: solo se anotan las peticiones a la API del panel. |
| La tarjeta **Registros de acceso** indica "Tu cuenta no tiene permiso para ver los registros de acceso." | `/api/activity` respondió `403`: la cuenta no tiene `users.info.search`. |
| La tarjeta **Registros de acceso** indica "Esta versión del panel todavía no ofrece los registros de acceso." | `/api/activity` falló con algo distinto de `403` (por ejemplo `404` en una versión antigua del panel, un `500` o un error de red). Revise el registro del panel y actualícelo si es una versión antigua. |
| No se ve ninguna IP en la actividad del servidor | Es lo esperado para quien no es administrador: la API omite `ip` salvo que quien llama tenga `admin`. |

## Véase también

- [Tareas programadas](scheduled-tasks.md)
- [Seguridad](../general/security.md)
- [Referencia de permisos](../general/scopes-reference.md)
- [Referencia de la API](../reference/11-api-reference.md)
