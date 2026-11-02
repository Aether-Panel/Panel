# Archivos del servidor y archivos comprimidos

Todo lo que el daemon hace con los archivos de un servidor (el gestor de archivos, SFTP, las descargas de instalación, los archivos comprimidos, las copias de seguridad, las restauraciones y las clonaciones) pasa por el paquete `files` (`files/` en la raíz del repositorio). Su función es mantener cada lectura y escritura dentro de la carpeta del servidor, incluso cuando el juego, o quien lo administra, deja enlaces simbólicos, tuberías con nombre o nombres engañosos.

La parte del kernel (`openat2` y la alternativa `security.forceOpenat`) se describe en [Aislamiento del kernel: openat2 y unshare](20-security-unshare.md).

## El servidor de archivos

`files.NewFileServer(root, uid, gid)` abre una vez la carpeta del servidor y devuelve un `FileServer`. A partir de ahí, cada operación se resuelve respecto a esa carpeta abierta, nunca por su ruta en disco.

| Método | Qué hace |
|--------|----------|
| `Open`, `OpenFile` | Abre un archivo dentro de la raíz. |
| `Stat` | Abre el archivo dentro de la raíz y devuelve sus datos. |
| `ReadDir` | Lista una carpeta. |
| `Glob(patrón)` | Compara el **último** elemento del patrón dentro de su carpeta (`mods/*.jar`, `*`). Las partes de carpeta no se expanden. |
| `Mkdir`, `MkdirAll` | Crea carpetas, cada una desde su carpeta padre abierta dentro de la raíz. |
| `Rename` | Renombra desde las carpetas padre abiertas dentro de la raíz (`renameat2`). |
| `Remove`, `RemoveAll` | Borra desde la carpeta padre abierta dentro de la raíz. `RemoveAll` borra los enlaces simbólicos que encuentra dentro de una carpeta como enlaces, sin entrar en ellos. |
| `Symlink(destino, ruta)` | Crea un enlace desde su carpeta padre abierta dentro de la raíz. |
| `Prefix()` | La carpeta del servidor en disco. |
| `Close()` | Libera la carpeta abierta. |

Reglas que siguen todas las operaciones:

- **Solo rutas relativas.** La ruta se normaliza y se quita la `/` inicial. Una ruta que sale de la carpeta (`../`) se rechaza con `invalid path`.
- **Dentro de la raíz.** Con `openat2`, los archivos se abren con `RESOLVE_BENEATH`: un enlace simbólico que se queda dentro de la carpeta funciona y uno que apunta fuera falla. En la alternativa con `openat`, cada parte de la ruta se abre con `O_NOFOLLOW`, así que cualquier enlace simbólico en el camino falla, aunque apunte dentro.
- **Las tuberías con nombre no bloquean.** Cada apertura añade `O_NONBLOCK`. Si no, una tubería con nombre (FIFO) que dejara el juego bloquearía la petición hasta que alguien escribiera en ella. Los archivos normales no se ven afectados.
- **Propietario.** Cuando el servidor de archivos tiene `uid`/`gid` (el usuario del servidor), los archivos, carpetas y enlaces nuevos se asignan a ese usuario. El cambio es de mejor esfuerzo: un archivo que se abrió bien se puede usar igualmente. `-1` deja el propietario como está.

### Qué muestra el gestor de archivos

- Un archivo que no es un archivo normal (una tubería, un dispositivo, un socket) no se puede abrir desde el gestor: el daemon responde `<nombre> is not a regular file`.
- Los listados muestran primero las carpetas y después los archivos, en orden alfabético sin distinguir mayúsculas, con `..` al principio fuera de la raíz.

## Escritura de descargas: `CreateIn`, `WriteFile`, `CopyFile`

Las operaciones de instalación (`download`, `mojangdl`, `paperdl`, `forgedl`, `neoforgedl`, `fabricdl`, `spongedl`, `githubdl`, `scraperdl`, `curseforge`) escriben sus archivos con estas funciones:

