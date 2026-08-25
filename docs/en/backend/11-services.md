# Internal Services Reference

Aether Panel uses a service layer pattern where each service encapsulates GORM queries and business logic. Services receive `*gorm.DB` in their constructor and are instantiated in the HTTP handlers.

---

## Service Architecture

```go
// Typical service pattern
type UserService struct {
    DB *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
    return &UserService{DB: db}
}
```

Services are created per-request via middleware (`NeedsDatabase`) which injects `*gorm.DB` into the Gin context.

---

## Core Services

### 1. SessionService (`internal/services/session.go`)

Manages authentication sessions (SHA-256 hashed tokens, 1-hour expiry).

```go
type SessionService struct {
    DB *gorm.DB
}

func (s *SessionService) CreateForUser(user *models.User) (*models.Session, error)
func (s *SessionService) CreateForClient(client *models.Client) (*models.Session, error)
func (s *SessionService) Validate(token string) (*models.Session, error)
func (s *SessionService) ValidateNode(token string) (*models.Session, error)
func (s *SessionService) Expire(token string) error
```

**Key behaviors:**
- Tokens are UUID v4, stored as SHA-256 hash in DB
- 1-hour expiration (`time.Hour`)
- `ValidateNode` used for daemon-to-panel communication
- Automatic cleanup of expired sessions on create

---

### 2. UserService (`internal/services/user.go`)

Complete user lifecycle management.

```go
type UserService struct {
    DB *gorm.DB
}

func (s *UserService) Get(id uint) (*models.User, error)
func (s *UserService) GetByID(id uint) (*models.User, error)
func (s *UserService) GetByEmail(email string) (*models.User, error)
func (s *UserService) ValidateLogin(email, password string) (*models.User, error)
func (s *UserService) ValidOtp(user *models.User, code string) bool
func (s *UserService) IsValidCredentials(email, password string) bool
func (s *UserService) Create(username, email, password string, admin bool) (*models.User, error)
func (s *UserService) Update(user *models.User) error
func (s *UserService) Delete(id uint) error
func (s *UserService) ChangePassword(id uint, current, newPassword string) error
func (s *UserService) CreatePasswordResetToken(email string) (*models.PasswordReset, error)
func (s *UserService) ConsumePasswordResetToken(token, password string) error
func (s *UserService) GetOtpStatus(user *models.User) (bool, []string, error)
func (s *UserService) StartOtpEnroll(user *models.User) (string, error)
func (s *UserService) ValidateOtpEnroll(user *models.User, code string) error
func (s *UserService) RegenerateOtpRecoveryCodes(user *models.User) ([]string, error)
func (s *UserService) DisableOtp(user *models.User) error
func (s *UserService) Search(query string, page, limit int) ([]*models.User, int64, error)
func (s *UserService) IsSecurePassword(password string) bool
```

**Key behaviors:**
- Passwords: bcrypt with cost 12
- OTP: TOTP (RFC 6238), 6 digits, 30s window
- Recovery codes: 10 codes, SHA-256 hashed, single-use
- Password reset: Blake2b hash, 30-minute expiry
- Password strength: min 8 chars, upper+lower+digit+special

---

### 3. ServerService (`internal/services/server.go`)

Server CRUD with permission-aware queries.

```go
type ServerService struct {
    DB *gorm.DB
}

func (s *ServerService) Search(filters ServerSearchFilters, user *models.User, page, limit int) ([]*models.ServerView, int64, error)
func (s *ServerService) Get(identifier string) (*models.Server, error)
func (s *ServerService) Create(server *models.Server) error
func (s *ServerService) Update(server *models.Server) error
func (s *ServerService) Delete(identifier string) error
```

**SearchFilters:**
```go
type ServerSearchFilters struct {
    Name     string
    NodeID   uint
    Username string
}
```

**Permission-aware:** `Search` filters by user permissions (global scopes + server-specific scopes + role scopes).

---

### 4. NodeService (`internal/services/node.go`)

Node management + daemon communication.

```go
type NodeService struct {
    DB *gorm.DB
}

func (s *NodeService) GetAll() ([]*models.NodeView, error)
func (s *NodeService) Get(id uint) (*models.Node, error)
func (s *NodeService) Create(node *models.Node) error
func (s *NodeService) Update(node *models.Node) error
func (s *NodeService) Delete(id uint) error

// Daemon communication
func (s *NodeService) CallNode(node *models.Node, method, path string, body io.ReadCloser, headers http.Header) (*http.Response, error)
func (s *NodeService) OpenSocket(ctx context.Context, node *models.Node, serverID string, params string) (*websocket.Conn, error)
```

**Daemon Communication:**
- `CallNode`: HTTP with Ed25519 JWT auth (via `TokenService`)
- `OpenSocket`: WebSocket proxy for console/stats/status bridging

---

