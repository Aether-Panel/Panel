# Internationalization (i18n)

The interface is available in **English** (default) and **Spanish**. Translations are plain JSON files read through a React context; there is no external i18n library.

## Translation files

`src/lib/locales/`:

| File | Language | Size |
|---|---|---|
| `en.json` | English (default and fallback) | 3,572 lines, about 2,710 strings |
| `es.json` | Spanish | Same keys as `en.json` |

Top-level namespaces: `auth`, `header`, `sidebar`, `userMenu`, `profileSettings`, `dashboard`, `servers`, `users`, `roles`, `permissions`, `nodes`, `databaseHosts`, `templates`, `common`, `settings`, `apiKeys`, `products`, `apiErrors`, `extensions`.

Keys are nested and addressed with dot notation:

```json
{
  "sidebar": {
    "dashboard": "Dashboard",
    "collapse": "Collapse menu"
  },
  "dashboard": {
    "welcome": "Welcome, {{name}}!"
  }
}
```

## Context (`src/contexts/translations-context.tsx`)

`TranslationsProvider` is part of `Providers` (see [03](./03-components.md#global-state-srccontexts)) and exposes, through `useTranslations()`:

| Property | Type | Description |
|---|---|---|
| `language` | `'en' \| 'es'` | Active language. |
| `setLanguage(lang)` | function | Changes the language immediately and stores it in `localStorage`. |
| `t(key, options?)` | function | Translates a key, with optional `{{placeholder}}` interpolation. |

### `t()`

- Resolves dot-separated keys: `t('sidebar.dashboard')` → `"Dashboard"`.
- Missing in the active language → falls back to English; missing in English too → returns the key itself.
- Interpolation replaces every `{{name}}`: `t('dashboard.welcome', { name: 'Admin' })`.

### Default language and persistence

- The interface starts in **English**. The choice is stored in `localStorage` under `aether_panel_language` and applied on the next load; if storage is blocked the interface stays in English.
- The language is chosen in **Account Settings** (`/profile/settings/`, language selector), and applies at once without reloading.
- The provider keeps `<html lang>` in sync with the active language, so screen readers and the browser use the right one.

## Where else the language is used

- **API errors**: `lib/api-client.ts` translates structured API errors by their code (below).
- **Charts and dates**: `lib/chart-theme.ts` formats chart times with `es-ES` or `en-US` according to the panel language, not the browser locale. Relative times ("5 minutes ago") are also built from the active locale.
- **AI answers**: the assistant and log analysis send `language` to the API so Gemini answers in the same language (see [08](./08-ai-detailed.md)).
- **Extensions**: extension pages receive the language in their `aether:context` message (see [07](./07-components-detailed.md#extensions-featuresextensions)).
- **Server templates**: known template variables and groups have translations; unknown ones keep the template's own text.

## Localized API errors

When the API answers `{ "error": { "code": "...", "msg": "..." } }`, the client looks up `apiErrors.<code>` in the language stored in `aether_panel_language` and uses it as the error message; codes without a translation keep the server's `msg`. A translation can hold `{{name}}` placeholders, filled from the error's `metadata` (a list is joined with commas): for example `ErrPortTaken` reads `Port {{port}} is already used by another server on this node.` Values that are themselves words get their own translation under `apiErrorValues.<code>`: the webhook names `alerts`, `reports`, `status` and `transfers` of `ErrDiscordTestFailed` read as the Discord settings sections. Translated codes today (47):

`ErrAIAnalysisDisabled`, `ErrAPIKeyNotAllowed`, `ErrAPIKeyPermissions`, `ErrBackupLimit`, `ErrCaptchaFailed`, `ErrCloudflareBadAnswer`, `ErrCloudflareDomainNotInZone`, `ErrCloudflareNotConfigured`, `ErrCloudflareRecordID`, `ErrCloudflareRefused`, `ErrCloudflareRefusedCode`, `ErrCloudflareUnreachable`, `ErrCloudflareZoneID`, `ErrDNSBaseDomainInvalid`, `ErrDatabaseHostInUse`, `ErrDiscordNoWebhooks`, `ErrDiscordTestFailed`, `ErrDiskLimitExceeded`, `ErrFileExists`, `ErrInviteSignupsClosed`, `ErrNodeAuthFailed`, `ErrNotificationLanguage`, `ErrPluginBadFileName`, `ErrPluginNoCompatibleVersion`, `ErrPortBusy`, `ErrPortDuplicated`, `ErrPortOutOfRange`, `ErrPortTaken`, `ErrPortUsedByPanel`, `ErrSSHKeyExists`, `ErrSSHKeyInvalid`, `ErrSSHKeyIsPrivate`, `ErrSSHKeyLimit`, `ErrSSHKeyName`, `ErrSSHKeyWeak`, `ErrSSONotForAdmins`, `ErrServerOwner`, `ErrSubdomainDisabled`, `ErrSubdomainInvalid`, `ErrSubdomainNoTarget`, `ErrSubdomainRateLimited`, `ErrSubdomainReserved`, `ErrSubdomainTaken`, `ErrTransferBackupRunning`, `ErrTransferPortsTaken`, `ErrTransferRunning`, `ErrTransferSubServer`.

## Usage

```tsx
import { useTranslations } from '@/contexts/translations-context';

function Example({ count }: { count: number }) {
  const { t, language } = useTranslations();
  return <p lang={language}>{t('dashboard.admin.history.unavailable', { count })}</p>;
}
```

## Adding or changing strings

1. Add the key to **both** `en.json` and `es.json`, in the same place of the tree (the files are kept key-for-key identical).
2. Use `t('namespace.key')` in the component; never hard-code visible text.
3. For a new structured API error, add its code under `apiErrors` in both files, with `{{name}}` for each `metadata` field the message should show.
4. Run `npm run typecheck`: the locale files are imported as JSON modules.
