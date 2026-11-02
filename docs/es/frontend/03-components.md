# Componentes e interfaz de usuario

Visión general de las piezas que forman la interfaz de Aether Panel: los dos shells de página, los componentes compartidos, el estado global y los comportamientos transversales (toasts, gráficos, captcha). La referencia archivo por archivo de cada carpeta de features, hook y utilidad está en [07. Referencia detallada](./07-components-detailed.md); las rutas, en [06. Rutas y páginas](./06-pages.md).

## Modelo de renderizado

- Las páginas `.astro` son contenedores estáticos. Cada una monta un único wrapper de React con `client:only="react"` ([01. Arquitectura](./01-architecture.md#de-la-ruta-a-la-pantalla)).
- Toda la interfaz interactiva es React 19 con estilos de Tailwind. La interfaz usa tema oscuro: `BaseLayout.astro` pone `class="dark"` en `<html>` y no hay selector de tema.
- Los tokens de diseño (colores como `--primary`, `--chart-1…5`, `--success`, `--warning`; radios; sombras `shadow-elevation-*`; la curva `ease-panel`) son variables CSS de `src/styles/globals.css` que lee `tailwind.config.ts`.
- Las animaciones respetan `prefers-reduced-motion` (variantes `motion-reduce:`; `AnimatedNumber` muestra el valor final directamente).

## Shells de página

### `AppShell` (páginas con sesión)

`components/AppShell.tsx` envuelve cada página con sesión en `Providers`, la barra lateral, la barra superior y el contenedor de toasts.

**Barra lateral plegable** (basada en `components/ui/sidebar.tsx` con `collapsible="icon"`):

- En escritorio se puede plegar a un carril de iconos de 4,5rem y volver a desplegar con el botón de la barra superior o con **Ctrl/⌘+B** (se ignora mientras se escribe en un campo o en el editor de código). El estado se guarda en la cookie `sidebar_state` durante 7 días.
- Plegada, al pasar el puntero o enfocar el carril se abre por encima de la página (tras un breve retardo) sin desplazar el contenido; las etiquetas, la tarjeta de la cuenta y el enlace "Impulsado por Aether Panel" vuelven a aparecer.
- En móviles (< 768 px) la barra lateral se convierte en un panel deslizante que se abre con el botón de menú.
- Los elementos se agrupan en **Resumen** (Inicio), **Gestión** (Servidores, Usuarios, Roles, Nodos, Hosts de BD, Plantillas, Configuración) y **Extensiones** (páginas de administración de las extensiones activas). Cada elemento solo aparece si el usuario tiene alguno de sus scopes, y un grupo sin elementos visibles no se muestra. Los scopes de cada elemento se listan en [06](./06-pages.md#navegación-lateral).
- Un `GlideMenu` dibuja una única píldora de resaltado que se desliza entre elementos.
- El pie contiene la tarjeta de la cuenta (avatar, nombre, insignia de rol, correo) con un menú para **Configuración de la cuenta** y **Cerrar sesión**.

**Barra superior** (`AppTopBar`): el botón de la barra lateral, migas de pan construidas a partir de la URL (las páginas de detalle sustituyen el id por el nombre de la entidad emitiendo el evento `aether:breadcrumb`; en móviles una ruta profunda oculta la miga raíz) y la **búsqueda rápida**.

**Búsqueda rápida** (`components/quick-search.tsx`, **Ctrl/⌘+K**): una paleta de comandos con las páginas que el usuario puede abrir (la misma lista que la barra lateral, así que nunca sugiere una página prohibida), sus servidores y las pestañas principales de cada uno, y acciones de encendido sobre un servidor. La coincidencia ignora mayúsculas y tildes. En escritorio es un campo con desplegable; en móviles, un diálogo.

**Aviso de actualización**: los usuarios con `settings.edit` reciben un toast informativo por sesión del navegador cuando `GET /api/settings/update-check` indica que hay una versión nueva del panel.

### `AuthShell` (páginas de acceso)

`components/AuthShell.tsx` renderiza `Providers`, el fondo ambiental y el contenedor de toasts, y exporta las piezas que comparten todas las pantallas de acceso: `AuthFrame` (columna de marca + columna del formulario, usada por las pantallas de contraseña), `AuthPane`, `AuthInput`, `AuthPasswordInput`, `PasswordStrength`, `AuthNotice`, `AuthEmblem` (el emblema en 3D con three.js en escritorio; la imagen plana aparece primero y se mantiene si no hay WebGL) y clases compartidas.

El inicio de sesión y el registro comparten una **tarjeta deslizante** (`features/auth/auth-card.tsx`): en escritorio el panel de marca cubre una mitad y se desliza a la otra al cambiar de formulario, mientras los formularios se desplazan y se funden por debajo. La URL acompaña el cambio (`/login` ↔ `/register`, con `history.pushState`) y el botón Atrás del navegador vuelve al anterior. El formulario de registro (y su captcha) solo se monta la primera vez que hace falta. En móviles se ve un formulario cada vez, con un enlace al otro. Con el registro desactivado, `/register` muestra el formulario de inicio de sesión. Detalles en [06](./06-pages.md#páginas-de-acceso).

## Componentes compartidos (`src/components/`)

| Componente | Función |
|---|---|
| `ui/*` | 25 primitivas estilo shadcn: accordion, alert-dialog, avatar, badge, button, card, chart, checkbox, dialog, dropdown-menu, form, input, label, progress, scroll-area, select, separator, sheet, sidebar, skeleton, switch, table, tabs, textarea, tooltip. |
| `page-wrappers/*` | Un componente de entrada por ruta (ver [06](./06-pages.md)). |
| `server-card.tsx` | `ServerCard` (panel principal y lista de servidores), `StatusBadge`, `StatusIndicator`, `MetricBar`, `Sparkline`, `usageColor`. |
| `user-avatar.tsx` | `UserAvatar`: la foto propia de la cuenta o un avatar de Kodama desde `api.kodama.sh`, generado a partir de un hash de un solo sentido de la cuenta (nunca el correo en sí); iniciales mientras carga. |
| `empty-state.tsx` | El único recuadro de vacío / sin resultados / error de carga. |
| `page-header.tsx` | `PageHeader` con `title`, `description`, `badges` y acciones alineadas a la derecha (`children`). |
| `ErrorBoundary.tsx` | Captura errores de renderizado por zona (cada pestaña del servidor tiene el suyo) y muestra un recuadro para reintentar. Si el error se debe a un build obsoleto (un fragmento eliminado por una actualización del panel), recarga la página una vez. |
| `Turnstile.tsx` | `useCaptcha()` y el widget de Turnstile (ver más abajo). |
| `SileoToaster.tsx` | Renderiza los toasts e incluye el CSS de sileo para que sobreviva a los cambios de página de Astro. |
| `quick-search.tsx`, `glide-menu.tsx` | Paleta de comandos y píldora de resaltado de la barra lateral. |
| `player-presence.tsx` | `PlayerAvatar`, `PlayerStack` (caras superpuestas de los jugadores conectados), `LiveDot`. |
| `animated-number.tsx` | Cifras que cuentan hasta su valor y se deslizan entre valores. |
| `ambient-backdrop.tsx`, `spotlight.tsx` | Fondo decorativo y el brillo que sigue al puntero en los paneles `[data-spotlight]` (desactivado en pantallas táctiles). |
| `beta-badge.tsx` | Etiqueta "Beta" para funciones experimentales. |
| `icons.tsx` | El conjunto de iconos (Phosphor), exportado con nombres estables. |
| `logo.tsx` | Logotipo del panel. |

## Estado global (`src/contexts/`)

| Contexto | Hook | Proporciona |
|---|---|---|
| `AuthContext` (`providers.tsx`) | `useAuth()` | `role` (`'admin'` si los scopes incluyen `admin`, si no `'user'`), `user`, `scopes`, `login`, `verifyOtp`, `register`, `logout`, `hasScope`, `loading`, `fetchSelf`. |
| `ConfigContext` (`config-context.tsx`) | `useConfig()`, `useExperimental()` | `GET /api/config`: `branding.name`, `registrationEnabled`, `captcha` (`'turnstile'`, `'builtin'` o vacío), `turnstileSiteKey`, `turnstileEnabled`, `hideAiAnalysis`, indicadores `experimental` (`versionSwitch`, `propertiesEditor`, `serverClone`), `version`, `themes`. |
| `TranslationsContext` (`translations-context.tsx`) | `useTranslations()` | `language`, `setLanguage`, `t()`. Ver [05](./05-translations.md). |

`hasScope(scope)` devuelve `true` para los administradores, para los scopes que el usuario tiene y para cualquier scope `server.*` (salvo `server.create` y el propio `server.admin`) cuando el usuario tiene `server.admin`. En el detalle del servidor, las pestañas usan en cambio los scopes del usuario **en ese servidor** (ver [06](./06-pages.md#detalle-del-servidor-serversviewidid)). El frontend solo oculta lo que el usuario no puede usar; la API aplica todos los permisos.

## Toasts (`lib/toast.ts`)

```typescript
import { sileo } from '@/lib/toast';

sileo.success({ title: t('common.success'), description: t('servers.actions.startSuccess') });
sileo.error({ title: t('common.error'), description: err.message });
```

El envoltorio reexporta sileo (`show`, `success`, `info`, `action`, `promise`, `dismiss`, `clear`) y fuerza los toasts de `error` y `warning` a la posición `top-center` para que los problemas se vean. Toasts destacados: el **toast de caída** cuando el socket del servidor informa de una caída (título con el código de salida y una descripción de lo que suele significar ese código y de si el servidor se reinició), un breve aviso de "consola conectada" cuando la consola reconecta tras un corte, el aviso de actualización y el resultado de cada acción.

## Gráficos

Todos los gráficos usan recharts a través de `components/ui/chart.tsx`. `lib/chart-theme.ts` asigna a cada tipo de dato el mismo color en todos los gráficos (`CHART_COLORS`: CPU `--chart-1`, memoria `--chart-2`, descarga `--chart-3`, subida `--chart-5`, jugadores `--success`, pico de jugadores `--chart-4`) y formatea las horas en el idioma del panel (`chartLocale`, `formatClock`), no en el del navegador. Los gráficos en sí se describen en [07](./07-components-detailed.md#gráficos).

## Captcha (`components/Turnstile.tsx`)

`useCaptcha()` devuelve `{ widget, fields, ready, reset }` para los formularios de acceso, según `config.captcha`:

- `builtin`: pide un reto a `GET /auth/challenge` y resuelve la prueba de trabajo en segundo plano (`lib/proof-of-work.ts`, esquema ALTCHA, con su propio SHA-256 porque `crypto.subtle` no existe en HTTP sin cifrar). Una línea discreta bajo el formulario muestra el progreso; no hay nada que pulsar.
- `turnstile`: carga Cloudflare Turnstile una sola vez desde `challenges.cloudflare.com` y muestra el widget con `turnstileSiteKey`.
- vacío: sin comprobación. Los paneles demasiado antiguos para enviar `captcha` recurren a Turnstile cuando `turnstileEnabled` y `turnstileSiteKey` tienen valor.

El botón de envío permanece desactivado hasta `ready` (hay un token, o no hay comprobación configurada). Si la comprobación integrada falla, la línea ofrece reintentar. El formulario incluye `fields` (`captchaToken` o `turnstileToken`) en la petición y llama a `reset()` tras cada intento, porque un token solo vale una vez. El comportamiento en el servidor se documenta en [Turnstile y captcha](../general/17-turnstile.md).
