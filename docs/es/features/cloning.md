# Clonar servidores

La clonación crea una copia de un servidor existente en el mismo nodo, de modo que los cambios (una versión nueva, un modpack, otra configuración) se puedan probar en la copia sin tocar el original. Es una función **experimental**: está apagada hasta que un administrador la activa.

## Qué hace

La copia recibe:

- La definición del servidor original (plantilla, versión, ajustes y variables, ejecución y entorno).
- Los mismos límites de recursos (CPU, memoria y disco).
- El mismo propietario y una fila de permisos para cada usuario del original, con los mismos permisos.
- Puertos nuevos para cada variable de puerto, elegidos para no chocar con ningún servidor del nodo.
- Opcionalmente (activado por defecto), todos los archivos y el mundo del original, junto con los archivos auxiliares que describen un modpack o un mapa del mundo instalado (`<id>.modpack`, `<id>.worldmap`).

La copia **no** recibe:

- La dirección y el puerto del original (se asignan otros nuevos).
- El subdominio.
- Las copias de seguridad.
- Las tareas programadas, alertas, métricas, informes de fallos, historial de archivos ni registro de actividad.
- El vínculo con la facturación ni el estado de suspensión.
- Los clientes OAuth2 del original (cuentas SFTP, clientes de API): solo se copian los permisos de los usuarios.

## Cómo funciona

