# Configuración del panel

**Configuración del panel** (`/settings`) es donde los administradores configuran el panel en su conjunto: su nombre y dirección, las funciones de IA, la protección del registro, las notificaciones, el correo, las copias externas, los dominios, la integración con facturación, las extensiones, las actualizaciones y las funciones experimentales. Esta página explica la propia página y la pestaña **General** en detalle, e indica qué documento cubre cada una de las demás pestañas.

## Qué hace

La página tiene una pestaña por área. En móviles y tabletas las pestañas se convierten en una lista desplegable.

| Pestaña | Qué configura | Quién la ve | Documentada en |
|---|---|---|---|
| **General** | Nombre, URL principal, IP del nodo maestro, idioma de las notificaciones, IA, clave de CurseForge, registro y anti-bots, funciones experimentales | `settings.edit` | Esta página |
| **Discord** | Webhooks de Discord para alertas, informes, estado de nodos y transferencias | `settings.edit` | [Discord](../general/16-discord.md) |
| **Correo** | Proveedor y remitente del correo, con botón de prueba | `settings.edit` | [Correo electrónico](../general/15-email.md) |
| **Copias externas** | Almacenamiento compatible con S3 para las copias de seguridad y la copia periódica de cada servidor (intervalo y copias a conservar por defecto) | `settings.edit` | [Copias de seguridad](./backups.md#copias-externas) |
| **Dominios** | Token, zona y dominio base de Cloudflare para los subdominios automáticos | `settings.edit` | [Subdominios](./subdomains.md) |
| **Claves API** | Claves de la API de aprovisionamiento | `admin` | [Aprovisionamiento y SSO](./provisioning-and-sso.md) |
| **Productos** | Planes que vende el sistema de facturación | `admin` | [Aprovisionamiento y SSO](./provisioning-and-sso.md) |
| **Extensiones** | Extensiones instaladas | `admin` | [Extensiones](../reference/15-extensions.md) |
| **Actualizaciones** | Comprobación de versión y actualización en un clic | `settings.edit` | [Actualizaciones del panel](./panel-updates.md) |

Quien no tiene `settings.edit` y abre `/settings` vuelve al panel de inicio. Una pestaña se puede abrir directamente con `?tab=<nombre>` (por ejemplo `/settings?tab=mail`).

## Cómo funciona

- **General**, **Discord**, **Correo**, **Copias externas** y **Dominios** comparten una barra de guardado. Los cambios en cualquiera de ellas se conservan hasta pulsar **Guardar cambios**; **Descartar** vuelve a los valores guardados. Todo se envía en una sola petición, `POST /api/settings`.
- El panel valida antes todos los valores (por ejemplo, las URL de webhooks de Discord, o el intervalo y las copias a conservar de la copia periódica) para que un valor incorrecto no deje la configuración a medio guardar. Después escribe cada clave en el `config.json` del panel (el archivo indicado en `SKYPANEL_CONFIG`, `/etc/SkyPanel/config.json` o `config.json` en el directorio de trabajo) y la aplica al momento; no hace falta reiniciar.
- Los secretos nunca se devuelven: `GET /api/settings` y `GET /api/settings/:key` responden `********` en lugar de su valor (un secreto vacío sigue vacío, para que la página sepa si está definido). Son `panel.email.key`, `panel.email.password`, `panel.settings.geminiApiKey`, `daemon.curseforge.key`, los cuatro `panel.notifications.discordWebhook*`, `panel.license.key`, `daemon.backups.s3.secretKey`, `panel.dns.cloudflare.token` y `panel.turnstile.secretKey`. La página reenvía `********` cuando no se toca el campo, y el panel lo interpreta como «conservar el valor guardado», tanto al guardar (`POST /api/settings`, `PUT /api/settings/:key`) como en los botones de prueba (correo, Discord, S3 y Cloudflare). Para cambiar un secreto basta con escribir el valor nuevo.
- **IP global del nodo maestro** es la excepción: se guarda en la base de datos (tabla `panel_settings`, clave `master_node_ip`) para que varios paneles que comparten base de datos lean el mismo valor.
- **Claves API**, **Productos**, **Extensiones** y **Actualizaciones** guardan con sus propios botones y rutas.

### La pestaña Copias externas

Además del bucket (endpoint, nombre, región, carpeta, claves, prueba de conexión e interruptor **Subir las copias al bucket**, descritos en [Copias de seguridad](./backups.md#desde-el-panel)), la sección **Copia periódica de cada servidor** fija los valores por defecto de la [copia periódica](./backups.md#copia-periódica):

- **Intervalo** (`daemon.backups.s3.periodicHours`): **Desactivada** (`0`, el valor por defecto), 12 h, 24 h, 48 h, 72 h, 7 días, 14 días o 30 días. Cada servidor recibe una copia completa que va directa al bucket con esa frecuencia, salvo los que tengan su propio intervalo.
- **Copias a conservar en S3** (`daemon.backups.s3.periodicKeep`): de 1 a 50, 4 por defecto. Al subir una nueva, se borran del bucket las periódicas más antiguas de ese servidor.

Son números: el panel solo acepta un número entero (o su texto, `"24"`) con uno de esos valores, y rechaza el resto con `ErrOffsitePeriodicHours` o `ErrOffsiteKeep` sin guardar nada, para que un error de escritura no se convierta en `0` y apague la copia. Cada servidor puede cambiar ambos valores en su pestaña **Copias de seguridad**. Como el resto de claves `daemon.backups.s3.*`, se aplican a los servidores del nodo propio del panel; cada nodo remoto las lee de su propia configuración (aviso **Nodos remotos** bajo la sección).

### La pestaña General

**Panel**

- **Nombre de la empresa**: el nombre que se muestra en la web, en el manifiesto web del navegador, en los correos y como emisor de los códigos de verificación en dos pasos.
- **URL principal**: la dirección desde la que se abre el panel, por ejemplo `https://panel.example.com` o `http://IP:8080`. Se usa en los enlaces de los correos, como origen permitido y como dirección del nodo propio del panel. En las instalaciones hechas con el instalador, guardar aquí un dominio hace que el servidor configure por sí solo Nginx, el certificado HTTPS y el cortafuegos; el resultado aparece bajo el campo. Ver [Instalación: añadir un dominio más tarde](../setup/01-installation.md).
- **IP global del nodo maestro**: la dirección que reciben los jugadores y los clientes SFTP para el nodo propio del panel. Si está vacía, el panel usa la IP del servidor cuando el dominio de la URL principal está detrás de un proxy que solo deja pasar la web (la nube naranja de Cloudflare) y, si no, el host de la URL principal. Útil cuando varios paneles comparten una base de datos.

**Notificaciones**

- **Idioma de las notificaciones** (`panel.settings.notificationLanguage`): **español (España)** (`es`, el valor por defecto) o **English (US)** (`en`). Es el idioma de lo que el panel envía fuera de sus propias páginas: las alertas e informes de Discord del panel, los avisos de caída, los textos de estado de las copias de seguridad, los avisos de Discord, Telegram y webhook de cada servidor y su mensaje de prueba, el título y el mensaje de los eventos que reciben las extensiones, y los informes de Discord de las migraciones entre paneles. Las páginas del panel siguen el idioma de cada persona. Cualquier otro valor se rechaza con `ErrNotificationLanguage`. El panel la envía a cada nodo remoto con su sondeo de estado cada 30 segundos, y al momento cuando se guarda con un valor nuevo, así que los avisos de los servidores de nodos remotos también la siguen; un nodo cuya configuración o entorno defina la clave conserva ese valor. Ver [Notificaciones del servidor](./server-notifications.md) y [Discord](../general/16-discord.md).

**Inteligencia artificial**

- **Ocultar análisis IA de la consola**: desactiva el análisis de registros con IA en todos los servidores y para todos los usuarios: oculta la tarjeta bajo la consola y **Explicar con IA** en los informes de caída, y `POST /api/servers/:id/ai/analyze` responde `403` con `ErrAIAnalysisDisabled`. El chat del **Asistente** no se ve afectado.
- **Dejar que el asistente trabaje con los archivos del servidor** (`panel.settings.aiFileTools`, activado por defecto): el asistente del servidor puede leer, crear, editar, mover y borrar archivos del servidor en el que se abre, dentro de los permisos de archivos de cada usuario, y siempre pide confirmación antes de cambiar o borrar algo. Desactivado, el asistente solo responde preguntas. Ver [Asistente de IA: trabajar con archivos](./ai-agent.md).
- **Clave API de Gemini** (visible mientras el análisis no está oculto): la clave de Google Gemini que usan el análisis de la consola y el asistente del servidor. Ver [Asistente de IA](./ai-assistant.md).

**Modpacks**

- **Clave de API de CurseForge**: necesaria para buscar e instalar modpacks de CurseForge (Modrinth no necesita ninguna). La usa el nodo propio del panel; los nodos remotos leen `daemon.curseforge.key` de su propia configuración. Ver [Modpacks y plugins](./modpacks-and-plugins.md).

**Registro y seguridad**

- **Permitir que los usuarios se puedan registrar**: registro público. Las cuentas registradas por su cuenta no pueden hacer nada hasta que se les den permisos. Desactivarlo no afecta a las invitaciones a servidores ni a las cuentas creadas desde **Usuarios**.
- **Comprobación anti-bots integrada**: al iniciar sesión, registrarse o pedir un restablecimiento de contraseña, el navegador resuelve un pequeño cálculo en segundo plano, sin servicios externos. Activada por defecto.
- **Protección Cloudflare Turnstile**, con **Site key de Turnstile** y **Secret key de Turnstile**: usa Turnstile en lugar de la comprobación integrada. Ver [Turnstile](../general/17-turnstile.md).

**Funciones experimentales**

Funciones nuevas que todavía se están afinando. Vienen apagadas; si se apaga una, vuelve a ocultarse y no se borra nada.

| Interruptor | Dónde aparece | Página |
|---|---|---|
| **Cambio de versión del juego** | En la pestaña **Configuración** de cada servidor | [Versiones del juego](./game-versions.md) |
| **Editor de propiedades** | Pestaña **Propiedades**, en servidores de Minecraft Java | [Propiedades del servidor](./server-properties.md) |
| **Clonar servidor** | En **Administración › Clonar** de cada servidor | [Clonado](./cloning.md) |

Mientras **Cambio de versión del juego** o **Clonar servidor** están apagados, sus rutas de la API responden `403` con `ErrFeatureDisabled` («this experimental feature is turned off in the panel settings»): apagarlos los detiene, no solo oculta sus botones. El editor de propiedades es un interruptor solo de la web.

## Uso

### Desde el panel

1. Abrir **Configuración** en la barra lateral (visible para administradores y para quien tenga `settings.edit`). La página se titula **Configuración del panel**.
2. Elegir una pestaña y cambiar los valores.
3. Pulsar **Guardar cambios** en la barra inferior. Un mensaje confirma **Configuración guardada correctamente**.

### Mediante la API

| Método y ruta | Permiso | Para qué |
|---|---|---|
| `GET /api/settings` | `settings.edit` | Todas las claves editables con su valor actual (los secretos como `********`), más `panel.settings.masterNodeIp` |
| `POST /api/settings` | `settings.edit` | Guardar varias claves a la vez: `{"clave": valor, ...}` |
| `GET /api/settings/:key` | `settings.edit` | Leer una clave (`204` si no es editable; un secreto, como `********`) |
| `PUT /api/settings/:key` | `settings.edit` | Guardar una clave: `{"value": ...}` (`"********"` en un secreto lo deja como está) |
| `GET /api/config` | pública | Nombre, tema, registro, captcha, `hideAiAnalysis`, interruptores experimentales y versión, tal como los lee la web |

Ejemplo:

```bash
curl -X POST https://panel.example.com/api/settings \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"panel.settings.companyName": "Example Hosting", "panel.experimental.propertiesEditor": true}'
```

Las claves desconocidas se ignoran. Las rutas de prueba (`/api/settings/test/email`, `/test/discord`, `/test/s3`, `/test/cloudflare`) y las de actualización se describen en sus propias páginas y en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

Las claves de la pestaña General, con sus valores por defecto y variables de entorno, están en [Configuración](../backend/02-configuration.md):

`panel.settings.companyName`, `panel.settings.masterUrl`, `panel.settings.notificationLanguage`, `panel.settings.geminiApiKey`, `panel.settings.hideAiAnalysis`, `panel.settings.aiFileTools`, `daemon.curseforge.key`, `panel.registrationEnabled`, `panel.registrationApproval`, `panel.captcha.builtin`, `panel.turnstile.enabled`, `panel.turnstile.siteKey`, `panel.turnstile.secretKey`, `panel.experimental.versionSwitch`, `panel.experimental.propertiesEditor`, `panel.experimental.serverClone`.

Las variables de entorno usan el prefijo `SKYPANEL_` y la clave en mayúsculas con los puntos cambiados por guiones bajos, por ejemplo `SKYPANEL_PANEL_EXPERIMENTAL_SERVERCLONE`.

## Límites

- La API no devuelve los secretos, pero quien tiene `settings.edit` puede sustituirlos (por ejemplo, apuntar el correo o las copias externas a otro servicio). Conviene dar ese permiso solo a administradores de confianza.
- Al guardar se reescribe `config.json` completo, así que su formato y el orden de las claves pueden cambiar.
- La **Clave de API de CurseForge** y la pestaña **Copias externas** (incluida la copia periódica por defecto) solo se aplican al nodo propio del panel; los nodos remotos usan su propia configuración. El **Idioma de las notificaciones** llega a los nodos remotos con el sondeo de estado, salvo que un nodo defina el suyo.

## Problemas conocidos y soluciones

| Qué se ve | Causa | Solución |
|---|---|---|
| Al abrir `/settings` se vuelve al panel de inicio | Falta `settings.edit` | Pedir el permiso a un administrador |
| `No se pudo guardar la configuración` o un mensaje de validación | Se ha rechazado un valor (por ejemplo, una URL de Discord que no es un webhook) | Corregir el valor; no se ha guardado nada |
| «La copia periódica externa solo puede hacerse cada 12, 24, 48, 72, 168, 336 o 720 horas, o desactivarse (0).» (`ErrOffsitePeriodicHours`) o «Las copias externas que se conservan deben ser entre 1 y 50.» (`ErrOffsiteKeep`) | Se envió para `daemon.backups.s3.periodicHours` o `daemon.backups.s3.periodicKeep` un valor que no está permitido o que no es un número entero | Enviar uno de los valores permitidos |
| «El idioma de las notificaciones debe ser español o inglés.» (`ErrNotificationLanguage`) | Se envió para `panel.settings.notificationLanguage` un valor distinto de `es` o `en` | Enviar `es` o `en` |
| Prueba de Discord: «El mensaje de prueba no llegó a estos webhooks: …» (`422`, `ErrDiscordTestFailed`) | Los webhooks indicados (con el nombre de las secciones de la pestaña Discord: **Alertas de servidores**, **Informes del sistema**, **Estado de los nodos**, **ExTransfer**) se borraron o son incorrectos | Volver a crear el webhook en Discord y pegar la URL nueva. Ver [Discord](../general/16-discord.md) |
| El guardado falla con un error de archivo | El panel no puede escribir `config.json` | Revisar el propietario y los permisos del archivo |
| El dominio no obtiene HTTPS tras guardar la URL principal | El dominio aún no apunta al servidor, el puerto 80 está cerrado o la instalación no tiene el servicio `aether-domain` | Ver [Instalación](../setup/01-installation.md) |
| `ErrFeatureDisabled` desde la API | La función experimental está apagada | Activarla en **Funciones experimentales** |

## Véase también

- [Configuración](../backend/02-configuration.md)
- [Seguridad](../general/security.md)
- [Referencia de permisos](../general/scopes-reference.md)
