# Solución de problemas

Problemas habituales al instalar y usar Aether Panel, con su causa y cómo resolverlos. Las rutas suponen una instalación hecha con el [instalador](./01-installation.md) en `/opt/skypanel`.

## Dónde mirar primero

```bash
cd /opt/skypanel
docker compose ps                        # ¿están en marcha skypanel y skypanel-mysql?
docker compose logs -f skypanel          # salida del panel
docker compose logs -f mysql             # salida de MariaDB
tail -f storage/skypanel-logs/skypanel.log
```

Las respuestas de error de la API llevan un código (`ErrNodeDiskFull`, `ErrBackupLimit`…) que identifica el problema; los mensajes de este documento salen del código del panel.

---

## Instalación

### La construcción de la imagen falla

El instalador termina con «La construcción de la imagen falló». Las causas habituales son poca memoria, poco disco o falta de red. El instalador ya avisa al empezar si hay menos de 3 GB entre RAM y swap; para añadir 2 GB de swap:

```bash
fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
```

Si falta disco, libere espacio de Docker (`docker system prune`) y vuelva a ejecutar el instalador: es seguro repetirlo y no borra datos.

### «Docker Hub ha limitado las descargas desde esta IP»

Docker Hub limita las descargas sin cuenta por IP, y en servidores con IP compartida se agotan. El instalador reintenta tres veces. Espere unas horas o inicie sesión con `docker login` y repita la instalación.

### «Cambiar puertos necesita Docker Compose >= 2.24.4»

El archivo `docker-compose.override.yml` que genera el instalador usa `ports: !override`. Hace falta en cuanto los puertos no son los predeterminados, al usar `--bind` (incluido el `127.0.0.1` automático con dominio) o cuando el 3307 está ocupado. Actualice Docker (el plugin `docker-compose-plugin`) o use los puertos por defecto sin dominio.

### «Puerto N (…) ocupado por …»

El instalador comprueba el puerto del panel, el SFTP y, con Nginx, el 80 y el 443, y dice qué contenedor o proceso los ocupa. Libérelos o elija otros con `--port` / `--sftp-port`. `--force` continúa a pesar del conflicto. Si el 3307 (MariaDB) está ocupado, no es un error: el instalador usa el primero libre entre 3308 y 3399.

### «Ya existe un contenedor llamado 'skypanel' que no pertenece a /opt/skypanel»

Otro proyecto tiene un contenedor con ese nombre. El instalador no lo toca: renómbrelo o elimínelo usted.

### «Ya hay otra ejecución del instalador en curso»

Hay otro instalador en marcha (bloqueo `/run/aetherpanel-install.lock`). Espere a que termine.

### El certificado de Let's Encrypt no se emite

El instalador reintenta `certbot` una vez y, si falla, deja el panel por HTTP. Compruebe que:

1. El dominio apunta a la IP pública del servidor (`dig +short su-dominio`).
2. El puerto 80 está abierto también en el cortafuegos del proveedor.
3. Con el proxy de Cloudflare, «Always Use HTTPS» está desactivado durante la primera emisión.

Después, repita con `--domain su-dominio --ssl`, o guarde de nuevo la URL principal en **Configuración → General**. El servicio `aether-domain` no repite un intento fallido para la misma URL hasta pasada una hora.

### La base de datos externa no es accesible

El instalador prueba la conexión antes de cambiar nada y se detiene con «Base de datos externa no accesible». Compruebe que el servidor de base de datos acepta conexiones desde esta IP (`bind-address`, cortafuegos), el usuario, la contraseña y que la base de datos existe, o indique una cuenta de administrador para crearlas (`--db-admin-user`, `AETHER_DB_ADMIN_PASSWORD`). Con `localhost`, `127.*` o `::1` el instalador se conecta a `host.docker.internal` (la máquina vista desde un contenedor), así que el MySQL del propio servidor debe escuchar en la IP del puente de Docker y no solo en `127.0.0.1` (`bind-address`); véase [Instalación](./01-installation.md).

### El panel no arranca después de instalar o actualizar

