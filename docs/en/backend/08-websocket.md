# WebSocket — Real-time Console

Each server has one WebSocket that pushes its console output, its resource usage and its state changes as they happen. It is **one-way**: the browser only listens. Commands are sent with `POST /api/servers/:serverId/console`, and the console history is read with `GET /api/servers/:serverId/console` (see the [API reference](../reference/11-api-reference.md)).

Code: `internal/web/api/servers.go` (panel endpoint and proxy), `internal/web/daemon/server.go` (`openSocket`), `internal/services/node.go` (`OpenSocket`, remote nodes), `pkg/skypanel/tracker.go` (sockets and trackers), `pkg/skypanel/message.go` (message types), `internal/middleware/security.go` (origin check).

---

## Endpoints

| Endpoint | Who calls it | Authentication |
|----------|--------------|----------------|
| `GET /api/servers/:serverId/socket` | the browser (or any API client) | panel session cookie `skypanel_auth` or `Authorization: Bearer <token>`, plus the `server.view` scope on that server |
| `GET /daemon/server/:serverId/socket` | the panel, never the browser | `Authorization: Bearer <JWT>` signed by the panel for that node (see [Security](./09-security.md#panel-and-node-tokens)) |

The frontend opens `ws(s)://<panel>/api/servers/<id>/socket?console` after loading the console history over HTTP, and reconnects with exponential back-off (1 s, 2 s, 4 s… up to 15 s) when the connection drops.

---

## Which streams a client receives

A socket can carry three streams. On the panel endpoint **the query string from the client is discarded**: the panel works out the streams from the user's permissions and builds the query for the daemon itself, so nobody can ask for more than they may see.

| Stream | Granted when the user has | Message `type` |
|--------|---------------------------|----------------|
| `console` | `server.console` | `console` |
| `status` | `server.status` | `status`, `crash` |
| `stats` | `server.stats` | `stat` |

The scopes are the ones `middleware.RequiresPermission` resolved for this request (`callerScopes` in `websocketPathWithGrants`), as on every other route: the user's rows on that server and global, the owner scopes when they own it, their role's panel-wide scopes, and, for a session opened by an OAuth2 client, only what the client may use. `admin` and `server.admin` count as every scope. A user without any of the three still gets a connection, just with no messages.

On the daemon endpoint the streams are chosen by the presence of query keys: `?console&status&stats` (values are ignored, so `?console=1` also works). A comma-separated list such as `?console,stats` is **not** understood.

---

## Path through the panel

1. The browser connects to `/api/servers/:serverId/socket`. Authentication, the `server.view` check and the [origin check](#origin-check) run first.
2. The panel generates a short-lived JWT for the server's node and rewrites the path to `/daemon/server/<id>/socket?<granted streams>`.
3. **Local node** (panel and daemon in the same process): the request is handed back to the router and served by the daemon handler directly.
4. **Remote node**: the panel accepts the browser's WebSocket, dials `ws://` or `wss://` `<node private host>:<node private port>/daemon/server/<id>/socket?...` with `Authorization: Bearer <JWT>`, and copies frames in both directions until either side closes. The browser's cookies are never forwarded to the node.
5. If the server exists in the database but its files are missing on the local node, the panel accepts the socket, sends one console message saying the server's files have disappeared, and closes it a second later.

`daemon.console.forward` has nothing to do with this: it only copies server consoles to the daemon's standard output.

---

## Messages (server → client)

All messages are **text frames** containing JSON in the `Transmission` shape:

```json
{ "type": "<console|stat|status|crash>", "data": { ... } }
```

There are no binary frames.

### `console`

```json
{ "type": "console", "data": { "epoch": 0, "logs": "W1NlcnZlcl0gRG9uZSAoMy4ycykh" } }
```

`data` is a `ServerLogs`. `logs` is the raw console output as a byte array, which JSON encodes as **base64**: decode it, then split on newlines (a message can hold several lines, or part of one). Lines written by the daemon itself start with `[DAEMON] `. On live messages `epoch` is not filled in (`0`); it is meaningful in the HTTP history response.

### `stat`

Sent every 5 seconds for each server whose stats can be read.

```json
{
  "type": "stat",
  "data": {
    "cpu": 45.2,
    "maxCpu": 200,
    "memory": 1536000000,
    "maxMemory": 2147483648,
    "storage": 1024000000,
    "maxStorage": 10737418240,
    "networkRx": 1024,
    "networkTx": 2048,
    "jvm": { "heapUsed": 512, "heapTotal": 1024, "metaspaceUsed": 64, "metaspaceTotal": 128 },
    "running": true
  }
}
```

| Field | Meaning |
|-------|---------|
| `cpu` | CPU in percent of **one core** (200 = two cores busy) |
| `maxCpu` | What the server may use in the same unit: its CPU limit, or every core of the node when it has none |
| `memory`, `maxMemory` | Bytes |
| `storage`, `maxStorage` | Bytes |
| `networkRx`, `networkTx` | Network counters reported by the environment |
| `jvm` | Only present when the server reports JVM data |
| `running` | Whether the process is running |

Divide `cpu` by `maxCpu` to get the 0–100 % figure the panel shows everywhere.

### `status`

```json
{ "type": "status", "data": { "running": true, "installing": false } }
```

Sent when the process starts or stops, by the Docker and host environments.

### `crash`

```json
{ "type": "crash", "data": { "id": "<report id>", "exitCode": 1, "restarted": false } }
```

Sent on the **status** stream when a server dies on its own. The full crash report (with the last console lines) is fetched separately from the API; the frontend uses this message to show the crash toast.

---

## Messages (client → server)

None are processed. The daemon reads incoming frames only to keep the connection alive and discards their content; frames larger than 1024 bytes close the connection. To send a command use `POST /api/servers/:serverId/console` with the command as the body (scope `server.console.send`).

---

## Keep-alive and cleanup

`Socket.Serve()` runs on the daemon side of every connection (for remote nodes, on the panel↔node leg):

- it sends a WebSocket ping every **25 s**;
- if no pong (or other frame) arrives within **60 s**, the connection is closed;
- each write has a 10 s deadline; a socket whose write fails is dropped from its streams;
- when the connection ends, the socket is removed from every tracker it was registered on.

Internally each server environment has three trackers (`ConsoleTracker`, `StatusTracker`, `StatsTracker`). Console output, status changes and the 5-second stats loop write to them, and each tracker fans the message out to the sockets registered on it.

---

## Console history

The live socket does **not** replay old output. The history lives in a per-server in-memory buffer of `daemon.console.buffer` KB (default 50; oldest output is dropped first) and is read over HTTP:

```
GET /api/servers/:serverId/console?time=<microseconds>
→ { "epoch": 1730000000123456, "logs": "<base64>" }
```

`epoch` and `time` are Unix time in **microseconds**. Without `time` (or with `0`) the whole buffer is returned; with it, only what was written after that moment. Pass the returned `epoch` as `time` next time to fetch only new output. Requires `server.console`.

---

## Origin check

The upgraders accept any `Origin` themselves; cross-site protection comes from the global `SameOriginGuard` middleware, which treats a WebSocket handshake as a state-changing request even though it is a `GET`:

- It only applies when the request carries a panel cookie (`skypanel_auth` or `session`). Requests authenticated with a bearer token, the daemon's own calls and OAuth2 clients are not affected.
- The request passes if the browser's `Sec-Fetch-Site` header is `same-origin` or `none`. Without that header, the `Origin` host must match the request's `Host`, `X-Forwarded-Host` or the host of `panel.settings.masterUrl`. A request with neither header did not come from a web page and passes.
- Anything else gets `403 {"error": "cross-origin request refused"}`.

This matters because browsers treat every port of the same IP as the same *site*: without the check, a web page served by a game server at `http://IP:25566` could open a socket to the panel at `http://IP:8080` with the visitor's cookies and read consoles. If the panel sits behind a reverse proxy that changes the host name, make sure it forwards `Host` or `X-Forwarded-Host`, or set `panel.settings.masterUrl` to the public URL.

---

## Example client

```js
const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
const ws = new WebSocket(`${proto}//${location.host}/api/servers/${serverId}/socket`);
const decoder = new TextDecoder();

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  switch (msg.type) {
    case 'console': {
      const bytes = Uint8Array.from(atob(msg.data.logs), (c) => c.charCodeAt(0));
      decoder.decode(bytes).split(/\r?\n/).filter(Boolean).forEach((line) => console.log(line));
      break;
    }
    case 'stat':
      console.log('CPU %', (msg.data.cpu / (msg.data.maxCpu || 100)) * 100);
      break;
    case 'status':
      console.log('running:', msg.data.running);
      break;
    case 'crash':
      console.warn('crashed with exit code', msg.data.exitCode);
      break;
  }
};
```

---

## Related

- [RCON and Telnet](./12-rcon-telnet.md): console connections for games that expose RCON instead of standard input. RCON over WebSocket (`rconws`, `internal/connections/rconws.go`) is a third network input type.
- [Process metrics](./13-process-metrics.md)
