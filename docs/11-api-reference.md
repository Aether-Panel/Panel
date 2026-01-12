# 🔌 Referencia Completa de API de SkyPanel

## Tabla de Contenidos

- [Introducción](#introducción)
- [Autenticación](#autenticación)
- [Formato de Datos](#formato-de-datos)
- [Manejo de Errores](#manejo-de-errores)
- [Paginación](#paginación)
- [Endpoints de Servidores](#endpoints-de-servidores)
- [Endpoints de Usuarios](#endpoints-de-usuarios)
- [Endpoints de Nodos](#endpoints-de-nodos)
- [Endpoints de Configuración](#endpoints-de-configuración)
- [Endpoints de Plantillas](#endpoints-de-plantillas)
- [WebSocket API](#websocket-api)
- [Ejemplos de Uso](#ejemplos-de-uso)
- [SDKs y Librerías](#sdks-y-librerías)

---

## Introducción

La API de SkyPanel es una **API RESTful** completa que permite la automatización y gestión programática de todos los aspectos del panel. Está diseñada siguiendo los estándares de la industria y soporta OAuth2 para autenticación.

### Características de la API

- ✅ **RESTful**: Sigue principios REST estándar
- ✅ **OAuth2**: Autenticación segura con tokens
- ✅ **JSON**: Formato de datos JSON para requests y responses
- ✅ **Versionada**: API versionada para compatibilidad
- ✅ **Documentada**: Documentación completa con ejemplos
- ✅ **WebSocket**: Soporte para comunicación en tiempo real
- ✅ **Paginación**: Resultados paginados para grandes conjuntos de datos
- ✅ **Filtrado**: Búsqueda y filtrado avanzado

### URL Base

```
http://tu-servidor:8080/api
```

O con HTTPS en producción:

```
https://panel.tudominio.com/api
```

---

## Autenticación

SkyPanel utiliza **OAuth2** con el flujo de **Client Credentials** para autenticación de API.

### Paso 1: Crear un Cliente OAuth2

Puedes crear un cliente OAuth2 desde:
- **Panel Web**: Configuración → OAuth2 Clients
- **CLI**: `skypanel oauth2 create`

### Paso 2: Obtener Token de Acceso

**Endpoint**: `POST /oauth2/token`

**Headers**:
```http
Content-Type: application/x-www-form-urlencoded
```

**Body** (form-urlencoded):
```
grant_type=client_credentials
client_id=TU_CLIENT_ID
client_secret=TU_CLIENT_SECRET
```

**Ejemplo con cURL**:
```bash
curl -X POST http://localhost:8080/oauth2/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_CLIENT_SECRET"
```

**Respuesta**:
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "server.view server.edit"
}
```

### Paso 3: Usar el Token

Incluye el token en el header `Authorization` de todas las peticiones:

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Ejemplo**:
```bash
curl -X GET http://localhost:8080/api/servers \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Scopes (Permisos)

Los scopes definen qué acciones puede realizar un token:

| Scope | Descripción |
|-------|-------------|
| `admin` | Acceso administrativo completo |
| `server.view` | Ver servidores |
| `server.create` | Crear servidores |
| `server.edit` | Editar servidores |
| `server.delete` | Eliminar servidores |
| `server.start` | Iniciar servidores |
| `server.stop` | Detener servidores |
| `server.console` | Acceso a consola |
| `server.files.view` | Ver archivos |
| `server.files.edit` | Editar archivos |
| `users.view` | Ver usuarios |
| `users.edit` | Editar usuarios |
| `nodes.view` | Ver nodos |
| `nodes.edit` | Editar nodos |

---

## Formato de Datos

### Content-Type

Todas las peticiones y respuestas usan JSON:

```http
Content-Type: application/json
Accept: application/json
```

### Estructura de Respuesta Exitosa

```json
{
  "data": { ... },
  "metadata": {
    "paging": {
      "page": 1,
      "size": 25,
      "maxSize": 100,
      "total": 150
    }
  }
}
```

### Códigos de Estado HTTP

| Código | Significado | Descripción |
|--------|-------------|-------------|
| `200` | OK | Petición exitosa |
| `201` | Created | Recurso creado exitosamente |
| `204` | No Content | Petición exitosa sin contenido de respuesta |
| `400` | Bad Request | Datos de entrada inválidos |
| `401` | Unauthorized | Token inválido o expirado |
| `403` | Forbidden | Sin permisos para esta acción |
| `404` | Not Found | Recurso no encontrado |
| `500` | Internal Server Error | Error del servidor |

---

## Manejo de Errores

### Estructura de Error

```json
{
  "error": {
    "code": "ErrServerNotFound",
    "msg": "Server with ID {id} not found",
    "metadata": {
      "id": "ABC12345"
    }
  }
}
```

### Códigos de Error Comunes

| Código | Descripción |
|--------|-------------|
| `ErrFieldRequired` | Campo requerido faltante |
| `ErrFieldInvalid` | Valor de campo inválido |
| `ErrServerNotFound` | Servidor no encontrado |
| `ErrUserNotFound` | Usuario no encontrado |
| `ErrNodeNotFound` | Nodo no encontrado |
| `ErrPermissionDenied` | Permiso denegado |
| `ErrDatabaseError` | Error de base de datos |

---

## Paginación

Los endpoints que retornan listas soportan paginación:

### Parámetros de Query

| Parámetro | Tipo | Default | Descripción |
|-----------|------|---------|-------------|
| `page` | int | 1 | Número de página (1-indexed) |
| `limit` | int | 25 | Elementos por página |

**Máximo**: 100 elementos por página

### Ejemplo

```bash
GET /api/servers?page=2&limit=50
```

### Respuesta con Metadata

```json
{
  "servers": [...],
  "metadata": {
    "paging": {
      "page": 2,
      "size": 50,
      "maxSize": 100,
      "total": 237
    }
  }
}
```

---

## Endpoints de Servidores

### Listar Servidores

**Endpoint**: `GET /api/servers`

**Scopes**: `server.view`

**Parámetros de Query**:
| Parámetro | Tipo | Descripción |
|-----------|------|-------------|
| `username` | string | Filtrar por usuario (solo admin) |
| `node` | uint | Filtrar por ID de nodo |
| `name` | string | Filtrar por nombre (soporta `*` como wildcard) |
| `page` | uint | Número de página |
| `limit` | uint | Elementos por página |

**Ejemplo**:
```bash
curl -X GET "http://localhost:8080/api/servers?name=minecraft*&limit=10" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Respuesta**:
```json
{
  "servers": [
    {
      "identifier": "ABC12345",
      "name": "Minecraft Survival",
      "node": {
        "id": 1,
        "name": "Node-01"
      },
      "ip": "192.168.1.100",
      "port": 25565,
      "type": "minecraft-java",
      "canGetStatus": true
    }
  ],
  "metadata": {
    "paging": {
      "page": 1,
      "size": 10,
      "maxSize": 100,
      "total": 1
    }
  }
}
```

---

### Obtener Servidor

**Endpoint**: `GET /api/servers/:serverId`

**Scopes**: `server.view`

**Parámetros de Path**:
- `serverId` (string): ID del servidor

**Parámetros de Query**:
- `perms` (boolean): Incluir permisos del usuario

**Ejemplo**:
```bash
curl -X GET "http://localhost:8080/api/servers/ABC12345?perms=true" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Respuesta**:
```json
{
  "server": {
    "identifier": "ABC12345",
    "name": "Minecraft Survival",
    "node": {
      "id": 1,
      "name": "Node-01"
    },
    "ip": "192.168.1.100",
    "port": 25565,
    "type": "minecraft-java",
    "icon": "minecraft.png"
  },
  "perms": {
    "scopes": [
      "server.view",
      "server.console",
      "server.files.view"
    ]
  }
}
```

---

### Crear Servidor

**Endpoint**: `PUT /api/servers/:serverId`

**Scopes**: `server.create`

**Parámetros de Path**:
- `serverId` (string, opcional): ID personalizado (se genera automáticamente si se omite)

**Body**:
```json
{
  "name": "Mi Servidor Minecraft",
  "nodeId": 1,
  "type": {
    "type": "minecraft-java"
  },
  "icon": "minecraft.png",
  "users": ["admin@example.com"],
  "server": {
    "environment": {
      "type": "standard"
    },
    "install": [
      {
        "type": "mojangdl",
        "version": "1.20.1"
      }
    ],
    "run": {
      "command": "java -Xmx2G -jar server.jar nogui",
      "stop": "stop",
      "pre": [],
      "post": [],
      "environmentVars": {}
    },
    "data": {
      "memory": 2048,
      "cpu": 200,
      "disk": 5000
    }
  },
  "variables": {
    "ip": "0.0.0.0",
    "port": 25565
  }
}
```

**Ejemplo**:
```bash
curl -X PUT "http://localhost:8080/api/servers/MYSERVER" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d @server-create.json
```

**Respuesta**:
```json
{
  "id": "MYSERVER"
}
```

---

### Actualizar Definición de Servidor

**Endpoint**: `PUT /api/servers/:serverId/definition`

**Scopes**: `server.definition.edit`

**Body**:
```json
{
  "name": "Nuevo Nombre",
  "type": {
    "type": "minecraft-java"
  },
  "icon": "nuevo-icono.png",
  "server": {
    "run": {
      "command": "java -Xmx4G -jar server.jar nogui"
    },
    "data": {
      "memory": 4096
    }
  },
  "variables": {
    "port": 25566
  }
}
```

**Respuesta**: `204 No Content`

---

### Eliminar Servidor

**Endpoint**: `DELETE /api/servers/:serverId`

**Scopes**: `server.delete`

**Parámetros de Query**:
- `skipNode` (boolean): No eliminar del nodo, solo de la base de datos

**Ejemplo**:
```bash
curl -X DELETE "http://localhost:8080/api/servers/ABC12345" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Respuesta**: `204 No Content`

---

### Iniciar Servidor

**Endpoint**: `POST /api/servers/:serverId/start`

**Scopes**: `server.start`

**Ejemplo**:
```bash
curl -X POST "http://localhost:8080/api/servers/ABC12345/start" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Respuesta**: `204 No Content`

---

### Detener Servidor

**Endpoint**: `POST /api/servers/:serverId/stop`

**Scopes**: `server.stop`

**Ejemplo**:
```bash
curl -X POST "http://localhost:8080/api/servers/ABC12345/stop" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Respuesta**: `204 No Content`

---

### Reiniciar Servidor

**Endpoint**: `POST /api/servers/:serverId/restart`

**Scopes**: `server.start`, `server.stop`

**Ejemplo**:
```bash
curl -X POST "http://localhost:8080/api/servers/ABC12345/restart" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Respuesta**: `204 No Content`

---

### Matar Servidor (Force Stop)

**Endpoint**: `POST /api/servers/:serverId/kill`

**Scopes**: `server.kill`

**Ejemplo**:
```bash
curl -X POST "http://localhost:8080/api/servers/ABC12345/kill" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Respuesta**: `204 No Content`

---

### Instalar Servidor

**Endpoint**: `POST /api/servers/:serverId/install`

**Scopes**: `server.install`

**Ejemplo**:
```bash
curl -X POST "http://localhost:8080/api/servers/ABC12345/install" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Respuesta**: `204 No Content`

---

### Obtener Estado del Servidor

**Endpoint**: `GET /api/servers/:serverId/status`

**Scopes**: `server.status`

**Respuesta**:
```json
{
  "running": true
}
```

---

### Obtener Estadísticas del Servidor

**Endpoint**: `GET /api/servers/:serverId/stats`

**Scopes**: `server.stats`

**Respuesta**:
```json
{
  "cpu": 45.2,
  "memory": 1536000000,
  "memoryTotal": 2147483648
}
```

---

### Obtener Consola

**Endpoint**: `GET /api/servers/:serverId/console`

**Scopes**: `server.console`

**Parámetros de Query**:
- `time` (int): Timestamp desde el cual obtener logs

**Respuesta**:
```json
{
  "logs": [
    "[10:30:15] [Server thread/INFO]: Starting minecraft server version 1.20.1",
    "[10:30:16] [Server thread/INFO]: Loading properties",
    "[10:30:17] [Server thread/INFO]: Done (2.5s)! For help, type \"help\""
  ]
}
```

---

### Enviar Comando a Consola

**Endpoint**: `POST /api/servers/:serverId/console`

**Scopes**: `server.sendCommand`

**Body**:
```json
{
  "command": "say Hello World!"
}
```

**Respuesta**: `204 No Content`

---

### Gestión de Archivos

#### Listar Archivos

**Endpoint**: `GET /api/servers/:serverId/file/*filename`

**Scopes**: `server.files.view`

**Ejemplo**:
```bash
curl -X GET "http://localhost:8080/api/servers/ABC12345/file/" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Respuesta**:
```json
{
  "files": [
    {
      "name": "server.jar",
      "size": 45678901,
      "modified": "2024-01-15T10:30:00Z",
      "isFile": true
    },
    {
      "name": "world",
      "size": 0,
      "modified": "2024-01-15T10:25:00Z",
      "isFile": false
    }
  ]
}
```

#### Descargar Archivo

**Endpoint**: `GET /api/servers/:serverId/file/*filename`

**Scopes**: `server.files.view`

**Ejemplo**:
```bash
curl -X GET "http://localhost:8080/api/servers/ABC12345/file/server.properties" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o server.properties
```

#### Subir Archivo

**Endpoint**: `PUT /api/servers/:serverId/file/*filename`

**Scopes**: `server.files.edit`

**Ejemplo**:
```bash
curl -X PUT "http://localhost:8080/api/servers/ABC12345/file/config.yml" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/octet-stream" \
  --data-binary @config.yml
```

#### Eliminar Archivo

**Endpoint**: `DELETE /api/servers/:serverId/file/*filename`

**Scopes**: `server.files.edit`

**Ejemplo**:
```bash
curl -X DELETE "http://localhost:8080/api/servers/ABC12345/file/old-backup.zip" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

### Backups

#### Listar Backups

**Endpoint**: `GET /api/servers/:serverId/backup`

**Scopes**: `server.backup.view`

**Respuesta**:
```json
{
  "backups": [
    {
      "id": "backup-20240115-103000",
      "size": 123456789,
      "created": "2024-01-15T10:30:00Z"
    }
  ]
}
```

#### Crear Backup

**Endpoint**: `POST /api/servers/:serverId/backup/create`

**Scopes**: `server.backup.create`

**Respuesta**:
```json
{
  "id": "backup-20240115-120000"
}
```

#### Restaurar Backup

**Endpoint**: `POST /api/servers/:serverId/backup/restore/:backupId`

**Scopes**: `server.backup.restore`

**Respuesta**: `204 No Content`

#### Eliminar Backup

**Endpoint**: `DELETE /api/servers/:serverId/backup/:backupId`

**Scopes**: `server.backup.delete`

**Respuesta**: `204 No Content`

#### Descargar Backup

**Endpoint**: `GET /api/servers/:serverId/backup/download/:backupId`

**Scopes**: `server.backup.view`

**Ejemplo**:
```bash
curl -X GET "http://localhost:8080/api/servers/ABC12345/backup/download/backup-20240115-103000" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o backup.tar.gz
```

---

### Usuarios del Servidor

#### Listar Usuarios

**Endpoint**: `GET /api/servers/:serverId/user`

**Scopes**: `server.users.view`

**Respuesta**:
```json
[
  {
    "username": "admin",
    "email": "admin@example.com",
    "scopes": [
      "server.view",
      "server.console",
      "server.files.view"
    ]
  }
]
```

#### Obtener Usuario Específico

**Endpoint**: `GET /api/servers/:serverId/user/:email`

**Scopes**: `server.users.view`

#### Editar Permisos de Usuario

**Endpoint**: `PUT /api/servers/:serverId/user/:email`

**Scopes**: `server.users.edit`

**Body**:
```json
{
  "scopes": [
    "server.view",
    "server.console",
    "server.files.view",
    "server.files.edit"
  ]
}
```

**Respuesta**: `204 No Content`

#### Eliminar Usuario del Servidor

**Endpoint**: `DELETE /api/servers/:serverId/user/:email`

**Scopes**: `server.users.delete`

**Respuesta**: `204 No Content`

---

## Endpoints de Usuarios

### Listar Usuarios

**Endpoint**: `GET /api/users`

**Scopes**: `users.info.search`

**Parámetros de Query**:
- `username` (string): Filtrar por username (soporta `*`)
- `email` (string): Filtrar por email (soporta `*`)
- `page` (uint): Número de página
- `limit` (uint): Elementos por página

**Ejemplo**:
```bash
curl -X GET "http://localhost:8080/api/users?username=admin*" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Respuesta**:
```json
{
  "users": [
    {
      "id": 1,
      "username": "admin",
      "email": "admin@example.com"
    }
  ],
  "metadata": {
    "paging": {
      "page": 1,
      "size": 25,
      "maxSize": 100,
      "total": 1
    }
  }
}
```

---

### Crear Usuario

**Endpoint**: `POST /api/users`

**Scopes**: `users.info.edit`

**Body**:
```json
{
  "username": "newuser",
  "email": "newuser@example.com",
  "password": "SecurePassword123!"
}
```

**Respuesta**:
```json
{
  "id": 2,
  "username": "newuser",
  "email": "newuser@example.com"
}
```

---

### Obtener Usuario

**Endpoint**: `GET /api/users/:id`

**Scopes**: `users.info.view`

**Respuesta**:
```json
{
  "id": 1,
  "username": "admin",
  "email": "admin@example.com"
}
```

---

### Actualizar Usuario

**Endpoint**: `POST /api/users/:id`

**Scopes**: `users.info.edit`

**Body**:
```json
{
  "username": "newusername",
  "email": "newemail@example.com",
  "password": "NewPassword123!"
}
```

**Respuesta**: `204 No Content`

---

### Eliminar Usuario

**Endpoint**: `DELETE /api/users/:id`

**Scopes**: `users.info.edit`

**Respuesta**: `204 No Content`

---

### Obtener Permisos de Usuario

**Endpoint**: `GET /api/users/:id/perms`

**Scopes**: `users.perms.view`

**Respuesta**:
```json
{
  "scopes": [
    "admin",
    "server.view",
    "server.create"
  ]
}
```

---

### Actualizar Permisos de Usuario

**Endpoint**: `PUT /api/users/:id/perms`

**Scopes**: `users.perms.edit`

**Body**:
```json
{
  "scopes": [
    "server.view",
    "server.create",
    "server.edit"
  ]
}
```

**Respuesta**: `204 No Content`

---

## Endpoints de Nodos

### Listar Nodos

**Endpoint**: `GET /api/nodes`

**Scopes**: `nodes.view`

**Respuesta**:
```json
[
  {
    "id": 1,
    "name": "Node-01",
    "publicHost": "node1.example.com",
    "privateHost": "192.168.1.10",
    "publicPort": 8080,
    "privatePort": 8080,
    "sftpPort": 5657
  }
]
```

---

### Crear Nodo

**Endpoint**: `POST /api/nodes`

**Scopes**: `nodes.create`

**Body**:
```json
{
  "name": "Node-02",
  "publicHost": "node2.example.com",
  "privateHost": "192.168.1.11",
  "publicPort": 8080,
  "privatePort": 8080,
  "sftpPort": 5657
}
```

**Respuesta**:
```json
{
  "id": 2,
  "name": "Node-02",
  "publicHost": "node2.example.com",
  "privateHost": "192.168.1.11",
  "publicPort": 8080,
  "privatePort": 8080,
  "sftpPort": 5657,
  "secret": "abc123def456..."
}
```

---

### Obtener Nodo

**Endpoint**: `GET /api/nodes/:id`

**Scopes**: `nodes.view`

**Respuesta**:
```json
{
  "id": 1,
  "name": "Node-01",
  "publicHost": "node1.example.com",
  "privateHost": "192.168.1.10",
  "publicPort": 8080,
  "privatePort": 8080,
  "sftpPort": 5657
}
```

---

### Actualizar Nodo

**Endpoint**: `PUT /api/nodes/:id`

**Scopes**: `nodes.edit`

**Body**:
```json
{
  "name": "Node-01-Updated",
  "publicHost": "node1-new.example.com"
}
```

**Respuesta**: `204 No Content`

---

### Eliminar Nodo

**Endpoint**: `DELETE /api/nodes/:id`

**Scopes**: `nodes.delete`

**Respuesta**: `204 No Content`

---

### Obtener Información del Sistema del Nodo

**Endpoint**: `GET /api/nodes/:id/system`

**Scopes**: `nodes.view`

**Respuesta**:
```json
{
  "cpu": {
    "model": "Intel(R) Xeon(R) CPU E5-2680 v4",
    "cores": 8,
    "threads": 16,
    "mhz": 2400.0
  },
  "memory": {
    "total": 17179869184,
    "used": 8589934592,
    "free": 8589934592
  },
  "disk": {
    "total": 1099511627776,
    "used": 549755813888,
    "free": 549755813888
  },
  "os": {
    "platform": "linux",
    "family": "debian",
    "version": "22.04"
  }
}
```

---

### Obtener Features del Nodo

**Endpoint**: `GET /api/nodes/:id/features`

**Scopes**: `nodes.view`

**Respuesta**:
```json
{
  "docker": true,
  "environments": [
    "standard",
    "docker"
  ]
}
```

---

### Obtener Datos de Deployment

**Endpoint**: `GET /api/nodes/:id/deployment`

**Scopes**: `nodes.deploy`

**Respuesta**:
```json
{
  "clientId": ".node_1",
  "clientSecret": "abc123def456..."
}
```

---

## Endpoints de Configuración

### Obtener Configuración

**Endpoint**: `GET /api/settings`

**Scopes**: `settings.view`

**Respuesta**:
```json
{
  "companyName": "SkyPanel",
  "defaultTheme": "SkyPanel",
  "masterUrl": "https://panel.example.com",
  "registrationEnabled": false
}
```

---

### Actualizar Configuración

**Endpoint**: `PUT /api/settings`

**Scopes**: `settings.edit`

**Body**:
```json
{
  "companyName": "Mi Empresa",
  "registrationEnabled": true
}
```

**Respuesta**: `204 No Content`

---

## Endpoints de Plantillas

### Listar Plantillas

**Endpoint**: `GET /api/templates`

**Scopes**: `templates.view`

**Respuesta**:
```json
{
  "templates": [
    {
      "name": "minecraft-java",
      "display": "Minecraft Java Edition",
      "type": "java",
      "supportedVersions": ["1.20.1", "1.19.4", "1.18.2"]
    },
    {
      "name": "terraria",
      "display": "Terraria",
      "type": "native",
      "supportedVersions": ["1.4.4.9"]
    }
  ]
}
```

---

### Obtener Plantilla

**Endpoint**: `GET /api/templates/:name`

**Scopes**: `templates.view`

**Respuesta**:
```json
{
  "name": "minecraft-java",
  "display": "Minecraft Java Edition",
  "type": "java",
  "install": [
    {
      "type": "mojangdl",
      "version": "{{version}}"
    }
  ],
  "run": {
    "command": "java -Xmx{{memory}}M -jar server.jar nogui",
    "stop": "stop"
  },
  "variables": {
    "version": {
      "type": "string",
      "default": "1.20.1",
      "required": true
    },
    "memory": {
      "type": "integer",
      "default": 2048,
      "required": true
    }
  }
}
```

---

## WebSocket API

### Conectar a Consola en Tiempo Real

**Endpoint**: `WS /api/servers/:serverId/socket`

**Scopes**: `server.view`

**Protocolo**: WebSocket

**Ejemplo con JavaScript**:
```javascript
const token = 'YOUR_ACCESS_TOKEN';
const serverId = 'ABC12345';
const ws = new WebSocket(`ws://localhost:8080/api/servers/${serverId}/socket`);

// Autenticación
ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'auth',
    token: token
  }));
};

// Recibir mensajes
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  
  if (data.type === 'console') {
    console.log('Console:', data.data);
  } else if (data.type === 'stats') {
    console.log('Stats:', data.data);
  } else if (data.type === 'status') {
    console.log('Status:', data.data);
  }
};

// Enviar comando
function sendCommand(command) {
  ws.send(JSON.stringify({
    type: 'console',
    data: command
  }));
}

// Ejemplo de uso
sendCommand('say Hello from WebSocket!');
```

### Tipos de Mensajes WebSocket

#### Autenticación
```json
{
  "type": "auth",
  "token": "YOUR_ACCESS_TOKEN"
}
```

#### Consola (Servidor → Cliente)
```json
{
  "type": "console",
  "data": "[10:30:15] [Server thread/INFO]: Starting server"
}
```

#### Comando (Cliente → Servidor)
```json
{
  "type": "console",
  "data": "say Hello World!"
}
```

#### Estadísticas (Servidor → Cliente)
```json
{
  "type": "stats",
  "data": {
    "cpu": 45.2,
    "memory": 1536000000
  }
}
```

#### Estado (Servidor → Cliente)
```json
{
  "type": "status",
  "data": {
    "running": true
  }
}
```

---

## Ejemplos de Uso

### Python

#### Instalación
```bash
pip install requests
```

#### Ejemplo Completo
```python
import requests
import json

class SkyPanelAPI:
    def __init__(self, base_url, client_id, client_secret):
        self.base_url = base_url
        self.client_id = client_id
        self.client_secret = client_secret
        self.token = None
    
    def authenticate(self):
        """Obtener token de acceso"""
        url = f"{self.base_url}/oauth2/token"
        data = {
            'grant_type': 'client_credentials',
            'client_id': self.client_id,
            'client_secret': self.client_secret
        }
        response = requests.post(url, data=data)
        response.raise_for_status()
        self.token = response.json()['access_token']
        return self.token
    
    def get_headers(self):
        """Headers con autenticación"""
        if not self.token:
            self.authenticate()
        return {
            'Authorization': f'Bearer {self.token}',
            'Content-Type': 'application/json'
        }
    
    def list_servers(self, **filters):
        """Listar servidores"""
        url = f"{self.base_url}/api/servers"
        response = requests.get(url, headers=self.get_headers(), params=filters)
        response.raise_for_status()
        return response.json()
    
    def get_server(self, server_id):
        """Obtener servidor específico"""
        url = f"{self.base_url}/api/servers/{server_id}"
        response = requests.get(url, headers=self.get_headers())
        response.raise_for_status()
        return response.json()
    
    def start_server(self, server_id):
        """Iniciar servidor"""
        url = f"{self.base_url}/api/servers/{server_id}/start"
        response = requests.post(url, headers=self.get_headers())
        response.raise_for_status()
        return True
    
    def stop_server(self, server_id):
        """Detener servidor"""
        url = f"{self.base_url}/api/servers/{server_id}/stop"
        response = requests.post(url, headers=self.get_headers())
        response.raise_for_status()
        return True
    
    def send_command(self, server_id, command):
        """Enviar comando a consola"""
        url = f"{self.base_url}/api/servers/{server_id}/console"
        data = {'command': command}
        response = requests.post(url, headers=self.get_headers(), json=data)
        response.raise_for_status()
        return True
    
    def create_server(self, server_data):
        """Crear servidor"""
        server_id = server_data.get('identifier', '')
        url = f"{self.base_url}/api/servers/{server_id}"
        response = requests.put(url, headers=self.get_headers(), json=server_data)
        response.raise_for_status()
        return response.json()

# Uso
api = SkyPanelAPI(
    base_url='http://localhost:8080',
    client_id='YOUR_CLIENT_ID',
    client_secret='YOUR_CLIENT_SECRET'
)

# Listar servidores
servers = api.list_servers(name='minecraft*')
print(f"Encontrados {len(servers['servers'])} servidores")

# Iniciar servidor
api.start_server('ABC12345')
print("Servidor iniciado")

# Enviar comando
api.send_command('ABC12345', 'say Hello from Python!')
print("Comando enviado")
```

---

### Node.js

#### Instalación
```bash
npm install axios
```

#### Ejemplo Completo
```javascript
const axios = require('axios');

class SkyPanelAPI {
  constructor(baseURL, clientId, clientSecret) {
    this.baseURL = baseURL;
    this.clientId = clientId;
    this.clientSecret = clientSecret;
    this.token = null;
    
    this.client = axios.create({
      baseURL: this.baseURL
    });
  }
  
  async authenticate() {
    const response = await this.client.post('/oauth2/token', 
      new URLSearchParams({
        grant_type: 'client_credentials',
        client_id: this.clientId,
        client_secret: this.clientSecret
      }),
      {
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded'
        }
      }
    );
    
    this.token = response.data.access_token;
    return this.token;
  }
  
  getHeaders() {
    if (!this.token) {
      throw new Error('Not authenticated. Call authenticate() first.');
    }
    
    return {
      'Authorization': `Bearer ${this.token}`,
      'Content-Type': 'application/json'
    };
  }
  
  async listServers(filters = {}) {
    const response = await this.client.get('/api/servers', {
      headers: this.getHeaders(),
      params: filters
    });
    return response.data;
  }
  
  async getServer(serverId) {
    const response = await this.client.get(`/api/servers/${serverId}`, {
      headers: this.getHeaders()
    });
    return response.data;
  }
  
  async startServer(serverId) {
    await this.client.post(`/api/servers/${serverId}/start`, {}, {
      headers: this.getHeaders()
    });
    return true;
  }
  
  async stopServer(serverId) {
    await this.client.post(`/api/servers/${serverId}/stop`, {}, {
      headers: this.getHeaders()
    });
    return true;
  }
  
  async sendCommand(serverId, command) {
    await this.client.post(`/api/servers/${serverId}/console`, 
      { command },
      { headers: this.getHeaders() }
    );
    return true;
  }
  
  async createServer(serverData) {
    const serverId = serverData.identifier || '';
    const response = await this.client.put(`/api/servers/${serverId}`, 
      serverData,
      { headers: this.getHeaders() }
    );
    return response.data;
  }
}

// Uso
(async () => {
  const api = new SkyPanelAPI(
    'http://localhost:8080',
    'YOUR_CLIENT_ID',
    'YOUR_CLIENT_SECRET'
  );
  
  // Autenticar
  await api.authenticate();
  console.log('Autenticado');
  
  // Listar servidores
  const servers = await api.listServers({ name: 'minecraft*' });
  console.log(`Encontrados ${servers.servers.length} servidores`);
  
  // Iniciar servidor
  await api.startServer('ABC12345');
  console.log('Servidor iniciado');
  
  // Enviar comando
  await api.sendCommand('ABC12345', 'say Hello from Node.js!');
  console.log('Comando enviado');
})();
```

---

### cURL

#### Autenticación
```bash
# Obtener token
TOKEN=$(curl -X POST http://localhost:8080/oauth2/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  | jq -r '.access_token')

echo "Token: $TOKEN"
```

#### Listar Servidores
```bash
curl -X GET "http://localhost:8080/api/servers" \
  -H "Authorization: Bearer $TOKEN" \
  | jq '.'
```

#### Crear Servidor
```bash
curl -X PUT "http://localhost:8080/api/servers/MYSERVER" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Mi Servidor",
    "nodeId": 1,
    "type": {"type": "minecraft-java"},
    "users": ["admin@example.com"],
    "server": {
      "environment": {"type": "standard"},
      "install": [{"type": "mojangdl", "version": "1.20.1"}],
      "run": {
        "command": "java -Xmx2G -jar server.jar nogui",
        "stop": "stop"
      },
      "data": {"memory": 2048, "cpu": 200, "disk": 5000}
    },
    "variables": {"ip": "0.0.0.0", "port": 25565}
  }'
```

#### Iniciar Servidor
```bash
curl -X POST "http://localhost:8080/api/servers/ABC12345/start" \
  -H "Authorization: Bearer $TOKEN"
```

#### Enviar Comando
```bash
curl -X POST "http://localhost:8080/api/servers/ABC12345/console" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"command": "say Hello from cURL!"}'
```

---

## SDKs y Librerías

### Oficiales

- **Python**: `pip install skypanel-api` (próximamente)
- **Node.js**: `npm install skypanel-api` (próximamente)
- **Go**: `go get github.com/SkyPanel/skypanel-go` (próximamente)

### Comunitarias

- **PHP**: [skypanel-php](https://github.com/community/skypanel-php)
- **Ruby**: [skypanel-ruby](https://github.com/community/skypanel-ruby)
- **Java**: [skypanel-java](https://github.com/community/skypanel-java)

---

## Rate Limiting

La API de SkyPanel implementa rate limiting para prevenir abuso:

- **Límite**: 100 peticiones por minuto por token
- **Header de Respuesta**: `X-RateLimit-Remaining`
- **Código de Error**: `429 Too Many Requests`

**Ejemplo de Respuesta**:
```http
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1642345678

{
  "error": {
    "code": "ErrRateLimitExceeded",
    "msg": "Rate limit exceeded. Try again in {seconds} seconds",
    "metadata": {
      "seconds": 45
    }
  }
}
```

---

## Versionado de API

La API de SkyPanel usa versionado semántico:

- **Versión Actual**: `v3`
- **URL**: `/api/...` (v3 es la versión por defecto)
- **Versiones Antiguas**: `/api/v2/...` (deprecado)

### Política de Deprecación

- Las versiones antiguas se mantienen por **6 meses** después de una nueva versión mayor
- Se notifica con **3 meses** de anticipación antes de eliminar una versión
- Header de deprecación: `X-API-Deprecated: true`

---

## Mejores Prácticas

### 1. Cachear Tokens
```python
# ❌ Malo: Autenticar en cada petición
def get_servers():
    token = authenticate()
    return requests.get('/api/servers', headers={'Authorization': f'Bearer {token}'})

# ✅ Bueno: Reutilizar token
class API:
    def __init__(self):
        self.token = None
        self.token_expires = 0
    
    def get_token(self):
        if time.time() > self.token_expires:
            self.token = authenticate()
            self.token_expires = time.time() + 3600
        return self.token
```

### 2. Manejo de Errores
```python
try:
    response = api.start_server('ABC12345')
except requests.exceptions.HTTPError as e:
    if e.response.status_code == 404:
        print("Servidor no encontrado")
    elif e.response.status_code == 403:
        print("Sin permisos")
    else:
        print(f"Error: {e.response.json()}")
```

### 3. Paginación
```python
def get_all_servers():
    page = 1
    all_servers = []
    
    while True:
        response = api.list_servers(page=page, limit=100)
        all_servers.extend(response['servers'])
        
        if page * 100 >= response['metadata']['paging']['total']:
            break
        
        page += 1
    
    return all_servers
```

### 4. Rate Limiting
```python
import time

def make_request_with_retry(func, *args, **kwargs):
    max_retries = 3
    retry_delay = 1
    
    for attempt in range(max_retries):
        try:
            return func(*args, **kwargs)
        except requests.exceptions.HTTPError as e:
            if e.response.status_code == 429:
                if attempt < max_retries - 1:
                    time.sleep(retry_delay * (2 ** attempt))
                    continue
            raise
```

---

## Soporte y Ayuda

### Recursos

- 📖 **Documentación**: [docs.skypanel.com](https://docs.skypanel.com)
- 💬 **Discord**: [discord.gg/skypanel](https://discord.gg/skypanel)
- 🐛 **Issues**: [github.com/SkyPanel/SkyPanel/issues](https://github.com/SkyPanel/SkyPanel/issues)
- 📧 **Email**: api-support@skypanel.com

### Reportar Problemas de API

Al reportar problemas, incluye:

1. **Endpoint** afectado
2. **Método HTTP** usado
3. **Headers** enviados (sin tokens)
4. **Body** de la petición
5. **Respuesta** recibida
6. **Código de error**
7. **Versión de SkyPanel**

---

**¡Feliz desarrollo con la API de SkyPanel! 🚀**
