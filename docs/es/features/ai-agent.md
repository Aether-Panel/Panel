# Asistente de IA: trabajar con archivos

El [asistente del servidor](./ai-assistant.md) puede hacer algo más que responder preguntas: puede mirar los archivos del servidor y, con su aprobación, cambiarlos. Basta con pedírselo con palabras normales («crea un `config.json` con…», «cambia el nombre del servidor en la lista de servidores», «mueve los archivos de configuración a una carpeta») y lista carpetas, lee archivos, busca, crea, edita, copia, mueve, renombra y borra, siempre dentro del servidor en el que se ha abierto.

Todo lo que cambia o elimina datos existentes espera a la persona usuaria: el asistente muestra lo que quiere hacer y no ocurre nada hasta pulsar **Aprobar**.

Cómo está construido (herramientas, function calling, aislamiento, confirmaciones) se describe en [Kit de herramientas del agente de IA](../backend/14-ai-toolkit.md).

## Qué hace

| Si se pide… | Lo que usa el asistente | Necesita aprobación |
|---|---|---|
| Ver lo que hay: listar una carpeta, leer un archivo, consultar el tamaño o la fecha de un archivo, buscar archivos por nombre o por una palabra que contengan | `list_directory`, `read_file`, `file_info`, `search_files` | No |
| Algo nuevo: crear un archivo de texto, crear una carpeta, copiar un archivo o una carpeta (por ejemplo, una copia de seguridad antes de un cambio) | `create_file`, `create_directory`, `copy_path` | No: nunca sustituyen nada que ya exista |
| Cambiar lo que existe: editar parte de un archivo, sustituir un archivo entero, mover o renombrar | `edit_file`, `write_file`, `move_path`, `rename_path` | Sí |
| Borrar un archivo, o una carpeta con todo su contenido | `delete_path` | Sí |

