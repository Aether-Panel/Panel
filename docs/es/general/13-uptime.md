# Seguimiento de disponibilidad (uptime)

Aether Panel registra cuándo está encendido y cuándo está apagado cada servidor, y con esos registros calcula un porcentaje de disponibilidad, un historial reciente y la cifra de "salud" del panel principal.

---

## Resumen

- El panel comprueba el estado de cada servidor cada **5 segundos** y guarda una fila por cada periodo continuo "encendido" o "apagado".
- `GET /api/uptime` devuelve un resumen de todos los servidores que puede ver quien llama; `GET /api/uptime/:serverId` devuelve el detalle y el historial de un servidor.
- El periodo por defecto son los últimos **30 días** (máximo 365).
- Los avisos de encendido/apagado para administradores se envían por Discord y se documentan en [Discord y notificaciones](16-discord.md); no dependen de estos registros.

---

## Modelo de datos

`UptimeStatus` (`internal/models/uptime.go`), tabla `uptime_statuses`:

| Columna | Tipo | Descripción |
|---------|------|-------------|
| `id` | uint | Clave primaria |
| `server_id` | string (20) | Identificador del servidor (con índice). No se incluye en el JSON |
| `is_running` | bool | `true` = periodo encendido, `false` = periodo apagado |
| `start_time` | time | Inicio del periodo (con índice) |
| `end_time` | time, admite nulo | Fin del periodo; `NULL` mientras el periodo sigue abierto (con índice) |
| `duration` | int64 | Duración en segundos, se rellena al cerrar el periodo (por defecto `0`) |
| `created_at`, `updated_at` | time | Marcas de tiempo de GORM |

Nombres de los campos en JSON: `id`, `isRunning`, `startTime`, `endTime` (se omite mientras está abierto), `duration`, `createdAt`, `updatedAt`.

Al borrar un servidor, sus filas de uptime se borran de forma explícita junto con el resto de sus datos (`internal/services/server.go`, `internal/web/api/servers.go`).

---

## Cómo se registra

`trackUptimeForAllServers()` en `internal/servers/server.go` es el único punto que escribe uptime. Se ejecuta una vez al arrancar y después en cada ciclo de estadísticas de 5 segundos (`processStats()`):

1. No hace nada si `panel.enable` es `false` (un nodo solo-daemon no tiene base de datos del panel). Una marca impide que una ronda lenta se solape con la siguiente.
2. Para cada servidor cargado en este proceso pregunta al entorno si está en marcha y llama a `Uptime.TrackStatus(serverID, isRunning)` (`internal/services/uptime.go`).

`TrackStatus`:

1. Omite los servidores que aún no existen en la tabla `servers` (evita errores de clave foránea).
2. Busca la fila abierta (`end_time IS NULL`, la más reciente).
3. Sin fila abierta: crea una con el estado actual y `start_time = ahora`.
4. El estado ha cambiado: cierra la fila abierta (`end_time = ahora`, `duration = ahora - start_time` en segundos) y abre otra con el nuevo estado.
5. El estado no ha cambiado: solo actualiza `updated_at`.

> **Alcance:** solo se registran los servidores que ejecuta el propio proceso del panel (el nodo local). Los servidores de nodos remotos no tienen historial guardado; `/api/uptime` les asigna una entrada derivada de su estado actual (ver más abajo).

Un reinicio del panel conserva las filas abiertas y las continúa, de modo que el tiempo que el panel estuvo caído cuenta para el estado que estaba abierto en ese momento.

---

## Estadísticas

### `GetUptimeStats(serverID, since)`

Devuelve `uptimeSeconds`, `downtimeSeconds` y `uptimePercent`:

1. Lee los periodos que se solapan con la ventana: los que siguen abiertos y los que terminaron después de `since`.
2. Recorta cada periodo a la ventana: uno que empezó antes de `since` solo cuenta desde `since`, y el abierto cuenta hasta ahora. El resultado se suma a encendido o apagado según su estado.
3. `uptimePercent = encendido / (encendido + apagado) * 100`; **100** si no hay ningún dato.

Así, un servidor encendido durante toda la ventana, cuyo único periodo empezó antes, cuenta como encendido el 100 % del tiempo, y ningún periodo cuenta más tiempo del que cabe en la ventana.

