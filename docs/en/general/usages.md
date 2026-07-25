# Panel Usage

## Interface Navigation

The panel interface is divided into several main sections, accessible from the sidebar. The dashboard provides an overview, while other sections allow you to manage specific resources like servers, users, and nodes. The panel consists of two parts: a Go backend serving the REST API and web frontend, and a modern frontend built with Astro and React.

### Main Sections

#### Dashboard

Overview with statistics, active servers, and system status with resource usage charts.

#### Servers

Manage all your game servers: real-time console, files, backups, databases, plugins, and more.

#### Users

Manage users, granular scope-based permissions, and roles.

#### Nodes

Configure and manage the physical or virtual nodes where servers run.

#### Administration

Advanced panel configuration: Database Hosts, templates, global roles, and more.

## Panel Architecture

Aether Panel is a fork of PufferPanel built with Go 1.25. The backend exposes a RESTful API using Gin (web framework) and GORM (ORM). The panel runs in two modes: panel (web interface and API) and daemon (server execution). In standard installations, both modes run in a single binary.

### Technology Stack

#### Backend (Go)

- Go 1.25 with Gin framework for REST API
- GORM for database access (SQLite default, PostgreSQL/MySQL supported)
- Cobra CLI for administration commands
- Bearer token authentication with sessions and cookies
- Integrated OAuth2 server for third-party authentication
- Integrated SFTP server for file transfer
- WebSocket for real-time console and stats
- Docker for isolated server environments

#### Frontend (Dashboard)

- Astro + React 19 with TypeScript
- Tailwind CSS 3.4 for styling
- shadcn/ui + Radix UI for interface components
- Full control panel with responsive design
- Integrated code editor with syntax highlighting
- Real-time charts with Recharts
- Interactive web console via WebSocket

## Server Management

To add a new server, navigate to the "Servers" section and click "Create New". You will be prompted to select a node, choose a template, and configure its settings. Servers run in Docker containers for isolation and security.

### Creating a Server

- Click the "Create Server" button
- Select the node where the server will run
- Choose a template (Minecraft, Terraria, etc.)
- Configure name, port, IP, and resources
- Assign users to the server with their permissions
- Click "Create" — the panel will install the server automatically

### Server Lifecycle

The panel exposes REST endpoints to fully control the server lifecycle. Each action requires authentication and specific scope permissions.

- Start — powers on the server
- Stop — gracefully shuts down the server
- Restart — stops then starts the server
- Kill — force immediate shutdown
- Install/Reinstall — runs the template installation process
- Reload — reloads the server configuration

### Server Tabs

#### Console

Access the server console in real-time via WebSocket. Send commands and view live logs with auto-scroll. Includes timestamps and fullscreen mode.

#### Files

Full file manager with integrated code editor. Navigate, edit, rename, create, delete, compress, and extract files. The editor includes syntax highlighting and keyboard shortcuts.

#### Databases

Create and manage MySQL/MariaDB databases for your server. Credentials are auto-generated and connection info is displayed with quick-copy.

#### SFTP

Integrated SFTP access for file transfer using clients like FileZilla or WinSCP. Displays server, port, user, and password.

#### Backups

Create, download, restore, and delete server backups. Each backup is stored compressed on the node. You can name each backup for easy identification.

#### Plugins

Search, install, and manage plugins from the panel. Compatible with Minecraft servers (Spigot/Paper). Search by name in public plugin repositories.

#### Server Users

Manage which users have access to the server and with what permissions. Add or remove users and adjust scopes individually.

#### Settings

Environment variables, startup flags, server type, name, and advanced server configuration.

#### Statistics

Real-time CPU, RAM, disk, and network charts. Data updates via WebSocket for a smooth experience.

### Node Transfer

You can transfer servers between nodes in the panel. The transfer includes all files and configuration. External federated transfers between independent panels are also available.

## User & Role Management

In the "Users" section, you can create new user accounts and assign them to roles with specific permissions. The permission system is based on scopes that granularly control every action within the panel.

### Creating a User

- Click "Create User"
- Fill the form with username, email, and password
- Assign global permissions or roles
- Save the user

### Permission System (Scopes)

Aether Panel uses ~80 granular scopes that control every action. Permissions can be assigned at global level (affect all servers) or at individual server level. Roles group scopes for mass assignment.

#### Scope Categories

- **server**: Server control: view, start, stop, kill, restart, install, reload, status, stats, console, send commands, rename
- **files**: Files: view, edit, sftp, compress, extract
- **backups**: Backups: view, create, restore, delete, download
- **users**: Server users: view, create, edit, delete
- **data**: Server data: view, edit (user), edit (admin)
- **tasks**: Scheduled tasks: view, run, create, edit, delete
- **admin**: Global administration: admin, settings.edit, nodes (view/create/edit/delete/deploy), users (search/view/edit), templates, uptime
## Node Management

Nodes are physical or virtual servers where Docker containers for your game servers run. The panel can manage multiple nodes from a single interface. Each node runs a daemon that communicates with the panel via HTTP API.

### Creating a Node

- Go to the "Nodes" section in the admin panel
- Click "Create Node"
- Configure name, IP, port, and server folders
- Configure communication credentials (shared secret)
- Verify the connection with the new node
- Save the node — you can now create servers on it

### Local Node

The local node is the same server where the panel is installed. It is configured automatically during installation and runs the daemon in the same process.

### Node Features

Each node exposes its capabilities to the panel: environment type (Docker), file system, system information (CPU, RAM, disk). You can check any node's features from the interface.

## Database Hosts

Database Hosts allow managing MySQL/MariaDB servers to automatically create databases for your game servers.

### Creating a Database Host

- Go to Admin  Database Hosts
- Click "Create Database Host"
- Step 1: Enter MySQL credentials with GRANT ALL privileges
- Step 2: Configure host, port, name, and linked nodes
- Save the Database Host

### MySQL Setup

For the panel to automatically create databases, you need:

- A MySQL user with GRANT ALL PRIVILEGES ON *.* WITH GRANT OPTION
- MySQL configured to accept external connections (bind-address=0.0.0.0)
- Port 3306 open in the firewall

### Using Database Hosts

Once a Database Host is created, you can create databases from the server's "Database" tab. The panel will automatically generate:

- Unique database name
- User with specific permissions
- Secure password
- Remote connection information

## SFTP

Aether Panel includes an integrated SFTP server that allows users to access their server files using any standard SFTP client (FileZilla, WinSCP, Cyberduck).

- The SFTP server runs on the configured port (default 5657)
- Each user can only access servers where they have permissions
- Authentication uses the panel credentials
- Users need the server.sftp scope to access via SFTP

## Real-Time WebSocket Connection

The panel provides a WebSocket connection through the REST API that allows receiving real-time data from the server. The socket connects via proxy from the panel to the node daemon.

- Interactive server console with live logs
- Real-time CPU, RAM, and disk statistics
- Server status (running/stopped/installing)
- Bearer token authentication

## Command Line Interface (CLI)

The panel includes a full Cobra-based CLI for terminal administration.

- SkyPanel run — Starts the panel and daemon
- SkyPanel version — Shows the panel version
- SkyPanel user — User management commands (create, modify, delete)
- SkyPanel db — Database commands (migrate, upgrade)
- SkyPanel runservice — Runs as a system service

