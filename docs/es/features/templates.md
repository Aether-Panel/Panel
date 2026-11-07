# Plantillas

Una plantilla es el modelo JSON a partir del que se crea un servidor: sus variables, pasos de instalación, comando de ejecución, entorno y requisitos. La página **Plantillas** muestra las plantillas que el panel puede instalar y permite a los administradores añadir las suyas. Esta página cubre la propia página, de dónde salen las plantillas y cómo añadirlas; el formato de las plantillas se describe en [Conceptos básicos: Plantillas](../general/concepts.md) y en [Operaciones](../backend/06-operations.md).

## Qué hace

La página (**Gestión de plantillas**, `/templates`) tiene dos pestañas:

- **Locales**: plantillas guardadas en la base de datos de este panel. Aquí se pueden crear, editar, duplicar, subir y eliminar.
- **Comunidad**: plantillas leídas de una fuente externa, ya sea el índice de plantillas configurado en `templates.url` o los repositorios Git añadidos mediante la API. Son de solo lectura, pero se pueden duplicar como plantilla local.

Cada tarjeta muestra el logo, el nombre y la categoría de la plantilla y un botón **Usar plantilla** que abre el asistente de creación de servidores con esa plantilla ya elegida (`/servers/?create=true&template=<nombre>&repo=<id>`).

Botones, según el permiso:

