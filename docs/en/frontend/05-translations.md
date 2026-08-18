# Internationalization (i18n)

The frontend uses its own translation system based on React Context with plain JSON files, without external i18n dependencies.

## Translation Files

The files are located in `src/lib/locales/`:

| File | Language |
|---|---|
| `en.json` (1094 lines) | English |
| `es.json` (1094 lines) | Spanish |

Nested keys structure with dot notation:

```json
{
  "sidebar": {
    "dashboard": "Dashboard",
    "servers": "Servers"
  },
  "profileSettings": {
    "tabs": {
      "twoFactor": "2FA",
      "oauth": "OAuth"
    }
  }
}
```

## Context (`src/contexts/translations-context.tsx`)

`TranslationsProvider` wraps the application and exposes:

| Property | Type | Description |
|---|---|---|
| `language` | `'en' \| 'es'` | Active language |
| `setLanguage(lang)` | function | Changes the language and persists it to localStorage |
| `t(key, options?)` | function | Translates a key with optional interpolation |

### `t()` function

- Resolves nested keys with dot notation: `t('sidebar.dashboard')` → `"Dashboard"`
- If the key does not exist in the current language, fallback to English.
- If it does not exist in English either, it returns the literal key.
- Supports interpolation: `t('welcome', { name: 'Admin' })` replaces `{{name}}`.

### Persistence

The language is stored in `localStorage` under the key `aether_panel_language`. By default it is `'es'`.

## Usage in Components

```tsx
import { useTranslations } from '@/contexts/translations-context';

function Sidebar() {
  const { t, language, setLanguage } = useTranslations();
  return (
    <div>
      <span>{t('sidebar.dashboard')}</span>
      <select value={language} onChange={e => setLanguage(e.target.value)}>
        <option value="en">English</option>
        <option value="es">Español</option>
      </select>
    </div>
  );
}
```

## Coverage

The translations cover: sidebar, user menu, profile/settings, servers, nodes, users, roles, templates, settings, databases, dashboard, forms, validations, notifications, and loading/empty/error states.
