# Notificaciones del servidor

Cada servidor puede enviar sus propios avisos a los lugares que su propietario ya utiliza: un canal de Discord, un chat de Telegram o cualquier sistema que acepte un webhook JSON. Estos avisos se configuran por servidor, por quien lo gestiona, y son independientes de los avisos globales para administradores descritos en [Webhooks de Discord](../general/16-discord.md) y [Correo electrónico](../general/15-email.md).

## Qué hace

- Envía un aviso cuando el servidor **se cae**, cuando una **copia de seguridad** termina o falla, cuando **la CPU o la memoria se mantienen altas** y, opcionalmente, cada vez que el servidor **arranca o se detiene**.
- Entrega cada aviso a todos los canales configurados: Discord, Telegram y un webhook genérico pueden usarse a la vez.
- Incluye las últimas líneas de la consola en los avisos de caída.
- Ofrece un botón de **prueba** que publica un mensaje de ejemplo en cada canal e indica cuáles fallaron.

Un servidor nuevo no tiene ningún canal configurado. Los eventos **Caídas**, **Copias de seguridad** y **Uso alto de recursos** vienen activados por defecto, de modo que empiezan a funcionar en cuanto se añade un canal; **Encendido y apagado** viene desactivado.

## Cómo funciona

### Dónde se guarda la configuración

La configuración se guarda en el nodo que ejecuta el servidor, en `<daemon.data.servers>/<serverId>.notify` (permisos `0600`). Se mantiene fuera de la definición del servidor a propósito: las URL de webhook y los tokens de bot son secretos, y la definición la pueden leer más usuarios. El archivo se elimina junto con el servidor.

Como es el nodo quien guarda la configuración y envía los avisos, **el nodo necesita salida a internet** hacia Discord, Telegram o el host del webhook.

### Cuándo se dispara cada evento

| Evento | Condición | Contenido |
|--------|-----------|-----------|
| Caída (`crash`) | El proceso terminó con un código distinto del código de salida esperado de la plantilla y nadie pidió detenerlo ni forzar su cierre. | Código de salida, si el servidor se está reiniciando solo (intento *n* de `daemon.data.crashLimit`) y las últimas 12 líneas de la consola. |
| Copia de seguridad (`backup`) | Termina una copia de seguridad, con éxito o no. | Éxito o fallo. |
| Uso alto de recursos (`resources`) | Se comprueba cada 5 segundos mientras el servidor está en marcha: CPU por encima del **80 %** de lo que el servidor puede usar, o memoria por encima del **90 %**. | Recurso, valor actual y umbral. Como máximo un aviso cada **5 minutos** por recurso mientras siga alto; el contador se reinicia cuando el uso baja del umbral. |
| Encendido y apagado (`power`) | El estado del servidor cambió entre dos muestras de 5 segundos. | En línea o detenido. También se dispara con las paradas solicitadas por alguien. |

La primera muestra tras arrancar el daemon solo registra el estado actual, así que reiniciar el daemon no envía avisos. Un arranque y parada muy breve, ocurrido entre dos muestras, puede pasar desapercibido para el evento de encendido.

Los avisos se envían en segundo plano: un canal lento o inaccesible nunca retrasa al servidor. Los errores de entrega se escriben en el registro del servidor (`Error sending <canal> notification: ...`).

### Canales

| Canal | Qué se envía |
|-------|--------------|
| Discord | Un *embed* con título, descripción (nombre del servidor en negrita, líneas de consola en un bloque de código, recortadas a los últimos 3.500 caracteres), un color por evento y el pie `Aether Panel · <serverId>`. |
| Telegram | Un mensaje con formato HTML a través del bot (`sendMessage`, sin vista previa de enlaces). Las líneas de consola se recortan a los últimos 3.000 caracteres. |
| Webhook | Un `POST` JSON (ver más abajo), con `User-Agent: AetherPanel-Webhook/1`. |

Los títulos y mensajes de los avisos los genera el nodo y actualmente están **siempre en español** (por ejemplo `💥 Caída del servidor`), sea cual sea el idioma de la interfaz del panel.

### Contenido del webhook

