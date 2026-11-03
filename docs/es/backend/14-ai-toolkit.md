# Kit de herramientas del agente de IA

Cómo trabaja el asistente del servidor con los archivos de un servidor: el kit de `tools/toolkit/`, el bucle del agente que gestiona el function calling de Gemini, cómo se registran y se ofrecen las herramientas, cómo cada llamada queda limitada al servidor de la conversación, cómo funcionan las confirmaciones y qué se registra.

Páginas relacionadas: la guía de uso [Asistente de IA: trabajar con archivos](../features/ai-agent.md); el chat en sí, su contexto y el análisis de registros en [Asistente de IA](../features/ai-assistant.md) y [IA en detalle](../frontend/08-ai-detailed.md); las rutas en [Servidores → IA](../reference/api/03-servers.md#ia) y [Daemon → Archivos](../reference/api/09-daemon.md#archivos); el confinamiento del servidor de archivos en [Seguridad → Acceso a archivos](./09-security.md#acceso-a-archivos). La descripción de los paquetes para desarrolladores está en [`tools/toolkit/README.es.md`](../../../tools/toolkit/README.es.md).

---

## Visión general

| Paquete | Ruta | Función |
|---|---|---|
| `toolkit` | `tools/toolkit/toolkit.go`, `registry.go` | La interfaz `Tool`, los tipos (`read`, `create`, `modify`, `destructive`), `Summary`, `ExecContext`, `Result`, los nombres de los scopes y el `Registry` |
| `security` | `tools/toolkit/security/` | Limpieza de rutas (`CleanPath`, `CleanDir`), los límites de tamaño y búsqueda (`Limits`), el gancho `ProtectedPaths`, `LooksBinary` |
| `workspace` | `tools/toolkit/workspace/` | La interfaz `Workspace` (los archivos de un servidor) y `NewFileServerWorkspace`, construido sobre el `files.FileServer` confinado del nodo |
| `filesystem` | `tools/toolkit/filesystem/` | Las 12 herramientas de archivos y `Register(r)` |
| `agent` | `tools/toolkit/agent/` | El bucle: `Runner`, `Session`, `Step`, `Limits`, `ConfirmationStore` |
| `gemini` | `tools/toolkit/gemini/` | Un cliente REST de Gemini con function calling que implementa `agent.Generator` (sin SDK) |
| Integración en el panel | `internal/web/api/ai_agent.go`, `ai_workspace.go`, `ai_chat.go` | El modo agente de `POST /ai/chat`, `POST /ai/chat/confirm`, el `nodeWorkspace` que llega al servidor a través de su nodo, las entradas de actividad |
| Integración en el nodo | `internal/web/daemon/ai_files.go`, `server.go` | Las rutas `fs/stat`, `fs/copy`, `fs/search`, `PUT file?create=true` y el `DELETE file` reforzado |

El kit no importa `internal/web` y no sabe nada de Gemini fuera del paquete `gemini`. Los nombres de los scopes son cadenas simples (`server.files.view`, `server.files.edit`), así que tampoco importa `internal/scopes`.

## Recorrido de una petición

```
Navegador (assistant-panel.tsx)
   │  POST /api/servers/:serverId/ai/chat           { messages, language }
   │  POST /api/servers/:serverId/ai/chat/confirm   { token, approved, language }
   ▼
Panel  ── server.console, ResolveServerPanel, límite de peticiones (30 / 10 min por usuario)
   │      ¿modo agente? panel.settings.aiFileTools activado + usuario con sesión + server.files.view
   │      agent.Runner.Start / Resume
   │         ├─ gemini.Client.Generate ──► generativelanguage.googleapis.com (gemini-2.5-flash)
   │         └─ Tool.Describe / Tool.Run ──► nodeWorkspace (ligado al servidor de la ruta)
   ▼
Nodo   /daemon/server/<id>/file/*, /fs/stat, /fs/copy, /fs/search   (token del panel)
   │      workspace.NewFileServerWorkspace(server.GetFileServer()) o los manejadores del gestor de archivos
   ▼
files.FileServer: openat2 + RESOLVE_BENEATH bajo la carpeta del servidor
```

1. `chatWithAssistant` (`ai_chat.go`) comprueba la clave de Gemini, valida la conversación (de 1 a 20 mensajes de 1 a 2000 caracteres, el último del usuario), aplica el límite compartido de peticiones de IA y reúne el contexto del servidor (estado, jugadores, rendimiento, caídas, consola) igual que en el chat simple.
2. `aiAgentScopes` decide el modo. El modo agente exige todo esto: `panel.settings.aiFileTools` activado (por defecto `true`), un usuario con sesión en la petición (un cliente OAuth2 sin usuario recibe el chat simple) y `server.files.view` en el servidor. En otro caso la petición pasa al chat de texto (`askGemini`) y responde `{reply, sources}` como antes.
3. En modo agente el panel construye una `agent.Session` solo a partir de la petición: `ServerID` es el identificador del servidor que resolvió la ruta, `UserID` el usuario con sesión, `Scopes` los scopes de archivos de quien llama en ese servidor, `Exec.Workspace` un `nodeWorkspace` para ese servidor y `Language` el idioma de la petición.
4. `Runner.Start` ejecuta el bucle (abajo) con la instrucción de sistema (`agentInstruction`: la instrucción del asistente simple más las reglas de las herramientas) y la conversación como mensajes de texto.
5. La respuesta es `{reply, sources, steps, pending}`. Antes se escriben en el registro de actividad del servidor los cambios que se hicieron.

## El bucle del agente

`agent.Runner` (`tools/toolkit/agent/runner.go`) contiene un `Model` (un `agent.Generator`), un `Registry`, un `ConfirmationStore` y unos `Limits`. El panel mantiene un registro y un almacén de confirmaciones por proceso (`aiRunner`, creado una sola vez con `sync.Once`) y da a cada petición una copia del runner con su propio `gemini.Client`, porque la clave de API se lee en cada petición y un administrador puede cambiarla en cualquier momento.

### Rondas

En cada ronda:

1. Si el contexto de la tarea ha terminado (límite de tiempo) o se ha llegado a `MaxRounds` o `MaxCalls`, el bucle se detiene (véase [Límites de una tarea](#límites-de-una-tarea)).
2. Se llama a `Model.Generate(ctx, system, history, tools)` **solo** con `Registry.Available(session.Scopes)`.
3. La respuesta se guarda en el historial como mensaje `model`. Cada llamada a función recibe un ID: el del propio modelo cuando envió uno único y, si no, uno generado `local-<ronda>-<n>` (`GeneratedIDPrefix`). Los argumentos se copian en profundidad pasando por JSON para que una herramienta no pueda cambiar lo guardado.
4. Una respuesta sin llamadas a funciones termina la tarea: su texto es la respuesta final.
5. Si no, las llamadas de la ronda se tratan en orden:
   - por encima de `MaxCallsPerRound`: paso `refused` y respuesta de error (nunca se ejecuta);
   - una vez alcanzado `MaxCalls`: paso `refused` y respuesta de error;
   - un nombre que no está entre las herramientas ofrecidas: paso `refused` y respuesta de error `unknown tool "…"` o `the tool "…" is not allowed for this user` (nunca se ejecuta);
   - en otro caso `Describe` valida los argumentos; un error da un paso `failed`;
   - una llamada cuyo tipo necesita confirmación (`modify`, `destructive`) queda `pending` y no se ejecuta;
   - cualquier otra llamada (`read`, `create`) se ejecuta en el acto con `Run`: `done` o `failed`.
6. Si no queda ninguna llamada pendiente, las respuestas vuelven al modelo en un solo mensaje `tool` y empieza la ronda siguiente. Si hay alguna pendiente, el bucle se detiene y las devuelve (véase [Confirmaciones](#confirmaciones)); las respuestas de las llamadas que ya se ejecutaron en esa ronda se guardan para después.

Un pánico en `Describe` o `Run` se captura y se convierte en un paso `failed` (`the operation failed unexpectedly`).

### Llamadas y respuestas de funciones

`gemini.Client` (`tools/toolkit/gemini/client.go`) envía un POST a `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent` (`DefaultEndpoint`, sustituible en las pruebas) con la clave en la cabecera `x-goog-api-key`:

- `systemInstruction`: el texto de sistema;
- `contents`: el historial. Los mensajes `user` y `model` conservan su rol; las llamadas a funciones del modelo siguen como partes `functionCall`; las respuestas a ellas (rol `tool` en el agente) se envían con rol `user` y partes `functionResponse` `{id, name, response}`, en el orden de las llamadas. Los IDs generados por el runner (`local-…`) no se envían;
- `tools: [{functionDeclarations: [{name, description, parameters}]}]` construido con las herramientas ofrecidas, y `toolConfig.functionCallingConfig.mode = "AUTO"`;
- `generationConfig`: temperatura 0,4, `maxOutputTokens` (8192 para el agente, 4096 para el chat simple) y `thinkingConfig.thinkingBudget` (1024 para el agente, 512 para el chat simple). Gemini 2.5 cuenta su razonamiento dentro de los tokens de salida, así que sin ese tope el razonamiento podía no dejar sitio para la respuesta o para un `write_file` con un archivo entero;
- las firmas de razonamiento del modelo (`thoughtSignature`) vuelven sin cambios en las partes que las traían (`agent.Part.Signature`), para que el modelo conserve su razonamiento entre rondas.

El cliente lee `candidates[0].content.parts` (como mucho 4 MiB de respuesta): las partes `functionCall` se convierten en llamadas, las de texto en texto, y los resúmenes de razonamiento (`thought`) se descartan. Los motivos de fin `SAFETY`, `RECITATION`, `BLOCKLIST`, `PROHIBITED_CONTENT`, `SPII` e `IMAGE_SAFETY` dan `ErrBlocked`; `MALFORMED_FUNCTION_CALL` y `UNEXPECTED_TOOL_CALL` dan `ErrMalformedCall`. Un candidato sin texto ni llamadas da `ErrEmpty`, o `ErrTooLong` si su motivo de fin es `MAX_TOKENS`. Sin clave falla con `ErrNotConfigured`; un fallo de transporte da `ErrUnreachable`. Un estado distinto de 200 da un `*gemini.ServiceError` (`CheckResponse`), cuyo `Kind` es:

| Tipo | Cuándo |
|---|---|
| `ErrKeyRejected` | `401`, `403`, un motivo `API_KEY_*` o un mensaje sobre la clave (clave no válida, restringida o revocada, o la API sin activar) |
| `ErrQuota` | `429` (`RESOURCE_EXHAUSTED`) |
| `ErrUnavailable` | `500` o más (saturado o caído) |
| `ErrRejected` | cualquier otro estado (por ejemplo `400` para una petición que Gemini no acepta, `404` para un modelo desconocido) |

Su `Error()` es el texto seguro de su tipo; `Detail` guarda el estado, el motivo y hasta 300 caracteres del mensaje de Google, solo para el log del panel, y `RetryAfter` el `retryDelay` que pidió Google.

`Generate` hace hasta tres intentos. Vuelve a intentarlo tras 1 s y luego 3 s (`Client.Backoff`) cuando el servicio está saturado (`ErrUnavailable`) o no se alcanza, y tras un `429` solo si Google pide esperar 10 s o menos (un límite por minuto); un límite diario se informa en el acto. La espera termina antes si termina el contexto de la tarea. El cliente HTTP tiene un tiempo máximo de 90 s como red de seguridad.

Lo que vuelve al modelo como respuesta de función:

- un error de la herramienta: `{"error": "<mensaje>"}`;
- un resultado: el `Output` de la herramienta como objeto JSON (lo que no es un objeto se envuelve como `{"result": …}`; sin salida, `{"ok": true, "summary": "…"}`);
- una llamada rechazada por el usuario: `{"declined": true, "reason": "the user declined this operation"}`.

Cada cadena de una respuesta, y cada error de un paso, se recorta a 64 KiB (`maxOutputString`) y se le añade `[truncated]`.

### Límites de una tarea

`agent.DefaultLimits()` (un campo a cero en `Runner.Limits` toma el valor por defecto; el panel usa los valores por defecto):

| Límite | Valor por defecto | Al alcanzarlo |
|---|---|---|
| `MaxRounds` | 10 | El bucle se detiene y la respuesta la escribe el runner |
| `MaxCallsPerRound` | 8 | Las llamadas de más de esa respuesta quedan `refused` |
| `MaxCalls` | 30 | Las llamadas siguientes quedan `refused`; la ronda siguiente detiene la tarea |
| `Timeout` | 120 s | Por cada llamada a `Start` o `Resume`; la tarea se detiene |

Una parada por rondas, llamadas o tiempo no hace fallar la petición: la respuesta dice por qué (`He detenido esta tarea porque necesitaba demasiados pasos (límite: 10 rondas).`, `… demasiadas operaciones (límite: 30).`, `… estaba tardando demasiado.`) y enumera los pasos hechos hasta ese momento, o dice que no se ha cambiado nada. Estos textos los traduce el runner (`messages.go`, en inglés y español). Una petición cancelada por el cliente termina, en cambio, con el error del contexto.

## El registro y los scopes

`toolkit.Registry` (`registry.go`) asocia nombres a herramientas. Se llena una vez al arrancar y se lee de forma concurrente:

- `Register(t)` provoca un pánico con una herramienta nula, un nombre vacío o un nombre repetido: dos herramientas con el mismo nombre harían ambigua una llamada, así que es un error de programación.
- `Get(name)` devuelve una herramienta.
- `Available(scopes)` devuelve las herramientas cuyo `Scope()` está en `scopes`, ordenadas por nombre.

El panel traduce los scopes resueltos de quien llama en el servidor (`callerScopes`, ya acotados por los de un cliente OAuth2) a los nombres del kit: `server.files.view` → `toolkit.ScopeFilesView`, `server.files.edit` → `toolkit.ScopeFilesEdit`. Se usa `scopes.ContainsScope`, así que los administradores y quien tiene `server.admin` cuentan como si tuvieran ambos. Se leen en cada petición, de modo que un permiso retirado se aplica en el siguiente chat o confirmación.

Una herramienta para la que el usuario no tiene scope **no se declara a Gemini en absoluto**; si el modelo la llama de todos modos, la llamada se rechaza y nunca se ejecuta.

### Las herramientas de archivos

`filesystem.Register(reg)` añade las 12. La ruta del nodo es la que llama el `nodeWorkspace` del panel en el nodo del servidor (`/daemon/server/<id>/…`).

| Herramienta | Tipo | Scope | Qué hace | Rutas del nodo |
|---|---|---|---|---|
| `list_directory` | read | `server.files.view` | Lista una carpeta, primero las carpetas; como mucho 1000 entradas (`truncated` indica si había más) | `GET file/<dir>` |
| `read_file` | read | `server.files.view` | Devuelve un archivo de texto tal como está guardado, con `size` y `totalLines`; `offset` / `limit` eligen un rango de líneas | `GET file/<ruta>` |
| `file_info` | read | `server.files.view` | Tipo, tamaño y fecha de modificación, sin leerlo | `GET fs/stat` |
| `search_files` | read | `server.files.view` | Nombres que contienen `query` o, con `in_content`, líneas de archivos de texto; sin distinguir mayúsculas | `GET fs/search` |
| `create_file` | create | `server.files.edit` | Archivo de texto nuevo; falla si ya hay algo; crea las carpetas superiores que falten | `GET fs/stat`, `PUT file/<ruta>?create=true` |
| `create_directory` | create | `server.files.edit` | Carpeta y superiores; si ya existe, termina sin cambios; falla si hay un archivo en medio | `GET fs/stat`, `PUT file/<ruta>?folder` |
| `copy_path` | create | `server.files.edit` | Copia un archivo o un árbol de carpetas a una ruta que no debe existir; los enlaces simbólicos de dentro se omiten | `GET fs/stat`, `POST fs/copy` |
| `edit_file` | modify | `server.files.edit` | Sustituye `old_text` por `new_text`; `old_text` debe aparecer exactamente `expected_occurrences` veces (por defecto 1) | `GET file/<ruta>`, `PUT file/<ruta>` |
| `write_file` | modify | `server.files.edit` | Sustituye todo el contenido de un archivo existente | `GET fs/stat`, `PUT file/<ruta>` |
| `move_path` | modify | `server.files.edit` | Mueve (y puede renombrar) a una ruta que no debe existir; crea las carpetas superiores que falten | `GET fs/stat`, `PUT file/<superior>?folder`, `POST file/<origen>?destination=<destino>` |
| `rename_path` | modify | `server.files.edit` | Nombre nuevo en la misma carpeta (`new_name` es un nombre, no una ruta) | como `move_path` |
| `delete_path` | destructive | `server.files.edit` | Borra un archivo, o una carpeta con todo su contenido; la carpeta del servidor se rechaza | `GET fs/stat`, `DELETE file/<ruta>` |

Detalles que conviene conocer:

- `read_file` devuelve el texto con sus finales de línea, de modo que puede usarse como `old_text`. Los archivos binarios (un byte NUL en los primeros 8 KiB) se rechazan, igual que los de más de 1 MiB.
- `edit_file` vuelve a leer el archivo en `Run`, así que se aplica al archivo tal como esté en ese momento. Cuando `old_text` usa `\n` pero el archivo usa `\r\n`, encaja con los finales de línea de Windows. Se rechazan 0 coincidencias, un número distinto de `expected_occurrences`, un `old_text` vacío, un `new_text` igual a `old_text` o un resultado de más de 1 MiB.
- `create_file`, `write_file`: contenido de hasta 1 MiB.
- Copiar y mover rechazan un origen igual al destino y un destino dentro del origen.

## Escribir una herramienta

Una herramienta implementa `toolkit.Tool`:

```go
type Tool interface {
	Name() string                // snake_case, unique in a Registry
	Description() string         // for the model
	Parameters() map[string]any  // JSON schema object: type, properties, required
	Kind() Kind                  // KindRead, KindCreate, KindModify or KindDestructive
	Scope() string               // ScopeFilesView or ScopeFilesEdit
	Describe(ctx context.Context, ec ExecContext, args map[string]any) (Summary, error)
	Run(ctx context.Context, ec ExecContext, args map[string]any) (Result, error)
}
```

Reglas de las que depende el bucle:

- **Validar en `Describe` y en `Run`.** `Describe` se ejecuta antes de mostrar o ejecutar una llamada y otra vez cuando se reanuda una llamada confirmada; `Run` puede recibir argumentos válidos que describen un archivo que ha cambiado desde entonces.
- **`Describe` no debe cambiar nada.** Puede leer (por ejemplo, para contar coincidencias) y debe poder llamarse dos veces. `Summary.Text` es la frase que aparece en la lista de pasos y en la tarjeta de confirmación; `Summary.Paths` son las rutas relativas al servidor implicadas.
- **Tocar archivos solo a través de `ec.Workspace`**, que ya está ligado al servidor. No aceptar nunca un argumento de servidor.
- **Limpiar cada ruta** con `security.CleanPath` (un archivo o carpeta dentro de la carpeta del servidor) o `security.CleanDir` (donde `""` o `.` es la carpeta del servidor) y rechazar las rutas protegidas (`security.IsProtected` para lecturas, `security.TouchesProtected` para cambios).
- **Los errores los ven el usuario y el modelo**: envolver un valor `toolkit.Err…` con `%w` y no incluir nunca una ruta del host. Los errores del workspace ya siguen esta regla.
- **Elegir el tipo con honestidad.** `create` no debe sustituir nunca nada; todo lo que cambia o elimina datos existentes es `modify` o `destructive`, que es lo que obliga al usuario a confirmarlo.

Una herramienta de lectura mínima, escrita fuera del paquete `filesystem`:

```go
package mytools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
)

// CountLines counts the lines of a text file of the server of the conversation
type CountLines struct{}

func (CountLines) Name() string        { return "count_lines" }
func (CountLines) Description() string { return "Count the lines of a text file of this server." }
func (CountLines) Kind() toolkit.Kind  { return toolkit.KindRead }
func (CountLines) Scope() string       { return toolkit.ScopeFilesView }

func (CountLines) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File relative to the server folder, for example server.properties."},
		},
		"required": []string{"path"},
	}
}

func (CountLines) path(args map[string]any) (string, error) {
	raw, _ := args["path"].(string)
	p, err := security.CleanPath(raw)
	if err != nil {
		return "", fmt.Errorf("%w: path: %w", toolkit.ErrInvalidArgs, err)
	}
	if security.IsProtected(p) {
		return "", fmt.Errorf("%w: %s", toolkit.ErrProtected, p)
	}
	return p, nil
}

func (t CountLines) Describe(_ context.Context, _ toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	p, err := t.path(args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	return toolkit.Summary{Text: "Count the lines of " + p, Paths: []string{p}}, nil
}

func (t CountLines) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	p, err := t.path(args)
	if err != nil {
		return toolkit.Result{}, err
	}
	if ec.Workspace == nil {
		return toolkit.Result{}, errors.New("the files of this server are not available")
	}
	data, err := ec.Workspace.Read(ctx, p, security.Limits.MaxReadBytes)
	if err != nil {
		return toolkit.Result{}, err
	}
	n := strings.Count(string(data), "\n")
	return toolkit.Result{
		Output:  map[string]any{"path": p, "lines": n},
		Summary: fmt.Sprintf("Counted %d lines in %s", n, p),
	}, nil
}
```

Cómo registrarla:

- **Una herramienta de archivos** va en `tools/toolkit/filesystem`: se añade su constructor a `Tools()`. Allí el tipo `base` proporciona `Name`, `Description`, `Kind`, `Scope` y `Parameters` a partir de una lista de `param`, y las funciones auxiliares `pathArg`, `dirArg`, `readText`, `checkReadable`, `checkChangeable` y `mustNotExist` hacen las comprobaciones habituales. [`tools/toolkit/README.es.md`](../../../tools/toolkit/README.es.md) tiene el mismo ejemplo escrito de esa forma.
- **Cualquier otra herramienta** se registra donde el panel llena el registro, en `aiRunner` (`internal/web/api/ai_agent.go`): `registry.Register(mytools.CountLines{})` junto a `filesystem.Register(registry)`.

Una herramienta de tipo `create`, `modify` o `destructive` se registra en el registro de actividad como `ai.<nombre>`: hay que añadir la frase en `servers.activity.actions.ai.<nombre>` de `client/frontend/src/lib/locales/en.json` y `es.json` (una acción desconocida muestra una frase genérica). Conviene añadir pruebas del caso normal, de argumentos no válidos y del aislamiento (las existentes usan `NewFileServerWorkspace` sobre carpetas temporales y se ejecutan en los dos modos de resolución con `internal/resolvemode`).

## Aislamiento

### El servidor sale de la ruta

El servidor es el que `ResolveServerPanel` resolvió para `/api/servers/:serverId/ai/…`, después de comprobar `server.console` sobre él. `aiSession` construye el workspace para ese servidor (`newNodeWorkspace(nodeService, server)`), cuyo prefijo de URL es `/daemon/server/<identificador escapado>` en el nodo de ese servidor. Ninguna herramienta tiene un parámetro de servidor y nada de lo que escriba el modelo puede cambiar el prefijo: cada ruta es una ruta relativa al servidor que se añade detrás.

### Rutas

`security` comprueba cada ruta antes de llamar a nada: en la herramienta, otra vez en `nodeWorkspace` y una tercera vez en el nodo, en el workspace de las rutas `fs/*` (las rutas del gestor de archivos se apoyan solo en el servidor de archivos):

- `CleanPath` / `CleanDir` quitan espacios, convierten `\` en `/` y rechazan: un byte NUL, más de 4096 bytes, una letra de unidad de Windows (`C:`), una `/` inicial, cualquier elemento `..` y más de 64 elementos. Devuelven la forma de `path.Clean` sin `./` inicial.
- `CleanPath` rechaza la propia carpeta del servidor (`""`, `.`), de modo que una herramienta que necesita un archivo o carpeta (`delete_path`, `create_file`…) nunca puede nombrar la raíz. `delete_path` lo dice expresamente: `the server folder itself cannot be deleted`.
- `nodeWorkspace.filePath` escapa cada elemento con `url.PathEscape`, así que `#`, `?` o `%` en un nombre siguen formando parte del nombre.

### En el nodo: el servidor de archivos confinado

En el nodo cada operación pasa por el `files.FileServer` del servidor, que abre cada ruta por debajo de la carpeta del servidor con `openat2` y `RESOLVE_BENEATH` (o, con `security.forceOpenat`, carpeta a carpeta con `O_NOFOLLOW`): una ruta o un enlace simbólico que saldría de la carpeta lo rechaza el kernel, no unas comprobaciones de texto. Véase [Seguridad → Acceso a archivos](./09-security.md#acceso-a-archivos).

Los archivos internos del panel para un servidor (`.cron`, `.history`, `.crashes`, `.notify`, `.metrics`, `.worldmap`, `.modpack`) están junto a la carpeta del servidor, fuera de su raíz, así que ninguna herramienta puede alcanzarlos. Una ruta que sale de la raíz vuelve como `the path must stay inside the server folder`.

### Enlaces simbólicos

- Leer y listar nunca siguen un enlace fuera de la carpeta del servidor: un enlace que sale de ella, o roto, se lista como tal y no se puede abrir. `fs/stat` describe un enlace que queda dentro por aquello a lo que apunta.
- `Write` rechaza un enlace simbólico existente como destino y abre con `O_NOFOLLOW` (un enlace colocado después de la comprobación se rechaza en vez de escribirse a través de él). Una creación usa `O_EXCL`, así que nunca sustituye nada.
- `Copy` rechaza un enlace simbólico como origen; dentro de una carpeta copiada se omiten los enlaces simbólicos, dispositivos, sockets y tuberías.
- La búsqueda nunca entra en una carpeta enlazada (el nombre del propio enlace sí puede coincidir).
- `Delete` de un enlace simbólico elimina el propio enlace (`unlinkat` sobre su carpeta); aquello a lo que apunta no se toca. La ruta `DELETE file` del nodo hace lo mismo desde esta función: antes, borrar un enlace a una carpeta vaciaba esa carpeta y luego no conseguía eliminar el enlace.
- `Move` renombra la propia entrada; un enlace se mueve como enlace.

### Rutas protegidas

`security.ProtectedPaths` es una lista de rutas relativas al servidor que ninguna herramienta puede leer ni cambiar (`IsProtected` para lecturas y listados, `TouchesProtected` para cambios, que también cubre una carpeta que contenga una ruta protegida). **Está vacía por defecto**: los archivos propios del panel ya están fuera de la carpeta del servidor. Es un gancho para el código (se fija una vez al arrancar, antes de que se ejecute ninguna herramienta); ninguna clave de configuración la llena. Las entradas protegidas se omiten de los listados y de los resultados de búsqueda, y se rechazan con `this path is protected and cannot be used`.

## Permisos y confirmaciones

### Quién puede usar el modo agente

| Requisito | Dónde se comprueba |
|---|---|
| `server.console` en el servidor | Middleware de las rutas `/ai/chat` y `/ai/chat/confirm` |
| Una clave de Gemini (`panel.settings.geminiApiKey`) | Los dos manejadores (`400` sin ella) |
| `panel.settings.aiFileTools` activado | `aiAgentScopes` (chat: modo simple si está desactivado); confirmación: `409 ErrAIConfirmationInvalid` si está desactivado |
| Un usuario con sesión | Chat: modo simple sin él; confirmación: `403 ErrNoPermission` |
| `server.files.view` | Chat: modo simple sin él; confirmación: `403 ErrNoPermission` |
| `server.files.edit` | Por herramienta: sin él no se ofrecen las herramientas de crear, modificar ni borrar |

### Tipos

| Tipo | Herramientas | Confirmación |
|---|---|---|
| `read` | `list_directory`, `read_file`, `file_info`, `search_files` | No |
| `create` | `create_file`, `create_directory`, `copy_path` | No: nunca sustituyen nada |
| `modify` | `edit_file`, `write_file`, `move_path`, `rename_path` | Sí |
| `destructive` | `delete_path` | Sí |

`Kind.NeedsConfirmation()` es verdadero para `modify` y `destructive`.

### Confirmaciones

Cuando una ronda tiene llamadas pendientes, `round` guarda una `confirmationEntry` en el `ConfirmationStore` (`tools/toolkit/agent/store.go`) y devuelve `Outcome.Pending`:

```json
{ "token": "<64 caracteres hexadecimales>", "actions": [ { "id": "…", "tool": "delete_path", "kind": "destructive", "summary": "Delete the folder world_old and everything in it", "paths": ["world_old"], "status": "pending" } ], "expiresAt": "2026-11-02T10:10:00Z" }
```

La entrada contiene el texto de sistema, el historial hasta el mensaje del modelo con las llamadas, las respuestas de las llamadas que ya se ejecutaron en esa ronda, las llamadas pendientes y los contadores de rondas y llamadas. La respuesta de ese turno es el propio texto del modelo junto a las llamadas (puede estar vacío).

| Regla | Implementación |
|---|---|
| Token | 32 bytes aleatorios (`crypto/rand`), en hexadecimal |
| Ligado a | El ID del usuario y el identificador del servidor; además, cada llamada pendiente se guarda con el SHA-256 de su nombre de herramienta y sus argumentos en JSON canónico (`callHash`) |
| Duración | 10 minutos (`ConfirmationTTL`) |
| Una por usuario y servidor | Una entrada pendiente nueva sustituye a la anterior del mismo usuario en el mismo servidor |
| Un solo uso | `take` elimina la entrada antes de comprobar o ejecutar nada |
| Capacidad | Como mucho 1000 entradas en memoria (`MaxPendingConfirmations`); si se llena, se descarta la que caduca antes. Las caducadas se barren al guardar una nueva |
| Almacenamiento | Solo en la memoria del panel: un reinicio del panel descarta todas las confirmaciones pendientes |

### Responder a una confirmación

`POST /api/servers/:serverId/ai/chat/confirm` con `{token, approved, language}` (`approved`: como mucho 64 IDs; el token y cada ID, como mucho 128 caracteres) llama a `Runner.Resume`:

1. `take(token, usuario, servidor)`: un token desconocido o caducado, o ligado a otro usuario o servidor, da `ErrConfirmationInvalid`. Si el usuario o el servidor no coinciden, **la entrada se queda donde está**, para que otra persona con el token no pueda gastar la confirmación del propietario; una caducada se elimina.
2. Se vuelve a comprobar el hash de cada llamada guardada, y cada ID aprobado debe ser una de las llamadas guardadas; si no, `ErrConfirmationInvalid`. En ese punto la entrada ya está gastada, así que una respuesta incorrecta no puede repetirse con el mismo token.
3. Las llamadas rechazadas reciben la respuesta de rechazo. Las aprobadas se tratan de nuevo desde el principio con los scopes y los archivos **actuales**: la herramienta tiene que seguir ofreciéndose para los scopes del usuario (si no, `refused`), `Describe` vuelve a validar (un archivo que cambió puede fallar ahora) y después se ejecuta `Run`.
4. Las respuestas vuelven al modelo en su orden original y el bucle continúa, con los mismos contadores de rondas y llamadas y un nuevo tiempo máximo de 120 s. Puede detenerse de nuevo para otra confirmación.

El panel responde a `ErrConfirmationInvalid` con `409 ErrAIConfirmationInvalid` (mensaje `this request expired, was already answered or belongs to someone else: ask again`). Rechazar todo es enviar una lista `approved` vacía.

## Ejecución y auditoría

- El panel nunca toca él mismo los archivos de un servidor. `nodeWorkspace` (`ai_workspace.go`) envía cada operación al nodo del servidor con `services.Node.CallNodeContext` y el token firmado del panel (dentro del mismo proceso para el nodo local). La tabla de [Las herramientas de archivos](#las-herramientas-de-archivos) enumera las rutas.
- Las sobrescrituras (`edit_file`, `write_file`) usan la ruta `PUT file` del gestor de archivos, así que el nodo guarda el texto anterior en el [historial de archivos](../features/file-history.md#cuándo-se-guarda-una-versión) con las mismas reglas que una edición en la pestaña **Archivos**. Las creaciones (`?create=true`), copias, movimientos y borrados no guardan versiones.
- Se aplican los límites de disco: `PUT file` comprueba el disco y cuenta los bytes escritos; `fs/copy` mide lo que se copiaría y lo rechaza con `ErrDiskLimitExceeded` si no cabe, y después reinicia el uso de disco en caché.
- **Actividad.** Después de cada `Start` o `Resume`, `recordAgentActivity` escribe en el registro de actividad del servidor una entrada por cada paso `done` de tipo `create`, `modify` o `destructive`, a nombre del usuario y con su IP: acción `ai.<herramienta>` (`ai.create_file`, `ai.create_directory`, `ai.copy_path`, `ai.edit_file`, `ai.write_file`, `ai.move_path`, `ai.rename_path`, `ai.delete_path`) y como detalle las rutas unidas con ` → `. Las lecturas no se registran; van al log de depuración. También se registran los pasos que se ejecutaron antes de que fallara una tarea. El middleware de actividad ignora `POST /ai/chat` y `POST /ai/chat/confirm`, así que estas entradas son las únicas. Pertenecen a la familia `files`. Véase [Registro de actividad](../features/activity-and-access-log.md).

## Errores

Los errores de las herramientas y del workspace son seguros para el usuario: llevan como mucho una ruta relativa al servidor, nunca una ruta del host. Los valores centinela del workspace (`workspace.ErrNotFound`, `ErrExists`, `ErrTooLarge`, `ErrNotDir`, `ErrIsDir`, `ErrSymlink`, `ErrNotRegular`, `ErrInvalid`, `ErrPermission`, `ErrNoSpace`, `ErrFailed`, y los del paquete security `ErrInvalidPath`, `ErrOutsideRoot`, `ErrProtected`) se comparan con `errors.Is` en las dos capas.

- En el nodo, las rutas `fs/*` responden a cada centinela con un estado y un código (`FileErrorStatus` en `internal/web/daemon/ai_files.go`, enumerados en [Daemon → Archivos](../reference/api/09-daemon.md#archivos)); un error desconocido se responde como `ErrFileOperationFailed` sin su texto y se anota en el log.
- En el panel, `answerError` traduce el código de vuelta (`FileErrorForCode`). Para las rutas del gestor de archivos, que no envían estos códigos, se guía por el estado: `404` → no encontrado, `409` → ya existe, `413` → demasiado grande, `400` → fuera de la carpeta del servidor, `403` → permiso, cualquier otro → fallo; su texto no se transmite. `ErrDiskLimitExceeded` se convierte en `ErrNoSpace` con el mensaje de disco del panel.
- Un nodo inalcanzable da `could not reach the node that hosts this server`; un nodo que rechaza el token del panel da `the node that hosts this server refused the panel's credentials; an administrator has to check the node` (los detalles, en el log del panel).
- Una llamada de herramienta fallida no hace fallar la petición: el paso queda `failed` con su error, y el modelo recibe el error para actuar en consecuencia.
- Una tarea fallida (error del modelo, petición cancelada) responde `{"error": "…", "reason": "…", "steps": […], "pending": null}`, con `502`, o `504` si se agotó el tiempo. `classifyAIFailure` (`ai_agent.go`) convierte el error en un código `reason` estable (`keyRejected`, `quota`, `unavailable`, `unreachable`, `rejected`, `unreadable`, `empty`, `tooLong`, `blocked`, `malformed`, `notConfigured`, `timeout` o `failed`) y un texto en inglés que se puede mostrar, y anota la causa en el log del panel: `The AI service did not accept the assistant's request (status 429 RESOURCE_EXHAUSTED: …)` para un `ServiceError`, y el propio error para el resto. El chat simple responde el mismo `{error, reason}` con la misma función.

## Límites

Límites del kit (`security.Limits`, fijos en el código):

| Límite | Valor |
|---|---|
| `MaxReadBytes` (archivo más grande que devuelve `read_file`) | 1 MiB |
| `MaxWriteBytes` (contenido más grande de `create_file`, `write_file`) | 1 MiB |
| `MaxEditBytes` (archivo más grande con el que trabaja `edit_file`, antes y después) | 1 MiB |
| `MaxListEntries` | 1000 |
| `MaxSearchDepth` | 8 carpetas por debajo del inicio |
| `MaxSearchFiles` (entradas revisadas) | 5000 |
| `MaxSearchResults` | 200 |
| `MaxSearchFileBytes` (los archivos mayores no se buscan) | 1 MiB |
| `MaxQueryLength` | 200 caracteres |
| `MaxMatchText` (caracteres de una línea coincidente) | 300 |
| `SniffBytes` (se busca un byte NUL para considerar binario un archivo) | 8 KiB |
| Ruta | 4096 bytes, 64 elementos |
| `new_name` de `rename_path` | 255 bytes, sin `/` ni `\` |

Límites del agente, del modelo y de la API:

| Límite | Valor |
|---|---|
| Rondas / llamadas por respuesta / llamadas por tarea | 10 / 8 / 30 (`agent.DefaultLimits`) |
| Tiempo por `Start` o `Resume` | 120 s |
| Cadenas devueltas al modelo | 64 KiB cada una |
| Respuesta de Gemini | 8192 tokens de salida, de ellos hasta 1024 de razonamiento (chat simple: 4096 y 512), temperatura 0,4, como mucho 4 MiB leídos |
| Intentos con Gemini | 3 por llamada al modelo, separados 1 s y 3 s, solo si está saturado o no se alcanza (o un `429` que pide 10 s o menos) |
| Confirmaciones pendientes | 10 min, una por usuario y servidor, 1000 en memoria |
| Petición de confirmación | 64 IDs aprobados; token e IDs de hasta 128 caracteres |
| Límite de peticiones | 30 peticiones de IA por usuario cada 10 minutos, compartidas con el chat simple y el análisis de registros; un chat o una confirmación cuentan una vez, haga su tarea las llamadas a Gemini que haga |
| Respuestas del nodo que lee el panel | Listados 16 MiB, otro JSON 4 MiB, cuerpos de error 64 KiB |

## Lo que el asistente no puede hacer

- Ejecutar nada: no hay herramienta para comandos de shell, comandos de consola, acciones de encendido ni instalaciones. La instrucción de sistema también lo dice.
- Llegar a otro servidor: el workspace está ligado al servidor de la ruta, y las rutas son relativas a su carpeta.
- Borrar o sustituir la propia carpeta del servidor, ni salir de ella con `..`, rutas absolutas o enlaces simbólicos.
- Llegar a los archivos del panel para cada servidor (`.cron`, `.history`…), que están fuera de la carpeta del servidor.
- Leer o editar archivos binarios o de más de 1 MiB.
- Cambiar los ajustes del panel, los usuarios o los permisos.

## Limitaciones conocidas

- **Las confirmaciones pendientes viven en la memoria del panel.** Un reinicio del panel las descarta, y también una petición más reciente del mismo usuario en el mismo servidor. Al responder entonces se recibe `409`; hay que volver a preguntar.
- **No hay ruta para cancelar.** Borrar la conversación en el navegador solo quita la tarjeta allí; el token sigue siendo válido en el panel hasta que caduca o se sustituye.
- **La conversación que se reenvía es solo texto.** El navegador envía el texto de los mensajes anteriores; los pasos y el contenido de los archivos que el modelo leyó en una respuesta anterior no forman parte de la petición siguiente, así que el modelo vuelve a leer los archivos cuando los necesita.
- **Los resúmenes de los pasos y los errores de las herramientas están en inglés.** Las frases vienen de las herramientas (también van al modelo y a los registros); solo los mensajes de parada del runner están traducidos. En otros idiomas, el chat muestra una frase traducida construida con el nombre de la herramienta y sus rutas (`servers.assistant.tools.<tool>` en las traducciones), y la carpeta del servidor como `/`.
- **Archivos grandes.** Los archivos de más de 1 MiB no se pueden leer ni editar, y la búsqueda en el contenido los omite, aunque el error de lectura sugiere buscar en ellos.
- **En la práctica se escriben archivos pequeños.** La respuesta del modelo está limitada a 8192 tokens de salida, así que el contenido que puede escribir en una llamada queda muy por debajo del límite de 1 MiB; un archivo mayor termina en `ErrTooLong` y se pide al usuario que lo solicite por partes.
- **No es atómico entre pasos.** Mover comprueba que el destino está libre y después mueve; una llamada confirmada se vuelve a validar, pero los archivos pueden cambiar entre comprobaciones.
- **Los nodos deben tener la misma versión.** Un nodo sin `fs/stat`, `fs/copy` ni `fs/search` responde `404` a esas rutas: `file_info`, `search_files` y `copy_path` fallan, y las comprobaciones de existencia de las demás herramientas leen «no encontrado», así que las operaciones con archivos se comportan mal. Hay que actualizar los nodos junto con el panel.
- **Las peticiones sin usuario** (clientes OAuth2) reciben siempre el chat simple.

## Pruebas

| Paquete | Qué cubre |
|---|---|
| `tools/toolkit/security` | Limpieza de rutas (salidas con `..`, absolutas, letras de unidad, barras invertidas, NUL, longitud, profundidad), rutas protegidas |
| `tools/toolkit/workspace` | `NewFileServerWorkspace` sobre carpetas temporales: todas las operaciones, no escribir a través de un enlace, búsqueda sin seguir enlaces, límites de búsqueda, rutas protegidas, una petición cancelada |
| `tools/toolkit/filesystem` | El caso normal de cada herramienta, argumentos no válidos, ediciones con 0 o 2 coincidencias, sobrescribir solo con `write_file`, aislamiento entre dos servidores, en los dos modos de resolución (`internal/resolvemode`) |
| `tools/toolkit/agent` | Tareas de varias rondas, herramientas desconocidas y fuera de los scopes rechazadas, confirmaciones, `Resume` con otro usuario o servidor, un token reutilizado o caducado, un ID aprobado desconocido, llamadas rechazadas, límites |
| `tools/toolkit/gemini` | Forma de la petición (herramientas, respuestas de funciones, IDs, presupuesto de razonamiento, firmas de razonamiento), lectura de la respuesta, clasificación de errores y reintentos con `httptest` |
| `tools/toolkit` | Registro |
| `internal/web/api` (`ai_agent_test.go`, `ai_workspace_test.go`) | Ida y vuelta a través de un nodo, confirmación ligada a usuario y servidor, acciones rechazadas, sin permiso de edición, permisos comprobados de nuevo al confirmar, chat simple cuando está desactivado, errores del modelo seguros, rutas, enlaces y fallos de `nodeWorkspace` |
| `internal/web/daemon` (`ai_files_test.go`) | `fs/stat`, `fs/copy` (con el límite de disco), `fs/search`, `PUT ?create=true`, `DELETE` de un enlace simbólico y de la carpeta del servidor |

```sh
go test ./tools/toolkit/... ./internal/web/api/... ./internal/web/daemon/...
```
