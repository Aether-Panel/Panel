# Consumo de API desde el Frontend

El frontend no tiene un SDK empaquetado separado. Todas las llamadas HTTP se realizan a través de un objeto `api` compartido definido en `src/lib/api-client.ts`.

## Cliente HTTP (`api-client.ts`)

Exporta un objeto `api` con 4 métodos que usan `fetch` nativo:

```typescript
import { api } from '@/lib/api-client';

// GET  — obtener recursos
const servers = await api.get('/api/servers');

// POST — crear o ejecutar acción
await api.post('/api/servers/abc123/start', {});

// PUT  — crear o reemplazar (uso del backend: crear servidor)
await api.put('/api/servers/abc123', { name, ... });

// DELETE — eliminar
await api.delete('/api/servers/abc123');
```

Características:
- Usa `credentials: 'include'` — la autenticación se maneja mediante cookies de sesión.
- No usa axios ni js-cookie.
- Los errores HTTP lanzan `ApiError` con `status`, `message` y `data` (parsed desde JSON).
- No hay clases separadas por dominio (`AuthApi`, `ServerApi`, etc.). Cada feature llama a `api.get/post/put/delete` directamente.

## Hooks por Dominio

En lugar de clases, el frontend usa custom hooks que encapsulan la lógica de cada dominio. Los hooks se encuentran en `src/hooks/`:

| Hook | Rutas que consume | Propósito |
|---|---|---|
| `use-auth` (en `providers.tsx`) | `POST /auth/login`, `POST /auth/logout`, `POST /auth/register`, `GET /api/self` | Autenticación, sesión, scopes |
| `use-servers` | `GET /api/servers`, `PUT /api/servers/:id`, `DELETE /api/servers/:id`, `POST /api/servers/:id/suspend`, `PUT /api/servers/:id/name/:name` | CRUD de servidores |
| `use-users` | `GET /api/users`, `POST /api/users`, `GET /api/users/:id`, `POST /api/users/:id`, `DELETE /api/users/:id`, `GET /api/users/:id/perms`, `PUT /api/users/:id/perms` | Usuarios globales |
| `use-nodes` | `GET /api/nodes`, `POST /api/nodes`, `GET /api/nodes/:id`, `PUT /api/nodes/:id`, `DELETE /api/nodes/:id`, `GET /api/nodes/:id/features`, `GET /api/nodes/:id/system`, `GET /api/nodes/:id/deployment` | Nodos |
| `use-templates` | `GET /api/templates`, `POST /api/templates`, `GET /api/templates/:repo`, `DELETE /api/templates/:repo`, `GET /api/templates/:repo/:name`, `DELETE /api/templates/0/:name`, `PUT /api/templates/0/:name` | Plantillas |
| `use-database-hosts` | `GET /api/databasehosts`, `POST /api/databasehosts`, `GET /api/databasehosts/:id`, `PUT /api/databasehosts/:id`, `DELETE /api/databasehosts/:id` | Hosts de bases de datos |
| `use-settings` | `GET /api/settings`, `POST /api/settings`, `GET /api/settings/:key`, `PUT /api/settings/:key`, `POST /api/settings/test/email`, `POST /api/settings/test/discord`, `POST /api/settings/license/activate` | Configuración global |
| `use-server-settings` | `GET /api/servers/:serverId/data`, `POST /api/servers/:serverId/data`, `PUT /api/servers/:serverId/data` | Variables del servidor |
| `use-profile` | `GET /api/self`, `PUT /api/self`, `GET /api/self/otp`, `POST /api/self/otp`, `PUT /api/self/otp`, `POST /api/self/otp/recovery`, `DELETE /api/self/otp/:token`, `GET /api/self/oauth2`, `POST /api/self/oauth2`, `DELETE /api/self/oauth2/:clientID` | Perfil propio |
| `use-dashboard-data` | `GET /api/uptime`, `GET /api/servers` | Datos del dashboard |
| `use-toast` | — | Notificaciones toast (ui) |

## Llamadas Directas desde Features

Las operaciones de servidor en tiempo real (consola, archivos, backups, plugins, flags, tareas, estadísticas) se hacen mediante llamadas directas a `api.get/post/put/delete` desde los componentes en `src/features/servers/`:

### Consola WebSocket
```typescript
const ws = new WebSocket(`${protocol}//${host}/api/servers/${id}/socket?console`);
ws.onmessage = (event) => { /* datos de consola, stats, status */ };
ws.send(JSON.stringify({ type: 'console', data: command }));
```

### Archivos
```typescript
api.get(`/api/servers/${id}/file/${path}`)      // Descargar
api.put(`/api/servers/${id}/file/${path}`, body)  // Subir
api.delete(`/api/servers/${id}/file/${path}`)     // Eliminar
```

### Backups
```typescript
api.get(`/api/servers/${id}/backup`)                        // Listar
api.post(`/api/servers/${id}/backup/create`, {})            // Crear
api.post(`/api/servers/${id}/backup/restore/${backupID}`, {}) // Restaurar
api.get(`/api/servers/${id}/backup/download/${backupID}`)   // Descargar
api.delete(`/api/servers/${id}/backup/${backupID}`)         // Eliminar
```

### Plugins
```typescript
api.get(`/api/servers/${id}/plugins`)                       // Listar
api.get(`/api/servers/${id}/plugins/search?q=...`)          // Buscar
api.post(`/api/servers/${id}/plugins/${pluginId}`, {})      // Instalar
api.delete(`/api/servers/${id}/plugins`, { body: ... })     // Eliminar
```

### Acciones del Servidor
```typescript
api.post(`/api/servers/${id}/start`, {})    // Iniciar
api.post(`/api/servers/${id}/stop`, {})     // Detener
api.post(`/api/servers/${id}/restart`, {})  // Reiniciar
api.post(`/api/servers/${id}/kill`, {})     // Matar
api.post(`/api/servers/${id}/install`, {})  // Instalar
api.post(`/api/servers/${id}/reload`, {})   // Recargar
```
