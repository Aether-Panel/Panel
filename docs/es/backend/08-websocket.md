# WebSocket — consola en tiempo real

Cada servidor tiene un WebSocket que envía la salida de su consola, su consumo de recursos y sus cambios de estado en el momento en que ocurren. Es **unidireccional**: el navegador solo escucha. Los comandos se envían con `POST /api/servers/:serverId/console` y el historial de la consola se lee con `GET /api/servers/:serverId/console` (véase la [referencia de la API](../reference/11-api-reference.md)).

Código: `internal/web/api/servers.go` (endpoint del panel y proxy), `internal/web/daemon/server.go` (`openSocket`), `internal/services/node.go` (`OpenSocket`, nodos remotos), `pkg/skypanel/tracker.go` (sockets y trackers), `pkg/skypanel/message.go` (tipos de mensaje), `internal/middleware/security.go` (comprobación de origen).

---

## Endpoints

| Endpoint | Quién lo llama | Autenticación |
|----------|----------------|---------------|
| `GET /api/servers/:serverId/socket` | el navegador (o cualquier cliente de la API) | cookie de sesión del panel `skypanel_auth` o `Authorization: Bearer <token>`, más el scope `server.view` en ese servidor |
| `GET /daemon/server/:serverId/socket` | el panel, nunca el navegador | `Authorization: Bearer <JWT>` firmado por el panel para ese nodo (véase [Seguridad](./09-security.md#tokens-entre-el-panel-y-los-nodos)) |

El frontend abre `ws(s)://<panel>/api/servers/<id>/socket?console` después de cargar el historial de la consola por HTTP, y se reconecta con espera exponencial (1 s, 2 s, 4 s… hasta 15 s) cuando se corta la conexión.

---

## Qué flujos recibe un cliente

Un socket puede llevar tres flujos. En el endpoint del panel **se descarta la query string del cliente**: el panel calcula los flujos a partir de los permisos del usuario y construye él mismo la query para el daemon, de modo que nadie puede pedir más de lo que tiene permitido ver.

| Flujo | Se concede si el usuario tiene | `type` del mensaje |
|-------|--------------------------------|--------------------|
| `console` | `server.console` | `console` |
| `status` | `server.status` | `status`, `crash` |
| `stats` | `server.stats` | `stat` |

Los scopes son los que resolvió `middleware.RequiresPermission` para esta petición (`callerScopes` en `websocketPathWithGrants`), como en cualquier otra ruta: las filas del usuario en ese servidor y globales, los scopes de propietario si el servidor es suyo, los scopes globales de su rol y, en una sesión abierta por un cliente OAuth2, solo lo que el cliente puede usar. `admin` y `server.admin` cuentan como todos los scopes. Un usuario sin ninguno de los tres obtiene igualmente la conexión, solo que sin mensajes.

En el endpoint del daemon los flujos se eligen por la presencia de claves en la query: `?console&status&stats` (los valores se ignoran, así que `?console=1` también funciona). Una lista separada por comas como `?console,stats` **no** se entiende.

---

## Recorrido a través del panel

1. El navegador se conecta a `/api/servers/:serverId/socket`. Antes se ejecutan la autenticación, la comprobación de `server.view` y la [comprobación de origen](#comprobación-de-origen).
2. El panel genera un JWT de vida corta para el nodo del servidor y reescribe la ruta a `/daemon/server/<id>/socket?<flujos concedidos>`.
3. **Nodo local** (panel y daemon en el mismo proceso): la petición vuelve al router y la atiende directamente el manejador del daemon.
4. **Nodo remoto**: el panel acepta el WebSocket del navegador, se conecta por `ws://` o `wss://` a `<host privado del nodo>:<puerto privado del nodo>/daemon/server/<id>/socket?...` con `Authorization: Bearer <JWT>` y copia los frames en ambos sentidos hasta que uno de los dos lados cierra. Las cookies del navegador nunca se reenvían al nodo.
5. Si el servidor existe en la base de datos pero sus archivos no están en el nodo local, el panel acepta el socket, envía un mensaje de consola indicando que los archivos del servidor han desaparecido y lo cierra un segundo después.

`daemon.console.forward` no tiene nada que ver con esto: solo copia las consolas de los servidores a la salida estándar del daemon.

---

## Mensajes (servidor → cliente)

Todos los mensajes son **frames de texto** con JSON en la forma `Transmission`:

```json
{ "type": "<console|stat|status|crash>", "data": { ... } }
```

No hay frames binarios.

### `console`

```json
{ "type": "console", "data": { "epoch": 0, "logs": "W1NlcnZlcl0gRG9uZSAoMy4ycykh" } }
```

`data` es un `ServerLogs`. `logs` es la salida bruta de la consola como array de bytes, que JSON codifica en **base64**: hay que decodificarlo y después separar por saltos de línea (un mensaje puede contener varias líneas o parte de una). Las líneas que escribe el propio daemon empiezan por `[DAEMON] `. En los mensajes en directo `epoch` no se rellena (`0`); tiene sentido en la respuesta HTTP del historial.

### `stat`

Se envía cada 5 segundos por cada servidor cuyas estadísticas se pueden leer.

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

| Campo | Significado |
|-------|-------------|
| `cpu` | CPU en porcentaje de **un núcleo** (200 = dos núcleos ocupados) |
| `maxCpu` | Lo que puede usar el servidor en la misma unidad: su límite de CPU o, si no tiene, todos los núcleos del nodo |
| `memory`, `maxMemory` | Bytes |
| `storage`, `maxStorage` | Bytes |
| `networkRx`, `networkTx` | Contadores de red que informa el entorno |
| `jvm` | Solo aparece cuando el servidor informa datos de la JVM |
| `running` | Si el proceso está en marcha |

Divida `cpu` entre `maxCpu` para obtener la cifra de 0 a 100 % que muestra el panel en todas partes.

### `status`

```json
{ "type": "status", "data": { "running": true, "installing": false } }
```

Lo envían los entornos Docker y host cuando el proceso arranca o se detiene.

### `crash`

```json
{ "type": "crash", "data": { "id": "<id del informe>", "exitCode": 1, "restarted": false } }
```

Se envía por el flujo **status** cuando un servidor se cae por sí solo. El informe completo de la caída (con las últimas líneas de la consola) se obtiene aparte desde la API; el frontend usa este mensaje para mostrar el aviso de caída.

---

## Mensajes (cliente → servidor)

No se procesa ninguno. El daemon lee los frames entrantes solo para mantener viva la conexión y descarta su contenido; un frame de más de 1024 bytes cierra la conexión. Para enviar un comando use `POST /api/servers/:serverId/console` con el comando como cuerpo (scope `server.console.send`).

---

## Mantenimiento de la conexión y limpieza

`Socket.Serve()` se ejecuta en el lado del daemon de cada conexión (en los nodos remotos, en el tramo panel↔nodo):

- envía un ping de WebSocket cada **25 s**;
- si en **60 s** no llega un pong (u otro frame), cierra la conexión;
- cada escritura tiene un plazo de 10 s; un socket cuya escritura falla se retira de sus flujos;
- al terminar la conexión, el socket se elimina de todos los trackers en los que estaba registrado.

Internamente, cada entorno de servidor tiene tres trackers (`ConsoleTracker`, `StatusTracker`, `StatsTracker`). La salida de la consola, los cambios de estado y el bucle de estadísticas de 5 segundos escriben en ellos, y cada tracker reparte el mensaje entre los sockets registrados.

---

## Historial de la consola

El socket en directo **no** reenvía la salida anterior. El historial vive en un búfer en memoria por servidor de `daemon.console.buffer` KB (por defecto 50; se descarta primero la salida más antigua) y se lee por HTTP:

```
GET /api/servers/:serverId/console?time=<microsegundos>
→ { "epoch": 1730000000123456, "logs": "<base64>" }
```

`epoch` y `time` son tiempo Unix en **microsegundos**. Sin `time` (o con `0`) se devuelve todo el búfer; con él, solo lo escrito después de ese momento. Pase el `epoch` devuelto como `time` la vez siguiente para obtener solo la salida nueva. Requiere `server.console`.

---

## Comprobación de origen

Los upgraders aceptan cualquier `Origin` por sí mismos; la protección entre sitios la aporta el middleware global `SameOriginGuard`, que trata el handshake de un WebSocket como una petición que cambia estado aunque sea un `GET`:

- Solo se aplica cuando la petición lleva una cookie del panel (`skypanel_auth` o `session`). Las peticiones autenticadas con token bearer, las llamadas del propio daemon y los clientes OAuth2 no se ven afectados.
- La petición pasa si la cabecera `Sec-Fetch-Site` del navegador es `same-origin` o `none`. Sin esa cabecera, el host de `Origin` debe coincidir con el `Host` de la petición, con `X-Forwarded-Host` o con el host de `panel.settings.masterUrl`. Una petición sin ninguna de las dos cabeceras no viene de una página web y pasa.
- Cualquier otra recibe `403 {"error": "cross-origin request refused"}`.

Esto importa porque los navegadores consideran que todos los puertos de una misma IP son el mismo *sitio*: sin la comprobación, una página web servida por un servidor de juego en `http://IP:25566` podría abrir un socket contra el panel en `http://IP:8080` con las cookies del visitante y leer consolas. Si el panel está detrás de un proxy inverso que cambia el nombre de host, asegúrese de que reenvía `Host` o `X-Forwarded-Host`, o fije `panel.settings.masterUrl` con la URL pública.

---

## Cliente de ejemplo

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
      console.log('en marcha:', msg.data.running);
      break;
    case 'crash':
      console.warn('caída con código de salida', msg.data.exitCode);
      break;
  }
};
```

---

## Relacionado

- [RCON y Telnet](./12-rcon-telnet.md): conexiones de consola para juegos que exponen RCON en lugar de la entrada estándar. RCON sobre WebSocket (`rconws`, `internal/connections/rconws.go`) es un tercer tipo de entrada por red.
- [Métricas de procesos](./13-process-metrics.md)
