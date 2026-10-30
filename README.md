<div align="center">
  <img src="docs/images/logo.png" alt="Aether Panel Logo" width="200" height="auto" />

# Aether Panel
### Panel de Gestión de Servidores de Juegos de Código Abierto
</div>

[Documentación](./docs/es/README.md) • [Instalación](./docs/es/setup/01-installation.md) • [API](./docs/es/reference/11-api-reference.md) • [English docs](./docs/en/README.md) • [Discord](https://discord.gg/aetherpanel)

---

## Sobre el Proyecto

Aether Panel (nombre en clave interno: SkyPanel) es una solución moderna de gestión de servidores de juegos diseñada para proveedores de hosting, comunidades y administradores de servidores.

Ofrece una arquitectura flexible con dos backends de ejecución —Docker y ejecución directa con aislamiento por namespaces (unshare)—, gestión de usuarios con permisos granulares, servidor SFTP integrado, y monitorización en tiempo real.

**Versión:** 2.1.0

---

## Características Principales

### Gestión de Servidores
*   **Multi-Servidor:** Capacidad para gestionar instancias ilimitadas de diferentes juegos mediante plantillas descargables.
*   **Dos Backends de Ejecución:**
    *   **Docker:** Cada servidor se ejecuta en un contenedor Docker aislado.
    *   **Host/TTY:** Ejecución directa con aislamiento por contenedores de Linux (namespaces: user, mount, cgroup, ipc, uts) y `pivot_root`.
*   **Plantillas:** 56 plantillas en el repositorio oficial (Minecraft y sus variantes, Palworld, Rust, ARK, Valheim, FiveM, bases de datos, Node.js, Python…) y repositorios propios.
*   **Instalación automática:** Al crear un servidor se instala solo, sin pulsar «Instalar».
*   **Un asistente que evita fallos:** Al crear un servidor avisa de los puertos ya ocupados (por el panel, otro servidor u otro contenedor) y propone uno libre, y avisa si la RAM o el disco pedidos no caben en el nodo.
*   **Límites reales:** El límite de disco se aplica de verdad (también a las subidas por SFTP) y cada servidor tiene prioridad de E/S de disco configurable.
*   **Sin cortes al reiniciar:** Si el panel o el daemon se reinician, se vuelven a conectar a los servidores que siguieron en marcha.
*   **Subdominios automáticos:** Con Cloudflare, cada servidor tiene su subdominio, que se actualiza si cambia su puerto o su nodo.

### Administración y Seguridad
*   **Multi-Usuario:** Sistema completo de usuarios y roles con permisos detallados por servidor.
*   **Propietario y subusuarios:** Cada servidor tiene un propietario, que invita a subusuarios y les da solo los permisos que elija.
*   **API keys con permisos:** Cada clave (por ejemplo, la del sistema de facturación) solo puede hacer las acciones que se le dieron.
*   **Autenticación Múltiple:** Sesiones con cookie/token, API keys (`ak_*`), OAuth2 (client credentials + password grant), JWT con JWKS endpoint.
*   **2FA/TOTP:** Autenticación de dos factores vía códigos temporales, con inicio de sesión en dos pasos y códigos de recuperación.
*   **Anti-bots sin configurar nada:** Comprobación anti-bots propia (prueba de trabajo) activada por defecto en el registro, el inicio de sesión y la recuperación de contraseña. Cloudflare Turnstile sigue disponible como opción.
*   **Aislamiento:** Contenedores Docker o namespaces de Linux con `pivot_root` y mapeo de usuarios.
*   **Juegos sin root:** En Docker, cada juego corre con un usuario sin privilegios, sin los permisos de Linux que no necesita y con un límite de procesos, como en Pterodactyl. Los servidores existentes se migran solos al arrancar. Ver [Servidores](./docs/es/backend/04-servers.md#usuario-y-aislamiento-del-contenedor).
*   **Disco de los nodos vigilado:** El panel avisa a los administradores cuando a un nodo le queda poco disco y no deja empezar una instalación si no hay espacio.

### Herramientas Integradas
*   **Gestión de Archivos:** Servidor SFTP nativo (SSH + SFTP sobre Go) con contraseña o, en el nodo del panel, con claves SSH públicas.
*   **Panel Web:** Interfaz moderna construida con Astro, React 19 y Tailwind CSS, con menú lateral que se encoge a una barra de iconos y un inicio de sesión y registro en una sola tarjeta.
*   **Caídas a la vista:** Cuando un servidor se cae, el panel lo avisa al momento con la causa, guarda un informe con las últimas líneas de la consola y puede explicarlo con IA.
*   **Notificaciones:** Alertas automáticas vía Discord (embeds enriquecidos) y correo electrónico (Mailgun, Mailjet, SendGrid, SMTP).
*   **API RESTful:** Documentada con Swagger/OpenAPI, disponible en `/swagger/`.
*   **Monitorización:** Estadísticas de CPU, memoria y disco por servidor vía `gopsutil`; WebSocket en tiempo real para console y métricas.
*   **Programador de Tareas:** Motor de tareas programadas con gocron.
*   **AI Asistente:** Integración con Google GenAI para asistencia en consola/logs.
*   **Facturación:** API de aprovisionamiento para WHMCS u otros sistemas: crea servidores con los límites del plan, cambia de plan y genera enlaces de acceso directo de un solo uso.
*   **Extensiones:** Instala extensiones (`.zip`) que añaden páginas aisladas al panel y reciben eventos firmados. Ver [Extensiones](./docs/es/reference/15-extensions.md).

---

## Arquitectura del Sistema

Aether Panel opera como una aplicación monolítica modular con capacidad de despliegue distribuido (Panel + Nodos remotos).

### Componentes Principales

1.  **Core (Backend):**
    *   Escrito en **Go** (1.26).
    *   Framework web **Gin**, ORM **GORM**, CLI **Cobra**.
    *   Gestiona la API HTTP, autenticación, base de datos y lógica de negocio.
    *   Controla el ciclo de vida de servidores (Docker o directo).
    *   Provee el servidor SFTP integrado y WebSocket para consola/estadísticas.

2.  **Interfaz de Usuario (Frontend):**
    *   Aplicación SPA construida con **Astro + React 19**.
    *   Estilizada con **Tailwind CSS 3** y componentes **Radix UI**.
    *   Se comunica con el backend exclusivamente a través de la API REST.

3.  **Capa de Datos:**
    *   Soporte para **SQLite** (por defecto), **MySQL/MariaDB**, **PostgreSQL** y **SQL Server**.

4.  **Infraestructura de Ejecución:**
    *   **Docker:** Contenedores aislados con límites de recursos.
    *   **Host (directo):** Ejecución con namespaces (user, mount, cgroup, ipc, uts) y `pivot_root` para aislamiento de sistema de archivos.

```
[Cliente Web (Astro/React)] <---> [API (Go/Gin)] <---> [Base de Datos]
                                      |
                          +-----------+-----------+
                          |                       |
                    [Docker Engine]      [Unshare Namespaces]
                          |                       |
              [Contenedores de Juegos]   [Procesos Aislados]
```

---

## Instalación Rápida

### Opción 1: Instalación Automática (Script)
El método recomendado. Probado en Ubuntu 22.04, 24.04, 26.04 y 26.10, Debian 12, Rocky Linux 9, Arch Linux y openSUSE (amd64 y arm64).

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash
```

Opciones (`--non-interactive`, `--dry-run`, dominio y SSL…) en la [Guía de Instalación](./docs/es/setup/01-installation.md#opciones-útiles).

### Opción 2: Despliegue con Docker
Para ejecutar el panel dentro de un contenedor Docker.

```bash
docker run -d \
  --name skypanel \
  -p 8080:8080 \
  -p 5657:5657 \
  -v skypanel-data:/var/lib/skypanel \
  ghcr.io/aether-panel/panel:latest
```

### Opción 3: Construcción Manual
Requiere Go 1.26+ y Node.js 22+.

1.  Clonar el repositorio.
2.  Compilar el frontend (`yarn build` en `client/`).
3.  Compilar el backend (`make build` en raíz).
4.  Ejecutar el binario generado (`make run`).

Para instrucciones detalladas, consulte la [Guía de Instalación Completa](./docs/es/setup/01-installation.md).

---

## Stack Tecnológico

**Backend**
*   Lenguaje: Go 1.26
*   Framework Web: Gin
*   ORM: GORM
*   CLI: Cobra + Viper
*   Contenedores: Docker SDK
*   Aislamiento: unshare (namespaces Linux)
*   Documentación API: Swaggo / Swagger
*   Tareas programadas: gocron
*   Estadísticas: gopsutil

**Frontend**
*   Framework: Astro + React 19
*   Componentes: Radix UI
*   Estilos: Tailwind CSS 3
*   Editor: Monaco Editor
*   Charts: Recharts
*   Formularios: React Hook Form + Zod

**Infraestructura**
*   Base de Datos: SQLite, MySQL/MariaDB, PostgreSQL, SQL Server
*   Transferencia: SFTP nativo (Go)
*   Notificaciones: Discord, Mailgun, Mailjet, SendGrid, SMTP
*   Autenticación: Sesiones, OAuth2, JWT/JWKS, API Keys, 2FA/TOTP, claves SSH (SFTP)

---

## CLI (Command Line Interface)

```
SkyPanel run          Inicia el panel completo (web, daemon, SFTP)
SkyPanel runService   Inicia como servicio systemd
SkyPanel version      Muestra la versión
SkyPanel user add     Agrega un nuevo usuario
SkyPanel user edit    Edita un usuario existente
SkyPanel db upgrade   Ejecuta migraciones de base de datos
SkyPanel db migrate   Migra entre backends de base de datos (experimental)
```

---

## CI/CD

El proyecto utiliza **GitHub Actions** para integración continua:

*   `gofmt`, `go vet`, `golangci-lint`, `staticcheck`, `gosec`
*   Escaneo de vulnerabilidades con **Trivy**
*   Tests unitarios con detector de carreras (`-race`)
*   Tests de frontend (lint, typecheck, build)
*   Tests E2E en Python
*   Build multi-arquitectura (amd64 + arm64) y publish a `ghcr.io/aether-panel/panel`

Ejecución local con Docker:
```powershell
.\ci-local.ps1              # Todos los stages
.\ci-local.ps1 -Only tests  # Solo tests
.\ci-local.ps1 -Skip docker # Saltar build de Docker
```

---

## Contribuir

Las contribuciones son bienvenidas. El proyecto se distribuye bajo la licencia Apache 2.0.

---

**Copyright 2026 Aether Panel Team**
Licenciado bajo Apache License, Versión 2.0.
