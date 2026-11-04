# Configuración

Aether Panel lee su configuración con **Viper**. Cada opción es una clave declarada en `internal/config/entries.go` con su tipo y su valor por defecto; `internal/config/config.go` localiza el archivo y asocia las variables de entorno. Esta página recoge **todas las claves que declara el código** (97), agrupadas por área.

## De dónde salen los valores

### El archivo de configuración

El archivo se busca en este orden (`config.LoadConfigFile`):

1. La opción `--config <ruta>` del binario.
2. La variable de entorno `SKYPANEL_CONFIG`.
3. `/etc/SkyPanel/config.json`, si existe.
4. `config.json` en el directorio de trabajo (`--workDir` lo cambia antes de buscar).

Viper deduce el formato de la extensión del archivo; los archivos incluidos (`config.docker.json`, `config.linux.json`) y el instalador usan JSON. Las claves anidadas son objetos anidados: `panel.database.dialect` equivale a `{"panel": {"database": {"dialect": "..."}}}`. Las claves no distinguen mayúsculas de minúsculas.

Si el archivo no se puede leer, el binario muestra `Error loading config, this may impact features` y continúa con el entorno y los valores por defecto.

La imagen Docker copia `config.docker.json` a `/etc/SkyPanel/config.json`; `docker-compose.yml` monta ahí `./storage/skypanel-config` para que el archivo sobreviva a las actualizaciones.

### Variables de entorno

Cualquier clave se puede fijar con una variable de entorno: el prefijo `SKYPANEL_` seguido de la clave en mayúsculas, cambiando cada `.` por `_`.

| Clave | Variable |
|---|---|
| `web.host` | `SKYPANEL_WEB_HOST` |
| `panel.database.url` | `SKYPANEL_PANEL_DATABASE_URL` |
| `docker.disallowHost` | `SKYPANEL_DOCKER_DISALLOWHOST` |
| `panel.web.cookies.sameSite` | `SKYPANEL_PANEL_WEB_COOKIES_SAMESITE` |
| `logRetentionDays` | `SKYPANEL_LOGRETENTIONDAYS` |

Notas:

- Una variable **vacía** cuenta como no definida (Viper ignora las variables vacías), así que `SKYPANEL_DOCKER_ROOT=` en `docker-compose.yml` deja el valor por defecto. Para vaciar una lista como `docker.prefetchImages` hay que usar el archivo (`"prefetchImages": []`).
- Una clave de tipo lista (`security.trustedProxies`, `docker.prefetchImages`) fijada por variable se separa por espacios: `SKYPANEL_SECURITY_TRUSTEDPROXIES="10.0.0.0/8 172.16.0.0/12"`.
- El código lee directamente dos variables que no son claves de configuración: `SKYPANEL_CONFIG` (ubicación del archivo, ver arriba) y `SKYPANEL_PLATFORM`. La imagen y el compose fijan `SKYPANEL_PLATFORM=docker`; con ella el panel sigue funcionando si el daemon no arranca, y el daemon busca su propio contenedor para traducir las rutas de datos a rutas del host (ver `docker.root`).

### Prioridad

De mayor a menor:

1. Un valor cambiado en caliente desde la página Configuración del panel (`/api/settings`): se aplica al momento y se escribe en el archivo de configuración.
2. La variable de entorno.
3. El archivo de configuración.
4. El valor por defecto de `entries.go`.

Como el panel guarda la configuración en el archivo, **tras un reinicio la variable de entorno vuelve a ganar** a lo guardado desde Configuración. Por eso `docker-compose.yml` no fija opciones pensadas para cambiarse en Configuración (registro, tema, Turnstile).

### Valores que genera y guarda el panel

| Clave | Cuándo |
|---|---|
| `panel.sessionKey` | Al arrancar el panel, si está vacía: 32 bytes aleatorios en hexadecimal. Firma la cookie `session` y sirve de semilla del secreto del nodo local. |
| `panel.token` | La primera vez que arranca el servicio de tokens en un panel: una semilla Ed25519 aleatoria en base64. Firma cada petición del panel a los nodos. |
| `panel.exTransferKey` | La primera vez que hace falta la clave de transferencias federadas. |
| `daemon.data.root` | Tras leer el archivo, si está vacía: la carpeta padre de `daemon.data.servers`. |
| `panel.license.serverId`, `panel.license.serverIp` | Al activar una licencia, si están vacías (nombre del host y dirección detectada). |

