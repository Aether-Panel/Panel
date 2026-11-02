# Detección de caídas e informes de caída

Cuando un servidor de juego se detiene solo con un código de salida inesperado, Aether Panel lo trata como una caída: guarda un informe con las últimas líneas de la consola, avisa a quien tenga el servidor abierto, envía las notificaciones configuradas y, si está activado, reinicia el servidor automáticamente hasta un límite. El informe puede abrirse después y explicarse con la IA.

## Qué hace

- Detecta las caídas en el nodo en cuanto termina el proceso del servidor.
- Escribe `Server crashed (exit code <n>)` en la consola.
- Guarda un **informe de caída**: hora, código de salida, las últimas 60 líneas no vacías de la consola, si el servidor se reinició automáticamente y qué intento consecutivo fue.
- Conserva los **10 últimos informes** de cada servidor, del más reciente al más antiguo.
- Envía un evento `crash` por el WebSocket del servidor, de modo que la página abierta del servidor muestra un aviso al instante.
- Envía la caída a los canales de notificación propios del servidor, cuando el evento de **caída** está activado en ellos (véase [Notificaciones](server-notifications.md)), y al webhook de Discord general del panel (véase [Discord](../general/16-discord.md)).
- Opcionalmente **reinicia el servidor** de forma automática (**Reiniciar en caso de fallo**), con un límite de reinicios consecutivos.

## Cómo funciona

### Qué se considera una caída

Cuando el proceso del servidor termina, el nodo compara el código de salida con el código de salida esperado del servidor (`expectedExitCode` en la definición del servidor, `0` si no se indica):

| Situación | Resultado |
|---|---|
| Alguien detuvo o forzó el cierre del servidor (botón del panel, API, paso `stop` de una tarea) | No es una caída, sea cual sea el código de salida. |
| El proceso termina solo con el código esperado (por ejemplo, `stop` escrito en la consola) | Parada normal. No es una caída. Si **Reiniciar al detener** (`autoRestartOnGraceful`) está activado, el servidor se vuelve a iniciar. |
| El proceso termina con cualquier otro código sin que se le haya pedido | **Caída.** |

### Reinicio automático

- Lo controla el indicador del servidor **Reiniciar en caso de fallo** (`autoRestartOnCrash` en la API, guardado como `autorecover` en la definición del servidor). Está desactivado por defecto, salvo que la plantilla del servidor lo active.
- Tras una caída, el nodo reinicia el servidor solo mientras el contador de caídas sea menor que `daemon.data.crashLimit` (por defecto **3**). Cada reinicio automático incrementa el contador.
- El contador vuelve a cero cuando el servidor termina sin caerse o cuando llevaba **más de 10 minutos** en marcha antes de la caída. Así, caídas separadas por una semana no agotan los reinicios automáticos.
- El reinicio se produce después de los pasos posteriores a la ejecución del servidor. Si esos pasos fallan, el servidor no se reinicia.
- Cuando se alcanza el límite, el servidor queda detenido y el informe indica que no se reinició.

### Dónde se guardan los informes

Los informes se guardan en el nodo en `<daemon.data.servers>/<serverId>.crashes` (JSON, permisos `0600`, porque la salida de la consola puede contener nombres de jugadores e IP). Cada línea se recorta a 500 caracteres. Solo se conservan los 10 informes más recientes.

## Uso

### Desde el panel

- **Aviso emergente.** Con la página del servidor abierta, una caída muestra el aviso "El servidor se ha caído (código `<n>`)." junto con lo que suele significar el código de salida y si el servidor se reinició. En un reinicio automático, el aviso lee el informe guardado para mostrar el número de intento real y el límite (los mismos que el diálogo); si no puede leerlo (por ejemplo, sin `server.console`), solo dice «Reiniciado».
- **Banner.** En la pestaña **Consola**, la última caída aparece como un banner ("El servidor se cayó a las `<hora>`") durante 24 horas, con **Ver informe** y **Ocultar aviso**. El navegador recuerda que se ocultó.
- **Historial de caídas.** La pestaña **Resumen** incluye la tarjeta **Historial de caídas** con los últimos informes. El gráfico de historial de rendimiento de esa misma pestaña marca las caídas en su línea de tiempo.
- Diálogo **Informe de caída**: **Código de salida**, **Qué significa**, **Recuperación** (se reinició solo, intento N de M, o no se reinició) y **Últimas líneas de la consola**, con las líneas que parecen errores resaltadas.
- **Explicar con IA.** En el diálogo, la sección **Explicación con IA** envía las líneas y el código de salida del informe a la IA y muestra un resumen, las causas probables y **Cómo solucionarlo**. Requiere la clave de Gemini (véase más abajo) y no aparece cuando un administrador activa **Ocultar análisis IA de la consola**. Véase también el [asistente de IA](ai-assistant.md).
- **Activar el reinicio automático.** Pestaña **Configuración** → **Reiniciar en caso de fallo** ("Reiniciar el servidor cuando falle."). Requiere `server.flags.edit`.

El panel muestra estos significados para los códigos de salida más habituales:

