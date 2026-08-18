# CLI Commands Reference

Aether Panel includes a CLI built with **Cobra** that allows managing the panel, creating users, running migrations, and more.

> **Note:** The binary is compiled from `cmd/panel/main.go`. The internal name of the root command is `SkyPanel`, but the binary file is generated as `skypanel` (or `skypanel.exe` on Windows).

---

## 0. Usage with Docker

If you run the panel inside a Docker container (image `ghcr.io/aether-panel/panel`), you cannot run the binary directly on the host. Use `docker exec` to interact with the CLI of the running container.

### Running Commands in the Container

```bash
# Access the container shell
docker exec -it skypanel sh

# Or run commands directly
docker exec -it skypanel /SkyPanel/bin/SkyPanel version
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --admin
docker exec -it skypanel /SkyPanel/bin/SkyPanel db upgrade
```

> **Binary path inside the container:** `/SkyPanel/bin/SkyPanel`

### Using docker-compose

With the `docker-compose.yml` file included in the repository:

```bash
# Start services
docker compose up -d

# Run CLI in the running container
docker compose exec skypanel /SkyPanel/bin/SkyPanel user add --admin

# View logs
docker compose logs -f skypanel
```

### Example: Create Admin in Docker

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add \
  --name "admin" \
  --email "admin@example.com" \
  --admin
```

### Example: Migrate Database in Docker

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel db upgrade
```

### Notes for Docker

- The binary inside the container is at `/SkyPanel/bin/SkyPanel` (with a capital S).
- The config inside the container is at `/etc/SkyPanel/config.json`.
- Persistent data is in `/var/lib/SkyPanel/`.
- You can verify that the container is running with `docker ps` before running commands.

---

## 1. Compilation (without Docker)

### Prerequisites
- Go 1.25+
- Access to `cmd/panel/main.go`

### Basic Build
```bash
go build -o skypanel ./cmd/panel
```

### Build with Custom Version
```bash
go build -ldflags "-X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Version=1.2.0' -X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Hash=$(git rev-parse --short HEAD)'" -o skypanel ./cmd/panel
```

### Using Makefile
```bash
make build    # Generates bin/skypanel
make run      # Runs go run ./cmd/panel/main.go
```

---

## 2. Available Commands

| Command | Description |
|---------|-------------|
| `run` | Starts the full panel (web, SFTP daemon, scheduler) |
| `runService` | Starts as a systemd service with `NOTIFY_SOCKET` notification |
| `version` | Shows the panel version |
| `user` | User management (subcommands: `add`, `edit`) |
| `db` | Database operations (subcommands: `upgrade`, `migrate`) |

### Global Flags

| Flag | Description |
|------|-------------|
| `--workDir` | Changes the working directory before starting |
| `--config` | Path to the JSON configuration file |

Both flags are available in **all** subcommands.

---

## 3. `run` — Start the Panel

```bash
./skypanel run
```

Initializes and starts all panel services:

1. Loads the configuration from `config.json`.
2. Connects to the database (SQLite, MySQL, PostgreSQL, or SQL Server).
3. Starts the **HTTP server** (REST API + web panel) on the configured port (default `8080`).
4. Starts the **SFTP server** (default port `5657`).
5. Starts the **task scheduler** (gocron).
6. Starts the **game server** management (loads existing servers, starts the daemon).
7. Shows real-time console logs.

```bash
# Use custom directory and config
./skypanel run --workDir /opt/skypanel --config /opt/skypanel/production.json
```

---

## 4. `runService` — systemd Service

```bash
./skypanel runService
```

Identical to `run`, but also notifies systemd via `NOTIFY_SOCKET`:
- Sends `READY=1` when the panel is ready.
- Sends `STOPPING=1` during shutdown.

Useful for integrating with systemd units of type `Type=notify`.

---

## 5. `version` — Version

```bash
./skypanel version
```

Shows the panel version. By default: `SkyPanel nightly (unknown)`. It can be customized at compile time with `-ldflags` (see section 1).

---

## 6. `user` — User Management

### 6.1. `user add` — Create User

```bash
./skypanel user add --name "admin" --email "admin@example.com" --admin --password "secure_password"
```

All flags are **optional**. If any is omitted, the system will prompt for it interactively:

| Flag | Description |
|------|-------------|
| `--name` | Username |
| `--email` | Email address |
| `--admin` | Grants administrator permissions |
| `--password` | Password (if omitted, prompted with confirmation) |

The command:
1. Validates the username format, email, and password strength.
2. Connects to the database.
3. Creates the user with `ScopeLogin` + `ScopeAdmin` permissions (if `--admin`).

### 6.2. `user edit` — Edit User (Interactive)

```bash
./skypanel user edit
```

**Fully interactive** command (no flags). Steps:

1. Prompts for the username to edit.
2. Shows an interactive menu with options:

   | Option | Action |
   |--------|--------|
   | **Username** | Change username |
   | **Email** | Change email address |
   | **Password** | Change password |
   | **Admin Status** | Add or remove administrator permissions |
   | **Remove 2FA** | Disable two-factor authentication |
   | **Quit** | Exit |

3. Allows making multiple changes in the same session.

---

## 7. `db` — Database

### 7.1. `db upgrade` — Migrations

```bash
./skypanel db upgrade
```

Runs the database schema migrations. Useful after updating the panel to a new version.

- **SQLite:** Makes an automatic backup of the file (`skypanel.db.0.backup`, `skypanel.db.1.backup`, ...) before migrating.
- **Other dialects:** Runs migrations directly.
- If the migration fails, it automatically restores the backup (SQLite).

### 7.2. `db migrate` — Change Engine (Experimental)

```bash
./skypanel db migrate
```

> **Note:** This command is currently a **stub** and does not perform any action. It is designed to migrate data between database dialects (e.g., SQLite → MySQL) in the future.

---

## Quick Summary

```bash
# Compile
go build -o skypanel ./cmd/panel

# Version
./skypanel version

# Create admin
./skypanel user add --name admin --email admin@example.com --admin

# Edit user (interactive)
./skypanel user edit

# Start panel
./skypanel run

# Start as service
./skypanel runService

# Migrate database
./skypanel db upgrade

# With global flags
./skypanel --workDir /data/skypanel --config /data/skypanel/config.json run
```
