# Motor de operaciones

Las plantillas y las tareas programadas describen su trabajo como listas de **operaciones**: pasos JSON pequeños como «descarga este archivo», «escribe este texto», «ejecuta este comando» o «instala Forge». El daemon convierte cada lista en un proceso y ejecuta los pasos uno tras otro.

Código: un paquete por operación en `internal/operations/*`, el ejecutor en `internal/servers/operation_process.go`, las operaciones que actúan sobre el propio servidor en `internal/servers/task_operations.go` y las funciones de condición en `internal/servers/operation_functions.go`.

---

## Dónde se usan las listas de operaciones

| Lista | Dónde se declara | Cuándo se ejecuta |
|-------|------------------|-------------------|
| `install` | definición del servidor (plantilla) | Al pulsar **Instalar** y en la instalación automática al crear un servidor (véase `panel.servers.autoInstall` en [Configuración](./02-configuration.md)). Antes se detiene el servidor. |
| `uninstall` | definición del servidor | Al borrar el servidor, antes de eliminar su entorno. Si un paso falla, se registra en el log y se muestra en la consola, y el servidor se borra igualmente. |
| `run.pre` | definición del servidor, dentro de `run` | Cada vez que el servidor arranca, antes del comando de inicio. |
| `run.post` | definición del servidor, dentro de `run` | Cada vez que el proceso del servidor termina. Sus marcadores y condiciones reciben `success` y `exitCode`. |
| `operations` | cada tarea programada | Cuando la tarea se dispara (cron) o se ejecuta a mano. |

El panel también construye listas internamente: las instalaciones de modpacks y de versiones (`curseforge`, `javadl`, los pasos `writefile` de la plantilla) reutilizan el mismo ejecutor.

---

## Estructura de un paso

Un paso es un objeto JSON con un `type`, un `if` opcional y los argumentos de ese tipo, todos al mismo nivel:

```json
{
  "type": "download",
  "if": "modlauncher == 'vanilla'",
  "files": ["https://example.com/${version}/server.jar"]
}
```

