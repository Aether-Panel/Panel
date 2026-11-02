# Configuración del servidor

La pestaña **Configuración** de un servidor reúne lo que lo define en el día a día: su nombre, sus límites de recursos, sus puertos, qué ocurre cuando se detiene o se reinicia el nodo y las variables de la plantilla con la que se instaló. Esta página explica cada sección, quién puede cambiarla y qué hace el panel al guardar.

Las tarjetas del final de la misma pestaña tienen su propia página: la dirección del servidor y el subdominio automático ([Subdominios](./subdomains.md)), las notificaciones por servidor ([Notificaciones del servidor](./server-notifications.md)) y, si la función experimental está activada, el cambio de versión del juego ([Versiones del juego](./game-versions.md)).

## Qué hace

La pestaña se divide en secciones. Cada una solo aparece si se tiene el permiso que necesita:

| Sección | Muestra | Quién la ve | Quién puede cambiarla |
|---|---|---|---|
| **Información general** | El nombre del servidor y su tipo (solo lectura) | Todo el que ve la pestaña | `server.name.edit` |
| **Límites de recursos** (etiqueta `Solo administradores`) | CPU, memoria, disco y prioridad de disco | Administradores (`admin`, `server.admin`, `server.admin.config.view` o `server.admin.config.manage`) | `server.data.edit.admin` |
| **Puertos** | Puertos asignados, el principal y una nota por puerto | `server.data.view` | Puertos: `server.data.edit.admin`. Notas: `server.data.edit` |
| **Condiciones de inicio automático** | Iniciar en el arranque, Reiniciar en caso de fallo, Reiniciar al detener | Todo el que ve la pestaña | `server.flags.edit` |
| **Configuración de plugins** | Habilitar pestaña de plugins (solo Minecraft Java) | Todo el que ve la pestaña | Cualquiera (es una preferencia del navegador, ver más abajo) |
| Grupos de la plantilla (una sección por grupo) | Las variables de la plantilla, agrupadas como las define la plantilla | Todo el que ve la pestaña | `server.data.edit` (las variables marcadas como no editables por el usuario necesitan `server.data.edit.admin`) |
| **Variables** | Variables de la plantilla que no pertenecen a ningún grupo | Todo el que ve la pestaña | Igual que la anterior |
| **Metadatos del servidor** | Datos de la instalación (solo lectura) | `admin` o `server.admin` | — |

La pestaña aparece cuando se tiene `server.data.view` o `server.flags.view` en el servidor. El propietario de un servidor tiene todo lo de esta tabla salvo `server.data.edit.admin` y los permisos de administrador: puede renombrar el servidor, editar variables, notas de puertos y condiciones de inicio automático, pero no los límites ni los puertos (ver [Propietario y subusuarios](../general/scopes-reference.md)).

Cuando algo cambia aparece una barra abajo con **Tienes cambios sin guardar**, **Descartar** y **Guardar cambios**. No se envía nada hasta pulsar **Guardar cambios**.

## Cómo funciona

Al pulsar **Guardar cambios** se envían varias peticiones en este orden:

1. **Variables**: las variables editables por el usuario van a `POST /api/servers/:id/data`. Las que la plantilla marca con `userEdit: false` (CPU, memoria, disco y similares) se envían aparte a `PUT /api/servers/:id/data` cuando se tiene `server.data.edit.admin`; la ruta sin permisos de administrador las ignora.
2. **Condiciones de inicio automático**: van a `POST /api/servers/:id/flags`.
3. **Nombre**: si ha cambiado, `PUT /api/servers/:id/name/:name`.
4. **Solo Minecraft Java**: si entre las variables enviadas estaban `motd`, `ip` o `port`, el panel reescribe esas claves (`motd`, `server-ip`, `server-port`) en `server.properties`. Si el archivo aún no existe se crea solo con esas claves; cualquier otro error al leerlo cancela el guardado para no sobrescribir el archivo.
5. **Puertos** (administradores): `PUT /api/servers/:id/data` con `{ "ports": [...], "primaryPort": n }`.
6. **Notas de puertos**, solo si han cambiado: `PUT /api/servers/:id/port-settings` con el mapa `portNotes` completo (sustituye al guardado).

La definición del servidor nunca se envía desde esta pestaña: los administradores la editan en la pestaña **Administración**.

### Límites de recursos

