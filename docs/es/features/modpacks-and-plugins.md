# Modpacks y plugins

Aether Panel puede instalar un modpack completo de Minecraft: Java Edition desde **Modrinth** o **CurseForge** en un solo paso, y dispone de un pequeño explorador de plugins que busca en **Modrinth** y coloca archivos sueltos de plugins o mods en el servidor. Esta página cubre ambas funciones, ya que comparten orígenes y la misma red de seguridad (los puntos de restauración).

---

## Qué hace

**Modpacks** (pestaña **Modpacks**, solo en servidores de Minecraft Java):

- Busca modpacks en Modrinth o CurseForge. Una búsqueda vacía muestra los más populares (20 por página, con **Cargar más**).
- Lista las versiones de un modpack que el panel puede instalar: modpacks de Fabric, Forge o NeoForge. En CurseForge solo aparecen los archivos que son, o incluyen, un server pack.
- Instala la versión elegida en segundo plano: detiene el servidor, guarda un punto de restauración, quita lo que el modpack sustituye, descarga el modpack, instala el cargador, ajusta la versión de Java y de Minecraft, y vuelve a arrancar el servidor si estaba encendido.
- Recuerda qué modpack está instalado, lo muestra en la parte superior de la pestaña y bloquea el selector de versión del juego mientras haya un modpack instalado (véase [Versión del juego](game-versions.md)).

**Plugins** (pestaña **Plugins**):

- Trabaja en la carpeta de la que el servidor carga sus jars: `mods/` en servidores Fabric, Forge y NeoForge, y `plugins/` en Paper y en cualquier otro (vanilla, proxies, lanzadores desconocidos).
- Lista los archivos `.jar` de esa carpeta, con su tamaño y una versión deducida del nombre del archivo. En un servidor con mods incluye los que instaló un modpack.
- Busca en Modrinth (20 resultados) lo que el servidor puede cargar e instala la versión más reciente del proyecto elegido en esa carpeta.
- Elimina un jar de esa carpeta.

---

## Cómo funciona

### Instalación de un modpack

La instalación se ejecuta en el nodo que aloja el servidor. La petición se acepta con `202` y el trabajo continúa en segundo plano; la consola muestra cada paso y `GET /api/servers/:serverId/modpacks` informa del paso actual y del recuento de archivos (la pestaña lo consulta cada 3 segundos).

1. **Comprobaciones.** El origen debe ser `modrinth` o `curseforge`. Los identificadores o slugs de Modrinth deben cumplir `[A-Za-z0-9_-]{1,64}` y los de CurseForge deben ser numéricos. CurseForge exige la clave de API del nodo. La instalación se rechaza mientras haya en curso una copia o restauración, un cambio de versión u otra instalación de modpack.
2. **Parada.** Si el servidor está encendido, se detiene y el panel espera a que termine el proceso. Mientras se sustituyen los archivos, el servidor queda marcado como «instalando» para que nadie pueda arrancarlo.
3. **Punto de restauración.** Se crea un punto de restauración (motivo `modpack`). Si no se puede crear, no se modifica nada y la instalación falla. Los puntos de restauración aparecen en **Copias de seguridad**; véase [Copias de seguridad](backups.md).
4. **Limpieza.** En la raíz de la carpeta del servidor se elimina lo que el modpack trae en su propia versión:
   - las carpetas `mods`, `config`, `defaultconfigs`, `kubejs`, `libraries`, `scripts` y `.fabric`;
   - los lanzadores y scripts de arranque del cargador anterior (`server.jar`, `fabric-server-launch.jar`, `run.sh`, `run.bat`, `user_jvm_args.txt`, jars de instaladores, `forge-*.jar`, `neoforge-*.jar`, `minecraft_server*.jar` y similares).
   Los archivos `server.properties`, `whitelist.json`, `ops.json`, `banned-players.json`, `banned-ips.json` y `eula.txt` se leen antes y se vuelven a escribir después, aunque el modpack incluya su propia copia. El mundo se conserva salvo que se active **Mundo nuevo**: en ese caso también se borran las carpetas del mundo indicado en `level-name` (`world`, `world_nether` y `world_the_end` por defecto). Con un mundo nuevo se usa el `server.properties` del modpack, pero se mantienen los valores propios del servidor para `server-ip`, `server-port`, `query.port`, `enable-query`, `rcon.port`, `enable-rcon`, `rcon.password`, `motd`, `white-list`, `enforce-whitelist`, `online-mode` y `max-players`.
