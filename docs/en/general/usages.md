# Panel Usage

A tour of the web interface: what each part of the menu is for, who sees it, and where its detailed guide is. For the ideas behind it (nodes, owners, templates…) see [Core concepts](./concepts.md); for every feature in depth, the [feature guides](../features/README.md).

## Layout

- **Sidebar**: the menu. It can be collapsed to an icon rail and is a slide-out menu on phones. Each entry only shows to users with the permission it needs.
- **Quick search**: `Ctrl+K` (`⌘K` on macOS) to jump to a page or a server.
- **Account menu**: **Account Settings** and sign out.

| Menu entry | Shown with | What it is for |
|------------|------------|----------------|
| **Home** | Everyone | Dashboard: an overview of the installation for operators, the list of their servers for everyone else. |
| **Servers** | `login` | The servers you own, are a member of or administer. |
| **Users** | `users.info.search` | Accounts, their role and their permissions. |
| **Roles** | `admin` | Roles and the panel scopes they give. |
| **Nodes** | `nodes.view` | The machines that run servers. |
| **Database Hosts** | `admin` | MySQL/MariaDB servers for server databases. |
| **Templates** | `templates.view` | Template repositories and local templates. |
| **Settings** | `settings.edit` | Panel-wide settings. |

Installed extensions can add their own pages to the menu and tabs to every server ([Extensions](../reference/15-extensions.md)).

## Home

Users with `nodes.view` or `users.info.search` get an overview: active servers, overall health (30-day availability), live network traffic, low-disk alerts, average CPU, memory and storage, panel information, the nodes' network history (1 h, 6 h, 24 h) and the latest sign-ins. Everyone else gets **My Servers**, one card per server they can open. See [Dashboard and nodes](../features/dashboard-and-nodes.md).

## Servers

