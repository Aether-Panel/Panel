# Documentación de Aether Panel

Documentación para instalar, operar, usar y ampliar Aether Panel. Describe lo que hace el código hoy; cuando una funcionalidad es experimental o está implementada solo en parte, su página lo indica.

> El producto se llama Aether Panel. El módulo de Go, el binario y algunos identificadores (contenedor `skypanel`, variables de entorno `SKYPANEL_*`, cookie `skypanel_auth`) conservan el nombre SkyPanel.

La misma documentación está disponible en inglés en [`docs/en`](../en/README.md), con los mismos nombres de archivo.

## Por dónde empezar

1. [Guía de instalación](./setup/01-installation.md): requisitos, el instalador y sus opciones, actualización y desinstalación.
2. [Primeros pasos](./general/getting-started.md): de una instalación nueva al primer servidor en marcha.
3. [Conceptos básicos](./general/concepts.md): nodos, servidores, plantillas, usuarios, roles y permisos, SFTP.
4. [Uso del panel](./general/usages.md): dónde está cada cosa en la interfaz.
5. [Preguntas frecuentes](./general/faq.md) y [Solución de problemas](./setup/troubleshooting.md).

## Instalación y operación

| Página | Contenido |
|--------|-----------|
| [Guía de instalación](./setup/01-installation.md) | Sistemas compatibles, opciones de `install.sh`, dominio y HTTPS, nodos, actualizaciones, desinstalación |
| [Solución de problemas](./setup/troubleshooting.md) | Fallos conocidos y cómo resolverlos |
| [Documentación Docker](./docker/README.docker.md) | El panel con Docker Compose, volúmenes, imágenes de ejecución, entorno de desarrollo |
| [Referencia rápida de Docker](./docker/DOCKER-QUICKREF.md) | Comandos de Docker de uso diario para el panel |
| [Socket de Docker y permisos](./docker/DOCKER-SUDO-FIX.md) | Acceso al socket de Docker y errores de permisos |
| [Configuración](./backend/02-configuration.md) | Todas las claves de configuración, su valor por defecto y su variable de entorno |
| [Comandos CLI](./reference/14-cli-commands.md) | Todos los comandos y opciones del binario |

## Funcionalidades

Guías para usuarios y administradores, una página por funcionalidad. Lista completa: [Funcionalidades](./features/README.md).

- Páginas del servidor: [resumen y estadísticas](./features/server-statistics.md), [configuración](./features/server-settings.md), [editor de `server.properties`](./features/server-properties.md), [versión del juego](./features/game-versions.md), [modpacks y plugins](./features/modpacks-and-plugins.md), [mapa del mundo](./features/world-map.md), [tareas programadas](./features/scheduled-tasks.md), [copias de seguridad](./features/backups.md), [historial de archivos](./features/file-history.md), [informes de caída](./features/crash-reports.md), [notificaciones](./features/server-notifications.md), [subdominios](./features/subdomains.md), [asistente de IA](./features/ai-assistant.md) y su [trabajo con archivos](./features/ai-agent.md).
- Ciclo de vida del servidor: [clonación](./features/cloning.md), [subservidores](./features/sub-servers.md), [transferencias](./features/server-transfers.md).
- Administración del panel: [panel de inicio y nodos](./features/dashboard-and-nodes.md), [actividad y registro de accesos](./features/activity-and-access-log.md), [hosts de bases de datos](./features/database-hosts.md), [configuración del panel](./features/panel-settings.md), [plantillas](./features/templates.md), [actualizaciones del panel](./features/panel-updates.md), [integración con facturación y enlaces de acceso](./features/provisioning-and-sso.md).

## Integraciones

| Página | Contenido |
|--------|-----------|
| [Correo electrónico](./general/15-email.md) | Proveedores de correo y plantillas |
| [Discord y notificaciones](./general/16-discord.md) | Webhooks y embeds del panel |
| [Captcha / anti-bots](./general/17-turnstile.md) | Comprobación anti-bots propia (activada por defecto, sin claves) y Cloudflare Turnstile opcional |
| [Seguimiento de disponibilidad](./general/13-uptime.md) | Cómo se registra y se muestra la disponibilidad de los servidores |
| [Sistema de licencias](./general/14-license.md) | Comprobación de licencias contra la API externa |

## Seguridad y permisos

