# Database

The panel database (`internal/database`, `internal/models`) holds users, roles, permissions, nodes, server rows, sessions, templates and the rest of the panel's state. It only exists where the panel runs (`panel.enable`); a node without the panel has no database. Server definitions, files, tasks and backups live on the nodes (see [Servers](./04-servers.md#per-server-files-on-the-node)).

## Supported dialects

Chosen with `panel.database.dialect` and `panel.database.url` (see [Configuration](./02-configuration.md#database)).

| Dialect | Driver | Connection string when `panel.database.url` is empty |
|---|---|---|
| `sqlite3` | `gorm.io/driver/sqlite` | `file:skypanel.db` |
| `mysql` | `gorm.io/driver/mysql` | `SkyPanel:SkyPanel@/SkyPanel` |
| `postgresql` | `gorm.io/driver/postgres` | — (required) |
| `sqlserver` | `gorm.io/driver/sqlserver` | — (required) |

Any other value fails with `unknown dialect`. The Docker install uses MariaDB through the `mysql` dialect; `config.linux.json` uses SQLite at `/var/lib/SkyPanel/database.db`.

### Parameters added to the connection string

`GetConnectionString()` appends these parameters unless the string already contains them:

```text
mysql:   charset=utf8  parseTime=true
sqlite3: cache=shared  _loc=auto  _foreign_keys=1  _journal_mode=WAL  _busy_timeout=5000  _tx_lock=immediate
```

With SQLite the pool is limited to one open connection and foreign keys are not created by the migrations (`DisableForeignKeyConstraintWhenMigrating`). GORM logs nothing except with `panel.database.log`, which logs every statement.

## Migrations

`database.Upgrade()` runs at every panel start (and from the `db` CLI commands, see [CLI](./07-cli.md)). It uses **gormigrate** with the `migrations` table (`id` column, 255 characters); each migration runs alone in its own transaction, and migrations already recorded are skipped. They are grouped in three batches, and the models are auto-migrated (`AutoMigrate` of every table below) right after the first batch.

| Batch | ID | What it does |
|---|---|---|
| 1 | `1726675832-mysql` | MySQL only: drops every foreign key, so the model migration can rebuild them. |
| 1 | `1726675832` | Drops every non-primary index, rebuilt by the model migration. |
| 1 | `20260304-serverid-harmonization` | MySQL: drops the foreign keys towards `servers.identifier` so the columns that reference it can be resized to 20 characters. |
| 1 | `20260819-server-extra-ports` | Adds the `ports` column to existing `servers` tables. |
| 1 | `20261016-server-internal-id` | Adds `internal_id` as nullable, fills a UUIDv7 into every existing server, then sets `NOT NULL` (MySQL, PostgreSQL). |
| — | model migration | `AutoMigrate` of every model. |
| 2 | `1658926619` | Kept as a no-op (it used to create a default template repository). |
| 2 | `1677250619` | Rewrites stored templates whose environment was `tty` or `standard` to `host`; a template that cannot be read is saved to `migrations/template-<name>.json` and removed. |
| 2 | `permissions-from-v2` | Converts the old boolean permission columns into scopes. |
| 3 | `20260617-default-roles` | Creates the `Administrador` (scope `admin`) and `Usuario` roles. |
| 3 | `20260624-usuario-role-templates-view` | Adds `templates.view` and `uptime.view` to `Usuario`. |
| 3 | `20260625-assign-usuario-role-to-existing-users` | Gives the `Usuario` role to users without a role. |
| 3 | `20260625-usuario-role-add-server-create` | Adds `server.create` to `Usuario` (sub-servers). |
| 3 | `20260820-usuario-role-config-ports` | Added three scopes to `Usuario`; reverted by the next migration. |
| 3 | `20260821-fix-usuario-role-scopes` | Removes `server.admin.config.view`, `server.admin.config.manage` and `server.data.edit.admin` from `Usuario` and adds `server.definition.view/edit` and `server.flags.view/edit`. |
| 3 | `20260811-template-raw-value-size` | Turns `templates.raw_value` into `MEDIUMTEXT` (MySQL) so large templates are not truncated. |
| 3 | `20260821-server-port-notes` | Migrates the `servers` table again (port notes). |
| 3 | `20261026-server-owners` | Adds `servers.owner_id` and names an owner for every server without one: the member with the most scopes on it, the earliest on a tie. |
| 3 | `20261016-server-internal-id-verify` | Read-only check that no server lacks `internal_id`; it only logs. |

## Tables

`AutoMigrate` covers 25 models (`dbObjects` in `internal/database/upgrade.go`). Columns marked *json* use GORM's JSON serializer.

### Accounts and access

| Model (table) | File | Primary key | Main columns |
|---|---|---|---|
| `User` (`users`) | `user.go` | `id` | `username` (100, unique), `email` (255, unique), `password` (bcrypt hash), `otp_secret`, `otp_active`, `otp_pending_secret`, `role_id` (nullable), timestamps |
| `Role` (`roles`) | `role.go` | `id` | `name` (100, unique), `description` (500), `scopes` (comma-separated, 2000), timestamps |
| `Permissions` (`permissions`) | `permission.go` | `id` | `user_id` or `client_id` (nullable), `server_identifier` (20, nullable: empty means panel-wide), `scopes` (comma-separated, 1000) |
| `Session` (`sessions`) | `session.go` | `id` | `token` (SHA-256 of the token, unique), `expiration_time`, `user_id` / `client_id` (nullable), `server_identifier` (nullable) |
| `Client` (`clients`) | `client.go` | `id` | `client_id` (100, unique), `client_secret` (bcrypt hash), `user_id`, `server_id` (nullable), `name`, `description`, `scopes` (read-only) |
| `RecoveryCode` (`recovery_codes`) | `recoverycode.go` | `id` | `user_id`, `code` (BLAKE2b-256 hash) |
| `PasswordReset` (`password_resets`) | `passwordreset.go` | `id` | `user_id`, `token` (BLAKE2b-256 hash, unique), `expires_at` (30 minutes) |
| `SSHKey` (`ssh_keys`) | `sshkey.go` | `id` | `user_id`, `name`, `fingerprint` (SHA256, unique per user), `public_key`, `type` |
| `APIKey` (`api_keys`) | `apikey.go` | `id` | `name`, `hashed_key` (SHA-256), `prefix` (8), `permissions` (*json*, allowed actions) |
| `UserSetting` (`user_settings`) | `usersetting.go` | `key` + `user_id` | `value` (4000) |
| `ActivityLog` (`activity_logs`) | `activity.go` | `id` | `event` (`login`, `login_failed`, `logout`, `register`, `password_reset`, `login_sso`), `user_id` (nullable), `username` and `email` (copied), `ip`, `user_agent`, `created_at` |

### Servers and nodes

| Model (table) | File | Primary key | Main columns |
|---|---|---|---|
| `Server` (`servers`) | `server.go` | `identifier` (20) | `internal_id` (UUIDv7, unique, not null), `name` (40), `node_id` (nullable, written only on create), `ip`, `port`, `ports` (*json*), `port_notes` (*json*), `type`, `icon`, `parent_server_id` (sub-servers), `total_cpu`, `total_memory`, `total_disk`, `suspended`, `owner_id` (nullable), `external_id` (billing), `subdomain` (63), `dns_record_ids` (*json*), timestamps |
| `Node` (`nodes`) | `node.go` | `id` | `name` (100, unique), `public_host`, `private_host`, `public_port` (8080), `private_port` (8080), `sftp_port` (5657), `secret`, `ssl`, timestamps |
| `ServerActivity` (`server_activities`) | `serveractivity.go` | `id` | `server_id`, `user_id`, `username` (copied), `action` (e.g. `server.start`, `file.write`), `detail` (300), `ip`, `created_at` |
| `UptimeStatus` (`uptime_statuses`) | `uptime.go` | `id` | `server_id`, `is_running`, `start_time`, `end_time` (nullable), `duration` (seconds), timestamps |
| `Backup` (`backups`) | `backup.go` | `id` | `name`, `file_name`, `file_size`, `location` (`local`, `offsite`, `both`), `server_id`, `created_at` |
| `ProvisionProduct` (`provision_products`) | `product.go` | `id` | `product_id` (unique), `display_name`, `template`, `node_id` (nullable: pick automatically), `cpu` (100 = one core), `memory` (MB), `disk` (MB), `default_node`, `port_range_min`, `port_range_max` |

The **local node** (the daemon running in the panel's own binary) is not a row: it is `models.LocalNode` (ID `0`), built from `panel.settings.masterUrl`, `daemon.sftp.host` and the master node IP, with a secret derived from `panel.sessionKey`. Saving it is refused.

Server identifiers are restricted to a URL- and path-safe set (new ones are 12 uppercase hexadecimal characters); `internal_id` is an immutable UUIDv7 also returned to clients as `internalId`.

### Databases for game servers

| Model (table) | File | Primary key | Main columns |
|---|---|---|---|
| `DatabaseHost` (`database_hosts`) | `databasehost.go` | `id` | `name`, `host`, `port` (3306), `username`, `password`, `max_databases`, `node_id` (nullable), timestamps |
| `Database` (`databases`) | `database.go` | `id` | `server_id` (FK, cascade), `database_host_id` (FK, cascade), `database_name`, `username`, `password`, `remote_connection`, `max_connections`, timestamps |

### Templates

| Model (table) | File | Primary key | Main columns |
|---|---|---|---|
| `Template` (`templates`) | `template.go` | `name` (100) | `raw_value` (`mediumtext`, the template JSON), `readme` |
| `TemplateRepo` (`template_repos`) | `templaterepo.go` | `id` | `name`, `url`, `branch` (`main`), `pat`, `username`, `password`, `ssh_key` (credentials of a private repository, stored as given and never returned by the API) |

`templates` holds the **local** templates (repository `0`). Remote repositories are Git repositories stored in `template_repos`; when `templates.url` is set they are ignored and the JSON index is used instead (see [Configuration](./02-configuration.md#general-and-logging)).

### Panel state

| Model (table) | File | Primary key | Main columns |
|---|---|---|---|
| `PanelSetting` (`panel_settings`) | `panelsetting.go` | `key` (100) | `value` (255). Today it holds `master_node_ip`. |
| `Extension` (`extensions`) | `extension.go` | `id` (40) | `name`, `version`, `author`, `description`, `enabled`, `manifest` (the `extension.json`), `webhook_url`, `secret` (signs `X-Aether-Signature`), timestamps |
| `ExTransferSession` (`ex_transfer_sessions`) | `extransfer.go` | `id` | `session_uuid` (unique), `server_id` (FK cascade), `user_id` (FK cascade), `token_hash` (unique), `status` (`CREATED` by default), `dest_host`, `dest_public_key`, `current_nonce`, `nonce_expires_at`, `protocol_version` (`1.0`), `payload`, `expires_at`, `last_heartbeat_at`, timestamps |
| `ExTransferLog` (`ex_transfer_logs`) | `extransfer.go` | `id` | `session_id` (FK to `session_uuid`, cascade), `action`, `ip_address`, `is_error`, `details`, `created_at` |

The other files in `internal/models` (`*view.go`, `*api.go`, `Deployment`…) are API shapes, not tables. Related guides: [Database hosts](../features/database-hosts.md), [Provisioning and SSO](../features/provisioning-and-sso.md), [Activity and access log](../features/activity-and-access-log.md), [Server transfers](../features/server-transfers.md).

## Relationships

```text
Role 1──N User
User 1──N Session, Permissions, Client, RecoveryCode, PasswordReset, SSHKey, UserSetting
User 1──N ActivityLog                 (user_id nullable: rows outlive the account)
User 1──N Server                      (owner_id, nullable)

Node 1──N Server                      (node_id nullable: the local node)
Server 1──N Permissions               (server_identifier)
Server 1──N Backup, UptimeStatus, ServerActivity
Server 1──N Database N──1 DatabaseHost
Server 1──N Server                    (parent_server_id: sub-servers)
Server 1──N ExTransferSession 1──N ExTransferLog

Node 1──N DatabaseHost, ProvisionProduct   (nullable)
```

`Permissions` rows tie a user (or an OAuth2 client) to a server with a list of scopes; a row without a server holds panel-wide scopes. How they combine with the role and the owner scopes is described in [API layer: scope checks](./05-api-layer.md#scope-checks).

## Retention and housekeeping

- **Sessions** last one hour from the last activity (sliding expiration); expired rows are never accepted, and creating a session deletes the expired ones at most once an hour.
- **Activity feed** (`activity_logs`) and **server activity** (`server_activities`) keep 90 days; older rows are pruned when the feed is read, and at most once an hour when server activity is recorded.
- **Password reset tokens** expire after 30 minutes and are consumed on use.
- Usernames and emails are copied into activity rows so they still read correctly after an account is renamed or deleted.

## Services

Each service in `internal/services` wraps the GORM queries of one area and receives the `*gorm.DB` when it is created (`&services.User{DB: db}`). They are documented in [Services](./11-services.md).
