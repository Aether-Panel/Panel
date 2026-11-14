# Docker Quick Reference

Everyday commands. Run them from the folder holding `docker-compose.yml` (`/opt/skypanel` on an install made with the installer). Docker Compose v2 (`docker compose`) is used; `docker-compose` v1 is not supported. Details are in [Docker Documentation](./README.docker.md).

## Docker Compose

```bash
# Build and start (panel, MariaDB and Adminer)
docker compose up -d --build

# Status and logs
docker compose ps
docker compose logs -f skypanel
docker compose logs -f              # every service

# Restart, stop and start
docker compose restart
docker compose stop
docker compose start

# Remove the containers (data in ./storage is kept)
docker compose down

# Rebuild the panel image
docker compose build
```

To update an install made with the installer, use `install.sh --update` or **Settings → Updates**; see [Updating](../setup/01-installation.md#updating). `docker compose pull` only downloads MariaDB and Adminer: the panel image is built on the server.

## Panel commands

```bash
# Create an administrator (prompts for the password if --password is not given)
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin

# Edit a user: name, email, password, administrator, remove 2FA
docker exec -it skypanel /SkyPanel/bin/SkyPanel user edit

# Version
docker exec skypanel /SkyPanel/bin/SkyPanel version

# Schema migrations (the panel already applies them on start)
docker exec skypanel /SkyPanel/bin/SkyPanel db upgrade

# Shell in the container
docker exec -it skypanel sh
```

The installer can also create an administrator: `sudo bash install.sh --create-admin --admin-email admin@example.com`.

## Images

```bash
# Panel image
docker build -t skypanel:latest .
docker build --platform linux/arm64 -t skypanel:latest .

# Helper tools
docker build -f Dockerfile-curseforge -t curseforge-tester .
docker build -f Dockerfile-templatetester -t template-tester .
docker build -f Dockerfile-formatter -t formatter .
```

## Ports

| Port | Service | `docker-compose.yml` | `docker-compose.dev.yml` |
|---|---|---|---|
| `8080` | Web panel and API | All interfaces | All interfaces |
| `5657` | SFTP | All interfaces | All interfaces |
| `3307` | MariaDB (3306 inside) | `127.0.0.1` only (`DB_BIND_ADDRESS`) | — |
| `8081` | Adminer | `127.0.0.1` only | — |

## Data

| Host path | Container path | Contents | File |
|---|---|---|---|
| `./storage/skypanel-config/` | `/etc/SkyPanel` | `config.json` if placed here | `docker-compose.yml` |
| `./storage/skypanel-data/` | `/var/lib/SkyPanel` | Servers, backups, extensions, `config.json` | `docker-compose.yml` |
| `./storage/skypanel-logs/` | `/var/log/SkyPanel` | Panel logs | `docker-compose.yml` |
| `./storage/mysql-data/` | `/var/lib/mysql` | Database | `docker-compose.yml` |
| `./dev-data/data/` | `/var/lib/SkyPanel` | Data | `docker-compose.dev.yml` |
| `./dev-data/logs/` | `/var/log/SkyPanel` | Logs | `docker-compose.dev.yml` |

## Database

```bash
# MariaDB console (root password in .env)
docker exec -it skypanel-mysql mariadb -uroot -p

# Database backup
docker exec -e MYSQL_PWD="$(grep ^DB_ROOT_PASSWORD= .env | cut -d= -f2-)" skypanel-mysql \
  mariadb-dump -uroot --single-transaction --databases skypanel | gzip > skypanel.sql.gz

# Adminer from your computer (SSH tunnel), then http://localhost:8081
ssh -L 8081:127.0.0.1:8081 user@server
```

## Environment variables

Any configuration key can be set with `SKYPANEL_` + the key in upper case, with dots replaced by `_`. Put them in `docker-compose.override.yml`, not in `docker-compose.yml` (updates restore it). A variable always wins over what Settings saved, so only use variables for things you will not change from the panel.

```bash
SKYPANEL_TEMPLATES_URL=https://templates.aetherpanel.es/templates.json
SKYPANEL_PANEL_SETTINGS_COMPANYNAME="My Panel"
SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY=your-key
SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOK=https://discord.com/api/webhooks/...
SKYPANEL_DAEMON_MINFREEDISKMB=4096
SKYPANEL_PANEL_BACKUPS_MAXPERSERVER=20
```

The list of keys is in [Configuration](../backend/02-configuration.md).
