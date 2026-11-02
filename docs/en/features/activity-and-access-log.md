# Activity log and access records

Aether Panel keeps two separate audit trails:

- The **server activity log**: who changed what on a particular server, and when (power actions, console commands, files, backups, users, settings, tasks...).
- The **access records**: panel-wide sign-ins, failed sign-in attempts and account creations, shown on the administrator dashboard.

Both are stored in the panel database and kept for **90 days**.

## What it does

### Server activity log

- Records every **successful** change made through the panel API below `/api/servers/:serverId`: starting and stopping, console commands, file writes, uploads, renames and deletions, archive and extract, plugin, modpack and map installs, backups, server users, settings, scheduled tasks, migrations, version changes, file-history restores and clones.
- Each entry keeps the action key (for example `server.start`, `file.write`, `backup.restore`), a short detail (file name, command, backup name, task name, e-mail of the affected user...), the user who did it, the client IP and the time.
- Groups entries into families for filtering: **Power**, **Console**, **Files**, **Backups**, **Users**, **Settings** and **Tasks**.

### Access records

- Records these events:

| Event | When |
|---|---|
| `login` | A successful sign-in (password, or password plus 2FA code). |
| `login_failed` | A rejected password or 2FA code for an **existing** account. Unknown e-mail addresses are not recorded, so the public form cannot be used to fill the table. |
| `register` | An account was created, by self-registration or by an operator through the users API. |
| `login_sso` | A sign-in through a one-time SSO link from the billing system. |

- Each event keeps the username and e-mail (copied, so the record survives renames and deletions), the IP and the browser user agent. The API only returns a short summary of the user agent, such as `Chrome · Windows`, `Safari · iOS` or `CLI`.

## How it works

### Server activity log

- A middleware on every `/api/servers` route looks at mutating requests (`POST`, `PUT`, `DELETE`, `PATCH`). After the request finishes, if the response status is below 400, it writes the entry in the background. Failed requests are never recorded, and a failure to write the entry never fails the request.
- `POST /ai/analyze`, `POST /ai/chat` and `POST /notifications/test` are ignored because they change nothing. A mutating route without its own action key is recorded as `server.change` with the route as detail.
- Details are trimmed: console commands to 200 characters, any detail to 300 characters. Changes to settings, data, flags, notifications, ports and the server definition only record **that** they changed, never the body, because it may contain secrets (webhooks, passwords).
- Changes made outside the panel API are not recorded: SFTP transfers, and what scheduled tasks do when they fire on their own schedule (saving, deleting and running a task by hand *are* recorded).
- A node transfer runs in the background after the request was answered, so the panel records its outcome itself: `server.transferFailed` when a step failed and the transfer was undone, and `server.transferStartFailed` when the server moved but could not be started on the new node. The detail says at which step and why, and the page shows it as code (see [Server transfers](server-transfers.md)).
- When a server is deleted through the panel, its whole log is deleted with it. Retention runs at most once an hour while new entries are written: it removes entries older than 90 days and entries of servers that no longer exist.

### Access records

- Sign-in, registration and SSO handlers record the event as it happens. Rows older than 90 days are deleted each time the list is read.
- The event types `logout` and `password_reset` exist in the data model but are not recorded at the moment.

## Using it

### In the panel

**Server activity.** Open the server and select the **Activity** tab. The tab is shown to every user of the server, but the list requires `server.users.view` on that server; without it the tab shows "You can't see the activity" and "Ask the server owner for permission to view users."

- Entries are grouped by day (**Today**, **Yesterday**, then dates), newest first, 50 at a time; **Load more** fetches older ones and **Refresh** reloads.
- **Filter by type**: **All**, **Power**, **Console**, **Files**, **Backups**, **Users**, **Settings**, **Tasks**.
- The IP address of each entry is only shown to administrators (holders of the `admin` scope).

**Access records.** The administrator dashboard (shown to users with `nodes.view` or `users.info.search`) has an **Access records** card with the latest 6 events: who, when (relative time), the event (**Sign-in**, **Failed attempt**, **Billing sign-in**, **Account created**) and the IP. The card refreshes once a minute. Reading it requires `users.info.search`; a dashboard user who only holds `nodes.view` does not query the records, and the card then shows the empty-state text ("No access recorded yet...").

### Via the API

Full details are in the [API reference](../reference/11-api-reference.md).

