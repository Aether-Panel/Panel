# Consumo de la API desde el frontend

El frontend no tiene un SDK generado ni clases por dominio. Todas las llamadas HTTP pasan por el pequeño objeto `api` de `src/lib/api-client.ts`, directamente desde un componente o a través de un hook de `src/hooks/`. La API del panel se documenta en su referencia (Swagger en `/swagger/`, solo para administradores); esta página solo explica cómo la usa el frontend.

## Cliente HTTP (`lib/api-client.ts`)

```typescript
import { api, ApiError } from '@/lib/api-client';

const servers = await api.get('/api/servers?limit=100&page=1');
await api.post(`/api/servers/${id}/start`, {});
await api.post(`/api/nodes/${nodeId}/ports/check`, { ports }, controller.signal); // AbortSignal opcional
await api.postForm('/api/templates/upload', formData);                          // multipart, sin cabecera JSON
await api.put(`/api/servers/${id}`, payload);
await api.delete(`/api/servers/${id}/backup/${backupId}`);
```

| Método | Cuerpo | Notas |
|---|---|---|
| `get<T>(url)` | — | `Accept: application/json` |
| `post<T>(url, body, signal?)` | JSON | El único método que acepta un `AbortSignal`. |
| `postForm<T>(url, formData)` | `FormData` | Para subidas (plantillas, extensiones). |
| `put<T>(url, body)` | JSON | |
| `delete<T>(url)` | — | |

Todos usan `fetch` del navegador con `credentials: 'include'`, de modo que la cookie de sesión autentica cada petición. La respuesta se interpreta como JSON cuando el `Content-Type` lo indica; si no, se devuelve como texto.

### Errores

Una respuesta que no es 2xx lanza `ApiError` (`status`, `message`, `data` = el cuerpo interpretado). El mensaje se elige así:

1. Si el cuerpo es `{ "error": "texto" }`, ese texto.
2. Si el cuerpo es `{ "error": { "code": "ErrAlgo", "msg": "..." } }` (los errores estructurados del panel), la traducción de `code` dentro de `apiErrors` en el idioma del usuario (leído de la clave `aether_panel_language` de `localStorage`, inglés por defecto) cuando existe, con sus marcadores `{{nombre}}` rellenados con el `metadata` del error; si no, el `msg` del servidor.
3. En otro caso, el texto del estado HTTP.

