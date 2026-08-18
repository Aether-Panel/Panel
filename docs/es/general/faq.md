# Preguntas Frecuentes

¿Qué es Aether Panel?

Aether Panel es un panel open source de gestión de servidores de videojuegos. Está construido con Go 1.25 (Gin + GORM) para el backend y un frontend moderno con Astro + React 19. Se ejecuta en tu propio servidor (self-hosted) y te permite gestionar servidores de Minecraft, Terraria, Valheim y más desde una interfaz web.

¿Es Aether Panel gratuito?

Sí, el panel principal es 100% gratuito y open source bajo licencia Apache 2.0. Todas las funciones del panel son gratuitas sin excepciones. Pueden existir módulos de integración con software de código cerrado de terceros (como WHMCS) que tengan un costo para cubrir licencias y mantenimiento, pero el panel en sí mismo es y será siempre gratuito.

¿En qué se diferencia Aether Panel de otros paneles?

Aether Panel es un panel independiente. Mantenemos compatibilidad con el núcleo original pero añadimos mejoras propias, un frontend renovado con Astro + React 19, integración con la comunidad (sugerencias, votaciones), un sitio web de marketing y documentación, y características adicionales como transferencia externa entre paneles.

¿Qué tipo de servidores puedo ejecutar?

Aether Panel soporta cualquier servidor que pueda ejecutarse en Docker. Viene con plantillas para Minecraft (Vanilla, Spigot, Paper, Forge, Fabric), Terraria, Valheim, bots de Discord, y muchas más. Puedes crear plantillas personalizadas para cualquier aplicación.

¿Necesito Docker para usar Aether Panel?

Sí, el daemon de Aether Panel utiliza Docker para aislar y ejecutar los servidores. Cada servidor se ejecuta en su propio contenedor Docker, lo que proporciona aislamiento, seguridad y facilidad de gestión. La instalación incluye Docker automáticamente.

¿Cuál es la arquitectura del panel?

El panel tiene dos componentes principales: el panel (interfaz web y API REST en el puerto 8080) y el daemon (ejecución de servidores Docker, SFTP en puerto 5657). En instalaciones estándar ambos se ejecutan juntos. El frontend del panel está construido con Astro + React.

¿Puedo gestionar múltiples servidores físicos?

Sí, Aether Panel soporta múltiples nodos. Puedes añadir servidores físicos o virtuales como nodos y distribuir tus servidores de juegos entre ellos. Cada nodo ejecuta un daemon que se comunica con el panel central mediante API HTTP con un secreto compartido.

¿Cómo funciona la autenticación?

El panel soporta autenticación mediante tokens Bearer (header Authorization) o cookies (skypanel_auth). Las sesiones usan tokens UUID v4 hasheados con SHA256. También incluye un servidor OAuth2 completo con JWTs firmados con Ed25519, y soporte para 2FA/TOTP con códigos de recuperación.

¿Cómo funciona el sistema de permisos?

El panel tiene aproximadamente 74 scopes granulares (50 específicos de servidor + 24 globales) que controlan cada acción. Los permisos pueden asignarse a nivel global (afectan todos los servidores) o por servidor. También existen roles que agrupan scopes. El scope 'admin' concede todos los permisos, y 'server.admin' concede todos los permisos sobre un servidor específico.

¿Qué base de datos usa el panel?

Por defecto usa SQLite, que se configura automáticamente. También soporta PostgreSQL y MySQL/MariaDB. La configuración se realiza en el archivo config.json. El panel usa GORM como ORM.

¿Cómo creo bases de datos para mis servidores?

Primero crea un Database Host desde Admin  Database Hosts con credenciales MySQL que tengan permisos GRANT ALL. Luego, desde la pestaña Database de cualquier servidor, puedes crear bases de datos que se generarán automáticamente con usuario, contraseña y conexión remota.

¿Cómo accedo a los archivos de mi servidor?