| Página | Contenido |
|--------|-----------|
| [Seguridad](./general/security.md) | Recomendaciones para administradores, inicio de sesión en dos pasos, sesiones, claves SSH, endurecimiento |
| [Referencia de permisos](./general/scopes-reference.md) | Todos los scopes, roles por defecto, propietario y subusuarios, permisos de las claves API |
| [Seguridad (interna)](./backend/09-security.md) | Scopes, JWT entre panel y nodos, servidor SFTP, protecciones HTTP |
| [Aislamiento del kernel](./general/20-security-unshare.md) | `openat2` y `unshare` |

## Referencia

| Página | Contenido |
|--------|-----------|
| [Referencia de la API](./reference/11-api-reference.md) | Índice de la API HTTP, dividida por áreas: [convenciones](./reference/api/01-conventions.md), [autenticación](./reference/api/02-authentication.md), [servidores](./reference/api/03-servers.md), [contenido del servidor](./reference/api/04-server-content.md), [cuentas](./reference/api/05-accounts.md), [administración del panel](./reference/api/06-panel-admin.md), [aprovisionamiento](./reference/api/07-provisioning.md), [transferencia externa](./reference/api/08-external-transfer.md), [daemon](./reference/api/09-daemon.md) |
| [Comandos CLI](./reference/14-cli-commands.md) | Comandos y opciones |
| [Extensiones](./reference/15-extensions.md) | Paquetes de extensión: manifiesto, páginas, eventos, instalación |
| [Condiciones (CEL)](./general/18-conditions-cel.md) | Expresiones de condición que usan las plantillas y las operaciones |
| [Archivos del servidor y archivos comprimidos](./general/19-files-compression.md) | Servidor de archivos, reglas de compresión y extracción |

## Funcionamiento interno del backend

| Página | Contenido |
|--------|-----------|
| [01. Visión general](./backend/01-overview.md) | Estructura del código, modos panel y daemon |
| [02. Configuración](./backend/02-configuration.md) | `config.json`, variables de entorno y configuración del panel |
| [03. Base de datos](./backend/03-database.md) | Modelos, tablas y migraciones |
| [04. Servidores](./backend/04-servers.md) | Entornos, ciclo de vida, tareas programadas, límite de disco, reconexión, subdominios |
| [05. Capa API](./backend/05-api-layer.md) | Rutas, middleware, autenticación, OAuth2 |
| [06. Motor de operaciones](./backend/06-operations.md) | Operaciones de instalación y de tareas, descargas, condiciones |
| [07. CLI](./backend/07-cli.md) | Cómo está construida la línea de comandos |
| [08. WebSocket](./backend/08-websocket.md) | Consola, estadísticas y estado en tiempo real |
| [09. Seguridad](./backend/09-security.md) | Permisos, tokens, SFTP, protecciones HTTP |
| [10. Paquetes restantes](./backend/10-remaining-packages.md) | Otros paquetes de `internal/` y `pkg/` |
| [11. Servicios](./backend/11-services.md) | Servicios de negocio de `internal/services` |
| [12. Entrada de consola](./backend/12-rcon-telnet.md) | stdin, RCON, Telnet y RCON sobre WebSocket |
| [13. Procesos y métricas](./backend/13-process-metrics.md) | Gestión de procesos, historial de métricas, muestreo del nodo |
| [14. Kit de herramientas del agente de IA](./backend/14-ai-toolkit.md) | Las herramientas de archivos del asistente: bucle del agente, function calling de Gemini, aislamiento, confirmaciones, auditoría |

## Funcionamiento interno del frontend

| Página | Contenido |
|--------|-----------|
| [01. Arquitectura](./frontend/01-architecture.md) | Estructura de `client/frontend` (Astro y React) |
| [02. Consumo de la API](./frontend/02-endpoints.md) | Cliente de la API, errores, cómo llama la interfaz al backend |
| [03. Componentes](./frontend/03-components.md) | Componentes de la interfaz y estructura de la pantalla |
| [04. Funciones de IA](./frontend/04-ai.md) | Funciones de IA en la interfaz (visión general) |
| [05. Traducciones](./frontend/05-translations.md) | Internacionalización; el inglés es el idioma por defecto |
| [06. Páginas](./frontend/06-pages.md) | Rutas y páginas |
| [07. Referencia detallada](./frontend/07-components-detailed.md) | Componentes, hooks y contextos en detalle |
| [08. IA en detalle](./frontend/08-ai-detailed.md) | Funciones de IA en detalle |

## Legal

- [Términos y condiciones](./general/terms.md)
- [Política de privacidad](./general/privacy.md)
