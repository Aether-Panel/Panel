# Scheduled tasks

Scheduled tasks automate routine work on a server: daily restarts, automatic backups with retention, warning messages to players before a restart, or any console command at a fixed time. Tasks run on the node by themselves, whether or not anyone has the panel open.

For the internal structure of the scheduler (the `Scheduler` type and its file), see [Servers: Scheduler](../backend/04-servers.md). This page describes the feature as an administrator or server user sees it.

## What it does

- Each server keeps its own list of tasks, up to **50 tasks per server**.
- A task has a name, an optional description, a schedule and an ordered list of **steps**. The steps run one after another; if a step fails, the rest of the task is skipped.
- A task can run on a schedule (cron), or have no schedule at all ("manual only") and run only when someone presses **Run now**.
- Built-in steps for everyone with the right permissions:

| Step (`type`) | UI label | Arguments | What it does |
|---|---|---|---|
| `stdin` | Send command | `command` | Sends one line to the server console, as if typed. |
| `sleep` | Wait | `duration` (for example `5m`) | Pauses the task. |
| `start` | Start | — | Starts the server if it is not already running. If a backup or restore is running, waits for it to finish (up to 60 minutes). |
| `stop` | Stop | — | Stops the server and waits up to 5 minutes for it to exit. |
| `restart` | Restart | — | Stops and starts the server, **only if it is running**. A server its owner switched off is not brought back. If a backup or restore is running, waits for it to finish (up to 60 minutes) before stopping the server, since it could not start it again meanwhile. |
| `backup` | Backup | `keep` (1–50, default 5), `offsite` (`never`, `always`, `weekend`, `weekly`, `biweekly`, `monthly`; `weekly` on a new step, `always` when missing), `offsiteKeep` (1–50, default 4; the larger of `keep` and 4 on a step saved without `offsite` or `offsiteKeep`) | If another backup or restore is running, waits for it to finish (up to 60 minutes). Makes an automatic backup and deletes from the node the oldest automatic backups beyond `keep`. Backups made by hand are never deleted. Depending on `offsite` (**Upload to S3**), also uploads the backup to the bucket and keeps the task's newest `offsiteKeep` there; see [Backups](backups.md#uploading-task-backups). |

- Administrators (holders of the `admin` scope) may also use any operation from the [operations engine](../backend/06-operations.md) (shell commands, downloads, file writes...) and `if` conditions. Other users cannot, because a task runs unattended on the node.

## How it works

- The tasks of a server are stored on the node in `<daemon.data.servers>/<serverId>.cron` (JSON, mode `0600`). The node loads them when it loads the server, so tasks fire even while the game server is stopped.
- Scheduling uses gocron. Schedules are cron expressions with an optional seconds field, an optional `CRON_TZ=` or `TZ=` prefix, and descriptors such as `@daily`. A task with an empty schedule is registered with a run date in the year 9999, so it only runs through **Run now**.
- Each server's scheduler runs at most **5 tasks at the same time**; further runs wait for a free slot (`concurrentLimit: 5`, `limitMode: "wait"` in the `.cron` file). These values are not exposed in the panel or the API.
- While a server is being [transferred to another node](server-transfers.md), its tasks are **paused** on both nodes (`"paused": true` in the `.cron` file, kept across a node restart): runs that fall due and **Run now** are skipped, and the node logs `Task <id> skipped: the server is being moved to another node`. A task already running is not interrupted. The transfer copies the tasks to the new node and resumes them on the node that keeps the server; if the transfer is undone, they resume on the old node.
- When a task runs, the console shows `Running task <name>` and, when it ends, `Task <name> finished`. If a step fails, the console shows `Failed to setup tasks` followed by the error.
- Automatic backups are written as `auto-<task-id>-<uuid>.tar.gz` in the server's backup folder (those of earlier versions, `auto-<uuid>.tar.gz`). The scheduler gives every backup step the task's id when it runs it, **Run now** included, never reading it from the stored arguments. The panel adds them to the **Backups** list (named "Automatic backup") the next time the list is loaded, and removes the entries that retention deleted. Deleting a task does not delete its backups on the node or in the bucket. See [Backups](backups.md).
- The panel always writes a five-field schedule prefixed with the creator's browser time zone, for example `CRON_TZ=Europe/Madrid 0 5 * * *`. "05:00" therefore means 05:00 where the person who created the task lives, whatever time zone the node uses.

