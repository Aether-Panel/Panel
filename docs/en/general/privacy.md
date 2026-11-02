# Privacy Policy

## Introduction

Aether Panel is an open source panel published under the Apache 2.0 license. All source code is publicly available on GitHub for review, audit, and contribution. This privacy policy explains how we handle data in our community and on the website.

Since Aether Panel runs on your own server (self-hosted), **we do not have access to your installation or the data it contains**. All information about your servers, users, and configuration stays local on your infrastructure. This policy primarily covers your interaction with our community (Discord, GitHub, website).

We believe in total transparency: as an open source project, anyone can audit the code to verify there is no hidden data collection, telemetry, or backdoors.

## Our Philosophy

Aether Panel was created with a different philosophy:

This philosophy sets us apart from other panels that, as forks or similar projects, choose business models that compromise privacy or software openness.

### Real Open Source, No Tricks

The panel core is on GitHub under the Apache 2.0 license. No enterprise versions, no hidden premium features, no covert telemetry. Addons for integrations with closed-source software (WHMCS) may have a cost to cover licenses and maintenance, but the panel itself is and will always be free.

### Privacy by Design

The panel runs on your server. We collect nothing. No usage statistics, no telemetry, no trackers. What happens on your server stays on your server.

### Community, Not Customers

You are not a customer because we don't sell anything. You are part of a community. Decisions are made based on what the community needs, not what generates revenue.

### Total Transparency

Being open source (Apache 2.0), anyone can audit the code. No closed binaries, no mandatory cloud services, no surprises. The main panel is completely transparent.


## Data We Collect