Tienes dos opciones: el editor de archivos integrado en la pestaña Files del servidor (con resaltado de sintaxis, compresión y extracción), o SFTP usando las credenciales que se muestran en la pestaña SFTP (puerto 5657).

¿Cómo funcionan los backups?

Desde la pestaña Backups puedes crear backups manuales con nombre personalizado. Los backups se almacenan comprimidos en el nodo y puedes descargarlos, restaurarlos o eliminarlos. Cada backup tiene un nombre único para identificarlo fácilmente.

¿Cómo funciona la consola en tiempo real?

La consola usa WebSocket para transmitir logs en vivo del servidor. Puedes enviar comandos y ver la salida en tiempo real con auto-scroll. La conexión va del panel al daemon del nodo mediante proxy. También incluye timestamps y modo de pantalla completa.

¿Qué estadísticas puedo ver del servidor?

La pestaña Stats muestra gráficas en tiempo real de CPU, RAM, uso de disco y tráfico de red. Los datos se actualizan mediante WebSocket. También puedes ver el estado del servidor (running/stopped/installing) y consultar información del servidor (versión, jugadores, etc.) si el juego lo soporta.

¿Cómo funcionan los plugins?

La pestaña Plugins permite buscar e instalar plugins para servidores Minecraft compatibles (Spigot/Paper). Puedes buscar por nombre en repositorios públicos, ver detalles e instalarlos directamente desde el panel.

¿Qué es la transferencia externa?

La transferencia externa (extransfer) permite mover servidores entre instalaciones independientes de Aether Panel. Es útil para migrar servidores entre diferentes paneles o proveedores. También existe la transferencia interna entre nodos del mismo panel.

¿Qué es Gatus?

El repositorio incluye un archivo de configuración `data/gatus/config.yaml` para monitorear el uptime del panel y sus nodos con Gatus (dashboard web en el puerto 8081). Sin embargo, **Gatus no se despliega automáticamente** con el `docker-compose.yml`: si quieres usarlo, debes ejecutar Gatus por separado usando ese archivo de configuración.

¿Aether Panel tiene API?

Sí, el panel expone una API RESTful completa en /api/*. Todos los endpoints requieren autenticación mediante tokens Bearer. La API incluye endpoints para servidores, nodos, usuarios, archivos, backups, bases de datos, roles, plantillas, uptime, configuración y más. También hay un WebSocket para datos en tiempo real.

¿Soporta OAuth2?

Sí, el panel incluye un servidor OAuth2 con el endpoint /oauth2/token. Usa los flujos Client Credentials (autenticación panel↔nodo) y Password (autenticación SFTP), con JWTs firmados con Ed25519. Las claves públicas están disponibles en /auth/publickey en formato JWKS para verificación por terceros.

¿Qué línea de comandos (CLI) tiene el panel?

El panel incluye un CLI completo basado en Cobra. Los comandos principales son: SkyPanel run (iniciar panel y daemon), SkyPanel version (versión), SkyPanel user (gestión de usuarios), SkyPanel db (migraciones de base de datos), y SkyPanel runservice (ejecutar como servicio del sistema).

¿Cómo actualizo Aether Panel?

Si usas Docker: `docker compose build && docker compose up -d` (o `git pull` y reconstruir la imagen). Consulta la documentación de instalación para instrucciones detalladas.

¿El panel funciona con MySQL externo?

Sí, puedes configurar el panel para usar MySQL o PostgreSQL externos editando la configuración de base de datos en config.json. Por defecto usa SQLite que se configura automáticamente.

¿Cómo contribuyo al proyecto?

Puedes contribuir de varias formas: reportando errores en GitHub, enviando pull requests con mejoras, participando en las votaciones de la comunidad para priorizar características, uniéndote al Discord para dar feedback, o haciendo una donación voluntaria via PayPal.

¿Dónde puedo obtener ayuda?

Tenemos varios canales: el servidor de Discord de la comunidad para soporte y discusión, los issues de GitHub para reportes de errores y solicitudes de características, y la documentación en esta misma web. Como proyecto open source, el soporte es comunitario.


