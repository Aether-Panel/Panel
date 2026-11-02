# Game version

Aether Panel can change the Minecraft version of a server in one click: it stops the server, saves a restore point, sets the new version, reinstalls and starts it again. The same version lists also feed the create-server form, so a version that does not exist cannot be typed.

The one-click change is an **experimental feature** and is off by default.

---

## What it does

- Shows a **Game version** card in the server's **Settings** tab with the installed version, whether it is the newest, and a **New version** picker.
- Lists the stable releases of the server's platform, newest first: vanilla Minecraft (from Mojang) or Paper, Folia, Velocity and Waterfall (from PaperMC).
- Changes the version in the background, with a restore point first, and reports the result in the card (**Done: the server now runs version …** or **The version could not be changed: …**).
- Offers the same lists in the create-server form for a template's version field.

---

## How it works

### Which servers can switch

The node looks at the template's install steps that run for this server (conditional steps are evaluated) and takes the first one that downloads the game:

- a `paperdl` step: the platform is its `project` (default `paper`) and the version variable is the one named in `minecraftVersion`;
- a `mojangdl` step: the platform is `vanilla` and the variable is the one named in `version`.

The step's value must be exactly a `${variable}` reference to an existing server variable. Templates that do not follow this pattern (for example Forge, NeoForge or CurseForge templates) do not get the card. The card shows the software from the `modlauncher` variable when the template has one (a Fabric server, for example, lists vanilla's versions).

A server with an installed modpack cannot switch, because the pack fixes the game version; see [Modpacks and plugins](modpacks-and-plugins.md).

### Version lists

- Vanilla: `https://piston-meta.mojang.com/mc/game/version_manifest_v2.json`, entries of type `release`.
- Paper, Folia, Velocity, Waterfall: `https://fill.papermc.io/v3/projects/<project>/versions`, only versions shaped like `1.21` or `1.21.4`.

Lists are sorted newest first, cut to 80 entries and cached on the node (or panel) for one hour per platform.

### The change

`POST /api/servers/:serverId/version` checks that the target is in the list (or is `latest`), and refuses while a backup, restore or another version change is running. It answers `202` and continues in the background, printing each step in the console:

1. Stops the server if it is running and waits for the process to exit.
2. Creates a restore point (reason `version`). If that fails, nothing is changed.
3. Sets the version variable. For `paperdl`, if the step reads its build number from a variable, that variable is reset to `latest`, because a build pinned for the old version does not exist for the new one.
4. Saves the server and runs the template's installation again.
5. If the installation fails, the variables are put back and the error says a restore point was saved.
6. Starts the server again if it was running.

While a change runs, the card polls every 3 seconds. Going back to an older version may not open newer worlds; the restore point is the way back.

---

## Using it

### In the panel

1. An administrator enables **Settings › General › Experimental features › Game version switch**.
2. Open the server, go to **Settings** and find the **Game version** card. It is hidden when the template does not support switching, a modpack is installed, or the user lacks permission.
3. Pick the **New version** and click **Change version**. The confirmation lists the steps; confirm with **Change version**.
4. Follow the progress in the card or the console. If it fails, restore the restore point from **Backups**.

### Via the API

Details: [API reference](../reference/11-api-reference.md).

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| GET | `/api/servers/:serverId/version` | `server.view` | `VersionInfo`: `platform`, `software`, `variable`, `current`, `versions`, `changing`, `lastError`. Answers `204` when the server cannot switch, `502` when the lists cannot be fetched |
| POST | `/api/servers/:serverId/version` | `server.install` + experimental flag | Body `{"version":"1.21.4"}` (or `"latest"`); answers `202` |
| GET | `/api/game-versions/:platform` | `login` | Stable releases of `vanilla`, `paper`, `folia`, `velocity` or `waterfall` (used by the create-server form) |

---

## Requirements and configuration

| Key | Default | Environment variable | Notes |
|-----|---------|----------------------|-------|
| `panel.experimental.versionSwitch` | `false` | `SKYPANEL_PANEL_EXPERIMENTAL_VERSIONSWITCH` | Enables `POST /api/servers/:serverId/version` and shows the card. Editable in **Settings › General › Experimental features**. |

- The node (and, for `/api/game-versions`, the panel) must reach `piston-meta.mojang.com` and `fill.papermc.io`.
- A template built on `paperdl` or `mojangdl` with the version in a variable, as described above.

See [Configuration](../backend/02-configuration.md).

---

## Limits

- Only stable releases are offered; snapshots, pre-releases and RCs are not.
- At most the 80 newest versions are listed.
- One change per server at a time; not during a backup, restore or modpack install.
- Turning the experimental flag off blocks new changes (the `POST` returns `403`); `GET` keeps working.
- Older Minecraft versions may refuse worlds saved by newer ones.

---

## Troubleshooting

| Message | Cause and fix |
|---------|---------------|
| `this experimental feature is turned off in the panel settings` (403) | Enable **Game version switch** in the experimental features. |
| `this server's template does not let the panel change its game version` (400) | The template has no matching `paperdl`/`mojangdl` step, or a modpack is installed. |
| `that version is not available for this server` (400) | The version is not in the current list. Reload the card and pick one from the list. |
| `a version change is already running` (409) | Wait for it to finish. |
| `a backup or restore is already running` (409) | Wait for the backup or restore to finish. |
| `could not create a restore point: ...` | Not enough space for the restore point, or a backup started meanwhile. Free space and retry. |
| `the new version could not be installed (a restore point was saved): ...` | The download or install step failed. Check the console; restore from **Backups** if needed. |
| `unknown game platform; use vanilla, paper, folia, velocity or waterfall` (400) | Wrong platform in `/api/game-versions/:platform`. |
| `502 Bad Gateway` on the version routes | Mojang or PaperMC could not be reached. The card stays hidden until they answer again; the create form falls back to a text field. |

---

## See also

- [Modpacks and plugins](modpacks-and-plugins.md)
- [World map](world-map.md)
- [Backups](backups.md): restore points
- [Operations](../backend/06-operations.md): `paperdl` and `mojangdl`
- [API reference](../reference/11-api-reference.md) · [Scopes reference](../general/scopes-reference.md) · [Configuration](../backend/02-configuration.md)
