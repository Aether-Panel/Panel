# Backend — Visión General

## Stack Tecnológico

| Componente | Tecnología |
|---|---|
| Lenguaje | **Go 1.25+** |
| Módulo | `github.com/SkyPanel/SkyPanel/v3` |
| HTTP Framework | **Gin** (`github.com/gin-gonic/gin` v1.12) |
| ORM | **GORM** (`gorm.io/gorm`) con `auto` y `gormigrate` |
| Base de Datos | SQLite3, MySQL, PostgreSQL, SQL Server |
| CLI | **Cobra** (`github.com/spf13/cobra`) |
| Config | **Viper** (`github.com/spf13/viper`) con variables de entorno `SKYPANEL_*` |
| Auth | JWT (`golang-jwt/jwt/v5`), Ed25519, OAuth2, cookies de sesión |
| Scheduling | **gocron** (`go-co-op/gocron/v2`) |
| WebSocket | **gorilla/websocket** |
| SFTP | **pkg/sftp** (propio, no OpenSSH) |
| Containers | **Docker SDK** (`docker/docker` v28) |
| Server Query | **minequery** (consulta de servidores de juego) |
| AI | **Gemini API** (Google GenAI) |
| Template Engine | Go `text/template` |
| Validation | `go-playground/validator` |
| CEL | `google/cel-go` para evaluar condiciones en operaciones |

## Estructura de Paquetes

```
cmd/panel/              → Punto de entrada (CLI + servidor HTTP)
├── main.go             → Cobra root, init, Execute()
├── run.go              → servidor HTTP (panel + daemon)
├── user.go             → CLI interactivo: add, edit
├── version.go          → CLI: version
├── db.go               → CLI: db operations
├── dbmigrate.go        → migraciones
└── dbupgrade.go        → upgrade de BD

internal/
├── config/             → Sistema de configuración tipado con Viper
│   ├── config.go       → LoadConfigFile, init con prefijo SKYPANEL_
│   └── entries.go      → Declaración tipada de todas las variables
├── database/           → Conexión GORM multi-dialecto
│   ├── loader.go       → openConnection, GetConnection, Close
│   └── upgrade.go      → gormigrate (migraciones versionadas)
├── models/             → 21 modelos GORM (Server, User, Node, etc.)
├── services/           → 21 servicios (capa de negocio)
├── middleware/         → 6 middleware Gin
├── web/               → Capa HTTP
│   ├── loader.go      → RegisterRoutes (router principal)
│   ├── api/           → Rutas REST de la API
│   ├── auth/          → Autenticación (login, logout, OTP, register)
│   ├── oauth2/        → OAuth2 token endpoint
│   ├── daemon/        → Daemon routes (proxy a nodos)
│   └── swagger/       → Documentación Swagger embebida
├── servers/           → Núcleo del Daemon
│   ├── server.go      → Server runtime, colas, stats, trackers
│   ├── env_loader.go  → Fábrica de entornos (Docker/TTY)
│   ├── scheduler.go   → gocron scheduler por servidor
│   ├── docker/        → Implementación Docker
│   └── tty/           → Implementación TTY (host/standard/bubblewrap)
├── operations/        → Sistema de operaciones (25 tipos)
├── connections/       → RCON, Telnet, WebSocket RCON
├── sftp/              → Servidor SFTP integrado
├── scopes/            → 73 scopes de permisos tipados
├── oauth2/            → Lógica OAuth2 (grant types)
├── email/             → Proveedor de email (Mailgun, SMTP)
├── response/          → Utilidades de respuesta HTTP
├── logging/           → Logger propio
├── query/             → Consulta de servidores de juego
├── storage/           → Abstracción de almacenamiento
├── groups/            → Verificación de grupos del sistema
├── utils/             → Utilidades generales
├── sys/               → Syscalls y manipulación de namespace
├── systemd/           → Integración systemd (notify, watchdog)
└── services/          → 21 servicios de negocio (GORM queries)

pkg/skypanel/          → Paquete compartido público
├── server.go          → Server definition, DaemonServer interface
├── environment.go     → Environment runtime, EnvironmentImpl interface
├── environmentfactory.go → EnvironmentFactory interface
├── operation.go       → Operation, OperationFactory interfaces
├── authorization.go   → AuthorizationResponse
├── errors.go          → Errores compartidos
├── version.go         → Version y Display string (ldflags)
├── engine.go          → Gin engine global
├── tracker.go         → Tracker pub/sub para WebSocket
├── cache.go           → MemoryCache para buffer de consola
├── console.go         → Console interface
├── stdin.go           → Stdin configuration
├── task.go            → Task definition para scheduler
├── message.go         → Mensajes WebSocket
├── download.go        → Download utilities
├── filelist.go        → File listing types
├── httpmodels.go      → HTTP request/response models
├── requirements.go    → System requirements check
├── client.go          → Cliente HTTP para panel→daemon
├── variable.go        → Variable type definition
├── typewithmetadata.go → MetadataType genérico
└── server_test.go     → Tests

files/                 → Sistema de archivos (compresión, paths)
conditions/            → Evaluación de condiciones CEL
```

## Flujo de Inicio

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
       ├── Viper init (prefijo SKYPANEL_, auto env)
       ├── Busca: --config > SKYPANEL_CONFIG > /etc/SkyPanel/config.json > config.json
       └── Lee archivo con viper.ReadInConfig()

4. internalRun() (run.go)
   ├── logging.Initialize(true)
   ├── signal.Ignore(SIGPIPE, SIGHUP)
   ├── database.GetConnection() → GORM (SQLite/MySQL/Postgres/SQLServer)
   ├── database.RunMigrations() → gormigrate (versionadas)
   ├── Configura cookie store para sesiones
   ├── web.RegisterRoutes(gin.Engine) → registra todas las rutas
   │   ├── CORS global (AllowOriginFunc: true)
   │   ├── /swagger/*any
   │   ├── /daemon/* (si daemon.enable=true)
   │   └── /api/*, /oauth2/*, /auth/*, frontend estático (si panel.enable=true)
   ├── servers.InitService() → cola de procesos, stats, systemStatus
   ├── SFTP server (goroutine, si configurado)
   ├── manners.GracefulServer (graceful shutdown con señales)
   └── Espera señal SIGINT/SIGTERM
```

## Modos de Operación

El binario puede ejecutarse en dos modos simultáneos:

| Modo | Flag | Servicios |
|---|---|---|
| **Panel** | `panel.enable` (default: true) | API REST, Auth, OAuth2, Frontend estático, SFTP |
| **Daemon** | `daemon.enable` (default: true) | Proxy a nodos, autenticación contra panel |

Ambos modos pueden ejecutarse juntos (monolito) o separados (panel central + nodos daemon). El daemon se autentica contra el panel usando OAuth2 Client Credentials.
