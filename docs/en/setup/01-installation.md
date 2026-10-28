# Installation Guide

This guide details the steps required to install Aether Panel on your server. We support multiple installation methods to suit different environments and needs.

> **Note:** Installation is **Docker only**. There is no native mode (standalone binaries). The installation script and the `docker-compose.yml` in the repository build the image from the `Dockerfile`.

## System Requirements

Before you begin, ensure your server meets the following minimum requirements:

*   **Operating System:** Linux. Tested with real installs on:
    *   Ubuntu 22.04, 24.04, 26.04 and 26.10
    *   Debian 12
    *   Rocky Linux 9 (and AlmaLinux / RHEL 9, which are equivalent)
    *   Arch Linux and openSUSE Tumbleweed (with Docker from the distribution's repositories)

    It also works on Rocky/Alma/RHEL 8 and Fedora. Ubuntu 20.04 and Debian 11 no longer get security support: the installer warns and they are not recommended.
*   **Architecture:** amd64 or arm64
*   **Resources (Minimum):** 2 vCPU, 2GB RAM
*   **Docker:** Docker Engine + Docker Compose v2 (the installer installs it if missing)
*   **Ports:** 8080 (Web), 5657 (SFTP) open in your firewall

---

## Method 1: Automatic Installation (Recommended)

Our automatic installation script handles Docker installation, repository download, image build, and service configuration.

Run the following command as root or a user with sudo privileges:

```bash
bash -c "$(curl -fsSL https://install.aetherpanel.es/install.sh)"
```

Alternatives (avoid process substitution):

```bash
# Automatic mode (no questions)
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash

# Download and run
curl -fsSL https://install.aetherpanel.es/install.sh -o /tmp/install.sh
sudo bash /tmp/install.sh
```

The installer will guide you through the process:
1.  System checks: architecture, RAM (2 GB or swap), disk space, free ports.
2.  Installation of Docker and Docker Compose (if not present), with Docker's GPG key verified.
3.  Cloning of the repository from `https://github.com/Aether-Panel/Panel.git` (branch `dev2.0`, or the one already installed).
4.  Random database passwords (in `.env`, readable by root only).
5.  Building the image and setting up ports, domain, SSL and the firewall (optional).
6.  The panel URL (used by password reset and sign-in links) and the administrator account.
7.  Starting Aether Panel, with a summary of its address and where the credentials are (`/etc/SkyPanel/credenciales.txt`).

Running it again on an existing install is safe: it keeps the data, the passwords and the administrator account.

### Managing the panel with the installer

Run without options, the installer says whether the panel is **not installed**, **installed but stopped** or **running** (with its branch and version) and offers:

| Option | What it does | Data |
|---|---|---|
| **Install** | Asks for a domain or IP, ports (defaults or your own) and the database (internal or external), and installs everything. | — |
| **Update** (`--update`) | Backs up the database and checks the copy, pulls the branch, rebuilds and restarts. If the new version does not start, it goes back to the previous one by itself. | Kept |
| **Reinstall** (`--reinstall`) | Offers a backup, asks you to type `REINSTALAR`, removes the install and installs again from scratch. | Deleted (an external DB is not) |
| **Uninstall** (`--uninstall`, `--uninstall-data`, `--uninstall-all`) | Removes the panel keeping data, deleting data, or everything. Offers a backup before deleting and checks nothing is left. | Depends on the option |

Backups go to `/var/backups/aether-panel`, outside the panel's folder. Only what belongs to the install is removed: the containers docker compose created from `/opt/skypanel`, the panel's game servers, their networks and image. An external database is never changed or deleted.

**External database.** The connection is checked before installing. If the database or the user do not exist, the installer can create them with a MySQL admin account (`--db-admin-user root --db-admin-pass …`, or the `AETHER_DB_ADMIN_PASSWORD` variable), which is not stored. The host cannot be `localhost`: inside the panel's container that is the container itself, so use the server's private IP.

### Useful options

```bash
# All options
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --help

# Unattended install with a domain and SSL
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --non-interactive \
  --domain panel.example.com --ssl --email admin@example.com --admin-email admin@example.com

# See what it would do, changing nothing
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --dry-run

# Update or uninstall (keeping the data)
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --update
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --uninstall
```

**Adding a domain later.** Point the domain at the server's IP (with or without Cloudflare's proxy) and enter it in **Settings → General → Base URL**, for example `https://panel.example.com`. When you save it, the `aether-domain` service the installer leaves on the server sets up Nginx, the Let's Encrypt certificate and the firewall within seconds, and the result shows under the field. With `http://` it is set up without a certificate; with an IP, Nginx goes back to answering on the IP. If the certificate fails (the domain does not point here yet, or port 80 is closed at the provider), the panel keeps working over HTTP and it is retried after an hour or when the URL changes.

