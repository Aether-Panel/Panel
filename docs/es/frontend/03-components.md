# Componentes e Interfaz Gráfica (`client/frontend`)

La interfaz visual con la que los usuarios interactúan (dashboard, login, consola web, etc.). Usa un enfoque híbrido combinando Astro (renderizado estático) y React 19 (componentes interactivos).

## Tecnologías de UI Core

### 1. Astro (Enrutamiento y SSR)
- Las rutas están determinadas por archivos en `src/pages/`.
- Las páginas `.astro` actúan como contenedores e "hidratan" componentes React con directivas como `client:only="react"`.

### 2. React 19 + Tailwind CSS
- Todos los componentes interactivos (formularios, consolas, modales) están en React 19.
- Estilizado 100% utilitario con Tailwind CSS y `tailwind-merge` + `clsx` para clases condicionales.

## Componentes UI Base (Shadcn UI / Radix UI)

En `src/components/ui/` hay 35 componentes basados en Radix UI primitives:

| Componente | Propósito |
|---|---|
| Accordion, Collapsible | Paneles expandibles |
| Alert, AlertDialog | Diálogos de confirmación/alerta |
| Avatar | Avatares de usuario |
| Badge | Etiquetas/badges de estado |
| Button | Botones con variantes |
| Calendar, DatePicker | Selección de fechas |
| Card | Tarjetas de contenido |
| Carousel | Carrusel de imágenes |
| Chart | Gráficos (envuelve recharts) |
| Checkbox, RadioGroup, Switch | Inputs de selección |
| Dialog, Sheet, Sidebar | Paneles modales y laterales |
| DropdownMenu, Menubar | Menús desplegables |
| Form, Input, Label, Textarea, Select | Elementos de formulario |
| Popover, Tooltip | Overlays contextuales |
| Progress, Skeleton | Indicadores de carga |
| ScrollArea | Áreas con scroll personalizado |
| Separator | Separadores visuales |
| Slider | Control deslizante |
| Table | Tablas de datos |
| Tabs | Navegación por pestañas |
| Toast / Toaster | Notificaciones emergentes |

## Componentes Custom (`src/components/`)

- **AppShell.tsx** — Layout principal del dashboard (sidebar, header, contenido)
- **AuthShell.tsx** — Layout para páginas de autenticación
- **ErrorBoundary.tsx** — Manejo de errores de React
- **logo.tsx** — Componente del logo del panel
- **page-header.tsx** — Encabezado de página reutilizable
- **PageWrappers.tsx** — Wrappers de página con permisos
- **metrics-charts.tsx, network-usage-chart.tsx, resource-usage-chart.tsx** — Gráficos de métricas con recharts

## Features y Dominios (`src/features/`)

### Consola / Terminal (`features/servers/[id]/`)
- `console-view.tsx` — Terminal en vivo con WebSocket, parseo de colores ANSI (`lib/ansi-utils.tsx`), envío de comandos
- La conexión WebSocket se establece a `ws[s]://host/api/servers/:id/socket?console`

### File Manager (`features/servers/[id]/`)
- `file-manager-view.tsx` — Explorador de archivos con navegación, subida, descarga, creación, edición, eliminación
- Integración con Monaco Editor (`code-editor.tsx`) para edición de archivos (tema Tokyo Night, lazy-loaded)
- Operaciones: crear carpeta, subir archivo, comprimir/extraer ZIP

### Otras Features
- **auth/** — Login, registro, OTP, recuperación de contraseña (forgot/reset)
- **dashboard/** — Panel principal con métricas globales, uptime, estado de servidores
- **servers/** — CRUD, detalles, gestión de backups, plugins, tareas, flags, base de datos, sub-usuarios, transferencias
- **nodes/** — Listado, creación, detalle, despliegue
- **users/** — CRUD de usuarios globales, permisos
- **templates/** — Repositorios de plantillas
- **settings/** — Configuración global del panel, tests de email/discord, licencia
- **roles/** — Roles y asignación de scopes
- **database-hosts/** — Hosts de bases de datos externas
- **profile/** — Perfil propio, OTP, OAuth2 clients

## Estado Global (`src/contexts/`)

| Contexto | Archivo | Propósito |
|---|---|---|
| **AuthContext** | `providers.tsx` | Sesión del usuario (login, register, logout, role, scopes, hasScope). Redirige a `/login/` si no hay sesión. |
| **ConfigContext** | `config-context.tsx` | Configuración pública del panel desde `GET /api/config` (branding, nombre, registro habilitado, temas). |
| **TranslationsContext** | `translations-context.tsx` | Internacionalización (i18n) con `en.json` y `es.json`. |

## Custom Hooks (`src/hooks/`)

| Hook | Propósito |
|---|---|
| `use-servers` | CRUD de servidores |
| `use-users` | CRUD de usuarios y permisos |
| `use-nodes` | CRUD de nodos y datos de despliegue |
| `use-templates` | Repositorios y plantillas |
| `use-database-hosts` | Hosts de bases de datos |
| `use-settings` | Configuración global del panel |
| `use-server-settings` | Variables/configuración de un servidor |
| `use-profile` | Perfil propio, OTP, OAuth2 clients |
| `use-dashboard-data` | Datos del dashboard (uptime, servidores) |
| `use-toast` | Notificaciones toast |
| `use-mobile` | Detección de dispositivo móvil |

## Integración Genkit AI (`src/ai/`)

- `genkit.ts` — Configuración del cliente Genkit con Google Generative AI
- `flows/` — Flujos de IA:
  - `summarize-server-alerts.ts` — Resumen de alertas del servidor
  - `generate-troubleshooting-tips.ts` — Sugerencias de solución de problemas
- Se invoca desde el botón "AI Analyze" en la vista de servidor

## Flujo de Compilación

```bash
npm run build    # astro build → client/frontend/dist/
npm run dev      # astro dev --port 9002 (desarrollo)
npm run start    # astro preview
npm run lint     # astro check
npm run typecheck # tsc --noEmit
```

Los archivos compilados en `dist/` son embebidos por el backend Go mediante `embed.FS` y servidos en producción desde el binario del panel.