| Código | Significado mostrado |
|---|---|
| `1` | El juego terminó con un error |
| `134` | El proceso abortó (SIGABRT) |
| `137` | Sin memoria o cerrado a la fuerza (SIGKILL) |
| `139` | Fallo de memoria del programa (segfault) |
| `143` | Terminado por el sistema (SIGTERM) |
| otro | Salida inesperada |

### Mediante la API

Los detalles completos están en la [referencia de la API](../reference/11-api-reference.md).

| Método | Ruta | Permiso | Propósito |
|---|---|---|---|
| `GET` | `/api/servers/:serverId/crashes` | `server.console` | Los informes guardados, del más reciente al más antiguo. |
| `POST` | `/api/servers/:serverId/ai/analyze` | `server.console` | Explica líneas de registro con la IA (lo usa **Explicar con IA**). |
| `GET` | `/api/servers/:serverId/flags` | `server.flags.view` | Lee `autoStart`, `autoRestartOnCrash` y `autoRestartOnGraceful`. |
| `POST` | `/api/servers/:serverId/flags` | `server.flags.edit` | Cambia esos indicadores, por ejemplo `{"autoRestartOnCrash": true}`. |

Un informe tal como lo devuelve `GET /crashes`:

```json
{
  "id": "20261030T182241.123456789",
  "time": "2026-10-30T18:22:41.123456789Z",
  "exitCode": 137,
  "lines": ["[18:22:39] [Server thread/INFO]: ...", "..."],
  "restarted": true,
  "attempt": 1,
  "limit": 3
}
```

El evento que se envía por WebSocket a `/api/servers/:serverId/socket` tiene `"type": "crash"` y los datos `{"id", "exitCode", "restarted"}`.

## Requisitos y configuración

| Clave | Por defecto | Variable de entorno | Propósito |
|---|---|---|---|
| `daemon.data.crashLimit` | `3` | `SKYPANEL_DAEMON_DATA_CRASHLIMIT` | Número máximo de reinicios automáticos seguidos tras caídas. |
| `daemon.data.servers` | `servers` (dentro de la raíz de datos) | `SKYPANEL_DAEMON_DATA_SERVERS` | Carpeta donde se guardan los archivos `.crashes`. |
| `panel.settings.geminiApiKey` | vacío | `SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY` | Clave de Google Gemini que necesita **Explicar con IA**. |

La detección de caídas está siempre activa. Consulte [Configuración](../backend/02-configuration.md) para saber cómo se definen las claves y las variables de entorno.

## Límites

- 10 informes por servidor; 60 líneas de consola por informe; 500 caracteres por línea.
- Un informe solo contiene lo que seguía en el búfer de la consola cuando el servidor cayó. Si el juego escribió los detalles de la caída en un archivo (por ejemplo, un archivo de `crash-reports/` de Minecraft) y no en la consola, ábralo desde el gestor de archivos.
- El contador de reinicios automáticos se guarda en memoria en el nodo; reiniciar el daemon lo pone a cero.
- Que el banner se ha ocultado se guarda en el almacenamiento local del navegador, por lo que vuelve a aparecer en otro navegador o dispositivo.

## Problemas conocidos y soluciones

| Síntoma o mensaje | Causa y solución |
|---|---|
| El servidor se cae varias veces y después queda apagado | Ha agotado los `daemon.data.crashLimit` reinicios automáticos, cada uno en menos de 10 minutos. Lea el informe, corrija la causa e inicie el servidor. |
| Código de salida `137` | El contenedor se quedó sin memoria o se cerró a la fuerza. Aumente la memoria del servidor o reduzca lo que carga (mods, distancia de visión). |
| Una parada desde el panel no aparece como caída | Es lo esperado: las paradas y cierres forzados solicitados nunca son caídas. |
| Un servidor que termina con un código distinto de cero en una parada normal aparece como caído | Indique en `expectedExitCode`, dentro de la definición del servidor, el código que usa el juego al cerrarse correctamente. |
| El **Historial de caídas** aparece vacío para algunos usuarios | `GET /crashes` exige `server.console`; quien no lo tiene no ve informes. |
| "La IA no está configurada. Un administrador puede añadir la clave de Gemini en Configuración." | `panel.settings.geminiApiKey` está vacío. La API devuelve `Gemini API Key is not configured. Please add it in Settings.` |
| "No se pudo obtener la explicación. Inténtalo de nuevo en un momento." | Falló la llamada a Gemini (red, cuota o tiempo de espera). Inténtelo más tarde. |
| El reinicio automático no se produjo aunque está activado | Se alcanzó el límite, o fallaron los pasos posteriores a la ejecución del servidor (la consola muestra `Failed to run post-execution steps`). |

## Véase también

- [Notificaciones](server-notifications.md)
- [Asistente de IA](ai-assistant.md)
- [Discord](../general/16-discord.md)
- [Servidores: ciclo de vida](../backend/04-servers.md)
- [Referencia de la API](../reference/11-api-reference.md)
- [Configuración](../backend/02-configuration.md)