```json
{
  "event": "crash",
  "title": "💥 Caída del servidor",
  "message": "Survival se ha caído (código de salida 137). Se está reiniciando solo (intento 1 de 3).",
  "server": { "id": "a1b2c3d4", "name": "Survival" },
  "timestamp": "2026-11-01T10:15:00Z",
  "lines": ["...últimas líneas de la consola..."],
  "exitCode": 137,
  "restarted": true
}
```

`event` vale `crash`, `backup`, `resources`, `power` o `test`. `lines` solo aparece cuando hay líneas de consola. Campos adicionales por evento:

| Evento | Campos adicionales |
|--------|--------------------|
| `crash` | `exitCode`, `restarted` |
| `backup` | `success` |
| `resources` | `resource` (`CPU` o `Memoria`), `value`, `threshold` |
| `power` | `running` |

**Firma.** Si se define un secreto para firmar, cada petición lleva la cabecera `X-Aether-Signature: sha256=<hex>`, el HMAC-SHA256 del cuerpo exacto de la petición con el secreto como clave. El receptor debe calcular el mismo HMAC y compararlo antes de confiar en la petición.

### Extensiones

En un nodo que también ejecuta el panel, los mismos eventos se entregan a las extensiones instaladas (`server.crashed`, `server.backup`, `server.resources`, `server.started`, `server.stopped`), independientemente de la configuración de notificaciones del propietario. Véase [Extensiones](../reference/15-extensions.md).

## Uso

### Desde el panel

1. Abrir el servidor e ir a la pestaña **Configuración**. La tarjeta **Notificaciones** está al final de la página. Solo la ven los usuarios con el permiso `server.flags.edit` sobre ese servidor.
2. Elegir la pestaña de un canal (**Discord**, **Telegram** o **Webhook**) y rellenar sus campos. Un punto verde en una pestaña indica que ese canal está configurado.
   - **Webhook de Discord**: en Discord, ajustes del canal → Integraciones → Webhooks → Nuevo webhook → Copiar URL.
   - **Telegram**: **Token del bot** (se crea un bot con `/newbot` en @BotFather) e **ID del chat o canal** (un número, los grupos empiezan por `-100`, o `@tu_canal`). Antes hay que añadir el bot al grupo o canal.
   - **Webhook**: **URL del webhook** (solo `https://`) y un **Secreto para firmar (opcional)**.
3. En **Qué avisar**, activar los eventos deseados: **Caídas**, **Copias de seguridad**, **Uso alto de recursos**, **Encendido y apagado**. La **Vista previa** muestra el aspecto de los mensajes.
4. Pulsar **Guardar** y después **Enviar prueba**. Hay que guardar primero: la prueba usa la configuración guardada, no los valores en pantalla.

Para desactivar un canal, basta con vaciar sus campos y guardar.

### Mediante la API

| Método | Ruta | Permiso | Uso |
|--------|------|---------|-----|
| `GET` | `/api/servers/:serverId/notifications` | `server.flags.edit` | Leer la configuración (canales y eventos activados). |
| `PUT` | `/api/servers/:serverId/notifications` | `server.flags.edit` | Validar y guardar la configuración. Responde `204`. |
| `POST` | `/api/servers/:serverId/notifications/test` | `server.flags.edit` | Enviar una prueba a cada canal. Responde `200` con un resultado por canal: `"ok"` o el motivo del fallo. |

El cuerpo de `PUT` (y la respuesta de `GET`) es:

```json
{
  "discordWebhook": "https://discord.com/api/webhooks/123/abc",
  "telegramBotToken": "",
  "telegramChatId": "",
  "webhookUrl": "",
  "webhookSecret": "",
  "crash": true,
  "backup": true,
  "resources": true,
  "power": false
}
```

Al guardar se registra `settings.notifications` en el registro de actividad del servidor; las pruebas no se registran. Los detalles completos están en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

Esta función no tiene ajustes globales en el panel: todo se configura por servidor. Claves relacionadas:

| Clave | Valor por defecto | Variable de entorno | Efecto |
|-------|-------------------|---------------------|--------|
| `daemon.data.servers` | `servers` (dentro de la carpeta de datos) | `SKYPANEL_DAEMON_DATA_SERVERS` | Carpeta donde se guarda el archivo `.notify`. |
| `daemon.data.crashLimit` | `3` | `SKYPANEL_DAEMON_DATA_CRASHLIMIT` | Reinicios automáticos tras una caída; aparece en el aviso como «intento *n* de *límite*». |

Véase [Configuración](../backend/02-configuration.md) para saber cómo se leen las claves y las variables de entorno.

## Límites

- **Validación** (al guardar y de nuevo antes de enviar):
  - Discord: debe tener la forma `https://discord.com/api/webhooks/<id>/<token>` (también `discordapp.com`, `ptb.` y `canary.`).
  - Telegram: el token debe parecerse a `123456789:AA…` (dígitos, dos puntos y al menos 30 caracteres) y el ID del chat debe ser un número de hasta 20 dígitos (puede ser negativo) o `@nombre` (5–32 caracteres). Ambos son obligatorios a la vez.
  - Webhook: una URL `https://` con host y sin usuario ni contraseña.
- **Se rechazan las direcciones privadas.** Todas las peticiones usan un cliente HTTP que se niega a conectar con direcciones IP privadas o de bucle local, así que un webhook no puede usarse para llegar a la red interna del nodo. Por tanto, un webhook en un equipo de la LAN fallará.
- Las peticiones a Telegram y al webhook caducan a los 10 segundos. Solo cuentan como entregadas las respuestas `2xx`. No hay reintentos: un aviso fallido se registra y se descarta.
- El botón de prueba puede usarse una vez cada **10 segundos** por servidor.
- Los umbrales (80 % de CPU, 90 % de memoria, 5 minutos entre avisos) están fijados en el código.

## Problemas conocidos y soluciones

| Mensaje | Causa y solución |
|---------|------------------|
| `the URL is not a Discord webhook (https://discord.com/api/webhooks/...)` (400) | El campo de Discord contiene algo que no es una URL de webhook. Copiar de nuevo la URL desde la página de Webhooks del canal. |
| `the Telegram bot token or chat id is not valid` (400) | Formato de token incorrecto, falta el ID del chat o el ID contiene espacios o letras. Usar el token de @BotFather y un ID numérico o `@canal`. |
| `the webhook must be an https:// URL` (400) | URL con `http://`, sin host o con `usuario:contraseña@`. Usar HTTPS y emplear el secreto para firmar en lugar de credenciales en la URL. |
| `add a Discord webhook, a Telegram bot or a webhook URL first` (400, prueba) | Todavía no hay ningún canal guardado. Rellenar uno y pulsar **Guardar** antes de probar. |
| `wait a few seconds before sending another test` (429) | Dos pruebas en menos de 10 segundos. Esperar y volver a intentarlo. |
| `discord webhook returned status 404` | El webhook se eliminó en Discord. Crear uno nuevo y guardarlo. |
| `telegram answered with status 400` / `403` | El bot no está en el chat, fue bloqueado o el ID del chat es incorrecto. Añadir el bot al grupo o canal (como administrador en los canales) y revisar el ID. |
| `telegram: could not reach the server` / `webhook: could not reach the server` | El nodo no llega al host: sin salida a internet, fallo de DNS, tiempo agotado o el webhook resuelve a una dirección privada. |
| `webhook answered with status <n>` | El receptor rechazó la petición. Revisar sus registros; si valida la firma, comprobar que usa el mismo secreto y el cuerpo sin modificar. |
| No llegan avisos aunque la prueba funciona | El evento está desactivado o no se cumplió la condición (por ejemplo, una parada pedida desde el panel no es una caída; la CPU debe superar el 80 % del límite propio del servidor). |

## Véase también

- [Webhooks de Discord (avisos globales para administradores)](../general/16-discord.md)
- [Correo electrónico](../general/15-email.md)
- [Seguimiento del tiempo en línea](../general/13-uptime.md)
- [Extensiones](../reference/15-extensions.md)
- [Referencia de permisos](../general/scopes-reference.md)
- [Referencia de la API](../reference/11-api-reference.md)
