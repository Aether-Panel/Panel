# Transferencias de servidores

Aether Panel puede mover los archivos de un servidor de dos maneras:

- **Transferencia entre nodos**: mueve un servidor a otro nodo del **mismo panel**. El servidor conserva su identificador, sus usuarios y su configuración; solo cambia la máquina en la que se ejecuta.
- **Migración entre paneles** (transferencia federada o externa, "ExTransfer"): copia los archivos de un servidor desde **otra instancia de Aether Panel**, por ejemplo cuando un cliente se cambia de proveedor de hosting.

## Qué hace

| | Transferencia entre nodos | Migración entre paneles |
|---|---|---|
| Alcance | Dos nodos de un mismo panel | Dos paneles independientes |
| Qué se traslada | La definición del servidor y todos los archivos | Solo los archivos (un archivo comprimido de la carpeta del servidor) |
| Dónde se inicia | En el servidor de origen (**Administración › Transferencia**) | En ambos lados: un token en el origen y una importación en el destino (pestaña **Migración**) |
| Se detiene el servidor antes | Sí (se detiene y, si no para, se fuerza) | No |
| Progreso en la interfaz | No (solo en el registro del panel) | Sí (paso actual, señal de vida y resultado) |
| Se puede cancelar | No | Sí, desde cualquiera de los dos lados |

## Cómo funciona

### Transferencia entre nodos

Antes de empezar, el panel comprueba que los puertos del servidor (el principal y los adicionales) estén libres en el nodo de destino: no deben usarlos otros servidores de ese nodo, los puertos del propio nodo (panel, daemon, SFTP) ni contenedores publicados allí. Si alguno está ocupado, responde `409` con `ErrTransferPortsTaken` y la lista de puertos, y no se mueve nada. Si están libres, `POST /api/servers/:serverId/transfer` responde `202` al momento y el panel ejecuta estos pasos en segundo plano:

1. Lee la definición del servidor en el nodo actual y crea el servidor con ella en el nodo de destino.
2. Detiene el servidor en el nodo actual y espera; si sigue encendido, lo fuerza a parar. Si no consigue detenerlo, la transferencia se cancela.
3. Comprime todos los archivos en `transfer.tar.gz` en el nodo actual.
4. Envía el archivo del nodo actual al de destino en flujo continuo, sin guardarlo en el panel.
5. Lo descomprime en la carpeta del servidor del nodo de destino y borra allí el archivo.
6. Apunta el registro del servidor en la base de datos al nodo de destino y actualiza su subdominio para que siga a la dirección del nodo nuevo.
7. Pide al nodo antiguo que borre el servidor, conservando sus copias de seguridad (`?keepBackups=true`): las copias **no** se trasladan al nodo nuevo y siguen en el disco del nodo antiguo.

El servidor no se vuelve a iniciar en el nodo de destino; hay que iniciarlo al terminar. El registro de actividad anota `server.transfer`.

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

Todas las llamadas entre paneles posteriores al token van firmadas con la clave Ed25519 del destino (`<ACCIÓN>:<session_id>`), de modo que solo el panel que validó el token puede continuar la sesión. Los estados de la sesión son `CREATED`, `VALIDATED`, `MIGRATING`, `COMPLETED`, `FAILED` y `CANCELLED`. La versión del protocolo es `1.0`: el destino la envía en `protocol_version` al validar y el origen rechaza cualquier otra con `400` `UNSUPPORTED_PROTOCOL`. La clave de firma de cada panel se genera una vez y se guarda en `panel.exTransferKey`, así que un reinicio no invalida una sesión en curso.

El registro de actividad anota `extransfer.create`, `extransfer.pull` y `extransfer.cancel`.

## Uso

### Desde el panel

**Transferencia entre nodos**

1. Se abre el servidor y se entra en **Administración › Transferencia**.
2. Se pulsa **Transferir**, se elige el **Nodo de destino** y se pulsa **Iniciar transferencia**.
3. El panel muestra *Transferencia iniciada.* El servidor desaparece del nodo antiguo cuando termina la copia.

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
| POST | `/api/servers/:serverId/transfer` | `server.data.edit.admin` | Transferencia entre nodos. `409` `ErrTransferPortsTaken` si algún puerto del servidor está ocupado en el destino. Cuerpo `{"nodeId": 2}`. Responde `202 {"msg": "Transfer started"}`. |
| POST | `/api/servers/:serverId/extransfer/create` | `server.data.edit.admin` | Origen: crea una sesión. Responde `session_id`, `token` y `expires_in` (900). |
| POST | `/api/servers/:serverId/extransfer/pull` | `server.data.edit.admin` | Destino: inicia la importación. Cuerpo `{"origin_url": "https://panel.example.com", "token": "…"}`. Responde `202`. |
| GET | `/api/servers/:serverId/extransfer/status` | `server.data.edit.admin` | Ambos lados: `status` (paso actual de la importación), `pulling` y `origin` (la sesión más reciente creada aquí, mientras está activa o hasta 15 minutos después de terminar). |
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
| `panel.notifications.discordWebhookExTransfer` | vacío | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKEXTRANSFER` | Webhook de Discord que recibe un informe cuando una migración termina, falla o se cancela. Se edita en **Configuración › Notificaciones Discord** (**Webhook de ExTransfer**). |

Véase [Configuración](../backend/02-configuration.md).

Otros requisitos:

- Ambas acciones necesitan `server.data.edit.admin` sobre el servidor, que el propietario no tiene por defecto (véase la [referencia de permisos](../general/scopes-reference.md)).
- Transferencia entre nodos: ambos nodos encendidos y accesibles desde el panel; disco libre suficiente en el nodo de origen para el archivo comprimido y en el de destino para el archivo y los archivos descomprimidos.
- Migración: el panel de origen debe ser accesible desde el de destino por HTTP(S) en una dirección **pública**. Se rechazan localhost y las IP privadas o de loopback. La URL solo puede ser `http(s)://host[:puerto]` (sin ruta); si no lleva esquema, se supone `https://`.

