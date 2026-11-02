# Discord and Notifications

Aether Panel has two independent notification layers:

1. **Panel webhooks** (administrators): four Discord webhooks set in **Settings → Discord** that receive server alerts, system and node reports, and external transfer reports.
2. **Server notifications** (server owners): each server can send its own alerts to a Discord webhook, a Telegram bot and/or a generic JSON webhook, set in the server's **Settings → Notifications** card.

All requests go through a restricted HTTP client that refuses private, loopback and internal addresses (checked on the connection itself, after DNS resolution), so a webhook URL can never be used to reach the node's own network. Messages are sent once: there are no retries, and a failure is only logged.

---

## Notification language

Everything the panel sends outside its own pages is written in one language, set in **Settings → General → Notifications → Notification language**: the panel webhooks below (alerts, system and node reports, the Discord test, external transfer reports), each server's Discord, Telegram and JSON webhook alerts and their test message, and the title and message of the events handed to [extensions](../reference/15-extensions.md). The panel's own pages do not depend on it: they follow each user's language.

| Key | Environment variable | Values | Default |
|-----|----------------------|--------|---------|
| `panel.settings.notificationLanguage` | `SKYPANEL_PANEL_SETTINGS_NOTIFICATIONLANGUAGE` | `es`, `en` | `es` |

Saving any other value through the settings API is refused with `400` `ErrNotificationLanguage`. An empty or unknown value in the config file falls back to Spanish. The texts live in one catalogue, `internal/services/notification_text.go`.

