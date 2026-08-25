# Aether Panel Technical Documentation

Welcome to the official technical documentation of Aether Panel. Here you will find in-depth information about the architecture, backend, frontend, and configuration of the panel.

## Table of Contents

### Installation and Configuration
*   [01. Installation Guide](./setup/01-installation.md): Step-by-step instructions to deploy the panel.
*   [Troubleshooting](./setup/troubleshooting.md): Guide for solving common issues and errors (includes MySQL Docker, Monaco Editor build).
*   [Docker Quick Reference](./docker/DOCKER-QUICKREF.md): Essential commands for Docker environments.
*   [Sudo Troubleshooting (Docker)](./docker/DOCKER-SUDO-FIX.md): Guide to resolving permission issues in Docker.

### General (Concepts and Guides)
*   [Getting Started](./general/getting-started.md): Complete guide from installation to first server.
*   [Key Concepts](./general/concepts.md): Architecture, nodes, servers, auth, scopes, SFTP, templates, database hosts, federated transfer, **port management, Docker network auto-detection, updated roles, Monaco Editor pin**.
*   [FAQ](./general/faq.md): Frequently asked questions (v2.0.1+ features, roles, MySQL connectivity, Monaco).
*   [Uptime Tracking](./general/13-uptime.md): Server uptime/downtime tracking system.
*   [License System](./general/14-license.md): License system (external API).
*   [Email/SMTP](./general/15-email.md): Email providers and template system.
*   [Discord Webhooks](./general/16-discord.md): Webhook types and embed structures.
*   [Turnstile/Captcha](./general/17-turnstile.md): Cloudflare Turnstile.
*   [Conditions/CEL Engine](./general/18-conditions-cel.md): CEL engine + custom functions.
*   [Files/Compression](./general/19-files-compression.md): FileServer, MergedFS, archive/extract.
*   [Security/unshare](./general/20-security-unshare.md): openat2, unshare, kernel support.
*   [Terms and Conditions](./general/terms.md): Legal terms.
*   [Privacy Policy](./general/privacy.md): Data privacy.
*   [Security](./general/security.md): Security considerations.
*   [Usages](./general/usages.md): Use cases.

### Backend (Architecture and APIs)
*   [01. Overview](./backend/01-overview.md): Structure of the Go code, Panel and Daemon modes.
*   [02. Configuration](./backend/02-configuration.md): `config.json` file and Viper system.
*   [03. Database](./backend/03-database.md): GORM models and migrations.
*   [04. Server Management](./backend/04-servers.md): Lifecycle, scheduler, **ExtraPortBindings, Docker network auto-detection**.
*   [05. API and HTTP Layer](./backend/05-api-layer.md): REST routes, CORS, OAuth2, **port endpoints**.
*   [06. Operations and Conditions](./backend/06-operations.md): JSON-based operation engine for tasks (downloads, unzip, chown).
*   [07. CLI Tool](./backend/07-cli.md): Usage of the SkyPanel binary from the terminal.
*   [08. WebSocket and Console](./backend/08-websocket.md): Real-time transmission of logs and statistics.
*   [09. Security and SFTP](./backend/09-security.md): Scopes, JWKS, and integrated SFTP server.
*   [10. Remaining Packages](./backend/10-remaining-packages.md): Email, logs, utilities, and template system.
*   [11. Internal Services](./backend/11-services.md): 19 business services (Email, Discord, Token, License, etc.).
*   [12. RCON/Telnet/RCON-WS](./backend/12-rcon-telnet.md): Stdin connections, auto-proxy.
*   [13. Process/Metrics](./backend/13-process-metrics.md): Process mgmt, metrics, JVM stats.

### Frontend (Interface Architecture)
*   [01. Client Architecture](./frontend/01-architecture.md): General structure of Astro, React, and Tailwind.
*   [02. Endpoints and API Client](./frontend/02-endpoints.md): Native API SDK and React integration.
*   [03. Components and UI](./frontend/03-components.md): Shadcn UI, **Redesigned Settings View, fixed roles**.
*   [04. Artificial Intelligence (Genkit)](./frontend/04-ai.md): Google GenAI integration for log analysis.
*   [05. Translations System (i18n)](./frontend/05-translations.md): Internationalization using JSON dictionaries.
*   [06. Pages and Views](./frontend/06-pages.md): Main dashboard views, **Settings View, Sidebar fix**.
*   [07. Components & Hooks Detailed](./frontend/07-components-detailed.md): All components, hooks, contexts, API client.
*   [08. AI Detailed](./frontend/08-ai-detailed.md): Genkit flows, prompts, UI.

### References
*   [API Reference](./reference/11-api-reference.md): General schema for programmatic usage.
*   [CLI Commands Reference](./reference/14-cli-commands.md): Detailed list of console subcommands and flags.