# Getting Started - Complete Guide

## Welcome to Aether Panel

This guide will take you step by step from installation to having your first server running. Follow each section in order for a successful setup.

## System Requirements

Before starting, make sure your system meets the following requirements:

It is important to note that Aether Panel requires at least 2 CPU vcores, 2GB RAM, and 5GB storage just to function correctly. These requirements do not cover the usage of the services it will manage in the panel.

- **cardTitle**: Hardware and Software
- **cardTitle2**: Minimum
- Operating System: Linux (Ubuntu 20.04+, Debian 11+, CentOS 8+).
- Architecture: amd64 or arm64.
- Virtualization: KVM, OpenVZ, LXC or dedicated server (for nodes).
- Software: Docker and Docker Compose (v20.10.0+, optional).
- Hardware: 2 CPU vcores, 2GB RAM, 5GB storage (minimum).

- **cardTitle3**: Recommended
- Operating System: Linux (Ubuntu 20.04+, Debian 11+, CentOS 8+).
- Architecture: amd64 or arm64.
- Virtualization: KVM, OpenVZ, LXC or dedicated server (for nodes).
- Software: Docker and Docker Compose (v20.10.0+, optional).
- Hardware: 2 CPU vcores, 4GB RAM, 20GB storage.

### Required Ports

Make sure the following ports are available:

- 8080/TCP: Web panel (HTTP/HTTPS) - Required
- 5657/TCP: SFTP for file transfer - Required
- 22/TCP: SSH for administration - Recommended to change to non-standard port

### Verify Requirements

Before installing, verify that you have root or sudo access:

Verify that ports are free:

If any port is in use, stop the service or change the port in the configuration.

- **code1**: sudo whoami
- **code2**: # Check port 8080
sudo netstat -tuln | grep 8080

# Check port 5657
sudo netstat -tuln | grep 5657
## Step-by-Step Installation

### Installation Methods

Aether Panel can be installed in three different ways. Choose the one that best suits your needs:

- Automatic Installation with Script (Recommended): The easiest and fastest method
- Docker Installation: Ideal for containerized environments
- Manual Installation: For advanced users who want full control

