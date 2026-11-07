# Transferencias de servidores

Aether Panel puede mover los archivos de un servidor de dos maneras:

- **Transferencia entre nodos**: mueve un servidor a otro nodo del **mismo panel**. El servidor conserva su identificador, sus usuarios y su configuración; solo cambia la máquina en la que se ejecuta.
- **Migración entre paneles** (transferencia federada o externa, "ExTransfer"): copia los archivos de un servidor desde **otra instancia de Aether Panel**, por ejemplo cuando un cliente se cambia de proveedor de hosting.

## Qué hace

| | Transferencia entre nodos | Migración entre paneles |
|---|---|---|
| Alcance | Dos nodos de un mismo panel | Dos paneles independientes |
| Qué se traslada | La definición del servidor, las tareas programadas, el ajuste de copia periódica, todos los archivos y las copias de seguridad guardadas en el nodo, junto con sus subservidores de Splitter | Solo los archivos (un archivo comprimido de la carpeta del servidor) |
| Dónde se inicia | En el servidor de origen (**Administración › Transferencia**) | En ambos lados: un token en el origen y una importación en el destino (pestaña **Migración**) |
| Se detiene el servidor antes | Sí (se detiene y, si no para, se fuerza; se vuelve a iniciar en el nodo nuevo si estaba encendido) | No |
| Progreso en la interfaz | No (registro del panel; un fallo también se anota en el registro de actividad del servidor) | Sí (paso actual, señal de vida y resultado) |
| Se deshace si falla | Sí, automáticamente, también tras un reinicio del panel a mitad | Se borra el archivo parcial |
| Se puede cancelar | No | Sí, desde cualquiera de los dos lados |

## Cómo funciona

### Transferencia entre nodos

Un servidor se traslada junto con sus **subservidores de Splitter** (véase [Subservidores](sub-servers.md)): viven en el nodo de su servidor principal y toman sus recursos del plan de este allí. Un subservidor que ya está en el nodo de destino se queda donde está. Un subservidor no se puede transferir por separado: la petición se rechaza con `400` `ErrTransferSubServer`; hay que transferir su servidor principal.

Antes de mover nada, `POST /api/servers/:serverId/transfer` comprueba:

- Que el destino sea otro nodo (si no, `400` `cannot transfer to the same node`).
- Que no haya ya una transferencia en curso de ninguno de estos servidores, ni una registrada como pendiente desde antes de un reinicio del panel: `409` `ErrTransferRunning`.
- Que los puertos de todos los servidores que se trasladan (el principal y los adicionales) estén libres en el nodo de destino: no deben usarlos otros servidores de ese nodo, los puertos del propio nodo (panel, daemon, SFTP) ni contenedores publicados allí. Si no, `409` `ErrTransferPortsTaken` con la lista de puertos.
- Que el nodo antiguo no esté escribiendo una copia de seguridad, un punto de restauración o una restauración de ninguno de ellos (se pregunta con `GET /daemon/server/:id/backup/busy`): `409` `ErrTransferBackupRunning`. A un nodo demasiado antiguo para tener esa ruta no se le pregunta.