| Botón | Necesita |
|---|---|
| **Subir** (uno o varios archivos `.json`) | `templates.local.edit` |
| **Nueva plantilla** (abre el [editor de plantillas](#editor-de-plantillas)) | `templates.local.edit` |
| **Editar** / **Eliminar** en el menú de una plantilla local | `templates.local.edit` |
| **Duplicar** en el menú de cualquier plantilla, local o de la comunidad | `templates.local.edit` |

**Buscador.** Encima de las pestañas, filtra las dos por nombre, nombre visible y tipo, sin distinguir mayúsculas ni tildes. También entiende sinónimos habituales (`c#` y `csharp` encuentran .NET, `golang` Go, `mysql` MariaDB, `cs2` Counter-Strike 2, `spring` o `maven` Java; la lista está en `client/frontend/src/lib/template-search.ts`). Los contadores de las pestañas muestran las coincidencias. El asistente de creación de servidores tiene el mismo buscador en el paso de elegir plantilla.

**Logos.** El campo `icon` de la plantilla elige su logo entre los de `client/frontend/public/img/icons/` (`client/frontend/src/lib/template-logos.ts`): las marcas oficiales en SVG para lenguajes, bases de datos y servicios, y el icono oficial de cada juego en PNG. Una plantilla sin `icon` se reconoce por su nombre (`minecraft-*`, `terraria-*`, `discord-*`…). El logo también aparece en el asistente, en las tarjetas de servidor y en la cabecera del servidor, que usan el `icon` del servidor o, en servidores antiguos, su tipo.

La página y la entrada **Plantillas** del menú necesitan `templates.view` (el rol por defecto «Usuario» lo tiene); sin él, la página lleva al panel de inicio.

## Cómo funciona

### Plantillas locales

Se guardan en la base de datos del panel, una fila por nombre de plantilla. Subir un archivo crea o sustituye la plantilla que lleva el nombre del archivo (`spigot.json` pasa a ser `spigot`). Cada archivo subido:

- debe terminar en `.json` y ocupar como mucho 2 MB;
- debe ser JSON válido;
- debe tener pasos de instalación (`install`) o un comando de ejecución (`run.command`); si no, se rechaza con `ErrInvalidTemplate`.

Si el archivo no tiene `display`, se usa el nombre.

El panel guarda el JSON de la plantilla tal como se escribió, tanto al subirla como al guardarla: las variables conservan su orden y los campos que el panel no conoce no se pierden. Las plantillas guardadas antes de este cambio mantienen el orden alfabético con el que se guardaron hasta que se vuelvan a guardar.

Las plantillas locales se crean y se editan en el [editor de plantillas](#editor-de-plantillas), que las guarda con `PUT /api/templates/0/<nombre>`; una plantilla con el mismo nombre se sustituye.

### Editor de plantillas

Una página completa (`/templates/editor/`) para crear una plantilla rellenando formularios, sin escribir JSON, aunque el JSON sigue a mano para quien lo prefiera. Necesita `templates.local.edit`; sin él, la página lleva a **Plantillas**.

#### Cómo empezar

**Nueva plantilla** ofrece cuatro opciones. Ninguna guarda nada hasta pulsar **Guardar** en el editor.

| Opción | Qué abre | Dirección |
|---|---|---|
| **En blanco** | Lo mínimo: nombre, nombre visible y categoría vacíos, sin variables ni pasos, un comando de arranque y de parada por rellenar, y un entorno admitido Docker con la imagen `ghcr.io/aether-panel/generic` | `/templates/editor/` |
| **Plantilla base** | Una plantilla lista para un caso habitual, sin nombre: **Minecraft: Java (Paper)**, **Juego de Steam**, **App Node.js**, **App Python** o **Docker genérico** | `/templates/editor/?preset=<id>` |
| **Duplicar una existente** | Una copia de una plantilla local o de la comunidad, elegida con un buscador (el mismo de la lista: nombre, nombre visible, tipo y sinónimos) | `/templates/editor/?from=<repo>:<nombre>` |
| **Importar un .json** | Un archivo de plantilla, para revisarlo antes de guardarlo. Si no tiene `name`, se usa el nombre del archivo | `/templates/editor/?import` |

En la lista, el menú **⋯** de cada plantilla tiene **Duplicar** y, en las locales, **Editar** (`/templates/editor/?name=<nombre>`).

- En el buscador de **Duplicar**, la flecha abajo pasa a la lista e **Intro** abre el primer resultado (solo cuando se ha escrito algo).
- Una copia conserva el nombre de su origen si está libre entre las plantillas locales; si no, pasa a `<nombre>-copy` (o `-copy-2`…) y a su nombre visible se le añade «(copia)».
- Las plantillas base escriben los textos de sus variables (nombre visible y descripción) en el idioma de quien las crea: es lo que leerán quienes creen servidores con ella. La de Minecraft usa Paper; vanilla es cambiar el paso `paperdl` por `mojangdl`. La de Steam deja vacíos (y obligatorios) el `appid` y el ejecutable: conviene rellenarlos antes de guardar; si no, quien cree el servidor tendrá que escribirlos.
- Todas usan solo imágenes `ghcr.io/aether-panel/*` u oficiales, y el panel las acepta al guardarlas (la de Steam, una vez rellenados el `appid` y el ejecutable).

#### La página

- **Secciones**, en un menú lateral (en un móvil, un selector arriba): **Información**, **Variables**, **Instalación**, **Ejecución**, **Entorno**, **Requisitos** y **Avanzado**. Cada una muestra cuántos elementos tiene (variables, pasos) y cuántos errores y avisos.
- **Formulario | JSON | Dividido**: el formulario y el JSON son dos vistas del mismo objeto; lo que se cambia en una aparece en la otra al momento. **Dividido** (formulario y JSON lado a lado) solo está en pantallas de 1280 px o más; en una más estrecha se ve como **Formulario**.
- **Vista previa**, **Descargar .json** y **Guardar**. Por debajo de 1280 px, **Vista previa** y **Descargar .json** son solo iconos, y por debajo de 420 px también **Guardar**. **Ctrl+S** (**⌘S** en macOS) guarda desde cualquier campo, también dentro del JSON.
- Al salir con cambios sin guardar, el navegador pregunta antes.

| Sección | Qué se edita |
|---|---|
| **Información** | Nombre (el ID único), nombre visible, categoría (`type`) e icono |
| **Variables** | Lo que rellena quien crea un servidor. Cada variable es una tarjeta plegable, con su clave, tipo (texto, número entero, sí/no o lista de opciones), valor por defecto, nombre visible, descripción, opciones y las marcas obligatoria, editable por los usuarios e interna. Se reordenan arrastrando o con ↑ ↓, en el orden en que las muestra «Crear servidor». Una clave que es solo un número va siempre la primera y no se puede mover. Al cambiar una clave se puede cambiar también cada `${clave}` que la usa |
| **Instalación** | Los pasos de instalación y, aparte, los de desinstalación. **Añadir paso** abre un buscador con las operaciones por categoría (Archivos, Descargas, Juegos/Minecraft, Steam, Java/Node, Sistema). Cada operación tiene su formulario; cada paso, su condición (**Siempre**, **Solo en host**, **Solo en Docker** o **Personalizada**, una expresión CEL). Un paso de un tipo que el panel no conoce se edita con **Editar JSON de este paso** |
| **Ejecución** | Comando de arranque (**Un comando** o **Según condición**, una lista de comandos con su condición), parada (comando o código de señal), carpeta de trabajo, variables de entorno, consola (entrada estándar, RCON, RCON por WebSocket o telnet), arranque y reinicio automáticos, código de salida esperado y los pasos **Antes de arrancar** y **Después de parar** |
| **Entorno** | Entorno por defecto (host o Docker) y entornos admitidos (imagen, puertos, red, carpetas montadas, etiquetas, `hostConfig` y `config` de Docker) |
| **Requisitos** | Sistemas operativos, arquitecturas y programas que necesita el nodo (`java \|\| java21` acepta cualquiera de los dos) |
| **Avanzado** | Grupos de variables, estadísticas (`jcmd`), consulta del estado (Minecraft o Steam A2S) y keep-alive. Lo que no tiene formulario se edita como JSON en su propia sección |

**Insertar variable**, junto a los comandos y a los campos de los pasos, escribe `${variable}` en el último campo de texto que tuvo el foco (o en el cursor del JSON).

**Lo que «Crear servidor» usa del entorno.** El asistente crea siempre los servidores en Docker, con el **primer entorno Docker** de **Entornos admitidos** (su imagen, puertos y ajustes). El entorno por defecto se guarda en la plantilla, pero el asistente no lo usa. Sin ningún entorno Docker en la lista, el asistente usa `ubuntu:22.04`, y sin la lista, la imagen genérica del nodo.

#### Formulario y JSON

- Nada se pierde: el formulario trabaja siempre sobre la plantilla completa, y los campos que no muestra (claves de pasos desconocidas, ajustes de Docker que no tienen campo…) se conservan tal cual. Abrir una plantilla y guardarla sin tocar nada guarda el mismo JSON, salvo que `name` pasa a ser la primera clave.
- La vista JSON es el editor Monaco del panel, que se carga la primera vez que se abre. Valida la plantilla con su esquema (subraya los errores) y autocompleta claves, tipos de operación y sus argumentos.
- Un JSON a medio escribir no rompe nada: el formulario se queda con la última versión válida y la barra lo marca como **El JSON no es válido**. Si se edita el formulario mientras tanto, los cambios no válidos del JSON se descartan.

#### Errores y avisos

El formulario y el JSON comprueban la plantilla con el mismo esquema mientras se escribe:

- **Errores**: lo que el nodo rechaza o en lo que falla (un campo obligatorio vacío, un número que no lo es, un paso sin `type`…). Se muestran bajo cada campo y se anuncian a los lectores de pantalla.
- **Avisos**: lo que el nodo acepta pero probablemente no como se pretendía. No impiden guardar. Por ejemplo: una `${variable}` sin definir o una variable que nadie usa, un número entero que el nodo convertiría en 0, comandos con condición sin ninguno por defecto, una parada sin comando ni código de señal, ningún entorno admitido Docker, un keep-alive incompleto o una **imagen de terceros**: el proyecto solo usa sus propias imágenes (`ghcr.io/aether-panel/*`) o imágenes oficiales, de Docker Hub (como `ubuntu` o `eclipse-temurin`) o del fabricante (como `mcr.microsoft.com` o `steamcmd/steamcmd`).

La línea de estado de la barra resume qué falta (**Sin problemas**, cuántos errores y avisos, o qué impide guardar). Con algo pendiente es un botón que abre **Errores y avisos**: la lista completa agrupada por sección, con primero lo que impide guardar; al elegir uno, el editor abre su sección y lleva el foco a su campo.

**Guardar** se desactiva en una plantilla ya guardada que no tiene cambios, o cuando hay un error que impide guardar: el nombre vacío o no válido, un nombre que ya usa otra plantilla local, no haber podido leer las plantillas locales para comprobarlo (**Volver a comprobar** lo reintenta), un paso que no es un objeto o un JSON roto. Con otros errores, **Guardar** pregunta antes (**Guardar igualmente**). Al guardar, el panel valida la plantilla como siempre; si la rechaza, su mensaje se muestra.

#### Vista previa

**Vista previa** abre un panel lateral con el paso de configuración del asistente «Crear servidor» (recursos, variables y puertos), dibujado con los mismos componentes y los mismos valores iniciales que el asistente real. Se puede rellenar y pulsar **Comprobar**, que aplica las comprobaciones del asistente y lleva el foco al primer campo por corregir; **Restablecer** vuelve a los valores de la plantilla. No se crea nada ni se cambia la plantilla. Sin nodo elegido, no se comprueban los puertos libres ni la memoria del nodo.

#### Guardar y descargar

- **Guardar** envía la plantilla con `PUT /api/templates/0/<nombre>`. Cambiar el nombre de una plantilla existente guarda una plantilla nueva y deja la anterior como estaba; cambiar solo mayúsculas o acentos no cuenta como otro nombre. Tras guardar, la dirección pasa a `?name=<nombre>`, así que recargar la página sigue editando lo guardado.
- **Descargar .json** baja la plantilla tal como está en el editor como `<nombre>.json`, sin guardarla. No está disponible mientras el JSON no es válido.

### Plantillas de la comunidad

Su origen depende de `templates.url`:

**Con `templates.url` definido** (el archivo de Docker Compose pone `https://templates.aetherpanel.es/templates.json`): el panel ignora los repositorios Git guardados en la base de datos y ofrece un único repositorio llamado `community` (id `1`) construido a partir de ese índice. El índice es un objeto JSON que asocia cada nombre de plantilla con la URL de su archivo JSON; las URL relativas se resuelven respecto a la dirección del propio índice:

```json
{
  "paper": { "url": "minecraft/paper.json" },
  "terraria": { "url": "https://example.com/templates/terraria.json" }
}
```

El índice y las plantillas que lista se guardan en memoria durante 5 minutos: en ese tiempo, pedir la lista no vuelve a descargar nada (10 segundos de tiempo máximo por petición cuando sí se descarga). Al leer una plantilla concreta se usa el índice guardado y se descarga solo esa plantilla. Una plantilla que no se puede descargar o interpretar se omite y queda en el registro; en ese caso la lista incompleta no se guarda y la petición siguiente lo vuelve a intentar. Si cambia `templates.url`, nunca se sirve la copia del índice anterior.

**Sin `templates.url`**: la pestaña de la comunidad muestra los repositorios Git guardados con `POST /api/templates`. Cada uno se clona (una sola rama) en `<daemon.data.cache>/template-repos/<id>` la primera vez que se lee, y se actualiza de nuevo cuando la copia tiene más de 5 minutos. Estructura esperada: una carpeta por grupo, cada una con archivos `<nombre>.json` y, opcionalmente, un `README.md` que se muestra con la plantilla; los archivos `data.json` y las carpetas ocultas se ignoran.

**Repositorios privados**: `POST /api/templates` acepta credenciales opcionales que se guardan con el repositorio y se usan al clonar y al actualizar. La API nunca las devuelve.

| Dirección del repositorio | Campos | Cómo se usan |
|---|---|---|
| `https://…` / `http://…` | `pat` (token de acceso personal), o `username` y `password` | Autenticación básica. Con `pat`, el usuario es `username` o, si está vacío, `git` (GitHub y GitLab aceptan cualquier usuario con un token; Bitbucket necesita el real). Sin `pat`, se usan `username` y `password`. |
| SSH (`git@host:…`, `ssh://…`) | `sshKey` (clave privada), `username`, `password` | La clave SSH; el usuario es `username`, o el de la URL, o `git`. `password` desbloquea una clave cifrada. El host debe estar en el `known_hosts` del panel. |

Sin credenciales no se envía ninguna autenticación, lo que sirve para repositorios públicos por HTTPS. Los errores de autenticación no incluyen las credenciales: solo indican qué tipo falló (por ejemplo, `the SSH key of template repo <nombre> cannot be read`).

## Uso

### Desde el panel

1. Abrir **Plantillas** en la barra lateral.
2. Para añadir plantillas propias, pulsar **Subir** y elegir uno o varios archivos `.json`, o pulsar **Nueva plantilla**, elegir cómo empezar, rellenar las secciones del editor y pulsar **Guardar**.
3. Para cambiar una plantilla local, **⋯ → Editar** en su tarjeta. Para partir de cualquier otra, **⋯ → Duplicar**.
4. Para crear un servidor a partir de cualquier plantilla, pulsar **Usar plantilla** en su tarjeta.

### Mediante la API

| Método y ruta | Permiso | Para qué |
|---|---|---|
| `GET /api/templates` | `login` | Listar los repositorios (incluido `Local`, id `0`) |
| `GET /api/templates/:repo` | `login` | Listar las plantillas de un repositorio (resúmenes) |
| `GET /api/templates/:repo/:name` | `login` | Leer una plantilla completa |
| `PUT /api/templates/0/:name` | `templates.local.edit` | Crear o sustituir una plantilla local (cuerpo: el JSON de la plantilla) |
| `DELETE /api/templates/0/:name` | `templates.local.edit` | Eliminar una plantilla local; se rechaza (`409` `ErrTemplateInUse`) mientras un producto de la tienda cree sus servidores con ella y ningún otro repositorio tenga una plantilla con ese nombre |
| `POST /api/templates/upload` | `templates.local.edit` | Subir plantillas locales (`multipart/form-data`, campo `files`, repetible) |
| `POST /api/templates` | `templates.repo.create` | Añadir un repositorio Git: `{"name": "...", "url": "https://...", "branch": "main"}`, más `pat`, `username`, `password` o `sshKey` opcionales para un repositorio privado |
| `DELETE /api/templates/:repo` | `templates.repo.delete` | Quitar un repositorio Git; se rechaza (`409` `ErrTemplateRepoInUse`) mientras sea el único con la plantilla de un producto de la tienda |

Ejemplo de subida:

```bash
curl -X POST https://panel.example.com/api/templates/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "files=@paper.json" -F "files=@velocity.json"
```

Más detalles en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

| Clave | Valor por defecto | Variable de entorno | Efecto |
|---|---|---|---|
| `templates.url` | vacío (Docker Compose pone `https://templates.aetherpanel.es/templates.json`) | `SKYPANEL_TEMPLATES_URL` | Índice de plantillas usado como repositorio `community`; si está definido, se ignoran los repositorios Git de la base de datos |
| `daemon.data.cache` | `cache` (dentro de `daemon.data.root`) | `SKYPANEL_DAEMON_DATA_CACHE` | Dónde se clonan los repositorios Git |

El panel necesita salida HTTPS hacia el índice (o hacia los servidores Git). Ver [Configuración](../backend/02-configuration.md).

## Límites

- Las plantillas de la comunidad no se pueden editar directamente. Para cambiar una, **⋯ → Duplicar** y guardar la copia como plantilla local (con el mismo nombre u otro).
- El editor no tiene historial de versiones ni edición compartida: si dos personas guardan la misma plantilla, se queda la última.
- La condición de un grupo de variables y su campo «Orden» se guardan, pero el panel aún no los usa.
- Los avisos sobre condiciones no reconocen los literales CEL con prefijo (`r"…"`, `b"…"`) ni los nombres de tipo usados sin llamar (`string`, `int`…): los marcan como variables sin definir. Ninguna plantilla de la comunidad los usa.
- Las credenciales de un repositorio privado solo se pueden indicar al añadirlo con la API; no hay forma de cambiarlas después salvo quitar el repositorio y añadirlo de nuevo.
- No hay botón en la web para añadir o quitar repositorios Git; se hace mediante la API. Mientras `templates.url` esté definido, los repositorios añadidos no se muestran.
- Con un índice en `templates.url`, los cambios publicados en el índice tardan hasta 5 minutos en verse (la caché en memoria se pierde al reiniciar el panel).
- El editor lee la plantilla como lo hace el navegador, así que reescribe los números que este no guarda tal cual: un entero mayor que 2^53 (9007199254740992) pierde sus últimas cifras y un número escrito como `1.0` se guarda como `1`. Escribe entre comillas los identificadores largos (un Steam64 o un ID de Discord): `"76561198000000000"`.
- El editor compara los nombres de plantilla sin distinguir mayúsculas ni acentos en todas las bases de datos, como hace MariaDB. Con SQLite, que sí los distingue, da igualmente por ocupado un nombre que solo cambia en eso (`Paper` cuando existe `paper`) y cambiar solo mayúsculas o acentos no renombra la plantilla.
- Una plantilla guardada con `PUT` puede ocupar como mucho 2 MB, como un archivo subido; si es más grande, se rechaza con `413`.

## Problemas conocidos y soluciones

| Qué se ve | Causa | Solución |
|---|---|---|
| **No hay plantillas de la comunidad** | No hay índice configurado ni repositorios añadidos, o no se pudo descargar el índice | Revisar `templates.url` y que el panel pueda acceder a él; buscar `Error reading template from …` en el registro del panel |
| `file X is not a JSON template` / `template X has no install steps or run command` (`ErrInvalidTemplate`) | Se ha subido un archivo incorrecto | Subir un JSON de plantilla con `install` o `run.command` |
| **El archivo JSON no es válido.** | **Importar un .json** recibió JSON no válido | Corregir el archivo |
| **Guardar** desactivado y **Corrige los errores marcados para poder guardar.** | Un error que impide guardar (nombre vacío, repetido o no válido, un paso que no es un objeto) | Abrir **Errores y avisos** desde la barra y elegir el error para ir a su campo |
| **No se pudieron leer las plantillas locales…** | El editor no pudo comprobar si el nombre ya existe | **Volver a comprobar** junto al nombre |
| **No se pudo guardar la plantilla: …** | El panel rechazó la plantilla al validarla | Corregir lo que dice el mensaje (por ejemplo, una plantilla sin `install` ni `run.command`) |
| **No hay nada que importar** al abrir `?import` | El archivo importado se guarda solo en esa pestaña del navegador | Volver a importarlo desde **Nueva plantilla → Importar un .json** |
| `repo already exists with given name` (`ErrRepoExists`) | Se añade un repositorio con un nombre ya usado | Usar otro nombre |
| `bad status 403 from …` en el registro | El servidor del índice rechazó la descarga | Revisar la URL y la protección de ese servidor |

## Véase también

- [Conceptos básicos: Plantillas](../general/concepts.md)
- [Operaciones](../backend/06-operations.md): los pasos de instalación que puede usar una plantilla.
- [Referencia de permisos](../general/scopes-reference.md)
