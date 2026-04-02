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
*   **Plantillas:** Soporte para más de 24 tipos de juegos, incluyendo Minecraft, Terraria, Valheim, Rust y ARK, etc.

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
