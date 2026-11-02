<div align="center">
  <img src="docs/images/logo.png" alt="Aether Panel Logo" width="200" height="auto" />

# Aether Panel
### Panel de gestión de servidores de juegos de código abierto
</div>

[Documentación](./docs/es/README.md) • [Instalación](./docs/es/setup/01-installation.md) • [Funcionalidades](./docs/es/features/README.md) • [API](./docs/es/reference/11-api-reference.md) • [Solución de problemas](./docs/es/setup/troubleshooting.md) • [English docs](./docs/en/README.md) • [Discord](https://discord.gg/aetherpanel)

---

## Sobre el proyecto

Aether Panel (nombre en clave interno: SkyPanel) es un panel de gestión de servidores de juegos para proveedores de hosting, comunidades y administradores de servidores.

Cada servidor de juego se ejecuta en su propio contenedor Docker, con un usuario sin privilegios y límites de recursos. El panel incluye gestión de usuarios con permisos por servidor, SFTP integrado, consola y métricas en tiempo real, copias de seguridad, tareas programadas, nodos remotos y una API REST.

**Versión:** 2.2.0

---

## Características principales

### Gestión de servidores
*   **Multi-servidor y multi-nodo:** gestiona servidores de distintos juegos en el nodo del panel y en nodos secundarios, desde un único panel maestro.
*   **Plantillas:** descargables del repositorio oficial (`templates.aetherpanel.es`: Minecraft y sus variantes, juegos de Steam, bases de datos, Node.js, Python…) y de repositorios propios.
*   **Contenedores Docker:** cada servidor en su contenedor, sobre las imágenes base `ghcr.io/aether-panel/{ubuntu,steamcmd,generic}`, que cada nodo descarga y mantiene al día en segundo plano. En la instalación con Docker solo se permiten servidores en contenedores (`docker.disallowHost`); el entorno de ejecución directa con aislamiento por namespaces (`tty` con `unshare`) existe en el código pero queda desactivado.
*   **Instalación automática:** al crear un servidor se instala solo, sin pulsar «Instalar» (`panel.servers.autoInstall`).
*   **Un asistente que evita fallos:** al crear un servidor avisa de los puertos ya ocupados (por el panel, otro servidor u otro contenedor) y propone uno libre, y avisa si la RAM o el disco pedidos no caben en el nodo.
*   **Límites reales:** el límite de disco se aplica de verdad (también a las subidas por SFTP), cada servidor tiene prioridad de E/S de disco configurable y un límite de procesos.
*   **Disco de los nodos vigilado:** el panel avisa cuando a un nodo le queda poco disco y no deja empezar una instalación sin espacio suficiente (`daemon.minFreeDiskMB`).
*   **Sin cortes al reiniciar:** si el panel o el daemon se reinician, se vuelven a conectar a los servidores que siguieron en marcha.
*   **Caídas a la vista:** cuando un servidor se cae, el panel lo avisa al momento con la causa, guarda un informe con las últimas líneas de la consola y puede explicarlo con IA.
*   **Modpacks y mundos:** instalación de modpacks y plugins (CurseForge, Modrinth) y mapa del mundo.
*   **Subdominios automáticos:** con Cloudflare, cada servidor tiene su subdominio, que se actualiza si cambia su puerto o su nodo.
*   **Bases de datos para los servidores:** hosts MySQL/MariaDB desde los que cada servidor crea sus propias bases de datos.

### Administración y seguridad
*   **Usuarios y roles:** permisos detallados por servidor; cada servidor tiene un propietario que invita a subusuarios y les da solo los permisos que elija.
*   **Autenticación:** sesiones con cookie, API keys (`ak_*`) con permisos limitados, OAuth2 (client credentials y password grant), JWT con endpoint JWKS.
*   **2FA/TOTP:** inicio de sesión en dos pasos con códigos de recuperación, y límite de intentos fallidos (compartido con el SFTP).
*   **Anti-bots sin configurar nada:** comprobación propia (prueba de trabajo) activada por defecto en el registro, el inicio de sesión y la recuperación de contraseña. Cloudflare Turnstile sigue disponible como opción.
*   **Juegos sin root:** cada juego corre con un usuario sin privilegios (`docker.serverUser`, 988:988), sin las capacidades de Linux que no necesita, con `no-new-privileges` y con un límite de procesos. Ver [Servidores](./docs/es/backend/04-servers.md#usuario-y-aislamiento-del-contenedor).
*   **Endurecimiento de la web:** cabeceras de seguridad, rechazo de peticiones con la cookie del panel iniciadas desde otra web, descargas de plantillas bloqueadas hacia redes privadas (`security.blockPrivateDownloads`) y límite de copias por servidor (`panel.backups.maxPerServer`). Ver [Seguridad](./docs/es/backend/09-security.md).

### Herramientas integradas
*   **Archivos:** servidor SFTP nativo (SSH + SFTP en Go) con contraseña o con claves SSH públicas, en todos los nodos; editor de archivos con historial.
*   **Panel web:** Astro, React 19 y Tailwind CSS, en inglés (por defecto) y español, con menú lateral que se encoge a una barra de iconos, consola ampliable e inicio de sesión y registro en una sola tarjeta.
*   **Monitorización:** CPU, memoria, disco y red por servidor y por nodo, con WebSocket en tiempo real para consola y métricas, e historial de disponibilidad.
*   **Copias de seguridad:** locales y en S3 o compatible (MinIO, R2…).
*   **Tareas programadas:** motor de tareas con gocron.
*   **Notificaciones:** Discord (embeds), Telegram y webhooks JSON por servidor, con idioma configurable (español o inglés), y correo (Mailgun, Mailjet, SendGrid, SMTP).
*   **Asistente de IA:** Google Gemini para analizar la consola y los logs.
*   **Facturación:** API de aprovisionamiento para WHMCS u otros sistemas: crea servidores con los límites del plan, cambia de plan y genera enlaces de acceso directo de un solo uso.
*   **Extensiones:** instala extensiones (`.zip`) que añaden páginas aisladas al panel y reciben eventos firmados. Ver [Extensiones](./docs/es/reference/15-extensions.md).
*   **API REST:** documentada con Swagger/OpenAPI en `/swagger/` (solo para administradores con sesión iniciada).
*   **Actualizaciones desde el panel:** **Ajustes → Actualizaciones** actualiza el panel y los nodos.

---

## Arquitectura

Aether Panel es un único binario en Go que, según su configuración, sirve el panel web y la API, el daemon que gestiona los servidores y el SFTP. En los nodos secundarios solo se activa el daemon (`panel.enable=false`).

1.  **Backend:** Go 1.26, Gin, GORM, Cobra y Viper. API HTTP, autenticación, base de datos, ciclo de vida de los servidores, SFTP y WebSocket.
2.  **Frontend:** Astro + React 19, Tailwind CSS 3 y Radix UI. Se compila dentro de la imagen, va embebido en el binario y se comunica con el backend solo por la API.
3.  **Base de datos:** MariaDB en la instalación con Docker. El código también admite SQLite, PostgreSQL y SQL Server.
4.  **Ejecución:** contenedores Docker a través del socket del host.

```
[Navegador (Astro/React)] <---> [Nginx (HTTPS)] <---> [Panel: API + daemon + SFTP (Go)] <---> [MariaDB]
                                                                  |
                                                     [Docker del host]  ---  [Nodos secundarios (daemon)]
                                                                  |
                                                     [Contenedores de juego (red skypanel-servers)]
```

---

## Instalación rápida

### Requisitos
*   Linux `amd64` o `arm64` con `apt`, `dnf`, `pacman` o `zypper`: probado en Ubuntu 22.04, 24.04, 26.04 y 26.10, Debian 12, Rocky Linux 9, Arch Linux y openSUSE Tumbleweed.
*   2 GB de RAM (la imagen se compila en el servidor) y 10 GB libres.
*   Docker Engine y Docker Compose v2 (el instalador los instala si faltan). Para puertos propios o un dominio, Docker Compose 2.24.4 o posterior.

Todos los detalles en la [Guía de instalación](./docs/es/setup/01-installation.md#requisitos-del-sistema).

### Opción 1: instalador automático (recomendado)

Asistente interactivo, como `root`:

```bash
bash -c "$(curl -fsSL https://install.aetherpanel.es/install.sh)"
```

Sin preguntas, con dominio y HTTPS:

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --non-interactive \
  --domain panel.ejemplo.com --ssl --email admin@ejemplo.com --admin-email admin@ejemplo.com
```

El instalador instala Docker, construye la imagen, configura Nginx, Let's Encrypt y el cortafuegos, y crea el administrador. También actualiza (`--update`, con copia de la base de datos y vuelta atrás automática), reinstala y desinstala. Todas las opciones en la [Guía de instalación](./docs/es/setup/01-installation.md#referencia-de-opciones).

### Opción 2: Docker Compose a mano

```bash
git clone -b main https://github.com/Aether-Panel/Panel.git /opt/skypanel
cd /opt/skypanel
# Cambie antes DB_ROOT_PASSWORD y DB_PASSWORD en .env (trae valores públicos de ejemplo)
docker compose up -d --build
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@ejemplo.com --admin
```

Ver [Método 2](./docs/es/setup/01-installation.md#método-2-docker-compose-a-mano).

### Desarrollo

Para compilar fuera de Docker hacen falta Go 1.26 o posterior y Node.js 22 o posterior. El frontend se compila primero porque el binario lo embebe:

```bash
cd client && yarn install && yarn build && cd ..
go build -o bin/SkyPanel ./cmd/panel
./bin/SkyPanel run
```

Con el `Makefile`, `make build` compila el paquete `./cmd/panel` en `bin/skypanel` y `make run` ejecuta `go run ./cmd/panel run` (el frontend se compila igualmente antes).

---

## Stack tecnológico

**Backend:** Go 1.26 · Gin · GORM · Cobra + Viper · cliente de Docker (moby) · Swaggo · gocron · gopsutil · Google GenAI

**Frontend:** Astro · React 19 · Radix UI · Tailwind CSS 3 · Monaco Editor · Recharts · React Hook Form + Zod

**Infraestructura:** Docker · MariaDB (también SQLite, PostgreSQL, SQL Server) · Nginx + Let's Encrypt (instalador) · SFTP nativo en Go

---

## CLI

```
SkyPanel run          Inicia el panel (web, daemon y SFTP según la configuración)
SkyPanel runService   Inicia el panel como servicio
SkyPanel version      Muestra la versión
SkyPanel user add     Añade un usuario (--name, --email, --password, --admin, --force)
SkyPanel user edit    Edita un usuario (nombre, email, contraseña, administrador, quitar 2FA)
SkyPanel db upgrade   Aplica las migraciones del esquema
SkyPanel db migrate   Aplica las migraciones pendientes, como `run` al arrancar (no hace nada con el panel desactivado)
```

Un comando que falla (desconocido, opciones incorrectas, o un error en `user add`, `user edit` o `db migrate`) termina con código de salida 1. `user edit` valida el nombre de usuario y el correo con las mismas reglas que la API y no acepta valores vacíos.

Opciones globales: `--config` (ruta del `config.json`) y `--workDir`. En Docker: `docker exec -it skypanel /SkyPanel/bin/SkyPanel …`. Ver [Comandos CLI](./docs/es/reference/14-cli-commands.md).

---

## CI/CD

GitHub Actions ejecuta en cada push y pull request a `main`, `dev2.0` y `fide-dev2.0`:

*   Tests de Go con detector de carreras y cobertura.
*   `gofmt`, `go vet`, `golangci-lint`, Staticcheck, gosec y Trivy (vulnerabilidades, secretos y configuración).
*   Frontend: lint, typecheck y compilación.
*   Tests E2E en Python.
*   Imagen multi-arquitectura (`amd64` y `arm64`), publicada en `ghcr.io/aether-panel/panel` desde `main`, e imágenes base de los servidores (`ubuntu`, `steamcmd`, `generic`) desde `main` y `dev2.0`.

Ejecución local con Docker:

```powershell
.\ci-local.ps1                     # Todas las etapas
.\ci-local.ps1 -Only tests         # Solo tests
.\ci-local.ps1 -Skip docker,trivy  # Saltar etapas
```

---

## Contribuir

Las contribuciones son bienvenidas. El proyecto se distribuye bajo la licencia Apache 2.0.

---

**Copyright 2026 Aether Panel Team**
Licenciado bajo Apache License, Versión 2.0.
