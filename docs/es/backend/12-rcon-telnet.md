# Entrada de consola: stdin, RCON, Telnet y RCON sobre WebSocket

Cada servidor en marcha tiene una única **entrada de consola**: el canal por el que el daemon escribe los comandos. Por defecto es el stdin del propio proceso (la PTY en el host, el flujo adjunto en Docker). Una plantilla puede, en su lugar, enviar los comandos por red mediante RCON, Telnet o RCON sobre WebSocket.

Esto solo cambia **adónde van los comandos**. La **salida** de la consola procede siempre del stdout/stderr del proceso (PTY o `docker attach`), sea cual sea el tipo de entrada.

Código:

| Archivo | Función |
|---|---|
| `pkg/skypanel/stdin.go` | `StdinConsoleConfiguration` (el bloque `stdin` de una plantilla) |
| `pkg/skypanel/console.go` | Interfaz `Console` y `NoStartConsole` (stdin simple) |
| `pkg/skypanel/environment.go` | `CreateConsoleStdinProxy()` y `ExecuteInMainProcess()` |
| `internal/connections/rcon.go` | `RCONConnection` |
| `internal/connections/telnet.go` | `TelnetConnection` |
| `internal/connections/rconws.go` | `RCONWSConnection` |

---

## Tipos de entrada

| `type` | Implementación | Transporte | Biblioteca |
|---|---|---|---|
| `stdin` (o vacío) | `NoStartConsole` | Escribe en el stdin del proceso (PTY / attach de Docker) | — |
| `rcon` | `RCONConnection` | RCON de Source sobre TCP | `github.com/gorcon/rcon` |
| `telnet` | `TelnetConnection` | TCP sin protocolo | `net` |
| `rconws` | `RCONWSConnection` | JSON sobre WebSocket: `wss://` por defecto, `ws://` sin cifrar con `"secure": false` | `github.com/gorilla/websocket` |

El tipo se escribe `rconws`. Cualquier otro valor que el `switch` no conozca (por ejemplo `rcon-ws`) vuelve al stdin simple y registra un error con el servidor, el tipo y los tipos conocidos: `Unknown console type "rcon-ws" (known: stdin, rcon, telnet, rconws): commands go to the standard input`.

---

## Configuración

El bloque `stdin` está en la definición del servidor, dentro de `run`, o en una entrada concreta de una lista de comandos:

```json
{
  "run": {
    "command": "java -jar server.jar nogui",
    "stop": "stop",
    "stdin": {
      "type": "rcon",
      "ip": "127.0.0.1",
      "port": "${rconport}",
      "password": "${rconpassword}"
    }
  }
}
```

```go
type StdinConsoleConfiguration struct {
    Type     string `json:"type,omitempty"`
    IP       string `json:"ip,omitempty"`
    Port     string `json:"port,omitempty"`
    Password string `json:"password,omitempty"`
    Secure   *bool  `json:"secure,omitempty"`
}
```

| Campo | Notas |
|---|---|
| `type` | `stdin`, `rcon`, `telnet` o `rconws`. Un bloque `stdin` sin `type` se lee como `stdin` (`UnmarshalJSON` propio). |
| `ip` | Dirección a la que conecta el daemon. Vacía, `0.0.0.0` o `::` equivalen a la máquina del propio juego (véase abajo). |
| `port` | Es una **cadena**, para poder contener una variable (`"${rconport}"`). |
| `password` | Contraseña RCON, línea de contraseña de Telnet o segmento de ruta de RCON-WS. |
| `secure` | Solo para `rconws`, booleano opcional. Ausente o `true`: el daemon conecta por `wss://` (TLS). `false`: `ws://` sin cifrar, para juegos que sirven RCON por un WebSocket sin cifrar, como el WebRCON de Rust. Las plantillas escritas antes de este campo siguen usando `wss://` (`StdinConsoleConfiguration.UsesTLS`). No admite variables: `Replace` lo copia tal cual. |

**Variables.** Antes de arrancar el proceso, `ExecuteAsync()` (y `Reattach()`) sustituye los tokens `${nombre}` de `ip`, `port` y `password` por las variables del servidor (`StdinConsoleConfiguration.Replace` → `utils.ReplaceTokens`). La sintaxis es `${nombre}`, no `{{nombre}}`. No hay detección automática del puerto: la plantilla debe apuntar `port` a la variable correcta.

**Qué bloque se aplica.** Cuando `run.command` es una lista, gana el `stdin` de la entrada elegida; si esa entrada no tiene (`type` vacío), se usa `run.stdin` (`Server.Start()` en `internal/servers/server.go`).

