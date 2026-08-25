# Conceptos Clave

Aether Panel es una plataforma de gestión de servidores con una arquitectura de dos componentes: el Panel (API REST + frontend web) y el Daemon (agente que ejecuta los servidores). Esta página explica los conceptos fundamentales para entender cómo funciona el sistema.

## Arquitectura: Panel y Daemon

El sistema se compone de dos procesos que pueden ejecutarse juntos o por separado:

- **panel**: Panel — Servidor API REST construido con Gin y GORM. Gestiona usuarios, nodos, servidores, plantillas y permisos. Sirve el frontend web (Astro + React) y expone la API pública en el puerto 8080.
- **daemon**: Daemon — Agente que se ejecuta en cada máquina donde corren los servidores de aplicaciones. Gestiona el ciclo de vida de los procesos (iniciar, detener, reiniciar), el sistema de archivos SFTP, la consola WebSocket y la ejecución en contenedores Docker. Corre en el puerto 8080 y SFTP en el puerto 5657.
- **comms**: Comunicación — Si el nodo es local, el Panel llama al Daemon directamente a través del router de Gin sin pasar por la red (httptest.ResponseRecorder). Si el nodo es remoto, el Panel se comunica mediante HTTP autenticado con JWT Ed25519.
## Nodos

Un nodo representa una máquina física o virtual donde se ejecutan servidores de aplicaciones.

- **local**: Nodo Local — Es la propia máquina donde corre el Panel. Se identifica con ID 0 y usa las IPs configuradas en la variable MasterUrl. Su secreto se genera automáticamente como UUID.
- **remote**: Nodo Remoto — Cualquier otra máquina registrada en el Panel. Al registrarse recibe un client_id (formato .node_{ID}) y un client_secret que usa para autenticarse vía OAuth2.
- **deploy**: Despliegue — El endpoint GET /api/nodes/:id/deployment devuelve las credenciales necesarias para configurar un nodo remoto.
- **features**: Características — Cada nodo reporta sus capacidades (Docker, sistemas de archivos, SO, arquitectura) mediante GET /api/nodes/:id/features.
## Servidores

Un servidor es una instancia de una aplicación (Minecraft, Discord bot, web, etc.) ejecutándose en un nodo.

- **definition**: Definición — Cada servidor tiene una definición JSON (server.json) que incluye variables de configuración, comandos de ejecución, archivos de instalación/desinstalación, variables de entorno, requisitos y grupos de archivos.
- **lifecycle**: Ciclo de Vida — Crear  Instalar (descargar, extraer assets)  Iniciar (ejecución de pre-commands + comando principal)  Detener / Matar  Desinstalar  Destruir. Cada etapa ejecuta operaciones definidas en el template.
- **environments**: Entornos de Ejecución — TTY (proceso directo en el host usando PTY) para servers standard/host/tty, y Docker (contenedor aislado) para servers docker. El entorno se selecciona según el tipo de servidor.
- **scheduler**: Planificador — Cada servidor tiene un scheduler basado en gocron que ejecuta tareas programadas (cron jobs) como backups automáticos, reinicios, etc.
- **stats**: Estadísticas — El Daemon recolecta métricas en tiempo real (CPU, RAM, disco, red) que se envían a los clientes WebSocket.
- **backups**: Backups — El servidor puede crear, restaurar y eliminar backups comprimidos de su directorio de archivos.
## Usuarios y Autenticación

El sistema de autenticación soporta múltiples mecanismos para diferentes casos de uso:

- **sessions**: Sesiones — Al iniciar sesión (POST /auth/login con email + contraseña), se genera un token UUID que se almacena como hash SHA-256 en la base de datos con expiración de 1 hora. El token se devuelve como cookie skypanel_auth y en el cuerpo JSON.
- **bearer**: Bearer Token — El middleware AuthMiddleware busca primero el header Authorization: Bearer <token>, y si no está presente, fallback a la cookie skypanel_auth.
- **jwt**: JWT Ed25519 — El Daemon usa tokens JWT firmados con Ed25519 para autenticar peticiones del Panel. La clave pública se expone en GET /auth/publickey en formato JWKS.
- **oauth2**: OAuth2 — Endpoint /oauth2/token que soporta client_credentials (para autenticación nodopanel) y password (para autenticación SFTP). Los tokens tienen validez de 1 hora.
- **twofactor**: 2FA — El panel soporta autenticación de dos factores (OTP) como capa adicional de seguridad en el inicio de sesión.
## Permisos y Scopes

El sistema de permisos se basa en scopes granulares (74 en total: 50 específicos de servidor + 24 globales) que controlan cada acción posible en el panel.

