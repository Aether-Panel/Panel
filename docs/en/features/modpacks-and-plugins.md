# Modpacks and plugins

Aether Panel can install a complete Minecraft: Java Edition modpack from **Modrinth** or **CurseForge** in one step, and it has a small plugin browser that searches **Modrinth** and drops single plugin or mod files into the server. This page covers both, since they share the same sources and the same safety net (restore points).

---

## What it does

**Modpacks** (tab **Modpacks**, Minecraft Java servers only):

- Searches Modrinth or CurseForge for modpacks. An empty search lists the most popular packs (20 per page, with **Load more**).
- Lists the versions of a pack that the panel can install: Fabric, Forge or NeoForge packs. On CurseForge only files that are, or come with, a server pack are listed.
- Installs the chosen version in the background: stops the server, saves a restore point, removes what the pack replaces, downloads the pack, installs the loader, sets the Java and Minecraft version, and starts the server again if it was running.
- Remembers which pack is installed, shows it at the top of the tab and locks the game version picker while a pack is installed (see [Game version](game-versions.md)).

**Plugins** (tab **Plugins**):

- Works on the folder the server loads its jars from: `mods/` on Fabric, Forge and NeoForge servers, `plugins/` on Paper and on anything else (vanilla, proxies, unknown launchers).
- Lists the `.jar` files in that folder, with their size and a version guessed from the file name. On a modded server this includes the mods a modpack installed.
- Searches Modrinth (20 results) for what the server can load and installs the newest version of the chosen project into that folder.
- Deletes a jar from that folder.

---

## How it works

### Modpack install

The install runs on the node that hosts the server. The request is accepted with `202` and the work continues in the background; the console prints each step and `GET /api/servers/:serverId/modpacks` reports the current step and file counts (the tab polls it every 3 seconds).

1. **Checks.** The source must be `modrinth` or `curseforge`. Modrinth ids or slugs must match `[A-Za-z0-9_-]{1,64}`, CurseForge ids must be numeric. CurseForge requires the node's API key. The install is refused while a backup or restore, a version change or another modpack install is running.
2. **Stop.** If the server is running it is stopped and the panel waits for the process to exit. While files are replaced the server is marked as installing, so nobody can start it.
3. **Restore point.** A restore point (reason `modpack`) is created. If it cannot be created, nothing is changed and the install fails. Restore points are listed in **Backups**; see [Backups](backups.md).
4. **Clean.** At the top of the server folder the panel removes what a pack brings its own version of:
   - folders `mods`, `config`, `defaultconfigs`, `kubejs`, `libraries`, `scripts`, `.fabric`;
   - launchers and start scripts of the previous loader (`server.jar`, `fabric-server-launch.jar`, `run.sh`, `run.bat`, `user_jvm_args.txt`, installer jars, `forge-*.jar`, `neoforge-*.jar`, `minecraft_server*.jar`, and similar).
   The files `server.properties`, `whitelist.json`, `ops.json`, `banned-players.json`, `banned-ips.json` and `eula.txt` are read before and written back afterwards, even if the pack ships its own copy. The world is kept unless **New world** is switched on: then the world folders named by `level-name` (`world`, `world_nether`, `world_the_end` by default) are deleted too. With a fresh world, the pack's `server.properties` is used but the server's own `server-ip`, `server-port`, `query.port`, `enable-query`, `rcon.port`, `enable-rcon`, `rcon.password`, `motd`, `white-list`, `enforce-whitelist`, `online-mode` and `max-players` are kept.
5. **Variables.** The server variables `version` and `javaversion` are set (Java 8 below 1.17, 17 for 1.17 to 1.20.4, 21 from 1.20.5, 25 for the year-numbered releases from 26.x). For CurseForge, `projectId` and `fileId` are also set when the template has them. If the install fails, these variables are put back.
6. **Download and loader.**
   - **Modrinth:** the `.mrpack` is downloaded and checked against its SHA-512 hash, the mod files listed in its index are downloaded (6 in parallel, each one hash-checked), the `overrides` are copied, and the loader named by the pack is installed (Fabric launcher, or the Forge/NeoForge installer). Pack files may only come from `cdn.modrinth.com`, `github.com`, `raw.githubusercontent.com` or `gitlab.com`.
   - **CurseForge:** the panel runs the same `curseforge` operation as the CurseForge template, with the pack's project and file id.
7. **Template files.** The template's own `writefile` install steps run again (for example `eula.txt`), and `server-port` in `server.properties` is set to the server's port.
8. **Record and start.** The panel looks for `fabric-server-launch.jar` or `server.jar`; if neither exists the install fails. It then writes `<servers folder>/<id>.modpack` with the pack, version, loader, game version and the launcher's SHA-256, and starts the server if it was running before.

The pack counts as installed only while that launcher file still has the same checksum. Restoring a backup or reinstalling the server therefore clears the "installed modpack" state and unlocks the version picker. The record is deleted with the server.

