# Troubleshooting

Common problems when installing and running Aether Panel, with their cause and how to fix them. Paths assume an install made with the [installer](./01-installation.md) in `/opt/skypanel`.

## Where to look first

```bash
cd /opt/skypanel
docker compose ps                        # are skypanel and skypanel-mysql running?
docker compose logs -f skypanel          # panel output
docker compose logs -f mysql             # MariaDB output
tail -f storage/skypanel-logs/skypanel.log
```

API error responses carry a code (`ErrNodeDiskFull`, `ErrBackupLimit`…) that identifies the problem; the messages quoted here come from the panel's code.

---

## Installation

### The image build fails

The installer ends with "La construcción de la imagen falló" (the image build failed). The usual causes are low memory, low disk or no network. The installer already warns at the start when RAM plus swap is under 3 GB; to add 2 GB of swap:

```bash
fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
```

If disk is short, free Docker space (`docker system prune`) and run the installer again: repeating it is safe and does not delete data.

### "Docker Hub ha limitado las descargas desde esta IP"

Docker Hub rate-limits anonymous pulls per IP, and servers with shared IPs run out. The installer retries three times. Wait a few hours, or sign in with `docker login`, and run the install again.

### "Cambiar puertos necesita Docker Compose >= 2.24.4"

The `docker-compose.override.yml` the installer writes uses `ports: !override`. It is needed as soon as ports are not the defaults, when `--bind` is used (including the automatic `127.0.0.1` with a domain) or when port 3307 is taken. Update Docker (the `docker-compose-plugin` package) or use the default ports without a domain.

### "Puerto N (…) ocupado por …" (port taken)

The installer checks the panel port, SFTP and, with Nginx, 80 and 443, and says which container or process holds them. Free them or choose others with `--port` / `--sftp-port`. `--force` continues despite the conflict. Port 3307 (MariaDB) being taken is not an error: the installer uses the first free port between 3308 and 3399.

### "Ya existe un contenedor llamado 'skypanel' que no pertenece a /opt/skypanel"

Another project has a container with that name. The installer leaves it alone: rename or remove it yourself.

### "Ya hay otra ejecución del instalador en curso"

Another installer run is in progress (lock `/run/aetherpanel-install.lock`). Wait for it to finish.

### The Let's Encrypt certificate is not issued

The installer retries `certbot` once and, if it still fails, leaves the panel on HTTP. Check that:

1. The domain points at the server's public IP (`dig +short your-domain`).
2. Port 80 is also open in the provider's firewall.
3. With Cloudflare's proxy, "Always Use HTTPS" is off for the first issue.

Then run again with `--domain your-domain --ssl`, or save the base URL again in **Settings → General**. The `aether-domain` service does not retry a failed attempt for the same URL until an hour has passed.

### The external database cannot be reached

The installer tests the connection before changing anything and stops with "Base de datos externa no accesible". Check that the database server accepts connections from this IP (`bind-address`, firewall), the user, the password and that the database exists, or pass an admin account to create them (`--db-admin-user`, `AETHER_DB_ADMIN_PASSWORD`). With `localhost`, `127.*` or `::1` the installer connects to `host.docker.internal` (the machine as seen from a container), so a MySQL on the same server must listen on the Docker bridge IP and not only on `127.0.0.1` (`bind-address`); see [Installation](./01-installation.md).

### The panel does not start after installing or updating

The installer waits up to 5 minutes and, if the container stops or restarts three times, prints its last 40 log lines. During an update with `--update` it goes back to the previous version (code and image) by itself and keeps the data; the database backup stays in `/var/backups/aether-panel/actualizaciones/`. Updating from **Settings → Updates** has no such rollback: if it fails, run `install.sh --update` on the server.