- **CPU asignada** (`%`): 100% equivale a un hilo del procesador.
- **Memoria RAM** (MB): límite estricto de memoria del contenedor.
- **Espacio en disco** (MB): al llenarse, el servidor se detiene y no arranca ni acepta archivos nuevos hasta que se libere espacio. Cómo se mide y qué se bloquea está en [Servidores: límite de disco](../backend/04-servers.md).
- **Prioridad de disco** (`io`): reparto de lectura y escritura de disco frente a los demás servidores del nodo, de 10 a 1000; 500 es un reparto igual. Detalles en [Servidores: prioridad de disco](../backend/04-servers.md).

El panel guarda los nuevos límites solo cuando el nodo los acepta, así que la página nunca muestra límites que el contenedor no tenga. Los valores negativos se rechazan. En servidores divididos con Splitter, un subservidor debe caber en lo que le queda libre a su padre, y un padre no puede bajar de lo que ya usan sus subservidores (ver [Subservidores](./sub-servers.md)).

### Puertos

El primer puerto de la lista es el **Principal**: es el puerto del juego y la dirección que muestra el panel. Los demás son puertos extra enlazados al mismo contenedor; el nodo los expone como las variables de plantilla `port`, `port2`, `port3`… (ver [ExtraPortBindings](../backend/04-servers.md)). Cada puerto puede llevar una **Nota** corta (por ejemplo «Juego», «RCON», «Query») que solo sirve como etiqueta.

Reglas que se comprueban al guardar:

- Al menos un puerto; todos entre 1024 y 65535; sin repetir (lo comprueban la página y la API).
- La API rechaza además un puerto en el que escucha el propio nodo (sus puertos web y SFTP) y un puerto ya asignado a otro servidor del mismo nodo.
- Cambiar **cuál** es el puerto principal exige que el servidor esté apagado (lo comprueba la página). Añadir o quitar puertos extra se permite con el servidor encendido; el cambio se aplica en el siguiente reinicio y el panel lo avisa al guardar.
- Cuando cambia el puerto principal se actualiza el registro del subdominio del servidor (ver [Subdominios](./subdomains.md)).

### Condiciones de inicio automático

| Interruptor | Campo (`/flags`) | Qué hace el nodo |
|---|---|---|
| **Iniciar en el arranque** | `autoStart` | Pone el servidor en cola para arrancar cuando arranca el daemon (por ejemplo tras reiniciar el nodo o el servicio). Los servidores en cola arrancan de uno en uno, uno por segundo. |
| **Reiniciar en caso de fallo** | `autoRestartOnCrash` | Vuelve a arrancar el servidor tras una caída, hasta `daemon.data.crashLimit` veces seguidas (por defecto `3`). La cuenta se reinicia si el servidor llevaba más de 10 minutos en marcha antes de caer. Una parada o un kill pedidos desde el panel nunca cuentan como caída. |
| **Reiniciar al detener** | `autoRestartOnGraceful` | Vuelve a arrancar el servidor cuando el proceso termina **por sí mismo** con su código de salida esperado (por ejemplo, alguien escribe `stop` dentro del juego). Una parada o un kill desde el panel no lo activan. |

Las caídas también se registran y se notifican; ver [Informes de caídas](./crash-reports.md).

### Interruptor de la pestaña de plugins

**Habilitar pestaña de plugins** solo muestra u oculta la pestaña **Plugins** de ese servidor. Se guarda en el navegador (`localStorage`), no en el servidor: solo afecta al navegador donde se cambió y no a otros usuarios.

## Uso

### Desde el panel

1. Abrir el servidor e ir a **Configuración**.
2. Cambiar lo necesario. Los campos que no se pueden editar no aparecen o se muestran como texto (por ejemplo, los números de puerto para quien no es administrador).
3. Pulsar **Guardar cambios** en la barra inferior, o **Descartar** para volver a los valores guardados.

Para añadir un puerto (administradores): **Puertos → Añadir puerto**, escribir el número y guardar. Para quitarlo, el botón **×** de su fila. El puerto principal no se puede quitar con el servidor encendido.

### Mediante la API