### Plugin browser

The daemon works out the server's platform the same way as the [world map](world-map.md) does: from the modpack loader, the `modlauncher` variable or the install steps. It is `paper` (Paper and its forks), `fabric`, `forge`, `neoforge`, or unknown. The platform decides the folder (`mods/` for Fabric, Forge and NeoForge, `plugins/` otherwise) and the search, which calls Modrinth's public search API with:

| Platform | Search filter |
|---|---|
| Paper | Projects of type `plugin` or `mod` with a build for `paper`, `spigot` or `bukkit` |
| Fabric, Forge, NeoForge | Mods with a build for that loader that run on a server (server side required or optional; client-only mods are left out) |
| Unknown | Projects of type `plugin` or `mod`, any loader |

The search box says *Search plugins on Modrinth* on Paper, *Search mods on Modrinth* on Fabric, Forge and NeoForge, and *Search in Modrinth (plugins, mods...)* otherwise.

Installing takes the **newest version built for this server**: the tab reads the platform and the Minecraft version from `GET /api/servers/:id/map` and asks for versions for the same loaders (`paper`, `spigot` and `bukkit` on Paper, `fabric`, `forge` or `neoforge` otherwise) and that game version. When the platform or the version is unknown, that part of the filter is left out; when Modrinth has no version that fits, the install answers `404` with `ErrPluginNoCompatibleVersion` and the tab says so. The primary file (or the first file) of the chosen version must have a plain `.jar` file name, with no folder in it; otherwise the install answers `502` with `ErrPluginBadFileName` and nothing is written. Then the daemon checks the server's disk limit (`409` when it is used up), tries to save a restore point (reason `plugin`; skipped when one was taken in the last 30 minutes, and a busy backup only means the install goes ahead without one), and downloads the file into the folder, creating it if needed. A removal saves the restore point only once the file is found. The download counts against the server's disk limit; if the limit is exceeded the partial file is removed. The console prints `Plugin/Mod <id> installed successfully as <file>` and the server must be restarted to load it.

---

## Using it

### In the panel

**Modpacks**

1. Open the server and go to the **Modpacks** tab. It appears only on servers whose type is `minecraft-java`.
2. Choose the source with the **Modpack source** toggle (Modrinth or CurseForge). If CurseForge has no API key, the toggle shows it as locked and administrators get a link to **Open settings**.
3. Search for a pack, or browse **Most popular**, and pick one.
4. In the dialog choose the **Version**, and switch on **New world** only if the current world should be deleted. The dialog lists **What will happen** and warns when the server has less than 4096 MB of memory.
5. Click **Install modpack**. Progress is shown in the tab and in the console. When it finishes, the first start takes longer than usual while the pack generates its files.

**Plugins**

1. Open the **Plugins** tab (visible to users with `server.files.edit`).
2. **Marketplace**: search Modrinth and click **Install**. **Installed**: see the jars in `mods/` or `plugins/` and click **Uninstall** to delete one.
3. Restart the server to apply the changes.

On Minecraft Java servers the tab can be hidden in **Settings › Plugin Settings › Enable Plugins Tab**. This switch is stored in the browser (`localStorage`), so it only affects that browser; it does not block the API.

### Via the API

All routes are under `/api/servers/:serverId` and are forwarded to the server's node. Full request and response formats: [API reference](../reference/11-api-reference.md).

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| GET | `/modpacks` | `server.files.view` | Installed pack, running install (step, counts, last error), whether CurseForge is available, current loader and game version, memory |
| GET | `/modpacks/search?source=&q=&offset=` | `server.files.view` | Search packs; `source` is `modrinth` or `curseforge`; `q` is cut to 100 characters |
| GET | `/modpacks/versions?source=&project=` | `server.files.view` | Installable versions of a pack, newest first |
| POST | `/modpacks/install` | `server.install` | Start an install. Body: `{"source","projectId","versionId","freshWorld","label"}`. Answers `202` |
| GET | `/plugins` | `server.files.view` | Jars in `mods/` (Fabric, Forge, NeoForge) or `plugins/` (otherwise) |
| GET | `/plugins/search?q=` | `server.files.view` | Search Modrinth for what the server's platform can load |
| POST | `/plugins/:pluginId?loaders=<list>&game_versions=<list>` | `server.files.edit` | Install the newest version of a Modrinth project, limited to the given loaders and game versions (comma-separated, both optional); answers `204`, `404` `ErrPluginNoCompatibleVersion` when no version fits, `502` `ErrPluginBadFileName` when the file name is not a plain `.jar` name, or `409` when the disk limit is used up |
| DELETE | `/plugins?name=<file>.jar` | `server.files.edit` | Delete a jar from `mods/` or `plugins/`, the same folder as the list (`.jar` is added if missing); answers `204` |

Scopes are explained in the [scopes reference](../general/scopes-reference.md).

---

## Requirements and configuration

