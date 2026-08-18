# Docker Documentation

## Overview

The project includes 4 Dockerfiles and 2 Docker Compose configurations for different purposes.

| File | Purpose |
|---|---|
| `Dockerfile` | Main production image (multi-stage) |
| `Dockerfile-curseforge` | CurseForge integration tester |
| `Dockerfile-templatetester` | Template tester (Ubuntu + Mono + Node) |
| `Dockerfile-formatter` | Source code formatter |
| `docker-compose.yml` | Production: Panel + MariaDB |
| `docker-compose.dev.yml` | Development: Standalone panel with SQLite |

## Main Image (`Dockerfile`)

Multi-stage build:

```
Stage 1 (node):26-alpine
  → Build frontend (Yarn workspaces, Astro)

Stage 2 (golang:1.26-alpine + tonistiigi/xx)
  → Compiles Go binary with CGO, swag, ldflags
  → Frontend build copied from stage 1

Stage 3 (alpine:3.24)
  → Minimal final image (3.24 MB base + binary + frontend)
```

### Exposed Ports

| Port | Service |
|---|---|
| `8080` | Web Panel (API + frontend) |
| `5657` | SFTP |

### Entrypoint

`/SkyPanel/bin/entrypoint.sh`:
1. Waits for MySQL to be available (up to 60s, via `nc`)
2. Executes `SkyPanel db migrate`
3. Starts `SkyPanel run`

### Environment Variables

| Variable | Description | Default in Docker |
|---|---|---|
| `GIN_MODE` | Gin Mode | `release` |
| `SKYPANEL_PLATFORM` | Platform | `docker` |
| `SKYPANEL_DOCKER_ROOT` | Docker Root | `""` |
| `SKYPANEL_DOCKER_DISALLOWHOST` | Force Docker | `true` |
| `SKYPANEL_WEB_HOST` | Bind address | `0.0.0.0:8080` |
| `SKYPANEL_PANEL_DATABASE_DIALECT` | DB dialect | `mysql` |
| `SKYPANEL_PANEL_DATABASE_URL` | Connection string | — |
| `SKYPANEL_PANEL_SETTINGS_COMPANYNAME` | Brand | `Aether Panel` |
| `SKYPANEL_PANEL_REGISTRATIONENABLED` | Open registration | `true` |
| `SKYPANEL_PANEL_SETTINGS_DEFAULTTHEME` | Default theme | `SkyPanel` |

### User

Runs as non-root user `SkyPanel` (UID 1000). Data directories are created with `mkdir -p` in the build and assigned to `SkyPanel:SkyPanel`.

### Volumes

| Path in container | Purpose |
|---|---|
| `/etc/SkyPanel` | Configuration (config.json) |
| `/var/lib/SkyPanel` | Runtime data (servers, backups, cache, logs) |
| `/var/log/SkyPanel` | Logs |

> **Note:** The frontend (`/var/www/SkyPanel`) is **no longer a volume**. It is copied at build time (`COPY`) from `client/frontend/dist` and replaced on every image rebuild, avoiding an anonymous volume with stale files. The `client/frontend/dist` folder is also embedded in the Go binary (`go:embed all:dist`), and `panel.web.files` can override the embedded FS.

## Production (`docker-compose.yml`)

