<div align="center">
  <img src="docs/images/logo.png" alt="Aether Panel Logo" width="200" height="auto" />

# Aether Panel
### Panel de Gestión de Servidores de Juegos de Código Abierto
</div>

[Documentación](./docs/README.md) • [Instalación](./docs/setup/01-installation.md) • [API](./docs/reference/11-api-reference.md) • [Discord](https://discord.gg/aetherpanel)

---

## Sobre el Proyecto

Aether Panel (nombre en clave interno: SkyPanel) es una solución moderna de gestión de servidores de juegos diseñada para proveedores de hosting, comunidades y administradores de servidores.

Ofrece una arquitectura flexible con dos backends de ejecución —Docker y ejecución directa con aislamiento por namespaces (unshare)—, gestión de usuarios con permisos granulares, servidor SFTP integrado, y monitorización en tiempo real.

**Versión:** 2.0.0

---

## Características Principales

### Gestión de Servidores
*   **Multi-Servidor:** Capacidad para gestionar instancias ilimitadas de diferentes juegos mediante plantillas descargables.
*   **Dos Backends de Ejecución:**
    *   **Docker:** Cada servidor se ejecuta en un contenedor Docker aislado.
    *   **Host/TTY:** Ejecución directa con aislamiento por contenedores de Linux (namespaces: user, mount, cgroup, ipc, uts) y `pivot_root`.
*   **Plantillas: ** Soporte para más de 24 tipos de juegos (Minecraft, Terraria, Valheim, Rust, ARK, etc.) mediante repositorios de plantillas externos.

### Administración y Seguridad
*   **Multi-Usuario:** Sistema completo de usuarios y roles con permisos detallados por servidor.
*   **Autenticación Múltiple:** Sesiones con cookie/token, API keys (`ak_*`), OAuth2 (client credentials + password grant), JWT con JWKS endpoint.
*   **2FA/TOTP:** Autenticación de dos factores vía códigos temporales.
*   **Aislamiento:** Contenedores Docker o namespaces de Linux con `pivot_root` y mapeo de usuarios.

### Herramientas Integradas
*   **Gestión de Archivos:** Servidor SFTP nativo (SSH + SFTP sobre Go) con autenticación por OAuth2 o base de datos.
*   **Panel Web:** Interfaz moderna construida con Astro, React 19 y Tailwind CSS.
*   **Notificaciones:** Alertas automáticas vía Discord (embeds enriquecidos) y correo electrónico (Mailgun, Mailjet, SendGrid, SMTP).
*   **API RESTful:** Documentada con Swagger/OpenAPI, disponible en `/swagger/`.
*   **Monitorización:** Estadísticas de CPU, memoria y disco por servidor vía `gopsutil`; WebSocket en tiempo real para console y métricas.
*   **Programador de Tareas:** Motor de tareas programadas con gocron.
*   **AI Asistente:** Integración con Google GenAI para asistencia en consola/logs.

---

## Arquitectura del Sistema

Aether Panel opera como una aplicación monolítica modular con capacidad de despliegue distribuido (Panel + Nodos remotos).

### Componentes Principales

1.  **Core (Backend):**
    *   Escrito en **Go** (1.25).
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
El método recomendado para la mayoría de los usuarios en sistemas Linux (Ubuntu/Debian/CentOS).

```bash
bash <(curl -s https://install.aetherpanel.es/install.sh)
```

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
Requiere Go 1.25+ y Node.js 22+.

1.  Clonar el repositorio.
2.  Compilar el frontend (`yarn build` en `client/`).
3.  Compilar el backend (`make build` en raíz).
4.  Ejecutar el binario generado (`make run`).

Para instrucciones detalladas, consulte la [Guía de Instalación Completa](./docs/setup/01-installation.md).

---

## Stack Tecnológico

**Backend**
*   Lenguaje: Go 1.25
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
*   Autenticación: Sesiones, OAuth2, JWT/JWKS, API Keys, 2FA/TOTP

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
