# Base de Datos

## Dialectos Soportados

| Dialecto | Driver | Default Conn String |
|---|---|---|
| `sqlite3` | `gorm.io/driver/sqlite` | `file:skypanel.db` |
| `mysql` | `gorm.io/driver/mysql` | `SkyPanel:SkyPanel@/SkyPanel` |
| `postgresql` | `gorm.io/driver/postgres` | — |
| `sqlserver` | `gorm.io/driver/sqlserver` | — |

### SQLite Settings

```go
// Aplicadas automáticamente en GetConnectionString():
cache=shared
_loc=auto
_foreign_keys=1
_journal_mode=WAL
_busy_timeout=5000
_tx_lock=immediate
// MaxOpenConns forzado a 1
```

### MySQL Settings

```go
charset=utf8
parseTime=true
```

## Migraciones

Usan **gormigrate** (migraciones versionadas) definidas en `internal/database/upgrade.go`. Se agrupan en batches y cada batch se ejecuta en su propia transacción contra la tabla `migrations`.

```go
options := &gormigrate.Options{TableName: "migrations", IDColumnName: "id", IDColumnSize: 255}

// por cada batch:
for _, z := range batch {
    gormigrate.New(session, options, []*gormigrate.Migration{z}).Migrate()
}
```

IDs de migración reales (fecha/timestamp como versionado):

```go
"1726675832", "1726675832-mysql", "1658926619", "1677250619",
"permissions-from-v2", "20260304-serverid-harmonization",
"20260617-default-roles", "20260624-usuario-role-templates-view",
"20260625-assign-usuario-role-to-existing-users",
"20260625-usuario-role-add-server-create", "20260811-template-raw-value-size"
```

## Modelos (21 tablas)

### Core

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `User` | `user.go` | `id` (uint, auto) | `username` (unique), `email` (unique), `password` (hash), `otp_secret`, `otp_active`, `role_id` (nullable) |
| `Session` | `session.go` | `id` (uint) | `token` (size 64, unique), `expiration_time`, `user_id` (nullable), `client_id` (nullable), `server_identifier` (nullable) |
| `APIKey` | `apikey.go` | `id` (uint) | `name`, `hashed_key`, `prefix` (8, ej. `ak_a1b2c3`), `permissions` (serializer JSON) |
| `RecoveryCode` | `recoverycode.go` | `id` (uint) | `user_id`, `code` (hash del código) |
| `PasswordReset` | `passwordreset.go` | `id` (uint) | `user_id`, `token` (hash, size 64, unique), `expires_at` (30 min) |

### Servidores

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Server` | `server.go` | `identifier` (string, 20) | `name`, `node_id` (nullable, solo escritura), `ip`, `port`, `type`, `icon`, `parent_server_id`, `total_cpu`, `total_memory`, `total_disk`, `suspended`, `external_id` |
| `ServerView` | `serverview.go` | — | View model (no tabla) |
| `ServerAPI` | `serverapi.go` | — | DTO |

### Nodos

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Node` | `node.go` | `id` (uint) | `name` (unique), `public_host`, `private_host`, `public_port`, `private_port`, `sftp_port`, `secret` (36), `local` (no columna, runtime) |
| `NodeView` | `nodeview.go` | — | View model |
| `NodeAPI` | `nodeapi.go` | — | DTO |

### Permisos y Roles

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Permissions` | `permission.go` | `id` (uint) | `user_id` (nullable), `client_id` (nullable), `server_identifier` (nullable), `scopes` (JSON) |
| `PermissionView` | `permissionview.go` | — | View model |
| `Role` | `role.go` | `id` (uint) | `name` (unique), `description`, `scopes` (JSON, size 2000), `created_at`, `updated_at` |

### Bases de Datos

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `DatabaseHost` | `databasehost.go` | `id` (uint) | `name`, `host`, `port`, `username`, `password`, `max_databases`, `node_id` (nullable) |
| `DatabaseHostAPI` | `databasehostapi.go` | — | DTO |
| `Database` | `database.go` | `id` (uint) | `server_id` (FK, cascade), `database_host_id` (FK, cascade), `database_name`, `username`, `password`, `remote_connection`, `max_connections` |
| `DatabaseAPI` | `databaseapi.go` | — | DTO |

### Backups

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Backup` | `backup.go` | `id` (uint) | `name`, `file_name`, `server_id` (FK) |

### Settings

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `PanelSetting` | `panelsetting.go` | `key` (string, 100) | `value` (size 255) |
| `SettingAPI` | `settingapi.go` | — | DTO |
| `UserSetting` | `usersetting.go` | `key` + `user_id` (compuesta) | `value` (size 4000) |
| `UserSettingView` | `usersettingview.go` | — | View model |
| `UserSettingAPI` | `usersettingapi.go` | — | DTO |

