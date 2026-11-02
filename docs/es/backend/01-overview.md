# Backend — Visión general

El backend de Aether Panel es un único binario Go (módulo `github.com/SkyPanel/SkyPanel/v3`; el binario y las rutas siguen llamándose `SkyPanel`/`skypanel`) que contiene el panel web, su API, el daemon que ejecuta los servidores de juego, un servidor SFTP y el cliente web, incluido al compilar.

## Tecnologías

Las versiones son las de `go.mod`.

| Área | Tecnología |
|---|---|
| Lenguaje | **Go 1.26** |
| HTTP | **Gin** v1.12, `gin-contrib/cors`, `gin-contrib/sessions` |
| ORM | **GORM** v1.31 con **gormigrate** v2 (migraciones versionadas) |
| Bases de datos | SQLite, MySQL/MariaDB, PostgreSQL, SQL Server (drivers de GORM) |
| CLI | **Cobra** |
| Configuración | **Viper**, variables de entorno `SKYPANEL_*` |
| Autenticación | `golang-jwt/jwt/v5` con EdDSA (Ed25519), `jwkset` / `keyfunc` (JWKS), bcrypt, TOTP (`pquerna/otp`), endpoint de tokens OAuth2 propio |
| Planificación | **gocron** v2 |
| WebSocket | **gorilla/websocket** |
| SFTP | `pkg/sftp` sobre `golang.org/x/crypto/ssh` (servidor propio, no OpenSSH) |
| Contenedores | SDK de Docker (`moby/moby` api v1.56, cliente v0.6) |
| Procesos en el host | `creack/pty`, espacios de nombres de Linux (`unshare`), `openat2` |
| Consultas a juegos | `minequery` (Minecraft), Source A2S, `gorcon/rcon`, Telnet |
| Copias externas | `minio-go` v7 (cualquier almacenamiento compatible con S3) |
| Plantillas | `go-git` (repositorios Git) o un índice JSON |
| Archivos comprimidos | `mholt/archives`, `klauspost/compress` |
| Correo | SMTP (`wneessen/go-mail`), Mailgun, Mailjet, SendGrid |
| IA | Google GenAI (Gemini) |
| Condiciones | `google/cel-go` |
| Métricas del sistema | `gopsutil` |
| Documentación de la API | `swaggo` (Swagger / OpenAPI) |

## Estructura de paquetes

```text
cmd/panel/              Punto de entrada (Cobra)
├── main.go             Comando raíz, opciones --workDir y --config, carga de configuración
├── run.go              `run`: arranca panel y daemon, apagado ordenado
├── runservice.go       `runService`: igual que run, más avisos READY/STOPPING a systemd
├── user.go             `user add` / `user edit`
├── version.go          `version`
└── db.go, dbmigrate.go, dbupgrade.go   comandos `db`

internal/
├── config/             Claves de configuración tipadas (entries.go) y carga de archivo/entorno (config.go)
├── database/           Conexión GORM (loader.go) y migraciones (upgrade.go)
├── models/             Modelos GORM (25 tablas) y formas de la API
├── services/           Capa de negocio sobre la base de datos, ver 11-services.md
├── middleware/         Middleware de Gin: autenticación, permisos, cabeceras de seguridad, control de origen, claves de API
├── web/                Capa HTTP
│   ├── loader.go       RegisterRoutes: CORS, seguridad, /daemon, /api, /oauth2, /auth, cliente web
│   ├── api/            API del panel (/api)
│   ├── auth/           Inicio de sesión, 2FA, registro, restablecer contraseña, SSO, captcha, JWKS (/auth)
│   ├── oauth2/         Endpoint de tokens (/oauth2/token)
│   ├── daemon/         API del daemon (/daemon), historial de red, muestreo de CPU
│   └── swagger/        Archivos OpenAPI generados
├── servers/            Núcleo del daemon: servidores, entornos, planificador, copias, métricas...
│   ├── docker/         Entorno Docker
│   └── tty/            Entorno host (PTY + unshare)
├── operations/         Operaciones de instalación y tareas (26 paquetes), ver 06-operations.md
├── connections/        Consolas RCON, Telnet y RCON sobre websocket
├── query/              Consultas a juegos (ping de Minecraft, Source A2S)
├── sftp/               Servidor SFTP propio
├── scopes/             Los 73 permisos (scopes)
├── oauth2/             Lado cliente de la comprobación de contraseñas SFTP nodo → panel
├── email/              Proveedores de correo (smtp, mailgun, mailjet, sendgrid, debug)
├── extensions/         Extensiones instaladas: archivos y webhooks de eventos
├── offsite/            Copia de las copias de seguridad en almacenamiento compatible con S3
├── update/             Autoactualización del panel mediante un contenedor efímero
├── logging/            Registro y rotación diaria de logs
├── response/           Utilidades para respuestas de error HTTP
├── groups/             Comprobación de grupos del sistema (la usa la CLI de usuarios)
├── sys/                Utilidades de llamadas al sistema
├── utils/              Utilidades: descargas (con el filtro de direcciones privadas), comprobaciones del kernel, estadísticas JVM...
└── systemd/            Sin código Go: skypanel.service, perfil de AppArmor, plantillas de empaquetado Debian

pkg/skypanel/           Tipos compartidos: definición de servidor, Environment, operaciones, trackers,
                        caché de consola, modelos HTTP, errores, versión (fijada con ldflags)
files/                  Sistema de archivos confinado a la raíz (openat2), compresión, FS combinado
conditions/             Evaluación de condiciones CEL
client/frontend/        Cliente web Astro + React, incluido desde client/frontend/dist
```

