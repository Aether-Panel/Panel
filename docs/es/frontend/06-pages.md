# Rutas y páginas

Las rutas salen de los archivos de `src/pages/` (enrutamiento basado en archivos de Astro). Cada página es un archivo `.astro` estático que monta un wrapper de `src/components/page-wrappers/` con `client:only="react"`; el wrapper coloca la página del feature dentro de `AppShell` o `AuthShell` (ver [03. Componentes](./03-components.md#shells-de-página)). Las páginas de detalle toman el id de la query string, no de la ruta, para que todo el sitio siga siendo estático.

## Mapa de rutas

| URL | Archivo Astro | Wrapper → feature | Shell |
|---|---|---|---|
| `/` | `index.astro` | Redirección (meta refresh) a `/dashboard/` | — |
| `/login/` | `login.astro` | `Login` → `features/auth/login/page.tsx` (tarjeta deslizante) | Acceso |
| `/register/` | `register.astro` | `Register` → `features/auth/register/page.tsx` (tarjeta deslizante) | Acceso |
| `/forgot-password/` | `forgot-password.astro` | `ForgotPassword` → `features/auth/forgot-password/page.tsx` | Acceso |
| `/reset-password/?token=…` | `reset-password.astro` | `ResetPassword` → `features/auth/reset-password/page.tsx` | Acceso |
| `/dashboard/` | `dashboard.astro` | `Dashboard` → `features/dashboard/page.tsx` | App |
| `/servers/` | `servers/index.astro` | `Servers` → `features/servers/page.tsx` | App |
| `/servers/view/?id=<id>` | `servers/view.astro` | `ServerDetail` → `features/servers/[id]/page.tsx` | App |
| `/nodes/` | `nodes/index.astro` | `Nodes` → `features/nodes/page.tsx` | App |
| `/nodes/view/?id=<id>` | `nodes/view.astro` | `NodeDetail` → `features/nodes/[id]/page.tsx` | App |
| `/users/` | `users.astro` | `Users` → `features/users/page.tsx` | App |
| `/roles/` | `roles.astro` | `Roles` → `features/roles/page.tsx` | App |
| `/templates/` | `templates.astro` | `Templates` → `features/templates/page.tsx` | App |
| `/database-hosts/` | `database-hosts.astro` | `DatabaseHosts` → `features/database-hosts/page.tsx` | App |
| `/settings/` | `settings.astro` | `Settings` → `features/settings/page.tsx` | App |
| `/profile/settings/` | `profile/settings.astro` | `ProfileSettings` → `features/profile/settings/page.tsx` | App |
| `/extension/?id=<ext>&page=<página>` | `extension.astro` | `Extension` → `features/extensions/page.tsx` | App |

Las carpetas `[id]` de `features/` son solo una convención de nombres para las pantallas de detalle; no hay rutas dinámicas de Astro. Los wrappers de detalle leen `id` de `Astro.url.searchParams` (o de `window.location.search`) y lo pasan como prop, y el shell recibe `currentPath` (`/servers/<id>`) para las migas de pan y el elemento activo del menú.

Las rutas desconocidas las responde el manejador 404 del backend.

## Redirecciones

`AuthProvider` (`contexts/providers.tsx`) decide dónde puede estar cada visitante:

- Sin sesión en una página con sesión → `/login/`. La ruta actual se guarda en `sessionStorage` (`aether:return-to`) para volver a ella después de iniciar sesión.
- Con sesión en una página de acceso → la ruta guardada si es una página del propio panel (nunca `/login` ni `/register`, nunca una URL externa); si no, `/dashboard/`.
- Una petición que responde 401 con una página abierta → `/login/?expired=1` con un aviso de sesión caducada (ver [02](./02-endpoints.md#sesiones-caducadas)).

## Navegación lateral

Los administradores ven todos los elementos; el resto, solo los que corresponden a alguno de los scopes indicados. La API aplica las mismas reglas: ocultar un elemento es una comodidad, no la protección.

| Elemento | Grupo | Scope necesario |
|---|---|---|
| Inicio (panel principal) | Resumen | ninguno |
| Servidores | Gestión | `login` (la lista solo contiene servidores que el usuario puede abrir) |
| Usuarios | Gestión | `users.info.search` |
| Roles | Gestión | `admin` |
| Nodos | Gestión | `nodes.view` |
| Hosts de BD | Gestión | `admin` |
| Plantillas | Gestión | `templates.view` |
| Configuración | Gestión | `settings.edit` |
| Páginas de extensiones | Extensiones | `admin` (un elemento por cada página `admin` de una extensión activa) |

Configuración de la cuenta y Cerrar sesión están en el menú de la cuenta, al pie de la barra lateral. El plegado de la barra lateral se describe en [03](./03-components.md#appshell-páginas-con-sesión).

## Páginas de acceso

- **Inicio de sesión y registro** comparten la tarjeta deslizante (`features/auth/auth-card.tsx`): cambiar de formulario se anima en la misma página y actualiza la URL; el botón Atrás vuelve al anterior. Con el registro desactivado (`registrationEnabled: false` en `/api/config`), `/register/` muestra el formulario de inicio de sesión.
- **Inicio de sesión** envía `POST /auth/login` con correo, contraseña y token del captcha. Si la cuenta tiene verificación en dos pasos, la API responde `otpNeeded` y la tarjeta pasa a un paso de código de 6 dígitos (`POST /auth/otp`), con la opción de escribir un código de recuperación; el servidor conserva el inicio de sesión pendiente durante 5 minutos. Un 429 muestra "demasiados intentos".
- La página de inicio de sesión también muestra avisos según la URL: `?expired=1` (sesión caducada) y `?sso=expired` / `?sso=denied` (un enlace de inicio de sesión único caducado o rechazado).
- **Contraseña olvidada** (`POST /auth/forgot-password`, con captcha) y **restablecer contraseña** (`POST /auth/reset-password` con el `token` del enlace del correo y la nueva contraseña, con medidor de robustez) usan `AuthFrame`.
- El captcha es la prueba de trabajo integrada o Cloudflare Turnstile, según la configuración (ver [03](./03-components.md#captcha-componentsturnstiletsx)).

## Panel principal (`/dashboard/`)

Lo que muestra depende de los scopes del usuario:

- **Con `nodes.view` o `users.info.search`** (operadores): una franja de estado (saludo, disponibilidad de los últimos 30 días según `/api/uptime`, servidores en línea, tráfico de red en directo), una alerta por cada nodo cuyo disco de servidores esté casi lleno (solo con `nodes.view`), tarjetas KPI de CPU, memoria y almacenamiento (medias de los nodos si se ven los nodos; si no, medias de los servidores) y número de servidores, una tarjeta con información del panel (versión, servidores, usuarios, nodos, estado del registro), el **gráfico de historial de red** y el **registro de accesos recientes** (inicios de sesión, intentos fallidos, registros; requiere `users.info.search`).
- **El resto**: una cabecera de bienvenida y una cuadrícula con las tarjetas de sus servidores.

El gráfico de historial de red se alimenta de `GET /api/nodes/network-history`, que suma el historial que guarda cada nodo (un punto por minuto durante 24 h), así que muestra el tráfico anterior a abrir la página. Ver [07](./07-components-detailed.md#panel-principal-featuresdashboard).

## Servidores (`/servers/`)

Lista de los servidores del usuario con búsqueda y filtros (`/` enfoca la búsqueda; filtro de estado todos/en línea/apagados), con CPU, memoria, disco y jugadores en directo. La página solo exige `login`, como la entrada del menú: `GET /api/servers` ya devuelve únicamente los servidores de los que el usuario es propietario o miembro, así que también la ven los propietarios y subusuarios sin `server.view` global. Los administradores tienen el diálogo **Crear servidor** (`create-server-stepper.tsx`), un asistente de tres pasos:

1. **Entorno**: nodo (Docker es el único entorno que se ofrece).
2. **Plantilla**: repositorio y plantilla, con logotipos de juegos.
3. **Configuración**: nombre, dueño, recursos, puertos (comprobados en directo contra el nodo con `POST /api/nodes/:id/ports/check`) y variables de la plantilla (las versiones de Minecraft se obtienen de `/api/game-versions/:platform`).

El servidor se crea con `PUT /api/servers/:id`. `/servers/?create=true` abre el diálogo directamente. Los usuarios sin `admin` solo pueden crear subservidores desde la pestaña Splitter de un servidor.

## Detalle del servidor (`/servers/view/?id=<id>`)

La cabecera muestra el nombre, el estado, una insignia de "suspendido", la dirección de conexión, el botón **Asistente** (con `server.console`, ver [04](./04-ai.md)) y los botones de encendido (iniciar, detener, reiniciar; **Forzar detención** sustituye a Detener, con confirmación, si el servidor sigue en línea 7 segundos después de pedir la parada y el usuario tiene `server.kill`).

Las pestañas se muestran según los scopes del usuario **en este servidor** (`GET /api/servers/:id?perms`): un subusuario solo tiene lo que el dueño le concedió, así que los scopes globales del panel solo se usan hasta conocer esos. `admin` y `server.admin` en el servidor dan acceso a todas las pestañas. En móviles las pestañas se convierten en un selector; en escritorio, las que no caben pasan a un menú "Más". La última pestaña usada se recuerda por servidor en `localStorage` (`serverTab_<id>`).

| Pestaña | Se muestra cuando |
|---|---|
| Consola | `server.console` (escribir requiere `server.console.send`) |
| Resumen | siempre |
| Archivos | `server.files.view` |
| Configuración | `server.data.view` o `server.flags.view` |
| Propiedades (editor de `server.properties`, Beta) | Minecraft Java + `propertiesEditor` experimental + `server.files.edit` |
| Usuarios | `server.users.view` |
| Base de datos | `server.data.edit` |
| Copias de seguridad | `server.backup.view` |
| Tareas | `server.tasks.view` |
| Actividad | siempre |
| Acceso SFTP | `server.sftp` |
| Migración (transferencia externa) | `server.data.edit.admin` |
| Splitter | dueño del servidor o `server.data.edit.admin` |
| Plugins | `server.files.edit`, salvo que el interruptor de Plugins de la pestaña Configuración esté apagado (una preferencia de este navegador guardada en `localStorage` como `pluginsEnabled_<id>`) |
| Modpacks, Mapa | servidores Minecraft Java |
| Administración | cualquiera de `server.admin.view`, `server.admin.install.view`, `server.install`, `server.admin.transfer.view`, `server.data.edit.admin`, `server.admin.config.view`, `server.definition.edit`, `server.admin.assignments.view`, `server.delete` |
| Pestañas de extensiones | una por cada página `server` de una extensión activa |

El contenido de cada pestaña se describe en [07](./07-components-detailed.md#detalle-del-servidor-featuresserversid). Dos comportamientos que conviene conocer:

- **Consola**: salida en directo, filtros, búsqueda, historial y favoritos de comandos, y un **modo ampliado** que abre la consola sobre toda la página (Esc lo cierra). Un aviso sobre ella muestra la última caída durante 24 horas.
- **Caídas**: cuando el socket del servidor informa de una caída, aparece un toast esté donde esté el usuario dentro de la página, el aviso de caída y el historial de caídas del Resumen se refrescan, y el diálogo de la caída puede pedir una explicación a la IA.

### Pestaña Configuración

Las secciones son tarjetas con el título y la descripción a la izquierda y los campos a la derecha (apilados en móviles): General, Recursos (límites de CPU, memoria y disco; visible con `admin`, `server.admin`, `server.admin.config.view` o `server.admin.config.manage`), Puertos (cambiar números requiere `server.data.edit.admin`; las notas, `server.data.edit`), Arranque automático, Plugins (muestra u oculta la pestaña Plugins en este navegador), los grupos de variables de la plantilla, otras variables y Metadatos (valores de instalación de solo lectura como `resolved*`, `forge*`, `javaVersion*`, `build*`, `git*`; solo administradores). La barra de guardado aparece abajo solo cuando hay cambios. Debajo de la tarjeta de configuración están la tarjeta de **versión del juego** (`versionSwitch` experimental), la de **dirección** (el subdominio del servidor) y la de **notificaciones** (avisos del servidor a Discord, Telegram o un webhook).

## Otras páginas

| Página | Contenido | Visible con |
|---|---|---|
| Nodos | Lista de nodos con estado y recursos, alertas de disco bajo e instrucciones de despliegue de un nodo nuevo (una configuración de `docker compose` para copiar) | `nodes.view` |
| Detalle del nodo | Información del sistema, discos, servidores del nodo, alerta de disco bajo, editar y eliminar | `nodes.view` |
| Usuarios | Lista de usuarios, crear, editar, permisos | `users.info.search` (las acciones de gestión requieren más scopes) |
| Roles | Roles y sus scopes | `admin` |
| Plantillas | Repositorios, plantillas locales, editor, subida | `templates.view` (editar requiere `templates.local.edit`; añadir repositorios, `templates.repo.create`) |
| Hosts de BD | Hosts externos MySQL/MariaDB, prueba de conexión | `admin` |
| Configuración | Pestañas: General, Discord, Correo, Copias externas (S3), Dominios (Cloudflare), Claves API, Productos y Extensiones (estas tres solo para `admin`), Actualizaciones | `settings.edit` |
| Configuración de la cuenta (`/profile/settings/`) | Datos de la cuenta, contraseña, verificación en dos pasos, claves SSH, clientes OAuth2, idioma de la interfaz | cualquier usuario con sesión |
| Página de extensión | La página de administración de una extensión en un iframe aislado | `admin` |

## Guías de usuario

Estas páginas describen la interfaz desde el código. Cómo funciona cada función para usuarios y administradores se explica en las [guías de funciones](../features/README.md), por ejemplo: [Panel principal y nodos](../features/dashboard-and-nodes.md), [Configuración del servidor](../features/server-settings.md), [Estadísticas del servidor](../features/server-statistics.md), [Informes de caídas](../features/crash-reports.md), [Historial de archivos](../features/file-history.md), [Copias de seguridad](../features/backups.md), [Tareas programadas](../features/scheduled-tasks.md), [Subservidores](../features/sub-servers.md), [Migraciones de servidores](../features/server-transfers.md), [Modpacks y plugins](../features/modpacks-and-plugins.md), [Mapa del mundo](../features/world-map.md), [Versiones del juego](../features/game-versions.md), [Propiedades del servidor](../features/server-properties.md), [Subdominios](../features/subdomains.md), [Notificaciones del servidor](../features/server-notifications.md), [Clonado](../features/cloning.md), [Actividad y registro de accesos](../features/activity-and-access-log.md), [Plantillas](../features/templates.md), [Hosts de bases de datos](../features/database-hosts.md), [Configuración del panel](../features/panel-settings.md), [Actualizaciones del panel](../features/panel-updates.md), [Aprovisionamiento y SSO](../features/provisioning-and-sso.md) y [Asistente de IA](../features/ai-assistant.md).
