# Asistente de IA y análisis de registros

Aether Panel puede usar Google Gemini para ayudar a los propietarios a entender qué ocurre en su servidor. Hay tres puntos de entrada, y todos usan la misma clave de API de Gemini que un administrador configura en los ajustes del panel:

- **Asistente del servidor**: un chat sobre un servidor concreto que consulta su estado, los jugadores, el rendimiento del último día, las caídas recientes y la consola antes de responder. Con las herramientas de archivos activadas (por defecto) también puede leer y, con la aprobación del usuario, cambiar los archivos de ese servidor: véase [Asistente de IA: trabajar con archivos](./ai-agent.md).
- **Análisis de consola por IA**: una tarjeta bajo la consola que envía las líneas de error y aviso seleccionadas y devuelve un resumen, las causas probables y sugerencias.
- **Explicar con IA** en un informe de caída: el mismo análisis aplicado a las líneas de consola guardadas con la caída, junto con su código de salida.

La función es opcional. Sin clave de Gemini el panel funciona con normalidad y los controles de IA responden con un mensaje de «no está configurada».

## Qué hace

| Punto de entrada | Dónde | Qué se envía a Gemini | Qué se recibe |
|------------------|-------|-----------------------|---------------|
| Asistente del servidor | Botón **Asistente** en la cabecera del servidor | La conversación (hasta 20 mensajes) más un bloque de contexto que el panel construye a partir del servidor (ver más abajo) | Una respuesta breve en Markdown y la lista de fuentes que el panel consiguió reunir |
| Análisis de consola por IA | Pestaña **Consola**, tarjeta bajo el terminal | Las líneas de consola marcadas (líneas de error o aviso detectadas entre las 50 últimas mostradas) | `summary`, `rootCauses[]`, `suggestions[]` |
| Explicar con IA | Diálogo **Informe de caída** (desde el aviso de caída o el historial de caídas de la pestaña **Resumen**) | Las líneas de consola guardadas con la caída y su código de salida | `summary`, `rootCauses[]`, `suggestions[]` |

Las respuestas se redactan en el idioma de la interfaz: en español si la interfaz está en español y en inglés en cualquier otro caso.

### Contexto que reúne el asistente

Con cada mensaje del chat, el panel pide estos datos al nodo del servidor. Cada uno se obtiene «si se puede»: el que falla o tarda más de 6 segundos se omite y el resto se envía igualmente. Los nombres de los datos obtenidos se devuelven en `sources` y se muestran bajo la respuesta («Revisó: …»).

| Fuente | Endpoint del daemon | Contenido que se añade al prompt |
|--------|---------------------|----------------------------------|
| `status` | `/daemon/server/:id/stats` | En línea o detenido, % de CPU, memoria usada y su límite |
| `players` | `/daemon/server/:id/query` | Jugadores conectados / máximo, versión y mapa, por protocolo de consulta |
| `performance` | `/daemon/server/:id/metrics?range=24h` | Últimas 24 h: CPU media y máxima, % del tiempo en línea, memoria media respecto al límite, máximo de jugadores simultáneos |
| `crashes` | `/daemon/server/:id/crashes` | Las 3 caídas más recientes como máximo (hora, código de salida, si se reinició automáticamente) y las 20 últimas líneas de consola antes de la más reciente |
| `console` | `/daemon/server/:id/console` | Las 60 últimas líneas no vacías de la consola, sin colores ANSI y recortadas a 300 caracteres cada una |

El nombre y el tipo del servidor se incluyen siempre. Este contexto no lee contraseñas, variables de entorno ni archivos. En modo agente (abajo) el asistente también puede leer los archivos del servidor cuando los necesita, mediante sus herramientas.

### Modo agente

