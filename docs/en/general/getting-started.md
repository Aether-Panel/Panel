# Getting Started

This guide goes from a clean Linux server to a first game server running in Aether Panel. Each step links to the page with the full details.

## 1. Check the requirements

| | Minimum |
|---|---|
| Operating system | Linux: Ubuntu 22.04/24.04 (and newer), Debian 12, Rocky/Alma/RHEL 9; also Arch, openSUSE Tumbleweed, RHEL 8 and Fedora |
| Architecture | amd64 or arm64 |
| Resources | 2 vCPU and 2 GB of RAM for the panel itself (or swap), plus whatever your game servers need |
| Access | root or `sudo` |
| Network | A public IP. A domain is optional but recommended (see step 4). |

Docker and Docker Compose v2 are installed by the installer if they are missing. The full list, with the systems tested, is in the [Installation guide](../setup/01-installation.md#system-requirements).

## 2. Install the panel

Run as root (or with `sudo`):

```bash
bash -c "$(curl -fsSL https://install.aetherpanel.es/install.sh)"
```

The installer checks the machine (architecture, RAM, disk, free ports), installs Docker if needed, clones the repository into `/opt/skypanel`, generates random database passwords, builds the image and starts the containers. It asks for:

- a **domain** or only the IP (with a domain it sets up Nginx and, if you want, a Let's Encrypt certificate);
- the **ports** (defaults: 8080 for the panel, 5657 for SFTP) and the **database** (the bundled MariaDB, or an external MySQL/MariaDB);
- the **administrator's email**. If you do not give a password, it generates one.

To install without questions, with a domain and HTTPS:

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --non-interactive \
  --domain panel.example.com --ssl --email admin@example.com --admin-email admin@example.com
```

Running the installer again offers to update, reinstall or uninstall. All of its options are in the [Installation guide](../setup/01-installation.md).

> There is no native (non-Docker) installation. The image is always built from the repository's `Dockerfile`.

## 3. Sign in

At the end the installer prints the panel's address and where the credentials are:

- **Address**: `https://panel.example.com` with a domain, otherwise `http://YOUR_IP:8080`.
- **Credentials**: a generated password is saved in `/etc/SkyPanel/credenciales.txt` (readable by root only), together with the database passwords.

```bash
sudo cat /etc/SkyPanel/credenciales.txt
```

Open the address, sign in with the administrator's email and password, and change the password in **Account Settings** if it was generated.

If the administrator account could not be created (for example the panel was not ready yet), create it by hand:

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin
```

It asks for the password (8 to 72 bytes). The [CLI reference](../reference/14-cli-commands.md) lists every command.

## 4. Secure the panel

Do this before giving access to anyone else:

1. **Use a domain with HTTPS.** A panel at `http://IP:8080` sends its session cookie to every port of that IP, including your customers' game servers. Point a domain at the server and either install with `--domain … --ssl` or enter `https://your-domain` in **Settings → General → Base URL**: the panel sets up Nginx, the certificate and the firewall, and publishes its own port on `127.0.0.1` only. Why and how: [Security](./security.md#expose-the-panel-with-a-domain-and-https).
2. **Set the Base URL** even without a domain (`http://YOUR_IP:8080`): password reset and sign-in links are built from it.
3. **Turn on 2FA** for the administrator in **Account Settings → Two-factor authentication**, and keep the recovery codes.
4. **Decide about sign-ups.** Public sign-up is on by default (`panel.registrationEnabled`). Turn it off in **Settings → General** if accounts come from you or from a billing system. The anti-bot check on the sign-in and sign-up forms is on by default ([Captcha / anti-bot](17-turnstile.md)).

The installer opens SSH, SFTP (5657), 80/443 and, when it is not bound to `127.0.0.1`, the panel port with ufw or firewalld. Game server ports need no rule: Docker publishes them itself. Do not open MariaDB (3307) or Adminer (8081); both listen on `127.0.0.1` only. See [Firewall and ports](./security.md#firewall-and-ports).

## 5. Check the node

The machine where the panel runs is already a node: the **local node** (`LocalNode`) appears in **Nodes** without any setup. Its address comes from the Base URL. You can run every server there.

To run servers on other machines, install the same image there as a node and register it in **Nodes**: see [Installation as a secondary node](../setup/01-installation.md) and [Dashboard and nodes](../features/dashboard-and-nodes.md).

## 6. Create your first server

Go to **Servers** and click **Add Server** (only administrators create servers from scratch; customers split their own servers with Splitter). You can also click **Use Template** on a template in **Templates** to open the wizard with that template already chosen. The wizard has three steps.

### Step 1: Environment

| Field | What to enter |
|-------|---------------|
| **Server name** | Any name; it is what users see. The server's ID is made from it plus a random suffix. |
| **Node** | Where the server runs. Each node shows its public address. |
| **Environment** | Always **Docker**: the panel's install only runs servers in containers (`docker.disallowHost`). There is nothing to choose. |
| **Users with access** | Optional. The **first** user picked becomes the server's **owner**; the others are added as members. Leave it empty for a server of your own. |

What the users get: the owner holds the full owner set (console, files, backups, tasks, sub-users…); the other users get the same customer permissions except backups. Neither can change limits, ports or the definition. See [Owner and sub-users](./scopes-reference.md#owner-and-sub-users).

### Step 2: Template

1. Pick the **template repository**: **Community** (the index at `templates.aetherpanel.es`, set by the Docker install) or **Local** (templates uploaded to your panel).
2. Pick the **template** (for example a Minecraft flavour). The wizard loads its details; wait until it finishes.

### Step 3: Configuration

**Resources.** CPU (percent of one core: 100 is one core, 200 two), RAM (MB), disk (MB) and disk priority. They start with the template's values (or 100 % CPU, 1024 MB, 10240 MB and 500). The wizard warns you, under the fields, when:

- the RAM is more than the node has (red), more than it has free right now, or less than the template recommends (amber);
- the node is low on disk or has less free disk than the limit you set.

**Template variables.** One field per variable the template defines (required ones are marked with `*`):

- The game **version** is a list of real releases (from Mojang or PaperMC) when the template downloads the game with them; `latest` stays available.
- For Minecraft, the **Java** image follows the version you pick (26.1+ → Java 25, 1.20.5–1.21.x → 21, 1.17–1.20.4 → 17, older → 8). You can still change it; a warning appears if it does not match.
- **Port** fields are checked against the node as you type: a port below 1024, one the panel uses, one another server has or one another container publishes is flagged, with a **Use** button for the next free port. Administrators also see who holds it.
- Before creating, values are validated: required fields, whole numbers within the template's limits, and the Minecraft **EULA**, which must be accepted (without it the server stops as soon as it starts).

**Extra ports.** **Add port** adds ports beyond the main one; they become `port2`, `port3`… and are published over TCP and UDP. They are checked against the node too.

Click **Create server**. If something is wrong, the wizard marks the field and moves the focus to it; nothing is created.

### What happens next

1. The panel creates the server on the node and, with `panel.servers.autoInstall` on (the default), **starts the installation right away**: the template downloads the game and its files. You do not need to press Install.
2. Open the server (the wizard says it is installing) and watch the progress in the **Console**. A first install can take a few minutes; the runtime images are usually already downloaded, because nodes prefetch them (`docker.prefetchImages`).
3. When the install finishes, click **Start**. If the server fails to start, the console says why (for example a missing EULA, a port in use or not enough memory), and a crash notice appears if it stops unexpectedly.
4. Connect with your game to the address shown on the server page (the node's address and the main port).

The game runs in its container as an unprivileged user (uid 988), with its own `/tmp`, a process cap and dropped capabilities. See [Game server containers](./security.md#game-server-containers).

## 7. Next steps

- Give access to someone: the server's **Users** tab ([Scopes reference](./scopes-reference.md)).
- Files and SFTP: the **File Manager** and **SFTP Access** tabs ([Core concepts: SFTP](./concepts.md#sftp)).
- Backups and scheduled tasks: [Backups](../features/backups.md), [Scheduled tasks](../features/scheduled-tasks.md).
- Databases for your servers: [Database hosts](../features/database-hosts.md).
- Email, Discord alerts and the AI assistant: [Email](15-email.md), [Discord](16-discord.md), [AI assistant](../features/ai-assistant.md).
- Selling servers from WHMCS or Paymenter: [Billing integration](../features/provisioning-and-sso.md).
- Keeping the panel updated: **Settings → Updates**, or the installer with `--update` ([Panel updates](../features/panel-updates.md)).
- Something not working: [Troubleshooting](../setup/troubleshooting.md) and the [FAQ](./faq.md).
