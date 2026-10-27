# Referencia de permisos (scopes)

Un **scope** es un permiso con nombre, por ejemplo `server.start`. Cada ruta de la API pide uno, y la petición pasa solo si el usuario lo tiene. La lista completa está en `internal/scopes/scopes.go`: 73 scopes, 23 del panel y 50 de servidor.

- **Scopes del panel**: valen para todo el panel (`nodes.view`, `settings.edit`…).
- **Scopes de servidor**: valen para un servidor concreto. La ruta tiene que llevar `:serverId`; sin él, la petición se rechaza.

Dos scopes cubren a otros: `admin` cubre todos, y `server.admin` cubre todos los de servidor en ese servidor.

Las claves API de facturación no usan scopes: tienen su propia lista de acciones (ver [Claves API](#claves-api)).

## De dónde salen los permisos

En cada petición el panel junta los permisos del usuario de estas fuentes:

| Fuente | En rutas del panel | En rutas de un servidor |
|--------|--------------------|-------------------------|
| Rol del usuario | Cuenta | Solo cuentan sus scopes del panel (por ejemplo `admin`). Sus scopes de servidor **no cuentan**. |
| Fila global del usuario (permisos sin servidor) | Cuenta | Cuenta |
| Ser propietario del servidor | — | Da el conjunto del propietario (ver abajo) |
| Fila del usuario en ese servidor | — | Cuenta |

Si la sesión viene de un cliente OAuth2, el cliente pone otro límite encima: un cliente ligado a un servidor solo actúa sobre ese servidor, y un cliente con scopes declarados nunca pasa de ellos.

## Propietario y subusuarios

Cada servidor tiene un **propietario** (`servers.owner_id`): el cliente al que pertenece. El resto de personas con acceso son **subusuarios**.

### Quién es el propietario de un servidor nuevo

| Cómo se crea | Propietario |
|--------------|-------------|
| Desde la facturación (`POST /api/v1/provision`) | El cliente para el que se compró |
| Sub-servidor (Splitter) | El propietario del servidor padre |
| Copia de un servidor (clonar) | El propietario del original |
| Un administrador lo crea para otros usuarios | El primer usuario de la lista |
| Un administrador lo crea sin usuarios | Nadie: es un servidor del administrador |

El propietario siempre es también miembro del servidor (tiene una fila de permisos en él), así que el servidor aparece en su lista.

### Qué puede hacer el propietario

El propietario tiene en su servidor el conjunto `scopes.OwnerScopes`, diga lo que diga su fila:

| Grupo | Scopes |
|-------|--------|
| Ver | `server.view`, `server.status`, `server.stats` |
| Consola | `server.console`, `server.console.send` |
| Encendido e instalación | `server.start`, `server.stop`, `server.kill`, `server.reload`, `server.install` |
| Archivos | `server.files.view`, `server.files.edit`, `server.sftp` |
| Copias de seguridad | `server.backup.view`, `server.backup.create`, `server.backup.restore`, `server.backup.delete` |
| Configuración | `server.data.view`, `server.data.edit`, `server.flags.view`, `server.flags.edit`, `server.name.edit`, `server.definition.view` |
| Tareas | `server.tasks.view`, `server.tasks.run`, `server.tasks.create`, `server.tasks.edit`, `server.tasks.delete` |
| Usuarios y clientes | `server.users.view`, `server.users.create`, `server.users.edit`, `server.users.delete`, `server.clients.view`, `server.clients.create`, `server.clients.edit`, `server.clients.delete` |

Además, solo el propietario (o un administrador) puede **dividir** el servidor con Splitter: dividir saca recursos de su plan.

Lo que queda fuera es decisión del administrador:

- Borrar el servidor (`server.delete`).
- Recursos y variables de administrador, suspender y transferir (`server.data.edit.admin`).
- Editar la definición y el entorno (`server.definition.edit`).
- Puertos (`server.ports.edit`) y la sección de administración (`server.admin.*`).

Al propietario no se le pueden quitar permisos ni sacar del servidor: la API responde `409 ErrServerOwner`.

### Cambiar el propietario

Solo un administrador. En la pestaña **Usuarios** del servidor, cada subusuario tiene el botón **Hacer propietario** (solo lo ven los administradores). Por la API:

```
PUT /api/servers/:serverId/owner/:email
```

Requiere `admin`. Si el usuario no era miembro, pasa a serlo con `server.view`. El propietario anterior se queda como subusuario con los permisos que ya tenía en su fila. El propietario no puede ceder el servidor por su cuenta.

### Subusuarios

Un subusuario tiene **exactamente** los scopes de su fila en ese servidor. Los scopes de servidor de su rol no cuentan: antes, el rol "Usuario" daba a cualquier miembro control total, aunque el propietario solo le hubiera dado "ver".

- Añade y edita subusuarios quien tenga `server.users.edit` en el servidor (el propietario lo tiene). Las dos cosas usan la misma ruta: `PUT /api/servers/:serverId/user/:email`.
- Quita subusuarios quien tenga `server.users.delete`.
- Nadie da más de lo que tiene: quien edita solo puede poner o quitar los scopes que él mismo tiene en ese servidor. Los demás scopes de la fila se quedan como estaban. Las excepciones son quien tiene `server.admin` (en el servidor o en su fila global) o `admin` en su fila global: esos pueden poner cualquiera.
- Si el correo no tiene cuenta, el panel crea una cuenta invitada (con permiso para iniciar sesión) y le manda un correo con el enlace para elegir su contraseña (`/forgot-password/`).

### El editor de permisos

En la pestaña **Usuarios** del servidor, el propietario ve la lista de miembros. El propietario sale primero, con la etiqueta **Propietario** y sin botones. Cada subusuario tiene **Permisos** (editar), **Revocar** y, para administradores, **Hacer propietario**.

Al añadir o editar, los permisos se marcan por grupos. Cada grupo tiene una casilla que marca o desmarca todo el grupo:

| Grupo | Scopes |
|-------|--------|
| Consola | `server.console`, `server.console.send`, `server.stats`, `server.status` |
| Encendido e instalación | `server.start`, `server.stop`, `server.kill`, `server.reload`, `server.install` |
| Archivos y SFTP | `server.files.view`, `server.files.edit`, `server.sftp` |
| Copias de seguridad | `server.backup.view`, `server.backup.create`, `server.backup.restore`, `server.backup.delete` |
| Configuración | `server.data.view`, `server.data.edit`, `server.flags.view`, `server.flags.edit`, `server.name.edit`, `server.definition.view` |
| Tareas programadas | `server.tasks.view`, `server.tasks.run`, `server.tasks.create`, `server.tasks.edit`, `server.tasks.delete` |
| Usuarios y clientes | `server.users.view`, `server.users.create`, `server.users.edit`, `server.users.delete`, `server.clients.view`, `server.clients.create`, `server.clients.edit`, `server.clients.delete` |

Son los mismos scopes que tiene el propietario. Un subusuario nuevo empieza con `server.console`, `server.stats`, `server.status`, `server.start` y `server.stop`. La web siempre añade `server.view`: sin él el servidor no aparece en su lista.

### Qué ve cada uno en la web

La página del servidor pide `GET /api/servers/:serverId?perms`. La respuesta trae los scopes que el usuario tiene en ese servidor y `owner: true` si es el propietario. Con eso la web solo muestra lo que el usuario puede usar:

| Pestaña o acción | Se muestra con |
|------------------|----------------|
| Consola | `server.console` |
| Escribir en la consola | `server.console.send` (sin él, la consola es de solo lectura) |
| Iniciar, Reiniciar, Detener, Forzar | `server.start`, `server.start` + `server.stop`, `server.stop`, `server.kill` |
| Archivos | `server.files.view` |
| Configuración | `server.data.view` o `server.flags.view` |
| Usuarios | `server.users.view` |
| Base de datos | `server.data.edit` |
| Copias de seguridad | `server.backup.view` |
| Tareas | `server.tasks.view` |
| SFTP | `server.sftp` |
| Transferencia externa | `server.data.edit.admin` |
| Splitter | Ser propietario o `server.data.edit.admin` |
| Plugins | `server.files.edit` |

`admin` o `server.admin` muestran todo. Ocultar es solo comodidad: la API comprueba cada petición igualmente.

### Entorno y definición

El entorno decide qué puede tocar el contenedor en el nodo (carpetas del host, modo privilegiado, ejecutar en el host). Por eso:

- Editar la definición (`PUT /api/servers/:serverId/definition`) pide `server.definition.edit`, que el propietario no tiene.
- Aunque alguien tenga `server.definition.edit`, si no tiene también `server.data.edit.admin` el entorno se queda como está. En un entorno `docker` solo puede cambiar `image`, `portBindings`, `containerRoot`, `config` y `labels`. No puede cambiar el tipo de entorno.
- Quien no es administrador solo puede crear sub-servidores: de un servidor del que es propietario, en `docker` y con esas mismas claves. `bindings`, `hostConfig` y `networkName` se descartan.

### Sub-servidores y copias

- Un sub-servidor pertenece al propietario del padre (si el padre no tiene propietario, el hijo tampoco). Los miembros del padre pasan al hijo con exactamente los permisos que tenían en el padre.
- Una copia pertenece al propietario del original y copia las filas de permisos de sus usuarios tal cual.

### Servidores que ya existían

Al actualizar, la migración `20261026-server-owners` nombra un propietario para cada servidor que no tiene: el miembro con más scopes en ese servidor y, si hay empate, el que se añadió primero. Los clientes de la facturación tienen el conjunto completo, así que suelen ser ellos.

Un servidor sin miembros se queda sin propietario. Después de actualizar, revisa en la pestaña **Usuarios** que cada servidor tenga el propietario correcto, y cámbialo con **Hacer propietario** si hace falta.

## Roles

Un rol da scopes del panel a todos sus usuarios. Sus scopes de servidor no cuentan en ningún servidor: lo que un usuario puede hacer en un servidor sale de ser su propietario o de su fila en él.

Roles que crea el panel:

| Rol | Scopes del panel |
|-----|------------------|
| Administrador | `admin` |
| Usuario | `login`, `self.edit`, `self.clients`, `server.create`, `templates.view`, `uptime.view` |

El rol "Usuario" todavía guarda scopes de servidor de versiones anteriores; ya no tienen efecto en los servidores. `server.create` le deja dividir sus propios servidores; crear un servidor sin padre es solo para administradores.

## Claves API

Las claves API son para sistemas de facturación (WHMCS, Paymenter…). Solo sirven en `/api/v1` y cada clave solo hace las acciones que se le dieron.

Las crea un administrador en **Ajustes → Claves API**. Al crearla se marca **Qué puede hacer**; por defecto vienen todas marcadas. La clave completa (`ak_...`) se muestra una sola vez; el panel guarda solo su hash. Para cambiar las acciones de una clave, crea otra y revoca la antigua: una clave no se puede editar.

La clave va en la cabecera `X-Api-Key: ak_...` o en `Authorization: Bearer ak_...`.

| Acción | Rutas | En Ajustes |
|--------|-------|------------|
| `create` | `POST /api/v1/provision` | Crear servidores |
| `terminate` | `POST /api/v1/terminate` | Dar de baja |
| `suspend` | `POST /api/v1/suspend`, `POST /api/v1/unsuspend` | Suspender y reactivar |
| `change-package` | `POST /api/v1/change-package` | Cambiar de plan |
| `sso` | `POST /api/v1/sso` | Acceso directo |

- `GET /api/v1/ping` no pide ninguna acción.
- Una ruta que la clave no tiene responde `403 ErrAPIKeyNotAllowed`.
- Las claves creadas antes de esto guardan `provision` (o ninguna lista). Eso significa todas las acciones, para que las integraciones existentes sigan funcionando. En Ajustes aparecen como **Todas las acciones**.

Por la API (requiere `admin`):

```
POST /api/settings/apikeys
{ "name": "WHMCS", "permissions": ["create", "suspend"] }
```

Sin `permissions`, la clave recibe `provision` (todas las acciones). Los nombres que no se conocen o repetidos se descartan. Si no queda ninguno conocido, responde `400 ErrAPIKeyPermissions`.

Límites de `sso` aunque la clave lo tenga:

- El enlace solo lleva a un servidor que el cliente puede abrir (`server.view`).
- Se rechaza para cuentas con poderes en el panel, directos o por su rol: `admin`, `settings.edit`, `oauth2.auth` y los scopes que cambian nodos, plantillas, usuarios o sus permisos. Así una clave filtrada no abre la cuenta de un administrador.

## Scopes de servidor (50)

La columna **Propietario** dice si está en el conjunto del propietario.

### Ver y encendido

| Scope | Qué permite | Propietario |
|-------|-------------|-------------|
| `server.view` | Ver el servidor, su versión, su mapa y la lista de bases de datos. Conectar al socket. | sí |
| `server.status` | Ver si está encendido. | sí |
| `server.stats` | Estadísticas, métricas y consulta (`query`). | sí |
| `server.console` | Ver la consola, el historial de caídas y usar el asistente de IA. | sí |
| `server.console.send` | Enviar comandos a la consola. | sí |
| `server.start` | Iniciar. Reiniciar pide `server.start` y `server.stop`. | sí |
| `server.stop` | Detener. | sí |
| `server.kill` | Forzar la detención. | sí |
| `server.reload` | Recargar la configuración. | sí |
| `server.install` | Instalar o reinstalar, instalar modpacks y cambiar de versión. | sí |

### Archivos y copias

| Scope | Qué permite | Propietario |
|-------|-------------|-------------|
| `server.files.view` | Listar y leer archivos, su historial, plugins y modpacks. | sí |
| `server.files.edit` | Subir, editar, borrar, comprimir y extraer archivos; restaurar versiones; instalar o quitar plugins y el mapa. | sí |
| `server.sftp` | Entrar por SFTP. | sí |
| `server.backup.view` | Ver y descargar copias. | sí |
| `server.backup.create` | Crear copias. | sí |
| `server.backup.restore` | Restaurar copias. | sí |
| `server.backup.delete` | Borrar copias. | sí |

### Configuración

| Scope | Qué permite | Propietario |
|-------|-------------|-------------|
| `server.data.view` | Ver las variables. | sí |
| `server.data.edit` | Editar las variables de usuario, el puerto principal y las notas de puertos, bases de datos y subdominio. | sí |
| `server.data.edit.admin` | Variables de administrador (recursos), suspender, transferir entre nodos y transferencias externas. Desbloquea el entorno al editar la definición. | no |
| `server.flags.view` | Ver inicio y reinicio automático. También basta con `server.flags.edit`. | sí |
| `server.flags.edit` | Editar inicio y reinicio automático, avisos de Discord del servidor y subdominio. | sí |
| `server.name.edit` | Renombrar. | sí |
| `server.definition.view` | Ver la definición. | sí |
| `server.definition.edit` | Editar la definición (ver [Entorno y definición](#entorno-y-definición)). | no |
| `server.ports.edit` | Ninguna ruta de la API lo comprueba; la web lo usa. | no |
| `server.delete` | Borrar el servidor. | no |

### Tareas

| Scope | Qué permite | Propietario |
|-------|-------------|-------------|
| `server.tasks.view` | Ver tareas. | sí |
| `server.tasks.run` | Ejecutar una tarea a mano. | sí |
| `server.tasks.create` | Crear tareas. Crear y editar usan la misma ruta, que acepta cualquiera de los dos. | sí |
| `server.tasks.edit` | Editar tareas. | sí |
| `server.tasks.delete` | Borrar tareas. | sí |

### Usuarios y clientes

| Scope | Qué permite | Propietario |
|-------|-------------|-------------|
| `server.users.view` | Ver los miembros y la actividad del servidor. | sí |
| `server.users.edit` | Añadir subusuarios y cambiar sus permisos. | sí |
| `server.users.delete` | Quitar subusuarios. | sí |
| `server.users.create` | Ninguna ruta de la API lo comprueba: añadir usa `server.users.edit`. | sí |
| `server.clients.view`, `server.clients.create`, `server.clients.edit`, `server.clients.delete` | Ninguna ruta de la API los comprueba hoy. | sí |

### Administración

| Scope | Qué permite | Propietario |
|-------|-------------|-------------|
| `server.admin` | Todos los scopes de servidor en ese servidor. Además deja dar cualquier scope a los subusuarios. | no |
| `server.admin.view` | Ninguna ruta de la API lo comprueba; la web lo usa para la pestaña Administración. | no |
| `server.admin.install.view`, `server.admin.install.manage` | Igual que el anterior. | no |
| `server.admin.transfer.view`, `server.admin.transfer.manage` | Igual que el anterior. | no |
| `server.admin.config.view`, `server.admin.config.manage` | Igual que el anterior. | no |
| `server.admin.assignments.view`, `server.admin.assignments.manage` | Igual que el anterior. | no |

## Scopes del panel (23)

| Scope | Qué permite |
|-------|-------------|
| `admin` | Todo. Gestionar roles, claves API, extensiones, hosts de bases de datos, productos y propietarios de servidores. |
| `login` | Iniciar sesión, ver el propio perfil y leer plantillas. |
| `panel` | Ninguna ruta de la API lo comprueba. |
| `self.edit` | Editar el propio perfil, 2FA y claves SSH. |
| `self.clients` | Crear y borrar clientes OAuth2 propios. |
| `oauth2.auth` | Validar credenciales de usuarios por OAuth2. |
| `server.create` | Crear servidores. Sin `admin`, solo sub-servidores de un servidor propio. También clonar (experimental). |
| `settings.edit` | Editar los ajustes del panel y activar la licencia. |
| `nodes.view` | Ver nodos. |
| `nodes.create` | Crear nodos. |
| `nodes.edit` | Editar nodos. |
| `nodes.delete` | Borrar nodos. |
| `nodes.deploy` | Ver los datos de despliegue de un nodo (incluyen secretos). |
| `templates.view` | Ninguna ruta de la API lo comprueba: leer plantillas pide `login`. |
| `templates.local.edit` | Subir y editar plantillas locales. |
| `templates.repo.create` | Añadir repositorios de plantillas. |
| `templates.repo.delete` | Borrar repositorios de plantillas. |
| `users.info.search` | Buscar usuarios y ver la actividad del panel. |
| `users.info.view` | Ver un usuario. |
| `users.info.edit` | Crear, editar y borrar usuarios. |
| `users.perms.view` | Ver los permisos de un usuario. |
| `users.perms.edit` | Cambiar los permisos de un usuario. |
| `uptime.view` | Ver el historial de disponibilidad. |
