# Backend — Overview

The Aether Panel backend is a single Go binary (module `github.com/SkyPanel/SkyPanel/v3`, binary and paths still named `SkyPanel`/`skypanel`) that contains the web panel, its API, the daemon that runs game servers, an SFTP server and the web client, embedded at build time.

## Technology stack

Versions are the ones in `go.mod`.

| Area | Technology |
|---|---|
| Language | **Go 1.26** |
| HTTP | **Gin** v1.12, `gin-contrib/cors`, `gin-contrib/sessions` |
| ORM | **GORM** v1.31 with **gormigrate** v2 (versioned migrations) |
| Databases | SQLite, MySQL/MariaDB, PostgreSQL, SQL Server (GORM drivers) |
| CLI | **Cobra** |
| Configuration | **Viper**, `SKYPANEL_*` environment variables |
| Auth | `golang-jwt/jwt/v5` with EdDSA (Ed25519), `jwkset` / `keyfunc` (JWKS), bcrypt, TOTP (`pquerna/otp`), OAuth2 token endpoint of its own |
| Scheduling | **gocron** v2 |
| WebSocket | **gorilla/websocket** |
| SFTP | `pkg/sftp` over `golang.org/x/crypto/ssh` (built-in server, not OpenSSH) |
| Containers | Docker SDK (`moby/moby` api v1.56, client v0.6) |
| Host processes | `creack/pty`, Linux namespaces (`unshare`), `openat2` |
| Game queries | `minequery` (Minecraft), Source A2S, `gorcon/rcon`, Telnet |
| Off-site backups | `minio-go` v7 (any S3-compatible storage) |
| Templates | `go-git` (Git repositories) or a JSON index |
| Archives | `mholt/archives`, `klauspost/compress` |
| Email | SMTP (`wneessen/go-mail`), Mailgun, Mailjet, SendGrid |
| AI | Google GenAI (Gemini) |
| Conditions | `google/cel-go` |
| System metrics | `gopsutil` |
| API docs | `swaggo` (Swagger / OpenAPI) |

## Package structure

```text
cmd/panel/              Entry point (Cobra)
├── main.go             Root command, --workDir and --config flags, config loading
├── run.go              `run`: starts panel and daemon, graceful shutdown
├── runservice.go       `runService`: same as run, plus systemd READY/STOPPING notifications
├── user.go             `user add` / `user edit`
├── version.go          `version`
└── db.go, dbmigrate.go, dbupgrade.go   `db` commands

internal/
├── config/             Typed configuration keys (entries.go) and file/env loading (config.go)
├── database/           GORM connection (loader.go) and migrations (upgrade.go)
├── models/             GORM models (25 tables) and API shapes
├── services/           Business layer over the database, see 11-services.md
├── middleware/         Gin middleware: auth, scopes, security headers, origin guard, API keys
├── web/                HTTP layer
│   ├── loader.go       RegisterRoutes: CORS, security, /daemon, /api, /oauth2, /auth, web client
│   ├── api/            Panel API (/api)
│   ├── auth/           Sign-in, 2FA, sign-up, password reset, SSO, captcha, JWKS (/auth)
│   ├── oauth2/         Token endpoint (/oauth2/token)
│   ├── daemon/         Daemon API (/daemon), network history, CPU sampler
│   └── swagger/        Generated OpenAPI files
├── servers/            Daemon core: servers, environments, scheduler, backups, metrics...
│   ├── docker/         Docker environment
│   └── tty/            Host environment (PTY + unshare)
├── operations/         Install/task operations (26 packages), see 06-operations.md
├── connections/        RCON, Telnet and RCON-over-websocket consoles
├── query/              Game queries (Minecraft ping, Source A2S)
├── sftp/               Built-in SFTP server
├── scopes/             The 73 permission scopes
├── oauth2/             Client side of the node → panel SFTP password check
├── email/              Email providers (smtp, mailgun, mailjet, sendgrid, debug)
├── extensions/         Installed extensions: files and event webhooks
├── offsite/            S3-compatible copy of backups
├── update/             Panel self-update through an ephemeral container
├── logging/            Logger and daily log rotation
├── response/           HTTP error helpers
├── groups/             System group check (used by the user CLI)
├── sys/                Syscall helpers
├── utils/              Helpers: downloads (with the private-address guard), kernel checks, JVM stats...
└── systemd/            No Go code: skypanel.service, AppArmor profile, Debian packaging templates

pkg/skypanel/           Shared types: Server definition, Environment, operations, trackers,
                        console cache, HTTP models, errors, version (set with ldflags)
files/                  Root-confined file system (openat2), compression, merged FS
conditions/             CEL condition evaluation
client/frontend/        Astro + React web client, embedded from client/frontend/dist
```

