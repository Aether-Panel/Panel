# Arquitectura del frontend

Este documento describe cómo está construida la interfaz web de Aether Panel (`client/frontend`): las tecnologías que usa, cómo se organiza el código fuente, cómo una página pasa de un archivo Astro a una pantalla interactiva de React y cómo el resultado acaba dentro del binario del panel.

Documentos relacionados:

- [02. Consumo de la API](./02-endpoints.md): el cliente HTTP, los hooks y los endpoints que llama cada pantalla.
- [03. Componentes e interfaz](./03-components.md): visión general de los shells, los componentes compartidos y el estado global.
- [05. Traducciones](./05-translations.md): el sistema de i18n.
- [06. Rutas y páginas](./06-pages.md): cada ruta y lo que muestra.
- [07. Referencia detallada](./07-components-detailed.md): referencia archivo por archivo de features, hooks y utilidades.
- [04. IA](./04-ai.md) y [08. IA en detalle](./08-ai-detailed.md): el asistente del servidor y el análisis de registros.

## Stack tecnológico

Las versiones son los rangos declarados en `client/frontend/package.json`.

| Tecnología | Versión | Uso |
|---|---|---|
| **Astro** | ^7.4.4 | Herramienta de compilación y enrutamiento basado en archivos (`src/pages/`). Cada página es HTML estático que monta una única isla de React con `client:only="react"`. La navegación usa el `ClientRouter` de Astro (view transitions). |
| **React** / **React DOM** | ^19.2.8 | Toda la interfaz interactiva. |
| **Tailwind CSS** | ^3.4.1 | Estilos utilitarios, con `tailwindcss-animate`. Los tokens de diseño (colores, radios, sombras, curvas de animación) son variables CSS en `src/styles/globals.css`. |
| Primitivas de **Radix UI** | varias | Bloques accesibles que envuelven los componentes estilo shadcn de `src/components/ui/` (25 archivos). |
| **class-variance-authority**, **clsx**, **tailwind-merge** | ^0.7.1 / ^2.1.1 / ^3.0.1 | Variantes de componentes y el helper de clases `cn()` (`src/lib/utils.ts`). |
| **react-hook-form** + **@hookform/resolvers** + **zod** | ^7.90.0 / ^6.0.2 / ^4.4.3 | Formularios y validación de esquemas. |
| **Monaco Editor** (`monaco-editor`, `@monaco-editor/react`) | ^0.57.0 / ^4.6.1 | Editor de archivos y visor de diferencias del gestor de archivos. Se carga bajo demanda y su hoja de estilos va incluida (funciona sin conexión y con una CSP estricta). |
| **recharts** | ^2.15.1 | Todos los gráficos (métricas del servidor, historial de rendimiento, historial de red del panel principal). |
| **@phosphor-icons/react** | ^2.1.10 | Iconos, reexportados desde `src/components/icons.tsx`. |
| **sileo** | ^0.1.6 | Notificaciones toast (`src/lib/toast.ts`). |
| **three** | 0.186.1 | El emblema 3D del panel de marca en el inicio de sesión (`src/lib/emblem-3d.ts`). |
| **TypeScript** | ^6.0.3 | Comprobación de tipos (`npm run typecheck`). |

El frontend no incluye ningún SDK de IA: las funciones de IA llaman a la API del panel, que habla con Google Gemini desde el servidor (ver [04. IA](./04-ai.md)). Tampoco hay librería de gestión de estado ni de obtención de datos (ni React Query ni Redux): los datos viven en hooks propios y en contextos de React.

## Árbol de código (`client/frontend/src/`)

