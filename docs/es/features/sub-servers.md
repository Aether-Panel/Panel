# Subservidores (Splitter)

Un subservidor es un servidor creado con parte de los recursos de otro servidor. El propietario de un servidor con, por ejemplo, 8 GB de memoria puede dividirlo en un lobby y dos servidores de juego que comparten esos 8 GB, sin pedir un plan nuevo a un administrador. En el panel esta función se llama **Splitter**.

## Qué hace

- Crea servidores hijos cuya CPU, memoria y disco se descuentan del plan del servidor padre.
- Mantiene el contenedor del padre limitado a lo que dejan libre sus subservidores.
- Muestra en el padre los recursos totales, cuánto usan los subservidores y cuánto queda libre, además de una tarjeta por subservidor con su estado y sus límites.
- Asigna a los subservidores el nodo, el propietario y los miembros del padre.

## Cómo funciona

**Modelo de datos.** Un subservidor es un servidor normal con `parent_server_id` igual al identificador del padre (`servers.parent_server_id`). El panel guarda el plan completo de cada servidor en `TotalCPU`, `TotalMemory` y `TotalDisk`.

**Creación.** Cuando se crea un servidor con padre (`PUT /api/servers/:serverId` con `parent_server_id`):

1. El subservidor se coloca en el **nodo del padre**, indique la petición el nodo que indique.
2. Sus usuarios son los miembros del padre y cada uno conserva **exactamente los permisos que tiene en el padre** (un subusuario que en el padre solo ve la consola, en el hijo tampoco puede hacer más).
3. Su propietario es el propietario del padre. Si el padre no tiene propietario, el hijo tampoco.
4. Debe tener un límite de CPU, memoria y disco mayor que 0 (para Docker, 0 significa "sin límite").
5. En una transacción que bloquea la fila del padre, el panel suma los límites de los subservidores existentes y rechaza el nuevo si el padre quedaría por debajo de cero en algún recurso.
6. Se crea el servidor en el nodo y, si `panel.servers.autoInstall` está activado, empieza su instalación.
7. Los límites del padre en el nodo se reducen a `plan − suma de los subservidores` (nunca por debajo de 1, porque 0 es ilimitado para Docker). En la base de datos del panel se mantiene el plan completo del padre.

**Usuarios que no son administradores.** Sin `admin`, solo se pueden crear servidores mediante este flujo:

- La petición debe indicar un padre y el usuario debe ser su **propietario** (los subusuarios del padre no pueden dividirlo).
- El entorno debe ser `docker`. Solo se conservan las claves `image`, `portBindings`, `containerRoot` y `config`; se descartan los montajes del host (`bindings`), `hostConfig`, la red y las etiquetas, y se elimina el usuario del contenedor de `config`.
- Cada puerto publicado debe usar una de las variables de puerto del propio servidor (por ejemplo `0.0.0.0:${port}:${port}`).

**Cambio de límites.** Cuando un administrador cambia los recursos (`PUT /api/servers/:serverId/data`):

- En un subservidor, los límites nuevos deben caber en lo que le queda al padre; después se recalculan los límites del padre en el nodo.
- En un padre, el plan nuevo no puede ser inferior a lo que ya ocupan sus subservidores, y al nodo se envía lo que los subservidores dejan libre.

**Borrado.** Al borrar un subservidor, sus recursos vuelven al padre (se recalculan sus límites en el nodo). Un padre con subservidores no se puede borrar.

**Suspensión.** Suspender o reactivar un padre hace lo mismo con todos sus subservidores; al suspender, además, se detienen.

## Uso

### Desde el panel

1. Se abre el servidor padre y se entra en la pestaña **Splitter**. La ven el propietario del servidor y los usuarios con `server.data.edit.admin` (y los administradores).
2. La tarjeta **División del servidor** muestra **CPU total**, **Memoria RAM total** y **Disco total**, con lo **Usado** y lo **Libre** de cada recurso.
3. Se pulsa **Crear subservidor**. Se abre el asistente de creación con el padre fijado: el nodo, el entorno y los usuarios se heredan (**Entorno heredado**). Se eligen el nombre, la plantilla y la CPU, memoria y disco del subservidor.
4. El subservidor nuevo aparece en **Subservidores activos**. Al pulsar una tarjeta se abre.

Si el servidor ya es un subservidor, la pestaña solo indica: *Este servidor ya es un subservidor. Solo los servidores principales se pueden dividir.*

### Mediante la API

