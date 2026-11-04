# Documentación Docker

Aether Panel se despliega solo con Docker. Este documento describe la imagen del panel, los archivos de Docker Compose y las imágenes base de los servidores de juego. Para instalar, use la [Guía de Instalación](../setup/01-installation.md); los comandos de uso diario están en la [Referencia rápida](./DOCKER-QUICKREF.md).

## Archivos

| Archivo | Propósito |
|---|---|
| `Dockerfile` | Imagen del panel (multi-stage) |
| `docker-compose.yml` | Producción: MariaDB, panel y Adminer. Es el que usan el instalador y las actualizaciones |
| `docker-compose.dev.yml` | Desarrollo: solo el panel (ver [limitaciones](#desarrollo-docker-composedevyml)) |
| `config.docker.json` | `config.json` que la imagen copia en `/etc/SkyPanel/config.json` |
| `docker/runtime-images/{ubuntu,steamcmd,generic,php}/Dockerfile` | Imágenes base de los servidores |
| `Dockerfile-curseforge` | Herramienta de pruebas de modpacks de CurseForge |
| `Dockerfile-templatetester` | Herramienta de pruebas de plantillas |
| `Dockerfile-formatter` | Herramienta que reordena los JSON de las plantillas |

## Imagen del panel (`Dockerfile`)

| Etapa | Base | Qué hace |
|---|---|---|
| `node` | `node:26-alpine` | Instala dependencias con Yarn 1.22.22 (`--frozen-lockfile`) y compila el frontend Astro (`yarn build`) |
| `xx` | `tonistiigi/xx:1.10.0` | Utilidades de compilación cruzada |
| `builder` | `golang:1.27-alpine` | Genera la documentación Swagger (`swag init`) y compila el binario con CGO para la plataforma de destino, con la versión, el commit y la clave de CurseForge en `-ldflags` |
| final | `alpine:3.24` | Binario en `/SkyPanel/bin/SkyPanel`, frontend en `/var/www/SkyPanel`, `netcat-openbsd` y `git` |

Las etapas de compilación se ejecutan en la plataforma de la máquina (`BUILDPLATFORM`) y el binario se compila para `TARGETPLATFORM`, por lo que la misma receta produce imágenes `amd64` y `arm64`. Necesita BuildKit (el constructor por defecto en Docker actual).

### Argumentos de compilación

| Argumento | Por defecto | Uso |
|---|---|---|
| `version` | `devel` | Versión que muestra `SkyPanel version` (`docker-compose.yml` usa `2.2.0`) |
| `sha` | `devel` | Commit (`docker-compose.yml` usa `local-build`) |
| `tags` | — | Etiquetas de compilación de Go |
| `curseforgeKey` | vacío | Clave de CurseForge incluida en el binario (valor por defecto de `daemon.curseforge.key`) |

### Contenido de la imagen final

- **Puertos expuestos:** `8080` (web, API y daemon) y `5657` (SFTP).
- **Usuario:** `SkyPanel` (UID/GID 1000). `docker-compose.yml` lo cambia por root (`user: "0:0"`) para que el panel use el socket de Docker y ejecute los juegos con un usuario sin privilegios (`docker.serverUser`).
- **Variables de entorno:** `GIN_MODE=release`, `SKYPANEL_PLATFORM=docker`, `SKYPANEL_DOCKER_ROOT=""`, `SKYPANEL_DOCKER_DISALLOWHOST=true`.
- **Configuración:** `config.docker.json` en `/etc/SkyPanel/config.json` (MySQL en `mysql:3306`, frontend en `/var/www/SkyPanel`, datos en `/var/lib/SkyPanel`, `docker.disallowHost` activo).
- **Volúmenes:** `/etc/SkyPanel`, `/var/lib/SkyPanel` y `/var/log/SkyPanel`. El frontend no es un volumen: se copia en cada compilación, y además va embebido en el binario (`go:embed`); `panel.web.files` apunta a la copia en disco, que tiene prioridad.
- **Comprobación de estado (`HEALTHCHECK`):** `nc -z localhost 8080`.

### Entrypoint

`/SkyPanel/bin/entrypoint.sh`:

1. Muestra la plataforma, la dirección de escucha y la URL de la base de datos con la contraseña oculta, y avisa si la contraseña es `skypanel_secret`.
2. Espera hasta 60 segundos a que responda el host y puerto de `SKYPANEL_PANEL_DATABASE_URL` (`mysql:3306` si no se indica). Si no responde, termina con error. Esta espera ocurre siempre, también con SQLite o con el panel desactivado, así que la imagen necesita un MySQL/MariaDB alcanzable.
3. Ejecuta `SkyPanel db migrate`, que aplica las migraciones pendientes (las mismas que `run`) y termina con un código distinto de cero si fallan. Si fallan, el `entrypoint` muestra el error y continúa; con el panel desactivado no hace nada.
4. Inicia `SkyPanel run`, que aplica las migraciones del esquema y arranca la web, el daemon y el SFTP.

### Dónde queda `config.json`

El panel lee `/etc/SkyPanel/config.json` si existe y, si no, `config.json` en su directorio de trabajo (`/var/lib/SkyPanel`). `docker-compose.yml` monta `./storage/skypanel-config` sobre `/etc/SkyPanel`; si esa carpeta está vacía, oculta el archivo de la imagen y el panel usa `./storage/skypanel-data/config.json`. Los cambios de **Configuración** se guardan en el archivo que se esté usando.

## Producción (`docker-compose.yml`)

| Servicio | Contenedor | Imagen | Puertos | Datos |
|---|---|---|---|---|
| `mysql` | `skypanel-mysql` | `mariadb:10.11` | `${DB_BIND_ADDRESS:-127.0.0.1}:3307 → 3306` | `./storage/mysql-data` |
| `skypanel` | `skypanel` | Se construye desde el `Dockerfile` | `8080:8080`, `5657:5657` | `./storage/skypanel-config`, `./storage/skypanel-data`, `./storage/skypanel-logs` |
| `adminer` | `skypanel-adminer` | `adminer:4` | `127.0.0.1:8081 → 8080` | — |

Todos los datos son carpetas del host bajo `./storage/` (no volúmenes con nombre). Credenciales de MariaDB, desde `.env`: `DB_ROOT_PASSWORD`, `DB_DATABASE`, `DB_USER`, `DB_PASSWORD` (por defecto `skypanel_secret`/`skypanel`, valores públicos que hay que cambiar; ver [Método 2](../setup/01-installation.md#método-2-docker-compose-a-mano)).

**Servicio `skypanel`:**

- Espera a que MariaDB esté sana (`depends_on: condition: service_healthy`) y tiene `stop_grace_period: 90s`: al pararse, el panel pide a todos los servidores de juego que se detengan (hasta 60 s) para que guarden.
- Montajes adicionales: el socket de Docker, `./.git` en `/repo/.git` (solo lectura, para comprobar el commit desplegado) y `/proc` del host en `/host/proc` (solo lectura, para medir el tráfico de red del nodo).
- Comprobación de estado (`healthcheck`): `wget --spider http://localhost:8080/`.
- Redes: `skypanel-network` (panel, MariaDB y Adminer) y `skypanel-servers` (nombre fijo; la de los servidores de juego, que comparten con MariaDB y el panel).

**Variables de entorno de `skypanel`:**

| Variable | Valor | Efecto |
|---|---|---|
| `GIN_MODE` | `release` | Modo de Gin |
| `SKYPANEL_PLATFORM` | `docker` | Si el daemon no puede iniciarse, el panel web sigue funcionando |
| `SKYPANEL_DOCKER_ROOT` | vacío | `docker.root` |
| `SKYPANEL_DOCKER_DISALLOWHOST` | `true` | Solo servidores en contenedores |
| `SKYPANEL_DAEMON_HOSTPROC` | `/host/proc` | `/proc` del host para el tráfico de red del nodo |
| `SKYPANEL_DOCKER_SERVERSNETWORK` | `skypanel-servers` | Red de los servidores de juego |
| `SKYPANEL_WEB_HOST` | `0.0.0.0:8080` | Dirección de escucha |
| `SKYPANEL_PANEL_DATABASE_DIALECT` | `mysql` | Dialecto |
| `SKYPANEL_PANEL_DATABASE_URL` | `${DB_USER}:${DB_PASSWORD}@tcp(mysql:3306)/${DB_DATABASE}?charset=utf8&parseTime=true` | Conexión |
| `SKYPANEL_TEMPLATES_URL` | `https://templates.aetherpanel.es/templates.json` | Repositorio de plantillas |

El archivo trae comentadas las líneas de nodo secundario (`SKYPANEL_PANEL_ENABLE=false`, `SKYPANEL_TOKEN_PUBLIC=…`) y de Turnstile. El registro público y el tema no se fijan aquí a propósito: una variable de entorno gana siempre a lo guardado en Configuración.

**Cambios locales.** `docker-compose.yml` está en git y las actualizaciones lo restauran (salvo las dos líneas de nodo secundario). Los cambios propios van en `docker-compose.override.yml`, que Compose combina automáticamente; para sustituir una lista de puertos use `ports: !override` (Docker Compose 2.24.4 o posterior). El instalador genera ese archivo cuando cambian los puertos, con `--bind` o con base de datos externa.

## Desarrollo (`docker-compose.dev.yml`)

Un solo servicio, `skypanel` (contenedor `skypanel-dev`), compilado con `version=dev-local`, `sha=dev`:

- Puertos `8080` y `5657`; datos en `./dev-data/data` y logs en `./dev-data/logs`; socket de Docker montado.
- `privileged: true`, `user: "0:0"`, `restart: "no"`, red `skypanel-dev`.
- Variables: `GIN_MODE=debug`, `SKYPANEL_PLATFORM=docker`, `SKYPANEL_WEB_HOST=0.0.0.0:8080`, `SKYPANEL_PANEL_SETTINGS_COMPANYNAME=Aether Panel` y `SKYPANEL_LOGS_LEVEL=DEBUG` (esta última no corresponde a ninguna clave de configuración y no tiene efecto).

> **Limitación:** este archivo no define base de datos ni `SKYPANEL_PANEL_DATABASE_URL`, y no monta `/etc/SkyPanel`, así que se usa el `config.json` de la imagen (MySQL en `mysql:3306`). Como el `entrypoint` espera a MySQL, el contenedor se detiene a los 60 segundos tal como está. Para usarlo, añada un servicio MariaDB llamado `mysql` en la misma red (o apunte `SKYPANEL_PANEL_DATABASE_URL` a uno existente).

## Imágenes base de los servidores de juego

`docker/runtime-images/` contiene las imágenes que usan las plantillas. CI las publica en `ghcr.io/aether-panel/<nombre>` (`amd64` y `arm64`) desde `main` y `dev2.0`, con etiqueta de rama, `sha-<commit>` y `latest` en `main`.

| Imagen | Base | Contenido |
|---|---|---|
| `ghcr.io/aether-panel/ubuntu` | `ubuntu:24.04` | `ca-certificates`, `curl`, `git`, `unzip`, `wget`, `xz-utils`, `zip`; `DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=1` |
| `ghcr.io/aether-panel/steamcmd` | `steamcmd/steamcmd:ubuntu-22` | Bibliotecas i386 para SteamCMD, `xvfb`, `curl`, `wget`, `unzip`, `zip`; `HOME=/home/container` |
| `ghcr.io/aether-panel/generic` | `ubuntu:24.04` | `bash`, `ca-certificates`, `curl`, `git`, `unzip`, `wget`, `xz-utils`, `zip`; directorio de trabajo `/home/container` |
| `ghcr.io/aether-panel/php:<versión>` | `php:<versión>-cli` (8.2, 8.3, 8.4) | Composer 2, `git`, `unzip`, `zip`, `curl` y las extensiones `bcmath`, `exif`, `gd`, `intl`, `mysqli`, `opcache`, `pcntl`, `pdo_mysql`, `pdo_pgsql`, `pgsql`, `redis`, `sockets`, `zip`; `php.ini` de producción. Una imagen por versión, etiquetada con ella. La usan las plantillas PHP y Laravel |

Todas crean el usuario `container` y se ejecutan con él. Cada nodo Docker descarga y mantiene al día en segundo plano las tres primeras (`docker.prefetchImages`, con esas tres por defecto); la de PHP se descarga la primera vez que la usa un servidor, para que el primer servidor que las usa no tenga que esperar.

La imagen del panel se publica en `ghcr.io/aether-panel/panel` (`amd64` y `arm64`) solo desde `main`, con las etiquetas `main`, `sha-<commit>` y `latest`. El instalador no la usa: compila la imagen en el servidor.

## Herramientas auxiliares

| Dockerfile | Base final | Qué hace |
|---|---|---|
| `Dockerfile-curseforge` | `eclipse-temurin:25-alpine` | Compila `tools/curseforge` (`curseforgetester`), que comprueba modpacks de CurseForge |
| `Dockerfile-templatetester` | `ubuntu:26.04` | Compila `tools/templatetester` y añade Mono, Node.js 20, bibliotecas i386 y utilidades para probar plantillas |
| `Dockerfile-formatter` | `alpine` | Compila `tools/formatter`, que reescribe los JSON de un repositorio de plantillas (`-workDir`) con el orden de campos estándar |

## Comandos

Los comandos habituales están en la [Referencia rápida](./DOCKER-QUICKREF.md). Compilación para otra arquitectura:

```bash
docker build --platform linux/amd64 -t skypanel:latest .
docker build --platform linux/arm64 -t skypanel:latest .
```

## Solución de problemas

### «Bind for 0.0.0.0:8080 failed: port is already allocated»

Otro proceso usa el puerto. Cambie el mapeo en `docker-compose.override.yml` (Docker Compose 2.24.4 o posterior):

```yaml
services:
  skypanel:
    ports: !override
      - "9000:8080"
      - "5657:5657"
```

Con el instalador, use `--port` / `--sftp-port`.

### «permission denied» al usar el socket de Docker

El socket del host pertenece a `root:docker`. En `docker-compose.yml` el panel se ejecuta como root (`user: "0:0"`), que evita el problema; `docker-compose.dev.yml` además usa `privileged: true`. Ver [Socket de Docker y permisos](./DOCKER-SUDO-FIX.md).

### El panel no conecta con MySQL

El `entrypoint` espera hasta 60 segundos. Revise:

```bash
docker logs skypanel
docker logs skypanel-mysql
```

Si cambió las contraseñas del `.env` después de crear la base de datos, MariaDB sigue con las antiguas: ver [Solución de problemas](../setup/troubleshooting.md#access-denied-tras-cambiar-el-env).

### Los datos no persisten

Los datos viven en `./storage/` junto al `docker-compose.yml`. Compruebe que los contenedores se crearon desde esa carpeta:

```bash
docker inspect skypanel --format '{{range .Mounts}}{{.Source}} -> {{.Destination}}{{"\n"}}{{end}}'
```