| Campo | Descripción |
|-------|-------------|
| `type` | Nombre de la operación (véase [la lista](#referencia-de-operaciones)). Un tipo desconocido detiene la lista con un error. |
| `if` | [Condición CEL](#condiciones) opcional. Si es falsa, el paso se omite. |
| resto de claves | Argumentos de la operación. Las claves desconocidas se ignoran. |

No existen `description`, `ignoreError` ni un tiempo máximo por paso: **el primer paso que falla detiene la lista** y el error aparece en la consola del servidor (`Failed to install server`, `Error running pre execute`, `Failed to run post-execution steps`, `Failed to setup tasks`).

---

## Sustitución de variables

Al ejecutarse cada paso, justo antes de empezar, en cada argumento de **texto**, y en cada texto de una lista de textos, se sustituyen los marcadores `${nombre}`:

```json
{ "type": "writefile", "target": "eula.txt", "text": "eula=${eula}" }
```

| Marcador | Valor |
|----------|-------|
| `${<variable>}` | Cada variable del servidor (el bloque `data` de la definición), por su nombre |
| `${rootDir}` | Ruta absoluta de la carpeta del servidor en el nodo |
| `${core:os}`, `${core:arch}` | `runtime.GOOS` / `runtime.GOARCH` del daemon (por ejemplo `linux`, `amd64`) |
| `${success}`, `${exitCode}` | Solo en `run.post`: si el proceso terminó como se esperaba y su código de salida |

Notas:

- La sintaxis es `${nombre}`, no `{{nombre}}`. Un marcador cuyo nombre no existe se deja tal cual (así conserva `scraperdl` su propio `${match}`).
- Los números, los booleanos y los objetos anidados se pasan sin cambios; solo se sustituyen textos y listas de textos.
- Las variables de `run.environmentVars` pasan por la misma sustitución y la operación `command` las recibe como variables de entorno.
- La sustitución se hace **paso a paso**, con los valores conocidos en ese momento. El valor que un paso escribe en una variable (véase *variables de salida* más abajo) lo ven los marcadores `${...}` y las condiciones `if` de los pasos siguientes de la misma lista, y las ejecuciones posteriores.

---

## Condiciones

`if` es una expresión [CEL](https://github.com/google/cel-spec) que debe devolver un booleano. La sintaxis, los operadores y más ejemplos están en [Motor CEL](../general/18-conditions-cel.md); aquí solo se describe lo que ven las operaciones.

| Nombre | Valor |
|--------|-------|
| cada variable del servidor | su valor actual, por su nombre (`modlauncher == 'paper'`, no `variables.modlauncher`) |
| `os`, `arch` | `runtime.GOOS` / `runtime.GOARCH` del daemon |
| `env` | el **tipo de entorno** del servidor (`docker`, `host`…), no las variables de entorno |
| `serverId` | el identificador del servidor |
| `success` | `true` mientras corre una lista (un paso fallido detiene la lista, así que no existe un estado «falló el paso anterior»). En `run.post` indica si el proceso terminó con el código de salida esperado, así que `"if": "!success"` ejecuta un paso solo tras una caída |
| `exitCode` | Solo en `run.post`: el código de salida del proceso |

| Función | Descripción |
|---------|-------------|
| `file_exists(ruta)` | `true` si `ruta`, relativa a la carpeta del servidor, existe. Las rutas que salen de la carpeta devuelven `false`. |
| `in_path(cmd)` | `true` si `cmd` está en el `PATH` del daemon |
| `is_server_running()` | `true` mientras el proceso del servidor está en marcha (sin argumentos) |

```json
{ "type": "mojangdl", "if": "!file_exists('server.jar')", "version": "${version}", "target": "server.jar" }
```

Un `if` vacío o ausente siempre se ejecuta. Una condición que no compila, o que no devuelve un booleano, detiene la lista con un error.

También se puede escribir `${nombre}` dentro de una condición, como en el resto del paso: `"if": "in_path('java${javaversion}')"`. Antes de compilar la condición se sustituye cada `${nombre}` cuyo nombre sea uno de los valores anteriores (variables del servidor, `env`, `serverId`, `success`, `exitCode`, `os`, `arch`). Dentro de un literal de texto, el valor se inserta escapado para ese literal, así que nunca puede convertirse en código CEL; fuera de un literal, el marcador equivale a la propia variable. Los demás nombres (como `rootDir`) y los literales de bytes (`b"..."`) se dejan como están. Código: `conditions/variables.go`.

---

## Seguridad de los archivos

Toda operación que escribe en la carpeta del servidor pasa por el **servidor de archivos** confinado del servidor (`files.FileServer`, `openat2` con `RESOLVE_BENEATH`; véase [Seguridad](./09-security.md#acceso-a-archivos)):

- Las rutas son relativas a la carpeta del servidor; se aceptan rutas absolutas que estén dentro de ella y se rechaza cualquier ruta que salga (`../`, rutas absolutas a otro sitio).
- Las descargas se guardan primero en una carpeta temporal dentro de la caché del daemon (`daemon.data.cache`) y después se copian con `O_NOFOLLOW`: si el juego dejó un **enlace simbólico** donde va el archivo (por ejemplo `server.jar -> /etc/...`), la escritura se rechaza en lugar de sobrescribir el destino del enlace. Las carpetas intermedias que falten se crean.
- `alterfile`, `writefile`, `mkdir`, `move`, `extract`, `archive`, `resolveforgeversion` y la extracción de `curseforge` usan el mismo servidor de archivos.

---

## Reglas de red de las descargas

`download` y `scraperdl` toman sus URL de la plantilla y, a menudo, de una variable que el propietario del servidor puede editar; `githubdl` descarga una dirección que viene de la release (y que GitHub redirige a su almacenamiento). Su cliente HTTP comprueba cada dirección a la que se conecta (en el momento de conectar, así que un nombre DNS que cambie de dirección no puede saltarse la comprobación):

| Dirección | Comportamiento |
|-----------|----------------|
| Loopback, enlace local (incluye los metadatos de la nube `169.254.169.254`), sin especificar, multicast | Siempre se rechaza (`refusing to download from internal address ...`). |
| Redes privadas (RFC 1918, redes de Docker, la LAN del nodo) | Se rechazan mientras `security.blockPrivateDownloads` sea `true` (**el valor por defecto**). Con `false` se permiten y se escribe un aviso en la consola. |
| Direcciones públicas | Se permiten. |

Ponga `security.blockPrivateDownloads` a `false` (variable de entorno `SKYPANEL_SECURITY_BLOCKPRIVATEDOWNLOADS=false`) solo si las plantillas descargan de réplicas en su propia red. Los demás descargadores, y la llamada a la API de GitHub de `githubdl`, usan servidores fijos y conocidos (Mojang, PaperMC, Maven, Adoptium, nodejs.org, GitHub, Sponge, CurseForge) con el cliente HTTP normal del panel.

---

## Referencia de operaciones

Hay 30 tipos registrados: 26 de `internal/operations` más `start`, `stop`, `restart` y `backup` de `internal/servers`. Los nombres de los argumentos distinguen mayúsculas y minúsculas, salvo en `spongedl`.

### Archivos

#### `writefile` — escribir un archivo de texto

| Argumento | Descripción |
|-----------|-------------|
| `target` | Archivo que se crea o sobrescribe |
| `text` | Contenido |

```json
{ "type": "writefile", "target": "eula.txt", "text": "eula=true" }
```

#### `alterfile` — buscar y reemplazar en un archivo

| Argumento | Descripción |
|-----------|-------------|
| `file` | Archivo existente que se edita |
| `search` | Texto (o expresión regular) que se busca |
| `replace` | Reemplazo, insertado literalmente (los grupos como `$1` no se expanden) |
| `regex` | `true` para tratar `search` como expresión regular de Go en modo multilínea (`(?m)`: `^` y `$` coinciden con cada línea) |

```json
{ "type": "alterfile", "file": "server.properties", "regex": true,
  "search": "^server-port=.*$", "replace": "server-port=${port}" }
```

#### `mkdir` — crear una carpeta

| Argumento | Descripción |
|-----------|-------------|
| `target` | Carpeta que se crea, con las intermedias que falten (modo `0755`) |

#### `move` — mover o renombrar

| Argumento | Descripción |
|-----------|-------------|
| `source` | Archivo o patrón glob (`*.jar`) |
| `target` | Archivo de destino, o una carpeta existente a la que mover |

Varias coincidencias necesitan una carpeta existente como `target`. Cuando la combinación no es posible (varios archivos sobre un solo archivo, una carpeta que no existe) el paso registra el problema y no hace nada, sin fallar.

#### `extract` — descomprimir un archivo

| Argumento | Descripción |
|-----------|-------------|
| `source` | Archivo comprimido dentro de la carpeta del servidor |
| `destination` | Carpeta donde se extrae |

Cuenta para el límite de disco del servidor. Formatos admitidos: [Archivos y compresión](../general/19-files-compression.md).

#### `archive` — crear un archivo comprimido

| Argumento | Descripción |
|-----------|-------------|
| `source` | Lista de archivos y carpetas |
| `destination` | Archivo que se crea; se rechaza si ya existe |

Cuenta para el límite de disco del servidor.

### Procesos y consola

#### `command` — ejecutar comandos

| Argumento | Descripción |
|-----------|-------------|
| `commands` | Lista de líneas de comando, ejecutadas una tras otra |
| `stdin` | Configuración de entrada opcional (misma forma que `run.stdin`) |

Cada comando se ejecuta a través del entorno del servidor (con Docker, en un contenedor con la imagen del servidor) y con `run.environmentVars`. Un código de salida distinto de cero hace fallar el paso. El servidor debe estar **detenido**: no se puede lanzar un comando mientras el proceso del servidor está en marcha.

```json
{ "type": "command", "commands": ["chmod +x ./start.sh"] }
```

#### `stdin` — enviar una línea al servidor en marcha

| Argumento | Descripción |
|-----------|-------------|
| `command` | Texto que se escribe en la entrada de la consola del servidor |

Falla con «server offline» si el servidor no está en marcha. Habitual en tareas: `{ "type": "stdin", "command": "save-all" }`.

#### `console` — mostrar un mensaje

| Argumento | Descripción |
|-----------|-------------|
| `message` | Se muestra en la consola del servidor como `[DAEMON] Message: ...` |

#### `sleep` — esperar

| Argumento | Descripción |
|-----------|-------------|
| `duration` | Duración con formato de Go (`5s`, `1m30s`) |

#### `dockerpull` — descargar una imagen de Docker

| Argumento | Descripción |
|-----------|-------------|
| `imageName` | Imagen que se descarga |

Solo funciona en servidores Docker; en otros entornos falla con «environment not supported».

### Control del servidor (tareas)

Actúan sobre el propio servidor y están pensadas para tareas programadas. No llevan argumentos, salvo `backup`.

| Tipo | Comportamiento |
|------|----------------|
| `start` | Arranca el servidor; no hace nada si ya está en marcha. |
| `stop` | Detiene el servidor y espera hasta 5 minutos a que termine. |
| `restart` | Solo si el servidor está en marcha: lo detiene, espera hasta 5 minutos y lo vuelve a arrancar. Un servidor detenido se queda detenido. |
| `backup` | Hace una copia de seguridad automática (nombre que empieza por `auto-`). `keep` (1–50, por defecto 5) es cuántas copias automáticas se conservan; las `auto-` más antiguas se borran, las manuales nunca. |

### Descargas genéricas

#### `download` — descargar archivos por URL

| Argumento | Descripción |
|-----------|-------------|
| `files` | Lista de URL |

Cada archivo se guarda en la carpeta del servidor con el nombre que indica el servidor remoto (`Content-Disposition`) o la última parte de la URL. No se puede elegir la carpeta ni el nombre; si hace falta, añada después un `move`. Sujeto a las [reglas de red](#reglas-de-red-de-las-descargas).

```json
{ "type": "download", "files": ["https://example.com/builds/${version}/server.jar"] }
```

#### `scraperdl` — buscar una versión en una página web y descargarla

| Argumento | Descripción |
|-----------|-------------|
| `url` | Página que se lee (debe responder `200`) |
| `regex` | Expresión regular de Go; su **primer grupo de captura** es la coincidencia |
| `downloadUrl` | URL que se descarga, donde `${match}` se sustituye por la coincidencia |
| `outputVariable` | Variable del servidor opcional que recibe la coincidencia |

Tanto la página como el archivo están sujetos a las [reglas de red](#reglas-de-red-de-las-descargas). El archivo se guarda igual que en `download`.

#### `githubdl` — recurso de la última versión de GitHub

| Argumento | Descripción |
|-----------|-------------|
| `repository` | `propietario/nombre` |
| `assetMatch` | Texto que debe contener el nombre del recurso; se usa el primero que lo contenga |
| `outputVariable` | Variable del servidor opcional que recibe el **nombre del archivo** guardado (no la URL), para que los pasos siguientes puedan hacer `extract` o `move` |

Usa la API de GitHub sin autenticar (`/repos/<repo>/releases/latest`), limitada por IP. El recurso se guarda en la carpeta del servidor con el nombre de archivo de la URL. Su descarga sigue las [reglas de red](#reglas-de-red-de-las-descargas).

### Minecraft

#### `mojangdl` — jar del servidor vanilla

| Argumento | Descripción |
|-----------|-------------|
| `version` | `release` o `latest` (última versión estable), `snapshot` (última snapshot) o un identificador de versión exacto |
| `target` | Nombre con el que se guarda (por ejemplo `server.jar`) |

Lee el manifiesto de versiones de Mojang y descarga el jar de servidor de esa versión.

#### `paperdl` — proyectos de PaperMC

| Argumento | Descripción |
|-----------|-------------|
| `minecraftVersion` | Obligatorio. `latest` elige la versión más reciente del proyecto |
| `build` | Obligatorio. Build que se descarga |
| `target` | Nombre con el que se guarda |
| `project` | Proyecto de PaperMC (por defecto `paper`) |

Usa la API Fill v3 de PaperMC (`fill.papermc.io`). Los archivos se guardan en la caché del daemon y se reutiliza la copia en caché cuando su SHA-256 coincide con el que publica la API.

#### `fabricdl` — instalador de Fabric

Sin argumentos. Descarga el instalador de Fabric más reciente que lista `meta.fabricmc.net` y lo guarda como `fabric-installer.jar`; la plantilla lo ejecuta después con `command`.

#### `forgedl` — instalador de Forge

| Argumento | Descripción |
|-----------|-------------|
| `minecraftVersion` | Sirve para buscar la versión recomendada cuando `version` está vacío |
| `version` | Versión completa de Forge (`1.20.1-47.2.0`). Vacío: la promoción `<minecraftVersion>-latest` |
| `target` | Nombre con el que se guarda el instalador |
| `outputVariable` | Variable del servidor que recibe la versión (por defecto `opForgeVersion`) |

Hace falta `version` o `minecraftVersion`. Descarga de `maven.minecraftforge.net`; los archivos `.sha1` de Maven sirven para reutilizar copias en caché.

#### `neoforgedl` — instalador de NeoForge

| Argumento | Descripción |
|-----------|-------------|
| `minecraftVersion` | Sirve para elegir la build de NeoForge más reciente cuando `version` está vacío (`1.21.1` corresponde a NeoForge `21.1.*`) |
| `version` | Versión exacta de NeoForge |
| `target` | Nombre con el que se guarda el instalador |
| `outputVariable` | Por defecto `opNeoForgeVersion` |

Hace falta `version` o `minecraftVersion`. Consulta `maven.neoforged.net`.

#### `resolveforgeversion` / `resolveneoforgeversion` — detectar la versión instalada

| Argumento | Descripción |
|-----------|-------------|
| `version` | Si se indica, simplemente se copia en la variable de salida |
| `minecraftVersion` | Véase abajo |
| `outputVariable` | Por defecto `opForgeVersion` / `opNeoForgeVersion` |

Sin `version`, buscan en `libraries/net/minecraftforge/forge` (o `libraries/net/neoforged/neoforge`), a través del servidor de archivos del servidor, carpetas de versión que contengan `unix_args.txt` y guardan la elegida:

- `resolveneoforgeversion` elige la carpeta más reciente para `minecraftVersion`. Las versiones de NeoForge quitan el `1.` inicial de la versión de Minecraft (`1.21.4` → `21.4.*`, `1.21` → `21.0.*`). Si ninguna carpeta coincide, o `minecraftVersion` está vacío, usa la primera carpeta compatible que encuentra.
- `resolveforgeversion` se queda con la primera carpeta compatible que encuentra (las carpetas se leen por orden de nombre); en la práctica `minecraftVersion` no cambia la elección.

Si la carpeta no existe, la variable queda vacía; no es un error.

#### `spongedl` — SpongeVanilla / SpongeForge

| Argumento | Descripción |
|-----------|-------------|
| `spongeType` | `spongevanilla` o `spongeforge` |
| `spongeVersion` | Versión exacta de Sponge; vacío significa la más reciente que cumpla los filtros siguientes |
| `minecraftVersion` | Filtra por versión de Minecraft |
| `recommended` | `true` para considerar solo builds recomendadas |

Usa la API de descargas v2 de Sponge. Para ambos tipos, el jar es el recurso `universal` de la build, o el jar sin clasificador si la build no lo tiene (builds antiguas). `spongevanilla` guarda `server.jar`. `spongeforge` ejecuta antes `forgedl` para la versión de Minecraft que declara Sponge (guardado como `forge-installer.jar`), crea `mods/` y guarda SpongeForge como `mods/_aspongeforge.jar`.

#### `curseforge` — server pack de un modpack de CurseForge

| Argumento | Descripción |
|-----------|-------------|
| `projectId` | Obligatorio. Id del proyecto en CurseForge |
| `fileId` | Id de archivo opcional (también se acepta `fileID`). Vacío: el archivo estable más reciente del proyecto |
| `java` | Binario de Java con el que se ejecutan los instaladores (por defecto `java`) |

Necesita una clave de la API de CurseForge en `daemon.curseforge.key` (las compilaciones oficiales pueden incluirla al compilar); sin ella el paso falla antes de empezar. Lo que hace:

1. Resuelve el **server pack** (si el archivo elegido es un pack de cliente que apunta a un server pack, se usa ese). Un archivo que no es server pack falla con `not server pack`.
2. Lo descarga a la caché del daemon y lo extrae en la carpeta del servidor a través del servidor de archivos (se quita una única carpeta raíz).
3. Detecta el cargador: un jar instalador incluido en el pack (`forge-*-installer.jar` para Forge, `neoforge-*-installer.jar` para NeoForge), si no el `variables.txt` del pack (`MODLOADER`, `MINECRAFT_VERSION`, `MODLOADER_VERSION`, `FABRIC_INSTALLER_VERSION`), y si no el cargador principal del manifiesto del cliente.
4. **Forge / NeoForge**: descarga el instalador si hace falta, ejecuta `<java> -jar <instalador> --installServer`, borra el instalador y su log, renombra el jar anterior a 1.17 o el shim de Forge a `server.jar` y, si sigue sin haber `server.jar`, descarga el ServerStarterJar de NeoForge (0.1.25) como `server.jar`.
5. **Fabric**: descarga el lanzador de servidor de Fabric de `meta.fabricmc.net` como `server.jar`. Solo si responde `404` recurre al instalador de Fabric de `maven.fabricmc.net`, que guarda como `fabric-installer.jar` y ejecuta con `server -mcversion <v> -loader <v> -downloadMinecraft`; cualquier otro error hace fallar el paso. Una descarga que no responde `200` nunca se guarda como jar.
6. Con un cargador desconocido se dejan los archivos extraídos y termina sin error.

Ejecutar instaladores requiere que el servidor esté detenido (véase `command`).

### Entornos de ejecución y Steam

#### `javadl` — Java (Eclipse Temurin)

| Argumento | Descripción |
|-----------|-------------|
| `version` | Versión principal como número entero (`17`, `21`) |

Si `java<versión>` no está ya en el `PATH`, descarga el JDK Temurin correspondiente de la API de Adoptium en la carpeta de binarios (`daemon.data.binaries`) y crea allí los enlaces `java<versión>` y `javac<versión>`. El daemon añade esa carpeta a su `PATH` al arrancar. Se usa sobre todo con servidores en el entorno host; los servidores Docker traen Java en su imagen.

#### `nodejsdl` — Node.js

| Argumento | Descripción |
|-----------|-------------|
| `version` | Versión principal como número entero (`20`) |

Elige la versión `v<versión>.*` más reciente de `nodejs.org/dist/index.json`, la extrae en la carpeta de binarios y enlaza `node<versión>` y `npm<versión>`.

#### `steamgamedl` — juego de Steam con DepotDownloader

| Argumento | Descripción |
|-----------|-------------|
| `appId` | Obligatorio. Id de la aplicación en Steam (también se acepta `appID`) |
| `username`, `password` | Inicio de sesión de Steam opcional, para juegos que necesitan cuenta |
| `extraArgs` | Argumentos adicionales de DepotDownloader (lista) |

Pasos: descarga DepotDownloader en `<binarios>/depotdownloader` si no está (versión `daemon.depotDownloader.version`, por defecto `latest`), obtiene los metadatos del cliente de Steam en `.steam/`, ejecuta DepotDownloader una vez con `-manifest-only` en `.manifest/` y otra para la descarga real en la carpeta del servidor con `-validate`, y marca como ejecutables los archivos que indican los manifiestos. Se añade `-use-lancache` salvo que `daemon.depotDownloader.disableLancache` sea `true`. Se ejecuta a través del entorno del servidor, así que el servidor debe estar detenido.

---

## Variables de salida

Algunas operaciones escriben un valor en una variable del servidor al terminar:

| Operación | Variable | Valor |
|-----------|----------|-------|
| `forgedl`, `resolveforgeversion` | `outputVariable` (por defecto `opForgeVersion`) | Versión de Forge |
| `neoforgedl`, `resolveneoforgeversion` | `outputVariable` (por defecto `opNeoForgeVersion`) | Versión de NeoForge |
| `githubdl` | `outputVariable` (si se indica) | Nombre del archivo guardado |
| `scraperdl` | `outputVariable` (si se indica) | Coincidencia de la expresión regular |

La variable se crea si la definición no la tiene. Las plantillas suelen usarla en el comando de inicio (`${opForgeVersion}`) o en condiciones posteriores.

---

## Ejemplo: lista de instalación

```json
"install": [
  { "type": "mojangdl", "if": "modlauncher == 'vanilla'", "version": "${version}", "target": "server.jar" },
  { "type": "paperdl", "if": "modlauncher == 'paper'", "minecraftVersion": "${version}", "build": "${paperbuild}", "target": "server.jar" },
  { "type": "writefile", "target": "eula.txt", "text": "eula=${eula}" }
]
```

## Ejemplo: tarea programada

```json
{
  "name": "Reinicio nocturno",
  "cronSchedule": "0 4 * * *",
  "operations": [
    { "type": "stdin", "command": "say Reinicio en 1 minuto" },
    { "type": "sleep", "duration": "1m" },
    { "type": "backup", "keep": 7 },
    { "type": "restart" }
  ]
}
```

---

## Añadir una operación

1. Cree un paquete en `internal/operations/<nombre>` con un tipo que implemente `skypanel.Operation` (`Run(skypanel.RunOperatorArgs) skypanel.OperationResult`) y una factoría que implemente `skypanel.OperationFactory` (`Create(skypanel.CreateOperation)` y `Key()`).
2. Lea los argumentos de `op.OperationArgs` (use `op.Arg("a", "b")` para aceptar grafías alternativas). Escriba archivos con `args.Server.GetFileServer()` o `files.WriteFile`, nunca con llamadas `os` directas sobre la ruta del servidor.
3. Añada la factoría a la lista `factories` de `internal/servers/operation_process.go`.
4. Devuelva `VariableOverrides` en el resultado para fijar variables del servidor.

```go
func (of OperationFactory) Key() string { return "mycustom" }

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
    return MyCustom{Target: cast.ToString(op.OperationArgs["target"])}, nil
}

func (m MyCustom) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
    err := args.Server.GetFileServer().MkdirAll(m.Target, 0755)
    return skypanel.OperationResult{Error: err}
}
```
