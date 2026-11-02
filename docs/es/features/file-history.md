# Historial de archivos

Cada vez que se guarda un archivo de configuración desde el panel, Aether Panel conserva el texto que tenía antes. Las versiones anteriores se pueden comparar con el texto actual y recuperar con un clic, sin restaurar una copia de seguridad completa.

## Qué hace

- Conserva hasta **20 versiones anteriores** de cada archivo de configuración de texto de un servidor.
- Las muestra, de la más reciente a la más antigua, en un panel **Historial de versiones** junto al editor de archivos.
- Compara cualquier versión con el texto actual del editor (incluidos los cambios sin guardar).
- **Restaura** una versión; antes guarda como versión nueva el texto que se va a sustituir, de modo que la propia restauración se puede deshacer.

## Cómo funciona

### Cuándo se guarda una versión

El nodo guarda una versión **justo antes de sobrescribir un archivo mediante la API de archivos del panel** (`PUT /api/servers/:serverId/file/*`). Esto incluye:

- guardar un archivo en el editor de la pestaña **Archivos**;
- guardar en el editor de **Propiedades** (escribe `server.properties` mediante la misma API);
- subir desde el panel un archivo que sustituye a otro existente.

Los cambios hechos por otras vías **no** se registran: SFTP, el propio juego, los plugins, la extracción de archivos comprimidos, la restauración de copias o la instalación de modpacks.

El texto que se va a sustituir solo se conserva si se cumplen todas estas condiciones:

| Comprobación | Regla |
|--------------|-------|
| Tipo de archivo | La extensión es una de `.properties`, `.yml`, `.yaml`, `.json`, `.json5`, `.toml`, `.conf`, `.cfg`, `.ini`, `.txt`, `.xml`, `.sk`, `.js`, `.ts`, `.lua`, `.mcmeta`, `.cnf`, `.env`, o el archivo no tiene extensión y su contenido es UTF-8 válido |
| Texto, no binario | Ningún byte NUL en los primeros 8 KiB |
| Tamaño | 1 MiB o menos |
| Existe | Un archivo nuevo todavía no tiene nada que conservar |
| Ha cambiado | Si es idéntico a la última versión guardada (mismo SHA-256), no se crea otra |

Guardar la versión nunca hace fallar el guardado: cualquier problema solo queda en el registro del nodo.

### Almacenamiento

Las versiones se guardan en el nodo, junto a la carpeta del servidor y **fuera** de ella:

```
<daemon.data.servers>/<id-del-servidor>.history/<hash-de-la-ruta>/
    index.json      lista de versiones, de la más reciente a la más antigua
    <id-version>.v  el texto de cada versión
```

Al estar fuera de la raíz del servidor, sus usuarios no pueden verla ni modificarla desde el gestor de archivos ni por SFTP. Cada archivo tiene su propia carpeta, con el nombre de un hash de su ruta relativa, así que el mismo archivo siempre corresponde al mismo historial (`/config/a.yml` y `config/a.yml` son el mismo archivo). Se rechazan las rutas que contienen `..`.

### Retención

- **20 versiones por archivo**: al guardar la vigésimo primera se borra la más antigua de ese archivo.
- **50 MiB por servidor** en total: si el historial de todos los archivos juntos lo supera, se borran primero las versiones más antiguas de cualquier archivo hasta quedar por debajo.
- Todo el historial de un servidor se elimina al eliminar el servidor.
- Clonar un servidor no copia su historial de archivos ([Clonar servidores](./cloning.md)).

### Fecha de la versión

Cada versión indica cuándo se **escribió ese texto en el archivo** (la fecha de modificación del archivo en el momento de sustituirlo), no cuándo se sustituyó.

### Restauración

1. Se lee la versión elegida.
2. El texto actual del archivo se guarda como versión nueva (con las mismas reglas de arriba).
3. Se crean las carpetas superiores que falten y se escribe el texto de la versión en el archivo.

No es necesario detener el servidor. Como con cualquier cambio de configuración, la mayoría de juegos solo vuelven a leer el archivo al reiniciar.

## Uso

### Desde el panel

