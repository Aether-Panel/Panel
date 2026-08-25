# Troubleshooting Guide

This document details solutions to common problems encountered during the deployment and operation of Aether Panel.

---

## 1. Isolation with Unshare (Ubuntu 24.04+)

**Problem:** `Permission denied` error when starting the "security jail" or the game server.

**Cause:** Ubuntu 24.04+ restricts by default the use of *unprivileged user namespaces* (`unshare`), which the panel uses to isolate game server processes.

**Symptom in logs:**
```
[ERROR] error starting server testserver: fork/exec /bin/bash: operation not permitted
```

#### Solutions:

*   **Option A (Recommended for production):** Enable namespaces in the kernel:
    ```bash
    sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0
    ```
*   **Option B (Disable isolation):** Add to `config.json`:
    ```json
    {
        "security": {
            "disableUnshare": true
        }
    }
    ```
    It can also be configured per server in its individual JSON with `"disableUnshare": true` in the environment (tty) section.

---

## 2. Docker Connection

**Problem:** The panel cannot connect to the Docker engine.

**Symptom in logs:**
```
[ERROR] Cannot connect to the Docker daemon
```

**Cause:** The panel uses the Docker SDK with `client.FromEnv()`, which reads the standard Docker environment variables.

#### Solutions:

*   **Verify that Docker is running:**
    ```bash
    docker info
    ```
*   **Verify socket permissions:** The user running the panel must have permissions to access the Docker socket:
    ```bash
    sudo usermod -aG docker $USER
    # Log out and log back in
    ```
*   **Use a custom Docker socket:** Configure via environment variable:
    ```bash
    export DOCKER_HOST=unix:///var/run/docker.sock
    ```
*   **Running the panel inside Docker:** The panel automatically detects `SKYPANEL_PLATFORM=docker` and skips the Docker verification, continuing without the internal Docker engine.

---

## 3. SFTP — Connection and Authentication

**Problem:** Unable to connect to the panel via SFTP.

**Default port:** `5657`

#### Solutions:

**Error: `incorrect username or password`**
*   **Database authentication:** The user format is `email#serverId` (e.g., `user@example.com#abc123`). Verify that the user has the `ScopeServerSftp` permission assigned.
*   **OAuth2 authentication:** Verify that the OAuth2 authentication server is accessible and returns the `sftp` scope for the corresponding server.

**Error: `error talking to auth server`**
*   Verify that `daemon.auth.url` points to the correct panel URL (default: `http://localhost:8080`).
*   Verify that `daemon.auth.clientId` and `daemon.auth.clientSecret` are configured.

**Error: `no access` / `invalid response from authorization server`**
*   The OAuth2 server rejected the credentials or the requested scope is not available.

**Error: `connection refused`**
*   The SFTP port (5657) is not open or the panel is not running.
*   Check with: `ss -tlnp | grep 5657`

---

## 4. Database

**Problem:** Database connection error when starting the panel.

**Supported dialects:** `sqlite3`, `mysql`, `postgresql`, `sqlserver`

**Common symptoms:**

*   `dial tcp 127.0.0.1:3306: connect: connection refused` — MySQL/MariaDB is not running.
*   `could not load driver` — Incorrect dialect or driver not compiled.

#### Solutions:

*   **For SQLite (recommended for testing):**
    ```json
    {
        "panel": {
            "database": {
                "dialect": "sqlite3",
                "url": "skypanel.db"
            }
        }
    }
    ```
*   **For MySQL/MariaDB:** Verify that the service is running and that the user has permissions:
    ```bash
    mysql -u skypanel -p -h 127.0.0.1 skypanel
    ```
*   **For PostgreSQL:** Verify that `pg_hba.conf` allows connections from `localhost`.
*   **For SQL Server:** Verify that TCP/IP is enabled in the server configuration.

---

## 5. Ports in Use

**Problem:** `address already in use` error when starting the panel.

**Default ports:**
| Service | Port | Config Key |
|----------|--------|------------|
| HTTP (Web) | `8080` | `web.host` |
| SFTP | `5657` | `daemon.sftp.host` |