**Adónde conecta el daemon.** La conexión sale del proceso del daemon hacia `ip:puerto`. Cuando `ip` está vacía, es `0.0.0.0` o `::`, las tres consolas de red la resuelven igual que las consultas al juego (`query.HostOrLocal`, `internal/query/host.go`): `127.0.0.1` en una instalación normal, o la puerta de enlace por defecto del contenedor cuando el propio daemon corre en Docker, que llega a los puertos que los contenedores de juego publican en el host. Una dirección concreta se usa tal cual. Véase [Paquetes restantes](./10-remaining-packages.md#internalquery--consultas-a-juegos).

---

## Cómo se crea la entrada

`Environment.CreateConsoleStdinProxy(config, base)` elige la implementación y la guarda en `Environment.Console`:

```go
func (e *Environment) CreateConsoleStdinProxy(config StdinConsoleConfiguration, base io.WriteCloser) {
    switch config.Type {
    case "telnet":
        e.Console = &connections.TelnetConnection{IP: config.IP, Port: config.Port, Password: config.Password}
    case "rcon":
        e.Console = &connections.RCONConnection{IP: config.IP, Port: config.Port, Password: config.Password}
    case "rconws":
        e.Console = &connections.RCONWSConnection{IP: config.IP, Port: config.Port, Password: config.Password, NoTLS: !config.UsesTLS()}
    case "", "stdin":
        e.Console = &NoStartConsole{Base: base}
    default:
        logging.Error.Printf("[%s] Unknown console type %q (known: stdin, rcon, telnet, rconws): ...", e.ServerID, config.Type)
        e.Console = &NoStartConsole{Base: base}
    }
}
```

Cada entorno la llama justo después de levantar el proceso:

| Entorno | Dónde se llama | `base` (stdin simple) |
|---|---|---|
| Host (`tty`, `host`, `standard`) | `internal/servers/tty/tty.go` `ExecuteAsyncImpl` | la PTY de `pty.Start` |
| Docker | `internal/servers/docker/docker.go` `ExecuteAsyncImpl` | la conexión secuestrada de `ContainerAttach` |
| Docker, tras reiniciar el daemon | `docker.go` `ReattachImpl` | la nueva conexión de attach |

A continuación se ejecuta `environment.Console.Start()`. Cuando el proceso termina, el entorno llama a `environment.Console.Close()`.

```go
type Console interface {
    io.WriteCloser
    Start()
}
```

`NoStartConsole.Start()` no hace nada; `Write` y `Close` van directamente al flujo base.

---

## Qué escribe en la consola

Todos los comandos pasan por `Environment.ExecuteInMainProcess(cmd)`, que añade `\n` y escribe en `Environment.Console`:

| Origen | Código |
|---|---|
| Caja de consola del panel: `POST /api/servers/:id/console` (scope `server.console.send`) | `daemon/server.go postConsole` → `Server.Execute` |
| Botón de parada, cuando la plantilla no define `stopCode` | `Server.Stop` → `ExecuteInMainProcess(run.stop)` |
| Comando de keep-alive | `Server.startKeepAlive` |
| Operación `stdin` (tareas programadas, pasos de instalación) | `internal/operations/stdin` |

Errores posibles:

| Error | Cuándo |
|---|---|
| `ErrServerOffline` | El servidor no está en marcha |
| `ErrConsoleDetached` | El servidor sigue en marcha desde antes de reiniciar el daemon y aún no tiene consola conectada ("kill and start the server to reconnect"). `Execute`, `Stop` y `Kill` llaman antes a `Reattach()`, que vuelve a conectar los servidores Docker. |
| `"rcon not available"`, `"telnet not available"`, `"rconws not available"` | La conexión de red no está lista ni tras esperar un segundo |

El WebSocket que transmite la consola al navegador no acepta comandos: los mensajes entrantes solo se leen para mantener viva la conexión (`Socket.Serve`).

---

## RCON (`internal/connections/rcon.go`)

```go
type RCONConnection struct {
    io.WriteCloser
    IP, Port, Password string
    Reconnect          bool
    // connection, ready, closer, lock: internos
}
```

- **`Start()`**: pone `Reconnect = true`, resuelve `IP` con `query.HostOrLocal` (véase [Adónde conecta el daemon](#configuración)) y lanza el bucle de reconexión en una goroutine.
- **Bucle de reconexión**: `rcon.Dial("ip:puerto", contraseña)`. El primer intento es inmediato; los siguientes esperan **5 segundos**. La conexión sigue abierta hasta que falla una escritura o se llama a `Close()`; entonces el bucle vuelve a marcar. Los errores de conexión solo se registran en el nivel debug.
- **`Write(p)`**: si no hay conexión, espera un segundo y vuelve a comprobar. Ejecuta `conn.Execute(string(p))`. La respuesta del servidor **solo se escribe en el log de debug** (`RCON Response: ...`), no en la consola. Un comando fallido descarta la conexión para que el bucle abra otra.
- **`Close()`**: detiene el bucle, lo despierta y cierra la conexión. Cerrar dos veces la misma conexión no es un error.
- El estado está protegido por un mutex (cubierto por `internal/connections/rcon_test.go`).

Los tiempos de espera son los predeterminados de `gorcon/rcon`; el panel no fija los suyos.

---

## Telnet (`internal/connections/telnet.go`)

```go
type TelnetConnection struct {
    io.WriteCloser
    IP, Port, Password string
    Reconnect          bool
}
```

- **`Start()`** resuelve `IP` igual que RCON.
- **Bucle de reconexión**: `net.DialTCP` a `ip:puerto`, reintento cada 5 segundos tras el primero. Activa el keep-alive de TCP.
- Tras conectar espera **un segundo** (por si aparece una petición de contraseña) y, si `password` tiene valor, la envía seguida de `\n`; si esa escritura falla, se cierra el socket y el bucle vuelve a conectar.
- Los datos entrantes se leen y se **descartan**; el bucle solo usa las lecturas para detectar que la conexión se ha cerrado y volver a conectar.
- **`Write(p)`** escribe los bytes tal cual en el socket (espera de un segundo si no está listo).
- No hay latido a nivel de aplicación, ni búfer de líneas, ni control de eco.
- El estado está protegido por un mutex, como en RCON: un socket que se rompe se cierra antes de volver a conectar, una conexión abierta mientras se ejecuta `Close()` se cierra en el acto en lugar de conservarse, y cerrar dos veces no es un error (cubierto por `internal/connections/telnet_test.go`).

---

## RCON sobre WebSocket (`internal/connections/rconws.go`)

```go
type RCONWSConnection struct {
    io.WriteCloser
    IP, Port, Password string
    NoTLS              bool
    Reconnect          bool
}
```

- **Bucle de reconexión**: conecta a `wss://<ip>:<puerto>/<contraseña>` (`rconwsURL`), o a `ws://` cuando `NoTLS` está activo (el `"secure": false` de la plantilla), con el dialer por defecto de gorilla, con reintento cada 5 segundos. La contraseña es la ruta de la URL y se escapa (`url.PathEscape`), así que un `/`, `?`, `#` o `%` en ella llega al juego tal como se escribió.
- **`Start()`** resuelve `IP` igual que RCON.
- Los mensajes entrantes se leen y se descartan.
- El estado está protegido por un mutex y las escrituras se serializan (un WebSocket admite un solo escritor a la vez). Como en Telnet, un socket roto se cierra antes de volver a conectar, una conexión abierta mientras se ejecuta `Close()` se cierra en el acto, y cerrar dos veces no es un error (cubierto por `internal/connections/rconws_test.go`).
- **`Write(p)`** envía un mensaje JSON por comando, con un identificador que aumenta en cada escritura:

```json
{ "Message": "say Hello\n", "Identifier": 1, "Stacktrace": "" }
```

(El `\n` final es el que añade `ExecuteInMainProcess`.)

---

## Ejemplos de plantilla

Minecraft con RCON (el juego debe tener `enable-rcon=true` y la misma contraseña y puerto):

```json
"run": {
  "command": "java -Xmx${memory}M -jar server.jar nogui",
  "stop": "stop",
  "stdin": { "type": "rcon", "port": "${rconport}", "password": "${rconpassword}" }
}
```

Un servidor con consola de administración por Telnet:

```json
"stdin": { "type": "telnet", "ip": "127.0.0.1", "port": "${telnetport}", "password": "${telnetpassword}" }
```

Un juego que sirve RCON por un WebSocket sin cifrar, como el WebRCON de Rust:

```json
"stdin": { "type": "rconws", "port": "${rconport}", "password": "${rconpass}", "secure": false }
```

Los nombres de variable (`rconport`, `rconpassword`, ...) son los que defina la plantilla; el panel no les da ningún significado especial.

---

## Solución de problemas

| Síntoma | Causa probable |
|---|---|
| Los comandos fallan con `rcon not available` | RCON no está activado en el juego, el puerto o la contraseña no coinciden, o el juego escucha en una dirección que el daemon no alcanza. Active el log de debug para ver `Error connecting to RCON`. |
| El comando se acepta pero no aparece nada en la consola | Es lo esperado con `rcon`: la respuesta solo va al log de debug. La salida propia del juego sigue apareciendo si la imprime por stdout. |
| `rconws` nunca conecta | El esquema no coincide con el del juego: solo ofrece `ws://` y la plantilla no indica `"secure": false`, o espera TLS y la plantilla lo indica. El log de debug muestra `Error waiting for RCON WS socket`. |
| Hay `type` pero los comandos siguen yendo a stdin | El tipo está mal escrito (`rcon-ws` en lugar de `rconws`); el log del daemon muestra `Unknown console type`. |
| `ErrConsoleDetached` | El daemon se reinició mientras un servidor de host (no Docker) seguía en marcha; deténgalo a la fuerza (kill) y vuelva a arrancarlo. |

## Notas de seguridad

- RCON y Telnet no van cifrados. Manténgalos en loopback o en una red privada y no publique sus puertos.
- La `password` forma parte de la definición del servidor; quien pueda leer la definición puede leerla.
- Con `rconws` la contraseña viaja en la ruta de la URL, visible en los logs del servidor remoto. Con `wss://` va cifrada por TLS; con `"secure": false` viaja sin cifrar, como en RCON y Telnet, así que conviene mantener también ese puerto en loopback o en una red privada.