1. Abrir **Archivos** y editar un archivo con historial (p. ej. `server.properties` o un `.yml` de configuración).
2. Pulsar **Historial** en la barra del editor (su descripción emergente es **Ver el historial de versiones**, con el número de versiones si las hay). El botón solo aparece en los tipos de archivo que tienen historial.
3. El panel **Historial de versiones** lista las versiones guardadas, de la más reciente a la más antigua, con fecha y tamaño; la más reciente lleva la marca **Última**. El pie recuerda que se guardan las 20 versiones más recientes de cada archivo de configuración.
4. Elegir una versión para compararla con el texto actual. En pantallas anchas la comparación es lado a lado (izquierda: esa versión; derecha: el texto actual del editor); en pantallas estrechas es en línea (las líneas con `−` son de esa versión y las de `+`, del texto actual). Si coinciden, el panel indica «Esta versión es igual al texto actual.».
5. Pulsar **Restaurar esta versión** y confirmar (**¿Restaurar esta versión?**). Si el editor tiene cambios sin guardar, el diálogo avisa de que se perderán. Tras restaurar, el aviso **Versión restaurada** confirma que el texto anterior quedó en el historial.
6. **Volver al editor** regresa al archivo sin cambios.

Si todavía no hay versiones, el panel muestra **Todavía no hay versiones**: la primera aparecerá la próxima vez que se guarde el archivo.

### Mediante la API

Todas las rutas reciben el archivo en el parámetro `path`, relativo a la carpeta del servidor. El panel las reenvía al nodo.

| Método | Ruta | Permiso | Uso |
|--------|------|---------|-----|
| `GET` | `/api/servers/:serverId/file-history?path=<archivo>` | `server.files.view` | Listar versiones: `[{"id": "…", "savedAt": <unix ms>, "size": <bytes>}]`, de la más reciente a la más antigua; `[]` si no hay ninguna |
| `GET` | `/api/servers/:serverId/file-history/:versionId?path=<archivo>` | `server.files.view` | Texto de una versión (`text/plain; charset=utf-8`, `Cache-Control: no-store`) |
| `POST` | `/api/servers/:serverId/file-history/:versionId/restore?path=<archivo>` | `server.files.edit` | Restaurar esa versión; `204` si va bien |

Los ID de versión tienen la forma `<marca de tiempo en base 36>-<6 caracteres hex>`. Detalles completos: [Referencia de la API](../reference/11-api-reference.md). Permisos: [Referencia de permisos](../general/scopes-reference.md).

## Requisitos y configuración

Esta función no tiene ajustes: siempre está activa y sus límites (20 versiones por archivo, 50 MiB por servidor, 1 MiB por archivo) están fijados en el código (`internal/servers/file_history.go`).

La carpeta del historial se crea dentro de la carpeta de servidores, `daemon.data.servers` (por defecto `servers`, relativa a `daemon.data.root`; `/var/lib/SkyPanel/servers` en la instalación con Docker). Véase [Configuración](../backend/02-configuration.md).

## Límites

- Solo se registran las ediciones hechas desde el panel; no las de SFTP ni los cambios del juego.
- Nunca se registran archivos de más de 1 MiB, archivos binarios ni tipos fuera de la lista.
- 20 versiones por archivo y 50 MiB por servidor; las versiones antiguas se borran sin aviso.
- La restauración sustituye el archivo entero; no hay fusión línea a línea.
- Borrar o renombrar un archivo no traslada su historial; un archivo renombrado empieza un historial nuevo (el antiguo permanece hasta que el límite del servidor lo desplace o se elimine el servidor).

## Problemas conocidos y soluciones

| Mensaje / síntoma | Causa | Solución |
|-------------------|-------|----------|
| No aparece el botón **Historial** en el editor | El tipo de archivo no tiene historial (p. ej. `.jar`, `.log`, imágenes) | Nada que hacer; solo se guardan formatos de configuración de texto |
| **Todavía no hay versiones** tras editar por SFTP | Solo se registran los guardados hechos desde el panel | Editar el archivo en el editor del panel |
| Un guardado no creó versión nueva | El texto era idéntico a la última versión, el archivo superaba 1 MiB o parecía binario | Comportamiento esperado |
| **No se pudo cargar el historial** | El nodo no respondió | Comprobar que el nodo está en línea y reintentar |
| `invalid file path` (`400`) | `path` vacío o con `..` | Enviar una ruta relativa a la carpeta del servidor |
| `file version not found` (`404`) | El ID de versión no es válido o la versión ya se eliminó | Recargar la lista y elegir una versión existente |
| **No se pudo restaurar la versión.** | Error de escritura en el nodo (p. ej. disco lleno) o no se pudo guardar antes el texto actual (`keeping the current text before restoring: …`) | Liberar espacio en el nodo y reintentar |

## Véase también

- [Copias de seguridad](./backups.md) — copias completas y puntos de restauración
- [Archivos y compresión](../general/19-files-compression.md)
- [Referencia de la API](../reference/11-api-reference.md)
