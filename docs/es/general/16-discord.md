# Discord Webhooks

Aether Panel sends rich Discord embed notifications for server events, system status, and alerts via configurable webhooks.

---

## Overview

- **4 separate webhook URLs** for different notification categories
- **Rich embeds** with colors, fields, timestamps
- **Per-event customization** of title, description, fields
- **System status aggregates** with per-server details

---

## Configuration

```json
{
  "panel": {
    "notifications": {
      "discordWebhook": "https://discord.com/api/webhooks/xxx/yyy",
      "discordWebhookSystem": "https://discord.com/api/webhooks/aaa/bbb",
      "discordWebhookNode": "https://discord.com/api/webhooks/ccc/ddd",
      "discordWebhookExTransfer": "https://discord.com/api/webhooks/eee/fff"
    }
  }
}
```

**Environment Variables:**
| Variable | Config Key | Purpose |
|----------|------------|---------|
| `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOK` | `panel.notifications.discordWebhook` | General alerts |
| `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKSYSTEM` | `panel.notifications.discordWebhookSystem` | System status (hourly) |
| `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKNODE` | `panel.notifications.discordWebhookNode` | Node alerts |
| `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKEXTRANSFER` | `panel.notations.discordWebhookExTransfer` | External transfers |

---

## Webhook Types & Routing

| Webhook | Config Key | Events |
|---------|------------|--------|
| **General** | `discordWebhook` | Server offline/online, resource alerts, backup success/fail, license expiry |
| **System** | `discordWebhookSystem` | Hourly system status summary |
| **Node** | `discordWebhookNode` | Node offline/online, node resource alerts |
| **External Transfer** | `discordWebhookExTransfer` | Transfer created/validated/consumed/completed/cancelled |

**Fallback Logic:** If specific webhook not set, falls back to `discordWebhook`.

---

## DiscordService (`internal/services/discord.go`)

```go
type DiscordService struct {
    DB *gorm.DB
}

type DiscordField struct {
    Name   string
    Value  string
    Inline bool
}

func (s *DiscordService) SendWebhook(webhookURL, title, description string, color int, fields []DiscordField) error
func (s *DiscordService) SendWebhookToURL(webhookURL, title, description string, color int, fields []DiscordField) error

// Convenience methods
func (s *DiscordService) SendAlert(title, description string) error
func (s *DiscordService) SendServerOfflineAlert(serverName, serverID string) error
func (s *DiscordService) SendServerOnlineAlert(serverName, serverID string) error
func (s *DiscordService) SendResourceAlert(serverName, serverID, resource string, current, threshold float64) error
func (s *DiscordService) SendBackupAlert(serverName, serverID, status, backupName string) error
func (s *DiscordService) SendSystemStatus(servers []ServerStatusSummary) error
func (s *DiscordService) SendNodeStatus(nodeName string, info SystemInfo, servers []ServerStatusSummary) error
```

---

## Embed Structure

```json
{
  "embeds": [{
    "title": "🔴 Server Offline",
    "description": "Server **MyServer** (`abc123`) went offline",
    "color": 16711680,
    "fields": [
      {"name": "Server", "value": "MyServer", "inline": true},
      {"name": "ID", "value": "abc123", "inline": true},
      {"name": "Time", "value": "2024-01-15 14:30:00 UTC", "inline": true}
    ],
    "timestamp": "2024-01-15T14:30:00Z",
    "footer": {"text": "Aether Panel"}
  }]
}
```

**Color Constants:**
| Color | Hex | Decimal | Use Case |
|-------|-----|---------|----------|
| Red | `#FF0000` | 16711680 | Offline, failed, critical |
| Green | `#00FF00` | 65280 | Online, success, recovered |
| Yellow | `#FFFF00` | 16776960 | Warning, resource threshold |
| Blue | `#0000FF` | 255 | Info, system status |
| Orange | `#FFA500` | 16753920 | Warning, maintenance |

---

## Event Types & Payloads

### 1. Server Offline
```go
SendServerOfflineAlert("MyServer", "abc123")
```
**Embed:**
```json
{
  "title": "🔴 Server Offline",
  "description": "Server **MyServer** (`abc123`) went offline",
  "color": 16711680,
  "fields": [
    {"name": "Server", "value": "MyServer", "inline": true},
    {"name": "ID", "value": "abc123", "inline": true},
    {"name": "Time", "value": "2024-01-15 14:30:00 UTC", "inline": true}
  ]
}
```

### 2. Server Online
```go
SendServerOnlineAlert("MyServer", "abc123")
```
**Color:** Green (65280) 🟢

### 3. Resource Alert
```go
SendResourceAlert("MyServer", "abc123", "CPU", 95.5, 80.0)
```
**Fields:** Resource, Current Value, Threshold, Server, Time

### 4. Backup Alert
```go
SendBackupAlert("MyServer", "abc123", "failed", "backup_20240115.tar.gz")
```
**Status:** `success` (Green) / `failed` (Red)

### 5. System Status (Hourly)
```go
SendSystemStatus([]ServerStatusSummary{
    {Name: "Server1", ID: "abc", Running: true, CPU: 45.2, Memory: 60.1},
    {Name: "Server2", ID: "def", Running: false, CPU: 0, Memory: 0},
})
```
**Summary Embed:**
- Total servers, online/offline counts
- Average CPU/RAM across online servers
- Per-server fields (max 25 due to Discord limit)