- **scopes**: Scopes — Cada permiso es un string como server.start, nodes.view, users.edit, etc. Los scopes pueden ser globales o específicos de un servidor (ForServer: true).
- **admin**: Jerarquía — El scope admin concede todos los permisos. A nivel de servidor, server.admin concede todos los scopes de ese servidor.
- **roles**: Roles — Los roles agrupan múltiples scopes bajo un nombre (ej. "Admin", "Moderador") y se asignan a usuarios para simplificar la gestión de permisos.
- **checking**: Verificación — El middleware RequiresPermission y RequiresAnyPermission cargan los permisos del usuario (globales + específicos del servidor + rol) y verifican si contienen el scope requerido.
- **serverscopes**: Ejemplos de scopes de servidor — server.start, server.stop, server.kill, server.console, server.files.view, server.files.edit, server.sftp, server.backup.create, server.stats, server.status, server.users.view, server.users.edit.
## SFTP

El Daemon incluye un servidor SFTP integrado para acceso a archivos de servidores.

- **port**: Puerto — Corre en el puerto 5657 por defecto (configurable en daemon.sftp.host), ejecutándose como un servidor SSH independiente usando el paquete github.com/pkg/sftp y clave de host Ed25519.
- **auth**: Autenticación — El formato de usuario es email#serverId. Cuando el Panel está habilitado, valida contra la base de datos (DatabaseSFTPAuthorization). Cuando el Daemon está standalone, llama al endpoint /oauth2/token con grant_type=password (WebSSHAuthorization).
- **isolation**: Aislamiento — Cada conexión SFTP está aislada al directorio del servidor específico. No se puede acceder a archivos de otros servidores ni del sistema.
## Consola WebSocket

El Daemon proporciona una consola en tiempo real mediante WebSocket en GET /daemon/server/:serverId/socket.

- **streams**: Tres canales de streaming — console (salida en vivo del proceso), stat (CPU, RAM, disco, red en intervalos regulares), status (estado del servidor: online/offline/installing).
- **tracker**: Tracker — Cada Environment tiene tres Trackers (ConsoleTracker, StatsTracker, StatusTracker) que registran Sockets y transmiten mensajes JSON en formato {message, type}.
- **proxy**: Proxy — El Panel hace proxy de las conexiones WebSocket: si el nodo es local, reescribe la URL internamente; si es remoto, crea un proxy bidireccional.
## Templates

Los templates son plantillas JSON que definen cómo desplegar y ejecutar un servidor.

- **structure**: Estructura — Un template contiene variables de configuración, comandos de instalación/desinstalación, configuración de ejecución, variables de entorno, requisitos del sistema, tipo de entorno (standard, docker, tty, host) y grupos de archivos.
- **storage**: Almacenamiento — Los templates se guardan en la base de datos. El repositorio 0 es local. Se pueden agregar repositorios remotos (URLs git) que se sincronizan mediante SyncRepo() clonando el repo y parseando los JSON.
- **usage**: Uso — Al crear un servidor, se selecciona un template como blueprint y se pueden sobrescribir variables específicas.
## Database Hosts

Los Database Hosts permiten gestionar bases de datos MySQL externas para los servidores.

- **model**: Modelo — Cada Database Host tiene nombre, host, puerto (por defecto 3306), usuario y contraseña, límite máximo de bases de datos y nodo asociado (opcional).
- **api**: API CRUD completa en /api/databasehosts para crear, leer, actualizar y eliminar hosts de bases de datos.
## External Transfer (Transferencia Federada)

Permite migrar servidores entre instancias independientes de Aether Panel (cross-panel).

- **protocol**: Protocolo — Usa Ed25519 para firmar peticiones, HMAC-SHA256 para hashear tokens con la salt AETHER_FEDERATED_SALT_v1, nonces para challenge/response, y las sesiones expiran a los 15 minutos.
- **states**: Estados de una transferencia — CREADA  VALIDADA  MIGRANDO  CONSUMADA / COMPLETADA / FALLIDA / CANCELADA.
- **endpoints**: Endpoints — /api/servers/:id/extransfer/create (origen), /api/extransfer/validate (destino), /api/extransfer/consume, /api/extransfer/heartbeat, /api/extransfer/confirm, /api/extransfer/cancel, /api/servers/:id/extransfer/pull.
## CLI (Interfaz de Línea de Comandos)

El binario de Aether Panel incluye una CLI basada en Cobra con los siguientes comandos:

- **run**: run — Inicia el Panel y/o Daemon según la configuración. Comando oculto que no aparece en la ayuda.
- **version**: version — Muestra la versión del panel.
- **user**: user add / user edit — Gestiona usuarios desde la terminal. Permite crear usuarios con nombre, email, contraseña y opción de admin, y editar usuarios existentes (cambiar email, contraseña, admin, remover 2FA).
- **db**: db upgrade / db migrate — Gestiona la base de datos (`upgrade` ejecuta migraciones de esquema; `migrate` es un stub experimental).
- **runservice**: runService — Igual que run pero con soporte para systemd notify (NOTIFY_SOCKET).

