# Internacionalización (i18n)

El frontend usa un sistema de traducciones propio basado en React Context con archivos JSON planos, sin dependencias externas de i18n.

## Archivos de Traducción

Los archivos están en `src/lib/locales/`:

| Archivo | Idioma |
|---|---|
| `en.json` (1094 líneas) | Inglés |
| `es.json` (1094 líneas) | Español |

Estructura de claves anidadas con notación de puntos:

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

## Contexto (`src/contexts/translations-context.tsx`)

`TranslationsProvider` envuelve la aplicación y expone:

| Propiedad | Tipo | Descripción |
|---|---|---|
| `language` | `'en' \| 'es'` | Idioma activo |
| `setLanguage(lang)` | función | Cambia el idioma y lo persiste en localStorage |
| `t(key, options?)` | función | Traduce una clave con interpolación opcional |

### Función `t()`

- Resuelve claves anidadas con notación de puntos: `t('sidebar.dashboard')` → `"Dashboard"`
- Si la clave no existe en el idioma actual, fallback a inglés.
- Si tampoco existe en inglés, devuelve la clave literal.
- Soporta interpolación: `t('welcome', { name: 'Admin' })` reemplaza `{{name}}`.

### Persistencia

El idioma se guarda en `localStorage` bajo la clave `aether_panel_language`. Por defecto es `'es'`.

## Uso en Componentes

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

## Cobertura

Las traducciones cubren: sidebar, menú de usuario, perfil/configuración, servidores, nodos, usuarios, roles, templates, settings, bases de datos, dashboard, formularios, validaciones, notificaciones y estados de carga/vacío/error.
