# Backend — Overview

## Technology Stack

| Component | Technology |
|---|---|
| Language | **Go 1.25+** |
| Module | `github.com/SkyPanel/SkyPanel/v3` |
| HTTP Framework | **Gin** (`github.com/gin-gonic/gin` v1.12) |
| ORM | **GORM** (`gorm.io/gorm`) with `auto` and `gormigrate` |
| Database | SQLite3, MySQL, PostgreSQL, SQL Server |
| CLI | **Cobra** (`github.com/spf13/cobra`) |
| Config | **Viper** (`github.com/spf13/viper`) with `PUFFER_*` environment variables |
| Auth | JWT (`golang-jwt/jwt/v5`), Ed25519, OAuth2, session cookies |
| Scheduling | **gocron** (`go-co-op/gocron/v2`) |
| WebSocket | **gorilla/websocket** |
| SFTP | **pkg/sftp** (proprietary, not OpenSSH) |
| Containers | **Docker SDK** (`docker/docker` v28) |
| Server Query | **minequery** (game server query) |
| AI | **Gemini API** (Google GenAI) |
| Template Engine | Go `text/template` |
| Validation | `go-playground/validator` |
| CEL | `google/cel-go` for evaluating conditions in operations |

## Package Structure

```
cmd/panel/              → Entry point (CLI + HTTP server)
├── main.go             → Cobra root, init, Execute()
├── run.go              → HTTP server (panel + daemon)
├── user.go             → Interactive CLI: add, edit
├── version.go          → CLI: version
├── db.go               → CLI: db operations
├── dbmigrate.go        → migrations
└── dbupgrade.go        → DB upgrade

internal/
├── config/             → Typed configuration system with Viper
│   ├── config.go       → LoadConfigFile, init with PUFFER_ prefix
│   └── entries.go      → Typed declaration of all variables
├── database/           → Multi-dialect GORM connection
│   ├── loader.go       → openConnection, GetConnection, Close
│   └── upgrade.go      → gormigrate (versioned migrations)
├── models/             → 34 GORM models (Server, User, Node, etc.)
├── services/           → 21 services (business layer)
├── middleware/         → 6 Gin middleware
├── web/               → HTTP Layer
│   ├── loader.go      → RegisterRoutes (main router)
│   ├── api/           → API REST Routes
│   ├── auth/          → Authentication (login, logout, OTP, register)
│   ├── oauth2/        → OAuth2 token endpoint
│   ├── daemon/        → Daemon routes (proxy to nodes)
│   └── swagger/       → Embedded Swagger documentation
├── servers/           → Daemon Core
│   ├── server.go      → Server runtime, queues, stats, trackers
│   ├── env_loader.go  → Environment factory (Docker/TTY)
│   ├── scheduler.go   → per-server gocron scheduler
│   ├── docker/        → Docker Implementation
│   └── tty/           → TTY Implementation (host/standard/bubblewrap)
├── operations/        → Operations System (25 types)
├── connections/       → RCON, Telnet, WebSocket RCON
├── sftp/              → Integrated SFTP server
├── scopes/            → 73 typed permission scopes
├── oauth2/            → OAuth2 Logic (grant types)
├── email/             → Email Provider (Mailgun, SMTP)
├── response/          → HTTP Response Utilities
├── logging/           → Custom Logger
├── query/             → Game server query
├── storage/           → Storage Abstraction
├── groups/            → System group verification
├── utils/             → General Utilities
├── sys/               → Syscalls and namespace manipulation
├── systemd/           → systemd integration (notify, watchdog)
└── services/          → 21 business services (GORM queries)

pkg/skypanel/          → Public Shared Package
├── server.go          → Server definition, DaemonServer interface
├── environment.go     → Environment runtime, EnvironmentImpl interface
├── environmentfactory.go → EnvironmentFactory interface
├── operation.go       → Operation, OperationFactory interfaces
├── authorization.go   → AuthorizationResponse
├── errors.go          → Shared Errors
├── version.go         → Version and Display string (ldflags)
├── engine.go          → Global Gin engine
├── tracker.go         → Pub/sub Tracker for WebSocket
├── cache.go           → MemoryCache for console buffer
├── console.go         → Console interface
├── stdin.go           → Stdin configuration
├── task.go            → Task definition for scheduler
├── message.go         → WebSocket Messages
├── download.go        → Download utilities
├── filelist.go        → File listing types
├── httpmodels.go      → HTTP request/response models
├── requirements.go    → System requirements check
├── client.go          → HTTP Client for panel→daemon
├── variable.go        → Variable type definition
├── typewithmetadata.go → Generic MetadataType
└── server_test.go     → Tests

files/                 → Filesystem (compression, paths)
conditions/            → CEL condition evaluation
```

## Startup Flow

```
1. cmd/panel/main.go
   ├── defer logging.Close()
   ├── defer recover() → logging.Error
   └── Execute() (Cobra)

2. init()
   ├── Flags --workDir, --config
   ├── cobra.OnInitialize → setWorkDir(), loadConfig()
   └── AddCommand(run, version, user, runService, db)

3. loadConfig()
   └── config.LoadConfigFile(configFile)
       ├── Viper init (PUFFER_ prefix, auto env)
       ├── Searches: --config > PUFFER_CONFIG > /etc/SkyPanel/config.json > config.json
       └── Reads file with viper.ReadInConfig()

4. internalRun() (run.go)
   ├── logging.Initialize(true)
   ├── signal.Ignore(SIGPIPE, SIGHUP)
   ├── database.GetConnection() → GORM (SQLite/MySQL/Postgres/SQLServer)
   ├── database.RunMigrations() → gormigrate (versioned)
   ├── Configures cookie store for sessions
   ├── web.RegisterRoutes(gin.Engine) → registers all routes
   │   ├── Global CORS (AllowOriginFunc: true)
   │   ├── /swagger/*any
   │   ├── /daemon/* (if daemon.enable=true)
   │   └── /api/*, /oauth2/*, /auth/*, static frontend (if panel.enable=true)
   ├── servers.InitService() → process queue, stats, systemStatus
   ├── SFTP server (goroutine, if configured)
   ├── manners.GracefulServer (graceful shutdown with signals)
   └── Waits for SIGINT/SIGTERM signal
```

## Operation Modes

The binary can run in two simultaneous modes:

| Mode | Flag | Services |
|---|---|---|
| **Panel** | `panel.enable` (default: true) | REST API, Auth, OAuth2, Static Frontend, SFTP |
| **Daemon** | `daemon.enable` (default: true) | Proxy to nodes, authentication against panel |

Both modes can run together (monolith) or separately (central panel + daemon nodes). The daemon authenticates against the panel using OAuth2 Client Credentials.