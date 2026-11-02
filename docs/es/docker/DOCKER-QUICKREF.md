# Referencia rápida de Docker

Comandos de uso diario. Ejecútelos desde la carpeta del `docker-compose.yml` (`/opt/skypanel` en una instalación con el instalador). Se usa Docker Compose v2 (`docker compose`); `docker-compose` v1 no es compatible. Los detalles están en [Documentación Docker](./README.docker.md).

## Docker Compose

```bash
# Construir e iniciar (panel, MariaDB y Adminer)
docker compose up -d --build

# Estado y logs
docker compose ps
docker compose logs -f skypanel
docker compose logs -f              # todos los servicios

# Reiniciar, detener y arrancar
docker compose restart
docker compose stop
docker compose start

# Eliminar los contenedores (los datos de ./storage se conservan)
docker compose down

# Volver a construir la imagen del panel
docker compose build
```

Para actualizar una instalación hecha con el instalador, use `install.sh --update` o **Configuración → Actualizaciones**; ver [Actualizar](../setup/01-installation.md#actualizar). `docker compose pull` solo descarga MariaDB y Adminer: la imagen del panel se construye en el servidor.

## Comandos del panel

```bash
# Crear un administrador (pide la contraseña si no se indica --password)
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@ejemplo.com --admin

# Editar un usuario: nombre, email, contraseña, administrador, quitar 2FA
docker exec -it skypanel /SkyPanel/bin/SkyPanel user edit

# Versión
docker exec skypanel /SkyPanel/bin/SkyPanel version

# Migraciones del esquema (el panel ya las aplica al arrancar)
docker exec skypanel /SkyPanel/bin/SkyPanel db upgrade

# Shell en el contenedor
docker exec -it skypanel sh
```

Con el instalador también se puede crear un administrador: `sudo bash install.sh --create-admin --admin-email admin@ejemplo.com`.

## Imágenes

```bash
# Imagen del panel
docker build -t skypanel:latest .
docker build --platform linux/arm64 -t skypanel:latest .

# Herramientas auxiliares
docker build -f Dockerfile-curseforge -t curseforge-tester .
docker build -f Dockerfile-templatetester -t template-tester .
docker build -f Dockerfile-formatter -t formatter .
```

## Puertos

| Puerto | Servicio | `docker-compose.yml` | `docker-compose.dev.yml` |
|---|---|---|---|
| `8080` | Panel web y API | Todas las interfaces | Todas las interfaces |
| `5657` | SFTP | Todas las interfaces | Todas las interfaces |
| `3307` | MariaDB (3306 dentro) | Solo `127.0.0.1` (`DB_BIND_ADDRESS`) | — |
| `8081` | Adminer | Solo `127.0.0.1` | — |

## Datos

| Ruta en el host | Ruta en el contenedor | Contenido | Archivo |
|---|---|---|---|
| `./storage/skypanel-config/` | `/etc/SkyPanel` | `config.json` si se deja aquí | `docker-compose.yml` |
| `./storage/skypanel-data/` | `/var/lib/SkyPanel` | Servidores, copias, extensiones, `config.json` | `docker-compose.yml` |
| `./storage/skypanel-logs/` | `/var/log/SkyPanel` | Logs del panel | `docker-compose.yml` |
| `./storage/mysql-data/` | `/var/lib/mysql` | Base de datos | `docker-compose.yml` |
| `./dev-data/data/` | `/var/lib/SkyPanel` | Datos | `docker-compose.dev.yml` |
| `./dev-data/logs/` | `/var/log/SkyPanel` | Logs | `docker-compose.dev.yml` |

## Base de datos

```bash
# Consola de MariaDB (contraseña de root en .env)
docker exec -it skypanel-mysql mariadb -uroot -p

# Copia de la base de datos
docker exec -e MYSQL_PWD="$(grep ^DB_ROOT_PASSWORD= .env | cut -d= -f2-)" skypanel-mysql \
  mariadb-dump -uroot --single-transaction --databases skypanel | gzip > skypanel.sql.gz

# Adminer desde su equipo (túnel SSH) y luego http://localhost:8081
ssh -L 8081:127.0.0.1:8081 usuario@servidor
```

## Variables de entorno

Cualquier clave de configuración se puede fijar con `SKYPANEL_` + la clave en mayúsculas, con los puntos cambiados por `_`. Póngalas en `docker-compose.override.yml`, no en `docker-compose.yml` (las actualizaciones lo restauran). Una variable gana siempre a lo guardado en Configuración, así que use variables solo para lo que no vaya a cambiar desde el panel.

```bash
SKYPANEL_TEMPLATES_URL=https://templates.aetherpanel.es/templates.json
SKYPANEL_PANEL_SETTINGS_COMPANYNAME="Mi Panel"
SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY=su-clave
SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOK=https://discord.com/api/webhooks/...
SKYPANEL_DAEMON_MINFREEDISKMB=4096
SKYPANEL_PANEL_BACKUPS_MAXPERSERVER=20
```

La lista de claves está en [Configuración](../backend/02-configuration.md).
