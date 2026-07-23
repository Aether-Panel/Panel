# Installation Guide

This guide details the steps required to install Aether Panel on your server. We support multiple installation methods to suit different environments and needs.

## System Requirements

Before you begin, ensure your server meets the following minimum requirements:

*   **OS:** Linux (Ubuntu 20.04+, Debian 11+, CentOS 8+)
*   **Architecture:** amd64 or arm64
*   **Resources (Minimum):** 2 vCPU, 2GB RAM
*   **Ports:** 8080 (Web), 5657 (SFTP) open in your firewall

---

## Method 1: Automatic Installation (Recommended)

Our automatic installation script handles user configuration, dependency installation, and systemd service setup.

Run the following command as root or a user with sudo privileges:

```bash
bash <(curl -s https://install.aetherpanel.com/install.sh)
```

The installer will guide you through the process:
1.  Dependency verification.
2.  Docker installation (if not present).
3.  Downloading the SkyPanel binary.
4.  Creating the initial administrator user.
5.  Starting the SkyPanel service.

---

## Method 2: Manual Installation

If you prefer full control over the installation process or your distribution is not compatible with the script, follow these steps.

### 1. Prepare Environment

Create the system user and necessary directories:

```bash
# Create unprivileged user
useradd -r -m -d /var/lib/SkyPanel -s /bin/false skypanel

# Create directory structure
mkdir -p /var/lib/SkyPanel
mkdir -p /etc/SkyPanel
mkdir -p /var/log/SkyPanel
```

### 2. Download and Install

Download the latest stable release from our releases page:

```bash
# Example for Linux AMD64
wget https://github.com/aetherpanel/aetherpanel/releases/latest/download/SkyPanel_linux_amd64 -O /usr/local/bin/SkyPanel
chmod +x /usr/local/bin/SkyPanel
```

### 3. Initial Configuration

Enable the service and create the first admin user:

```bash
# Add admin user
/usr/local/bin/SkyPanel user add --admin

# Enable service (if using systemd)
/usr/local/bin/SkyPanel runservice
```

---

## Method 3: Docker Installation

To run Aether Panel in an isolated container, use Docker.

### Direct Execution

```bash
docker run -d \
  --name skypanel \
  -p 8080:8080 \
  -p 5657:5657 \
  -v skypanel-config:/etc/SkyPanel \
  -v skypanel-data:/var/lib/SkyPanel \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --restart=always \
  aetherpanel/skypanel:latest
```

### Docker Compose

Save the following to a `docker-compose.yml` file:

```yaml
version: '3'
services:
  skypanel:
    image: aetherpanel/skypanel:latest
    ports:
      - "8080:8080"
      - "5657:5657"
    volumes:
      - ./config:/etc/SkyPanel
      - ./data:/var/lib/SkyPanel
      - /var/run/docker.sock:/var/run/docker.sock
    restart: always
```

Start the service with:
```bash
docker-compose up -d
```

### Create Admin User (Docker)

Once the container is running, execute this command to create your admin account:

```bash
docker exec -it skypanel /usr/local/bin/SkyPanel user add --name admin --email admin@example.com --password 'admin123' --admin
```

> **Important:** Remember to change `admin@example.com` and `'admin123'` to your actual details.

---

## Post-Installation

Once installed, the panel will be accessible at:
`http://<YOUR-SERVER-IP>:8080`

Log in with the administrator credentials created during the installation process.
