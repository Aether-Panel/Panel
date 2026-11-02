# Referencia de la API

Aether Panel se maneja mediante una API HTTP: la usa el cliente web y pueden usarla también scripts, bots y sistemas de facturación. Esta página es el punto de entrada. Cada área tiene su propia página, con una tabla de rutas al principio y los detalles (cuerpo, parámetros de consulta, respuesta y errores) a continuación.

> El producto se llama Aether Panel. El módulo de Go, el binario y algunos identificadores (la cookie `skypanel_auth`, las variables `SKYPANEL_*`) siguen usando el nombre SkyPanel.

Todo lo que aparece aquí se ha comprobado con las rutas que registra el código (`internal/web/api`, `internal/web/auth`, `internal/web/oauth2`, `internal/web/daemon`). Cuando la página Swagger generada (`/swagger/index.html`, solo para administradores) no coincide con esta referencia, la correcta es esta referencia: varias anotaciones se corrigieron en el código, pero los archivos de Swagger que se sirven aún no se han regenerado, y algunas siguen nombrando scopes que no existen (véase [Convenciones → Swagger](./api/01-conventions.md#swagger)).

## Páginas

| Página | Contenido |
|--------|-----------|
| [Convenciones](./api/01-conventions.md) | Rutas base, formato de peticiones y respuestas, errores, códigos de estado, paginación, cabeceras, CORS, límites de frecuencia |
| [Autenticación](./api/02-authentication.md) | Cookie de sesión, tokens bearer, clientes OAuth2, claves de API, la protección de mismo origen, scopes, `/auth/*`, `/oauth2/token`, `/api/config` |
| [Servidores](./api/03-servers.md) | Lista de servidores, creación, eliminación, encendido, consola, estadísticas, definición, variables, puertos, usuarios, actividad, subdominio, clonación, traslado, IA, WebSocket |
| [Contenido del servidor](./api/04-server-content.md) | Archivos, archivos comprimidos, historial de archivos, plugins, modpacks, mapa del mundo, copias de seguridad, tareas, bases de datos |
| [Cuentas y usuarios](./api/05-accounts.md) | La cuenta propia (perfil, 2FA, clientes OAuth2, claves SSH), ajustes de usuario, usuarios, roles, actividad de acceso |
| [Administración del panel](./api/06-panel-admin.md) | Nodos, configuración, claves de API, plantillas, versiones de juego, hosts de bases de datos, extensiones, disponibilidad |
| [Aprovisionamiento / facturación](./api/07-provisioning.md) | Productos y las rutas `/api/v1` para WHMCS, Paymenter y similares |
| [Transferencia externa](./api/08-external-transfer.md) | Copia de los archivos de un servidor desde otro Aether Panel |
| [Daemon](./api/09-daemon.md) | Las rutas `/daemon` a las que llama el panel en cada nodo |

## Inicio rápido

Cree un cliente OAuth2 en **Configuración de la cuenta → OAuth** (o con `POST /api/self/oauth2`), cámbielo por un token y envíe el token en una cabecera bearer. El cliente actúa con todos los permisos de la cuenta:

```bash
TOKEN=$(curl -s -X POST https://panel.example.com/oauth2/token \
  -d grant_type=client_credentials \
  -d client_id=CLIENT_ID \
  -d client_secret=CLIENT_SECRET | jq -r .access_token)

curl -s -H "Authorization: Bearer $TOKEN" https://panel.example.com/api/servers
curl -s -X POST -H "Authorization: Bearer $TOKEN" https://panel.example.com/api/servers/a1b2c3d4/start
```

A las peticiones con token bearer no les afecta la [protección de mismo origen](./api/02-authentication.md#protección-de-mismo-origen); a las que llevan las cookies del panel, sí.

## Mapa de rutas

Cada área enlaza con la página que la documenta.

### Autenticación

`/auth/login`, `/auth/otp`, `/auth/logout`, `/auth/register`, `/auth/forgot-password`, `/auth/reset-password`, `/auth/reauth`, `/auth/sso`, `/auth/challenge`, `/auth/publickey`, `/oauth2/token` y la ruta pública `GET /api/config`. Véase [Autenticación](./api/02-authentication.md).

### Servidores

`/api/servers` y `/api/servers/:serverId/...`: lista, creación, eliminación, suspensión, cambio de nombre, definición, variables, ajustes de puertos, flags, acciones de encendido, consola, estadísticas, estado, consulta del juego, métricas, informes de caída, versión del juego, notificaciones, usuarios y propietario, registro de actividad, subdominio, clonación, traslado de nodo, IA. Véase [Servidores](./api/03-servers.md).

### WebSocket

`GET /api/servers/:serverId/socket` transmite la consola, las estadísticas y el estado. Véase [Servidores → WebSocket](./api/03-servers.md#websocket).

### Archivos

`/api/servers/:serverId/file/*`, `archive/*`, `extract/*` y `file-history`. Véase [Contenido del servidor → Archivos](./api/04-server-content.md#archivos).

### Plugins y modpacks

`/api/servers/:serverId/plugins` y `/modpacks`. Véase [Contenido del servidor](./api/04-server-content.md#plugins).

### Mapa del mundo

`/api/servers/:serverId/map` y el visor firmado `/api/map-view/...`. Véase [Contenido del servidor → Mapa del mundo](./api/04-server-content.md#mapa-del-mundo).

### Copias de seguridad

`/api/servers/:serverId/backup/...`: lista, creación (`409 ErrBackupLimit` al superar `panel.backups.maxPerServer`), restauración, descarga, eliminación. Véase [Contenido del servidor → Copias de seguridad](./api/04-server-content.md#copias-de-seguridad).

### Tareas

`/api/servers/:serverId/tasks/...`. Véase [Contenido del servidor → Tareas](./api/04-server-content.md#tareas).

### Bases de datos del servidor

`/api/servers/:serverId/databases` y `/databasehosts`. Véase [Contenido del servidor → Bases de datos](./api/04-server-content.md#bases-de-datos).

### Cuenta

`/api/self/...` (perfil, 2FA, clientes OAuth2, claves SSH) y `/api/userSettings`. Véase [Cuentas y usuarios](./api/05-accounts.md).

### Usuarios y roles

`/api/users`, `/api/roles` y el registro de acceso `/api/activity`. Véase [Cuentas y usuarios](./api/05-accounts.md#usuarios).

### Nodos

`/api/nodes/...`, incluidas la comprobación de puertos y el historial de red. Véase [Administración del panel → Nodos](./api/06-panel-admin.md#nodos).

### Configuración

`/api/settings/...`: valores, pruebas (correo, Discord, S3, Cloudflare), licencia, actualizaciones, estado del dominio. Véase [Administración del panel → Configuración](./api/06-panel-admin.md#configuración).

### Claves de API

`/api/settings/apikeys`. Véase [Administración del panel → Claves de API](./api/06-panel-admin.md#claves-de-api).

### Plantillas

`/api/templates/...` y `/api/game-versions/:platform`. Véase [Administración del panel → Plantillas](./api/06-panel-admin.md#plantillas).

### Hosts de bases de datos

`/api/databasehosts/...`. Véase [Administración del panel → Hosts de bases de datos](./api/06-panel-admin.md#hosts-de-bases-de-datos).

### Extensiones

`/api/extensions/...` y `/api/extension-files/...`. Véase [Administración del panel → Extensiones](./api/06-panel-admin.md#extensiones).

### Disponibilidad

`/api/uptime` y `/api/uptime/:serverId`. Véase [Administración del panel → Disponibilidad](./api/06-panel-admin.md#disponibilidad).

### Aprovisionamiento / facturación

`/api/provision/products` y `/api/v1/*`. Véase [Aprovisionamiento / facturación](./api/07-provisioning.md).

### Transferencia externa

`/api/servers/:serverId/extransfer/*` y la ruta pública `/api/extransfer/*`. Véase [Transferencia externa](./api/08-external-transfer.md).

### Daemon

`/daemon`, `/daemon/features`, `/daemon/system`, `/daemon/ports`, `/daemon/network/history`, `/daemon/update`, `/daemon/update-status` y `/daemon/server/:serverId/...`. Véase [Daemon](./api/09-daemon.md).

## Documentación relacionada

La versión anterior de esta página, en un único archivo, recogía también detalles internos que no forman parte de la API HTTP. Ahora están documentados aquí:

- Claves de configuración y variables de entorno: [Configuración](../backend/02-configuration.md)
- Línea de comandos: [Referencia de comandos CLI](./14-cli-commands.md)
- Tablas de la base de datos: [Base de datos](../backend/03-database.md)
- Definiciones de servidor, entornos, planificador, límite de disco: [Servidores](../backend/04-servers.md)
- Middleware y detalles internos del enrutado: [Capa API](../backend/05-api-layer.md)
- Detalles internos del WebSocket (trackers, búfer de consola): [WebSocket](../backend/08-websocket.md)
- Detalles internos de JWT, SFTP y claves de API: [Seguridad: permisos, tokens, SFTP y protecciones HTTP](../backend/09-security.md)
- Cómo llama a la API el cliente web: [Consumo de la API desde el frontend](../frontend/02-endpoints.md)
- Nombres de los permisos y quién los recibe: [Referencia de permisos](../general/scopes-reference.md)
- Paquetes y eventos de las extensiones: [Extensiones](./15-extensions.md)
