# Guía de Instalación

Esta guía explica cómo instalar, actualizar y desinstalar Aether Panel, y cómo preparar un nodo secundario.

> **Solo Docker.** Aether Panel se instala únicamente con Docker: no hay paquetes ni binarios sueltos para instalar en el sistema. El instalador y el `docker-compose.yml` del repositorio construyen la imagen a partir del `Dockerfile` en el propio servidor. Los detalles de la imagen y de los archivos de Compose están en [Documentación Docker](../docker/README.docker.md).

## Requisitos del sistema

| Requisito | Detalle |
|---|---|
| **Sistema operativo** | Linux con `apt`, `dnf`, `pacman` o `zypper`. El instalador reconoce Ubuntu, Debian, Fedora, RHEL/Rocky/AlmaLinux/Oracle Linux, Arch (y Manjaro, EndeavourOS…) y openSUSE/SLES, además de sus derivadas. Se ha probado en Ubuntu 22.04, 24.04, 26.04 y 26.10, Debian 12, Rocky Linux 9, Arch Linux y openSUSE Tumbleweed. Avisa (sin detenerse) en Ubuntu anterior a 22.04, Debian anterior a 12 y RHEL/Rocky/Alma anteriores a 8, que ya no tienen soporte. |
| **Arquitectura** | `amd64` o `arm64`. Cualquier otra se rechaza. |
| **Memoria** | 2 GB recomendados: la imagen se compila en el servidor y eso consume mucha memoria. Con menos de 1 GB pide confirmación; con menos de 3 GB entre RAM y swap avisa de que la compilación puede fallar y sugiere crear swap. |
| **Disco** | Se recomiendan 10 GB libres en `/opt` (código y datos) y en `/var/lib/docker` (imágenes). Con menos, pide confirmación. |
| **Docker** | Docker Engine y Docker Compose v2. Si faltan, el instalador los instala (repositorio oficial de Docker con la huella de su clave GPG comprobada; en Arch y openSUSE, los paquetes de la distribución). `docker-compose` v1 no es compatible. |
| **Docker Compose 2.24.4 o posterior** | Necesario siempre que el instalador genere `docker-compose.override.yml` con puertos: al cambiar `--port` o `--sftp-port`, al usar `--bind` (también el `127.0.0.1` automático cuando hay dominio) o cuando el puerto 3307 de MariaDB está ocupado. Con una versión anterior el instalador se detiene con un mensaje. |
| **systemd** | Recomendado. Sin él, el instalador no puede habilitar Docker ni Nginx ni instalar el servicio `aether-domain`. |
| **Acceso** | Usuario `root` (o `sudo`) y salida a Internet (GitHub, Docker Hub, Let's Encrypt). |

### Puertos

| Puerto | Servicio | Dónde escucha |
|---|---|---|
| `8080/tcp` | Panel web y API (y API del daemon para el panel maestro) | Todas las interfaces; solo `127.0.0.1` cuando el panel se sirve por un dominio con Nginx |
| `5657/tcp` | SFTP integrado | Todas las interfaces |
| `80/tcp`, `443/tcp` | Nginx delante del panel (443 solo con dominio) | Todas las interfaces |
| `3307/tcp` | MariaDB del panel | Solo `127.0.0.1` (si está ocupado, el instalador usa el primero libre entre 3308 y 3399) |
| `8081/tcp` | Adminer (administración de la base de datos) | Solo `127.0.0.1`: úselo con un túnel SSH (`ssh -L 8081:127.0.0.1:8081 …`) |
| Puertos de juego | Cada servidor de juego (por ejemplo, 25565 para Minecraft) | Los publica Docker al crear cada servidor |

El instalador abre en `ufw` (Debian/Ubuntu/Arch) o `firewalld` (RHEL/Fedora/openSUSE) el SSH, el panel, el SFTP y los puertos de Nginx. Docker publica los puertos de sus contenedores por delante de esos cortafuegos, así que también hay que abrirlos en el cortafuegos del proveedor (Hetzner, OVH, AWS…), junto con los puertos de los servidores de juego.

---

## Método 1: instalador automático (recomendado)

El instalador (`tools/install/install.sh`, versión 4.2.2) instala Docker si hace falta, descarga el código, genera las contraseñas, construye la imagen, configura Nginx, HTTPS y el cortafuegos, crea la cuenta de administrador y arranca el panel.

### Ejecutarlo

Asistente interactivo, como `root` (pregunta todo paso a paso):

```bash
sudo -i
bash -c "$(curl -fsSL https://install.aetherpanel.es/install.sh)"
```

Descargar, revisar y ejecutar:

```bash
curl -fsSL https://install.aetherpanel.es/install.sh -o /tmp/install.sh
sudo bash /tmp/install.sh --help
sudo bash /tmp/install.sh
```

Sin preguntas (por ejemplo, desde un script):

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --non-interactive [opciones]
```

> Evite `bash <(curl …)`: la sustitución de procesos (`/dev/fd/N`) no existe en todos los entornos.

### Modos

| Modo | Cómo | Qué pregunta |
|---|---|---|
| Manual | `sudo bash install.sh` en una terminal | Todo: dominio y HTTPS, puertos, base de datos y cuenta de administrador |
| Automático | `-y` / `--yes` | Solo los datos sensibles (email y contraseña del administrador); el resto sale de las opciones o de los valores por defecto. `-y` también confirma los borrados |
| Sin preguntas | `--non-interactive`, o la entrada no es una terminal (`curl … \| sudo bash`) | Nada. Sin `--admin-email` ni dominio no se crea la cuenta: el resumen final explica cómo crearla |
| Simulación | `--dry-run` | Muestra lo que haría sin cambiar el sistema |

Las opciones se toman en este orden: argumentos, variables de entorno `AETHER_*`, la instalación anterior (`/etc/SkyPanel/installer.conf`) y los valores por defecto.

### Qué hace

1. Comprueba el sistema: distribución, arquitectura, memoria, disco y si ya hay una instalación (y si el contenedor `skypanel` existente pertenece a ella; si es de otro proyecto, se detiene sin tocarlo).
2. Pregunta o lee la configuración (dominio y HTTPS, puertos, base de datos, administrador) y la valida entera antes de cambiar nada.
3. Instala las dependencias (`git`, `jq`, Nginx, `certbot` si hay HTTPS, `ufw` o `firewalld`) y Docker si no está.
4. Comprueba que los puertos están libres y dice qué los ocupa (otro contenedor, otro proceso).
5. Clona `https://github.com/Aether-Panel/Panel.git` en `/opt/skypanel` (rama `main`, la ya instalada al repetirlo, u otra con `-b RAMA`). Si había cambios locales en el repositorio, guarda un parche en `/etc/SkyPanel/backups/` antes de descartarlos.
6. Genera contraseñas propias para MariaDB en `/opt/skypanel/.env` (solo legible por `root`) y, si hace falta, `docker-compose.override.yml`.
7. Construye la imagen (de 5 a 15 minutos la primera vez), configura el cortafuegos y Nginx, y pide el certificado de Let's Encrypt.
8. Escribe en el `config.json` del panel la URL principal y las opciones indicadas (SFTP, Turnstile, Cloudflare, S3, CurseForge, retención de logs) y, detrás de Nginx, confía en la puerta de enlace de la red Docker como proxy (`security.trustedProxies`) para ver la IP real de los visitantes.
9. Arranca el panel, espera a que responda (hasta 5 minutos), crea la cuenta de administrador e instala el servicio `aether-domain`.
10. Muestra un resumen: dirección del panel, puertos que abrir en el proveedor, contenedores y dónde están las credenciales.

Volver a ejecutarlo sobre una instalación existente es seguro: conserva los datos, las contraseñas de la base de datos y las cuentas.

### Qué deja instalado

| Ruta | Contenido |
|---|---|
| `/opt/skypanel` | Clon del repositorio, desde donde se ejecuta `docker compose` |
| `/opt/skypanel/.env` | Contraseñas de la base de datos (permisos 600) |
| `/opt/skypanel/docker-compose.override.yml` | Solo con puertos propios, `--bind` o base de datos externa. Está fuera de git y sobrevive a las actualizaciones |
| `/opt/skypanel/storage/` | **Todos los datos**: MariaDB (`mysql-data`), servidores, copias, extensiones y `config.json` del panel (`skypanel-data`), logs (`skypanel-logs`) |
| `/etc/SkyPanel/installer.conf` | Opciones de la instalación, sin secretos; se usan como valores por defecto al repetir |
| `/etc/SkyPanel/credenciales.txt` | Contraseñas generadas (permisos 600). Bórrelo cuando las tenga a salvo |
| `/var/backups/aether-panel/` | Copias que hace el instalador antes de actualizar, reinstalar o desinstalar |
| `/etc/systemd/system/aether-domain.{path,service}` | Servicio que aplica la URL principal guardada en Configuración |

### Administrar el panel con el instalador

Sin opciones y en una terminal, el instalador dice si el panel **no está instalado**, está **instalado pero detenido** o **funcionando** (con su rama y versión) y ofrece un menú:

| Opción | Qué hace | Datos |
|---|---|---|
| **Instalar** | Pregunta dominio o IP, puertos y base de datos (interna o externa) e instala todo. | — |
| **Actualizar** (`--update`) | Copia la base de datos y comprueba la copia, descarga la rama, reconstruye la imagen y reinicia. Si la versión nueva no arranca, vuelve sola al código y a la imagen anteriores. | Se conservan |
| **Reconfigurar** (sobre una instalación existente) | Cambia dominio, puertos, etc. | Se conservan |
| **Reinstalar** (`--reinstall`) | Ofrece una copia, pide escribir `REINSTALAR` (sin terminal, `--yes`), borra la instalación y vuelve a instalar desde cero. | Se borran (una BD externa no) |
| **Desinstalar** (`--uninstall`, `--uninstall-data`, `--uninstall-all`) | Ver [Desinstalar](#desinstalar). | Según la opción |
| **Crear cuenta administrador** (`--create-admin`) | Crea un administrador en una instalación en marcha. | — |

### Actualizar

Hay dos formas de actualizar, que usan la misma rama instalada (nunca la cambian):

| | Instalador (`--update`) | Panel (**Configuración → Actualizaciones → Actualizar ahora**) |
|---|---|---|
| Ejecuta | `install.sh --update` | `tools/panelUpdate/panelUpdate.sh` en cada nodo, desde un contenedor auxiliar |
| Copia de la base de datos antes | Sí, comprobada (salvo `--skip-backup`). Sin copia válida no sigue | No |
| Vuelta atrás si la versión nueva no arranca | Sí (código e imagen anteriores, guardada como `:anterior`) | No |
| Conserva | `.env`, `docker-compose.override.yml`, la configuración de nodo secundario | `.env` y la configuración de nodo secundario |

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --update
```

Ambas compilan la imagen nueva con el panel aún en marcha y solo reinician al terminar. `panelUpdate.sh` además libera la caché de compilación, dejando 3 GB. El funcionamiento de la actualización desde el panel se describe en [Actualizaciones del panel](../features/panel-updates.md).

`--update` no reconfigura Nginx ni el certificado: para cambiar de dominio, ejecute el instalador sin `--update` (o use la URL principal de Configuración, ver más abajo).

### Desinstalar

| Opción | Elimina | Conserva |
|---|---|---|
| `--uninstall` | Contenedores y sitio de Nginx | Código, `.env` y todos los datos. Se vuelve a poner en marcha con `--update` |
| `--uninstall-data` | Contenedores, servidores de juego y datos (`storage/`) | Código, `.env` y opciones |
| `--uninstall-all` | Todo lo del panel: contenedores, imagen, redes, servidores de juego, `/opt/skypanel`, `/etc/SkyPanel`, sitio de Nginx, certificado del panel, servicio `aether-domain` y reglas del cortafuegos del panel y del SFTP | Docker, Nginx, `certbot` y las reglas de SSH, 80 y 443 |

Las opciones que borran datos piden escribir `BORRAR` en una terminal, o `--yes` sin ella, y en modo manual ofrecen antes una copia. Solo se elimina lo que pertenece a la instalación: los contenedores que `docker compose` creó desde `/opt/skypanel` y los servidores de juego del panel (etiqueta `skypanel.server`). Una base de datos externa nunca se modifica ni se borra.

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --uninstall
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --uninstall-all --yes
```

### Base de datos

**Interna (por defecto).** MariaDB 10.11 en el contenedor `skypanel-mysql`, con datos en `storage/mysql-data`. Las contraseñas se generan al crear la base de datos y no cambian al repetir el instalador (las opciones `--db-pass`/`--db-root-pass` se ignoran entonces). Para cambiarlas:

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --update --rotate-db-passwords
```

`--rotate-db-passwords` guarda primero las contraseñas nuevas en `credenciales.txt`, las aplica en MariaDB y actualiza `.env`. El instalador avisa si la base de datos usa alguna de las contraseñas por defecto públicas del repositorio (como `skypanel_secret`), porque los servidores de juego comparten red con MariaDB.

**Externa** (`--db-mode external`). Se comprueba la conexión antes de instalar. Si la base de datos o el usuario no existen, el instalador puede crearlos con una cuenta de administrador de MySQL (`--db-admin-user root --db-admin-pass …`, mejor con la variable `AETHER_DB_ADMIN_PASSWORD`), que no se guarda. Si el host es `localhost`, `127.*` o `::1` (una base de datos en el propio servidor), el instalador lo cambia por `host.docker.internal`, porque dentro del contenedor del panel `localhost` es el propio contenedor: lo escribe así en el `.env`, añade `host.docker.internal:host-gateway` al contenedor del panel (`docker-compose.override.yml`) y comprueba la conexión, y hace la copia de seguridad de `--update`, desde un contenedor con `--add-host host.docker.internal:host-gateway`. En ese caso MySQL tiene que escuchar en la IP del puente de Docker (`bind-address`), y el instalador lo recuerda. Para una base de datos en otra máquina, use su IP privada o su nombre. La contraseña no puede contener espacios, comillas, `$`, `\`, `` ` `` ni `#`.

### Dominio, HTTPS y Nginx

- **Sin dominio**, Nginx contesta en el puerto 80 por la IP (y por cualquier dominio que apunte al servidor) y el panel sigue disponible en `http://IP:8080`.
- **Con dominio** (`--domain`), Nginx sirve el panel en ese dominio; con `--ssl`, pide un certificado de Let's Encrypt validado por HTTP en el puerto 80 (se renueva solo y Nginx se recarga tras cada renovación). Si la emisión falla, el panel queda por HTTP y el resumen indica cómo reintentarlo.
- **Puerto del panel cerrado tras un dominio.** Con dominio y Nginx, el instalador (también con `--update`) publica el puerto del panel solo en `127.0.0.1`: así nadie entra por `http://IP:8080` saltándose el HTTPS (Docker publica sus puertos por delante de `ufw`, así que el cortafuegos no bastaba). Para dejarlo abierto a propósito, `--bind 0.0.0.0`.
- **Cloudflare.** El certificado también se emite con el proxy de Cloudflare activo si el puerto 80 llega al servidor; la primera emisión falla con "Always Use HTTPS" activado. En modo SSL "Full", Cloudflare entra por el 443 y hace falta el certificado (URL con `https://`). Los juegos y el SFTP no pasan por el proxy de Cloudflare: si el dominio no apunta a la IP del servidor, el panel les da la IP.

> Un panel servido solo por `http://IP:8080` envía la cookie de sesión del administrador a cualquier puerto de esa IP, también al servidor de juego de un cliente. Para un panel con clientes, use un dominio con HTTPS.

**Añadir o cambiar el dominio después.** Apunte el dominio a la IP del servidor y escríbalo en **Configuración → General → URL principal**, por ejemplo `https://panel.ejemplo.com`. Al guardarlo, el servicio `aether-domain` (una unidad `.path` de systemd que vigila el `config.json` del panel y ejecuta `install.sh --apply-panel-url`) configura Nginx, el certificado y el cortafuegos en unos segundos, y el resultado aparece bajo el campo. Con `http://` se configura sin certificado; con una IP, Nginx vuelve a contestar por la IP; una URL con puerto (`http://dominio:8080`) no toca Nginx. Si el certificado falla, el panel sigue por HTTP y el mismo intento no se repite hasta pasada una hora o hasta cambiar la URL.

Este servicio no cambia la publicación del puerto 8080: tras añadir el dominio así, ejecute `--update` para que el puerto quede solo en `127.0.0.1`. El servicio lo instalan el instalador 4.1.3 y posteriores (también con `--update`), solo con systemd y sin `--skip-nginx`. En una instalación sin él:

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --domain panel.ejemplo.com --ssl
```

### Cambiar los puertos

`--port` y `--sftp-port` cambian los puertos publicados mediante `docker-compose.override.yml` (necesita Docker Compose 2.24.4 o posterior). El puerto SFTP también se escribe en `daemon.sftp.host`, porque el panel anuncia ese puerto a los usuarios. El instalador rechaza el puerto de SSH, el 80 y el 443 (con Nginx), el 3307 y el 8081, y los puertos que ya usa otro contenedor o proceso.

### Referencia de opciones

Todas las opciones con valor admiten `--opcion VALOR` y `--opcion=VALOR` (use la segunda forma para valores que empiezan por `-`).

**Instalación**

| Opción | Descripción | Por defecto |
|---|---|---|
| `-b`, `--branch RAMA` | Rama de git | `main`, o la ya instalada |
| `-d`, `--domain DOMINIO` | Dominio del panel (Nginx en 80/443) | — |
| `--no-domain` | Quitar el dominio de una instalación anterior | — |
| `-s`, `--ssl` / `--no-ssl` | Activar o desactivar HTTPS con Let's Encrypt (requiere dominio) | Desactivado |
| `-e`, `--email EMAIL` | Email para Let's Encrypt | — |
| `-p`, `--port PUERTO` | Puerto del panel | `8080` |
| `--sftp-port PUERTO` | Puerto SFTP | `5657` |
| `--bind IP` | Publicar el panel solo en esa IPv4 | `0.0.0.0`; `127.0.0.1` con dominio y Nginx |
| `--ip IP` | IP del servidor para la URL | Autodetectada (la local sin dominio, la pública con dominio) |
| `--master-url URL` | URL pública del panel (enlaces de recuperación de contraseña y de acceso directo) | Según dominio y HTTPS, o `http://IP:puerto` |
| `--db-mode local\|external` | Base de datos | `local` |
| `--db-host`, `--db-port`, `--db-name`, `--db-user` | Conexión de la base de datos | Puerto `3306`, nombre y usuario `skypanel` |
| `--db-pass`, `--db-root-pass` | Contraseñas (la de root solo en modo local) | Generadas |
| `--rotate-db-passwords` | Cambiar las contraseñas de la base de datos local ya creada | — |
| `--admin-email`, `--admin-pass`, `--admin-name` | Cuenta de administrador inicial (contraseña de 8 a 72 bytes) | Contraseña generada, nombre `admin` |
| `--no-admin` | No crear cuenta de administrador | — |
| `--replace-admin` | Borrar y recrear un usuario con ese email o nombre | — |
| `-f`, `--force` | Recrear los contenedores (los datos se conservan) y seguir aunque haya puertos ocupados | — |
| `-y`, `--yes` / `--non-interactive` / `--dry-run` | Ver [Modos](#modos) | — |
| `--skip-deps`, `--skip-docker`, `--skip-firewall`, `--skip-nginx` | Omitir ese paso | — |

**Funciones del panel** (opcionales; se pueden cambiar después en Configuración)

| Opción | Descripción |
|---|---|
| `--turnstile-site-key`, `--turnstile-secret-key` | Cloudflare Turnstile en lugar del captcha integrado (hacen falta las dos) |
| `--cf-dns-token`, `--cf-zone-id`, `--dns-base-domain` | Subdominios automáticos con Cloudflare (token con permiso Zone DNS Edit) |
| `--dns-target DESTINO` | Destino de los registros (por defecto, la IP pública de cada nodo) |
| `--dns-auto-assign` | Asignar subdominio a los servidores nuevos |
| `--s3-endpoint`, `--s3-region`, `--s3-bucket`, `--s3-access-key`, `--s3-secret-key`, `--s3-prefix`, `--s3-insecure` | Copias de seguridad en S3 o compatible (MinIO, R2…). Necesitan al menos bucket y claves; prefijo por defecto del panel `aether-backups`; `--s3-insecure` para HTTP sin TLS |
| `--curseforge-key KEY` | Clave API de CurseForge |
| `--log-retention-days N` | Días de logs rotados que se conservan (por defecto del panel, 14; `0` = siempre) |

**Mantenimiento**

| Opción | Descripción |
|---|---|
| `--update` | Actualizar sin borrar datos |
| `--reinstall` | Borrar la instalación y volver a instalar |
| `--skip-backup` | No copiar la base de datos antes de actualizar o reinstalar |
| `--db-admin-user`, `--db-admin-pass` | Con BD externa: cuenta que crea la base de datos y el usuario si no existen |
| `-u`, `--uninstall`, `--uninstall-data`, `--uninstall-all` | Desinstalar |
| `--create-admin` | Solo crear una cuenta de administrador |
| `--apply-panel-url` | Aplicar la URL principal de Configuración (lo usa el servicio `aether-domain`) |
| `--debug`, `-V`/`--version`, `-h`/`--help` | Depuración, versión y ayuda |

**Variables de entorno.** Cada opción tiene su variable (`AETHER_DOMAIN`, `AETHER_PORT`, `AETHER_ADMIN_PASSWORD`, `AETHER_DB_PASSWORD`, `AETHER_S3_SECRET_KEY`…; la lista completa está en `--help`), además de `AETHER_YES`, `AETHER_NONINTERACTIVE`, `AETHER_DRY_RUN`, `AETHER_FORCE`, `AETHER_DEBUG` y `AETHER_REPO` (otro repositorio, para forks). Use variables para las contraseñas: los argumentos quedan en el historial y en `ps`, y el instalador avisa si se pasan así.

### Ejemplos

```bash
# Instalación desatendida con dominio y HTTPS
curl -fsSL https://install.aetherpanel.es/install.sh | sudo AETHER_ADMIN_PASSWORD='...' bash -s -- \
  --non-interactive --domain panel.ejemplo.com --ssl \
  --email admin@ejemplo.com --admin-email admin@ejemplo.com

# Ver qué haría, sin cambiar nada
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --dry-run

# Base de datos externa, creando la base de datos y el usuario
sudo AETHER_DB_PASSWORD='...' AETHER_DB_ADMIN_PASSWORD='...' bash install.sh \
  --db-mode external --db-host 10.0.0.5 --db-admin-user root

# Crear otro administrador en una instalación existente
sudo bash install.sh --create-admin --admin-email otro@ejemplo.com
```

---

## Método 2: Docker Compose a mano

Para controlar cada paso, clone el repositorio y use el `docker-compose.yml` incluido (MariaDB, panel y Adminer).

### 1. Clonar el repositorio

```bash
git clone -b main https://github.com/Aether-Panel/Panel.git /opt/skypanel
cd /opt/skypanel
```

Use `/opt/skypanel`: la actualización desde **Configuración → Actualizaciones** ejecuta `tools/panelUpdate/panelUpdate.sh` en esa ruta.

### 2. Poner contraseñas propias

El `docker-compose.yml` lee estas variables de `.env`:

| Variable | Valor sin `.env` | Uso |
|---|---|---|
| `DB_ROOT_PASSWORD` | `skypanel_secret` | Contraseña de root de MariaDB |
| `DB_DATABASE` | `skypanel` | Nombre de la base de datos |
| `DB_USER` | `skypanel` | Usuario de la base de datos |
| `DB_PASSWORD` | `skypanel_secret` | Contraseña del usuario |
| `DB_BIND_ADDRESS` | `127.0.0.1` | IP del host donde se publica MariaDB (puerto 3307) |

El `.env` del repositorio trae valores de ejemplo públicos. Cámbielos **antes del primer arranque**: MariaDB solo los aplica al crear la base de datos, y los servidores de juego comparten red con ella.

```bash
sed -i "s/^DB_ROOT_PASSWORD=.*/DB_ROOT_PASSWORD=$(openssl rand -hex 24)/; s/^DB_PASSWORD=.*/DB_PASSWORD=$(openssl rand -hex 24)/" .env
chmod 600 .env
```

### 3. Construir e iniciar

```bash
docker compose up -d --build
docker compose logs -f skypanel
```

El panel crea las tablas al arrancar. Para otros puertos, cree un `docker-compose.override.yml` con `ports: !override` (Docker Compose 2.24.4 o posterior).

### 4. Crear el administrador

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@ejemplo.com --admin
```

Si no se pasa `--password`, el comando la pide (así no queda en el historial).

Un despliegue manual no tiene Nginx, HTTPS ni el servicio `aether-domain`: póngalos usted (por ejemplo, con un proxy inverso delante de `127.0.0.1:8080`) y configure `security.trustedProxies` para que el panel vea la IP real. Ver [Configuración](../backend/02-configuration.md).

---

## Método 3: nodo secundario (solo daemon)

Un nodo secundario ejecuta servidores de juego en otra máquina y lo gestiona el panel maestro. En él, el panel web está desactivado (`panel.enable=false`) y solo funciona la API del daemon (`/daemon`) en el puerto HTTP, así que abrir ese puerto en el navegador da un 404: es lo esperado.

### 1. Crear el nodo en el panel maestro

En **Nodos**, cree el nodo con su host público, el puerto HTTP (8080) y el puerto SFTP. El diálogo **Desplegar** del nodo muestra los valores que necesita:

| Clave | Variable de entorno | Valor |
|---|---|---|
| `panel.enable` | `SKYPANEL_PANEL_ENABLE` | `false` |
| `token.public` | `SKYPANEL_TOKEN_PUBLIC` | `https://<maestro>/auth/publickey` |
| `daemon.auth.url` | `SKYPANEL_DAEMON_AUTH_URL` | `https://<maestro>/oauth2/token` |
| `daemon.auth.clientId` | `SKYPANEL_DAEMON_AUTH_CLIENTID` | `.node_<ID del nodo>` |
| `daemon.auth.clientSecret` | `SKYPANEL_DAEMON_AUTH_CLIENTSECRET` | El secreto del nodo |
| `daemon.sftp.host` | `SKYPANEL_DAEMON_SFTP_HOST` | `0.0.0.0:<puerto SFTP>` |

> **Atención: dos limitaciones del diálogo Desplegar en la versión actual.**
> - El `clientSecret` que muestra es un valor fijo, no el del nodo. El secreto real lo devuelve `GET /api/nodes/{id}/deployment` (permiso `nodes.deploy`), en el campo `clientSecret`.
> - El `docker-compose.yml` que propone no incluye base de datos, y el `entrypoint` de la imagen espera a MySQL (por defecto en el host `mysql`) y se detiene tras 60 segundos si no responde. Por eso se recomienda preparar el nodo con el instalador, como se indica a continuación.

`daemon.auth.*` lo usa el SFTP del nodo para validar a los usuarios contra el maestro, y `token.public` para comprobar las peticiones que le envía el maestro. Si `token.public` está vacío con el panel desactivado, el log lo avisa al arrancar. Si `daemon.auth.url` es solo la dirección del maestro, sin ruta (por ejemplo `https://<maestro>`), el nodo le añade `/oauth2/token`; una URL con ruta se usa tal cual.

### 2. Instalar y convertir la máquina en nodo

1. Instale con el [instalador](#método-1-instalador-automático-recomendado) en la máquina del nodo, sin dominio (el maestro tiene que llegar al puerto 8080) y con `--no-admin`.
2. En `/opt/skypanel/docker-compose.yml`, descomente estas dos líneas del servicio `skypanel` y ponga la URL del maestro:

   ```yaml
         - SKYPANEL_PANEL_ENABLE=false
         - SKYPANEL_TOKEN_PUBLIC=https://<maestro>/auth/publickey
   ```

   El instalador (`--update`) y la actualización desde el panel detectan estas dos líneas y las vuelven a aplicar después de descargar el código. Cualquier otro cambio en `docker-compose.yml` se pierde al actualizar.
3. Con el contenedor parado, añada el resto al `config.json` del panel (`/opt/skypanel/storage/skypanel-data/config.json`, o `storage/skypanel-config/config.json` si existe), que vive en `storage/` y sobrevive a las actualizaciones:

   ```json
   {
     "daemon": {
       "auth": {
         "url": "https://<maestro>/oauth2/token",
         "clientId": ".node_<ID>",
         "clientSecret": "<secreto del nodo>"
       }
     }
   }
   ```

4. Arranque: `cd /opt/skypanel && docker compose up -d`.

Al volver a ejecutar el instalador en una máquina así, la reconoce como nodo: no crea cuentas de administrador ni instala el servicio `aether-domain`.

---

## Después de instalar

1. Entre en la dirección que muestra el resumen (`https://su-dominio`, o `http://IP:8080` sin dominio) con la cuenta de administrador. Si la contraseña se generó, está en `/etc/SkyPanel/credenciales.txt` (`sudo cat /etc/SkyPanel/credenciales.txt`). Cámbiela y borre el archivo.
2. Configure el correo en **Configuración → Correo**: sin él no funciona la recuperación de contraseñas.
3. Revise en Configuración la URL principal, el registro público (activado por defecto), Turnstile, los subdominios de Cloudflare, las copias externas y CurseForge.

Comandos útiles (desde `/opt/skypanel`):

```bash
docker compose ps                         # estado
docker compose logs -f skypanel           # logs en directo
docker compose restart                    # reiniciar
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name NOMBRE --email EMAIL --admin
docker exec -it skypanel /SkyPanel/bin/SkyPanel user edit   # nombre, email, contraseña, administrador, quitar 2FA
docker exec skypanel /SkyPanel/bin/SkyPanel version
```

Si algo falla, consulte [Solución de problemas](./troubleshooting.md).
