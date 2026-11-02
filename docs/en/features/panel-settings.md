# Panel settings

**Panel Settings** (`/settings`) is where administrators configure the panel as a whole: its name and address, the AI features, sign-up protection, notifications, mail, off-site backups, domains, billing integration, extensions, updates and the experimental features. This page explains the page itself and the **General** tab in detail, and points to the page that covers each of the other tabs.

## What it does

The page has one tab per area. On phones and tablets the tabs become a drop-down list.

| Tab | What it configures | Who sees it | Documented in |
|---|---|---|---|
| **General** | Name, Base URL, master node IP, notification language, AI, CurseForge key, sign-up and anti-bot, experimental features | `settings.edit` | This page |
| **Discord** | Discord webhooks for alerts, reports, node status and transfers | `settings.edit` | [Discord](../general/16-discord.md) |
| **Mail** | Mail provider and sender, with a test button | `settings.edit` | [Email](../general/15-email.md) |
| **Off-site backups** | S3-compatible storage for copies of the backups | `settings.edit` | [Backups](./backups.md) |
| **Domains** | Cloudflare token, zone and base domain for automatic subdomains | `settings.edit` | [Subdomains](./subdomains.md) |
| **API keys** | Keys for the provisioning API | `admin` | [Provisioning and SSO](./provisioning-and-sso.md) |
| **Products** | Plans the billing system sells | `admin` | [Provisioning and SSO](./provisioning-and-sso.md) |
| **Extensions** | Installed extensions | `admin` | [Extensions](../reference/15-extensions.md) |
| **Updates** | Version check and one-click update | `settings.edit` | [Panel updates](./panel-updates.md) |

Users without `settings.edit` who open `/settings` are sent back to the dashboard. A tab can be opened directly with `?tab=<name>` (for example `/settings?tab=mail`).

## How it works

- **General**, **Discord**, **Mail**, **Off-site backups** and **Domains** share one save bar. Changes in any of them are kept until **Save Changes** is pressed; **Discard** returns to the saved values. Everything is sent in one request, `POST /api/settings`.
- The panel validates every value first (Discord webhook URLs, for instance) so a bad value does not leave the settings half saved. Then each key is written to the panel's `config.json` (the file in `SKYPANEL_CONFIG`, `/etc/SkyPanel/config.json` or `config.json` in the working directory) and applied at once; no restart is needed.
- Secrets are never sent back: `GET /api/settings` and `GET /api/settings/:key` answer `********` instead of their value (an empty secret stays empty, so the page can tell whether it is set). They are `panel.email.key`, `panel.email.password`, `panel.settings.geminiApiKey`, `daemon.curseforge.key`, the four `panel.notifications.discordWebhook*`, `panel.license.key`, `daemon.backups.s3.secretKey`, `panel.dns.cloudflare.token` and `panel.turnstile.secretKey`. The page sends `********` back when the field is not touched, and the panel reads it as "keep the saved value", both when saving (`POST /api/settings`, `PUT /api/settings/:key`) and in the test buttons (email, Discord, S3 and Cloudflare). To change a secret, type the new value.
- **Global Master Node IP** is the exception: it is stored in the database (`panel_settings` table, key `master_node_ip`) so that several panels sharing a database read the same value.
- **API keys**, **Products**, **Extensions** and **Updates** save through their own buttons and routes.

### The General tab

**Panel**

- **Company Name**: the name shown in the web, in the browser's web manifest, in emails and as the issuer of two-factor codes.
- **Base URL**: the address the panel is opened at, for example `https://panel.example.com` or `http://IP:8080`. It is used in email links, as an allowed origin and as the address of the panel's own node. On installs made with the installer, saving a domain here makes the server set up Nginx, the HTTPS certificate and the firewall by itself; the result shows under the field. See [Installation: adding a domain later](../setup/01-installation.md).
- **Global Master Node IP**: the address players and SFTP clients get for the panel's own node. When empty, the panel uses the server's IP if the Base URL's domain sits behind a proxy that only passes the website (Cloudflare's orange cloud), and otherwise the Base URL's host. Useful when several panels share one database.

**Notifications**

- **Notification language** (`panel.settings.notificationLanguage`): **Spanish (Spain)** (`es`, the default) or **English (US)** (`en`). It is the language of what the panel sends outside its own pages: the panel's Discord alerts and reports, crash alerts, backup status texts, each server's Discord, Telegram and webhook alerts and their test message, the title and message of the events extensions receive, and the Discord reports of panel-to-panel migrations. The panel's pages keep following each person's own language. Any other value is refused with `ErrNotificationLanguage`. The panel sends it to every remote node with its 30-second status poll, and at once when it is saved with a new value, so the alerts of servers on remote nodes follow it too; a node whose own config or environment sets the key keeps that value. See [Server notifications](./server-notifications.md) and [Discord](../general/16-discord.md).

**Artificial intelligence**

- **Hide AI Console Analysis**: turns the AI log analysis off on every server and for every user: it hides the card under the console and **Explain with AI** in crash reports, and `POST /api/servers/:id/ai/analyze` answers `403` with `ErrAIAnalysisDisabled`. The **Assistant** chat is not affected.
- **Let the assistant work with server files** (`panel.settings.aiFileTools`, on by default): the server assistant can read, create, edit, move and delete files of the server it is opened on, within each user's file permissions, and always asks for confirmation before changing or deleting anything. Off, the assistant only answers questions. See [AI assistant: working with files](./ai-agent.md).
- **Gemini API Key** (shown while the analysis is not hidden): the Google Gemini key used by the console analysis and the server assistant. See [AI assistant](./ai-assistant.md).

