# Frontend Components & Hooks Detailed Reference

Complete reference for all React components, hooks, contexts, and utilities in the Aether Panel frontend.

---

## Components Overview

```
client/frontend/src/components/
├── ui/                    # 33 Shadcn UI components (Radix UI)
├── AppShell.tsx           # Main dashboard layout
├── AuthShell.tsx          # Auth pages layout
├── ErrorBoundary.tsx      # React error boundary
├── SileoToaster.tsx       # Toast notifications
├── logo.tsx               # Panel logo
├── page-header.tsx        # Page header component
├── PageWrappers.tsx       # Permission-gated page wrappers
├── metrics-charts.tsx     # Dashboard metrics
├── network-usage-chart.tsx
├── resource-usage-chart.tsx
└── Turnstile.tsx          # Cloudflare Turnstile widget
```

---

## UI Components (Shadcn/Radix)

Located in `src/components/ui/`. All extend Radix UI primitives with Tailwind styling.

| Component | Radix Primitive | Key Props |
|-----------|----------------|-----------|
| `Accordion` | `Accordion` | `type`, `collapsible` |
| `Alert` | - | `variant` (default/destructive) |
| `AlertDialog` | `AlertDialog` | `onConfirm` |
| `Avatar` | `Avatar` | `src`, `alt`, `fallback` |
| `Badge` | - | `variant` |
| `Button` | `Button` | `variant`, `size`, `asChild` |
| `Calendar` | `Calendar` | `mode`, `selected`, `onSelect` |
| `Card` | - | `className` |
| `Checkbox` | `Checkbox` | `checked`, `onCheckedChange` |
| `Collapsible` | `Collapsible` | `open`, `onOpenChange` |
| `Dialog` | `Dialog` | `open`, `onOpenChange` |
| `DropdownMenu` | `DropdownMenu` | `trigger`, `content` |
| `Form` | `React Hook Form` | `schema`, `defaultValues` |
| `Input` | - | `type`, `placeholder`, `disabled` |
| `Label` | - | `htmlFor` |
| `Menubar` | `Menubar` | `items` |
| `Popover` | `Popover` | `open`, `onOpenChange` |
| `Progress` | `Progress` | `value`, `max` |
| `RadioGroup` | `RadioGroup` | `value`, `onValueChange` |
| `ScrollArea` | `ScrollArea` | `type` (hover/always) |
| `Select` | `Select` | `value`, `onValueChange`, `options` |
| `Separator` | `Separator` | `orientation` |
| `Sheet` | `Sheet` | `open`, `onOpenChange`, `side` |
| `Sidebar` | `Sidebar` | `collapsible`, `defaultOpen` |
| `Slider` | `Slider` | `value`, `onValueChange`, `min`, `max`, `step` |
| `Switch` | `Switch` | `checked`, `onCheckedChange` |
| `Table` | - | `columns`, `data` |
| `Tabs` | `Tabs` | `defaultValue`, `onValueChange` |
| `Textarea` | - | `rows`, `placeholder` |
| `Tooltip` | `Tooltip` | `content`, `side` |

---

## Layout Components

### AppShell.tsx

Main authenticated dashboard layout.

```tsx
<AppShell>
  <Sidebar />
  <Header />
  <Main />
</AppShell>
```

**Features:**
- Responsive sidebar (collapsible, mobile drawer)
- Header with user menu, notifications, theme toggle
- Permission-gated navigation via `hasScope()`
- WebSocket connection status indicator

**Sidebar Navigation:**
```tsx
const navItems = [
  { href: '/dashboard', label: 'Dashboard', icon: Home, scope: 'panel' },
  { href: '/servers', label: 'Servers', icon: Server, scope: 'server.view' },
  { href: '/nodes', label: 'Nodes', icon: ServerCog, scope: 'nodes.view' },
  { href: '/users', label: 'Users', icon: Users, scope: 'users.info.view' },
  // ... gated by hasScope()
];
```

---

### AuthShell.tsx

Layout for unauthenticated pages (`/login`, `/register`, `/forgot-password`, `/reset-password`).

```tsx
<AuthShell>
  <Card>
    <Form />
  </Card>
</AuthShell>
```

---

### PageWrappers.tsx

Higher-order components for permission-gated pages.

