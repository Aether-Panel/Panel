# Conceptos básicos

Esta página explica las piezas que forman Aether Panel y cómo encajan: nodos, servidores, plantillas, sub-servidores, usuarios y permisos, SFTP, hosts de bases de datos, transferencias y las redes de Docker. Cada sección enlaza con la página que entra en detalle.

## Panel y daemon

Aether Panel es un único programa (el binario `SkyPanel`) con dos funciones que pueden ir juntas o por separado:

| Función | Qué hace | Se activa con |
|---------|----------|---------------|
| **Panel** | La interfaz web (Astro + React) y la API REST (`/api`, `/auth`, `/oauth2`). Guarda usuarios, permisos, nodos, servidores y plantillas en la base de datos. | `panel.enable` (`true`) |
| **Daemon** | Ejecuta los servidores de juego en su máquina: contenedores Docker, archivos, consola, estadísticas, copias de seguridad, tareas programadas y el servidor SFTP. Su API está en `/daemon`. | `daemon.enable` (`true`) |

Los dos escuchan en la misma dirección HTTP (`web.host`, `0.0.0.0:8080`); el SFTP escucha en `daemon.sftp.host` (`0.0.0.0:5657`).

La instalación oficial funciona en Docker: `docker-compose.yml` arranca el contenedor `skypanel` (panel y daemon), `skypanel-mysql` (MariaDB 10.11, la base de datos del panel) y `skypanel-adminer` (Adminer, solo en `127.0.0.1:8081`). El daemon crea los contenedores de los servidores de juego a través del socket de Docker del host. Ver la [Guía de instalación](../setup/01-installation.md).

La base de datos del panel puede ser SQLite (`panel.database.dialect: sqlite3`, el valor por defecto del binario), MySQL/MariaDB (`mysql`, la que usa la instalación Docker) o PostgreSQL (`postgresql`).

## Nodos

Un **nodo** es una máquina que ejecuta servidores de juego, es decir, una máquina con un daemon.

- **Nodo local**: el daemon dentro del propio contenedor del panel. Siempre existe (ID `0`, llamado `LocalNode`), no se guarda en la base de datos y no se puede editar. Su dirección sale de la URL principal del panel (`panel.settings.masterUrl`) o de la IP del servidor cuando el dominio está detrás del proxy de Cloudflare. Cuando el panel le habla, la llamada se atiende dentro del mismo proceso, sin pasar por la red.
- **Nodos remotos**: otras máquinas que ejecutan la misma imagen con el panel desactivado (`SKYPANEL_PANEL_ENABLE=false`) y `token.public` apuntando al `/auth/publickey` del panel. Un administrador los registra en **Nodos** (`nodes.create`) con un host público y uno privado, el puerto del daemon (8080) y el de SFTP (5657). `GET /api/nodes/:id/deployment` (`nodes.deploy`) da las credenciales que usa el nodo ante el panel: el ID de cliente `.node_<id>` y su secreto, que van en `daemon.auth.clientId` y `daemon.auth.clientSecret` (con `daemon.auth.url` apuntando a la dirección del panel).

Cómo se comunican:

- **Panel → nodo**: peticiones HTTP con un JWT firmado por el panel (Ed25519, válido 15 minutos, ligado a ese nodo). El nodo lo comprueba con la clave pública del panel y responde `401` (`invalid_token`) cuando no es válido. A un nodo que se ha visto usar TLS una vez ya nunca se le llama por HTTP sin cifrar. Además, cada 30 segundos el panel pregunta a cada nodo remoto por el estado de sus servidores (`GET /daemon/servers/status`) para registrar su [disponibilidad](./13-uptime.md).
- **Nodo → panel**: el nodo se identifica con sus credenciales de cliente (`/oauth2/token`, `client_credentials`) y comprueba los inicios de sesión SFTP contra el panel: las contraseñas con el tipo `password` y las claves SSH con `POST /auth/sftp-key`, firmado con el secreto del nodo.
- **Navegador ↔ nodo**: la consola y las estadísticas pasan por el panel, que hace de proxy del websocket hacia el nodo.

