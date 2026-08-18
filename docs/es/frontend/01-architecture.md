# Arquitectura del Frontend (Client)

Este documento detalla la estructura, las tecnologías y la arquitectura general de la interfaz de usuario (frontend) de Aether Panel.

## Stack Tecnológico

| Tecnología | Versión | Uso |
|---|---|---|
| **Astro** | ^7.1.3 | Framework web con arquitectura de Islas, enrutamiento basado en archivos (`src/pages/`) |
| **React** | ^19.2.8 | Componentes interactivos de UI |
| **Tailwind CSS** | ^3.4.1 | Estilos utilitarios |
| **Shadcn UI / Radix UI** | — | 33 componentes accesibles (Dialog, Select, Tabs, etc.) |
| **react-hook-form** | ^7.83.0 | Manejo de formularios |
| **zod** | ^4.4.3 | Validación de esquemas |
| **Monaco Editor** | ^4.6.1 | Editor de código/configuraciones en el navegador |
| **Genkit** | ^1.40.1 | Integración con Google GenAI para análisis de logs |
| **recharts** | ^2.15.1 | Gráficos de métricas (CPU, RAM) |
| **lucide-react** | ^1.26.0 | Iconos |
| **clsx + tailwind-merge** | ^2.1.1 / ^3.0.1 | Manejo condicional de clases CSS |
| **sileo** | ^0.1.5 | Notificaciones toast |

## Estructura de Directorios (`frontend/src/`)

```
src/
├── ai/                      # Lógica de Genkit AI (flows, prompts)
│   ├── genkit.ts            # Configuración del cliente Genkit
│   ├── dev.ts               # Utilidad de desarrollo
│   └── flows/               # Flujos de IA (summarize, troubleshooting)
├── components/              # Componentes visuales
│   ├── ui/                  # 33 componentes Shadcn/Radix UI
│   ├── AppShell.tsx         # Layout principal del dashboard
│   ├── AuthShell.tsx        # Layout de autenticación
│   ├── SileoToaster.tsx     # Renderer de notificaciones toast
│   ├── Turnstile.tsx        # Widget Cloudflare Turnstile
│   ├── server-card.tsx      # Tarjeta de servidor
│   ├── metrics-charts.tsx   # Gráficos de métricas
│   ├── network-usage-chart.tsx
│   ├── resource-usage-chart.tsx
│   ├── page-header.tsx
│   ├── PageWrappers.tsx
│   ├── logo.tsx
│   └── ErrorBoundary.tsx
├── contexts/                # Estado global de React
│   ├── providers.tsx        # AuthContext + Providers wrapper
│   ├── config-context.tsx   # ConfigContext (branding, temas desde /api/config)
│   └── translations-context.tsx  # i18n (en/es)
├── features/                # Lógica agrupada por dominio
│   ├── auth/                # Login, registro
│   ├── dashboard/           # Panel principal con métricas globales
│   ├── database-hosts/      # Administración de hosts de base de datos
│   ├── nodes/               # Gestión de nodos
│   ├── profile/             # Perfil de usuario
│   ├── roles/               # Roles y permisos
│   ├── servers/             # CRUD, consola, archivos, backups, plugins
│   ├── settings/            # Configuración del panel
│   ├── templates/           # Plantillas de servidores
│   └── users/               # Usuarios globales
├── hooks/                   # Custom hooks de React
│   ├── use-dashboard-data.ts
│   ├── use-database-hosts.ts
│   ├── use-mobile.tsx
│   ├── use-nodes.ts
│   ├── use-profile.ts
│   ├── use-server-settings.ts
│   ├── use-servers.ts
│   ├── use-settings.ts
│   ├── use-templates.ts
│   └── use-users.ts
├── layouts/
│   └── BaseLayout.astro     # Layout base HTML
├── lib/                     # Utilidades y configuración
│   ├── api-client.ts        # SDK HTTP (fetch nativo, singleton api object)
│   ├── utils.ts             # Utilidades generales (cn())
│   ├── ansi-utils.tsx       # Parseo de colores ANSI para la consola
│   ├── data.ts              # Datos y constantes
│   ├── toast.ts             # Notificaciones toast (sileo)
│   └── locales/             # Traducciones (en.json, es.json)
├── pages/                   # Rutas de Astro (file-based routing)
│   ├── index.astro
│   ├── login.astro
│   ├── register.astro
│   ├── dashboard.astro
│   ├── servers/             # CRUD de servidores
│   ├── nodes/               # Detalle de nodos
│   ├── users.astro
│   ├── templates.astro
│   ├── settings.astro
│   ├── roles.astro
│   ├── database-hosts.astro
│   └── profile/
└── styles/
    └── globals.css          # Estilos globales y variables CSS
```

## SDK de API (`lib/api-client.ts`)

No existe un paquete separado `client/api/`. El SDK es un único archivo de 94 líneas en `frontend/src/lib/api-client.ts` que exporta un objeto `api` con métodos `get`, `post`, `postForm`, `put`, `delete` usando la **API nativa `fetch`** del navegador con `credentials: 'include'` para autenticación por cookies/sesión.

Las llamadas a la API se realizan directamente desde los features y hooks, no desde clases separadas por dominio:

```typescript
import { api } from '@/lib/api-client';
const servers = await api.get('/api/servers');
await api.post('/api/servers/' + id + '/start', {});
```

## Cliente HTTP Nativo

El SDK usa `fetch` nativo (no axios, no js-cookie). Las peticiones incluyen automáticamente las cookies de sesión gracias a `credentials: 'include'`. Los errores HTTP se convierten en `ApiError` con `status`, `message` y `data`.

## Compilación

El paquete `frontend/` se compila con `astro build`, generando un sitio estático en `frontend/dist/`. El servidor Go embebe estos archivos usando `embed.FS` y los sirve en producción.

```bash
cd client/frontend
npm run build    # Genera client/frontend/dist/
# Opcional: vista previa
npm run start    # astro preview
npm run dev      # astro dev --port 9002
```