#### Solutions:

*   **Check which process is using the port:**
    ```bash
    ss -tlnp | grep -E '8080|5657'
    ```
*   **Change the port** in `config.json`:
    ```json
    {
        "web": {
            "host": "0.0.0.0:9090"
        },
        "daemon": {
            "sftp": {
                "host": "0.0.0.0:6565"
            }
        }
    }
    ```
*   It can also be changed via environment variables:
    ```bash
    export SKYPANEL_WEB_HOST=0.0.0.0:9090
    export SKYPANEL_DAEMON_SFTP_HOST=0.0.0.0:6565
    ```

---

## 6. File Permissions (UID/GID)

**Problem:** Files created by the panel have incorrect owners or permission errors.

**Behavior:** The panel assigns UID/GID to server files based on the configured user. If the UID is `-1`, ownership is not changed (it uses the process user).

#### Solutions:

*   **Verify the UID/GID of the panel process:**
    ```bash
    ps aux | grep skypanel
    ```
*   **Docker containers** inherit the UID/GID of the panel process automatically.
*   **TTY environment with unshare:** The process inside the jail runs as root (UID 0) mapped to the real system user. Files created inside the jail will belong to the real user outside the jail.
*   **If there are permission errors** reading/writing server files, verify that the panel user has access to the `servers/`, `cache/`, and `binaries/` directories.

---

## 7. CORS — Connections from the Frontend

**Problem:** The frontend cannot make requests to the API (CORS errors in the browser console).

**Behavior:** The panel allows **all origins** (`AllowOriginFunc` always returns `true`). This is intentional to support deployments where the frontend and backend are on separate domains.

#### Solutions:

*   If there are CORS errors, verify that the frontend is using the correct API URL.
*   Verify that the `Authorization` and `Content-Type` headers are included in the requests.
*   If a reverse proxy (nginx, Caddy) that modifies headers is used, make sure it does not strip the CORS headers.

---

## 8. Environment Variables and Configuration

**Problem:** The panel does not use the values you configured.

**Behavior:** All `config.json` settings can be overridden with environment variables using the `SKYPANEL_` prefix and replacing `.` with `_`.

### Examples:

| Environment Variable | Config JSON |
|---------------------|-------------|
| `SKYPANEL_WEB_HOST` | `web.host` |
| `SKYPANEL_DAEMON_SFTP_HOST` | `daemon.sftp.host` |
| `SKYPANEL_PANEL_DATABASE_URL` | `panel.database.url` |
| `SKYPANEL_PANEL_DATABASE_DIALECT` | `panel.database.dialect` |
| `SKYPANEL_LOGS` | `logs` |
| `SKYPANEL_PANEL_SETTINGS_COMPANYNAME` | `panel.settings.companyName` |

Environment variables **take precedence** over the `config.json` file.

---

## 9. Logs and Debugging

**Problem:** You need more information to diagnose an error.

**Behavior:** The panel writes logs to `logs/skypanel.log` with automatic rotation upon receiving `SIGUSR1`.

**Log levels:**
| Prefix | Level | Destination |
|---------|-------|---------|
| `[ERROR]` | Error | Stderr + file |
| `[INFO]` | Info | Stdout + file |
| `[DEBUG]` | Debug | Stdout + file |
| `[SERVER]` | Server | Stdout + file |

#### Solutions:

*   **View logs in real time:**
    ```bash
    tail -f logs/skypanel.log
    ```
*   **Force log rotation** (without restarting the panel):
    ```bash
    kill -USR1 $(pidof SkyPanel)
    ```
*   **Increase detail level:** Start with `GIN_MODE=debug` to see all HTTP routes:
    ```bash
    GIN_MODE=debug ./SkyPanel run
    ```

---

## 10. SSL/TLS (HTTPS)

**Problem:** You need HTTPS for production.

**Behavior:** The panel **does not include native HTTPS support**. It listens only on plain HTTP.

### Solution:

Use a reverse proxy for SSL termination (nginx, Caddy, Traefik):

**Example with Caddy:**
```caddyfile
panel.yourdomain.com {
    reverse_proxy localhost:8080
}
```

