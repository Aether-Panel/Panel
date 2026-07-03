# Endpoints del Frontend SDK (`client/api`)

El SDK de Aether Panel (`client/api/src`) expone varias clases que actúan como "wrappers" (envoltorios) para hacer peticiones HTTP hacia el backend. A continuación se detallan los módulos principales y sus responsabilidades.

## 1. Módulo de Autenticación (`AuthApi`)
Maneja el inicio de sesión, registro y la gestión de sesiones mediante JWT/OAuth2.

*   `login(email, password)`: `POST /auth/login` - Inicia sesión, soporta OTP (One Time Password).
*   `loginOtp(token)`: `POST /auth/otp` - Validación de doble factor.
*   `register(username, email, password)`: `POST /auth/register` - Crea una nueva cuenta.
*   `oauth(clientId, clientSecret)`: `POST /oauth2/token` - Autenticación máquina a máquina o aplicaciones de terceros.
*   `reauth()`: `POST /auth/reauth` - Refresca la sesión actual.
*   `logout()`: `POST /auth/logout` - Cierra la sesión activa.

## 2. Módulo de Servidores (`ServerApi`)
El módulo más extenso, encargado de toda la gestión del ciclo de vida de los servidores de juego.

### Ciclo de vida y Estado
*   `create(data)`: `PUT /api/servers/:id` - Crea un nuevo servidor generando un ID único de forma local.
*   `start(id)`, `stop(id)`, `restart(id)`, `kill(id)`: Llaman a `POST /api/servers/:id/:action` para controlar la energía del servidor.
*   `getStatus(id)`: `GET /api/servers/:id/status` - Retorna si el servidor está `online`, `offline` o `installing`.
*   `install(id)`: `POST /api/servers/:id/install` - Lanza la instalación/reinstalación del servidor.

### Interacción y Consola
*   `sendCommand(id, command)`: `POST /api/servers/:id/console` - Envía comandos al proceso activo.
*   `getConsole(id)`: `GET /api/servers/:id/console` - Obtiene el historial reciente de la consola.
*   `getStats(id)`: `GET /api/servers/:id/stats` - Obtiene métricas de uso de CPU, RAM, Disco, etc.

### Gestión de Archivos (SFTP y Web)
*   `getFile(id, path)`: `GET /api/servers/:id/file/:path` - Descarga un archivo.
*   `uploadFile(id, path, content)`: `PUT /api/servers/:id/file/:path` - Sube un archivo nuevo usando FormData.
*   `createFolder(id, path)`: `PUT /api/servers/:id/file/:path` (con flag de carpeta).
*   `archiveFile(id, destination, files)`: `POST /api/servers/:id/archive/:destination` - Comprime archivos/carpetas.
*   `extractFile(id, path, destination)`: `POST /api/servers/:id/extract/:path` - Extrae un archivo comprimido.
*   `deleteFile(id, path)`: `DELETE /api/servers/:id/file/:path`

### Otras funciones de servidor
*   Gestión de backups (`createBackup`, `restoreBackup`).
*   Configuración (`getFlags`, `setFlags`, `getDefinition`).
*   Gestión de sub-usuarios (`getUsers`, `updateUser`, `deleteUser` dentro de un servidor específico).
*   Instalación de Plugins (`installPlugin`).

> **Nota importante sobre WebSockets:** La clase `Server` establece una conexión `wss://` o `ws://` hacia `/api/servers/:id/socket`. Esto permite eventos en tiempo real (`on('message')`, `on('open')`) para el flujo de la consola y la sincronización del estado sin necesidad de hacer *polling* continuo.

## 3. Módulo de Usuarios (`UserApi`)
Gestión administrativa de los usuarios del panel.

*   `list(page)`: `GET /api/users` - Paginación de usuarios.
*   `search(name, limit)` / `searchEmail(email)`: Búsqueda avanzada de usuarios.
*   `create(...)`: `POST /api/users` - Crear usuario desde la vista de administrador.
*   `get(id)`: `GET /api/users/:id` - Detalles de un usuario específico.
*   `getPermissions(id)`: `GET /api/users/:id/perms` - Obtiene los permisos (Scopes) del usuario.
*   `update(...)` y `delete(id)`: Modificación y eliminación de la cuenta.

## Otros Módulos
Existen otros módulos similares que mapean la API REST, tales como:
*   `NodesApi` (`nodes.js`): Para gestionar servidores físicos/nodos donde se alojan los servidores virtuales.
*   `DatabasesApi` (`databases.js`): Para crear/borrar bases de datos (MySQL/MariaDB) asignadas a servidores.
*   `TemplatesApi` (`templates.js`): Gestión de plantillas/huevos de los diferentes tipos de juegos que soporta el panel.