- **Modrinth** needs no key. The node must reach `api.modrinth.com` and the download hosts listed above.
- **CurseForge** needs an API key, created at console.curseforge.com, on the node that runs the server:

| Key | Default | Environment variable | Notes |
|-----|---------|----------------------|-------|
| `daemon.curseforge.key` | empty (a build can embed one with the `curseforgeKey` build argument of the Dockerfile) | `SKYPANEL_DAEMON_CURSEFORGE_KEY` | Read by the daemon. For the main node it can be set in **Settings › General › Modpacks › CurseForge API key**. Remote nodes read the value in their own configuration. |

- The server must be of type `minecraft-java` for the Modpacks tab, and enough memory for the pack (the panel suggests at least 4096 MB).
- Enough disk: the server's disk limit applies to everything written.

See [Configuration](../backend/02-configuration.md) for how keys and environment variables are read.

---

## Limits

- Loaders: only **Fabric, Forge and NeoForge** packs can be installed. Quilt-only packs are hidden or refused.
- CurseForge: only files with a server pack are offered, and packs whose author disallows third-party distribution are hidden or refused. CurseForge search results are capped at 10,000.
- Size limits for Modrinth packs: `.mrpack` up to 1 GB, each mod file up to 512 MB (or the size the index declares), overrides up to 4 GB in total. Each file download times out after 10 minutes; the whole Modrinth install after 2 hours.
- Only one modpack install per server at a time, and not during a backup, restore or version change.
- While a modpack is installed, the game version cannot be changed from the panel; install another version of the pack instead.
- Plugin browser: the list only shows `.jar` files directly in `mods/` or `plugins/`. On a server whose platform the panel cannot tell (vanilla, a proxy, a custom launcher) it uses `plugins/` and the search is not filtered, so check that what you install fits the server.

---

## Troubleshooting

| Message | Cause and fix |
|---------|---------------|
| `CurseForge needs an API key; an administrator can add it in the panel settings` | No `daemon.curseforge.key` on the node. Add it in **Settings › General** (main node) or in the remote node's configuration. |
| `CurseForge: refused the request (check the API key)` | The key is wrong or revoked. Replace it. |
| `<site>: too many requests, try again in a minute` | Rate limit at Modrinth or CurseForge. Wait and retry. |
| `a modpack install is already running` / `a backup or restore is running; try again when it finishes` / `a version change is already running` | Another job holds the server. Wait for it to finish. |
| `this modpack version uses a loader the panel cannot install (Fabric, Forge and NeoForge are supported)` | Pick another version or another pack. |
| `invalid modpack request: that version has no server pack` | CurseForge file without a server pack. Choose a version marked **Server pack**. |
| `invalid modpack request: the author of this pack does not allow installs outside the CurseForge app` | The pack cannot be installed by the panel. |
| `could not create a restore point, nothing was changed: ...` | The backup could not be written (disk full, backup running). Free space and retry. |
| `the pack was installed but no server.jar was produced, so the server could not start` | The loader installer did not produce a launcher. Check the console output; restore the restore point from **Backups** if needed. |
| `... did not match its sha512 hash; the download is damaged or was changed` | Corrupted or altered download. Retry later. |
| `the pack wants a download from a host that is not allowed: ...` | The `.mrpack` points to a host outside the allow-list. The pack cannot be installed. |
| `the modpack was installed but the server did not start: ...` | Files are in place, but the start failed (often memory or Java). Check the console and the server's memory. |
| `project or versions not found on Modrinth` (plugins) | The project has no versions or the id is wrong. |
| "This project has no version for this server's loader and Minecraft version." (`ErrPluginNoCompatibleVersion`, plugins) | The project publishes no version for the server's platform and Minecraft version. Look for another project or a compatible alternative. |
| "The file Modrinth offers for this project is not a .jar with a plain file name, so it was not installed." (`502`, `ErrPluginBadFileName`, plugins) | The project's file name has a folder in it or is not a `.jar`. Download the file from Modrinth and check it before uploading it by hand. |
| `search query is required` (plugins) | `GET /plugins/search` was called without `q`. |
| Plugin does not load after install | Restart the server. If the panel could not tell the platform, the jar went to `plugins/`: a Fabric, Forge or NeoForge server loads mods from `mods/`. |

Every failed modpack install ends with `(a restore point was saved before the install; it is in Backups)` when files had already been touched: restore it from the **Backups** tab to return to the previous state.

---

## See also

- [Game version](game-versions.md): one-click version changes, locked while a modpack is installed.
- [World map](world-map.md): BlueMap and Dynmap, also downloaded from Modrinth.
- [Backups](backups.md): restore points and how to restore them.
- [Operations](../backend/06-operations.md): the `curseforge`, `fabricdl`, `forgedl` and `neoforgedl` operations used by templates.
- [API reference](../reference/11-api-reference.md) · [Scopes reference](../general/scopes-reference.md) · [Configuration](../backend/02-configuration.md)