La carpeta `storage/` de la raíz del repositorio no es código: `docker-compose.yml` guarda ahí los volúmenes (`storage/mysql-data`, `storage/skypanel-config`, `storage/skypanel-data`, `storage/skypanel-logs`).

## Arranque

```text
1. main()                          se difieren logging.Close y un manejador de pánicos (código de salida 2)
2. OnInitialize de Cobra
   ├── setWorkDir()                --workDir
   └── loadConfig()                --config > SKYPANEL_CONFIG > /etc/SkyPanel/config.json > ./config.json
3. run / runService → internalRun()
   ├── logging.Initialize, ignora SIGPIPE/SIGHUP, vigila SIGINT/SIGTERM
   ├── motor Gin: Recovery, registro de accesos sin secretos, proxies de confianza
   ├── si panel.enable → setupPanel()
   │   ├── servicio de correo, autorización SFTP contra la base de datos, envío de eventos a extensiones
   │   ├── database.GetConnection() + database.Upgrade()   (migraciones)
   │   ├── panel.sessionKey (se genera si está vacía) → almacén de la cookie de sesión, secreto del nodo local
   │   └── si daemon.enable: la dirección del nodo local se mantiene al día con los ajustes
   ├── si daemon.enable → daemon()
   │   ├── comprobación de openat2 en el kernel (security.forceOpenat la omite)
   │   ├── servidor SFTP, origen de montajes del contenedor, precarga de imágenes, historial de red
   │   ├── crea las carpetas servers/binaries/cache y añade binaries al PATH
   │   ├── servers.LoadFromFolder() + servers.InitService()
   │   └── encola los servidores marcados para arrancar automáticamente
   ├── web.RegisterRoutes()
   └── escucha en web.host (ReadHeaderTimeout 10 s)
4. Con SIGINT/SIGTERM → closePanel()
   ├── apagado del servidor HTTP (hasta 10 s para las peticiones en curso)
   ├── parada de SFTP y del bucle de servicio (se guardan las métricas)
   ├── se pide a todos los servidores de juego que paren a la vez, hasta 1 minuto cada uno
   └── se cierran las conexiones a la base de datos
```

Si el daemon no arranca y `SKYPANEL_PLATFORM=docker`, el panel lo registra y sigue funcionando sin daemon; en otro caso el proceso se detiene. `runService` además envía `READY=1` y `STOPPING=1` a `NOTIFY_SOCKET` cuando systemd lo proporciona.

Los comandos de la CLI se describen en [CLI](./07-cli.md).

## Modos de funcionamiento

| Modo | Clave | Qué ejecuta |
|---|---|---|
| **Panel** | `panel.enable` (por defecto `true`) | `/api`, `/auth`, `/oauth2`, `/swagger`, el cliente web y la base de datos |
| **Daemon** | `daemon.enable` (por defecto `true`) | `/daemon`, el servidor SFTP y los servidores de juego de esta máquina |

En una instalación normal ambos corren juntos: el daemon del propio panel es el **nodo local** (ID `0`, no se guarda en la base de datos). Otras máquinas ejecutan la misma imagen con `panel.enable=false` como nodos remotos.

Los dos lados se autentican de forma distinta en cada dirección:

- **Panel → nodo:** cada llamada a `/daemon/*` lleva un JWT que el panel firma con su clave Ed25519 (`panel.token`), válido 15 minutos y ligado al secreto del nodo de destino. Un nodo remoto lo verifica con el conjunto de claves públicas del panel (`token.public`, `/auth/publickey`).
- **Nodo → panel:** solo para comprobar contraseñas SFTP, mediante `POST /oauth2/token` (`grant_type=password`) con el secreto del nodo como token bearer (`daemon.auth.url`, `daemon.auth.clientSecret`).

Detalles en [Capa API](./05-api-layer.md#peticiones-del-panel-a-los-nodos) y [Configuración: nodo remoto](./02-configuration.md#ejemplo-nodo-remoto).

## Dónde seguir

| Tema | Página |
|---|---|
| Todas las claves de configuración | [Configuración](./02-configuration.md) |
| Tablas y migraciones | [Base de datos](./03-database.md) |
| Entornos, ciclo de vida, copias, disco | [Servidores](./04-servers.md) |
| Rutas, middleware, autenticación | [Capa API](./05-api-layer.md) |
| Operaciones de instalación y tareas | [Operaciones](./06-operations.md) |
| Servicios de negocio | [Servicios](./11-services.md) |
| Websocket de consola | [WebSocket](./08-websocket.md) |
| SFTP, tokens, claves de API | [Seguridad](./09-security.md) |