| Method | Path | Scope | Purpose |
|---|---|---|---|
| `GET` | `/api/servers/:serverId/activity` | `server.users.view` | A page of the server's activity, newest first. |
| `GET` | `/api/activity` | `users.info.search` | A page of access records, newest first. |

`GET /api/servers/:serverId/activity` query parameters:

| Parameter | Default | Description |
|---|---|---|
| `limit` | `50` | Entries per page, 1 to 100 (larger values are capped at 100). |
| `before` | — | Only entries with an id lower than this one (the cursor from the previous page). |
| `type` | all | A family (`power`, `console`, `files`, `backups`, `users`, `settings`, `tasks`), `all`, or an action prefix such as `file` or `backup.restore` (lowercase letters with at most one dot). Any other value returns no entries. |

Response:

```json
{
  "items": [
    { "id": 812, "action": "file.write", "detail": "server.properties", "username": "alice", "userId": 3, "ip": "203.0.113.7", "createdAt": "2026-10-30T18:22:41Z" }
  ],
  "nextCursor": 763
}
```

`ip` is only present for administrators. `nextCursor` is `null` on the last page.

`GET /api/activity` query parameters: `event` (`login`, `login_failed`, `register` or `login_sso`), `page` (default 1) and `limit` (default 10, maximum 100). The response is `{"activity": [...], "metadata": {"paging": {...}}}`; each item has `id`, `event`, `username`, `email`, `ip`, `userAgent` (summary) and `createdAt`.

### Action keys

| Family | Action keys |
|---|---|
| Power | `server.start`, `server.stop`, `server.restart`, `server.kill`, `server.install`, `server.reload` |
| Console | `console.command` |
| Files | `file.write`, `file.upload`, `file.mkdir`, `file.rename`, `file.delete`, `file.archive`, `file.extract`, `file.restore`, `plugin.install`, `plugin.delete`, `modpack.install`, `map.install`, `map.remove` |
| Backups | `backup.create`, `backup.restore`, `backup.delete` |
| Users | `user.add`, `user.edit`, `user.remove` |
| Settings | `settings.data`, `settings.ports`, `settings.definition`, `settings.flags`, `settings.notifications`, `server.create`, `server.rename`, `server.suspend`, `server.unsuspend`, `server.transfer`, `server.transferFailed`, `server.transferStartFailed`, `server.change`, `server.clone`, `extransfer.create`, `extransfer.pull`, `extransfer.cancel` |
| Tasks | `task.save`, `task.delete`, `task.run` |

`server.version` (game version change) is recorded but belongs to no family: it only appears under **All** or with `type=server`.

## Requirements and configuration

- Both trails are always on. No configuration key enables, disables or changes their retention; the 90-day window is fixed in code.
- They use the panel database configured with `panel.database.*`. See [Configuration](../backend/02-configuration.md).

## Limits

- Retention is 90 days for both trails.
- A page of server activity holds at most 100 entries; a page of access records at most 100.
- Only requests through the panel API are recorded. SFTP activity and unattended task runs do not appear in the server activity log.
- Access records do not include sign-outs or password resets.
- Requests that fail (status 400 or higher) are not recorded, so denied attempts to change a server do not appear in its activity log.

## Troubleshooting

| Symptom or message | Cause and fix |
|---|---|
| `invalid limit` (400) | `limit` is not a positive integer. |
| `invalid cursor` (400) | `before` is not a non-negative integer. Pass the `nextCursor` value from the previous page. |
| The **Activity** tab says "You can't see the activity" | The user lacks `server.users.view` on that server. The owner can grant it in the **Users** tab. |
| Filtering returns nothing with a custom `type` | The value is not a family and not a lowercase prefix like `file` or `backup.restore`. |
| A change made over SFTP is missing | Expected: only panel API requests are recorded. |
| The **Access records** card says "Your account is not allowed to read the access records." | `/api/activity` answered `403`: the account lacks `users.info.search`. |
| The **Access records** card says "This panel build does not serve the access records yet." | `/api/activity` failed with anything other than `403` (for example `404` on an old panel build, a `500`, or a network error). Check the panel log, and update the panel if it is an old build. |
| No IP is shown in the server activity | Expected for non-administrators: the API omits `ip` unless the caller holds `admin`. |

## See also

- [Scheduled tasks](scheduled-tasks.md)
- [Security](../general/security.md)
- [Scopes reference](../general/scopes-reference.md)
- [API reference](../reference/11-api-reference.md)