| Método | Ruta | Permiso | Finalidad |
|--------|------|---------|-----------|
| PUT | `/api/servers/:serverId` | `server.create` (sin `admin`: ser propietario del padre) | Crea un servidor; con `parent_server_id` es un subservidor. |
| PUT | `/api/servers/:serverId/data` | `server.data.edit.admin` | Cambia los límites; se comprueban frente al padre o a los subservidores. |
| DELETE | `/api/servers/:serverId` | `server.delete` | Borra; se rechaza si el padre todavía tiene subservidores. `204`, o `200 {"leftoverDatabases": [...]}` cuando un host de bases de datos no pudo borrar alguna de sus bases de datos MySQL (véase la [referencia de la API](../reference/api/03-servers.md#delete-apiserversserverid)). |
| GET | `/api/servers` | sesión (lista los servidores visibles para el usuario) | Lista servidores; cada elemento incluye `parent_server_id` cuando es un subservidor. |

Cuerpo mínimo de un subservidor (el resto es la definición de la plantilla, como en cualquier servidor):

```json
{
  "name": "Lobby",
  "parent_server_id": "a1b2c3d4",
  "environment": { "type": "docker", "image": "…" },
  "data": {
    "cpu":    { "value": 100 },
    "memory": { "value": 2048 },
    "disk":   { "value": 10240 }
  }
}
```

La definición completa del servidor y el formato de las respuestas están en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

- El padre debe tener límites reales (CPU, memoria y disco). Un padre con límite 0 (ilimitado) no tiene nada que repartir.
- Los usuarios que no son administradores necesitan el permiso global `server.create`. Lo incluye el rol por defecto "Usuario"; véase la [referencia de permisos](../general/scopes-reference.md).
- `panel.servers.autoInstall` (por defecto `true`, `SKYPANEL_PANEL_SERVERS_AUTOINSTALL`) decide si un subservidor nuevo se instala en el acto. Véase [Configuración](../backend/02-configuration.md).

No hay ajustes específicos de los subservidores.

## Límites

- La interfaz admite un solo nivel: un subservidor no se puede volver a dividir desde la pestaña **Splitter**.
- Un subservidor siempre está en el nodo de su padre. Una [transferencia entre nodos](server-transfers.md) del padre traslada con él sus subservidores, con sus archivos, copias de seguridad y tareas programadas; un subservidor no se puede transferir por separado (`400` `ErrTransferSubServer`).
- La CPU es un porcentaje (100 = un núcleo); la memoria y el disco van en MB, como en cualquier servidor.
- El plan del padre en la base de datos no se reduce; solo los límites aplicados en el nodo. Por eso las gráficas de uso del padre se comparan con los límites del nodo.
- Los clientes no pueden elegir montajes del host, modo privilegiado, redes, etiquetas ni el usuario del contenedor de un subservidor.

## Problemas conocidos y soluciones

| Mensaje | Causa | Solución |
|---------|-------|----------|
| `not enough CPU available in parent server` (o memory, disk) | El subservidor nuevo o redimensionado no cabe en lo que le queda al padre. | Bajar los límites del subservidor, o reducir o borrar otros subservidores. |
| `a sub-server needs a CPU, memory and disk limit` | Algún límite era 0. | Dar a cada recurso un valor mayor que 0. |
| `only the parent server's owner can split it` | Un subusuario del padre intentó dividirlo. | Debe crearlo el propietario (o un administrador). |
| `only administrators can create servers without a parent server` | Un usuario sin `admin` llamó a la ruta de creación sin `parent_server_id`. | Usar la pestaña **Splitter** o pedirlo a un administrador. |
| `only docker environments are allowed for sub-servers` | El entorno no era `docker`. | Usar una plantilla de Docker. |
| `sub-servers can only publish their port variables, not "…"` / `… is not a port variable of the server` | Un puerto publicado usa un número fijo o una variable desconocida. | Usar publicaciones basadas en las variables de puerto del servidor. |
| `parent server not found` | `parent_server_id` no existe. | Revisar el identificador del padre. |
| `the sub-servers of this server already use more than that` | Un administrador intentó reducir un padre por debajo de lo que ocupan sus subservidores. | Reducir o borrar antes los subservidores. |
| `cannot delete server because it has active child servers` | Se intentó borrar un padre que aún tiene subservidores. | Borrar antes sus subservidores. |
| `403` sin mensaje al pulsar **Crear subservidor** | Al usuario le falta `server.create`. | Concederlo directamente o mediante un rol. |

## Véase también

- [Referencia de permisos: propietario, subusuarios y subservidores](../general/scopes-reference.md)
- [Clonar servidores](cloning.md) (la copia de un subservidor es un subservidor del mismo padre)
- [Servidores (backend): usuario del contenedor y aislamiento](../backend/04-servers.md)
- [Referencia de la API](../reference/11-api-reference.md)