1. El panel valida el nombre (de 1 a 40 caracteres, solo ASCII imprimible, la misma regla que los nombres de servidor).
2. Comprueba quién lo solicita (véase [Límites](#límites)) y lee la definición del original desde su nodo.
3. Genera un ID de servidor nuevo y asigna los puertos. Para cada variable de puerto (`port`, `port2`…, `queryport`, `rconport`, `steamport`, `gameport`, `serverport`, `voiceport`, `webport`, `httpport`, `appport` y cualquier clave terminada en `_port`, `-port` o `.port`) busca el primer puerto libre entre los 200 siguientes al del original (25565 → 25566) y, si no lo hay, cualquier puerto libre del nodo. Las variables que compartían puerto siguen compartiendo el nuevo. Los puertos escritos como números en los `portBindings` de Docker y las etiquetas de puertos del original también se trasladan a los puertos nuevos.
4. Guarda la copia con los usuarios del original y, si es un subservidor, comprueba que el servidor padre todavía tiene sitio para ella (véase [Subservidores](sub-servers.md)).
5. Crea el servidor en el nodo. Si la copia es un subservidor, se reducen en el nodo los límites del servidor padre.
6. Con **Copiar archivos y mundo** activado, el panel pide al nodo que copie los archivos (`POST /daemon/server/<nuevo>/clone-from/<original>`). El nodo copia en segundo plano:
   - El servidor nuevo figura como *instalándose* hasta que termina la copia, así que no se puede iniciar a medio copiar.
   - La consola de la copia muestra el progreso (`Copying files from …`, `Copied N files (X MB) so far…`) y el total final.
   - Se copian archivos normales, carpetas y enlaces simbólicos que no salen de la carpeta del servidor, con sus permisos y fechas; se omiten los enlaces que apuntan fuera, los dispositivos, sockets y tuberías; los archivos que desaparecen durante la copia (registros rotados) se ignoran.
   - Al final se reescribe `server.properties`: las líneas cuya clave contiene `port` y cuyo valor era uno de los puertos del original pasan a usar los puertos de la copia.
7. Si falla la creación en el nodo o el inicio de la copia, el panel elimina la copia a medio hacer del nodo y de la base de datos.

El registro de actividad del original anota la acción como `server.clone`, con el nombre de la copia.

## Uso

### Desde el panel

1. Un administrador activa **Configuración › General › Funciones experimentales › Clonar servidor**.
2. Se abre el servidor y se entra en **Administración › Clonar** (la pestaña aparece marcada como beta).
3. Se indica el **Nombre de la copia** (por defecto, el nombre del original seguido de « (copia)»).
4. Se elige si **Copiar archivos y mundo**. Si está desactivado, la copia empieza vacía con la misma plantilla y hay que instalarla desde **Administración**.
5. Se pulsa **Clonar servidor** y se confirma.

Al terminar, el panel muestra el **Puerto nuevo** y el botón **Abrir la copia**. Si los archivos se están copiando, el progreso se sigue en la consola de la copia y se puede iniciar en cuanto termine.

Si el original está encendido, el panel avisa de que el mundo puede copiarse a medio guardar. Para obtener una copia limpia conviene detener el servidor antes.

### Mediante la API

| Método | Ruta | Permiso | Finalidad |
|--------|------|---------|-----------|
| POST | `/api/servers/:serverId/clone` | `server.create` | Crea la copia. Responde `{"id": "<id nuevo>", "port": <puerto principal nuevo>}`. |
| POST | `/daemon/server/:serverId/clone-from/:sourceId` | nodo (token del panel) | Interna: copia los archivos de `sourceId` en el servidor nuevo y vacío `serverId`. Responde `202`. |

Cuerpo de `POST /api/servers/:serverId/clone`:

```json
{
  "name": "Mi servidor (copia)",
  "copyFiles": true,
  "label": "Mi servidor (copia)"
}
```

Si se omite `copyFiles`, vale `true`. `label` es el nombre que muestra el registro de actividad del original. El formato común de los errores está en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

| Clave | Valor por defecto | Variable de entorno | Significado |
|-------|-------------------|---------------------|-------------|
| `panel.experimental.serverClone` | `false` | `SKYPANEL_PANEL_EXPERIMENTAL_SERVERCLONE` | Activa la clonación. Mientras está apagada, la ruta responde `403` con `ErrFeatureDisabled` y la pestaña se oculta. Apagarla no borra nada. |

El usuario necesita el permiso global `server.create` (lo incluye el rol por defecto "Usuario") o `admin`. Véanse [Configuración](../backend/02-configuration.md) y la [referencia de permisos](../general/scopes-reference.md).

## Límites

- La copia siempre se crea en el **mismo nodo** que el original. Para llevarla a otro, se usa después una [transferencia entre nodos](server-transfers.md).
- **Sin `admin`**, un usuario solo puede clonar un **subservidor** de un servidor del que es propietario: crear servidores sin padre es cosa de administradores. Clonar un servidor principal sin ser administrador responde `403` ("only administrators can create servers without a parent server").
- Un usuario que no es administrador debe tener acceso al original, y el original no puede estar suspendido.
- La copia de un subservidor es un subservidor del mismo padre y descuenta su CPU, memoria y disco del plan del padre.
- Para una copia de un servidor principal, el panel no comprueba la capacidad total del nodo: simplemente recibe los mismos límites que el original.
- La copia de archivos necesita un destino sin archivos. Es una copia directa de la carpeta, no una instantánea coherente: copiar un servidor encendido puede capturar archivos a medio escribir.
- El nodo debe tener disco libre suficiente para una segunda copia de los archivos.

## Problemas conocidos y soluciones

| Mensaje o código | Causa | Solución |
|------------------|-------|----------|
| `403` `this experimental feature is turned off in the panel settings` | `panel.experimental.serverClone` está apagada. | Activar **Clonar servidor** en **Configuración › General**. |
| `403` `you can only copy servers you have access to` | El usuario no es miembro del original. | Usar una cuenta con acceso o un administrador. |
| `403` `a suspended server cannot be copied` | El original está suspendido. | Reactivarlo antes (administrador). |
| `403` `only administrators can create servers without a parent server` / `only the parent server's owner can split it` | Un usuario sin `admin` intentó clonar un servidor principal o un subservidor cuyo padre no es suyo. | Pedir la copia a un administrador. |
| `400` `the copy needs a name` / `the name can be at most 40 characters long` / `the name can only use letters without accents, digits, spaces and common symbols` | Nombre no válido. | Usar de 1 a 40 caracteres ASCII imprimibles, sin tildes. |
| `409` `there is no free port left on this node for the copy` | No se encontró ningún puerto libre. | Liberar puertos del nodo o borrar servidores que no se usan. |
| `409` `the node of this server is not reachable: …` / `the server's files are not on its node, so it cannot be copied` | El nodo no responde o el servidor es "fantasma" (existe en el panel pero no en el nodo). | Recuperar el nodo; comprobar que el servidor existe en él. |
| `422` `not enough CPU/memory/disk available in parent server` | Se copia un subservidor cuyo padre ya no tiene recursos libres. | Liberar recursos del padre (reducir o borrar otros subservidores). |
| Consola de la copia: `Copying the files failed: …` | La copia en segundo plano se detuvo (disco lleno, error de permisos). Al servidor le faltan los archivos restantes. | Corregir la causa, borrar la copia y volver a clonar. |
| Consola de la copia: `Could not update the ports in server.properties: …` | No se pudo leer o reescribir el archivo (por ejemplo, supera 1 MB). | Editar a mano `server-port` y las demás líneas de puertos de `server.properties`. |

## Véase también

- [Subservidores (Splitter)](sub-servers.md)
- [Transferencias de servidores](server-transfers.md)
- [Referencia de permisos: propietario y copias](../general/scopes-reference.md)
- [Referencia de la API](../reference/11-api-reference.md)
- [Configuración](../backend/02-configuration.md)