| Método y ruta | Permiso | Para qué |
|---|---|---|
| `GET /api/servers/:id/data` | `server.data.view` | Leer las variables |
| `POST /api/servers/:id/data` | `server.data.edit` | Guardar variables editables por el usuario |
| `PUT /api/servers/:id/data` | `server.data.edit.admin` | Guardar variables de administrador, `cpu`/`memory`/`disk`, `ip`/`port`, `ports` + `primaryPort`, `portNotes` |
| `PUT /api/servers/:id/port-settings` | `server.data.edit` | Cambiar el puerto principal entre los asignados y/o sustituir `portNotes` |
| `GET /api/servers/:id/flags` | `server.flags.view` o `server.flags.edit` | Leer las condiciones de inicio automático |
| `POST /api/servers/:id/flags` | `server.flags.edit` | Guardar `autoStart`, `autoRestartOnCrash`, `autoRestartOnGraceful` |
| `PUT /api/servers/:id/name/:name` | `server.name.edit` | Renombrar el servidor |
| `GET /api/servers/:id/definition` | `server.definition.view` | Leer la definición (grupos, metadatos de variables) |

Ejemplo, cambio de puertos como administrador (`25566` pasa a ser el principal y `25575` queda como puerto extra):

```bash
curl -X PUT https://panel.example.com/api/servers/abc123/data \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"ports": [25566, 25575], "primaryPort": 25566, "portNotes": {"25566": "Juego", "25575": "RCON"}}'
```

Los detalles de peticiones y respuestas están en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

| Clave | Valor por defecto | Variable de entorno | Efecto |
|---|---|---|---|
| `daemon.data.crashLimit` | `3` | `SKYPANEL_DAEMON_DATA_CRASHLIMIT` | Reinicios automáticos seguidos tras caídas |

La lista completa de claves está en [Configuración](../backend/02-configuration.md).

## Límites

- La regla «puerto principal solo con el servidor apagado» la comprueba la página, no la API.
- **Habilitar pestaña de plugins** es por navegador y no oculta la pestaña a nadie más.
- Las variables marcadas como `internal` en la plantilla nunca se muestran.
- La página lee sus datos una vez al abrirse; si otra persona cambia algo mientras tanto, un guardado posterior desde esta página lo sobrescribe.

## Problemas conocidos y soluciones

| Mensaje | Causa | Solución |
|---|---|---|
| `El servidor debe estar apagado para cambiar el puerto principal.` | El servidor está encendido | Apagarlo, cambiar el puerto principal y guardar |
| `El puerto N no es válido (debe estar entre 1024 y 65535).` / `El puerto N ya está asignado a este servidor.` | Validación de la página | Usar un puerto dentro del rango y sin repetir |
| `El puerto N lo usa el propio panel en este nodo.` (`ErrPortUsedByPanel`) | Es el puerto web o SFTP del nodo | Elegir otro puerto |
| `El puerto N ya está en uso por otro servidor en este nodo.` (`ErrPortTaken`) | Otro servidor del nodo lo tiene | Elegir un puerto libre. `POST /api/nodes/:id/ports/check` (lo usa el asistente de creación de servidores) indica qué puertos están libres en un nodo; ver [Panel de inicio y nodos](./dashboard-and-nodes.md) |
| `primaryPort must be one of the assigned ports` | Llamada a la API con un `primaryPort` que no está en `ports` | Incluirlo en `ports` |
| `the sub-servers of this server already use more than that` | Se intenta bajar un padre por debajo de lo que tienen sus subservidores | Reducir o eliminar antes los subservidores |
| `resources cannot be negative` | CPU, memoria o disco negativos | Enviar cero o un número positivo |
| `Error al guardar la configuración` | Ha fallado una de las peticiones (nodo caído, falta de permiso) | Comprobar que el nodo está en línea y que se tiene el permiso de la sección modificada |

Los errores de puertos de la API llevan un código (`ErrPortOutOfRange`, `ErrPortDuplicated`, `ErrPortUsedByPanel`, `ErrPortTaken`) y el puerto en sus metadatos, como número o, para una variable de la plantilla, como `25566 (rcon_port)`; el panel los muestra en el idioma de quien lo mira. Los demás mensajes de la API de esta tabla están en inglés.

## Véase también

- [Propiedades del servidor](./server-properties.md): `server.properties` como formulario (Minecraft Java).
- [Servidores: entornos, ciclo de vida y planificador](../backend/04-servers.md)
- [Referencia de permisos](../general/scopes-reference.md)
- [Informes de caídas](./crash-reports.md)