Cuando **Dejar que el asistente trabaje con los archivos del servidor** está activado (`panel.settings.aiFileTools`, por defecto `true`), el usuario tiene sesión iniciada y tiene `server.files.view` en el servidor, el chat funciona en **modo agente**: Gemini recibe herramientas de archivos para ese servidor (listar, leer, buscar; con `server.files.edit` también crear, copiar, editar, sobrescribir, mover, renombrar y borrar). Las lecturas y las creaciones se ejecutan en el acto; los cambios y los borrados esperan la aprobación del usuario en una tarjeta de confirmación. Cada respuesta enumera debajo las operaciones realizadas. En otro caso el chat responde solo a partir del contexto, como se describe en esta página. La guía de uso es [Asistente de IA: trabajar con archivos](./ai-agent.md); la arquitectura está en [Kit de herramientas del agente de IA](../backend/14-ai-toolkit.md).

## Cómo funciona

1. El navegador llama al panel (`/api/servers/:serverId/ai/chat` o `/api/servers/:serverId/ai/analyze`). Se comprueba el permiso `server.console` sobre ese servidor.
2. El panel comprueba que hay una clave de Gemini configurada, valida la petición y aplica el límite de peticiones por usuario.
3. En el chat, el panel reúne el contexto anterior desde el nodo. En el análisis, conserva las líneas más recientes que caben en el tamaño máximo.
4. El panel llama a `https://generativelanguage.googleapis.com/v1beta/models/gemini-3.8-flash:generateContent` desde la máquina del panel. La clave viaja en la cabecera `x-goog-api-key`, nunca en la URL.
5. La respuesta vuelve al navegador. El historial del chat vive en el `sessionStorage` del navegador (los 20 últimos mensajes por servidor) y desaparece al cerrar la sesión de la pestaña o al pulsar **Nueva conversación**. El panel no guarda ninguna conversación; en modo agente solo mantiene en memoria una solicitud pendiente de aprobación (10 minutos como máximo) y registra en el registro de actividad los cambios hechos.

Seguridad del prompt: las líneas de consola y el contexto del servidor son texto no fiable escrito por el juego y sus jugadores. El panel los encierra entre `<logs>…</logs>` o `<server-context>…</server-context>` e indica al modelo que los trate solo como datos. También se le indica que nunca sugiera borrar mundos o archivos sin una copia de seguridad previa y que nunca pida contraseñas ni claves.

Parámetros del modelo: el chat usa temperatura 0,4 y un máximo de 4096 tokens de salida, de ellos hasta 512 de razonamiento (8192 y 1024 en modo agente). El análisis pide una respuesta JSON con el esquema fijo `{ summary, suggestions[], rootCauses[] }`.

Las peticiones de IA en sí no se registran en el registro de actividad del servidor. En modo agente, cada cambio que el asistente hace en los archivos se registra como `ai.<herramienta>` (por ejemplo `ai.edit_file`), véase [Registro de actividad](./activity-and-access-log.md).

## Uso

### Desde el panel

**Configuración del administrador**

1. Obtener una clave de API de Gemini en Google AI Studio.
2. Abrir **Configuración → General**, sección **Inteligencia artificial**.
3. Pegarla en **Clave API de Gemini**. El valor se guarda en el archivo de configuración del panel.
4. Opcional: activar **Ocultar análisis IA de la consola** para desactivar el análisis de registros (la tarjeta bajo la consola y **Explicar con IA**) para todos los usuarios. El chat del **Asistente** sigue disponible.

**Propietarios y subusuarios**

- **Asistente**: pulsar **Asistente** en la cabecera del servidor. Elegir una de las preguntas sugeridas o escribir otra (hasta 2000 caracteres). **Intro** envía y **Mayús + Intro** añade un salto de línea. **Nueva conversación** borra el historial.
- **Análisis de consola por IA**: en la pestaña **Consola**, la tarjeta lista los errores y avisos detectados (líneas que contienen `error`, `warn`, `fail`, `exception`, `timeout`, `denied` o `critical`). Todas aparecen marcadas; se desmarcan las que no se quieran enviar y se pulsa **Analizar registros**.
- **Explicar con IA**: abrir un informe de caída y pulsar **Explicar con IA**. La respuesta muestra **Causas probables** y **Cómo solucionarlo**.