### `GetRecentHistory(serverID, limit)`

Las últimas `limit` filas del servidor, de la más reciente a la más antigua. No se filtra por el periodo.

### `GetAllServerUptime(since)`

El mismo cálculo (con los periodos recortados a la ventana) para todos los servidores a la vez. Devuelve un mapa por identificador de servidor con `uptime`, `downtime`, `uptimePercent` y, si hay una fila abierta, `currentStatus` y `currentStartTime`.

---

## API

| Método | Ruta | Permiso | Descripción |
|--------|------|---------|-------------|
| GET | `/api/uptime` | `admin` o `uptime.view` | Resumen de todos los servidores visibles |
| GET | `/api/uptime/:serverId` | `server.view` sobre ese servidor | Detalle e historial de un servidor |

Parámetros de consulta (los valores menores que 1 o no numéricos vuelven al valor por defecto; los que superan el máximo se recortan):

| Parámetro | Endpoint | Por defecto | Máximo | Descripción |
|-----------|----------|-------------|--------|-------------|
| `days` | ambos | 30 | 365 | Periodo que se calcula |
| `limit` | `/api/uptime/:serverId` | 50 | 200 | Número de filas de historial |

El scope `uptime.view` se describe en la [referencia de scopes](scopes-reference.md).

### `GET /api/uptime`

- Los administradores ven todos los servidores. El resto de usuarios (con `uptime.view`) solo ven los servidores a los que tienen acceso.
- Todos los servidores se leen por páginas (1000 por consulta), así que ninguno desaparece del resumen.
- El estado en vivo de cada servidor se pregunta a su nodo en paralelo (como máximo 16 peticiones a la vez, 5 segundos de límite cada una). Un nodo que no responde hace que sus servidores cuenten como apagados.
- Los servidores sin datos guardados reciben una entrada mínima: si ahora están encendidos cuentan como encendidos todo el periodo (100 %); si no, como apagados todo el periodo (0 %).
- Si no hay ningún servidor, la respuesta es `{}`.

Respuesta (objeto con el identificador del servidor como clave):

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

`nodeName` es `LocalNode` para el nodo local o para un nodo sin nombre. `uptime` y `downtime` están en segundos.

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

- `currentStatus` lo da el daemon (`GET /daemon/server/:id/status`); cualquier error cuenta como apagado.
- `currentUptime` y `currentStartTime` solo se rellenan si el servidor está encendido y tiene una fila abierta.
- `totalSeconds` es `uptimeSeconds + downtimeSeconds`. Ambos ya incluyen el periodo en curso, que no se vuelve a sumar.

### Ejemplos

```bash
# Resumen de los últimos 7 días
curl -H "Authorization: Bearer $TOKEN" "https://panel.example.com/api/uptime?days=7"

# Un servidor, 90 días, 100 filas de historial
curl -H "Authorization: Bearer $TOKEN" "https://panel.example.com/api/uptime/a1b2c3d4?days=90&limit=100"
```

---

## Dónde lo muestra el panel

- **Lista de servidores y panel principal** (`hooks/use-servers.ts`): la lista llama a `GET /api/uptime` junto con la lista de servidores. `currentStatus` fija el estado inicial en línea/apagado de cada servidor (las peticiones de estadísticas en vivo lo actualizan después) y `uptimePercent` se guarda por servidor. Si la llamada falla, la lista carga igualmente, sin datos de uptime.
- **Franja de estado del panel principal** (`features/dashboard/status-strip.tsx`): la cifra de "salud" es la media de `uptimePercent` de todos los servidores en 30 días. Color: verde a partir del 99 %, ámbar entre el 95 % y el 99 %, rojo por debajo del 95 %.
- El endpoint por servidor (`/api/uptime/:serverId`) no lo usa el frontend actual; está disponible para integraciones.

Las gráficas de rendimiento del servidor (CPU, memoria, red y uptime por hora/día/semana) salen de las métricas del daemon, no de esta tabla.

---

## Limitaciones conocidas

- Solo se registra el historial de los servidores del nodo local; los de nodos remotos reciben una entrada derivada de su estado actual (véase [Cómo se registra](#cómo-se-registra)).
- El tiempo que el panel está apagado cuenta para el estado que estaba abierto en ese momento.