El instalador espera hasta 5 minutos y, si el contenedor se detiene o se reinicia tres veces, muestra sus últimas 40 líneas de log. En una actualización con `--update` vuelve solo a la versión anterior (código e imagen) y conserva los datos; la copia de la base de datos queda en `/var/backups/aether-panel/actualizaciones/`. La actualización desde **Configuración → Actualizaciones** no tiene esa vuelta atrás: si falla, ejecute `install.sh --update` desde el servidor.

Si el log termina con `OpenAt2 is not supported. Cowardly not starting to avoid security issues.`, el kernel no tiene `openat2` o un filtro seccomp (por ejemplo, un perfil de Docker antiguo) la bloquea con `EPERM`. Actualice el kernel o Docker; si no es posible, véase [`security.forceOpenat`](../general/20-security-unshare.md#alternativa-securityforceopenat).

---

## Base de datos y arranque del panel

### «MySQL no disponible aún» / «MySQL no está disponible después de 60 intentos»

El `entrypoint` de la imagen espera hasta 60 segundos a que responda el host y puerto de `SKYPANEL_PANEL_DATABASE_URL` (`mysql:3306` si no se indica) y, si no, se detiene. Revise `docker compose logs mysql`. Esta espera ocurre siempre, también con SQLite o con el panel desactivado: la imagen necesita un MySQL/MariaDB alcanzable.

### «Access denied» tras cambiar el `.env`

MariaDB solo aplica `DB_ROOT_PASSWORD` y `DB_PASSWORD` al crear la base de datos. Si se cambian después en `.env`, el panel deja de poder entrar. Devuelva los valores anteriores (están en `/etc/SkyPanel/credenciales.txt` si los generó el instalador) o cambie las contraseñas de forma coordinada en MariaDB y en `.env`:

```bash
sudo bash install.sh --update --rotate-db-passwords
```

### «AVISO: la base de datos usa la contraseña por defecto»

El `entrypoint` lo muestra si la URL de la base de datos contiene `skypanel_secret`, el valor público del repositorio. Los servidores de juego comparten red con MariaDB, así que cámbiela con `--rotate-db-passwords`.

### Otros errores de conexión

- `unknown dialect …`: `panel.database.dialect` debe ser `sqlite3`, `mysql`, `postgresql` o `sqlserver`.
- `error connecting to database` / `database not available`: la URL de conexión (`panel.database.url`) es incorrecta o el servidor no responde.
- PostgreSQL y SQL Server: compruebe que aceptan conexiones TCP desde el contenedor (`pg_hba.conf`, TCP/IP habilitado).

### Migraciones

El panel aplica las migraciones del esquema al arrancar (`SkyPanel run`); el usuario de la base de datos necesita permiso para crear y modificar tablas. Para ejecutarlas a mano:

```bash
docker exec skypanel /SkyPanel/bin/SkyPanel db upgrade
```

Con SQLite, `db upgrade` copia antes el archivo (`<archivo>.N.backup`) y lo restaura si falla. Con MySQL no hace copia: use la del instalador (`--update` la hace) o `mariadb-dump`. El paso «Ejecutando migraciones» del `entrypoint` llama a `SkyPanel db migrate`, que aplica las migraciones pendientes (las mismas que `run` al arrancar) y termina con un código distinto de cero si fallan; el `entrypoint` muestra entonces `ERROR: Fallo en migraciones` y sigue arrancando, y `run` vuelve a intentarlo. Con el panel desactivado (nodo secundario), `db migrate` no hace nada.

---

## Acceso al panel

### `403 {"error":"cross-origin request refused"}`

El panel rechaza las peticiones que cambian algo (y los websockets de la consola) cuando llevan su cookie de sesión pero las inició otra web, por ejemplo una página servida por un servidor de juego en la misma IP. Se considera mismo origen si `Sec-Fetch-Site` es `same-origin`, o si el `Origin` coincide con el `Host`, con `X-Forwarded-Host` o con la URL principal (`panel.settings.masterUrl`). Si ocurre al usar el panel normalmente detrás de un proxy propio, haga que el proxy conserve la cabecera `Host` (`proxy_set_header Host $host;` en Nginx) o que la URL principal de Configuración sea la dirección con la que se entra. Las peticiones con token Bearer, del daemon o de clientes OAuth2 no se ven afectadas.

### «too many failed attempts, wait a few minutes and try again»

Tras 10 intentos fallidos en 15 minutos, el inicio de sesión de esa cuenta (contraseña o código 2FA) se bloquea hasta que salgan intentos de la ventana. Las contraseñas de SFTP comparten el mismo contador; las comprobaciones de claves SSH para nodos remotos tienen uno aparte (véase [Límite de intentos](../general/security.md#límite-de-intentos)). El contador está en memoria: se vacía al reiniciar el panel.

### `http://IP:8080` ya no responde tras configurar un dominio

Es lo esperado: con dominio y Nginx, el instalador publica el puerto del panel solo en `127.0.0.1` y el panel se usa por el dominio. Para volver a abrirlo, `install.sh --update --bind 0.0.0.0`. Ver [Dominio, HTTPS y Nginx](./01-installation.md#dominio-https-y-nginx).

### HTTPS sin el instalador

El panel solo escucha en HTTP: el HTTPS lo pone un proxy inverso. El instalador configura Nginx y Let's Encrypt; en un despliegue propio, por ejemplo:

```nginx
server {
    listen 443 ssl;
    server_name panel.su-dominio.com;

    ssl_certificate     /etc/ssl/certs/panel.crt;
    ssl_certificate_key /etc/ssl/private/panel.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Para que el panel vea la IP real de los visitantes (límites de intentos, captcha, registros), indique el proxy en `config.json`:

```json
{
  "security": {
    "trustedProxies": ["127.0.0.1/32"],
    "trustedProxyHeader": "X-Forwarded-For"
  }
}
```

---

## Servidores de juego

### Puerto ya ocupado

El asistente de creación comprueba los puertos en el nodo antes de crear nada y avisa si los usa el propio panel (web, SFTP, base de datos, Adminer), otro servidor u otro contenedor, o si son menores de 1024, y propone el siguiente libre. Si un puerto se ocupa después, el arranque falla con un mensaje de Docker como `driver failed programming external connectivity: port is already allocated`: cambie el puerto del servidor o detenga lo que lo usa (`ss -tlnup | grep PUERTO`).

### La instalación se rechaza por falta de disco en el nodo

`ErrNodeDiskFull` («the node is almost out of disk space, free some before installing»), y en la consola: «The node only has X GB free, and an install needs at least Y GB». Una instalación no empieza si en la carpeta de servidores del nodo quedan menos de `daemon.minFreeDiskMB` MB libres (2048 por defecto; `0` desactiva la comprobación), para que una descarga no llene el disco y pare todos los servidores. Libere espacio o amplíe el disco. El panel muestra además un aviso de disco bajo en el inicio y en Nodos cuando quedan menos de 5 GB o menos del 10 %.

### El servidor no arranca: «the server has used all of its disk space»

`ErrDiskLimitExceeded`: el servidor ya usa todo su límite de disco y no arranca hasta que se borren archivos (las subidas por SFTP también se cortan al llegar al límite). Borre archivos o pida que se amplíe el límite. Ver [Límite de disco](../backend/04-servers.md#límite-de-disco).

### «environment "tty" is disabled on this node (docker.disallowHost)»

Con `docker.disallowHost` activo (lo está en la imagen Docker y en `docker-compose.yml`), el nodo solo ejecuta servidores en contenedores: los entornos `host`, `tty`, `standard` y `bubblewrap` se ocultan, se rechazan al crear o redefinir un servidor y, si una definición llega por otro camino (una copia restaurada, un panel antiguo), el arranque se niega con «This node only runs servers in Docker containers (docker.disallowHost).». Cambie el servidor a un entorno Docker.

### Una descarga de la plantilla se rechaza

`refusing to download from private address … (security.blockPrivateDownloads)`: por defecto las plantillas no pueden descargar de direcciones de red privada (las redes Docker, los servicios del host, la LAN), porque algunas toman la URL de una variable que edita el dueño del servidor. Las direcciones de loopback y link-local (como 169.254.169.254) se rechazan siempre (`refusing to download from internal address …`). Si tiene réplicas en su LAN, ponga `security.blockPrivateDownloads` a `false`.

### Errores de permisos dentro del juego

En Docker, con el panel como root (así funciona `docker-compose.yml`), las instalaciones se ejecutan como root y el juego como `docker.serverUser` (`988:988` por defecto), sin las capacidades de Linux que no necesita, con `no-new-privileges` y un límite de procesos (`docker.pidsLimit`, 4096). Antes de cada arranque el panel da todos los archivos del servidor a ese usuario, así que reiniciar el servidor corrige los archivos que se dejaron con otro dueño. Una plantilla que fija su propio usuario lo conserva; `docker.serverUser` a `"0:0"` vuelve a ejecutar los juegos como root (no recomendado). Ver [Usuario y aislamiento del contenedor](../backend/04-servers.md#usuario-y-aislamiento-del-contenedor).

### No se puede crear otra copia: «this server already has as many backups as it may keep»

`ErrBackupLimit` (HTTP 409): el servidor ya tiene `panel.backups.maxPerServer` copias (10 por defecto). Las copias no cuentan en el disco del servidor, así que sin límite un cliente podría llenar el disco de copias del nodo. Borre una copia antigua. Los administradores no tienen límite, y `0` lo desactiva para todos.

### La restauración de una copia falla: «cannot backup server, is running»

Restaurar una copia exige el servidor detenido. Deténgalo y repita.

<a id="16-conectividad-mysql-desde-servidores-docker-mysql3306-no-resuelve"></a>

### Un servidor no llega a la base de datos (`mysql:3306`)

Los servidores de juego se crean en la red de Docker `docker.serversNetwork` (`skypanel-servers` en `docker-compose.yml`), salvo que la plantilla indique otra. MariaDB está también en esa red, así que los servidores la alcanzan como `mysql:3306`, pero no llegan al panel. El panel también está en esa red para hablar con los servidores (mapa del mundo, jugadores). Si esa red no existe, el panel lo registra y usa su propia red. Sin `docker.serversNetwork`, los servidores se unen a la red del contenedor del panel.

```bash
docker inspect <id-del-servidor> --format '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}'
# Debe mostrar: skypanel-servers
```

Un servidor creado antes de ese cambio puede seguir en otra red: elimínelo y vuelva a crearlo.

### Aislamiento con unshare: «Permission denied while starting the security jail»

Solo afecta al entorno `tty` (ejecución directa en el host), que no está disponible en la instalación con Docker. Ubuntu 24.04 y posteriores restringen los *user namespaces* sin privilegios que usa el aislamiento. La consola muestra las dos soluciones:

```bash
sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0
```

o desactivar el aislamiento con `"security": {"disableUnshare": true}` en `config.json` (o `"disableUnshare": true` en el entorno de un servidor). Ver [Seguridad y unshare](../general/20-security-unshare.md).

---

## SFTP

El puerto por defecto es `5657` (`daemon.sftp.host`). El usuario es `email#idDelServidor` (por ejemplo `usuario@ejemplo.com#a1b2c3d4`) y la contraseña, la de la cuenta del panel o una clave SSH pública añadida al perfil. Las claves funcionan en todos los nodos: un nodo secundario pide al panel que las compruebe (`POST /auth/sftp-key`, véase [Claves SSH para SFTP](../general/security.md#claves-ssh-para-sftp)).

| Error | Causa y solución |
|---|---|
| `incorrect username or password` | Credenciales incorrectas, formato de usuario incorrecto o el usuario no tiene el permiso `server.sftp` en ese servidor. |
| `too many failed attempts…` | Contraseñas: se agotaron los 10 intentos en 15 minutos que comparten con el inicio de sesión del panel. Las claves SSH tienen sus propios 10 intentos en 15 minutos por cuenta, contados en todos los nodos (cada clave que ofrece el cliente es un intento); espere u ofrezca solo la clave correcta (`sftp -o IdentitiesOnly=yes -i <clave> …`). |
| `invalid response from authorization server` (y `error talking to auth server` en el log) | En un nodo secundario: `daemon.auth.url`, `daemon.auth.clientId` o `daemon.auth.clientSecret` son incorrectos o el maestro no responde. Ver [nodo secundario](./01-installation.md#método-3-nodo-secundario-solo-daemon). |
| Una clave SSH se rechaza en un nodo secundario aunque la contraseña funciona | El nodo tiene una versión anterior a la comprobación de claves en nodos remotos, que solo ofrece acceso con contraseña: actualícelo. En un nodo actual, su log muestra `Cannot check SFTP keys with the panel` cuando `daemon.auth.url` no es una URL `http(s)` absoluta, y `Error checking an SFTP key with the auth server: [<estado>]` cuando el panel rechazó al nodo (`401`: `daemon.auth.clientSecret` incorrecto; `429`: demasiados intentos con claves para esa cuenta) o falló. Una clave que simplemente no es del usuario, o un usuario sin `server.sftp`, se rechaza igual que una contraseña incorrecta. |
| `connection refused` | El puerto no está abierto o el panel no está en marcha: `ss -tlnp \| grep 5657`. |

---

## Nodos secundarios

- **El navegador muestra 404 en `http://nodo:8080`.** Es lo esperado: con `panel.enable=false` solo funciona la API del daemon.
- **«Panel is disabled but token.public (TokenPublicURL) is empty!»** El nodo no puede validar las peticiones del maestro. Ponga `SKYPANEL_TOKEN_PUBLIC=https://<maestro>/auth/publickey`.
- **El nodo no se autentica con el secreto del diálogo Desplegar.** El diálogo muestra un secreto fijo; el real lo devuelve `GET /api/nodes/{id}/deployment`.
- **`502` «El nodo rechazó las credenciales del panel» (`ErrNodeAuthFailed`).** El nodo respondió al panel con `401` (o con un `403` que pide autenticación), así que el panel lo indica como un problema del nodo en lugar de cerrar la sesión del usuario. El log del panel nombra el nodo: `Node "<nombre>" refused the panel's token (status 401)…`, y el log del nodo muestra `Refused a request with an invalid panel token: …`. Compruebe que el `daemon.auth.clientSecret` del nodo es el secreto que muestra el panel para ese nodo (`GET /api/nodes/{id}/deployment`), que su `token.public` apunta al `/auth/publickey` de este panel, que la hora de ambas máquinas es correcta (use NTP: los tokens del panel valen 15 minutos) y que ningún proxy delante del nodo pide sus propias credenciales.
- **Los servidores de un nodo no tienen historial de disponibilidad, o su informe de nodo en Discord dice «Sin respuesta».** El panel consulta cada nodo remoto cada 30 segundos con `GET /daemon/servers/status`. El log del panel dice `Node <nombre> did not answer the server status poll…` con el motivo: un nodo caído o inalcanzable, uno que rechaza el token del panel (véase el punto anterior) o uno con una versión anterior, que responde `status 404` y debe actualizarse. Ver [Seguimiento de disponibilidad](../general/13-uptime.md#cómo-se-registra).
- **Una transferencia entre nodos falla al momento y se deshace, con `pausing its tasks on the old node` (o `on the new node`) y `the node answered 404` en la actividad del servidor.** Un daemon anterior a las rutas para pausar las tareas no puede formar parte de una [transferencia entre nodos](../features/server-transfers.md), ni como origen ni como destino: el panel primero pausa las tareas programadas del servidor en ambos nodos. Actualice ese nodo y vuelva a transferir.
- **Un reinicio del panel interrumpió una transferencia entre nodos.** El panel la deshace (o la completa, si ya había apuntado el servidor al nodo nuevo) al arrancar; la actividad del servidor muestra `the panel restarted while …`. Las llamadas a un nodo que esté apagado en ese momento no se reintentan: el log del panel las indica (`Transfer of <id>: …`), y puede quedar en ese nodo una copia o un archivo comprimido sobrante.
- **La configuración del nodo desaparece al actualizar.** Solo se conservan las líneas `SKYPANEL_PANEL_ENABLE=false` y `SKYPANEL_TOKEN_PUBLIC=…` de `docker-compose.yml`; el resto debe ir en el `config.json` de `storage/`.

---

## Docker

### El panel no puede conectarse a Docker

El panel usa el cliente de Docker con las variables estándar (`DOCKER_HOST`; por defecto `unix:///var/run/docker.sock`). En `docker-compose.yml` el socket del host se monta en el contenedor, que se ejecuta como root (`user: "0:0"`). Si el daemon no puede iniciarse dentro de Docker, el panel web sigue funcionando y lo registra («Continuing without daemon in Docker environment»), pero los servidores no estarán disponibles. Compruebe:

```bash
docker info
docker exec skypanel ls -la /var/run/docker.sock
```

Más detalles en [Socket de Docker y permisos](../docker/DOCKER-SUDO-FIX.md).

---

## Configuración

### El panel no usa los valores configurados

El panel lee un único `config.json`: el de `--config`, el de la variable `SKYPANEL_CONFIG`, `/etc/SkyPanel/config.json` si existe o, si no, `config.json` en el directorio de trabajo. En la instalación con Docker es `storage/skypanel-data/config.json` (o `storage/skypanel-config/config.json` si existe). Los cambios de **Configuración** se guardan en ese archivo.

Cualquier clave se puede sobrescribir con una variable de entorno: prefijo `SKYPANEL_`, la clave en mayúsculas y los puntos cambiados por `_`.

| Variable de entorno | Clave |
|---|---|
| `SKYPANEL_WEB_HOST` | `web.host` |
| `SKYPANEL_DAEMON_SFTP_HOST` | `daemon.sftp.host` |
| `SKYPANEL_PANEL_DATABASE_URL` | `panel.database.url` |
| `SKYPANEL_DOCKER_DISALLOWHOST` | `docker.disallowHost` |
| `SKYPANEL_LOGRETENTIONDAYS` | `logRetentionDays` |

Las variables de entorno **tienen prioridad** sobre `config.json`, también sobre lo guardado en Configuración: un valor fijado en `docker-compose.yml` gana siempre y el cambio hecho en Configuración no tendrá efecto. Por eso `docker-compose.yml` no fija el registro público ni el tema. La lista de claves está en [Configuración](../backend/02-configuration.md).

### Plantillas que no se descargan

El índice se carga pero fallan las plantillas individuales: la URL de `templates.url` (en `docker-compose.yml`, `https://templates.aetherpanel.es/templates.json`) apunta a un servidor que solo tiene `templates.json`. Cada ruta que referencia el índice (por ejemplo `minecraft/minecraft.json`) debe existir bajo la misma ruta base.

### El asistente de IA no responde

Falta la clave de Google Gemini (`panel.settings.geminiApiKey`, o `SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY`). Ver [Asistente de IA](../frontend/04-ai.md).

---

## Logs

El panel escribe el día actual en `skypanel.log` dentro de la carpeta `logs` (`/var/log/SkyPanel` en la imagen; `storage/skypanel-logs` en el host con Docker Compose). La rotación es diaria y no necesita `logrotate`:

- Al cambiar de día el archivo pasa a `skypanel-AAAA-MM-DD.log.gz` y se empieza uno nuevo.
- Si un día pasa de 100 MB, se archiva antes como `skypanel-AAAA-MM-DD.1.log.gz`, `.2`, etc.
- Los archivos de más de `logRetentionDays` días (14 por defecto) se borran solos; con `0` se guardan todos. Solo se tocan los archivos `skypanel-*`.

| Prefijo | Salida |
|---|---|
| `[ERROR]` | stderr y archivo |
| `[INFO]`, `[DEBUG]`, `[SERVER]` | stdout y archivo |

```bash
tail -f /opt/skypanel/storage/skypanel-logs/skypanel.log
zcat /opt/skypanel/storage/skypanel-logs/skypanel-2026-10-25.log.gz | grep ERROR
```

Con un `logrotate` externo, ponga `logRetentionDays` a `0` y, tras rotar, envíe `SIGUSR1` al proceso para que reabra `skypanel.log`. El modo de Gin lo fija `GIN_MODE` (`release` en Docker; `debug` muestra todas las rutas HTTP).
