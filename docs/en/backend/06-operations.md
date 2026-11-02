# Operations Engine

Templates and scheduled tasks describe their work as lists of **operations**: small JSON steps such as "download this file", "write this text", "run this command" or "install Forge". The daemon turns each list into a process and runs the steps one after another.

Code: one package per operation in `internal/operations/*`, the runner in `internal/servers/operation_process.go`, the server-level operations in `internal/servers/task_operations.go` and the condition functions in `internal/servers/operation_functions.go`.

---

## Where operation lists are used

| List | Where it is declared | When it runs |
|------|----------------------|--------------|
| `install` | server definition (template) | On **Install** and on automatic installs when a server is created (see `panel.servers.autoInstall` in [Configuration](./02-configuration.md)). The server is stopped first. |
| `uninstall` | server definition | When the server is deleted, before its environment is removed. A failed step is logged and shown on the console, and the server is deleted anyway. |
| `run.pre` | server definition, inside `run` | Every time the server starts, before the start command. |
| `run.post` | server definition, inside `run` | Every time the server process exits. Its tokens and conditions get `success` and `exitCode`. |
| `operations` | each scheduled task | When the task fires (cron) or is run by hand. |

The panel also builds lists internally: modpack and version installs (`curseforge`, `javadl`, the template's `writefile` steps) reuse the same runner.

---

## Anatomy of a step

A step is a JSON object with a `type`, an optional `if` and the arguments of that type, all at the same level:

```json
{
  "type": "download",
  "if": "modlauncher == 'vanilla'",
  "files": ["https://example.com/${version}/server.jar"]
}
```

| Field | Description |
|-------|-------------|
| `type` | Operation name (see [the list](#operation-reference)). An unknown type stops the list with an error. |
| `if` | Optional [CEL condition](#conditions). The step is skipped when it is false. |
| other keys | Arguments of the operation. Unknown keys are ignored. |

There is no `description`, `ignoreError` or per-step timeout: **the first step that fails stops the list** and the error is shown in the server console (`Failed to install server`, `Error running pre execute`, `Failed to run post-execution steps`, `Failed to setup tasks`).

---

## Variable substitution

When each step runs, just before it starts, every **string** argument, and every string inside a list of strings, has its `${name}` tokens replaced:

```json
{ "type": "writefile", "target": "eula.txt", "text": "eula=${eula}" }
```

| Token | Value |
|-------|-------|
| `${<variable>}` | Each server variable (the `data` block of the definition), by name |
| `${rootDir}` | Absolute path of the server folder on the node |
| `${core:os}`, `${core:arch}` | Go's `runtime.GOOS` / `runtime.GOARCH` of the daemon (for example `linux`, `amd64`) |
| `${success}`, `${exitCode}` | Only in `run.post`: whether the process ended as expected and its exit code |

Notes:

- The syntax is `${name}`, not `{{name}}`. A token whose name does not exist is left as it is (this is how `scraperdl` keeps its own `${match}` placeholder).
- Numbers, booleans and nested objects are passed unchanged; only strings and lists of strings are substituted.
- The server's `run.environmentVars` go through the same substitution and are handed to the `command` operation as environment variables.
- Substitution happens **step by step**, with the values known at that moment. A value an earlier step writes into a variable (see *output variables* below) is visible to the `${...}` tokens and the `if` conditions of the later steps of the same list, and to later runs.

---

## Conditions

`if` is a [CEL](https://github.com/google/cel-spec) expression that must evaluate to a boolean. The syntax, operators and more examples are in [CEL engine](../general/18-conditions-cel.md); this section only covers what operations see.

| Name | Value |
|------|-------|
| each server variable | its current value, by name (`modlauncher == 'paper'`, not `variables.modlauncher`) |
| `os`, `arch` | `runtime.GOOS` / `runtime.GOARCH` of the daemon |
| `env` | the server's **environment type** (`docker`, `host`…), not environment variables |
| `serverId` | the server identifier |
| `success` | `true` while a list runs (a failed step stops the list, so there is no "previous step failed" state). In `run.post` it says whether the process ended with the expected exit code, so `"if": "!success"` runs a step only after a crash |
| `exitCode` | Only in `run.post`: the exit code of the process |

| Function | Description |
|----------|-------------|
| `file_exists(path)` | `true` when `path`, relative to the server folder, exists. Paths that leave the folder return `false`. |
| `in_path(cmd)` | `true` when `cmd` is found in the daemon's `PATH` |
| `is_server_running()` | `true` while the server process is running (no arguments) |

```json
{ "type": "mojangdl", "if": "!file_exists('server.jar')", "version": "${version}", "target": "server.jar" }
```

An empty or missing `if` always runs. A condition that does not compile, or does not return a boolean, stops the list with an error.

`${name}` can also be written inside a condition, as in the rest of the step: `"if": "in_path('java${javaversion}')"`. Before the condition is compiled, each `${name}` whose name is one of the values above (server variables, `env`, `serverId`, `success`, `exitCode`, `os`, `arch`) is replaced. Inside a string literal the value is inserted escaped for that literal, so it can never become CEL code; outside a literal the token stands for the variable itself. Other names (such as `rootDir`) and bytes literals (`b"..."`) are left untouched. Code: `conditions/variables.go`.

---

## File safety

Every operation that writes into the server folder goes through the server's confined **file server** (`files.FileServer`, `openat2` with `RESOLVE_BENEATH`; see [Security](./09-security.md#file-access)):

- Paths are relative to the server folder; absolute paths inside it are accepted, anything that leaves it (`../`, absolute paths elsewhere) is refused.
- Downloads are first saved into a temporary folder under the daemon cache (`daemon.data.cache`) and then copied in with `O_NOFOLLOW`: if the game left a **symlink** where the file goes (for example `server.jar -> /etc/...`), the write is refused instead of overwriting the link's target. Missing parent folders are created.
- `alterfile`, `writefile`, `mkdir`, `move`, `extract`, `archive`, `resolveforgeversion` and the `curseforge` extraction all use the same file server.

---

## Download network rules

`download` and `scraperdl` take their URLs from the template, and often from a variable the server owner can edit; `githubdl` downloads an address that comes from the release (and that GitHub redirects to its storage). Their HTTP client checks every address it connects to (at connection time, so a DNS name that changes what it resolves to cannot get around it):

| Address | Behaviour |
|---------|-----------|
| Loopback, link-local (includes cloud metadata `169.254.169.254`), unspecified, multicast | Always refused (`refusing to download from internal address ...`). |
| Private networks (RFC 1918, Docker networks, the node's LAN) | Refused while `security.blockPrivateDownloads` is `true` (**the default**). With `false`, allowed and a warning line is written to the console. |
| Public addresses | Allowed. |

Set `security.blockPrivateDownloads` to `false` (env `SKYPANEL_SECURITY_BLOCKPRIVATEDOWNLOADS=false`) only if templates download from mirrors on your own network. The other downloaders, and the GitHub API call of `githubdl`, use fixed, well-known hosts (Mojang, PaperMC, Maven, Adoptium, nodejs.org, GitHub, Sponge, CurseForge) and the panel's regular HTTP client.

---

## Operation reference

30 types are registered: 26 from `internal/operations` plus `start`, `stop`, `restart` and `backup` from `internal/servers`. Argument names are case-sensitive except in `spongedl`.

### Files

#### `writefile` — write a text file

| Argument | Description |
|----------|-------------|
| `target` | File to create or overwrite |
| `text` | Content |

```json
{ "type": "writefile", "target": "eula.txt", "text": "eula=true" }
```

#### `alterfile` — search and replace in a file

| Argument | Description |
|----------|-------------|
| `file` | Existing file to edit |
| `search` | Text (or regular expression) to look for |
| `replace` | Replacement, inserted literally (regex groups such as `$1` are not expanded) |
| `regex` | `true` to treat `search` as a Go regular expression in multi-line mode (`(?m)`, so `^` and `$` match each line) |

```json
{ "type": "alterfile", "file": "server.properties", "regex": true,
  "search": "^server-port=.*$", "replace": "server-port=${port}" }
```

#### `mkdir` — create a folder

| Argument | Description |
|----------|-------------|
| `target` | Folder to create, with any missing parents (mode `0755`) |

#### `move` — move or rename

| Argument | Description |
|----------|-------------|
| `source` | File or glob pattern (`*.jar`) |
| `target` | Destination file, or an existing folder to move into |

Several matches need an existing folder as `target`. When the combination is not possible (several files onto one file, a missing folder) the step logs the problem and does nothing, without failing.

#### `extract` — unpack an archive

| Argument | Description |
|----------|-------------|
| `source` | Archive inside the server folder |
| `destination` | Folder to extract into |

Counts against the server's disk limit. Supported formats: [Files and compression](../general/19-files-compression.md).

#### `archive` — create an archive

| Argument | Description |
|----------|-------------|
| `source` | List of files and folders |
| `destination` | Archive to create; refused if it already exists |

Counts against the server's disk limit.

### Processes and console

#### `command` — run commands

| Argument | Description |
|----------|-------------|
| `commands` | List of command lines, run one after another |
| `stdin` | Optional stdin configuration (same shape as `run.stdin`) |

Each command runs through the server's environment (with Docker, in a container from the server's image) with `run.environmentVars`. A non-zero exit code fails the step. The server must be **stopped**: a command cannot start while the server process is running.

```json
{ "type": "command", "commands": ["chmod +x ./start.sh"] }
```

#### `stdin` — send a line to the running server

| Argument | Description |
|----------|-------------|
| `command` | Text written to the server's console input |

Fails with "server offline" if the server is not running. Typical in tasks: `{ "type": "stdin", "command": "save-all" }`.

#### `console` — print a message

| Argument | Description |
|----------|-------------|
| `message` | Shown in the server console as `[DAEMON] Message: ...` |

#### `sleep` — wait

| Argument | Description |
|----------|-------------|
| `duration` | Go duration string (`5s`, `1m30s`) |

#### `dockerpull` — pull a Docker image

| Argument | Description |
|----------|-------------|
| `imageName` | Image to pull |

Only works for Docker servers; other environments fail with "environment not supported".

### Server control (tasks)

These act on the server itself and are meant for scheduled tasks. They take no arguments except `backup`.

| Type | Behaviour |
|------|-----------|
| `start` | Starts the server; does nothing if it is already running. |
| `stop` | Stops the server and waits up to 5 minutes for it to exit. |
| `restart` | Only if the server is running: stops it, waits up to 5 minutes, starts it again. A stopped server is left stopped. |
| `backup` | Makes an automatic backup (file name starting with `auto-`). `keep` (1–50, default 5) is how many automatic backups to keep; older `auto-` backups are deleted, manual ones never. |

### Generic downloads

#### `download` — download files by URL

| Argument | Description |
|----------|-------------|
| `files` | List of URLs |

Each file is saved in the server folder under the name the server gives it (`Content-Disposition`) or the last part of the URL. You cannot choose the folder or the name; follow with `move` if needed. Subject to the [network rules](#download-network-rules).

```json
{ "type": "download", "files": ["https://example.com/builds/${version}/server.jar"] }
```

#### `scraperdl` — find a version on a web page and download it

| Argument | Description |
|----------|-------------|
| `url` | Page to read (must answer `200`) |
| `regex` | Go regular expression; its **first capture group** is the match |
| `downloadUrl` | URL to download, where `${match}` is replaced by the match |
| `outputVariable` | Optional server variable that receives the match |

Both the page and the file are subject to the [network rules](#download-network-rules). The file is saved like `download` does.

#### `githubdl` — latest GitHub release asset

| Argument | Description |
|----------|-------------|
| `repository` | `owner/name` |
| `assetMatch` | Text the asset's file name must contain; the first asset that contains it is used |
| `outputVariable` | Optional server variable that receives the saved **file name** (not the URL), so later steps can `extract` or `move` it |

Uses the unauthenticated GitHub API (`/repos/<repo>/releases/latest`), which is rate limited per IP. The asset is saved in the server folder under the URL's file name. Its download follows the [network rules](#download-network-rules).

### Minecraft

#### `mojangdl` — vanilla server jar

| Argument | Description |
|----------|-------------|
| `version` | `release` or `latest` (latest release), `snapshot` (latest snapshot) or an exact version id |
| `target` | File name to save as (for example `server.jar`) |

Reads Mojang's version manifest and downloads that version's server jar.

#### `paperdl` — PaperMC projects

| Argument | Description |
|----------|-------------|
| `minecraftVersion` | Required. `latest` picks the newest version of the project |
| `build` | Required. Build to download |
| `target` | File name to save as |
| `project` | PaperMC project (default `paper`) |

Uses the PaperMC Fill v3 API (`fill.papermc.io`). Files are kept in the daemon cache and a cached copy is reused when its SHA-256 matches the one the API publishes.

#### `fabricdl` — Fabric installer

No arguments. Downloads the newest Fabric installer listed by `meta.fabricmc.net` and saves it as `fabric-installer.jar`; a template then runs it with `command`.

#### `forgedl` — Forge installer

| Argument | Description |
|----------|-------------|
| `minecraftVersion` | Used to find the recommended version when `version` is empty |
| `version` | Full Forge version (`1.20.1-47.2.0`). Empty: the `<minecraftVersion>-latest` promotion |
| `target` | File name to save the installer as |
| `outputVariable` | Server variable that receives the version (default `opForgeVersion`) |

One of `version` or `minecraftVersion` is required. Downloads from `maven.minecraftforge.net`; Maven `.sha1` files are used to reuse cached copies.

#### `neoforgedl` — NeoForge installer

| Argument | Description |
|----------|-------------|
| `minecraftVersion` | Used to pick the newest NeoForge build when `version` is empty (`1.21.1` matches NeoForge `21.1.*`) |
| `version` | Exact NeoForge version |
| `target` | File name to save the installer as |
| `outputVariable` | Default `opNeoForgeVersion` |

One of `version` or `minecraftVersion` is required. Reads `maven.neoforged.net`.

#### `resolveforgeversion` / `resolveneoforgeversion` — detect the installed version

| Argument | Description |
|----------|-------------|
| `version` | If set, it is simply copied to the output variable |
| `minecraftVersion` | See below |
| `outputVariable` | Default `opForgeVersion` / `opNeoForgeVersion` |

Without `version`, they look in `libraries/net/minecraftforge/forge` (or `libraries/net/neoforged/neoforge`), through the server's file server, for version folders that contain `unix_args.txt` and store the chosen one:

- `resolveneoforgeversion` picks the newest folder for `minecraftVersion`. NeoForge versions drop the leading `1.` of the Minecraft version (`1.21.4` → `21.4.*`, `1.21` → `21.0.*`). When no folder matches, or `minecraftVersion` is empty, it uses the first supported folder found.
- `resolveforgeversion` keeps the first supported folder found (folders are read in name order); in practice `minecraftVersion` does not change the pick.

If the folder does not exist the variable is set to an empty string; that is not an error.

#### `spongedl` — SpongeVanilla / SpongeForge

| Argument | Description |
|----------|-------------|
| `spongeType` | `spongevanilla` or `spongeforge` |
| `spongeVersion` | Exact Sponge version; empty means the newest one matching the filters below |
| `minecraftVersion` | Filter by Minecraft version |
| `recommended` | `true` to only consider recommended builds |

Uses the Sponge download API v2. For both types the jar is the build's `universal` asset, or the jar without a classifier when the build has none (older builds). `spongevanilla` saves `server.jar`. `spongeforge` first runs `forgedl` for the Minecraft version Sponge declares (saved as `forge-installer.jar`), creates `mods/` and saves SpongeForge as `mods/_aspongeforge.jar`.

#### `curseforge` — CurseForge modpack server pack

| Argument | Description |
|----------|-------------|
| `projectId` | Required. CurseForge project id |
| `fileId` | Optional file id (`fileID` is also accepted). Empty: the newest release file of the project |
| `java` | Java binary used to run installers (default `java`) |

Requires a CurseForge API key in `daemon.curseforge.key` (official builds can embed one at build time); without it the step fails before running. What it does:

1. Resolves the **server pack** (if the chosen file is a client pack that points to a server pack, that one is used). A file that is not a server pack fails with `not server pack`.
2. Downloads it into the daemon cache and extracts it into the server folder through the file server (a single top-level folder is stripped).
3. Detects the loader: an installer jar shipped in the pack (`forge-*-installer.jar` for Forge, `neoforge-*-installer.jar` for NeoForge), else the pack's `variables.txt` (`MODLOADER`, `MINECRAFT_VERSION`, `MODLOADER_VERSION`, `FABRIC_INSTALLER_VERSION`), else the client manifest's primary loader.
4. **Forge / NeoForge**: downloads the installer if needed, runs `<java> -jar <installer> --installServer`, removes the installer and its log, renames a pre-1.17 jar or the Forge shim to `server.jar`, and if there is still no `server.jar` fetches NeoForge's ServerStarterJar (0.1.25) as `server.jar`.
5. **Fabric**: downloads the Fabric server launcher from `meta.fabricmc.net` as `server.jar`. Only when that answers `404` does it fall back to the Fabric installer from `maven.fabricmc.net`, saved as `fabric-installer.jar` and run with `server -mcversion <v> -loader <v> -downloadMinecraft`; any other error fails the step. A download that does not answer `200` is never saved as a jar.
6. An unknown loader leaves the extracted files in place and ends without error.

Running installers needs the server to be stopped (see `command`).

### Runtimes and Steam

#### `javadl` — Java (Eclipse Temurin)

| Argument | Description |
|----------|-------------|
| `version` | Integer feature version (`17`, `21`) |

If `java<version>` is not already in `PATH`, downloads the matching Temurin JDK from the Adoptium API into the binaries folder (`daemon.data.binaries`) and creates the `java<version>` and `javac<version>` links there. The daemon adds that folder to its `PATH` at start. Used mainly for servers on the host environment; Docker servers bring Java in their image.

#### `nodejsdl` — Node.js

| Argument | Description |
|----------|-------------|
| `version` | Integer major version (`20`) |

Picks the newest `v<version>.*` release from `nodejs.org/dist/index.json`, extracts it into the binaries folder and links `node<version>` and `npm<version>`.

#### `steamgamedl` — Steam game via DepotDownloader

| Argument | Description |
|----------|-------------|
| `appId` | Required. Steam app id (`appID` is also accepted) |
| `username`, `password` | Optional Steam login, for games that need an account |
| `extraArgs` | Extra DepotDownloader arguments (list) |

Steps: downloads DepotDownloader into `<binaries>/depotdownloader` if missing (version `daemon.depotDownloader.version`, default `latest`), fetches the Steam client metadata into `.steam/`, runs DepotDownloader once with `-manifest-only` into `.manifest/` and then for the real download into the server folder with `-validate`, and marks the files the manifests flag as executable. `-use-lancache` is added unless `daemon.depotDownloader.disableLancache` is `true`. Runs through the server's environment, so the server must be stopped.

---

## Output variables

Some operations write a value into a server variable when they finish:

| Operation | Variable | Value |
|-----------|----------|-------|
| `forgedl`, `resolveforgeversion` | `outputVariable` (default `opForgeVersion`) | Forge version |
| `neoforgedl`, `resolveneoforgeversion` | `outputVariable` (default `opNeoForgeVersion`) | NeoForge version |
| `githubdl` | `outputVariable` (if set) | File name saved |
| `scraperdl` | `outputVariable` (if set) | Regex match |

The variable is created if the definition does not have it. Templates typically use it in the start command (`${opForgeVersion}`) or in later conditions.

---

## Example: install list

```json
"install": [
  { "type": "mojangdl", "if": "modlauncher == 'vanilla'", "version": "${version}", "target": "server.jar" },
  { "type": "paperdl", "if": "modlauncher == 'paper'", "minecraftVersion": "${version}", "build": "${paperbuild}", "target": "server.jar" },
  { "type": "writefile", "target": "eula.txt", "text": "eula=${eula}" }
]
```

## Example: scheduled task

```json
{
  "name": "Nightly restart",
  "cronSchedule": "0 4 * * *",
  "operations": [
    { "type": "stdin", "command": "say Restarting in 1 minute" },
    { "type": "sleep", "duration": "1m" },
    { "type": "backup", "keep": 7 },
    { "type": "restart" }
  ]
}
```

---

## Adding an operation

1. Create a package under `internal/operations/<name>` with a type that implements `skypanel.Operation` (`Run(skypanel.RunOperatorArgs) skypanel.OperationResult`) and a factory that implements `skypanel.OperationFactory` (`Create(skypanel.CreateOperation)` and `Key()`).
2. Read arguments from `op.OperationArgs` (use `op.Arg("a", "b")` to accept alternative spellings). Write files through `args.Server.GetFileServer()` or `files.WriteFile`, never with plain `os` calls on the server path.
3. Add the factory to the `factories` list in `internal/servers/operation_process.go`.
4. Return `VariableOverrides` in the result to set server variables.

```go
func (of OperationFactory) Key() string { return "mycustom" }

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
    return MyCustom{Target: cast.ToString(op.OperationArgs["target"])}, nil
}

func (m MyCustom) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
    err := args.Server.GetFileServer().MkdirAll(m.Target, 0755)
    return skypanel.OperationResult{Error: err}
}
```
