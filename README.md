<div align="center">
  <img src="docs/images/logo.png" alt="Aether Panel Logo" width="200" height="auto" />

# Aether Panel
### Panel de Gestión de Servidores de Juegos de Código Abierto
</div>

[Documentación](./docs/README.md) • [Instalación](./docs/01-installation.md) • [API](./docs/11-api-reference.md) • [Discord](https://discord.gg/aetherpanel) • [Demo](https://demo.aetherpanel.es)

---

## Sobre el Proyecto

Aether Panel (nombre en clave interno: SkyPanel) es una solución moderna de gestión de servidores de juegos diseñada para proveedores de hosting, comunidades y administradores de servidores.

Esta plataforma ofrece una arquitectura robusta para gestionar múltiples servidores de juegos desde una interfaz centralizada. Proporciona despliegue automático mediante contenedores Docker, gestión de usuarios con sistema de permisos granular y herramientas de monitoreo en tiempo real.

**Versión Actual:** 1.0.1

---

## Características Principales

### Gestión de Servidores
*   **Multi-Servidor:** Capacidad para gestionar instancias ilimitadas de diferentes juegos.
*   **Contenerización:** Cada servidor se ejecuta en un contenedor Docker aislado para máxima seguridad y control de recursos.
*   **Plantillas:** Soporte para más de 24 tipos de juegos, incluyendo Minecraft, Terraria, Valheim, Rust y ARK.

### Administración y Seguridad
*   **Multi-Usuario:** Sistema completo de usuarios y roles con permisos detallados.
*   **Autenticación:** Soporte para OAuth2, JWT y autenticación de dos factores (2FA/TOTP).
*   **Seguridad:** Aislamiento de procesos y gestión segura de credenciales.

### Herramientas Integradas
*   **Monitoreo:** Integración nativa con Gatus para seguimiento de uptime y estado del servicio.
*   **Gestión de Archivos:** Servidor SFTP incorporado y editor de archivos web.
*   **Notificaciones:** Alertas automáticas vía Discord y correo electrónico.
*   **API RESTful:** Interfaz de programación completa para automatización e integración con terceros.

---

## Arquitectura del Sistema

Aether Panel opera como una aplicación monolítica modular con capacidad de despliegue distribuido (Panel + Nodos).

### Componentes Principales

1.  **Core (Backend):**
    *   Escrito en **Go**.
    *   Gestiona la API HTTP, autenticación, base de datos y lógica de negocio.
    *   Controla el ciclo de vida de los contenedores Docker.
    *   Provee el servidor SFTP integrado.

2.  **Interfaz de Usuario (Frontend):**
    *   Aplicación SPA (Single Page Application) construida con **Vue.js 3**.
    *   Desarrollada con Vite y estilizada con Tailwind CSS.
    *   Se comunica con el backend exclusivamente a través de la API REST.

3.  **Capa de Datos:**
    *   Soporte para **SQLite** (por defecto, archivo local) o **MySQL/MariaDB/PostgreSQL** para despliegues de producción.

4.  **Infraestructura de Ejecución:**
    *   Utiliza **Docker** para aislar cada servidor de juego.
    *   Cada instancia tiene su propio sistema de archivos, límites de recursos y red virtual.

```
[Cliente Web (Vue.js)] <---> [API Gateway (Go)] <---> [Base de Datos]
                                    |
                                    v
                           [Docker Engine]
                                    |
                       [Contenedores de Juegos]
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
  aetherpanel/aetherpanel:latest
```

### Opción 3: Construcción Manual
Requiere Go 1.21+ y Node.js 18+.

1.  Clonar el repositorio.
2.  Compilar el frontend (`yarn build` en `client/frontend`).
3.  Compilar el backend (`go build` en raíz).
4.  Ejecutar el binario generado.

Para instrucciones detalladas, consulte la [Guía de Instalación Completa](./docs/01-installation.md).

---

## Stack Tecnológico

**Backend**
*   Lenguaje: Go 1.24.4
*   Framework Web: Gin
*   ORM: GORM
*   Contenedores: Docker SDK

**Frontend**
*   Framework: Vue.js 3
*   Build Tool: Vite
*   Estilos: Tailwind CSS

**Infraestructura**
*   Base de Datos: SQLite, MySQL, PostgreSQL
*   Monitoreo: Gatus
*   Transferencia: Servidor SFTP nativo (Go)

---

## Contribuir

Las contribuciones son bienvenidas. Por favor, consulte las guías de contribución antes de enviar un Pull Request. El proyecto se distribuye bajo la licencia Apache 2.0.

---

**Copyright 2024 Aether Panel Team**
Licenciado bajo Apache License, Versión 2.0.

Aether Panel (nombre en clave interno: SkyPanel) es una solución moderna de gestión de servidores de juegos diseñada para proveedores de hosting, comunidades y administradores de servidores.

Esta plataforma ofrece una arquitectura robusta para gestionar múltiples servidores de juegos desde una interfaz centralizada. Proporciona despliegue automático mediante contenedores Docker, gestión de usuarios con sistema de permisos granular y herramientas de monitoreo en tiempo real.

**Versión Actual:** 1.0.1

---

## Características Principales

### Gestión de Servidores
*   **Multi-Servidor:** Capacidad para gestionar instancias ilimitadas de diferentes juegos.
*   **Contenerización:** Cada servidor se ejecuta en un contenedor Docker aislado para máxima seguridad y control de recursos.
*   **Plantillas:** Soporte para más de 24 tipos de juegos (Minecraft, Terraria, Valheim, Rust, ARK, etc.).

### Administración y Seguridad
*   **Multi-Usuario:** Sistema completo de usuarios y roles con permisos detallados.
*   **Autenticación:** Soporte para OAuth2, JWT y autenticación de dos factores (2FA/TOTP).
*   **Seguridad:** Aislamiento de procesos y gestión segura de credenciales.

### Herramientas Integradas
*   **Monitoreo:** Integración nativa con Gatus para seguimiento de uptime y estado del servicio.
*   **Gestión de Archivos:** Servidor SFTP incorporado y editor de archivos web.
*   **Notificaciones:** Alertas automáticas vía Discord y correo electrónico.
*   **API RESTful:** Interfaz de programación completa para automatización e integración con terceros.

---

## Arquitectura del Sistema

Aether Panel utiliza una arquitectura de microservicios híbrida:

1.  **Backend (Go):** Núcleo del sistema, maneja la lógica de negocio, bases de datos y orquestación de Docker.
2.  **Frontend (Vue.js):** Interfaz de usuario reactiva y moderna.
3.  **Daemon:** Agente ligero que se ejecuta en los nodos remotos para gestionar los contenedores.

```
[Cliente Web] <--> [API Gateway / Panel] <--> [Base de Datos]
                          ^
                          |
                  [Daemon / Nodo]
                          |
             [Contenedores de Juegos (Docker)]
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
  aetherpanel/aetherpanel:latest
```

### Opción 3: Construcción Manual
Requiere Go 1.21+ y Node.js 18+.

1.  Clonar el repositorio.
2.  Compilar el frontend (`yarn build` en `client/frontend`).
3.  Compilar el backend (`go build` en raíz).
4.  Ejecutar el binario generado.

Para instrucciones detalladas, consulte la [Guía de Instalación Completa](./docs/01-installation.md).

---

## Stack Tecnológico

**Backend**
*   Lenguaje: Go 1.24.4
*   Framework Web: Gin
*   ORM: GORM (Soporte SQLite, MySQL, PostgreSQL)
*   Contenedores: Docker SDK

**Frontend**
*   Framework: Vue.js 3
*   Build Tool: Vite
*   Estilos: Tailwind CSS

**Infraestructura**
*   Base de Datos: SQLite (por defecto) o MySQL/MariaDB
*   Monitoreo: Gatus
*   Transferencia: Servidor SFTP nativo en Go

---

## Contribuir

Las contribuciones son bienvenidas. Por favor, consulte las guías de contribución antes de enviar un Pull Request. El proyecto se distribuye bajo la licencia Apache 2.0.

---

**Copyright 2024 Aether Panel Team**
Licenciado bajo Apache License, Versión 2.0.

**Aether Panel** es el nombre oficial del proyecto. **SkyPanel** es el nombre en clave (codename) utilizado internamente en el código fuente, binarios, servicios del sistema y comandos CLI.

> **Versión Actual**: 3.0.0

---

## 📖 Descripción

**Aether Panel** es un panel de gestión de servidores de juegos moderno, potente y fácil de usar. Diseñado para proveedores de hosting, comunidades gaming y administradores de servidores, Aether Panel ofrece una interfaz intuitiva y una arquitectura robusta para gestionar múltiples servidores de juegos desde un solo lugar.

### ✨ Características Principales

- 🎮 **Multi-Servidor**: Gestiona múltiples servidores de diferentes juegos
- 🌐 **Multi-Nodo**: Distribuye servidores en múltiples máquinas
- 📊 **Monitoreo en Tiempo Real**: Integración con Gatus para uptime monitoring
- 🖥️ **Consola Web**: Acceso a la consola del servidor via WebSocket
- 📁 **Gestión de Archivos**: SFTP integrado para transferencia segura
- 💾 **Sistema de Backups**: Respaldos automáticos y programados
- 🔐 **Seguridad Avanzada**: OAuth2, JWT, 2FA (TOTP)
- 🎨 **Plantillas**: 24+ tipos de operaciones pre-configuradas
- 🔔 **Notificaciones**: Integración con Discord y Email
- 🐳 **Docker**: Soporte nativo para contenedores
- 🔌 **API RESTful**: Automatización completa via API
- 🌍 **Multi-idioma**: Soporte para múltiples idiomas

---

## 🎯 Casos de Uso

### Para Proveedores de Hosting
- Gestión centralizada de servidores de clientes
- Sistema de permisos granular por usuario
- API para integración con sistemas de facturación
- Monitoreo automático de uptime

### Para Comunidades Gaming
- Administrar múltiples servidores (Minecraft, Terraria, Valheim, etc.)
- Roles de administrador/moderador
- Notificaciones Discord automáticas
- Consola web para gestión remota

### Para Administradores
- Panel único para todos tus servidores
- Backups automáticos
- Monitoreo de recursos
- Gestión de archivos vía SFTP

---

## 🚀 Instalación Rápida

### Instalación Automática (Recomendado)

```bash
bash <(curl -s https://install.aetherpanel.es/install.sh)
```

### Instalación con Docker

```bash
docker run -d \
  --name skypanel \
  -p 8080:8080 \
  -p 5657:5657 \
  -v skypanel-data:/var/lib/skypanel \
  aetherpanel/aetherpanel:latest
```

### Instalación Manual

```bash
# Clonar repositorio
git clone https://github.com/aetherpanel/aetherpanel.git
cd aetherpanel

# Compilar frontend
cd client/frontend
yarn install && yarn build

# Compilar backend
cd ../..
go build -o skypanel ./cmd

# Ejecutar
./skypanel run
```

📖 **[Ver Guía Completa de Instalación](./docs/01-installation.md)**

---

## 🎮 Juegos Soportados

Aether Panel soporta una amplia variedad de juegos mediante su sistema de plantillas:

| Juego | Versiones | Plantillas |
|-------|-----------|------------|
| **Minecraft** | Java, Bedrock | Vanilla, Forge, Paper, Fabric, Spigot, Bungeecord |
| **Terraria** | 1.4+ | Vanilla, TShock |
| **Valheim** | Latest | Vanilla, Mods |
| **ARK: Survival Evolved** | Latest | Vanilla, Mods |
| **Rust** | Latest | Vanilla, Oxide |
| **CS:GO** | Latest | Vanilla, SourceMod |
| **Garry's Mod** | Latest | Vanilla, DarkRP |
| **7 Days to Die** | Latest | Vanilla, Mods |
| **Project Zomboid** | Latest | Vanilla, Mods |
| **Satisfactory** | Latest | Vanilla |

Y muchos más... 🎯

---

## 🏗️ Arquitectura

```
┌─────────────────────────────────────────────────────────────┐
│                    Aether Panel System                       │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │   Web Panel  │    │   Frontend   │    │   Backend    │  │
│  │  (Next.js)   │    │   (Vue.js)   │    │    (Go)      │  │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘  │
│         │                   │                    │           │
│         └───────────────────┴────────────────────┘           │
│                             │                                │
│                    ┌────────┴────────┐                       │
│                    │                 │                       │
│              ┌─────▼─────┐    ┌─────▼─────┐                │
│              │  Database │    │   Gatus   │                │
│              │  (SQLite) │    │ (Monitor) │                │
│              └───────────┘    └───────────┘                │
│                                                               │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              Game Servers Layer                        │  │
│  ├───────────────────────────────────────────────────────┤  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐            │  │
│  │  │ Server 1 │  │ Server 2 │  │ Server N │            │  │
│  │  │ (Docker) │  │ (Native) │  │ (Docker) │            │  │
│  │  └──────────┘  └──────────┘  └──────────┘            │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 💻 Stack Tecnológico

### Backend
- **Lenguaje**: Go 1.24.4
- **Framework**: Gin (HTTP)
- **ORM**: GORM
- **Base de Datos**: SQLite, MySQL, PostgreSQL, SQL Server
- **Autenticación**: OAuth2, JWT
- **WebSocket**: Gorilla WebSocket
- **Contenedores**: Docker SDK

### Frontend
- **Framework**: Vue.js 3
- **Build**: Yarn
- **UI**: Custom components
- **i18n**: Vue I18n

### Admin Panel
- **Framework**: Next.js 15.5.9
- **Lenguaje**: TypeScript
- **UI**: React 19 + Tailwind CSS + Radix UI
- **Backend**: Firebase
- **IA**: Google Genkit

---

## 📚 Documentación

La documentación completa está disponible en el directorio [`docs/`](./docs/README.md):

### 🏁 Primeros Pasos
- [Instalación](./docs/01-installation.md)
- [Configuración](./docs/02-configuration.md)
- [Crear tu Primer Servidor](./docs/03-creating-servers.md)

### 👥 Gestión
- [Gestión de Usuarios](./docs/04-user-management.md)
- [Gestión de Nodos](./docs/05-node-management.md)
- [Gestión de Servidores](./docs/06-server-management.md)

### 🔧 Avanzado
- [Sistema de Plantillas](./docs/07-templates.md)
- [Monitoreo con Gatus](./docs/08-monitoring.md)
- [Integración Discord](./docs/09-discord-integration.md)
- [Referencia de API](./docs/11-api-reference.md)

---

## 🔌 API

Aether Panel incluye una API RESTful completa para automatización:

```bash
# Autenticación OAuth2
POST /oauth2/token

# Listar servidores
GET /api/servers

# Crear servidor
POST /api/servers

# Iniciar servidor
POST /api/servers/:id/start

# Consola WebSocket
WS /api/servers/:id/console
```

📖 **[Ver Documentación Completa de API](./docs/11-api-reference.md)**

---

## 🛠️ Desarrollo

### Requisitos

- Go 1.21+
- Node.js 18+
- Yarn
- SQLite3 (o MySQL/PostgreSQL)

### Configurar Entorno de Desarrollo

```bash
# Clonar repositorio
git clone https://github.com/aetherpanel/aetherpanel.git
cd aetherpanel

# Instalar dependencias de Go
go mod download

# Instalar dependencias del frontend
cd client/frontend
yarn install

# Ejecutar en modo desarrollo
# Terminal 1: Backend
go run ./cmd run

# Terminal 2: Frontend
cd client/frontend
yarn dev
```

### Ejecutar Tests

```bash
# Tests de Go
go test ./...

# Tests con cobertura
go test -cover ./...
```

---

## 🤝 Contribuir

¡Las contribuciones son bienvenidas! Por favor lee nuestra [Guía de Contribución](./docs/27-contributing.md) antes de enviar un Pull Request.

### Proceso de Contribución

1. 🍴 Fork el proyecto
2. 🌿 Crea tu rama de feature (`git checkout -b feature/AmazingFeature`)
3. 💾 Commit tus cambios (`git commit -m 'Add some AmazingFeature'`)
4. 📤 Push a la rama (`git push origin feature/AmazingFeature`)
5. 🔃 Abre un Pull Request

---

## 🐛 Reportar Bugs

¿Encontraste un bug? Por favor [abre un issue](https://github.com/aetherpanel/aetherpanel/issues/new) con:

- Versión de Aether Panel (actualmente 3.0.0)
- Sistema operativo
- Pasos para reproducir
- Logs relevantes

---

## 💬 Comunidad

- 💬 **Discord**: [discord.gg/aetherpanel](https://discord.gg/aetherpanel)
- 🐦 **Twitter**: [@AetherPanel](https://twitter.com/aetherpanel)
- 📧 **Email**: support@aetherpanel.es

---

## 📄 Licencia

Aether Panel está licenciado bajo la [Apache License 2.0](LICENSE).

```
Copyright 2024 Aether Panel Team

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

---

## 🌟 Agradecimientos

Aether Panel es posible gracias a:

- Todos los contribuidores del proyecto
- La comunidad de código abierto
- [Go](https://golang.org/), [Vue.js](https://vuejs.org/), [Next.js](https://nextjs.org/)
- [Gatus](https://github.com/TwiN/gatus) por el sistema de monitoreo
- Y muchos otros proyectos increíbles

---

## 📊 Estado del Proyecto

**Versión Actual**: 3.0.0

Aether Panel (nombre en clave: SkyPanel) es un proyecto activo en desarrollo continuo.

---

<div align="center">

**Hecho con ❤️ por el equipo de Aether Panel**

[Sitio Web](https://aetherpanel.es) • [Documentación](./docs/README.md) • [Discord](https://discord.gg/aetherpanel) • [Twitter](https://twitter.com/aetherpanel)

</div>
# Panel
# Panel
