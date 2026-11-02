# World map

The **Map** tab installs a web map of the Minecraft world (BlueMap in 3D or Dynmap in 2D) in one click and shows it inside the panel, with players live. The map runs as a plugin or mod on the game server itself; Aether Panel downloads it, configures its web server and serves it to the browser through a sandboxed, signed proxy.

---

## What it does

- Detects whether the server can load a map: Paper (and Spigot, Bukkit, Purpur, Pufferfish, Folia), Fabric (and Quilt), Forge or NeoForge.
- Offers two maps:
  - **BlueMap**: 3D view, live players. Downloads Minecraft's resources from Mojang, so its install needs explicit consent to Mojang's EULA. The first render takes a while and uses CPU.
  - **Dynmap**: classic 2D top-down map, players and chat, lighter.
- Downloads the newest build that matches the server's loader and Minecraft version from Modrinth, saves a restore point first, and restarts the server if it is running.
- Sets the map's web port and (for BlueMap) the resource download, then shows the map in an iframe, with **Open in a new tab**, **Fullscreen** and **Reload the map**.
- Removes the map (**Remove map**) while keeping what was already rendered.

---

## How it works

### Detecting the platform

The node decides which build to install by looking, in order, at the installed modpack's loader (see [Modpacks and plugins](modpacks-and-plugins.md)), the template's `modlauncher` variable, and the install steps that download the game (`paperdl`, `fabricdl`, `forgedl`, `neoforgedl`, `mojangdl`). A vanilla server (`mojangdl` only) or a proxy (Velocity, Waterfall) cannot load a map. The Minecraft version comes from the modpack, the server's version variable, the version the running server reports, or the newest release when the variable says `latest`.

### Install

`POST /api/servers/:serverId/map/install` answers `202` and the node continues in the background, printing each step in the console:

1. A restore point (reason `plugin`) is attempted; a busy backup only means the install goes ahead without one.
2. The Modrinth project `bluemap` or `dynmap` is queried for the platform and game version. For Paper the loaders `paper`, then `spigot`/`bukkit`, then `folia` are tried. The newest release is taken (or the newest beta if there is no release; alphas are never used).
3. Any previous BlueMap or Dynmap jar in the target folder is deleted, so only one map is loaded.
4. The primary jar is downloaded from `cdn.modrinth.com` only, up to 128 MB, and checked against Modrinth's SHA-512. It goes into `plugins/` on Paper and into `mods/` on Fabric, Forge and NeoForge.
5. The panel records what it installed in `<servers folder>/<id>.worldmap` (kind, version, port, consent, jar path, platform), outside the server's own folder.
6. If the server is running it is stopped and started again so the map loads; otherwise the map starts with the next start.

### Configuration on each start

The maps write their configuration files on their first start, so the panel patches them afterwards. While a map is installed, each run of the server is checked once: the node waits up to 3 minutes for the files, sets the values below, and runs `bluemap reload` (BlueMap) or `dynmap reload` (Dynmap, when the port changed) in the console.

| Map | File (Paper) | File (Fabric/Forge/NeoForge) | Key set |
|-----|--------------|------------------------------|---------|
| BlueMap | `plugins/BlueMap/core.conf` | `config/bluemap/core.conf` | `accept-download: true` (only with consent) |
| BlueMap | `plugins/BlueMap/webserver.conf` | `config/bluemap/webserver.conf` | `port` |
| Dynmap | `plugins/dynmap/configuration.txt` | `dynmap/configuration.txt` | `webserver-port` |

The web port is the server's **first extra port** (`port2`, `port3`... that differs from the game port), so the map is also reachable from outside on that port. Without an extra port, the map's default is used: **8100** for BlueMap, **8123** for Dynmap; in Docker that port is only reachable inside the container network.

### Viewing

The map is a web app served by the game server from files its owner controls, so the panel never shows it with the user's session:

- `GET /api/servers/:serverId/map` returns the status and, when a map is installed, a signed `viewUrl` of the form `/api/map-view/<serverId>/<token>/`. The token is an HMAC-SHA256 over the server id and an expiry; it stays the same for an hour and is valid for 12 to 13 hours. The signing secret is generated when the panel starts, so a panel restart invalidates open links (reopening the tab fixes it).
- `/api/map-view/...` requires no session; the signed link is the only credential. The panel forwards the request to the node (`/daemon/server/<id>/map/view/<path>`), which proxies it to the map's web server (the container's address in Docker, or the server's IP or `127.0.0.1` otherwise).
- Every answer carries `Content-Security-Policy: sandbox allow-scripts allow-popups allow-forms allow-pointer-lock allow-downloads`, `Referrer-Policy: no-referrer` and `X-Content-Type-Options: nosniff`, and cookies are stripped. The page therefore runs in an opaque origin that cannot call the panel's API. A small script injected into HTML pages replaces `localStorage`, `sessionStorage`, `document.cookie` and workers with in-memory stand-ins when the sandbox blocks them.
- At most 32 viewer requests per server run at once; extra requests wait up to 15 seconds.