### 6. Node Status
```go
SendNodeStatus("Node-01", SystemInfo{...}, servers)
```
**Includes:** Node specs (CPU model, cores, RAM), server counts, per-server status

---

## Webhook Payload Format

Discord expects:
```json
{
  "username": "Aether Panel",
  "avatar_url": "https://example.com/icon.png",
  "embeds": [{
    "title": "Title",
    "description": "Description with **markdown**",
    "color": 16711680,
    "fields": [
      {"name": "Field Name", "value": "Field Value", "inline": true}
    ],
    "timestamp": "2024-01-15T14:30:00Z",
    "footer": {"text": "Aether Panel"}
  }]
}
```

---

## SendWebhook() - Low Level

```go
func (s *DiscordService) SendWebhook(
    webhookURL, title, description string,
    color int, fields []DiscordField
) error
```

**Usage:**
```go
s.SendWebhook(
    config.DiscordWebhook,
    "Custom Alert",
    "Something happened",
    16776960, // Yellow
    []DiscordField{
        {Name: "Key", Value: "Value", Inline: true},
    },
)
```

---

## Integration Points

### Server Alerts (`internal/servers/server.go:checkServerAlerts`)

```go
func (s *Server) checkServerAlerts(running bool) {
    // State change detection
    wasRunning := s.wasRunning[serverID]
    if wasRunning != running {
        if running {
            discord.SendServerOnlineAlert(s.Name, s.Identifier)
        } else {
            discord.SendServerOfflineAlert(s.Name, s.Identifier)
        }
        s.wasRunning[serverID] = running
    }
    
    // Resource thresholds (configurable)
    if cpu > 80 && !alertedCPU {
        discord.SendResourceAlert(s.Name, s.Identifier, "CPU", cpu, 80)
        alertedCPU = true
    }
    // ... RAM, Disk similar
}
```

**Deduplication:** Tracks `serverStateTracking` map to avoid spam.

### Backup Alerts (`internal/servers/server.go:StartBackup`)

```go
func (s *Server) StartBackup() error {
    // ... backup logic ...
    if err != nil {
        discord.SendBackupAlert(s.Name, s.Identifier, "failed", backupName)
    } else {
        discord.SendBackupAlert(s.Name, s.Identifier, "success", backupName)
    }
}
```

### System Status (`internal/services/discord.go:SendSystemStatus`)

Called from `processSystemStatus()` ticker (every 1 minute).

### License Expiry (`internal/services/license.go`)

```go
if daysUntilExpiry <= 7 {
    discord.SendAlert("License Expiring", 
        fmt.Sprintf("License expires in %d days", daysUntilExpiry))
}
```

---

## Creating a Discord Webhook

1. Open Discord → Server Settings → Integrations → Webhooks
2. Click "New Webhook"
3. Select channel
4. Copy Webhook URL
4. Paste in Panel Settings → Notifications

---

## Testing

### Test Endpoint
```bash
curl -X POST http://panel/api/settings/test/discord \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json"
```

### Manual Test (cURL)
```bash
curl -X POST "https://discord.com/api/webhooks/WEBHOOK_ID/WEBHOOK_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "Aether Panel",
    "embeds": [{
      "title": "Test Alert",
      "description": "This is a test from Aether Panel",
      "color": 16776960,
      "fields": [
        {"name": "Test", "value": "Success", "inline": true}
      ],
      "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
    }]
  }'
```

---

## Discord Limits & Best Practices

| Limit | Value | Handling |
|--------|-------|----------|
| Embeds per message | 10 | Panel sends 1 embed per webhook |
| Fields per embed | 25 | System status truncates at 25 |
| Field name length | 256 chars | Truncated if needed |
| Field value length | 1024 chars | Truncated if needed |
| Description length | 4096 chars | Truncated if needed |
| Rate limit | 30 req/min/webhook | Async sending, queue if needed |

**Rate Limit Handling:**
```go
func (s *DiscordService) SendWebhook(...) error {
    // Retry with exponential backoff on 429
    for i := 0; i < 3; i++ {
        resp, err := http.Post(...)
        if resp.StatusCode == 429 {
            retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
            time.Sleep(retryAfter)
            continue
        }
        return err
    }
    return err
}
```

---

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| No notifications | Webhook URL wrong | Verify URL in settings, test endpoint |
| Rate limited | Too many alerts | Reduce alert frequency, check rate limits |
| Missing embed | Field too long | Check field value lengths (<1024) |
| Wrong color | Color decimal | Use decimal (16711680 = red) |
| Missing webhook | Not configured | Add URL in Settings → Notifications |

---

## API Reference

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| POST | `/api/settings/test/discord` | `settings.edit` | Send test webhook |

**Request:**
```json
{
  "webhookType": "general" // or "system", "node", "extransfer"
}
```

---

## Security

- Webhook URLs stored in config (not DB)
- URLs masked in logs (show `.../xxx/yyy`)
- No sensitive data in embeds
- HTTPS only (Discord requirement)