The `storage/` folder at the repository root is not code: `docker-compose.yml` keeps the volumes there (`storage/mysql-data`, `storage/skypanel-config`, `storage/skypanel-data`, `storage/skypanel-logs`).

## Start-up

```text
1. main()                          logging.Close and a panic handler (exit code 2) are deferred
2. Cobra OnInitialize
   ├── setWorkDir()                --workDir
   └── loadConfig()                --config > SKYPANEL_CONFIG > /etc/SkyPanel/config.json > ./config.json
3. run / runService → internalRun()
   ├── logging.Initialize, ignore SIGPIPE/SIGHUP, watch SIGINT/SIGTERM
   ├── Gin engine: Recovery, access log with secrets redacted, trusted proxies
   ├── if panel.enable → setupPanel()
   │   ├── email service, database SFTP authorisation, extension event sink
   │   ├── database.GetConnection() + database.Upgrade()   (migrations)
   │   ├── panel.sessionKey (generated if empty) → session cookie store, local node secret
   │   └── if daemon.enable: local node address kept in sync with the settings
   ├── if daemon.enable → daemon()
   │   ├── openat2 kernel check (security.forceOpenat skips it)
   │   ├── SFTP server, container mount source, image prefetch, network history
   │   ├── create servers/binaries/cache folders, add binaries to PATH
   │   ├── servers.LoadFromFolder() + servers.InitService()
   │   └── queue the servers marked to start automatically
   ├── web.RegisterRoutes()
   └── listen on web.host (ReadHeaderTimeout 10 s)
4. On SIGINT/SIGTERM → closePanel()
   ├── HTTP server shutdown (up to 10 s for requests in progress)
   ├── SFTP stop, service loop stop (metrics saved)
   ├── every game server asked to stop at once, up to 1 minute each
   └── database connections closed
```

If the daemon fails to start and `SKYPANEL_PLATFORM=docker`, the panel logs it and keeps running without the daemon; otherwise the process stops. `runService` also sends `READY=1` and `STOPPING=1` to `NOTIFY_SOCKET` when systemd provides it.

The CLI commands are described in [CLI](./07-cli.md).

## Operation modes

| Mode | Key | What it runs |
|---|---|---|
| **Panel** | `panel.enable` (default `true`) | `/api`, `/auth`, `/oauth2`, `/swagger`, the web client and the database |
| **Daemon** | `daemon.enable` (default `true`) | `/daemon`, the SFTP server and the game servers of this machine |

Both run together in a normal install: the panel's own daemon is the **local node** (ID `0`, not stored in the database). Extra machines run the same image with `panel.enable=false` as remote nodes.

The two sides authenticate differently in each direction:

- **Panel → node:** every call to `/daemon/*` carries a JWT the panel signs with its Ed25519 key (`panel.token`), valid for 15 minutes and bound to the target node's secret. A remote node verifies it with the panel's public key set (`token.public`, `/auth/publickey`).
- **Node → panel:** only to check SFTP passwords, through `POST /oauth2/token` (`grant_type=password`) with the node's secret as bearer token (`daemon.auth.url`, `daemon.auth.clientSecret`).

Details in [API layer](./05-api-layer.md#panel-to-node-requests) and [Configuration: remote node](./02-configuration.md#example-remote-node).

## Where to go next

| Topic | Page |
|---|---|
| Every configuration key | [Configuration](./02-configuration.md) |
| Tables and migrations | [Database](./03-database.md) |
| Environments, lifecycle, backups, disk | [Servers](./04-servers.md) |
| Routes, middleware, authentication | [API layer](./05-api-layer.md) |
| Install and task operations | [Operations](./06-operations.md) |
| Business services | [Services](./11-services.md) |
| Console websocket | [WebSocket](./08-websocket.md) |
| SFTP, tokens, API keys | [Security](./09-security.md) |