- **linuxTitle**: Method 1: Automatic Installation with Script
- **linuxP1**: This is the most recommended method. The script detects your operating system and installs everything automatically.
- **linuxP2**: Run the following command as root or with sudo to start the installation:
- **linuxCode**: bash <(curl -s https://install.aetherpanel.es/install.sh)
- **linuxP3**: Or download the script first and then run it:
- **linuxCode2**: # Download the script
wget https://install.aetherpanel.es/install.sh

# Give execution permissions
chmod +x install.sh

# Run as root
sudo bash install.sh
### Interactive Process

During installation, the script will ask you:

- Installation type: With Docker or without Docker?
- Domain or IP: Do you want to use a domain or just IP?
- SSL: Do you want to configure SSL with Let's Encrypt? (only if using domain)
- Ports: Do you want to change the default ports? (optional)

### The installation script will perform the following steps:

- Automatically detect your operating system (Ubuntu, Debian, Fedora, CentOS, RHEL)
- Install all necessary dependencies (Go, Node.js, Yarn, etc.)
- Configure the firewall and open necessary ports
- Clone the repository from GitHub
- Compile the frontend and backend
- Configure Nginx as reverse proxy
- Create the systemd service for automatic execution
- Allow you to configure domain and SSL with Let's Encrypt

- **dockerTitle**: Method 2: Docker Installation
- **dockerP1**: If you prefer to use Docker, you can deploy Aether Panel with Docker Compose. This method is ideal if you are already familiar with Docker.
### Prerequisites

Make sure you have Docker and Docker Compose installed:

- **checkDocker**: # Verify Docker
docker --version

# Verify Docker Compose
docker compose version
#### If you don't have Docker installed:

- **ubuntu**: # Ubuntu/Debian
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
- **fedora**: # Fedora/RHEL/CentOS
sudo dnf install -y docker docker-compose-plugin
sudo systemctl enable docker
sudo systemctl start docker
### Steps to install with Docker:

#### 1. Clone the repository

This will download all the panel source code.

```text
git clone https://github.com/Aether-Panel/Panel.git
cd Panel
```

#### 2. Configure database (optional)

If you want to use external MySQL, edit config.docker.json:

If you use the MySQL from docker-compose.yml, you don't need to change anything.

```text
{
  "panel": {
    "database": {
      "dialect": "mysql",
      "url": "user:password@tcp(IP:3306)/database?charset=utf8mb4&parseTime=true"
    }
  }
}
```

#### 3. Build the Docker image

This process may take several minutes the first time, as it downloads dependencies and compiles the panel.

```text
docker compose build
```

#### 4. Start the containers

The -d flag runs the containers in the background (detached mode).

```text
docker compose up -d
```

#### 5. Verify that containers are running

You should see the 'aetherpanel' and 'aetherpanel-mysql' containers (if you configured MySQL) with 'Up' status.

```text
docker compose ps
```

#### 6. View logs (optional)

Press Ctrl+C to exit the logs.

```text
docker compose logs -f
```

### Quick option: Use pre-built image

If you prefer not to compile from source, you can use the pre-built Docker image:

This command will download the latest image and run the panel with persistent data.

```text
docker run -d \
  --name skypanel \
  -p 8080:8080 \
  -p 5657:5657 \
  -v skypanel-config:/etc/SkyPanel \
  -v skypanel-data:/var/lib/SkyPanel \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --restart=always \
  ghcr.io/aether-panel/panel:latest
```

### Simplified Docker Compose option

You can also use this simplified docker-compose.yml:

Start the service with: docker compose up -d

```text
services:
  skypanel:
    image: ghcr.io/aether-panel/panel:latest
    ports:
      - "8080:8080"
      - "5657:5657"
    volumes:
      - ./config:/etc/SkyPanel
      - ./data:/var/lib/SkyPanel
      - /var/run/docker.sock:/var/run/docker.sock
    restart: always
```

### Create Admin User (Docker)

Once the container is running, run this command to create your admin account:

Remember to change admin@example.com and admin123 with your actual data.

```text
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --password 'admin123' --admin
```

### Post-Installation

Once installed, the panel will be accessible at:

Log in with the admin credentials created during the installation process.

```text
http://<YOUR-SERVER-IP>:8080
```

### Docker Configuration

The docker-compose.yml includes:

- MySQL/MariaDB service for the database
- Aether Panel service with all necessary ports
- Persistent volumes for data and configuration
- Health checks to verify service status

- **nativeTitle**: Method 3: Manual Installation (Without Docker)
- **nativeP1**: To install manually without Docker, you need to compile the panel yourself. This method gives you full control over the process.
### Install Dependencies Manually

#### Ubuntu/Debian

```text
# Update system
sudo apt update && sudo apt upgrade -y

# Install basic dependencies
sudo apt install -y git build-essential curl wget sqlite3

# Install Go 1.25.0
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify Go
go version

# Install Node.js 22
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs

# Install Yarn
sudo npm install -g yarn

# Verify installations
node --version
yarn --version
```

#### Fedora/RHEL/CentOS

```text
# Update system
sudo dnf update -y

# Install basic dependencies
sudo dnf install -y git gcc make curl wget sqlite

# Install Go 1.25.0
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Install Node.js 22
curl -fsSL https://rpm.nodesource.com/setup_22.x | sudo bash -
sudo dnf install -y nodejs

# Install Yarn
sudo npm install -g yarn
```

### Manual Compilation Process:

#### 1. Clone the repository

```text
git clone https://github.com/Aether-Panel/Panel.git
cd Panel
```

#### 2. Compile the frontend

This will compile the web interface (Astro + React).

```text
cd client/frontend
yarn install
yarn build
```

#### 3. Compile the backend

This will create the executable binary 'SkyPanel'.

```text
cd ../..
go mod download
go build -o SkyPanel ./cmd/panel
```

#### 4. Create directory structure

```text
sudo mkdir -p /etc/SkyPanel
sudo mkdir -p /var/lib/SkyPanel
sudo mkdir -p /var/log/SkyPanel
sudo mkdir -p /var/www/aetherpanel
```

#### 5. Copy compiled files

```text
sudo cp -r client/frontend/dist/* /var/www/aetherpanel/
sudo cp SkyPanel /usr/sbin/
sudo chmod +x /usr/sbin/SkyPanel
```

#### 6. Create configuration file

```text
sudo tee /etc/SkyPanel/config.json > /dev/null <<EOF
{
  "logs": "/var/log/SkyPanel",
  "panel": {
    "database": {
      "dialect": "sqlite3",
      "url": "file:/var/lib/SkyPanel/database.db?cache=shared"
    },
    "web": {
      "files": "/var/www/aetherpanel"
    }
  },
  "daemon": {
    "data": {
      "root": "/var/lib/SkyPanel"
    }
  }
}
EOF
```

#### 7. Create system user

```text
sudo useradd -r -m -d /var/lib/SkyPanel -s /bin/bash skypanel
sudo chown -R skypanel:skypanel /var/lib/SkyPanel /var/log/SkyPanel
```

#### 8. Create systemd service

```text
sudo tee /etc/systemd/system/skypanel.service > /dev/null <<EOF
[Unit]
Description=Aether Panel - Server Management Panel
After=network.target

[Service]
Type=simple
User=skypanel
WorkingDirectory=/var/lib/SkyPanel
ExecStart=/usr/sbin/SkyPanel runService --config /etc/SkyPanel/config.json
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable skypanel
```

## Installation Verification

After installing, verify that everything is working correctly:

### Verify Service (Native Installation)

```text
# Check service status
sudo systemctl status skypanel

# View logs in real-time
sudo journalctl -u skypanel -f
```

### Verify Containers (Docker Installation)

```text
# Check container status
docker compose ps

# View logs
docker compose logs -f skypanel
```

### Verify Ports

```text
# Verify that ports are listening
sudo netstat -tuln | grep -E '8080|5657'

# Or use ss
sudo ss -tuln | grep -E '8080|5657'
```

### Verify Web Panel

```text
# From the server
curl http://localhost:8080

# Or from your browser
# http://YOUR_IP:8080
```

## First Access and Initial Configuration

Once installation and verification are complete, it's time to access the panel and configure it for the first time.

### Access the Panel

Open your browser and access:

You will be greeted by the login screen.

- **ip**: http://YOUR_IP:8080
- **domain**: https://your-domain.com (if you configured domain and SSL)
### Step 1: Create Administrator User

Before you can log in, you need to create an administrator user using the CLI.

If you installed with Docker, run the commands inside the container:

If you installed natively, run directly:

- **dockerCommand**: # Enter the container
docker exec -it skypanel sh

# Create admin user
/SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin
- **nativeCommand**: sudo -u skypanel /usr/sbin/SkyPanel user add --name admin --email admin@example.com --admin
- **p4**: Or from the installation directory:
- **localCommand**: SkyPanel user add --name admin --email admin@example.com --admin
- **p5**: The system will ask you for a password. Choose a secure password.
#### Command Parameters

- --username: Username for login
- --email: Email address
- --admin: Grants full administrator permissions

### Step 2: Log In

Now you can log in to the panel:

- Open http://YOUR_IP:8080 in your browser
- Enter the username you created
- Enter the password
- Click 'Log In'

### Available CLI Commands

To see all available commands from the Aether Panel CLI:

Or from the Docker container:

- **helpCommand**: SkyPanel --help
- **dockerHelp**: docker exec skypanel /SkyPanel/bin/SkyPanel --help
#### Main Commands

##### User Management

- SkyPanel user add --name USER --email EMAIL --admin
- SkyPanel user add --name USER --email EMAIL --password PASS (non-interactive)
- SkyPanel user edit (interactive menu: username/email/password/admin/2FA)

##### Database Management

- SkyPanel db upgrade (update the database schema to the new version)

##### Run Panel

- SkyPanel runService (start as service with systemd notify)
- SkyPanel version (show version)

### Step 3: Configure Firewall

If you don't have the ports enabled in the firewall, the panel will not be accessible from outside the server.

#### Ubuntu/Debian (UFW)

```text
# Enable UFW if not active
sudo ufw enable

# Allow necessary ports
sudo ufw allow 8080/tcp
sudo ufw allow 5657/tcp
sudo ufw allow 8081/tcp

# Verify rules
sudo ufw status
```

#### Fedora/RHEL/CentOS (firewalld)

```text
# Start firewalld if not active
sudo systemctl start firewalld
sudo systemctl enable firewalld

# Allow necessary ports
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=5657/tcp
sudo firewall-cmd --permanent --add-port=8081/tcp

# Apply changes
sudo firewall-cmd --reload

# Verify rules
sudo firewall-cmd --list-ports
```

### Step 4: Initial Configuration in the Panel

Once you have logged in, configure the basic settings:

#### Basic Configuration

- Go to Settings in the menu
- Configure the panel URL (e.g., http://YOUR_IP:8080 or https://your-domain.com)
- Configure the administrator email
- Configure the company/organization name
- Save changes

#### Configure Nodes

If you installed on the same server where servers will run (local node), you don't need to configure anything else. The local node is configured automatically.

If you want to use remote nodes:

- Go to Admin  Nodes
- Click 'Create Node'
- Enter the remote node IP
- Enter the daemon port (default 5656)
- Enter the daemon authentication token
- Verify the connection
- Save the node

#### Configure Discord (Optional)

To receive notifications on Discord:

To create a Discord webhook:

- Go to Settings  Notifications
- Enter the Discord webhook URL
- Configure the types of notifications you want to receive
- Save changes

- Go to your Discord server
- Channel Settings  Integrations  Webhooks
- Create a new webhook
- Copy the webhook URL

### Step 5: Create Your First Server

Now you're ready to create your first server:

Congratulations! You now have your first server running on Aether Panel.

- Go to the Servers section
- Click 'Create Server'
- Select the node where it will run (or use the local node)
- Choose a template (e.g., Minecraft Java Edition)
- Configure the server name
- Configure the server port
- Adjust resources (RAM, CPU) if necessary
- Click 'Create'
- Wait for the server to install
- Click 'Start' to boot the server

## Installation Summary

If you have followed all the steps, you should have:

-  Aether Panel installed and running
-  Administrator user created
-  Firewall configured
-  Panel accessible at http://YOUR_IP:8080
-  Basic configuration completed
-  First server created (optional)

### Next Steps

Now that you have the panel running, you can:

- Explore all panel functionalities
- Create more servers of different types
- Configure Database Hosts for MySQL databases
- Add additional users with specific permissions
- Configure automatic backups
- Explore the API for automation
- Read the complete documentation in other sections

### Need Help?

If you encounter problems during installation:

- Review the Troubleshooting section in the documentation
- Check the panel logs
- Consult the FAQ for common issues
- Join the community Discord for help

## Advanced Panel Configuration

### Configuration File

The main configuration file is located at:

You can edit this file to customize the panel configuration.

- **native**: /etc/SkyPanel/config.json (native installation)
- **docker**: /etc/SkyPanel/config.json (inside Docker container)
### Database Configuration

Aether Panel supports SQLite (default) and MySQL/MariaDB.

#### SQLite (Default)

SQLite is configured automatically and requires no additional configuration. It's ideal for small installations.

#### MySQL/MariaDB

To use MySQL, edit the configuration file:

Make sure the database and user exist before starting the panel.

```text
{
  "panel": {
    "database": {
      "dialect": "mysql",
      "url": "user:password@tcp(host:3306)/database?charset=utf8mb4&parseTime=true"
    }
  }
}
```

### Port Configuration

The default ports are:

You can change these ports by editing the configuration file or environment variables.

- 8080: Web panel (HTTP)
- 5657: SFTP for file transfer