Alerts about servers on a remote node are sent by that node (see [Who sends what](#who-sends-what)). The panel passes its notification language to every remote node in the header `X-Aether-Notification-Language` of the status poll it sends every 30 seconds (`GET /daemon/servers/status`), and polls at once when the setting is saved with a new value, so remote alerts follow the panel's setting. On a node the language is chosen in this order: a value set in that node's own config file or environment (`SKYPANEL_PANEL_SETTINGS_NOTIFICATIONLANGUAGE`) wins; otherwise the panel's value; otherwise `es`. The node keeps the panel's value in memory only: right after a node restart its alerts use `es` until the next poll, at most 30 seconds later. A node on a version from before this header ignores it and keeps using its own config.

The JSON webhook keeps its machine-readable values whatever the language: `event` is always one of the codes listed below, and `resource` in a resource alert is always `CPU` or `Memoria`, as before the setting existed.

---

## Panel webhooks

### Configuration

| Key | Environment variable | Settings section | Receives |
|-----|----------------------|------------------|----------|
| `panel.notifications.discordWebhook` | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOK` | Server alerts | Online, unexplained offline, crash, high CPU/memory, backup result |
| `panel.notifications.discordWebhookSystem` | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKSYSTEM` | System reports | System status, every minute |
| `panel.notifications.discordWebhookNode` | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKNODE` | Node status | One status message per node, every minute |
| `panel.notifications.discordWebhookExTransfer` | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKEXTRANSFER` | ExTransfer | External transfer completed, failed or cancelled |

All default to empty (that category is off). There is no fallback between them: an empty webhook simply sends nothing. They are editable with `settings.edit` through the settings API; see [Configuration](../backend/02-configuration.md).

**Validation.** Only Discord webhook URLs are accepted, both when saving and when sending:

```
^https://(?:(?:ptb|canary)\.)?discord(?:app)?\.com/api/webhooks/\d+/[\w-]+$
```

Anything else is refused with `400` (`the URL is not a Discord webhook (https://discord.com/api/webhooks/...)`).

### Who sends what

Server alerts (online, offline, crash, resources, backup) are sent by **the process that runs the server**, using the webhooks in **that process's** configuration. Servers on the panel's local node use the panel settings; a remote node only sends them if its own configuration has these keys.

System and node reports are only sent by the process that runs the panel (`panel.enable: true`).

### Server alerts (`discordWebhook`)

The stats loop (`internal/servers/server.go`) evaluates every server every 5 seconds. The first time a server is seen nothing is sent.

| Event | When | Title (`es` / `en`) | Colour |
|-------|------|-------|--------|
| Online | The server goes from stopped to running | `✅ Servidor Conectado` / `✅ Server online` | green `0x00FF00` |
| Offline | The server stopped **without** a requested stop, clean exit or handled crash, and is still down 15 seconds later | `⚠️ Servidor Desconectado` / `⚠️ Server offline` | red `0xFF0000` |
| Crash | The process exited with an unexpected code and nobody asked it to stop | `💥 Caída del servidor` / `💥 Server crashed` (fields: server, ID, automatic restart yes/no) | red |
| High CPU | CPU above **80 %** of what the server may use | `⚠️ Alto Uso de CPU` / `⚠️ High CPU usage` | red |
| High memory | Memory above **90 %** | `⚠️ Alto Uso de Memoria` / `⚠️ High memory usage` | red |
| Backup | A backup finishes | `✅ Backup Completado` / `✅ Backup completed`, `❌ Backup Fallido` / `❌ Backup failed` | green / red |

Resource alerts are repeated at most every **5 minutes** per resource while it stays high, and reset once it drops below the threshold. The thresholds are fixed in code.

Embed texts follow the [notification language](#notification-language) of the process that sends them. Every embed gets the current timestamp and the footer `Aether Panel`.

### System status (`discordWebhookSystem`)

Every minute (`processSystemStatus()`), one embed titled `📊 Estado del Sistema` / `📊 System status`, blue `0x3498DB`, footer `Actualizado cada minuto` / `Updated every minute`:

- Total servers, online and offline counts.
- Average CPU and memory of the online servers.
- One inline field per server (status, CPU, RAM). Discord accepts at most 25 fields per embed: above 25 servers the first 24 are shown plus a last `Nota` / `Note` field ("Mostrando 24 de N servidores" / "Showing 24 of N servers"), 25 fields in all; the totals in the description still count all of them.

Only the servers loaded in the panel's own process (the local node) are included; a panel without servers of its own sends no system report. Servers on remote nodes appear in the node reports below.

### Node status (`discordWebhookNode`)

Every minute, one embed per node in the database, titled `🖥️ Estado del Nodo` / `🖥️ Node status`: node name (`Nodo Local` / `Local node` for the local node), address, server counts (total, online, offline), CPU model, cores, threads, frequency, total/used/free RAM and operating system. Colour: green when nothing is offline, orange when some servers are offline, red when every server is offline.

Where the figures come from:

- **Local node**: the server counts come from the servers the panel's process runs, and the machine figures from the machine running the panel.
- **Remote node**: the server counts come from the panel's poll of the node's servers (`GET /daemon/servers/status`, every 30 seconds; see [Uptime tracking](13-uptime.md#how-it-is-recorded)). When the last poll of that node is older than 60 seconds (the panel has just started, or the node was just added), the node is asked directly for this report. Only servers the panel places on that node are counted. The machine figures come from the node's daemon (`GET /daemon/system`, the same data the node page shows, with a 10-second timeout). The daemon does not send the CPU frequency, so that field is left out; when `/daemon/system` does not answer, the message is sent without the machine figures.
- **Remote node that does not answer** the status poll (down, unreachable, refusing the panel's token, or on a version without `/daemon/servers/status`): a red report with the node's name and address, the line "El nodo no responde: el estado de sus servidores es desconocido." / "The node does not answer: the state of its servers is unknown.", the status `🔴 Sin respuesta` / `🔴 No answer` and the number of servers the panel places on the node. No online/offline counts or machine figures are given.

When `discordWebhookNode` is empty no node report is built and remote nodes are not asked for it (the uptime poll runs anyway).

### External transfers (`discordWebhookExTransfer`)

Sent by the ExTransfer flow (`internal/web/api/extransfer_logic.go`) when a transfer completes, fails or is cancelled. Title `ExTransfer: <status>`, fields server, status and details, all in the notification language; green on success, red otherwise.

### Testing: `POST /api/settings/test/discord`

Permission: `settings.edit`. The **Test Webhook** button in **Settings → Discord** calls it.

- Sends a test embed (`🧪 Test de Webhook Discord` / `🧪 Discord webhook test`, in the notification language) to **every** configured panel webhook.
- The body may contain unsaved values keyed by config key (for example `{"panel.notifications.discordWebhook": "https://discord.com/api/webhooks/…"}`); they are validated and used for the test only. A webhook set to `********` (what the page receives instead of the saved one) uses the saved value.

| Status | Meaning |
|--------|---------|
| `204` | Every configured webhook accepted the message |
| `400` | `ErrDiscordNoWebhooks`: no webhook configured; or an override is not a Discord webhook URL |
| `422` | `ErrDiscordTestFailed`: Discord rejected the message for one or more webhooks (usually the webhook was deleted) |

`ErrDiscordTestFailed` names the failed webhooks in `metadata.webhooks` as `alerts`, `reports`, `status` and `transfers` (in that order: server alerts, system reports, node status, ExTransfer); the settings page shows them with the names of its own sections, in the viewer's language.

```bash
curl -X POST https://panel.example.com/api/settings/test/discord \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

---

## Server notifications

Each server has its own channels and event switches, independent of the panel webhooks.

### Storage

Stored in `<daemon.data.servers>/<serverId>.notify` on the node that runs the server, with mode `0600` (webhook URLs and bot tokens are secrets). A server without the file uses the defaults.

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

| Field | Default | Description |
|-------|---------|-------------|
| `discordWebhook` | `""` | Discord webhook URL (same validation as the panel webhooks) |
| `telegramBotToken` / `telegramChatId` | `""` | Bot token from @BotFather and the chat it posts to. Both empty, or both valid: token `<5-15 digits>:<30+ characters>`, chat a numeric id (may be negative) or `@channelname` |
| `webhookUrl` | `""` | Any `https://` URL with a host and no credentials |
| `webhookSecret` | `""` | Optional; signs the JSON body |
| `crash` | `true` | The server died on its own (includes the last 12 console lines) |
| `backup` | `true` | A backup finished or failed |
| `resources` | `true` | High CPU (> 80 %) or memory (> 90 %), same limits and 5-minute repeat as above |
| `power` | `false` | The server was switched on or off (any stop, including requested ones) |

Nothing is sent until at least one channel is set. Delivery runs in the background so a slow channel never holds the server up.

### API

Proxied by the panel to the node (`internal/web/api/servers.go` → `internal/web/daemon/server.go`). Permission: `server.flags.edit` on the server.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/servers/:serverId/notifications` | Current settings (defaults if never saved) |
| PUT | `/api/servers/:serverId/notifications` | Validate and save; `204`, or `400` with the validation error |
| POST | `/api/servers/:serverId/notifications/test` | Send a test message (`🔔 Notificaciones conectadas` / `🔔 Notifications connected`) to every channel |

The test answers `200` with each channel's result, for example `{"discord": "ok", "telegram": "telegram answered with status 401"}`. It returns `400` when no channel is set and `429` if called again within 10 seconds for the same server.

### Message formats

- **Discord:** an embed with the event title, the message (server name in bold), the console lines in a code block when present (last 3500 characters), and the footer `Aether Panel · <serverId>`. Colours: crash `0xF2726B`, warning `0xF5A524`, success `0x34D399`, info `0x3B9EF5`.
- **Telegram:** `sendMessage` with `parse_mode: HTML`, the title in bold, the console lines in `<pre>` (last 3000 characters), link previews disabled.
- **JSON webhook:** `POST` with `Content-Type: application/json` and `User-Agent: AetherPanel-Webhook/1`, 10-second timeout:

  ```json
  {
    "event": "crash",
    "title": "💥 Caída del servidor",
    "message": "Survival se ha caído (código de salida 1). No se reinició automáticamente.",
    "server": { "id": "a1b2c3d4", "name": "Survival" },
    "timestamp": "2026-11-01T10:00:00Z",
    "lines": ["…last console lines…"],
    "exitCode": 1,
    "restarted": false
  }
  ```

  `title` and `message` are in the [notification language](#notification-language) of the node that runs the server (the example is in Spanish, the default). `event` is `crash`, `backup`, `resources`, `power` or `test`. Extra fields per event: `exitCode`/`restarted` (crash), `success` (backup), `resource`/`value`/`threshold` (resources), `running` (power).

  With a `webhookSecret`, the header `X-Aether-Signature: sha256=<hex>` carries the HMAC-SHA256 of the raw body with the secret as key. Verify it on the receiver, for example:

  ```python
  import hmac, hashlib
  expected = "sha256=" + hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
  ok = hmac.compare_digest(expected, request.headers["X-Aether-Signature"])
  ```

The same events (crash, backup, resources, started/stopped) are also handed to installed extensions regardless of the owner's settings; see [Extensions](../reference/15-extensions.md).

---

## Creating a Discord webhook

1. In Discord: **Server Settings → Integrations → Webhooks → New Webhook**.
2. Choose the channel and copy the webhook URL.
3. Paste it into **Settings → Discord** (panel) or the server's **Notifications** card, save, and use the test button.

---

## Troubleshooting

| Symptom | Cause |
|---------|-------|
| `400` when saving a webhook | The URL is not a `https://discord.com/api/webhooks/<id>/<token>` URL (`ptb.`, `canary.` and `discordapp.com` are also accepted) |
| Test answers `422` / a channel shows `discord webhook returned status 404` | The webhook was deleted in Discord |
| No alerts for servers on a remote node | That node's configuration has no `panel.notifications.discordWebhook` |
| Alerts from a remote node arrive in Spanish while the panel is set to English | The node's own config or environment sets `panel.settings.notificationLanguage` to `es`, which wins over the panel's value (remove it there or set `en`); or the node restarted less than 30 seconds ago and has not been polled yet; or the panel's status poll does not reach the node |
| A node report says `🔴 No answer` | The panel's status poll did not reach that node: it is down or unreachable, refuses the panel's token (see [Troubleshooting](../setup/troubleshooting.md)), or runs a version without `/daemon/servers/status` and needs updating |
| No offline alert after stopping a server | Expected: requested stops and handled crashes do not produce an offline alert (crashes have their own alert) |
| Telegram `400` when saving | Token or chat id do not match the expected format |