**Modpacks**

- **CurseForge API key**: needed to search and install CurseForge modpacks (Modrinth needs none). The panel's own node uses this value; remote nodes read `daemon.curseforge.key` from their own configuration. See [Modpacks and plugins](./modpacks-and-plugins.md).

**Sign-up and security**

- **Allow users to register**: public sign-up. Self-registered accounts cannot do anything until they are given permissions. Turning it off does not affect server invitations or accounts created from **Users**.
- **Built-in anti-bot check**: on sign-in, sign-up and password reset the browser solves a small calculation in the background, with no outside service. On by default.
- **Cloudflare Turnstile Protection**, with **Turnstile Site Key** and **Turnstile Secret Key**: uses Turnstile instead of the built-in check. See [Turnstile](../general/17-turnstile.md).

**Experimental features**

New features still being refined. They start off; turning one off hides it again and deletes nothing.

| Switch | Where it appears | Page |
|---|---|---|
| **Game version switch** | In each server's **Settings** tab | [Game versions](./game-versions.md) |
| **Properties editor** | **Properties** tab, on Minecraft Java servers | [Server properties](./server-properties.md) |
| **Clone server** | In each server's **Administration › Clone** | [Cloning](./cloning.md) |

While **Game version switch** or **Clone server** is off, their API routes answer `403` with `ErrFeatureDisabled` ("this experimental feature is turned off in the panel settings"), so turning them off stops them, not just their buttons. The properties editor is a web-only switch.

## Using it

### In the panel

1. Open **Settings** in the sidebar (shown to administrators and anyone with `settings.edit`). The page is titled **Panel Settings**.
2. Pick a tab, change the values.
3. Press **Save Changes** in the bottom bar. A message confirms **Settings saved successfully**.

### Via the API

| Method and path | Scope | Purpose |
|---|---|---|
| `GET /api/settings` | `settings.edit` | Every editable key with its current value (secrets as `********`), plus `panel.settings.masterNodeIp` |
| `POST /api/settings` | `settings.edit` | Save several keys at once: `{"key": value, ...}` |
| `GET /api/settings/:key` | `settings.edit` | Read one key (`204` when it is not editable; a secret as `********`) |
| `PUT /api/settings/:key` | `settings.edit` | Save one key: `{"value": ...}` (`"********"` on a secret leaves it unchanged) |
| `GET /api/config` | public | Name, theme, registration, captcha, `hideAiAnalysis`, experimental flags and version, as the web reads them |

Example:

```bash
curl -X POST https://panel.example.com/api/settings \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"panel.settings.companyName": "Example Hosting", "panel.experimental.propertiesEditor": true}'
```

Unknown keys are ignored. The test routes (`/api/settings/test/email`, `/test/discord`, `/test/s3`, `/test/cloudflare`) and the update routes are described in their own pages and in the [API reference](../reference/11-api-reference.md).

## Requirements and configuration

The keys behind the General tab, with their defaults and environment variables, are listed in [Configuration](../backend/02-configuration.md):

`panel.settings.companyName`, `panel.settings.masterUrl`, `panel.settings.notificationLanguage`, `panel.settings.geminiApiKey`, `panel.settings.hideAiAnalysis`, `panel.settings.aiFileTools`, `daemon.curseforge.key`, `panel.registrationEnabled`, `panel.captcha.builtin`, `panel.turnstile.enabled`, `panel.turnstile.siteKey`, `panel.turnstile.secretKey`, `panel.experimental.versionSwitch`, `panel.experimental.propertiesEditor`, `panel.experimental.serverClone`.

Environment variables use the prefix `SKYPANEL_` and the key in upper case with dots turned into underscores, for example `SKYPANEL_PANEL_EXPERIMENTAL_SERVERCLONE`.

## Limits

- The API does not return the secrets, but anyone with `settings.edit` can replace them (for example, point mail or off-site backups at another service). Give that scope only to trusted administrators.
- Saving rewrites `config.json` as a whole, so its formatting and key order may change.
- **CurseForge API key** only applies to the panel's own node; remote nodes use their own configuration. **Notification language** reaches remote nodes with the status poll, unless a node sets its own.

## Troubleshooting

| What you see | Cause | Fix |
|---|---|---|
| Sent back to the dashboard when opening `/settings` | No `settings.edit` | Ask an administrator for the scope |
| `Failed to save settings` or a validation message | A value was refused (for example a Discord URL that is not a webhook) | Fix the value; nothing was saved |
| "The notification language must be Spanish or English." (`ErrNotificationLanguage`) | A value other than `es` or `en` was sent for `panel.settings.notificationLanguage` | Send `es` or `en` |
| Discord test: "The test message did not reach these webhooks: …" (`422`, `ErrDiscordTestFailed`) | The listed webhooks (named after the Discord tab's sections: **Server alerts**, **System reports**, **Node status**, **ExTransfer**) were deleted or are wrong | Create the webhook again in Discord and paste the new URL. See [Discord](../general/16-discord.md) |
| Saving fails with a file error | The panel cannot write `config.json` | Check the file's owner and permissions |
| The domain does not get HTTPS after saving the Base URL | The domain does not point at the server yet, port 80 is closed, or the install has no `aether-domain` service | See [Installation](../setup/01-installation.md) |
| `ErrFeatureDisabled` from the API | The experimental feature is off | Turn it on under **Experimental features** |

## See also

- [Configuration](../backend/02-configuration.md)
- [Security](../general/security.md)
- [Scopes reference](../general/scopes-reference.md)