### Plantillas

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Template` | `template.go` | `name` (string, 100) | `raw_value` (mediumtext), `readme` |
| `TemplateRepo` | `templaterepo.go` | `id` (uint) | `name`, `url`, `branch` (default `main`), `pat`, `username`, `password`, `ssh_key`, `is_local` (no columna) |

### Productos (Provision)

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `ProvisionProduct` | `product.go` | `id` (uint) | `product_id` (unique, ej. `minecraft_2gb`), `display_name`, `template`, `node_id` (nullable = auto-select), `cpu`, `memory` (MB), `disk` (MB), `default_node`, `port_range_min`, `port_range_max` |

### Uptime

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `UptimeStatus` | `uptime.go` | `id` (uint) | `server_id` (FK), `is_running`, `start_time`, `end_time` (nullable), `duration` (segundos) |

### Transferencias

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `ExTransferSession` | `extransfer.go` | `id` (uint) | `session_uuid` (char 36, unique), `server_id` (FK cascade), `user_id` (FK cascade), `token_hash` (unique), `status` (default `CREATED`), `dest_host`, `dest_public_key`, `current_nonce`, `protocol_version` (default `1.0`), `payload`, `expires_at` |
| `ExTransferLog` | `extransfer.go` | — | `session_id` (FK), `action`, `ip_address`, `is_error`, `details` |

### Clientes OAuth2

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Client` | `client.go` | `id` (uint) | `client_id` (unique), `client_secret` (hash), `user_id` (FK), `server_id` (nullable), `name`, `description`, `scopes` (size 4000) |

## Relaciones Clave

```
User 1──N Session
User 1──N APIKey
User N──1 Role (nullable)
User N──N Server (via Permissions)
User 1──N RecoveryCode
User 1──N PasswordReset
User 1──N Client

Server N──1 Node
Server N──N User (via Permissions)
Server N──N DatabaseHost (via Database)
Server 1──N Backup
Server 1──1..N UptimeStatus
Server 1──1 ExTransferSession (más ExTransferLog)
Server N──1 Server (parent, splitter)

DatabaseHost 1──N Database

TemplateRepo 1──N Template
```

## Servicios (Capa de Negocio)

Cada servicio encapsula queries GORM y lógica de negocio. Reciben `*gorm.DB` en constructor.

| Servicio | Archivo | Métodos principales |
|---|---|---|
| `Session` | `session.go` | `CreateForUser`, `CreateForClient`, `Validate`, `ValidateNode`, `Expire` |
| `User` | `user.go` | `Get`, `GetByID`, `GetByEmail`, `ValidateLogin`, `ValidOtp`, `IsValidCredentials`, `Create`, `Update`, `Delete`, `ChangePassword`, `CreatePasswordResetToken`, `ConsumePasswordResetToken`, `GetOtpStatus`, `StartOtpEnroll`, `ValidateOtpEnroll`, `RegenerateOtpRecoveryCodes`, `DisableOtp`, `Search`, `IsSecurePassword` |
| `Server` | `server.go` | `Search`, `Get`, `Create`, `Update`, `Delete` |
| `Node` | `node.go` | `GetAll`, `Get`, `Create`, `Update`, `Delete`, `CallNode`, `OpenSocket` |
| `Permission` | `permission.go` | `GetForUser`, `GetForServer`, `GetForUserAndServer`, `HasPermission`, `GetForClient`, `GetForClientAndServer`, `UpdatePermissions`, `Remove` |
| `Role` | `role.go` | `Get`, `List`, `GetByName`, `Create`, `Update`, `Delete` |
| `APIKeyService` | `apikey.go` | `GenerateKey`, `ValidateKey`, `GetAll`, `Delete` |
| `Backup` | `backup.go` | `GetAllForServer`, `Get`, `Create`, `Update`, `Delete` |
| `Database` | `database.go` | `GetAllForServer`, `Get`, `Create`, `Delete`, `createInMySQL`, `deleteFromMySQL` |
| `DatabaseHost` | `databasehost.go` | `GetAll`, `Get`, `Create`, `Update`, `Delete` |
| `Template` | `templates.go` | `GetRepos`, `AddRepo`, `DeleteRepo`, `GetAllFromRepo`, `Get`, `Save`, `Delete` |
| `OAuth2` | `oauth2.go` | `Get`, `GetForUser`, `Create`, `Update`, `Delete` |
| `Token` | `token.go` | `GenerateRequest`, `ValidateRequest`, `GetKeyFunc`, `GetTokenStore` |
| `UserSettings` | `usersettings.go` | `GetAllForUser`, `Update` |
| `License` | `license.go` | `VerifyLicense`, `BindLicense`, `ExtractPermissions`, `GetLicenseType` |
| `Email` | `email.go` | `SendEmail` |
| `Discord` | `discord.go` | `SendWebhook`, `SendWebhookToURL`, `SendAlert`, `SendServerOfflineAlert`, `SendServerOnlineAlert`, `SendResourceAlert`, `SendBackupAlert`, `SendSystemStatus`, `SendNodeStatus` |
| `Uptime` | `uptime.go` | `TrackStatus`, `GetUptimeStats`, `GetRecentHistory`, `GetAllServerUptime` |
| `SFTP` | `sftp.go` | `Validate` |