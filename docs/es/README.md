# Documentación Técnica de Aether Panel

Bienvenido a la documentación oficial técnica de Aether Panel. Aquí encontrará toda la información profunda sobre la arquitectura, backend, frontend y configuración del panel.

## Índice de Contenidos

### Instalación y Configuración
*   [01. Guía de Instalación](./setup/01-installation.md): Instrucciones paso a paso para desplegar el panel.
*   [Troubleshooting](./setup/troubleshooting.md): Guía de solución de problemas y errores comunes (incluye MySQL Docker, Monaco Editor build).
*   [Referencia Rápida de Docker](./docker/DOCKER-QUICKREF.md): Comandos esenciales para entornos Docker.
*   [Solución de Problemas Sudo (Docker)](./docker/DOCKER-SUDO-FIX.md): Guía para resolver permisos en Docker.

### General (Conceptos y Guías)
*   [Primeros Pasos](./general/getting-started.md): Guía completa desde instalación hasta primer servidor.
*   [Conceptos Clave](./general/concepts.md): Arquitectura, nodos, servidores, auth, scopes, SFTP, templates, database hosts, transferencia federada, **gestión de puertos, red Docker auto-detección, roles actualizados, Monaco Editor pin**.
*   [FAQ](./general/faq.md): Preguntas frecuentes (novedades v2.0.0+, roles, conectividad MySQL, Monaco).
*   [Uptime Tracking](./general/13-uptime.md): Sistema de seguimiento de tiempo activo/inactivo.
*   [License System](./general/14-license.md): Sistema de licencias (API externa).
*   [Email/SMTP](./general/15-email.md): Proveedores de email y sistema de plantillas.
*   [Discord Webhooks](./general/16-discord.md): Tipos de webhook y estructuras de embeds.
*   [Turnstile/Captcha](./general/17-turnstile.md): Cloudflare Turnstile.
*   [Conditions/CEL Engine](./general/18-conditions-cel.md): Motor CEL + funciones personalizadas.
*   [Files/Compression](./general/19-files-compression.md): FileServer, MergedFS, archive/extract.
*   [Security/unshare](./general/20-security-unshare.md): openat2, unshare, soporte kernel.
*   [Términos y Condiciones](./general/terms.md): Términos legales.
*   [Política de Privacidad](./general/privacy.md): Privacidad de datos.
*   [Seguridad](./general/security.md): Consideraciones de seguridad.
*   [Usos](./general/usages.md): Casos de uso.

### Backend (Arquitectura y APIs)
*   [01. Visión General](./backend/01-overview.md): Estructura del código Go, modos Panel y Daemon.
*   [02. Configuración](./backend/02-configuration.md): Archivo `config.json` y sistema Viper.
*   [03. Base de Datos](./backend/03-database.md): Modelos GORM y migraciones.
*   [04. Gestión de Servidores](./backend/04-servers.md): Ciclo de vida, scheduler, **ExtraPortBindings, red Docker auto-detección**.
*   [05. Capa API y HTTP](./backend/05-api-layer.md): Rutas REST, CORS, OAuth2, **endpoints de puertos**.
*   [06. Operaciones y Condiciones](./backend/06-operations.md): Motor de operaciones tipo JSON para tareas (descargas, unzip, chown).
*   [07. Herramienta CLI](./backend/07-cli.md): Uso del binario de SkyPanel desde la terminal.
*   [08. WebSocket y Consola](./backend/08-websocket.md): Transmisión en tiempo real de logs y estadísticas.
*   [09. Seguridad y SFTP](./backend/09-security.md): Scopes, JWKS, y servidor SFTP integrado.
*   [10. Paquetes Restantes](./backend/10-remaining-packages.md): Email, logs, utilidades y sistema de plantillas.
*   [11. Servicios Internos](./backend/11-services.md): 19 servicios de negocio (Email, Discord, Token, License, etc.).
*   [12. RCON/Telnet/RCON-WS](./backend/12-rcon-telnet.md): Conexiones stdin, proxy automático.
*   [13. Process/Metrics](./backend/13-process-metrics.md): Gestión procesos, métricas, JVM stats.

### Frontend (Arquitectura de Interfaz)
*   [01. Arquitectura del Cliente](./frontend/01-architecture.md): Estructura general de Astro, React y Tailwind.
*   [02. Endpoints y API Client](./frontend/02-endpoints.md): SDK nativo de API e integración de React.
*   [03. Componentes y UI](./frontend/03-components.md): Shadcn UI, **Settings View rediseñada, roles corregidos**.
*   [04. Inteligencia Artificial (Genkit)](./frontend/04-ai.md): Integración de Google GenAI para análisis de logs.
*   [05. Sistema de Traducciones (i18n)](./frontend/05-translations.md): Internacionalización usando diccionarios JSON.
*   [06. Páginas y Vistas](./frontend/06-pages.md): Vistas principales, **Settings View, Sidebar fix**.
*   [07. Componentes y Hooks Detallados](./frontend/07-components-detailed.md): Todos los componentes, hooks, contexts, API client.
*   [08. AI Detallado](./frontend/08-ai-detailed.md): Genkit flows, prompts, UI.

### Referencias
*   [Referencia de API](./reference/11-api-reference.md): Esquema general para uso programático.
*   [Referencia de Comandos CLI](./reference/14-cli-commands.md): Lista detallada de subcomandos y flags de la consola.