5. **Variables.** Se ajustan las variables del servidor `version` y `javaversion` (Java 8 por debajo de 1.17, 17 de 1.17 a 1.20.4, 21 desde 1.20.5 y 25 para las versiones numeradas por año, desde la 26.x). En CurseForge también se ajustan `projectId` y `fileId` si la plantilla las tiene. Si la instalación falla, estas variables recuperan su valor anterior.
6. **Descarga y cargador.**
   - **Modrinth:** se descarga el `.mrpack` y se comprueba su hash SHA-512, se descargan los mods que indica su índice (6 en paralelo, cada uno con su hash comprobado), se copian los `overrides` y se instala el cargador que pide el modpack (lanzador de Fabric o instalador de Forge/NeoForge). Los archivos del modpack solo pueden venir de `cdn.modrinth.com`, `github.com`, `raw.githubusercontent.com` o `gitlab.com`.
   - **CurseForge:** el panel ejecuta la misma operación `curseforge` que la plantilla de CurseForge, con el proyecto y el archivo del modpack.
7. **Archivos de la plantilla.** Se vuelven a ejecutar los pasos `writefile` de la instalación de la plantilla (por ejemplo, `eula.txt`) y se fija `server-port` en `server.properties` al puerto del servidor.
8. **Registro y arranque.** El panel busca `fabric-server-launch.jar` o `server.jar`; si no existe ninguno, la instalación falla. Después escribe `<carpeta de servidores>/<id>.modpack` con el modpack, la versión, el cargador, la versión del juego y el SHA-256 del lanzador, y arranca el servidor si antes estaba encendido.

El modpack solo se considera instalado mientras ese lanzador conserve la misma suma de comprobación. Por eso, restaurar una copia o reinstalar el servidor borra el estado de «modpack instalado» y desbloquea el selector de versión. El registro se elimina junto con el servidor.

### Explorador de plugins

El daemon deduce la plataforma del servidor igual que el [visor de mapa](world-map.md): a partir del cargador del modpack, la variable `modlauncher` o los pasos de instalación. Puede ser `paper` (Paper y sus variantes), `fabric`, `forge`, `neoforge` o desconocida. La plataforma decide la carpeta (`mods/` en Fabric, Forge y NeoForge; `plugins/` en los demás) y la búsqueda, que consulta la API pública de búsqueda de Modrinth con:

| Plataforma | Filtro de búsqueda |
|---|---|
| Paper | Proyectos de tipo `plugin` o `mod` con una versión para `paper`, `spigot` o `bukkit` |
| Fabric, Forge, NeoForge | Mods con una versión para ese cargador que funcionan en un servidor (lado servidor obligatorio u opcional; se excluyen los mods solo de cliente) |
| Desconocida | Proyectos de tipo `plugin` o `mod`, de cualquier cargador |

El cuadro de búsqueda indica *Buscar plugins en Modrinth* en Paper, *Buscar mods en Modrinth* en Fabric, Forge y NeoForge, y *Buscar en Modrinth (plugins, mods...)* en los demás.

Al instalar se toma la **versión más reciente hecha para este servidor**: la pestaña lee de `GET /api/servers/:id/map` la plataforma y la versión de Minecraft y pide las versiones de los mismos cargadores (`paper`, `spigot` y `bukkit` en Paper; `fabric`, `forge` o `neoforge` en los demás) y de esa versión del juego. Si la plataforma o la versión no se conocen, esa parte del filtro se omite; si Modrinth no tiene ninguna versión que encaje, la instalación responde `404` con `ErrPluginNoCompatibleVersion` y la pestaña lo indica. El archivo principal (o el primero) de la versión elegida debe tener un nombre `.jar` simple, sin carpetas; si no, la instalación responde `502` con `ErrPluginBadFileName` y no se escribe nada. Después el daemon comprueba el límite de disco del servidor (`409` si está agotado), intenta guardar un punto de restauración (motivo `plugin`; se omite si ya se creó uno en los últimos 30 minutos, y si hay una copia en curso la instalación continúa sin él) y descarga el archivo en la carpeta, creándola si hace falta. Al quitar un plugin, el punto de restauración se guarda solo cuando se encuentra el archivo. La descarga cuenta para el límite de disco del servidor; si se supera, el archivo parcial se elimina. La consola muestra `Plugin/Mod <id> installed successfully as <archivo>` y hay que reiniciar el servidor para cargarlo.

---

## Uso

### Desde el panel

**Modpacks**

1. Abrir el servidor e ir a la pestaña **Modpacks**. Solo aparece en servidores de tipo `minecraft-java`.
2. Elegir el origen con el selector **Origen de los modpacks** (Modrinth o CurseForge). Si CurseForge no tiene clave de API, el selector lo muestra bloqueado y los administradores ven un enlace a **Abrir ajustes**.
3. Buscar un modpack, o recorrer **Los más populares**, y elegir uno.
4. En el diálogo, elegir la **Versión** y activar **Mundo nuevo** solo si se quiere borrar el mundo actual. El diálogo detalla **Qué va a pasar** y avisa si el servidor tiene menos de 4096 MB de memoria.
5. Pulsar **Instalar modpack**. El progreso se ve en la pestaña y en la consola. Al terminar, el primer arranque tarda más de lo normal mientras el modpack genera sus archivos.

