# Enrutamiento y Páginas (Astro)

Las rutas del frontend están determinadas por la estructura de archivos en `src/pages/` (file-based routing de Astro).

## Estructura de Páginas

```
src/pages/
├── index.astro              # /          → Landing/redirección a login o dashboard
├── login.astro              # /login/    → Inicio de sesión
├── register.astro           # /register/ → Registro de usuario
├── forgot-password.astro    # /forgot-password/ → Solicitar reset de contraseña
├── reset-password.astro     # /reset-password/ → Ingresar nueva contraseña (?token=)
├── dashboard.astro          # /dashboard/ → Panel principal con métricas globales
├── servers/
│   ├── index.astro          # /servers/  → Lista de servidores
│   ├── view.astro           # /servers/view/ → Vista de servidor individual
│   └── [id].astro           # /servers/:id/ → Detalle del servidor (parámetro dinámico)
├── nodes/
│   ├── index.astro          # /nodes/    → Lista de nodos
│   ├── view.astro           # /nodes/view/ → Vista de nodo individual
│   └── [id].astro           # /nodes/:id/ → Detalle de nodo
├── users.astro              # /users/    → Administración de usuarios
├── templates.astro          # /templates/ → Repositorios y plantillas
├── settings.astro           # /settings/ → Configuración global del panel
├── roles.astro              # /roles/    → Roles y permisos
├── database-hosts.astro     # /database-hosts/ → Hosts de bases de datos
└── profile/
    └── settings.astro       # /profile/settings/ → Perfil del usuario
```

## Hidratación de Componentes React

Las páginas Astro actúan como contenedores que hidratan componentes React con la directiva `client:only`:

```astro
---
// login.astro
// Carga el componente de login completamente del lado del cliente
---
<Layout title="Login">
  <LoginWrapper client:only="react" />
</Layout>
```

## Navegación

- **AuthShell.tsx** — Layout para páginas sin autenticación (`/login/`, `/register/`, `/forgot-password/`, `/reset-password/`).
- **AppShell.tsx** — Layout principal con sidebar y header para páginas autenticadas. Usa `hasScope()` para verificar permisos y renderizar enlaces del sidebar (ej. `/servers/` requiere `server.view`).
- `AuthContext` redirige automáticamente:
  - Usuario no autenticado en ruta privada → `/login/`
  - Usuario autenticado en `/login/` o `/register/` → `/dashboard/`

## Vista de Configuración del Servidor (`/servers/:id` → Settings)

La pestaña **Settings** del servidor (`features/servers/[id]/settings-view.tsx`) fue rediseñada completamente con:

- **Layout 2 columnas** (grid responsive: 1 col en móvil, 2 en desktop).
- **Identidad visual por sección**: cada tarjeta tiene color/border/gradient propio:
  - Información General (slate), Grupos/Variables (blue), Plugins (violet), Auto-start (emerald)
  - Puertos (primary/sky), Límites de Recursos (cyan con barras de progreso), Metadatos (amber, solo admin)
- **Sticky Save Bar** inferior fija al hacer scroll.
- **Permission Gates** por rol:
  - Admin: todas las secciones visibles + CRUD completo.
  - Usuario: Información General (ver), Grupos/Variables (ver+editar), Plugins (ver+editar), Auto-start (ver+editar), Puertos (ver, elegir primario, notas, sin CRUD números), **NO** Límites, Metadatos, Admin tab.
- **Puertos Extra**: Usuarios ven puertos asignados (solo lectura), eligen primario, editan notas; Admins tienen CRUD completo.
- **Metadatos Card**: Solo admin, key-value read-only, filtra vars internas (`resolved*`, `forge*`, `javaVersion*`, `build*`, `git*`).
- **Recursos**: Barras de progreso coloreadas (CPU cyan, RAM emerald, Disco violet) con porcentajes.

## Corrección Sidebar (`AppShell.tsx`)

Se corrigió la verificación de scopes en la navegación lateral:

```tsx
// Antes (incorrecto)
scopes.includes('admin')

// Después (correcto)
hasScope('admin') || hasScope('server.admin')
```

Y la ruta `/servers/` ahora requiere `['server.view']` (no `['admin']`), permitiendo acceso a usuarios con scope de servidor.

## Parámetros Dinámicos

Las rutas con `[id].astro` usan parámetros dinámicos de Astro:

```
/servers/:id   → servers/[id].astro → params.id
/nodes/:id     → nodes/[id].astro   → params.id
```

Los componentes React acceden al ID mediante `Astro.params` y lo pasan como prop a los componentes hijos.