```tsx
// Server-scoped page
<ServerPageWrapper serverId="abc123" requiredScope="server.view">
  <ServerSettings />
</ServerPageWrapper>

// Admin-only page
<AdminPageWrapper>
  <AdminPanel />
</AdminPageWrapper>

// Permission-specific
<PermissionWrapper scope="server.files.edit">
  <FileManager />
</PermissionWrapper>
```

---

## Custom Components

### ErrorBoundary.tsx

```tsx
<ErrorBoundary fallback={<ErrorFallback />}>
  <App />
</ErrorBoundary>
```

**Features:**
- Catches React render errors
- Logs to Sentry (if configured)
- Shows user-friendly fallback
- "Try again" button to reset

---

### PageHeader.tsx

```tsx
<PageHeader 
  title="Server Settings"
  description="Configure your server"
  actions={<Button>Save</Button>}
  breadcrumbs={[{label: 'Servers', href: '/servers'}]}
/>
```

---

### Charts

| Component | Library | Data Source |
|-----------|---------|-------------|
| `metrics-charts.tsx` | Recharts | WebSocket stats |
| `resource-usage-chart.tsx` | Recharts | CPU/RAM/Disk |
| `network-usage-chart.tsx` | Recharts | Net RX/TX |

---

## Contexts (`src/contexts/`)

### AuthContext (`providers.tsx`)

```tsx
const { 
  user,           // Current user
  session,        // Session token
  login,          // Login function
  register,       // Register function
  logout,         // Logout function
  hasScope,       // Permission check
  scopes,         // User's scopes
  role,           // User's role
  isLoading,      // Auth state loading
} = useAuth();
```

**hasScope() Logic:**
```tsx
function hasScope(scope: string, serverId?: string): boolean {
  // 1. Check global scopes
  // 2. Check server-specific scopes (if serverId)
  // 3. Check role scopes
  // 4. Admin scope grants all
}
```

---

### ConfigContext (`config-context.tsx`)

```tsx
const { config } = useConfig();
// config = { branding, registrationEnabled, themes, turnstile }
```

**Loaded from:** `GET /api/config` on app init

---

### TranslationsContext (`translations-context.tsx`)

```tsx
const { t, locale, setLocale } = useTranslations();
// t('key') → translated string
// locale: 'en' | 'es'
```

**Files:** `en.json`, `es.json` in `src/lib/locales/`

---

## Hooks (`src/hooks/`)

### useServers

```tsx
const { 
  servers, 
  loading, 
  create, 
  update, 
  delete, 
  start, 
  stop, 
  restart, 
  kill, 
  install, 
  reload,
  refresh 
} = useServers();
```

---

### useServerSettings

```tsx
const { 
  settings, 
  loading, 
  error, 
  saveSettings, 
  isMinecraftJava, 
  refresh 
} = useServerSettings(serverId);

// saveSettings(data, canEditAdminData)
// data = { variables, flags, definition }
```

---

### useNodes

```tsx
const { 
  nodes, 
  loading, 
  create, 
  update, 
  delete, 
  getDeployment, 
  getFeatures, 
  getSystem,
  refresh 
} = useNodes();
```

---

### useTemplates

```tsx
const { 
  templates, 
  loading, 
  createRepo, 
  deleteRepo, 
  syncRepo, 
  createLocal, 
  updateLocal, 
  deleteLocal,
  refresh 
} = useTemplates();
```

---

### useDatabaseHosts

```tsx
const { 
  hosts, 
  loading, 
  create, 
  update, 
  delete, 
  testConnection,
  refresh 
} = useDatabaseHosts();
```

---

### useSettings

```tsx
const { 
  settings, 
  loading, 
  saving, 
  saveSettings, 
  sendTestEmail, 
  sendTestDiscord,
  refresh 
} = useSettings();
```

---

### useProfile

```tsx
const { 
  profile, 
  loading, 
  update, 
  otp: { status, enroll, validate, recovery, disable },
  oauth2: { clients, create, delete },
  refresh 
} = useProfile();
```

---

### useDashboardData

```tsx
const { 
  uptime, 
  servers, 
  loading, 
  refresh 
} = useDashboardData();
```

---

### useUserSettings

```tsx
const { value, setValue } = useUserSettings('theme');
// value: 'dark' | 'light' | 'system'
```

