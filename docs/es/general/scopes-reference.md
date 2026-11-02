# Referencia de permisos (scopes)

Un **scope** es un permiso con nombre, por ejemplo `server.start`. Cada ruta de la API pide uno, y la petición pasa solo si el usuario lo tiene. La lista completa está en `internal/scopes/scopes.go`: 73 scopes, 23 del panel y 50 de servidor.

- **Scopes del panel**: valen para todo el panel (`nodes.view`, `settings.edit`…).
- **Scopes de servidor**: valen para un servidor concreto. La ruta tiene que llevar `:serverId`; sin él, la petición se rechaza.

Dos scopes cubren a otros: `admin` cubre todos, y `server.admin` cubre todos los de servidor en ese servidor. Como `admin` lo cubre todo, también cuenta como `login`: una cuenta con `admin` puede iniciar sesión aunque no tenga `login`.

Cuando a una petición le falta el scope, la API responde `403` sin cuerpo. Un scope de servidor pedido en una ruta sin `:serverId` se rechaza siempre.

Las claves API de facturación no usan scopes: tienen su propia lista de acciones (ver [Claves API](#claves-api)). Las rutas que hay detrás de cada scope están en la [referencia de la API](../reference/11-api-reference.md).

## De dónde salen los permisos

En cada petición el panel junta los permisos del usuario de estas fuentes:

| Fuente | En rutas del panel | En rutas de un servidor |
|--------|--------------------|-------------------------|
| Rol del usuario | Cuenta | Solo cuentan sus scopes del panel (por ejemplo `admin`). Sus scopes de servidor **no cuentan**. |
| Fila global del usuario (permisos sin servidor) | Cuenta | Cuenta |
| Ser propietario del servidor | — | Da el conjunto del propietario (ver abajo) |
| Fila del usuario en ese servidor | — | Cuenta, solo sus scopes de servidor (un scope del panel como `admin` en ella se ignora) |

Si la sesión viene de un cliente OAuth2, el cliente pone otro límite encima: un cliente ligado a un servidor solo actúa sobre ese servidor, y un cliente con scopes declarados nunca pasa de ellos. Ese límite se aplica también a los scopes que la petición guarda para las comprobaciones posteriores dentro de cada ruta (ver la contraseña de una base de datos, editar el entorno, conceder scopes, los pasos de una tarea), así que esas comprobaciones nunca ven los poderes más amplios del usuario.

El socket del servidor (consola, estado y estadísticas en tiempo real) usa los mismos scopes resueltos que cualquier otra ruta: los del propietario, la fila global, el rol y los límites del cliente OAuth2. Recibe la consola con `server.console`, el estado con `server.status` y las estadísticas con `server.stats`.

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

#### Variables que solo cambia un administrador

Algunas variables de la plantilla guardan lo que el cliente paga: los límites del servidor, su dirección y sus puertos. Solo las cambia quien tiene `server.data.edit.admin`, **aunque la plantilla las marque como editables por el usuario**:

| Tipo | Variables |
|------|-----------|
| Límites y dirección | `memory`, `cpu`, `io`, `disk`, `swap`, `ip` (sin distinguir mayúsculas) |
| Puertos | `port`, `port2`, `port3`…, `queryport`, `rconport`, `steamport`, `gameport`, `serverport`, `voiceport`, `webport`, `httpport`, `appport`, cualquier nombre que termine en `_port`, `-port` o `.port`, y cualquier variable `integer` cuyo nombre termine en `port` |

Sin `server.data.edit.admin`:

- `POST /api/servers/:serverId/data` (la pestaña **Configuración**) las salta sin avisar, como cualquier otra variable que la plantilla no abre al usuario.
- `PUT /api/servers/:serverId/definition` conserva su valor actual y descarta las variables nuevas de ese tipo.

El propietario sí puede elegir cuál de los puertos ya asignados es el principal y ponerles notas (`PUT /api/servers/:serverId/port-settings`, con `server.data.edit`): eso no toma ningún puerto nuevo.

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
- La fila de permisos de un servidor **solo guarda scopes de servidor**. Los scopes del panel enviados a `PUT /api/servers/:serverId/user/:email` (`admin`, `nodes.view`, `settings.edit`…) se descartan antes de guardar, también cuando quien edita tiene `server.admin`. Al reunir los permisos de un usuario, los scopes del panel que aparezcan en una fila de servidor también se ignoran. Antes, quien tenía `server.admin` en un servidor podía escribir `admin` en su propia fila y convertirse en administrador del panel.
- Nadie da más de lo que tiene: quien edita solo puede poner o quitar los scopes que él mismo tiene en ese servidor. Los demás scopes de la fila se quedan como estaban. Las excepciones son quien tiene `server.admin` (en el servidor o en su fila global) o `admin` en su fila global: esos pueden poner cualquiera.
- Si el correo no tiene cuenta, el panel crea una cuenta invitada (con permiso para iniciar sesión) y le manda un correo con el enlace para elegir su contraseña (`/forgot-password/`). El correo debe ser una dirección simple (`ana@example.com`, no `Ana <ana@example.com>`). Con el registro público desactivado (`panel.registrationEnabled: false`), solo un administrador puede invitar a alguien sin cuenta; cualquier otro recibe `403 ErrInviteSignupsClosed`.
- Buscar a una persona por correo en un servidor (`GET /api/servers/:serverId/user/:email`) responde `404` para una cuenta que no es miembro de ese servidor, igual que para un correo desconocido. Así un propietario no puede averiguar qué correos tienen cuenta en el panel.

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
| Plugins | `server.files.edit`, y la pestaña activada para ese servidor |
| Resumen, Actividad | Siempre (sus datos piden igualmente su propio scope) |
| Propiedades (beta) | Minecraft: Java Edition, `panel.experimental.propertiesEditor` activado y `server.files.edit` |
| Modpacks, Mapa | Servidores de Minecraft: Java Edition |
| Administración | Cualquiera de `server.install`, `server.delete`, `server.data.edit.admin`, `server.definition.edit` o los scopes de vista `server.admin.*`. El propietario la ve por `server.install` (reinstalar); las secciones de transferencia, configuración, suspensión y borrado piden los scopes del administrador. |

`admin` o `server.admin` muestran todo. Ocultar es solo comodidad: la API comprueba cada petición igualmente.

### Entorno y definición

El entorno decide qué puede tocar el contenedor en el nodo (carpetas del host, modo privilegiado, ejecutar en el host). Por eso:

- Editar la definición (`PUT /api/servers/:serverId/definition`) pide `server.definition.edit`, que el propietario no tiene.
- Aunque alguien tenga `server.definition.edit`, si no tiene también `server.data.edit.admin` el entorno se queda como está. En un entorno `docker` solo puede cambiar `image`, `containerRoot` y `config`, y `config` pierde cualquier clave `User` (el juego se ejecuta con el usuario que le da el nodo, ver [Seguridad](./security.md#contenedores-de-los-servidores-de-juego)). Los puertos publicados (`portBindings`) se quedan como en la plantilla. No puede cambiar el tipo de entorno. Las [variables solo de administrador](#variables-que-solo-cambia-un-administrador) conservan su valor.
- Quien no es administrador solo puede crear sub-servidores: de un servidor del que es propietario, en `docker` y solo con `image`, `portBindings`, `containerRoot` y `config` (sin su `User`). `bindings`, `hostConfig`, `networkName` y `labels` se descartan. Cada puerto publicado debe ser una de las variables de puerto del propio sub-servidor (`${port}:25565`, `0.0.0.0:${port2}:…` o `${ip}:${port}:…`); un puerto fijo como `0.0.0.0:25565:25565` se rechaza, porque podría quitarle el puerto a otro cliente.

### Sub-servidores y copias

- Un sub-servidor pertenece al propietario del padre (si el padre no tiene propietario, el hijo tampoco). Los miembros del padre pasan al hijo con exactamente los permisos que tenían en el padre.
- Una copia pertenece al propietario del original y copia las filas de permisos de sus usuarios tal cual.

### Servidores que ya existían

Al actualizar, la migración `20261026-server-owners` nombra un propietario para cada servidor que no tiene: el miembro con más scopes en ese servidor y, si hay empate, el que se añadió primero. Los clientes de la facturación tienen el conjunto completo, así que suelen ser ellos.

Un servidor sin miembros se queda sin propietario. Después de actualizar conviene revisar en la pestaña **Usuarios** que cada servidor tenga el propietario correcto y, si hace falta, cambiarlo con **Hacer propietario**.

## Roles

Un rol da scopes del panel a todos sus usuarios. Sus scopes de servidor no cuentan en ningún servidor: lo que un usuario puede hacer en un servidor sale de ser su propietario o de su fila en él.

Roles que el panel crea en la base de datos (migración `20260617-default-roles`, ajustada por `20260624-usuario-role-templates-view`, `20260625-usuario-role-add-server-create` y `20260821-fix-usuario-role-scopes`). Esto es lo que tienen en una instalación nueva; un administrador puede cambiarlos después en **Roles**:

| Rol | Scopes del panel | Scopes de servidor guardados (sin efecto) |
|-----|------------------|-------------------------------------------|
| Administrador | `admin` | — |
| Usuario | `login`, `self.edit`, `self.clients`, `server.create`, `templates.view`, `uptime.view` | `server.view`, `server.status`, `server.stats`, `server.console`, `server.console.send`, `server.start`, `server.stop`, `server.kill`, `server.install`, `server.files.view`, `server.files.edit`, `server.sftp`, `server.data.view`, `server.data.edit`, `server.flags.view`, `server.flags.edit`, `server.definition.view`, `server.definition.edit`, `server.backup.view`, `server.backup.create`, `server.backup.restore`, `server.backup.delete`, `server.clients.view`, `server.clients.create`, `server.clients.edit`, `server.clients.delete`, `server.users.view`, `server.tasks.view`, `server.tasks.run` |

Los scopes de servidor de "Usuario" vienen de versiones anteriores, cuando el rol daba a cualquier miembro el control de sus servidores; ya no tienen efecto en los servidores. `server.create` le deja dividir sus propios servidores; crear un servidor sin padre es solo para administradores.

- "Administrador" y "Usuario" no se pueden borrar (`cannot delete a default role`). Borrar cualquier otro rol deja a sus usuarios sin rol.
- La descripción de los roles se guarda en inglés ("Full access to all panel features (super-admin).", "Standard user: …").

Quién recibe cada rol:

- Los clientes creados por la API de facturación (`POST /api/v1/provision`) reciben "Usuario".
- Al actualizar, a todos los usuarios sin rol se les dio "Usuario" (migraciones `20260624-usuario-role-templates-view` y `20260625-assign-usuario-role-to-existing-users`).
- Quien se registra en la página de registro **no recibe rol**: solo `login` en su fila global.
- `SkyPanel user add --admin` pone `login` y `admin` en la fila global del usuario, sin rol.
- Un administrador puede elegir el rol al crear o editar un usuario en **Usuarios**. Solo los administradores crean, editan y borran roles; listarlos (`GET /api/roles`) también funciona con `users.info.view` o `users.info.edit`. Solo un administrador puede cambiar el rol de un usuario: el rol que envía alguien del personal sin `admin` se ignora.
- Una cuenta creada en **Usuarios** sin rol no tiene ningún scope, ni siquiera `login`, así que no puede iniciar sesión hasta que reciba un rol o `login` en su fila global (`PUT /api/users/:id/perms`, con `users.perms.edit`). La página web no tiene editor para la fila global.
- Quien es invitado a un servidor por correo recibe `login` en su fila global y ningún rol (ver [Subusuarios](#subusuarios)).

### Editar otras cuentas

`users.info.edit` (editar y borrar cuentas) y `users.perms.edit` (cambiar sus permisos) son poderes del personal. Quien los tiene sin `admin` **no puede editar, borrar ni cambiar los permisos de una cuenta que tenga un poder que él no tiene**. El panel compara la fila global y el rol de la cuenta con los scopes de quien edita, para estos scopes: `admin`, `settings.edit`, `nodes.create`, `nodes.edit`, `nodes.delete`, `nodes.deploy`, `templates.local.edit`, `templates.repo.create`, `templates.repo.delete`, `users.info.edit`, `users.perms.edit`, `oauth2.auth` y `server.admin` (en la fila global vale para todos los servidores). La respuesta es `403` ("you cannot modify an account with permissions you do not have"). Si no, alguien del personal con solo `users.info.edit` podría poner la contraseña de una cuenta que edita los ajustes y entrar con ella.

Además, sin `admin`, `users.perms.edit` solo pone o quita scopes del panel que tiene quien edita; los scopes de servidor de la fila global de alguien se quedan como están, porque valen en todos los servidores.

Cambiar la contraseña de un usuario desde **Usuarios** (`POST /api/users/:id` con `password`) cierra todas las sesiones abiertas de ese usuario.

### Quitar el acceso

`login` se comprueba cada vez que alguien entra, no solo al abrir la sesión. Cuenta si está en la fila global del usuario o en su rol (o si tiene `admin`):

| Vía de entrada | Qué pasa sin `login` |
|----------------|----------------------|
| Página de inicio de sesión (contraseña, 2FA, entrada tras registrarse) | Se rechaza con `403 ErrLoginNotPermitted`. |
| Cualquier petición con una sesión abierta (cookie o cabecera `Authorization`) | La sesión se cierra y la petición responde `401`. Así, quitar `login`, o el rol que lo daba, saca a la persona en su siguiente petición en lugar de esperar a que caduque la sesión. |
| SFTP, con contraseña o clave SSH | Se rechaza la conexión. |
| Enlaces de acceso de la facturación (`/api/v1/sso`) | El navegador va a `/login/?sso=denied`. |

Una sesión que pertenecía a un cliente OAuth2 que ya no existe se cierra del mismo modo.

## Claves API

Las claves API son para sistemas de facturación (WHMCS, Paymenter…). Solo sirven en `/api/v1` y cada clave solo hace las acciones que se le dieron.

Las crea un administrador en **Configuración → Claves API**. Al crearla se marca **Qué puede hacer**; por defecto vienen todas marcadas. La clave completa (`ak_...`) se muestra una sola vez; el panel guarda solo su hash. Para cambiar las acciones de una clave hay que crear otra y revocar la antigua: una clave no se puede editar.

La clave va en la cabecera `X-Api-Key: ak_...` o en `Authorization: Bearer ak_...`.

| Acción | Rutas | En Configuración |
|--------|-------|------------|
| `create` | `POST /api/v1/provision` | Crear servidores |
| `terminate` | `POST /api/v1/terminate` | Dar de baja |
| `suspend` | `POST /api/v1/suspend`, `POST /api/v1/unsuspend` | Suspender y reactivar |
| `change-package` | `POST /api/v1/change-package` | Cambiar de plan |
| `sso` | `POST /api/v1/sso` | Acceso directo |

- `GET /api/v1/ping` no pide ninguna acción.
- Una ruta que la clave no tiene responde `403 ErrAPIKeyNotAllowed`.
- Las claves creadas antes de esto guardan `provision` (o ninguna lista). Eso significa todas las acciones, para que las integraciones existentes sigan funcionando. En Configuración aparecen como **Todas las acciones**.

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
| `server.view` | Ver el servidor, su versión, su historial de disponibilidad, su mapa y la lista de bases de datos (sin contraseñas; estas requieren `server.data.edit`). Conectar al socket. | sí |
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
| `server.backup.create` | Crear copias. Sin `admin`, hasta `panel.backups.maxPerServer` por servidor (10 por defecto; `0` quita el límite); después, `409 ErrBackupLimit`. | sí |
| `server.backup.restore` | Restaurar copias. | sí |
| `server.backup.delete` | Borrar copias. | sí |

### Configuración

| Scope | Qué permite | Propietario |
|-------|-------------|-------------|
| `server.data.view` | Ver las variables. | sí |
| `server.data.edit` | Editar las variables que la plantilla abre al usuario (nunca límites, IP ni puertos, ver [arriba](#variables-que-solo-cambia-un-administrador)), elegir el puerto principal entre los asignados, notas de puertos, bases de datos (incluida la contraseña de cada una) y subdominio. | sí |
| `server.data.edit.admin` | Variables de administrador (recursos), suspender, transferir entre nodos y transferencias externas. Desbloquea el entorno al editar la definición. | no |
| `server.flags.view` | Ver inicio y reinicio automático. También basta con `server.flags.edit`. | sí |
| `server.flags.edit` | Editar inicio y reinicio automático, avisos de Discord del servidor y subdominio. | sí |
| `server.name.edit` | Renombrar. | sí |
| `server.definition.view` | Ver la definición. | sí |
| `server.definition.edit` | Editar la definición (ver [Entorno y definición](#entorno-y-definición)). | no |
| `server.ports.edit` | Nada lo comprueba: solo aparece como casilla en el editor de roles. Los puertos se cambian con `server.data.edit.admin`. | no |
| `server.delete` | Borrar el servidor. | no |

### Tareas

| Scope | Qué permite | Propietario |
|-------|-------------|-------------|
| `server.tasks.view` | Ver tareas. | sí |
| `server.tasks.run` | Ejecutar una tarea a mano. | sí |
| `server.tasks.create` | Crear tareas. Crear y editar usan la misma ruta, que acepta cualquiera de los dos. Sin `admin`, cada paso pide además el scope de su acción (`server.console.send` para un comando, `server.start`/`server.stop` para el encendido, `server.backup.create` para una copia); ver [Tareas programadas](../features/scheduled-tasks.md#comprobaciones-para-usuarios-que-no-son-administradores). | sí |
| `server.tasks.edit` | Editar tareas. | sí |
| `server.tasks.delete` | Borrar tareas. | sí |

### Usuarios y clientes

| Scope | Qué permite | Propietario |
|-------|-------------|-------------|
| `server.users.view` | Ver los miembros y la actividad del servidor. | sí |
| `server.users.edit` | Añadir subusuarios y cambiar sus permisos (solo scopes de servidor). | sí |
| `server.users.delete` | Quitar subusuarios. | sí |
| `server.users.create` | Ninguna ruta de la API lo comprueba: añadir usa `server.users.edit`. | sí |
| `server.clients.view`, `server.clients.create`, `server.clients.edit`, `server.clients.delete` | Ninguna ruta de la API los comprueba hoy. | sí |

### Administración

| Scope | Qué permite | Propietario |
|-------|-------------|-------------|
| `server.admin` | Todos los scopes de servidor en ese servidor. Además deja dar cualquier scope a los subusuarios. | no |
| `server.admin.view` | Ninguna ruta de la API lo comprueba; la web lo usa para mostrar la pestaña **Administración**. | no |
| `server.admin.install.view`, `server.admin.transfer.view`, `server.admin.config.view`, `server.admin.assignments.view` | Ninguna ruta de la API los comprueba; la web los usa para mostrar esa sección de la pestaña **Administración** (`server.admin.config.view` también muestra los recursos en **Configuración**). Actuar en cada sección sigue pidiendo el scope real: `server.install`, `server.data.edit.admin`, `server.definition.edit`. | no |
| `server.admin.install.manage`, `server.admin.transfer.manage`, `server.admin.config.manage`, `server.admin.assignments.manage` | Ninguna ruta de la API los comprueba (`server.admin.config.manage` solo muestra los recursos en **Configuración**). | no |

## Scopes del panel (23)

| Scope | Qué permite |
|-------|-------------|
| `admin` | Todo, incluido iniciar sesión. Solo `admin` gestiona roles, claves API, extensiones, hosts de bases de datos, productos y propietarios de servidores, abre la documentación de la API (`/swagger`), crea servidores sin padre, pone cualquier paso en una tarea y ve todos los servidores. |
| `login` | Iniciar sesión (ver [Quitar el acceso](#quitar-el-acceso)), ver el propio perfil, los ajustes de usuario, las extensiones activas y las versiones de juego, y leer plantillas. |
| `panel` | Nada lo comprueba. |
| `self.edit` | Editar el propio perfil y la contraseña, la 2FA (incluidos los códigos de recuperación) y las claves SSH. |
| `self.clients` | Crear y borrar clientes OAuth2 propios. |
| `oauth2.auth` | Validar credenciales de usuarios por OAuth2. |
| `server.create` | Crear servidores. Sin `admin`, solo sub-servidores de un servidor propio (ver [Subservidores](../features/sub-servers.md)). También clonar, con el experimento activado y siendo miembro del original (ver [Clonar servidores](../features/cloning.md)), y comprobar si un puerto está libre en un nodo (`POST /api/nodes/:id/ports/check`). |
| `settings.edit` | Editar los ajustes del panel, enviar las pruebas de correo, Discord, S3 y Cloudflare, activar la licencia, buscar e iniciar actualizaciones del panel y ver el estado del dominio (ver [Configuración del panel](../features/panel-settings.md) y [Actualizaciones del panel](../features/panel-updates.md)). |
| `nodes.view` | Ver nodos, sus funciones, datos del sistema e historial de red. |
| `nodes.create` | Crear nodos. |
| `nodes.edit` | Editar nodos. |
| `nodes.delete` | Borrar nodos. |
| `nodes.deploy` | Ver los datos de despliegue de un nodo (incluyen secretos). |
| `templates.view` | Ninguna ruta de la API lo comprueba: leer plantillas pide `login`. La web lo usa para mostrar **Plantillas** en el menú. |
| `templates.local.edit` | Subir y editar plantillas locales. |
| `templates.repo.create` | Añadir repositorios de plantillas. |
| `templates.repo.delete` | Borrar repositorios de plantillas. |
| `users.info.search` | Buscar usuarios y ver la actividad del panel (ver [Registro de actividad y registros de acceso](../features/activity-and-access-log.md)). |
| `users.info.view` | Ver un usuario y listar los roles. |
| `users.info.edit` | Crear, editar y borrar usuarios, salvo cuentas con poderes que quien edita no tiene (ver [Editar otras cuentas](#editar-otras-cuentas)). |
| `users.perms.view` | Ver los permisos de un usuario. |
| `users.perms.edit` | Cambiar los permisos del panel de un usuario, con el mismo límite. |
| `uptime.view` | Ver el historial de disponibilidad (`GET /api/uptime`). Sin `admin`, solo de los servidores de los que el usuario es miembro. |
