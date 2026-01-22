<div align="center">

# 🚀 Aether Panel

### Panel de Gestión de Servidores de Juegos de Código Abierto

[**Documentación**](./docs/README.md) • [**Instalación**](./docs/01-installation.md) • [**API**](./docs/11-api-reference.md) • [**Discord**](https://discord.gg/aetherpanel) • [**Demo**](https://demo.aetherpanel.com)

</div>

---

## 📌 Sobre el Nombre del Proyecto

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
bash <(curl -s https://install.aetherpanel.com/install.sh)
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
- 📧 **Email**: support@aetherpanel.com

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

[Sitio Web](https://aetherpanel.com) • [Documentación](./docs/README.md) • [Discord](https://discord.gg/aetherpanel) • [Twitter](https://twitter.com/aetherpanel)

</div>
# Panel
# Panel
