# Uptime Tracking System

Aether Panel tracks server uptime/downtime with per-second precision, providing statistics, history, and Discord alerts.

---

## Overview

The uptime system records every state transition (running ↔ stopped) and provides:
- Real-time uptime percentage
- Historical records with duration
- Active record tracking (current running session)
- Discord notifications on state changes
- Aggregate stats for dashboard

---

## Data Model

### UptimeStatus (`internal/models/uptime.go`)

```go
type UptimeStatus struct {
    ID          uint      `gorm:"primaryKey"`
    ServerID    string    `gorm:"index;size:20"`
    IsRunning   bool      // true = running, false = stopped
    StartTime   time.Time // When state started
    EndTime     *time.Time // When state ended (nil = active)
    Duration    int64     // Seconds (calculated on close)
}
```

**Indexes:**
- `idx_uptime_status_server_id` on `server_id`
- Composite: `(server_id, start_time)`

**Relationships:**
- `ServerID` → `Server.Identifier` (FK, CASCADE DELETE)

---

## Tracking Logic (`internal/services/uptime.go`)

### TrackStatus()

Called every 5 seconds by `processStats()` in `server.go`:

```go
func (s *UptimeService) TrackStatus(serverID string, isRunning bool) (uptimeSeconds, downtimeSeconds int64, uptimePercent float64, err error)
```

**Logic:**
1. Find active record (`end_time IS NULL`) for server
2. If state changed (running ↔ stopped):
   - Close previous record: set `EndTime = now`, `Duration = now - StartTime`
   - Create new record with `StartTime = now`, `IsRunning = newState`
3. If first record ever: create new record
4. If state unchanged: no-op
5. Returns: `uptimeSeconds, downtimeSeconds, uptimePercent`

**Edge Cases:**
- Server deleted: cascade deletes uptime records
- Panel restart: active records preserved, continues tracking
- Time drift: uses `time.Now()` (monotonic not required)

---

## Statistics (`GetUptimeStats`)

```go
func (s *UptimeService) GetUptimeStats(serverID string, since time.Time) (uptimeSeconds, downtimeSeconds int64, uptimePercent float64, err error)
```

**Parameters:**
- `serverID`: Server identifier
- `since`: Time threshold (records after this time)

**Returns:**
| Return Value | Type | Description |
|--------------|------|-------------|
| `uptimeSeconds` | `int64` | Total running seconds |
| `downtimeSeconds` | `int64` | Total stopped seconds |
| `uptimePercent` | `float64` | Uptime percentage (0-100) |

**Calculation:**
```go
for _, record := range records {
    if record.IsRunning {
        totalUptime += record.Duration
    } else {
        totalDowntime += record.Duration
    }
}

// Active record handling
if activeRecord != nil && activeRecord.IsRunning {
    activeDuration := time.Since(activeRecord.StartTime)
    totalUptime += activeDuration
}

total := totalUptime + totalDowntime
if total > 0 {
    uptimePercent = float64(totalUptime) / float64(total) * 100
}
```

**Note:** No `UptimeStats` struct - returns multiple values directly.

---

## API Endpoints

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| GET | `/api/uptime` | `admin` or `uptime.view` | All servers uptime |
| GET | `/api/uptime/:serverId` | `server.view` | Single server uptime |

**Query Parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `days` | int | 30 | Days of history |
| `limit` | int | 100 | Max records |

**Response (Single Server):**
```json
{
  "uptime_seconds": 7200000,
  "downtime_seconds": 3600000,
  "uptime_percent": 66.67
}
```

**Response (All Servers):**
```json
{
  "servers": {
    "abc123": { "uptime_percent": 99.5, "uptime_seconds": 7200000, "downtime_seconds": 3600000 },
    "def456": { "uptime_percent": 45.2, "uptime_seconds": 3600000, "downtime_seconds": 7200000 }
  }
}
```

---

## Integration Points

### Server Process (`internal/servers/server.go`)

```go
// In processStats() - called every 5s
func (s *ServerService) processStats() {
    for _, server := range servers {
        // ... get stats ...
        
        // Track uptime
        if uptimeService := services.NewUptimeService(); uptimeService != nil {
            uptimeService.TrackStatus(server.Identifier, running)
        }
        
        // ... rest of stats processing ...
    }
}
```

