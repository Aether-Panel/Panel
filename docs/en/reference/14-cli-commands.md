# CLI Commands Reference

Every command and flag of the `SkyPanel` binary (`cmd/panel`). For what each command does in detail, how configuration is loaded and when to use them, see [CLI](../backend/07-cli.md).

```text
SkyPanel [--workDir <dir>] [--config <file>] <command> [subcommand] [flags]
```

In Docker, prefix every command with `docker exec -it skypanel /SkyPanel/bin/SkyPanel`.

---

## Global flags

Available on every command.

| Flag | Default | Description |
|------|---------|-------------|
| `--workDir <dir>` | current directory | Directory to change to before anything else |
| `--config <file>` | `SKYPANEL_CONFIG`, then `/etc/SkyPanel/config.json` if it exists, then `config.json` | Configuration file |
| `-h`, `--help` | | Help for the command (added by Cobra) |

There is no `--version` flag; use `version`.

---

## Commands

| Command | Description |
|---------|-------------|
| `run` | Starts the panel and/or daemon. Hidden from `--help`. |
| `runService` | Same as `run`, with systemd `sd_notify` messages |
| `version` | Prints the version |
| `user add` | Creates a user |
| `user edit` | Edits a user interactively |
| `db upgrade` | Runs the database migrations, with a progress bar and an SQLite backup |
| `db migrate` | Runs the pending database migrations, exactly as `run` does |
| `help [command]` | Cobra's help command |

`user` and `db` on their own only print the list of their subcommands. Shell completion (`completion`) is disabled.

---

## `run`

```bash
SkyPanel run
```

No flags of its own. Starts whatever `panel.enable` and `daemon.enable` turn on, runs the database migrations when the panel is on, and listens on `web.host` until SIGINT/SIGTERM. Details: [Starting the panel](../backend/07-cli.md#starting-the-panel-run-and-runservice).

## `runService`

```bash
SkyPanel runService --config /etc/SkyPanel/config.json
```

No flags of its own. Like `run`; sends `READY=1` and `STOPPING=1` to `$NOTIFY_SOCKET` (`Type=notify` units).

## `version`

```bash
SkyPanel version
# SkyPanel nightly (unknown)
```

Prints `SkyPanel <version> (<hash>)`, both set with `-ldflags` at build time.

## `user add`

```bash
SkyPanel user add [--name <username>] [--email <email>] [--password <password>] [--admin] [--force]
```

| Flag | Type | Description |
|------|------|-------------|
| `--name` | string | Username: required, printable ASCII, 1–100 characters |
| `--email` | string | Email, up to 255 characters |
| `--password` | string | Password, 8–72 bytes. Prompted twice when not given |
| `--admin` | bool | Also grant the `admin` scope (ignored in fully interactive mode, where a prompt asks) |
| `--force` | bool | Delete any user with the same email or username first, then create |

The user always gets the `login` scope. Values not given are prompted for; when at least one of `--name`, `--email` or `--password` is given, an invalid value given as a flag ends the command with exit code 1. See [Creating users](../backend/07-cli.md#creating-users-user-add).

```bash
# First administrator, non-interactive (the password ends up in the shell history)
SkyPanel user add --name admin --email admin@example.com --password 'a-long-password' --admin

# Fully interactive
SkyPanel user add
```

## `user edit`

```bash
SkyPanel user edit
```

No flags. Asks for a username, then offers: **Username**, **Email**, **Password** (ends the user's sessions), **Admin Status** (`yes`/`no`/`cancel`), **Remove 2FA**, **Quit**. Only `root` or members of the `SkyPanel` group may run it. A new username or email must pass the same checks as the API (username required, printable ASCII, 1–100 characters; a valid email of up to 255 characters); a rejected value is not saved and the user keeps the old one. Not having permission, an unreachable database or an unknown username end the command with exit code 1. See [Editing users](../backend/07-cli.md#editing-users-user-edit).

## `db upgrade`

```bash
SkyPanel db upgrade
```

No flags. Runs the schema migrations with a progress bar. With SQLite it saves `<file>.<N>.backup` first and restores it if the upgrade fails. Skips (exit 0) when `panel.enable` is `false`; exit 1 on failure.

## `db migrate`

```bash
SkyPanel db migrate
```

No flags or arguments. Applies the pending schema migrations, the same step `run` takes before it starts, without a progress bar or SQLite backup. Prints `Database migrations complete` and exits 0; skips (exit 0, "Panel not enabled, skipping migrations") when `panel.enable` is `false`; exits 1 when the database cannot be reached or a migration fails. The Docker entrypoint runs it before `run`.

---

## Exit codes

| Code | When |
|------|------|
| `0` | Success, or a command that only prints help |
| `1` | Any failed command: an unknown command or flag, wrong arguments, a `user add` or `user edit` that fails (invalid flag value, database unreachable, unknown user, no permission), a `db upgrade` or `db migrate` failure |
| `2` | An unexpected panic (the stack trace is logged) |

Cobra prints the error as `Error: <message>`.

---

## Quick reference

```bash
SkyPanel version
SkyPanel user add --name admin --email admin@example.com --admin
SkyPanel user edit
SkyPanel db upgrade
SkyPanel --workDir /var/lib/SkyPanel --config /etc/SkyPanel/config.json run

# Docker
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin
docker compose logs -f skypanel
```
