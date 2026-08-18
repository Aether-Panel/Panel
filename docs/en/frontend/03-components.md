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

| Hook | Purpose |
|---|---|
| `use-servers` | Server CRUD |
| `use-users` | User and permissions CRUD |
| `use-nodes` | Node CRUD and deployment data |
| `use-templates` | Repositories and templates |
| `use-database-hosts` | Database hosts |
| `use-settings` | Global panel configuration |
| `use-server-settings` | Server variables/configuration |
| `use-profile` | Own profile, OTP, OAuth2 clients |
| `use-dashboard-data` | Dashboard data (uptime, servers) |
| toast (`lib/toast.ts`) | Toast notifications (sileo) |
| `use-mobile` | Mobile device detection |

## Genkit AI Integration (`src/ai/`)

- `genkit.ts` — Genkit client configuration with Google Generative AI
- `flows/` — AI flows:
  - `summarize-server-alerts.ts` — Summary of server alerts
  - `generate-troubleshooting-tips.ts` — Troubleshooting tips suggestions
- Invoked from the "AI Analyze" button in the server view

## Build Flow

```bash
npm run build    # astro build → client/frontend/dist/
npm run dev      # astro dev --port 9002 (development)
npm run start    # astro preview
npm run lint     # astro check
npm run typecheck # tsc --noEmit
```

The compiled files in `dist/` are embedded by the Go backend using `embed.FS` and served in production from the panel's binary.