La clave de host de SFTP es un archivo, no un valor: si `daemon.sftp.key` apunta a un archivo que no existe, se genera ahí una clave Ed25519.

### Carpetas de datos

Las claves de tipo «carpeta de datos» aceptan una ruta absoluta o una relativa, que se resuelve respecto a `daemon.data.root`: `daemon.sftp.key`, `daemon.data.cache`, `daemon.data.servers`, `daemon.data.backups.folder`, `daemon.data.binaries` y `panel.extensions.folder`. Las demás claves de ruta (`logs`, `panel.web.files`, `panel.email.templateFolder`) son relativas al directorio de trabajo.

## Referencia de configuración

La columna **Configuración** marca las claves que un administrador puede cambiar desde el panel (página Configuración, `GET/POST /api/settings`, `GET/PUT /api/settings/{key}`, permiso `settings.edit`). Un guion en **Por defecto** significa cadena vacía.

### General y registros

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `web.host` | string | `0.0.0.0:8080` | | Dirección y puerto del servidor HTTP (panel, API y `/daemon`). En la instalación con Docker se queda en `0.0.0.0:8080` dentro del contenedor; el instalador decide en qué dirección del host se publica el puerto (solo `127.0.0.1` cuando el panel se sirve por un dominio). |
| `logs` | string | `logs` | | Carpeta del registro del panel. La configuración Docker usa `/var/log/SkyPanel`. |
| `logRetentionDays` | int | `14` | | Días que se conservan los registros rotados; `0` los conserva todos. |
| `templates.url` | string | — | | URL de un índice JSON de plantillas. Si se fija, se ignoran los repositorios de plantillas guardados en la base de datos y el panel ofrece este índice (como repositorio `community`) más las plantillas locales. `docker-compose.yml` fija `https://templates.aetherpanel.es/templates.json`. |
| `token.public` | string | — | | Solo en un nodo sin panel (`panel.enable=false`): URL del conjunto de claves públicas del panel, `<panel>/auth/publickey`, con la que se comprueban las peticiones que firma el panel. El panel avisa al arrancar si está vacía en un nodo así. |

El panel escribe el día en curso en `skypanel.log`, dentro de `logs`. Cuando cambia el día, o el archivo supera 100 MB, lo archiva como `skypanel-AAAA-MM-DD.log.gz` (`.1`, `.2`… si ese día ya tiene uno) y empieza otro. Los archivos `skypanel-*` escritos por última vez hace más de `logRetentionDays` días se borran. No hace falta `logrotate`; si se usa, conviene poner `logRetentionDays` a `0` y enviar `SIGUSR1` para que el panel reabra el archivo.

### Panel

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `panel.enable` | bool | `true` | | Ejecuta el panel web: API, inicio de sesión, OAuth2, cliente web y base de datos. Con `false` el binario es un nodo que solo atiende `/daemon`. |
| `panel.registrationEnabled` | bool | `true` | Sí | Permite el registro público (`POST /auth/register`). Los administradores pueden crear cuentas igualmente. |
| `panel.servers.autoInstall` | bool | `true` | | Ejecuta la instalación de la plantilla nada más crear un servidor (también desde la facturación), para que se pueda arrancar sin pulsar Instalar. |
| `panel.backups.maxPerServer` | int | `10` | | Copias de seguridad que pueden conservar los usuarios de un servidor; crear una más responde `409`. Los administradores no tienen límite. `0` lo desactiva. Las copias no cuentan en el disco del servidor. |
| `panel.web.files` | string | `www` | | Carpeta que se superpone al cliente web incluido en el binario: un archivo que esté aquí sustituye al incluido. Los temas se leen de su subcarpeta `theme/` (`*.tar`). Las configuraciones incluidas usan `/var/www/SkyPanel`. |
| `panel.extensions.folder` | carpeta de datos | `extensions` | | Donde guardan sus archivos las extensiones instaladas, una carpeta por extensión. Ver [Extensiones](../reference/15-extensions.md). |

### Base de datos

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `panel.database.dialect` | string | `sqlite3` | | `sqlite3`, `mysql`, `postgresql` o `sqlserver`. |
| `panel.database.url` | string | — | | Cadena de conexión. Vacía equivale a `file:skypanel.db` en SQLite y a `SkyPanel:SkyPanel@/SkyPanel` en MySQL; PostgreSQL y SQL Server necesitan una. |
| `panel.database.log` | bool | `false` | | Registra cada sentencia SQL. |