El asistente sigue recibiendo el contexto habitual del servidor (estado, jugadores, rendimiento, caídas y consola, véase [Asistente de IA](./ai-assistant.md#contexto-que-reúne-el-asistente)), así que puede combinar lo que dice la consola con lo que hay en los archivos.

Lo que **no** puede hacer: ejecutar comandos (de shell o de consola), arrancar, detener o reiniciar el servidor, instalar nada, llegar a los archivos de otro servidor ni a nada fuera de la carpeta de este servidor, borrar la propia carpeta del servidor, leer archivos binarios (jars, mundos, imágenes) o de más de 1 MiB, ni cambiar los ajustes del panel o los permisos.

### Ejemplos

**Crear un archivo.** *«Crea `plugins/MyPlugin/config.json` con `{"enabled": true}`.»* El asistente crea el archivo (y las carpetas que falten) en el acto, porque crear nunca sustituye nada; si el archivo ya existe, lo dice en lugar de hacerlo. Bajo su respuesta: *Created plugins/MyPlugin/config.json (17 bytes)* — **Hecho**.

**Cambiar un ajuste.** *«Cambia el nombre que ven los jugadores en la lista de servidores a Isla Supervivencia.»* El asistente lee `server.properties`, encuentra la línea `motd=` y propone *Replace 1 occurrence in server.properties*. Aparece la tarjeta; al pulsar **Aprobar** se cambia la línea. El texto anterior se guarda en el [historial de versiones](./file-history.md) del archivo, así que se puede comparar y restaurar. El juego lee `server.properties` al arrancar: hay que reiniciar el servidor para aplicarlo.

**Ordenar archivos.** *«Pon todos los archivos `.yml` de la carpeta del servidor en una carpeta `configs`.»* El asistente lista la carpeta, crea `configs` (sin aprobación) y propone un movimiento por archivo. Cada movimiento tiene su propia casilla: se desmarcan los archivos que deban quedarse donde están y se pulsa **Aprobar**. Con muchos archivos puede trabajar en varios turnos y volver a preguntar.

**Diagnosticar.** *«¿Por qué se para mi servidor justo después de arrancar?»* El asistente mira la consola y las caídas que ya tiene y luego lee o busca los archivos que necesita (por ejemplo `logs/latest.log`, la configuración de un plugin o una búsqueda de `ERROR` en la carpeta `logs`). Leer no necesita aprobación, así que la explicación llega directamente. Si propone corregir un archivo, pregunta antes de aplicarlo.

## Cómo funciona

1. Se hace una pregunta al asistente (botón **Asistente** en la cabecera del servidor).
2. El panel envía a Google Gemini la conversación y el contexto del servidor junto con la lista de herramientas de archivos que permiten los permisos de quien pregunta. Gemini responde con texto o con peticiones para usar herramientas.
3. El panel ejecuta en el nodo del servidor las herramientas que solo leen o crean y devuelve los resultados a Gemini. Esto puede ir y venir varias veces dentro de una misma pregunta.
4. Cuando Gemini quiere cambiar o borrar algo, el panel se detiene y muestra la tarjeta de confirmación.
5. Tras la respuesta, el panel ejecuta lo aprobado (comprobándolo antes otra vez) y Gemini continúa hasta tener una respuesta final.

Cada ruta es relativa a la carpeta del servidor: el asistente no tiene forma de nombrar otro servidor ni una carpeta de la máquina.

### Lo que muestra la respuesta

Bajo cada respuesta, una lista compacta, **Lo que hizo el asistente**, muestra cada operación: un icono según su tipo (leer, crear, cambiar, borrar), una frase breve como *Read server.properties (1.2 KB)* o *Moved paper.yml to configs/paper.yml*, las rutas implicadas y un estado:

| Estado | Significado |
|---|---|
| **Hecho** | Se ejecutó |
| **Falló** | Se intentó y falló; el motivo aparece debajo (por ejemplo, el archivo cambió o no existe) |
| **En espera** | Está en la tarjeta de confirmación |
| **Rechazado** | No se aprobó, así que no se ejecutó |
| **No permitido** | Los permisos no lo permiten, o superaba un límite; no se ejecutó |

Cada paso se muestra en el idioma de la interfaz («Leer server.properties», «Mover paper.yml a config/paper.yml»); en inglés el panel muestra la frase más detallada que escribió la herramienta (por ejemplo, el tamaño leído o el número de sustituciones). Los mensajes de error de los pasos fallidos están en inglés. La respuesta del asistente sigue el idioma de quien pregunta.

### La tarjeta de confirmación

Cuando el asistente quiere cambiar o borrar algo, aparece bajo su respuesta una tarjeta titulada **El asistente quiere:**

- Una fila por operación, con su frase y sus rutas. Los borrados se marcan en rojo con **Borra o reemplaza datos**.
- Todas las filas empiezan marcadas. Se desmarca lo que no se quiera.
- **Aprobar** ejecuta las operaciones marcadas y rechaza el resto (está desactivado si no hay ninguna marcada). **Rechazar todo** no ejecuta ninguna.
- Mientras no se responde, el cuadro de preguntas queda bloqueado (*Primero aprueba o rechaza los cambios de arriba*). La tarjeta se mantiene al cambiar de pestaña o recargar la página en la misma sesión del navegador.
- Cada operación se vuelve a comprobar justo antes de ejecutarse: si el archivo cambió entretanto y el cambio ya no encaja, ese paso falla en lugar de hacer algo distinto de lo que se vio.
- Tras la respuesta, el asistente continúa: explica el resultado y puede proponer más cambios en una tarjeta nueva.

Una solicitud es válida durante **10 minutos**, se puede responder **una vez**, y solo puede hacerlo **la misma persona** en **este servidor**. Preguntar otra cosa que necesite aprobación en el mismo servidor sustituye a la solicitud anterior. **Nueva conversación** quita la tarjeta del navegador.

### Dónde quedan registrados los cambios

- **Registro de actividad.** Cada cambio que hizo el asistente se registra en el [registro de actividad](./activity-and-access-log.md) del servidor a nombre de quien lo pidió, con la ruta (u `origen → destino`), por ejemplo *usó el asistente para editar el archivo `server.properties`*. Aparecen en el filtro **Archivos**. Las lecturas no se registran.
- **Historial de archivos.** Las ediciones y sobrescrituras guardan el texto anterior en el [historial de versiones](./file-history.md) del archivo con las reglas habituales (archivos de configuración y de texto de hasta 1 MiB). Los archivos nuevos, las copias, los movimientos y los borrados no tienen una versión a la que volver.
- Los borrados no se pueden deshacer desde el panel: conviene pedir al asistente que copie antes el archivo o la carpeta, o hacer una [copia de seguridad](./backups.md).

## Uso

### Desde el panel

**Administradores**

1. Configurar la clave de API de Gemini en **Configuración → General → Inteligencia artificial** (véase [Asistente de IA](./ai-assistant.md#desde-el-panel)).
2. **Dejar que el asistente trabaje con los archivos del servidor** está activado por defecto en la misma sección. Al desactivarlo, el asistente se limita a responder preguntas.

**Propietarios y subusuarios**

- Abrir **Asistente** en la cabecera del servidor y preguntar con palabras propias. Conviene nombrar los archivos con su ruta desde la carpeta del servidor cuando se conoce (`config/paper-global.yml`), o pedir al asistente que los busque.
- Leer la tarjeta de confirmación antes de aprobar: enumera exactamente los archivos que van a cambiar.

### Mediante la API

El modo agente usa el mismo `POST /api/servers/:serverId/ai/chat` que el chat simple; su respuesta añade `steps` y `pending`, y `POST /api/servers/:serverId/ai/chat/confirm` responde a una solicitud pendiente. Véase [Servidores → IA](../reference/api/03-servers.md#post-apiserversserveridaichat).

## Requisitos y configuración

| Requisito | Para qué |
|---|---|
| Una clave de API de Gemini (`panel.settings.geminiApiKey`) | Sin ella el asistente responde que no está configurado |
| **Dejar que el asistente trabaje con los archivos del servidor** activado (`panel.settings.aiFileTools`, por defecto `true`) | Desactivado, el asistente solo responde preguntas |
| `server.console` en el servidor | Para abrir el asistente |
| `server.files.view` en el servidor | Para que pueda mirar archivos; sin él el asistente solo responde preguntas |
| `server.files.edit` en el servidor | Para que pueda crear, cambiar, mover y borrar; sin él solo puede mirar |
| Nodos actualizados a la versión del panel | El asistente usa rutas del nodo añadidas con esta función |

| Clave | Valor por defecto | Variable de entorno | Uso |
|---|---|---|---|
| `panel.settings.aiFileTools` | `true` | `SKYPANEL_PANEL_SETTINGS_AIFILETOOLS` | Permite al asistente trabajar con los archivos del servidor en el que se abre, dentro de los permisos de archivos del usuario |

Los permisos se dan a cada subusuario en la pestaña **Usuarios** del servidor; véase [Subusuarios](../general/scopes-reference.md#subusuarios). El asistente usa los permisos de quien está conversando: nunca puede hacer más de lo que esa persona podría hacer en la pestaña **Archivos**.

## Privacidad

Cuando el asistente usa sus herramientas de archivos, **lo que lee se envía a Google Gemini**: el contenido de los archivos que lee, los nombres de las carpetas que lista, las líneas que encuentran sus búsquedas y las rutas implicadas, además de la conversación y el contexto del servidor. Esto solo ocurre cuando un administrador ha configurado una clave de Gemini, el ajuste está activado y alguien pregunta algo al asistente. Los archivos de configuración pueden contener contraseñas o tokens (credenciales de bases de datos, contraseñas de RCON): el asistente lee lo que decide que necesita, así que no conviene pedirle que trabaje con archivos que no se quieran compartir con Google, o bien hay que desactivar el ajuste. Véase el [aviso de privacidad](../general/privacy.md#servicios-externos-que-contacta-el-panel).

El panel no guarda nada más aparte de las entradas de actividad y las versiones de archivos indicadas arriba (y, con el nivel de log de depuración, las rutas que leyó el asistente); una solicitud pendiente de aprobación se guarda en la memoria del panel durante un máximo de 10 minutos.

## Límites

| Límite | Valor |
|---|---|
| Archivos que puede leer o editar | Archivos de texto de hasta 1 MiB; los binarios se rechazan |
| Contenido que puede escribir | Hasta 1 MiB por archivo en teoría; en la práctica mucho menos, porque cada respuesta de Gemini está limitada a 2048 tokens de salida |
| Listado de una carpeta | 1000 entradas |
| Búsqueda | 8 carpetas de profundidad, 5000 entradas revisadas, 200 coincidencias, archivos de hasta 1 MiB, consultas de hasta 200 caracteres |
| Una pregunta | Como mucho 10 rondas con Gemini, 8 operaciones por ronda, 30 operaciones en total y 120 segundos |
| Una solicitud pendiente de aprobación | 10 minutos; una por usuario y servidor; se pierde si el panel se reinicia |
| Límite de peticiones | 30 peticiones de IA por usuario cada 10 minutos (chat, aprobaciones y análisis de registros sumados); una pregunta o una aprobación cuentan una vez |

Cuando una pregunta necesita más que estos límites, el asistente se detiene y dice lo que hizo hasta ese momento.

Cada pregunta nueva envía los mensajes anteriores solo como texto: el asistente no recuerda el contenido de los archivos que leyó en respuestas anteriores y vuelve a leerlos cuando los necesita.

## Problemas conocidos y soluciones

| Mensaje o síntoma | Causa | Solución |
|---|---|---|
| *Esta solicitud ha caducado. Vuelve a preguntar.* (HTTP 409, `ErrAIConfirmationInvalid`) | La solicitud tiene más de 10 minutos, ya se respondió, la sustituyó otra más reciente en el mismo servidor, el panel se reinició o se desactivaron entretanto las herramientas de archivos | Volver a hacer la pregunta. |
| *Necesitas acceso a la consola de este servidor para usar el asistente.* (HTTP 403) | Falta `server.console`; al aprobar, ya no se tiene `server.files.view` en el servidor | Pedir el permiso al propietario. |
| El asistente dice que no puede ver ni cambiar archivos; no aparece la lista de pasos | **Dejar que el asistente trabaje con los archivos del servidor** está desactivado, o falta `server.files.view` | Activar el ajuste (administradores) o pedir el permiso. |
| Puede leer pero dice que no puede cambiar nada, o los pasos muestran **No permitido** | Falta `server.files.edit` | Pedir el permiso al propietario. |
| *La IA no está configurada…* (HTTP 400) | No hay clave de Gemini | Un administrador la añade en **Configuración → General → Inteligencia artificial**. |
| Los pasos fallan con *not found* en archivos que existen, o fallan las búsquedas y las copias | El nodo del servidor tiene una versión anterior sin las rutas de archivos del asistente (responden 404) | Actualizar el nodo. |
| *could not reach the node that hosts this server* | El nodo está apagado o el panel no llega a él | Revisar el nodo en la página **Nodos**. |
| *the node that hosts this server refused the panel's credentials…* | El secreto del nodo no coincide con el del panel, o un reloj va mal | Véase [Fallos de autenticación con el nodo](../reference/api/01-conventions.md#fallos-de-autenticación-con-el-nodo). |
| Un paso falla con *old_text was not found* o *appears N times* | El archivo cambió, o el texto que hay que sustituir no es único | Volver a preguntar; el asistente vuelve a leer antes el archivo. |
| *too large* o *is a binary file* | El archivo supera 1 MiB, o no es de texto | Editarlo en la pestaña **Archivos** o por SFTP. |
| *the server has used all of its disk space…* | Se ha alcanzado el límite de disco del servidor | Liberar espacio y volver a preguntar. |
| *No pude responder ahora mismo.* | Gemini falló o tardó demasiado | Volver a intentarlo. Los cambios hechos antes del fallo están en el registro de actividad. |

## Véase también

- [Asistente de IA y análisis de registros](./ai-assistant.md): el chat, su contexto, la clave y el límite de peticiones.
- [Kit de herramientas del agente de IA](../backend/14-ai-toolkit.md): arquitectura, herramientas, aislamiento y confirmaciones.
- [Historial de archivos](./file-history.md): comparar y restaurar archivos editados.
- [Registro de actividad](./activity-and-access-log.md): las entradas `ai.*`.
- [Configuración del panel](./panel-settings.md): la sección **Inteligencia artificial**.