### Checks for non-administrators

When the caller does not hold `admin`, the panel validates the task before passing it to the node:

| Rule | Limit |
|---|---|
| Allowed step types | `stdin`, `sleep`, `start`, `stop`, `restart`, `backup` |
| Steps per task | 1 to 10 |
| Name / description length | 64 / 256 characters |
| Command (`stdin`) | one line, up to 512 characters |
| Wait (`sleep`) | more than 0, up to 1 hour |
| `keep` (`backup`) | 1 to 50 |
| `offsite` / `offsiteKeep` (`backup`) | `never`, `always`, `weekend`, `weekly`, `biweekly` or `monthly` / 1 to 50 |
| Schedule | five fields (minute, hour, day of month, month, day of week) or `@yearly`, `@annually`, `@monthly`, `@weekly`, `@daily`, `@midnight`, `@hourly`, optionally behind `CRON_TZ=`/`TZ=`. No seconds field and no `@every`, so a task runs at most once a minute. |
| Body size | 64 KiB |
| Conditions (`if`) | not allowed |

Each step also requires the scope that the matching button needs: `stdin` needs `server.console.send`, `start` needs `server.start`, `stop` needs `server.stop`, `restart` needs both, and `backup` needs `server.backup.create`.

## Using it

### In the panel

Open the server and select the **Tasks** tab (shown to users with `server.tasks.view`).

- With no tasks yet, the tab offers three **Templates**: **Daily restart** (every day at 05:00), **Daily backup** (every day at 04:00, keeps the last 7 and uploads one every week to S3) and **Warn and restart** (sends `say The server restarts in 5 minutes`, waits 5 minutes and restarts).
- **New task** opens the editor:
  - **Name**.
  - **When** → **Frequency**: **Every day** (a **Time**), **Days of the week** (a time and one or more **Days**), **Every few hours** (every 1, 2, 3, 4, 6, 8 or 12 h, on the hour), **Custom (cron)** (a five-field **Cron expression**, for example `30 4 * * 1`) or **Manual only**. The editor shows which time zone the times use.
  - **What it does**: **Add step**, reorder with the arrows, remove. The step list only offers the steps the user is allowed to use. **Wait** offers 1, 5, 10, 15, 30 or 60 minutes. **Backup** has **Keep the last** (1–50), **Upload to S3** (**Never**, **Always**, **Every weekend**, **Every week**, **Every 15 days** or **Every month**; a new step starts with **Every week**) and, except with **Never**, **Keep in S3** (1–50, default 4).
  - **Save task**.
