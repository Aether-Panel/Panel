# Routing and Pages

Routes come from the files in `src/pages/` (Astro file-based routing). Every page is a static `.astro` file that mounts one wrapper from `src/components/page-wrappers/` with `client:only="react"`; the wrapper puts the feature page inside `AppShell` or `AuthShell` (see [03. Components](./03-components.md#page-shells)). Detail pages take the id from the query string, not from the path, so the whole site stays static.

## Route map

| URL | Astro file | Wrapper → feature | Shell |
|---|---|---|---|
| `/` | `index.astro` | Meta refresh to `/dashboard/` | — |
| `/login/` | `login.astro` | `Login` → `features/auth/login/page.tsx` (sliding card) | Auth |
| `/register/` | `register.astro` | `Register` → `features/auth/register/page.tsx` (sliding card) | Auth |
| `/forgot-password/` | `forgot-password.astro` | `ForgotPassword` → `features/auth/forgot-password/page.tsx` | Auth |
| `/reset-password/?token=…` | `reset-password.astro` | `ResetPassword` → `features/auth/reset-password/page.tsx` | Auth |
| `/dashboard/` | `dashboard.astro` | `Dashboard` → `features/dashboard/page.tsx` | App |
| `/servers/` | `servers/index.astro` | `Servers` → `features/servers/page.tsx` | App |
| `/servers/view/?id=<id>` | `servers/view.astro` | `ServerDetail` → `features/servers/[id]/page.tsx` | App |
| `/nodes/` | `nodes/index.astro` | `Nodes` → `features/nodes/page.tsx` | App |
| `/nodes/view/?id=<id>` | `nodes/view.astro` | `NodeDetail` → `features/nodes/[id]/page.tsx` | App |
| `/users/` | `users.astro` | `Users` → `features/users/page.tsx` | App |
| `/roles/` | `roles.astro` | `Roles` → `features/roles/page.tsx` | App |
| `/templates/` | `templates.astro` | `Templates` → `features/templates/page.tsx` | App |
| `/database-hosts/` | `database-hosts.astro` | `DatabaseHosts` → `features/database-hosts/page.tsx` | App |
| `/settings/` | `settings.astro` | `Settings` → `features/settings/page.tsx` | App |
| `/profile/settings/` | `profile/settings.astro` | `ProfileSettings` → `features/profile/settings/page.tsx` | App |
| `/extension/?id=<ext>&page=<page>` | `extension.astro` | `Extension` → `features/extensions/page.tsx` | App |

The `[id]` folders under `features/` are only a naming convention for the detail screens; there are no dynamic Astro routes. The detail wrappers read `id` from `Astro.url.searchParams` (or `window.location.search`) and pass it as a prop, and the shell receives `currentPath` (`/servers/<id>`) for breadcrumbs and the active menu item.

Unknown paths are answered by the backend's 404 handler.

## Redirects

`AuthProvider` (`contexts/providers.tsx`) decides where a visitor may be:

- No session on a signed-in page → `/login/`. The current path is stored in `sessionStorage` (`aether:return-to`) so the user comes back to it after signing in.
- Session on an auth page → the stored path if it is one of the panel's own pages (never `/login` or `/register`, never an external URL), otherwise `/dashboard/`.
- A request that answers 401 while a page is open → `/login/?expired=1` with a "session expired" notice (see [02](./02-endpoints.md#expired-sessions)).

## Sidebar navigation

Items are visible to administrators and, for everyone else, only with one of the listed scopes. The API enforces the same rules; hiding an item is a convenience, not the protection.

| Item | Group | Required scope |
|---|---|---|
| Dashboard | Overview | none |
| Servers | Management | `login` (the list only contains servers the user may open) |
| Users | Management | `users.info.search` |
| Roles | Management | `admin` |
| Nodes | Management | `nodes.view` |
| Database hosts | Management | `admin` |
| Templates | Management | `templates.view` |
| Settings | Management | `settings.edit` |
| Extension pages | Extensions | `admin` (one item per `admin` page of an active extension) |

Account Settings and Log out are in the account menu at the bottom of the sidebar. The sidebar's collapse behaviour is described in [03](./03-components.md#appshell-signed-in-pages).

## Authentication pages

- **Sign in / sign up** share the sliding card (`features/auth/auth-card.tsx`): switching forms animates on the same page and updates the URL; the back button switches back. With registrations disabled (`registrationEnabled: false` in `/api/config`) `/register/` shows the sign-in form.
- **Sign in** sends `POST /auth/login` with email, password and the captcha token. If the account has two-factor authentication the API answers `otpNeeded` and the card switches to a 6-digit code step (`POST /auth/otp`), with the option to type a recovery code instead; the server keeps the pending sign-in for 5 minutes. A 429 shows "too many attempts".
- The sign-in page also shows notices from the URL: `?expired=1` (session expired) and `?sso=expired` / `?sso=denied` (a single-sign-on link that expired or was refused).
- **Forgot password** (`POST /auth/forgot-password`, with captcha) and **reset password** (`POST /auth/reset-password` with the `token` from the email link and the new password, with a strength meter) use `AuthFrame`.
- The captcha is the built-in proof of work or Cloudflare Turnstile, as configured (see [03](./03-components.md#captcha-componentsturnstiletsx)).

## Dashboard (`/dashboard/`)

What the dashboard shows depends on the user's scopes:

- **With `nodes.view` or `users.info.search`** (operators): a status strip (greeting, 30-day availability from `/api/uptime`, servers online, live network throughput), a low-disk alert per node whose servers' disk is nearly full (`nodes.view` only), KPI cards for CPU, memory and storage (node averages when nodes are visible, otherwise server averages) and server count, a panel info card (version, servers, users, nodes, registration state), the **network history chart** and the **recent access log** (sign-ins, failed sign-ins, registrations; needs `users.info.search`).
- **Everyone else**: a welcome header and a grid of their own server cards.

The network history chart is fed by `GET /api/nodes/network-history`, which adds up the history each node keeps (a point per minute for 24 h), so it shows traffic from before the page was opened. See [07](./07-components-detailed.md#dashboard-featuresdashboard).

## Servers (`/servers/`)

A searchable, filterable list of the user's servers (`/` focuses the search; status filter all/online/offline) with live CPU, memory, disk and players. The page only requires `login`, like the sidebar entry: `GET /api/servers` already returns only the servers the user owns or was added to, so owners and sub-users without the global `server.view` see it too. Administrators get the **Create server** dialog (`create-server-stepper.tsx`), a three-step wizard:

1. **Environment**: node (Docker is the only environment offered).
2. **Template**: repository and template, with game logos.
3. **Configuration**: name, owner, resources, ports (checked live against the node with `POST /api/nodes/:id/ports/check`) and template variables (Minecraft versions come from `/api/game-versions/:platform`).

The server is created with `PUT /api/servers/:id`. `/servers/?create=true` opens the dialog directly. Users without `admin` can only create sub-servers from a server's Splitter tab.

## Server detail (`/servers/view/?id=<id>`)

The header shows the name, status, a "suspended" badge, the connection address, the **Assistant** button (with `server.console`, see [04](./04-ai.md)) and the power buttons (start, stop, restart; **Force stop** replaces Stop, with a confirmation, when the server is still online 7 seconds after a stop and the user has `server.kill`).

Tabs are shown according to the user's scopes **on this server** (`GET /api/servers/:id?perms`): a sub-user only has what the owner granted, so panel-wide scopes are only used until those are known. `admin` and `server.admin` on the server grant every tab. On phones the tabs become a select; on desktop extra tabs collapse into a "More" menu. The last tab used is remembered per server in `localStorage` (`serverTab_<id>`).

| Tab | Shown when |
|---|---|
| Console | `server.console` (typing needs `server.console.send`) |
| Overview | always |
| File Manager | `server.files.view` |
| Settings | `server.data.view` or `server.flags.view` |
| Properties (`server.properties` editor, Beta) | Minecraft Java + experimental `propertiesEditor` + `server.files.edit` |
| Users | `server.users.view` |
| Database | `server.data.edit` |
| Backups | `server.backup.view` |
| Tasks | `server.tasks.view` |
| Activity | always |
| SFTP Access | `server.sftp` |
| Migration (external transfer) | `server.data.edit.admin` |
| Splitter | owner of the server or `server.data.edit.admin` |
| Plugins | `server.files.edit`, unless the Plugins switch in the Settings tab is off (a per-browser preference stored in `localStorage` as `pluginsEnabled_<id>`) |
| Modpacks, Map | Minecraft Java servers |
| Administration | any of `server.admin.view`, `server.admin.install.view`, `server.install`, `server.admin.transfer.view`, `server.data.edit.admin`, `server.admin.config.view`, `server.definition.edit`, `server.admin.assignments.view`, `server.delete` |
| Extension tabs | one per `server` page of an active extension |

What each tab contains is described in [07](./07-components-detailed.md#server-detail-featuresserversid). Two behaviours worth knowing:

- **Console**: live output, filters, search, command history and favourites, and an **expand mode** that opens the console over the whole page (Esc closes it). A crash banner above it shows the latest crash for 24 hours.
- **Crashes**: when the server's socket reports a crash, a toast appears wherever the user is on the page, the crash banner and the Overview's crash history refresh, and the crash dialog can ask the AI for an explanation.

### Settings tab

Sections are cards with the title and description on the left and the fields on the right (stacked on phones): General, Resources (CPU, memory, disk limits; visible with `admin`, `server.admin`, `server.admin.config.view` or `server.admin.config.manage`), Ports (editing numbers needs `server.data.edit.admin`; notes need `server.data.edit`), Auto-start, Plugins (shows or hides the Plugins tab in this browser), the template's variable groups, other variables, and Metadata (read-only installation values such as `resolved*`, `forge*`, `javaVersion*`, `build*`, `git*`; administrators only). A save bar appears at the bottom only when something changed. Below the settings card come the **game version** card (experimental `versionSwitch`), the **address** card (the server's subdomain) and the **notifications** card (per-server alerts to Discord, Telegram or a webhook).

## Other pages

| Page | Content | Visible with |
|---|---|---|
| Nodes | Node list with status and resources, low-disk alerts, and deployment instructions for a new node (a `docker compose` setup to copy) | `nodes.view` |
| Node detail | System info, disks, servers on the node, low-disk alert, edit and delete | `nodes.view` |
| Users | User list, create, edit, permissions | `users.info.search` (management actions need more scopes) |
| Roles | Roles and their scopes | `admin` |
| Templates | Repositories, local templates, editor, upload | `templates.view` (editing needs `templates.local.edit`, adding repositories `templates.repo.create`) |
| Database hosts | External MySQL/MariaDB hosts, connection test | `admin` |
| Settings | Tabs: General, Discord, Mail, Off-site backups (S3), Domains (Cloudflare), API keys, Products and Extensions (these three for `admin` only), Updates | `settings.edit` |
| Account Settings (`/profile/settings/`) | Account details, password, two-factor authentication, SSH keys, OAuth2 clients, interface language | any signed-in user |
| Extension page | An extension's admin page in a sandboxed iframe | `admin` |

## User guides

These pages describe the interface from the code side. How each feature works for users and administrators is explained in the [feature guides](../features/README.md), for example: [Dashboard and nodes](../features/dashboard-and-nodes.md), [Server settings](../features/server-settings.md), [Server statistics](../features/server-statistics.md), [Crash reports](../features/crash-reports.md), [File history](../features/file-history.md), [Backups](../features/backups.md), [Scheduled tasks](../features/scheduled-tasks.md), [Sub-servers](../features/sub-servers.md), [Server transfers](../features/server-transfers.md), [Modpacks and plugins](../features/modpacks-and-plugins.md), [World map](../features/world-map.md), [Game versions](../features/game-versions.md), [Server properties](../features/server-properties.md), [Subdomains](../features/subdomains.md), [Server notifications](../features/server-notifications.md), [Cloning](../features/cloning.md), [Activity and access log](../features/activity-and-access-log.md), [Templates](../features/templates.md), [Database hosts](../features/database-hosts.md), [Panel settings](../features/panel-settings.md), [Panel updates](../features/panel-updates.md), [Provisioning and SSO](../features/provisioning-and-sso.md) and [AI assistant](../features/ai-assistant.md).
