# Components and Graphical Interface (`client/frontend`)

The visual interface that users interact with (dashboard, login, web console, etc.). It uses a hybrid approach combining Astro (static rendering) and React 19 (interactive components).

## Core UI Technologies

### 1. Astro (Routing and SSR)
- Routes are determined by files in `src/pages/`.
- `.astro` pages act as containers and "hydrate" React components with directives like `client:only="react"`.

### 2. React 19 + Tailwind CSS
- All interactive components (forms, consoles, modals) are in React 19.
- 100% utility-first styling with Tailwind CSS and `tailwind-merge` + `clsx` for conditional classes.

## Base UI Components (Shadcn UI / Radix UI)

In `src/components/ui/` there are 33 components based on Radix UI primitives:

| Component | Purpose |
|---|---|
| Accordion, Collapsible | Expandable panels |
| Alert, AlertDialog | Confirmation/alert dialogs |
| Avatar | User avatars |
| Badge | Status labels/badges |
| Button | Buttons with variants |
| Calendar | Date selection |
| Card | Content cards |
| Carousel | Image carousel |
| Chart | Charts (wraps recharts) |
| Checkbox, RadioGroup, Switch | Selection inputs |
| Dialog, Sheet, Sidebar | Modal and side panels |
| DropdownMenu, Menubar | Dropdown menus |
| Form, Input, Label, Textarea, Select | Form elements |
| Popover, Tooltip | Contextual overlays |
| Progress, Skeleton | Loading indicators |
| ScrollArea | Custom scroll areas |
| Separator | Visual separators |
| Slider | Slider control |
| Table | Data tables |
| Tabs | Tabbed navigation |

> Note: toast notifications are not a `ui/` component; they are implemented with **sileo** (`src/lib/toast.ts`) and rendered with `SileoToaster.tsx`.

## Custom Components (`src/components/`)

- **AppShell.tsx** — Main dashboard layout (sidebar, header, content)
- **AuthShell.tsx** — Layout for authentication pages
- **ErrorBoundary.tsx** — React error handling
- **logo.tsx** — Panel logo component
- **page-header.tsx** — Reusable page header
- **PageWrappers.tsx** — Page wrappers with permissions
- **metrics-charts.tsx, network-usage-chart.tsx, resource-usage-chart.tsx** — Metrics charts with recharts

## Features and Domains (`src/features/`)

### Console / Terminal (`features/servers/[id]/`)
- `console-view.tsx` — Live terminal with WebSocket, ANSI color parsing (`lib/ansi-utils.tsx`), command sending
- The WebSocket connection is established to `ws[s]://host/api/servers/:id/socket?console`

### File Manager (`features/servers/[id]/`)
- `file-manager-view.tsx` — File explorer with navigation, upload, download, creation, editing, deletion
- Integration with Monaco Editor (`code-editor.tsx`) for file editing (Tokyo Night theme, lazy-loaded)
- Operations: create folder, upload file, compress/extract ZIP