**Plugins**

1. Abrir la pestaña **Plugins** (visible para usuarios con `server.files.edit`).
2. **Marketplace**: buscar en Modrinth y pulsar **Instalar**. **Instalados**: ver los jars de `mods/` o `plugins/` y pulsar **Eliminar** para borrar uno.
3. Reiniciar el servidor para aplicar los cambios.

En servidores de Minecraft Java la pestaña puede ocultarse en **Configuración › Configuración de plugins › Habilitar pestaña de plugins**. Este interruptor se guarda en el navegador (`localStorage`), así que solo afecta a ese navegador y no bloquea la API.

### Mediante la API

Todas las rutas cuelgan de `/api/servers/:serverId` y se reenvían al nodo del servidor. Formatos completos de petición y respuesta: [Referencia de la API](../reference/11-api-reference.md).

| Método | Ruta | Scope | Finalidad |
|--------|------|-------|-----------|
| GET | `/modpacks` | `server.files.view` | Modpack instalado, instalación en curso (paso, recuentos, último error), si CurseForge está disponible, cargador y versión actuales, memoria |
| GET | `/modpacks/search?source=&q=&offset=` | `server.files.view` | Buscar modpacks; `source` es `modrinth` o `curseforge`; `q` se recorta a 100 caracteres |
| GET | `/modpacks/versions?source=&project=` | `server.files.view` | Versiones instalables de un modpack, de la más reciente a la más antigua |
| POST | `/modpacks/install` | `server.install` | Iniciar una instalación. Cuerpo: `{"source","projectId","versionId","freshWorld","label"}`. Responde `202` |
| GET | `/plugins` | `server.files.view` | Jars de `mods/` (Fabric, Forge, NeoForge) o de `plugins/` (los demás) |
| GET | `/plugins/search?q=` | `server.files.view` | Buscar en Modrinth lo que puede cargar la plataforma del servidor |
| POST | `/plugins/:pluginId?loaders=<lista>&game_versions=<lista>` | `server.files.edit` | Instalar la versión más reciente de un proyecto de Modrinth, limitada a los cargadores y versiones del juego indicados (listas separadas por comas, ambas opcionales); responde `204`, `404` `ErrPluginNoCompatibleVersion` si ninguna versión encaja, `502` `ErrPluginBadFileName` si el nombre del archivo no es un nombre `.jar` simple, o `409` si el límite de disco está agotado |
| DELETE | `/plugins?name=<archivo>.jar` | `server.files.edit` | Borrar un jar de `mods/` o `plugins/`, la misma carpeta de la lista (se añade `.jar` si falta); responde `204` |

Los scopes se explican en la [referencia de scopes](../general/scopes-reference.md).

---

## Requisitos y configuración

- **Modrinth** no necesita clave. El nodo debe poder conectar con `api.modrinth.com` y con los servidores de descarga indicados más arriba.
- **CurseForge** necesita una clave de API, creada en console.curseforge.com, en el nodo que ejecuta el servidor:

| Clave | Valor por defecto | Variable de entorno | Notas |
|-------|-------------------|---------------------|-------|
| `daemon.curseforge.key` | vacía (una compilación puede incluir una mediante el argumento de compilación `curseforgeKey` del Dockerfile) | `SKYPANEL_DAEMON_CURSEFORGE_KEY` | La lee el daemon. Para el nodo principal se puede indicar en **Configuración › General › Modpacks › Clave de API de CurseForge**. Los nodos remotos leen el valor de su propia configuración. |

- El servidor debe ser de tipo `minecraft-java` para que aparezca la pestaña Modpacks, y tener memoria suficiente para el modpack (el panel recomienda al menos 4096 MB).
- Espacio en disco suficiente: el límite de disco del servidor se aplica a todo lo que se escribe.

Véase [Configuración](../backend/02-configuration.md) para saber cómo se leen las claves y las variables de entorno.

---

## Límites