**In your panel installation:** We collect absolutely nothing. The panel has no telemetry and no usage reporting. It does contact some external services: a few always (user avatars, and the update check when an administrator opens the panel), the rest only when you configure or use them. They are listed in [External services the panel contacts](#external-services-the-panel-contacts).

**In our community (Discord, GitHub, website):** If you choose to interact with us, we voluntarily collect:

**Important:** If you install the panel, you are solely responsible for your users' data. We have no access, collect no metrics, and cannot see your installation.

### Discord Data (Only if you join)

By joining our Discord server:

- Username, avatar, and Discord ID (public profile information)
- Messages you send in public channels
- Roles assigned to you (member, contributor, etc.)

This is standard for any Discord server. We do not store this data outside of Discord.

### Website Data (Suggestions/Voting)

If you participate in suggestions or voting on the website:

- Name you provide when submitting a suggestion
- Votes on feature proposals (stored in Firestore)
- Approximate country (determined by IP, for statistical purposes only)

We do not require registration for most interactions. You can participate anonymously.

### GitHub Data (Only if you contribute)

If you contribute code to the project:

- GitHub username
- Code and commit messages you submit
- Issues and pull requests you create

All this information is public by nature on GitHub.


## How We Use Your Data

The limited data we collect (community only) is used exclusively for:

### Project Improvement

Suggestions and bug reports help us prioritize development of the fork.

### Community Communication

Discord is our main communication channel. Your interactions help us build community.

### Transparent Decision Making

Voting helps us know which features to prioritize. Results are public.

**What we do NOT do (and never will):**

- We do not sell or share personal data with third parties
- We do not implement telemetry in the panel
- We do not collect panel usage statistics
- We have no mandatory cloud services
- We do not monetize user data
- The main panel is 100% free and open source (Apache 2.0) — we will never put panel features behind a paywall

The main panel is and will always be completely free and open source. The only addons that could have a cost are integrations with third-party closed-source software (like WHMCS), whose development and maintenance requires covering licenses and infrastructure. These will be sold at the lowest possible price to only cover costs.

## Open Source Transparency

Being published under the Apache 2.0 license, the complete source code is available for public audit:

- Public GitHub repository with full change history
- Apache 2.0 license — you can use, modify, and distribute without restrictions
- Anyone can audit the code to verify no data collection exists
- No closed pre-compiled binaries — everything builds from source
- No mandatory external services — the panel starts and runs without internet access, but installing game servers, pulling container images and browsing plugins or modpacks need it (see [External services the panel contacts](#external-services-the-panel-contacts))
- Community contributions are welcome and transparent

## Self-Hosted Nature

Aether Panel is designed to be self-hosted. This means:

- You control where and how the panel runs
- All data (users, servers, configurations) stays on your infrastructure
- No mandatory outbound connections — the panel works without internet access, except for the features that download something or talk to a service you configured (see [External services the panel contacts](#external-services-the-panel-contacts))
- You are responsible for the security and privacy of data in your installation
- You can audit the panel's network traffic to verify no unwanted communication

## Data Storage

Where and how data is stored depending on context:

### Panel Installation (Your Server)

Local database (SQLite by default; MySQL/MariaDB, PostgreSQL or SQL Server per your config). The personal data it holds:

- Accounts: username, email, bcrypt-hashed password, role and permissions, creation and update dates. Accounts invited to a server by email get a random username until the person changes it.
- Two-factor authentication: the TOTP secret (stored as it is, because the panel needs it to check codes) and the recovery codes (stored as hashes).
- SSH keys added for SFTP: name, public key, type and fingerprint.
- Sessions: a SHA-256 hash of the token, the account and the expiry time. Password reset tokens are also stored as SHA-256 hashes and last 30 minutes. OAuth2 client secrets are stored as bcrypt hashes and billing API keys as SHA-256 hashes.
- Access log: sign-ins, failed sign-ins, sign-outs, registrations, password resets and sign-in links, each with the username, email, IP address, browser user agent and time (see [Activity and access log](../features/activity-and-access-log.md)).
- Server activity: who did what on each server (username, action, a short detail and IP address).
- Per-user interface preferences (user settings).

Outside the database, on the panel's disk:

- Panel configuration (`config.json`, `/etc/SkyPanel/config.json` on Linux installs), and some settings in the `panel_settings` table
- Game server files, backups, file history and crash reports (crash reports keep console lines, which can include player names)
- Panel logs: each HTTP request is logged with its time, status, client IP address, method and path (secrets in paths are masked)

### Community (Firebase/Firestore)

Data you voluntarily provide on the website:

- Suggestions and votes (Firestore)
- Testimonials (if you choose to submit one)
- Contact messages (sent to Discord via webhook)

### Data Retention

Retention policy:

- Panel data: you keep it until you decide to delete it, except what the panel prunes on its own:
  - Access log entries older than 90 days are deleted whenever the access log is read.
  - Server activity entries older than 90 days are deleted (checked at most once an hour, when new activity is recorded).
  - Rotated panel log files are deleted after `logRetentionDays` days (14 by default; `0` keeps them all).
  - A server keeps its 10 newest crash reports.
  - Sessions stop working after 1 hour without activity; they are deleted on sign-out, on a password change and when the account is deleted. Expired ones are removed from the database at most once an hour, when a new session is created.
  - Deleting an account deletes its permissions, OAuth2 clients, sessions, 2FA recovery codes, pending password resets, SSH keys and interface preferences. Its access log and server activity entries stay until the 90-day pruning removes them.
- Discord accounts: as long as you are a server member
- Suggestions and votes: kept as project reference
- Contributed code: permanent (as part of the open source repository)
- You can request deletion of your community data at any time

### Cookies and browser storage

The panel sets these cookies:

| Cookie | Set by | What it holds | Lifetime |
|--------|--------|---------------|----------|
| `skypanel_auth` | Panel, on sign-in | The session token. `HttpOnly`, `SameSite=Lax`, and `Secure` whenever the panel is reached over HTTPS. | 1 hour, renewed while you use the panel |
| `session` | Panel, during a sign-in with two-factor authentication | The account's email and the time, between the password step and the code step. Signed with the panel's session key. Its options come from `panel.web.cookies.*`. | `panel.web.cookies.age` (30 days by default) |
| `sidebar_state` | The web interface | Whether the sidebar is open or collapsed | 7 days |

The web interface also keeps preferences in the browser's local and session storage (interface language, the last tab opened on each server, dismissed notices, whether the update check already ran). None of these cookies or storage entries is used for tracking or advertising.

## External services the panel contacts

The panel does not send telemetry. These are the outside services it can talk to, and when. "Browser" means the visitor's browser makes the request, so the service sees the visitor's IP address; "panel" or "node" means your server makes it.

| Service | When | Who connects | What is sent |
|---------|------|--------------|--------------|
| Kodama avatars (`api.kodama.sh`) | **Always**, wherever an account avatar is shown | Browser | A seed derived from the account with a one-way hash (never the email or username itself) and display options. There is no setting to turn it off; if the service is unreachable, initials are shown instead. |
| GitHub API (`api.github.com`) | When someone with `settings.edit` opens the panel (once per browser session) and in the update screen | Panel | Requests for the latest commits of the `Aether-Panel/Panel` repository. No account data. See [Panel updates](../features/panel-updates.md). |
| Container registries (GitHub Container Registry, Docker Hub…) | When a panel update is applied and when a server's image is pulled | Panel / node | Image pulls. |
| Game and runtime downloads (Mojang, PaperMC, Fabric, Forge, NeoForge, Quilt, Spigot, Sponge, Magma, Mohist, Adoptium, Node.js, Steam, GitHub releases…) | When a server is installed or its version changed, and when version lists are shown, depending on its template | Node | Download requests. See [Game versions](../features/game-versions.md). |
| Template repositories | Only if repositories (or `templates.url`) are configured | Panel | Requests for the template files. See [Templates](../features/templates.md). |
| Modrinth, CurseForge | When someone browses or installs plugins and modpacks | Node | Search terms and project/version identifiers. See [Modpacks and plugins](../features/modpacks-and-plugins.md). |
| Google Gemini (`generativelanguage.googleapis.com`) | **Optional**: only with a Gemini API key, and only when someone uses the assistant or an AI analysis | Panel | Chat messages, selected console lines and the server context described in [AI assistant](../features/ai-assistant.md). Console lines can include player names. |
| Cloudflare Turnstile (`challenges.cloudflare.com`) | **Optional**: only when Turnstile is enabled with its keys | Browser and panel | The browser loads Cloudflare's widget; the panel sends the challenge token and the visitor's IP address to verify it. The built-in check used by default needs no outside service. See [Turnstile](./17-turnstile.md). |
| Cloudflare DNS API (`api.cloudflare.com`) | **Optional**: only with a Cloudflare token and zone for subdomains | Panel | DNS record names and targets. See [Subdomains](../features/subdomains.md). |
| Discord webhooks | **Optional**: only when a webhook URL is set (panel-wide or per server) | Panel / node | Event notices: server name, event, and for panel events details such as the node or transfer. See [Server notifications](../features/server-notifications.md) and [Discord](./16-discord.md). |
| Telegram Bot API (`api.telegram.org`) and custom JSON webhooks | **Optional**: only when a server owner sets them up | Node | The same server event notices. |
| Email provider (SMTP server, Mailgun, Mailjet or SendGrid) | **Optional**: only when `panel.email.provider` is set | Panel | Recipient address and the email content (account creation, invitations, password resets…). See [Email](./15-email.md). |
| S3-compatible storage (AWS S3, Cloudflare R2, MinIO…) | **Optional**: only when off-site backups are enabled | Node | Copies of the backup archives. See [Backups](../features/backups.md). |
| Extension webhooks | **Optional**: only for extensions an administrator installs and turns on | Panel | The events each extension subscribed to, signed. See [Extensions](../reference/15-extensions.md#events). |
| Another Aether Panel | **Optional**: only during an external transfer an administrator starts | Panel | The server's files and definition. See [Server transfers](../features/server-transfers.md). |
| License server (`prueba.skyhostingcloud.com`) | **Optional**: only when an administrator activates a license key | Panel | The license key, the machine's hostname and its IP address. See [License](./14-license.md). |
| Database hosts | **Optional**: only the ones an administrator adds | Panel | Database and user management commands. See [Database hosts](../features/database-hosts.md). |

## Your Rights

We respect your rights over your personal data:

- **Right of Access**: You can request a copy of the data we hold about you in our community.
- **Right of Deletion**: You can request deletion of your data from our community at any time.
- **Right of Portability**: You can export data from your panel installation directly from the database.

### How to Exercise Your Rights

For requests related to data in our community:

- Contact us through Discord
- Specify which data you want to review or delete
- We will process your request within a reasonable timeframe

For data in your local panel installation, you have full direct control. We cannot access your installation.

## Third-Party Services

The project uses minimal external services, each with independent privacy policies:

- **Discord**: Our community server. Subject to Discord's privacy policy. We only access public profile information.
- **GitHub**: Source code, issues, and pull requests. Everything is public. Subject to GitHub's privacy policy.
- **Firebase (Google)**: We use Firestore to store suggestions, votes, and testimonials on the website. No sensitive data stored. Subject to Google's policy.
- **PayPal**: For voluntary donations. We do not store or process payment information — everything is handled through PayPal.

We do not share data with third parties for advertising, marketing, or any commercial purpose.

## Minors

The project is not directed at individuals under 13. We do not intentionally collect information from minors.

## Changes to This Policy

If we update this policy:

- We will post an announcement on our Discord
- Update the date on this page
- Substantial changes will be notified in the community

## Contact

For privacy questions or to exercise your rights:

- Discord: Join our server and talk to the team
- GitHub: Open an issue in the repository
- The project is maintained by volunteers — no commercial support

As a non-profit open source project, we respond to the best of our ability. Your privacy matters, and we are committed to protecting it.

## In Summary

Aether Panel is an open source game server management panel. This means:

- The panel code is 100% public and auditable (Apache 2.0 license)
- The panel runs on YOUR server — we see nothing
- No telemetry, trackers, or data collection in the panel
- The main panel is free — no paid premium features
- We only collect data if you voluntarily interact with our community
- We are open source — we build in the open and contribute back to the community
- Integrations with closed-source software (WHMCS) may have a cost to cover licenses and maintenance, but the base panel is and will always be free

Don't trust us, trust the code. Everything is on GitHub for you to verify.

## Paid Modules

The main Aether Panel is and will always be 100% free and open source (Apache 2.0). However, integration modules with third-party software may have an associated cost.

We are currently developing a WHMCS integration module (billing and automation platform). WHMCS is closed-source software that requires a paid license. The WHMCS module for Aether Panel will have a cost because:

- WHMCS is proprietary software — requires a paid license for development and testing
- The module requires ongoing maintenance to maintain compatibility
- Cover infrastructure costs for development and testing
- Not for profit — will be sold at the lowest possible price

The WHMCS module is an optional addon. You don't need to buy it to use the panel. The main panel, all its features, and the panel source code remain 100% free and open source. We will never put panel features behind a paywall.

