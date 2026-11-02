# Discord y notificaciones

Aether Panel tiene dos capas de notificación independientes:

1. **Webhooks del panel** (administradores): cuatro webhooks de Discord configurados en **Configuración → Discord** que reciben avisos de servidores, informes del sistema y de los nodos, e informes de transferencias externas.
2. **Notificaciones del servidor** (propietarios): cada servidor puede enviar sus propios avisos a un webhook de Discord, a un bot de Telegram y/o a un webhook JSON genérico, configurados en la tarjeta **Notificaciones** de los ajustes del servidor.

Todas las peticiones pasan por un cliente HTTP restringido que rechaza direcciones privadas, de loopback e internas (se comprueba en la propia conexión, después de resolver el DNS), de modo que una URL de webhook nunca puede usarse para llegar a la red del propio nodo. Los mensajes se envían una sola vez: no hay reintentos y un fallo solo se registra en el log.

---

## Idioma de las notificaciones

Todo lo que el panel envía fuera de sus propias páginas se escribe en un solo idioma, configurado en **Configuración → General → Notificaciones → Idioma de las notificaciones**: los webhooks del panel que se describen más abajo (avisos, informes del sistema y de los nodos, la prueba de Discord, informes de transferencias externas), los avisos de Discord, Telegram y webhook JSON de cada servidor y su mensaje de prueba, y el título y el mensaje de los eventos que reciben las [extensiones](../reference/15-extensions.md). Las páginas del panel no dependen de este ajuste: siguen el idioma de cada usuario.

| Clave | Variable de entorno | Valores | Por defecto |
|-------|---------------------|---------|-------------|
| `panel.settings.notificationLanguage` | `SKYPANEL_PANEL_SETTINGS_NOTIFICATIONLANGUAGE` | `es`, `en` | `es` |

Guardar cualquier otro valor mediante la API de ajustes se rechaza con `400` `ErrNotificationLanguage`. Un valor vacío o desconocido en el archivo de configuración vuelve al español. Los textos están en un único catálogo, `internal/services/notification_text.go`.

