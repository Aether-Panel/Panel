# Backups

Aether Panel keeps full copies of a server's files so they can be downloaded or put back after a mistake, a broken plugin or a lost disk. There are four kinds of copies: **backups** made by hand, **automatic backups** made by a scheduled task, **periodic copies** the node makes of each server straight into the bucket, and **restore points** taken by the panel itself just before a risky change. Optionally, an S3-compatible bucket keeps copies away from the node (**off-site backups**): each server's periodic copy and the copies of the tasks that ask for it. Backups made by hand stay on the node only.

## What it does

- **Create** a full backup of the server folder as a `.tar.gz` archive.
- **List** every backup with its size, kind (manual, automatic, periodic, restore point) and where it is kept (on the node, off-site, or both).
- **Download** a backup to your computer.
- **Restore** a backup: every current file of the server is deleted and the archive is extracted in its place.
- **Delete** a backup (from the node and, when off-site backups are on, from the bucket).
- **Automatic backups** from a scheduled task, with their own retention (keep the newest *N*) and how often they also go to the bucket (never, always, every weekend, every week, every 15 days or every month).
- **Restore points** before plugin installs/removals, game version changes, modpack installs, world map installs and backup restores, so that step can be undone.
- **Off-site copies** in AWS S3, Backblaze B2, Cloudflare R2, Wasabi, MinIO or any S3-compatible service, with a **periodic copy** of each server (every 12 hours to 30 days) even when it has no tasks, and a retention of their own in the bucket.

## How it works

### Where backups live

Backups are made and stored by the daemon of the node that hosts the server, outside the server folder:

```
<daemon.data.root>/<daemon.data.backups.folder>/<server-id>/
```

With the Docker installation (`daemon.data.root` = `/var/lib/SkyPanel`, folder `backups`) this is `/var/lib/SkyPanel/backups/<server-id>/` inside the container, which `docker-compose.yml` maps to `./storage/skypanel-data/backups/<server-id>/` on the host.