```
src/
├── pages/                 # Rutas de Astro; cada página monta un page wrapper (ver 06-pages.md)
├── layouts/
│   └── BaseLayout.astro   # <html>, fuentes, ClientRouter, scripts de build obsoleto y limpieza de portales
├── components/
│   ├── page-wrappers/     # Una entrada de React por página: AppShell o AuthShell + la página del feature
│   ├── ui/                # 25 primitivas estilo shadcn sobre Radix (button, dialog, sheet, sidebar, chart…)
│   ├── AppShell.tsx       # Layout con sesión: barra lateral plegable, barra superior con migas y búsqueda rápida
│   ├── AuthShell.tsx      # Layout de acceso y piezas compartidas de autenticación (AuthFrame, campos, emblema)
│   ├── quick-search.tsx   # Paleta de comandos Ctrl/⌘+K
│   ├── server-card.tsx    # Tarjeta de servidor, insignia de estado, barras de uso y sparkline
│   ├── user-avatar.tsx    # Foto de la cuenta o avatar de Kodama
│   ├── Turnstile.tsx      # useCaptcha(): prueba de trabajo integrada o Cloudflare Turnstile
│   ├── SileoToaster.tsx   # Renderizador de toasts
│   ├── ErrorBoundary.tsx, empty-state.tsx, page-header.tsx, logo.tsx, icons.tsx,
│   ├── ambient-backdrop.tsx, spotlight.tsx, glide-menu.tsx, animated-number.tsx,
│   └── beta-badge.tsx, player-presence.tsx
├── contexts/
│   ├── providers.tsx            # Envoltorio Providers + AuthContext (useAuth)
│   ├── config-context.tsx       # Configuración pública del panel desde GET /api/config (useConfig, useExperimental)
│   └── translations-context.tsx # i18n (useTranslations)
├── features/              # Pantallas agrupadas por dominio
│   ├── auth/              # Tarjeta deslizante de acceso/registro, contraseña olvidada/restablecer
│   ├── dashboard/         # Franja de estado, tarjetas KPI, historial de red, registro de accesos
│   ├── servers/           # Lista de servidores, asistente de creación y detalle del servidor ([id]/)
│   ├── nodes/             # Lista de nodos, detalle del nodo ([id]/), alerta de disco bajo
│   ├── users/, roles/, templates/, database-hosts/
│   ├── settings/          # Pestañas de configuración del panel (general, Discord, correo, copias externas, dominios, claves API…)
│   ├── profile/settings/  # Cuenta propia: datos, contraseña, 2FA, claves SSH, clientes OAuth2, idioma
│   └── extensions/        # Páginas añadidas por extensiones instaladas (iframes aislados)
├── hooks/                 # Hooks de datos (use-servers, use-nodes, use-network-history…)
├── lib/
│   ├── api-client.ts      # Envoltorio de fetch, ApiError, errores de API traducidos, redirección al caducar la sesión
│   ├── paged.ts           # fetchAllPages() para listas paginadas
│   ├── chart-theme.ts     # Colores y formato de hora comunes a todos los gráficos
│   ├── toast.ts           # Envoltorio de sileo
│   ├── ansi-utils.tsx     # Colores ANSI en las líneas de la consola
│   ├── data.ts            # Tipos compartidos (Server, User, Node)
│   ├── players.ts, game-logos.ts, keyed.ts, stale-build.ts, proof-of-work.ts,
│   ├── monaco-setup.ts, emblem-3d.ts, utils.ts
│   └── locales/           # en.json, es.json
└── styles/globals.css     # Capas de Tailwind, tokens de diseño, variables de tema
```

El alias de importación `@/` apunta a `src/` (configurado en `astro.config.mjs` y `tsconfig.json`).

## De la ruta a la pantalla

1. Una petición como `/servers/view/?id=abc` recibe `pages/servers/view.astro`, convertido en HTML estático durante la compilación.
2. La página usa `BaseLayout.astro` y monta su wrapper de `components/page-wrappers/` con `client:only="react"`, de modo que React no renderiza nada en el servidor.
3. El wrapper coloca la página del feature dentro de un shell:
   - `AppShell` para las páginas con sesión (barra lateral, barra superior, toasts),
   - `AuthShell` para `/login/`, `/register/`, `/forgot-password/` y `/reset-password/`.
4. Ambos shells renderizan `Providers` (`contexts/providers.tsx`), que anida `TranslationsProvider` → `ConfigProvider` → `AuthProvider`. `AuthProvider` llama a `GET /api/self`; mientras no responde muestra una pantalla de carga, y envía a `/login/` a quien no tiene sesión (recordando la página a la que volver).
5. La página del feature obtiene sus datos mediante hooks y `api` (ver [02](./02-endpoints.md)).

El detalle de cada ruta está en [06. Rutas y páginas](./06-pages.md).

## `BaseLayout.astro`

- Fija `<html lang="en" class="dark">`; el contexto de traducciones actualiza `lang` cuando cambia el idioma.
- Precarga la fuente Geist incluida en `public/fonts` (sin peticiones a Google Fonts).
- Activa `ClientRouter` y añade dos scripts en línea:
  - elimina los portales de Radix (diálogos y menús abiertos) antes de que Astro sustituya la página, para que no queden visibles tras una navegación en el cliente;
  - ante `vite:preloadError` (un fragmento que ya no existe porque el panel se actualizó con la página abierta) recarga la página, como mucho una vez por minuto. `lib/stale-build.ts` hace lo mismo con las importaciones diferidas de React.

## Compilación y servicio

```bash
cd client/frontend
npm run dev        # astro dev --port 9002
npm run build      # astro build -> client/frontend/dist/
npm run start      # astro preview
npm run lint       # astro check
npm run typecheck  # tsc --noEmit
```

- `client/frontend/webfiles.go` incrusta `dist/` en el binario de Go (`//go:embed all:dist`), por lo que `npm run build` debe ejecutarse antes de `go build`. La imagen Docker lo hace en su etapa de compilación con Node.
- `internal/web/loader.go` sirve esos archivos para cualquier ruta fuera de los prefijos de la API. Cuando `panel.web.files` tiene valor (por defecto `www`, relativo al directorio de trabajo del panel), un archivo presente en ese directorio se sirve en lugar del incrustado con la misma ruta; lo que no esté allí se toma de la compilación incrustada. Así se pueden sustituir recursos estáticos sin recompilar.
- Las rutas de la API, `/auth` y `/oauth2` se registran en el mismo servidor, de modo que el frontend siempre habla con su propio origen usando la cookie de sesión.
