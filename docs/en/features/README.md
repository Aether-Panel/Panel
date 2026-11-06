# Features

User and administrator guides for Aether Panel features. Each page explains what the feature does, how it works, how to use it from the panel and the API, the configuration keys it depends on, its limits, and known errors with their fixes.

For full endpoint details see the [API reference](../reference/11-api-reference.md); for every configuration key and its environment variable see [Configuration](../backend/02-configuration.md); for permissions see the [scopes reference](../general/scopes-reference.md).

## Server pages

| Page | What it covers |
|------|----------------|
| [Server overview and statistics](server-statistics.md) | Overview tab: live cards and charts, performance history, availability, players and peak hours |
| [Server settings](server-settings.md) | Settings tab: name and description, resource limits, ports, auto-start and crash restart flags |
| [Server properties editor](server-properties.md) | Experimental `server.properties` editor for Minecraft Java servers |
| [Game version](game-versions.md) | Experimental one-click game version switch (Paper and vanilla templates) |
| [Modpacks and plugins](modpacks-and-plugins.md) | Modpack install from Modrinth and CurseForge; Modrinth plugin browser |
| [World map](world-map.md) | BlueMap and Dynmap install and the in-panel map viewer |
| [Scheduled tasks](scheduled-tasks.md) | Cron and manual tasks: steps, permissions, limits |
| [Backups](backups.md) | Manual and scheduled backups, restore, restore points, per-server cap, S3 off-site copies (periodic and per task) |
| [File history](file-history.md) | Saved versions of files edited in the panel, diff and restore |
| [Crash detection and crash reports](crash-reports.md) | How crashes are detected, automatic restart, crash reports |
| [Server notifications](server-notifications.md) | Per-server alerts to Discord, Telegram or a signed webhook |
| [Automatic subdomains (Cloudflare)](subdomains.md) | Server subdomains and Minecraft SRV records managed through Cloudflare |
| [Install from GitHub](github.md) | Connect a GitHub account and install a repository (private ones too) and its branch into a server |
| [AI assistant and log analysis](ai-assistant.md) | Server assistant chat, console analysis and "Explain with AI" (Gemini) |
| [AI assistant: working with files](ai-agent.md) | What the assistant can do with a server's files, approvals, where changes are recorded, privacy, limits |

## Server lifecycle

| Page | What it covers |
|------|----------------|
| [Cloning servers](cloning.md) | Experimental server cloning: what is copied, ports, permissions |
| [Sub-servers (Splitter)](sub-servers.md) | Splitting a server's resources into sub-servers |
| [Server transfers](server-transfers.md) | Moving a server to another node, and migrating it to another panel |

## Panel administration

| Page | What it covers |
|------|----------------|
| [Dashboard and nodes](dashboard-and-nodes.md) | Dashboard figures, node pages, network history, disk alerts, port checks, image prefetch |
| [Activity log and access records](activity-and-access-log.md) | Per-server activity log and panel sign-in records |
| [Database hosts and server databases](database-hosts.md) | MySQL/MariaDB hosts and per-server databases |
| [Panel settings](panel-settings.md) | The Panel Settings page, its tabs and the experimental feature switches |
| [Templates](templates.md) | Local and community templates, upload rules, template repositories |
| [Panel updates](panel-updates.md) | Update check and in-panel self-update for the panel and its nodes |
| [Billing integration: provisioning API and sign-in links](provisioning-and-sso.md) | API keys, products, `/api/v1` provisioning routes and one-time sign-in links |

## Related guides elsewhere

- [Uptime tracking](../general/13-uptime.md)
- [Email](../general/15-email.md) and [Discord](../general/16-discord.md) panel integrations
- [Captcha / anti-bot (built-in check and Turnstile)](../general/17-turnstile.md)
- [File compression](../general/19-files-compression.md)
- [Extensions](../reference/15-extensions.md)
