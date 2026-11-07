# Templates

A template is the JSON blueprint a server is created from: its variables, install steps, run command, environment and requirements. The **Templates** page lists the templates the panel can install and lets administrators add their own. This page covers the page, where templates come from and how to add them; the template format itself is described in [Core concepts: Templates](../general/concepts.md) and [Operations](../backend/06-operations.md).

## What it does

The page (**Template Management**, `/templates`) has two tabs:

- **Local**: templates stored in this panel's database. They can be created, edited, duplicated, uploaded and deleted here.
- **Community**: templates read from an outside source, either the template index configured in `templates.url` or the Git repositories added through the API. They are read only, but can be duplicated as a local template.

Each card shows the template's logo, name, category and a **Use Template** button that opens the create-server wizard with that template already chosen (`/servers/?create=true&template=<name>&repo=<id>`).

Buttons, by permission:

| Button | Needs |
|---|---|
| **Upload** (one or more `.json` files) | `templates.local.edit` |
| **New template** (opens the [template editor](#template-editor)) | `templates.local.edit` |
| **Edit** / **Delete** in a local template's menu | `templates.local.edit` |
| **Duplicate** in any template's menu, local or community | `templates.local.edit` |

**Search.** Above the tabs, it filters both by name, display name and type, ignoring case and accents. It also knows common synonyms (`c#` and `csharp` find .NET, `golang` Go, `mysql` MariaDB, `cs2` Counter-Strike 2, `spring` or `maven` Java; the list is in `client/frontend/src/lib/template-search.ts`). The tab counts show the matches. The create-server wizard has the same search in its template step.

**Logos.** The template's `icon` field picks its logo from `client/frontend/public/img/icons/` (`client/frontend/src/lib/template-logos.ts`): the official brand marks as SVG for languages, databases and services, and each game's official icon as PNG. A template without `icon` is recognised by its name (`minecraft-*`, `terraria-*`, `discord-*`…). The logo also shows in the wizard, on server cards and in the server header, which use the server's `icon` or, for older servers, its type.

The page itself and the **Templates** menu entry need `templates.view` (the default "Usuario" role has it); without it the page sends you to the dashboard.

## How it works

### Local templates

Stored in the panel database, one row per template name. Uploading a file creates or replaces the template named after the file (`spigot.json` becomes `spigot`). Each uploaded file:

- must end in `.json` and be at most 2 MB;
- must be valid JSON;
- must have install steps (`install`) or a run command (`run.command`), otherwise it is refused with `ErrInvalidTemplate`.

If the file has no `display`, the name is used.

The panel keeps a template's JSON as it was written, whether uploaded or saved: the variables keep their order and fields the panel does not know are not lost. Templates saved before this behaviour came in keep the alphabetical order they were stored with until they are saved again.

Local templates are created and edited in the [template editor](#template-editor), which saves them with `PUT /api/templates/0/<name>`; a template with the same name is replaced.

### Template editor

A full page (`/templates/editor/`) to build a template by filling in forms, without writing JSON, while the JSON stays at hand for whoever prefers it. It needs `templates.local.edit`; without it, the page sends you to **Templates**.

#### Getting started

**New template** offers four ways to start. None of them saves anything until you press **Save** in the editor.

| Option | What it opens | Address |
|---|---|---|
| **Blank** | The minimum: empty name, display name and category, no variables or steps, a start and a stop command to fill in, and one Docker supported environment with the `ghcr.io/aether-panel/generic` image | `/templates/editor/` |
| **Base template** | A ready template for a common case, without a name: **Minecraft: Java (Paper)**, **Steam game**, **Node.js app**, **Python app** or **Generic Docker** | `/templates/editor/?preset=<id>` |
| **Duplicate an existing one** | A copy of a local or community template, picked with a search (the same as the list's: name, display name, type and synonyms) | `/templates/editor/?from=<repo>:<name>` |
| **Import a .json** | A template file, to review it before saving it. Without a `name`, the file's name is used | `/templates/editor/?import` |

In the list, each template's **⋯** menu has **Duplicate** and, on local ones, **Edit** (`/templates/editor/?name=<name>`).

- In the **Duplicate** search, the down arrow moves into the list and **Enter** opens the first result (only once something has been typed).
- A copy keeps its source's name when no local template has it; otherwise it becomes `<name>-copy` (or `-copy-2`…) and its display name gets "(copy)" added.
- Base templates write their variables' texts (display name and description) in the language of whoever creates them: that is what server creators will read. The Minecraft one uses Paper; vanilla means swapping the `paperdl` step for `mojangdl`. The Steam one leaves the `appid` and the executable empty (and required): fill them in before saving, or whoever creates the server will have to type them.
- They all use only `ghcr.io/aether-panel/*` or official images, and the panel accepts each of them when saved (the Steam one once its `appid` and executable are filled in).

#### The page

- **Sections**, in a side menu (on a phone, a selector at the top): **Information**, **Variables**, **Installation**, **Run**, **Environment**, **Requirements** and **Advanced**. Each shows how many items it has (variables, steps) and how many errors and warnings.
- **Form | JSON | Split**: the form and the JSON are two views of the same object; a change in one shows in the other at once. **Split** (form and JSON side by side) is only offered on screens 1280 px wide or more; on a narrower one it shows as **Form**.
- **Preview**, **Download .json** and **Save**. Below 1280 px, **Preview** and **Download .json** are icon-only, and below 420 px **Save** is too. **Ctrl+S** (**⌘S** on macOS) saves from any field, inside the JSON too.
- Leaving with unsaved changes makes the browser ask first.

| Section | What it edits |
|---|---|
| **Information** | Name (the unique ID), display name, category (`type`) and icon |
| **Variables** | What whoever creates a server fills in. Each variable is a card that folds, with its key, type (text, whole number, yes/no or list of options), default value, display name, description, options and the required, user-editable and internal flags. They are reordered by dragging or with ↑ ↓, in the order "Create server" shows them. A key that is only a number always comes first and cannot be moved. Renaming a key can also rename every `${key}` that uses it |
| **Installation** | The install steps and, apart, the uninstall ones. **Add step** opens a search with the operations by category (Files, Downloads, Games/Minecraft, Steam, Java/Node, System). Each operation has its form; each step, its condition (**Always**, **Host only**, **Docker only** or **Custom**, a CEL expression). A step of a type the panel doesn't know is edited with **Edit this step's JSON** |
| **Run** | Start command (**One command** or **By condition**, a list of commands with their condition), stop (command or signal code), working directory, environment variables, console (standard input, RCON, RCON over WebSocket or telnet), automatic start and restart, expected exit code and the **Before start** and **After stop** steps |
| **Environment** | Default environment (host or Docker) and supported environments (image, ports, network, mounted folders, labels, Docker `hostConfig` and `config`) |
| **Requirements** | Operating systems, architectures and programs the node needs (`java \|\| java21` accepts either) |
| **Advanced** | Variable groups, stats (`jcmd`), status query (Minecraft or Steam A2S) and keep-alive. What has no form is edited as JSON in its own section |

**Insert variable**, next to commands and step fields, types `${variable}` into the text field that last had the focus (or at the JSON's cursor).

**What "Create server" takes from the environment.** The wizard always creates servers in Docker, with the **first Docker environment** under **Supported environments** (its image, ports and settings). The default environment is kept in the template, but the wizard doesn't use it. With no Docker environment in the list, the wizard uses `ubuntu:22.04`, and without the list, the node's generic image.

#### Form and JSON

- Nothing is lost: the form always works on the whole template, and fields it doesn't show (unknown step keys, Docker settings without a field…) are kept as they are. Opening a template and saving it untouched saves the same JSON, except that `name` becomes the first key.
- The JSON view is the panel's Monaco editor, loaded the first time it opens. It validates the template against its schema (underlining errors) and completes keys, operation types and their arguments.
- Half-typed JSON breaks nothing: the form keeps the last valid version and the toolbar says **The JSON is not valid**. Editing the form meanwhile discards the invalid JSON changes.

#### Errors and warnings

The form and the JSON check the template against the same schema while you type:

- **Errors**: what the node refuses or fails on (an empty required field, a number that isn't one, a step without `type`…). They show under each field and are announced to screen readers.
- **Warnings**: what the node accepts but probably not as meant. They don't block saving. For example: an undefined `${variable}` or a variable nothing uses, a whole number the node would turn into 0, conditional commands with no default, a stop with neither command nor signal code, no Docker supported environment, an incomplete keep-alive, or a **third-party image**: the project only uses its own images (`ghcr.io/aether-panel/*`) or official ones, from Docker Hub (such as `ubuntu` or `eclipse-temurin`) or from the vendor (such as `mcr.microsoft.com` or `steamcmd/steamcmd`).

The toolbar's status line sums up what is left (**No problems**, how many errors and warnings, or what blocks saving). With anything left it is a button that opens **Errors and warnings**: the whole list grouped by section, what blocks saving first; picking one opens its section and moves the focus to its field.

**Save** is off for a saved template without changes, or when there is an error that blocks saving: an empty or invalid name, a name another local template has, the local templates not being readable to check it (**Check again** retries), a step that is not an object, or broken JSON. With other errors, **Save** asks first (**Save anyway**). On save, the panel validates the template as always; if it refuses it, its message is shown.

#### Preview

**Preview** opens a side panel with the configuration step of the "Create server" wizard (resources, variables and ports), drawn with the same components and the same starting values as the real wizard. You can fill it in and press **Check**, which runs the wizard's checks and moves the focus to the first field to fix; **Reset** goes back to the template's values. Nothing is created and the template is not changed. With no node chosen, free ports and the node's memory are not checked.

#### Save and download

- **Save** sends the template with `PUT /api/templates/0/<name>`. Renaming an existing template saves a new one and leaves the old one as it was; changing only case or accents doesn't count as another name. After saving, the address becomes `?name=<name>`, so reloading keeps editing what was saved.
- **Download .json** downloads the template as it is in the editor as `<name>.json`, without saving it. It is off while the JSON is not valid.

### Community templates

Where they come from depends on `templates.url`:

**With `templates.url` set** (the Docker Compose file sets `https://templates.aetherpanel.es/templates.json`): the panel ignores any Git repository saved in the database and offers one repository called `community` (id `1`) built from that index. The index is a JSON object that maps each template name to the URL of its JSON file; relative URLs are resolved against the index's own address:

```json
{
  "paper": { "url": "minecraft/paper.json" },
  "terraria": { "url": "https://example.com/templates/terraria.json" }
}
```

The index and the templates it lists are kept in memory for 5 minutes: during that time, asking for the list downloads nothing again (10-second timeout per request when it does download). Reading a single template uses the cached index and downloads only that template. A template that fails to download or parse is skipped and logged; the incomplete list is then not cached, so the next request tries again. If `templates.url` changes, the cached copy of the old index is never served.

**Without `templates.url`**: the community tab lists the Git repositories saved with `POST /api/templates`. Each one is cloned (single branch) into `<daemon.data.cache>/template-repos/<id>` the first time it is read, and pulled again when the copy is older than 5 minutes. Expected layout: one folder per group, each with `<name>.json` files and optionally a `README.md` shown with the template; `data.json` files and hidden folders are skipped.

**Private repositories**: `POST /api/templates` accepts optional credentials that are saved with the repository and used for cloning and pulling. The API never returns them.

| Repository address | Fields | How they are used |
|---|---|---|
| `https://…` / `http://…` | `pat` (personal access token), or `username` and `password` | Basic authentication. With `pat`, the user is `username` or, when empty, `git` (GitHub and GitLab accept any user with a token; Bitbucket needs the real one). Without `pat`, `username` and `password` are used. |
| SSH (`git@host:…`, `ssh://…`) | `sshKey` (private key), `username`, `password` | The SSH key; the user is `username`, else the one in the URL, else `git`. `password` unlocks an encrypted key. The host must be in the panel's `known_hosts`. |

Without credentials no authentication is sent, which works for public repositories over HTTPS. Authentication errors never include the credentials: they only say which kind failed (for example `the SSH key of template repo <name> cannot be read`).

## Using it

### In the panel

1. Open **Templates** in the sidebar.
2. To add your own, either press **Upload** and choose one or more `.json` files, or press **New template**, pick how to start, fill in the editor's sections and press **Save**.
3. To change a local template, **⋯ → Edit** on its card. To start from any other, **⋯ → Duplicate**.
4. To start a server from any template, press **Use Template** on its card.

### Via the API

| Method and path | Scope | Purpose |
|---|---|---|
| `GET /api/templates` | `login` | List the repositories (including `Local`, id `0`) |
| `GET /api/templates/:repo` | `login` | List the templates of a repository (summaries) |
| `GET /api/templates/:repo/:name` | `login` | Read one template in full |
| `PUT /api/templates/0/:name` | `templates.local.edit` | Create or replace a local template (body: the template JSON) |
| `DELETE /api/templates/0/:name` | `templates.local.edit` | Delete a local template; refused (`409` `ErrTemplateInUse`) while a store product creates its servers with it and no other repository has a template of that name |
| `POST /api/templates/upload` | `templates.local.edit` | Upload local templates (`multipart/form-data`, field `files`, repeatable) |
| `POST /api/templates` | `templates.repo.create` | Add a Git repository: `{"name": "...", "url": "https://...", "branch": "main"}`, plus optional `pat`, `username`, `password` or `sshKey` for a private repository |
| `DELETE /api/templates/:repo` | `templates.repo.delete` | Remove a Git repository; refused (`409` `ErrTemplateRepoInUse`) while it is the only one with a store product's template |

Example upload:

```bash
curl -X POST https://panel.example.com/api/templates/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "files=@paper.json" -F "files=@velocity.json"
```

More details in the [API reference](../reference/11-api-reference.md).

## Requirements and configuration

| Key | Default | Environment variable | Effect |
|---|---|---|---|
| `templates.url` | empty (Docker Compose sets `https://templates.aetherpanel.es/templates.json`) | `SKYPANEL_TEMPLATES_URL` | Template index used as the `community` repository; when set, Git repositories in the database are ignored |
| `daemon.data.cache` | `cache` (inside `daemon.data.root`) | `SKYPANEL_DAEMON_DATA_CACHE` | Where Git repositories are cloned |

The panel needs outbound HTTPS to the index (or to the Git hosts). See [Configuration](../backend/02-configuration.md).

## Limits

- Community templates cannot be edited in place. To change one, **⋯ → Duplicate** and save the copy as a local template (with the same or another name).
- The editor has no version history or shared editing: if two people save the same template, the last save wins.
- A variable group's condition and its "Order" field are saved, but the panel doesn't use them yet.
- The condition warnings don't recognise prefixed CEL literals (`r"…"`, `b"…"`) or type names used without a call (`string`, `int`…): they flag them as undefined variables. No community template uses them.
- The credentials of a private repository can only be given when adding it through the API; to change them, remove the repository and add it again.
- There is no button to add or remove Git repositories in the web; it is done through the API. While `templates.url` is set, added repositories are not shown at all.
- With a `templates.url` index, changes published to the index take up to 5 minutes to show (the in-memory cache is lost when the panel restarts).
- The editor reads the template the way the browser does, so it rewrites numbers the browser can't hold as written: an integer above 2^53 (9007199254740992) loses its last digits and a number written as `1.0` is saved as `1`. Write long IDs (a Steam64 or Discord ID) as strings: `"76561198000000000"`.
- The editor compares template names ignoring case and accents in every database, as MariaDB does. On SQLite, which does tell them apart, the editor still treats a name that differs only in that way as taken (`Paper` when `paper` exists), and changing only case or accents doesn't rename the template.
- A template saved with `PUT` can be at most 2 MB, like an uploaded file; a larger one is refused with `413`.

## Troubleshooting

| What you see | Cause | Fix |
|---|---|---|
| **No community templates** | No index configured and no repository added, or the index could not be downloaded | Check `templates.url` and that the panel can reach it; look at the panel log for `Error reading template from …` |
| `file X is not a JSON template` / `template X has no install steps or run command` (`ErrInvalidTemplate`) | Upload of a wrong file | Upload a template JSON with `install` or `run.command` |
| **The JSON file isn't valid.** | **Import a .json** got invalid JSON | Fix the file |
| **Save** off and **Fix the marked errors to save.** | An error that blocks saving (empty, taken or invalid name, a step that is not an object) | Open **Errors and warnings** from the toolbar and pick the error to go to its field |
| **Couldn't read the local templates…** | The editor couldn't check whether the name is taken | **Check again** next to the name |
| **Couldn't save the template: …** | The panel refused the template when validating it | Fix what the message says (for example, a template with neither `install` nor `run.command`) |
| **Nothing to import** when opening `?import` | The imported file is kept only in that browser tab | Import it again from **New template → Import a .json** |
| `repo already exists with given name` (`ErrRepoExists`) | Adding a repository with a name already used | Use another name |
| `bad status 403 from …` in the log | The index host refused the download | Check the URL and the host's protection |

## See also

- [Core concepts: Templates](../general/concepts.md)
- [Operations](../backend/06-operations.md): the install steps a template can use.
- [Scopes reference](../general/scopes-reference.md)
