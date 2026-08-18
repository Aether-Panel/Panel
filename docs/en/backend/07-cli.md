# CLI (Command Line Interface)

The `SkyPanel` binary uses **Cobra** for the command-line interface.

## Usage

```bash
SkyPanel [--config file] [--workDir directory] <command>
```

## Commands

### `run`

Starts the server (HTTP panel + daemon). It is the main production command.

```bash
SkyPanel run
```

- Initializes logging, DB, migrations
- Registers HTTP routes
- Starts servers: HTTP (Gin), SFTP, Daemon
- Graceful shutdown with signal handling

### `version`

Displays the binary version.

```bash
SkyPanel version
# → SkyPanel v3.x.x (ldflags)
```

The version is injected with ldflags during build: `-X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Version=v3.x.x'`

### `user`

Interactive user management.

#### `user add`

Creates a new user with an interactive wizard or flags:

```bash
# Interactive:
SkyPanel user add

# With flags (useful for scripting in Docker):
SkyPanel user add --name admin --email admin@example.com --password secret --admin
```

Flags: `--name`, `--email`, `--password`, `--admin`, `--force` (recreates the user if it already exists)

Flow:
1. Prompt for username (or flag)
2. Prompt for email (or flag)
3. Prompt for password with confirmation (or flag)
4. Admin confirmation
5. Connects to DB, creates user with `services.User.Create()`
6. Assigns scopes: always `login`, plus `admin` if applicable

#### `user edit`

Interactive editor to modify existing users:

```bash
SkyPanel user edit
```

Available options in the menu:
- Username
- Email
- Password
- Admin Status
- Remove 2FA
- Quit

### `db`

Database operations.

#### Subcommands (defined in `db.go`, `dbmigrate.go`, `dbupgrade.go`)

| Command | File | Description |
|---|---|---|
| `db migrate` | `dbmigrate.go` | Stub (no `Run` implemented). Designed to migrate between DB dialects in the future |
| `db upgrade` | `dbupgrade.go` | Runs the schema migrations (gormigrate + `AutoMigrate`). With SQLite it creates a `*.N.backup` and restores it on failure |

### `runService`

Internal command for systemd operations (runs a specific service as a subprocess).

```bash
SkyPanel runService <serviceName>
```

## Build

```bash
# Standard build
go build -o SkyPanel ./cmd/panel

# Build with ldflags for version
go build -ldflags "-X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Version=v3.0.0'" -o SkyPanel ./cmd/panel
```

## Docker

```bash
# Execute commands in a running container
docker exec -it <container> SkyPanel user add --name admin --email admin@example.com

# View logs
docker logs -f <container>
```
