# Uptime Tracking

Aether Panel records when each server is running and when it is stopped, and turns those records into an availability percentage, a recent history and the "health" figure on the dashboard.

---

## Overview

- The panel checks every server's state every **5 seconds** and stores one row per continuous period of "running" or "stopped".
- `GET /api/uptime` returns a summary for every server the caller can see; `GET /api/uptime/:serverId` returns the detail and history of one server.
- The default period is the last **30 days** (maximum 365).
- Online/offline alerts for administrators are sent through Discord and are documented in [Discord and notifications](16-discord.md); they do not depend on these records.

---

## Data model

`UptimeStatus` (`internal/models/uptime.go`), table `uptime_statuses`:

| Column | Type | Description |
|--------|------|-------------|
| `id` | uint | Primary key |
| `server_id` | string (20) | Server identifier (indexed). Not included in JSON |
| `is_running` | bool | `true` = running period, `false` = stopped period |
| `start_time` | time | Start of the period (indexed) |
| `end_time` | time, nullable | End of the period; `NULL` while the period is still open (indexed) |
| `duration` | int64 | Length in seconds, filled in when the period is closed (default `0`) |
| `created_at`, `updated_at` | time | GORM timestamps |

JSON field names: `id`, `isRunning`, `startTime`, `endTime` (omitted while open), `duration`, `createdAt`, `updatedAt`.

When a server is deleted, its uptime rows are deleted explicitly together with the rest of its data (`internal/services/server.go`, `internal/web/api/servers.go`).

---

## How it is recorded

`trackUptimeForAllServers()` in `internal/servers/server.go` is the only place that writes uptime. It runs once at start-up and then on every 5-second stats tick (`processStats()`):

1. It does nothing when `panel.enable` is `false` (a daemon-only node has no panel database). A flag prevents a slow round from overlapping the next one.
2. For each server loaded in this process it asks the environment whether the server is running and calls `Uptime.TrackStatus(serverID, isRunning)` (`internal/services/uptime.go`).

`TrackStatus`:

1. Skips servers that do not exist in the `servers` table yet (avoids foreign-key errors).
2. Looks for the open row (`end_time IS NULL`, newest first).
3. No open row: creates one with the current state and `start_time = now`.
4. State changed: closes the open row (`end_time = now`, `duration = now - start_time` in seconds) and opens a new one with the new state.
5. State unchanged: only refreshes `updated_at`.

> **Scope:** only the servers run by the panel's own process (the local node) are recorded. Servers on remote nodes have no stored history; `/api/uptime` gives them an entry derived from their current status (see below).

A panel restart keeps the open rows and continues them, so the time the panel was down counts towards whatever state was open at that moment.

---

## Statistics

### `GetUptimeStats(serverID, since)`

Returns `uptimeSeconds`, `downtimeSeconds` and `uptimePercent`:

1. Reads the periods that overlap the window: those still open and those that ended after `since`.
2. Clips each period to the window: one that started before `since` only counts from `since` on, and the open one counts until now. The result is added to uptime or downtime depending on its state.
3. `uptimePercent = uptime / (uptime + downtime) * 100`; **100** when there is no data at all.

So a server that was running for the whole window, whose only period started before it, counts as up 100 % of the time, and no period counts for more time than fits in the window.

### `GetRecentHistory(serverID, limit)`

The latest `limit` rows of the server, newest first. It is not filtered by the period.

### `GetAllServerUptime(since)`

The same calculation (with the periods clipped to the window) for all servers at once. Returns a map keyed by server identifier with `uptime`, `downtime`, `uptimePercent` and, when a row is open, `currentStatus` and `currentStartTime`.

---

## API

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/uptime` | `admin` or `uptime.view` | Summary of every visible server |
| GET | `/api/uptime/:serverId` | `server.view` on that server | Detail and history of one server |

Query parameters (values below 1 or not numeric fall back to the default; values above the maximum are capped):

| Parameter | Endpoint | Default | Maximum | Description |
|-----------|----------|---------|---------|-------------|
| `days` | both | 30 | 365 | Period to calculate |
| `limit` | `/api/uptime/:serverId` | 50 | 200 | Number of history rows |

The `uptime.view` scope is described in the [scopes reference](scopes-reference.md).

### `GET /api/uptime`

- Administrators see every server. Other users (with `uptime.view`) only see servers they have access to.
- All servers are read page by page (1000 per query), so no server is dropped from the summary.
- The live state of each server is asked to its node in parallel (at most 16 requests at once, 5-second timeout each). A node that does not answer makes its servers count as stopped.
- Servers with no stored data get a minimal entry: if they are running now they count as running for the whole period (100 %), otherwise as stopped for the whole period (0 %).
- With no servers at all the answer is `{}`.

Response (object keyed by server identifier):

```json
{
  "a1b2c3d4": {
    "serverName": "Survival",
    "nodeName": "LocalNode",
    "uptime": 2505600,
    "downtime": 86400,
    "uptimePercent": 96.67,
    "currentStatus": true,
    "currentStartTime": "2026-10-30T08:12:00Z"
  }
}
```

`nodeName` is `LocalNode` for the local node or a node without a name. `uptime` and `downtime` are seconds.

### `GET /api/uptime/:serverId`

```json
{
  "serverId": "a1b2c3d4",
  "serverName": "Survival",
  "nodeName": "LocalNode",
  "currentStatus": true,
  "currentStartTime": "2026-10-30T08:12:00Z",
  "currentUptime": 7200,
  "uptimeSeconds": 2505600,
  "downtimeSeconds": 86400,
  "totalSeconds": 2599200,
  "uptimePercent": 96.67,
  "period": { "days": 30, "since": "2026-10-02T10:12:00Z", "until": "2026-11-01T10:12:00Z" },
  "history": [
    { "id": 812, "isRunning": true, "startTime": "2026-10-30T08:12:00Z", "duration": 0, "createdAt": "…", "updatedAt": "…" }
  ]
}
```

- `currentStatus` comes from the daemon (`GET /daemon/server/:id/status`); any error counts as stopped.
- `currentUptime` and `currentStartTime` are only filled when the server is running and has an open row.
- `totalSeconds` is `uptimeSeconds + downtimeSeconds`. Both already include the running period, which is not added again.

### Examples

```bash
# Summary of the last 7 days
curl -H "Authorization: Bearer $TOKEN" "https://panel.example.com/api/uptime?days=7"

# One server, 90 days, 100 history rows
curl -H "Authorization: Bearer $TOKEN" "https://panel.example.com/api/uptime/a1b2c3d4?days=90&limit=100"
```

---

## Where the panel shows it

- **Server list and dashboard** (`hooks/use-servers.ts`): the list calls `GET /api/uptime` together with the server list. `currentStatus` sets each server's initial online/offline state (the live stats requests update it afterwards) and `uptimePercent` is kept per server. If the call fails the list still loads, without uptime data.
- **Dashboard status strip** (`features/dashboard/status-strip.tsx`): the "health" figure is the average `uptimePercent` of all servers over 30 days. Colour: green at 99 % or more, amber from 95 % to 99 %, red below 95 %.
- The per-server endpoint (`/api/uptime/:serverId`) is not used by the current frontend; it is available for integrations.

The server performance charts (CPU, memory, network, uptime per hour/day/week) come from the daemon's metrics, not from this table.

---

## Known limitations

- Only servers of the local node have a recorded history; servers on remote nodes get an entry derived from their current status (see [How it is recorded](#how-it-is-recorded)).
- Time while the panel is down counts towards the state that was open at that moment.
