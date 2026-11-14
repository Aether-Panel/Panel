# Frequently Asked Questions

## General

### What is Aether Panel?

A self-hosted panel to run game servers. You install it on your own server; it runs each game server in its own Docker container and gives you, your staff and your customers a web interface to manage them: console, files, backups, scheduled tasks, databases, SFTP and more. The backend is written in Go (Gin and GORM) and the web interface in Astro and React. See [Core concepts](./concepts.md).

### Is it free?

The code is published under the Apache 2.0 license. The panel has a license activation (**Settings**, `panel.license.key`), but nothing in the panel reads the stored license today: no feature is enabled or blocked by it. See [License system](14-license.md).

### What can I run on it?

Anything a template describes and Docker can run. The Docker install reads the community templates from `https://templates.aetherpanel.es/templates.json`; you can also upload your own templates to the **Local** repository (`templates.local.edit`). The new-server wizard has extra help for Minecraft: real version lists from Mojang and PaperMC and the matching Java image. Modpacks, the world map and the properties editor are for Minecraft: Java Edition servers.

### Do I need Docker?

Yes. The panel itself is installed with Docker (there is no native install), and game servers run only in Docker containers: the Docker install turns off the environments that run on the host (`docker.disallowHost`). The installer installs Docker if it is missing.

### Which languages does the interface speak?

English (the default) and Spanish. Each user picks their language in **Account Settings**; it is remembered in the browser.

