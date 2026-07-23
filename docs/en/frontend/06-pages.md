# Enrutamiento y Páginas (Astro)

Las rutas del frontend están determinadas por la estructura de archivos en `src/pages/` (file-based routing de Astro).

## Estructura de Páginas

```
src/pages/
├── index.astro              # /          → Landing/redirección a login o dashboard
├── login.astro              # /login/    → Inicio de sesión
├── register.astro           # /register/ → Registro de usuario
├── dashboard.astro          # /dashboard/ → Panel principal con métricas globales
├── servers/
│   ├── index.astro          # /servers/  → Lista de servidores
│   ├── view.astro           # /servers/view/ → Vista de servidor individual
│   └── [id].astro           # /servers/:id/ → Detalle del servidor (parámetro dinámico)
├── nodes/
│   ├── index.astro          # /nodes/    → Lista de nodos
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

- **AuthShell.tsx** — Layout para páginas sin autenticación (`/login/`, `/register/`).
- **AppShell.tsx** — Layout principal con sidebar y header para páginas autenticadas.
- `AuthContext` redirige automáticamente:
  - Usuario no autenticado en ruta privada → `/login/`
  - Usuario autenticado en `/login/` o `/register/` → `/dashboard/`

## Parámetros Dinámicos

Las rutas con `[id].astro` usan parámetros dinámicos de Astro:

```
/servers/:id   → servers/[id].astro → params.id
/nodes/:id     → nodes/[id].astro   → params.id
```

Los componentes React acceden al ID mediante `Astro.params` y lo pasan como prop a los componentes hijos.
