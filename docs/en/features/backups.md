# Backups

Aether Panel keeps full copies of a server's files so they can be downloaded or put back after a mistake, a broken plugin or a lost disk. There are three kinds of copies: **backups** made by hand, **automatic backups** made by a scheduled task, and **restore points** taken by the panel itself just before a risky change. Optionally, every finished backup is also uploaded to an S3-compatible bucket (**off-site backups**).

## What it does

- **Create** a full backup of the server folder as a `.tar.gz` archive.
- **List** every backup with its size, kind (manual, automatic, restore point) and where it is kept (on the node, off-site, or both).
- **Download** a backup to your computer.
- **Restore** a backup: every current file of the server is deleted and the archive is extracted in its place.
- **Delete** a backup (from the node and, when off-site backups are on, from the bucket).
- **Automatic backups** from a scheduled task, with their own retention (keep the newest *N*).
- **Restore points** before plugin installs/removals, game version changes, modpack installs, world map installs and backup restores, so that step can be undone.
- **Off-site copies** in AWS S3, Backblaze B2, Cloudflare R2, Wasabi, MinIO or any S3-compatible service.

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

| Kind | File name | Name shown in the list |
|------|-----------|------------------------|
| Manual backup | `<uuid>.tar.gz` | The name the user typed |
| Automatic backup (scheduled task) | `auto-<uuid>.tar.gz` | "Automatic backup" badge |
| Restore point | `restore-<reason>-<uuid>.tar.gz` (`plugin`, `version`, `modpack`, `restore`) | "Restore point" badge and the reason |

Retention of automatic backups only ever removes `auto-` files, and restore point pruning only removes `restore-` files: a backup made by hand is never deleted automatically.

### Creating

1. The panel checks the per-server cap (`panel.backups.maxPerServer`) unless the caller is an administrator (`admin` scope).
2. The node refuses if another backup or restore of that server is already running (`backup in progress`): only one runs at a time per server.
3. The node answers straight away with the file name and writes the archive in the background. The panel stores the record at that moment.
4. The archive is written under a hidden name (`.partial-<name>`) and renamed when complete, so a half-written backup is never listed, downloaded or restored. Files are listed and opened beneath the server root, so a symlink swapped in while the backup runs cannot pull host files into the archive. Every entry is stored under a folder named after the server ID.
5. The console shows `Backing up server` and then `Backup complete` or `Backup failed`. The global Discord backup alert ([Discord](../general/16-discord.md)) and the per-server backup notification are sent. When the archive of a manual backup could not be written, the node marks it as failed (a hidden `.failed-<name>` file) and the panel deletes its record the next time the list is opened, so no entry is left without a file behind it.
6. If off-site backups are on, the finished archive is uploaded afterwards (console: `Uploading backup to off-site storage`, then `Backup uploaded to off-site storage` or `Off-site upload failed: …`). A failed upload does not fail the backup, which exists locally.

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

A scheduled task can include a **Backup** step with a `keep` value (1 to 50, default 5). The step waits until the archive is finished, so the task's next step sees it, and then deletes the oldest `auto-` backups beyond `keep`. The **Daily backup** preset backs up every day at 04:00 and keeps the last 7. Creating a task with a backup step requires the `server.backup.create` scope. Tasks are described in [Scheduled tasks](./scheduled-tasks.md).

Each time the backup list is opened, the panel compares its records with the files on the node: automatic backups and restore points made by the node are added, those removed by retention and the manual backups the node marks as failed are dropped (and the node is told to remove the mark), and sizes and locations are updated.

### When a server is deleted or transferred

- **Deleting a server** also deletes its backups on the node (the folder `<daemon.data.backups.folder>/<server-id>/`) and its records in the panel. Off-site copies in the bucket are **not** deleted.
- **Transferring a server to another node** ([Server transfers](./server-transfers.md)) copies the backups kept on the old node (location `local` or `both`, restore points included) to the new node through the panel, with their names and dates, and then deletes them from the old node with the server. The records follow the server. Off-site copies stay where they are: the bucket is the same for every node. If the transfer fails, the copies it made on the new node are removed and the old node keeps its own.

### Off-site copies

When off-site backups are on and complete (endpoint, bucket, access key and secret key filled in):

- Every finished backup (manual or automatic) is uploaded to `<prefix>/<server-id>/<file>` in the bucket. The local copy stays: the bucket is a second copy.
- The list merges both places: `local`, `offsite` (only in the bucket) or `both`.
- **Deleting** a backup removes it from the node and from the bucket.
- **Downloading** a backup that is only off-site streams it from the bucket.
- **Restoring** a backup that is only off-site downloads it to the node first.
- If the bucket cannot be listed (wrong key, network), the local list is shown as is and the error is logged.

Timeouts: upload or download 2 hours per transfer, listing 15 seconds, deletion 1 minute.

## Using it

### In the panel

**Server › Backups** tab:

- **Create Backup**: type a **Name** and confirm. The backup appears in the list immediately; its size is filled in once the archive is written (reload the list).
- Each row shows the name, date, size and badges: **Automatic**, **Restore point** (with the reason, e.g. "Before a plugin change"), **Off-site** (also in the bucket) or **Off-site only** (local copy gone).
- **Restore** asks for confirmation ("All current files will be overwritten"). Stop the server first.
- **Download** opens the archive download in a new tab.
- **Delete** asks for confirmation.
- The header shows **Space used by backups**, the largest backup and how many are also off-site.

Automatic backups are configured in the **Tasks** tab (step **Backup**, preset **Daily backup**).

Off-site storage is configured by an administrator in **Settings › Off-site backups**:

1. Optionally click a provider (Backblaze B2, Cloudflare R2, AWS S3, Wasabi) to fill in the endpoint and region.
2. Fill in **Endpoint**, **Bucket name**, **Region** (needed by some providers; `auto` on R2) and **Folder in the bucket** (default `aether-backups`).
3. Fill in **Access key** and **Secret key** for a key allowed to read, write and delete in that bucket.
4. Turn on **Unencrypted connection (http)** only for a MinIO on your own network.
5. Click **Test connection**: the panel writes and deletes a small object in the bucket using the values on screen, saved or not.
6. Turn on **Upload backups to the bucket** and save.

### Via the API

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| `GET` | `/api/servers/:serverId/backup` | `server.backup.view` | List backups (syncs automatic backups and restore points first) |
| `GET` | `/api/servers/:serverId/backup/:backupID` | `server.backup.view` | Get one backup record |
| `POST` | `/api/servers/:serverId/backup/create?name=<name>` | `server.backup.create` | Start a backup; `204` when started |
| `POST` | `/api/servers/:serverId/backup/restore/:backupID` | `server.backup.restore` | Start a restore; `204` when started |
| `GET` | `/api/servers/:serverId/backup/download/:backupID` | `server.backup.view` | Download the archive |
| `DELETE` | `/api/servers/:serverId/backup/:backupID` | `server.backup.delete` | Delete the backup (node and bucket) |
| `POST` | `/api/settings/test/s3` | `settings.edit` | Test S3 settings; body values override the saved ones (a `********` secret key uses the saved one) |

A backup record looks like `{"id": 12, "name": "before update", "fileName": "…tar.gz", "fileSize": 104857600, "location": "both", "createdAt": "…"}`. Errors from the node (server running, backup in progress…) are passed through with the node's status code. Full details: [API reference — Backups](../reference/api/04-server-content.md#backups). Scopes: [Scopes reference](../general/scopes-reference.md).

## Requirements and configuration

| Key | Default | Environment variable | Description |
|-----|---------|----------------------|-------------|
| `panel.backups.maxPerServer` | `10` | `SKYPANEL_PANEL_BACKUPS_MAXPERSERVER` | Backups a server's users may keep; administrators are not capped; `0` = no limit. Not editable from the Settings page |
| `daemon.data.backups.folder` | `backups` | `SKYPANEL_DAEMON_DATA_BACKUPS_FOLDER` | Backup folder; relative paths are resolved under `daemon.data.root` |
| `daemon.backups.s3.enabled` | `false` | `SKYPANEL_DAEMON_BACKUPS_S3_ENABLED` | Upload finished backups to the bucket |
| `daemon.backups.s3.endpoint` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_ENDPOINT` | `host`, `host:port` or a full URL (its scheme decides http/https) |
| `daemon.backups.s3.region` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_REGION` | Region, when the provider needs it |
| `daemon.backups.s3.bucket` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_BUCKET` | Bucket name (must already exist) |
| `daemon.backups.s3.accessKey` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_ACCESSKEY` | Access key |
| `daemon.backups.s3.secretKey` | `""` | `SKYPANEL_DAEMON_BACKUPS_S3_SECRETKEY` | Secret key |
| `daemon.backups.s3.prefix` | `aether-backups` | `SKYPANEL_DAEMON_BACKUPS_S3_PREFIX` | Folder inside the bucket |
| `daemon.backups.s3.insecure` | `false` | `SKYPANEL_DAEMON_BACKUPS_S3_INSECURE` | Plain http (MinIO on a local network) |

Off-site backups are only active when `enabled` is true **and** endpoint, bucket, access key and secret key are all set. See [Configuration](../backend/02-configuration.md) for the full list and how environment variables map to keys.

**Remote nodes:** uploads, listings and deletions run in the daemon of the node that hosts the server, which reads the `daemon.backups.s3.*` keys from its own configuration. The **Settings › Off-site backups** page writes the panel's `config.json`, so it applies to servers on the panel's own (local) node. On a remote node, set the same keys in that node's `config.json` or environment. The **Test connection** button tests from the panel host.

## Limits

- One backup or restore at a time per server.
- The cap counts every backup record of the server: manual backups, automatic backups and restore points.
- Backup names must be printable ASCII (the record is validated with `printascii`); accented letters or emoji are rejected.
- Restore always replaces **all** files of the server; there is no partial restore. To recover a single file, download the backup and upload that file with the file manager or SFTP.
- Restore points: 3 kept, local only.
- Automatic backup retention: `keep` from 1 to 50.
- Cloning a server does not copy its backups ([Cloning servers](./cloning.md)).
- When a server is deleted, its copies in the off-site bucket stay there; remove them by hand if they are no longer needed.
- A node transfer copies every backup through the panel, so it takes longer and needs room on the new node for them. Both nodes must run a current version: the new node needs the `/daemon/server/:id/backup/file` route to receive them, and both need the scheduler pause routes the transfer uses (see [Server transfers](./server-transfers.md#requirements-and-configuration)).
- The S3 settings saved in **Settings › Off-site backups** only apply to the panel's own node; each remote node needs its own (see [Remote nodes](#requirements-and-configuration)).

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
| Console: `Off-site upload failed: …` | Network error, wrong credentials, or the node has no S3 settings (remote node) | The local backup is fine; fix the settings on the node and make a new backup |

## See also

- [Scheduled tasks](./scheduled-tasks.md) — automatic backups
- [File history](./file-history.md) — undo edits of single config files without a full restore
- [Files and compression](../general/19-files-compression.md)
- [Server notifications](./server-notifications.md) — per-server backup notifications
- [Discord](../general/16-discord.md) — global backup alerts
- [Configuration](../backend/02-configuration.md)
- [API reference](../reference/api/04-server-content.md#backups)
