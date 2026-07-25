# Database

## Supported Dialects

| Dialect | Driver | Default Conn String |
|---|---|---|
| `sqlite3` | `gorm.io/driver/sqlite` | `file:skypanel.db` |
| `mysql` | `gorm.io/driver/mysql` | `SkyPanel:SkyPanel@/SkyPanel` |
| `postgresql` | `gorm.io/driver/postgres` | — |
| `sqlserver` | `gorm.io/driver/sqlserver` | — |

### SQLite Settings

```go
// Automatically applied in GetConnectionString():
cache=shared
_loc=auto
_foreign_keys=1
_journal_mode=WAL
_busy_timeout=5000
_tx_lock=immediate
// MaxOpenConns forced to 1
```

### MySQL Settings

```go
charset=utf8
parseTime=true
```

## Migrations

They use **gormigrate** (versioned migrations). They run in `database/upgrade.go`.

```go
m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
    {ID: "2025-01-01-initial", Migrate: func(tx *gorm.DB) error { ... }},
    {ID: "2025-04-10-add-template-repo", Migrate: func(tx *gorm.DB) error { ... }},
    {ID: "2025-05-01-refactor-permissions", Migrate: func(tx *gorm.DB) error { ... }},
    // ...
})
m.Migrate()
```

## Models (34 tables)

### Core

| Model | File | PK | Key Columns |
|---|---|---|---|
| `User` | `user.go` | `id` (uint, auto) | `username`, `email`, `hashedPassword`, `otpSecret`, `otpVerified`, `roleID` |
| `Session` | `session.go` | `id` (uint) | `userId`, `token`, `expiresAt` |
| `APIKey` | `apikey.go` | `id` (uint) | `userId`, `token` (prefix `ak_`), `scopes` (JSON), `memo` |
| `RecoveryCode` | `recoverycode.go` | `userId`+`code` | `consumed` |

### Servers

| Model | File | PK | Key Columns |
|---|---|---|---|
| `Server` | `server.go` | `identifier` (string, 20) | `name`, `nodeID`, `ip`, `port`, `type`, `parentServerID`, `totalCPU`, `totalMemory`, `totalDisk`, `suspended`, `externalID` |
| `ServerView` | `serverview.go` | — | View model (no table) |
| `ServerAPI` | `serverapi.go` | — | DTO |

### Nodes

| Model | File | PK | Key Columns |
|---|---|---|---|
| `Node` | `node.go` | `id` (uint) | `name`, `host`, `port`, `sftp`, `tokenID`, `publicKey`, `secret`, `active`, `useInternal` |
| `NodeView` | `nodeview.go` | — | View model |
| `NodeAPI` | `nodeapi.go` | — | DTO |

### Permissions and Roles

| Model | File | PK | Key Columns |
|---|---|---|---|
| `Permissions` | `permission.go` | `id` (uint) | `userID`, `serverID`, `scopes` (JSON) |
| `PermissionView` | `permissionview.go` | — | View model |
| `Role` | `role.go` | `id` (uint) | `name`, `description`, `scopes` (JSON), `isAdmin` |

### Databases

| Model | File | PK | Key Columns |
|---|---|---|---|
| `DatabaseHost` | `databasehost.go` | `id` (uint) | `host`, `port`, `username`, `password`, `database`, `nodeID` |
| `DatabaseHostAPI` | `databasehostapi.go` | — | DTO |
| `Database` | `database.go` | `id` (uint) | `serverID`, `databaseHostID`, `databaseName`, `username`, `password`, `remote` |
| `DatabaseAPI` | `databaseapi.go` | — | DTO |

### Backups

| Model | File | PK | Key Columns |
|---|---|---|---|
| `Backup` | `backup.go` | `id` (uint) | `serverID`, `name`, `size`, `sha256`, `completedAt` |

### Settings

| Model | File | PK | Key Columns |
|---|---|---|---|
| `PanelSetting` | `panelsetting.go` | `key` (string) | `value` |
| `SettingAPI` | `settingapi.go` | — | DTO |
| `UserSetting` | `usersetting.go` | `id` (uint) | `userID`, `key`, `value` |
| `UserSettingView` | `usersettingview.go` | — | View model |
| `UserSettingAPI` | `usersettingapi.go` | — | DTO |

### Templates

| Model | File | PK | Key Columns |
|---|---|---|---|
| `Template` | `template.go` | `id` (uint) | `name`, `display`, `author`, `description`, `data` (JSON), `version`, `templateRepoID` |
| `TemplateRepo` | `templaterepo.go` | `id` (uint) | `url`, `name` |

### Products (Provision)

| Model | File | PK | Key Columns |
|---|---|---|---|
| `Product` | `product.go` | `id` (uint) | `name`, `description`, `data` (JSON) |

### Uptime

| Model | File | PK | Key Columns |
|---|---|---|---|
| `Uptime` | `uptime.go` | `id` (uint) | `serverID`, `timestamp`, `uptime` |

### Transfers

| Model | File | PK | Key Columns |
|---|---|---|---|
| `ExTransfer` | `extransfer.go` | `id` (uint) | `serverID`, `token`, `sourcePanel`, `targetPanel`, `status`, `expiresAt` |

### OAuth2 Clients

| Model | File | PK | Key Columns |
|---|---|---|---|
| `Client` | `client.go` | `id` (uint) | `userID`, `name`, `secret`, `redirectURI`, `personal` |

## Key Relationships

```
User 1──N Session
User 1──N APIKey
User N──1 Role (nullable)
User N──N Server (via Permissions)
User 1──N RecoveryCode

Server N──1 Node
Server N──N User (via Permissions)
Server N──N DatabaseHost (via Database)
Server 1──N Backup
Server 1──1..N Uptime
Server 1──1 ExTransfer
Server N──1 Server (parent, splitter)

DatabaseHost 1──N Database

TemplateRepo 1──N Template
```

## Services (Business Layer)

Each service encapsulates GORM queries and business logic. They receive `*gorm.DB` in the constructor.

| Service | File | Main Methods |
|---|---|---|
| `Session` | `session.go` | `Get`, `Create`, `DeleteExpired` |
| `User` | `user.go` | `Get`, `Create`, `Update`, `Delete`, `GetAll`, `Search`, `IsSecurePassword`, `DisableOtp` |
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