## Gestión de Puertos (Port Management)

Aether Panel permite asignar múltiples puertos a un servidor, no solo el puerto principal. Esta funcionalidad es crítica para servidores que requieren puertos adicionales (ej. Minecraft + plugins que usan puertos separados para query, RCON, voice, etc.).

### Características Principales

- **Puerto Primario**: El primer puerto de la lista es el "puerto principal" usado por el panel para conexiones por defecto.
- **Puertos Extra**: Se pueden asignar puertos adicionales (port2, port3, etc.) que se bindean automáticamente al contenedor Docker.
- **Notas de Puerto**: Cada puerto puede tener una etiqueta/descripción personalizada para identificar su propósito (ej. "RCON", "Voice Chat", "Query").
- **Selección de Primario**: El usuario puede elegir qué puerto es el principal reordenando la lista.

### Flujo Técnico

1. El panel guarda la lista ordenada en `server.Ports` (array) y el primario en `server.Port`.
2. Al guardar (`PUT /api/servers/:id/data`), el panel envía la lista al Daemon que la convierte en variables `port`, `port2`, `port3`...
3. El Daemon crea el contenedor con `ExtraPortBindings()` que bindea todos los puertos extra (TCP y UDP).
4. `PUT /api/servers/:id/port-settings` gestiona solo metadatos (notas, primario) sin tocar la lista completa.
5. La sincronización con el Daemon es automática: cambios en `port-settings` también propagan la lista de puertos.

### Permisos

| Acción | Scope Requerido |
|--------|-----------------|
| Ver puertos asignados | `server.data.view` |
| Editar lista completa (admin) | `server.data.edit.admin` |
| Cambiar primario / notas (usuario) | `server.data.view` (via `port-settings`) |

> **Importante**: Los puertos extra solo funcionan en entorno Docker. En entorno TTY/Host, el binding de puertos es responsabilidad del proceso del servidor.

## Red Docker: Auto-Detección `skypanel-network`

Para que los servidores en contenedores Docker puedan conectarse a bases de datos MySQL y otros servicios en la red interna, Aether Panel detecta automáticamente la red Docker del contenedor Panel y conecta todos los servidores a esa misma red.

### Problema Resuelto

Antes, los contenedores de servidores se creaban en la red `bridge` por defecto de Docker, donde no podían resolver hostnames como `mysql` (el servicio MySQL corre en `skypanel-network`). El error típico era:
```
Communications link failure... database address 'mysql:3306' accessible
```

### Solución Implementada

1. **Detección automática**: Al iniciar, el Panel inspecciona su propio contenedor vía Docker API y detecta a qué red pertenece (ej. `panel_skypanel-network`).
2. **Default automático**: Si un template no especifica `networkName`, se usa la red detectada.
3. **Resultado**: Todos los servidores se crean en `skypanel-network` y pueden resolver `mysql:3306` directamente.

### Configuración

No requiere configuración adicional. Funciona automáticamente con `docker compose up`. Si se usa instalación nativa (sin Docker), la red no aplica y se usa la IP del host.

> **Nota**: Servidores creados antes de este cambio deben recrearse para unirse a la red correcta.

## Roles y Permisos Actualizados (Migración `20260821-fix-usuario-role-scopes`)

Se corrigió la asignación de scopes al rol "Usuario" para eliminar accesos de admin indebidos y otorgar permisos correctos de definición y flags.

### Cambios

| Acción | Antes | Después |
|--------|-------|---------|
| `server.admin.config.view` | ✅ Concedido | ❌ Revocado |
| `server.admin.config.manage` | ✅ Concedido | ❌ Revocado |
| `server.data.edit.admin` | ✅ Concedido | ❌ Revocado |
| `server.definition.view` | ❌ No tenía | ✅ Concedido |
| `server.definition.edit` | ❌ No tenía | ✅ Concedido |
| `server.flags.view` | ❌ No tenía | ✅ Concedido |
| `server.flags.edit` | ❌ No tenía | ✅ Concedido |

### Resultado

- **Admin**: Acceso completo a todo (config, límites, metadata, admin tab, CRUD puertos).
- **Usuario**: Ver/editar Grupos, Variables, Plugins, Auto-start, Puertos (ver + primario + notas, sin CRUD números), **NO** ve Límites de Recursos, Metadatos, Admin tab.

## Monaco Editor — Pin a v0.44.0

La versión 0.56+ de `monaco-editor` introdujo restricciones en `package.json` `exports` que impiden importar CSS vía `?inline` con Vite 8/Rolldown. Se mantiene la versión `^0.44.0` hasta que haya una solución upstream.

```json
"monaco-editor": "^0.44.0"
```

El CSS se inyecta inline y el font codicon se sirve desde CDN jsDelivr apuntando a la versión 0.44.0.
