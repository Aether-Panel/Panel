# Toolkit del agente de IA

[English](README.md)

El toolkit permite que el asistente de IA de un servidor consulte y cambie **los archivos de ese servidor** mediante function calling de Gemini. Cada herramienta trabaja sobre un workspace ligado al servidor de la conversación: ninguna herramienta recibe un servidor como argumento y todas las rutas son relativas a la carpeta del servidor.

La arquitectura, el flujo de function calling, los permisos y las confirmaciones se describen en [docs/es/backend/14-ai-toolkit.md](../../docs/es/backend/14-ai-toolkit.md).

## Estructura

| Ruta | Contenido |
|------|-----------|
| `toolkit.go` | `Tool`, `Kind` (read, create, modify, destructive), `Summary`, `ExecContext`, `Result`, nombres de permisos, errores comunes |
| `registry.go` | `Registry`: `Register` (entra en pánico con un nombre repetido), `Get`, `Available(scopes)` |
| `security/` | `CleanPath` / `CleanDir`, los límites de tamaño y búsqueda (`Limits`), `ProtectedPaths`, `LooksBinary` |
| `workspace/` | la interfaz `Workspace` y `NewFileServerWorkspace`, sobre el `files.FileServer` confinado del nodo |
| `filesystem/` | las 12 herramientas de archivos y `Register(r)` |
| `agent/` | el bucle del agente: rondas, confirmaciones, límites |
| `gemini/` | el cliente de function calling de Gemini |
| `internal/resolvemode/` | ayuda de tests que los ejecuta con y sin openat2 |

## Las herramientas de archivos

| Herramienta | Tipo | Permiso | Confirmación |
|-------------|------|---------|--------------|
| `list_directory`, `read_file`, `file_info`, `search_files` | read | `server.files.view` | no |
| `create_file`, `create_directory`, `copy_path` | create | `server.files.edit` | no (nunca reemplazan nada) |
| `edit_file`, `write_file`, `move_path`, `rename_path` | modify | `server.files.edit` | sí |
| `delete_path` | destructive | `server.files.edit` | sí |

## Añadir una herramienta

1. Escribe un tipo que implemente `toolkit.Tool`. En `filesystem/`, incrusta `base` para tener `Name`, `Description`, `Kind`, `Scope` y `Parameters`.
2. Valida los argumentos **tanto** en `Describe` como en `Run` (Run puede llamarse sola, o después de que el archivo haya cambiado). Las rutas pasan por `pathArg` / `dirArg`, que usan `security.CleanPath` / `CleanDir`.
3. Toca archivos solo a través de `ec.Workspace`. Devuelve errores que envuelvan un valor `toolkit.Err…` y que nunca contengan una ruta del host.
4. Añádela en `Tools()` y escribe tests (caso normal, argumentos inválidos, aislamiento).

Una herramienta de lectura mínima:

```go
type countLines struct{ base }

func newCountLines() countLines {
	return countLines{base{
		name:        "count_lines",
		description: "Count the lines of a text file of this server.",
		kind:        toolkit.KindRead,
		scope:       toolkit.ScopeFilesView,
		params: []param{
			{"path", "string", "File to count. " + pathRules, true},
		},
	}}
}

func (t countLines) Describe(_ context.Context, _ toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	p, err := pathArg(args, "path")
	if err != nil {
		return toolkit.Summary{}, err
	}
	return toolkit.Summary{Text: "Count the lines of " + p, Paths: []string{p}}, nil
}

func (t countLines) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	p, err := pathArg(args, "path")
	if err != nil {
		return toolkit.Result{}, err
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return toolkit.Result{}, err
	}
	data, err := readText(ctx, ws, p, security.Limits.MaxReadBytes)
	if err != nil {
		return toolkit.Result{}, err
	}
	n := strings.Count(string(data), "\n")
	return toolkit.Result{Output: map[string]any{"path": p, "lines": n}, Summary: fmt.Sprintf("Counted %d lines in %s", n, p)}, nil
}
```

Las descripciones van en inglés porque las lee el modelo.

## Reglas de seguridad

- **Un servidor por workspace.** El servidor sale de la ruta autenticada, nunca del modelo. Un workspace no puede nombrar otro servidor.
- **Las rutas se comprueban dos veces.** `security.CleanPath` rechaza rutas absolutas, letras de unidad, cualquier elemento `..`, bytes NUL, más de 4096 caracteres y más de 64 carpetas. Después, el file server del nodo abre cada ruta por debajo de la carpeta del servidor (openat2 con `RESOLVE_BENEATH`, o carpeta a carpeta sin seguir enlaces), así que el kernel rechaza un enlace simbólico que salga de la carpeta.
- **Los enlaces no se siguen donde importa.** Las escrituras rechazan un enlace como destino (`O_NOFOLLOW`), las creaciones son atómicas (`O_EXCL`), las copias se saltan los enlaces, la búsqueda nunca entra en ellos y borrar un enlace borra solo el enlace.
- **Límites.** Leer, escribir y editar hasta 1 MiB; listar 1000 entradas; buscar 8 carpetas de profundidad, 5000 entradas, 200 resultados, archivos de hasta 1 MiB y textos de hasta 200 caracteres. Los archivos binarios (un NUL en los primeros 8 KB) no se leen, editan ni buscan.
- **Confirmación.** Las llamadas `modify` y `destructive` esperan al usuario; las herramientas para las que el usuario no tiene permiso no se ofrecen al modelo.
- **Sin comandos.** Ninguna herramienta ejecuta comandos de shell ni de consola.
- **Errores seguros.** Los errores y resultados solo llevan rutas relativas al servidor, nunca rutas del host.

## Tests

```sh
go test ./tools/toolkit/...
```

Los tests de aislamiento dan a las herramientas del servidor A todos los trucos de ruta (`..`, rutas absolutas, barras invertidas, enlaces al servidor B y a una carpeta del host) y comprueban que no se lee, lista, encuentra, copia, cambia, mueve ni borra nada del servidor B ni del host, en los dos modos de resolución.
