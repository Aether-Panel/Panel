# Server overview and statistics

The **Overview** tab of a server shows how it is doing now and how it behaved while nobody was watching: live resource usage, who is playing, a performance history of up to a week, the busiest hours of the week and the latest crashes. Owners use it to size their plan, spot lag and pick a good time for events or restarts.

## What it does

The tab is made of these cards, top to bottom:

| Card | Shows | Source |
|---|---|---|
| **Live Resources** | CPU, memory and storage against the server's limits | `GET /api/servers/:id/stats`, every 5 s |
| **Players Online** | Players online / slots, a bar, the player names when the game shares them, and the game, map or version | `GET /api/servers/:id/query`, every 5 s |
| **Performance history** | Charts for the last **1 h**, **24 h** or **7 days**, with three views: **CPU and memory**, **Network**, **Players** | `GET /api/servers/:id/metrics`, refreshed every minute |
| **Peak hours** | Average players for each weekday and hour over the last week, in the viewer's local time | `GET /api/servers/:id/metrics?range=7d` |
| **CPU & Memory Usage** and **Network Traffic** | Live line charts since the page was opened, one sample every 5 s | The same 5 s polling as **Live Resources** |
| **Crash history** | Latest crashes with the console lines before them | See [Crash reports](./crash-reports.md) |

**Performance history** also shows summary figures above the chart:

- **CPU and memory**: **Average CPU**, **CPU peak**, **Average memory**, **Availability**.
- **Network**: **Average download**, **Average upload**, **Availability**.
- **Players**: **Average players**, **Player peak**, **All-time record** (with its date), **Availability**.

In the chart, grey areas are time offline and red lines mark crashes. A step with no data at all ("No data: the node was down.") is left blank and counts as offline.

**Peak hours** highlights the busiest cell ("Busiest: Saturday at 21:00 (12.4 on average)"); empty cells (dashed border) have no data, grey ones had nobody online. The card is hidden for servers that never reported players.

## How it works

### Live figures

While the server page is open and the browser tab is visible, the web asks the node every 5 seconds for the server's stats and its player query. The live charts keep the last 60 samples (about 5 minutes) and are lost when the page is closed. CPU and memory are shown as a share of the server's limits.

### Performance history (stored on the node)

The node already samples every server every 5 seconds for its alerts. Those samples are folded into:

- **one-minute points**, kept for **24 hours**;
- **fifteen-minute points**, kept for **7 days**.

CPU, memory and network are averaged over the samples taken while the server was running, so an hour spent half stopped does not halve them. **Availability** is the share of samples taken while the server was running; gaps when the node itself was down count as offline.

The panel offers three windows, each averaged to its own step:

| Range | Step |
|---|---|
| `1h` | 1 minute |
| `24h` (default) | 5 minutes |
| `7d` | 1 hour |

The history is written to `<daemon.data.servers>/<server id>.metrics` every 5 minutes and when the node shuts down, so a crash of the node loses at most the last few minutes. The file is deleted together with the server.

### Player counts

Every minute the node asks each running server who is online and adds the answer to the history (average, peak and slots per step). The all-time record (most players at once, and when) is kept in the same file. How it asks depends on the template's `query` type:

| Query type | Protocol | Port |
|---|---|---|
| `minecraft` | Minecraft server list ping | The game port |
| `source` or `steam` | Valve A2S | The template's query port variable (`queryport`, `query_port`, `queryPort`, `steamqueryport` or `steam_query_port`), or the game port |
| not set | A2S is tried; if the game ignores it, it is not asked again for 2 minutes | As above |

Answers are cached for 10 seconds, shared between the live card and the history. Bots on Steam servers are not counted as players.

## Using it

### In the panel

Open a server and go to **Overview**. The tab is always listed, but its figures come from routes that need `server.stats` on the server (the owner holds it; sub-users only if the owner gave it). In **Performance history**, choose the view with **What to show** and the window with **Period**. Hover a point to read its values.

### Via the API

| Method and path | Scope | Purpose |
|---|---|---|
| `GET /api/servers/:id/stats` | `server.stats` | Current CPU, memory, disk and network |
| `GET /api/servers/:id/query` | `server.stats` | Who is playing: `{"minecraft": {...}}` or `{"source": {...}}`; `204` when the server is off or does not answer |
| `GET /api/servers/:id/metrics?range=1h\|24h\|7d` | `server.stats` | Performance history |

Response of `/metrics`:

```json
{
  "range": "24h",
  "step": 300,
  "points": [
    { "t": 1730448000, "cpu": 23.5, "memory": 812000000, "maxMemory": 2147483648,
      "rx": 15320, "tx": 40211, "online": 1,
      "players": 4.2, "playersPeak": 7, "maxPlayers": 20, "hasPlayers": true }
  ],
  "record": { "players": 31, "at": 1729980000 }
}
```

- `t` is the start of the step (Unix seconds); `online` goes from 0 to 1.
- A point with `"noData": true` is a step with nothing recorded (the node was down).
- `hasPlayers` is `false` when the game did not answer any query in that step.

More details in the [API reference](../reference/11-api-reference.md).

## Requirements and configuration

- Nothing to turn on: the history is recorded on every node for every server.
- Player counts need a game that answers the query: for Minecraft, `enable-status` must stay `true` in `server.properties` (it is by default); for Steam games, the query port must be reachable from the node.
- The history file lives in the servers folder:

| Key | Default | Environment variable |
|---|---|---|
| `daemon.data.servers` | `servers` (inside `daemon.data.root`) | `SKYPANEL_DAEMON_DATA_SERVERS` |

See [Configuration](../backend/02-configuration.md).

## Limits

- The history covers 24 hours at one-minute detail and 7 days at fifteen-minute detail; older data is dropped.
- **Peak hours** uses only the last 7 days.
- The live charts start empty each time the page is opened.
- Polling pauses while the browser tab is hidden.
- Servers whose game does not answer either protocol show no player data.

## Troubleshooting

| What you see | Cause | Fix |
|---|---|---|
| **No history yet** | The node has not saved a point yet (new server or just after an update) | Wait a few minutes |
| **The game did not answer the player query.** | The game does not support the query, the query port is wrong, or (Minecraft) `enable-status=false` | Check the template's query type and port, or turn `enable-status` back on |
| **Players Online** card missing | `/query` answered `204` | Same as above; it also happens while the server is off |
| Long grey gaps | The node (or its daemon) was down | Check the node's uptime |
| `range must be 1h, 24h or 7d` | API call with another range | Use one of the three values |

## See also

- [Crash reports](./crash-reports.md)
- [Dashboard and nodes](./dashboard-and-nodes.md): panel-wide and node-wide charts.
- [Uptime tracking](../general/13-uptime.md)
- [Process metrics](../backend/13-process-metrics.md)
