# Installation Guide

This guide explains how to install, update and uninstall Aether Panel, and how to set up a secondary node.

> **Docker only.** Aether Panel is installed with Docker only: there are no packages or standalone binaries to install on the system. The installer and the repository's `docker-compose.yml` build the image from the `Dockerfile` on the server itself. Image and Compose details are in [Docker Documentation](../docker/README.docker.md).

## System requirements

| Requirement | Details |
|---|---|
| **Operating system** | Linux with `apt`, `dnf`, `pacman` or `zypper`. The installer recognises Ubuntu, Debian, Fedora, RHEL/Rocky/AlmaLinux/Oracle Linux, Arch (and Manjaro, EndeavourOS…) and openSUSE/SLES, plus their derivatives. Tested on Ubuntu 22.04, 24.04, 26.04 and 26.10, Debian 12, Rocky Linux 9, Arch Linux and openSUSE Tumbleweed. It warns (without stopping) on Ubuntu older than 22.04, Debian older than 12 and RHEL/Rocky/Alma older than 8, which no longer get support. |
| **Architecture** | `amd64` or `arm64`. Anything else is refused. |
| **Memory** | 2 GB recommended: the image is compiled on the server, which uses a lot of memory. Under 1 GB it asks for confirmation; under 3 GB of RAM plus swap it warns that the build may fail and suggests adding swap. |
| **Disk** | 10 GB free recommended on `/opt` (code and data) and `/var/lib/docker` (images). With less, it asks for confirmation. |
| **Docker** | Docker Engine and Docker Compose v2. If missing, the installer installs them (Docker's official repository, with its GPG key fingerprint checked; on Arch and openSUSE, the distribution's packages). `docker-compose` v1 is not supported. |
| **Docker Compose 2.24.4 or later** | Needed whenever the installer writes ports to `docker-compose.override.yml`: when `--port` or `--sftp-port` change, when `--bind` is used (including the automatic `127.0.0.1` with a domain), or when MariaDB's port 3307 is taken. With an older version the installer stops with a message. |
| **systemd** | Recommended. Without it the installer cannot enable Docker or Nginx, nor install the `aether-domain` service. |
| **Access** | `root` (or `sudo`) and Internet access (GitHub, Docker Hub, Let's Encrypt). |

### Ports

| Port | Service | Listens on |
|---|---|---|
| `8080/tcp` | Web panel and API (and the daemon API used by the master panel) | All interfaces; only `127.0.0.1` when the panel is served on a domain through Nginx |
| `5657/tcp` | Built-in SFTP | All interfaces |
| `80/tcp`, `443/tcp` | Nginx in front of the panel (443 only with a domain) | All interfaces |
| `3307/tcp` | The panel's MariaDB | `127.0.0.1` only (if taken, the installer uses the first free one between 3308 and 3399) |
| `8081/tcp` | Adminer (database administration) | `127.0.0.1` only: use it through an SSH tunnel (`ssh -L 8081:127.0.0.1:8081 …`) |
| Game ports | Each game server (for example 25565 for Minecraft) | Published by Docker when each server is created |

The installer opens SSH, the panel, SFTP and the Nginx ports in `ufw` (Debian/Ubuntu/Arch) or `firewalld` (RHEL/Fedora/openSUSE). Docker publishes container ports ahead of those firewalls, so they must also be opened in the provider's firewall (Hetzner, OVH, AWS…), together with the game servers' ports.

---

## Method 1: automatic installer (recommended)

The installer (`tools/install/install.sh`, version 4.2.2) installs Docker if needed, downloads the code, generates the passwords, builds the image, sets up Nginx, HTTPS and the firewall, creates the administrator account and starts the panel.

### Running it

Interactive wizard, as `root` (asks everything step by step):

```bash
sudo -i
bash -c "$(curl -fsSL https://install.aetherpanel.es/install.sh)"
```

Download, review and run:

```bash
curl -fsSL https://install.aetherpanel.es/install.sh -o /tmp/install.sh
sudo bash /tmp/install.sh --help
sudo bash /tmp/install.sh
```

No questions (for example from a script):

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --non-interactive [options]
```

> Avoid `bash <(curl …)`: process substitution (`/dev/fd/N`) is not available everywhere.

### Modes

| Mode | How | What it asks |
|---|---|---|
| Manual | `sudo bash install.sh` in a terminal | Everything: domain and HTTPS, ports, database and administrator account |
| Automatic | `-y` / `--yes` | Only sensitive data (administrator email and password); the rest comes from options or defaults. `-y` also confirms deletions |
| No questions | `--non-interactive`, or input is not a terminal (`curl … \| sudo bash`) | Nothing. Without `--admin-email` or a domain no account is created: the final summary explains how to create it |
| Simulation | `--dry-run` | Shows what it would do without changing the system |

Options are taken in this order: arguments, `AETHER_*` environment variables, the previous install (`/etc/SkyPanel/installer.conf`) and defaults.

### What it does

1. Checks the system: distribution, architecture, memory, disk, and whether there is an install already (and whether an existing `skypanel` container belongs to it; if it belongs to another project, it stops without touching it).
2. Asks for or reads the configuration (domain and HTTPS, ports, database, administrator) and validates all of it before changing anything.
3. Installs the dependencies (`git`, `jq`, Nginx, `certbot` with HTTPS, `ufw` or `firewalld`) and Docker if missing.
4. Checks that the ports are free and says what holds them (another container, another process).
5. Clones `https://github.com/Aether-Panel/Panel.git` into `/opt/skypanel` (branch `main`, the installed one when run again, or another with `-b BRANCH`). If the repository had local changes, it saves a patch in `/etc/SkyPanel/backups/` before discarding them.
6. Generates the server's own MariaDB passwords in `/opt/skypanel/.env` (readable by `root` only) and, when needed, `docker-compose.override.yml`.
7. Builds the image (5 to 15 minutes the first time), sets up the firewall and Nginx, and requests the Let's Encrypt certificate.
8. Writes the base URL and the given options (SFTP, Turnstile, Cloudflare, S3, CurseForge, log retention) into the panel's `config.json` and, behind Nginx, trusts the Docker network gateway as a proxy (`security.trustedProxies`) so the panel sees visitors' real IPs.
9. Starts the panel, waits for it to answer (up to 5 minutes), creates the administrator account and installs the `aether-domain` service.
10. Prints a summary: the panel's address, the ports to open at the provider, the containers and where the credentials are.

Running it again on an existing install is safe: it keeps the data, the database passwords and the accounts.

### What it leaves on the server

| Path | Contents |
|---|---|
| `/opt/skypanel` | Repository clone, where `docker compose` runs |
| `/opt/skypanel/.env` | Database passwords (mode 600) |
| `/opt/skypanel/docker-compose.override.yml` | Only with custom ports, `--bind` or an external database. Outside git, so it survives updates |
| `/opt/skypanel/storage/` | **All data**: MariaDB (`mysql-data`), servers, backups, extensions and the panel's `config.json` (`skypanel-data`), logs (`skypanel-logs`) |
| `/etc/SkyPanel/installer.conf` | The install's options, no secrets; used as defaults when run again |
| `/etc/SkyPanel/credenciales.txt` | Generated passwords (mode 600). Delete it once they are safe |
| `/var/backups/aether-panel/` | Backups the installer takes before updating, reinstalling or uninstalling |
| `/etc/systemd/system/aether-domain.{path,service}` | Service that applies the base URL saved in Settings |

### Managing the panel with the installer

Run without options in a terminal, the installer says whether the panel is **not installed**, **installed but stopped** or **running** (with its branch and version) and shows a menu:

| Option | What it does | Data |
|---|---|---|
| **Install** | Asks for a domain or IP, ports and the database (internal or external) and installs everything. | — |
| **Update** (`--update`) | Backs up the database and checks the copy, pulls the branch, rebuilds the image and restarts. If the new version does not start, it goes back to the previous code and image by itself. | Kept |
| **Reconfigure** (on an existing install) | Changes the domain, ports, etc. | Kept |
| **Reinstall** (`--reinstall`) | Offers a backup, asks you to type `REINSTALAR` (without a terminal, `--yes`), removes the install and installs again from scratch. | Deleted (an external DB is not) |
| **Uninstall** (`--uninstall`, `--uninstall-data`, `--uninstall-all`) | See [Uninstalling](#uninstalling). | Depends on the option |
| **Create administrator account** (`--create-admin`) | Creates an administrator on a running install. | — |

### Updating

There are two ways to update; both use the installed branch (they never switch it):

| | Installer (`--update`) | Panel (**Settings → Updates → Update now**) |
|---|---|---|
| Runs | `install.sh --update` | `tools/panelUpdate/panelUpdate.sh` on each node, from a helper container |
| Database backup first | Yes, verified (unless `--skip-backup`). It does not continue without a valid copy | No |
| Rollback if the new version does not start | Yes (previous code and image, kept as `:anterior`) | No |
| Keeps | `.env`, `docker-compose.override.yml`, the secondary node settings | `.env` and the secondary node settings |

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --update
```

Both build the new image while the panel keeps running and only restart at the end. `panelUpdate.sh` also prunes the build cache, keeping 3 GB. Updating from the panel is described in [Panel updates](../features/panel-updates.md).

`--update` does not reconfigure Nginx or the certificate: to change the domain, run the installer without `--update` (or use the base URL in Settings, see below).

### Uninstalling

| Option | Removes | Keeps |
|---|---|---|
| `--uninstall` | Containers and the Nginx site | Code, `.env` and all data. Bring it back with `--update` |
| `--uninstall-data` | Containers, game servers and data (`storage/`) | Code, `.env` and options |
| `--uninstall-all` | Everything of the panel: containers, image, networks, game servers, `/opt/skypanel`, `/etc/SkyPanel`, the Nginx site, the panel's certificate, the `aether-domain` service and the panel and SFTP firewall rules | Docker, Nginx, `certbot` and the SSH, 80 and 443 rules |

The options that delete data ask you to type `BORRAR` in a terminal, or need `--yes` without one, and in manual mode offer a backup first. Only what belongs to the install is removed: the containers `docker compose` created from `/opt/skypanel` and the panel's game servers (label `skypanel.server`). An external database is never changed or deleted.

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --uninstall
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --uninstall-all --yes
```

### Database

**Internal (default).** MariaDB 10.11 in the `skypanel-mysql` container, with its data in `storage/mysql-data`. Passwords are generated when the database is created and do not change when the installer runs again (`--db-pass`/`--db-root-pass` are then ignored). To change them:

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --update --rotate-db-passwords
```

`--rotate-db-passwords` first saves the new passwords in `credenciales.txt`, applies them in MariaDB and updates `.env`. The installer warns when the database uses one of the repository's public default passwords (such as `skypanel_secret`), because game servers share a network with MariaDB.

**External** (`--db-mode external`). The connection is checked before installing. If the database or the user do not exist, the installer can create them with a MySQL admin account (`--db-admin-user root --db-admin-pass …`, better through the `AETHER_DB_ADMIN_PASSWORD` variable), which is not stored. If the host is `localhost`, `127.*` or `::1` (a database on the same server), the installer replaces it with `host.docker.internal`, because inside the panel's container `localhost` is the container itself: it writes it that way to `.env`, adds `host.docker.internal:host-gateway` to the panel container (`docker-compose.override.yml`), and checks the connection, and takes the `--update` backup, from a container started with `--add-host host.docker.internal:host-gateway`. MySQL then has to listen on the Docker bridge IP (`bind-address`), which the installer reminds you of. For a database on another machine, use its private IP or its name. The password cannot contain spaces, quotes, `$`, `\`, `` ` `` or `#`.

### Domain, HTTPS and Nginx

- **Without a domain**, Nginx answers on port 80 for the IP (and for any domain pointed at the server) and the panel stays available at `http://IP:8080`.
- **With a domain** (`--domain`), Nginx serves the panel on that domain; with `--ssl` it requests a Let's Encrypt certificate validated over HTTP on port 80 (renewed automatically, with Nginx reloaded after each renewal). If issuing fails, the panel stays on HTTP and the summary says how to retry.
- **Panel port closed behind a domain.** With a domain and Nginx, the installer (also with `--update`) publishes the panel's port on `127.0.0.1` only, so nobody reaches it on `http://IP:8080` around HTTPS (Docker publishes its ports ahead of `ufw`, so the firewall was not enough). To keep it open on purpose, pass `--bind 0.0.0.0`.
- **Cloudflare.** The certificate can also be issued with Cloudflare's proxy on, as long as port 80 reaches the server; the first issue fails with "Always Use HTTPS" on. In "Full" SSL mode, Cloudflare connects on 443 and the certificate is needed (a URL with `https://`). Games and SFTP do not go through Cloudflare's proxy: if the domain does not point at the server's IP, the panel gives them the IP.

> A panel served only at `http://IP:8080` sends the administrator's session cookie to every port of that IP, a customer's game server included. A panel with customers should use a domain with HTTPS.

**Adding or changing the domain later.** Point the domain at the server's IP and enter it in **Settings → General → Base URL**, for example `https://panel.example.com`. When you save it, the `aether-domain` service (a systemd `.path` unit that watches the panel's `config.json` and runs `install.sh --apply-panel-url`) sets up Nginx, the certificate and the firewall within seconds, and the result shows under the field. With `http://` it is set up without a certificate; with an IP, Nginx goes back to answering on the IP; a URL with a port (`http://domain:8080`) leaves Nginx alone. If the certificate fails, the panel keeps working over HTTP and the same attempt is not repeated until an hour has passed or the URL changes.

This service does not change how port 8080 is published: after adding the domain this way, run `--update` so the port is bound to `127.0.0.1` only. The service is installed by installer 4.1.3 and later (also with `--update`), only with systemd and without `--skip-nginx`. On an install without it:

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --domain panel.example.com --ssl
```

### Changing the ports

`--port` and `--sftp-port` change the published ports through `docker-compose.override.yml` (needs Docker Compose 2.24.4 or later). The SFTP port is also written to `daemon.sftp.host`, because the panel shows that port to users. The installer refuses the SSH port, 80 and 443 (with Nginx), 3307 and 8081, and ports already used by another container or process.

### Options reference

Every option with a value accepts `--option VALUE` and `--option=VALUE` (use the second form for values starting with `-`).

**Installation**

| Option | Description | Default |
|---|---|---|
| `-b`, `--branch BRANCH` | Git branch | `main`, or the installed one |
| `-d`, `--domain DOMAIN` | Panel domain (Nginx on 80/443) | — |
| `--no-domain` | Remove the domain of a previous install | — |
| `-s`, `--ssl` / `--no-ssl` | Turn HTTPS with Let's Encrypt on or off (needs a domain) | Off |
| `-e`, `--email EMAIL` | Email for Let's Encrypt | — |
| `-p`, `--port PORT` | Panel port | `8080` |
| `--sftp-port PORT` | SFTP port | `5657` |
| `--bind IP` | Publish the panel on that IPv4 only | `0.0.0.0`; `127.0.0.1` with a domain and Nginx |
| `--ip IP` | Server IP for the URL | Detected (the local one without a domain, the public one with a domain) |
| `--master-url URL` | Public panel URL (password reset and sign-in links) | From domain and HTTPS, or `http://IP:port` |
| `--db-mode local\|external` | Database | `local` |
| `--db-host`, `--db-port`, `--db-name`, `--db-user` | Database connection | Port `3306`, name and user `skypanel` |
| `--db-pass`, `--db-root-pass` | Passwords (root only in local mode) | Generated |
| `--rotate-db-passwords` | Change the passwords of an existing local database | — |
| `--admin-email`, `--admin-pass`, `--admin-name` | Initial administrator account (password of 8 to 72 bytes) | Generated password, name `admin` |
| `--no-admin` | Do not create an administrator account | — |
| `--replace-admin` | Delete and recreate a user with that email or name | — |
| `-f`, `--force` | Recreate the containers (data is kept) and continue even if ports are taken | — |
| `-y`, `--yes` / `--non-interactive` / `--dry-run` | See [Modes](#modes) | — |
| `--skip-deps`, `--skip-docker`, `--skip-firewall`, `--skip-nginx` | Skip that step | — |

**Panel features** (optional; can be changed later in Settings)

| Option | Description |
|---|---|
| `--turnstile-site-key`, `--turnstile-secret-key` | Cloudflare Turnstile instead of the built-in captcha (both are required) |
| `--cf-dns-token`, `--cf-zone-id`, `--dns-base-domain` | Automatic subdomains with Cloudflare (token with Zone DNS Edit permission) |
| `--dns-target TARGET` | Record target (default: each node's public IP) |
| `--dns-auto-assign` | Assign a subdomain to new servers |
| `--s3-endpoint`, `--s3-region`, `--s3-bucket`, `--s3-access-key`, `--s3-secret-key`, `--s3-prefix`, `--s3-insecure` | Backups to S3 or compatible (MinIO, R2…). Need at least the bucket and keys; the panel's default prefix is `aether-backups`; `--s3-insecure` for HTTP without TLS |
| `--curseforge-key KEY` | CurseForge API key |
| `--log-retention-days N` | Days of rotated logs to keep (panel default 14; `0` = forever) |

**Maintenance**

| Option | Description |
|---|---|
| `--update` | Update without deleting data |
| `--reinstall` | Delete the install and install again |
| `--skip-backup` | Do not back up the database before updating or reinstalling |
| `--db-admin-user`, `--db-admin-pass` | With an external DB: account that creates the database and user if missing |
| `-u`, `--uninstall`, `--uninstall-data`, `--uninstall-all` | Uninstall |
| `--create-admin` | Only create an administrator account |
| `--apply-panel-url` | Apply the base URL from Settings (used by the `aether-domain` service) |
| `--node --panel URL --join CODE` | Install this machine as a [secondary node](#method-3-secondary-node-daemon-only) with the panel's one-time code (also `AETHER_NODE_PANEL` and `AETHER_NODE_JOIN`) |
| `--debug`, `-V`/`--version`, `-h`/`--help` | Debugging, version and help |

**Environment variables.** Each option has its variable (`AETHER_DOMAIN`, `AETHER_PORT`, `AETHER_ADMIN_PASSWORD`, `AETHER_DB_PASSWORD`, `AETHER_S3_SECRET_KEY`…; the full list is in `--help`), plus `AETHER_YES`, `AETHER_NONINTERACTIVE`, `AETHER_DRY_RUN`, `AETHER_FORCE`, `AETHER_DEBUG` and `AETHER_REPO` (another repository, for forks). Use variables for passwords: arguments end up in the shell history and in `ps`, and the installer warns when passwords are passed that way.

### Examples

```bash
# Unattended install with a domain and HTTPS
curl -fsSL https://install.aetherpanel.es/install.sh | sudo AETHER_ADMIN_PASSWORD='...' bash -s -- \
  --non-interactive --domain panel.example.com --ssl \
  --email admin@example.com --admin-email admin@example.com

# See what it would do, changing nothing
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --dry-run

# External database, creating the database and the user
sudo AETHER_DB_PASSWORD='...' AETHER_DB_ADMIN_PASSWORD='...' bash install.sh \
  --db-mode external --db-host 10.0.0.5 --db-admin-user root

# Create another administrator on an existing install
sudo bash install.sh --create-admin --admin-email other@example.com
```

---

## Method 2: Docker Compose by hand

To control every step, clone the repository and use the included `docker-compose.yml` (MariaDB, panel and Adminer).

### 1. Clone the repository

```bash
git clone -b main https://github.com/Aether-Panel/Panel.git /opt/skypanel
cd /opt/skypanel
```

Use `/opt/skypanel`: updating from **Settings → Updates** runs `tools/panelUpdate/panelUpdate.sh` in that path.

### 2. Set your own passwords

`docker-compose.yml` reads these variables from `.env`:

| Variable | Value without `.env` | Usage |
|---|---|---|
| `DB_ROOT_PASSWORD` | `skypanel_secret` | MariaDB root password |
| `DB_DATABASE` | `skypanel` | Database name |
| `DB_USER` | `skypanel` | Database user |
| `DB_PASSWORD` | `skypanel_secret` | User password |
| `DB_BIND_ADDRESS` | `127.0.0.1` | Host IP where MariaDB is published (port 3307) |

The repository's `.env` contains public example values. Change them **before the first start**: MariaDB only applies them when the database is created, and game servers share a network with it.

```bash
sed -i "s/^DB_ROOT_PASSWORD=.*/DB_ROOT_PASSWORD=$(openssl rand -hex 24)/; s/^DB_PASSWORD=.*/DB_PASSWORD=$(openssl rand -hex 24)/" .env
chmod 600 .env
```

### 3. Build and start

```bash
docker compose up -d --build
docker compose logs -f skypanel
```

The panel creates its tables on start. For other ports, create a `docker-compose.override.yml` with `ports: !override` (Docker Compose 2.24.4 or later).

### 4. Create the administrator

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin
```

Without `--password` the command prompts for it (so it does not end up in the shell history).

A manual deployment has no Nginx, HTTPS or `aether-domain` service: add them yourself (for example a reverse proxy in front of `127.0.0.1:8080`) and set `security.trustedProxies` so the panel sees the real IP. See [Configuration](../backend/02-configuration.md).

---

## Method 3: secondary node (daemon only)

A secondary node runs game servers on another machine and is managed by the master panel. On it the web panel is off (`panel.enable=false`) and only the daemon API (`/daemon`) runs on the HTTP port, so opening that port in a browser gives a 404: that is expected. It has no database: the container does not wait for MySQL or run migrations.

### With one command (recommended)

1. On the master panel, **Settings → General → Base URL**: it must be one the node can reach (for example `https://panel.example.com`), not `localhost`.
2. **Nodes → Add node**: name, public host of the new machine, HTTP port (8080) and SFTP port.
3. **Nodes → node → Deploy → Generate command**, and paste the command as `root` on the new machine:

   ```bash
   curl -fsSL https://raw.githubusercontent.com/Aether-Panel/Panel/main/tools/install/install.sh \
     | sudo bash -s -- --node --panel https://panel.example.com --join <code>
   ```

The code works once and for an hour; generating another one cancels the previous one. The installer:

- trades the code at the panel (`POST /api/node-join`) for the node's client ID, secret and ports, and keeps them in `/etc/SkyPanel/node.json` (root only): if something fails later, the same command is run again without a new code;
- installs Docker and the dependencies, fetches the code into `/opt/skypanel` and builds the image (5 to 15 minutes the first time);
- turns on `SKYPANEL_PANEL_ENABLE=false` in `docker-compose.yml` and writes to `docker-compose.override.yml` that MariaDB and Adminer are not started (profile `panel-database`) and that the health check uses `/daemon`;
- writes `panel.enable`, `panel.settings.masterUrl`, `token.public`, `daemon.auth.url`, `daemon.auth.clientId`, `daemon.auth.clientSecret` and `daemon.sftp.host` to the node's `config.json`;
- opens SSH, the HTTP port and the SFTP port in the firewall, and starts the node.

The node is updated with the panel (Settings → Updates) or with `install.sh --update`: the `SKYPANEL_PANEL_ENABLE=false` line and `docker-compose.override.yml` are kept, and the rest of the configuration lives in `storage/`.

### By hand

In **Nodes → node → Deploy → Show the manual setup**, the panel shows a `docker-compose.yml` with the node's real credentials. These are the values it needs:

| Key | Environment variable | Value |
|---|---|---|
| `panel.enable` | `SKYPANEL_PANEL_ENABLE` | `false` |
| `token.public` | `SKYPANEL_TOKEN_PUBLIC` | `https://<master>/auth/publickey` (when missing, taken from `daemon.auth.url`) |
| `daemon.auth.url` | `SKYPANEL_DAEMON_AUTH_URL` | `https://<master>/oauth2/token` |
| `daemon.auth.clientId` | `SKYPANEL_DAEMON_AUTH_CLIENTID` | `.node_<node ID>` |
| `daemon.auth.clientSecret` | `SKYPANEL_DAEMON_AUTH_CLIENTSECRET` | The node's secret (`GET /api/nodes/{id}/deployment`) |
| `daemon.sftp.host` | `SKYPANEL_DAEMON_SFTP_HOST` | `0.0.0.0:<SFTP port>` |

`daemon.auth.*` is used by the node's SFTP to check users against the master, and the public key at `token.public` to check the requests the master sends it. The node reads it when it starts; when it cannot (the master does not answer or the address is wrong), it tries again at most every 15 seconds while requests arrive, and its log says `This node cannot read the panel's public key at <url>…`. When `daemon.auth.url` is only the master's address, without a path, the node adds `/oauth2/token`; a URL with a path is used as is. Logins with an SSH key are checked at `/auth/sftp-key` of the same address.

`daemon.auth.clientSecret` must be the node's real secret: the master's requests are tied to it, and the node answers them with `401` when it does not match (users then see `ErrNodeAuthFailed`, see [Troubleshooting](./troubleshooting.md#secondary-nodes)). Both machines' clocks must also agree, because the master's requests carry short-lived tokens.

After changing variables in `docker-compose.yml`, apply them with `docker compose up -d`: `docker compose restart` keeps the old ones.

---

## After installing

1. Open the address shown in the summary (`https://your-domain`, or `http://IP:8080` without a domain) with the administrator account. If the password was generated, it is in `/etc/SkyPanel/credenciales.txt` (`sudo cat /etc/SkyPanel/credenciales.txt`). Change it and delete the file.
2. Set up email in **Settings → Mail**: without it, password recovery does not work.
3. Review in Settings the base URL, public registration (on by default), Turnstile, Cloudflare subdomains, off-site backups and CurseForge.

Useful commands (from `/opt/skypanel`):

```bash
docker compose ps                         # status
docker compose logs -f skypanel           # live logs
docker compose restart                    # restart
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name NAME --email EMAIL --admin
docker exec -it skypanel /SkyPanel/bin/SkyPanel user edit   # name, email, password, administrator, remove 2FA
docker exec skypanel /SkyPanel/bin/SkyPanel version
```

If something fails, see [Troubleshooting](./troubleshooting.md).
