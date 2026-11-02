# Services (`internal/services`)

`internal/services` is the business layer between the HTTP handlers and the database. Most services are a struct with a `DB *gorm.DB` field and methods that wrap GORM queries plus the rules that go with them (hashing, limits, cascades). A few are singletons or plain functions that talk to outside services (Discord, Telegram, Cloudflare, the license API).

Today the package defines **23 service types** and **5 helper modules** (rate limiting, sign-in links, notification channels, domain status, local node transport). The old "19 services" count, and names such as `UserService` or `SessionService`, no longer match the code: the types are called `User`, `Session`, etc.

---

## Using a service

There are no constructors for the database services; handlers build them inline with the request's connection:

```go
db := middleware.GetDatabase(c)      // set by middleware.NeedsDatabase
us := &services.User{DB: db}
user, err := us.GetByID(id)
```

`middleware.NeedsDatabase` calls `database.GetConnection()` and stores the `*gorm.DB` under the `"db"` key of the Gin context; `middleware.GetDatabase(c)` reads it back. Code outside a request (the daemon loops, SFTP) calls `database.GetConnection()` itself. Passing a transaction (`tx`) as `DB` makes the service run inside it, which is how `Permission.SetServerOwner` and `Server.Delete` callers get atomicity.

---

## Overview

| # | Type | File | Kind | Responsibility |
|---|---|---|---|---|
| 1 | `Session` | `session.go` | DB | Session tokens: create, validate (sliding expiry), expire |
| 2 | `User` | `user.go` | DB | Accounts, login, 2FA, password reset, search |
| 3 | `Server` | `server.go` | DB | Server rows: search, get, create, update, cascade delete |
| 4 | `Node` | `node.go`, `node_local.go` | DB | Nodes, local node address, calls and WebSocket proxy to daemons |
| 5 | `Permission` | `permission.go` | DB | Permission rows, ownership, scope checks |
| 6 | `Role` | `role.go` | DB | Roles CRUD; protects default roles |
| 7 | `APIKeyService` | `apikey.go` | DB | Admin API keys (`ak_...`) |
| 8 | `Backup` | `backup.go` | DB | Backup metadata rows |
| 9 | `Database` | `database.go` | DB + MySQL | Per-server MySQL databases and users |
| 10 | `DatabaseHost` | `databasehost.go` | DB | Database host CRUD |
| 11 | `Template` | `templates.go` | DB + Git/HTTP | Template repositories and templates |
| 12 | `OAuth2` | `oauth2.go` | DB | OAuth2 client rows |
| 13 | `UserSettings` | `usersettings.go` | DB | Per-user key/value settings |
| 14 | `Activity` | `activity.go` | DB | Panel access log (sign-ins), 90-day retention |
| 15 | `ServerActivity` | `server_activity.go` | DB | Per-server activity log, filters, cursor paging |
| 16 | `Uptime` | `uptime.go` | DB | Online/offline periods per server |
| 17 | `SSHKey` | `sshkey.go` | DB | Users' SFTP public keys |
| 18 | `DatabaseSFTPAuthorization` | `sftp.go` | DB | SFTP password and key sign-in |
| 19 | `TokenService` (interface) | `token.go` | Crypto | EdDSA JWTs for panel → daemon calls, JWKS |
| 20 | `EmailService` (interface) | `email.go` | Singleton | Email templates and sending |
| 21 | `DiscordService` | `discord.go` | Singleton | Discord webhook embeds |
| 22 | `LicenseService` | `license.go` | Singleton | External license API |
| 23 | `Cloudflare` | `cloudflare.go` | API client | DNS records of one Cloudflare zone |

Helper modules:

| Module | File | Responsibility |
|---|---|---|
| `AttemptLimiter` (`LoginAttempts`, `PasswordResetRequests`) | `login_attempts.go` | In-memory throttling per account |
| `SSOLinks` | `sso.go` | One-time sign-in links for billing systems |
| Telegram / JSON webhook | `channels.go` | Validation and delivery for owner notifications and extensions |
| Notification texts | `notification_text.go` | Texts sent outside the panel's pages, in Spanish and English |
| `ReadDomainStatus` | `domain_status.go` | Reads what the installer's domain service did |
| `serveLocal` | `node_local.go` | Runs a "node call" in-process for the panel's own node |