- Each task card shows its schedule, the **Next** run (only for simple schedules in the viewer's own time zone), its steps, and the buttons **Run now**, **Edit** and **Delete**. A step the user cannot edit is shown as "Administrator step: `<type>`".
- **Run now** starts the task immediately; follow its output in the console.

### Via the API

All routes are relative to `/api/servers/:serverId`. Full request and response details are in the [API reference](../reference/11-api-reference.md).

| Method | Path | Scope | Purpose |
|---|---|---|---|
| `GET` | `/tasks` | `server.tasks.view` | List tasks, with their steps (`{"tasks": {"<id>": Task}}`). |
| `GET` | `/tasks/:taskId` | `server.tasks.view` | Get one task. `404` if it does not exist. |
| `PUT` | `/tasks/:taskId` | `server.tasks.create` or `server.tasks.edit` | Create the task, or replace the one with that id. `204` on success. |
| `DELETE` | `/tasks/:taskId` | `server.tasks.delete` | Delete the task. `404` if it does not exist. |
| `POST` | `/tasks/:taskId/run` | `server.tasks.run` | Run the task now. Returns `204` at once; the task continues in the background. |

The task id is chosen by the client: 1 to 64 letters, digits, `-` or `_`. The panel generates ids such as `task-lq3x9k2a`.

Example body for `PUT /api/servers/:serverId/tasks/nightly-backup`:

```json
{
  "name": "Nightly backup",
  "cronSchedule": "CRON_TZ=Europe/Madrid 0 4 * * *",
  "operations": [
    { "type": "stdin", "command": "save-all" },
    { "type": "sleep", "duration": "1m" },
    { "type": "backup", "keep": 7, "offsite": "weekly", "offsiteKeep": 4 }
  ]
}
```

Saving, deleting and running a task are recorded in the server's [activity log](activity-and-access-log.md) (`task.save`, `task.delete`, `task.run`). The steps a task performs when it fires on its own schedule are not recorded there.

## Requirements and configuration

- No configuration key turns the feature on or off; it is always available.
- The `.cron` files live in the folder set by `daemon.data.servers` (default `servers` under the daemon data root; env `SKYPANEL_DAEMON_DATA_SERVERS`). See [Configuration](../backend/02-configuration.md).
- The time zone in `CRON_TZ=` must exist in the node's time zone database; otherwise the node rejects the schedule.

## Limits

- 50 tasks per server; 10 steps per task for non-administrators.
- Non-administrators cannot schedule more often than once a minute.
- `stop` and `restart` give the game 5 minutes to exit; past that, the step fails and the rest of the task is skipped.
- Only one backup per server can run at a time. The `backup`, `start` and `restart` steps wait up to 60 minutes for a backup or restore in progress to finish (console: `Waiting for a backup in progress to finish`); if it is still running after that, the step fails. `restart` waits before stopping the server, so it never leaves it stopped for that reason.
- The **Next** run is only computed in the browser for daily, weekly and hourly schedules in the viewer's time zone. Custom cron expressions are only evaluated by the node.
- Runs that were due while the node was offline, or while the tasks were paused for a node transfer, are not caught up later.
- A node transfer copies the tasks but not the time zone or concurrency settings of the `.cron` file; the new node uses its defaults.

## Troubleshooting

| Message | Cause and fix |
|---|---|
| `invalid task: the task id may only use letters, digits, - and _` | The id in the URL is not 1–64 characters of `A-Z a-z 0-9 - _`. Use a valid id. |
| `invalid task: the task needs a name` | The `name` field is empty. |
| `invalid task: a server can have at most 50 tasks` | Delete tasks you no longer need. |
| `invalid task: <cron error>` | The node could not parse `cronSchedule`. Check the number of fields and the `CRON_TZ=` zone name. |
| `the schedule must have five fields: minute, hour, day of month, month and day of week` | Non-administrators cannot use a seconds field or `@every`. Use five fields or an allowed descriptor. |
| `only administrators can use "<type>" steps` / `only administrators can use conditions` | The step type or an `if` condition is reserved for administrators. |
| `you need the <scope> permission for "<type>" steps` | Ask the server owner or an administrator for that scope. |
| `a task needs between 1 and 10 steps` | Add at least one step, or split long tasks. |
| `a command must be a single line of at most 512 characters` | Remove line breaks or shorten the command. |
| `a wait step can last up to one hour` | Use a `sleep` duration greater than 0 and up to `1h`. |
| `keep must be between 1 and 50` | Adjust the backup step's `keep`. |
| `invalid off-site frequency "…"` / `off-site keep must be between 1 and 50` | Use one of `never`, `always`, `weekend`, `weekly`, `biweekly` or `monthly` for `offsite`, and a number from 1 to 50 for `offsiteKeep`. |
| Console: `Off-site copy skipped: …` | The bucket could not be listed to decide whether the backup uploads. The backup is on the node and the task goes on; see [Backups](backups.md#uploading-task-backups). |
| `the task is too large` (413) | The request body exceeds 64 KiB. |
| Console: `Failed to setup tasks` + `backup in progress` | Another backup or restore was still running after the 60 minutes the step waits. Schedule backups apart from each other. |
| Console: `Server is not running, skipping the restart` | Expected: `restart` does not start a stopped server. Use `start` if that is what you want. |
| The tab says "You do not have permission to see this server's tasks." | The user lacks `server.tasks.view` on this server. |

## See also

- [Servers: Scheduler](../backend/04-servers.md)
- [Operations engine](../backend/06-operations.md)
- [Backups](backups.md)
- [Activity log and access records](activity-and-access-log.md)
- [Scopes reference](../general/scopes-reference.md)
- [API reference](../reference/11-api-reference.md)
