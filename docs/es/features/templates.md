# Plantillas

Una plantilla es el modelo JSON a partir del que se crea un servidor: sus variables, pasos de instalación, comando de ejecución, entorno y requisitos. La página **Plantillas** muestra las plantillas que el panel puede instalar y permite a los administradores añadir las suyas. Esta página cubre la propia página, de dónde salen las plantillas y cómo añadirlas; el formato de las plantillas se describe en [Conceptos básicos: Plantillas](../general/concepts.md) y en [Operaciones](../backend/06-operations.md).

## Qué hace

La página (**Gestión de plantillas**, `/templates`) tiene dos pestañas:

- **Locales**: plantillas guardadas en la base de datos de este panel. Aquí se pueden crear, editar, subir y eliminar.
- **Comunidad**: plantillas leídas de una fuente externa, ya sea el índice de plantillas configurado en `templates.url` o los repositorios Git añadidos mediante la API. Son de solo lectura.

Cada tarjeta muestra el logo, el nombre y la categoría de la plantilla y un botón **Usar plantilla** que abre el asistente de creación de servidores con esa plantilla ya elegida (`/servers/?create=true&template=<nombre>&repo=<id>`).

Botones, según el permiso:

| Botón | Necesita |
|---|---|
| **Subir** (uno o varios archivos `.json`) | `templates.local.edit` |
| **Crear plantilla** | `templates.repo.create` para verlo; guardar necesita `templates.local.edit` |
| **Editar** / **Eliminar** en una plantilla local | `templates.local.edit` |

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

El diálogo **Crear plantilla** / **Editar plantilla** tiene tres pestañas sincronizadas: **General** (**Nombre (ID único)**, **Nombre visible**, **Categoría (tipo)**), **Configuración de ejecución** (**Comando de inicio**, **Comando de parada**, **Directorio de trabajo**) y **JSON** (la plantilla completa, con **Cargar desde archivo JSON**). Al guardar se envía el JSON con `PUT /api/templates/0/<nombre>`; una plantilla con el mismo nombre se sustituye.

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
2. Para añadir plantillas propias, pulsar **Subir** y elegir uno o varios archivos `.json`, o pulsar **Crear plantilla**, rellenar los campos (o pegar el JSON) y pulsar **Guardar plantilla**.
3. Para crear un servidor a partir de cualquier plantilla, pulsar **Usar plantilla** en su tarjeta.

### Mediante la API

| Método y ruta | Permiso | Para qué |
|---|---|---|
| `GET /api/templates` | `login` | Listar los repositorios (incluido `Local`, id `0`) |
| `GET /api/templates/:repo` | `login` | Listar las plantillas de un repositorio (resúmenes) |
| `GET /api/templates/:repo/:name` | `login` | Leer una plantilla completa |
| `PUT /api/templates/0/:name` | `templates.local.edit` | Crear o sustituir una plantilla local (cuerpo: el JSON de la plantilla) |
| `DELETE /api/templates/0/:name` | `templates.local.edit` | Eliminar una plantilla local |
| `POST /api/templates/upload` | `templates.local.edit` | Subir plantillas locales (`multipart/form-data`, campo `files`, repetible) |
| `POST /api/templates` | `templates.repo.create` | Añadir un repositorio Git: `{"name": "...", "url": "https://...", "branch": "main"}`, más `pat`, `username`, `password` o `sshKey` opcionales para un repositorio privado |
| `DELETE /api/templates/:repo` | `templates.repo.delete` | Quitar un repositorio Git |

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

- Las plantillas de la comunidad no se pueden editar directamente. Para cambiar una, abrirla, copiar su JSON en **Crear plantilla** y guardarla como plantilla local (con el mismo nombre u otro).
- Las credenciales de un repositorio privado solo se pueden indicar al añadirlo con la API; no hay forma de cambiarlas después salvo quitar el repositorio y añadirlo de nuevo.
- No hay botón en la web para añadir o quitar repositorios Git; se hace mediante la API. Mientras `templates.url` esté definido, los repositorios añadidos no se muestran.
- Con un índice en `templates.url`, los cambios publicados en el índice tardan hasta 5 minutos en verse (la caché en memoria se pierde al reiniciar el panel).
- **Crear plantilla** se muestra a quien tiene `templates.repo.create`, pero guardar necesita `templates.local.edit`.

## Problemas conocidos y soluciones

| Qué se ve | Causa | Solución |
|---|---|---|
| **No hay plantillas de la comunidad** | No hay índice configurado ni repositorios añadidos, o no se pudo descargar el índice | Revisar `templates.url` y que el panel pueda acceder a él; buscar `Error reading template from …` en el registro del panel |
| `file X is not a JSON template` / `template X has no install steps or run command` (`ErrInvalidTemplate`) | Se ha subido un archivo incorrecto | Subir un JSON de plantilla con `install` o `run.command` |
| **El archivo JSON no es válido.** | **Cargar desde archivo JSON** recibió JSON no válido | Corregir el archivo |
| **La plantilla debe tener un campo «name».** | Guardar desde el diálogo sin nombre | Rellenar **Nombre (ID único)** |
| `repo already exists with given name` (`ErrRepoExists`) | Se añade un repositorio con un nombre ya usado | Usar otro nombre |
| `bad status 403 from …` en el registro | El servidor del índice rechazó la descarga | Revisar la URL y la protección de ese servidor |

## Véase también

- [Conceptos básicos: Plantillas](../general/concepts.md)
- [Operaciones](../backend/06-operations.md): los pasos de instalación que puede usar una plantilla.
- [Referencia de permisos](../general/scopes-reference.md)
