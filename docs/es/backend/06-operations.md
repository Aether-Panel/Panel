# Sistema de Operaciones

Las operaciones son unidades atómicas de trabajo que se ejecutan como parte del ciclo de vida de un servidor (instalación, desinstalación, tareas programadas, etc.).

## Interfaces

Todas las operaciones implementan dos interfaces en `pkg/skypanel/operation.go`:

```go
type Operation interface {
    Run(args RunOperatorArgs) OperationResult
}

type OperationFactory interface {
    Create(CreateOperation) (Operation, error)
    Key() string  // identificador único de la operación
}

type RunOperatorArgs struct {
    Environment *Environment   // entorno de ejecución
    Server      DaemonServer   // servidor (para acceso a archivos)
}

type OperationResult struct {
    Error             error
    VariableOverrides map[string]interface{}  // variables a actualizar post-ejecución
}
```

## Registro de Operaciones

Las factorías se registran mediante `init()` en cada paquete de operación. El mapa global está en `internal/servers/operation_functions.go`.

## Operaciones Disponibles (26)

| Key | Paquete | Descripción |
|---|---|---|
| `alterfile` | `operations/alterfile/` | Modifica archivos (sed-like, regex) |
| `archive` | `operations/archive/` | Comprime archivos/carpetas en ZIP |
| `command` | `operations/command/` | Ejecuta un comando arbitrario |
| `console` | `operations/console/` | Envía un comando a la consola del servidor |
| `curseforge` | `operations/curseforge/` | Descarga modpacks de CurseForge |
| `dockerpull` | `operations/dockerpull/` | Descarga imágenes Docker |
| `download` | `operations/download/` | Descarga archivos vía HTTP/HTTPS |
| `extract` | `operations/extract/` | Extrae archivos comprimidos (ZIP, tar.gz) |
| `fabricdl` | `operations/fabricdl/` | Descarga Fabric Loader para Minecraft |
| `forgedl` | `operations/forgedl/` | Descarga Minecraft Forge |
| `githubdl` | `operations/githubdl/` | Descarga archivos desde GitHub (release/raw) |
| `javadl` | `operations/javadl/` | Descarga Java Runtime |
| `mkdir` | `operations/mkdir/` | Crea directorios |
| `mojangdl` | `operations/mojangdl/` | Descarga server de Minecraft de Mojang |
| `move` | `operations/move/` | Mueve o renombra archivos |
| `neoforgedl` | `operations/neoforgedl/` | Descarga NeoForge |
| `nodejsdl` | `operations/nodejsdl/` | Descarga Node.js |
| `paperdl` | `operations/paperdl/` | Descarga PaperMC |
| `resolveforgeversion` | `operations/resolveforgeversion/` | Resuelve versión de Forge |
| `resolveneoforgeversion` | `operations/resolveneoforgeversion/` | Resuelve versión de NeoForge |
| `scraperdl` | `operations/scraperdl/` | Descarga usando scraping de URLs |
| `sleep` | `operations/sleep/` | Pausa por un tiempo determinado |
| `spongedl` | `operations/spongedl/` | Descarga Sponge |
| `stdin` | `operations/stdin/` | Envía texto a stdin del proceso |
| `steamgamedl` | `operations/steamgamedl/` | Descarga juegos de Steam (DepotDownloader) |
| `writefile` | `operations/writefile/` | Escribe contenido en un archivo |

## Condiciones (CEL)

Las operaciones pueden tener condiciones que determinan si deben ejecutarse:

```go
type ConditionalMetadataType struct {
    MetadataType
    If string // expresión CEL
}
```

Las condiciones se evalúan con `google/cel-go`. Por ejemplo:

```json
{
  "if": "variables.server_jar == 'paper'",
  "command": "java -jar paper.jar"
}
```

## Flujo de Ejecución

1. Las operaciones se definen en la sección `install` o `uninstall` del Server Definition
2. También en tareas programadas del scheduler
3. `internal/servers/operation_process.go:` procesa la cola:
   - Para cada operación en la cola, obtiene la factory del mapa global
   - Crea la operación con `CreateOperation` (args + environment variables)
   - Ejecuta `Run(args)` con el entorno y servidor actual
   - Si hay `VariableOverrides`, actualiza las variables del servidor

## Downloads

Las operaciones de descarga usan `cavaliergopher/grab/v3` para descargas HTTP con reanudación, progreso y timeouts. La configuración de descargas está en `pkg/skypanel/download.go`.

## Downloaders (Tests)

`internal/operations/downloaders_test.go` contiene tests para verificar downloaders de Minecraft Paper, Forge, Fabric, NeoForge, y CurseForge.
