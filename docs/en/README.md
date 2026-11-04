# Aether Panel Documentation

Documentation for installing, running, using and extending Aether Panel. It describes what the code does today; when a feature is experimental or only partly implemented, its page says so.

> The product is called Aether Panel. The Go module, the binary and some identifiers (`skypanel` container, `SKYPANEL_*` environment variables, `skypanel_auth` cookie) still use the name SkyPanel.

The same documentation is available in Spanish in [`docs/es`](../es/README.md), with the same file names.

## Start here

1. [Installation guide](./setup/01-installation.md): requirements, the installer and its options, updating and uninstalling.
2. [Getting started](./general/getting-started.md): from a fresh install to the first running server.
3. [Core concepts](./general/concepts.md): nodes, servers, templates, users, roles and permissions, SFTP.
4. [Panel usage](./general/usages.md): where everything is in the interface.
5. [Frequently asked questions](./general/faq.md) and [Troubleshooting](./setup/troubleshooting.md).

## Installation and operation

| Page | Contents |
|------|----------|
| [Installation guide](./setup/01-installation.md) | Supported systems, `install.sh` options, domain and HTTPS, nodes, updates, uninstall |
| [Troubleshooting](./setup/troubleshooting.md) | Known failures and how to fix them |
| [Docker documentation](./docker/README.docker.md) | Running the panel with Docker Compose, volumes, runtime images, development setup |
| [Docker quick reference](./docker/DOCKER-QUICKREF.md) | Everyday Docker commands for the panel |
| [Docker socket and permissions](./docker/DOCKER-SUDO-FIX.md) | Access to the Docker socket and permission errors |
| [Configuration](./backend/02-configuration.md) | Every configuration key, its default and its environment variable |
| [CLI commands](./reference/14-cli-commands.md) | Every command and flag of the binary |

## Features

User and administrator guides, one page per feature. Full list: [Features](./features/README.md).

- Server pages: [overview and statistics](./features/server-statistics.md), [settings](./features/server-settings.md), [`server.properties` editor](./features/server-properties.md), [game version](./features/game-versions.md), [modpacks and plugins](./features/modpacks-and-plugins.md), [world map](./features/world-map.md), [scheduled tasks](./features/scheduled-tasks.md), [backups](./features/backups.md), [file history](./features/file-history.md), [crash reports](./features/crash-reports.md), [notifications](./features/server-notifications.md), [subdomains](./features/subdomains.md), [install from GitHub](./features/github.md), [AI assistant](./features/ai-assistant.md) and its [work with files](./features/ai-agent.md).
- Server lifecycle: [cloning](./features/cloning.md), [sub-servers](./features/sub-servers.md), [transfers](./features/server-transfers.md).
- Panel administration: [dashboard and nodes](./features/dashboard-and-nodes.md), [activity and access log](./features/activity-and-access-log.md), [database hosts](./features/database-hosts.md), [panel settings](./features/panel-settings.md), [templates](./features/templates.md), [panel updates](./features/panel-updates.md), [billing integration and sign-in links](./features/provisioning-and-sso.md).

## Integrations

| Page | Contents |
|------|----------|
| [Email](./general/15-email.md) | Email providers and templates |
| [Discord and notifications](./general/16-discord.md) | Panel webhooks and embeds |
| [Captcha / anti-bot](./general/17-turnstile.md) | Built-in anti-bot check (on by default, no keys) and optional Cloudflare Turnstile |
| [Uptime tracking](./general/13-uptime.md) | How server availability is recorded and shown |
| [License system](./general/14-license.md) | License checks against the external API |

## Security and permissions

| Page | Contents |
|------|----------|
| [Security](./general/security.md) | Recommendations for administrators, two-step sign-in, sessions, SSH keys, hardening |
| [Permissions reference](./general/scopes-reference.md) | Every scope, default roles, server owner and sub-users, API key permissions |
| [Security internals](./backend/09-security.md) | Scopes, JWT between panel and nodes, SFTP server, HTTP protections |
| [Kernel isolation](./general/20-security-unshare.md) | `openat2` and `unshare` |

## Reference

| Page | Contents |
|------|----------|
| [API reference](./reference/11-api-reference.md) | Index of the HTTP API, split by area: [conventions](./reference/api/01-conventions.md), [authentication](./reference/api/02-authentication.md), [servers](./reference/api/03-servers.md), [server content](./reference/api/04-server-content.md), [accounts](./reference/api/05-accounts.md), [panel administration](./reference/api/06-panel-admin.md), [provisioning](./reference/api/07-provisioning.md), [external transfer](./reference/api/08-external-transfer.md), [daemon](./reference/api/09-daemon.md) |
| [CLI commands](./reference/14-cli-commands.md) | Commands and flags |
| [Extensions](./reference/15-extensions.md) | Extension packages: manifest, pages, events, installation |
| [Conditions (CEL)](./general/18-conditions-cel.md) | Condition expressions used by templates and operations |
| [Server files and archives](./general/19-files-compression.md) | File server, archive and extract rules |

## Backend internals

| Page | Contents |
|------|----------|
| [01. Overview](./backend/01-overview.md) | Code layout, panel and daemon modes |
| [02. Configuration](./backend/02-configuration.md) | `config.json`, environment variables and panel settings |
| [03. Database](./backend/03-database.md) | Models, tables and migrations |
| [04. Servers](./backend/04-servers.md) | Environments, lifecycle, scheduler, disk limit, reconnection, subdomains |
| [05. API layer](./backend/05-api-layer.md) | Routes, middleware, authentication, OAuth2 |
| [06. Operations engine](./backend/06-operations.md) | Install and task operations, downloads, conditions |
| [07. CLI](./backend/07-cli.md) | How the command line is built |
| [08. WebSocket](./backend/08-websocket.md) | Real-time console, stats and status |
| [09. Security](./backend/09-security.md) | Permissions, tokens, SFTP, HTTP protections |
| [10. Remaining packages](./backend/10-remaining-packages.md) | Other packages under `internal/` and `pkg/` |
| [11. Services](./backend/11-services.md) | Business services in `internal/services` |
| [12. Console input](./backend/12-rcon-telnet.md) | stdin, RCON, Telnet and RCON over WebSocket |
| [13. Processes and metrics](./backend/13-process-metrics.md) | Process management, metrics history, node sampling |
| [14. AI agent toolkit](./backend/14-ai-toolkit.md) | The assistant's file tools: agent loop, Gemini function calling, isolation, confirmations, audit |

## Frontend internals

| Page | Contents |
|------|----------|
| [01. Architecture](./frontend/01-architecture.md) | Structure of `client/frontend` (Astro and React) |
| [02. API consumption](./frontend/02-endpoints.md) | API client, errors, how the UI calls the backend |
| [03. Components](./frontend/03-components.md) | Interface components and layout |
| [04. AI features](./frontend/04-ai.md) | AI features in the interface (overview) |
| [05. Translations](./frontend/05-translations.md) | Internationalization; English is the default language |
| [06. Pages](./frontend/06-pages.md) | Routing and pages |
| [07. Detailed reference](./frontend/07-components-detailed.md) | Components, hooks and contexts in detail |
| [08. AI in detail](./frontend/08-ai-detailed.md) | AI features in detail |

## Legal

- [Terms and conditions](./general/terms.md)
- [Privacy policy](./general/privacy.md)
