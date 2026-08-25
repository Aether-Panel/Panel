# Routing and Pages (Astro)

The frontend routes are determined by the file structure in `src/pages/` (Astro file-based routing).

## Page Structure

```
src/pages/
├── index.astro              # /          → Landing/redirect to login or dashboard
├── login.astro              # /login/    → Sign in
├── register.astro           # /register/ → User registration
├── forgot-password.astro    # /forgot-password/ → Request password reset
├── reset-password.astro     # /reset-password/ → Enter new password (?token=)
├── dashboard.astro          # /dashboard/ → Main panel with global metrics
├── servers/
│   ├── index.astro          # /servers/  → Server list
│   ├── view.astro           # /servers/view/ → Individual server view
│   └── [id].astro           # /servers/:id/ → Server details (dynamic parameter)
├── nodes/
│   ├── index.astro          # /nodes/    → Node list
│   ├── view.astro           # /nodes/view/ → Individual node view
│   └── [id].astro           # /nodes/:id/ → Node details
├── users.astro              # /users/    → User administration
├── templates.astro          # /templates/ → Repositories and templates
├── settings.astro           # /settings/ → Global panel configuration
├── roles.astro              # /roles/    → Roles and permissions
├── database-hosts.astro     # /database-hosts/ → Database hosts
└── profile/
    └── settings.astro       # /profile/settings/ → User profile
```

## React Component Hydration

Astro pages act as containers that hydrate React components with the `client:only` directive:

```astro
---
// login.astro
// Loads the login component fully on the client side
---
<Layout title="Login">
  <LoginWrapper client:only="react" />
</Layout>
```

## Navigation

- **AuthShell.tsx** — Layout for pages without authentication (`/login/`, `/register/`, `/forgot-password/`, `/reset-password/`).
- **AppShell.tsx** — Main layout with sidebar and header for authenticated pages. Uses `hasScope()` to verify permissions and render sidebar links (e.g., `/servers/` requires `server.view`).
- `AuthContext` automatically redirects:
  - Unauthenticated user on a private route → `/login/`
  - Authenticated user on `/login/` or `/register/` → `/dashboard/`

## Server Settings View (`/servers/:id` → Settings)

The server **Settings** tab (`features/servers/[id]/settings-view.tsx`) was completely redesigned with:

- **2-column layout** (responsive grid: 1 col on mobile, 2 on desktop).
- **Visual identity per section**: each card has its own color/border/gradient:
  - General Info (slate), Groups/Variables (blue), Plugins (violet), Auto-start (emerald)
  - Ports (primary/sky), Resource Limits (cyan with progress bars), Metadata (amber, admin only)
- **Sticky Save Bar** fixed at bottom when scrolling.
- **Permission Gates** by role:
  - Admin: all sections visible + full CRUD.
  - User: General Info (view), Groups/Variables (view+edit), Plugins (view+edit), Auto-start (view+edit), Ports (view, pick primary, notes, no number CRUD), **NO** Limits, Metadata, Admin tab.
- **Extra Ports**: Users see assigned ports (read-only), pick primary, edit notes; Admins have full CRUD.
- **Metadata Card**: Admin only, key-value read-only, filters internal vars (`resolved*`, `forge*`, `javaVersion*`, `build*`, `git*`).
- **Resources**: Colored progress bars (CPU cyan, RAM emerald, Disk violet) with percentages.

## Sidebar Fix (`AppShell.tsx`)

Corrected scope checking in sidebar navigation:

```tsx
// Before (incorrect)
scopes.includes('admin')

// After (correct)
hasScope('admin') || hasScope('server.admin')
```

And `/servers/` route now requires `['server.view']` (not `['admin']`), allowing access to users with server scope.

## Dynamic Parameters

Routes with `[id].astro` use Astro dynamic parameters:

```
/servers/:id   → servers/[id].astro → params.id
/nodes/:id     → nodes/[id].astro   → params.id
```

React components access the ID via `Astro.params` and pass it as a prop to child components.
