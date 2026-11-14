# File history

Every time a configuration file is saved from the panel, Aether Panel keeps the text it had before. The previous versions can be compared side by side with the current text and brought back in one click, without restoring a whole backup.

## What it does

- Keeps up to **20 previous versions** of each text configuration file of a server.
- Shows them, newest first, in a **Version history** panel next to the file editor.
- Compares any version with the editor's current text (including unsaved edits).
- **Restores** a version; the text being replaced is kept as a new version first, so a restore can itself be undone.

## How it works

### When a version is recorded

A version is recorded by the node **just before a file is overwritten through the panel's file API** (`PUT /api/servers/:serverId/file/*`). That covers:

- saving a file in the **File Manager** editor;
- saving in the **Properties** editor (it writes `server.properties` through the same API);
- uploading a file over an existing one through the panel.

Files changed any other way are **not** recorded: SFTP, the game itself, plugins, archive extraction, backup restores, modpack installs.

The text that is about to be replaced is kept only when all of the following hold:

| Check | Rule |
|-------|------|
| File type | Extension is one of `.properties`, `.yml`, `.yaml`, `.json`, `.json5`, `.toml`, `.conf`, `.cfg`, `.ini`, `.txt`, `.xml`, `.sk`, `.js`, `.ts`, `.lua`, `.mcmeta`, `.cnf`, `.env`, or the file has no extension and its content is valid UTF-8 |
| Text, not binary | No NUL byte in the first 8 KiB |
| Size | 1 MiB or less |
| Exists | A new file has nothing to keep yet |
| Changed | Identical to the newest stored version (same SHA-256) means no new version |

Recording never makes a save fail: any problem is only logged on the node.

### Storage

Versions live on the node, next to the server folder and **outside** it:

```
<daemon.data.servers>/<server-id>.history/<hash-of-path>/
    index.json      list of versions, newest first
    <version-id>.v  the text of each version
```

Because the folder is outside the server root, the server's users cannot see or modify it through the file manager or SFTP. Each file gets its own folder, named after a hash of its relative path, so the same file always maps to the same history (`/config/a.yml` and `config/a.yml` are the same file). Paths containing `..` are rejected.

### Retention

- **20 versions per file**: when a 21st is recorded, the oldest of that file is deleted.
- **50 MiB per server** in total: when the history of all files together exceeds it, the oldest versions across every file are deleted first until it fits.
- The whole history of a server is deleted when the server is deleted.
- Cloning a server does not copy its file history ([Cloning servers](./cloning.md)).

### Version date

Each version shows when that text was **written to the file** (the file's modification time at the moment it was replaced), not when it was replaced.

### Restoring

1. The chosen version is read.
2. The file's current text is recorded as a new version (same rules as above).
3. Missing parent folders are created and the version's text is written to the file.

The server does not need to be stopped. As with any config edit, most games only read the file again on restart.

## Using it

### In the panel

1. Open **File Manager** and edit a tracked file (e.g. `server.properties`, a `.yml` config).
2. Click **History** in the editor toolbar (the button's tooltip reads **View version history**, with the number of versions when there are some). The button only appears for file types that are tracked.
3. The **Version history** panel lists the saved versions, newest first, with their date and size; the newest is marked **Latest**. The footer reminds that the 20 most recent versions of each config file are kept.
4. Pick a version to compare it with the current text. On wide screens the comparison is side by side (left: that version, right: the editor's current text); on narrow screens it is inline (lines marked `−` are that version, `+` the current text). If they match, the panel says "This version matches the current text."
5. Click **Restore this version** and confirm (**Restore this version?**). If the editor has unsaved changes, the dialog warns they will be lost. After restoring, the toast **Version restored** confirms that the previous text was kept in the history.
6. **Back to editor** returns to the file without changes.

If there are no versions yet, the panel shows **No versions yet**: the first one appears the next time the file is saved.

### Via the API

All routes take the file as a `path` query parameter, relative to the server folder. The panel forwards them to the node.

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| `GET` | `/api/servers/:serverId/file-history?path=<file>` | `server.files.view` | List versions: `[{"id": "…", "savedAt": <unix ms>, "size": <bytes>}]`, newest first; `[]` when there are none |
| `GET` | `/api/servers/:serverId/file-history/:versionId?path=<file>` | `server.files.view` | Text of one version (`text/plain; charset=utf-8`, `Cache-Control: no-store`) |
| `POST` | `/api/servers/:serverId/file-history/:versionId/restore?path=<file>` | `server.files.edit` | Restore that version; `204` on success |

Version IDs have the form `<base36 timestamp>-<6 hex chars>`. Full details: [API reference](../reference/11-api-reference.md). Scopes: [Scopes reference](../general/scopes-reference.md).

## Requirements and configuration

There are no settings for this feature: it is always on, and its limits (20 versions per file, 50 MiB per server, 1 MiB per file) are fixed in the code (`internal/servers/file_history.go`).

The history folder is placed under the servers folder, `daemon.data.servers` (default `servers`, relative to `daemon.data.root`; `/var/lib/SkyPanel/servers` in the Docker installation). See [Configuration](../backend/02-configuration.md).

## Limits

- Only edits made through the panel are recorded; SFTP edits and changes made by the game are not.
- Files over 1 MiB, binary files and file types outside the list are never recorded.
- 20 versions per file and 50 MiB per server; older versions are deleted without notice.
- Restoring replaces the whole file; there is no line-by-line merge.
- Deleting or renaming a file does not move its history; a renamed file starts a new history (the old one stays until the server's cap pushes it out or the server is deleted).

## Troubleshooting

| Message / symptom | Cause | Fix |
|-------------------|-------|-----|
| No **History** button in the editor | The file type is not tracked (e.g. `.jar`, `.log`, images) | Nothing to do; only text config formats are kept |
| **No versions yet** after editing over SFTP | Only saves made through the panel are recorded | Edit the file in the panel's editor |
| A save did not create a new version | The text was identical to the newest version, the file was over 1 MiB or looked binary | Expected behaviour |
| **Couldn't load the history** | The node did not answer | Check the node is online and retry |
| `invalid file path` (`400`) | Empty `path`, or a path containing `..` | Send a path relative to the server folder |
| `file version not found` (`404`) | The version ID is malformed or the version was pruned | Reload the list and pick an existing version |
| **Couldn't restore the version.** | Write error on the node (e.g. disk full) or the current text could not be kept first (`keeping the current text before restoring: …`) | Free space on the node and retry |

## See also

- [Backups](./backups.md) — full copies and restore points
- [Files and compression](../general/19-files-compression.md)
- [API reference](../reference/11-api-reference.md)
