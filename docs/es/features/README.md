# Funcionalidades

Guías para usuarios y administradores sobre las funcionalidades de Aether Panel. Cada página explica qué hace la funcionalidad, cómo funciona, cómo se usa desde el panel y desde la API, las claves de configuración de las que depende, sus límites y los errores conocidos con su solución.

Para el detalle completo de los endpoints, consulte la [referencia de la API](../reference/11-api-reference.md); para cada clave de configuración y su variable de entorno, [Configuración](../backend/02-configuration.md); para los permisos, la [referencia de scopes](../general/scopes-reference.md).

## Páginas del servidor

| Página | Contenido |
|--------|-----------|
| [Resumen y estadísticas del servidor](server-statistics.md) | Pestaña de resumen: tarjetas y gráficas en vivo, historial de rendimiento, disponibilidad, jugadores y horas punta |
| [Configuración del servidor](server-settings.md) | Pestaña de configuración: nombre y descripción, límites de recursos, puertos, arranque automático y reinicio tras caída |
| [Editor de propiedades del servidor](server-properties.md) | Editor experimental de `server.properties` para servidores Minecraft Java |
| [Versión del juego](game-versions.md) | Cambio experimental de versión del juego con un clic (plantillas Paper y vanilla) |
| [Modpacks y plugins](modpacks-and-plugins.md) | Instalación de modpacks desde Modrinth y CurseForge; explorador de plugins de Modrinth |
| [Mapa del mundo](world-map.md) | Instalación de BlueMap y Dynmap y visor del mapa integrado en el panel |
| [Tareas programadas](scheduled-tasks.md) | Tareas cron y manuales: pasos, permisos y límites |
| [Copias de seguridad](backups.md) | Copias manuales y programadas, restauración, puntos de restauración, límite por servidor, copias externas en S3 (periódicas y por tarea) |
| [Historial de archivos](file-history.md) | Versiones guardadas de los archivos editados en el panel, comparación y restauración |
| [Detección de caídas e informes de caída](crash-reports.md) | Cómo se detectan las caídas, reinicio automático e informes de caída |
| [Notificaciones del servidor](server-notifications.md) | Avisos por servidor a Discord, Telegram o un webhook firmado |
| [Subdominios automáticos (Cloudflare)](subdomains.md) | Subdominios de servidor y registros SRV de Minecraft gestionados mediante Cloudflare |
| [Instalar desde GitHub](github.md) | Conectar la cuenta de GitHub e instalar un repositorio (también privado) y su rama en un servidor |
| [Asistente de IA y análisis de registros](ai-assistant.md) | Chat del asistente, análisis de la consola y «Explicar con IA» (Gemini) |
| [Asistente de IA: trabajar con archivos](ai-agent.md) | Lo que el asistente puede hacer con los archivos de un servidor, aprobaciones, dónde quedan registrados los cambios, privacidad, límites |

## Ciclo de vida del servidor

| Página | Contenido |
|--------|-----------|
| [Clonar servidores](cloning.md) | Clonación experimental de servidores: qué se copia, puertos y permisos |
| [Subservidores (Splitter)](sub-servers.md) | División de los recursos de un servidor en subservidores |
| [Transferencias de servidores](server-transfers.md) | Traslado de un servidor a otro nodo y migración a otro panel |

## Administración del panel

| Página | Contenido |
|--------|-----------|
| [Panel de inicio y nodos](dashboard-and-nodes.md) | Cifras del panel de inicio, páginas de nodos, historial de red, avisos de disco, comprobación de puertos y precarga de imágenes |
| [Registro de actividad y registros de acceso](activity-and-access-log.md) | Registro de actividad por servidor y registros de inicio de sesión del panel |
| [Hosts de bases de datos y bases de datos de servidor](database-hosts.md) | Hosts MySQL/MariaDB y bases de datos por servidor |
| [Configuración del panel](panel-settings.md) | La página de configuración del panel, sus pestañas y los interruptores de funciones experimentales |
| [Plantillas](templates.md) | Plantillas locales y de la comunidad, reglas de subida y repositorios de plantillas |
| [Actualizaciones del panel](panel-updates.md) | Comprobación de actualizaciones y autoactualización del panel y sus nodos |
| [Integración con facturación: API de aprovisionamiento y enlaces de acceso](provisioning-and-sso.md) | Claves de API, productos, rutas de aprovisionamiento `/api/v1` y enlaces de acceso de un solo uso |

## Guías relacionadas en otras secciones

- [Seguimiento de disponibilidad](../general/13-uptime.md)
- Integraciones del panel con [correo electrónico](../general/15-email.md) y [Discord](../general/16-discord.md)
- [Captcha / anti-bots (comprobación propia y Turnstile)](../general/17-turnstile.md)
- [Compresión de archivos](../general/19-files-compression.md)
- [Extensiones](../reference/15-extensions.md)
