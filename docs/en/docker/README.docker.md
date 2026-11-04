# Docker Documentation

Aether Panel is deployed with Docker only. This document describes the panel image, the Docker Compose files and the base images for game servers. To install, use the [Installation Guide](../setup/01-installation.md); everyday commands are in the [Quick reference](./DOCKER-QUICKREF.md).

## Files

| File | Purpose |
|---|---|
| `Dockerfile` | Panel image (multi-stage) |
| `docker-compose.yml` | Production: MariaDB, panel and Adminer. Used by the installer and by updates |
| `docker-compose.dev.yml` | Development: the panel only (see [limitations](#development-docker-composedevyml)) |
| `config.docker.json` | `config.json` the image copies to `/etc/SkyPanel/config.json` |
| `docker/runtime-images/{ubuntu,steamcmd,generic,php}/Dockerfile` | Base images for servers |
| `Dockerfile-curseforge` | CurseForge modpack test tool |
| `Dockerfile-templatetester` | Template test tool |
| `Dockerfile-formatter` | Tool that reorders template JSON files |

## Panel image (`Dockerfile`)

| Stage | Base | What it does |
|---|---|---|
| `node` | `node:26-alpine` | Installs dependencies with Yarn 1.22.22 (`--frozen-lockfile`) and builds the Astro frontend (`yarn build`) |
| `xx` | `tonistiigi/xx:1.10.0` | Cross-compilation helpers |
| `builder` | `golang:1.27-alpine` | Generates the Swagger docs (`swag init`) and builds the binary with CGO for the target platform, with the version, commit and CurseForge key in `-ldflags` |
| final | `alpine:3.24` | Binary in `/SkyPanel/bin/SkyPanel`, frontend in `/var/www/SkyPanel`, `netcat-openbsd` and `git` |

The build stages run on the machine's platform (`BUILDPLATFORM`) and the binary is built for `TARGETPLATFORM`, so the same recipe produces `amd64` and `arm64` images. It needs BuildKit (the default builder in current Docker).

### Build arguments

| Argument | Default | Usage |
|---|---|---|
| `version` | `devel` | Version shown by `SkyPanel version` (`docker-compose.yml` uses `2.2.0`) |
| `sha` | `devel` | Commit (`docker-compose.yml` uses `local-build`) |
| `tags` | — | Go build tags |
| `curseforgeKey` | empty | CurseForge key built into the binary (default value of `daemon.curseforge.key`) |

### Final image contents

- **Exposed ports:** `8080` (web, API and daemon) and `5657` (SFTP).
- **User:** `SkyPanel` (UID/GID 1000). `docker-compose.yml` switches it to root (`user: "0:0"`) so the panel can use the Docker socket and run games as an unprivileged user (`docker.serverUser`).
- **Environment variables:** `GIN_MODE=release`, `SKYPANEL_PLATFORM=docker`, `SKYPANEL_DOCKER_ROOT=""`, `SKYPANEL_DOCKER_DISALLOWHOST=true`.
- **Configuration:** `config.docker.json` in `/etc/SkyPanel/config.json` (MySQL at `mysql:3306`, frontend in `/var/www/SkyPanel`, data in `/var/lib/SkyPanel`, `docker.disallowHost` on).
- **Volumes:** `/etc/SkyPanel`, `/var/lib/SkyPanel` and `/var/log/SkyPanel`. The frontend is not a volume: it is copied on every build, and it is also embedded in the binary (`go:embed`); `panel.web.files` points at the on-disk copy, which takes precedence.
- **Healthcheck:** `nc -z localhost 8080`.

### Entrypoint

`/SkyPanel/bin/entrypoint.sh`:

1. Prints the platform, the listen address and the database URL with the password hidden, and warns if the password is `skypanel_secret`.
2. Waits up to 60 seconds for the host and port in `SKYPANEL_PANEL_DATABASE_URL` (`mysql:3306` if not set). If it does not answer, it exits with an error. This wait always happens, also with SQLite or with the panel off, so the image needs a reachable MySQL/MariaDB.
3. Runs `SkyPanel db migrate`, which applies the pending migrations (the same ones `run` applies) and exits non-zero when they fail. On failure the `entrypoint` prints the error and carries on; with the panel turned off it does nothing.
4. Starts `SkyPanel run`, which applies schema migrations and starts the web panel, the daemon and SFTP.

### Where `config.json` lives

The panel reads `/etc/SkyPanel/config.json` if it exists and otherwise `config.json` in its working directory (`/var/lib/SkyPanel`). `docker-compose.yml` mounts `./storage/skypanel-config` over `/etc/SkyPanel`; when that folder is empty it hides the image's file and the panel uses `./storage/skypanel-data/config.json`. Changes made in **Settings** are saved to whichever file is in use.

## Production (`docker-compose.yml`)

| Service | Container | Image | Ports | Data |
|---|---|---|---|---|
| `mysql` | `skypanel-mysql` | `mariadb:10.11` | `${DB_BIND_ADDRESS:-127.0.0.1}:3307 → 3306` | `./storage/mysql-data` |
| `skypanel` | `skypanel` | Built from the `Dockerfile` | `8080:8080`, `5657:5657` | `./storage/skypanel-config`, `./storage/skypanel-data`, `./storage/skypanel-logs` |
| `adminer` | `skypanel-adminer` | `adminer:4` | `127.0.0.1:8081 → 8080` | — |

All data lives in host folders under `./storage/` (not named volumes). MariaDB credentials come from `.env`: `DB_ROOT_PASSWORD`, `DB_DATABASE`, `DB_USER`, `DB_PASSWORD` (defaults `skypanel_secret`/`skypanel`, public values that must be changed; see [Method 2](../setup/01-installation.md#method-2-docker-compose-by-hand)).

**`skypanel` service:**

- Waits for MariaDB to be healthy (`depends_on: condition: service_healthy`) and has `stop_grace_period: 90s`: when stopping, the panel asks every game server to stop (up to 60 s) so they save.
- Extra mounts: the Docker socket, `./.git` at `/repo/.git` (read-only, to check the deployed commit) and the host's `/proc` at `/host/proc` (read-only, to measure the node's network traffic).
- Healthcheck: `wget --spider http://localhost:8080/`.
- Networks: `skypanel-network` (panel, MariaDB and Adminer) and `skypanel-servers` (fixed name; the game servers' network, shared with MariaDB and the panel).

**`skypanel` environment variables:**

| Variable | Value | Effect |
|---|---|---|
| `GIN_MODE` | `release` | Gin mode |
| `SKYPANEL_PLATFORM` | `docker` | If the daemon cannot start, the web panel keeps running |
| `SKYPANEL_DOCKER_ROOT` | empty | `docker.root` |
| `SKYPANEL_DOCKER_DISALLOWHOST` | `true` | Containers only |
| `SKYPANEL_DAEMON_HOSTPROC` | `/host/proc` | Host `/proc` for the node's network traffic |
| `SKYPANEL_DOCKER_SERVERSNETWORK` | `skypanel-servers` | Game servers' network |
| `SKYPANEL_WEB_HOST` | `0.0.0.0:8080` | Listen address |
| `SKYPANEL_PANEL_DATABASE_DIALECT` | `mysql` | Dialect |
| `SKYPANEL_PANEL_DATABASE_URL` | `${DB_USER}:${DB_PASSWORD}@tcp(mysql:3306)/${DB_DATABASE}?charset=utf8&parseTime=true` | Connection |
| `SKYPANEL_TEMPLATES_URL` | `https://templates.aetherpanel.es/templates.json` | Template repository |

The file ships with commented-out lines for a secondary node (`SKYPANEL_PANEL_ENABLE=false`, `SKYPANEL_TOKEN_PUBLIC=…`) and for Turnstile. Public registration and the theme are deliberately not set here: an environment variable always wins over what Settings saved.

**Local changes.** `docker-compose.yml` is tracked in git and updates restore it (except the two secondary node lines). Your own changes go in `docker-compose.override.yml`, which Compose merges automatically; to replace a port list use `ports: !override` (Docker Compose 2.24.4 or later). The installer writes that file when ports change, with `--bind` or with an external database.

## Development (`docker-compose.dev.yml`)

A single service, `skypanel` (container `skypanel-dev`), built with `version=dev-local`, `sha=dev`:

- Ports `8080` and `5657`; data in `./dev-data/data` and logs in `./dev-data/logs`; Docker socket mounted.
- `privileged: true`, `user: "0:0"`, `restart: "no"`, network `skypanel-dev`.
- Variables: `GIN_MODE=debug`, `SKYPANEL_PLATFORM=docker`, `SKYPANEL_WEB_HOST=0.0.0.0:8080`, `SKYPANEL_PANEL_SETTINGS_COMPANYNAME=Aether Panel` and `SKYPANEL_LOGS_LEVEL=DEBUG` (the last one maps to no configuration key and has no effect).

> **Limitation:** this file defines no database and no `SKYPANEL_PANEL_DATABASE_URL`, and does not mount `/etc/SkyPanel`, so the image's `config.json` is used (MySQL at `mysql:3306`). Since the `entrypoint` waits for MySQL, the container exits after 60 seconds as it stands. To use it, add a MariaDB service named `mysql` on the same network (or point `SKYPANEL_PANEL_DATABASE_URL` at an existing one).

## Game server base images

`docker/runtime-images/` holds the images templates use. CI publishes them as `ghcr.io/aether-panel/<name>` (`amd64` and `arm64`) from `main` and `dev2.0`, tagged with the branch, `sha-<commit>` and `latest` on `main`.

| Image | Base | Contents |
|---|---|---|
| `ghcr.io/aether-panel/ubuntu` | `ubuntu:24.04` | `ca-certificates`, `curl`, `git`, `unzip`, `wget`, `xz-utils`, `zip`; `DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=1` |
| `ghcr.io/aether-panel/steamcmd` | `steamcmd/steamcmd:ubuntu-22` | i386 libraries for SteamCMD, `xvfb`, `curl`, `wget`, `unzip`, `zip`; `HOME=/home/container` |
| `ghcr.io/aether-panel/generic` | `ubuntu:24.04` | `bash`, `ca-certificates`, `curl`, `git`, `unzip`, `wget`, `xz-utils`, `zip`; working directory `/home/container` |
| `ghcr.io/aether-panel/php:<version>` | `php:<version>-cli` (8.2, 8.3, 8.4) | Composer 2, `git`, `unzip`, `zip`, `curl` and the extensions `bcmath`, `exif`, `gd`, `intl`, `mysqli`, `opcache`, `pcntl`, `pdo_mysql`, `pdo_pgsql`, `pgsql`, `redis`, `sockets`, `zip`; production `php.ini`. One image per version, tagged with it. Used by the PHP and Laravel templates |

All of them create the `container` user and run as it. Every Docker node downloads the first three and keeps them up to date in the background (`docker.prefetchImages`, with those three by default); the PHP one is pulled the first time a server uses it, so the first server using one does not wait.

The panel image is published as `ghcr.io/aether-panel/panel` (`amd64` and `arm64`) from `main` only, tagged `main`, `sha-<commit>` and `latest`. The installer does not use it: it builds the image on the server.

## Helper tools

| Dockerfile | Final base | What it does |
|---|---|---|
| `Dockerfile-curseforge` | `eclipse-temurin:25-alpine` | Builds `tools/curseforge` (`curseforgetester`), which checks CurseForge modpacks |
| `Dockerfile-templatetester` | `ubuntu:26.04` | Builds `tools/templatetester` and adds Mono, Node.js 20, i386 libraries and utilities to test templates |
| `Dockerfile-formatter` | `alpine` | Builds `tools/formatter`, which rewrites the JSON files of a template repository (`-workDir`) in the standard field order |

## Commands

Everyday commands are in the [Quick reference](./DOCKER-QUICKREF.md). Building for another architecture:

```bash
docker build --platform linux/amd64 -t skypanel:latest .
docker build --platform linux/arm64 -t skypanel:latest .
```

## Troubleshooting

### "Bind for 0.0.0.0:8080 failed: port is already allocated"

Another process uses the port. Change the mapping in `docker-compose.override.yml` (Docker Compose 2.24.4 or later):

```yaml
services:
  skypanel:
    ports: !override
      - "9000:8080"
      - "5657:5657"
```

With the installer, use `--port` / `--sftp-port`.

### "permission denied" on the Docker socket

The host's socket belongs to `root:docker`. In `docker-compose.yml` the panel runs as root (`user: "0:0"`), which avoids the problem; `docker-compose.dev.yml` also uses `privileged: true`. See [Docker socket and permissions](./DOCKER-SUDO-FIX.md).

### The panel cannot reach MySQL

The `entrypoint` waits up to 60 seconds. Check:

```bash
docker logs skypanel
docker logs skypanel-mysql
```

If you changed the `.env` passwords after the database was created, MariaDB still uses the old ones: see [Troubleshooting](../setup/troubleshooting.md#access-denied-after-changing-env).

### Data does not persist

Data lives in `./storage/` next to `docker-compose.yml`. Check that the containers were created from that folder:

```bash
docker inspect skypanel --format '{{range .Mounts}}{{.Source}} -> {{.Destination}}{{"\n"}}{{end}}'
```
