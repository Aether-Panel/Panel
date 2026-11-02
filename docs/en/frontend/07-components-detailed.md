# Frontend Detailed Reference

File-by-file reference of `client/frontend/src`. It complements, and does not repeat, the other frontend documents:

- Shells, shared components, contexts, toasts and captcha: [03. Components and UI](./03-components.md).
- HTTP client, data hooks and endpoints: [02. API consumption](./02-endpoints.md).
- Routes, redirects and which scopes show each page or tab: [06. Routing and pages](./06-pages.md).
- Translations: [05](./05-translations.md). AI: [04](./04-ai.md) and [08](./08-ai-detailed.md).
- What each feature does for users: the [feature guides](../features/README.md) (listed at the end of [06](./06-pages.md#user-guides)).

## Page wrappers (`components/page-wrappers/`)

One file per route. Each one is a few lines: it picks the shell and the feature page, and for detail pages reads `id` from the query string.

```tsx
// components/page-wrappers/ServerDetail.tsx
export default function ServerDetail({ id }: Readonly<{ id?: string }>) {
    const searchParams = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
    const effectiveId = id || searchParams?.get('id') || '';
    return <AppShell currentPath={`/servers/${effectiveId}`}><ServerDetailPage params={{ id: effectiveId }} /></AppShell>;
}
```

Files: `Dashboard`, `Servers`, `ServerDetail`, `Nodes`, `NodeDetail`, `Users`, `Roles`, `Templates`, `DatabaseHosts`, `Settings`, `ProfileSettings`, `Extension` (all in `AppShell`), and `Login`, `Register`, `ForgotPassword`, `ResetPassword` (in `AuthShell`).

## Auth (`features/auth/`)

| File | Content |
|---|---|
| `auth-card.tsx` | `AuthCard`: sign-in and sign-up on one sliding card. Keeps `mode` in sync with the URL (`pushState`, `popstate`), mounts the sign-up form lazily, falls back to sign-in when registrations are closed. On desktop the `BrandPanel` (logo, 3D emblem, invitation to the other form) slides over the inactive half; hidden forms are `inert`. |
| `login/page.tsx` | `LoginPanel`: email/password form (react-hook-form + zod), captcha, notices for `?expired=1` and `?sso=expired\|denied`, then the 2FA step (6 boxes for the app code, or a recovery code). The default export renders `<AuthCard initial="login" />`. |
| `register/page.tsx` | `RegisterPanel`: username, email, password with strength meter, captcha. Default export renders `<AuthCard initial="register" />`. |
| `forgot-password/page.tsx` | Email + captcha → `POST /auth/forgot-password`, inside `AuthFrame`. |
| `reset-password/page.tsx` | New password (with strength meter) for the `?token=` of the email link → `POST /auth/reset-password`. |

## Dashboard (`features/dashboard/`)

| File | Content |
|---|---|
| `page.tsx` | Chooses the operator dashboard (with `nodes.view` or `users.info.search`) or the user dashboard (own server cards). Computes CPU, memory and storage averages from the nodes' `systemInfo` (storage uses the disk the servers live on, `serversDisk`), or from the servers when nodes are not visible. Shows a page-shaped skeleton while loading. |
| `status-strip.tsx` | Greeting, 30-day availability (from `/api/uptime`), servers online / total, live throughput. |
| `kpi-card.tsx` | KPI card with a meter; `usageTone()` is green below 60 %, amber to 80 %, red above. |
| `panel-info-card.tsx` | Panel version, servers, users, nodes and whether registration is open. |
| `network-history-chart.tsx` | Network history of all nodes (see below). |
| `access-log.tsx` | Recent sign-ins, failed sign-ins and registrations from `useAccessLog` (`GET /api/activity`), with relative times in the active locale. |

The low-disk warning comes from `features/nodes/node-disk-alert.tsx`: one line per node whose `systemInfo.serversDisk.low` is set by the node (under 10 % or 5 GB free on the servers' disk), linking to the node.

### Network history

`NetworkHistoryChart` uses `useNetworkHistory(range)` (`hooks/use-network-history.ts`):

- `GET /api/nodes/network-history?range=1h|6h|24h` (requires `nodes.view`). The panel asks every node for `/daemon/network/history` in parallel (10 s timeout each) and adds their points up step by step. Each node measures its host's traffic and keeps a point per minute for 24 h, so the chart includes traffic from before anyone opened the dashboard.
- Response: `{ step, points: [{ t, rx, tx }], nodes, unavailable }`. `t` is epoch seconds; `rx`/`tx` are KB/s averaged over the step; the step is 1 min, 5 min or 15 min depending on the range. `unavailable` counts nodes that did not answer (down or too old to keep a history); the chart footer shows it.
- The hook refreshes every 60 s while the tab is visible, keeps the last good data on error, and shows nothing from the previous range while a new range loads. It also removes the old browser-side buffer (`localStorage` key `aether.dashboard.network-history.v1`) used before the nodes kept the history.
- The legend shows the live rate from `useGlobalNetworkMetrics` (differences between consecutive readings of each node's cumulative counters, treating a lower reading as a counter reset), falling back to the last history point.
- Axis helpers: `timeTicks()` puts labels on round clock times (every 15 min, 1 h or 4 h) and `rateTicks()` picks round rate steps in one unit (B/s, KB/s, MB/s, GB/s).

## Servers (`features/servers/`)

| File | Content |
|---|---|
| `page.tsx` | Server list: search (`/` focuses it), status filter, cards with live stats from `useServers`. Administrators get the create dialog; `?create=true` opens it. |
| `create-server-stepper.tsx` | Three-step creation wizard (environment, template, configuration) described in [06](./06-pages.md#servers-servers): template variables fill the resource fields, ports are checked against the node as they are typed, Java versions are limited to those with an `eclipse-temurin` image, field errors move the focus to the field. Creates with `PUT /api/servers/:id`. |

## Server detail (`features/servers/[id]/`)

`page.tsx` loads the server, its per-server permissions (`GET /api/servers/:id?perms`) and detail, owns the console WebSocket ([02](./02-endpoints.md#console-websocket)) and the power actions, and renders the header and tabs. Each tab is wrapped in its own `ErrorBoundary`, so one failing tab does not blank the page. Screen readers hear status changes and crashes through a polite live region, not every log line.

| Tab / piece | Files | What it does |
|---|---|---|
| Console | `console-view.tsx`, `ai-summary.tsx`, `crash-report.tsx` | See [Console](#console-console-viewtsx). |
| Overview | `page.tsx` (`ServerOverviewCards`), `performance-history.tsx`, `peak-hours.tsx`, `metrics-charts.tsx`, `network-usage-chart.tsx`, `crash-report.tsx` | Resource cards, history, peak hours, live charts and crash history. See [Charts](#charts). |
| File Manager | `file-manager-view.tsx`, `code-editor.tsx`, `file-history-sheet.tsx`, `file-diff-view.tsx`, `file-diff-editor.tsx` | See [File manager](#file-manager). |
| Settings | `settings-view.tsx`, `version-card.tsx`, `address-card.tsx`, `server-address.tsx`, `notifications-card.tsx` | Variables, resources, ports with notes, auto-start, plugin tab switch, metadata; one-click game version change (experimental: stops the server, takes a restore point, installs and starts again); subdomain (3–32 lowercase letters, digits and inner hyphens); per-server notifications with a Discord-style preview. |
| Properties | `properties-view.tsx`, `server-properties.ts`, `server-properties-catalog.ts` | Form editor for `server.properties` with a catalogue of known keys and a MOTD editor (experimental, Minecraft Java). |
| Users | `users-view.tsx` | Sub-users and their permissions; the owner can only grant scopes they hold (the backend's `OwnerScopes`). `server.view` is always included. |
| Database | `database-view.tsx` | Databases on the database hosts available to the server. |
| Backups | `backups-view.tsx` | List, create (optional name), restore, download, delete; backups made by scheduled tasks are labelled as such. |
| Tasks | `tasks-view.tsx`, `task-dialog.tsx`, `task-schedule.ts` | Scheduled tasks: schedule editor and an ordered list of steps (`stdin` command, `sleep`, `restart`, `stop`, `start`, `backup`), run now. |
| Activity | `activity-view.tsx` | Paginated activity log (`GET /api/servers/:id/activity?limit=&before=&type=`), filterable by family. |
| SFTP Access | `sftp-view.tsx` | Connection details with copy buttons. |
| Migration | `external-transfer-view.tsx` | Move a server from or to another panel (create, pull, status, cancel). The import status arrives as codes (`STEP_*`, `ERROR_*`, `DONE`...) that the view translates under `servers.extransfer.progress`, falling back to the API's English `message`. |
| Splitter | `splitter-view.tsx` | Split the server's CPU, memory and disk into sub-servers, with a resource strip showing what is used and free. |
| Plugins | `plugins-view.tsx` | Search and install plugins or mods, remove installed ones. The search placeholder follows the platform (plugins on Paper, mods on Fabric, Forge and NeoForge); `ErrPluginNoCompatibleVersion` and `ErrPluginBadFileName` are shown with their own text. |
| Modpacks | `modpacks-view.tsx`, `modpack-install-dialog.tsx`, `modpack-status.tsx`, `modpack-shared.tsx` | One-click modpacks from Modrinth or CurseForge; the install dialog spells out each step and keeps "new world" behind a switch that is off by default; progress is polled. |
| Map | `world-map-view.tsx`, `world-map-choice.tsx`, `world-map-frame.tsx`, `world-map-types.ts` | Install BlueMap or Dynmap (BlueMap asks for consent to Mojang's download) and show the map. |
| Administration | `admin-view.tsx`, `clone-server-card.tsx` | Install status and reinstall, transfer to another node, edit definition, assignments (owner), suspend, delete, and clone (experimental `serverClone`, needs `server.create` or `admin`). |
| Header | `server-tab-nav.tsx`, `assistant-panel.tsx`, `server-address.tsx` | Desktop tab bar with a "More" overflow and Beta tags; the AI assistant; the connection address (follows subdomain changes through a window event). |

### Console (`console-view.tsx`)

The page owns the socket and passes `logs`, `status` and `canSend` (false for a sub-user who may read but not type). The view:

- Fills the space down to the page's bottom padding (never under 420 px) and follows new output only while scrolled to the bottom; scrolling up pauses it and offers a way back.
- **Expand mode**: the expand button opens the console over the whole page (rendered in a portal on `<body>` with a dimmed backdrop); **Esc**, the backdrop or the same button returns it. Its place in the page keeps its height meanwhile.
- **Filters**: all / errors / warnings, with counts (lines containing `ERROR`, `FATAL`, `EXCEPTION` or `WARN`).
- **Search**: **Ctrl/⌘+F** inside the console opens a search box (matches ignore ANSI codes), with previous/next and the current match scrolled into view.
- **Command history**: ↑/↓ walk the last 50 commands, Tab completes from history; **favourites** (up to 12). Both are stored per server in `localStorage` (`consoleHistory_<id>`, `consoleFavorites_<id>`).
- Commands are sent with `POST /api/servers/:id/console` (plain-text body); a failure shows a toast and a line in the console.
- Colour is reserved for warnings, errors and the user's own commands; ANSI colours are rendered by `lib/ansi-utils.tsx`. When most lines already carry their own timestamp, the received-at column is hidden.
- A short "connected" toast appears when the console reconnects after a drop (not on the first connection).
- Below the console, `ai-summary.tsx` lists recent lines (last 50) that look like problems and asks the AI to explain the selected ones, unless `panel.settings.hideAiAnalysis` is set (see [04](./04-ai.md)).

### Crashes (`crash-report.tsx`)

- `useCrashReports(serverId)` reads `GET /api/servers/:id/crashes`. A crash report has `id`, `time`, `exitCode`, `lines` (console before the crash), `restarted`, `attempt` and `limit`.
- When the socket sends `{ type: 'crash' }`, the page shows an error toast (title with the exit code, description from `crashSummary()`: what the exit code usually means for 1, 134, 137, 139 and 143, and whether the server was restarted, with attempt / limit). The socket event only carries `{id, exitCode, restarted}`, so for an automatic restart the page reads `GET /api/servers/:id/crashes` and takes the attempt and limit from the report with that `id`; when it cannot, the description is just "Restarted". It also fires the `aether:server-crash` window event so the banner and history refresh.
- `CrashBanner` shows the newest crash above the console for 24 hours or until dismissed. `CrashHistory` lists recent crashes in Overview. `CrashReportDialog` shows the full report, highlights error-looking lines and offers "Explain with AI".

### File manager

- `file-manager-view.tsx`: breadcrumb navigation, upload, download, new file and folder, rename/move, delete, extract archives into the current folder, and compress a file or folder ("Compress" in each row's menu: asks for a name, accepts `.zip`, `.tar` and `.tar.gz`, adds `.zip` to any other name and never overwrites; see [Files and compression](../general/19-files-compression.md#compressing-from-the-file-manager)).
- `code-editor.tsx`: Monaco, loaded lazily, with a Tokyo Night theme, the JSON/CSS/HTML/TypeScript workers and Ctrl/⌘+S to save. `lib/monaco-setup.ts` keeps Monaco's styles attached to `<html>` so they survive Astro page swaps.
- `file-history-sheet.tsx`: previous versions of a text file kept by the node on each save (up to 20 per file; config-like extensions such as `properties`, `yml`, `json`, `toml`, `conf`, `cfg`, `ini`, `txt`, `xml`, `js`, `ts`, `lua`, `env`, and files without an extension), with view, compare and restore. `file-diff-view.tsx` / `file-diff-editor.tsx` show the comparison side by side or inline on narrow screens, folding long unchanged runs.

## Charts

| Component | Data | Notes |
|---|---|---|
| `features/servers/[id]/metrics-charts.tsx` | Live CPU and memory samples gathered in the browser by `useServers` (every 5 s) | CPU and memory are shares of the server's own limits (`maxCpu` from the stats, or the node's cores). |
| `features/servers/[id]/network-usage-chart.tsx` | Same live samples | Download / upload in KB/s formatted as B/s…MB/s. |
| `features/servers/[id]/performance-history.tsx` | `GET /api/servers/:id/metrics?range=1h\|24h\|7d` (recorded by the node) | Views for resources (CPU and memory), network (download/upload) and players (average and peak); offline stretches shaded, gaps where nothing was recorded, crashes marked. |
| `features/servers/[id]/peak-hours.tsx` | `GET /api/servers/:id/metrics?range=7d` | Average players per weekday and hour in the viewer's time zone; hidden for servers that never reported players. |
| `features/dashboard/network-history-chart.tsx` | `GET /api/nodes/network-history` | See [Network history](#network-history). |
| `components/server-card.tsx` (`Sparkline`) | Live samples | Small trend line on a fixed 0–100 scale for percentages. |

All of them take colours from `CHART_COLORS` and format times with `chartLocale()` / `formatClock()` (`lib/chart-theme.ts`).

## Nodes, users, roles, templates, database hosts

| Folder | Content |
|---|---|
| `features/nodes/page.tsx` | Node list with status (including "deploying"), create, edit, delete and the deployment instructions for a new node (directory, `docker-compose` file and `docker compose up -d`, each with a copy button). |
| `features/nodes/[id]/page.tsx` | System information, disks, servers on the node, edit (with a notice for the local node), delete. |
| `features/nodes/node-disk-alert.tsx` | `NodeDiskAlert`, used on the dashboard and node pages. |
| `features/users/page.tsx` | Users list, create, edit, permissions. |
| `features/roles/page.tsx` | Roles with their scopes, grouped as in the permission editor. |
| `features/templates/page.tsx` | Repositories, templates per repository, local template editor and upload. |
| `features/database-hosts/page.tsx` | Database hosts with a connection test (`POST /api/databasehosts/test`). |

## Settings (`features/settings/`)

| File | Tab |
|---|---|
| `page.tsx` | Tab container. Secrets arrive from `GET /api/settings` as `********` and are sent back that way when untouched, which keeps the saved value (see [Panel settings](../features/panel-settings.md#how-it-works)); API keys, Products and Extensions are only listed for `admin`. |
| `general-tab.tsx` | Sections: Panel (company name, base URL, master node IP), Notifications (`panel.settings.notificationLanguage`, Spanish or English), Artificial intelligence (Gemini API key, turn the AI log analysis off for everyone), Sign-up and security (registration, built-in captcha, Turnstile keys), Modpacks (CurseForge key) and Experimental features (`panel.experimental.versionSwitch`, `propertiesEditor`, `serverClone`). |
| `discord-tab.tsx` | Discord webhooks with test (see [Discord and notifications](../general/16-discord.md)). |
| `mail-tab.tsx` | Email provider (SMTP, Mailgun, Mailjet, SendGrid or Debug) and test (see [Email](../general/15-email.md)). For Mailjet the fields are **API Key (public)** (`panel.email.domain`) and **Secret Key** (`panel.email.key`); Debug shows no fields. |
| `offsite-tab.tsx` | Off-site backups to S3-compatible storage (`daemon.backups.s3.*`) with a connection test. |
| `domains-tab.tsx` | Cloudflare domain for server subdomains, with status and test. |
| `api-keys-view.tsx` | Panel API keys. |
| `products-view.tsx` | Provisioning products (`/api/provision/products`). |
| `extensions-view.tsx` | Install (upload), enable, test and remove extensions. |
| `updates-tab.tsx` | Version check and panel update (`/api/settings/update-check`, `/update-status`, `/update-panel`). |

## Profile (`features/profile/settings/`)

`page.tsx` has sections for account details, password, two-factor authentication (enrolment with QR code, recovery codes, disable), SSH keys (`ssh-keys.tsx`: add and delete public keys used for SFTP; errors such as a weak, duplicated or private key are localized), OAuth2 clients and the interface language.

## Extensions (`features/extensions/`)

- `page.tsx`: an extension's admin page at `/extension/?id=<ext>&page=<page>`.
- `extension-frame.tsx`: renders an extension page in a **sandboxed iframe** (opaque origin: no panel cookies, storage or API). After loading it receives `{ type: 'aether:context', serverId, language, theme }` by `postMessage`, and it may answer `{ type: 'aether:resize', height }` to fit its content.
- Server pages of extensions appear as extra tabs on the server detail page; admin pages appear in the sidebar's Extensions group.

## Utilities (`src/lib/`)

| File | Exports |
|---|---|
| `api-client.ts` | `api`, `ApiError`, `rememberPageForLogin`, `RETURN_TO_KEY` ([02](./02-endpoints.md)). |
| `paged.ts` | `fetchAllPages(url, key)`. |
| `chart-theme.ts` | `CHART_COLORS`, `chartLocale`, `formatClock`. |
| `toast.ts` | `sileo` wrapper ([03](./03-components.md#toasts-libtoastts)). |
| `ansi-utils.tsx` | `AnsiText` and ANSI parsing for console lines. |
| `data.ts` | Shared `Server`, `User`, `Node` types. |
| `players.ts` | `parsePlayerQuery()` for `/api/servers/:id/query` (`{ minecraft }` or `{ source }` answers). |
| `game-logos.ts` | `gameLogo(template)`: logo from the template's `icon` field, or by name for older templates. |
| `keyed.ts` | `keyedByContent()`: stable React keys for lists without ids. |
| `stale-build.ts` | `isStaleBuildError`, `reloadForNewBuild` (reload once per minute after a panel update). |
| `proof-of-work.ts` | Built-in captcha solver (ALTCHA scheme, pure-JS SHA-256). |
| `monaco-setup.ts` | `ensureMonacoStylesheet`, `markMonacoInUse`. |
| `emblem-3d.ts` | `mountEmblem()`: the 3D emblem, built in slices so the page stays responsive. |
| `utils.ts` | `cn()`, `formatBytes()`. |

## Conventions

- **State**: data lives in hooks and component state; global state is limited to the three contexts. There is no React Query or Redux.
- **Forms**: react-hook-form with zod schemas; error messages come from the translations.
- **Text**: every visible string goes through `t()` ([05](./05-translations.md)).
- **Icons**: import from `@/components/icons`, never from the icon package directly.
- **Responsive**: every screen must work from phone to desktop. Layouts use Tailwind breakpoints (`sm`, `md`, `lg`, `xl`), and `useIsMobile()` switches to sheets and selects below 768 px.
- **New component**: primitives go in `components/ui/`, shared pieces in `components/`, screen-specific code in its `features/` folder. Wrap new server tabs in `ErrorBoundary` and gate them with `can()` in `features/servers/[id]/page.tsx`.