---

## 1. `Session` (`session.go`)

| Method | Description |
|---|---|
| `CreateForUser(user) (string, error)` | New session for a user |
| `CreateForClient(client) (string, error)` | New session for an OAuth2 client (also records the client's owner as `UserID`) |
| `Validate(token) (*models.Session, error)` | Finds a live session and renews it |
| `ValidateNode(token) (*models.Node, error)` | Finds the node whose `Secret` equals the token (the local node first) |
| `Expire(token) error` | Deletes the session; a missing one is not an error |
| `ExpireAllForUser(userID, keepToken) error` | Deletes every session of the user except `keepToken` (used after a password change) |
| `PruneExpired(now) error` | Deletes every session that expired by `now` |
| `HashToken(source) (string, error)` | Package function: SHA-256, hex |

- The token handed out is a UUID v4; only its SHA-256 hash is stored.
- `SessionLength = time.Hour`. Expiry slides: `Validate` pushes it to now + 1 h once less than half (30 min) is left, so only one write per half hour of activity.
- `Validate` preloads `Client`, `User.Permissions`, `User.Role` and `Server`, and ignores rows with neither user nor client.
- Expired rows are deleted by `PruneExpired(now)`, which `CreateForUser`/`CreateForClient` run after storing the new session at most once an hour (`sessionPruneEvery`, shared by every request). A failed clean-up is only logged; it never fails the sign-in.

## 2. `User` (`user.go`)

| Method | Description |
|---|---|
| `Get(username)`, `GetByID(id)`, `GetByEmail(email)` | Lookups; an empty value returns `ErrRecordNotFound` instead of the first row |
| `ValidateLogin(email, password) (user, otpNeeded, err)` | Password check with rate limiting; `otpNeeded` when 2FA is on |
| `ValidOtp(email, token) (user, err)` | TOTP code, or a recovery code (which is consumed) |
| `IsValidCredentials(user, password) bool` | bcrypt comparison |
| `Create`, `Update` | Plain create / save |
| `Delete(user)` | Transaction: deletes the user's permissions, OAuth2 clients, sessions, recovery codes, password resets, SSH keys and user settings (`user_settings`), clears `owner_id` on their servers, then deletes the user |
| `ChangePassword(username, newPass)` | Re-hashes and saves |
| `CreatePasswordResetToken(email) (token, accountEmail, err)` | One reset per user (older ones deleted), valid 30 minutes; returns the stored address to send to |
| `ConsumePasswordResetToken(token) (*models.User, error)` | Checks expiry and deletes the reset |
| `GetOtpStatus(userID)` | Whether 2FA is active |
| `StartOtpEnroll(userID) (secret, qrDataURI, err)` | Generates a TOTP secret kept as *pending*; refuses with `ErrOtpAlreadyEnabled` if 2FA is already on |
| `ValidateOtpEnroll(userID, token) ([]string, error)` | Confirms the pending secret, activates 2FA and returns 10 new recovery codes |
| `RegenerateOtpRecoveryCodes(userID)` | Replaces the recovery codes |
| `DisableOtp(userID)` | Clears secrets and recovery codes |
| `Search(usernameFilter, emailFilter, pageSize, page)` | `*` works as wildcard; preloads `Role` and `Permissions` |
| `IsSecurePassword(password) error` | Only checks length: 8 to 72 **bytes** (bcrypt's limit) |

Details verified in code:

- Passwords: bcrypt with `bcrypt.DefaultCost` (`models.User.SetPassword`).
- Rate limiting: `LoginAttempts` (10 attempts / 15 min) keyed by `password:user:<id>` (or the lowercased email when no account matches); 2FA codes use a separate `otp:` key. A successful attempt resets the counter. Unknown emails still run a bcrypt comparison so timing does not reveal which emails exist.
- TOTP via `github.com/pquerna/otp` with library defaults, issuer `panel.settings.companyName`, QR code as a 256×256 PNG data URI.
- Recovery codes: 10 codes of 12 alphanumeric characters, stored as BLAKE2b-256 hashes. Reset tokens: `crypto/rand.Text()`, stored as BLAKE2b-256.

## 3. `Server` (`server.go`)

```go
type ServerSearch struct {
    Username string
    NodeID   uint
    NodeName string   // "LocalNode" = servers with node_id NULL
    Name     string   // * wildcard
    PageSize uint
    Page     uint
}
```

| Method | Description |
|---|---|
| `Search(ServerSearch) ([]*models.Server, total, err)` | Filters by node (id or name), by a user's permission rows (`Username`) and by name; ordered by name, associations preloaded |
| `Get(identifier)` | By the public identifier |
| `GetByInternalID(internalID)` | By the immutable internal UUIDv7 |
| `Create(model)` | Generates a 12-character identifier (up to 5 attempts against collisions) and the internal UUIDv7 when missing |
| `Update(model)` | Saves without associations and without touching `node_id` |
| `Delete(identifier)` | Deletes permissions, OAuth2 clients, backups, the server's MySQL databases (see `Database.DropAllForServer`) and their rows, uptime rows, sessions scoped to the server, then the server. Not wrapped in a transaction: callers pass one. |

`Search` itself does not check the caller's permissions; handlers decide what to pass.

## 4. `Node` (`node.go`, `node_local.go`)

| Method / function | Description |
|---|---|
| `GetAll()` | Nodes from the database plus `models.LocalNode` when `panel.enable` is true |
| `Get(id)` | The local node for its id, otherwise a database row |
| `Create`, `Update`, `Delete` | `Update` is a no-op for the local node; `Delete` refuses the local node and nodes that still have servers (`ErrNodeHasServers`) |
| `CallNode(node, method, path, body, headers)` / `CallNodeContext(ctx, ...)` | HTTP call to the node's daemon with a fresh JWT bound to that node |
| `OpenSocket(node, path, w, r)` | Upgrades the browser connection and proxies it to the daemon's WebSocket (`ws`/`wss`) |
| `SyncNodeToConfig()` | Runs at `init`: local node host/port from `panel.settings.masterUrl`, SFTP port from `daemon.sftp.host` |
| `KeepLocalNodeHostInSync(db)` | Called at startup; refreshes the local node address now and every 15 s |

**Local node address** (`syncLocalNodeHost`): the `master_node_ip` panel setting if set; otherwise, when the Base URL's domain is proxied (Cloudflare orange cloud, read from `domain-status.json`), the server's public IP; otherwise the Base URL host.

**Calls to a node** (`CallNodeContext`):
- The browser's headers are copied and the `Cookie` header removed: nodes authenticate with the panel's token, never the user's session.
- `Authorization: Bearer <JWT>` from `TokenService.GenerateRequest(node)`.
- Local node: the request is served in-process by `serveLocal` (the daemon routes on `skypanel.Engine`). Responses up to 1 MiB are buffered; larger ones (downloads, backups) are streamed through a pipe.
- Remote node: `http(s)://PrivateHost:PrivatePort/<path>`. The scheme comes from `doesDaemonUseSSL`: an `OPTIONS /daemon` probe over https, then http (10 s timeout), cached for 5 minutes. A node seen speaking TLS is pinned (`nodes.ssl = true` in the database) and never downgraded to http.

## 5. `Permission` (`permission.go`)

| Method | Description |
|---|---|
| `GetForUser(id)`, `GetForServer(serverID)`, `GetForClient(id)` | Rows for a user, server or OAuth2 client |
| `GetForUserAndServer(userID, serverID)` | One row (global when `serverID` is empty); returns an empty unsaved row when there is none |
| `GetForClientAndServer(id, serverID)` | `FirstOrCreate` for an OAuth2 client |
| `IsServerOwner(userID, serverID)` | `servers.owner_id` check |
| `SetServerOwner(serverID, userID)` | Transaction: the new owner gets a row with `server.view` if they had none; the previous owner keeps their row |
| `HasPermission(userID, serverID, scope)` | See below |
| `MaySignIn(userID)` | `HasPermission(userID, "", login)`; checked by web, OAuth2 and SFTP sign-in |
| `UpdatePermissions(perms)` | Saves the row (rows are never deleted when emptied) |
| `Remove(perms)` | Deletes the row |

`HasPermission` order: (1) for server scopes, the owner gets everything in `scopes.OwnerScopes`; (2) the user's row on that server or their global row; (3) their role, where every role scope counts panel-wide but the role's **server** scopes are ignored on a specific server (same rule as `middleware.collectScopes`). Full rules: [API layer](./05-api-layer.md) and [scopes reference](../general/scopes-reference.md).

## 6. `Role` (`role.go`)

`Get(id)`, `GetByName(name)`, `List()` (ordered by name), `Create`, `Update`, `Delete(id)`.

`Delete` refuses roles named `admin`, `Administrador` and `Usuario` (the defaults created by the migrations in `internal/database/upgrade.go`), sets `role_id = NULL` on users of the role and deletes it, in a transaction.

## 7. `APIKeyService` (`apikey.go`)

| Method | Description |
|---|---|
| `GenerateKey(name, permissions) (token, *models.APIKey, err)` | 24 random bytes → `ak_<5 hex>_<43 hex>`; stores the 8-character prefix and the SHA-256 of the whole token |
| `ValidateKey(token)` | Checks the format and looks up prefix + hash |
| `GetAll()`, `Delete(id)` | List and delete |

The clear token is only returned once, at creation. See [Security](./09-security.md#api-keys).

## 8. `Backup` (`backup.go`)

`GetAllForServer(serverID)`, `Get(serverID, id)`, `Create`, `Update` (without associations), `Delete(id)`. Backup **files** live on the node (and optionally in an S3 bucket, see [offsite](./10-remaining-packages.md#internaloffsite--off-site-backups)); this service only handles rows. Feature: [Backups](../features/backups.md).

## 9. `Database` (`database.go`)

| Method | Description |
|---|---|
| `GetAllForServer(serverID)`, `Get(id)` | Rows with their `DatabaseHost` preloaded |
| `UsableHosts(serverID)` | Hosts the server may use: tied to its node (or to none) and below `MaxDatabases` |
| `Create(database)` | Checks the host, builds the name, creates database + user on MySQL, saves the row |
| `Delete(id)` | Drops the MySQL user and database, then the row |
| `DropAllForServer(serverID)` | Best effort drop of every database of a server; failures are logged, not returned |

Rules:
- Physical name `s_<serverId>_<name>`, only `[a-zA-Z0-9_-]`, at most 64 characters (`ErrInvalidDatabaseName`); unique per host (`ErrDatabaseNameTaken`).
- Host limits: `ErrDatabaseHostFull` (`MaxDatabases`, 0 = unlimited) and `ErrDatabaseHostOtherNode`.
- Generated user `db_` + 10 lowercase alphanumerics, 16-character password; `RemoteConnection` = `host:port`.
- MySQL statements: `CREATE DATABASE` **without** `IF NOT EXISTS` (utf8mb4 / utf8mb4_unicode_ci), `CREATE USER ... @'%'`, `GRANT ALL PRIVILEGES ON db.*`, `FLUSH PRIVILEGES`. If any step after the database exists fails, the user and database are dropped again. 5 s connect/read/write timeouts. Feature: [Database hosts](../features/database-hosts.md).

## 10. `DatabaseHost` (`databasehost.go`)

CRUD: `GetAll`, `Get`, `Create`, `Update`, `Delete`. `Delete` counts the host's databases first and returns `skypanel.ErrDatabaseHostInUse` while there is any (the API answers `409`): removing the host would cascade-delete their rows and leave the databases and their users on the MySQL server with nothing in the panel to drop them.

## 11. `Template` (`templates.go`)

| Method | Description |
|---|---|
| `GetLocalRepoID()` | Always `0` |
| `GetRepos()` | Database repos plus the synthetic `Local` repo (id 0). When `templates.url` is set, returns a synthetic repo `community` (id 1, branch `vps`) plus `Local` and ignores the database repos. |
| `GetAllFromRepo(repoID)` | Template summaries (name, display, type, environment, supported environments, requirements) |
| `Get(repoID, name)` | Full template, with `README.md` for Git repos |
| `Save(template)`, `Delete(name)` | Local templates (database) |
| `AddRepo(repo)` | Refuses duplicate names (`ErrRepoExists`) |
| `DeleteRepo(id)` | Deletes the repo row |

Sources:
- **Local (id 0)**: rows of the `templates` table.
- **Git repos**: cloned with go-git into `<daemon.data.cache>/template-repos/<id>`, single branch, refreshed with a pull when the checkout is older than 5 minutes. Each top-level folder may hold `<name>.json` files (`data.json` is skipped) and a `README.md`.
- **JSON index (`templates.url`)**: an object mapping template names to `{ "url": "..." }`, relative URLs resolved against the index URL. Downloads use a 10 s timeout and browser-like headers. The index, and the full list of templates read from it, are cached in memory for 5 minutes (`vpsCacheTTL`) for that URL; a changed `templates.url` is never served from the old cache, and a list where some template failed to download is not cached, so the next request tries again.

```json
{
  "minecraft-java": { "url": "minecraft-java.json" },
  "paper": { "url": "https://example.com/templates/paper.json" }
}
```

**Private repositories.** `TemplateRepo` has `pat`, `username`, `password` and `sshKey` columns, which `POST /api/templates` accepts and never returns. `repoAuth` turns them into go-git credentials for `cloneRepo` and `pullRepo`:

| Address | Credentials used |
|---|---|
| `https://` / `http://` | `pat` as HTTP basic auth password (user `username`, or `git` when empty); otherwise `username` + `password`; none when all are empty |
| SSH (`git@host:...`, `ssh://`) | `sshKey` (PEM private key; `password` unlocks it when encrypted), user `username`, else the user in the URL, else `git`. The host key must be in the `known_hosts` of the account running the panel. Without `sshKey`, no credentials |

The errors `repoAuth` returns name the repository and the kind of credential that could not be used, never the credentials or the address. The web client has no form for repositories yet; they are added through the API. Feature: [Templates](../features/templates.md).

## 12. `OAuth2` (`oauth2.go`)

`Get(clientID)` (by client id string), `GetForUser(userID)`, `Create`, `Update`, `Delete(clientID)`. `Delete` also deletes the sessions the client's credentials opened, in a transaction. OAuth2 flows: [API layer](./05-api-layer.md#oauth2).

## 13. `UserSettings` (`usersettings.go`)

`GetAllForUser(userID)` returns a `models.UserSettingsView` map; `Update(setting)` creates or saves the `(user, key)` row.

## 14. `Activity` (`activity.go`)

`Record(event, userID, username, email, ip, userAgent)` appends an access event; `List(limit, page, event)` returns newest first with a total. `ActivityRetention` = 90 days; older rows are deleted every time the list is read. Feature: [Activity and access log](../features/activity-and-access-log.md).

## 15. `ServerActivity` (`server_activity.go`)

| Method | Description |
|---|---|
| `Record(entry)` | Trims detail/username/IP, saves, and at most once an hour prunes old entries and entries of deleted servers |
| `Prune(now)` | Retention (90 days) and orphan cleanup |
| `DeleteForServer(serverID)` | Drops a server's log |
| `List(serverID, limit, before, filter)` | Newest first; `limit` default 50, max 100; `before` is the id cursor; returns the next cursor or nil |
| `TruncateRunes(text, limit)` | Package helper, cuts with `…` |

Filters: a family name (`power`, `console`, `files`, `backups`, `users`, `settings`, `tasks`, see `ServerActivityFamilies`) or a lowercase action prefix such as `file.` or `backup.restore`. Anything else matches nothing. The `settings` family includes `server.transferFailed` and `server.transferStartFailed`, which a node transfer records after the request was answered, with the step and the reason as detail (`internal/web/api/transfer_logic.go`).

## 16. `Uptime` (`uptime.go`)

| Method | Description |
|---|---|
| `TrackStatus(serverID, isRunning)` | Opens a period, closes it (with `duration`) when the state changes, or touches `updated_at`. Skips servers missing from the database. |
| `GetUptimeStats(serverID, since)` | Uptime/downtime seconds between `since` and now, running period included, and percent (100 when there is no data) |
| `GetRecentHistory(serverID, limit)` | Latest periods |
| `GetAllServerUptime(since)` | Map per server with `uptime`, `downtime`, `uptimePercent`, `currentStatus`, `currentStartTime` |
| `RecordNodeStatuses(serverIDs, reported, staleAfter)` | Records what a remote node reported for the servers the panel places on it (see below) |

`TrackStatus` is called every 5 seconds for every server the panel's own process runs, by the daemon's stats loop, only where the panel runs ([Process management & metrics](./13-process-metrics.md#uptime-tracking)). The stats read every period that overlaps the window (still open, or ended after `since`) and count only the part inside it: a period that began before `since` is clipped to `since`, and the open period runs until now. The running period is counted once: `GET /api/uptime/:serverId` does not add it again to `totalSeconds`. Feature overview: [Uptime](../general/13-uptime.md).

`RecordNodeStatuses` is called by the panel's remote status poller every 30 seconds for each remote node ([Process management & metrics](./13-process-metrics.md#uptime-tracking)). `reported` holds the state of each server the node answered for, and is `nil` when the node did not answer. Unknown is not stopped: a server the node left out, or every server of a node that did not answer, records nothing. Before recording, it closes the open periods of those servers that nobody confirmed for `staleAfter` (3 minutes), at their last confirmation (`updated_at`), so a long outage counts as neither up nor down (a gap between periods); a new period starts when the node answers again.

## 17. `SSHKey` (`sshkey.go`)

| Method | Description |
|---|---|
| `GetForUser(userID)` | Newest first |
| `Add(userID, name, line)` | One `authorized_keys` line; name defaults to the key comment (1–100 characters) |
| `Delete(userID, id)` | Only the user's own keys |
| `FindForLogin(userID, offered)` | Fingerprint lookup, then full key comparison |
| `ParseSSHPublicKey(line)` | Package function used by `Add` |

Limits: 25 keys per user (`ErrSSHKeyLimit`), no duplicates per user (`ErrSSHKeyExists`). Accepted: Ed25519, ECDSA P-256/384/521, `sk-` Ed25519/ECDSA and RSA ≥ 2048 bits. Refused: private keys, certificates, DSA, keys with options (`ErrSSHKeyIsPrivate`, `ErrSSHKeyWeak`, `ErrSSHKeyInvalid`).

## 18. `DatabaseSFTPAuthorization` (`sftp.go`)

Implements `skypanel.SFTPAuthorization` and `skypanel.SFTPKeyAuthorization`. The username is `email#serverId`.

- `Validate(username, password)`: same `LoginAttempts` budget as the web login; bcrypt check.
- `ValidateKey(username, key)`: key from the user's `SSHKey` list.
- Both then require `MaySignIn` and the `server.sftp` scope on that server, and return `ssh.Permissions` with the `server_id` extension. Every failure answers the same `incorrect username or password`.

See [Security: SFTP](./09-security.md#sftp-server).

## 19. `TokenService` (`token.go`)

```go
type TokenService interface {
    GetKeyFunc() jwt.Keyfunc
    GetTokenStore() jwkset.Storage
    GenerateRequest(node *models.Node) (string, error)
    ValidateRequest(string) error
}
```

- `NewTokenService()` keeps process-wide state. On a panel (`panel.enable`), the Ed25519 key comes from the seed in `panel.token`; if empty a seed is generated and saved. The public key is published through `GetTokenStore()` (served by `internal/web/auth/jwks.go`). On a daemon-only node, keys are fetched from `token.public`.
- `GenerateRequest(node)`: EdDSA JWT, `kid` `SkyPanel`, claims `iat`, `exp` (+15 min) and `aud` = SHA-256 of `"skypanel-daemon-aud:" + node secret`, so a token only works on the node it was minted for.
- `ValidateRequest(token)` (used by `middleware.ValidateJWT`): EdDSA only, `exp` required, 1 min leeway, `aud` must match the local node secret or `daemon.auth.clientSecret`.

## 20. `EmailService` (`email.go`)

- `LoadEmailService()` (startup) reads `emails.json` and the HTML bodies from the embedded `assets/email`, merged with `panel.email.templateFolder` when set (files there take precedence). Subjects and bodies are Go `html/template`.
- `GetEmailService().SendEmail(to, template, data, async)`: needs `panel.email.provider` (`ErrEmailNotConfigured`), adds `COMPANY_NAME`, `MASTER_URL` and `HOST_NAME` (the host of `panel.settings.masterUrl`, or the company name when the URL has no host) to `data`, renders, and sends with the provider from `internal/email`. With `async` the send runs in a goroutine and errors are only logged.
- Templates shipped: `accountCreation`, `passwordReset`, `login`, `passwordChanged`, `emailChanged`, `otpEnabled`, `recoveryCodesRegenerated`, `otpDisabled`, `oauthCreated`, `oauthDeleted`, `addedToServer`, `deletedServer`, `removedFromServer`, `test`.

Configuration: [Email](../general/15-email.md).

## 21. `DiscordService` (`discord.go`)

Singleton from `GetDiscordService()`.

| Method | Webhook | Notes |
|---|---|---|
| `SendWebhook(title, desc, color, fields)` | `panel.notifications.discordWebhook` | |
| `SendWebhookToURL(url, ...)` | given | Empty URL = silently skipped |
| `SendEmbedToURL(url, embed)` | given | Adds timestamp and footer "Aether Panel" |
| `SendAlert(title, desc, fields)` | main | Red |
| `SendServerOnlineAlert`, `SendServerOfflineAlert` | main | |
| `SendResourceAlert(name, id, resource, value, threshold)` | main | |
| `SendBackupAlert(name, id, status, ok)` | main | |
| `SendSystemStatus(servers []ServerInfo)` | `panel.notifications.discordWebhookSystem` | One field per server; above 25 servers (Discord's field limit) the first 24 and a note with how many are left out; the counts in the description cover all |
| `SendNodeStatus(...)` | `panel.notifications.discordWebhookNode` | Fields left empty or zero are left out |
| `SendNodeUnreachable(total, name, host, port)` | `panel.notifications.discordWebhookNode` | Red report "no answer" for a remote node that did not answer: only the number of servers placed on it, since their states and its machine are unknown |
| `ValidateDiscordWebhook(url)` | — | Package function |

Every post requires a real Discord webhook URL (`https://discord.com/api/webhooks/...`, also `ptb.`, `canary.` and `discordapp.com`) and goes through `utils.NewRestrictedHTTPClient`, which refuses private addresses. Embed texts come from the notification catalogue, in the language of `panel.settings.notificationLanguage` (see [Notification texts](#notification-texts-notification_textgo)).

`POST /api/settings/test/discord` (`internal/web/api/settings.go`) sends a test message to every configured webhook and names the ones it did not reach: `422 ErrDiscordTestFailed` with `metadata.webhooks` listing `alerts`, `reports`, `status` and/or `transfers` (the web client shows its own section names); with no webhook configured it answers `400 ErrDiscordNoWebhooks`.

The fourth webhook, `panel.notifications.discordWebhookExTransfer`, is used by the external transfer code in `internal/web/api`. More: [Discord](../general/16-discord.md).

## 22. `LicenseService` (`license.go`)

Singleton from `GetLicenseService()`; 30 s HTTP timeout.

- `VerifyLicense(key)`: `GET https://prueba.skyhostingcloud.com/api/public/licenses/verify?licenseKey=...`, with the key URL-encoded.
- `BindLicense(key, serverID, serverIP)`: `POST .../verify` with `{licenseKey, serverId, serverIp}`.
- `GetLicenseType(resp)`: `free` (invalid or plan `personal`), `pro` (`professional` or unknown plan), `enterprise`.
- `ExtractPermissions(resp)`: only `hasPlugins` (plans `professional`/`enterprise`).

The base URL is a constant in code and points at a test host (`prueba.skyhostingcloud.com`); it cannot be configured. More: [License](../general/14-license.md).

## 23. `Cloudflare` (`cloudflare.go`)

`NewCloudflare(token, zoneID)` builds a client for one zone (`https://api.cloudflare.com/client/v4`, 15 s timeout). Methods: `Zone`, `ListRecords(name)`, `CreateRecord`, `UpdateRecord(id, ...)`, `DeleteRecord(id)` (an already missing record counts as deleted) and `Verify(domain)` (reads the zone, checks the domain belongs to it, creates and deletes a TXT `_aether-panel-test.<domain>`). The token is only sent as a Bearer header and scrubbed from errors; zone and record ids must be 32 hex characters. Errors are panel error codes the web client translates: `ErrCloudflareNotConfigured`, `ErrCloudflareZoneID`, `ErrCloudflareRecordID`, `ErrCloudflareDomainNotInZone`, `ErrCloudflareUnreachable`, `ErrCloudflareBadAnswer`, and Cloudflare's own refusals as `ErrCloudflareRefused` or `ErrCloudflareRefusedCode`, with its message (and its error code) in the metadata. Used by `internal/web/api/server_dns.go` for server subdomains ([Servers: subdomain](./04-servers.md#subdomain-cloudflare), [Subdomains](../features/subdomains.md)).

---

## Helper modules

### `AttemptLimiter` (`login_attempts.go`)

In-memory sliding-window counter keyed by account, not IP (works the same behind a reverse proxy); reset on restart. `Take(key)` checks and counts atomically; also `Allowed`, `Record`, `Reset`. Expired keys are swept once more than 10,000 are tracked.

| Instance | Limit | Used by |
|---|---|---|
| `LoginAttempts` | 10 per 15 min | Panel login, 2FA codes, OAuth2 password grant, SFTP |
| `PasswordResetRequests` | 5 per hour | `internal/web/auth/password_reset.go` |

### `SSOLinks` (`sso.go`)

One-time sign-in links for billing systems (WHMCS, Paymenter): `Create(userID, redirect)` returns a 32-byte random token (only its SHA-256 kept, in memory); `Consume(token)` works once, within `SSOLinkLifetime` (1 minute). Created by the provisioning API (`internal/web/api/provision.go`), consumed by `internal/web/auth/sso.go`. Feature: [Provisioning and SSO](../features/provisioning-and-sso.md).

### Notification channels (`channels.go`)

- `ValidateTelegram(token, chat)`, `SendTelegram(token, chat, text)` (HTML parse mode), `TelegramHTML(text)`.
- `ValidateWebhook(url)` (https only, no credentials in the URL) and `SendWebhookJSON(url, secret, payload)`: with a secret, the body is signed in `X-Aether-Signature: sha256=<hex HMAC-SHA256>`; `User-Agent: AetherPanel-Webhook/1`.
- Both use the restricted HTTP client and a 10 s timeout; error texts never include the URL (a Telegram URL contains the bot token).

Used by per-server notifications ([Server notifications](../features/server-notifications.md)) and by extensions ([Extensions](../reference/15-extensions.md)).

### Notification texts (`notification_text.go`)

Every text the panel sends outside its own pages lives in one catalogue, `notificationTexts`, with a Spanish (`es`) and an English (`en`) version of each key; a test checks that both have the same keys and the same format verbs in the same order. It covers the panel's Discord alerts and reports (crash, offline/online, resources, backups, system and node reports, the Discord test), each owner's Discord, Telegram and JSON webhook alerts and their test, the title and message of extension events, and the external transfer reports and step texts.

| Function | Description |
|---|---|
| `NotificationLanguage()` | The value of `panel.settings.notificationLanguage` when the catalogue has it, otherwise `es` (`DefaultNotificationLanguage`) |
| `NotificationText(key, args...)` | The text in that language, filled in with `fmt` |
| `NotificationTextIn(lang, key, args...)` | The same in a given language: a key missing in it falls back to English, and a key missing everywhere is returned as it is |
| `IsNotificationLanguage(lang)` | Used by the settings API to refuse other values (`ErrNotificationLanguage`) |

Each process reads its own configuration: a remote node uses the value set on that node for its servers' alerts. In the JSON webhook payload of a resource alert, the `resource` field keeps `CPU` or `Memoria` whatever the language, for compatibility. The external transfer status API and the reason sent to the origin panel always use the English text, and the web client translates the step codes itself.

### `ReadDomainStatus` (`domain_status.go`)

Reads `domain-status.json` next to the panel's `config.json`, written by the installer's domain service (`tools/install/install.sh --apply-panel-url`). `State` is `working`, `ok`, `error` or `none` (file missing). Exposed by `GET /api/settings/domain-status` (scope `settings.edit`) and used for the local node address.