## Límites

- **Transferencia entre nodos**
  - No hay indicador de progreso ni marcha atrás automática. Si un paso falla, el error solo va al registro del panel; el servidor puede quedar creado en el nodo de destino mientras sigue funcionando desde el de origen, o quedarse detenido tras el paso 2.
  - Solo se mueve el servidor elegido. Los subservidores se quedan en su nodo (véase [Subservidores](sub-servers.md)).
  - Las copias de seguridad no se mueven: siguen en el nodo antiguo y la pestaña **Copias de seguridad** del servidor ya no las muestra. Antes de transferir, descargue las que quiera conservar (véase [Copias de seguridad](backups.md)).
- **Migración entre paneles**
  - Solo viajan los archivos. El servidor de destino conserva su plantilla, variables, recursos, usuarios y copias de seguridad; hay que configurarlos para que coincidan.
  - Los archivos del paquete sobrescriben los del destino que tienen el mismo nombre; el resto de archivos existentes se conserva. La importación no detiene el servidor de destino ni cambia su configuración, como indica el aviso de la pestaña **Importar**: conviene detenerlo y hacer antes una copia de seguridad (véase [Copias de seguridad](backups.md)).
  - El servidor de origen no se detiene mientras se comprime, así que un mundo en marcha puede copiarse a medio guardar, y tampoco se borra tras la migración.
  - El origen debe tener listo el paquete en las 2 horas posteriores al primer intento de descarga (que empieza 5 segundos después del consume); si no, la importación falla.
  - Los mensajes de los pasos de la importación los genera el backend del destino en español, sea cual sea el idioma de la interfaz.
  - El archivo comprimido se escribe dentro de la carpeta del servidor de origen y cuenta para su disco.

## Problemas conocidos y soluciones

| Mensaje | Lado | Causa | Solución |
|---------|------|-------|----------|
| `cannot transfer to the same node` | Panel | El nodo de destino es el actual. | Elegir otro nodo. |
| `ports … of this server are already used on the target node: free them or pick another node` (`409`, `ErrTransferPortsTaken`) | Panel | Otro servidor, el propio nodo o un contenedor usa esos puertos en el nodo de destino. | Liberarlos allí, cambiar los puertos del servidor o elegir otro nodo. |
| La transferencia entre nodos no termina; el servidor sigue en el nodo antiguo | Panel | Falló un paso en segundo plano. | Buscar `Failed to …` / `Transfer of … cancelled` en el registro del panel, corregir el problema del nodo y repetir. |
| `ERROR: La URL de origen no es válida o apunta a una dirección no permitida` | Destino | URL con ruta, credenciales, IP privada o localhost. | Usar la dirección pública `https://host[:puerto]` del panel de origen. |
| `ERROR: Fallo de red al conectar con origen` | Destino | El origen no es accesible. | Revisar DNS, cortafuegos y TLS del panel de origen. |
| `ERROR: Token inválido o expirado (Status 403)` | Destino | Token mal copiado, ya usado, cancelado o con más de 15 minutos. | Generar un token nuevo en el origen. |
| `ERROR: El origen rechazó la transferencia` | Destino | Se rechazó el consume firmado (nonce de más de 3 minutos, sesión que no está en `VALIDATED`). | Empezar de nuevo con otro token. |
| `ERROR: El origen no preparó el paquete a tiempo (timeout 2h0m0s)` | Destino | Comprimir el servidor de origen tardó más de 2 horas. | Reducir el tamaño del servidor (copias antiguas, registros) y reintentar. |
| `ERROR: El origen rechazó la descarga (Status …)` | Destino | La sesión se canceló, falló o caducó en el origen. | Revisar la pestaña **Exportar** del origen y generar un token nuevo. |
| `ERROR: Error al subir archivo al daemon` / `ERROR: Error al descomprimir en el destino: …` | Destino | El nodo de destino rechazó el archivo (límite de disco, nodo apagado). | Liberar disco en el servidor o el nodo de destino. |
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