The service is installed by installer 4.1.3 and later (also with `--update`). On an install without it, the same result is reached by hand:

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --domain panel.example.com --ssl
```

Without a domain, Nginx answers on port 80 for the IP and for any domain pointed at the server. With Cloudflare in "Full" SSL mode, Cloudflare connects on 443 and the certificate is needed (a URL with `https://`).

It can also set up Turnstile (`--turnstile-site-key`, `--turnstile-secret-key`), Cloudflare automatic subdomains (`--cf-dns-token`, `--cf-zone-id`, `--dns-base-domain`), S3 backups (`--s3-*`) and the CurseForge key; all of it can be changed later in Settings. Changing the ports (`--port`, `--sftp-port`) needs Docker Compose 2.24.4 or later. The installer's source is `tools/install/install.sh`.

---

## Method 2: Manual Installation via Docker

If you prefer full control over the process, clone the repository and use the included `docker-compose.yml`.

### 1. Clone the Repository

```bash
git clone -b dev2.0 https://github.com/Aether-Panel/Panel.git /opt/skypanel
cd /opt/skypanel
```

### 2. Configure the Environment

The `docker-compose.yml` uses variables with defaults (defining a `.env` is optional):

| Variable | Default | Usage |
|---|---|---|
| `DB_ROOT_PASSWORD` | `skypanel_secret` | MariaDB root password |
| `DB_DATABASE` | `skypanel` | Database name |
| `DB_USER` | `skypanel` | Database user |
| `DB_PASSWORD` | `skypanel_secret` | User password |

### 3. Build and Start

```bash
docker compose up -d --build
```

### 4. Create Administrator User

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --password 'admin123' --admin
```

> **Important:** Remember to change `admin@example.com` and `'admin123'` to your actual details.

---

## Method 3: Installation as a Secondary Node (Slave)

If you wish to set up an additional server that functions solely as a daemon (slave node) for hosting game servers, connecting to an existing Master Panel:

### Using Docker Compose

Add the following environment variables to the `skypanel` service configuration in your `docker-compose.yml`:

```yaml
    environment:
      # Turn off the web panel interface and the database
      - SKYPANEL_PANEL_ENABLE=false
      # Configure the Master Panel's public key for token validation
      # Make sure to replace <MASTER-PANEL-IP> with the real IP or domain
      - SKYPANEL_TOKEN_PUBLIC=http://<MASTER-PANEL-IP>:8080/auth/publickey
```

### Without Docker (Manual Installation)

> **There is no native mode.** If you installed the node with the automatic installer, the equivalent configuration is applied via environment variables. In a manual deployment without compose, edit the `/etc/SkyPanel/config.json` file inside the container and modify the `panel` and `token` sections:

```json
  "panel": {
    "enable": false
  },
  "token": {
    "public": "http://<MASTER-PANEL-IP>:8080/auth/publickey"
  }
```

Restart the container with `docker restart skypanel`.

> **Clarification Note:** Even if you disable the Panel, the process will still listen on the HTTP port (default `8080`) because the Daemon requires this port for its REST API (to communicate with the Master). If you attempt to access it via a web browser, you will see a 404 error; this is completely normal and indicates that the Node is functioning correctly.

---

## Post-Installation

Once the main panel is installed, it will be accessible at:
`http://<YOUR-SERVER-IP>:8080`

Log in with the administrator credentials created during the installation process.