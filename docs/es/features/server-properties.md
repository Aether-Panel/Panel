# Editor de propiedades del servidor

La pestaña **Propiedades** convierte el `server.properties` de un servidor de Minecraft: Java Edition en un formulario: cada opción con una explicación sencilla, agrupada por tema, con una vista previa en directo de cómo se ve el servidor en la lista de servidores del juego. Es una función **experimental** (marcada como *Beta* en la barra de pestañas) y viene desactivada.

## Qué hace

- Lee `server.properties` de los archivos del servidor y muestra sus opciones en seis secciones: **La partida**, **Jugadores**, **El mundo**, **Presentación**, **Red y rendimiento** y **Avanzado**.
- Usa el control adecuado para cada opción: interruptores para verdadero/falso, listas para valores fijos (modo de juego, dificultad, tipo de mundo…), campos numéricos con límites, campos de texto y un campo oculto para `rcon.password`.
- Muestra el valor por defecto de Minecraft en las opciones que todavía no están en el archivo, marcadas como **valor por defecto**.
- Tiene una tarjeta propia para el **Mensaje del día (MOTD)**: dos líneas como máximo, una paleta con los 16 códigos de color (`§0`–`§f`) y los de formato (negrita, cursiva, subrayado, tachado, quitar el formato), y una **Vista previa en la lista de servidores** (aproximada: el juego usa su propia fuente).
- Avisa bajo los valores arriesgados: `online-mode=false` («cualquiera puede entrar con cualquier nombre, incluido el de un administrador») y `hardcore=true`.
- Marca las opciones que **Solo afecta a mundos nuevos** (`level-seed`, `level-type`, `generate-structures`, `generator-settings`).
- Muestra `server-port` y `server-ip` en solo lectura: el panel los gestiona a partir de los puertos del servidor, y el aviso del campo remite a **Configuración → Puertos** (ver [Configuración del servidor](./server-settings.md)).
- Muestra en **Avanzado**, con la etiqueta *unknown*, las opciones que el panel no conoce; se conservan y se pueden editar como texto.
- Tiene un buscador (**Buscar una opción…**) que filtra por clave.

## Cómo funciona

El editor trabaja sobre el propio archivo mediante la API de archivos; no hay un almacenamiento aparte.

1. **Carga**: `GET /api/servers/:id/file/server.properties`. El texto se interpreta con las mismas reglas que `Properties` de Java (`clave=valor`, `clave: valor` o `clave valor`, comentarios con `#`/`!`, líneas continuadas, escapes `\uXXXX`).
2. **Edición**: cada cambio se guarda como borrador. Los campos numéricos se comprueban contra sus límites (por ejemplo `max-players` 1–10000, `view-distance` y `simulation-distance` 3–32); mientras un valor es incorrecto la barra indica **Corrige los valores marcados.** y no se puede guardar.
3. **Guardado**: el archivo se **vuelve a leer**, los cambios se aplican sobre esa copia recién leída y el resultado se escribe con `PUT /api/servers/:id/file/server.properties`. Las líneas no tocadas (comentarios, líneas en blanco, orden, claves desconocidas) quedan idénticas; las claves modificadas se reescriben en su sitio; las que no estaban en el archivo se añaden al final. Los valores se escapan como lo hace `Properties.store` (lo que no es ASCII como `\uXXXX`, `:` y `=` escapados), que todas las versiones de Minecraft leen sin problema.
4. **Aplicación**: Minecraft solo lee el archivo al arrancar. Tras guardar, la página indica **Cambios guardados.** y, si el servidor está encendido, ofrece **Reiniciar ahora** (`POST /api/servers/:id/restart`).

Qué opciones se muestran: las habituales («core») aparecen siempre, con el valor por defecto si faltan; el resto solo aparecen si el archivo ya las tiene. La sección **Avanzado** queda plegada tras **Mostrar opciones avanzadas (N)** y se abre sola al buscar.

## Uso

### Desde el panel

1. Un administrador la activa una vez: **Configuración del panel → General → Funciones experimentales → Editor de propiedades** (ver [Configuración del panel](./panel-settings.md)).
2. Abrir un servidor de Minecraft Java e ir a la pestaña **Propiedades**.
3. Cambiar las opciones. Cada campo modificado tiene un botón para deshacer; **Descartar** quita todos los cambios.
4. Pulsar **Guardar cambios** (o Ctrl/Cmd+S). Salir de la página con cambios sin guardar pide confirmación.
5. Si el servidor está encendido, pulsar **Reiniciar ahora** en el aviso de confirmación, o reiniciarlo más tarde.

### Mediante la API

No hay un endpoint propio: el editor usa las rutas de archivos y de encendido.

| Método y ruta | Permiso | Para qué |
|---|---|---|
| `GET /api/servers/:id/file/server.properties` | `server.files.view` | Leer el archivo |
| `PUT /api/servers/:id/file/server.properties` | `server.files.edit` | Escribir el archivo completo (cuerpo en bruto) |
| `POST /api/servers/:id/restart` | `server.start` y `server.stop` | Aplicar los cambios |

Ver la sección de archivos en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

- El tipo del servidor debe ser `minecraft-java`.
- El usuario necesita `server.files.edit` en el servidor (si no, la pestaña no aparece). El propietario lo tiene.
- El interruptor experimental debe estar activado:

| Clave | Valor por defecto | Variable de entorno | Efecto |
|---|---|---|---|
| `panel.experimental.propertiesEditor` | `false` | `SKYPANEL_PANEL_EXPERIMENTAL_PROPERTIESEDITOR` | Muestra la pestaña **Propiedades** en los servidores de Minecraft Java |

Desactivarlo solo oculta la pestaña; el archivo no se toca. Ver [Configuración](../backend/02-configuration.md).

## Límites

- Solo para servidores cuyo tipo es exactamente `minecraft-java`. Bedrock, los proxies y otros juegos no tienen la pestaña.
- El interruptor solo oculta la pestaña en la web: las rutas de archivos siguen disponibles para quien tenga los permisos de archivos, como siempre.
- El catálogo cubre las opciones habituales de vanilla; las que añaden versiones nuevas o mods aparecen en **Avanzado** como campos de texto desconocidos, sin validación.
- Los cambios solo se aplican tras un reinicio; el editor no envía comandos al juego en marcha.
- La vista previa del MOTD es aproximada.

## Problemas conocidos y soluciones

| Qué se ve | Causa | Solución |
|---|---|---|
| **Aún no hay server.properties** | Minecraft crea el archivo en el primer arranque (la API respondió `404`) | Encender el servidor una vez y volver |
| **Sin acceso a los archivos** | `403`: falta `server.files.view` | Pedir al propietario permisos de archivos |
| **No se pudo leer server.properties** | Nodo apagado u otro error | Revisar el nodo y pulsar **Reintentar** |
| **No se pudieron guardar los cambios.** (o el mensaje del nodo) | Ha fallado la escritura (nodo apagado, límite de disco alcanzado, falta `server.files.edit`) | Revisar el nodo y el uso de disco y volver a guardar |
| No aparece la pestaña **Propiedades** | Interruptor experimental apagado, tipo de servidor distinto de `minecraft-java` o falta `server.files.edit` | Comprobar las tres condiciones |
| No se puede editar `server-port` | Lo gestiona el panel | Cambiar el puerto principal en **Configuración → Puertos** |

## Véase también

- [Configuración del servidor](./server-settings.md): puertos, variables e inicio automático.
- [Archivos y compresión](../general/19-files-compression.md)
- [Configuración del panel](./panel-settings.md): funciones experimentales.