The list of servers you have access to. Administrators create servers with **Add Server** (the wizard is described step by step in [Getting started](./getting-started.md#6-create-your-first-server)).

### Server page

The tabs depend on your permissions on that server (the exact rules are in the [Scopes reference](./scopes-reference.md#what-each-person-sees-on-the-web)):

| Tab | What you do there | Guide |
|-----|-------------------|-------|
| **Console** | Live output, commands (with `server.console.send`), start / restart / stop / kill. It can be expanded over the page; a crash shows a notice and a crash report. | [Crash reports](../features/crash-reports.md), [AI assistant](../features/ai-assistant.md) |
| **Overview** | Address, version, players, and CPU, memory and network charts. | [Server overview and statistics](../features/server-statistics.md), [Game version](../features/game-versions.md) |
| **File Manager** | Browse, edit, upload, compress and extract files; file history. | [File history](../features/file-history.md) |
| **Settings** | Name, variables the template opens to you, auto start and restart, port notes and primary port, notifications, subdomain. | [Server settings](../features/server-settings.md), [Notifications](../features/server-notifications.md), [Subdomains](../features/subdomains.md) |
| **Properties** (beta) | `server.properties` editor for Minecraft: Java Edition. | [Server properties](../features/server-properties.md) |
| **Users** | The owner and sub-users, and their permissions. | [Scopes reference](./scopes-reference.md#owner-and-sub-users) |
| **Database** | Databases on the database hosts this server may use. | [Database hosts](../features/database-hosts.md) |
| **Backups** | Create, download, restore and delete backups. | [Backups](../features/backups.md) |
| **Tasks** | Scheduled tasks (commands, power actions, backups). | [Scheduled tasks](../features/scheduled-tasks.md) |
| **Activity** | Who did what on the server. | [Activity log](../features/activity-and-access-log.md) |
| **SFTP Access** | The SFTP address and username (`email#serverID`). | [Core concepts: SFTP](./concepts.md#sftp) |
| **Migration** | Move the server to another Aether Panel installation (administrators). | [Server transfers](../features/server-transfers.md) |
| **Splitter** | Split the server into sub-servers (owner or administrator). | [Sub-servers](../features/sub-servers.md) |
| **Plugins**, **Modpacks** | Install plugins, or mods on Fabric, Forge and NeoForge servers (from Modrinth), and modpacks (Modrinth, CurseForge). | [Modpacks and plugins](../features/modpacks-and-plugins.md) |
| **Map** | World map for Minecraft: Java Edition. | [World map](../features/world-map.md) |
| **Administration** | Reinstall, and for administrators: transfer to another node, definition, suspension, cloning, deletion. | [Server transfers](../features/server-transfers.md), [Cloning](../features/cloning.md) |

## Users and roles

**Users** lists accounts with their role. With `users.info.edit` you create, edit and delete accounts; with `users.perms.edit` you change their panel permissions. Staff cannot edit accounts with powers they lack ([Editing other accounts](./scopes-reference.md#editing-other-accounts)). Only administrators pick a user's role.

**Roles** (administrators) groups panel scopes under a name. The panel creates "Administrador" and "Usuario"; a role's server scopes have no effect on servers. See [Roles](./scopes-reference.md#roles).

Server access is given on the server itself, in its **Users** tab, not here.

## Nodes

**Nodes** lists the local node and the remote nodes with their state and resources. Administrators add remote nodes (`nodes.create`), edit them and see their deployment credentials (`nodes.deploy`). See [Core concepts: Nodes](./concepts.md#nodes) and [Dashboard and nodes](../features/dashboard-and-nodes.md).

## Database hosts

**Database Hosts** (administrators) holds the MySQL/MariaDB servers where server databases are created. **Add Host** opens a three-step wizard:

1. **Credentials**: the username and password of a MySQL account that can create users and databases.
2. **Instructions**: the SQL to create that account, with buttons to copy it:

   ```sql
   CREATE USER 'aetheruser'@'%' IDENTIFIED BY 'your_strong_password';
   GRANT ALL PRIVILEGES ON *.* TO 'aetheruser'@'%' WITH GRANT OPTION;
   ```

   The user is created for `'%'` because a panel in Docker connects from its container's IP. If the database is on another machine, it also shows how to allow external connections (`bind-address=0.0.0.0` under `[mysqld]` in `my.cnf`, then restart MySQL and open port 3306 to the panel).
3. **Configuration**: **Host Name** (a label), **Connection Host** (IP or domain the panel connects to), **Port** (3306), **Database Limit** (empty or 0 for no limit) and **Linked Nodes** (optional). **Test connection** checks host, username and password before saving.

A host linked to a node is only offered to servers on that node. Hosts can be edited later (leave username and password empty to keep them) and deleted, but only once they have no databases left.

Then, in a server's **Database** tab, anyone with `server.data.edit` creates a database: the panel names it `s_<serverId>_<name>`, creates a user with a random name and password for it alone, and shows the connection details. Game servers reach the panel's own MariaDB as `mysql:3306`. Full details: [Database hosts](../features/database-hosts.md).

## Templates

**Templates** shows the repositories (**Community** and **Local**) and their templates. **Use Template** opens the new-server wizard with that template. With `templates.local.edit` you upload, create, edit and delete local templates. See [Templates](../features/templates.md).

## Settings

**Settings** (`settings.edit`) has these tabs: **General** (company name, Base URL, master node IP, notification language, Gemini key, CurseForge key, sign-ups, anti-bot check and Turnstile, experimental features), **Discord Notifications**, **Mail**, **API keys** (billing), **Products** (plans for billing), **Updates**, **Off-site backups** (S3), **Domains** (Cloudflare subdomains) and **Extensions**. See [Panel settings](../features/panel-settings.md).

## Account Settings

Every user's own page: account details (username, email), password, two-factor authentication, SSH keys for SFTP, OAuth2 clients (with `self.clients`) and preferences (language: English or Spanish). See [Security](./security.md#sign-in-and-sessions).
