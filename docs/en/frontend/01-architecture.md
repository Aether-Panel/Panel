# Frontend Architecture

This document describes how the Aether Panel web interface (`client/frontend`) is built: the technologies it uses, how the source tree is organised, how a page goes from an Astro file to an interactive React screen, and how the result ends up inside the panel binary.

Related documents:

- [02. API consumption](./02-endpoints.md): the HTTP client, hooks and the endpoints each screen calls.
- [03. Components and UI](./03-components.md): overview of the shell, shared components and global state.
- [05. Translations](./05-translations.md): the i18n system.
- [06. Routing and pages](./06-pages.md): every route and what it shows.
- [07. Detailed reference](./07-components-detailed.md): file-by-file reference of features, hooks and utilities.
- [04. AI](./04-ai.md) and [08. AI in detail](./08-ai-detailed.md): the server assistant and log analysis.

## Technology stack

Versions are the ranges declared in `client/frontend/package.json`.

| Technology | Version | Usage |
|---|---|---|
| **Astro** | ^7.4.4 | Build tool and file-based routing (`src/pages/`). Every page is static HTML that mounts one React island with `client:only="react"`. Navigation uses Astro's `ClientRouter` (view transitions). |
| **React** / **React DOM** | ^19.2.8 | All interactive UI. |
| **Tailwind CSS** | ^3.4.1 | Utility-first styles, with `tailwindcss-animate`. Design tokens (colours, radii, shadows, easing) are CSS variables in `src/styles/globals.css`. |
| **Radix UI** primitives | various | Accessible building blocks wrapped by the shadcn-style components in `src/components/ui/` (25 files). |
| **class-variance-authority**, **clsx**, **tailwind-merge** | ^0.7.1 / ^2.1.1 / ^3.0.1 | Component variants and the `cn()` class helper (`src/lib/utils.ts`). |
| **react-hook-form** + **@hookform/resolvers** + **zod** | ^7.90.0 / ^6.0.2 / ^4.4.3 | Forms and schema validation. |
| **Monaco Editor** (`monaco-editor`, `@monaco-editor/react`) | ^0.57.0 / ^4.6.1 | File editor and diff viewer in the file manager. Loaded lazily, with its stylesheet bundled (works offline and under a strict CSP). |
| **recharts** | ^2.15.1 | Every chart (server metrics, performance history, dashboard network history). |
| **@phosphor-icons/react** | ^2.1.10 | Icons, re-exported from `src/components/icons.tsx`. |
| **sileo** | ^0.1.6 | Toast notifications (`src/lib/toast.ts`). |
| **three** | 0.186.1 | The 3D emblem on the sign-in brand panel (`src/lib/emblem-3d.ts`). |
| **TypeScript** | ^6.0.3 | Type checking (`npm run typecheck`). |

There is no AI SDK in the frontend: the AI features call the panel API, which talks to Google Gemini on the server side (see [04. AI](./04-ai.md)). There is also no state-management or data-fetching library (no React Query, no Redux): data lives in custom hooks and React context.

## Source tree (`client/frontend/src/`)

