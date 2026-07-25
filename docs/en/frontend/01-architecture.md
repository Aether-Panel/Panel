# Frontend (Client) Architecture

This document details the structure, technologies, and general architecture of the Aether Panel user interface (frontend).

## Technology Stack

| Technology | Version | Usage |
|---|---|---|
| **Astro** | ^6.4.6 | Web framework with Islands architecture, file-based routing (`src/pages/`) |
| **React** | ^19.2.7 | Interactive UI components |
| **Tailwind CSS** | ^3.4.1 | Utility-first styles |
| **Shadcn UI / Radix UI** | — | 35+ accessible components (Dialog, Select, Tabs, etc.) |
| **react-hook-form** | ^7.54.2 | Form handling |
| **zod** | ^3.24.2 | Schema validation |
| **Monaco Editor** | ^4.6.1 | In-browser code/configuration editor |
| **Genkit** | ^1.28.0 | Integration with Google GenAI for log analysis |
| **recharts** | ^2.15.1 | Metrics charts (CPU, RAM) |
| **lucide-react** | ^0.475.0 | Icons |
| **clsx + tailwind-merge** | — | Conditional CSS class handling |

## Directory Structure (`frontend/src/`)

```
src/
├── ai/                      # Genkit AI logic (flows, prompts)
│   ├── genkit.ts            # Genkit client configuration
│   ├── dev.ts               # Development utility
│   └── flows/               # AI flows (summarize, troubleshooting)
├── components/              # Visual components
│   ├── ui/                  # 35+ Shadcn/Radix UI components
│   ├── AppShell.tsx         # Main dashboard layout
│   ├── AuthShell.tsx        # Authentication layout
│   ├── metrics-charts.tsx   # Metrics charts
│   ├── network-usage-chart.tsx
│   ├── resource-usage-chart.tsx
│   ├── page-header.tsx
│   ├── PageWrappers.tsx
│   ├── logo.tsx
│   └── ErrorBoundary.tsx
├── contexts/                # React global state
│   ├── providers.tsx        # AuthContext + Providers wrapper
│   ├── config-context.tsx   # ConfigContext (branding, themes from /api/config)
│   └── translations-context.tsx  # i18n (en/es)
├── features/                # Logic grouped by domain
│   ├── auth/                # Login, registration
│   ├── dashboard/           # Main panel with global metrics
│   ├── database-hosts/      # Database host management
│   ├── nodes/               # Node management
│   ├── profile/             # User profile
│   ├── roles/               # Roles and permissions
│   ├── servers/             # CRUD, console, files, backups, plugins
│   ├── settings/            # Panel configuration
│   ├── templates/           # Server templates
│   └── users/               # Global users
├── hooks/                   # Custom React hooks
│   ├── use-dashboard-data.ts
│   ├── use-database-hosts.ts
│   ├── use-mobile.tsx
│   ├── use-nodes.ts
│   ├── use-profile.ts
│   ├── use-server-settings.ts
│   ├── use-servers.ts
│   ├── use-settings.ts
│   ├── use-templates.ts
│   ├── use-toast.ts
│   └── use-users.ts
├── layouts/
│   └── BaseLayout.astro     # Base HTML layout
├── lib/                     # Utilities and configuration
│   ├── api-client.ts        # HTTP SDK (native fetch, singleton api object)
│   ├── utils.ts             # General utilities (cn())
│   ├── ansi-utils.tsx       # ANSI color parsing for console
│   ├── data.ts              # Data and constants
│   └── locales/             # Translations (en.json, es.json)
├── pages/                   # Astro routes (file-based routing)
│   ├── index.astro
│   ├── login.astro
│   ├── register.astro
│   ├── dashboard.astro
│   ├── servers/             # Server CRUD
│   ├── nodes/               # Node details
│   ├── users.astro
│   ├── templates.astro
│   ├── settings.astro
│   ├── roles.astro
│   ├── database-hosts.astro
│   └── profile/
└── styles/
    └── globals.css          # Global styles and CSS variables
```

## API SDK (`lib/api-client.ts`)

There is no separate `client/api/` package. The SDK is a single 81-line file in `frontend/src/lib/api-client.ts` that exports an `api` object with `get`, `post`, `put`, `delete` methods using the **native `fetch` API** of the browser with `credentials: 'include'` for cookie/session authentication.

API calls are made directly from features and hooks, not from separate domain classes:

```typescript
import { api } from '@/lib/api-client';
const servers = await api.get('/api/servers');
await api.post('/api/servers/' + id + '/start', {});
```

## Native HTTP Client

The SDK uses native `fetch` (no axios, no js-cookie). Requests automatically include session cookies thanks to `credentials: 'include'`. HTTP errors are converted to `ApiError` with `status`, `message`, and `data`.

## Compilation

The `frontend/` package is compiled with `astro build`, generating a static site in `frontend/dist/`. The Go server embeds these files using `embed.FS` and serves them in production.

```bash
cd client/frontend
npm run build    # Generates client/frontend/dist/
# Optional: preview
npm run start    # astro preview
npm run dev      # astro dev --port 9002
```