If the log ends with `OpenAt2 is not supported. Cowardly not starting to avoid security issues.`, the kernel lacks `openat2` or a seccomp filter (an older Docker profile, for example) blocks it with `EPERM`. Update the kernel or Docker; if that is not possible, see [`security.forceOpenat`](../general/20-security-unshare.md#fallback-securityforceopenat).

---

## Database and panel start-up

### "MySQL no disponible aún" / "MySQL no está disponible después de 60 intentos"

The image's `entrypoint` waits up to 60 seconds for the host and port in `SKYPANEL_PANEL_DATABASE_URL` (`mysql:3306` if not set) and exits otherwise. Check `docker compose logs mysql`. This wait always happens, also with SQLite or with the panel off: the image needs a reachable MySQL/MariaDB.

### "Access denied" after changing `.env`

MariaDB only applies `DB_ROOT_PASSWORD` and `DB_PASSWORD` when the database is created. If they change in `.env` later, the panel can no longer sign in. Put the previous values back (they are in `/etc/SkyPanel/credenciales.txt` if the installer generated them) or change the passwords in MariaDB and `.env` together:

```bash
sudo bash install.sh --update --rotate-db-passwords
```

### "AVISO: la base de datos usa la contraseña por defecto"

The `entrypoint` prints this when the database URL contains `skypanel_secret`, the repository's public value. Game servers share a network with MariaDB, so change it with `--rotate-db-passwords`.

### Other connection errors

- `unknown dialect …`: `panel.database.dialect` must be `sqlite3`, `mysql`, `postgresql` or `sqlserver`.
- `error connecting to database` / `database not available`: the connection URL (`panel.database.url`) is wrong or the server does not answer.
- PostgreSQL and SQL Server: make sure they accept TCP connections from the container (`pg_hba.conf`, TCP/IP enabled).

### Migrations

The panel applies schema migrations on start (`SkyPanel run`); the database user needs permission to create and alter tables. To run them by hand:

```bash
docker exec skypanel /SkyPanel/bin/SkyPanel db upgrade
```

With SQLite, `db upgrade` copies the file first (`<file>.N.backup`) and restores it on failure. With MySQL it takes no copy: use the installer's (`--update` takes one) or `mariadb-dump`. The `entrypoint` step "Ejecutando migraciones" calls `SkyPanel db migrate`, which applies the pending migrations (the same ones `run` applies on start) and exits non-zero when they fail; the `entrypoint` then prints `ERROR: Fallo en migraciones` and starts anyway, and `run` tries again. With the panel turned off (secondary node), `db migrate` does nothing.

---

## Accessing the panel

### `403 {"error":"cross-origin request refused"}`

The panel refuses requests that change something (and console websockets) when they carry its session cookie but another site started them, such as a page served by a game server on the same IP. A request counts as same-origin when `Sec-Fetch-Site` is `same-origin`, or when `Origin` matches `Host`, `X-Forwarded-Host` or the base URL (`panel.settings.masterUrl`). If it happens during normal use behind your own proxy, make the proxy keep the `Host` header (`proxy_set_header Host $host;` in Nginx) or set the base URL in Settings to the address people use. Requests with a Bearer token, from the daemon or from OAuth2 clients are not affected.

### "too many failed attempts, wait a few minutes and try again"

After 10 failed attempts in 15 minutes, sign-in for that account (password or 2FA code) is blocked until attempts leave the window. SFTP passwords share the same counter; SSH key checks for remote nodes have a separate one (see [Attempt limits](../general/security.md#attempt-limits)). The counter is kept in memory: it is cleared when the panel restarts.

### `http://IP:8080` no longer answers after setting up a domain

That is expected: with a domain and Nginx, the installer publishes the panel's port on `127.0.0.1` only and the panel is used through the domain. To open it again, `install.sh --update --bind 0.0.0.0`. See [Domain, HTTPS and Nginx](./01-installation.md#domain-https-and-nginx).

### HTTPS without the installer

The panel only listens on HTTP: HTTPS comes from a reverse proxy. The installer sets up Nginx and Let's Encrypt; for your own deployment, for example:

```nginx
server {
    listen 443 ssl;
    server_name panel.your-domain.com;

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

So the panel sees visitors' real IPs (attempt limits, captcha, logs), declare the proxy in `config.json`:

```json
{
  "security": {
    "trustedProxies": ["127.0.0.1/32"],
    "trustedProxyHeader": "X-Forwarded-For"
  }
}
```

---

## Game servers

### Port already in use

The creation wizard checks the ports on the node before creating anything and warns when the panel itself (web, SFTP, database, Adminer), another server or another container uses them, or when they are below 1024, and suggests the next free one. If a port gets taken later, the start fails with a Docker message such as `driver failed programming external connectivity: port is already allocated`: change the server's port or stop whatever uses it (`ss -tlnup | grep PORT`).

### An install is refused for lack of disk on the node

`ErrNodeDiskFull` ("the node is almost out of disk space, free some before installing"), and in the console: "The node only has X GB free, and an install needs at least Y GB". An install does not start when the node's servers folder has less than `daemon.minFreeDiskMB` MB free (2048 by default; `0` turns the check off), so a download cannot fill the disk and stop every server. Free space or grow the disk. The panel also shows a low-disk alert on the dashboard and in Nodes when less than 5 GB or less than 10 % is free.

### The server does not start: "the server has used all of its disk space"

`ErrDiskLimitExceeded`: the server already uses all of its disk limit and does not boot until files are deleted (SFTP uploads are also cut off at the limit). Delete files or have the limit raised. See [Disk limit](../backend/04-servers.md#disk-limit).

### "environment "tty" is disabled on this node (docker.disallowHost)"

With `docker.disallowHost` on (it is in the Docker image and in `docker-compose.yml`), the node only runs servers in containers: the `host`, `tty`, `standard` and `bubblewrap` environments are hidden, refused when a server is created or redefined and, if a definition gets there another way (a restored backup, an older panel), the start is refused with "This node only runs servers in Docker containers (docker.disallowHost).". Switch the server to a Docker environment.

### A template download is refused

`refusing to download from private address … (security.blockPrivateDownloads)`: by default templates cannot download from private network addresses (the Docker networks, the host's services, the LAN), because some take the URL from a variable the server's owner edits. Loopback and link-local addresses (such as 169.254.169.254) are always refused (`refusing to download from internal address …`). If you keep mirrors on your LAN, set `security.blockPrivateDownloads` to `false`.

### Permission errors inside the game

In Docker, with the panel running as root (as `docker-compose.yml` does), installs run as root and the game as `docker.serverUser` (`988:988` by default), without the Linux capabilities it does not need, with `no-new-privileges` and a process limit (`docker.pidsLimit`, 4096). Before every start the panel gives all of the server's files to that user, so restarting the server fixes files left with another owner. A template that sets its own user keeps it; `docker.serverUser` set to `"0:0"` runs games as root again (not recommended). See [Container user and isolation](../backend/04-servers.md#container-user-and-isolation).

### Another backup cannot be created: "this server already has as many backups as it may keep"

`ErrBackupLimit` (HTTP 409): the server already has `panel.backups.maxPerServer` backups (10 by default). Backups do not count against the server's disk, so without a cap one customer could fill the node's backup disk. Delete an old backup. Administrators are not capped, and `0` turns the cap off for everyone.

### Restoring a backup fails: "cannot backup server, is running"

Restoring a backup needs the server stopped. Stop it and try again.

<a id="16-mysql-connectivity-from-docker-servers-mysql3306-not-resolving"></a>

### A server cannot reach the database (`mysql:3306`)

Game servers are created on the Docker network `docker.serversNetwork` (`skypanel-servers` in `docker-compose.yml`) unless the template picks another. MariaDB is on that network too, so servers reach it as `mysql:3306`, but they cannot reach the panel. The panel is also on that network to talk to the servers (world map, players). If that network does not exist, the panel logs it and uses its own network. Without `docker.serversNetwork`, servers join the panel container's network.

```bash
docker inspect <server-id> --format '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}'
# Should print: skypanel-servers
```

A server created before that change may still be on another network: delete it and create it again.

### Unshare isolation: "Permission denied while starting the security jail"

This only affects the `tty` environment (running directly on the host), which is not available in the Docker install. Ubuntu 24.04 and later restrict the unprivileged user namespaces the isolation uses. The console shows both fixes:

```bash
sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0
```

or turn the isolation off with `"security": {"disableUnshare": true}` in `config.json` (or `"disableUnshare": true` in a server's environment). See [Security and unshare](../general/20-security-unshare.md).

---

## SFTP

The default port is `5657` (`daemon.sftp.host`). The username is `email#serverId` (for example `user@example.com#a1b2c3d4`) and the password is the panel account's, or an SSH public key added to the profile. Keys work on every node: a secondary node asks the panel to check them (`POST /auth/sftp-key`, see [SSH keys for SFTP](../general/security.md#ssh-keys-for-sftp)).

| Error | Cause and fix |
|---|---|
| `incorrect username or password` | Wrong credentials, wrong username format, or the user lacks the `server.sftp` permission on that server. |
| `too many failed attempts…` | The 10 attempts in 15 minutes shared with the panel sign-in are used up. |
| `invalid response from authorization server` (and `error talking to auth server` in the log) | On a secondary node: `daemon.auth.url`, `daemon.auth.clientId` or `daemon.auth.clientSecret` are wrong, or the master does not answer. See [secondary node](./01-installation.md#method-3-secondary-node-daemon-only). |
| An SSH key is refused on a secondary node while the password works | The node runs a version from before key checks on remote nodes, which only offers password sign-in: update it. On a current node, its log shows `Cannot check SFTP keys with the panel` when `daemon.auth.url` is not an absolute `http(s)` URL, and `Error checking an SFTP key with the auth server: [<status>]` when the panel refused the node (`401`: wrong `daemon.auth.clientSecret`; `429`: too many key attempts for that account) or failed. A key that is simply not the user's, or a user without `server.sftp`, is refused like a wrong password. |
| `connection refused` | The port is not open or the panel is not running: `ss -tlnp \| grep 5657`. |

---

## Secondary nodes

- **The browser shows 404 on `http://node:8080`.** That is expected: with `panel.enable=false` only the daemon API runs.
- **"Panel is disabled but token.public (TokenPublicURL) is empty!"** The node cannot verify the master's requests. Set `SKYPANEL_TOKEN_PUBLIC=https://<master>/auth/publickey`.
- **The node does not authenticate with the Deploy dialog's secret.** The dialog shows a fixed secret; the real one is returned by `GET /api/nodes/{id}/deployment`.
- **`502` "The node refused the panel's credentials" (`ErrNodeAuthFailed`).** The node answered the panel with `401` (or a `403` with an authentication challenge), so the panel reports it as a node problem instead of signing the user out. The panel log names the node: `Node "<name>" refused the panel's token (status 401)…`, and the node log shows `Refused a request with an invalid panel token: …`. Check that the node's `daemon.auth.clientSecret` is the secret the panel shows for that node (`GET /api/nodes/{id}/deployment`), that its `token.public` points at this panel's `/auth/publickey`, that both machines have the right time (use NTP: the panel's tokens are valid for 15 minutes) and that no proxy in front of the node asks for its own credentials.
- **The servers of a node have no uptime history, or its Discord node report says "No answer".** The panel polls each remote node every 30 seconds with `GET /daemon/servers/status`. The panel log says `Node <name> did not answer the server status poll…` with the reason: a node that is down or unreachable, one that refuses the panel's token (see above), or one on an older version, which answers `status 404` and must be updated. See [Uptime tracking](../general/13-uptime.md#how-it-is-recorded).
- **The node settings disappear after updating.** Only the `SKYPANEL_PANEL_ENABLE=false` and `SKYPANEL_TOKEN_PUBLIC=…` lines of `docker-compose.yml` are kept; the rest belongs in the `config.json` under `storage/`.

---

## Docker

### The panel cannot connect to Docker

The panel uses the Docker client with the standard variables (`DOCKER_HOST`; by default `unix:///var/run/docker.sock`). In `docker-compose.yml` the host's socket is mounted into the container, which runs as root (`user: "0:0"`). If the daemon cannot start inside Docker, the web panel keeps running and logs it ("Continuing without daemon in Docker environment"), but servers will not be available. Check:

```bash
docker info
docker exec skypanel ls -la /var/run/docker.sock
```

More details in [Docker socket and permissions](../docker/DOCKER-SUDO-FIX.md).

---

## Configuration

### The panel ignores the configured values

The panel reads a single `config.json`: the one from `--config`, the one in the `SKYPANEL_CONFIG` variable, `/etc/SkyPanel/config.json` if it exists, or else `config.json` in the working directory. In the Docker install it is `storage/skypanel-data/config.json` (or `storage/skypanel-config/config.json` if it exists). Changes made in **Settings** are saved to that file.

Any key can be overridden with an environment variable: `SKYPANEL_` prefix, the key in upper case and dots replaced with `_`.

| Environment variable | Key |
|---|---|
| `SKYPANEL_WEB_HOST` | `web.host` |
| `SKYPANEL_DAEMON_SFTP_HOST` | `daemon.sftp.host` |
| `SKYPANEL_PANEL_DATABASE_URL` | `panel.database.url` |
| `SKYPANEL_DOCKER_DISALLOWHOST` | `docker.disallowHost` |
| `SKYPANEL_LOGRETENTIONDAYS` | `logRetentionDays` |

Environment variables **take precedence** over `config.json`, including what Settings saved: a value set in `docker-compose.yml` always wins and a change made in Settings has no effect. That is why `docker-compose.yml` does not set public registration or the theme. The list of keys is in [Configuration](../backend/02-configuration.md).

### Templates do not download

The index loads but individual templates fail: the `templates.url` URL (`https://templates.aetherpanel.es/templates.json` in `docker-compose.yml`) points to a server that only has `templates.json`. Every path the index references (for example `minecraft/minecraft.json`) must exist under the same base path.

### The AI assistant does not answer

The Google Gemini key is missing (`panel.settings.geminiApiKey`, or `SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY`). See [AI assistant](../frontend/04-ai.md).

---

## Logs

The panel writes the current day to `skypanel.log` in the `logs` folder (`/var/log/SkyPanel` in the image; `storage/skypanel-logs` on the host with Docker Compose). Rotation is daily and needs no `logrotate`:

- When the day changes the file becomes `skypanel-YYYY-MM-DD.log.gz` and a new one starts.
- If a single day goes over 100 MB, it is archived early as `skypanel-YYYY-MM-DD.1.log.gz`, `.2`, etc.
- Files older than `logRetentionDays` days (14 by default) are deleted automatically; `0` keeps them all. Only `skypanel-*` files are touched.

| Prefix | Output |
|---|---|
| `[ERROR]` | stderr and file |
| `[INFO]`, `[DEBUG]`, `[SERVER]` | stdout and file |

```bash
tail -f /opt/skypanel/storage/skypanel-logs/skypanel.log
zcat /opt/skypanel/storage/skypanel-logs/skypanel-2026-10-25.log.gz | grep ERROR
```

With an external `logrotate`, set `logRetentionDays` to `0` and, after rotating, send `SIGUSR1` to the process so it reopens `skypanel.log`. Gin's mode is set by `GIN_MODE` (`release` in Docker; `debug` prints every HTTP route).