Si las comprobaciones pasan, el panel guarda un registro de la transferencia en la tabla `node_transfers` (véase [Recuperación tras un reinicio del panel](#recuperación-tras-un-reinicio-del-panel)), responde `202` al momento y ejecuta la transferencia en segundo plano. Primero elimina los registros del panel de las copias que fallaron, para que ninguno apunte a una copia que el nodo antiguo nunca tuvo, y después:

1. **Preparación**, para cada servidor por orden (primero el principal):
   1. Pausa las tareas programadas del servidor en el nodo antiguo (`POST /daemon/server/:id/scheduler/pause`), para que ninguna tarea lo inicie, lo detenga o haga una copia de seguridad durante la copia.
   2. Lee la definición del servidor en el nodo antiguo, crea el servidor con ella en el nodo nuevo, pausa allí también sus tareas y copia todas las tareas programadas del nodo antiguo al nuevo (`GET /daemon/server/:id/tasks` y después `PUT /daemon/server/:id/tasks/:taskId` para cada una). Después copia el ajuste propio de [copia periódica](backups.md#copia-periódica) del servidor (`GET` y `PUT /daemon/server/:id/offsite`), si tiene uno; un nodo antiguo que no conoce la ruta (`404`) cuenta como «sin ajuste propio», y cualquier otro fallo hace fallar la transferencia, como con las tareas.
   3. Detiene el servidor en el nodo antiguo y espera; si sigue encendido, lo fuerza a parar. Si no consigue detenerlo, la transferencia falla. Una copia de seguridad que haya empezado mientras tanto también la hace fallar.
   4. Comprime todos los archivos en `transfer.tar.gz` en el nodo antiguo, envía el archivo al nodo nuevo en flujo continuo sin guardarlo en el panel, lo descomprime allí en la carpeta del servidor y lo borra.
   5. Copia a través del panel cada copia de seguridad que el nodo antiguo guarda del servidor (ubicación `local` o `both`) al nodo nuevo (`PUT /daemon/server/:id/backup/file`), con su nombre y su fecha. Una copia guardada solo fuera del nodo (S3) no necesita copiarse: el bucket es el mismo esté donde esté el servidor. Tampoco se copian las periódicas (`offsite-`), cuyo sitio es el bucket: una que aún está en el nodo antiguo se está subiendo. Si el nodo nuevo ya tiene una copia con el mismo nombre, se deja como está.
2. **Confirmación**: apunta al nodo nuevo el registro de cada servidor trasladado en una sola transacción de la base de datos, que además marca como confirmado el registro de la transferencia. Solo se escribe el nodo, así que se conservan los demás cambios hechos en el servidor durante la transferencia.
3. **Limpieza**: borra cada servidor del nodo antiguo, junto con sus copias de seguridad allí (sus copias ya están en el nodo nuevo; las copias externas no se tocan). Un fallo en este paso solo se anota en el registro: el servidor ya funciona desde el nodo nuevo y lo que queda es una copia sobrante. Se actualiza el subdominio de cada servidor trasladado para que siga a la dirección del nodo nuevo.
4. **Arranque**: reanuda las tareas de cada servidor trasladado en el nodo nuevo (`POST /daemon/server/:id/scheduler/resume`) e inicia allí los servidores que estaban encendidos antes de la transferencia. Uno que no arranca no se devuelve al nodo antiguo; el panel anota `server.transferStartFailed` en su registro de actividad con el motivo.

Si falla cualquier paso hasta la confirmación, toda la transferencia se deshace, del paso más reciente al más antiguo, para cada servidor al que se llegó: se quitan del nodo nuevo las copias de seguridad que trajo y el propio servidor, se borran los archivos comprimidos del nodo antiguo, se reanudan las tareas en el nodo antiguo y un servidor que estaba encendido se vuelve a iniciar allí. Los servidores se quedan en su nodo antiguo y el panel anota `server.transferFailed` en el registro de actividad de cada uno, con el paso y el motivo (véase [Registro de actividad](activity-and-access-log.md)). Una llamada de limpieza que falla se escribe en el registro del panel.

El registro de actividad también anota la propia petición como `server.transfer`. Cuando la transferencia termina, bien o mal, se borra su registro.

#### Recuperación tras un reinicio del panel

El registro de `node_transfers` guarda, para cada servidor que se traslada, su nodo antiguo, el último paso iniciado, si estaba encendido y qué copias de seguridad se copiaron (o se estaban copiando) al nodo nuevo. Se actualiza en cada paso y se marca como confirmado en la misma transacción de la base de datos que apunta los servidores al nodo nuevo, así que el registro y los servidores nunca discrepan.

Al arrancar, una vez registradas sus rutas (el propio nodo del panel responde a través de ellas), el panel busca registros sin terminar y cierra cada uno en una sola pasada:

- **Sin confirmar**: la transferencia se deshace como una que falló. Se deshace todo lo que pudo hacerse (deshacer un paso al que no se llegó no causa problemas): se quitan del nodo nuevo las copias de seguridad copiadas y el servidor, se borran los archivos comprimidos del nodo antiguo, se reanudan las tareas en el nodo antiguo y un servidor que estaba encendido se vuelve a iniciar allí. El registro de actividad de cada servidor recibe `server.transferFailed` con el detalle `the panel restarted while <paso>: the transfer was undone`.
- **Confirmada**: la transferencia se completa. Se borran las copias antiguas, se actualizan los subdominios, se reanudan las tareas en el nodo nuevo y se inician allí los servidores que estaban encendidos (salvo que ya lo estén); uno que no arranca recibe `server.transferStartFailed`.

Las llamadas que fallan durante la recuperación se escriben en el registro del panel y no se reintentan. El registro de la transferencia se borra en ambos casos.

### Migración entre paneles

El panel del que sale el servidor es el **origen**; el panel al que llega es el **destino**. En el destino tiene que existir ya un servidor en el que importar: la importación rellena su carpeta.

1. **Token (origen).** El propietario genera un token (`POST /api/servers/:serverId/extransfer/create`). Es un valor aleatorio de 32 bytes, válido **15 minutos**, y la primera validación correcta lo consume (una sesión que terminó en `FAILED` puede validarse de nuevo hasta que el token caduque). El origen solo guarda un hash HMAC-SHA256.
2. **Importación (destino).** En el servidor de destino, el propietario introduce la URL del panel de origen y el token (`POST /api/servers/:serverId/extransfer/pull`). El destino entonces:
   1. Llama a `POST /api/extransfer/validate` del origen con el token y su clave pública Ed25519. El origen responde con un ID de sesión y un nonce válido 3 minutos.
   2. Firma nonce + ID de sesión y llama a `POST /api/extransfer/consume`. El origen comprueba la firma, marca la sesión como `MIGRATING` y empieza a comprimir la carpeta del servidor (el servidor de origen sigue encendido). El paquete se escribe como `.transfer-partial.tar.gz` y solo se renombra a `transfer.tar.gz` cuando está completo, de modo que nunca se envía un paquete a medio escribir.
   3. Envía una señal de vida firmada cada 20 segundos (`/api/extransfer/heartbeat`). Cada una amplía la caducidad de la sesión 5 minutos, hasta un máximo de **6 horas** desde su creación. Si el origen responde que la sesión se canceló o falló, el destino se detiene.
   4. Consulta la descarga firmada (`GET /api/extransfer/download`) cada 5 segundos hasta que el paquete está listo, y lo envía a su propio nodo. Mientras se está creando, el origen responde `503` `NOT_READY` (`retryable: true`) y el destino vuelve a preguntar; espera como máximo **2 horas**, y antes se detiene si una señal de vida indica que el origen canceló o falló.
   5. Lo descomprime en la carpeta del servidor, borra el archivo y llama a `POST /api/extransfer/confirm` (firmado). El origen marca la sesión como `COMPLETED` y borra su archivo.
3. **Cancelación.** Cualquiera de los dos lados puede cancelar. El destino firma `POST /api/extransfer/cancel` en el origen; el propietario en el origen cancela desde su panel y el destino se entera en la siguiente señal de vida. Una importación cancelada o fallida borra el archivo parcial.
4. **Caducidad.** El origen termina las sesiones cuya caducidad pasó sin que acabaran (el destino se cayó, perdió la red o no pudo confirmar): al arrancar y cada 10 minutos las marca como `FAILED`, registra `EXPIRE` y, si el paquete ya se había hecho, envía un reporte de fallo y lo borra. El paquete se queda mientras otra transferencia del mismo servidor lo esté enviando o una importación a ese servidor esté en marcha en este panel: lo borra el final de esa transferencia.

Todas las llamadas entre paneles posteriores al token van firmadas con la clave Ed25519 del destino (`<ACCIÓN>:<session_id>`), de modo que solo el panel que validó el token puede continuar la sesión. Los estados de la sesión son `CREATED`, `VALIDATED`, `MIGRATING`, `COMPLETED`, `FAILED` y `CANCELLED`. La versión del protocolo es `1.0`: el destino la envía en `protocol_version` al validar y el origen rechaza cualquier otra con `400` `UNSUPPORTED_PROTOCOL`. La clave de firma de cada panel se genera una vez y se guarda en `panel.exTransferKey`, así que un reinicio no invalida una sesión en curso.

El registro de actividad anota `extransfer.create`, `extransfer.pull` y `extransfer.cancel`.

## Uso

### Desde el panel

**Transferencia entre nodos**

1. Se abre el servidor y se entra en **Administración › Transferencia**.
2. Se pulsa **Transferir**, se elige el **Nodo de destino** y se pulsa **Iniciar transferencia**.
3. El panel muestra *Transferencia iniciada.* Cuando termina la copia, el servidor, sus subservidores, sus tareas programadas y sus copias de seguridad están en el nodo nuevo, y los servidores que estaban encendidos se inician allí. Si la transferencia falla, se deshace y la **Actividad** del servidor indica el motivo.

**Migración entre paneles**

En el panel de origen:

1. Se abre el servidor, se entra en la pestaña **Migración** y se queda en **Exportar**.
2. Se pulsa **Generar token**. El panel muestra la **URL de este panel**, el **Token de migración** y una cuenta atrás (**Caduca en**). El token sigue visible durante 15 minutos aunque se salga de la página (se guarda en el almacenamiento local del navegador).
3. Se entregan la URL y el token al proveedor de destino. Mientras dura la transferencia, la pestaña muestra **Transferencia en curso** y la **Última señal del destino**; al final, **Migración completada**.

En el panel de destino:

1. Se crea el servidor que recibirá los datos (mismo juego y plantilla), se abre y se entra en **Migración › Importar**.
2. Se introducen la **URL del panel de origen** y el **Token de migración** y se pulsa **Iniciar importación**.
3. La pestaña muestra el paso actual hasta que la migración termina, falla o se cancela.

Ambos lados ofrecen **Cancelar transferencia** mientras hay una transferencia activa.

### Mediante la API

| Método | Ruta | Permiso | Finalidad |
|--------|------|---------|-----------|
| POST | `/api/servers/:serverId/transfer` | `server.data.edit.admin` | Transferencia entre nodos del servidor y sus subservidores. Cuerpo `{"nodeId": 2}`. Responde `202 {"msg": "Transfer started"}`. `400` `ErrTransferSubServer` para un subservidor; `409` `ErrTransferPortsTaken`, `ErrTransferRunning` o `ErrTransferBackupRunning`. |
| POST | `/api/servers/:serverId/extransfer/create` | `server.data.edit.admin` | Origen: crea una sesión. Responde `session_id`, `token` y `expires_in` (900). |
| POST | `/api/servers/:serverId/extransfer/pull` | `server.data.edit.admin` | Destino: inicia la importación. Cuerpo `{"origin_url": "https://panel.example.com", "token": "…"}`. Responde `202`. |
| GET | `/api/servers/:serverId/extransfer/status` | `server.data.edit.admin` | Ambos lados: `status` (código del paso actual o del resultado de la importación: `STEP_*`, `ERROR_*`, `DONE`, `CANCELLED`, `CANCELLED_BY_ORIGIN`), `detail` (lo que añaden algunos fallos, como `HTTP 403`), `message` (texto en inglés de un código `STEP_*` o `ERROR_*`, para un cliente que no lo conozca), `pulling` y `origin` (la sesión más reciente creada aquí, mientras está activa o hasta 15 minutos después de terminar). |
| POST | `/api/servers/:serverId/extransfer/cancel` | `server.data.edit.admin` | Cancela las sesiones de origen activas y la importación en curso de este servidor. `409 NO_ACTIVE_TRANSFER` si no hay ninguna. |
| POST | `/api/extransfer/validate` | pública (token) | La llama el destino. |
| POST | `/api/extransfer/consume` | pública (firma) | La llama el destino. |
| POST | `/api/extransfer/heartbeat` | pública (firma) | La llama el destino. |
| GET | `/api/extransfer/download` | pública (firma) | La llama el destino. |
| POST | `/api/extransfer/confirm` | pública (firma) | La llama el destino. |
| POST | `/api/extransfer/cancel` | pública (firma) | La llama el destino. |

Los errores de las rutas públicas tienen la forma `{"error_code", "message", "retryable"}` (el tipo también tiene un campo `session_id`, pero ningún error lo rellena). Véase la [referencia de la API: transferencia externa](../reference/api/08-external-transfer.md).

## Requisitos y configuración

| Clave | Valor por defecto | Variable de entorno | Significado |
|-------|-------------------|---------------------|-------------|
| `panel.exTransferKey` | se genera en el primer uso | `SKYPANEL_PANEL_EXTRANSFERKEY` | Semilla de la clave de firma Ed25519 de este panel. Se escribe sola en el archivo de configuración; no debe cambiarse durante una transferencia. |
| `panel.notifications.discordWebhookExTransfer` | vacío | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKEXTRANSFER` | Webhook de Discord que recibe un informe cuando una migración termina, falla o se cancela. Se edita en **Configuración › Notificaciones Discord** (**Webhook de ExTransfer**). El informe se redacta en el idioma de las notificaciones (`panel.settings.notificationLanguage`, véase [Ajustes del panel](panel-settings.md)). |

Véase [Configuración](../backend/02-configuration.md).

Otros requisitos:

- Ambas acciones necesitan `server.data.edit.admin` sobre el servidor, que el propietario no tiene por defecto (véase la [referencia de permisos](../general/scopes-reference.md)).
- Transferencia entre nodos: ambos nodos encendidos y accesibles desde el panel; disco libre suficiente en el nodo de origen para el archivo comprimido y en el de destino para el archivo, los archivos descomprimidos y las copias de seguridad. Ambos nodos deben tener una versión con las rutas `/daemon/server/:id/scheduler/pause` y `/scheduler/resume`: con un daemon más antiguo en cualquiera de los dos lados, la transferencia falla al pausar allí las tareas y se deshace (véase [Solución de problemas](../setup/troubleshooting.md)). Para recibir copias de seguridad, el nodo de destino también necesita la ruta `/daemon/server/:id/backup/file`, que tienen todas las versiones con las rutas de tareas. Si el servidor tiene un ajuste propio de copia periódica, el nodo de destino necesita además la ruta `/daemon/server/:id/offsite`.
- Migración: el panel de origen debe ser accesible desde el de destino por HTTP(S) en una dirección **pública**. Se rechazan localhost y las IP privadas o de loopback. La URL solo puede ser `http(s)://host[:puerto]` (sin ruta); si no lleva esquema, se supone `https://`.

## Límites

- **Transferencia entre nodos**
  - No hay indicador de progreso: el resultado aparece en el registro de actividad del servidor (si falla) y en el registro del panel.
  - Una transferencia interrumpida por un reinicio del panel no continúa: se deshace, o se completa si el panel ya apuntaba al nodo nuevo. La recuperación es una sola pasada al arrancar el panel; un nodo que no responda en ese momento conserva lo que quedó en él (una copia en el nodo nuevo, un archivo comprimido o las tareas pausadas en el antiguo), y el registro del panel indica cada llamada fallida. Conviene revisar ambos nodos antes de volver a intentarlo.
  - El detalle de una entrada de recuperación en el registro de actividad (`the panel restarted while …`) está en inglés.
  - Las tareas programadas se pausan en ambos nodos mientras se traslada el servidor, así que las ejecuciones que tocan entretanto (y **Ejecutar ahora**) se omiten y no se recuperan después. Una tarea que ya se esté ejecutando cuando empieza la transferencia no se interrumpe. Las tareas se copian, pero no los ajustes del planificador del archivo `.cron` (zona horaria, límite de concurrencia): el nodo nuevo usa sus valores por defecto (véase [Tareas programadas](scheduled-tasks.md)).
  - Solo se traslada un nivel de subservidores con el servidor: no se buscan subservidores de un subservidor.
  - Los servidores no están disponibles desde que se detienen hasta que se inician en el nodo nuevo.
- **Migración entre paneles**
  - Solo viajan los archivos. El servidor de destino conserva su plantilla, variables, recursos, usuarios y copias de seguridad; hay que configurarlos para que coincidan.
  - Los archivos del paquete sobrescriben los del destino que tienen el mismo nombre; el resto de archivos existentes se conserva. La importación no detiene el servidor de destino ni cambia su configuración, como indica el aviso de la pestaña **Importar**: conviene detenerlo y hacer antes una copia de seguridad (véase [Copias de seguridad](backups.md)).
  - El servidor de origen no se detiene mientras se comprime, así que un mundo en marcha puede copiarse a medio guardar, y tampoco se borra tras la migración.
  - El origen debe tener listo el paquete en las 2 horas posteriores al primer intento de descarga (que empieza 5 segundos después del consume); si no, la importación falla.
  - La API de estado devuelve cada paso de la importación como un código que la página traduce al idioma de quien la mira; los informes de Discord del destino usan el idioma de las notificaciones del panel, y el motivo que envía al origen cuando abandona va en inglés.
  - El archivo comprimido se escribe dentro de la carpeta del servidor de origen y cuenta para su disco hasta que la transferencia se confirma, se cancela o caduca. Si el nodo está desconectado cuando se borra, el archivo se queda: borra `transfer.tar.gz` de los archivos del servidor a mano.

## Problemas conocidos y soluciones

| Mensaje | Lado | Causa | Solución |
|---------|------|-------|----------|
| `cannot transfer to the same node` | Panel | El nodo de destino es el actual. | Elegir otro nodo. |
| `ports … of this server are already used on the target node: free them or pick another node` (`409`, `ErrTransferPortsTaken`) | Panel | Otro servidor, el propio nodo o un contenedor usa esos puertos en el nodo de destino. | Liberarlos allí, cambiar los puertos del servidor o elegir otro nodo. |
| `Un subservidor se queda en el nodo de su servidor principal…` (`400`, `ErrTransferSubServer`) | Panel | El servidor es un subservidor de Splitter. | Transferir el servidor principal: sus subservidores van con él. |
| `Este servidor ya se está transfiriendo…` (`409`, `ErrTransferRunning`) | Panel | Hay en curso una transferencia de este servidor o de uno de sus subservidores. | Esperar a que termine. |
| `Hay una copia de seguridad o una restauración de este servidor en curso…` (`409`, `ErrTransferBackupRunning`) | Panel | El nodo antiguo está escribiendo una copia, un punto de restauración o una restauración de alguno de los servidores. | Transferir cuando termine. |
| El servidor sigue en el nodo antiguo; su actividad muestra `server.transferFailed` | Panel | Un paso falló y la transferencia se deshizo. El detalle indica el servidor, el paso y la respuesta del nodo. | Corregir la causa (disco, nodo apagado, nodo de destino demasiado antiguo para recibir copias) y repetir. El registro del panel contiene `Transfer of … failed and was undone`. |
| La actividad muestra `server.transferFailed` con `the panel restarted while <paso>: the transfer was undone` | Panel | El panel se detuvo durante la transferencia y la deshizo al volver a arrancar. | Revisar en el registro del panel las llamadas de limpieza que fallaron (por ejemplo, un nodo que estaba apagado) y volver a transferir. |
| `reading its off-site settings` / `copying its off-site settings` en el detalle del fallo | Panel | No se pudo leer el ajuste de copia periódica en el nodo antiguo o guardarlo en el nuevo (por ejemplo, un nodo de destino sin la ruta `/daemon/server/:id/offsite`). | Actualizar el nodo y volver a intentarlo. |
| `pausing its tasks on the old node` / `on the new node` en el detalle del fallo | Panel | Ese nodo tiene un daemon sin la ruta para pausar las tareas. | Actualizar el nodo (véase [Solución de problemas](../setup/troubleshooting.md)) y volver a intentarlo. |
| El servidor se trasladó pero está detenido; su actividad muestra `server.transferStartFailed` | Panel | No se pudo iniciar en el nodo nuevo. | Leer el motivo en el detalle y en la consola del servidor e iniciarlo a mano. |
| `La URL de origen no es válida o apunta a una dirección no permitida` (`ERROR_ORIGIN_URL`) | Destino | URL con ruta, credenciales, IP privada o localhost. | Usar la dirección pública `https://host[:puerto]` del panel de origen. |
| `Fallo de red al conectar con origen` (`ERROR_ORIGIN_UNREACHABLE`) | Destino | El origen no es accesible. | Revisar DNS, cortafuegos y TLS del panel de origen. |
| `Token inválido o expirado (HTTP 403)` (`ERROR_TOKEN_REJECTED`) | Destino | Token mal copiado, ya usado, cancelado o con más de 15 minutos. | Generar un token nuevo en el origen. |
| `El origen rechazó la transferencia` (`ERROR_ORIGIN_REFUSED`) | Destino | Se rechazó el consume firmado (nonce de más de 3 minutos, sesión que no está en `VALIDATED`). | Empezar de nuevo con otro token. |
| `El origen no preparó el paquete a tiempo (2h0m0s)` (`ERROR_DOWNLOAD_TIMEOUT`) | Destino | Comprimir el servidor de origen tardó más de 2 horas. | Reducir el tamaño del servidor (copias antiguas, registros) y reintentar. |
| `El origen rechazó la descarga (HTTP …)` (`ERROR_DOWNLOAD_REFUSED`) | Destino | La sesión se canceló, falló o caducó en el origen. | Revisar la pestaña **Exportar** del origen y generar un token nuevo. |
| `Error al subir archivo al daemon` (`ERROR_UPLOAD`) / `Error al descomprimir en el destino (…)` (`ERROR_EXTRACT`) | Destino | El nodo de destino rechazó el archivo (límite de disco, nodo apagado). | Liberar disco en el servidor o el nodo de destino. |
| `A transfer is already running for this server` (`409`) | Destino | Ya hay una importación en curso. | Esperar a que termine o cancelarla. |
| `There is no transfer in progress for this server` (`409 NO_ACTIVE_TRANSFER`) | Ambos | No hay nada que cancelar. | — |
| `UNSUPPORTED_PROTOCOL` (`400`) | Origen (rutas públicas) | El destino usa una versión del protocolo distinta de `1.0`. | Actualizar ambos paneles a una versión compatible. |
| `TOKEN_ERROR` / `CONSUME_ERROR` / `INVALID_SIGNATURE` / `SESSION_EXPIRED` | Origen (rutas públicas) | Token no válido, firma incorrecta o sesión caducada. | Iniciar una migración nueva. |

## Véase también

- [Conceptos básicos: transferencia externa](../general/concepts.md)
- [Subservidores](sub-servers.md)
- [Clonar servidores](cloning.md) (una copia en el mismo nodo)
- [Referencia de la API](../reference/11-api-reference.md)
- [Configuración](../backend/02-configuration.md)