Los códigos traducidos hoy se listan en [05. Traducciones](./05-translations.md#errores-de-api-traducidos).

### Sesiones caducadas

Cuando una petición responde **401** mientras el usuario está en una página con sesión, el cliente entiende que la sesión terminó (caducó o se cerró en otro sitio): guarda la ruta actual en `sessionStorage` (`aether:return-to`) y envía el navegador a `/login/?expired=1`, que muestra un aviso de sesión caducada. Tras iniciar sesión, `AuthProvider` vuelve a la página guardada si es una ruta del propio panel. Un 401 de las llamadas `/auth/*` (contraseña incorrecta) o en las propias páginas de acceso no activa este comportamiento. La ruta de vuelta nunca se lee de la URL, así que un enlace manipulado no puede redirigir tras el inicio de sesión.

Algunos componentes llaman a `fetch` directamente en lugar de a `api` cuando necesitan la respuesta en bruto: subidas y guardado en el gestor de archivos, creación de carpetas (`?folder`), renombrar/mover (`POST …/file/<ruta>?destination=`), migraciones externas, el envío de comandos a la consola y la tarjeta "Análisis de consola por IA".

## Listas paginadas (`lib/paged.ts`)

`/api/servers` y `/api/users` devuelven 20 elementos salvo que se pidan más. `fetchAllPages(url, key)` pide páginas de 100 (`limit=100&page=N`) hasta reunir `paging.total` elementos, con un tope de seguridad de 50 páginas. `useServers` y `useUsers` lo usan para que los selectores y las listas vean todos los elementos.

## Hooks de datos (`src/hooks/`)

| Hook | Endpoints | Devuelve |
|---|---|---|
| `useServers(statsFor?)` (`use-servers.ts`) | `GET /api/servers` (todas las páginas), `GET /api/uptime` y después, cada 5 s y por servidor, `GET /api/servers/:id/stats` y `GET /api/servers/:id/query` | `{ servers, loading, error, refresh }`. `statsFor` limita el sondeo a algunos servidores (la página de detalle solo sondea el suyo). El sondeo se detiene con la pestaña oculta y se salta una vuelta si la anterior sigue en curso. |
| `useNodes(skip?)` (`use-nodes.ts`) | `GET /api/nodes` | `{ nodes, loading, error, refresh }` |
| `useNodes(skip?)` (`use-dashboard-data.ts`) | `GET /api/nodes` + `GET /api/nodes/:id/system` de cada nodo, cada 5 s con la pestaña visible | Nodos con `systemInfo`; lo usa el panel principal. |
| `useUsersCount(skip?)` | `GET /api/users?limit=1` | `{ count, loading }` a partir de `paging.total`. |
| `useGlobalNetworkMetrics(nodes)` | — (derivado) | KB/s en directo de todos los nodos, calculados a partir de los contadores acumulados de `systemInfo`. |
| `useNetworkHistory(range, enabled)` | `GET /api/nodes/network-history?range=1h\|6h\|24h`, refrescado cada 60 s con la pestaña visible | `{ samples, stepMs, nodes, unavailable, loading, error }` (ver [07](./07-components-detailed.md#panel-principal-featuresdashboard)). |
| `useAccessLog(limit?, skip?)` | `GET /api/activity?limit=N` | Inicios de sesión, intentos fallidos y registros recientes. |
| `useUsers(skip?)` | `GET /api/users` (todas las páginas) | `{ users, loading, error, refresh }` |
| `useDatabaseHosts()` | `GET/POST /api/databasehosts`, `PUT/DELETE /api/databasehosts/:id` | `{ hosts, loading, error, createHost, updateHost, deleteHost, refresh }` |
| `useTemplates()` | `GET /api/templates`, `GET /api/templates/:repo`, `GET /api/templates/:repo/:name`, `PUT /api/templates/0/:name`, `POST /api/templates/upload`, `DELETE /api/templates/0/:name` | `{ repos, loading, error, refresh, getTemplatesForRepo, getTemplateDetails, saveTemplate, uploadTemplates, deleteTemplate }` |
| `useSettings()` | `GET/POST /api/settings`, `POST /api/settings/test/email`, `POST /api/settings/test/discord` | `{ settings, loading, saving, error, saveSettings, sendTestEmail, sendTestDiscord, refresh }` |
| `useServerSettings(serverId)` | `GET/POST/PUT /api/servers/:id/data`, `GET/POST /api/servers/:id/flags`, `GET/PUT /api/servers/:id/definition`, `PUT /api/servers/:id/name/:name`, `GET /api/servers/:id/file/server.properties` | `{ settings, loading, error, saveSettings, saveDefinition, isMinecraftJava, refresh }` |
| `useProfile()` | `PUT /api/self`, `GET/POST/PUT /api/self/otp`, `DELETE /api/self/otp/:token`, `POST /api/self/otp/recovery`, `GET/POST /api/self/oauth2`, `DELETE /api/self/oauth2/:clientId` | `{ otpEnabled, oauthClients, loading, error, updateDetails, startOtpEnroll, validateOtpEnroll, disableOtp, regenerateRecoveryCodes, createOauthClient, deleteOauthClient, refresh }` |
| `useActiveExtensions()` | `GET /api/extensions/active` | `{ extensions, ready }`; `pagesFor(extensions, 'admin' \| 'server')` elige las páginas que se muestran. |
| `useIsMobile()` | — | `true` por debajo de 768 px. |

Las llamadas de autenticación están en `AuthContext` (`contexts/providers.tsx`), no en un archivo de hook: `GET /api/self`, `POST /auth/login`, `POST /auth/otp`, `POST /auth/register`, `POST /auth/logout`. Las pantallas de contraseña llaman a `POST /auth/forgot-password` y `POST /auth/reset-password`, y el hook del captcha a `GET /auth/challenge` (ver [06](./06-pages.md#páginas-de-acceso)). `ConfigContext` llama a `GET /api/config`.

## Llamadas del detalle del servidor

La página del servidor (`features/servers/[id]/`) llama a la API directamente desde cada pestaña. Los grupos principales:

| Área | Endpoints usados |
|---|---|
| Encendido | `POST /api/servers/:id/start`, `/stop`, `/restart`, `/kill`, `/install` |
| Consola | `GET /api/servers/:id/console` (historial), el WebSocket descrito abajo y `POST /api/servers/:id/console` con el comando como cuerpo de texto plano |
| Caídas | `GET /api/servers/:id/crashes` |
| Historial | `GET /api/servers/:id/metrics?range=1h\|24h\|7d` |
| Archivos | `GET/PUT/DELETE /api/servers/:id/file/<ruta>`, `PUT …?folder`, `POST …?destination=<nueva ruta>` (renombrar/mover), `POST /api/servers/:id/extract/<ruta>?destination=…&skipRoot`, `POST /api/servers/:id/archive/<ruta>` (comprimir), `GET/POST /api/servers/:id/file-history/<ruta>…` (versiones y restauración) |
| Copias de seguridad | `GET /api/servers/:id/backup`, `POST /api/servers/:id/backup/create?name=…`, `POST /api/servers/:id/backup/restore/:backupId`, `DELETE /api/servers/:id/backup/:backupId` |
| Plugins | `GET /api/servers/:id/plugins`, `GET /api/servers/:id/plugins/search?q=…`, `POST /api/servers/:id/plugins/:pluginId?loaders=…&game_versions=…` (filtro sacado de `GET /api/servers/:id/map`), `DELETE /api/servers/:id/plugins?name=…` |
| Tareas | `GET /api/servers/:id/tasks`, `PUT/DELETE /api/servers/:id/tasks/:taskId`, `POST /api/servers/:id/tasks/:taskId/run` |
| Subusuarios | `GET /api/servers/:id/user`, `PUT/DELETE /api/servers/:id/user/:email` |
| Bases de datos | `GET/POST /api/servers/:id/databases`, `DELETE /api/servers/:id/databases/:dbId`, `GET /api/servers/:id/databasehosts` |
| Otros | `/notifications`, `/offsite`, `/subdomain`, `/version`, `/modpacks`, `/map`, `/clone`, `/transfer`, `/extransfer/*`, `/port-settings`, `/owner/:userId`, `/ai/analyze`, `/ai/chat` |

### WebSocket de la consola

La página (no el componente de consola) es dueña del socket:

```typescript
const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
const socket = new WebSocket(`${protocol}//${window.location.host}/api/servers/${id}/socket?console`);
```

- Los mensajes son JSON. `{ type: 'console', data: { logs } }` añade líneas; `{ type: 'crash', data }` muestra un toast de caída y refresca las vistas de caídas; los mensajes `stat` y `status` se ignoran aquí (las estadísticas llegan por sondeo).
- Al cerrarse, reconecta con espera exponencial (1 s, 2 s, 4 s… hasta un máximo de 15 s). Un servidor que responde 404 en `/console` se marca como no disponible y no se abre ningún socket.
- La vista de consola no escribe en este socket: los comandos van por `POST /api/servers/:id/console` (ver [07](./07-components-detailed.md#consola-console-viewtsx)).