**Example with nginx:**
```nginx
server {
    listen 443 ssl;
    server_name panel.yourdomain.com;

    ssl_certificate /etc/ssl/certs/panel.crt;
    ssl_certificate_key /etc/ssl/private/panel.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

If you use a trusted proxy, configure in `config.json`:
```json
{
    "security": {
        "trustedProxies": ["127.0.0.1/32", "10.0.0.0/8"],
        "trustedProxyHeader": "X-Forwarded-For"
    }
}
```

---

## 11. Database Migrations

**Problem:** Error running migrations or the panel does not start after an update.

#### Solutions:

*   **Run migrations manually:**
    ```bash
    ./SkyPanel db upgrade
    ```
    This command performs an automatic backup before migrating.
*   **If the migration fails**, verify that the database user has permissions to create/modify tables.
*   **SQLite:** The `skypanel.db` file must have write permissions for the panel user.

---

## 12. Templates

**Problem:** The panel loads the template index but fails to download individual templates.

**Cause:** The URL configured in `templates.url` points to a server that only has the `templates.json` but not the JSON files for each template.

**Solution:** Make sure the template server has the complete structure. If `templates.json` references `minecraft/minecraft.json`, that file must be accessible at the same base path.

---

## 13. AI Assistant (Google GenAI)

**Problem:** The AI assistant does not respond or shows errors.

**Cause:** The Google Gemini API Key has not been configured.

**Solution:** Configure in `config.json`:
```json
{
    "panel": {
        "settings": {
            "geminiApiKey": "your-gemini-api-key"
        }
    }
}
```
Or via environment variable:
```bash
export SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY=your-gemini-api-key
```

---

## 14. Configuration File

**Problem:** The panel cannot find or ignores the configuration file.

**Behavior:** By default, the panel looks for `config.json` in the current working directory. A custom path can be specified with the `--config` flag or the `SKYPANEL_CONFIG` environment variable.

### Included configuration files:

*   `config.json` — Main configuration (customizable).
*   `config.docker.json` — Predefined configuration for the Docker environment.
*   `config.linux.json` — Predefined configuration for Linux (local SQLite).

```bash
./SkyPanel run --config config.linux.json
```

---

## 16. MySQL Connectivity from Docker Servers (`mysql:3306` not resolving)

**Problem:** Docker container servers cannot connect to MySQL using the `mysql` hostname (e.g., error `Communications link failure... database address 'mysql:3306' accessible`).

**Cause:** The server container was created in Docker's default `bridge` network, while MySQL runs in the internal `skypanel-network`. Containers in different networks cannot resolve each other by hostname.

**Implemented Solution (v2.0.1+):**
Aether Panel now automatically detects the Panel container's Docker network (`skypanel-network` or `panel_skypanel-network` depending on docker-compose project name) and connects all servers to that network.

**Verification:**
```bash
# Check panel network
docker inspect skypanel --format '{{range $k, $v := .NetworkSettings.Networks}}{{$k}}{{end}}'
# Should show: panel_skypanel-network (or skypanel-network)

# Check server network
docker inspect <server-id> --format '{{range $k, $v := .NetworkSettings.Networks}}{{$k}}{{end}}'
# Must match the panel's network
```

**If the server already exists:** Delete and recreate the server so it joins the correct network.

**Native installation (without Docker):** Not applicable. Use host IP or `host.docker.internal` in Database Host config.

---

## 17. Frontend Build Error: `monaco-editor/min/vs/editor/editor.main.css?inline`

**Problem:** `astro build` fails with:
```
Rolldown failed to resolve import "monaco-editor/min/vs/editor/editor.main.css?inline"
```

**Cause:** `monaco-editor@0.56.0+` introduced `package.json` `exports` restrictions that prevent resolving CSS imports with `?inline` under Vite 8/Rolldown.

**Solution:** Keep `monaco-editor` at `^0.44.0` in `client/frontend/package.json`:
```json
"monaco-editor": "^0.44.0"
```
CSS is injected inline and codicon font served from jsDelivr CDN pointing to version 0.44.0.