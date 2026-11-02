# CLI (Command Line Interface)

Aether Panel ships as a single Go binary built from `cmd/panel` with [Cobra](https://github.com/spf13/cobra). The same binary runs the panel and the daemon, and has a few administration commands (users, database). The root command is still called `SkyPanel`.

This page explains how the CLI works and when to use each command. The full list of commands and flags, with examples, is in the [CLI commands reference](../reference/14-cli-commands.md).

---

## Where the binary is

| Install | Binary | Configuration |
|---------|--------|---------------|
| Docker image (`docker-compose.yml`, the installer) | `/SkyPanel/bin/SkyPanel` inside the `skypanel` container | `/etc/SkyPanel/config.json` inside the container, plus `SKYPANEL_*` variables from compose |
| systemd unit (`internal/systemd/servicefiles/skypanel.service`) | `/usr/sbin/SkyPanel` | `--config /etc/SkyPanel/config.json` |
| Built by hand (`go build ./cmd/panel`, `make build`) | wherever you put it (`make build` builds the `./cmd/panel` package into `bin/skypanel`; `make run` runs `go run ./cmd/panel run`) | see [Configuration loading](#configuration-loading) |

With Docker every command is run inside the container:

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel <command>
```

`-it` is needed for the interactive prompts of `user add` and `user edit`.

---

## Configuration loading

Before any command runs, the binary:

1. Changes to `--workDir` if it was given (it stops with an error if the folder does not exist).
2. Loads the configuration file, in this order: `--config`, the `SKYPANEL_CONFIG` environment variable, `/etc/SkyPanel/config.json` if it exists, otherwise `config.json` in the working directory. If the file cannot be read it prints `Error loading config, this may impact features` and carries on with defaults and environment variables.

Every key can also come from the environment: `SKYPANEL_` plus the key in upper case with dots turned into underscores (`web.host` → `SKYPANEL_WEB_HOST`, `security.blockPrivateDownloads` → `SKYPANEL_SECURITY_BLOCKPRIVATEDOWNLOADS`). Keys and defaults: [Configuration](./02-configuration.md).

---

## Starting the panel: `run` and `runService`

`run` is what the Docker image starts (`exec /SkyPanel/bin/SkyPanel run` at the end of its entrypoint). It is hidden from `SkyPanel --help` but works normally. `runService` does the same and also talks to systemd.

What a start does, depending on `panel.enable` and `daemon.enable` (both `true` by default):

**Panel** (`panel.enable`)
1. Loads the email service, switches SFTP to database authentication and connects server events to the [extensions](../reference/15-extensions.md).
2. Connects to the database and **runs the schema migrations** (the same as `db migrate`; `db upgrade` adds a progress bar and the SQLite backup). A failure stops the start.
3. Creates `panel.sessionKey` if it is empty and saves it, then sets up the session cookie store.

**Daemon** (`daemon.enable`)
1. Checks that the kernel supports `openat2`; if it does not, the process refuses to start unless `security.forceOpenat` is `true` (see [Security](./09-security.md#file-access)).
2. Starts the SFTP server (`daemon.sftp.host`, default `0.0.0.0:5657`).
3. Prepares Docker mounts, starts the background image prefetch (`docker.prefetchImages`) and the node network history.
4. Creates the servers, binaries and cache folders, adds the binaries folder to `PATH`, loads every server and queues those with auto-start.

Then it listens for HTTP on `web.host` (default `0.0.0.0:8080`). If the panel is disabled it logs a warning when `token.public` is empty, because such a node cannot validate the panel's tokens.

In Docker (`SKYPANEL_PLATFORM=docker`, set by the image) a daemon that fails to start does not stop the process: the web panel keeps running without it. Elsewhere the start is aborted.

**Shutdown** (SIGINT or SIGTERM): HTTP requests in progress get up to 10 seconds, the SFTP server stops, every game server is asked to stop **at the same time** and each gets up to one minute before it is killed, and the database connections are closed. `docker-compose.yml` gives the container 90 seconds (`stop_grace_period`) for this.

`runService` sends `READY=1` to `$NOTIFY_SOCKET` once started and `STOPPING=1` when shutting down, for units with `Type=notify`. Without `NOTIFY_SOCKET` it behaves like `run`.

---

## Managing users

### Creating users: `user add`

Creates an account directly in the database, with the `login` scope and, optionally, `admin`. It is the way to create the first administrator (the installer runs it for you):

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add \
  --name admin --email admin@example.com --admin
```

How the flags and prompts combine:

- If **none** of `--name`, `--email` or `--password` is given, everything is asked interactively, including "Set as Admin" (in this mode the prompt decides, not `--admin`).
- If **any** of them is given, the command runs in flag mode: values that were not given are still prompted for, but a value given on the command line that fails validation ends the command with exit code 1 instead of asking again. `--admin` is taken from the flag.
- The password is asked twice when it is prompted.

Validation is the same as on the web: username required, printable ASCII, 1–100 characters; a valid email of up to 255 characters; a password of 8 to 72 **bytes** (the bcrypt limit; accented letters and emoji take more than one byte).

`--force` deletes any existing user with the same email **and** any with the same username before creating the new one. Use it to reset an administrator account, knowing that it really deletes the old account: its permissions, OAuth2 clients, sessions, recovery codes, SSH keys and user settings go with it, and servers it owned are left without an owner.

### Editing users: `user edit`

Fully interactive, no flags. It asks for a **username** (not an email) and then offers a menu until **Quit** is chosen:

| Option | Effect |
|--------|--------|
| Username | Changes the username |
| Email | Changes the email |
| Password | Sets a new password (8–72 bytes) and **ends every open session** of that user |
| Admin Status | `yes` adds `admin`, `no` removes it, `cancel` does nothing; `login` is always kept |
| Remove 2FA | Turns off two-factor authentication, for a user who lost their device and recovery codes |
| Quit | Leaves |

`user edit` only runs for `root` or members of the `SkyPanel` group; anyone else gets "You do not have permission to use this command". In the Docker image the process runs as the `SkyPanel` user, so `docker exec` works. A new username or email is checked with the same rules as the API and `user add` (username required, printable ASCII, 1–100 characters; a valid email of up to 255 characters); a rejected value prints the error, is not saved, and the user keeps the old one. Lacking permission, a database that cannot be reached or an unknown username end the command with exit code 1.

---

## Database: `db upgrade`

Migrations already run on every start of `run`/`runService`, so `db upgrade` is only needed to migrate without starting the panel (for example before switching binaries, or to see the progress bar).

- With the panel disabled (`panel.enable=false`) it prints "Panel not enabled, skipping upgrade" and exits with 0.
- With **SQLite**, it first copies the database file to `<file>.<N>.backup`, using the first `N` that is free (`skypanel.db.0.backup`, `skypanel.db.1.backup`…). If the upgrade fails, that copy is restored.
- With MySQL/MariaDB, PostgreSQL or SQL Server there is no automatic backup: back up the database yourself first.
- Exit code 1 on any failure.

`db migrate` applies the pending migrations exactly as `run` does (no progress bar, no SQLite backup) and exits: 0 on success or when the panel is disabled, 1 when the database cannot be reached or a migration fails. The Docker entrypoint runs it before `run`, so a failed migration is reported on its own before the panel starts.

Every CLI command that fails (an unknown command or flag, wrong arguments, an error returned by `user add`, `user edit`, `db migrate` or `db upgrade`) exits with code 1; an unexpected panic exits with 2. Full table: [CLI commands reference → Exit codes](../reference/14-cli-commands.md#exit-codes).

---

## Version and builds

`SkyPanel version` prints `SkyPanel <version> (<hash>)`. Both values are set at build time; a plain `go build` gives `SkyPanel nightly (unknown)`:

```bash
go build -ldflags "\
  -X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Version=v3.0.0' \
  -X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Hash=$(git rev-parse --short HEAD)'" \
  -o SkyPanel ./cmd/panel
```

The Dockerfile also sets `internal/config.curseforgeKey` (the default CurseForge API key) from a build argument. The module requires Go 1.26 (`go.mod`), and the frontend must be built first: `client/frontend/dist` is embedded in the binary.

---

## Related

- [CLI commands reference](../reference/14-cli-commands.md)
- [Configuration](./02-configuration.md)
- [Database](./03-database.md)
- [Security](../general/security.md) (2FA recovery with `user edit`)