### Other Features
- **auth/** — Login, registration, OTP, password recovery (forgot/reset)
- **dashboard/** — Main panel with global metrics, uptime, server status
- **servers/** — CRUD, details, management of backups, plugins, tasks, flags, database, sub-users, transfers
  - `settings-view.tsx` — **Redesigned Settings View**: 2-column layout, per-section themed cards (colors, gradient bars), sticky save bar, permission gates by role (admin vs user).
- **nodes/** — Listing, creation, detail, deployment
- **users/** — Global user CRUD, permissions
- **templates/** — Template repositories
- **settings/** — Global panel configuration, email/discord tests, license
- **roles/** — Roles and scope assignment
- **database-hosts/** — External database hosts
- **profile/** — Own profile, OTP, OAuth2 clients

## Global State (`src/contexts/`)

| Context | File | Purpose |
|---|---|---|
| **AuthContext** | `providers.tsx` | User session (login, register, logout, role, scopes, hasScope). Redirects to `/login/` if no session. |
| **ConfigContext** | `config-context.tsx` | Public panel configuration from `GET /api/config` (branding, name, registration enabled, themes). |
| **TranslationsContext** | `translations-context.tsx` | Internationalization (i18n) with `en.json` and `es.json`. |

## Custom Hooks (`src/hooks/`)

| Hook | Purpose | Key Returns |
|---|---|---|
| `use-servers` | Server CRUD | `{ servers, loading, create, update, delete, start, stop, restart, kill, install, reload }` |
| `use-users` | User and permissions CRUD | `{ users, loading, create, update, delete, getPermissions, updatePermissions }` |
| `use-nodes` | Node CRUD and deployment data | `{ nodes, loading, create, update, delete, getDeployment, getFeatures, getSystem }` |
| `use-templates` | Repositories and templates | `{ templates, loading, createRepo, deleteRepo, syncRepo, createLocal, updateLocal, deleteLocal }` |
| `use-database-hosts` | Database hosts | `{ hosts, loading, create, update, delete, testConnection }` |
| `use-settings` | Global panel configuration | `{ settings, loading, saving, saveSettings, sendTestEmail, sendTestDiscord }` |
| `use-server-settings` | Server variables/configuration | `{ settings, loading, error, saveSettings, isMinecraftJava, refresh }` |
| `use-profile` | Own profile, OTP, OAuth2 clients | `{ profile, loading, update, otp: { status, enroll, validate, recovery, disable }, oauth2: { clients, create, delete } }` |
| `use-dashboard-data` | Dashboard data (uptime, servers) | `{ uptime, servers, loading, refresh }` |
| `use-user-settings` | User-specific settings | `{ value, setValue }` |
| `use-mobile` | Mobile device detection | `true` if viewport < 768px |

## Toast Notifications (`lib/toast.ts`)

Uses **sileo** library for toast notifications:

```typescript
import { sileo } from '@/lib/toast';

// Success
sileo.success({ title: 'Success', description: 'Settings saved' });

// Error
sileo.error({ title: 'Error', description: 'Failed to save' });

// Info
sileo.info({ title: 'Info', description: 'Server started' });

// Warning
sileo.warning({ title: 'Warning', description: 'High memory usage' });
```

Rendered by `SileoToaster.tsx` in `AppShell.tsx`.

## Genkit AI Integration (`src/ai/`)

- `genkit.ts` — Genkit client configuration with Google Generative AI
- `flows/` — AI flows:
  - `summarize-server-alerts.ts` — **Input:** server alerts array. **Output:** `{ summary, rootCauses[], suggestions[] }`. Prompt: analyzes alerts for patterns.
  - `generate-troubleshooting-tips.ts` — **Input:** error logs + context. **Output:** `{ tips[] }`. Prompt: generates actionable troubleshooting steps.
- Invoked from the "AI Analyze" button in the server view (requires `geminiApiKey` in config)

### AI Flow Details

| Flow | Input | Output | Prompt Focus |
|---|---|---|---|
| `summarize-server-alerts` | `logs: string[]` | `{ summary, rootCauses: string[], suggestions: string[] }` | Pattern recognition in server logs |
| `generate-troubleshooting-tips` | `logs: string[], context: object` | `{ tips: string[] }` | Actionable steps for specific errors |

## Console / Terminal (`features/servers/[id]/`)

- `console-view.tsx` — Live terminal with WebSocket (`ws[s]://host/api/servers/:id/socket?console`)
- `ansi-utils.tsx` — ANSI escape code parser → HTML spans (colors, bold, cursor movement)
- `xterm.js` — Terminal emulator (fit addon, webgl addon for performance)
- Connection auto-reconnect with exponential backoff
- Command history (up/down arrows), paste support, fullscreen

## File Manager (`features/servers/[id]/`)

- `file-manager-view.tsx` — Tree navigation, drag-drop upload, context menus
- `code-editor.tsx` — Monaco Editor integration (lazy-loaded, Tokyo Night theme)
- Operations: create file/folder, upload, download, rename, delete, zip/extract
- Syntax highlighting for 50+ languages, minimap, word wrap

## AI Flows

| File | Purpose | Input | Output |
|---|---|---|---|
| `summarize-server-alerts.ts` | Summarize server alerts | `logs: string[]` | `{ summary, rootCauses[], suggestions[] }` |
| `generate-troubleshooting-tips.ts` | Generate tips for errors | `logs: string[], context` | `{ tips: string[] }` |

## Build Flow

```bash
npm run build    # astro build → client/frontend/dist/
npm run dev      # astro dev --port 9002 (development)
npm run start    # astro preview
npm run lint     # astro check
npm run typecheck # tsc --noEmit
```

The compiled files in `dist/` are embedded by the Go backend using `embed.FS` and served in production from the panel's binary.
