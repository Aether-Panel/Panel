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
func (s *UptimeService) TrackStatus(serverID string, isRunning bool) error
```

**Logic:**
1. Find active record (`end_time IS NULL`) for server
2. If state changed (running ↔ stopped):
   - Close previous record: set `EndTime = now`, `Duration = now - StartTime`
   - Create new record with `StartTime = now`, `IsRunning = newState`
3. If first record ever: create new record
4. No-op if state unchanged

**Edge Cases:**
- Server deleted: cascade deletes uptime records
- Panel restart: active records preserved, continues tracking
- Time drift: uses `time.Now()` (monotonic not required)

---

## Statistics (`GetUptimeStats`)

```go
func (s *UptimeService) GetUptimeStats(serverID string, days int) (*UptimeStats, error)

type UptimeStats struct {
    TotalUptime     time.Duration // Sum of all running durations
    TotalDowntime   time.Duration // Sum of all stopped durations
    UptimePercent   float64       // TotalUptime / (TotalUptime + TotalDowntime) * 100
    CurrentStatus   bool          // Current running state
    CurrentStart    time.Time     // Start of current active record
    Records         []*models.UptimeStatus // All records in period
}
```

**Calculation:**
```go
for _, record := range records {
    if record.IsRunning {
        stats.TotalUptime += record.Duration
    } else {
        stats.TotalDowntime += record.Duration
    }
}

// Active record handling
if activeRecord != nil && activeRecord.IsRunning {
    activeDuration := time.Since(activeRecord.StartTime)
    stats.TotalUptime += activeDuration
    stats.CurrentStatus = true
    stats.CurrentStart = activeRecord.StartTime
}

total := stats.TotalUptime + stats.TotalDowntime
if total > 0 {
    stats.UptimePercent = float64(stats.TotalUptime) / float64(total) * 100
}
```

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
  "totalUptime": 7200000000000,
  "totalDowntime": 3600000000000,
  "uptimePercent": 66.67,
  "currentStatus": true,
  "currentStart": "2024-01-15T10:30:00Z",
  "records": [
    {
      "id": 1,
      "server_id": "abc123",
      "is_running": true,
      "start_time": "2024-01-15T10:30:00Z",
      "end_time": "2024-01-15T12:00:00Z",
      "duration": 5400
    }
  ]
}
```

**Response (All Servers):**
```json
{
  "servers": {
    "abc123": { "uptimePercent": 99.5, "currentStatus": true, ... },
    "def456": { "uptimePercent": 45.2, "currentStatus": false, ... }
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
- Shows `uptimePercent` with color coding:
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

## Troubleshooting

### Uptime Not Tracking
1. Check `processStats()` is running (logs: `[INFO] Processing stats`)
2. Verify `UptimeService.TrackStatus` called (add debug log)
3. Check DB for `uptime_status` records

### Incorrect Percentages
1. Active record not counted → verify `EndTime IS NULL` logic
2. Duration calculation → check `time.Since()` vs `EndTime - StartTime`
3. Timezone issues → all times stored as UTC

### Missing Records
1. Cascade delete on server delete → expected
3. Panel restart during transition → active record preserved

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
print(f"Uptime: {stats['uptimePercent']:.2f}%")
print(f"Current: {'Running' if stats['currentStatus'] else 'Stopped'}")
```