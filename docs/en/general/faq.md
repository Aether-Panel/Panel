# Frequently Asked Questions

What is Aether Panel?

Aether Panel is an open source game server management panel. It is built with Go 1.25 (Gin + GORM) for the backend and a modern frontend with Astro + React 19. It runs on your own server (self-hosted) and lets you manage Minecraft, Terraria, Valheim, and more servers from a web interface.

Is Aether Panel free?

Yes, the main panel is 100% free and open source under the Apache 2.0 license. All panel features are free with no exceptions. There may be integration modules with third-party closed-source software (like WHMCS) that have a cost to cover licenses and maintenance, but the panel itself is and will always be free.

How is Aether Panel different from other panels?

Aether Panel is an independent panel. We maintain compatibility with the original core but add our own improvements, a renewed frontend with Astro + React 19, community integration (suggestions, voting), a marketing website and documentation, and additional features like external transfer between panels.

What kind of servers can I run?

Aether Panel supports any server that can run in Docker. It comes with templates for Minecraft (Vanilla, Spigot, Paper, Forge, Fabric), Terraria, Valheim, Discord bots, and many more. You can create custom templates for any application.

Do I need Docker to use Aether Panel?

Yes, the Aether Panel daemon uses Docker to isolate and run servers. Each server runs in its own Docker container, providing isolation, security, and easy management. Docker is included automatically during installation.

What is the panel architecture?

The panel has two main components: the panel (web interface and REST API on port 8080) and the daemon (Docker server execution, SFTP on port 5657). In standard installations both run together. The panel frontend is built with Astro + React.

Can I manage multiple physical servers?

Yes, Aether Panel supports multiple nodes. You can add physical or virtual servers as nodes and distribute your game servers among them. Each node runs a daemon that communicates with the central panel via HTTP API with a shared secret.

How does authentication work?

The panel supports Bearer token authentication (Authorization header) or cookies (skypanel_auth). Sessions use SHA256-hashed UUID v4 tokens. It also includes a full OAuth2 server with Ed25519-signed JWTs, and 2FA/TOTP support with recovery codes.

How does the permission system work?

The panel has approximately 74 granular scopes (50 server-specific + 24 global) that control every action. Permissions can be assigned at global level (affect all servers) or per-server. There are also roles that group scopes. The 'admin' scope grants all permissions, and 'server.admin' grants all permissions on a specific server.

What database does the panel use?

It uses SQLite by default, which is configured automatically. It also supports PostgreSQL and MySQL/MariaDB. Configuration is done in config.json. The panel uses GORM as the ORM.

How do I create databases for my servers?

First create a Database Host from Admin  Database Hosts with MySQL credentials that have GRANT ALL privileges. Then, from the Database tab of any server, you can create databases that are automatically generated with user, password, and remote connection info.

How do I access my server files?

You have two options: the integrated file editor in the server's Files tab (with syntax highlighting, compression, and extraction), or SFTP using the credentials shown in the SFTP tab (port 5657).

How do backups work?

From the Backups tab you can create manual backups with custom names. Backups are stored compressed on the node and you can download, restore, or delete them. Each backup has a unique name for easy identification.

How does the real-time console work?

The console uses WebSocket to stream live server logs. You can send commands and see output in real-time with auto-scroll. The connection goes from the panel to the node daemon via proxy. It includes timestamps and fullscreen mode.

What statistics can I see for a server?

The Stats tab shows real-time charts for CPU, RAM, disk usage, and network traffic. Data updates via WebSocket. You can also see server status (running/stopped/installing) and query server information (version, players, etc.) if the game supports it.

How do plugins work?

The Plugins tab lets you search and install plugins for compatible Minecraft servers (Spigot/Paper). You can search by name in public repositories, view details, and install them directly from the panel.

What is external transfer?

External transfer (extransfer) allows moving servers between independent Aether Panel installations. It is useful for migrating servers between different panels or providers. Internal transfer between nodes of the same panel is also available.

What is Gatus?

The repository includes a configuration file `data/gatus/config.yaml` to monitor the uptime of the panel and its nodes with Gatus (web dashboard on port 8081). However, **Gatus is not deployed automatically** with the `docker-compose.yml`: if you want to use it, you must run Gatus separately using that configuration file.

Does Aether Panel have an API?

Yes, the panel exposes a complete RESTful API at /api/*. All endpoints require Bearer token authentication. The API covers servers, nodes, users, files, backups, databases, roles, templates, uptime, configuration, and more. There is also a WebSocket for real-time data.

Does it support OAuth2?

Yes, the panel includes an OAuth2 server with the /oauth2/token endpoint. It uses the Client Credentials flow (panel↔node authentication) and Password (SFTP authentication), with Ed25519-signed JWTs. Public keys are available at /auth/publickey in JWKS format for third-party verification.

What CLI commands does the panel have?

The panel includes a full Cobra-based CLI. Main commands: SkyPanel run (start panel and daemon), SkyPanel version (show version), SkyPanel user (user management), SkyPanel db (database migrations), SkyPanel runservice (run as a system service).

How do I update Aether Panel?

If using Docker: `docker compose build && docker compose up -d` (or `git pull` and rebuild the image). Check the installation documentation for detailed instructions.

Can I use an external MySQL database?

Yes, you can configure the panel to use external MySQL or PostgreSQL by editing the database configuration in config.json. It uses SQLite by default which is configured automatically.

How can I contribute to the project?

You can contribute in several ways: reporting bugs on GitHub, submitting pull requests with improvements, participating in community voting to prioritize features, joining the Discord for feedback, or making a voluntary donation via PayPal.

Where can I get help?

We have several channels: the Discord community server for support and discussion, GitHub issues for bug reports and feature requests, and the documentation on this website. As an open source project, support is community-driven.

What's new in the redesigned Settings view (v2.0.1+)?

The server Settings tab now has a 2-column layout, per-section visual identity, sticky save bar, and role-based permissions. Admin sees all + CRUD; User sees General Info, Groups/Variables, Plugins, Auto-start, Ports (view + primary + notes), but NOT Limits, Metadata, Admin tab. Extra ports are managed with per-port notes and primary selection.

Why doesn't my Docker server connect to MySQL (`mysql:3306`)?

Previously containers were created in the `bridge` network and couldn't resolve `mysql`. Since v2.0.1 the Panel auto-detects its Docker network (`skypanel-network`) and connects all servers to it. If the server already existed, delete and recreate it to join the correct network.

What changed in the "User" role permissions?

Improper admin accesses were revoked (`server.admin.config.view/manage`, `server.data.edit.admin`) and correct permissions granted: `server.definition.view/edit`, `server.flags.view/edit`. Users can now view/edit server definition and flags, but not admin config or limits.

Why does the build fail with `monaco-editor/min/vs/editor/editor.main.css?inline`?

`monaco-editor@0.56.0+` changed its `exports` and breaks CSS `?inline` imports in Vite 8/Rolldown. Version `^0.44.0` is pinned in `client/frontend/package.json` until upstream fix.