Los parámetros que se añaden a cada cadena de conexión se detallan en [Base de datos](./03-database.md).

### Cookies de sesión

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `panel.web.cookies.secure` | bool | `false` | | Marca las cookies como `Secure`. La cookie `skypanel_auth` ya es `Secure` siempre que la petición llega por HTTPS (directamente o con `X-Forwarded-Proto: https`); esta clave la fuerza en todas las peticiones. |
| `panel.web.cookies.httpOnly` | bool | `true` | | Atributo `HttpOnly` de la cookie `session`. |
| `panel.web.cookies.domain` | string | — | | `Domain` de la cookie `session`. |
| `panel.web.cookies.age` | int | `2592000` | | `Max-Age` de la cookie `session`, en segundos (30 días). |
| `panel.web.cookies.sameSite` | string | `Strict` | | `SameSite` de la cookie `session`: `Strict`, `Lax` o `None`; cualquier otro valor equivale a `Strict`. |
| `panel.web.cookies.path` | string | `/` | | `Path` de la cookie `session`. |

Estas claves configuran el almacén de la cookie `session`, que solo guarda el inicio de sesión entre el paso de la contraseña y el del segundo factor (caduca a los 5 minutos). La cookie que mantiene la sesión del panel, `skypanel_auth`, es siempre `HttpOnly`, `SameSite=Lax` y de ruta `/`; solo le afecta `panel.web.cookies.secure`. Ver [Capa API](./05-api-layer.md#autenticación).

### Secretos del panel

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `panel.sessionKey` | string | (generada) | | Clave del almacén de la cookie `session`; también es la semilla del secreto estable del nodo local. Cambiarla anula los inicios de sesión a medias del segundo factor y cambia ese secreto. |
| `panel.token` | string | (generada) | | Semilla Ed25519 (base64) con la que el panel firma sus peticiones a los nodos. Su mitad pública se sirve en `/auth/publickey`. Tras cambiarla, los nodos remotos rechazan las peticiones del panel hasta que cargan el nuevo conjunto de claves públicas. |
| `panel.exTransferKey` | string | (generada) | | Semilla de la clave que firma las peticiones de transferencia federada (entre paneles). Se guarda para que un reinicio a mitad de una transferencia no la invalide. |

### Marca y ajustes generales

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `panel.settings.companyName` | string | `SkyPanel` | Sí | Nombre que muestran el cliente web, el manifiesto web y los correos, y emisor de los códigos de doble factor. |
| `panel.settings.defaultTheme` | string | `SkyPanel` | Sí | Tema activo que devuelve `GET /api/config`. |
| `panel.settings.themeSettings` | string | `{}` | Sí | Ajustes del tema (texto JSON) que devuelve `GET /api/config`. |
| `panel.settings.masterUrl` | string | `http://localhost:8080` | Sí | URL pública del panel (la «URL base»). Se usa en los enlaces de restablecer contraseña y en las plantillas de correo, como origen permitido por la protección de origen cruzado y como dirección del nodo local (host y puerto; sin puerto, 80 o 443 según el esquema). En un nodo sin panel es la URL del panel. |
| `panel.settings.nodeIp` | string | `0.0.0.0` | Sí | Dirección de esta máquina. Un nodo registrado con esta dirección cuenta como nodo local, y los subdominios de los servidores del nodo local apuntan a ella cuando es pública. |
| `panel.settings.geminiApiKey` | string | — | Sí | Clave de Google Gemini para el análisis de registros y el asistente con IA. Sin ella esos endpoints fallan. |
| `panel.settings.hideAiAnalysis` | bool | `false` | Sí | Desactiva el análisis de registros con IA para todos los usuarios: el cliente web oculta la tarjeta de la consola y el botón «Explicar con IA» del informe de fallo (lee el interruptor de `GET /api/config`, campo `hideAiAnalysis`), y `POST /api/servers/{id}/ai/analyze` responde `403 ErrAIAnalysisDisabled`. El chat de IA no se ve afectado. |
| `panel.settings.aiFileTools` | bool | `true` | Sí | Permite al asistente del servidor trabajar con los archivos del servidor en el que se abre (modo agente), dentro de los permisos `server.files.view` / `server.files.edit` del usuario; los cambios y los borrados siempre esperan la confirmación del usuario. Desactivado, el asistente solo responde preguntas y las confirmaciones pendientes responden `409 ErrAIConfirmationInvalid`. Variable de entorno `SKYPANEL_PANEL_SETTINGS_AIFILETOOLS`. No se publica en `GET /api/config`. Véase [Kit de herramientas del agente de IA](./14-ai-toolkit.md). |

`panel.settings.masterNodeIp` también aparece en la API de configuración, pero no es una clave de configuración: se guarda en la base de datos (tabla `panel_settings`, clave `master_node_ip`). Si tiene valor, es la dirección que reciben jugadores y clientes SFTP para el nodo local, en lugar del host de `panel.settings.masterUrl`.

### Protección del inicio de sesión

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `panel.captcha.builtin` | bool | `true` | Sí | Comprobación antibots propia en el inicio de sesión, el registro y el restablecimiento de contraseña: el navegador resuelve una pequeña prueba de trabajo (`GET /auth/challenge`), sin servicios externos. |
| `panel.turnstile.enabled` | bool | `false` | Sí | Usa Cloudflare Turnstile en lugar de la comprobación propia. Solo toma el relevo si también están fijadas las claves de sitio y secreta. |
| `panel.turnstile.siteKey` | string | — | Sí | Clave de sitio de Turnstile (pública, la devuelve `GET /api/config`). |
| `panel.turnstile.secretKey` | string | — | Sí | Clave secreta de Turnstile. |

Ver [Turnstile](../general/17-turnstile.md).

### Correo

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `panel.email.provider` | string | — | Sí | `smtp`, `mailgun`, `mailjet`, `sendgrid` o `debug`. Vacío desactiva el correo. |
| `panel.email.from` | string | — | Sí | Dirección del remitente (todos los proveedores). |
| `panel.email.host` | string | — | Sí | Servidor SMTP (`smtp`). |
| `panel.email.username` | string | — | Sí | Usuario SMTP (`smtp`). |
| `panel.email.password` | string | — | Sí | Contraseña SMTP (`smtp`; se quitan los espacios, así que las contraseñas de aplicación se pueden pegar tal cual se muestran). |
| `panel.email.domain` | string | — | Sí | Dominio de envío de Mailgun (`mailgun`) o clave pública de la API (`mailjet`). |
| `panel.email.key` | string | — | Sí | Clave de API (`mailgun`, `sendgrid`) o clave privada de la API (`mailjet`). |
| `panel.email.templateFolder` | string | — | | Carpeta que se superpone a las plantillas de correo incluidas. |

`POST /api/settings/test/email` envía un mensaje de prueba con los valores del cuerpo de la petición sin guardarlos. Ver [Correo](../general/15-email.md).

### Notificaciones

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `panel.notifications.discordWebhook` | string | — | Sí | Webhook de Discord para las alertas de servidores (caídas, apagado/encendido, recursos, copias de seguridad). |
| `panel.notifications.discordWebhookSystem` | string | — | Sí | Webhook de Discord para el resumen periódico del sistema. |
| `panel.notifications.discordWebhookNode` | string | — | Sí | Webhook de Discord para el estado del nodo. Mientras esté vacío, el informe de nodos no se genera y no se pide su hardware a los nodos remotos. |
| `panel.notifications.discordWebhookExTransfer` | string | — | Sí | Webhook de Discord para las transferencias federadas. |
| `panel.settings.notificationLanguage` | string | `es` | Sí | Idioma (`es` o `en`) de lo que el panel envía fuera de sus propias páginas: las alertas e informes de Discord del panel, las alertas de Discord, Telegram y webhook JSON que configura cada dueño y su prueba, el título y el mensaje de los eventos de las extensiones y los informes de las transferencias externas. Al guardar se rechaza cualquier otro valor (`ErrNotificationLanguage`). En el cliente web: Configuración → General → Notificaciones. Variable de entorno `SKYPANEL_PANEL_SETTINGS_NOTIFICATIONLANGUAGE`. El panel envía su valor a cada nodo remoto en la cabecera `X-Aether-Notification-Language` del sondeo de estado (cada 30 segundos, y al momento cuando el ajuste se guarda con un valor nuevo). En un nodo el orden es: un valor definido en el archivo de configuración o el entorno de ese nodo, después el valor del panel y después `es`; el valor del panel se guarda en memoria, así que tras reiniciar el nodo es `es` (o el propio del nodo) hasta el siguiente sondeo. Las páginas del panel siguen el idioma de cada usuario. |

Al guardar solo se aceptan URL `https://discord.com/api/webhooks/...` (o un valor vacío). Ver [Discord](../general/16-discord.md). Las alertas propias de cada servidor, que configura su dueño, no son configuración: se guardan junto al servidor (ver [Servidores](./04-servers.md#archivos-de-cada-servidor-en-el-nodo)).

### Licencia

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `panel.license.key` | string | — | Sí | Clave de licencia, la guarda `POST /api/settings/license/activate`. |
| `panel.license.status` | string | `free` | Sí | Tipo de licencia que devuelve el servidor de licencias al activarla. |
| `panel.license.serverId` | string | — | Sí | Identificador del servidor que se envía al vincular la licencia (por defecto, el nombre del host). |
| `panel.license.serverIp` | string | — | Sí | Dirección del servidor que se envía al vincular la licencia. |

Ver [Licencia](../general/14-license.md).

### Funciones experimentales

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `panel.experimental.versionSwitch` | bool | `false` | Sí | Cambio de versión del juego en un clic. Mientras está desactivada, `POST /api/servers/{id}/version` responde `403`. |
| `panel.experimental.propertiesEditor` | bool | `false` | Sí | Muestra en el cliente web el editor de formulario de `server.properties` (solo un indicador, lo devuelve `GET /api/config`). |
| `panel.experimental.serverClone` | bool | `false` | Sí | Clonado de servidores. Mientras está desactivada, `POST /api/servers/{id}/clone` responde `403`. |

### Subdominios de servidores (DNS de Cloudflare)

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `panel.dns.cloudflare.token` | string | — | Sí | Token de la API de Cloudflare; solo necesita Zona → DNS → Editar sobre la zona. |
| `panel.dns.cloudflare.zoneId` | string | — | Sí | ID de la zona. |
| `panel.dns.baseDomain` | string | — | Sí | Dominio bajo el que reciben nombre los servidores (`survival` → `survival.<baseDomain>`). Hacen falta token, zona y dominio base. |
| `panel.dns.target` | string | — | Sí | Adónde apuntan los registros (IP o nombre de host) de un nodo sin IP pública ni nombre utilizable. Los nodos con dirección propia siempre reciben la suya. Al cambiarlo se actualizan todos los subdominios. |
| `panel.dns.autoAssign` | bool | `false` | Sí | Da a cada servidor nuevo un subdominio derivado de su nombre (`-2`, `-3`… si ya existe). |

Ver [Subdominio (Cloudflare)](./04-servers.md#subdominio-cloudflare) y [Subdominios](../features/subdomains.md).

### Daemon

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `daemon.enable` | bool | `true` | | Ejecuta el daemon: rutas `/daemon`, SFTP y los servidores de juego de esta máquina. |
| `daemon.data.root` | string | (generada) | | Carpeta base de las carpetas de datos relativas de abajo. Las configuraciones incluidas usan `/var/lib/SkyPanel`. |
| `daemon.data.servers` | carpeta de datos | `servers` | | Carpetas de los servidores y sus archivos asociados (`<id>.json`, `<id>.cron`…). |
| `daemon.data.backups.folder` | carpeta de datos | `backups` | | Copias de seguridad, una subcarpeta por servidor. |
| `daemon.data.cache` | carpeta de datos | `cache` | | Caché de descargas. |
| `daemon.data.binaries` | carpeta de datos | `binaries` | | Herramientas que descarga el panel (Java, DepotDownloader…). Se añade al `PATH` del daemon y se monta en los contenedores en `/var/lib/SkyPanel/binaries`. |
| `daemon.data.crashLimit` | int | `3` | | Reinicios automáticos tras caídas seguidas (ver [caídas](./04-servers.md#caídas-y-reinicios-automáticos)). |
| `daemon.minFreeDiskMB` | int | `2048` | | Espacio libre (MB) que debe conservar el disco de los servidores: se rechaza una instalación que empezaría con menos. `0` desactiva la comprobación. |
| `daemon.hostProc` | string | — | | Dónde está montado el `/proc` del host cuando el daemon corre en un contenedor (`docker-compose.yml` lo monta en solo lectura en `/host/proc`). El tráfico de red del nodo se lee ahí; sin ello solo se ve el tráfico del propio contenedor del panel. |
| `daemon.sftp.host` | string | `0.0.0.0:5657` | | Dirección de escucha de SFTP. Su puerto es también el puerto SFTP que se anuncia para el nodo local. |
| `daemon.sftp.key` | carpeta de datos | `sftp.key` | | Archivo de la clave de host de SFTP (se genera si no existe). |
| `daemon.sftp.log` | bool | `false` | | Registra cada petición SFTP. |
| `daemon.auth.url` | string | `http://localhost:8080` | | Solo en un nodo sin panel: el endpoint de tokens OAuth2 del panel, `<panel>/oauth2/token`, con el que se comprueban las contraseñas SFTP. A una URL sin ruta (como la predeterminada, o `https://panel.example.com`) se le añade `/oauth2/token`; una URL con ruta se usa tal cual. Los inicios de sesión con clave SSH se comprueban en `/auth/sftp-key` de la misma dirección (se conserva la ruta que haya delante de `/oauth2/token`), así que para ellos el valor debe ser una URL `http(s)` absoluta. |
| `daemon.auth.clientId` | string | — | | ID de cliente del nodo (`.node_<id>`). Declarada y fijada por el fragmento de despliegue, pero el código actual no la lee. |
| `daemon.auth.clientSecret` | string | — | | Secreto del nodo, de `GET /api/nodes/{id}/deployment`. Se envía al panel al comprobar contraseñas y claves SSH de SFTP, y sirve para reconocer los tokens que el panel firma para este nodo. |
| `daemon.console.buffer` | int | `50` | | Declarada, pero el código actual no la lee. |
| `daemon.console.forward` | bool | `false` | | Declarada, pero el panel no la lee (solo la fijan las herramientas de desarrollo de `tools/`). |
| `daemon.curseforge.key` | string | (valor de compilación) | Sí | Clave de la API de CurseForge para los modpacks y descargas de CurseForge. El valor por defecto se puede inyectar al compilar (argumento de compilación `curseforgeKey` del Dockerfile); si no, está vacío. En un nodo remoto se fija en la configuración de ese nodo. |
| `daemon.depotDownloader.version` | string | `latest` | | Versión de DepotDownloader para las descargas de Steam. |
| `daemon.depotDownloader.disableLancache` | bool | `false` | | Omite `-use-lancache` al ejecutar DepotDownloader. |

### Copias de seguridad externas (S3)

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `daemon.backups.s3.enabled` | bool | `false` | Sí | Copia cada copia de seguridad terminada a un bucket compatible con S3 (AWS S3, Backblaze B2, Cloudflare R2, Wasabi, MinIO…). La copia local se conserva. |
| `daemon.backups.s3.endpoint` | string | — | Sí | Endpoint: `s3.example.com`, `host:puerto` o una URL completa. |
| `daemon.backups.s3.region` | string | — | Sí | Región. |
| `daemon.backups.s3.bucket` | string | — | Sí | Bucket. |
| `daemon.backups.s3.accessKey` | string | — | Sí | Clave de acceso. |
| `daemon.backups.s3.secretKey` | string | — | Sí | Clave secreta. |
| `daemon.backups.s3.prefix` | string | `aether-backups` | Sí | Prefijo de los objetos en el bucket. |
| `daemon.backups.s3.insecure` | bool | `false` | Sí | HTTP sin cifrar hacia el endpoint (para un MinIO en la red local). |

La copia solo se hace si está activada y están fijados endpoint, bucket, clave de acceso y clave secreta. Son claves del daemon: La página Configuración las cambia en la máquina donde corre el panel (su nodo local); un nodo remoto las lee de su propia configuración. `POST /api/settings/test/s3` comprueba que el bucket existe y luego escribe y borra un objeto pequeño (no se prueba la lectura, así que una clave que puede escribir pero no leer también pasa la prueba). Ver [Copias de seguridad](../features/backups.md).

### Docker

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `docker.disallowHost` | bool | `false` | | Rechaza los entornos que corren en el host (`host`, `tty`, `standard`, `bubblewrap`). Se comprueba al crear, redefinir, instalar y arrancar un servidor, y esos entornos no aparecen en `/daemon/features`. La imagen y el compose la ponen a `true`. |
| `docker.root` | string | — | | Ruta en el host de la carpeta de datos del daemon, para construir los montajes cuando el propio daemon corre en un contenedor. Si está vacía y `SKYPANEL_PLATFORM` tiene valor, el daemon localiza su propio contenedor y lee el montaje de `daemon.data.root`. |
| `docker.serversNetwork` | string | — | | Red Docker de los contenedores de juego cuya plantilla no indica ninguna. Si la red no existe, se usa la red del propio panel. El compose fija `skypanel-servers`. |
| `docker.serverUser` | string | `988:988` | | `uid:gid` con el que corren los juegos cuando el panel corre como root; las instalaciones siguen como root y el usuario propio de una plantilla tiene prioridad. `0:0` mantiene los juegos como root. Un valor no numérico vuelve a `988:988`. |
| `docker.pidsLimit` | int | `4096` | | Procesos e hilos por contenedor de servidor; el límite propio de una plantilla tiene prioridad. `0` quita el límite. |
| `docker.prefetchImages` | []string | `ghcr.io/aether-panel/steamcmd:latest`, `ghcr.io/aether-panel/ubuntu:latest`, `ghcr.io/aether-panel/generic:latest` | | Imágenes de ejecución que se descargan en segundo plano en cada nodo Docker, dos minutos después de arrancar y luego a diario; una copia sustituida se borra si ningún contenedor la usa. Una lista vacía lo desactiva. |

Ver [Servidores: entorno Docker](./04-servers.md#entorno-docker).

### Seguridad

| Clave | Tipo | Por defecto | Configuración | Descripción |
|---|---|---|---|---|
| `security.trustedProxies` | []string | `[]` | | Más proxies (IP o CIDR) cuyas cabeceras de reenvío se aceptan para la dirección del cliente, además de esta máquina, las redes privadas y el borde de Cloudflare, en los que siempre se confía. |
| `security.trustedProxyHeader` | string | — | | Cabecera en la que una plataforma de confianza pone la dirección del cliente (`TrustedPlatform` de Gin, por ejemplo `CF-Connecting-IP`). |
| `security.blockPrivateDownloads` | bool | `true` | | Rechaza las descargas de plantillas desde direcciones de red privadas (redes Docker, la LAN). Las direcciones de loopback, de enlace local y multicast se rechazan siempre. Desactivarla solo para espejos en la LAN. |
| `security.forceOpenat` | bool | `false` | | Omite la comprobación de `openat2` al arrancar y usa `openat` normal. Sin ella, un daemon que no puede usar `openat2` (el kernel no lo tiene, o un perfil seccomp lo bloquea) se niega a arrancar. |
| `security.disableUnshare` | bool | `false` | | Ejecuta los servidores de entorno host sin espacios de nombres `unshare` (cada servidor también puede fijar `disableUnshare`). Ver [unshare](../general/20-security-unshare.md). |

## API de configuración

| Ruta | Descripción |
|---|---|
| `GET /api/settings` | Todas las claves marcadas en **Configuración**, más `panel.settings.masterNodeIp`. Los secretos (contraseñas, claves de API, tokens, webhooks, la clave de licencia) se devuelven como `********` cuando están definidos; enviar esa máscara de vuelta a cualquiera de las rutas siguientes conserva el valor guardado. |
| `POST /api/settings` | Cambia varias claves a la vez (`{"clave": valor, ...}`). Primero se valida todo, así que un valor incorrecto no guarda nada. Las claves desconocidas se ignoran. |
| `GET /api/settings/{key}` / `PUT /api/settings/{key}` | Lee o cambia una clave (`{"value": ...}`). `GET` responde `204` para una clave que no es editable. |
| `POST /api/settings/test/email`, `/test/discord`, `/test/s3`, `/test/cloudflare` | Comprueba los ajustes de correo, Discord, S3 y Cloudflare. |
| `POST /api/settings/license/activate` | Verifica y vincula una clave de licencia. |
| `GET /api/config` | Pública: nombre, tema, registro, captcha, `hideAiAnalysis`, funciones experimentales y versión, para el cliente web. |

Todas salvo `/api/config` requieren `settings.edit`. Los textos se recortan antes de guardarse y se convierten al tipo de la clave. La propia página Configuración se describe en [Configuración del panel](../features/panel-settings.md).

## Ejemplo: instalación con Docker

`docker-compose.yml` fija estas variables en el contenedor del panel (los datos de la base de datos salen de `.env`):

```yaml
environment:
  - GIN_MODE=release
  - SKYPANEL_PLATFORM=docker
  - SKYPANEL_DOCKER_ROOT=
  - SKYPANEL_DOCKER_DISALLOWHOST=true
  - SKYPANEL_DAEMON_HOSTPROC=/host/proc
  - SKYPANEL_DOCKER_SERVERSNETWORK=skypanel-servers
  - SKYPANEL_WEB_HOST=0.0.0.0:8080
  - SKYPANEL_PANEL_DATABASE_DIALECT=mysql
  - SKYPANEL_PANEL_DATABASE_URL=${DB_USER:-skypanel}:${DB_PASSWORD:-skypanel_secret}@tcp(mysql:3306)/${DB_DATABASE:-skypanel}?charset=utf8&parseTime=true
  - SKYPANEL_TEMPLATES_URL=https://templates.aetherpanel.es/templates.json
```

y la imagen incluye este `/etc/SkyPanel/config.json` (`config.docker.json`):

```json
{
  "logs": "/var/log/SkyPanel",
  "panel": {
    "enable": true,
    "database": {
      "dialect": "mysql",
      "url": "skypanel:CHANGE_ME@tcp(mysql:3306)/skypanel?charset=utf8mb4&parseTime=true"
    },
    "web": { "files": "/var/www/SkyPanel" }
  },
  "daemon": { "data": { "root": "/var/lib/SkyPanel" } },
  "docker": { "disallowHost": true }
}
```

Las variables de entorno ganan al archivo, así que se usa la URL de la base de datos del compose.

## Ejemplo: nodo remoto

Un nodo ejecuta la misma imagen con el panel desactivado. La página de Nodos del panel muestra un fragmento de compose como este; el secreto de cliente es el secreto del nodo, que devuelve `GET /api/nodes/{id}/deployment` (permiso `nodes.deploy`):

```yaml
environment:
  - GIN_MODE=release
  - SKYPANEL_PLATFORM=docker
  - SKYPANEL_WEB_HOST=0.0.0.0:8080
  - SKYPANEL_PANEL_ENABLE=false
  - SKYPANEL_PANEL_SETTINGS_MASTERURL=https://panel.example.com
  - SKYPANEL_TOKEN_PUBLIC=https://panel.example.com/auth/publickey
  - SKYPANEL_DAEMON_AUTH_URL=https://panel.example.com/oauth2/token
  - SKYPANEL_DAEMON_AUTH_CLIENTID=.node_2
  - SKYPANEL_DAEMON_AUTH_CLIENTSECRET=<secreto del nodo>
  - SKYPANEL_DAEMON_SFTP_HOST=0.0.0.0:5657
```

Lo mismo en `config.json`:

```json
{
  "panel": {
    "enable": false,
    "settings": { "masterUrl": "https://panel.example.com" }
  },
  "token": { "public": "https://panel.example.com/auth/publickey" },
  "daemon": {
    "auth": {
      "url": "https://panel.example.com/oauth2/token",
      "clientId": ".node_2",
      "clientSecret": "<secreto del nodo>"
    },
    "sftp": { "host": "0.0.0.0:5657" }
  },
  "web": { "host": "0.0.0.0:8080" }
}
```

Cómo se autentican ambos lados:

- **Panel → nodo.** En cada llamada a `/daemon/*` el panel firma un JWT de vida corta (EdDSA con `panel.token`, 15 minutos, audiencia derivada del secreto del nodo). El nodo lo comprueba con el conjunto de claves de `token.public` y con `daemon.auth.clientSecret`.
- **Nodo → panel.** Cuando alguien entra por SFTP con contraseña, el nodo la envía a `daemon.auth.url` (`grant_type=password`, usuario `email#idServidor`) con `Authorization: Bearer <clientSecret>`. Con una clave SSH, envía el usuario y la clave pública ofrecida a `/auth/sftp-key` del panel con la misma cabecera. En ambos casos el panel solo confirma el permiso `server.sftp` sobre ese servidor; no abre ninguna sesión.

Ver [Seguridad](./09-security.md) para el detalle de los tokens y de SFTP.
