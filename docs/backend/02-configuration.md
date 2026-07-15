# Configuración

El sistema usa **Viper** con prefijo `PUFFER_`. Las variables de entorno tienen prioridad sobre el archivo de configuración.

## Archivo de Configuración

Rutas de búsqueda (por orden): `--config` flag > `PUFFER_CONFIG` env > `/etc/SkyPanel/config.json` > `./config.json`

Formato: JSON (compatible con Viper, también YAML/TOML).

## Variables de Entorno

Todas las claves anidadas con `.` se convierten a `_`. Prefijo `PUFFER_`:

| Variable | Clave Viper |
|---|---|
| `PUFFER_WEB_HOST` | `web.host` |
| `PUFFER_PANEL_DATABASE_DIALECT` | `panel.database.dialect` |
| `PUFFER_DAEMON_SFTP_HOST` | `daemon.sftp.host` |

## Tabla Completa de Configuración

### Web

| Clave | Tipo | Default | Descripción |
|---|---|---|---|
| `web.host` | string | `0.0.0.0:8080` | Dirección y puerto del servidor HTTP |

### Panel

| Clave | Tipo | Default | Descripción |
|---|---|---|---|
| `panel.enable` | bool | `true` | Habilita el modo panel (API, frontend) |
| `panel.database.dialect` | string | `sqlite3` | Dialecto: `sqlite3`, `mysql`, `postgresql`, `sqlserver` |
| `panel.database.url` | string | — | Connection string (default según dialecto) |
| `panel.database.log` | bool | `false` | Habilita logging detallado de GORM (SQL) |
| `panel.web.files` | string | `www` | Ruta a archivos frontend (override del FS embebido) |
| `panel.web.cookies.secure` | bool | `false` | Cookie Secure flag |
| `panel.web.cookies.httpOnly` | bool | `true` | Cookie HttpOnly flag |
| `panel.web.cookies.domain` | string | — | Cookie Domain |
| `panel.web.cookies.age` | int | `2592000` (30d) | Cookie Max-Age en segundos |
| `panel.web.cookies.sameSite` | string | `Strict` | Cookie SameSite |
| `panel.web.cookies.path` | string | `/` | Cookie Path |
| `panel.email.templateFolder` | string | — | Carpeta de plantillas de email |
| `panel.email.provider` | string | — | Proveedor de email (`mailgun`, `smtp`) |
| `panel.email.from` | string | — | Dirección From |
| `panel.email.domain` | string | — | Dominio (Mailgun) |
| `panel.email.host` | string | — | Host SMTP |
| `panel.email.key` | string | — | API Key (Mailgun) o password |
| `panel.email.username` | string | — | Usuario SMTP |
| `panel.email.password` | string | — | Contraseña SMTP |
| `panel.settings.companyName` | string | `SkyPanel` | Nombre de la empresa/marca |
| `panel.settings.defaultTheme` | string | `SkyPanel` | Tema por defecto |
| `panel.settings.themeSettings` | string | `{}` | Configuración del tema (JSON) |
| `panel.settings.masterUrl` | string | `http://localhost:8080` | URL del panel maestro |
| `panel.settings.nodeIp` | string | `0.0.0.0` | IP del nodo |
| `panel.settings.geminiApiKey` | string | — | API Key de Google Gemini |
| `panel.settings.hideAiAnalysis` | bool | `false` | Oculta la funcionalidad de AI Analysis |
| `panel.notifications.discordWebhook` | string | — | Webhook de Discord (notificaciones) |
| `panel.notifications.discordWebhookSystem` | string | — | Webhook de Discord (sistema) |
| `panel.notifications.discordWebhookNode` | string | — | Webhook de Discord (nodos) |
| `panel.license.key` | string | — | Clave de licencia |
| `panel.license.status` | string | `free` | Estado de licencia |
| `panel.license.serverId` | string | — | ID del servidor para licencia |
| `panel.license.serverIp` | string | — | IP del servidor para licencia |
| `panel.sessionKey` | string | — | Clave para cifrado de sesiones (autogenerada) |
| `panel.registrationEnabled` | bool | `true` | Permite registro de usuarios |
| `panel.token` | string | — | Clave privada Ed25519 (JWT) |

### Templates

| Clave | Tipo | Default | Descripción |
|---|---|---|---|
| `templates.url` | string | — | URL de repositorio de plantillas VPS JSON |

### Daemon

| Clave | Tipo | Default | Descripción |
|---|---|---|---|
| `daemon.enable` | bool | `true` | Habilita el modo daemon |
| `daemon.console.buffer` | int | `50` | Tamaño del buffer de consola en memoria |
| `daemon.console.forward` | bool | `false` | Reenvía consola al panel |
| `daemon.sftp.host` | string | `0.0.0.0:5657` | Dirección del servidor SFTP |
| `daemon.sftp.key` | string | `sftp.key` | Ruta a la clave privada SFTP |
| `daemon.sftp.log` | bool | `false` | Logging detallado de SFTP |
| `daemon.auth.url` | string | `http://localhost:8080` | URL del panel para autenticación |
| `daemon.auth.clientId` | string | — | Client ID OAuth2 para daemon |
| `daemon.auth.clientSecret` | string | — | Client Secret OAuth2 para daemon |
| `daemon.data.cache` | string | `cache` | Carpeta de caché |
| `daemon.data.servers` | string | `servers` | Carpeta de servidores |
| `daemon.data.backups.folder` | string | `backups` | Carpeta de backups |
| `daemon.data.binaries` | string | `binaries` | Carpeta de binarios |
| `daemon.data.crashLimit` | int | `3` | Límite de crashes antes de suspender |
| `daemon.data.root` | string | — | Raíz de datos (por defecto: parent de servers) |
| `daemon.curseforge.key` | string | — | API Key de CurseForge |
| `daemon.depotDownloader.version` | string | `latest` | Versión de DepotDownloader |
| `daemon.depotDownloader.disableLancache` | bool | `false` | Deshabilita Lancache |

### Seguridad

| Clave | Tipo | Default | Descripción |
|---|---|---|---|
| `security.forceOpenat` | bool | `false` | Fuerza uso de openat() en lugar de open() |
| `security.trustedProxies` | []string | `[]` | Proxies de confianza |
| `security.trustedProxyHeader` | string | — | Header de proxy confiable |
| `security.disableUnshare` | bool | `false` | Deshabilita unshare de namespaces |

### Docker

| Clave | Tipo | Default | Descripción |
|---|---|---|---|
| `docker.root` | string | — | Ruta raíz de Docker |
| `docker.disallowHost` | bool | `false` | Deshabilita entorno host (fuerza Docker) |

### Logs

| Clave | Tipo | Default | Descripción |
|---|---|---|---|
| `logs` | string | `logs` | Carpeta de logs |

## Claves Generadas Automáticamente

- `panel.sessionKey` — Se genera automáticamente si no está configurada (usando `securecookie.GenerateRandomKey`)
- `panel.token` — Clave privada Ed25519 para JWT (se genera automáticamente)
- `daemon.data.root` — Se calcula como `filepath.Dir(daemon.data.servers)` si está vacío

## Sistema de Config Editables

El endpoint `GET /api/config` expone una versión filtrada de la configuración (branding, temas, registro). Los cambios en `panel.settings.*` y `panel.notifications.*` se pueden realizar mediante `POST /api/settings` y se persisten al archivo de configuración.
