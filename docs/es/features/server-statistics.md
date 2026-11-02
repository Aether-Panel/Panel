# Resumen y estadísticas del servidor

La pestaña **Resumen** de un servidor muestra cómo está ahora y cómo se ha comportado mientras nadie miraba: consumo de recursos en directo, quién está jugando, un historial de rendimiento de hasta una semana, las horas con más gente de la semana y las últimas caídas. Sirve para dimensionar el plan, detectar lag y elegir buen momento para eventos o reinicios.

## Qué hace

La pestaña se compone de estas tarjetas, de arriba abajo:

| Tarjeta | Muestra | Origen |
|---|---|---|
| **Recursos en vivo** | CPU, memoria y almacenamiento frente a los límites del servidor | `GET /api/servers/:id/stats`, cada 5 s |
| **Jugadores en línea** | Jugadores conectados / plazas, una barra, los nombres si el juego los comparte y el juego, mapa o versión | `GET /api/servers/:id/query`, cada 5 s |
| **Historial de rendimiento** | Gráficas de la última **1 h**, **24 h** o **7 días**, con tres vistas: **CPU y memoria**, **Red**, **Jugadores** | `GET /api/servers/:id/metrics`, se actualiza cada minuto |
| **Horas punta** | Media de jugadores por día de la semana y hora durante la última semana, en la hora local de quien mira | `GET /api/servers/:id/metrics?range=7d` |
| **Uso de CPU y memoria** y **Tráfico de red** | Gráficas en directo desde que se abrió la página, una muestra cada 5 s | La misma consulta cada 5 s que **Recursos en vivo** |
| **Historial de caídas** | Últimas caídas con las líneas de consola previas | Ver [Informes de caídas](./crash-reports.md) |

El **Historial de rendimiento** muestra además unas cifras resumen sobre la gráfica:

- **CPU y memoria**: **CPU media**, **Pico de CPU**, **Memoria media**, **Disponibilidad**.
- **Red**: **Descarga media**, **Subida media**, **Disponibilidad**.
- **Jugadores**: **Media de jugadores**, **Pico de jugadores**, **Récord histórico** (con su fecha), **Disponibilidad**.

En la gráfica, las zonas grises son ratos apagado y las líneas rojas marcan caídas. Un tramo sin ningún dato («Sin datos: el nodo estaba apagado.») queda en blanco y cuenta como apagado.

**Horas punta** resalta la celda con más gente («Más gente: sábado a las 21:00 (media 12.4)»); las celdas vacías (borde discontinuo) no tienen datos y las grises no tenían a nadie conectado. La tarjeta se oculta en servidores que nunca han informado de jugadores.

## Cómo funciona

### Cifras en directo

Mientras la página del servidor está abierta y la pestaña del navegador visible, la web pide al nodo cada 5 segundos las estadísticas del servidor y su consulta de jugadores. Las gráficas en directo guardan las últimas 60 muestras (unos 5 minutos) y se pierden al cerrar la página. La CPU y la memoria se muestran como porcentaje de los límites del servidor.

### Historial de rendimiento (guardado en el nodo)

El nodo ya toma una muestra de cada servidor cada 5 segundos para sus alertas. Esas muestras se agrupan en:

- **puntos de un minuto**, que se conservan **24 horas**;
- **puntos de quince minutos**, que se conservan **7 días**.

La CPU, la memoria y la red se promedian sobre las muestras tomadas con el servidor encendido, así que una hora con el servidor apagado la mitad del tiempo no las reduce a la mitad. La **Disponibilidad** es la proporción de muestras tomadas con el servidor encendido; los huecos en los que el propio nodo estaba caído cuentan como apagado.

El panel ofrece tres ventanas, cada una promediada a su propio paso:

| Rango | Paso |
|---|---|
| `1h` | 1 minuto |
| `24h` (por defecto) | 5 minutos |
| `7d` | 1 hora |

El historial se escribe en `<daemon.data.servers>/<id del servidor>.metrics` cada 5 minutos y al apagar el nodo, así que una caída del nodo pierde como mucho los últimos minutos. El archivo se borra junto con el servidor.

### Recuento de jugadores

Cada minuto el nodo pregunta a cada servidor encendido quién está conectado y añade la respuesta al historial (media, pico y plazas por tramo). El récord histórico (máximo de jugadores a la vez, y cuándo) se guarda en el mismo archivo. Cómo pregunta depende del tipo de `query` de la plantilla:

