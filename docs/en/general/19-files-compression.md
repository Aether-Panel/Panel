# Server files and archives

Everything the daemon does with a server's files (the file manager, SFTP, install downloads, archives, backups, restores and clones) goes through the `files` package (`files/` at the repository root). Its job is to keep every read and write inside the server's folder, even when the game, or the person running it, leaves symbolic links, named pipes or tricky names behind.

How the kernel side of this works (`openat2`, the `security.forceOpenat` fallback) is described in [Kernel isolation: openat2 and unshare](20-security-unshare.md).

## The file server

`files.NewFileServer(root, uid, gid)` opens the server's folder once and returns a `FileServer`. Every operation is then resolved relative to that open folder, never by its path on disk.

| Method | What it does |
|--------|--------------|
| `Open`, `OpenFile` | Opens a file beneath the root. |
| `Stat` | Opens the file beneath the root and returns its details. |
| `ReadDir` | Lists a folder. |
| `Glob(pattern)` | Matches the **last** element of the pattern inside its folder (`mods/*.jar`, `*`). Folder parts are not expanded. |
| `Mkdir`, `MkdirAll` | Creates folders, each one from its parent opened beneath the root. |
| `Rename` | Renames from the parent folders opened beneath the root (`renameat2`). |
| `Remove`, `RemoveAll` | Deletes from the parent folder opened beneath the root. `RemoveAll` deletes the symbolic links it finds inside a folder as links, without walking into them. |
| `Symlink(target, path)` | Creates a link from its parent folder opened beneath the root. |
| `Prefix()` | The server's folder on disk. |
| `Close()` | Releases the open folder. |

Rules every operation follows:

- **Relative paths only.** The path is cleaned and a leading `/` is removed. A path that leaves the folder (`../`) is refused with `invalid path`.
- **Beneath the root.** With `openat2`, files are opened with `RESOLVE_BENEATH`: a symbolic link that stays inside the folder works, one that points outside fails. In the `openat` fallback, each part of the path is opened with `O_NOFOLLOW`, so any symbolic link on the way fails, even one that points inside.
- **Named pipes do not hang.** Every open adds `O_NONBLOCK`. A named pipe (FIFO) left by the game would otherwise block the request until someone wrote to it. Regular files are not affected.
- **Ownership.** When the file server has a `uid`/`gid` (the server's user), new files, folders and links are given to that user. The change is best effort: a file that opened fine is still usable without it. `-1` leaves ownership as it is.

### What the file manager shows

- A file that is not a regular file (a pipe, a device, a socket) cannot be opened from the file manager: the daemon answers `<name> is not a regular file`.
- Listings put folders first, then files, alphabetically without case, with `..` first outside the root.

## Writing downloads: `CreateIn`, `WriteFile`, `CopyFile`

Install operations (`download`, `mojangdl`, `paperdl`, `forgedl`, `neoforgedl`, `fabricdl`, `spongedl`, `githubdl`, `scraperdl`, `curseforge`) write their files with these helpers:

| Function | What it does |
|----------|--------------|
| `CreateIn(root, dest)` | Creates or empties `dest` inside `root`, creating the missing folders. The file is opened through the file server with `O_NOFOLLOW`. |
| `WriteFile(reader, root, dest)` | Writes a stream to `dest` through `CreateIn`. |
| `CopyFile(src, root, dest)` | Copies a file from the panel's cache to `dest` through `CreateIn`. |

`dest` may be relative or absolute; an absolute path must be inside `root`. Anything that leaves it fails with `path traversal detected`. Installs run as the daemon's user, so without this a game that left `server.jar` pointing at a host file would have the download overwrite that file.

## Archives

Archives are handled by [`mholt/archives`](https://github.com/mholt/archives). The format is recognised from the file name and, when extracting, from its contents: `.zip`, `.tar`, `.tar.gz`/`.tgz`, `.tar.bz2`, `.tar.xz` and the other formats the library supports. The file manager shows **Unarchive** for names ending in `.zip`, `.tar`, `.tar.gz`, `.tgz`, `.gz`, `.bz2`, `.rar`, `.7z` and `.xz`, and **Compress** in the menu of every file and folder (see [Compressing from the file manager](#compressing-from-the-file-manager)).

### Creating an archive: `Compress`

`Compress(fs, target, files)` and `CompressLimited(..., limit)` create an archive inside the server's folder from a list of paths.

- Each path must be relative and inside the folder; otherwise `path traversal detected`.
- A `*` in the **last** part of a path is expanded inside its folder (`logs/*.gz`). A wildcard in a folder part (`a*/file`) is not expanded.
- The archive being written is left out of itself.
- The archive is created with `O_EXCL`: if the name already exists, the answer is `file already exists` and nothing is overwritten. It is opened through the file server, so it cannot be written through a link that points outside.
- The archive format comes from the target's extension.
- `CompressLimited` counts the bytes written against the server's disk limit. If the limit cuts the archive short, the half-written archive is deleted. See [Disk limit](../backend/04-servers.md#disk-limit).

### Backups and restore points: `CompressServer`

`CompressServer(fs, target)` archives the whole server folder into a file **outside** it. Backups and restore points use it; both are `.tar.gz` files in the server's backup folder.

- Every entry goes under a folder named like the server's folder (its identifier).
- The archive is created with mode `0600` and `O_EXCL`.
- The server writes it under a hidden `.partial-<name>` and renames it once complete, so a half-written backup is never listed, downloaded or restored.

### How files are collected (both functions)

`Compress` and `CompressServer` list files through the file server (`archiveWalk`): each folder is read and each file opened beneath the root, never by its path on disk.

| Found | What goes into the archive |
|-------|----------------------------|
| Folder | The folder, then its contents. |
| Regular file | Its contents, read up to the size it had when it was listed. A log that grows meanwhile does not break the archive. |
| Symbolic link | The link itself, with its target text. It is never followed, whatever it points to. |
| Named pipe, device, socket | Nothing: it is left out (reading a pipe would hang the archive). |
| A file deleted while listing | Nothing: it is skipped. |

If a folder is swapped for a link that leads out of the server while the archive is being built, the walk stops with `path traversal detected` instead of reading host files. If a listed file is no longer a regular file when it is read, the archive fails with `<name> is no longer a regular file`.

### Extracting: `Extract`, `ExtractLimited`, `ExtractReader`

| Function | Used by |
|----------|---------|
| `Extract(fs, source, target, filter, skipRoot, forcedType)` | Extraction with an archive inside the server folder. |
| `ExtractLimited(fs, source, target, filter, skipRoot, limit)` | The `extract` operation and the extract endpoint; counts against the disk limit. |
| `ExtractReader(fs, reader, source, target, filter, skipRoot)` | Restoring a backup and installing a CurseForge server pack: the archive is outside the server folder, but its entries are written through the server's file server. |
| `ExtractFromReader(reader, source, target, filter, skipRoot, forcedType)` | Extraction without a file server (panel-side folders). `target` must be relative. |

For each entry:

- Leading `/` and `\` are removed, then the entry must be a relative path inside the target; otherwise the extraction fails with `path traversal detected`.
- `filter` is a wildcard matched against the entry's file name: `*` (or empty) takes everything, `*.jar` only names ending in `.jar`, and text without `*` only that exact name.
- **Folders** are created; **regular files** are created or overwritten with their mode plus `0600`.
- **Symbolic links** are recreated only if their target stays inside the target folder. A link with an absolute target, or one that leads outside, is **left out without failing**: game files often hold absolute links (for example `steamclient.so`), and a restore that stopped there would already have deleted the server's files. Links are created from their parent folder opened beneath the root, so an entry under a link cannot land outside.
- In ZIP archives, a link whose target the library does not report fails with `zip symlink unsupported`.
- Other entry types (devices, pipes, hard links) are ignored.

`skipRoot` drops the first folder of every entry, but only when the whole archive sits under a single top folder (`DetermineIfSingleRoot`). An archive with several top-level entries is extracted as it is.

### Checking an archive before a restore: `CheckArchive`

`CheckArchive(reader, source, skipRoot)` reads every entry and returns an error for the first path that an extraction would refuse, without writing anything. A restore goes in this order:

1. It finds out whether the backup has a single top folder (`DetermineIfSingleRoot`).
2. It checks every entry with `CheckArchive`. An unreadable backup, or one with an entry that leaves the folder, stops here, before anything is deleted (`Backup file cannot be read: ...` in the console).
3. It writes a restore point of the current files. Without it, the restore does not go ahead.
4. It deletes the server's files and extracts the backup with `ExtractReader`.

### Reading a link: `Readlink`

`Readlink(fs, name)` returns the target of the symbolic link `name`, reaching it beneath the root. It fails with `<name> is not a symlink` for anything else. Cloning a server uses it: links whose target stays inside the server are recreated in the copy, the rest are skipped and counted as skipped.

## HTTP endpoints

Both are panel routes that need `server.files.edit` and are forwarded to the server's node.

### `POST /api/servers/{id}/archive/{filename}`

Creates the archive `{filename}` (a path inside the server) from the paths in the body:

```bash
curl -X POST "https://panel.example.com/api/servers/abc123/archive/backups/world.zip" \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '["world", "plugins/*.jar", "server.properties"]'
```

| Answer | When |
|--------|------|
| `204` | The archive was created. |
| `400` | Empty list, a path or name that leaves the folder, a file that does not exist, or a link out of the server on the way. |
| `409` | The server's disk limit is used up. |

An existing `{filename}` is refused with the error code `ErrFileExists` (HTTP 409) and not overwritten.

#### Compressing from the file manager

The menu of every file and folder has **Compress**. The dialog asks for the **Archive name** (it suggests `<name>.zip`) and creates the archive in the current folder through this endpoint, with the chosen item as the only path:

- The format comes from the extension: `.zip`, `.tar` or `.tar.gz`. Any other name gets `.zip` added.
- If a file with that name already exists, it is not overwritten and the dialog shows "A file with that name already exists. Choose another name."
- When it is done, "Created <name>." and the list refreshes.

### `POST /api/servers/{id}/extract/{filename}`

Extracts `{filename}` into the folder given in `destination` (relative to the server root; empty means the root):

```bash
curl -X POST "https://panel.example.com/api/servers/abc123/extract/uploads/pack.zip?destination=uploads&skipRoot" \
  -H "Authorization: Bearer <token>"
```

| Parameter | Meaning |
|-----------|---------|
| `destination` | Target folder inside the server. |
| `skipRoot` | Present (with any value or none): drops the archive's single top folder, if it has one. The file manager always sends it and extracts next to the archive. |

| Answer | When |
|--------|------|
| `204` | Extracted. |
| `400` | `filename` or `destination` leaves the folder. |
| `409` | The server's disk limit is used up, before or during the extraction. |

## Operations

The `archive` and `extract` operations call the same code, with the disk limit applied. Their arguments are described in [Operations engine](../backend/06-operations.md).

```json
{ "type": "archive", "source": ["world"], "destination": "world-${version}.zip" }
```

```json
{ "type": "extract", "source": "pack.zip", "destination": "" }
```

The `extract` operation does not drop a top folder (`skipRoot` is off).

## Merged file systems: `MergedFS`

`files.NewMergedFS(fs1, fs2, ...)` combines several read-only file systems. `Open` and `ReadFile` return the first file system that has the file; `ReadDir` merges the listings, with earlier file systems winning on repeated names. The panel uses it for:

- The web interface: the folder in `panel.web.files` (default `www`) over the files built into the binary.
- Email templates: the folder in `panel.email.templateFolder`, when set, over the built-in templates (see [Email](15-email.md)).

## Errors

| Message | Meaning |
|---------|---------|
| `path traversal detected` | A path, archive entry or link leaves the server's folder (`files.ErrPathTraversal`). |
| `file already exists` | The archive to create already exists (`files.ErrFileExists`; over HTTP it arrives as `ErrFileExists`, "file exists where there should be none"). |
| `invalid path: ...` | The file server was given a path that leaves the folder. |
| `no files to compress` | `Compress` was called with an empty list. |
| `format is not an extractor` / `format is not an archiver` | The format cannot be extracted or created. |
| `<name> is not a regular file` | The file manager was asked to open a pipe, device or socket. |