**Called from:** `processStats()` ticker (every 5 seconds)

---

### Server Lifecycle Events

| Event | TrackStatus Called With |
|-------|------------------------|
| Start | `isRunning = true` |
| Stop | `isRunning = false` |
| Kill | `isRunning = false` |
| Crash | `isRunning = false` (via `afterExit`) |
| Install | `isRunning = false` |
| Suspend | `isRunning = false` |

---

## Active Record Handling

An **active record** has `EndTime = NULL` and represents the current state.

**On Query:**
```go
// If active record is running, add elapsed time to uptime
if activeRecord != nil && activeRecord.IsRunning {
    activeDuration := time.Since(activeRecord.StartTime)
    totalUptime += activeDuration
}
```

**On State Change:**
```go
// Close active record
db.Model(&UptimeStatus{}).Where("server_id = ? AND end_time IS NULL", serverID).
    Updates(map[string]interface{}{
        "end_time": time.Now(),
        "duration": int64(time.Since(activeRecord.StartTime).Seconds()),
    })

// Create new record
db.Create(&UptimeStatus{
    ServerID:  serverID,
    IsRunning: newState,
    StartTime: time.Now(),
})
```

---

## Discord Notifications (`internal/services/discord.go`)

```go
func (s *DiscordService) SendServerOfflineAlert(serverName, serverID string) error
func (s *DiscordService) SendServerOnlineAlert(serverName, serverID string) error
```

**Triggered in:** `checkServerAlerts()` when `wasRunning != running`

**Embed Example (Offline):**
```json
{
  "title": "🔴 Server Offline",
  "description": "Server **MyServer** (`abc123`) went offline",
  "color": 16711680,
  "fields": [
    { "name": "Server", "value": "MyServer", "inline": true },
    { "name": "ID", "value": "abc123", "inline": true },
    { "name": "Time", "value": "2024-01-15 14:30:00 UTC", "inline": true }
  ],
  "timestamp": "2024-01-15T14:30:00Z"
}
```

**Deduplication:** Only sends on state change (not every 5s tick)

---

## Database Queries

### Get Active Record
```sql
SELECT * FROM uptime_status 
WHERE server_id = ? AND end_time IS NULL 
ORDER BY start_time DESC LIMIT 1
```

### Get History (with limit)
```sql
SELECT * FROM uptime_status 
WHERE server_id = ? AND start_time >= ? 
ORDER BY start_time DESC LIMIT ?
```

### Aggregate Stats (Single Query)
```sql
SELECT 
    SUM(CASE WHEN is_running THEN duration ELSE 0 END) as uptime,
    SUM(CASE WHEN is_running THEN 0 ELSE duration END) as downtime,
    COUNT(*) as total_records
FROM uptime_status 
WHERE server_id = ? AND start_time >= ?
```

---

## Frontend Integration

### Dashboard Uptime Cards
- Shows `uptime_percent` with color coding:
  - Green: ≥ 99%
  - Yellow: 95-99%
  - Red: < 95%

### Server View
- Current status badge (🟢 Running / 🔴 Stopped)
- Uptime percentage with trend
- Recent history table (last 10 records)

### Uptime Page (`/uptime/`)
- All servers table with sortable uptime %
- Date range picker
- Export to CSV

---

## API Examples

```bash
# All servers uptime (30 days)
curl -H "Authorization: Bearer $TOKEN" \
  "http://panel/api/uptime?days=30"

# Single server (7 days, 50 records)
curl -H "Authorization: Bearer $TOKEN" \
  "http://panel/api/uptime/abc123?days=7&limit=50"
```

```python
import requests

r = requests.get(
    "http://panel/api/uptime/abc123",
    headers={"Authorization": f"Bearer {token}"},
    params={"days": 7, "limit": 50}
)
stats = r.json()
print(f"Uptime: {stats['uptime_percent']:.2f}%")
print(f"Current: {'Running' if stats['current_status'] else 'Stopped'}")
```