- Cargadores: solo se pueden instalar modpacks de **Fabric, Forge y NeoForge**. Los que solo funcionan con Quilt se ocultan o se rechazan.
- CurseForge: solo se ofrecen archivos con server pack, y los modpacks cuyo autor no permite la distribución por terceros se ocultan o se rechazan. Los resultados de búsqueda de CurseForge se limitan a 10 000.
- Límites de tamaño para modpacks de Modrinth: `.mrpack` de hasta 1 GB, cada mod de hasta 512 MB (o el tamaño que declare el índice) y overrides de hasta 4 GB en total. Cada descarga de archivo caduca a los 10 minutos y la instalación completa de Modrinth a las 2 horas.
- Solo una instalación de modpack por servidor a la vez, y nunca durante una copia, una restauración o un cambio de versión.
- Mientras haya un modpack instalado, la versión del juego no se puede cambiar desde el panel; en su lugar, se instala otra versión del modpack.
- Explorador de plugins: la lista solo muestra archivos `.jar` situados directamente en `mods/` o `plugins/`. En un servidor cuya plataforma el panel no reconoce (vanilla, un proxy, un lanzador propio) usa `plugins/` y la búsqueda no se filtra, así que conviene comprobar que lo que se instala encaja con el servidor.

---

## Problemas conocidos y soluciones

| Mensaje | Causa y solución |
|---------|------------------|
| `CurseForge needs an API key; an administrator can add it in the panel settings` | El nodo no tiene `daemon.curseforge.key`. Añadirla en **Configuración › General** (nodo principal) o en la configuración del nodo remoto. |
| `CurseForge: refused the request (check the API key)` | La clave es incorrecta o se ha revocado. Sustituirla. |
| `<sitio>: too many requests, try again in a minute` | Límite de peticiones de Modrinth o CurseForge. Esperar y reintentar. |
| `a modpack install is already running` / `a backup or restore is running; try again when it finishes` / `a version change is already running` | Otra tarea ocupa el servidor. Esperar a que termine. |
| `this modpack version uses a loader the panel cannot install (Fabric, Forge and NeoForge are supported)` | Elegir otra versión u otro modpack. |
| `invalid modpack request: that version has no server pack` | Archivo de CurseForge sin server pack. Elegir una versión marcada como **Server pack**. |
| `invalid modpack request: the author of this pack does not allow installs outside the CurseForge app` | El panel no puede instalar este modpack. |
| `could not create a restore point, nothing was changed: ...` | No se pudo escribir la copia (disco lleno, copia en curso). Liberar espacio y reintentar. |
| `the pack was installed but no server.jar was produced, so the server could not start` | El instalador del cargador no generó un lanzador. Revisar la consola; si hace falta, restaurar el punto de restauración desde **Copias de seguridad**. |
| `... did not match its sha512 hash; the download is damaged or was changed` | Descarga dañada o alterada. Reintentar más tarde. |
| `the pack wants a download from a host that is not allowed: ...` | El `.mrpack` apunta a un servidor fuera de la lista permitida. El modpack no se puede instalar. |
| `the modpack was installed but the server did not start: ...` | Los archivos están en su sitio, pero el arranque falló (a menudo por memoria o Java). Revisar la consola y la memoria del servidor. |
| `project or versions not found on Modrinth` (plugins) | El proyecto no tiene versiones o el identificador es incorrecto. |
| «Este proyecto no tiene ninguna versión para el loader y la versión de Minecraft de este servidor.» (`ErrPluginNoCompatibleVersion`, plugins) | El proyecto no publica ninguna versión para la plataforma y la versión de Minecraft del servidor. Buscar otro proyecto o una alternativa compatible. |
| «El archivo que ofrece Modrinth para este proyecto no es un .jar con un nombre de archivo normal, así que no se ha instalado.» (`502`, `ErrPluginBadFileName`, plugins) | El nombre del archivo del proyecto incluye una carpeta o no es un `.jar`. Descargar el archivo desde Modrinth y revisarlo antes de subirlo a mano. |
| `search query is required` (plugins) | Se llamó a `GET /plugins/search` sin `q`. |
| El plugin no se carga tras instalarlo | Reiniciar el servidor. Si el panel no reconoció la plataforma, el jar se instaló en `plugins/`: un servidor Fabric, Forge o NeoForge carga los mods desde `mods/`. |

Cuando la instalación de un modpack falla después de haber tocado archivos, el mensaje termina con `(a restore point was saved before the install; it is in Backups)`: basta con restaurarlo desde la pestaña **Copias de seguridad** para volver al estado anterior.

---

## Véase también

- [Versión del juego](game-versions.md): cambios de versión en un clic, bloqueados mientras haya un modpack instalado.
- [Mapa del mundo](world-map.md): BlueMap y Dynmap, también descargados de Modrinth.
- [Copias de seguridad](backups.md): puntos de restauración y cómo restaurarlos.
- [Operaciones](../backend/06-operations.md): las operaciones `curseforge`, `fabricdl`, `forgedl` y `neoforgedl` que usan las plantillas.
- [Referencia de la API](../reference/11-api-reference.md) · [Referencia de scopes](../general/scopes-reference.md) · [Configuración](../backend/02-configuration.md)
