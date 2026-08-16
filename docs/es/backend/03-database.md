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

Usan **gormigrate** (migraciones versionadas). Se ejecutan en `database/upgrade.go`.

```go
m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
    {ID: "2025-01-01-initial", Migrate: func(tx *gorm.DB) error { ... }},
    {ID: "2025-04-10-add-template-repo", Migrate: func(tx *gorm.DB) error { ... }},
    {ID: "2025-05-01-refactor-permissions", Migrate: func(tx *gorm.DB) error { ... }},
    // ...
})
m.Migrate()
```

## Modelos (21 tablas)

### Core

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `User` | `user.go` | `id` (uint, auto) | `username`, `email`, `hashedPassword`, `otpSecret`, `otpActive`, `roleID` |
| `Session` | `session.go` | `id` (uint) | `userId`, `token`, `expiresAt` |
| `APIKey` | `apikey.go` | `id` (uint) | `userId`, `token` (prefijo `ak_`), `scopes` (JSON), `memo` |
| `RecoveryCode` | `recoverycode.go` | `id` (uint) | `userId`, `code` (hash blake2b del código) |
| `PasswordReset` | `passwordreset.go` | `id` (uint) | `userId`, `token` (hash blake2b), `expiresAt` (30 min) |

### Servidores

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Server` | `server.go` | `identifier` (string, 20) | `name`, `nodeID`, `ip`, `port`, `type`, `parentServerID`, `totalCPU`, `totalMemory`, `totalDisk`, `suspended`, `externalID` |
| `ServerView` | `serverview.go` | — | View model (no tabla) |
| `ServerAPI` | `serverapi.go` | — | DTO |

### Nodos

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Node` | `node.go` | `id` (uint) | `name`, `host`, `port`, `sftp`, `tokenID`, `publicKey`, `secret`, `active`, `useInternal` |
| `NodeView` | `nodeview.go` | — | View model |
| `NodeAPI` | `nodeapi.go` | — | DTO |

### Permisos y Roles

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Permissions` | `permission.go` | `id` (uint) | `userID`, `serverID`, `scopes` (JSON) |
| `PermissionView` | `permissionview.go` | — | View model |
| `Role` | `role.go` | `id` (uint) | `name`, `description`, `scopes` (JSON), `isAdmin` |

### Bases de Datos

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `DatabaseHost` | `databasehost.go` | `id` (uint) | `host`, `port`, `username`, `password`, `database`, `nodeID` |
| `DatabaseHostAPI` | `databasehostapi.go` | — | DTO |
| `Database` | `database.go` | `id` (uint) | `serverID`, `databaseHostID`, `databaseName`, `username`, `password`, `remote` |
| `DatabaseAPI` | `databaseapi.go` | — | DTO |

### Backups

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Backup` | `backup.go` | `id` (uint) | `serverID`, `name`, `size`, `sha256`, `completedAt` |

### Settings

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `PanelSetting` | `panelsetting.go` | `key` (string) | `value` |
| `SettingAPI` | `settingapi.go` | — | DTO |
| `UserSetting` | `usersetting.go` | `id` (uint) | `userID`, `key`, `value` |
| `UserSettingView` | `usersettingview.go` | — | View model |
| `UserSettingAPI` | `usersettingapi.go` | — | DTO |

### Plantillas

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Template` | `template.go` | `id` (uint) | `name`, `display`, `author`, `description`, `data` (JSON), `version`, `templateRepoID` |
| `TemplateRepo` | `templaterepo.go` | `id` (uint) | `url`, `name` |

### Productos (Provision)

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `ProvisionProduct` | `product.go` | `id` (uint) | `productID`, `displayName`, `template`, `nodeID`, `cpu`, `memory`, `disk`, `defaultNode`, `portRangeMin`, `portRangeMax` |

### Uptime

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `UptimeStatus` | `uptime.go` | `id` (uint) | `serverID`, `isRunning`, `startTime`, `endTime`, `duration` |

### Transferencias

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `ExTransferSession` | `extransfer.go` | `id` (uint) | `sessionUUID`, `serverID`, `userID`, `tokenHash`, `status`, `destHost`, `destPublicKey`, `currentNonce`, `nonceExpiresAt`, `protocolVersion`, `payload`, `expiresAt` |
| `ExTransferLog` | `extransfer.go` | `id` (uint) | `sessionID`, `action`, `ipAddress`, `isError`, `details` |

### Clientes OAuth2

| Modelo | Archivo | PK | Columnas clave |
|---|---|---|---|
| `Client` | `client.go` | `id` (uint) | `userID`, `name`, `secret`, `redirectURI`, `personal` |

## Relaciones Clave

```
User 1──N Session
User 1──N APIKey
User N──1 Role (nullable)
User N──N Server (via Permissions)
User 1──N RecoveryCode
User 1──N PasswordReset

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
| `Session` | `session.go` | `Get`, `Create`, `DeleteExpired` |
| `User` | `user.go` | `Get`, `Create`, `Update`, `Delete`, `GetAll`, `Search`, `IsSecurePassword`, `DisableOtp`, `CreatePasswordResetToken`, `ConsumePasswordResetToken` |
| `Server` | `server.go` | `Get`, `GetAll`, `Create`, `Update`, `Delete`, `Search` |
| `Node` | `node.go` | `Get`, `GetAll`, `Create`, `Update`, `Delete`, `GetDeployment` |
| `Permission` | `permission.go` | `GetForUserAndServer`, `UpdatePermissions` |
| `Role` | `role.go` | `Get`, `GetAll`, `Create`, `Update`, `Delete` |
| `APIKeyService` | `apikey.go` | `GetAll`, `Create`, `Delete`, `ValidateKey` |
| `Backup` | `backup.go` | `Get`, `GetAll`, `Create`, `Delete`, `Update` |
| `Database` | `database.go` | `GetAllForServer`, `Create`, `Delete` |
| `DatabaseHost` | `databasehost.go` | `Get`, `GetAll`, `Create`, `Update`, `Delete`, `TestConnection` |
| `Template` | `templates.go` | `GetRepos`, `AddRepo`, `DeleteRepo`, `GetFromRepo`, `GetTemplate`, `PutTemplate`, `DeleteTemplate` |
| `OAuth2` | `oauth2.go` | `Token` (client_credentials, password) |
| `Token` | `token.go` | `Sign`, `Validate`, `GetPublicKey` |
| `UserSettings` | `usersettings.go` | `Get`, `GetAll`, `Set` |
| `License` | `license.go` | `Activate`, `Validate` |
| `Email` | `email.go` | `Send` |
| `Discord` | `discord.go` | `SendNotification`, `SendSystemAlert` |
| `Uptime` | `uptime.go` | `GetAll`, `GetForServer`, `Record`, `Track` |
| `SFTP` | `sftp.go` | `GetCredentials`, `Validate` |
