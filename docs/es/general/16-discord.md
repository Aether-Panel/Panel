# Discord y notificaciones

Aether Panel tiene dos capas de notificación independientes:

1. **Webhooks del panel** (administradores): cuatro webhooks de Discord configurados en **Configuración → Discord** que reciben avisos de servidores, informes del sistema y de los nodos, e informes de transferencias externas.
2. **Notificaciones del servidor** (propietarios): cada servidor puede enviar sus propios avisos a un webhook de Discord, a un bot de Telegram y/o a un webhook JSON genérico, configurados en la tarjeta **Notificaciones** de los ajustes del servidor.

Todas las peticiones pasan por un cliente HTTP restringido que rechaza direcciones privadas, de loopback e internas (se comprueba en la propia conexión, después de resolver el DNS), de modo que una URL de webhook nunca puede usarse para llegar a la red del propio nodo. Los mensajes se envían una sola vez: no hay reintentos y un fallo solo se registra en el log.

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

| Evento | Cuándo | Título | Color |
|--------|--------|--------|-------|
| Encendido | El servidor pasa de apagado a en marcha | `✅ Servidor Conectado` | verde `0x00FF00` |
| Apagado | El servidor se detuvo **sin** una parada solicitada, una salida limpia o una caída ya gestionada, y sigue apagado 15 segundos después | `⚠️ Servidor Desconectado` | rojo `0xFF0000` |
| Caída | El proceso terminó con un código inesperado y nadie pidió detenerlo | `💥 Caída del servidor` (campos: servidor, ID, reinicio automático sí/no) | rojo |
| CPU alta | CPU por encima del **80 %** de lo que puede usar el servidor | `⚠️ Alto Uso de CPU` | rojo |
| Memoria alta | Memoria por encima del **90 %** | `⚠️ Alto Uso de Memoria` | rojo |
| Copia de seguridad | Termina una copia | `✅ Backup Completado` / `❌ Backup Fallido` | verde / rojo |

Los avisos de recursos se repiten como mucho cada **5 minutos** por recurso mientras siga alto, y se reinician cuando baja del umbral. Los umbrales están fijados en el código.

Los textos de los embeds están en español. Cada embed lleva la marca de tiempo actual y el pie `Aether Panel`.

### Estado del sistema (`discordWebhookSystem`)

Cada minuto (`processSystemStatus()`), un embed titulado `📊 Estado del Sistema`, azul `0x3498DB`, con el pie `Actualizado cada minuto`:

- Total de servidores y número de encendidos y apagados.
- Media de CPU y memoria de los servidores encendidos.
- Un campo en línea por servidor (estado, CPU, RAM). Discord acepta como máximo 25 campos por embed: con más de 25 servidores se muestran los 24 primeros y un último campo `Nota` («Mostrando 24 de N servidores»); los totales de la descripción siguen contando todos.

Solo se incluyen los servidores cargados en el propio proceso del panel (el nodo local).

### Estado de los nodos (`discordWebhookNode`)

Cada minuto, un embed por cada nodo de la base de datos, titulado `🖥️ Estado del Nodo`: nombre del nodo (`Nodo Local` para el nodo local), dirección, número de servidores, modelo de CPU, núcleos, hilos, frecuencia, RAM total/usada/libre y sistema operativo. Color: verde si no hay nada apagado, naranja si hay algunos servidores apagados, rojo si todos están apagados.

Las cifras de la máquina salen de cada nodo: para el nodo local, de la máquina que ejecuta el panel; para un nodo remoto, de su propio daemon (`GET /daemon/system`, el mismo dato que muestra la página del nodo, con 10 segundos de tiempo máximo). El daemon no envía la frecuencia de la CPU, así que ese campo no aparece en los nodos remotos; si un nodo remoto no responde, el mensaje se envía sin sus datos de máquina. Si `discordWebhookNode` está vacío no se recoge nada, ni siquiera se consulta a los nodos remotos.

> **Limitación:** el número de servidores solo cuenta los servidores cargados en el proceso del panel (el nodo local); para un nodo remoto aparece a cero.

### Transferencias externas (`discordWebhookExTransfer`)

Las envía el flujo de ExTransfer (`internal/web/api/extransfer_logic.go`) cuando una transferencia se completa, falla o se cancela. Título `ExTransfer: <estado>`, campos servidor, estado y detalles; verde si sale bien, rojo en otro caso.

### Prueba: `POST /api/settings/test/discord`

Permiso: `settings.edit`. Lo llama el botón **Probar webhook** de **Configuración → Discord**.

- Envía un embed de prueba (`🧪 Test de Webhook Discord`) a **todos** los webhooks del panel configurados.
- El cuerpo puede incluir valores sin guardar con la clave de configuración como nombre (por ejemplo `{"panel.notifications.discordWebhook": "https://discord.com/api/webhooks/…"}`); se validan y se usan solo para la prueba. Un webhook con el valor `********` (lo que la página recibe en lugar del guardado) usa el valor guardado.

| Estado | Significado |
|--------|-------------|
| `204` | Todos los webhooks configurados aceptaron el mensaje |
| `400` | No hay ningún webhook configurado, o un valor enviado no es una URL de webhook de Discord |
| `422` | Discord rechazó el mensaje en uno o varios webhooks (se nombran en el mensaje; normalmente el webhook se borró) |

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
| POST | `/api/servers/:serverId/notifications/test` | Envía un mensaje de prueba (`🔔 Notificaciones conectadas`) a todos los canales |

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

  `event` es `crash`, `backup`, `resources`, `power` o `test`. Campos adicionales por evento: `exitCode`/`restarted` (caída), `success` (copia), `resource`/`value`/`threshold` (recursos), `running` (encendido/apagado).

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
| No llega aviso de apagado tras detener un servidor | Es lo esperado: las paradas solicitadas y las caídas gestionadas no generan aviso de apagado (las caídas tienen su propio aviso) |
| El estado del sistema deja de llegar con más de 25 servidores | Con más de 25 servidores el embed lleva 25 campos de servidor más un campo de nota (26), por encima del límite de 25 campos por embed de Discord, que lo rechaza |
| `400` de Telegram al guardar | El token o el id del chat no tienen el formato esperado |