Cada nodo informa de lo que admite (`GET /api/nodes/:id/features`) y de su información de sistema (`GET /api/nodes/:id/system`). El panel de inicio muestra el historial de CPU, memoria, disco y red de cada nodo. Además, los nodos:

- descargan en segundo plano las imágenes de `docker.prefetchImages` y las mantienen actualizadas, para que el primer servidor que use una no tenga que esperar;
- rechazan una instalación si la carpeta de servidores se quedaría con menos de `daemon.minFreeDiskMB` (2048 MB) libres;
- responden a la comprobación de puertos del asistente de nuevo servidor (`POST /api/nodes/:id/ports/check`).

Más en [Panel de inicio y nodos](../features/dashboard-and-nodes.md).

## Servidores

Un **servidor** es una instancia de un juego o aplicación que se ejecuta en un nodo.

- **Identificador**: letras, números, `-` y `_`, de 1 a 20 caracteres, sin empezar por `-` ni `_`. El asistente lo forma con el nombre más un sufijo aleatorio; el nombre en sí es texto libre.
- **Definición**: el nodo guarda una definición JSON por servidor con sus variables, pasos de instalación, comando de arranque y entorno. Empieza siendo una copia de la plantilla.
- **Entorno**: Docker. La instalación Docker desactiva los entornos que se ejecutan en el host con `docker.disallowHost`, y el asistente solo ofrece Docker.
- **Recursos**: CPU (porcentaje de un núcleo, así que 200 son dos núcleos), memoria (MB), disco (MB) y prioridad de E/S de disco, guardados en las variables `cpu`, `memory`, `disk` e `io`. Solo un administrador los cambia.
- **Puertos**: un puerto principal y puertos extra (`port2`, `port3`…), cada uno publicado por TCP y UDP. El propietario puede elegir cuál de los puertos asignados es el principal y ponerles notas; solo un administrador asigna puertos.
- **Ciclo de vida**: crear → instalar (ejecuta los pasos de instalación de la plantilla; es automático justo después de crearlo mientras `panel.servers.autoInstall` esté activado) → iniciar / detener / reiniciar / forzar → reinstalar → borrar. Un servidor que se cae se reinicia según su ajuste de reinicio automático, y la caída queda registrada.
- **Propietario y subusuarios**: cada servidor tiene un propietario (el cliente al que pertenece) y puede tener subusuarios con exactamente los permisos que el propietario les dé.

Dentro de la página del servidor: consola, resumen con estadísticas, gestor de archivos, configuración, usuarios, base de datos, copias de seguridad, tareas programadas, actividad, SFTP y, según el juego y los permisos, plugins, modpacks, el mapa del mundo, sub-servidores (Splitter), migración a otro panel y la pestaña de administración. En [Uso del panel](./usages.md) hay un recorrido, y en las [guías de funcionalidades](../features/README.md) el detalle de cada parte.

## Plantillas

Una **plantilla** es un modelo JSON para un tipo de servidor: sus variables (con tipo, valor por defecto, opciones y si el usuario puede editarlas), los pasos de instalación, el comando de arranque, la imagen de Docker y los puertos.

Las plantillas vienen de repositorios:

| Repositorio | Qué es |
|-------------|--------|
| **Local** (ID `0`) | Plantillas guardadas en la base de datos del panel. Se suben y editan con `templates.local.edit`. |
| **Comunidad** | Cuando `templates.url` está configurado, un único repositorio leído de ese índice JSON. La instalación Docker lo fija en `https://templates.aetherpanel.es/templates.json` (`SKYPANEL_TEMPLATES_URL`), y entonces se ignoran los repositorios guardados en la base de datos. |
| Repositorios git | Sin `templates.url`, los repositorios añadidos con `templates.repo.create` se clonan y se leen del disco. |

Cualquier usuario con sesión y `login` puede leer plantillas; la entrada **Plantillas** del menú pide `templates.view`. Al crear un servidor, la plantilla elegida se copia en la definición del servidor con los valores escritos en el asistente; editar después la plantilla no cambia los servidores existentes. Más en [Plantillas](../features/templates.md).

## Sub-servidores (Splitter)

