# Server properties editor

The **Properties** tab turns a Minecraft: Java Edition server's `server.properties` into a form: every option with a plain explanation, grouped by topic, with a live preview of how the server looks in the game's server list. It is an **experimental** feature (marked *Beta* in the tab bar) and starts turned off.

## What it does

- Reads `server.properties` from the server's files and shows its options in six sections: **The game**, **Players**, **The world**, **Presentation**, **Network and performance** and **Advanced**.
- Uses the right control for each option: switches for true/false, lists for fixed choices (game mode, difficulty, level type…), number fields with limits, text fields, and a hidden field for `rcon.password`.
- Shows Minecraft's own default for options that are not in the file yet, marked **default value**.
- Has a dedicated **Message of the day (MOTD)** card: two lines at most, a palette with the 16 colour codes (`§0`–`§f`) and the formatting codes (bold, italic, underline, strikethrough, clear), and a **Preview in the server list** (approximate: the game uses its own font).
- Warns under risky values: `online-mode=false` ("anyone can join with any name, including an admin's") and `hardcore=true`.
- Marks options that **Only affect new worlds** (`level-seed`, `level-type`, `generate-structures`, `generator-settings`).
- Shows `server-port` and `server-ip` read only: the panel manages them from the server's ports, and the field's hint points to **Settings → Ports** (see [Server settings](./server-settings.md)).
- Lists options the panel does not know under **Advanced** with an *unknown* tag; they are kept and can still be edited as text.
- Has a search box (**Search an option…**) that filters by key.

## How it works

The editor works on the file itself through the file API; there is no separate storage.

1. **Load**: `GET /api/servers/:id/file/server.properties`. The text is parsed with the same rules as Java's `Properties` (`key=value`, `key: value` or `key value`, `#`/`!` comments, line continuations, `\uXXXX` escapes).
2. **Edit**: each change is kept as a draft. Number fields are checked against their limits (for example `max-players` 1–10000, `view-distance` and `simulation-distance` 3–32); while a value is wrong the bar says **Fix the highlighted values.** and saving is disabled.
3. **Save**: the file is **read again**, the changed keys are applied on top of that fresh copy and the result is written with `PUT /api/servers/:id/file/server.properties`. Untouched lines (comments, blank lines, order, unknown keys) stay byte for byte; changed keys are rewritten in place; keys that were not in the file are appended at the end. Values are escaped the way `Properties.store` does (non-ASCII as `\uXXXX`, `:` and `=` escaped), which every Minecraft version reads back.
4. **Apply**: Minecraft only reads the file on start. After saving, the page says **Changes saved.** and, if the server is running, offers **Restart now** (`POST /api/servers/:id/restart`).

Which options are shown: the common ones ("core") always appear, using the default when missing; the rest appear only when the file already has them. The **Advanced** section is collapsed behind **Show advanced options (N)** and opens by itself while searching.

## Using it

### In the panel

1. An administrator turns it on once: **Panel Settings → General → Experimental features → Properties editor** (see [Panel settings](./panel-settings.md)).
2. Open a Minecraft Java server and go to the **Properties** tab.
3. Change the options. Each changed field has an undo button; **Discard** drops every change.
4. Press **Save changes** (or Ctrl/Cmd+S). Leaving the page with unsaved changes asks for confirmation.
5. If the server is running, press **Restart now** in the confirmation strip, or restart later.

### Via the API

There is no dedicated endpoint: the editor uses the file and power routes.

| Method and path | Scope | Purpose |
|---|---|---|
| `GET /api/servers/:id/file/server.properties` | `server.files.view` | Read the file |
| `PUT /api/servers/:id/file/server.properties` | `server.files.edit` | Write the whole file (raw body) |
| `POST /api/servers/:id/restart` | `server.start` and `server.stop` | Apply the changes |

See [Files](../reference/11-api-reference.md) in the API reference.

## Requirements and configuration

- The server's type must be `minecraft-java`.
- The user needs `server.files.edit` on the server (the tab is hidden otherwise). The owner holds it.
- The experimental switch must be on:

| Key | Default | Environment variable | Effect |
|---|---|---|---|
| `panel.experimental.propertiesEditor` | `false` | `SKYPANEL_PANEL_EXPERIMENTAL_PROPERTIESEDITOR` | Shows the **Properties** tab on Minecraft Java servers |

Turning it off only hides the tab; the file is not touched. See [Configuration](../backend/02-configuration.md).

## Limits

- Only for servers whose type is exactly `minecraft-java`. Bedrock, proxies and other games do not get the tab.
- The switch only hides the tab in the web: the file routes stay available to anyone with the file scopes, as they always were.
- The catalogue covers the usual vanilla options; options added by newer versions or by mods show under **Advanced** as unknown text fields, without validation.
- Changes apply only after a restart; the editor does not send commands to the running game.
- The MOTD preview is approximate.

## Troubleshooting

| What you see | Cause | Fix |
|---|---|---|
| **No server.properties yet** | Minecraft creates the file on the first start (the API answered `404`) | Start the server once and come back |
| **No access to the files** | `403`: no `server.files.view` | Ask the owner for file permissions |
| **Could not read server.properties** | Node offline or another error | Check the node and press **Retry** |
| **The changes could not be saved.** (or the node's message) | The write failed (node offline, disk limit reached, no `server.files.edit`) | Check the node and the disk usage, then save again |
| The **Properties** tab does not appear | Experimental switch off, server type not `minecraft-java`, or no `server.files.edit` | Check the three conditions |
| `server-port` cannot be edited | It is managed by the panel | Change the primary port in **Settings → Ports** |

## See also

- [Server settings](./server-settings.md): ports, variables and auto-start.
- [Files and compression](../general/19-files-compression.md)
- [Panel settings](./panel-settings.md): experimental features.
