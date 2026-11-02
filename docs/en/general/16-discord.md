# Discord and Notifications

Aether Panel has two independent notification layers:

1. **Panel webhooks** (administrators): four Discord webhooks set in **Settings → Discord** that receive server alerts, system and node reports, and external transfer reports.
2. **Server notifications** (server owners): each server can send its own alerts to a Discord webhook, a Telegram bot and/or a generic JSON webhook, set in the server's **Settings → Notifications** card.

All requests go through a restricted HTTP client that refuses private, loopback and internal addresses (checked on the connection itself, after DNS resolution), so a webhook URL can never be used to reach the node's own network. Messages are sent once: there are no retries, and a failure is only logged.

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

| Event | When | Title | Colour |
|-------|------|-------|--------|
| Online | The server goes from stopped to running | `✅ Servidor Conectado` | green `0x00FF00` |
| Offline | The server stopped **without** a requested stop, clean exit or handled crash, and is still down 15 seconds later | `⚠️ Servidor Desconectado` | red `0xFF0000` |
| Crash | The process exited with an unexpected code and nobody asked it to stop | `💥 Caída del servidor` (fields: server, ID, automatic restart yes/no) | red |
| High CPU | CPU above **80 %** of what the server may use | `⚠️ Alto Uso de CPU` | red |
| High memory | Memory above **90 %** | `⚠️ Alto Uso de Memoria` | red |
| Backup | A backup finishes | `✅ Backup Completado` / `❌ Backup Fallido` | green / red |

Resource alerts are repeated at most every **5 minutes** per resource while it stays high, and reset once it drops below the threshold. The thresholds are fixed in code.

Embed texts are in Spanish. Every embed gets the current timestamp and the footer `Aether Panel`.

### System status (`discordWebhookSystem`)

Every minute (`processSystemStatus()`), one embed titled `📊 Estado del Sistema`, blue `0x3498DB`, footer `Actualizado cada minuto`:

- Total servers, online and offline counts.
- Average CPU and memory of the online servers.
- One inline field per server (status, CPU, RAM). Discord accepts at most 25 fields per embed: above 25 servers the first 24 are shown plus a last `Nota` field ("Mostrando 24 de N servidores"); the totals in the description still count all of them.

Only the servers loaded in the panel's own process (the local node) are included.

### Node status (`discordWebhookNode`)

Every minute, one embed per node in the database, titled `🖥️ Estado del Nodo`: node name (`Nodo Local` for the local node), address, server counts, CPU model, cores, threads, frequency, total/used/free RAM and operating system. Colour: green when nothing is offline, orange when some servers are offline, red when every server is offline.

The machine figures come from each node: for the local node, from the machine running the panel; for a remote node, from its own daemon (`GET /daemon/system`, the same data the node page shows, with a 10-second timeout). The daemon does not send the CPU frequency, so that field is left out for remote nodes; when a remote node does not answer, the message is sent without its machine figures. When `discordWebhookNode` is empty nothing is collected and remote nodes are not asked.

> **Limitation:** the server counts only include servers loaded in the panel's process (the local node); for a remote node they show zero.

### External transfers (`discordWebhookExTransfer`)

Sent by the ExTransfer flow (`internal/web/api/extransfer_logic.go`) when a transfer completes, fails or is cancelled. Title `ExTransfer: <status>`, fields server, status and details; green on success, red otherwise.

### Testing: `POST /api/settings/test/discord`

Permission: `settings.edit`. The **Test Webhook** button in **Settings → Discord** calls it.

- Sends a test embed (`🧪 Test de Webhook Discord`) to **every** configured panel webhook.
- The body may contain unsaved values keyed by config key (for example `{"panel.notifications.discordWebhook": "https://discord.com/api/webhooks/…"}`); they are validated and used for the test only. A webhook set to `********` (what the page receives instead of the saved one) uses the saved value.

| Status | Meaning |
|--------|---------|
| `204` | Every configured webhook accepted the message |
| `400` | No webhook configured, or an override is not a Discord webhook URL |
| `422` | Discord rejected the message for one or more webhooks (named in the message; usually the webhook was deleted) |

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
| POST | `/api/servers/:serverId/notifications/test` | Send a test message (`🔔 Notificaciones conectadas`) to every channel |

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

  `event` is `crash`, `backup`, `resources`, `power` or `test`. Extra fields per event: `exitCode`/`restarted` (crash), `success` (backup), `resource`/`value`/`threshold` (resources), `running` (power).

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
| No offline alert after stopping a server | Expected: requested stops and handled crashes do not produce an offline alert (crashes have their own alert) |
| System status stops arriving with more than 25 servers | With more than 25 servers the embed carries 25 server fields plus a note field (26), above Discord's limit of 25 fields per embed, so Discord rejects it |
| Telegram `400` when saving | Token or chat id do not match the expected format |