What the panel sends outside its own pages (Discord embeds, each server's Discord, Telegram and webhook alerts, the panel's Discord reports and the events extensions receive) uses one language for everyone, chosen by an administrator in **Settings → General → Notifications** (Spanish by default; remote nodes receive the panel's value with every status poll, unless a node's own config sets one). See [Discord and notifications](16-discord.md#notification-language). API errors are in English; those that carry an error code are shown translated by the web interface, as are the steps of a [panel-to-panel migration](../features/server-transfers.md).

## Installing and running

### How do I install it?

Run the installer as root: `bash -c "$(curl -fsSL https://install.aetherpanel.es/install.sh)"`. See [Getting started](./getting-started.md) and the [Installation guide](../setup/01-installation.md).

### Where is the administrator password?

If the installer generated it, in `/etc/SkyPanel/credenciales.txt` (readable by root only), next to the database passwords.

### I forgot the administrator password. What now?

If email is configured, use **Forgot password** on the sign-in page. Otherwise, on the server:

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user edit
```

It asks for the user and lets you change the password, the email or the username, give or take administrator rights, or remove 2FA. See the [CLI reference](../reference/14-cli-commands.md).

### I lost my phone and my 2FA recovery codes.

An administrator removes 2FA from your account with `SkyPanel user edit` → **Remove 2FA** (command above). Then sign in with the password and turn 2FA on again.

### Which database does the panel use?

The Docker install uses its own MariaDB 10.11 container (`skypanel-mysql`). The installer can use an external MySQL/MariaDB instead; it checks the connection and can create the database and user. The binary also supports SQLite (its default outside Docker) and PostgreSQL (`panel.database.dialect`).

### Where does the panel keep its data?

In `/opt/skypanel/storage` on the host: `mysql-data` (the database), `skypanel-config` (the config file with the panel's keys), `skypanel-data` (game servers and backups) and `skypanel-logs`. The installer's backups go to `/var/backups/aether-panel`.

### How do I update?

From **Settings → Updates** (it only offers a version once GitHub's checks on it have passed, and it also updates remote nodes), or with the installer: `curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --update`. The installer backs up the database first and goes back to the previous version if the new one does not start. See [Panel updates](../features/panel-updates.md).

### Can I manage several machines?

Yes. The machine with the panel is the local node; other machines run the same image as nodes and are registered in **Nodes**. See [Core concepts: Nodes](./concepts.md#nodes).

### Should I use a domain?

Yes, for any panel with customers. A panel at `http://IP:8080` sends its session cookie to every port of that IP, including customers' game servers, and in clear text. With a domain the installer sets up HTTPS and publishes the panel's port on `127.0.0.1` only. See [Security](./security.md#expose-the-panel-with-a-domain-and-https).

### Which ports must be open?

80 and 443 (Nginx), 5657 (SFTP) and, only without a domain, 8080 (the panel). Game server ports are published by Docker without firewall rules. MariaDB (3307) and Adminer (8081) listen on `127.0.0.1` only and must stay closed. See [Firewall and ports](./security.md#firewall-and-ports).

## Servers

### Who can create servers?

Administrators, from **Servers → Add Server**. Customers cannot create servers from scratch; the owner of a server can split it into smaller ones with **Splitter**, using the resources of their own plan. Billing systems create servers through the [provisioning API](../features/provisioning-and-sso.md).

### Why is the environment always "Docker" in the wizard?

Because the panel's install only runs game servers in containers (`docker.disallowHost`). The host environments are refused when a server is created, installed or started.

### Do I have to press Install after creating a server?

No. With `panel.servers.autoInstall` on (the default) the installation starts as soon as the server is created; follow it in the console, then press **Start**.

### My server does not start. Where do I look?

In its **Console**: the panel writes why a start failed. Common reasons: the Minecraft EULA not accepted, a port already in use, the disk limit reached (delete files first), not enough memory, or an image that is still downloading. The new-server wizard checks ports, memory and disk before creating the server to avoid most of these.

### Why can the owner not change the RAM, CPU, disk or ports?

Those are what the customer pays for, so only an administrator (`server.data.edit.admin`) changes them, even when the template marks those variables as editable. The owner can pick which assigned port is the primary one and label the ports. See [Variables only an administrator changes](./scopes-reference.md#variables-only-an-administrator-changes).

### Why are my server's files owned by user 988?

Game servers run as an unprivileged user, uid:gid `988:988` (`docker.serverUser`), not as root. The panel gives the server's files to that user before each start. Over SFTP and in the file manager this makes no difference. See [Game server containers](./security.md#game-server-containers).

### How do game servers reach a MySQL database?

Create a database in the server's **Database** tab (it needs a database host, added by an administrator in **Database Hosts**). The panel creates the database and a user with a random password and shows the connection details. The panel's own MariaDB is reachable from game servers as `mysql:3306`, because game servers and MariaDB share the `skypanel-servers` Docker network. Containers are created again on every start, so a server picks up network changes on its next start. See [Database hosts](../features/database-hosts.md).

### How do I give someone access to my server?

In the server's **Users** tab, add their email and tick the permissions. If they have no account, they get an email to choose a password (only administrators can invite people without an account while sign-ups are closed). See [Sub-users](./scopes-reference.md#sub-users).

### Can I move a server to another node or another panel?

Yes. An administrator moves it between nodes of the same panel from the **Administration** tab, and to another Aether Panel installation from the **Migration** tab (federated transfer). Between nodes, the server's sub-servers, its scheduled tasks and the backups kept on the old node move with it; a transfer that fails is undone, and one cut short by a panel restart is undone (or finished, if the panel already pointed at the new node) when the panel starts again. See [Server transfers](../features/server-transfers.md).

### Is there a limit on backups?

Each server's users can keep up to `panel.backups.maxPerServer` backups on the node (10 by default; administrators are not limited). Those kept only in the S3 bucket (periodic copies, and task copies already deleted from the node) do not count. Delete an old one to make another. See [Backups](../features/backups.md).

## Privacy and integrations

### Does the panel contact outside services?

Only for features you use or configure. The browser loads user avatars from Kodama (`api.kodama.sh`, with a one-way hash of the account as the seed, never the email); the panel asks GitHub's API for updates (once per browser session when someone with `settings.edit` opens the panel, and in **Settings → Updates**), and talks to the template index, Mojang/PaperMC, Modrinth or CurseForge when you install or search for game files. Gemini (AI), Cloudflare (subdomains, Turnstile), Discord/Telegram webhooks, email providers and S3 storage are only contacted once configured. The full list is in the [Privacy policy](./privacy.md#external-services-the-panel-contacts).

### What does the AI assistant send to Google?

When a user asks for an analysis or chats with the assistant, the panel sends the server's console lines (and crash details) to Google's Gemini API with the key set in `panel.settings.geminiApiKey`. Nothing is sent without a key or without that action. See [AI assistant](../features/ai-assistant.md).

### Does the panel have an API?

Yes. The REST API is under `/api` and takes the session cookie or `Authorization: Bearer <token>`. Administrators can browse it at `/swagger/`. Billing systems use `/api/v1` with API keys. See the [API reference](../reference/11-api-reference.md).

### What is the Gatus file in the repository?

`data/gatus/config.yaml` is an example configuration for [Gatus](https://github.com/TwiN/gatus), an external uptime monitor. The panel does not start or use Gatus; its own uptime history is described in [Uptime tracking](13-uptime.md). The example listens on `127.0.0.1:8081`, which the Docker install uses for Adminer, so change the port if you run it on the same machine.

## Help

### Where can I get help?

Read [Troubleshooting](../setup/troubleshooting.md) first. Bugs and feature requests go to the GitHub repository (`Aether-Panel/Panel`); community help is on the project's Discord.
