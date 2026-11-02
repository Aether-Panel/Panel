# Condiciones (CEL)

Las plantillas y las tareas pueden hacer que un paso dependa de una condición, escrita en el campo `if`. Aether Panel evalúa estas condiciones con [Common Expression Language](https://github.com/google/cel-spec) (CEL), mediante la biblioteca `google/cel-go`. CEL es un lenguaje de expresiones pequeño: no tiene bucles ni asignaciones, y no accede a archivos ni a la red salvo con las funciones que añade el panel.

Los tipos de operación (`download`, `javadl`, `writefile`...) y la sustitución `${variable}` en sus argumentos se describen en [Motor de operaciones](../backend/06-operations.md).

## Dónde se evalúan las condiciones

| Lugar | Campo | Qué ocurre |
|-------|-------|------------|
| Pasos de instalación | `install[].if` | El paso solo se ejecuta si la condición es verdadera. |
| Pasos de desinstalación | `uninstall[].if` | Igual. |
| Pasos previos y posteriores al arranque | `run.pre[].if`, `run.post[].if` | Igual. |
| Comando de arranque | `run.command[].if` | Cuando `run.command` es una lista, se usa la primera entrada cuya condición sea verdadera. Si no coincide ninguna, se usa la entrada sin `if`. |
| Tareas programadas | `operations[].if` | Igual que en la instalación. Solo los administradores pueden guardar un paso de tarea con condición; para el resto, la API responde `only administrators can use conditions`. |
| Cambios de versión | `install[].if` | Al cambiar la versión de un servidor, el panel usa las condiciones para saber qué paso de descarga corresponde a ese servidor (por ejemplo, qué lanzador de la plantilla de Minecraft). |
| Detección del mapa del mundo | `install[].if` | Los pasos de instalación cuya condición se cumple indican al panel qué plataforma ejecuta el servidor. |

Una condición debe devolver un booleano. Cualquier otro resultado es un error (`invalid return type, expected bool, got ...`).

Qué pasa cuando hay un error:

- En los pasos de instalación, previos/posteriores y de tareas, un error de evaluación detiene todo el proceso y se muestra en la consola del servidor.
- En los pasos de desinstalación, un error detiene los pasos restantes y se registra y muestra en la consola, pero el servidor se borra igualmente (su entorno se elimina).
- En el comando de arranque, un error detiene el arranque (`Failed to start server`).
- En los cambios de versión y en la detección del mapa, un error cuenta como «falso» y el paso se ignora.

## Datos disponibles

| Nombre | Tipo | Valor |
|--------|------|-------|
| `os` | string | Sistema operativo del proceso del daemon (`runtime.GOOS` de Go): `linux`, `windows`... |
| `arch` | string | Arquitectura del proceso del daemon (`runtime.GOARCH`): `amd64`, `arm64`... |
| `env` | string | Tipo de entorno del servidor, por ejemplo `"docker"` o `"host"`. Es un texto, no un mapa de variables de entorno. |
| `serverId` | string | Identificador del servidor. |
| `success` | bool | En las listas de pasos, `true`: un paso que falla detiene la lista, así que los siguientes nunca ven `false`. En `run.post`, indica si el servidor se detuvo de forma ordenada (`false` tras una caída). |
| `exitCode` | int | Solo en `run.post`: el código de salida del proceso del servidor. |
| Cada variable del servidor | cualquiera | Valor actual de la variable, por su nombre (`javaversion`, `modlauncher`, `memory`...). |

Notas:

- Las variables del servidor se usan por su nombre (`javaversion != ""`). También se puede escribir `${nombre}` dentro de un `if`, como en el resto del paso: antes de evaluar, el panel lo sustituye por el valor de forma segura. Dentro de una cadena, el valor se escapa (`in_path("java${javaversion}")` pasa a `in_path("java17")`); fuera de una cadena, `${nombre}` se convierte en la propia variable. El valor nunca se interpreta como código CEL. Un `${...}` con un nombre desconocido se deja tal cual.
- Los `${...}` de un paso (en el `if` y en los argumentos) se resuelven justo cuando se ejecuta ese paso, así que ven las variables que haya cambiado un paso anterior (por ejemplo, el `outputVariable` de `githubdl` o la versión que encuentra un paso de resolución).
- Todas las variables se declaran con tipo dinámico, así que las comparaciones se comprueban al evaluar. Una variable numérica se compara con un número y una de texto con una cadena.
- Un `if` vacío significa que el paso se ejecuta siempre, también en `run.post` tras una caída.
- `os` y `arch` describen dónde se ejecuta el daemon, no el interior de un contenedor Docker.

## Funciones que añade el panel

Se registran en `internal/servers/operation_functions.go` (`CreateFunctions`).

### `file_exists(ruta)`

```cel
file_exists("server.properties")
```

Devuelve `true` si la ruta existe dentro de la carpeta del servidor. La ruta es relativa a la raíz del servidor; una ruta que sale de ella (`../`) siempre devuelve `false`. Usa `os.Stat`, así que un enlace simbólico cuenta si su destino existe.

### `in_path(comando)`

```cel
in_path("java")
```

Devuelve `true` si el comando está en el `PATH` (`exec.LookPath`), incluida una coincidencia en el directorio actual. La búsqueda se hace en el `PATH` del propio daemon, no dentro del contenedor del servidor.

### `is_server_running()`

```cel
is_server_running()
```

No recibe argumentos. Devuelve `true` si el proceso de **este** servidor está en marcha. Cualquier error al consultarlo cuenta como `false`.

## CEL estándar

Funciona toda la sintaxis estándar de CEL. Algunos ejemplos:

```cel
javaversion != ""
modlauncher == "fabric"
modlauncher in ["forge", "neoforge"]
os != "windows" && arch == "amd64"
!file_exists("server.properties")
```

Las cadenas admiten comillas dobles o simples (`loader == 'vanilla'` funciona).

## Ejemplos

Instalar Java solo si la plantilla fija una versión, y escribir `server.properties` solo la primera vez:

```json
"install": [
  { "if": "javaversion != \"\"", "type": "javadl", "version": "${javaversion}" },
  { "type": "mojangdl", "version": "${version}", "target": "server.jar" },
  {
    "if": "!file_exists(\"server.properties\")",
    "type": "writefile",
    "target": "server.properties",
    "text": "server-port=${port}\n"
  }
]
```

Un paso de descarga por lanzador; solo se ejecuta el que corresponde al servidor:

```json
"install": [
  { "if": "modlauncher == \"\"", "type": "mojangdl", "version": "${version}", "target": "server.jar" },
  { "if": "modlauncher == \"fabric\"", "type": "fabricdl", "version": "${version}" }
]
```

Un comando de arranque con alternativa:

```json
"run": {
  "command": [
    { "if": "in_path(\"java21\")", "command": "java21 -Xmx${memory}M -jar server.jar" },
    { "command": "java -Xmx${memory}M -jar server.jar" }
  ]
}
```

Un paso posterior que solo se ejecuta tras una caída (`success` y `exitCode` solo existen en las condiciones de `run.post`, no en los argumentos `${...}`):

```json
"run": {
  "post": [
    { "if": "!success && exitCode != 0", "type": "writefile", "target": "last-crash.txt", "text": "The server crashed\n" }
  ]
}
```

## Errores habituales

| Mensaje (comienzo) | Causa |
|--------------------|-------|
| `undeclared reference to 'x'` | La condición usa un nombre que no es una variable del servidor ni uno de los nombres anteriores. |
| `invalid return type, expected bool` | La expresión no devuelve verdadero o falso. |

Los errores de sintaxis y los operadores usados con tipos incorrectos (por ejemplo, comparar un número con una cadena) también fallan; el mensaje lo genera `cel-go` e indica la posición en la expresión.

## Para desarrolladores

El paquete `conditions` (`conditions/executor.go`) contiene el motor:

| Función | Qué hace |
|---------|----------|
| `ResolveIf(condition, data, extras)` | Sustituye los `${nombre}` de la condición (`SubstituteVariables`, en `conditions/variables.go`), la evalúa y devuelve un booleano. Si la condición está vacía y `data` contiene `success`, evalúa `success` (las listas de pasos no llaman a `ResolveIf` para un paso sin `if`: lo ejecutan siempre). |
| `Run[T](statement, data, extras)` | Evalúa una expresión y devuelve un valor de tipo `T`. Cada clave de `data` pasa a ser una variable CEL de tipo dinámico, además de las constantes `os` y `arch`. |

`Server.RunCondition` (`internal/servers/server.go`) construye los datos (`env`, `serverId`, las variables del servidor y los valores extra) y añade las funciones de `CreateFunctions`. Una función nueva se añade allí, como otro `cel.Function(...)` con su sobrecarga y su enlace.