Backups **do not count against the server's disk limit**. That is why the number of backups a server's users may keep is capped (see [Requirements and configuration](#requirements-and-configuration)).

The panel database keeps one record per backup (name, file name, size, location, date). The file itself only exists on the node (and in the bucket).

### File names

| Kind | File name | Name shown in the list | Goes to the bucket? |
|------|-----------|------------------------|---------------------|
| Manual backup | `<uuid>.tar.gz` | The name the user typed | No |
| Automatic backup (scheduled task) | `auto-<task-id>-<uuid>.tar.gz` | "Automatic backup", with the "Automatic" badge | Depending on the task's frequency ([Uploading task backups](#uploading-task-backups)) |
| Periodic copy | `offsite-<uuid>.tar.gz` | "Periodic backup (S3)", with the "Periodic" badge | Always, and only there ([Periodic copy](#periodic-copy)) |
| Restore point | `restore-<reason>-<uuid>.tar.gz` (`plugin`, `version`, `modpack`, `restore`) | "Restore point" badge and the reason | No |

Task backups made by earlier versions are named `auto-<uuid>.tar.gz`, without the task id. They are still automatic backups (listed and pruned on the node like the rest), but no task recognises them as its own in the bucket, so the bucket's retention never deletes them: they stay there until someone deletes them.

Retention of automatic backups only ever removes `auto-` files from the node, each series in the bucket only removes its own copies, and restore point pruning only removes `restore-` files: a backup made by hand is never deleted automatically.

### Creating

1. The panel checks the per-server cap (`panel.backups.maxPerServer`) unless the caller is an administrator (`admin` scope). Periodic copies do not count toward that cap.
2. The node refuses if another backup or restore of that server is already running (`backup in progress`): only one runs at a time per server.
3. The node answers straight away with the file name and writes the archive in the background. The panel stores the record at that moment.
4. The archive is written under a hidden name (`.partial-<name>`) and renamed when complete, so a half-written backup is never listed, downloaded or restored. Files are listed and opened beneath the server root, so a symlink swapped in while the backup runs cannot pull host files into the archive. Every entry is stored under a folder named after the server ID.
5. The console shows `Backing up server` and then `Backup complete` or `Backup failed`. The global Discord backup alert ([Discord](../general/16-discord.md)) and the per-server backup notification are sent. When the archive of a manual backup could not be written, the node marks it as failed (a hidden `.failed-<name>` file) and the panel deletes its record the next time the list is opened, so no entry is left without a file behind it.
6. A manual backup stays on the node only: it is not uploaded to the bucket even when off-site backups are on. Earlier versions did upload manual backups; the ones already in the bucket stay there and are listed, downloaded, restored and deleted as before. For copies away from the node there are the [periodic copy](#periodic-copy) and [uploading task backups](#uploading-task-backups).

The server does **not** have to be stopped to make a backup: the archive is a copy of the files as they are on disk at that moment. A game that writes its world continuously may leave those files mid-change; for a fully consistent copy, stop the server or make the game save first.

### Restoring

Restoring is destructive and runs in the background:

1. The server must be **stopped** and not installing, and no other backup or restore may be running.
2. If the local archive is gone but the bucket still has it, it is downloaded first (console: `Downloading backup from off-site storage`). A failed download leaves the server untouched.
3. The whole archive is read and checked **before anything is deleted**. An unreadable archive, or one with an entry the extraction would refuse, stops the restore with `Backup file cannot be read: …`.
4. A restore point (`restore-restore-…`) of the current files is written. If it cannot be written, the restore does not go ahead.
5. Every file of the server is deleted and the archive is extracted.
6. The console shows `Restoring server` and then `Restore complete` or `Restore failed`.

### Restore points

Restore points are quick backups taken just before a risky step:

| Reason | When | If the restore point cannot be made |
|--------|------|--------------------------------------|
| `plugin` | Installing or removing a plugin, installing a world map | The step goes ahead without one |
| `version` | Changing the game version | The change is cancelled |
| `modpack` | Installing a modpack | The install is cancelled, nothing is changed |
| `restore` | Restoring a backup | The restore is cancelled |

- Only the **3 newest** restore points are kept; older ones are deleted.
- For plugins, no new restore point is taken if one is less than **30 minutes** old, so a row of installs does not create one per plugin. An install takes it only once Modrinth has a compatible version with a valid file name, right before the download; a removal takes it only once the file is found. A refused install or a wrong name leaves no restore point behind.
- Restore points are kept on the node only; they are not uploaded off-site.
- They appear in the backup list and are restored, downloaded and deleted like any other backup.

### Automatic backups (scheduled tasks)

A scheduled task can include a **Backup** step with these values:

| Value | Field in the panel | Range | Default | Purpose |
|-------|--------------------|-------|---------|---------|
| `keep` | **Keep the last** | 1-50 | 5 | Automatic backups kept on the node |
| `offsite` | **Upload to S3** | `never`, `always`, `weekend`, `weekly`, `biweekly`, `monthly` | `weekly` on a new step; `always` on one saved before the option existed | Which runs also upload their backup to the bucket ([Uploading task backups](#uploading-task-backups)) |
| `offsiteKeep` | **Keep in S3** (hidden with **Never**) | 1-50 | 4 | Copies of that task kept in the bucket |

The step waits until the archive is finished, so the task's next step sees it, and then deletes from the node the oldest `auto-` backups beyond `keep` (every task backup of the server counts together). That pruning only deletes the file on the node: the copies the task uploaded to the bucket stay there, with a retention of their own. The **Daily backup** preset backs up every day at 04:00, keeps the last 7 on the node and uploads one every week (keeping 4 in the bucket). Creating a task with a backup step requires the `server.backup.create` scope. Tasks are described in [Scheduled tasks](./scheduled-tasks.md).

Each time the backup list is opened, the panel compares its records with the files on the node and in the bucket: automatic backups, periodic copies and restore points made by the node are added, those removed by retention and the manual backups the node marks as failed are dropped (and the node is told to remove the mark), and sizes and locations are updated. A task backup that retention deleted from the node but is still in the bucket stays in the list as **Off-site only**.

### When a server is deleted or transferred

- **Deleting a server** also deletes its backups on the node (the folder `<daemon.data.backups.folder>/<server-id>/`), its periodic copy setting (`<id>.offsite`) and its records in the panel. Off-site copies in the bucket are **not** deleted, periodic ones included.
- **Deleting a task** does not delete its backups: those on the node stay until another task's retention or a person deletes them, and those in the bucket stay there and remain in the list (badge **Off-site** or **Off-site only**), where they can be deleted by hand.
- **Transferring a server to another node** ([Server transfers](./server-transfers.md)) copies the backups kept on the old node (location `local` or `both`, restore points included) to the new node through the panel, with their names and dates, and then deletes them from the old node with the server. The records follow the server. Off-site copies stay where they are: the bucket is the same for every node. If the transfer fails, the copies it made on the new node are removed and the old node keeps its own.
- The transfer also copies the server's own periodic copy setting (`GET` and `PUT /daemon/server/:id/offsite`), just as it copies its tasks. When the server follows the default there is nothing to copy. An old node that does not know the route (answers `404`) counts as "no setting of its own"; any other failure reading it or saving it on the new node fails the transfer, which is undone. While the transfer runs, that server's periodic copy is not made on either node.

### Off-site copies

When off-site backups are on and complete (endpoint, bucket, access key and secret key filled in):

- Two series of copies go to the bucket, under `<prefix>/<server-id>/<file>`: those of the tasks whose turn it is to upload (the local copy stays) and each server's periodic copy (kept in the bucket only). Manual backups and restore points are not uploaded.
- The list merges both places: `local`, `offsite` (only in the bucket) or `both`.
- **Deleting** a backup removes it from the node and from the bucket.
- **Downloading** a backup that is only off-site streams it from the bucket.
- **Restoring** a backup that is only off-site downloads it to the node first.
- If the bucket cannot be listed (wrong key, network), the local list is shown as is and the error is logged.

Timeouts: upload or download 2 hours per transfer, listing 15 seconds for the backup list, 1 minute to list the bucket before deciding an upload or a prune, deletion 1 minute.

#### Uploading task backups

Each **Backup** step decides with `offsite` (**Upload to S3**) which runs upload their backup to the bucket. The decision is made after the backup is written on the node, from the newest copy **of that task** in the bucket (the object's date, that is, when it finished uploading) and the current time in the timezone of the server's scheduler (its tasks' timezone; the node's when none is set):

| Value | Panel | Uploads when… |
|-------|-------|---------------|
| `never` | Never | Never. |
| `always` | Always | Always. |
| `weekend` | Every weekend | Today is Saturday or Sunday **and** the task has no copy in the bucket since 00:00 on this weekend's Saturday. |
| `weekly` | Every week | The task has no copy in the bucket since 00:00 on this week's Monday. |
| `biweekly` | Every 15 days | The task's newest copy in the bucket is 15 days minus 2 hours old or older (or there is none). The 2 hours of slack keep a daily task at a fixed time from skipping a day because the previous upload finished a few minutes later. |
| `monthly` | Every month | The task has no copy in the bucket since 00:00 on the 1st of this month. |

- A step saved before this option existed, without `offsite`, counts as **Always**, so no task stops uploading without anyone asking. A new step created from the panel starts with **Every week**.
- The other runs keep their backup on the node only.
- The upload runs in the background (console: `Uploading backup to off-site storage`, then `Backup uploaded to off-site storage` or `Off-site upload failed: …`), so the task goes on with its steps without waiting for it. A failed upload does not fail the task: the backup is already on the node, and the next run decides again.
- While the same task's previous upload has not finished, the next run does not upload (it would not see it in the bucket yet and would upload twice); its backup stays on the node.
- If the bucket cannot be listed, the run uploads nothing and the console shows `Off-site copy skipped: …`; the task does not fail.
- A task's copies are recognised by the task id in the name (`auto-<task-id>-<uuid>.tar.gz`). The scheduler always sets the id when it runs the task, never from the stored arguments, so nobody can name another task and prune its copies.

#### Periodic copy

Each server can get a full copy that goes straight to the bucket every so often, whether it has tasks or not. The daemon of the node that hosts the server makes it:

- **Interval**: `daemon.backups.s3.periodicHours`, one of `0`, `12`, `24`, `48`, `72`, `168` (7 days), `336` (14 days) or `720` (30 days). `0`, the default, turns it off, so nothing changes until an administrator picks an interval. The copies kept in the bucket are `daemon.backups.s3.periodicKeep` (1 to 50, default 4).
- **Per-server setting**: each server can use the default, turn its copy off, or pick another interval and another number of copies. It is stored on the node, in `<daemon.data.servers>/<id>.offsite` (JSON, `0600` permissions): `{"periodicHours": null, "periodicKeep": null}`, where `null` means "default". Without the file, the server follows the defaults. A file edited by hand with an interval that is not on the list turns the copy off, and a number of copies outside 1-50 uses 4.
- **One worker per node**: every 10 minutes it goes through the servers one at a time, so they never all back up at once. The first pass starts 5 minutes after the node starts.
- **When it backs a server up**: off-site backups are on and complete on that node, the effective interval is above 0, and that server's last periodic copy in the bucket is older than the interval. The interval counts from when the previous copy finished uploading, and since the worker passes every 10 minutes (and waits for each copy before moving to the next server), it is a minimum, not an exact time.
- **When it leaves it for another pass**: the server is being installed or moved to another node, or another backup or restore of it is running. A running game is no reason: the copy is taken hot, like a manual one, and never stops the server.
- **The copy and its upload**: before writing anything, the node checks that the bucket answers. It then writes `offsite-<uuid>.tar.gz` in the backup folder, uploads it, deletes from the bucket the periodic copies of that server beyond the ones kept (oldest first) and deletes the file on the node. The local file is also deleted when the upload fails, so retries do not fill the disk. On start, the node deletes any `offsite-` files left in the backup folders (for instance, one a restart left half uploaded): the periodic copies that count are those in the bucket.
- **Alerts**: a periodic copy that works is silent (only the console shows `Backing up server`, `Backup complete` and the upload). If the backup or the upload fails, or the bucket does not answer, the server's owner ([Server notifications](./server-notifications.md)) and the global Discord backup alert are told as for a failed backup, and the console shows why (for example `Periodic off-site backup skipped, the bucket is not available: …`).
- **Retries**: after a failure, that server waits 1 h, 2 h, 4 h… up to 24 h before trying again; a copy that uploads resets the wait. That way a bucket down for a whole day does not produce copies that are then thrown away.
- **The last copy** is read from the bucket the first time the worker reaches the server after the node starts, and the node then remembers it in memory. The server's **Copy to S3** card shows that date.
- The backup list shows periodic copies as **Periodic backup (S3)**, with the **Periodic** and **Off-site only** badges. They are downloaded, restored and deleted like any other backup, and they do not count toward `panel.backups.maxPerServer`.

#### Retention in the bucket

- Each task keeps its newest `offsiteKeep` copies in the bucket; when an upload succeeds, the oldest copies **of that task** are deleted.
- The periodic copy keeps each server's newest `periodicKeep`; when an upload succeeds, that server's oldest periodic copies are deleted.
- Each series (each task, and each server's periodic copy) only counts and deletes its own copies: never another series', manual ones uploaded by earlier versions, nor the old `auto-<uuid>` ones without a task id.
- Pruning the copies on the node (`keep`) no longer deletes anything in the bucket. **Deleting** a backup from the panel still removes it from both places.

## Using it

### In the panel

**Server › Backups** tab:

- **Create Backup**: type a **Name** and confirm. The backup appears in the list immediately; its size is filled in once the archive is written (reload the list).
- Each row shows the name, date, size and badges: **Automatic**, **Periodic** (periodic copy), **Restore point** (with the reason, e.g. "Before a plugin change"), **Off-site** (also in the bucket) or **Off-site only** (local copy gone).
- **Restore** asks for confirmation ("All current files will be overwritten"). Stop the server first.
- **Download** opens the archive download in a new tab.
- **Delete** asks for confirmation.
- The header shows **Space used by backups**, the largest backup and how many are also off-site.

Above the list, the **Copy to S3** card shows that server's periodic copy:

- **How often**: **Default (…)** (the panel's value, in brackets when known), **Off**, 12 h, 24 h, 48 h, 72 h, 7 days, 14 days or 30 days.
- **Keep in S3**: 1 to 50; empty or the **Default** button goes back to the panel's value.
- **Save** applies the changes. Changing it takes `server.backup.create`; with only `server.backup.view` the card can be seen but not changed.
- Under the title, the date of the **Last periodic copy** or "No periodic copy yet.".
- When the server's node has no S3 set up, the card says "This node has no S3 set up." and disables the controls. When the node does not answer, it shows a notice with **Retry**. Without `server.backup.view`, or with a node from an earlier version without the periodic copy, the card is not shown.

Automatic backups are configured in the **Tasks** tab (step **Backup**, preset **Daily backup**): next to **Keep the last** there are **Upload to S3** (Never, Always, Every weekend, Every week, Every 15 days, Every month) and, except with **Never**, **Keep in S3**. With a frequency other than **Never** or **Always**, a line reminds that the other runs keep the backup on the server only.

Off-site storage is configured by an administrator in **Settings › Off-site backups**:

1. Optionally click a provider (Backblaze B2, Cloudflare R2, AWS S3, Wasabi) to fill in the endpoint and region.
2. Fill in **Endpoint**, **Bucket name**, **Region** (needed by some providers; `auto` on R2) and **Folder in the bucket** (default `aether-backups`).
3. Fill in **Access key** and **Secret key** for a key allowed to read, write and delete in that bucket.
4. Turn on **Unencrypted connection (http)** only for a MinIO on your own network.
5. Click **Test connection**: the panel writes and deletes a small object in the bucket using the values on screen, saved or not.
6. Turn on **Upload backups to the bucket**.
7. Under **Periodic copy of each server**, pick the default **Interval** (**Off**, 12 h, 24 h, 48 h, 72 h, 7 days, 14 days or 30 days) and the **Copies to keep in S3** (1 to 50). Each server can change it in its **Backups** tab.
8. Save.

### Via the API

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| `GET` | `/api/servers/:serverId/backup` | `server.backup.view` | List backups (syncs automatic backups and restore points first) |
| `GET` | `/api/servers/:serverId/backup/:backupID` | `server.backup.view` | Get one backup record |
| `POST` | `/api/servers/:serverId/backup/create?name=<name>` | `server.backup.create` | Start a backup; `204` when started |
| `POST` | `/api/servers/:serverId/backup/restore/:backupID` | `server.backup.restore` | Start a restore; `204` when started |
| `GET` | `/api/servers/:serverId/backup/download/:backupID` | `server.backup.view` | Download the archive |
| `DELETE` | `/api/servers/:serverId/backup/:backupID` | `server.backup.delete` | Delete the backup (node and bucket) |
| `GET` | `/api/servers/:serverId/offsite` | `server.backup.view` | Read the server's periodic copy (answered by its node) |
| `PUT` | `/api/servers/:serverId/offsite` | `server.backup.create` | Change the server's periodic copy; `204` |
| `POST` | `/api/settings/test/s3` | `settings.edit` | Test S3 settings; body values override the saved ones (a `********` secret key uses the saved one) |

`GET /api/servers/:serverId/offsite` answers `{"periodicHours": null, "periodicKeep": 6, "effectiveHours": 24, "effectiveKeep": 6, "configured": true, "lastPeriodic": "2026-11-05T04:12:00Z"}`: the server's own setting (`null` = default), the values it ends up using, whether its node has S3 set up, and the date of the last periodic copy the node knows of (`null` when it knows none). `PUT` takes `{"periodicHours": 48, "periodicKeep": null}`; an interval not on the list answers `400 ErrOffsitePeriodicHours` and a number of copies outside 1-50, `400 ErrOffsiteKeep`. The defaults are changed with `POST /api/settings` (keys `daemon.backups.s3.periodicHours` and `daemon.backups.s3.periodicKeep`, with the same checks; see [Panel settings](./panel-settings.md)).

A backup record looks like `{"id": 12, "name": "before update", "fileName": "…tar.gz", "fileSize": 104857600, "location": "both", "createdAt": "…"}`. Errors from the node (server running, backup in progress…) are passed through with the node's status code. Full details: [API reference — Backups](../reference/api/04-server-content.md#backups). Scopes: [Scopes reference](../general/scopes-reference.md).

## Requirements and configuration

| Key | Default | Environment variable | Description |
|-----|---------|----------------------|-------------|
| `panel.backups.maxPerServer` | `10` | `SKYPANEL_PANEL_BACKUPS_MAXPERSERVER` | Backups a server's users may keep; administrators are not capped; `0` = no limit. Not editable from the Settings page |
| `daemon.data.backups.folder` | `backups` | `SKYPANEL_DAEMON_DATA_BACKUPS_FOLDER` | Backup folder; relative paths are resolved under `daemon.data.root` |
| `daemon.backups.s3.enabled` | `false` | `SKYPANEL_DAEMON_BACKUPS_S3_ENABLED` | Turn the bucket on: the periodic copy and the backups of the tasks whose turn it is to upload |
| `daemon.backups.s3.endpoint` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_ENDPOINT` | `host`, `host:port` or a full URL (its scheme decides http/https) |
| `daemon.backups.s3.region` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_REGION` | Region, when the provider needs it |
| `daemon.backups.s3.bucket` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_BUCKET` | Bucket name (must already exist) |
| `daemon.backups.s3.accessKey` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_ACCESSKEY` | Access key |
| `daemon.backups.s3.secretKey` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_SECRETKEY` | Secret key |
| `daemon.backups.s3.prefix` | `aether-backups` | `SKYPANEL_DAEMON_BACKUPS_S3_PREFIX` | Folder inside the bucket |
| `daemon.backups.s3.insecure` | `false` | `SKYPANEL_DAEMON_BACKUPS_S3_INSECURE` | Plain http (MinIO on a local network) |
| `daemon.backups.s3.periodicHours` | `0` | `SKYPANEL_DAEMON_BACKUPS_S3_PERIODICHOURS` | Default interval of the periodic copy, in hours: `0` (off), `12`, `24`, `48`, `72`, `168`, `336` or `720` |
| `daemon.backups.s3.periodicKeep` | `4` | `SKYPANEL_DAEMON_BACKUPS_S3_PERIODICKEEP` | Periodic copies kept in the bucket per server, 1 to 50 |

Off-site backups are only active when `enabled` is true **and** endpoint, bucket, access key and secret key are all set. Each server's own setting (`<daemon.data.servers>/<id>.offsite`) takes precedence over `periodicHours` and `periodicKeep`. See [Configuration](../backend/02-configuration.md) for the full list and how environment variables map to keys.

**Remote nodes:** uploads, listings, deletions and the periodic copy run in the daemon of the node that hosts the server, which reads the `daemon.backups.s3.*` keys (`periodicHours` and `periodicKeep` included) from its own configuration. The **Settings › Off-site backups** page writes the panel's `config.json`, so it applies to servers on the panel's own (local) node. On a remote node, set the same keys in that node's `config.json` or environment; otherwise its servers get no periodic copy and do not upload their task backups, and their **Copy to S3** card says "This node has no S3 set up.". A server's own setting does work on any node, since it is stored on the node that hosts it, but the "default" it follows is that node's. The **Test connection** button tests from the panel host.

## Limits

- One backup or restore at a time per server.
- The cap counts the server's backup records: manual backups, automatic backups (also those left only in the bucket) and restore points. Periodic copies do not count: the administrator imposes them and, if they counted, a high `periodicKeep` could keep users from making backups by hand.
- Backup names must be printable ASCII (the record is validated with `printascii`); accented letters or emoji are rejected.
- Restore always replaces **all** files of the server; there is no partial restore. To recover a single file, download the backup and upload that file with the file manager or SFTP.
- Restore points: 3 kept, local only.
- Automatic backup retention: `keep` from 1 to 50 on the node, `offsiteKeep` from 1 to 50 in the bucket. Periodic copy: `periodicKeep` from 1 to 50.
- Manual backups are not uploaded to the bucket, and there is no button to upload one by hand.
- The periodic copy is made by a single worker per node, server after server: on a node with many large servers a pass can take hours, and the copies arrive later than the interval says.
- Cloning a server does not copy its backups ([Cloning servers](./cloning.md)).
- When a server or a task is deleted, its copies in the off-site bucket stay there; remove them by hand if they are no longer needed.
- A node transfer copies every backup through the panel, so it takes longer and needs room on the new node for them. Both nodes must run a current version: the new node needs the `/daemon/server/:id/backup/file` route to receive them, and both need the scheduler pause routes the transfer uses (see [Server transfers](./server-transfers.md#requirements-and-configuration)). When the server has its own periodic copy setting, the new node also needs the `/daemon/server/:id/offsite` route; without it the transfer fails and is undone.
- The S3 settings saved in **Settings › Off-site backups**, the periodic copy's defaults included, only apply to the panel's own node; each remote node needs its own (see [Remote nodes](#requirements-and-configuration)).

## Troubleshooting

| Message / symptom | Cause | Fix |
|-------------------|-------|-----|
| "This server already has as many backups as it may keep. Delete an old one before making another." (`409`, `ErrBackupLimit`) | `panel.backups.maxPerServer` reached | Delete old backups (including automatic ones or restore points), or raise the cap in the configuration |
| `backup in progress` (`ErrBackupInProgress`) | Another backup, restore or restore point of that server is running | Wait for `Backup complete` / `Restore complete` in the console |
| `cannot backup server, is running` (`ErrBackupServerRunning`) or `server running` on restore | The server is running or installing | Stop the server and retry the restore |
| `name is required` on create | Empty name | Give the backup a name |
| A manual backup disappears from the list shortly after it was made | Its archive could not be written (console showed `Failed to create backup file` / `Backup failed`); the panel drops the record when it syncs | Check the node's free disk and make the backup again |
| `Backup file cannot be read: …` during restore | Corrupt or incomplete archive | Nothing was deleted; pick another backup |
| `Could not create a restore point: …` | Not enough space on the node's backup disk, or write error | Free space on the node; restore and version changes will not run without it |
| "The backups could not be loaded" in the tab | The node did not answer | Check the node is online ([Troubleshooting](../setup/troubleshooting.md)) |
| Test: `off-site backups are not configured` (`400`) | Endpoint, bucket, access key or secret key missing | Fill in all four |
| Test: `the bucket "…" does not exist` | Wrong bucket name or region | Create the bucket or fix the name |
| Test: `could not write to the bucket: …` / `could write but not delete in the bucket: …` | The key lacks write or delete permission | Give the key read, write and delete rights on the bucket |
| Console: `Off-site upload failed: …` after a task backup | Network error or wrong credentials | The local backup is fine; fix the settings on the node. The next run whose turn it is uploads again |
| Console: `Off-site copy skipped: …` after a task backup | The bucket could not be listed to decide the upload | The local backup is fine and the task did not fail; fix the settings or wait for the bucket to answer |
| A task set to **Every week** (or weekend, 15 days, month) did not upload its backup | The bucket already has a copy of that task within the period, or the task's previous upload was still running | Expected: that run keeps the backup on the node only |
| A manual backup no longer shows as **Off-site** | Manual backups are no longer uploaded to the bucket | Use the periodic copy or a task with **Upload to S3** |
| Console: `Periodic off-site backup skipped, the bucket is not available: …` and a failed backup alert | The bucket does not answer or does not exist | Fix the node's S3 settings; the node tries again after 1 h, 2 h, 4 h… (up to 24 h) |
| Node log: `Error reading the off-site settings, no periodic copy until it is fixed: …` | The server's `<id>.offsite` file is damaged | Save the setting again from the **Copy to S3** card, or delete the file to go back to the defaults. Meanwhile that server gets no periodic copy |
| "This node has no S3 set up." on the **Copy to S3** card | The server's node does not have the `daemon.backups.s3.*` keys complete, or `enabled` is `false` (usual on a remote node) | Set the keys in that node's configuration (see [Remote nodes](#requirements-and-configuration)) |
| "This server's copy to S3 could not be read: the node did not answer." | The node did not answer | Check the node is online and click **Retry** |
| "The periodic off-site copy can only run every 12, 24, 48, 72, 168, 336 or 720 hours, or be turned off (0)." (`400`, `ErrOffsitePeriodicHours`) | An interval not on the list was sent | Use one of those values |
| "The off-site copies to keep must be between 1 and 50." (`400`, `ErrOffsiteKeep`) | Number of copies outside 1-50 | Use a value from 1 to 50 |
| The transfer fails with `reading its off-site settings: …` or `copying its off-site settings: …` | The periodic copy setting could not be read on the old node or saved on the new one (for instance, a new node without the `/daemon/server/:id/offsite` route) | Update the node and try again; the transfer was undone |

## See also

- [Scheduled tasks](./scheduled-tasks.md) — automatic backups and their upload to S3
- [Panel settings](./panel-settings.md) — the **Off-site backups** tab
- [File history](./file-history.md) — undo edits of single config files without a full restore
- [Files and compression](../general/19-files-compression.md)
- [Server notifications](./server-notifications.md) — per-server backup notifications
- [Discord](../general/16-discord.md) — global backup alerts
- [Configuration](../backend/02-configuration.md)
- [API reference](../reference/api/04-server-content.md#backups)
