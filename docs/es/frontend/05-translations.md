# Internacionalización (i18n)

La interfaz está disponible en **inglés** (idioma por defecto) y **español**. Las traducciones son archivos JSON planos que se leen mediante un contexto de React; no se usa ninguna librería externa de i18n.

## Archivos de traducción

`src/lib/locales/`:

| Archivo | Idioma | Tamaño |
|---|---|---|
| `en.json` | Inglés (por defecto y de respaldo) | 3.572 líneas, unas 2.710 cadenas |
| `es.json` | Español | Las mismas claves que `en.json` |

Espacios de nombres de primer nivel: `auth`, `header`, `sidebar`, `userMenu`, `profileSettings`, `dashboard`, `servers`, `users`, `roles`, `permissions`, `nodes`, `databaseHosts`, `templates`, `common`, `settings`, `apiKeys`, `products`, `apiErrors`, `extensions`.

Las claves están anidadas y se referencian con notación de puntos:

```json
{
  "sidebar": {
    "dashboard": "Inicio",
    "collapse": "Contraer menú"
  },
  "dashboard": {
    "welcome": "¡Bienvenido, {{name}}!"
  }
}
```

## Contexto (`src/contexts/translations-context.tsx`)

`TranslationsProvider` forma parte de `Providers` (ver [03](./03-components.md#estado-global-srccontexts)) y expone, mediante `useTranslations()`:

| Propiedad | Tipo | Descripción |
|---|---|---|
| `language` | `'en' \| 'es'` | Idioma activo. |
| `setLanguage(lang)` | función | Cambia el idioma al instante y lo guarda en `localStorage`. |
| `t(key, options?)` | función | Traduce una clave, con interpolación opcional de `{{marcador}}`. |

### `t()`

- Resuelve claves separadas por puntos: `t('sidebar.collapse')` → `"Contraer menú"`.
- Si falta en el idioma activo → recurre al inglés; si tampoco existe en inglés → devuelve la propia clave.
- La interpolación sustituye cada `{{nombre}}`: `t('dashboard.welcome', { name: 'Admin' })`.

### Idioma por defecto y persistencia

- La interfaz arranca en **inglés**. La elección se guarda en `localStorage` con la clave `aether_panel_language` y se aplica en la siguiente carga; si el almacenamiento está bloqueado, la interfaz se queda en inglés.
- El idioma se elige en **Configuración de la cuenta** (`/profile/settings/`, selector de idioma) y se aplica al momento, sin recargar.
- El proveedor mantiene `<html lang>` sincronizado con el idioma activo, para que los lectores de pantalla y el navegador usen el correcto.

## Dónde más se usa el idioma

- **Errores de la API**: `lib/api-client.ts` traduce los errores estructurados de la API según su código (ver abajo).
- **Gráficos y fechas**: `lib/chart-theme.ts` formatea las horas de los gráficos con `es-ES` o `en-US` según el idioma del panel, no según la configuración regional del navegador. Los tiempos relativos ("hace 5 minutos") también se construyen con el idioma activo.
- **Respuestas de la IA**: el asistente y el análisis de registros envían `language` a la API para que Gemini responda en el mismo idioma (ver [08](./08-ai-detailed.md)).
- **Extensiones**: las páginas de extensiones reciben el idioma en su mensaje `aether:context` (ver [07](./07-components-detailed.md#extensiones-featuresextensions)).
- **Plantillas de servidor**: las variables y grupos de plantilla conocidos tienen traducción; los desconocidos conservan el texto de la propia plantilla.

## Errores de API traducidos

Cuando la API responde `{ "error": { "code": "...", "msg": "..." } }`, el cliente busca `apiErrors.<code>` en el idioma guardado en `aether_panel_language` y lo usa como mensaje de error; los códigos sin traducción conservan el `msg` del servidor. Códigos traducidos hoy:

`ErrDiskLimitExceeded`, `ErrSSHKeyInvalid`, `ErrSSHKeyIsPrivate`, `ErrSSHKeyWeak`, `ErrSSHKeyExists`, `ErrSSHKeyLimit`, `ErrSSHKeyName`, `ErrCaptchaFailed`, `ErrSSONotForAdmins`, `ErrServerOwner`, `ErrAPIKeyPermissions`, `ErrAPIKeyNotAllowed`, `ErrBackupLimit`, `ErrInviteSignupsClosed`.

## Uso

```tsx
import { useTranslations } from '@/contexts/translations-context';

function Example({ count }: { count: number }) {
  const { t, language } = useTranslations();
  return <p lang={language}>{t('dashboard.admin.history.unavailable', { count })}</p>;
}
```

## Añadir o cambiar textos

1. Añadir la clave en **ambos** archivos, `en.json` y `es.json`, en el mismo lugar del árbol (los archivos se mantienen idénticos clave a clave).
2. Usar `t('espacio.clave')` en el componente; nunca escribir texto visible directamente.
3. Para un nuevo error estructurado de la API, añadir su código en `apiErrors` en ambos archivos.
4. Ejecutar `npm run typecheck`: los archivos de idioma se importan como módulos JSON.
