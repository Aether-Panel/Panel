# API Consumption from the Frontend

The frontend does not have a separate packaged SDK. All HTTP calls are made through a shared `api` object defined in `src/lib/api-client.ts`.

## HTTP Client (`api-client.ts`)

It exports an `api` object with 4 methods that use native `fetch`:

```typescript
import { api } from '@/lib/api-client';

// GET  — retrieve resources
const servers = await api.get('/api/servers');

// POST — create or execute action
await api.post('/api/servers/abc123/start', {});

// PUT  — create or replace (backend use: create server)
await api.put('/api/servers/abc123', { name, ... });

// DELETE — delete
await api.delete('/api/servers/abc123');
```

Features:
- Uses `credentials: 'include'` — authentication is handled by session cookies.
- Does not use axios or js-cookie.
- HTTP errors throw `ApiError` with `status`, `message`, and `data` (parsed from JSON).
- There are no separate domain-specific classes (`AuthApi`, `ServerApi`, etc.). Each feature calls `api.get/post/put/delete` directly.

## Domain-specific Hooks

Instead of classes, the frontend uses custom hooks that encapsulate the logic for each domain. The hooks are located in `src/hooks/`:

| Hook | Rutas que consume | Purpose |
|---|---|---|
| `use-auth` (in `providers.tsx`) | `POST /auth/login`, `POST /auth/logout`, `POST /auth/register`, `GET /api/self` | Authentication, session, scopes |
| `use-servers` | `GET /api/servers`, `PUT /api/servers/:id`, `DELETE /api/servers/:id`, `POST /api/servers/:id/suspend`, `PUT /api/servers/:id/name/:name` | Server CRUD |
| `use-users` | `GET /api/users`, `POST /api/users`, `GET /api/users/:id`, `POST /api/users/:id`, `DELETE /api/users/:id`, `GET /api/users/:id/perms`, `PUT /api/users/:id/perms` | Global users |
| `use-nodes` | `GET /api/nodes`, `POST /api/nodes`, `GET /api/nodes/:id`, `PUT /api/nodes/:id`, `DELETE /api/nodes/:id`, `GET /api/nodes/:id/features`, `GET /api/nodes/:id/system`, `GET /api/nodes/:id/deployment` | Nodes |
| `use-templates` | `GET /api/templates`, `POST /api/templates`, `GET /api/templates/:repo`, `DELETE /api/templates/:repo`, `GET /api/templates/:repo/:name`, `DELETE /api/templates/0/:name`, `PUT /api/templates/0/:name` | Templates |
| `use-database-hosts` | `GET /api/databasehosts`, `POST /api/databasehosts`, `GET /api/databasehosts/:id`, `PUT /api/databasehosts/:id`, `DELETE /api/databasehosts/:id` | Database hosts |
| `use-settings` | `GET /api/settings`, `POST /api/settings`, `GET /api/settings/:key`, `PUT /api/settings/:key`, `POST /api/settings/test/email`, `POST /api/settings/test/discord`, `POST /api/settings/license/activate` | Global settings |
| `use-server-settings` | `GET /api/servers/:serverId/data`, `POST /api/servers/:serverId/data`, `PUT /api/servers/:serverId/data` | Server variables |
| `use-profile` | `GET /api/self`, `PUT /api/self`, `GET /api/self/otp`, `POST /api/self/otp`, `PUT /api/self/otp`, `POST /api/self/otp/recovery`, `DELETE /api/self/otp/:token`, `GET /api/self/oauth2`, `POST /api/self/oauth2`, `DELETE /api/self/oauth2/:clientID` | Own profile |
| `use-dashboard-data` | `GET /api/uptime`, `GET /api/servers` | Dashboard data |
| `use-toast` | — | Toast notifications (ui) |

## Direct Calls from Features

Real-time server operations (console, files, backups, plugins, flags, tasks, statistics) are made via direct calls to `api.get/post/put/delete` from components in `src/features/servers/`:

### Console WebSocket
```typescript
const ws = new WebSocket(`${protocol}//${host}/api/servers/${id}/socket?console`);
ws.onmessage = (event) => { /* console data, stats, status */ };
ws.send(JSON.stringify({ type: 'console', data: command }));
```

### Files
```typescript
api.get(`/api/servers/${id}/file/${path}`)      // Download
api.put(`/api/servers/${id}/file/${path}`, body)  // Upload
api.delete(`/api/servers/${id}/file/${path}`)     // Delete
```

### Backups
```typescript
api.get(`/api/servers/${id}/backup`)                        // List
api.post(`/api/servers/${id}/backup/create`, {})            // Create
api.post(`/api/servers/${id}/backup/restore/${backupID}`, {}) // Restore
api.get(`/api/servers/${id}/backup/download/${backupID}`)   // Download
api.delete(`/api/servers/${id}/backup/${backupID}`)         // Delete
```

### Plugins
```typescript
api.get(`/api/servers/${id}/plugins`)                       // List
api.get(`/api/servers/${id}/plugins/search?q=...`)          // Search
api.post(`/api/servers/${id}/plugins/${pluginId}`, {})      // Install
api.delete(`/api/servers/${id}/plugins`, { body: ... })     // Delete
```

### Server Actions
```typescript
api.post(`/api/servers/${id}/start`, {})    // Start
api.post(`/api/servers/${id}/stop`, {})     // Stop
api.post(`/api/servers/${id}/restart`, {})  // Restart
api.post(`/api/servers/${id}/kill`, {})     // Kill
api.post(`/api/servers/${id}/install`, {})  // Install
api.post(`/api/servers/${id}/reload`, {})   // Reload
```