| Función | Qué hace |
|---------|----------|
| `CreateIn(root, dest)` | Crea o vacía `dest` dentro de `root` y crea las carpetas que falten. El archivo se abre con el servidor de archivos y `O_NOFOLLOW`. |
| `WriteFile(lector, root, dest)` | Escribe un flujo en `dest` mediante `CreateIn`. |
| `CopyFile(src, root, dest)` | Copia un archivo de la caché del panel a `dest` mediante `CreateIn`. |

`dest` puede ser relativo o absoluto; una ruta absoluta tiene que estar dentro de `root`. Cualquier cosa que salga de ella falla con `path traversal detected`. Las instalaciones se ejecutan con el usuario del daemon, así que sin esto un juego que dejara `server.jar` apuntando a un archivo del host haría que la descarga sobrescribiera ese archivo.

## Archivos comprimidos

Los gestiona [`mholt/archives`](https://github.com/mholt/archives). El formato se reconoce por el nombre del archivo y, al extraer, también por su contenido: `.zip`, `.tar`, `.tar.gz`/`.tgz`, `.tar.bz2`, `.tar.xz` y los demás formatos que admite la biblioteca. El gestor de archivos muestra **Descomprimir** para los nombres que terminan en `.zip`, `.tar`, `.tar.gz`, `.tgz`, `.gz`, `.bz2`, `.rar`, `.7z` y `.xz`, y **Comprimir** en el menú de cada archivo y carpeta (véase [Comprimir desde el gestor de archivos](#comprimir-desde-el-gestor-de-archivos)).

### Crear un archivo comprimido: `Compress`

`Compress(fs, destino, archivos)` y `CompressLimited(..., límite)` crean un archivo comprimido dentro de la carpeta del servidor a partir de una lista de rutas.

- Cada ruta debe ser relativa y estar dentro de la carpeta; si no, `path traversal detected`.
- Un `*` en la **última** parte de una ruta se expande dentro de su carpeta (`logs/*.gz`). Un comodín en una parte de carpeta (`a*/archivo`) no se expande.
- El archivo que se está escribiendo no se incluye a sí mismo.
- Se crea con `O_EXCL`: si el nombre ya existe, la respuesta es `file already exists` y no se sobrescribe nada. Se abre con el servidor de archivos, así que no se puede escribir a través de un enlace que apunte fuera.
- El formato sale de la extensión del destino.
- `CompressLimited` cuenta los bytes escritos contra el límite de disco del servidor. Si el límite corta el archivo, se borra lo escrito a medias. Ver [Límite de disco](../backend/04-servers.md#límite-de-disco).

### Copias de seguridad y puntos de restauración: `CompressServer`

`CompressServer(fs, destino)` comprime toda la carpeta del servidor en un archivo **fuera** de ella. Lo usan las copias de seguridad y los puntos de restauración; ambos son archivos `.tar.gz` en la carpeta de copias del servidor.

- Cada entrada va dentro de una carpeta con el nombre de la carpeta del servidor (su identificador).
- El archivo se crea con permisos `0600` y `O_EXCL`.
- El servidor lo escribe con un nombre oculto `.partial-<nombre>` y lo renombra al terminar, así que una copia a medias nunca se lista, se descarga ni se restaura.

### Cómo se recogen los archivos (ambas funciones)

`Compress` y `CompressServer` listan los archivos con el servidor de archivos (`archiveWalk`): cada carpeta se lee y cada archivo se abre dentro de la raíz, nunca por su ruta en disco.

| Se encuentra | Qué entra en el archivo comprimido |
|--------------|------------------------------------|
| Carpeta | La carpeta y después su contenido. |
| Archivo normal | Su contenido, leído hasta el tamaño que tenía al listarlo. Un log que crece mientras tanto no rompe el archivo. |
| Enlace simbólico | El propio enlace, con el texto de su destino. Nunca se sigue, apunte a donde apunte. |
| Tubería con nombre, dispositivo, socket | Nada: se omite (leer una tubería bloquearía el proceso). |
| Un archivo borrado durante el listado | Nada: se salta. |

Si mientras se crea el archivo una carpeta se sustituye por un enlace que sale del servidor, el recorrido se detiene con `path traversal detected` en lugar de leer archivos del host. Si un archivo listado ya no es un archivo normal cuando se lee, el proceso falla con `<nombre> is no longer a regular file`.

### Extraer: `Extract`, `ExtractLimited`, `ExtractReader`

| Función | La usa |
|---------|--------|
| `Extract(fs, origen, destino, filtro, skipRoot, tipoForzado)` | Extracción con el archivo dentro de la carpeta del servidor. |
| `ExtractLimited(fs, origen, destino, filtro, skipRoot, límite)` | La operación `extract` y el endpoint de extracción; cuenta contra el límite de disco. |
| `ExtractReader(fs, lector, origen, destino, filtro, skipRoot)` | La restauración de copias y la instalación de un server pack de CurseForge: el archivo está fuera de la carpeta del servidor, pero sus entradas se escriben con el servidor de archivos del servidor. |
| `ExtractFromReader(lector, origen, destino, filtro, skipRoot, tipoForzado)` | Extracción sin servidor de archivos (carpetas del panel). `destino` debe ser relativo. |

Para cada entrada:

- Se quitan las `/` y `\` iniciales y la entrada tiene que ser una ruta relativa dentro del destino; si no, la extracción falla con `path traversal detected`.
- `filtro` es un comodín que se compara con el nombre de archivo de la entrada: `*` (o vacío) lo acepta todo, `*.jar` solo los nombres que terminan en `.jar`, y un texto sin `*` solo ese nombre exacto.
- Las **carpetas** se crean; los **archivos normales** se crean o se sobrescriben con sus permisos más `0600`.
- Los **enlaces simbólicos** solo se recrean si su destino se queda dentro de la carpeta de destino. Un enlace con destino absoluto, o que sale fuera, **se omite sin fallar**: los archivos de los juegos suelen tener enlaces absolutos (por ejemplo `steamclient.so`), y una restauración que se detuviera ahí ya habría borrado los archivos del servidor. Los enlaces se crean desde su carpeta padre abierta dentro de la raíz, así que una entrada bajo un enlace no puede acabar fuera.
- En los archivos ZIP, un enlace cuyo destino no proporciona la biblioteca falla con `zip symlink unsupported`.
- Los demás tipos de entrada (dispositivos, tuberías, enlaces duros) se ignoran.

`skipRoot` quita la primera carpeta de cada entrada, pero solo cuando todo el archivo está dentro de una única carpeta superior (`DetermineIfSingleRoot`). Un archivo con varias entradas en el primer nivel se extrae tal cual.

### Comprobar un archivo antes de restaurar: `CheckArchive`

`CheckArchive(lector, origen, skipRoot)` lee cada entrada y devuelve un error en la primera ruta que una extracción rechazaría, sin escribir nada. Una restauración sigue este orden:

1. Averigua si la copia tiene una única carpeta superior (`DetermineIfSingleRoot`).
2. Comprueba cada entrada con `CheckArchive`. Una copia ilegible, o con una entrada que sale de la carpeta, se detiene aquí, antes de borrar nada (`Backup file cannot be read: ...` en la consola).
3. Escribe un punto de restauración de los archivos actuales. Sin él, la restauración no sigue.
4. Borra los archivos del servidor y extrae la copia con `ExtractReader`.

### Leer un enlace: `Readlink`

`Readlink(fs, nombre)` devuelve el destino del enlace simbólico `nombre`, llegando a él dentro de la raíz. Para cualquier otra cosa falla con `<nombre> is not a symlink`. Lo usa la clonación de servidores: los enlaces cuyo destino queda dentro del servidor se recrean en la copia y el resto se omite y se cuenta como omitido.

## Endpoints HTTP

Ambos son rutas del panel que requieren `server.files.edit` y se reenvían al nodo del servidor.

### `POST /api/servers/{id}/archive/{filename}`

Crea el archivo comprimido `{filename}` (una ruta dentro del servidor) con las rutas del cuerpo:

```bash
curl -X POST "https://panel.example.com/api/servers/abc123/archive/backups/world.zip" \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '["world", "plugins/*.jar", "server.properties"]'
```

| Respuesta | Cuándo |
|-----------|--------|
| `204` | Se ha creado el archivo. |
| `400` | Lista vacía, una ruta o nombre que sale de la carpeta, un archivo que no existe o un enlace hacia fuera del servidor en el camino. |
| `409` | El límite de disco del servidor está agotado. |

Un `{filename}` que ya existe se rechaza con el código `ErrFileExists` (HTTP 409) y no se sobrescribe.

#### Comprimir desde el gestor de archivos

El menú de cada archivo o carpeta tiene **Comprimir**. El diálogo pide el **Nombre del archivo comprimido** (propone `<nombre>.zip`) y crea el archivo en la carpeta actual con este endpoint, con el elemento elegido como única ruta:

- El formato sale de la extensión: `.zip`, `.tar` o `.tar.gz`. A cualquier otro nombre se le añade `.zip`.
- Si ya existe un archivo con ese nombre, no se sobrescribe y se muestra «Ya existe un archivo con ese nombre. Elige otro.».
- Al terminar, «Se ha creado <nombre>.» y la lista se actualiza.

### `POST /api/servers/{id}/extract/{filename}`

Extrae `{filename}` en la carpeta indicada en `destination` (relativa a la raíz del servidor; vacía significa la raíz):

```bash
curl -X POST "https://panel.example.com/api/servers/abc123/extract/uploads/pack.zip?destination=uploads&skipRoot" \
  -H "Authorization: Bearer <token>"
```

| Parámetro | Significado |
|-----------|-------------|
| `destination` | Carpeta de destino dentro del servidor. |
| `skipRoot` | Presente (con cualquier valor o sin él): quita la única carpeta superior del archivo, si la tiene. El gestor de archivos siempre lo envía y extrae junto al archivo. |

| Respuesta | Cuándo |
|-----------|--------|
| `204` | Extraído. |
| `400` | `filename` o `destination` salen de la carpeta. |
| `409` | El límite de disco del servidor está agotado, antes o durante la extracción. |

## Operaciones

Las operaciones `archive` y `extract` llaman al mismo código, con el límite de disco aplicado. Sus argumentos se describen en [Motor de operaciones](../backend/06-operations.md).

```json
{ "type": "archive", "source": ["world"], "destination": "world-${version}.zip" }
```

```json
{ "type": "extract", "source": "pack.zip", "destination": "" }
```

La operación `extract` no quita la carpeta superior (`skipRoot` desactivado).

## Sistemas de archivos combinados: `MergedFS`

`files.NewMergedFS(fs1, fs2, ...)` combina varios sistemas de archivos de solo lectura. `Open` y `ReadFile` devuelven el primero que tenga el archivo; `ReadDir` une los listados y, ante nombres repetidos, gana el primer sistema. El panel lo usa para:

- La interfaz web: la carpeta de `panel.web.files` (por defecto `www`) por encima de los archivos incluidos en el binario.
- Las plantillas de correo: la carpeta de `panel.email.templateFolder`, si está definida, por encima de las plantillas incluidas (ver [Correo electrónico](15-email.md)).

## Errores

| Mensaje | Significado |
|---------|-------------|
| `path traversal detected` | Una ruta, entrada del archivo o enlace sale de la carpeta del servidor (`files.ErrPathTraversal`). |
| `file already exists` | El archivo comprimido que se quiere crear ya existe (`files.ErrFileExists`; por HTTP llega como `ErrFileExists`, «file exists where there should be none»). |
| `invalid path: ...` | Se ha pasado al servidor de archivos una ruta que sale de la carpeta. |
| `no files to compress` | Se ha llamado a `Compress` con una lista vacía. |
| `format is not an extractor` / `format is not an archiver` | El formato no se puede extraer o crear. |
| `<nombre> is not a regular file` | Se ha pedido al gestor de archivos abrir una tubería, un dispositivo o un socket. |