El botón **Asistente** solo aparece a quien tiene `server.console` en el servidor.

### Mediante la API

| Método | Ruta | Permiso | Uso |
|--------|------|---------|-----|
| `POST` | `/api/servers/:serverId/ai/chat` | `server.console` | Conversar con el asistente del servidor |
| `POST` | `/api/servers/:serverId/ai/chat/confirm` | `server.console` | Aprobar o rechazar los cambios que espera el asistente (modo agente) |
| `POST` | `/api/servers/:serverId/ai/analyze` | `server.console` | Analizar líneas de consola (opcionalmente de una caída). Responde `403` `ErrAIAnalysisDisabled` si **Ocultar análisis IA de la consola** está activo |

Petición y respuesta del chat:

```json
// POST /api/servers/:serverId/ai/chat
{
  "language": "es",
  "messages": [
    { "role": "user", "content": "¿Por qué va lento mi servidor?" }
  ]
}

// 200
{ "reply": "**La CPU es el cuello de botella** …", "sources": ["status", "performance", "console"] }
```

Reglas: de 1 a 20 mensajes; `role` es `user` o `assistant`; cada `content` tiene de 1 a 2000 caracteres; el último mensaje debe ser del usuario.

En modo agente la respuesta incluye además `steps` (las operaciones ejecutadas) y `pending` (los cambios pendientes de aprobación, o `null`). Las dos formas y la ruta de confirmación están en [Servidores → IA](../reference/api/03-servers.md#post-apiserversserveridaichat).

Petición y respuesta del análisis:

```json
// POST /api/servers/:serverId/ai/analyze
{ "logs": ["[ERROR] ...", "[WARN] ..."], "language": "es", "exitCode": 137 }

// 200
{ "summary": "...", "rootCauses": ["..."], "suggestions": ["..."] }
```

`logs` es obligatorio; `exitCode` es opcional y cambia el prompt para describir una caída. El formato común de errores está en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

- Salida HTTPS desde la máquina del **panel** hacia `generativelanguage.googleapis.com`. Los nodos no contactan con Google.
- Una clave de API de Google Gemini (la facturación de esa cuenta de Google corre a cargo del administrador).

| Clave | Valor por defecto | Variable de entorno | Uso |
|-------|-------------------|---------------------|-----|
| `panel.settings.geminiApiKey` | vacío | `SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY` | Clave de API de Gemini. Vacía desactiva todas las llamadas de IA. |
| `panel.settings.hideAiAnalysis` | `false` | `SKYPANEL_PANEL_SETTINGS_HIDEAIANALYSIS` | Desactiva el análisis de registros para todos: oculta la tarjeta **Análisis de consola por IA** y **Explicar con IA**, y `/ai/analyze` responde `403`. Se publica en `GET /api/config` como `hideAiAnalysis`. |
| `panel.settings.aiFileTools` | `true` | `SKYPANEL_PANEL_SETTINGS_AIFILETOOLS` | Permite al asistente trabajar con los archivos del servidor en el que se abre (modo agente), dentro de los permisos de archivos del usuario. Desactivado, el asistente solo responde preguntas. |

Las tres claves se cambian desde **Configuración → General** (`settings.edit`). La relación entre claves, archivo de configuración y variables de entorno se explica en [Configuración](../backend/02-configuration.md).

Privacidad: las líneas de consola, el contexto del servidor y las preguntas se envían a Google y, en modo agente, también el contenido de los archivos que lee el asistente. Conviene informar a los usuarios si las condiciones del servicio lo exigen.

## Límites

| Límite | Valor |
|--------|-------|
| Límite de peticiones | 30 peticiones de IA por usuario cada 10 minutos, chat, aprobaciones y análisis sumados (en memoria; se reinicia al reiniciar el panel) |
| Historial enviado en el chat | Hasta 20 mensajes de hasta 2000 caracteres cada uno |
| Entrada del análisis | Las 200 líneas más recientes, recortadas además a 32 KiB |
| Líneas de consola en el contexto del chat | 60 (300 caracteres cada una) |
| Detalle de caídas en el contexto del chat | Las 3 más recientes; 20 líneas de consola de la última |
| Tiempos de espera | Análisis 60 s; chat 90 s en total y 6 s por cada dato de contexto; en modo agente, 120 s por pregunta o aprobación |
| Modelo | `gemini-3.8-flash` (fijo en el código, no configurable) |

Otros aspectos a tener en cuenta:

- **Ocultar análisis IA de la consola** se aplica a todos los usuarios (la web lo lee de la configuración pública, `GET /api/config`) y también en el servidor: `/ai/analyze` lo rechaza. No afecta al botón **Asistente**, que sigue disponible mientras haya clave.
- Las peticiones hechas con un cliente OAuth2 (sin usuario asociado) comparten un único contador del límite.

## Fallos del chat

Cuando una pregunta no se puede responder, el chat lo dice dos veces: con un aviso emergente y con una tarjeta roja **No se pudo completar lo que pediste** que se queda en la conversación. La tarjeta explica lo que pasó con palabras sencillas, enumera los pasos hechos antes de detenerse (modo agente) y avisa cuando se hicieron cambios, que se mantienen y están en el registro de actividad. La última pregunta fallida tiene un botón **Reintentar** que la vuelve a hacer; no aparece cuando volver a preguntar no puede servir (clave, ajuste, permiso o la clave sin cuota).

Antes de rendirse, el panel intenta cada llamada a Gemini hasta tres veces, con 1 s y luego 3 s de espera, mientras Gemini esté saturado (5xx) o no se alcance, y tras un `429` que pida esperar 10 s o menos.

| Texto de la tarjeta (inicio) | `reason` | Causa | Qué hacer |
|---|---|---|---|
| *Se perdió la conexión con el panel…* / *No hay conexión a internet…* | ninguno (sin respuesta) | El navegador perdió la conexión antes de llegar la respuesta | Comprobar la conexión y reintentar. |
| *El panel no respondió (código 502/503/504)…* | ninguno | Respondió un proxy delante del panel: el panel se está reiniciando, actualizando o está caído | Esperar y reintentar. |
| *Gemini rechazó la clave de API…* | `keyRejected` | La clave no es válida, está revocada, restringida a otras API o IP, o la Generative Language API no está activada | Un administrador revisa la clave en Google AI Studio y en **Configuración → General**. |
| *La clave de Gemini llegó a su límite de uso…* | `quota` | Se agotaron las peticiones por minuto o por día del plan de la clave (el nivel gratuito es bajo) | Esperar o subir el plan de la clave. |
| *Gemini está saturado o no disponible ahora mismo…* | `unavailable` | Gemini respondió 5xx tres veces | Reintentar en un momento. |
| *El panel no pudo conectar con Gemini…* | `unreachable` | La máquina del panel no llega a Google (cortafuegos, DNS, proxy) | Permitir la salida HTTPS hacia `generativelanguage.googleapis.com`. |
| *Gemini rechazó la petición…* | `rejected` | Gemini rechazó la petición por otro motivo (por ejemplo, un modelo desconocido) | Leer la línea del log del panel `The AI service did not accept the assistant's request (…)`. |
| *La respuesta no cabía en el límite de Gemini…* | `tooLong` | La respuesta (por ejemplo, un archivo grande que escribir) agotó los 8192 tokens de salida | Pedir la tarea por partes, un archivo cada vez. |
| *Gemini se negó a responder por sus filtros de seguridad…* | `blocked` | Los filtros de seguridad de Google pararon la respuesta | Reformular la petición. |
| *Gemini respondió sin contenido…*, *…no se pudo leer*, *Gemini intentó una operación que no se pudo leer…* | `empty`, `unreadable`, `malformed` | El modelo no dio una respuesta utilizable | Reintentar o decirlo con otras palabras. |
| *El asistente tardó demasiado y se detuvo…* | `timeout` | La tarea superó su tiempo máximo (120 s en modo agente, 90 s en el chat simple) | Pedir la tarea por partes. |
| *El asistente falló por un error interno…* | `failed` | Algo inesperado; la causa está en el log del panel | Leer la línea del log `The server assistant failed: …`. |

El log del panel siempre tiene la causa exacta de un rechazo de Gemini, con el estado y el mensaje de Google, por ejemplo `The AI service did not accept the assistant's request (status 429 RESOURCE_EXHAUSTED: Quota exceeded…)`. La clave nunca aparece en él.

## Problemas conocidos y soluciones

| Mensaje | Causa | Solución |
|---------|-------|----------|
| `Gemini API Key is not configured. Please add it in Settings.` (interfaz: «La IA no está configurada…») | `panel.settings.geminiApiKey` está vacía | Añadir la clave en **Configuración → General → Inteligencia artificial**. |
| `too many AI requests, try again in a few minutes` (HTTP 429; interfaz: «Has hecho muchas preguntas seguidas…») | Más de 30 peticiones en 10 minutos del mismo usuario | Esperar; la ventana es deslizante y se liberan peticiones a medida que caducan las antiguas. |
| `the AI log analysis is turned off in the panel settings` (HTTP 403, `ErrAIAnalysisDisabled`) | Un administrador activó **Ocultar análisis IA de la consola** | Desactivarlo en **Configuración → General → Inteligencia artificial**, si se quiere el análisis. |
| HTTP 403 (interfaz: «Necesitas acceso a la consola de este servidor para usar el asistente.») | El usuario no tiene `server.console` en ese servidor | Conceder el permiso de consola al subusuario. |
| `a conversation has between 1 and 20 messages`, `each message has between 1 and 2000 characters`, `the last message must be the user's question`, `messages are from the user or the assistant` | Petición de chat no válida (clientes de la API) | Corregir el cuerpo de la petición. La interfaz ya recorta el historial a 20 mensajes. |
| `No logs provided for analysis` / `Invalid payload` | Petición de análisis vacía o mal formada | Enviar un array `logs` no vacío. |
| Chat: una tarjeta roja *No se pudo completar lo que pediste* | Véase [Fallos del chat](#fallos-del-chat) | Seguir lo que dice la tarjeta. |
| Análisis de registros: `Failed to connect to AI API` | La máquina del panel no llega a Google (cortafuegos, DNS, proxy) | Permitir la salida HTTPS hacia `generativelanguage.googleapis.com`. |
| Análisis de registros: `AI API returned status N` | Google rechazó la llamada: clave no válida o revocada (400/403), cuota agotada (429), error del servicio (5xx) | Revisar la clave y su cuota en Google AI Studio. |
| Análisis de registros: `AI returned invalid JSON structure`, `Empty response from AI` | El modelo no produjo una respuesta utilizable (por ejemplo, bloqueada por los filtros de seguridad de Google) | Reintentar, o enviar menos líneas o líneas distintas. |
| `Esta solicitud ha caducado. Vuelve a preguntar.` (HTTP 409, `ErrAIConfirmationInvalid`) | Un cambio pendiente de aprobación caducó (10 minutos), ya se respondió, se sustituyó o el panel se reinició | Volver a preguntar. Véase [Asistente de IA: trabajar con archivos](./ai-agent.md#problemas-conocidos-y-soluciones). |
| La respuesta dice que no tiene información sobre algo | No se pudo obtener ese dato de contexto (nodo caído, consulta desactivada, todavía sin métricas) | Consultar «Revisó: …» bajo la respuesta para ver qué fuentes estaban disponibles. |

## Véase también

- [Asistente de IA: trabajar con archivos](./ai-agent.md): modo agente, aprobaciones y límites.
- [Informes de caída](./crash-reports.md): cómo se detectan y guardan las caídas.
- [Referencia de permisos](../general/scopes-reference.md): `server.console` y el resto de permisos de servidor.
- [Configuración](../backend/02-configuration.md): todas las claves de configuración.
- [Referencia de la API](../reference/11-api-reference.md): autenticación y formato de errores.
