# Templates

A template is the JSON blueprint a server is created from: its variables, install steps, run command, environment and requirements. The **Templates** page lists the templates the panel can install and lets administrators add their own. This page covers the page, where templates come from and how to add them; the template format itself is described in [Core concepts: Templates](../general/concepts.md) and [Operations](../backend/06-operations.md).

## What it does

The page (**Template Management**, `/templates`) has two tabs:

- **Local**: templates stored in this panel's database. They can be created, edited, uploaded and deleted here.
- **Community**: templates read from an outside source, either the template index configured in `templates.url` or the Git repositories added through the API. They are read only.

Each card shows the template's logo, name, category and a **Use Template** button that opens the create-server wizard with that template already chosen (`/servers/?create=true&template=<name>&repo=<id>`).

Buttons, by permission:

| Button | Needs |
|---|---|
| **Upload** (one or more `.json` files) | `templates.local.edit` |
| **Create Template** | `templates.repo.create` to see it; saving needs `templates.local.edit` |
| **Edit** / **Delete** on a local template | `templates.local.edit` |

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

The **Create Template** / **Edit template** dialog has three tabs that stay in sync: **General** (**Name (unique ID)**, **Display name**, **Category (type)**), **Run Configuration** (**Start command**, **Stop command**, **Working directory**) and **JSON** (the whole template, with **Load from JSON file**). Saving sends the JSON with `PUT /api/templates/0/<name>`; a template with the same name is replaced.

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
2. To add your own, either press **Upload** and choose one or more `.json` files, or press **Create Template**, fill in the fields (or paste the JSON) and press **Save template**.
3. To start a server from any template, press **Use Template** on its card.

### Via the API

| Method and path | Scope | Purpose |
|---|---|---|
| `GET /api/templates` | `login` | List the repositories (including `Local`, id `0`) |
| `GET /api/templates/:repo` | `login` | List the templates of a repository (summaries) |
| `GET /api/templates/:repo/:name` | `login` | Read one template in full |
| `PUT /api/templates/0/:name` | `templates.local.edit` | Create or replace a local template (body: the template JSON) |
| `DELETE /api/templates/0/:name` | `templates.local.edit` | Delete a local template |
| `POST /api/templates/upload` | `templates.local.edit` | Upload local templates (`multipart/form-data`, field `files`, repeatable) |
| `POST /api/templates` | `templates.repo.create` | Add a Git repository: `{"name": "...", "url": "https://...", "branch": "main"}`, plus optional `pat`, `username`, `password` or `sshKey` for a private repository |
| `DELETE /api/templates/:repo` | `templates.repo.delete` | Remove a Git repository |

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

- Community templates cannot be edited in place. To change one, open it, copy its JSON into **Create Template** and save it as a local template (with the same or another name).
- The credentials of a private repository can only be given when adding it through the API; to change them, remove the repository and add it again.
- There is no button to add or remove Git repositories in the web; it is done through the API. While `templates.url` is set, added repositories are not shown at all.
- With a `templates.url` index, changes published to the index take up to 5 minutes to show (the in-memory cache is lost when the panel restarts).
- **Create Template** is shown to holders of `templates.repo.create`, but saving needs `templates.local.edit`.

## Troubleshooting

| What you see | Cause | Fix |
|---|---|---|
| **No community templates** | No index configured and no repository added, or the index could not be downloaded | Check `templates.url` and that the panel can reach it; look at the panel log for `Error reading template from …` |
| `file X is not a JSON template` / `template X has no install steps or run command` (`ErrInvalidTemplate`) | Upload of a wrong file | Upload a template JSON with `install` or `run.command` |
| **The JSON file isn't valid.** | **Load from JSON file** got invalid JSON | Fix the file |
| **The template must have a "name" field.** | Saving from the dialog without a name | Fill **Name (unique ID)** |
| `repo already exists with given name` (`ErrRepoExists`) | Adding a repository with a name already used | Use another name |
| `bad status 403 from …` in the log | The index host refused the download | Check the URL and the host's protection |

## See also

- [Core concepts: Templates](../general/concepts.md)
- [Operations](../backend/06-operations.md): the install steps a template can use.
- [Scopes reference](../general/scopes-reference.md)
