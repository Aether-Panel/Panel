# Transferencia externa

Rutas que copian los archivos de un servidor de un Aether Panel a otro independiente (también llamada transferencia federada o "ExTransfer"). El panel del que sale el servidor es el **origen**; el panel al que llega es el **destino**, que trae los archivos a un servidor que ya existe en él. Lo que ve el usuario, los límites y la resolución de problemas están en [Transferencias de servidores](../../features/server-transfers.md); mover un servidor entre nodos del mismo panel es [`POST /api/servers/:serverId/transfer`](./03-servers.md#post-apiserversserveridtransfer). Volver a la [referencia de la API](../11-api-reference.md).

Código: `internal/web/api/extransfer_logic.go` (controladores), `internal/web/api/servers.go` (rutas por servidor), `internal/models/extransfer.go` (sesiones).

## Rutas

### En un servidor (con sesión iniciada)

Las llama un usuario o un script en su propio panel. Los dos lados usan las mismas rutas: el origen crea el token y el destino inicia la importación.

| Método | Ruta | Permiso | Qué hace |
|--------|------|---------|----------|
| POST | `/api/servers/:serverId/extransfer/create` | `server.data.edit.admin` | Origen: abre una sesión y devuelve un token de un solo uso |
| POST | `/api/servers/:serverId/extransfer/pull` | `server.data.edit.admin` | Destino: empieza a importar desde el panel de origen |
| GET | `/api/servers/:serverId/extransfer/status` | `server.data.edit.admin` | Progreso de la importación en este panel y la sesión más reciente creada aquí |
| POST | `/api/servers/:serverId/extransfer/cancel` | `server.data.edit.admin` | Cancela la importación en curso aquí y las sesiones vivas creadas aquí |

`server.data.edit.admin` no forma parte de lo que recibe por defecto el propietario de un servidor (véase la [referencia de permisos](../../general/scopes-reference.md)).

### Entre paneles (públicas)

Las llama el panel de destino en el panel de origen. No necesitan sesión: la primera recibe el token y las demás una firma hecha con la clave del destino.

| Método | Ruta | Autenticación | Qué hace |
|--------|------|---------------|----------|
| POST | `/api/extransfer/validate` | Token | Comprueba el token y registra la clave pública del destino |
| POST | `/api/extransfer/consume` | Firma sobre el nonce | Inicia la transferencia: el origen comprime la carpeta del servidor |
| POST | `/api/extransfer/heartbeat` | Firma | Mantiene viva la sesión e indica al destino si debe seguir |
| GET | `/api/extransfer/download` | Firma | Envía el archivo comprimido en flujo continuo |
| POST | `/api/extransfer/confirm` | Firma | El destino ya tiene los datos: la sesión se completa |
| POST | `/api/extransfer/cancel` | Firma | El destino abandona: la sesión se cancela |

Estas peticiones no llevan cookies, así que la [protección de mismo origen](./02-authentication.md#protección-de-mismo-origen) no se les aplica. No tienen un límite de peticiones propio.

## Cómo se fían los paneles entre sí

### El token

`create` genera 32 bytes aleatorios y los devuelve en hexadecimal (64 caracteres). El origen solo guarda un HMAC-SHA256 del token, así que no se puede leer de la base de datos. El token vale **15 minutos** y lo gasta el primer `validate` correcto: después, la sesión ya no está en un estado que `validate` acepte. La única excepción es una sesión que ha pasado a `FAILED` (el origen no pudo crear el archivo comprimido): se puede validar de nuevo con el mismo token mientras la sesión no caduque.

Crear un token no revoca los anteriores; cada `create` abre una sesión aparte.

### La clave del destino

Cada panel tiene una clave Ed25519 cuya semilla se guarda en `panel.exTransferKey` (se genera la primera vez que hace falta y se escribe en el archivo de configuración, de modo que un reinicio a mitad de una transferencia no rompe la sesión). En `validate` el destino envía su clave pública; a partir de ahí el origen solo acepta llamadas firmadas con la clave privada correspondiente, así que un token interceptado después de la validación no sirve de nada.

Qué se firma (claves y firmas van en base64 estándar):

| Llamada | Mensaje firmado |
|---------|-----------------|
| `consume` | El nonce de `validate` seguido del ID de sesión, sin nada entre ambos |
| `heartbeat` | `HEARTBEAT:<session_id>` |
| `download` | `DOWNLOAD:<session_id>` |
| `confirm` | `CONFIRM:<session_id>` |
| `cancel` | `CANCEL:<session_id>` |

La firma de `DOWNLOAD:<session_id>` no cambia, así que la misma URL de descarga sirve para todos los intentos mientras la sesión esté en migración.

### Cómo se llega al origen

El destino solo se comunica con la URL de origen indicada en `pull`, que debe tener la forma `http(s)://host[:puerto]` (sin ruta, consulta, credenciales ni fragmento; sin esquema se supone `https://`). Se rechazan `localhost` y las direcciones privadas, de loopback o sin especificar, tanto en la URL como cuando el nombre de host se resuelve. Cada llamada tiene 10 segundos para conectar y un límite total de 30 segundos, salvo la descarga, que no tiene límite total (la acotan la sesión y las señales de vida) pero debe empezar a responder en menos de un minuto.

## Estados y plazos de la sesión

| Estado | Significado |
|--------|-------------|
| `CREATED` | Token generado, aún sin usar |
| `VALIDATED` | Token aceptado y clave del destino guardada; espera a `consume` |
| `MIGRATING` | Se está creando y descargando el archivo comprimido |
| `COMPLETED` | El destino confirmó que tiene los datos |
| `FAILED` | El origen no pudo comprimir el servidor |
| `CANCELLED` | Cancelada por cualquiera de los dos lados |

| Plazo | Valor |
|-------|-------|
| Vida del token y de la sesión | 15 minutos desde `create` |
| Nonce | 3 minutos desde `validate` |
| Señal de vida | Cada 20 segundos desde el destino; cada una lleva la caducidad a 5 minutos después, nunca más de **6 horas** tras `create`. `consume` también la amplía, así que un `consume` tardío aún tiene tiempo para la primera señal |
| Espera del archivo comprimido | El destino pide la descarga cada 5 segundos mientras el origen responda `5xx` (`NOT_READY`, `DAEMON_ERROR`) y desiste a las **2 horas**; una respuesta `4xx` la detiene en el acto, igual que un latido que informe de la sesión en `CANCELLED` o `FAILED` |
| Resultado visible en `status` | 15 minutos después de que la sesión termine o caduque |
| Paso de la importación en `status` | Se borra 10 segundos después del último paso |

## Qué se traslada

Solo los archivos de la carpeta del servidor:

1. Con `consume`, el origen borra cualquier `transfer.tar.gz` y `.transfer-partial.tar.gz` anteriores de la carpeta del servidor, comprime todo su contenido en el archivo oculto `.transfer-partial.tar.gz` en su nodo y, cuando está completo, lo renombra a `transfer.tar.gz`. Si falla la compresión o el cambio de nombre, se borra el archivo parcial y la sesión pasa a `FAILED`. El servidor de origen sigue encendido.
2. El destino lee el archivo del `download` del origen (que a su vez lo lee del nodo del origen) y lo escribe directamente en `transfer.tar.gz` de su propio nodo; después lo descomprime en la carpeta del servidor y lo borra.
3. Con `confirm` o con una cancelación, el origen borra su `transfer.tar.gz` y cualquier `.transfer-partial.tar.gz` que aún se esté escribiendo.

No viajan la plantilla, las variables, los límites, los miembros, las copias de seguridad, las bases de datos ni el subdominio, y el servidor de origen no se detiene ni se borra. Tampoco se detiene el servidor de destino; los archivos extraídos sobrescriben los que tengan el mismo nombre. Véase [Transferencias de servidores → Límites](../../features/server-transfers.md#límites).

`transfer.tar.gz` solo existe cuando el archivo comprimido está completo, así que `download` nunca envía medio archivo: hasta entonces responde `503 NOT_READY` (`retryable: true`) y el destino vuelve a preguntar.

## Rutas en un servidor

### `POST /api/servers/:serverId/extransfer/create`

Sin cuerpo. Respuesta:

```json
{ "session_id": "6f1c2a9e-0b7d-4c55-9a51-2f0e8c3d7b14", "token": "9e4f...64 caracteres hexadecimales...", "expires_in": 900 }
```

La URL del panel de origen y el `token` se entregan a quien vaya a hacer la importación. `500 {"error": "Failed to create session"}` cuando no se puede guardar.

```bash
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  https://origin.example.com/api/servers/a1b2c3d4/extransfer/create
```

### `POST /api/servers/:serverId/extransfer/pull`

Se ejecuta en el panel de destino, sobre el servidor que va a recibir los archivos.

```json
{ "origin_url": "https://origin.example.com", "token": "TRANSFER_TOKEN" }
```

Los dos campos son obligatorios (si no, `400 {"error": "Invalid JSON payload"}`); se eliminan los espacios de alrededor. La importación se ejecuta en segundo plano y la respuesta llega al momento: `202 {"status": "ACCEPTED", "message": "Pull process initiated"}`. Una URL o un token incorrectos no se indican aquí, sino en `status`. `409 {"error": "A transfer is already running for this server"}` mientras haya otra importación de este servidor en curso.

En segundo plano, el destino llama a `validate` y `consume`, envía señales de vida, espera la `download` y la envía a su nodo, la descomprime, borra el archivo y llama a `confirm`. Cuando se detiene por un error o porque su propietario canceló, llama al `cancel` del origen (si puede) con el motivo. Si `confirm` falla, solo queda en el registro: la importación termina igualmente como completada.

```bash
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"origin_url": "https://origin.example.com", "token": "TRANSFER_TOKEN"}' \
  https://destination.example.com/api/servers/e5f6a7b8/extransfer/pull
```

### `GET /api/servers/:serverId/extransfer/status`

```json
{
  "status": "STEP_EXTRACTING",
  "message": "Unpacking the files...",
  "pulling": true,
  "origin": {
    "session_id": "6f1c2a9e-0b7d-4c55-9a51-2f0e8c3d7b14",
    "status": "MIGRATING",
    "expired": false,
    "expires_at": "2026-11-02T10:20:00Z",
    "last_heartbeat_at": "2026-11-02T10:15:20Z",
    "updated_at": "2026-11-02T10:15:20Z"
  }
}
```

| Campo | Significado |
|-------|-------------|
| `status` | Paso actual o resultado de la importación en este panel (lado destino), como código, o `""` si no hay ninguna |
| `detail` | Solo en algunos fallos: lo que añaden, como el estado HTTP del origen (`HTTP 403`) o el tiempo esperado (`2h0m0s`) |
| `message` | Solo con los códigos `STEP_*` y `ERROR_*`: el paso en inglés, con su detalle, para un cliente que no conozca el código |
| `pulling` | Si hay una importación de este servidor en curso aquí |
| `origin` | Solo cuando este panel creó una sesión para el servidor (lado origen): la más reciente, mientras está viva y hasta 15 minutos después de terminar o caducar |

El cliente web traduce los códigos al idioma de quien mira la página (`servers.extransfer.progress.<código>` en sus traducciones) y añade el `detail` entre paréntesis; si no conoce un código, muestra `message`. Los pasos, por orden:

| Código | Paso |
|--------|------|
| `STEP_STARTING` | Inicio |
| `STEP_VALIDATING` | Comprobación de la conexión con el panel de origen |
| `STEP_ORIGIN_START` | Inicio de la transferencia en el origen |
| `STEP_WAITING_ARCHIVE` | Espera a que el origen empaquete los archivos |
| `STEP_DOWNLOADING` | Descarga del paquete desde el origen |
| `STEP_UPLOADING` | Subida de los archivos al nodo de este panel |
| `STEP_EXTRACTING` | Extracción de los archivos |
| `STEP_CLEANUP` | Eliminación de los archivos temporales |

El último valor es uno de estos:

| Código | Significado |
|--------|-------------|
| `DONE` | La importación terminó |
| `CANCELLED` | Se canceló en este panel |
| `CANCELLED_BY_ORIGIN` | El origen canceló, o su sesión falló |
| `ERROR_ORIGIN_URL` | La URL de origen no es válida o apunta a una dirección no permitida |
| `ERROR_ORIGIN_UNREACHABLE` | Fallo de red al contactar con el origen |
| `ERROR_TOKEN_REJECTED` | El origen rechazó el token: no válido, ya usado o caducado (`detail`: su estado HTTP) |
| `ERROR_ORIGIN_BAD_ANSWER`, `ERROR_ORIGIN_BAD_SESSION` | El origen respondió algo ilegible, o un ID de sesión no válido |
| `ERROR_ORIGIN_START`, `ERROR_ORIGIN_REFUSED` | Falló la llamada que inicia la transferencia en el origen, o el origen la rechazó |
| `ERROR_DOWNLOAD_NETWORK`, `ERROR_DOWNLOAD_REFUSED` | Error de red durante la descarga, o el origen rechazó la descarga (`detail`: su estado HTTP) |
| `ERROR_DOWNLOAD_TIMEOUT` | El origen no tuvo listo el paquete a tiempo (`detail`: el tiempo esperado) |
| `ERROR_UPLOAD`, `ERROR_EXTRACT` | No se pudo subir el paquete al nodo de este panel, o extraerlo allí (`detail`: la respuesta del nodo) |

Qué hacer ante cada fallo: [Transferencias de servidores → Problemas conocidos y soluciones](../../features/server-transfers.md#problemas-conocidos-y-soluciones). El motivo que se envía al origen con `cancel` es el texto en inglés del fallo, sea cual sea el idioma de cada panel; los informes de transferencias de Discord usan el [idioma de las notificaciones](./06-panel-admin.md#configuración) de este panel.

### `POST /api/servers/:serverId/extransfer/cancel`

Sin cuerpo. Detiene la importación de este servidor que esté en curso en este panel y cancela todas las sesiones del servidor creadas en este panel que estén en `CREATED`, `VALIDATED`, `MIGRATING` o `FAILED` y no hayan caducado. Las descargas en curso de esas sesiones se cortan y se borra el archivo comprimido del origen. El destino se entera en su siguiente señal de vida.

```json
{ "cancelled_pull": false, "cancelled_sessions": 1 }
```

`409 {"error_code": "NO_ACTIVE_TRANSFER", "message": "There is no transfer in progress for this server", "retryable": false}` cuando no había nada que cancelar; `500 INTERNAL_ERROR` cuando no se pudieron actualizar las sesiones.

## Rutas entre paneles

### Errores

Estas rutas responden los errores con su propio formato, no con el [formato de error](./01-conventions.md#formato-de-error) habitual del panel:

```json
{ "error_code": "INVALID_SIGNATURE", "message": "Invalid signature", "retryable": false }
```

`retryable` es `true` en los errores de base de datos (`INTERNAL_ERROR`), en `DAEMON_ERROR` y en `NOT_READY`, y `false` en todo lo demás. La estructura define también un campo `session_id`, pero ninguna ruta lo rellena.

Errores comunes a `heartbeat`, `download`, `confirm` y `cancel`:

| Estado | `error_code` | Cuándo |
|--------|--------------|--------|
| `400` | `BAD_REQUEST` | El cuerpo no es JSON o falta `session_id` o `signature` (en `download`, falta alguno de los dos parámetros de la consulta) |
| `404` | `NOT_FOUND` | Sesión desconocida |
| `400` | `INVALID_SIGNATURE_FORMAT` | La firma o la clave guardada no son base64 válido, o la clave no es una clave pública Ed25519 |
| `403` | `INVALID_SIGNATURE` | La firma no corresponde a la clave dada en `validate` |
| `403` | `SESSION_EXPIRED` | La sesión caducó |
| `500` | `INTERNAL_ERROR` | Error de base de datos (`retryable: true`) |

### `POST /api/extransfer/validate`

```json
{ "token": "TRANSFER_TOKEN", "target_public_key": "BASE64_ED25519_PUBLIC_KEY", "protocol_version": "1.0" }
```

Los tres campos son obligatorios; se eliminan los espacios alrededor del token. `protocol_version` debe ser `1.0`, la única versión que habla este panel (es la que envía cuando actúa como destino). La sesión debe estar en `CREATED` (o en `FAILED`, véase [El token](#el-token)) y sin caducar. El origen genera un nonce válido 3 minutos, guarda la clave y pasa la sesión a `VALIDATED`.

```json
{
  "session_id": "6f1c2a9e-0b7d-4c55-9a51-2f0e8c3d7b14",
  "nonce": "0c9d8e7f-1a2b-4c3d-8e9f-a0b1c2d3e4f5",
  "capabilities_required": { "virtualization": "docker", "cpu_cores_min": 1, "ram_mb_min": 1024 }
}
```

`capabilities_required` es un valor fijo que el destino no comprueba. El formato de la clave solo se valida en `consume`.

| Estado | Respuesta |
|--------|-----------|
| `400` | `BAD_REQUEST`: falta un campo o no es JSON |
| `400` | `UNSUPPORTED_PROTOCOL`: `protocol_version` no es `1.0` (`retryable: false`) |
| `403` | `TOKEN_ERROR`, con `message` `not_found` (token desconocido), `invalid_state` (ya usado o cancelado) o `expired` |

```bash
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"token": "TRANSFER_TOKEN", "target_public_key": "BASE64_ED25519_PUBLIC_KEY", "protocol_version": "1.0"}' \
  https://origin.example.com/api/extransfer/validate
```

### `POST /api/extransfer/consume`

```json
{ "session_id": "6f1c2a9e-0b7d-4c55-9a51-2f0e8c3d7b14", "signature": "BASE64_SIGNATURE" }
```

`signature` firma el nonce seguido del ID de sesión. La sesión debe estar en `VALIDATED` y el nonce tener menos de 3 minutos. La sesión pasa a `MIGRATING` y el origen empieza a comprimir la carpeta del servidor en segundo plano. Respuesta: `202 {"status": "MIGRATING", "message": "Stream initialized"}`.

Si la compresión falla, la sesión pasa a `FAILED`; si la sesión se canceló mientras se creaba el archivo, este se borra.

| Estado | Respuesta |
|--------|-----------|
| `400` | `BAD_REQUEST` |
| `403` | `CONSUME_ERROR`, con `message` `invalid_state` (sesión desconocida o no `VALIDATED`), `nonce_expired`, `invalid_signature_format` o `invalid_signature` |

### `POST /api/extransfer/heartbeat`

Cuerpo: `session_id` y `signature` (`HEARTBEAT:<session_id>`). Con una sesión en `MIGRATING`, anota la señal y amplía la caducidad:

```json
{ "status": "MIGRATING", "expires_at": "2026-11-02T10:20:20Z" }
```

Una sesión `CANCELLED` o `FAILED` responde `200` con ese `status`, lo que indica al destino que se detenga. Cualquier otro estado responde `403 INVALID_STATE` ("Session not in migrating state").

### `GET /api/extransfer/download?session_id=...&signature=...`

`signature` firma `DOWNLOAD:<session_id>` y debe ir codificada para URL. La sesión debe estar en `MIGRATING` y sin caducar (`403 INVALID_STATE` / `SESSION_EXPIRED`). La respuesta envía el archivo comprimido como `application/octet-stream` con `Content-Disposition: attachment; filename=transfer.tar.gz`.

`503 NOT_READY` (`retryable: true`) mientras el archivo aún se está creando (el nodo todavía no tiene `transfer.tar.gz`); `500 DAEMON_ERROR` (`retryable: true`) cuando no se puede contactar con el nodo del origen o este responde con otro error. En ambos casos el destino sigue preguntando. `500 INTERNAL_ERROR` (`retryable: false`) cuando el servidor ya no existe en el origen. Una cancelación corta la descarga en curso.

### `POST /api/extransfer/confirm`

Cuerpo: `session_id` y `signature` (`CONFIRM:<session_id>`). La sesión debe estar en `MIGRATING`; pasa a `COMPLETED`, ya no se puede descargar y el origen borra su archivo comprimido. Confirmar de nuevo una sesión `COMPLETED` también responde `200`. Respuesta: `{"status": "COMPLETED"}`. Otros estados: `403 INVALID_STATE`.

### `POST /api/extransfer/cancel`

```json
{ "session_id": "6f1c2a9e-0b7d-4c55-9a51-2f0e8c3d7b14", "signature": "BASE64_SIGNATURE", "reason": "Disk full on the destination" }
```

`signature` firma `CANCEL:<session_id>`; `reason` es opcional y se corta a 500 caracteres. Se permite mientras la sesión esté en `VALIDATED`, `MIGRATING` o `FAILED`; cancelar de nuevo una sesión `CANCELLED` también responde `200`. El origen detiene las descargas en curso y borra su archivo comprimido. Respuesta: `{"status": "CANCELLED"}`. Las sesiones `CREATED` y `COMPLETED` responden `403 INVALID_STATE` ("Session can no longer be cancelled").

## Registros y notificaciones

- **Registro de actividad** del servidor: `extransfer.create`, `extransfer.pull` y `extransfer.cancel` para las rutas en un servidor (véase [`GET /api/servers/:serverId/activity`](./03-servers.md#get-apiserversserveridactivity)).
- **Registro de transferencias** (una tabla de la base de datos del origen): `CONFIRM`, `CANCEL` (con el motivo del destino) y `CANCEL_BY_OWNER`, cada uno con la IP de quien llamó. No hay ninguna ruta de la API para leerlo.
- **Discord**: si `panel.notifications.discordWebhookExTransfer` está configurado, los dos paneles publican un informe cuando una transferencia se completa, falla o se cancela. Los textos del informe están en español. Véase [Transferencias de servidores → Requisitos y configuración](../../features/server-transfers.md#requisitos-y-configuración).
