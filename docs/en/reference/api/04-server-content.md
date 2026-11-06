# Server content

Routes under `/api/servers/:serverId` that act on what a server holds: files, archives, file history, plugins, modpacks, world map, backups, scheduled tasks and MySQL databases. Back to the [API reference](../11-api-reference.md).

Routes marked "node" are forwarded to the server's node (`/daemon/server/:serverId/...`): the panel checks the scope and passes the node's status and body back unchanged. Paths inside the server (`*filename`, `path`, `destination`) are relative to the server folder; anything that would leave it (`..`, absolute paths, symbolic links pointing outside) is refused.

## Routes

| Method | Path | Scope | What it does |
|--------|------|-------|--------------|
| GET | `/file/*filename` | `server.files.view` | Downloads a file or lists a folder (node) |
| PUT | `/file/*filename` | `server.files.edit` | Uploads or overwrites a file; `?folder` creates a folder (node) |
| POST | `/file/*filename?destination=` | `server.files.edit` | Renames or moves (node) |
| DELETE | `/file/*filename` | `server.files.edit` | Deletes a file or a whole folder (node) |
| POST | `/archive/*filename` | `server.files.edit` | Compresses files into an archive (node) |
| HEAD | `/archive/*filename` | `server.files.edit` | Registered, but the node has no matching route: answers `404` |
| POST | `/extract/*filename` | `server.files.edit` | Extracts an archive (node) |
| GET | `/file-history?path=` | `server.files.view` | Previous versions of a file (node) |
| GET | `/file-history/:versionId?path=` | `server.files.view` | Text of one version (node) |
| POST | `/file-history/:versionId/restore?path=` | `server.files.edit` | Restores a version (node) |
| GET | `/plugins` | `server.files.view` | Installed `.jar` plugins or mods (node) |
| GET | `/plugins/search?q=` | `server.files.view` | Searches Modrinth for the server's platform (node) |
| POST | `/plugins/:pluginId` | `server.files.edit` | Installs a Modrinth project (node) |
| DELETE | `/plugins?name=` | `server.files.edit` | Removes a plugin or mod (node) |
| GET | `/modpacks` | `server.files.view` | Installed modpack and install progress (node) |
| GET | `/modpacks/search` | `server.files.view` | Searches Modrinth or CurseForge (node) |
| GET | `/modpacks/versions` | `server.files.view` | Versions of a modpack (node) |
| POST | `/modpacks/install` | `server.install` | Installs a modpack (node) |
| GET | `/map` | `server.view` | World map status and viewer link |
| POST | `/map/install` | `server.files.edit` | Installs BlueMap or Dynmap (node) |
| DELETE | `/map` | `server.files.edit` | Removes the map plugin (node) |
| GET | `/backup` | `server.backup.view` | Lists backups |
| GET | `/backup/:backupID` | `server.backup.view` | One backup |
| POST | `/backup/create?name=` | `server.backup.create` | Creates a backup |
| POST | `/backup/restore/:backupID` | `server.backup.restore` | Restores a backup |
| GET | `/backup/download/:backupID` | `server.backup.view` | Downloads the archive |
| DELETE | `/backup/:backupID` | `server.backup.delete` | Deletes a backup |
| GET | `/offsite` | `server.backup.view` | The server's periodic copy to S3 (node) |
| PUT | `/offsite` | `server.backup.create` | Changes the server's periodic copy to S3 (node) |
| GET | `/tasks` | `server.tasks.view` | Lists scheduled tasks (node) |
| GET | `/tasks/:taskId` | `server.tasks.view` | One task (node) |
| PUT | `/tasks/:taskId` | any of `server.tasks.create`, `server.tasks.edit` | Creates or replaces a task |
| DELETE | `/tasks/:taskId` | `server.tasks.delete` | Deletes a task (node) |
| POST | `/tasks/:taskId/run` | `server.tasks.run` | Runs a task now (node) |
| GET | `/databases` | `server.view` | The server's databases; passwords only with `server.data.edit` |
| POST | `/databases` | `server.data.edit` | Creates a database |
| DELETE | `/databases/:id` | `server.data.edit` | Drops a database |
| GET | `/databasehosts` | `server.data.edit` | Hosts the server can create databases on |