| Tipo de query | Protocolo | Puerto |
|---|---|---|
| `minecraft` | Ping de la lista de servidores de Minecraft | El puerto del juego |
| `source` o `steam` | A2S de Valve | La variable de puerto de consulta de la plantilla (`queryport`, `query_port`, `queryPort`, `steamqueryport` o `steam_query_port`), o el puerto del juego |
| sin definir | Se prueba A2S; si el juego no responde, no se vuelve a preguntar durante 2 minutos | Igual que el anterior |

Las respuestas se guardan en caché 10 segundos y las comparten la tarjeta en directo y el historial. Los bots de los servidores de Steam no cuentan como jugadores.

## Uso

### Desde el panel

Abrir un servidor e ir a **Resumen**. La pestaña aparece siempre, pero sus cifras vienen de rutas que necesitan `server.stats` en el servidor (el propietario lo tiene; los subusuarios, solo si el propietario se lo ha dado). En **Historial de rendimiento**, elegir la vista con **Qué mostrar** y la ventana con **Periodo**. Al pasar el ratón por un punto se ven sus valores.

### Mediante la API

| Método y ruta | Permiso | Para qué |
|---|---|---|
| `GET /api/servers/:id/stats` | `server.stats` | CPU, memoria, disco y red actuales |
| `GET /api/servers/:id/query` | `server.stats` | Quién está jugando: `{"minecraft": {...}}` o `{"source": {...}}`; `204` si el servidor está apagado o no responde |
| `GET /api/servers/:id/metrics?range=1h\|24h\|7d` | `server.stats` | Historial de rendimiento |

Respuesta de `/metrics`:

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

- `t` es el inicio del tramo (segundos Unix); `online` va de 0 a 1.
- Un punto con `"noData": true` es un tramo sin nada registrado (el nodo estaba caído).
- `hasPlayers` es `false` cuando el juego no respondió a ninguna consulta en ese tramo.

Más detalles en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

- No hay nada que activar: el historial se registra en todos los nodos para todos los servidores.
- El recuento de jugadores necesita un juego que responda a la consulta: en Minecraft, `enable-status` debe seguir en `true` en `server.properties` (lo está por defecto); en juegos de Steam, el puerto de consulta debe ser accesible desde el nodo.
- El archivo del historial está en la carpeta de servidores:

| Clave | Valor por defecto | Variable de entorno |
|---|---|---|
| `daemon.data.servers` | `servers` (dentro de `daemon.data.root`) | `SKYPANEL_DAEMON_DATA_SERVERS` |

Ver [Configuración](../backend/02-configuration.md).

## Límites

- El historial cubre 24 horas con detalle de un minuto y 7 días con detalle de quince minutos; los datos más antiguos se descartan.
- **Horas punta** usa solo los últimos 7 días.
- Las gráficas en directo empiezan vacías cada vez que se abre la página.
- Las consultas se pausan mientras la pestaña del navegador está oculta.
- Los servidores cuyo juego no responde a ninguno de los dos protocolos no muestran datos de jugadores.

## Problemas conocidos y soluciones

| Qué se ve | Causa | Solución |
|---|---|---|
| **Todavía no hay historial** | El nodo aún no ha guardado ningún punto (servidor nuevo o justo después de una actualización) | Esperar unos minutos |
| **El juego no respondió a la consulta de jugadores.** | El juego no admite la consulta, el puerto de consulta es incorrecto o (Minecraft) `enable-status=false` | Revisar el tipo y el puerto de query de la plantilla, o volver a activar `enable-status` |
| Falta la tarjeta **Jugadores en línea** | `/query` respondió `204` | Igual que el anterior; también ocurre con el servidor apagado |
| Huecos grises largos | El nodo (o su daemon) estaba caído | Revisar la disponibilidad del nodo |
| `range must be 1h, 24h or 7d` | Llamada a la API con otro rango | Usar uno de los tres valores |

## Véase también

- [Informes de caídas](./crash-reports.md)
- [Panel de inicio y nodos](./dashboard-and-nodes.md): gráficas del panel y de cada nodo.
- [Seguimiento de disponibilidad](../general/13-uptime.md)
- [Métricas de procesos](../backend/13-process-metrics.md)