Los avisos de los servidores de un nodo remoto los envía ese nodo (véase [Quién envía qué](#quién-envía-qué)). El panel pasa su idioma de las notificaciones a cada nodo remoto en la cabecera `X-Aether-Notification-Language` del sondeo de estado que envía cada 30 segundos (`GET /daemon/servers/status`), y sondea al momento cuando el ajuste se guarda con un valor nuevo, así que los avisos remotos siguen el ajuste del panel. En un nodo, el idioma se elige en este orden: un valor definido en el archivo de configuración o en el entorno de ese nodo (`SKYPANEL_PANEL_SETTINGS_NOTIFICATIONLANGUAGE`) tiene prioridad; si no, el valor del panel; si no, `es`. El nodo guarda el valor del panel solo en memoria: justo después de reiniciarse, sus avisos usan `es` hasta el siguiente sondeo, como mucho 30 segundos después. Un nodo con una versión anterior a esta cabecera la ignora y sigue usando su propia configuración.

El webhook JSON mantiene sus valores pensados para máquinas sea cual sea el idioma: `event` es siempre uno de los códigos indicados más abajo, y `resource` en un aviso de recursos es siempre `CPU` o `Memoria`, como antes de que existiera el ajuste.

---

## Webhooks del panel

### Configuración

| Clave | Variable de entorno | Sección en Configuración | Recibe |
|-------|---------------------|--------------------|--------|
| `panel.notifications.discordWebhook` | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOK` | Alertas de servidores | Encendido, apagado inesperado, caída, CPU/memoria alta, resultado de copias |
| `panel.notifications.discordWebhookSystem` | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKSYSTEM` | Informes del sistema | Estado del sistema, cada minuto |
| `panel.notifications.discordWebhookNode` | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKNODE` | Estado de los nodos | Un mensaje de estado por nodo, cada minuto |
| `panel.notifications.discordWebhookExTransfer` | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKEXTRANSFER` | ExTransfer | Transferencia externa completada, fallida o cancelada |

Todas están vacías por defecto (esa categoría queda desactivada). No hay respaldo entre ellas: un webhook vacío simplemente no envía nada. Se pueden editar con `settings.edit` mediante la API de ajustes; ver [Configuración](../backend/02-configuration.md).

**Validación.** Solo se aceptan URL de webhooks de Discord, tanto al guardar como al enviar:

```
^https://(?:(?:ptb|canary)\.)?discord(?:app)?\.com/api/webhooks/\d+/[\w-]+$
```

Cualquier otra cosa se rechaza con `400` (`the URL is not a Discord webhook (https://discord.com/api/webhooks/...)`).

### Quién envía qué

Los avisos de servidores (encendido, apagado, caída, recursos, copias) los envía **el proceso que ejecuta el servidor**, con los webhooks de la configuración de **ese proceso**. Los servidores del nodo local del panel usan los ajustes del panel; un nodo remoto solo los envía si su propia configuración tiene estas claves.

Los informes del sistema y de los nodos solo los envía el proceso que ejecuta el panel (`panel.enable: true`).

### Avisos de servidores (`discordWebhook`)

El bucle de estadísticas (`internal/servers/server.go`) evalúa cada servidor cada 5 segundos. La primera vez que se ve un servidor no se envía nada.

| Evento | Cuándo | Título (`es` / `en`) | Color |
|--------|--------|--------|-------|
| Encendido | El servidor pasa de apagado a en marcha | `✅ Servidor Conectado` / `✅ Server online` | verde `0x00FF00` |
| Apagado | El servidor se detuvo **sin** una parada solicitada, una salida limpia o una caída ya gestionada, y sigue apagado 15 segundos después | `⚠️ Servidor Desconectado` / `⚠️ Server offline` | rojo `0xFF0000` |
| Caída | El proceso terminó con un código inesperado y nadie pidió detenerlo | `💥 Caída del servidor` / `💥 Server crashed` (campos: servidor, ID, reinicio automático sí/no) | rojo |
| CPU alta | CPU por encima del **80 %** de lo que puede usar el servidor | `⚠️ Alto Uso de CPU` / `⚠️ High CPU usage` | rojo |
| Memoria alta | Memoria por encima del **90 %** | `⚠️ Alto Uso de Memoria` / `⚠️ High memory usage` | rojo |
| Copia de seguridad | Termina una copia | `✅ Backup Completado` / `✅ Backup completed`, `❌ Backup Fallido` / `❌ Backup failed` | verde / rojo |

Los avisos de recursos se repiten como mucho cada **5 minutos** por recurso mientras siga alto, y se reinician cuando baja del umbral. Los umbrales están fijados en el código.

Los textos de los embeds siguen el [idioma de las notificaciones](#idioma-de-las-notificaciones) del proceso que los envía. Cada embed lleva la marca de tiempo actual y el pie `Aether Panel`.

### Estado del sistema (`discordWebhookSystem`)

Cada minuto (`processSystemStatus()`), un embed titulado `📊 Estado del Sistema` / `📊 System status`, azul `0x3498DB`, con el pie `Actualizado cada minuto` / `Updated every minute`:

- Total de servidores y número de encendidos y apagados.
- Media de CPU y memoria de los servidores encendidos.
- Un campo en línea por servidor (estado, CPU, RAM). Discord acepta como máximo 25 campos por embed: con más de 25 servidores se muestran los 24 primeros y un último campo `Nota` / `Note` («Mostrando 24 de N servidores» / «Showing 24 of N servers»), 25 campos en total; los totales de la descripción siguen contando todos.

Solo se incluyen los servidores cargados en el propio proceso del panel (el nodo local); un panel sin servidores propios no envía informe del sistema. Los servidores de nodos remotos aparecen en los informes de nodo que se describen a continuación.

### Estado de los nodos (`discordWebhookNode`)

Cada minuto, un embed por cada nodo de la base de datos, titulado `🖥️ Estado del Nodo` / `🖥️ Node status`: nombre del nodo (`Nodo Local` / `Local node` para el nodo local), dirección, número de servidores (total, encendidos, apagados), modelo de CPU, núcleos, hilos, frecuencia, RAM total/usada/libre y sistema operativo. Color: verde si no hay nada apagado, naranja si hay algunos servidores apagados, rojo si todos están apagados.

De dónde salen las cifras:

- **Nodo local**: el número de servidores sale de los servidores que ejecuta el proceso del panel, y las cifras de la máquina, de la máquina que ejecuta el panel.
- **Nodo remoto**: el número de servidores sale de la consulta que hace el panel sobre los servidores del nodo (`GET /daemon/servers/status`, cada 30 segundos; véase [Seguimiento de disponibilidad](13-uptime.md#cómo-se-registra)). Si la última consulta a ese nodo tiene más de 60 segundos (el panel acaba de arrancar o el nodo se acaba de añadir), se pregunta directamente al nodo para este informe. Solo se cuentan los servidores que el panel tiene asignados a ese nodo. Las cifras de la máquina salen del daemon del nodo (`GET /daemon/system`, el mismo dato que muestra la página del nodo, con 10 segundos de tiempo máximo). El daemon no envía la frecuencia de la CPU, así que ese campo no aparece; si `/daemon/system` no responde, el mensaje se envía sin los datos de la máquina.
- **Nodo remoto que no responde** a la consulta de estado (caído, inalcanzable, que rechaza el token del panel o con una versión sin `/daemon/servers/status`): un informe en rojo con el nombre y la dirección del nodo, la línea «El nodo no responde: el estado de sus servidores es desconocido.» / «The node does not answer: the state of its servers is unknown.», el estado `🔴 Sin respuesta` / `🔴 No answer` y el número de servidores que el panel tiene asignados al nodo. No incluye recuento de encendidos/apagados ni cifras de la máquina.

Si `discordWebhookNode` está vacío no se prepara ningún informe de nodo ni se consulta a los nodos remotos para ello (la consulta de uptime se hace igualmente).

### Transferencias externas (`discordWebhookExTransfer`)

Las envía el flujo de ExTransfer (`internal/web/api/extransfer_logic.go`) cuando una transferencia se completa, falla o se cancela. Título `ExTransfer: <estado>`, campos servidor, estado y detalles, todo en el idioma de las notificaciones; verde si sale bien, rojo en otro caso.

### Prueba: `POST /api/settings/test/discord`

Permiso: `settings.edit`. Lo llama el botón **Probar webhook** de **Configuración → Discord**.

- Envía un embed de prueba (`🧪 Test de Webhook Discord` / `🧪 Discord webhook test`, en el idioma de las notificaciones) a **todos** los webhooks del panel configurados.
- El cuerpo puede incluir valores sin guardar con la clave de configuración como nombre (por ejemplo `{"panel.notifications.discordWebhook": "https://discord.com/api/webhooks/…"}`); se validan y se usan solo para la prueba. Un webhook con el valor `********` (lo que la página recibe en lugar del guardado) usa el valor guardado.

| Estado | Significado |
|--------|-------------|
| `204` | Todos los webhooks configurados aceptaron el mensaje |
| `400` | `ErrDiscordNoWebhooks`: no hay ningún webhook configurado; o un valor enviado no es una URL de webhook de Discord |
| `422` | `ErrDiscordTestFailed`: Discord rechazó el mensaje en uno o varios webhooks (normalmente el webhook se borró) |

`ErrDiscordTestFailed` nombra los webhooks que fallaron en `metadata.webhooks` como `alerts`, `reports`, `status` y `transfers` (en ese orden: avisos de servidores, informes del sistema, estado de los nodos, ExTransfer); la página de ajustes los muestra con los nombres de sus propias secciones, en el idioma de quien la ve.

```bash
curl -X POST https://panel.example.com/api/settings/test/discord \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

---

## Notificaciones del servidor

Cada servidor tiene sus propios canales e interruptores de eventos, independientes de los webhooks del panel.

### Almacenamiento

Se guardan en `<daemon.data.servers>/<serverId>.notify` en el nodo que ejecuta el servidor, con permisos `0600` (las URL de webhook y los tokens de bot son secretos). Un servidor sin este archivo usa los valores por defecto.

```json
{
  "discordWebhook": "https://discord.com/api/webhooks/123/abc",
  "telegramBotToken": "123456789:AA…",
  "telegramChatId": "-1001234567890",
  "webhookUrl": "https://example.com/hooks/aether",
  "webhookSecret": "s3cret",
  "crash": true,
  "backup": true,
  "resources": true,
  "power": false
}
```

| Campo | Por defecto | Descripción |
|-------|-------------|-------------|
| `discordWebhook` | `""` | URL de webhook de Discord (misma validación que los webhooks del panel) |
| `telegramBotToken` / `telegramChatId` | `""` | Token del bot de @BotFather y el chat donde publica. Los dos vacíos o los dos válidos: token `<5-15 dígitos>:<30+ caracteres>`, chat un id numérico (puede ser negativo) o `@nombredelcanal` |
| `webhookUrl` | `""` | Cualquier URL `https://` con host y sin credenciales |
| `webhookSecret` | `""` | Opcional; firma el cuerpo JSON |
| `crash` | `true` | El servidor se cayó por sí solo (incluye las últimas 12 líneas de consola) |
| `backup` | `true` | Una copia de seguridad terminó o falló |
| `resources` | `true` | CPU alta (> 80 %) o memoria alta (> 90 %), con los mismos límites y la misma repetición cada 5 minutos que arriba |
| `power` | `false` | El servidor se encendió o se apagó (cualquier parada, también las solicitadas) |

No se envía nada hasta que hay al menos un canal configurado. El envío se hace en segundo plano para que un canal lento nunca frene al servidor.

### API

El panel la reenvía al nodo (`internal/web/api/servers.go` → `internal/web/daemon/server.go`). Permiso: `server.flags.edit` sobre el servidor.

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/servers/:serverId/notifications` | Ajustes actuales (los valores por defecto si nunca se guardaron) |
| PUT | `/api/servers/:serverId/notifications` | Valida y guarda; `204`, o `400` con el error de validación |
| POST | `/api/servers/:serverId/notifications/test` | Envía un mensaje de prueba (`🔔 Notificaciones conectadas` / `🔔 Notifications connected`) a todos los canales |

La prueba responde `200` con el resultado de cada canal, por ejemplo `{"discord": "ok", "telegram": "telegram answered with status 401"}`. Devuelve `400` si no hay ningún canal y `429` si se vuelve a llamar antes de 10 segundos para el mismo servidor.

### Formato de los mensajes

- **Discord:** un embed con el título del evento, el mensaje (con el nombre del servidor en negrita), las líneas de consola en un bloque de código cuando las hay (últimos 3500 caracteres) y el pie `Aether Panel · <serverId>`. Colores: caída `0xF2726B`, aviso `0xF5A524`, éxito `0x34D399`, información `0x3B9EF5`.
- **Telegram:** `sendMessage` con `parse_mode: HTML`, el título en negrita, las líneas de consola en `<pre>` (últimos 3000 caracteres) y sin vista previa de enlaces.
- **Webhook JSON:** `POST` con `Content-Type: application/json` y `User-Agent: AetherPanel-Webhook/1`, con 10 segundos de límite:

  ```json
  {
    "event": "crash",
    "title": "💥 Caída del servidor",
    "message": "Survival se ha caído (código de salida 1). No se reinició automáticamente.",
    "server": { "id": "a1b2c3d4", "name": "Survival" },
    "timestamp": "2026-11-01T10:00:00Z",
    "lines": ["…últimas líneas de consola…"],
    "exitCode": 1,
    "restarted": false
  }
  ```

  `title` y `message` están en el [idioma de las notificaciones](#idioma-de-las-notificaciones) del nodo que ejecuta el servidor (el ejemplo está en español, el idioma por defecto). `event` es `crash`, `backup`, `resources`, `power` o `test`. Campos adicionales por evento: `exitCode`/`restarted` (caída), `success` (copia), `resource`/`value`/`threshold` (recursos), `running` (encendido/apagado).

  Con `webhookSecret`, la cabecera `X-Aether-Signature: sha256=<hex>` lleva el HMAC-SHA256 del cuerpo sin modificar, con el secreto como clave. Verifíquelo en el receptor, por ejemplo:

  ```python
  import hmac, hashlib
  expected = "sha256=" + hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
  ok = hmac.compare_digest(expected, request.headers["X-Aether-Signature"])
  ```

Los mismos eventos (caída, copia, recursos, encendido/apagado) también se entregan a las extensiones instaladas, sea cual sea la configuración del propietario; ver [Extensiones](../reference/15-extensions.md).

---

## Crear un webhook de Discord

1. En Discord: **Configuración del servidor → Integraciones → Webhooks → Nuevo webhook**.
2. Elija el canal y copie la URL del webhook.
3. Péguela en **Configuración → Discord** (panel) o en la tarjeta **Notificaciones** del servidor, guarde y use el botón de prueba.

---

## Solución de problemas

| Síntoma | Causa |
|---------|-------|
| `400` al guardar un webhook | La URL no es del tipo `https://discord.com/api/webhooks/<id>/<token>` (también se aceptan `ptb.`, `canary.` y `discordapp.com`) |
| La prueba responde `422` / un canal muestra `discord webhook returned status 404` | El webhook se borró en Discord |
| No llegan avisos de los servidores de un nodo remoto | La configuración de ese nodo no tiene `panel.notifications.discordWebhook` |
| Los avisos de un nodo remoto llegan en español aunque el panel está en inglés | La configuración o el entorno del propio nodo definen `panel.settings.notificationLanguage` como `es`, que tiene prioridad sobre el valor del panel (quítelo allí o ponga `en`); o el nodo se reinició hace menos de 30 segundos y aún no se ha sondeado; o el sondeo de estado del panel no llega al nodo |
| Un informe de nodo dice `🔴 Sin respuesta` | La consulta de estado del panel no llegó a ese nodo: está caído o no se alcanza, rechaza el token del panel (véase [Solución de problemas](../setup/troubleshooting.md)) o tiene una versión sin `/daemon/servers/status` y necesita actualizarse |
| No llega aviso de apagado tras detener un servidor | Es lo esperado: las paradas solicitadas y las caídas gestionadas no generan aviso de apagado (las caídas tienen su propio aviso) |
| `400` de Telegram al guardar | El token o el id del chat no tienen el formato esperado |