```
src/
├── pages/                 # Astro routes; each page mounts one page wrapper (see 06-pages.md)
├── layouts/
│   └── BaseLayout.astro   # <html>, fonts, ClientRouter, stale-build and portal clean-up scripts
├── components/
│   ├── page-wrappers/     # One React entry per page: AppShell or AuthShell + the feature page
│   ├── ui/                # 25 shadcn-style primitives on Radix (button, dialog, sheet, sidebar, chart…)
│   ├── AppShell.tsx       # Signed-in layout: collapsible sidebar, top bar with breadcrumbs and quick search
│   ├── AuthShell.tsx      # Sign-in layout and the shared auth building blocks (AuthFrame, inputs, emblem)
│   ├── quick-search.tsx   # Ctrl/⌘+K command palette
│   ├── server-card.tsx    # Server card, status badge, usage bars and sparkline
│   ├── user-avatar.tsx    # Account picture or Kodama avatar
│   ├── Turnstile.tsx      # useCaptcha(): built-in proof-of-work or Cloudflare Turnstile
│   ├── SileoToaster.tsx   # Toast renderer
│   ├── ErrorBoundary.tsx, empty-state.tsx, page-header.tsx, logo.tsx, icons.tsx,
│   ├── ambient-backdrop.tsx, spotlight.tsx, glide-menu.tsx, animated-number.tsx,
│   └── beta-badge.tsx, player-presence.tsx
├── contexts/
│   ├── providers.tsx            # Providers wrapper + AuthContext (useAuth)
│   ├── config-context.tsx       # Public panel config from GET /api/config (useConfig, useExperimental)
│   └── translations-context.tsx # i18n (useTranslations)
├── features/              # Screens grouped by domain
│   ├── auth/              # Sliding sign-in/sign-up card, forgot/reset password
│   ├── dashboard/         # Status strip, KPI cards, network history, access log
│   ├── servers/           # Server list, create-server stepper and the server detail ([id]/)
│   ├── nodes/             # Node list, node detail ([id]/), low-disk alert
│   ├── users/, roles/, templates/, database-hosts/
│   ├── settings/          # Panel settings tabs (general, Discord, mail, offsite, domains, API keys…)
│   ├── profile/settings/  # Own account: details, password, 2FA, SSH keys, OAuth2 clients, language
│   └── extensions/        # Pages added by installed extensions (sandboxed iframes)
├── hooks/                 # Data hooks (use-servers, use-nodes, use-network-history…)
├── lib/
│   ├── api-client.ts      # fetch wrapper, ApiError, localized API errors, session-expiry redirect
│   ├── paged.ts           # fetchAllPages() for paginated lists
│   ├── chart-theme.ts     # Shared chart colours and time formatting
│   ├── toast.ts           # sileo wrapper
│   ├── ansi-utils.tsx     # ANSI colours in console lines
│   ├── data.ts            # Shared types (Server, User, Node)
│   ├── players.ts, game-logos.ts, keyed.ts, stale-build.ts, proof-of-work.ts,
│   ├── monaco-setup.ts, emblem-3d.ts, utils.ts
│   └── locales/           # en.json, es.json
└── styles/globals.css     # Tailwind layers, design tokens, light/dark variables
```

The `@/` import alias points to `src/` (configured in `astro.config.mjs` and `tsconfig.json`).

## From route to screen

1. A request such as `/servers/view/?id=abc` is served `pages/servers/view.astro`, rendered at build time into static HTML.
2. The page uses `BaseLayout.astro` and mounts its wrapper from `components/page-wrappers/` with `client:only="react"`, so nothing React renders on the server.
3. The wrapper puts the feature page inside a shell:
   - `AppShell` for signed-in pages (sidebar, top bar, toasts),
   - `AuthShell` for `/login/`, `/register/`, `/forgot-password/` and `/reset-password/`.
4. Both shells render `Providers` (`contexts/providers.tsx`), which nests `TranslationsProvider` → `ConfigProvider` → `AuthProvider`. `AuthProvider` calls `GET /api/self`; until it answers it shows a loading screen, and it sends visitors without a session to `/login/` (remembering the page to come back to).
5. The feature page fetches its data through hooks and `api` (see [02](./02-endpoints.md)).

Details of every route are in [06. Routing and pages](./06-pages.md).

## `BaseLayout.astro`

- Sets `<html lang="en" class="dark">`; the translations context updates `lang` when the language changes.
- Preloads the Geist font shipped in `public/fonts` (no request to Google Fonts).
- Enables `ClientRouter` and adds two inline scripts:
  - removes Radix portals (open dialogs, dropdowns) before Astro swaps the page, so they do not linger after a client-side navigation;
  - on `vite:preloadError` (a chunk that no longer exists because the panel was updated while the page was open) reloads once per minute at most. `lib/stale-build.ts` does the same for React lazy imports.

## Building and serving

```bash
cd client/frontend
npm run dev        # astro dev --port 9002
npm run build      # astro build -> client/frontend/dist/
npm run start      # astro preview
npm run lint       # astro check
npm run typecheck  # tsc --noEmit
```

- `client/frontend/webfiles.go` embeds `dist/` in the Go binary (`//go:embed all:dist`), so `npm run build` must run before `go build`. The Docker image does this in its Node build stage.
- `internal/web/loader.go` serves those files for every path outside the API prefixes. When `panel.web.files` is set (default `www`, relative to the panel's working directory), a file found in that directory is served instead of the embedded one with the same path; anything missing there falls back to the embedded build. This allows overriding static assets without rebuilding.
- The API, `/auth` and `/oauth2` routes are registered on the same server, so the frontend always talks to its own origin with the session cookie.