### 5. PermissionService (`internal/services/permission.go`)

Scope-based permission evaluation with role inheritance.

```go
type PermissionService struct {
    DB *gorm.DB
}

func (s *PermissionService) GetForUser(userID uint) ([]models.Permission, error)
func (s *PermissionService) GetForServer(serverIdentifier string) ([]models.Permission, error)
func (s *PermissionService) GetForUserAndServer(userID uint, serverIdentifier string) ([]models.Permission, error)
func (s *PermissionService) GetForClient(clientID uint) ([]models.Permission, error)
func (s *PermissionService) GetForClientAndServer(clientID uint, serverIdentifier string) ([]models.Permission, error)
func (s *PermissionService) HasPermission(userID uint, serverIdentifier string, scope string) (bool, error)
func (s *PermissionService) HasAnyPermission(userID uint, serverIdentifier string, scopes []string) (bool, error)
func (s *PermissionService) UpdatePermissions(userID uint, serverIdentifier string, scopes []string) error
func (s *PermissionService) Remove(userID uint, serverIdentifier string) error
func (s *PermissionService) GetForUserAndServer(userID uint, serverIdentifier string) ([]models.Permission, error)
```

**Scope Resolution Order:**
1. Direct user permissions (`Permissions` with `user_id`)
2. Server-specific permissions (`server_identifier` + `user_id`)
3. Role scopes (user's `role_id` → `Role.scopes`)
4. Global scopes (`admin` = all, `server.admin` = all for server)

---

### 6. RoleService (`internal/services/role.go`)

Role management with scope aggregation.

```go
type RoleService struct {
    DB *gorm.DB
}

func (s *RoleService) Get(id uint) (*models.Role, error)
func (s *RoleService) List() ([]*models.Role, error)
func (s *RoleService) GetByName(name string) (*models.Role, error)
func (s *RoleService) Create(role *models.Role) error
func (s *RoleService) Update(role *models.Role) error
func (s *RoleService) Delete(id uint) error
```

**Default Roles:**
- `Admin` (id=1): `["admin"]` scope
- `Usuario` (id=2): Server scopes only (see migration `20260821-fix-usuario-role-scopes`)

---

### 7. APIKeyService (`internal/services/apikey.go`)

Provisioning API v1 keys.

```go
type APIKeyService struct {
    DB *gorm.DB
}

func (s *APIKeyService) GenerateKey(name string, permissions []string) (string, *models.APIKey, error)
func (s *APIKeyService) ValidateKey(key string) (*models.APIKey, error)
func (s *APIKeyService) GetAll() ([]*models.APIKey, error)
func (s *APIKeyService) Delete(id uint) error
```

**Key Format:**
- Prefix: `ak_` (8 chars)
- Full key: `ak_` + 32 random chars
- Stored: SHA-256 hash in `hashed_key`
- Permissions: JSON array (e.g., `["provision", "terminate"]`)

---

### 8. BackupService (`internal/services/backup.go`)

Backup metadata management (files stored on daemon).

```go
type BackupService struct {
    DB *gorm.DB
}

func (s *BackupService) GetAllForServer(serverID string) ([]*models.Backup, error)
func (s *BackupService) Get(id uint) (*models.Backup, error)
func (s *BackupService) Create(backup *models.Backup) error
func (s *BackupService) Update(backup *models.Backup) error
func (s *BackupService) Delete(id uint) error
```

**Note:** Actual backup files are on daemon filesystem; service only manages DB metadata.

---

### 9. DatabaseService (`internal/services/database.go`)

External MySQL database provisioning.

```go
type DatabaseService struct {
    DB *gorm.DB
}

func (s *DatabaseService) GetAllForServer(serverID string) ([]*models.DatabaseView, error)
func (s *DatabaseService) Get(id uint) (*models.Database, error)
func (s *DatabaseService) Create(db *models.Database) error
func (s *DatabaseService) Delete(id uint) error

// MySQL operations
func (s *DatabaseService) createInMySQL(db *models.Database, host *models.DatabaseHost) error
func (s *DatabaseService) deleteFromMySQL(db *models.Database, host *models.DatabaseHost) error
```

**MySQL Operations:**
- Connects to `DatabaseHost` with admin credentials
- `CREATE DATABASE`, `CREATE USER`, `GRANT ALL PRIVILEGES`
- Generates random username (`db_xxx`) and password
- Stores `remote_connection` as `host:port`

---

### 10. DatabaseHostService (`internal/services/databasehost.go`)

External MySQL host management.

```go
type DatabaseHostService struct {
    DB *gorm.DB
}

func (s *DatabaseHostService) GetAll() ([]*models.DatabaseHostView, error)
func (s *DatabaseHostService) Get(id uint) (*models.DatabaseHost, error)
func (s *DatabaseHostService) Create(host *models.DatabaseHost) error
func (s *DatabaseHostService) Update(host *models.DatabaseHost) error
func (s *DatabaseHostService) Delete(id uint) error
```

---

### 11. TemplateService (`internal/services/templates.go`)

Template repository management (local + Git + VPS JSON index).

```go
type TemplateService struct {
    DB *gorm.DB
}

func (s *TemplateService) GetRepos() ([]*models.TemplateRepo, error)
func (s *TemplateService) AddRepo(repo *models.TemplateRepo) error
func (s *TemplateService) DeleteRepo(id uint) error
func (s *TemplateService) GetAllFromRepo(repoID uint) ([]*models.Template, error)
func (s *TemplateService) Get(repoID uint, name string) (*models.Template, error)
func (s *TemplateService) Save(template *models.Template) error
func (s *TemplateService) Delete(repoID uint, name string) error

// VPS JSON Index
func (s *TemplateService) SyncRepo(repo *models.TemplateRepo) error
func (s *TemplateService) getAllFromVps(repo *models.TemplateRepo) ([]*models.Template, error)
func (s *TemplateService) getFromVps(repo *models.TemplateRepo, name string) (*models.Template, error)
func (s *TemplateService) getTemplateFromURL(url string) (*models.Template, error)
```

**Repo Types:**
- `id=0`: Local repository (DB storage)
- `id>0`: Remote repository (Git or VPS JSON index)

**VPS JSON Index:**
- Fetches `templates.json` from `repo.url`
- Each entry has `url` to individual template JSON
- Supports `pat`, `username`/`password`, `ssh_key` for private repos

---

### 12. OAuth2Service (`internal/services/oauth2.go`)

Personal OAuth2 clients + server-scoped clients.

```go
type OAuth2Service struct {
    DB *gorm.DB
}

func (s *OAuth2Service) Get(id uint) (*models.Client, error)
func (s *OAuth2Service) GetForUser(userID uint) ([]*models.Client, error)
func (s *OAuth2Service) Create(client *models.Client) error
func (s *OAuth2Service) Update(client *models.Client) error
func (s *OAuth2Service) Delete(id uint) error
func (s *OAuth2Service) GetForUserAndServer(userID uint, serverID string) ([]*models.Client, error)
```

**Client Types:**
- **Global** (`server_id` = null): Uses global scopes
- **Server-scoped** (`server_id` set): Uses `ForServer=true` scopes

**Security:**
- `client_secret`: bcrypt hash (cost 12)
- Returned **only once** on create
- Email notification on create/delete
- Max 10 clients per user (configurable)

---

### 13. TokenService (`internal/services/token.go`)

Ed25519 JWT for Panel↔Daemon authentication.

```go
type TokenService struct {
    privateKey ed25519.PrivateKey
    publicKey  ed25519.PublicKey
    keyFunc    jwt.Keyfunc
    tokenStore *jwkset.MemoryStorage
}

func NewTokenService() (*TokenService, error)
func (s *TokenService) GenerateRequest() (string, error)
func (s *TokenService) ValidateRequest(tokenString string) (*jwt.Token, error)
func (s *TokenService) GetKeyFunc() jwt.Keyfunc
func (s *TokenService) GetTokenStore() *jwkset.MemoryStorage
```

**JWT Details:**
- Algorithm: Ed25519
- Private key: base64 seed (32 bytes) or random generation
- Public key: JWKS at `/auth/publickey` (KID="SkyPanel")
- Claims: `serverId`, `iat`, `exp` (1 hour), `iss` (panel URL)
- Token store: in-memory JWKS with rotation support

---

### 14. UserSettingsService (`internal/services/usersettings.go`)

Per-user key-value settings.

```go
type UserSettingsService struct {
    DB *gorm.DB
}

func (s *UserSettingsService) GetAllForUser(userID uint) (map[string]string, error)
func (s *UserSettingsService) Update(userID uint, settings map[string]string) error
```

**Common Keys:**
- `theme`: `dark`/`light`/`system`
- `language`: `en`/`es`

---

### 15. LicenseService (`internal/services/license.go`)

External license validation (SkyHosting Cloud API).

```go
type LicenseService struct {
    DB *gorm.DB
    client *http.Client
}

func (s *LicenseService) VerifyLicense(licenseKey string) (*LicenseVerification, error)
func (s *LicenseService) BindLicense(licenseKey, serverID, serverIP string) (*LicenseBindResult, error)
func (s *LicenseService) ExtractPermissions(verification *LicenseVerification) LicensePermissions
func (s *LicenseService) GetLicenseType(verification *Verification) string
```

**External API:** `https://prueba.skyhostingcloud.com/api/public/licenses`

**Endpoints:**
- `GET /verify?licenseKey=` → plan, maxServers, expiry, bounds
- `POST /verify` + `serverId`/`serverIP` → binds license

**Plans:**
- `personal` → free tier
- `professional` → pro tier (HasPlugins=true)
- `enterprise` → enterprise tier (HasPlugins=true)

---

### 16. EmailService (`internal/services/email.go`)

Template-based email with multiple providers.

```go
type EmailService struct {
    DB       *gorm.DB
    provider EmailProvider
}

func (s *EmailService) SendEmail(to, templateName string, data map[string]interface{}, async bool) error
```

**Template System:**
- `emails.json` maps template name → `{ subject: "...", body: "template.tmpl" }`
- Templates: Go `text/template` with data
- FS: merged `embed.FS` (custom + embedded)

**Providers** (`internal/email/`):
| Provider | Package | Config |
|----------|---------|--------|
| `smtp` | `wneessen/go-mail` | host, port, username, password |
| `sendgrid` | `sendgrid-go` | API key |
| `mailjet` | `mailjet-go` | API key + secret |
| `mailgun` | `mailgun-go` | API key + domain |
| `debug` | log only | development |

**Async Sending:** Goroutine with error logging (non-blocking)

**Built-in Templates:** `addedToServer`, `removedFromServer`, `passwordReset`, `otpEnroll`, `licenseExpiring`, etc.

---

### 17. DiscordService (`internal/services/discord.go`)

Rich embed webhooks for notifications.

```go
type DiscordService struct {
    DB *gorm.DB
}

func (s *DiscordService) SendWebhook(webhookURL, title, description string, color int, fields []DiscordField) error
func (s *DiscordService) SendWebhookToURL(webhookURL, title, description string, color int, fields []DiscordField) error
func (s *DiscordService) SendAlert(title, description string) error
func (s *DiscordService) SendServerOfflineAlert(serverName, serverID string) error
func (s *DiscordService) SendServerOnlineAlert(serverName, serverID string) error
func (s *DiscordService) SendResourceAlert(serverName, serverID, resource string, current, threshold float64) error
func (s *DiscordService) SendBackupAlert(serverName, serverID, status, backupName string) error
func (s *DiscordService) SendSystemStatus(servers []ServerStatusSummary) error
func (s *DiscordService) SendNodeStatus(nodeName string, info SystemInfo, servers []ServerStatusSummary) error
```

**Webhook Types (4 URLs):**
| Config Key | Purpose |
|------------|---------|
| `DiscordWebhook` | General alerts |
| `DiscordWebhookSystem` | System status (hourly) |
| `DiscordWebhookNode` | Node status |
| `DiscordWebhookExTransfer` | External transfers |

**Embed Colors:**
- Red (0xFF0000): Alerts, offline, failed backups
- Green (0x00FF00): Online, successful backups
- Yellow (0xFFFF00): Warnings, resource alerts
- Blue (0x0000FF): Info, system status

---

### 18. UptimeService (`internal/services/uptime.go`)

Server uptime/downtime tracking.

```go
type UptimeService struct {
    DB *gorm.DB
}

func (s *UptimeService) TrackStatus(serverID string, isRunning bool) error
func (s *UptimeService) GetUptimeStats(serverID string, days int) (*UptimeStats, error)
func (s *UptimeService) GetRecentHistory(serverID string, limit int) ([]*models.UptimeStatus, error)
func (s *UptimeService) GetAllServerUptime() (map[string]*UptimeStats, error)
```

**Tracking:**
- Called every 5s via `processStats()` in `server.go`
- Creates/closes `UptimeStatus` records on state change
- Handles active (running) record without `end_time`

**UptimeStats:**
```go
type UptimeStats struct {
    TotalUptime     time.Duration
    TotalDowntime   time.Duration
    UptimePercent   float64
    CurrentStatus   bool
    CurrentStart    time.Time
    Records         []*models.UptimeStatus
}
```

---

### 19. SFTPService (`internal/services/sftp.go`)

SFTP authorization interface.

```go
type SFTPService struct {
    DB *gorm.DB
}

func (s *SFTPService) Validate(conn sftp_conn, username, password string) (string, error)
```

**Authorization Implementations:**
- `oauth2.WebSSHAuthorization`: Panel OAuth2 password grant
- `DatabaseSFTPAuthorization`: Direct DB lookup (`email#serverId`)

---

## Service Instantiation Pattern

All services follow this pattern in HTTP handlers:

```go
func handler(c *gin.Context) {
    db := middleware.GetDatabase(c)
    service := &services.UserService{DB: db}
    // ...
}
```

The `NeedsDatabase` middleware injects `*gorm.DB` into `c.MustGet("db")`.

---

## Adding a New Service

1. Create `internal/services/myservice.go` with struct + methods
2. Add constructor: `func NewMyService(db *gorm.DB) *MyService`
3. Use in handlers: `service := &services.MyService{DB: db}`
4. Add tests in `internal/services/myservice_test.go`