All paths in the table start with `/api/servers/:serverId`. The public world map viewer, `/api/map-view/...`, is described under [World map](#world-map).

## Files

### `GET /api/servers/:serverId/file/*filename`

For a folder, a JSON array: in subfolders `..` comes first, then folders, then files, alphabetically.

```json
[
  { "name": "..", "isFile": false },
  { "name": "world", "isFile": false },
  { "name": "server.properties", "isFile": true, "size": 1452, "modifyTime": 1793576400, "extension": ".properties" }
]
```

`modifyTime` is in Unix seconds. For a file, the raw content as `application/octet-stream` with `Content-Disposition: attachment`. `404` when it does not exist; `400 ErrIllegalFileAccess` for a path outside the server.

Do not add a query string to file reads: everything after `?` (except `folder`) is taken as part of the file name.

### `PUT /api/servers/:serverId/file/*filename`

Writes the request body to the file, or the multipart field `file` when the request is `multipart/form-data`. Missing parent folders are created. The previous text of the file is kept in the [file history](#file-history). `204`.

- `?folder`: creates the folder (and its parents) instead; `204`.
- `409 ErrDiskLimitExceeded`: the server's disk is full, or the upload filled it (the partial file is deleted).

### `POST /api/servers/:serverId/file/*filename?destination=new/path`

Renames or moves a file or folder; the extension can change. `204`. `400` when `destination` is missing or outside the server, `400 ErrFileExists` when something already exists there.

### `DELETE /api/servers/:serverId/file/*filename`

Deletes a file, or a folder with everything inside. `204`. Freed space counts at once, so a server stopped for a full disk can start again. There is no route that deletes several paths at once: the file manager's **Delete selected** calls this route once per item, one after another, and stops at the first failure.

### `POST /api/servers/:serverId/archive/*filename`

Body: the paths to compress, e.g. `["world", "server.properties"]`. `*filename` is the archive to create; its extension picks the format (see [Files and compression](../../general/19-files-compression.md)). `204`. The file manager sends one path from a row's **Compress**, and every selected path from **Compress selected**.

| Error | When |
|-------|------|
| `400` | Empty list, a path outside the server, a missing file, or a link pointing out of the server |
| `409 ErrFileExists` | The archive already exists |
| `409 ErrDiskLimitExceeded` | The disk limit was reached |

### `POST /api/servers/:serverId/extract/*filename`

Extracts the archive. `?destination=` is the folder to extract into (empty: the server folder); `?skipRoot` drops the archive's top folder. `204`; `400` for paths outside the server; `409 ErrDiskLimitExceeded`.

### File history

When a file is overwritten through the panel, its previous text is kept. Details and limits: [File history](../../features/file-history.md).

- `GET /file-history?path=config.yml` lists the versions, newest first: `[{"id": "lzx4k2-a1b2c3", "savedAt": 1793576400123, "size": 812}]` (`savedAt` in Unix milliseconds).
- `GET /file-history/:versionId?path=config.yml` answers the text of that version (`text/plain`).
- `POST /file-history/:versionId/restore?path=config.yml` writes it back; the current text is kept as a version first, so the restore can be undone. `204`.

`400` for an invalid path, `404` for an unknown version.

## Plugins

Plugins and mods come from Modrinth and only exist on Minecraft: Java Edition servers (`type` `minecraft-java`): on any other, the four routes answer `400 ErrPluginsNotMinecraft` and the panel does not show the Plugins tab. Details: [Modpacks and plugins](../../features/modpacks-and-plugins.md).

The node works out the server's platform the same way as for the [world map](#world-map) (the modpack's loader, the `modlauncher` variable, then the install steps): `paper`, `fabric`, `forge`, `neoforge`, or unknown (vanilla, proxies, other games, unknown launchers). The folder follows it: **`mods/`** on Fabric, Forge and NeoForge, **`plugins/`** otherwise. Listing, installing and removing all use that folder.

### `GET /api/servers/:serverId/plugins`

The `.jar` files in the server's folder (`mods/` or `plugins/`, see above); an empty list when the folder does not exist.

```json
[ { "name": "EssentialsX-2.20.1.jar", "version": "2.20.1", "size": 1234567 } ]
```

`version` is guessed from the part of the name after the last `-`, without `.jar`; it is empty when the name has no `-`.

### `GET /api/servers/:serverId/plugins/search?q=essentials`

Up to 20 Modrinth projects that fit the server's platform:

| Platform | What is searched |
|----------|------------------|
| Paper | Plugins and mods with a Paper, Spigot or Bukkit build |
| Fabric, Forge, NeoForge | Mods for that loader that run on a server (`server_side` required or optional; client-only mods are left out) |
| Unknown | Plugins and mods of any loader |

```json
[ { "id": "hXiIvTyT", "name": "EssentialsX", "tag": "", "description": "...", "author": "...", "iconUrl": "https://...", "downloads": 1000000, "version": "2.20.1" } ]
```

`400` without `q`; `500` when Modrinth does not answer.

### `POST /api/servers/:serverId/plugins/:pluginId`

`:pluginId` is the Modrinth project id. Optional query: `loaders` (comma-separated Modrinth loaders, such as `paper,spigot,bukkit`) and `game_versions` (comma-separated Minecraft versions, such as `1.21.4`); only lowercase letters, digits, `.`, `-` and `_` are kept, up to 32 characters per value. Picks the primary file of the newest version matching those filters (any version without them), checks that its name is a plain `.jar` file name (no folder, no `..`) and that the disk has room, then saves a restore point (a backup, at most one every 30 minutes for plugin changes), downloads the file into `mods/` or `plugins/` (see above) and reports it on the console. A refused install leaves no restore point behind. The web client sends the loaders that run on the server's platform and its game version, read from `GET /map`. `204`. `404 ErrPluginNoCompatibleVersion` when filters were sent and no version matches them; `404` when the project, its versions or its files are not found; `502 ErrPluginBadFileName` when the file Modrinth names is not a plain `.jar` name; `409 ErrDiskLimitExceeded`.

### `DELETE /api/servers/:serverId/plugins?name=EssentialsX.jar`

Removes `<name>` from `mods/` or `plugins/` (see above; `.jar` is added when missing). The restore point is saved once the file is found, so a wrong name leaves none behind. `204`; `400` without `name`; `404` when the file does not exist.

## Modpacks

| Route | Notes |
|-------|-------|
| `GET /modpacks` | `{"installed": {...}, "installing": false, "step": "", "current": 0, "total": 0, "lastError": "", "curseforge": true, "platform": {"loader": "fabric", "gameVersion": "1.21.1"}, "memory": 4096}`; `curseforge` says whether the node has a CurseForge API key |
| `GET /modpacks/search?source=modrinth&q=&offset=0` | `source` is `modrinth` or `curseforge`; an empty `q` lists the most popular packs |
| `GET /modpacks/versions?source=modrinth&project=<id>` | The pack's installable versions, newest first |
| `POST /modpacks/install` | Body `{"source": "modrinth", "projectId": "...", "versionId": "...", "freshWorld": false, "label": "Pack name"}`; answers `202` and runs in the background |

The install stops the server, saves a restore point, replaces mods and configuration, installs the loader, sets the Java and game version and starts the server again if it was running. Errors: `400` for a bad request, a missing CurseForge key or an unsupported pack; `409` while another install or version change runs or the server is busy; `422` when Modrinth or CurseForge refuse.

## World map

BlueMap or Dynmap rendered by the game server, shown in the panel through a sandboxed iframe. Details: [World map](../../features/world-map.md).

### `GET /api/servers/:serverId/map`

```json
{
  "supported": true, "reason": "", "platform": "paper", "gameVersion": "1.21.1",
  "installed": "bluemap", "version": "5.4", "installing": false, "running": true, "ready": true,
  "viewUrl": "/api/map-view/3F9A1C2B7D4E/<token>/"
}
```

`reason` explains an unsupported server (`vanilla` or `unknown`); `ready` says whether the map's web server answers. `viewUrl` is only set when a map is installed.

### `POST /api/servers/:serverId/map/install`

`{"kind": "bluemap", "acceptDownload": true}` (`kind` is `bluemap` or `dynmap`). Downloads the plugin or mod from Modrinth after a restore point and restarts the server if it runs. `202`. `400` for an unknown kind, a download not accepted or an unsupported server; `409` otherwise (already installing, busy...).

### `DELETE /api/servers/:serverId/map`

Removes the map's jar; the rendered map stays on disk. `204`; `404` when no map is installed; `409` while installing.

### `GET /api/map-view/:serverId/:token/*path`

Public: the token in the path is the authorization. It is an HMAC signed by the panel, valid for 12 to 13 hours and the same for a whole hour, and stops working when the panel restarts. The panel proxies the map's own web server and serves every answer with a sandbox Content-Security-Policy, so the map's scripts cannot reach the panel's origin, cookies or storage.

## Backups

The panel keeps a record per backup; the node keeps the archive, which goes with the server when it is [transferred to another node](./03-servers.md#post-apiserversserveridtransfer). Details (retention, off-site copies, restore points): [Backups](../../features/backups.md).

### `GET /api/servers/:serverId/backup`

```json
[ { "id": 12, "name": "before update", "fileName": "....tar.gz", "fileSize": 104857600, "location": "both", "createdAt": "2026-10-31T09:00:00Z" } ]
```

`location` is `local`, `offsite` or `both`. Before answering, the panel brings the list in line with the node: backups made by scheduled tasks and periodic copies (`offsite-…`, named `Periodic backup (S3)`) are added, ones removed by retention disappear, a backup made by hand whose archive could not be written is removed (the node reports it as `failed`) and sizes are filled in once the archive is written (best effort; when the node cannot be reached the stored list is shown).

`GET /backup/:backupID` answers one record.

### `POST /api/servers/:serverId/backup/create?name=before+update`

Starts a backup; the archive is written in the background and the console reports when it is done. The backup stays on the node only: manual backups are not uploaded to the bucket. `204`. The record is made right away; if the archive then fails, the next listing drops it.

| Error | When |
|-------|------|
| `400 ErrFieldRequired` | No `name` |
| `409 ErrBackupLimit` | The server already has `panel.backups.maxPerServer` backups on the node (default 10; `0` disables the cap). Copies kept only in the bucket do not count (periodic copies once uploaded, and task copies the retention already deleted from the node); a periodic copy still on the node does. Administrators are not capped |
| The node's error | For example another backup or restore running |

Backups store the server folder: files are opened beneath it, symbolic links are stored as links and never followed, and pipes are left out.

### `POST /api/servers/:serverId/backup/restore/:backupID`

Deletes the server's files and extracts the backup in their place. The server must be stopped (`400 ErrBackupServerRunning` from the node). `204`; `404` for an unknown backup.

### `GET /api/servers/:serverId/backup/download/:backupID`

Streams the archive (`application/octet-stream`, `Content-Disposition: attachment`). `404` for an unknown backup.

### `DELETE /api/servers/:serverId/backup/:backupID`

Deletes the archive on the node (and in the bucket, when it is there) and the record. `204`; `404` for an unknown backup; the node's error when it refuses.

### `GET /api/servers/:serverId/offsite`

The server's [periodic copy](../../features/backups.md#periodic-copy), as its node sees it:

```json
{ "periodicHours": null, "periodicKeep": 6, "effectiveHours": 24, "effectiveKeep": 6, "configured": true, "lastPeriodic": "2026-11-05T04:12:00Z" }
```

`periodicHours` and `periodicKeep` are the server's own setting (`null` = the node's default); `effectiveHours` and `effectiveKeep`, what it ends up using (`0` hours = no periodic copy); `configured`, whether the node has S3 set up; `lastPeriodic`, the date of the last periodic copy the node knows of (`null` when none). A node from an earlier version answers `404`.

### `PUT /api/servers/:serverId/offsite`

```json
{ "periodicHours": 48, "periodicKeep": null }
```

Saves the server's own setting; `null` goes back to the default and `periodicHours: 0` turns that server's periodic copy off. `204`.

| Error | When |
|-------|------|
| `400 ErrOffsitePeriodicHours` | `periodicHours` is not `0`, `12`, `24`, `48`, `72`, `168`, `336` or `720` |
| `400 ErrOffsiteKeep` | `periodicKeep` is not between 1 and 50 |

## Tasks

Scheduled tasks run on the node, even with nobody signed in. Details: [Scheduled tasks](../../features/scheduled-tasks.md).

A task:

```json
{
  "name": "Nightly restart",
  "description": "Warn, wait, restart",
  "cronSchedule": "0 4 * * *",
  "operations": [
    { "type": "stdin", "command": "say Restarting in 1 minute" },
    { "type": "sleep", "duration": "1m" },
    { "type": "restart" }
  ]
}
```

`GET /tasks` answers `{"tasks": {"<taskId>": { ...task... }}}`; `GET /tasks/:taskId` one task (`404` when unknown).

### `PUT /api/servers/:serverId/tasks/:taskId`

Creates the task or replaces the one with that id. `204`. Rules for everyone (checked by the node, `400` otherwise): the id is 1 to 64 letters, digits, `-` or `_`; `name` is required; `cronSchedule` must parse (an empty one means the task only runs by hand); at most 50 tasks per server.

Callers without `admin` get stricter rules (`400` with the reason; `413` for a body over 64 KB):

| Rule | Limit |
|------|-------|
| Steps | 1 to 10, without `if` conditions |
| `stdin` | `command`, one line of at most 512 characters; needs `server.console.send` |
| `sleep` | `duration` up to one hour |
| `start`, `stop`, `restart` | Need `server.start`, `server.stop`, or both |
| `backup` | Optional `keep` (1 to 50, default 5), optional `offsite` (`never`, `always`, `weekend`, `weekly`, `biweekly` or `monthly`; `always` when missing) and optional `offsiteKeep` (1 to 50, default 4); needs `server.backup.create` |
| Schedule | Five fields (no seconds) or `@yearly`, `@annually`, `@monthly`, `@weekly`, `@daily`, `@midnight`, `@hourly`; an optional `CRON_TZ=` or `TZ=` prefix |
| `name`, `description` | At most 64 and 256 characters |

Administrators may use any operation (commands, downloads, file writes...).

`DELETE /tasks/:taskId` answers `204` or `404`; `POST /tasks/:taskId/run` runs it now, `204` or `404`.

## Databases

MySQL databases on the hosts an administrator registered (see [Database hosts](./06-panel-admin.md#database-hosts)).

### `GET /api/servers/:serverId/databases`

```json
[
  {
    "id": 4, "server_id": "3F9A1C2B7D4E", "database_host_id": 1, "database_name": "s_3F9A1C2B7D4E_main",
    "username": "db_k3x9q2m7ab", "password": "<generated>", "remote_connection": "db1.example.com:3306", "max_connections": 0,
    "host": "db1.example.com", "port": 3306, "host_name": "Main MySQL",
    "created_at": "2026-10-31T09:00:00Z", "updated_at": "2026-10-31T09:00:00Z"
  }
]
```

The list only needs `server.view`, but `password` is only included when the caller holds `server.data.edit` on the server (the scope that creates and drops databases); otherwise the field is left out and the web client shows that the password is only visible to users who can manage databases. For an OAuth2 client session, the client's bounds count too.

### `GET /api/servers/:serverId/databasehosts`

The hosts this server can use: those serving its node (or every node) with room left under their `max_databases`. Only `[{"id": 1, "name": "Main MySQL"}]`, never addresses or credentials.

### `POST /api/servers/:serverId/databases`

`{"database_host_id": 1, "database_name": "main"}`. The database is named `s_<serverId>_<name>`; the user (`db_` plus 10 random characters) and its 16-character password are generated. Answers the database object. If a step after `CREATE DATABASE` fails, the database and user are dropped again so the name can be retried.

| Error | When |
|-------|------|
| `400 ErrInvalidDatabaseName` | The name uses something other than letters, digits, `_` and `-`, or `s_<serverId>_<name>` is longer than 64 characters |
| `400 ErrDatabaseNameTaken` | The name exists on that host |
| `400 ErrDatabaseHostFull` | The host reached `max_databases` |
| `400 ErrDatabaseHostOtherNode` | The host serves another node |

### `DELETE /api/servers/:serverId/databases/:id`

Drops the database and its user. `204`; `403` when the database belongs to another server. Deleting a server drops all its databases.