---

### useMobile

```tsx
const isMobile = useMobile();
// true if viewport < 768px
```

---

### Toast (`lib/toast.ts`)

```tsx
import { sileo } from '@/lib/toast';

sileo.success({ title: 'Success', description: 'Saved' });
sileo.error({ title: 'Error', description: 'Failed' });
sileo.info({ title: 'Info', description: 'Processing...' });
sileo.warning({ title: 'Warning', description: 'High usage' });
```

**Rendered by:** `SileoToaster.tsx` in `AppShell.tsx`

---

## Features (`src/features/`)

### Server Features (`features/servers/[id]/`)

| Component | Description |
|-----------|-------------|
| `console-view.tsx` | xterm.js WebSocket console |
| `file-manager-view.tsx` | File explorer + Monaco editor |
| `settings-view.tsx` | Server config (vars, flags, definition) |
| `ai-summary.tsx` | AI log analysis |
| `external-transfer-view.tsx` | Cross-panel migration |
| `plugins-view.tsx` | Plugin search/install |
| `database-view.tsx` | Database management |
| `backups-view.tsx` | Backup create/restore |
| `users-view.tsx` | Server user permissions |
| `tasks-view.tsx` | Cron task management |
| `flags-view.tsx` | Auto-start/restart flags |

---

### Auth (`features/auth/`)

| Component | Route | Description |
|-----------|-------|-------------|
| `login-form.tsx` | `/login` | Email/password + Turnstile |
| `register-form.tsx` | `/register` | Registration + Turnstile |
| `forgot-password-form.tsx` | `/forgot-password` | Email for reset link |
| `reset-password-form.tsx` | `/reset-password` | New password with token |

---

### Dashboard (`features/dashboard/`)

| Component | Description |
|-----------|-------------|
| `dashboard-view.tsx` | Main dashboard with metrics |
| `server-card.tsx` | Server summary card |
| `metrics-charts.tsx` | Recharts visualizations |

---

### Nodes (`features/nodes/`)

| Component | Description |
|-----------|-------------|
| `nodes-list.tsx` | Node table |
| `node-view.tsx` | Node detail + system info |
| `deployment-modal.tsx` | Remote node setup |

---

### Templates (`features/templates/`)

| Component | Description |
|-----------|-------------|
| `templates-list.tsx` | Template table |
| `template-editor.tsx` | JSON editor with validation |

---

### Settings (`features/settings/`)

| Component | Section |
|-----------|---------|
| `general-settings.tsx` | Branding, registration, theme |
| `notifications-settings.tsx` | Discord webhooks |
| `license-settings.tsx` | License activation |
| `email-settings.tsx` | SMTP/provider config |

---

### API Client (`lib/api-client.ts`)

```typescript
const api = {
  get: <T>(url: string) => Promise<T>,
  post: <T>(url: string, data: any) => Promise<T>,
  put: <T>(url: string, data: any) => Promise<T>,
  delete: <T>(url: string) => Promise<T>,
  // Auto-handles: auth headers, cookies, 401 redirect
};
```

**Features:**
- Automatic cookie-based auth
- Bearer token fallback
- 401 → redirect to `/login/`
- Type-safe responses

---

## Internationalization (`src/lib/locales/`)

### Structure
```
locales/
├── en.json
└── es.json
```

### Usage
```tsx
const { t } = useTranslations();
t('servers.settings.saveSuccess'); // "Settings saved successfully"
```

### Adding Keys
1. Add to `en.json`
2. Add to `es.json`
3. Use `t('namespace.key')` in components

---

## Build & Development

```bash
# Development
npm run dev        # astro dev --port 9002

# Build
npm run build      # astro build → dist/

# Type Check
npm run typecheck  # tsc --noEmit

# Lint
npm run lint       # astro check
```

---

## Adding a New Component

1. Create in appropriate folder (`ui/`, `components/`, `features/`)
2. Use TypeScript + Tailwind
3. Export from index.ts if shared
3. Add to relevant page/feature

---

## State Management

- **Server State**: React Query (via `use-servers`, `use-nodes`, etc.)
- **Auth State**: Context + cookies
- **UI State**: Local `useState` / `useReducer`
- **Forms**: React Hook Form + Zod validation