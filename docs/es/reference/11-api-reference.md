# Referencia de API

> **Nota:** El proyecto tiene los nombres Aether Panel (público) y SkyPanel (código, CLI, módulo Go v3).

Documentación OpenAPI/Swagger interactiva disponible en `http://localhost:8080/swagger/index.html` cuando el panel está en ejecución.

---

## Tabla de Contenidos

- [Autenticación](#autenticación)
- [Scopes (Permisos)](#scopes-permisos)
- [Formato de Respuestas](#formato-de-respuestas)
- [Tipos de Datos](#tipos-de-datos)
- [Config](#config)
- [Auth](#auth)
- [OAuth2](#oauth2)
- [Nodos](#nodos)
- [Servidores](#servidores)
- [Archivos](#archivos)
- [Archivos Comprimidos](#archivos-comprimidos)
- [Plugins](#plugins)
- [Backups](#backups)
- [Tareas](#tareas)
- [Flags](#flags)
- [Consola](#consola)
- [Bases de Datos del Servidor](#bases-de-datos-del-servidor)
- [Usuarios del Servidor](#usuarios-del-servidor)
- [Transferencia entre Nodos](#transferencia-entre-nodos)
- [Transferencia Externa](#transferencia-externa)
- [AI](#ai)
- [Usuarios Globales](#usuarios-globales)
- [Self (Perfil Propio)](#self-perfil-propio)
- [Settings](#settings)
- [User Settings](#user-settings)
- [API Keys](#api-keys)
- [Roles](#roles)
- [Database Hosts](#database-hosts)
- [Templates](#templates)
- [Provision Products](#provision-products)
- [Provision API v1](#provision-api-v1)
- [Uptime](#uptime)
- [Daemon](#daemon)
- [WebSocket](#websocket)

---

## Autenticación

### 1. OAuth2 Client Credentials (API externa)

```
POST /oauth2/token
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials&client_id=ID&client_secret=SECRET
```

**Respuesta:**
```json
{
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "server.view server.start"
}
```

Usar el token en todas las peticiones:
```
Authorization: Bearer eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...
```

### 2. OAuth2 Password Grant (SFTP/SSH)

```
POST /oauth2/token
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer <node-token>

grant_type=password&username=email%23serverId&password=USER_PASSWORD&scope=sftp
```

### 3. API Keys (Provision)

```
X-Api-Key: ak_<key>
```
O como Bearer:
```
Authorization: Bearer ak_<key>
```

### 4. Auth de Panel (Sesiones Web)

Usar los endpoints `/auth/login`, `/auth/otp`, etc. para obtener cookies de sesión.

---

## Scopes (Permisos)

### Scopes de Servidor (por servidor)

| Scope | Descripción |
|-------|-------------|
| `server.view` | Ver servidor |
| `server.admin` | Admin del servidor |
| `server.delete` | Eliminar servidor |
| `server.definition.edit` | Editar definición |
| `server.definition.view` | Ver definición |
| `server.data.edit` | Editar datos del servidor |
| `server.data.edit.admin` | Editar datos (admin) |
| `server.data.view` | Ver datos |
| `server.flags.edit` | Editar flags |
| `server.flags.view` | Ver flags |
| `server.name.edit` | Cambiar nombre |
| `server.clients.view` | Ver clients OAuth2 |
| `server.clients.edit` | Editar clients |
| `server.clients.create` | Crear clients |
| `server.clients.delete` | Eliminar clients |
| `server.users.view` | Ver usuarios del servidor |
| `server.users.create` | Agregar usuarios |
| `server.users.edit` | Editar permisos de usuarios |
| `server.users.delete` | Eliminar usuarios del servidor |
| `server.tasks.view` | Ver tareas |
| `server.tasks.run` | Ejecutar tarea |
| `server.tasks.create` | Crear tarea |
| `server.tasks.delete` | Eliminar tarea |
| `server.tasks.edit` | Editar tarea |
| `server.reload` | Recargar servidor |
| `server.start` | Iniciar servidor |
| `server.stop` | Detener servidor |
| `server.kill` | Matar proceso |
| `server.install` | Ejecutar instalación |
| `server.files.view` | Ver archivos |
| `server.files.edit` | Subir/editar/eliminar archivos |
| `server.sftp` | Acceso SFTP |
| `server.console` | Ver consola |
| `server.console.send` | Enviar comandos |
| `server.stats` | Ver estadísticas |
| `server.status` | Ver estado |
| `server.backup.view` | Ver backups |
| `server.backup.create` | Crear backup |
| `server.backup.restore` | Restaurar backup |
| `server.backup.delete` | Eliminar backup |
| `server.admin.view` | Admin: ver |
| `server.admin.install.view` | Admin: ver instalación |
| `server.admin.install.manage` | Admin: gestionar instalación |
| `server.admin.transfer.view` | Admin: ver transferencias |
| `server.admin.transfer.manage` | Admin: gestionar transferencias |
| `server.admin.config.view` | Admin: ver config |
| `server.admin.config.manage` | Admin: gestionar config |
| `server.admin.assignments.view` | Admin: ver asignaciones |
| `server.admin.assignments.manage` | Admin: gestionar asignaciones |

### Scopes Globales

| Scope | Descripción |
|-------|-------------|
| `admin` | Superadmin |
| `login` | Iniciar sesión |
| `panel` | Acceso al panel |
| `oauth2.auth` | Validar credenciales vía OAuth2 |
| `nodes.view` | Ver nodos |
| `nodes.create` | Crear nodos |
| `nodes.edit` | Editar nodos |
| `nodes.delete` | Eliminar nodos |
| `nodes.deploy` | Obtener datos de despliegue |
| `self.edit` | Editar propio perfil |
| `self.clients` | Gestionar clients OAuth2 propios |
| `settings.edit` | Editar configuración global |
| `templates.view` | Ver plantillas |
| `templates.local.edit` | Editar plantillas locales |
| `templates.repo.create` | Agregar repositorios |
| `templates.repo.delete` | Eliminar repositorios |
| `users.info.search` | Buscar usuarios |
| `users.info.view` | Ver usuarios |
| `users.info.edit` | Crear/editar/eliminar usuarios |
| `users.perms.view` | Ver permisos de usuarios |
| `users.perms.edit` | Editar permisos de usuarios |
| `uptime.view` | Ver estadísticas de uptime |
| `server.create` | Crear servidores |

---

## Formato de Respuestas

### Éxito
```json
{ "data": { ... } }
```
O directamente un array u objeto según el endpoint.

### Error
```json
{
  "error": {
    "code": "ErrFieldRequired",
    "msg": "username: required field is missing"
  }
}
```

Códigos de error: `ErrFieldRequired`, `ErrFieldLength`, `ErrServerNotFound`, `ErrUserNotFound`, `ErrNodeInvalid`, `ErrDatabaseNotAvailable`, `ErrNoPermission`, `ErrInvalidCredentials`, `ErrUnknownError`. Lista completa en `pkg/skypanel/errors.go`.

### Códigos HTTP

| Código | Significado |
|--------|-------------|
| 200 | OK |
| 201 | Created |
| 204 | No Content |
| 202 | Accepted (operación asíncrona) |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 500 | Internal Server Error |

---

## Tipos de Datos

### Paging
```json
{ "page": 1, "size": 20, "maxSize": 100, "total": 1 }
```

### Error
```json
{
  "error": { "code": "ErrFieldRequired", "msg": "...", "metadata": {} }
}
```

### Node
```json
{
  "id": 1, "name": "Node-01", "isLocal": true,
  "publicHost": "node1.example.com", "publicPort": 8080,
  "privateHost": "192.168.1.10", "privatePort": 8080,
  "sftpPort": 5657
}
```

### ServerInfo
```json
{
  "id": "abc123", "name": "Minecraft", "node": { "id": 1, "name": "Node" },
  "nodeId": 1, "ip": "192.168.1.100", "port": 25565, "type": "minecraft-java",
  "icon": "minecraft.png", "isGhost": false, "canGetStatus": true,
  "users": [ { "username": "admin", "scopes": ["server.view"] } ]
}
```

### ServerDefinition (Create/Edit)
```json
{
  "name": "Mi Servidor", "type": "minecraft-java", "icon": "minecraft.png",
  "environment": { "type": "standard" },
  "install": [ { "type": "mojangdl", "version": "1.20.1" } ],
  "uninstall": [],
  "run": {
    "command": "java -Xmx{{memory}}M -jar server.jar nogui",
    "stop": "stop", "stopCode": 0,
    "pre": [], "post": [],
    "environmentVars": { "KEY": "VALUE" },
    "workingDirectory": "",
    "autostart": false, "autorestart": false, "autorecover": false,
    "expectedExitCode": 0,
    "stdin": { "type": "", "ip": "", "port": "", "password": "" }
  },
  "data": {
    "memory": { "type": "integer", "value": 2048, "required": true, "desc": "Memoria MB", "display": "Memoria" },
    "port": { "type": "string", "value": "25565", "required": true },
    "version": { "type": "string", "value": "1.20.1", "required": true }
  },
  "groups": [ { "display": "Config", "variables": ["memory", "port"] } ],
  "requirements": { "os": "linux", "arch": "amd64", "binaries": ["java"] },
  "supportedEnvironments": [ { "type": "standard" }, { "type": "docker" } ],
  "keepAlive": { "command": "", "frequency": "" },
  "query": { "type": "" },
  "stats": { "type": "" }
}
```

### ServerData
```json
{
  "data": { "version": { "type": "string", "value": "1.20.1" } },
  "groups": [ { "display": "Config", "variables": ["memory", "port"] } ]
}
```

### ServerRunning (Status)
```json
{ "running": true, "installing": false }
```

### ServerStats
```json
{
  "cpu": 45.2, "memory": 1536000000, "maxMemory": 2147483648,
  "running": true, "storage": 5000000000,
  "jvm": { "heapUsed": 1000000000, "heapTotal": 2000000000, "metaspaceUsed": 50000000, "metaspaceTotal": 100000000 }
}
```

### ServerLogs
```json
{ "logs": ["[10:30:15] [Server thread/INFO]: Starting server"], "epoch": 1705312215 }
```

### ServerFlags
```json
{ "autoStart": false, "autoRestartOnCrash": true, "autoRestartOnGraceful": false }
```

### ServerTask
```json
{
  "name": "Backup diario", "description": "Ejecuta backup cada 6h",
  "cronSchedule": "0 */6 * * *",
  "operations": [ { "type": "backup" } ]
}
```

### User
```json
{ "id": 1, "username": "admin", "email": "admin@example.com", "otpActive": false, "roleId": null, "scopes": [] }
```

### Permissions
```json
{ "serverIdentifier": "abc123", "scopes": [ { "value": "server.view", "forServer": true } ] }
```

### Backup
```json
{ "id": 1, "name": "Backup 2024-01-15", "fileName": "backup_abc123.tar.gz", "createdAt": "2024-01-15T10:30:00Z" }
```

### Features
```json
{ "arch": "amd64", "os": "linux", "version": "1.0.0", "environments": ["standard", "docker"], "features": ["docker"] }
```

### SystemInfo
```json
{
  "hostname": "node1", "os": "linux", "platform": "ubuntu", "platformVersion": "22.04",
  "arch": "amd64", "uptime": 123456,
  "cpuModel": "Intel Core", "cpuCores": 8, "cpuThreads": 16, "cpuUsage": 25.5,
  "memoryTotal": 17179869184, "memoryUsed": 8589934592, "memoryFree": 8589934592,
  "disks": [ { "path": "/", "total": 1099511627776, "used": 549755813888, "free": 549755813888, "usedPercent": 50.0 } ],
  "networkBytesSent": 1000000, "networkBytesRecv": 2000000
}
```

### DatabaseHostView
```json
{
  "id": 1, "name": "DB Host 1", "host": "db1.example.com", "port": 3306,
  "username": "skypanel", "node_id": 1, "max_databases": 10,
  "created_at": "...", "updated_at": "..."
}
```

### DatabaseView
```json
{
  "id": 1, "server_id": "abc123", "database_host_id": 1, "database_name": "server_abc123",
  "host": "db1.example.com", "port": 3306, "username": "user_abc123",
  "password": "pass123", "max_connections": 5, "remote_connection": "",
  "host_name": "DB Host 1", "created_at": "...", "updated_at": "..."
}
```

### Template
```json
{
  "id": "minecraft-java", "name": "Minecraft Java Edition", "display": "Minecraft Java",
  "type": "minecraft-java", "icon": "minecraft.png",
  "install": [], "run": { "command": "" }, "data": {},
  "environment": { "type": "standard" }
}
```

### TemplateRepo
```json
{ "id": 1, "name": "official", "url": "https://templates.example.com/templates.json", "branch": "main", "isLocal": false }
```

### Role
```json
{ "id": 1, "name": "Admin", "description": "Full access", "scopes": ["admin"], "createdAt": "...", "updatedAt": "..." }
```

### Client (OAuth2)
```json
{ "client_id": "abc123", "client_secret": "secret...", "name": "My App", "description": "App description" }
```

### PluginInfo
```json
{ "name": "EssentialsX.jar", "version": "2.20.1", "size": 1234567 }
```

### PluginSearchResult
```json
{
  "id": "essentialssx", "name": "EssentialsX", "tag": "essentialsx",
  "version": "2.20.1", "author": "EssentialsX Team",
  "description": "Essential commands...", "iconUrl": "...", "downloads": 1000000
}
```

---

## Config

### `GET /api/config`
Sin autenticación. Retorna configuración pública del panel.

```json
{
  "branding": { "name": "SkyPanel" },
  "registrationEnabled": true,
  "themes": { "active": "default", "available": ["alternativeTheme"], "settings": "{}" }
}
```

---

## Auth

### `POST /auth/login`
**Body:**
```json
{ "email": "admin@example.com", "password": "..." }
```
**Respuesta:**
```json
{
  "scopes": [{ "value": "server.view", "forServer": true }],
  "otpNeeded": false,
  "session": "session_token"
}
```
- `otpNeeded: false` → login completo; también setea la cookie `skypanel_auth`.
- `otpNeeded: true` → continuar con `/auth/otp` (la sesión en curso se guarda en la cookie del navegador).
- `session` expone el token de sesión para autenticación de apps externas.

### `POST /auth/otp`
**Body:**
```json
{ "token": "123456" }
```
`token` es el **código OTP** (el email del usuario y el timestamp provienen de la sesión en cookie, con expiración de 5 minutos).
**Respuesta:** igual que `/auth/login` (LoginResponse con `scopes` y `session`).

### `POST /auth/logout`
Cierra la sesión actual.

### `POST /auth/register`
Requiere `registrationEnabled: true`.
**Body:**
```json
{ "username": "newuser", "email": "new@example.com", "password": "Secure123!" }
```

### `POST /auth/forgot-password`
Solicita un enlace de reset para un email. **Respuesta:** siempre `204` (evita enumeración de usuarios). Si el email existe, se envía un correo con el enlace `/reset-password/?token=<token>`.
**Body:**
```json
{ "email": "admin@example.com" }
```

### `POST /auth/reset-password`
Consume el token de reset y establece una nueva contraseña. **Respuesta:** `204`. **Errores:** `400` si el token es inválido/vencido o la contraseña no cumple los requisitos.
**Body:**
```json
{ "token": "reset_token", "password": "NuevaSegura123!" }
```

### `POST /auth/reauth`
Re-autentica la sesión actual. **Auth:** Bearer.

### `GET /auth/publickey`
Retorna la clave pública Ed25519 en formato JWK para validar JWTs.

---

## OAuth2

### `POST /oauth2/token`
**Form (urlencoded):** `grant_type`, `client_id`, `client_secret`, `username`, `password`

Ver [Autenticación](#autenticación) para ejemplos.

**Errores:**
```json
{ "error": "invalid_client", "error_description": "Invalid client credentials" }
```

---

## Nodos

| Método | Path | Scope | Descripción |
|--------|------|-------|-------------|
| GET | `/api/nodes` | `nodes.view` | Listar nodos |
| POST | `/api/nodes` | `nodes.create` | Crear nodo |
| GET | `/api/nodes/:id` | `nodes.view` | Obtener nodo |
| PUT | `/api/nodes/:id` | `nodes.edit` | Actualizar nodo |
| DELETE | `/api/nodes/:id` | `nodes.delete` | Eliminar nodo |
| GET | `/api/nodes/:id/features` | `nodes.view` | Features del nodo |
| GET | `/api/nodes/:id/system` | `nodes.view` | Info del sistema |
| GET | `/api/nodes/:id/deployment` | `nodes.deploy` | Datos de despliegue |

### `POST /api/nodes`
**Body:**
```json
{
  "name": "Node-02", "publicHost": "node2.example.com", "privateHost": "192.168.1.11",
  "publicPort": 8080, "privatePort": 8080, "sftpPort": 5657
}
```
**Respuesta:** `Node` (incluye `id`)

### `GET /api/nodes/:id/deployment`
```json
{ "clientId": ".node_1", "clientSecret": "abc123def456...", "publicKey": "..." }
```

---

## Servidores

La mayoría de los endpoints de acción usan `proxyServerRequest` que reenvía la petición al daemon del nodo.

| Método | Path | Scope | Descripción |
|--------|------|-------|-------------|
| GET | `/api/servers` | (auth) | Listar servidores |
| GET | `/api/servers/:serverId` | `server.view` | Obtener servidor |
| PUT | `/api/servers/:serverId` | `server.create` | Crear servidor |
| DELETE | `/api/servers/:serverId` | `server.delete` | Eliminar servidor |
| POST | `/api/servers/:serverId/suspend` | `server.edit.data.admin` | Suspender/activar |
| PUT | `/api/servers/:serverId/name/:name` | `server.name.edit` | Renombrar |
| GET | `/api/servers/:serverId/definition` | `server.definition.view` | Obtener definición |
| PUT | `/api/servers/:serverId/definition` | `server.definition.edit` | Editar definición |
| GET | `/api/servers/:serverId/data` | `server.data.view` | Obtener variables |
| POST | `/api/servers/:serverId/data` | `server.data.edit` | Editar variables |
| PUT | `/api/servers/:serverId/data` | `server.data.edit.admin` | Editar datos (admin) |
| POST | `/api/servers/:serverId/transfer` | `server.edit.data.admin` | Transferir a otro nodo |
| GET | `/api/servers/:serverId/status` | `server.status` | Estado (running/stopped) |
| GET | `/api/servers/:serverId/stats` | `server.stats` | Estadísticas |
| GET | `/api/servers/:serverId/console` | `server.console` | Logs de consola |
| POST | `/api/servers/:serverId/console` | `server.console.send` | Enviar comando |
| GET | `/api/servers/:serverId/flags` | `server.flags.view` | Obtener flags |
| POST | `/api/servers/:serverId/flags` | `server.flags.edit` | Editar flags |
| POST | `/api/servers/:serverId/start` | `server.start` | Iniciar |
| POST | `/api/servers/:serverId/stop` | `server.stop` | Detener |
| POST | `/api/servers/:serverId/restart` | `server.start`+`server.stop` | Reiniciar |
| POST | `/api/servers/:serverId/kill` | `server.kill` | Matar proceso |
| POST | `/api/servers/:serverId/install` | `server.install` | Ejecutar instalación |
| POST | `/api/servers/:serverId/reload` | `server.reload` | Recargar configuración |
| HEAD | `/api/servers/:serverId/query` | `server.stats` | Consultar(query) server |
| GET | `/api/servers/:serverId/query` | `server.stats` | Consultar server |
| GET | `/api/servers/:serverId/socket` | `server.view` | WebSocket (consola/stats) |

### `GET /api/servers`
**Query params:** `name` (filtro con `*`), `node` (ID), `username`, `page`, `limit`.

```json
{
  "servers": [ { "identifier": "abc123", "name": "Server", "node": { "id": 1, "name": "Node" }, "ip": "10.0.0.1", "port": 25565, "type": "minecraft-java", "canGetStatus": true } ],
  "metadata": { "paging": { "page": 1, "size": 20, "maxSize": 100, "total": 1 } }
}
```

### `GET /api/servers/:serverId?perms=true`
```json
{
  "server": { "identifier": "abc123", "name": "Server", "icon": "minecraft.png", "node": { "id": 1, "name": "Node" } },
  "perms": { "scopes": [{ "value": "server.view", "forServer": true }] }
}
```

### `PUT /api/servers/:serverId`
Crea un servidor. **Body:** `ServerDefinition` (ver tipos).
**Respuesta:** `{ "id": "abc123" }`

### `DELETE /api/servers/:serverId?skipNode=true`
`skipNode`: elimina solo de la BD, no del nodo.

### `POST /api/servers/:serverId/suspend`
Suspende o activa el servidor (toggle).

### `PUT /api/servers/:serverId/name/:name`
Renombra el servidor en la BD.

### `GET /api/servers/:serverId/definition`
Retorna la definición completa del servidor.

### `PUT /api/servers/:serverId/definition`
**Body:** `ServerDefinition`.
**Respuesta:** `204`.

### `GET /api/servers/:serverId/data`
```json
{ "data": { "version": { "type": "string", "value": "1.20.1" } }, "groups": [] }
```

### `POST /api/servers/:serverId/data`
Edita variables del servidor. **Body:** `{ "key": "value" }` (objeto plano).
**Respuesta:** `202`.

### `PUT /api/servers/:serverId/data`
Edición admin de datos. **Body:** `{ ... }`. **Respuesta:** `202`.

### `POST /api/servers/:serverId/transfer`
**Body:** `{ "nodeId": 2 }`. Transfiere el servidor a otro nodo.
**Respuesta:** `202 "Transfer started"`.

### `GET /api/servers/:serverId/status`
```json
{ "running": true, "installing": false }
```

### `GET /api/servers/:serverId/stats`
```json
{
  "cpu": 45.2, "memory": 1536000000, "maxMemory": 2147483648,
  "running": true, "storage": 5000000000,
  "jvm": { "heapUsed": 1000000000, "heapTotal": 2000000000, "metaspaceUsed": 50000000, "metaspaceTotal": 100000000 }
}
```

### `GET /api/servers/:serverId/console?time=1705312215000`
`time`: epoch en ms para obtener logs desde ese momento.
```json
{ "logs": ["[10:30:15] [Server thread/INFO]: Starting server"], "epoch": 1705312215 }
```

### `POST /api/servers/:serverId/console`
**Body:** `"command"` (string literal del comando).
**Respuesta:** `204`.

### `GET /api/servers/:serverId/flags`
```json
{ "autoStart": false, "autoRestartOnCrash": true, "autoRestartOnGraceful": false }
```

### `POST /api/servers/:serverId/flags`
**Body:** `ServerFlags`. **Respuesta:** `204`.

### Acciones de ciclo de vida

| Acción | Método | Respuesta |
|--------|--------|-----------|
| Iniciar | `POST /api/servers/:serverId/start` | `202` / `204` |
| Detener | `POST /api/servers/:serverId/stop` | `202` / `204` |
| Reiniciar | `POST /api/servers/:serverId/restart` | `202` / `204` |
| Matar | `POST /api/servers/:serverId/kill` | `204` |
| Instalar | `POST /api/servers/:serverId/install` | `202` / `204` |
| Recargar | `POST /api/servers/:serverId/reload` | `204` |

### `HEAD` / `GET /api/servers/:serverId/query`
Consulta el servidor de juego vía query protocol.

---

## Archivos

| Método | Path | Scope | Descripción |
|--------|------|-------|-------------|
| GET | `/api/servers/:serverId/file/*filename` | `server.files.view` | Listar/descargar archivo |
| PUT | `/api/servers/:serverId/file/*filename` | `server.files.edit` | Subir archivo |
| DELETE | `/api/servers/:serverId/file/*filename` | `server.files.edit` | Eliminar archivo |
| POST | `/api/servers/:serverId/file/*filename` | `server.files.edit` | Mover/copiar archivo |

**Listar directorio:** `GET /api/servers/:serverId/file/`
```json
{
  "files": [
    { "name": "server.jar", "size": 45678901, "modified": "2024-01-15T10:30:00Z", "isFile": true },
    { "name": "world", "size": 0, "modified": "2024-01-15T10:25:00Z", "isFile": false }
  ]
}
```

**Subir:** `PUT /api/servers/:serverId/file/config.yml` con `Content-Type: application/octet-stream`.

---

## Archivos Comprimidos

| Método | Path | Scope | Descripción |
|--------|------|-------|-------------|
| HEAD | `/api/servers/:serverId/archive/*filename` | `server.files.edit` | Verificar si existe |
| POST | `/api/servers/:serverId/archive/*filename` | `server.files.edit` | Crear ZIP |
| POST | `/api/servers/:serverId/extract/*filename` | `server.files.edit` | Extraer ZIP |

### `POST /api/servers/:serverId/archive/backup.zip`
**Body:** `["file1.txt", "folder/"]` — archivos a comprimir.
**Query:** `destination` — subdirectorio de destino.
**Respuesta:** `204`.

### `POST /api/servers/:serverId/extract/archive.zip`
**Query:** `destination` — directorio donde extraer (vacío = raíz del servidor).
**Respuesta:** `204`.

---

## Plugins

| Método | Path | Scope |
|--------|------|-------|
| GET | `/api/servers/:serverId/plugins` | `server.files.view` |
| DELETE | `/api/servers/:serverId/plugins` | `server.files.edit` |
| GET | `/api/servers/:serverId/plugins/search` | `server.files.view` |
| POST | `/api/servers/:serverId/plugins/:pluginId` | `server.files.edit` |

### `GET /api/servers/:serverId/plugins`
```json
[{ "name": "EssentialsX.jar", "version": "2.20.1", "size": 1234567 }]
```

### `GET /api/servers/:serverId/plugins/search?q=essentials`
```json
[{ "id": "essentialsx", "name": "EssentialsX", "version": "2.20.1", "author": "...", "downloads": 1000000 }]
```

### `DELETE /api/servers/:serverId/plugins?name=EssentialsX.jar`
Elimina el plugin `EssentialsX.jar`.

### `POST /api/servers/:serverId/plugins/:pluginId`
Instala el plugin desde SpigotMC (ID numérico de Spigot).

---

## Backups

| Método | Path | Scope |
|--------|------|-------|
| GET | `/api/servers/:serverId/backup` | `server.backup.view` |
| GET | `/api/servers/:serverId/backup/:backupID` | `server.backup.view` |
| DELETE | `/api/servers/:serverId/backup/:backupID` | `server.backup.delete` |
| POST | `/api/servers/:serverId/backup/create` | `server.backup.create` |
| POST | `/api/servers/:serverId/backup/restore/:backupID` | `server.backup.restore` |
| GET | `/api/servers/:serverId/backup/download/:backupID` | `server.backup.view` |

### `GET /api/servers/:serverId/backup`
```json
[{ "id": 1, "name": "Backup 2024-01-15", "fileName": "backup_abc123.tar.gz", "createdAt": "2024-01-15T10:30:00Z" }]
```

### `POST /api/servers/:serverId/backup/create`
**Respuesta:** `{ "backupFileName": "backup_abc123.tar.gz" }`

---

## Tareas (Tasks)

| Método | Path | Scope |
|--------|------|-------|
| GET | `/api/servers/:serverId/tasks` | `server.tasks.view` |
| GET | `/api/servers/:serverId/tasks/:taskId` | `server.tasks.view` |
| PUT | `/api/servers/:serverId/tasks/:taskId` | `server.tasks.edit` |
| DELETE | `/api/servers/:serverId/tasks/:taskId` | `server.tasks.delete` |
| POST | `/api/servers/:serverId/tasks/:taskId/run` | `server.tasks.run` |

### `GET /api/servers/:serverId/tasks`
```json
{
  "tasks": {
    "backup_task": { "name": "Backup diario", "cronSchedule": "0 */6 * * *", "operations": [{ "type": "backup" }] }
  }
}
```

### `PUT /api/servers/:serverId/tasks/:taskId`
**Body:** `ServerTask`. **Respuesta:** `204`.

---

## Bases de Datos del Servidor

| Método | Path | Scope |
|--------|------|-------|
| GET | `/api/servers/:serverId/databases` | `server.view` |
| POST | `/api/servers/:serverId/databases` | `server.data.edit` |
| DELETE | `/api/servers/:serverId/databases/:id` | `server.data.edit` |

### `POST /api/servers/:serverId/databases`
**Body:** `{ "database_host_id": 1, "database_name": "my_db" }`
**Respuesta:** `DatabaseView` (con username/password generados).

---

## Usuarios del Servidor

| Método | Path | Scope |
|--------|------|-------|
| GET | `/api/servers/:serverId/user` | `server.users.view` |
| GET | `/api/servers/:serverId/user/:email` | `server.users.view` |
| PUT | `/api/servers/:serverId/user/:email` | `server.users.edit` |
| DELETE | `/api/servers/:serverId/user/:email` | `server.users.delete` |

### `GET /api/servers/:serverId/user`
```json
[{ "username": "admin", "email": "admin@example.com", "scopes": ["server.view", "server.console"] }]
```

### `PUT /api/servers/:serverId/user/:email`
**Body:** `{ "permissions": { "scopes": ["server.view", "server.console"] } }`
**Respuesta:** `204`.

---

## Transferencia entre Nodos

| Método | Path | Scope |
|--------|------|-------|
| POST | `/api/servers/:serverId/transfer` | `server.edit.data.admin` |

**Body:** `{ "nodeId": 2 }`

---

## Transferencia Externa

Endpoints públicos para migrar servidores entre paneles (sin autenticación).

| Método | Path | Descripción |
|--------|------|-------------|
| POST | `/api/extransfer/validate` | Validar token de transferencia |
| POST | `/api/extransfer/consume` | Consumir transferencia |
| POST | `/api/extransfer/heartbeat` | Heartbeat durante transferencia |
| POST | `/api/extransfer/confirm` | Confirmar transferencia |
| GET | `/api/extransfer/download` | Descargar datos de transferencia |
| POST | `/api/extransfer/cancel` | Cancelar transferencia |

También desde el servidor:

| Método | Path | Scope |
|--------|------|-------|
| POST | `/api/servers/:serverId/extransfer/create` | `server.edit.data.admin` |
| POST | `/api/servers/:serverId/extransfer/pull` | `server.edit.data.admin` |
| GET | `/api/servers/:serverId/extransfer/status` | `server.edit.data.admin` |

---

## AI

| Método | Path | Scope |
|--------|------|-------|
| POST | `/api/ai/analyze` | `—` (autenticado) |
| POST | `/api/servers/:serverId/ai/analyze` | `server.console` |

Analiza logs del servidor usando Google GenAI (requiere `geminiApiKey` configurada).

### `POST /api/ai/analyze`
```json
// Request
{ "logs": ["[ERROR] Connection refused", "[WARN] Memory low"] }

// Response
{
  "summary": "Resumen del análisis...",
  "rootCauses": ["Causa raíz 1"],
  "suggestions": ["Sugerencia 1", "Sugerencia 2"]
}
```

---

## Usuarios Globales

| Método | Path | Scope | Descripción |
|--------|------|-------|-------------|
| GET | `/api/users` | `users.info.search` | Buscar usuarios |
| POST | `/api/users` | `users.info.edit` | Crear usuario |
| GET | `/api/users/:id` | `users.info.view` | Obtener usuario |
| POST | `/api/users/:id` | `users.info.edit` | Actualizar usuario |
| DELETE | `/api/users/:id` | `users.info.edit` | Eliminar usuario |
| GET | `/api/users/:id/perms` | `users.perms.view` | Obtener permisos |
| PUT | `/api/users/:id/perms` | `users.perms.edit` | Actualizar permisos |

### `GET /api/users?username=admin*&email=*@example.com&page=1&limit=25`
```json
{
  "users": [{ "id": 1, "username": "admin", "email": "admin@example.com" }],
  "metadata": { "paging": { "page": 1, "size": 20, "maxSize": 100, "total": 1 } }
}
```

### `POST /api/users`
**Body:** `{ "username": "newuser", "email": "new@example.com", "password": "Secure123!" }`

### `POST /api/users/:id`
**Body:** `{ "username": "newname", "email": "new@example.com", "password": "newpass" }`

### `GET /api/users/:id/perms`
```json
{ "serverIdentifier": "", "scopes": [{ "value": "server.view", "forServer": false }, { "value": "admin", "forServer": false }] }
```

### `PUT /api/users/:id/perms`
**Body:** `{ "scopes": ["admin", "server.view"] }` (array de strings).
**Respuesta:** `204`.

---

## Self (Perfil Propio)

| Método | Path | Scope | Descripción |
|--------|------|-------|-------------|
| GET | `/api/self` | `login` | Obtener perfil propio |
| PUT | `/api/self` | `self.edit` | Actualizar perfil |
| GET | `/api/self/otp` | `self.edit` | Estado de OTP |
| POST | `/api/self/otp` | `self.edit` | Iniciar enrolamiento OTP |
| PUT | `/api/self/otp` | `self.edit` | Validar enrolamiento |
| POST | `/api/self/otp/recovery` | `self.edit` | Regenerar códigos de recuperación |
| DELETE | `/api/self/otp/:token` | `self.edit` | Deshabilitar OTP |
| GET | `/api/self/oauth2` | `self.clients` | Listar OAuth2 clients |
| POST | `/api/self/oauth2` | `self.clients` | Crear client |
| DELETE | `/api/self/oauth2/:clientID` | `self.clients` | Eliminar client |

---

## Settings

| Método | Path | Scope | Descripción |
|--------|------|-------|-------------|
| GET | `/api/settings` | `settings.edit` | Obtener configuración |
| POST | `/api/settings` | `settings.edit` | Actualizar múltiples valores |
| GET | `/api/settings/:key` | `settings.edit` | Obtener un valor |
| PUT | `/api/settings/:key` | `settings.edit` | Actualizar un valor |
| POST | `/api/settings/test/email` | `settings.edit` | Enviar email de prueba |
| POST | `/api/settings/test/discord` | `settings.edit` | Enviar notificación Discord de prueba |
| POST | `/api/settings/license/activate` | `settings.edit` | Activar licencia |

### `POST /api/settings`
**Body:** `{ "companyName": "Mi Empresa", "registrationEnabled": false }`. **Respuesta:** `204`.

### `PUT /api/settings/:key`
**Body:** `{ "value": "nuevo_valor" }`. **Respuesta:** `204`.

---

## User Settings

| Método | Path | Scope |
|--------|------|-------|
| GET | `/api/userSettings` | `login` |
| PUT | `/api/userSettings/:key` | `login` |

### `PUT /api/userSettings/theme`
**Body:** `{ "value": "dark" }`. **Respuesta:** `204`.

---

## API Keys

| Método | Path | Scope |
|--------|------|-------|
| GET | `/api/settings/apikeys` | `admin` |
| POST | `/api/settings/apikeys` | `admin` |
| DELETE | `/api/settings/apikeys/:id` | `admin` |

---

## Roles

| Método | Path | Scope |
|--------|------|-------|
| GET | `/api/roles` | `admin` o `users.info.view/edit` |
| POST | `/api/roles` | `admin` |
| GET | `/api/roles/:id` | `admin` |
| POST | `/api/roles/:id` | `admin` |
| DELETE | `/api/roles/:id` | `admin` |

### `POST /api/roles`
**Body:** `{ "name": "Moderator", "description": "Can manage servers", "scopes": ["server.view", "server.start"] }`

---

## Database Hosts

| Método | Path | Scope |
|--------|------|-------|
| GET | `/api/databasehosts` | `admin` |
| POST | `/api/databasehosts` | `admin` |
| GET | `/api/databasehosts/:id` | `admin` |
| PUT | `/api/databasehosts/:id` | `admin` |
| DELETE | `/api/databasehosts/:id` | `admin` |

### `POST /api/databasehosts`
**Body:**
```json
{
  "name": "DB Host 1", "host": "db1.example.com", "port": 3306,
  "username": "skypanel", "password": "secret", "max_databases": 10, "node_id": 1
}
```

---

## Templates

| Método | Path | Scope |
|--------|------|-------|
| GET | `/api/templates` | `login` |
| POST | `/api/templates` | `templates.repo.create` |
| GET | `/api/templates/:repo` | `login` |
| DELETE | `/api/templates/:repo` | `templates.repo.delete` |
| GET | `/api/templates/:repo/:name` | `login` |
| PUT | `/api/templates/0/:name` | `templates.local.edit` |
| DELETE | `/api/templates/0/:name` | `templates.local.edit` |

`:repo=0` es el repositorio local. Los repositorios remotos tienen IDs > 0.

---

## Provision Products

| Método | Path | Scope |
|--------|------|-------|
| GET | `/api/provision/products` | `admin` |
| POST | `/api/provision/products` | `admin` |
| PUT | `/api/provision/products/:id` | `admin` |
| DELETE | `/api/provision/products/:id` | `admin` |

---

## Provision API v1

Autenticación por API Key en header. Endpoints para integración con sistemas externos (WHMCS, etc.).

| Método | Path | Descripción |
|--------|------|-------------|
| GET | `/api/v1/ping` | Verificar conectividad |
| POST | `/api/v1/provision` | Crear servidor automáticamente |
| POST | `/api/v1/terminate` | Terminar servidor |
| POST | `/api/v1/suspend` | Suspender servidor |
| POST | `/api/v1/unsuspend` | Reactivar servidor |

---

## Uptime

| Método | Path | Scope | Descripción |
|--------|------|-------|-------------|
| GET | `/api/uptime` | `admin` o `uptime.view` | Todos los registros |
| GET | `/api/uptime/:serverId` | `server.view` | Registros de un servidor |

**Query params:** `days` (días hacia atrás), `limit` (número de registros).

---

## Daemon

Endpoints del daemon para comunicación directa entre nodos y panel. No pasan por proxy. Usan autenticación JWT.

| Método | Path | Scope Swagger |
|--------|------|---------------|
| GET | `/daemon` | `none` |
| HEAD | `/daemon` | `none` |
| GET | `/daemon/features` | `none` |
| GET | `/daemon/system` | `none` |
| GET/PUT/DELETE | `/daemon/server/:serverId/...` | (según acción) |

### Acciones del Daemon por Servidor

| Método | Path | Scope Swagger |
|--------|------|---------------|
| PUT | `/daemon/server/:serverId` | — |
| DELETE | `/daemon/server/:serverId` | — |
| GET | `/daemon/server/:serverId/definition` | `server.definition.view` |
| PUT | `/daemon/server/:serverId/definition` | `server.definition.edit` |
| GET | `/daemon/server/:serverId/data` | `server.data.view` |
| POST | `/daemon/server/:serverId/data` | — |
| PUT | `/daemon/server/:serverId/data` | — |
| GET | `/daemon/server/:serverId/tasks` | `server.tasks.view` |
| GET/PUT/DELETE | `/daemon/server/:serverId/tasks/:taskID` | `server.tasks.*` |
| POST | `/daemon/server/:serverId/tasks/:taskID/run` | `server.tasks.run` |
| POST | `/daemon/server/:serverId/reload` | `server.reload` |
| POST | `/daemon/server/:serverId/start` | `server.start` |
| POST | `/daemon/server/:serverId/restart` | `server.start` + `server.stop` |
| POST | `/daemon/server/:serverId/stop` | `server.stop` |
| POST | `/daemon/server/:serverId/kill` | `server.kill` |
| POST | `/daemon/server/:serverId/install` | `server.install` |
| GET | `/daemon/server/:serverId/file/*filename` | `server.files.view` |
| PUT | `/daemon/server/:serverId/file/*filename` | `server.files.edit` |
| DELETE | `/daemon/server/:serverId/file/*filename` | `server.files.edit` |
| GET | `/daemon/server/:serverId/console` | `server.console` |
| POST | `/daemon/server/:serverId/console` | `server.console.send` |
| GET | `/daemon/server/:serverId/flags` | `server.flags.view` |
| POST | `/daemon/server/:serverId/flags` | `server.flags.edit` |
| GET | `/daemon/server/:serverId/stats` | `server.stats` |
| GET | `/daemon/server/:serverId/status` | `server.status` |
| POST | `/daemon/server/:serverId/archive/*filename` | `server.files.edit` |
| POST | `/daemon/server/:serverId/extract/*filename` | `server.files.edit` |
| POST | `/daemon/server/:serverId/backup/create` | `server.backup.create` |
| DELETE | `/daemon/server/:serverId/backup` | `server.backup.delete` |
| POST | `/daemon/server/:serverId/backup/restore` | `server.backup.restore` |
| GET | `/daemon/server/:serverId/backup/download` | `server.backup.restore` |
| HEAD/GET | `/daemon/server/:serverId/query` | `server.stats` |
| GET | `/daemon/server/:serverId/plugins` | — |
| DELETE | `/daemon/server/:serverId/plugins` | — |
| GET | `/daemon/server/:serverId/plugins/search` | — |
| POST | `/daemon/server/:serverId/plugins/:pluginId` | — |
| GET | `/daemon/server/:serverId/socket` | — |

---

## WebSocket

### `GET /api/servers/:serverId/socket`

Conecta a la consola y estadísticas en tiempo real.

```javascript
const ws = new WebSocket(`ws://localhost:8080/api/servers/${serverId}/socket`);
```

La autenticación se realiza mediante la cookie de sesión (`skypanel_auth`); no se requiere token en URL.

### Tipos de Mensaje

| Tipo | Dirección | Descripción |
|------|-----------|-------------|
| `console` | Servidor → Cliente | Línea de consola (struct `ServerLogs`) |
| `stat` | Servidor → Cliente | Estadísticas periódicas (struct `ServerStats`) |
| `status` | Servidor → Cliente | Cambio de estado (struct `ServerRunning`) |
| `console` | Cliente → Servidor | Enviar comando (data = string) |

### Eventos del Servidor
```json
{ "type": "console", "data": { "epoch": 1712345678000, "logs": "[10:30:15] [Server thread/INFO]: Starting server" } }
{ "type": "stat", "data": { "cpu": 45.2, "memory": 1536000000, "maxMemory": 2147483648, "storage": 5000000000, "maxStorage": 10737418240, "networkRx": 0, "networkTx": 0, "running": true } }
{ "type": "status", "data": { "running": true, "installing": false } }
```

### Enviar Comando
```json
{ "type": "console", "data": "say Hello World!" }
```

---

## Ejemplos

### cURL
```bash
TOKEN=$(curl -s -X POST http://localhost:8080/oauth2/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=ID" \
  -d "client_secret=SECRET" | jq -r '.access_token')

curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/servers
curl -s -X POST -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/servers/abc123/start
```

### Python
```python
import requests
api = "http://localhost:8080"
r = requests.post(f"{api}/oauth2/token", data={"grant_type": "client_credentials", "client_id": "ID", "client_secret": "SECRET"})
token = r.json()["access_token"]
headers = {"Authorization": f"Bearer {token}"}
servers = requests.get(f"{api}/api/servers", headers=headers).json()
```

### JavaScript
```javascript
const api = axios.create({ baseURL: 'http://localhost:8080' });
const { data } = await api.post('/oauth2/token', new URLSearchParams({ grant_type: 'client_credentials', client_id: 'ID', client_secret: 'SECRET' }));
api.defaults.headers.Authorization = `Bearer ${data.access_token}`;
const servers = (await api.get('/api/servers')).data;
```