La pestaña **Splitter** divide un servidor en otros más pequeños. Un sub-servidor:

- saca su CPU, memoria y disco del padre: el padre debe tener eso libre, y sus propios límites en el nodo se reducen a lo que queda;
- se ejecuta en el nodo del padre, en Docker, con el propietario del padre como propietario; los miembros del padre pasan a él con los mismos permisos que tienen en el padre;
- solo lo puede crear el propietario del padre o un administrador (el propietario necesita el scope del panel `server.create`, que incluye el rol "Usuario");
- si lo crea alguien que no es administrador, solo puede publicar sus propias variables de puerto y no puede indicar carpetas del host, `hostConfig`, la red, etiquetas ni un usuario de contenedor.

Detalles en [Sub-servidores](../features/sub-servers.md) y en la [Referencia de scopes](./scopes-reference.md#entorno-y-definición).

## Usuarios, roles y permisos

- Un **usuario** inicia sesión con correo y contraseña (y 2FA si la tiene activada). Las cuentas las crea un administrador, el registro público (si `panel.registrationEnabled` está activado), la API de facturación o la invitación de un correo a un servidor.
- Un **scope** es un permiso con nombre (`server.start`, `nodes.view`…). Hay 73: 23 del panel y 50 de servidor. `admin` los cubre todos; `server.admin` cubre todos los de servidor en un servidor.
- Un **rol** da scopes del panel a sus usuarios. El panel crea "Administrador" (`admin`) y "Usuario" (`login`, `self.edit`, `self.clients`, `server.create`, `templates.view`, `uptime.view`). Los scopes de servidor de un rol nunca se aplican en los servidores.
- En un servidor, el poder de un usuario sale de ser su **propietario** (un conjunto fijo, `scopes.OwnerScopes`) o de su **fila de permisos** en ese servidor, que solo guarda scopes de servidor.

Todo sobre scopes, propietarios, subusuarios, roles y claves API de facturación está en la [Referencia de scopes](./scopes-reference.md). El inicio de sesión, las sesiones, la 2FA y las claves SSH están en [Seguridad](./security.md).

## Autenticación

| Método | Lo usa |
|--------|--------|
| Cookie de sesión `skypanel_auth` | La interfaz web. Las sesiones duran 1 hora de inactividad y se renuevan mientras hay actividad. |
| `Authorization: Bearer <token>` | Clientes de la API. El mismo token de sesión, o un token OAuth2. |
| OAuth2 (`/oauth2/token`) | `client_credentials` para clientes OAuth2 y nodos, `password` para las comprobaciones de contraseñas SFTP de los nodos remotos. Cada usuario crea sus propios clientes con `self.clients`. |
| Secreto del nodo (`/auth/sftp-key`) | Comprobación de claves SFTP desde los nodos remotos: `Authorization: Bearer <secreto del nodo>`. |
| JWT (Ed25519) | Peticiones del panel a los nodos. Clave pública en `/auth/publickey`. |
| Claves API de facturación (`ak_...`) | Solo `/api/v1` (aprovisionamiento, suspensión, enlaces de acceso). |

Ver [Seguridad](./security.md#inicio-de-sesión-y-sesiones).

## SFTP

Cada daemon ejecuta un servidor SFTP (puerto 5657 por defecto, `daemon.sftp.host`).

- El usuario es `correo#idServidor`; la pestaña **SFTP** del servidor muestra la dirección completa.
- Se entra con la contraseña del panel o con una clave SSH añadida en la configuración de la cuenta (las claves funcionan en todos los nodos; un nodo remoto pide al panel que las compruebe).
- El usuario necesita `server.sftp` en ese servidor, o ser su propietario, y el permiso general `login`. Los dos se comprueban en cada conexión.
- Una sesión solo ve la carpeta de ese servidor. Lo que se sube cuenta en el límite de disco del servidor.

## Hosts de bases de datos

Un **host de bases de datos** es un servidor MySQL o MariaDB donde el panel crea bases de datos para los servidores de juego.

- Los administradores los gestionan en **Hosts de bases de datos** (`/api/databasehosts`, requiere `admin`): nombre, host, puerto (3306), un usuario que pueda crear usuarios y conceder privilegios, un máximo opcional de bases de datos (`0` es sin límite) y un nodo opcional (entonces solo lo pueden usar los servidores de ese nodo).
- En la pestaña **Base de datos** de un servidor, quien tiene `server.data.edit` crea una base de datos en uno de los hosts que ese servidor puede usar. Se llama `s_<idServidor>_<nombre>`, tiene su propio usuario con nombre y contraseña aleatorios, y ese usuario recibe todos los privilegios solo sobre esa base de datos.
- Al borrar el servidor se borran del host sus bases de datos y sus usuarios.
- Los servidores de juego llegan a la MariaDB del propio panel como `mysql:3306`, porque está en su red de Docker (ver más abajo).

Más en [Hosts de bases de datos](../features/database-hosts.md).

## Transferencias

- **Entre nodos del mismo panel**: un administrador (`server.data.edit.admin`) mueve un servidor a otro nodo desde la pestaña **Administración** (`POST /api/servers/:serverId/transfer`). Sus subservidores, sus tareas programadas y las copias de seguridad guardadas en el nodo antiguo se van con él; si falla un paso, o el panel se reinicia antes de confirmar el traslado, la transferencia se deshace y el servidor se queda donde estaba.
- **Entre paneles (transferencia federada)**: mueve un servidor de una instalación de Aether Panel a otra (pestaña **Migración**, `server.data.edit.admin`). Los paneles firman sus peticiones con claves Ed25519 (`panel.exTransferKey`), los tokens se guardan con HMAC-SHA256 y una sesión de transferencia vale 15 minutos; mientras se mueven los datos, los latidos del destino (cada 20 segundos) la mantienen viva, hasta 6 horas. Estados: `CREATED` → `VALIDATED` → `MIGRATING` → `CONSUMED` / `COMPLETED`, o `FAILED` / `CANCELLED`. Las rutas del origen son `/api/servers/:serverId/extransfer/{create,pull,status,cancel}`; las que llama el otro panel son `/api/extransfer/{validate,consume,heartbeat,confirm,download,cancel}`.

Más en [Transferencias de servidores](../features/server-transfers.md).

<a id="red-docker-auto-deteccion-skypanel-network"></a>

## Redes de Docker

La instalación Docker usa dos redes:

| Red | Quién está en ella |
|-----|--------------------|
| `skypanel-network` | El panel, MariaDB y Adminer. |
| `skypanel-servers` | Los contenedores de los servidores de juego, MariaDB y el panel. |

- Los servidores de juego van a `skypanel-servers` (`docker.serversNetwork`, que fija `docker-compose.yml`). Llegan a MariaDB como `mysql:3306`, pero no a la red propia del panel. El panel también está en `skypanel-servers` para llegar a los servidores de juego por dentro (mapa del mundo, lista de jugadores).
- Una plantilla puede indicar su propia red (`networkName`); solo los administradores pueden fijarla.
- Si `docker.serversNetwork` está vacío, o esa red no existe, los servidores se unen a la red en la que está el contenedor del panel (detectada al arrancar), como en versiones anteriores.

<a id="propagación-de-actualizaciones-del-panel"></a>

## Actualizaciones del panel

**Configuración → Actualizaciones** comprueba en GitHub si hay una versión más nueva de la rama desde la que se instaló el panel, y solo la ofrece cuando las comprobaciones de GitHub sobre ella han pasado. Actualizar (`POST /api/settings/update-panel`, `settings.edit`) arranca en el host un contenedor temporal que ejecuta el script de actualización del panel local, y pide a cada nodo remoto que se actualice (`POST /daemon/update`). El instalador hace lo mismo desde la línea de comandos con `--update`. Ver [Actualizaciones del panel](../features/panel-updates.md).

## Línea de comandos

El binario `SkyPanel` también tiene una CLI (`user add`, `user edit`, `db upgrade`, `version`…). En la instalación Docker se ejecuta con `docker exec -it skypanel /SkyPanel/bin/SkyPanel …`. Ver la [Referencia de comandos de la CLI](../reference/14-cli-commands.md).
