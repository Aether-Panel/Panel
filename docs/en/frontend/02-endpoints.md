# API Consumption from the Frontend

The frontend has no generated SDK and no domain classes. Every HTTP call goes through the small `api` object in `src/lib/api-client.ts`, either directly from a feature component or through a hook in `src/hooks/`. The panel API itself is documented in the API reference (Swagger at `/swagger/`, administrators only); this page only covers how the frontend uses it.

## HTTP client (`lib/api-client.ts`)

```typescript
import { api, ApiError } from '@/lib/api-client';

const servers = await api.get('/api/servers?limit=100&page=1');
await api.post(`/api/servers/${id}/start`, {});
await api.post(`/api/nodes/${nodeId}/ports/check`, { ports }, controller.signal); // optional AbortSignal
await api.postForm('/api/templates/upload', formData);                          // multipart, no JSON header
await api.put(`/api/servers/${id}`, payload);
await api.delete(`/api/servers/${id}/backup/${backupId}`);
```

| Method | Body | Notes |
|---|---|---|
| `get<T>(url)` | — | `Accept: application/json` |
| `post<T>(url, body, signal?)` | JSON | The only method that accepts an `AbortSignal`. |
| `postForm<T>(url, formData)` | `FormData` | For uploads (templates, extensions). |
| `put<T>(url, body)` | JSON | |
| `delete<T>(url)` | — | |

All of them use the browser's `fetch` with `credentials: 'include'`, so the session cookie authenticates every request. The response is parsed as JSON when the `Content-Type` says so, otherwise returned as text.

### Errors

A non-2xx response throws `ApiError` (`status`, `message`, `data` = the parsed body). The message is chosen like this:

1. If the body is `{ "error": "text" }`, that text.
2. If the body is `{ "error": { "code": "ErrSomething", "msg": "..." } }` (the panel's structured errors), the translation of `code` under `apiErrors` in the viewer's language (read from `localStorage` key `aether_panel_language`, English by default) when one exists; otherwise the server's `msg`.
3. Otherwise the HTTP status text.

The codes translated today are listed in [05. Translations](./05-translations.md#localized-api-errors).

### Expired sessions

When a request answers **401** while the user is on a signed-in page, the client assumes the session ended (expired or closed elsewhere): it stores the current path in `sessionStorage` (`aether:return-to`) and sends the browser to `/login/?expired=1`, which shows a "session expired" notice. After signing in, `AuthProvider` returns to the stored page if it is one of the panel's own paths. A 401 from `/auth/*` calls (wrong password) or on the auth pages themselves does not trigger this. The return path is never read from the URL, so a crafted link cannot redirect after login.

Some components call `fetch` directly instead of `api` when they need the raw response: file uploads and saves in the file manager, folder creation (`?folder`), rename/move (`POST …/file/<path>?destination=`), external transfers, sending console commands and the "AI Console Analysis" card.

## Paginated lists (`lib/paged.ts`)

`/api/servers` and `/api/users` answer 20 items unless asked for more. `fetchAllPages(url, key)` requests pages of 100 (`limit=100&page=N`) until it has `paging.total` items, with a safety stop of 50 pages. `useServers` and `useUsers` use it so pickers and lists see every item.

## Data hooks (`src/hooks/`)

| Hook | Endpoints | Returns |
|---|---|---|
| `useServers(statsFor?)` (`use-servers.ts`) | `GET /api/servers` (all pages), `GET /api/uptime`, then every 5 s per server `GET /api/servers/:id/stats` and `GET /api/servers/:id/query` | `{ servers, loading, error, refresh }`. `statsFor` limits polling to some servers (the detail page polls only its own). Polling pauses while the tab is hidden and skips a tick if the previous round is still running. |
| `useNodes(skip?)` (`use-nodes.ts`) | `GET /api/nodes` | `{ nodes, loading, error, refresh }` |
| `useNodes(skip?)` (`use-dashboard-data.ts`) | `GET /api/nodes` + `GET /api/nodes/:id/system` for each node, every 5 s while visible | Nodes with `systemInfo`; used by the dashboard. |
| `useUsersCount(skip?)` | `GET /api/users?limit=1` | `{ count, loading }` from `paging.total`. |
| `useGlobalNetworkMetrics(nodes)` | — (derived) | Live KB/s across all nodes, computed from the cumulative counters in each node's `systemInfo`. |
| `useNetworkHistory(range, enabled)` | `GET /api/nodes/network-history?range=1h\|6h\|24h`, refreshed every 60 s while visible | `{ samples, stepMs, nodes, unavailable, loading, error }` (see [07](./07-components-detailed.md#dashboard-featuresdashboard)). |
| `useAccessLog(limit?, skip?)` | `GET /api/activity?limit=N` | Recent sign-ins, failed sign-ins and registrations. |
| `useUsers(skip?)` | `GET /api/users` (all pages) | `{ users, loading, error, refresh }` |
| `useDatabaseHosts()` | `GET/POST /api/databasehosts`, `PUT/DELETE /api/databasehosts/:id` | `{ hosts, loading, error, createHost, updateHost, deleteHost, refresh }` |
| `useTemplates()` | `GET /api/templates`, `GET /api/templates/:repo`, `GET /api/templates/:repo/:name`, `PUT /api/templates/0/:name`, `POST /api/templates/upload`, `DELETE /api/templates/0/:name` | `{ repos, loading, error, refresh, getTemplatesForRepo, getTemplateDetails, saveTemplate, uploadTemplates, deleteTemplate }` |
| `useSettings()` | `GET/POST /api/settings`, `POST /api/settings/test/email`, `POST /api/settings/test/discord` | `{ settings, loading, saving, error, saveSettings, sendTestEmail, sendTestDiscord, refresh }` |
| `useServerSettings(serverId)` | `GET/POST/PUT /api/servers/:id/data`, `GET/POST /api/servers/:id/flags`, `GET/PUT /api/servers/:id/definition`, `PUT /api/servers/:id/name/:name`, `GET /api/servers/:id/file/server.properties` | `{ settings, loading, error, saveSettings, saveDefinition, isMinecraftJava, refresh }` |
| `useProfile()` | `PUT /api/self`, `GET/POST/PUT /api/self/otp`, `DELETE /api/self/otp/:token`, `POST /api/self/otp/recovery`, `GET/POST /api/self/oauth2`, `DELETE /api/self/oauth2/:clientId` | `{ otpEnabled, oauthClients, loading, error, updateDetails, startOtpEnroll, validateOtpEnroll, disableOtp, regenerateRecoveryCodes, createOauthClient, deleteOauthClient, refresh }` |
| `useActiveExtensions()` | `GET /api/extensions/active` | `{ extensions, ready }`; `pagesFor(extensions, 'admin' \| 'server')` picks the pages to show. |
| `useIsMobile()` | — | `true` below 768 px. |

Authentication calls live in `AuthContext` (`contexts/providers.tsx`), not in a hook file: `GET /api/self`, `POST /auth/login`, `POST /auth/otp`, `POST /auth/register`, `POST /auth/logout`. The password screens call `POST /auth/forgot-password` and `POST /auth/reset-password`, and the captcha hook calls `GET /auth/challenge` (see [06](./06-pages.md#authentication-pages)). `ConfigContext` calls `GET /api/config`.

## Server detail calls

The server page (`features/servers/[id]/`) calls the API directly from each tab. The main groups:

| Area | Endpoints used |
|---|---|
| Power | `POST /api/servers/:id/start`, `/stop`, `/restart`, `/kill`, `/install` |
| Console | `GET /api/servers/:id/console` (history), the WebSocket below, and `POST /api/servers/:id/console` with the command as plain-text body |
| Crashes | `GET /api/servers/:id/crashes` |
| History | `GET /api/servers/:id/metrics?range=1h\|24h\|7d` |
| Files | `GET/PUT/DELETE /api/servers/:id/file/<path>`, `PUT …?folder`, `POST …?destination=<new path>` (rename/move), `POST /api/servers/:id/extract/<path>?destination=…&skipRoot`, `POST /api/servers/:id/archive/<path>` (compress), `GET/POST /api/servers/:id/file-history/<path>…` (versions and restore) |
| Backups | `GET /api/servers/:id/backup`, `POST /api/servers/:id/backup/create?name=…`, `POST /api/servers/:id/backup/restore/:backupId`, `DELETE /api/servers/:id/backup/:backupId` |
| Plugins | `GET /api/servers/:id/plugins`, `GET /api/servers/:id/plugins/search?q=…`, `POST /api/servers/:id/plugins/:pluginId?loaders=…&game_versions=…` (filter taken from `GET /api/servers/:id/map`), `DELETE /api/servers/:id/plugins?name=…` |
| Tasks | `GET /api/servers/:id/tasks`, `PUT/DELETE /api/servers/:id/tasks/:taskId`, `POST /api/servers/:id/tasks/:taskId/run` |
| Sub-users | `GET /api/servers/:id/user`, `PUT/DELETE /api/servers/:id/user/:email` |
| Databases | `GET/POST /api/servers/:id/databases`, `DELETE /api/servers/:id/databases/:dbId`, `GET /api/servers/:id/databasehosts` |
| Other | `/notifications`, `/subdomain`, `/version`, `/modpacks`, `/map`, `/clone`, `/transfer`, `/extransfer/*`, `/port-settings`, `/owner/:userId`, `/ai/analyze`, `/ai/chat` |

### Console WebSocket

The page (not the console component) owns the socket:

```typescript
const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
const socket = new WebSocket(`${protocol}//${window.location.host}/api/servers/${id}/socket?console`);
```

- Messages are JSON. `{ type: 'console', data: { logs } }` appends lines; `{ type: 'crash', data }` raises a crash toast and refreshes the crash views; `stat` and `status` messages are ignored here (stats come from polling).
- On close it reconnects with exponential backoff (1 s, 2 s, 4 s… capped at 15 s). A server that answers 404 on `/console` is marked unavailable and no socket is opened.
- The console view does not write to this socket: commands go through `POST /api/servers/:id/console` (see [07](./07-components-detailed.md#console-console-viewtsx)).