### Removal

`DELETE /api/servers/:serverId/map` deletes the jar and the `.worldmap` record. Rendered tiles and the map's configuration stay on disk, so a reinstall does not start from zero. The plugin or mod unloads at the next restart.

---

## Using it

### In the panel

1. Open a Minecraft Java server and go to the **Map** tab (shown on servers of type `minecraft-java`).
2. Under **Choose the map**, pick BlueMap or Dynmap. For BlueMap, tick **I agree to BlueMap downloading Minecraft's resources (Mojang's EULA)**.
3. Click **Install BlueMap** / **Install Dynmap**. If the server is on it restarts; if it is off, the map starts the next time it is turned on (**Start server** is offered).
4. Wait while **The map is getting ready…**; the tab refreshes on its own (every 5 seconds while busy, every 15 seconds otherwise).
5. To uninstall, use **Remove map** and restart the server.

If the loader is not supported the tab says **The map needs Paper, Fabric or Forge** and offers **Change the loader** or **Install a modpack**.

### Via the API

Details: [API reference](../reference/11-api-reference.md).

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| GET | `/api/servers/:serverId/map` | `server.view` | Status: `supported`, `reason` (`vanilla`/`unknown`), `platform`, `gameVersion`, `installed`, `version`, `installing`, `lastError`, `running`, `ready`, `viewUrl` |
| POST | `/api/servers/:serverId/map/install` | `server.files.edit` | Body `{"kind":"bluemap"|"dynmap","acceptDownload":true}`; answers `202` |
| DELETE | `/api/servers/:serverId/map` | `server.files.edit` | Remove the map; answers `204` |
| GET, HEAD | `/api/map-view/:serverId/:token/*path` | none (signed link) | Viewer proxy used by the iframe |

---

## Requirements and configuration

- A Minecraft Java server running Paper (or a Paper-compatible fork), Fabric, Quilt, Forge or NeoForge.
- The node must reach `api.modrinth.com` and `cdn.modrinth.com`; BlueMap also downloads Mojang's resources from the game server.
- Optional but recommended: an extra port on the server (**Settings › Ports**) so the map has its own port reachable from outside.
- There are no configuration keys for this feature.

---

## Limits

- One map per server at a time; installing one removes the other's jar.
- Only one install per server at a time, and not during a backup or restore.
- The panel only patches the web port and BlueMap's `accept-download`; every other map setting is left to the map's own files.
- Viewer links expire after 12 to 13 hours and are invalidated when the panel restarts.
- The map's web server must answer plain HTTP on the node; the panel probes the candidates with a 1.5-second timeout and caches the address for 30 seconds.

---

## Troubleshooting

| Message | Cause and fix |
|---------|---------------|
| `the world map needs a Paper, Fabric, Forge or NeoForge server` | Vanilla, proxy or unknown loader. Change the loader or install a modpack. |
| `installing BlueMap needs consent to download Minecraft's resources (Mojang's EULA)` | `acceptDownload` was not sent. Tick the EULA box. |
| `the map must be bluemap or dynmap` | Wrong `kind` in the request. |
| `a map install is already running` / `a backup or restore is already running` | Wait for the other job to finish. |
| `<Map> has no build for <platform> <version> yet` | Modrinth has no build for that loader and version. Try the other map or another game version. |
| `could not tell which Minecraft version this server runs` | Set the version variable to a release, or start the server once so its version can be read. |
| `the file does not match Modrinth's checksum` / `refusing to download from <host>` | Download damaged or from an unexpected host. Retry later. |
| `the map's web server does not answer` (502 in the iframe) | The map is still starting, or the port is wrong or blocked. Wait, then **Reload the map**; check the console for the map's own errors. |
| `this map link is not valid any more; open the map again from the panel` (403) | The link expired or the panel restarted. Reopen the Map tab. |
| `too many map requests at once` (503) | More than 32 parallel requests for one server. Reload later. |
| `no world map is installed on this server` (404 on remove) | Nothing to remove; the jar may have been deleted by hand. |

---

## See also

- [Modpacks and plugins](modpacks-and-plugins.md)
- [Game version](game-versions.md)
- [Backups](backups.md): restore points
- [Security](../general/security.md) · [API reference](../reference/11-api-reference.md) · [Scopes reference](../general/scopes-reference.md)
