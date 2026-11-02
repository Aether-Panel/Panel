# Base de datos

La base de datos del panel (`internal/database`, `internal/models`) guarda usuarios, roles, permisos, nodos, filas de servidores, sesiones, plantillas y el resto del estado del panel. Solo existe donde corre el panel (`panel.enable`); un nodo sin panel no tiene base de datos. Las definiciones, archivos, tareas y copias de seguridad de los servidores viven en los nodos (ver [Servidores](./04-servers.md#archivos-de-cada-servidor-en-el-nodo)).

## Dialectos soportados

Se eligen con `panel.database.dialect` y `panel.database.url` (ver [Configuración](./02-configuration.md#base-de-datos)).

| Dialecto | Driver | Cadena de conexión si `panel.database.url` está vacía |
|---|---|---|
| `sqlite3` | `gorm.io/driver/sqlite` | `file:skypanel.db` |
| `mysql` | `gorm.io/driver/mysql` | `SkyPanel:SkyPanel@/SkyPanel` |
| `postgresql` | `gorm.io/driver/postgres` | — (obligatoria) |
| `sqlserver` | `gorm.io/driver/sqlserver` | — (obligatoria) |

Cualquier otro valor falla con `unknown dialect`. La instalación con Docker usa MariaDB mediante el dialecto `mysql`; `config.linux.json` usa SQLite en `/var/lib/SkyPanel/database.db`.

### Parámetros que se añaden a la cadena de conexión

`GetConnectionString()` añade estos parámetros salvo que la cadena ya los tenga:

```text
mysql:   charset=utf8  parseTime=true
sqlite3: cache=shared  _loc=auto  _foreign_keys=1  _journal_mode=WAL  _busy_timeout=5000  _tx_lock=immediate
```

Con SQLite el pool se limita a una conexión abierta y las migraciones no crean claves foráneas (`DisableForeignKeyConstraintWhenMigrating`). GORM no registra nada salvo con `panel.database.log`, que registra cada sentencia.

## Migraciones

`database.Upgrade()` se ejecuta en cada arranque del panel (y desde los comandos `db` de la CLI, ver [CLI](./07-cli.md)). Usa **gormigrate** con la tabla `migrations` (columna `id`, 255 caracteres); cada migración se ejecuta sola en su propia transacción y las ya registradas se omiten. Se agrupan en tres lotes, y los modelos se migran automáticamente (`AutoMigrate` de todas las tablas de abajo) justo después del primer lote.

| Lote | ID | Qué hace |
|---|---|---|
| 1 | `1726675832-mysql` | Solo MySQL: elimina todas las claves foráneas para que la migración de modelos las reconstruya. |
| 1 | `1726675832` | Elimina todos los índices que no son clave primaria; los reconstruye la migración de modelos. |
| 1 | `20260304-serverid-harmonization` | MySQL: elimina las claves foráneas hacia `servers.identifier` para poder redimensionar a 20 caracteres las columnas que lo referencian. |
| 1 | `20260819-server-extra-ports` | Añade la columna `ports` a las tablas `servers` existentes. |
| 1 | `20261016-server-internal-id` | Añade `internal_id` como anulable, rellena un UUIDv7 en cada servidor existente y después fija `NOT NULL` (MySQL, PostgreSQL). |
| — | migración de modelos | `AutoMigrate` de todos los modelos. |
| 2 | `1658926619` | Se mantiene sin efecto (antes creaba un repositorio de plantillas por defecto). |
| 2 | `1677250619` | Reescribe a `host` las plantillas guardadas cuyo entorno era `tty` o `standard`; una plantilla ilegible se guarda en `migrations/template-<nombre>.json` y se elimina. |
| 2 | `permissions-from-v2` | Convierte las antiguas columnas booleanas de permisos en scopes. |
| 3 | `20260617-default-roles` | Crea los roles `Administrador` (permiso `admin`) y `Usuario`. |
| 3 | `20260624-usuario-role-templates-view` | Añade `templates.view` y `uptime.view` a `Usuario`. |
| 3 | `20260625-assign-usuario-role-to-existing-users` | Da el rol `Usuario` a los usuarios sin rol. |
| 3 | `20260625-usuario-role-add-server-create` | Añade `server.create` a `Usuario` (sub-servidores). |
| 3 | `20260820-usuario-role-config-ports` | Añadía tres permisos a `Usuario`; la siguiente migración lo revierte. |
| 3 | `20260821-fix-usuario-role-scopes` | Quita a `Usuario` `server.admin.config.view`, `server.admin.config.manage` y `server.data.edit.admin`, y le añade `server.definition.view/edit` y `server.flags.view/edit`. |
| 3 | `20260811-template-raw-value-size` | Convierte `templates.raw_value` en `MEDIUMTEXT` (MySQL) para no truncar plantillas grandes. |
| 3 | `20260821-server-port-notes` | Vuelve a migrar la tabla `servers` (notas de puertos). |
| 3 | `20261026-server-owners` | Añade `servers.owner_id` y asigna dueño a cada servidor que no lo tiene: el miembro con más permisos sobre él y, en caso de empate, el más antiguo. |
| 3 | `20261016-server-internal-id-verify` | Comprobación de solo lectura de que ningún servidor carece de `internal_id`; solo registra. |
| 3 | `20261102-node-transfers` | `AutoMigrate` de `NodeTransfer`: garantiza que existe la tabla `node_transfers` (la migración de modelos, que también incluye el modelo, normalmente ya la crea antes). |

## Tablas

`AutoMigrate` cubre 26 modelos (`dbObjects` en `internal/database/upgrade.go`). Las columnas marcadas como *json* usan el serializador JSON de GORM.

### Cuentas y acceso

| Modelo (tabla) | Archivo | Clave primaria | Columnas principales |
|---|---|---|---|
| `User` (`users`) | `user.go` | `id` | `username` (100, única), `email` (255, única), `password` (hash bcrypt), `otp_secret`, `otp_active`, `otp_pending_secret`, `role_id` (anulable), fechas |
| `Role` (`roles`) | `role.go` | `id` | `name` (100, única), `description` (500), `scopes` (separados por comas, 2000), fechas |
| `Permissions` (`permissions`) | `permission.go` | `id` | `user_id` o `client_id` (anulables), `server_identifier` (20, anulable: vacío significa global), `scopes` (separados por comas, 1000) |
| `Session` (`sessions`) | `session.go` | `id` | `token` (SHA-256 del token, único), `expiration_time`, `user_id` / `client_id` (anulables), `server_identifier` (anulable) |
| `Client` (`clients`) | `client.go` | `id` | `client_id` (100, único), `client_secret` (hash bcrypt), `user_id`, `server_id` (anulable), `name`, `description`, `scopes` (solo lectura) |
| `RecoveryCode` (`recovery_codes`) | `recoverycode.go` | `id` | `user_id`, `code` (hash BLAKE2b-256) |
| `PasswordReset` (`password_resets`) | `passwordreset.go` | `id` | `user_id`, `token` (hash BLAKE2b-256, único), `expires_at` (30 minutos) |
| `SSHKey` (`ssh_keys`) | `sshkey.go` | `id` | `user_id`, `name`, `fingerprint` (SHA256, única por usuario), `public_key`, `type` |
| `APIKey` (`api_keys`) | `apikey.go` | `id` | `name`, `hashed_key` (SHA-256), `prefix` (8), `permissions` (*json*, acciones permitidas) |
| `UserSetting` (`user_settings`) | `usersetting.go` | `key` + `user_id` | `value` (4000) |
| `ActivityLog` (`activity_logs`) | `activity.go` | `id` | `event` (`login`, `login_failed`, `logout`, `register`, `password_reset`, `login_sso`), `user_id` (anulable), `username` y `email` (copiados), `ip`, `user_agent`, `created_at` |

### Servidores y nodos

| Modelo (tabla) | Archivo | Clave primaria | Columnas principales |
|---|---|---|---|
| `Server` (`servers`) | `server.go` | `identifier` (20) | `internal_id` (UUIDv7, único, no nulo), `name` (40), `node_id` (anulable, solo se escribe al crear), `ip`, `port`, `ports` (*json*), `port_notes` (*json*), `type`, `icon`, `parent_server_id` (sub-servidores), `total_cpu`, `total_memory`, `total_disk`, `suspended`, `owner_id` (anulable), `external_id` (facturación), `subdomain` (63), `dns_record_ids` (*json*), fechas |
| `Node` (`nodes`) | `node.go` | `id` | `name` (100, único), `public_host`, `private_host`, `public_port` (8080), `private_port` (8080), `sftp_port` (5657), `secret`, `ssl`, fechas |
| `ServerActivity` (`server_activities`) | `serveractivity.go` | `id` | `server_id`, `user_id`, `username` (copiado), `action` (p. ej. `server.start`, `file.write`), `detail` (300), `ip`, `created_at` |
| `UptimeStatus` (`uptime_statuses`) | `uptime.go` | `id` | `server_id`, `is_running`, `start_time`, `end_time` (anulable), `duration` (segundos), fechas |
| `Backup` (`backups`) | `backup.go` | `id` | `name`, `file_name`, `file_size`, `location` (`local`, `offsite`, `both`), `server_id`, `created_at` |
| `NodeTransfer` (`node_transfers`) | `nodetransfer.go` | `id` | `server_id` (20, único: el servidor para el que se pidió la transferencia), `target_node_id`, `user_id` (anulable: quién la pidió), `committed` (se marca en la misma transacción que cambia `node_id`), `moves` (*json*: por servidor, `serverId`, `sourceNodeId`, `step` alcanzado, `wasRunning`, `copiedBackups`), fechas. Una fila por cada [transferencia entre nodos](../features/server-transfers.md#recuperación-tras-un-reinicio-del-panel) en curso, para que un panel que se reinicia pueda deshacerla o completarla |
| `ProvisionProduct` (`provision_products`) | `product.go` | `id` | `product_id` (único), `display_name`, `template`, `node_id` (anulable: se elige automáticamente), `cpu` (100 = un núcleo), `memory` (MB), `disk` (MB), `default_node`, `port_range_min`, `port_range_max` |

El **nodo local** (el daemon que corre en el mismo binario que el panel) no es una fila: es `models.LocalNode` (ID `0`), construido a partir de `panel.settings.masterUrl`, `daemon.sftp.host` y la IP del nodo maestro, con un secreto derivado de `panel.sessionKey`. Guardarlo se rechaza.

Los identificadores de servidor se limitan a un conjunto de caracteres seguro para URL y rutas (los nuevos tienen 12 caracteres hexadecimales en mayúsculas); `internal_id` es un UUIDv7 inmutable que también se devuelve a los clientes como `internalId`.

### Bases de datos para servidores de juego

| Modelo (tabla) | Archivo | Clave primaria | Columnas principales |
|---|---|---|---|
| `DatabaseHost` (`database_hosts`) | `databasehost.go` | `id` | `name`, `host`, `port` (3306), `username`, `password`, `max_databases`, `node_id` (anulable), fechas |
| `Database` (`databases`) | `database.go` | `id` | `server_id` (FK, en cascada), `database_host_id` (FK, en cascada), `database_name`, `username`, `password`, `remote_connection`, `max_connections`, fechas |

### Plantillas

| Modelo (tabla) | Archivo | Clave primaria | Columnas principales |
|---|---|---|---|
| `Template` (`templates`) | `template.go` | `name` (100) | `raw_value` (`mediumtext`, el JSON de la plantilla), `readme` |
| `TemplateRepo` (`template_repos`) | `templaterepo.go` | `id` | `name`, `url`, `branch` (`main`), `pat`, `username`, `password`, `ssh_key` (credenciales de un repositorio privado, guardadas tal cual y nunca devueltas por la API) |

`templates` guarda las plantillas **locales** (repositorio `0`). Los repositorios remotos son repositorios Git guardados en `template_repos`; cuando `templates.url` tiene valor se ignoran y se usa el índice JSON (ver [Configuración](./02-configuration.md#general-y-registros)).

### Estado del panel

| Modelo (tabla) | Archivo | Clave primaria | Columnas principales |
|---|---|---|---|
| `PanelSetting` (`panel_settings`) | `panelsetting.go` | `key` (100) | `value` (255). Hoy guarda `master_node_ip`. |
| `Extension` (`extensions`) | `extension.go` | `id` (40) | `name`, `version`, `author`, `description`, `enabled`, `manifest` (el `extension.json`), `webhook_url`, `secret` (firma `X-Aether-Signature`), fechas |
| `ExTransferSession` (`ex_transfer_sessions`) | `extransfer.go` | `id` | `session_uuid` (único), `server_id` (FK en cascada), `user_id` (FK en cascada), `token_hash` (único), `status` (`CREATED` por defecto), `dest_host`, `dest_public_key`, `current_nonce`, `nonce_expires_at`, `protocol_version` (`1.0`), `payload`, `expires_at`, `last_heartbeat_at`, fechas |
| `ExTransferLog` (`ex_transfer_logs`) | `extransfer.go` | `id` | `session_id` (FK a `session_uuid`, en cascada), `action`, `ip_address`, `is_error`, `details`, `created_at` |

Los demás archivos de `internal/models` (`*view.go`, `*api.go`, `Deployment`…) son formas de la API, no tablas. Guías relacionadas: [Hosts de bases de datos](../features/database-hosts.md), [Aprovisionamiento y SSO](../features/provisioning-and-sso.md), [Actividad y registro de accesos](../features/activity-and-access-log.md), [Transferencias de servidores](../features/server-transfers.md).

## Relaciones

```text
Role 1──N User
User 1──N Session, Permissions, Client, RecoveryCode, PasswordReset, SSHKey, UserSetting
User 1──N ActivityLog                 (user_id anulable: las filas sobreviven a la cuenta)
User 1──N Server                      (owner_id, anulable)

Node 1──N Server                      (node_id anulable: el nodo local)
Server 1──N Permissions               (server_identifier)
Server 1──N Backup, UptimeStatus, ServerActivity
Server 1──N Database N──1 DatabaseHost
Server 1──N Server                    (parent_server_id: sub-servidores)
Server 1──N ExTransferSession 1──N ExTransferLog

Node 1──N DatabaseHost, ProvisionProduct   (anulables)
```

Las filas de `Permissions` unen a un usuario (o a un cliente OAuth2) con un servidor mediante una lista de permisos; una fila sin servidor guarda permisos globales. Cómo se combinan con el rol y con los permisos de dueño se explica en [Capa API: comprobación de permisos](./05-api-layer.md#comprobación-de-permisos).

## Retención y mantenimiento

- Las **sesiones** duran una hora desde la última actividad (caducidad deslizante); las filas caducadas nunca se aceptan, y al crear una sesión se borran las caducadas como mucho una vez por hora.
- El **registro de actividad** (`activity_logs`) y la **actividad de servidores** (`server_activities`) se conservan 90 días; las filas más antiguas se podan al leer el registro y, como mucho una vez por hora, al anotar actividad de servidores.
- Los **tokens de restablecimiento de contraseña** caducan a los 30 minutos y se consumen al usarse.
- **Transferencias entre nodos** (`node_transfers`): una fila se borra cuando termina su transferencia, y las que deja una parada del panel se cierran y se borran en el siguiente arranque.
- Los nombres de usuario y los correos se copian en las filas de actividad para que se sigan leyendo bien después de renombrar o borrar una cuenta.

## Servicios

Cada servicio de `internal/services` agrupa las consultas GORM de un área y recibe el `*gorm.DB` al crearse (`&services.User{DB: db}`). Se documentan en [Servicios](./11-services.md).
