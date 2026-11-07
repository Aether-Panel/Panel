# Administración del panel

Nodos, configuración del panel, claves de API, plantillas, versiones de juego, hosts de bases de datos, extensiones y disponibilidad. Volver a la [referencia de la API](../11-api-reference.md). Los productos de aprovisionamiento están en [Aprovisionamiento / facturación](./07-provisioning.md#productos).

## Rutas

| Método | Ruta | Permiso | Qué hace |
|--------|------|---------|----------|
| GET | `/api/nodes` | `nodes.view` | Lista los nodos |
| POST | `/api/nodes` | `nodes.create` | Añade un nodo |
| GET | `/api/nodes/network-history` | `nodes.view` | Tráfico de todos los nodos sumado |
| GET | `/api/nodes/:id` | `nodes.view` | Un nodo |
| PUT | `/api/nodes/:id` | `nodes.edit` | Actualiza un nodo |
| DELETE | `/api/nodes/:id` | `nodes.delete` | Borra un nodo sin servidores |
| GET | `/api/nodes/:id/features` | `nodes.view` | Sistema operativo, arquitectura, entornos, Docker |
| GET | `/api/nodes/:id/system` | `nodes.view` | CPU, memoria, discos, contadores de red |
| POST | `/api/nodes/:id/ports/check` | `server.create` | Qué puertos están ocupados, con alternativas libres |
| GET | `/api/nodes/:id/deployment` | `nodes.deploy` | Credenciales para desplegar el daemon del nodo |
| POST | `/api/nodes/:id/join` | `nodes.deploy` | Código de unión de un solo uso y el comando que instala el nodo |
| POST | `/api/node-join` | Público | Cambia un código de unión por las credenciales del nodo |
| GET | `/api/settings` | `settings.edit` | Todos los ajustes editables |
| POST | `/api/settings` | `settings.edit` | Guarda varios ajustes |
| GET | `/api/settings/:key` | `settings.edit` | Un ajuste |
| PUT | `/api/settings/:key` | `settings.edit` | Guarda un ajuste |
| POST | `/api/settings/test/email` | `settings.edit` | Envía un correo de prueba |
| POST | `/api/settings/test/discord` | `settings.edit` | Publica en todos los webhooks de Discord configurados |
| POST | `/api/settings/test/s3` | `settings.edit` | Comprueba el almacenamiento externo de copias de seguridad |
| POST | `/api/settings/test/cloudflare` | `settings.edit` | Comprueba el token, la zona y el dominio de Cloudflare |
| POST | `/api/settings/license/activate` | `settings.edit` | Activa una clave de licencia |
| POST | `/api/settings/update-panel` | `settings.edit` | Inicia la actualización del panel y de todos los nodos |
| GET | `/api/settings/update-check` | `settings.edit` | Indica si hay una versión más reciente |
| GET | `/api/settings/update-status` | `settings.edit` | Progreso de la última actualización en cada nodo |
| GET | `/api/settings/domain-status` | `settings.edit` | Qué hizo el instalador con el dominio del panel |
| GET | `/api/settings/node-sync` | `settings.edit` | Si cada nodo remoto tiene los ajustes de nodo del panel |
| GET | `/api/settings/apikeys` | `admin` | Lista las claves de API de facturación |
| POST | `/api/settings/apikeys` | `admin` | Crea una |
| DELETE | `/api/settings/apikeys/:id` | `admin` | Borra una |
| GET | `/api/templates` | `login` | Repositorios de plantillas |
| POST | `/api/templates` | `templates.repo.create` | Añade un repositorio |
| POST | `/api/templates/upload` | `templates.local.edit` | Sube plantillas locales (archivos `.json`) |
| GET | `/api/templates/:repo` | `login` | Plantillas de un repositorio |
| DELETE | `/api/templates/:repo` | `templates.repo.delete` | Quita un repositorio |
| GET | `/api/templates/:repo/:name` | `login` | Una plantilla |
| PUT | `/api/templates/0/:name` | `templates.local.edit` | Crea o sustituye una plantilla local |
| DELETE | `/api/templates/0/:name` | `templates.local.edit` | Borra una plantilla local |
| GET | `/api/game-versions/:platform` | `login` | Versiones estables de una plataforma de Minecraft |
| GET | `/api/databasehosts` | `admin` | Lista los hosts MySQL |
| POST | `/api/databasehosts` | `admin` | Añade un host |
| POST | `/api/databasehosts/test` | `admin` | Prueba una conexión |
| GET | `/api/databasehosts/:id` | `admin` | Un host |
| PUT | `/api/databasehosts/:id` | `admin` | Actualiza un host |
| DELETE | `/api/databasehosts/:id` | `admin` | Borra un host |
| GET | `/api/extensions` | `admin` | Extensiones instaladas |
| POST | `/api/extensions` | `admin` | Instala o actualiza una extensión (zip) |
| GET | `/api/extensions/active` | `login` | Páginas de las extensiones encendidas |
| PUT | `/api/extensions/:id` | `admin` | Enciende o apaga una extensión, cambia el webhook |
| DELETE | `/api/extensions/:id` | `admin` | Desinstala |
| POST | `/api/extensions/:id/test` | `admin` | Envía un evento de prueba |
| GET, HEAD | `/api/extension-files/:id/*path` | Pública | Archivos de una extensión encendida |
| GET | `/api/uptime` | uno de `admin`, `uptime.view` | Disponibilidad de todos los servidores visibles |
| GET | `/api/uptime/:serverId` | `server.view` | Disponibilidad de un servidor |

## Nodos

Detalles del panel de inicio y de las páginas de nodos: [Panel de inicio y nodos](../../features/dashboard-and-nodes.md).

### El objeto nodo

```json
{ "id": 1, "name": "node-1", "publicHost": "node1.example.com", "privateHost": "10.0.0.5", "publicPort": 8080, "privatePort": 8080, "sftpPort": 5657, "isLocal": false }
```

`isLocal` señala el nodo que se ejecuta dentro del propio proceso del panel.

### `POST /api/nodes`

Cuerpo: el objeto nodo sin `id`. Al crearlo todos los campos son obligatorios: `name` (ASCII imprimible que no necesite escaparse en una URL, así que sin espacios), `publicHost` (IP o dominio), `privateHost` y los tres puertos (de 1 a 65535); `sftpPort` no puede coincidir con `publicPort` ni con `privatePort`. El panel genera el secreto del nodo. Responde con el objeto nodo. Los errores de validación son `400` con `ErrFieldRequired`, `ErrFieldIsInvalidHost`, `ErrFieldHasURICharacters`, `ErrFieldNotBetween` o `ErrFieldEqual`.

### `PUT /api/nodes/:id`

El mismo cuerpo; los campos vacíos y los puertos a `0` conservan su valor. Responde `204`.

### `DELETE /api/nodes/:id`

Responde `204`. Un nodo que todavía tiene servidores responde `500` con el código `ErrNodeHasServers`; el nodo propio del panel no se puede borrar (`500`).

### `GET /api/nodes/:id/deployment`

```json
{ "clientId": ".node_2", "clientSecret": "4f9c...", "publicKey": "" }
```

`clientId` y `clientSecret` van en `daemon.auth.clientId` y `daemon.auth.clientSecret` del nodo. `publicKey` siempre está vacío: el nodo lee las claves del panel en `GET /auth/publickey`.

### `POST /api/nodes/:id/join`

```json
{ "code": "k3v7…", "expiresAt": "2026-11-05T05:10:00Z", "panelUrl": "https://panel.ejemplo.com",
  "command": "curl -fsSL https://raw.githubusercontent.com/Aether-Panel/Panel/main/tools/install/install.sh | sudo bash -s -- --node --panel https://panel.ejemplo.com --join k3v7…" }
```

El código (32 caracteres) funciona una vez y durante una hora, y un código nuevo para el mismo nodo anula el anterior. El panel solo guarda su hash, en memoria: reiniciarlo anula los códigos pendientes. `panelUrl` es la URL principal de Configuración; si no está puesta o es local (`localhost`), responde `400 ErrNodeJoinURL`. El nodo propio del panel responde `400 ErrNodeJoinLocal`.

### `POST /api/node-join`

Público: lo llama el instalador del nodo con `{ "code": "…" }`.

```json
{ "nodeId": 2, "name": "eu-2", "clientId": ".node_2", "clientSecret": "4f9c…", "port": 8080, "sftpPort": 5657 }
```

`port` es el puerto privado del nodo (en el que escucha el daemon). Un código inexistente, usado o caducado responde `404 ErrNodeJoinCode`; más de 10 intentos por IP en 15 minutos, `429 ErrTooManyAttempts`.

### `GET /api/nodes/:id/features`

```json
{ "features": ["docker"], "environments": ["docker"], "os": "linux", "arch": "amd64", "version": "3.x.y" }
```

`features` incluye `docker` cuando el nodo puede comunicarse con Docker. Con `docker.disallowHost` activado, los entornos del host (`host`, `tty`, `standard`, `bubblewrap`) no aparecen en `environments`.

### `GET /api/nodes/:id/system`

```json
{
  "hostname": "node1", "os": "linux", "platform": "ubuntu", "platformVersion": "24.04", "arch": "amd64",
  "cpuModel": "AMD EPYC", "cpuCores": 4, "cpuThreads": 8, "cpuUsage": 12.5,
  "memoryTotal": 17179869184, "memoryUsed": 6442450944, "memoryFree": 2147483648, "memoryAvailable": 10737418240,
  "disks": [ { "path": "/", "total": 214748364800, "used": 53687091200, "free": 161061273600, "usedPercent": 25 } ],
  "serversDisk": { "path": "servers", "total": 214748364800, "used": 53687091200, "free": 161061273600, "usedPercent": 25, "low": false },
  "uptime": 864000, "networkBytesSent": 123456789, "networkBytesRecv": 987654321
}
```

`cpuUsage` se muestrea en segundo plano cada dos segundos. Cada disco físico aparece una sola vez. `serversDisk` es el disco que contiene las carpetas de los servidores; `low` es verdadero cuando queda menos de un 10 % o menos de 5 GB libres, y en ese caso el panel avisa a los administradores. `networkBytes*` suman las interfaces físicas; dentro de un contenedor son las del host cuando `daemon.hostProc` está definido.

### `POST /api/nodes/:id/ports/check`

Lo usa el formulario de nuevo servidor antes de crear nada.

```json
{ "ports": { "port": 25565, "queryport": 25565 }, "exclude": "" }
```

`ports` asocia cada variable de puerto del formulario con el puerto que pide; `exclude` es un servidor que se está editando, cuyos propios puertos cuentan como libres.

```json
{ "ports": { "port": { "busy": true, "reason": "server", "by": "Survival", "suggestion": 25566 }, "queryport": { "busy": false } } }
```

`reason` es `range` (por debajo de 1024), `panel` (los puertos propios del nodo y los contenedores del panel), `server` o `container`. `by` indica quién ocupa el puerto y solo se envía a los administradores. `suggestion` es el siguiente puerto que nadie ocupa y que el formulario no pide. Los puertos ocupados salen de los registros del panel y de los contenedores que el nodo indica en ejecución.

### `GET /api/nodes/network-history?range=1h`

`range` es `1h`, `6h` o `24h` (un punto cada 1, 5 o 15 minutos; cualquier otro valor responde `400`). Cada nodo guarda en disco un día de tráfico y lo sirve en `GET /daemon/network/history`; el panel suma los nodos paso a paso.

```json
{ "step": 60, "points": [ { "t": 1793576400, "rx": 0.49, "tx": 0.37 } ], "nodes": 2, "unavailable": 0 }
```

`rx` y `tx` son KB/s promediados en cada paso. Un paso que ningún nodo midió no tiene punto. `unavailable` cuenta los nodos que no respondieron (caídos, o demasiado antiguos para guardar historial).

## Configuración

Qué hace cada ajuste: [Configuración del panel](../../features/panel-settings.md) y [Configuración](../../backend/02-configuration.md).

### `GET /api/settings`

Un objeto con cada clave editable y su valor actual, más `panel.settings.masterNodeIp` (que se guarda en la base de datos, no en el archivo de configuración).

Los secretos nunca se devuelven: un secreto guardado se lee como `********` y uno sin definir como `""`, para que un cliente sepa si está definido. Las claves enmascaradas son `panel.email.key`, `panel.email.password`, `panel.settings.geminiApiKey`, `daemon.curseforge.key`, las cuatro claves `panel.notifications.discordWebhook*`, `panel.license.key`, `daemon.backups.s3.secretKey`, `panel.dns.cloudflare.token` y `panel.turnstile.secretKey`. `GET /api/settings/:key` las enmascara igual.

Claves editables:

| Grupo | Claves |
|-------|--------|
| Marca | `panel.settings.companyName`, `panel.settings.defaultTheme`, `panel.settings.themeSettings` |
| Direcciones | `panel.settings.masterUrl` (se guarda sin barra final y con `https://` delante si llega solo el nombre, `panel.ejemplo.com`), `panel.settings.nodeIp`, `panel.settings.masterNodeIp` |
| Registro y antibots | `panel.registrationEnabled`, `panel.registrationApproval`, `panel.turnstile.enabled`, `panel.turnstile.siteKey`, `panel.turnstile.secretKey`, `panel.captcha.builtin` |
| Correo | `panel.email.provider`, `panel.email.from`, `panel.email.host`, `panel.email.username`, `panel.email.password`, `panel.email.domain`, `panel.email.key` |
| Notificaciones | `panel.settings.notificationLanguage` (`es` o `en`, por defecto `es`: el idioma de los avisos de Discord, Telegram y webhooks, de los informes de Discord del panel y de los eventos de las extensiones), `panel.notifications.discordWebhook`, `panel.notifications.discordWebhookSystem`, `panel.notifications.discordWebhookNode`, `panel.notifications.discordWebhookExTransfer` |
| IA e integraciones | `panel.settings.geminiApiKey`, `panel.settings.hideAiAnalysis`, `daemon.curseforge.key` |
| Experimental | `panel.experimental.versionSwitch`, `panel.experimental.propertiesEditor`, `panel.experimental.serverClone` |
| Copias externas | `daemon.backups.s3.enabled`, `daemon.backups.s3.endpoint`, `daemon.backups.s3.region`, `daemon.backups.s3.bucket`, `daemon.backups.s3.accessKey`, `daemon.backups.s3.secretKey`, `daemon.backups.s3.prefix`, `daemon.backups.s3.insecure`, `daemon.backups.s3.periodicHours` (`0`, `12`, `24`, `48`, `72`, `168`, `336` o `720`; otro valor: `400 ErrOffsitePeriodicHours`), `daemon.backups.s3.periodicKeep` (de 1 a 50; otro valor: `400 ErrOffsiteKeep`) |
| Subdominios | `panel.dns.cloudflare.token`, `panel.dns.cloudflare.zoneId`, `panel.dns.baseDomain`, `panel.dns.target`, `panel.dns.autoAssign` |
| GitHub | `panel.github.clientId` |
| Licencia | `panel.license.key`, `panel.license.status`, `panel.license.serverId`, `panel.license.serverIp` |

Las demás claves de configuración (por ejemplo `panel.backups.maxPerServer`) solo se definen en el archivo de configuración o mediante variables de entorno.

### `POST /api/settings`

Un objeto de claves y valores: `{"panel.settings.companyName": "My Hosting", "panel.registrationEnabled": false}`. Todos los valores se comprueban antes de guardar (los webhooks de Discord deben ser URL de webhook de Discord válidas y `panel.settings.notificationLanguage` debe ser `es` o `en`; si no, `400`, con `ErrNotificationLanguage` en el segundo caso), así que un valor incorrecto hace que no se guarde nada. A los valores de texto se les quitan los espacios de los extremos; las claves desconocidas se ignoran. Un secreto enviado como `********` (la máscara, sin cambiar) se descarta y conserva su valor guardado. Responde `204`. Cuando lo guardado incluye un ajuste de nodo (`daemon.backups.s3.*` o `daemon.curseforge.key`), el panel los envía después a todos los nodos remotos en segundo plano, sin hacer esperar la respuesta (véase [`GET /api/settings/node-sync`](#get-apisettingsnode-sync)).

### `GET /api/settings/:key` y `PUT /api/settings/:key`

`GET` responde `{"value": ...}` (con los secretos enmascarados como se indica arriba), o `204` si la clave no es editable. `PUT` recibe `{"value": ...}` y responde `204`; para una clave secreta, un valor `********` no cambia nada. Un ajuste de nodo guardado así también llega a los nodos remotos, igual que con `POST /api/settings`.

### Pruebas

Las rutas de prueba reciben como sustituciones los valores escritos en el formulario y aún no guardados (con los mismos nombres de clave); para los que faltan se usan los valores guardados, igual que para un secreto devuelto como `********`.

| Ruta | Sustituciones | Respuesta |
|------|---------------|-----------|
| `POST /test/email` | `panel.email.*` | `204`, enviado al correo de quien llama; `422` con el motivo del servidor de correo |
| `POST /test/discord` | Las cuatro claves `panel.notifications.discordWebhook*` | `204`; `400 ErrDiscordNoWebhooks` si no hay ninguno configurado, `400` si alguno no es válido; `422 ErrDiscordTestFailed` con los webhooks a los que no llegó el mensaje en `metadata.webhooks` (`alerts`, `reports`, `status`, `transfers`, que la página de ajustes muestra con los nombres de sus secciones). El mensaje se escribe en el idioma de las notificaciones |
| `POST /test/s3` | `daemon.backups.s3.*` | `204`, tras escribir y borrar un objeto pequeño; `400` si no está configurado; `422` con la respuesta del almacenamiento |
| `POST /test/cloudflare` | `panel.dns.cloudflare.token`, `panel.dns.cloudflare.zoneId`, `panel.dns.baseDomain` | `200 {"zone": ...}` tras crear y quitar un registro TXT; `400` si no está configurado; `422` con la respuesta de Cloudflare |

### `POST /api/settings/license/activate`

`{"key": "XXXX-XXXX-XXXX-XXXX"}` (16 caracteres sin guiones). Verifica la clave con el servidor de licencias, la vincula y la guarda. `200 {"success": true, "type": "...", "license": {...}, "permissions": ..., "bound": true, "message": "License activated successfully"}`; `400` con `{"success": false, "message": "..."}` si no es válida. Más detalles: [Sistema de licencias](../../general/14-license.md).

### Actualizaciones

- `POST /api/settings/update-panel` inicia una actualización en el host del panel y en todos los nodos (`/daemon/update`). `200 {"message": "Update initiated on the panel and all nodes.", "nodes": [{"name": "node-1", "local": true}, {"name": "node-2", "error": "node returned status 500"}]}`.
- `GET /api/settings/update-check` compara el commit en ejecución con el último de su rama en GitHub: `{"current": "<sha>", "latest": "<sha>", "version": "3.x.y", "branch": "main", "updateAvailable": true, "behindBy": 3}`. `reason` (`github_unreachable`, `github_rate_limited`, `github_error`) explica una comprobación fallida; `verification` (`pending` o `failed`) retiene una actualización cuyas comprobaciones de CI no han pasado.
- `GET /api/settings/update-status`: `{"nodes": [{"name", "address", "local", "online", "running", "containerId", "startedAt", "finishedAt", "exitCode", "log", "error"}]}`.

Más detalles: [Actualizaciones del panel](../../features/panel-updates.md).

### `GET /api/settings/domain-status`

Qué hizo el servicio de dominio del instalador con la URL base: `{"state": "...", "url": "...", "domain": "...", "https": true, "proxied": false, "serverIp": "...", "message": "...", "updatedAt": "..."}`. `proxied` indica que el dominio no apunta a este servidor (como ocurre con el proxy de Cloudflare), por lo que los puertos de juego y el SFTP usan `serverIp`.

### `GET /api/settings/node-sync`

Si cada nodo remoto tiene los ajustes de nodo del panel: las copias externas (`daemon.backups.s3.*`, incluida la copia periódica por defecto) y la clave de CurseForge (`daemon.curseforge.key`). El panel se los envía él mismo a cada nodo remoto (`PUT /daemon/settings`, véase [Daemon](./09-daemon.md#put-daemonsettings)) después de un guardado que cambie alguno y cada 10 minutos, la primera vez un minuto después de arrancar. El nodo propio del panel comparte su `config.json` y no aparece.

```json
[
  { "id": 2, "name": "node-2", "state": "synced", "checkedAt": "2026-11-06T10:00:00Z" },
  { "id": 3, "name": "node-3", "state": "failed", "error": "the off-site copies to keep must be between 1 and 50", "checkedAt": "2026-11-06T10:00:01Z" }
]
```

Un elemento por nodo remoto, por orden de id; `[]` si no hay nodos remotos. Nunca incluye el valor de un ajuste.

| `state` | Significado |
|---------|-------------|
| `synced` | El nodo tiene los mismos valores que el panel |
| `pending` | Aún sin comprobar (`checkedAt` es `null`), o el nodo no respondió o no pudo guardar; la siguiente pasada lo reintenta sola |
| `outdated` | El nodo tiene una versión sin `/daemon/settings` (respondió `404`) y hay que actualizarlo |
| `failed` | El nodo rechazó al panel (otro secreto, relojes desfasados) o los ajustes; `error` dice por qué |

## Claves de API

Claves para las rutas de facturación (`/api/v1`). Cómo se usan y se limitan: [Aprovisionamiento / facturación](./07-provisioning.md#autenticación).

- `GET /api/settings/apikeys`: `[{"id": 1, "name": "WHMCS", "prefix": "ak_a1b2c", "permissions": ["create", "suspend"], "created_at": "..."}]`.
- `POST /api/settings/apikeys`: `{"name": "WHMCS", "permissions": ["create", "terminate", "suspend", "change-package", "sso"]}`. `name` es obligatorio. Los permisos repetidos o desconocidos se ignoran; una lista sin ningún permiso conocido responde `400 ErrAPIKeyPermissions`; sin lista, la clave puede realizar todas las acciones (`provision`). Responde `{"token": "ak_...", "key": {...}}`; el token solo se muestra aquí.
- `DELETE /api/settings/apikeys/:id`: `204`.

## Plantillas

Las plantillas son definiciones de servidor. El repositorio `0` es el local, guardado en la base de datos del panel. Cuando `templates.url` está definido, la lista es ese índice (mostrado como repositorio `1`, «community») más el local, y no se usan los repositorios de la base de datos. Más detalles: [Plantillas](../../features/templates.md).

Leer plantillas solo requiere `login`, porque los formularios de nuevo servidor y de Splitter las leen; `templates.view` solo muestra la página de plantillas en el cliente web.

| Ruta | Notas |
|------|-------|
| `GET /api/templates` | `[{"id": 0, "name": "Local", "url": "", "branch": "", "isLocal": true}]` |
| `POST /api/templates` | `{"name": "mine", "url": "https://github.com/me/templates.git", "branch": "main"}`, más credenciales opcionales para un repositorio privado: `pat`, `username`, `password`, `sshKey` (véase abajo). Responde con el repositorio, nunca con sus credenciales; `400 ErrFieldRequired` sin `name` |
| `GET /api/templates/:repo` | Las plantillas del repositorio |
| `GET /api/templates/:repo/:name` | Una plantilla: la definición tal como la escribió su autor (mismo orden de claves, con los campos que el panel no conoce), seguida de `name` y `readme`; `404` si no existe |
| `DELETE /api/templates/:repo` | `204` |
| `PUT /api/templates/0/:name` | Cuerpo: una definición (un objeto JSON). Crea o sustituye la plantilla local. Se comprueba como antes (`400` si no se lee como definición de servidor) y se guarda tal como llega, sin sus claves `name` y `readme`: el nombre lo da la URL y el readme guardado se conserva. `204`; `413 ErrTemplateTooLarge` si el cuerpo pasa de 2 MB |
| `DELETE /api/templates/0/:name` | `204`; `404` si no existe |
| `POST /api/templates/upload` | Campo multipart `files`, uno o varios archivos `.json` de hasta 2 MB; el nombre de la plantilla es el del archivo (`minecraft.json` → `minecraft`). El archivo se guarda tal como está (si le falta `display`, se añade al final). Responde con las plantillas creadas; `400 ErrInvalidTemplate` para un archivo que no es JSON o que no tiene ni pasos de instalación ni comando de ejecución |

Repositorios privados: por HTTPS, `pat` (un token de acceso personal, enviado con el usuario `username`, o `git` si está vacío) o `username` + `password`; por SSH (`git@host:propietario/repo.git` o `ssh://...`), `sshKey` contiene la clave privada, `password` su frase de paso, y el usuario es `username`, si no el de la URL, si no `git`. Para SSH el host debe estar en el `known_hosts` de la cuenta que ejecuta el panel. El cliente web aún no tiene un formulario para añadir repositorios. Funcionamiento interno: [Servicios → Template](../../backend/11-services.md#11-template-templatesgo).

### `GET /api/game-versions/:platform`

Las versiones estables de `vanilla`, `paper`, `folia`, `velocity` o `waterfall`, de la más reciente a la más antigua: `["1.21.4", "1.21.3", ...]`. `400` para cualquier otra plataforma; `502` si no se puede contactar con Mojang o PaperMC.

## Hosts de bases de datos

Servidores MySQL en los que el panel crea las bases de datos de los servidores. Más detalles: [Hosts de bases de datos](../../features/database-hosts.md).

```json
{ "id": 1, "name": "Main MySQL", "host": "db1.example.com", "port": 3306, "username": "panel", "max_databases": 50, "node_id": null, "created_at": "...", "updated_at": "..." }
```

La contraseña nunca se devuelve. `max_databases` a `0` significa sin límite; `node_id` vincula el host a los servidores de un nodo (`null`: todos los nodos).

- `POST /api/databasehosts`: el objeto más `password`; `name`, `host`, `port`, `username` y `password` son obligatorios. Responde con el host.
- `PUT /api/databasehosts/:id`: los mismos campos; una `password` vacía conserva la actual. `204`.
- `POST /api/databasehosts/test`: `{"host": "...", "port": 3306, "username": "...", "password": "..."}` (el puerto es 3306 por defecto). `200 {"status": "ok"}` o `400` con el error de conexión.
- `DELETE /api/databasehosts/:id`: `204`; `409 ErrDatabaseHostInUse` mientras el host aún tenga bases de datos (hay que eliminarlas antes).

## Extensiones

Los paquetes de extensión, sus páginas y sus eventos se describen en [Extensiones](../15-extensions.md).

```json
{
  "id": "hello", "name": "Hello", "version": "1.0.0", "author": "...", "description": "...",
  "enabled": false, "webhookUrl": "https://...", "secret": "...", "installedAt": "...", "updatedAt": "...",
  "pages": [ { "...": "...", "url": "/api/extension-files/hello/index.html" } ],
  "events": ["server.created"],
  "manifestWebhook": "https://..."
}
```

- `GET /api/extensions` las lista con sus páginas, sus eventos y el secreto de su webhook.
- `POST /api/extensions` recibe el zip en el campo multipart `file`. Instalar un id que ya existe lo actualiza; una extensión nueva empieza apagada. Un paquete incorrecto responde `400 {"error": {"code": "ErrExtensionInvalid", "msg": "..."}}`.
- `GET /api/extensions/active`: `[{"id", "name", "pages"}]` de las extensiones encendidas; las páginas de administración solo se listan para los administradores.
- `PUT /api/extensions/:id`: `{"enabled": true, "webhookUrl": "https://..."}` (ambos opcionales). Responde con la extensión; `400` si el webhook no es válido; `404`.
- `DELETE /api/extensions/:id`: `204`; `404`.
- `POST /api/extensions/:id/test`: envía un evento `test` firmado; `204`, o `400` con `ErrExtensionNoWebhook` o `ErrExtensionWebhook`.
- `GET /api/extension-files/:id/*path` es pública y solo sirve extensiones encendidas, siempre con una Content-Security-Policy de sandbox.

## Disponibilidad

Más detalles: [Seguimiento de disponibilidad (uptime)](../../general/13-uptime.md).

Los servidores del nodo propio del panel los registra el panel cada pocos segundos; los de los nodos remotos, un sondeo que pregunta a cada nodo cada 30 segundos (`GET /daemon/servers/status`). Mientras un nodo remoto no responde no se registra nada de él, y tras 3 minutos sin respuesta sus periodos abiertos se cierran en su última confirmación, así que la caída no cuenta ni como tiempo en línea ni como tiempo caído. Los periodos que se solapan con la ventana solo cuentan por la parte que cae dentro, y el periodo en curso hasta ahora.

### `GET /api/uptime?days=30`

`days` vale 30 por defecto, con un máximo de 365. Devuelve un objeto indexado por id de servidor; los usuarios sin `admin` solo reciben sus propios servidores.

```json
{
  "3F9A1C2B7D4E": { "serverName": "Survival", "nodeName": "LocalNode", "uptime": 2505600, "downtime": 86400, "uptimePercent": 96.67, "currentStatus": true, "currentStartTime": "..." }
}
```

`currentStatus` se consulta a cada nodo (como mucho 5 segundos por nodo); los servidores sin historial reciben una entrada construida a partir de su estado actual.

### `GET /api/uptime/:serverId?days=30&limit=50`

```json
{
  "serverId": "3F9A1C2B7D4E", "serverName": "Survival", "nodeName": "LocalNode",
  "currentStatus": true, "currentStartTime": "...", "currentUptime": 3600,
  "uptimeSeconds": 2505600, "downtimeSeconds": 86400, "totalSeconds": 2595600, "uptimePercent": 96.67,
  "period": { "days": 30, "since": "...", "until": "..." },
  "history": [ ... ]
}
```

`limit` es el número de entradas del historial: 50 por defecto, 200 como máximo. `totalSeconds` es `uptimeSeconds + downtimeSeconds`; ambos incluyen ya el periodo en curso, que `currentUptime` muestra por separado.