```yaml
services:
  mysql:
    image: mariadb:10.11
    environment:
      - MYSQL_ROOT_PASSWORD=${DB_ROOT_PASSWORD:-skypanel_secret}
      - MYSQL_DATABASE=${DB_DATABASE:-skypanel}
      - MYSQL_USER=${DB_USER:-skypanel}
      - MYSQL_PASSWORD=${DB_PASSWORD:-skypanel_secret}
    volumes:
      - ./storage/mysql-data:/var/lib/mysql
    healthcheck:
      test: ["CMD-SHELL", "mysqladmin ping -h localhost -u root -p$${MYSQL_ROOT_PASSWORD}"]

  skypanel:
    build:
      context: .
      dockerfile: Dockerfile
      args:
        - version=dev-docker
        - sha=local-build
    ports:
      - "8080:8080"
      - "5657:5657"
    volumes:
      - ./storage/skypanel-config:/etc/SkyPanel
      - ./storage/skypanel-data:/var/lib/SkyPanel
      - ./storage/skypanel-logs:/var/log/SkyPanel
      - //var/run/docker.sock:/var/run/docker.sock
    environment:
      - SKYPANEL_PANEL_DATABASE_DIALECT=mysql
      - SKYPANEL_PANEL_DATABASE_URL=${DB_USER:-skypanel}:${DB_PASSWORD:-skypanel_secret}@tcp(mysql:3306)/${DB_DATABASE:-skypanel}?charset=utf8&parseTime=true
    depends_on:
      mysql:
        condition: service_healthy
```

## Development (`docker-compose.dev.yml`)

```yaml
services:
  skypanel:
    container_name: skypanel-dev
    build:
      context: .
      dockerfile: Dockerfile
      args:
        - version=dev-local
        - sha=dev
    ports:
      - "8080:8080"
      - "5657:5657"
    volumes:
      - ./dev-data/data:/var/lib/SkyPanel:z
      - ./dev-data/logs:/var/log/SkyPanel:z
      - /var/run/docker.sock:/var/run/docker.sock:z
    privileged: true
    user: "0:0"
    environment:
      - GIN_MODE=debug
      - SKYPANEL_LOGS_LEVEL=DEBUG
      # SQLite by default (no DB service required)
```

## Useful Commands

```bash
# Start production
docker-compose up -d

# Start development
docker-compose -f docker-compose.dev.yml up -d

# View logs
docker-compose logs -f

# Stop
docker-compose down

# Create admin user
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin

# Interactive shell
docker exec -it skypanel sh

# View version
docker exec skypanel /SkyPanel/bin/SkyPanel version

# Execute CLI command
docker exec skypanel /SkyPanel/bin/SkyPanel version

# Rebuild image
docker-compose build

# Update image
docker-compose pull
```

## Multi-Platform Build

The Dockerfile supports multi-architecture builds via `tonistiigi/xx`:

```bash
# Build for linux/amd64
docker build --platform linux/amd64 -t skypanel:latest .

# Build for linux/arm64
docker build --platform linux/arm64 -t skypanel:latest .
```

## Custom Configuration

The default configuration is in `config.docker.json` with MySQL. To use SQLite in production, change the environment variables:

```yaml
environment:
  - SKYPANEL_PANEL_DATABASE_DIALECT=sqlite3
  # SKYPANEL_PANEL_DATABASE_URL not required (default: file:skypanel.db)
  # Remove depends_on: mysql
```

## Auxiliary Dockerfiles

### `Dockerfile-curseforge`
Image for the CurseForge tester. Built with CGO, Eclipse Temurin 25 (Java) runtime. Used to verify CurseForge modpacks.

### `Dockerfile-templatetester`
Heavy image for testing templates. Includes Mono (for SteamCMD), Node.js 20, GCC, zip/unzip, and i386 dependencies. Builds the `templatetester` tool.

### `Dockerfile-formatter`
Minimal image for formatting Go code. Builds the `formatter` tool and runs it on the source code.

## Troubleshooting

### Error: "Bind for 0.0.0.0:8080 failed: port is already allocated"
Change the port mapping:
```yaml
ports:
  - "9000:8080"
```

### Error: "permission denied" when connecting the Docker socket
Ensure the container user has permissions. In production:
```yaml
user: "0:0"  # Temporary
```
In development, it's already configured with `privileged: true` and `user: "0:0"`.

### Error: MySQL connection refused
The entrypoint waits up to 60s. Verify:
```bash
docker logs skypanel
docker logs skypanel-mysql
```

### Changes do not persist
Verify the volumes:
```bash
docker volume ls
docker inspect skypanel | grep Mounts
```