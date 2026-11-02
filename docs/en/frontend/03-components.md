# Components and User Interface

Overview of the building blocks of the Aether Panel interface: the two page shells, the shared components, the global state and the cross-cutting UI behaviour (toasts, charts, captcha). For a file-by-file reference of every feature folder, hook and utility see [07. Detailed reference](./07-components-detailed.md); for routes see [06. Routing and pages](./06-pages.md).

## Rendering model

- `.astro` pages are static containers. Each one mounts a single React wrapper with `client:only="react"` ([01. Architecture](./01-architecture.md#from-route-to-screen)).
- All interactive UI is React 19 styled with Tailwind. The interface is dark-themed: `BaseLayout.astro` sets `class="dark"` on `<html>` and there is no theme switcher.
- Design tokens (colours such as `--primary`, `--chart-1…5`, `--success`, `--warning`; radii; shadows `shadow-elevation-*`; the `ease-panel` curve) are CSS variables in `src/styles/globals.css`, read by `tailwind.config.ts`.
- Motion respects `prefers-reduced-motion` (`motion-reduce:` variants, `AnimatedNumber` jumps straight to the value).

## Page shells

### `AppShell` (signed-in pages)

`components/AppShell.tsx` wraps every signed-in page in `Providers`, the sidebar, the top bar and the toaster.

**Collapsible sidebar** (built on `components/ui/sidebar.tsx` with `collapsible="icon"`):

- On desktop it can be collapsed to a 4.5rem icon rail and expanded again from the top-bar button or with **Ctrl/⌘+B** (ignored while typing in a field or in the code editor). The state is kept in the `sidebar_state` cookie for 7 days.
- While collapsed, hovering or focusing the rail opens it over the page (after a short delay) without moving the content; labels, the account card and the "Powered by" link fade back in.
- On phones (< 768 px) the sidebar becomes a sheet opened from the menu button.
- Items are grouped into **Overview** (Dashboard), **Management** (Servers, Users, Roles, Nodes, Database hosts, Templates, Settings) and **Extensions** (admin pages of active extensions). Each item is shown only when the user holds one of its scopes, and a group without visible items is not rendered. The scopes per item are listed in [06](./06-pages.md#sidebar-navigation).
- A `GlideMenu` draws one hover pill that slides between items.
- The footer holds the account card (avatar, name, role badge, email) with a menu for **Account Settings** and **Log out**.

**Top bar** (`AppTopBar`): the sidebar toggle, breadcrumbs built from the URL (detail pages replace the raw id with the entity name by dispatching the `aether:breadcrumb` event; on phones a deep trail drops the root crumb), and **quick search**.

**Quick search** (`components/quick-search.tsx`, **Ctrl/⌘+K**): a command palette with the pages the user may open (the same list as the sidebar, so it never suggests a forbidden page), their servers and each server's main tabs, and power actions on a server. Matching is case- and accent-insensitive. Desktop shows an inline field with a dropdown; phones get a dialog.

**Update notice**: users with `settings.edit` get one info toast per browser session when `GET /api/settings/update-check` reports a newer panel version.

### `AuthShell` (sign-in pages)

`components/AuthShell.tsx` renders `Providers`, the ambient backdrop and the toaster, and exports the pieces every auth screen shares: `AuthFrame` (brand column + form column, used by the password screens), `AuthPane`, `AuthInput`, `AuthPasswordInput`, `PasswordStrength`, `AuthNotice`, `AuthEmblem` (the emblem in 3D with three.js on desktop; the flat image shows first and stays when WebGL is unavailable) and shared class names.

Sign-in and sign-up share one **sliding card** (`features/auth/auth-card.tsx`): on desktop the brand panel covers one half and slides to the other when switching forms, while the forms slide and cross-fade under it. The URL follows (`/login` ↔ `/register`, via `history.pushState`) and the browser's back button switches back. The sign-up form (and its captcha) only mounts the first time it is needed. Phones show one form at a time with a link to the other. When registration is disabled, `/register` shows the sign-in form. Details in [06](./06-pages.md#authentication-pages).

## Shared components (`src/components/`)

| Component | Purpose |
|---|---|
| `ui/*` | 25 shadcn-style primitives: accordion, alert-dialog, avatar, badge, button, card, chart, checkbox, dialog, dropdown-menu, form, input, label, progress, scroll-area, select, separator, sheet, sidebar, skeleton, switch, table, tabs, textarea, tooltip. |
| `page-wrappers/*` | One entry component per route (see [06](./06-pages.md)). |
| `server-card.tsx` | `ServerCard` (dashboard and server list), `StatusBadge`, `StatusIndicator`, `MetricBar`, `Sparkline`, `usageColor`. |
| `user-avatar.tsx` | `UserAvatar`: the account's own picture or a Kodama avatar from `api.kodama.sh`, seeded with a one-way hash of the account (never the email itself); initials while loading. |
| `empty-state.tsx` | The single empty / no-results / failed-to-load box. |
| `page-header.tsx` | `PageHeader` with `title`, `description`, `badges` and right-aligned actions (`children`). |
| `ErrorBoundary.tsx` | Catches render errors per area (each server tab has its own) and shows a retry box. If the error is a stale build (a chunk removed by a panel update), it reloads the page once instead. |
| `Turnstile.tsx` | `useCaptcha()` and the Turnstile widget (see below). |
| `SileoToaster.tsx` | Renders toasts and bundles sileo's CSS so it survives Astro page swaps. |
| `quick-search.tsx`, `glide-menu.tsx` | Command palette and sidebar hover pill. |
| `player-presence.tsx` | `PlayerAvatar`, `PlayerStack` (overlapping faces of online players), `LiveDot`. |
| `animated-number.tsx` | Numbers that count up and glide between values. |
| `ambient-backdrop.tsx`, `spotlight.tsx` | Decorative background and the pointer glow on `[data-spotlight]` panels (disabled on touch). |
| `beta-badge.tsx` | "Beta" tag for experimental features. |
| `icons.tsx` | The icon set (Phosphor), exported under stable names. |
| `logo.tsx` | Panel logo. |

## Global state (`src/contexts/`)

| Context | Hook | Provides |
|---|---|---|
| `AuthContext` (`providers.tsx`) | `useAuth()` | `role` (`'admin'` when the scopes include `admin`, otherwise `'user'`), `user`, `scopes`, `login`, `verifyOtp`, `register`, `logout`, `hasScope`, `loading`, `fetchSelf`. |
| `ConfigContext` (`config-context.tsx`) | `useConfig()`, `useExperimental()` | `GET /api/config`: `branding.name`, `registrationEnabled`, `captcha` (`'turnstile'`, `'builtin'` or empty), `turnstileSiteKey`, `turnstileEnabled`, `hideAiAnalysis`, `experimental` flags (`versionSwitch`, `propertiesEditor`, `serverClone`), `version`, `themes`. |
| `TranslationsContext` (`translations-context.tsx`) | `useTranslations()` | `language`, `setLanguage`, `t()`. See [05](./05-translations.md). |

`hasScope(scope)` returns `true` for administrators, for scopes the user holds, and for any `server.*` scope (except `server.create` and `server.admin` itself) when the user holds `server.admin`. On the server detail page the tabs use the user's scopes **on that server** instead (see [06](./06-pages.md#server-detail-serversviewidid)). The frontend only hides what the user cannot use; the API enforces every permission.

## Toasts (`lib/toast.ts`)

```typescript
import { sileo } from '@/lib/toast';

sileo.success({ title: t('common.success'), description: t('servers.actions.startSuccess') });
sileo.error({ title: t('common.error'), description: err.message });
```

The wrapper re-exports sileo (`show`, `success`, `info`, `action`, `promise`, `dismiss`, `clear`) and forces `error` and `warning` toasts to `top-center` so problems are noticed. Notable toasts: a **crash toast** when the server's socket reports a crash (title with the exit code, description saying what the code means and whether the server was restarted), a short "console connected" toast when the console reconnects after a drop, the update notice, and the result of every action.

## Charts

All charts use recharts through `components/ui/chart.tsx`. `lib/chart-theme.ts` gives every chart the same colour per kind of data (`CHART_COLORS`: CPU `--chart-1`, memory `--chart-2`, download `--chart-3`, upload `--chart-5`, players `--success`, player peak `--chart-4`) and formats times in the panel language (`chartLocale`, `formatClock`), not the browser's. The charts themselves are described in [07](./07-components-detailed.md#charts).

## Captcha (`components/Turnstile.tsx`)

`useCaptcha()` returns `{ widget, fields, ready, reset }` for the auth forms, according to `config.captcha`:

- `builtin`: fetches a challenge from `GET /auth/challenge` and solves the proof of work in the background (`lib/proof-of-work.ts`, ALTCHA scheme, with its own SHA-256 because `crypto.subtle` is unavailable on plain HTTP). A quiet line under the form shows progress; there is nothing to click.
- `turnstile`: loads Cloudflare Turnstile once from `challenges.cloudflare.com` and renders the widget with `turnstileSiteKey`.
- empty: no check. Panels too old to send `captcha` fall back to Turnstile when `turnstileEnabled` and `turnstileSiteKey` are set.

The submit button stays disabled until `ready` (a token is available, or no check is configured). If the built-in check fails, the line offers a retry. The form spreads `fields` (`captchaToken` or `turnstileToken`) into the request and calls `reset()` after each attempt, because a token is valid once. Server-side behaviour is documented in [Turnstile and captcha](../general/17-turnstile.md).
