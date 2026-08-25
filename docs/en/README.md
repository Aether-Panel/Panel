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

### Frontend (Interface Architecture)
*   [01. Client Architecture](./frontend/01-architecture.md): General structure of Astro, React, and Tailwind.
*   [02. Endpoints and API Client](./frontend/02-endpoints.md): Native API SDK and React integration.
*   [03. Components and UI](./frontend/03-components.md): Shadcn UI, **Redesigned Settings View, fixed roles**.
*   [04. Artificial Intelligence (Genkit)](./frontend/04-ai.md): Google GenAI integration for log analysis.
*   [05. Translations System (i18n)](./frontend/05-translations.md): Internationalization using JSON dictionaries.
*   [06. Pages and Views](./frontend/06-pages.md): Main dashboard views, **Settings View, Sidebar fix**.

### References
*   [API Reference](./reference/11-api-reference.md): General schema for programmatic usage.
*   [CLI Commands Reference](./reference/14-cli-commands.md): Detailed list of console subcommands and flags.