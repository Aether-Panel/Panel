# Referencia detallada del frontend

Referencia archivo por archivo de `client/frontend/src`. Complementa, sin repetirlos, los demás documentos del frontend:

- Shells, componentes compartidos, contextos, toasts y captcha: [03. Componentes e interfaz](./03-components.md).
- Cliente HTTP, hooks de datos y endpoints: [02. Consumo de la API](./02-endpoints.md).
- Rutas, redirecciones y qué scopes muestran cada página o pestaña: [06. Rutas y páginas](./06-pages.md).
- Traducciones: [05](./05-translations.md). IA: [04](./04-ai.md) y [08](./08-ai-detailed.md).
- Qué hace cada función para los usuarios: las [guías de funciones](../features/README.md) (enumeradas al final de [06](./06-pages.md#guías-de-usuario)).

## Page wrappers (`components/page-wrappers/`)

Un archivo por ruta. Cada uno ocupa unas pocas líneas: elige el shell y la página del feature y, en las páginas de detalle, lee `id` de la query string.

```tsx
// components/page-wrappers/ServerDetail.tsx
export default function ServerDetail({ id }: Readonly<{ id?: string }>) {
    const searchParams = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
    const effectiveId = id || searchParams?.get('id') || '';
    return <AppShell currentPath={`/servers/${effectiveId}`}><ServerDetailPage params={{ id: effectiveId }} /></AppShell>;
}
```

Archivos: `Dashboard`, `Servers`, `ServerDetail`, `Nodes`, `NodeDetail`, `Users`, `Roles`, `Templates`, `DatabaseHosts`, `Settings`, `ProfileSettings`, `Extension` (todos con `AppShell`), y `Login`, `Register`, `ForgotPassword`, `ResetPassword` (con `AuthShell`).

## Acceso (`features/auth/`)

| Archivo | Contenido |
|---|---|
| `auth-card.tsx` | `AuthCard`: inicio de sesión y registro en una tarjeta deslizante. Mantiene `mode` sincronizado con la URL (`pushState`, `popstate`), monta el formulario de registro solo cuando hace falta y vuelve al inicio de sesión si el registro está cerrado. En escritorio, el `BrandPanel` (logotipo, emblema 3D e invitación al otro formulario) se desliza sobre la mitad inactiva; los formularios ocultos son `inert`. |
| `login/page.tsx` | `LoginPanel`: formulario de correo y contraseña (react-hook-form + zod), captcha, avisos para `?expired=1` y `?sso=expired\|denied` y, después, el paso de 2FA (6 casillas para el código de la aplicación, o un código de recuperación). La exportación por defecto renderiza `<AuthCard initial="login" />`. |
| `register/page.tsx` | `RegisterPanel`: nombre de usuario, correo, contraseña con medidor de robustez y captcha. La exportación por defecto renderiza `<AuthCard initial="register" />`. |
| `forgot-password/page.tsx` | Correo + captcha → `POST /auth/forgot-password`, dentro de `AuthFrame`. |
| `reset-password/page.tsx` | Nueva contraseña (con medidor de robustez) para el `?token=` del enlace del correo → `POST /auth/reset-password`. |

## Panel principal (`features/dashboard/`)

| Archivo | Contenido |
|---|---|
| `page.tsx` | Elige el panel de operador (con `nodes.view` o `users.info.search`) o el de usuario (sus tarjetas de servidor). Calcula las medias de CPU, memoria y almacenamiento a partir del `systemInfo` de los nodos (el almacenamiento usa el disco donde viven los servidores, `serversDisk`) o de los servidores cuando los nodos no son visibles. Muestra un esqueleto con la forma de la página mientras carga. |
| `status-strip.tsx` | Saludo, disponibilidad de 30 días (de `/api/uptime`), servidores en línea / total, tráfico en directo. |
| `kpi-card.tsx` | Tarjeta KPI con medidor; `usageTone()` es verde por debajo del 60 %, ámbar hasta el 80 % y rojo por encima. |
| `panel-info-card.tsx` | Versión del panel, servidores, usuarios, nodos y si el registro está abierto. |
| `network-history-chart.tsx` | Historial de red de todos los nodos (ver abajo). |
| `access-log.tsx` | Inicios de sesión, intentos fallidos y registros recientes de `useAccessLog` (`GET /api/activity`), con tiempos relativos en el idioma activo. |

El aviso de disco bajo procede de `features/nodes/node-disk-alert.tsx`: una línea por cada nodo cuyo `systemInfo.serversDisk.low` marca el propio nodo (menos del 10 % o de 5 GB libres en el disco de los servidores), con enlace al nodo.

### Historial de red

`NetworkHistoryChart` usa `useNetworkHistory(range)` (`hooks/use-network-history.ts`):

- `GET /api/nodes/network-history?range=1h|6h|24h` (requiere `nodes.view`). El panel pide a todos los nodos `/daemon/network/history` en paralelo (10 s de tiempo máximo cada uno) y suma sus puntos paso a paso. Cada nodo mide el tráfico de su equipo y guarda un punto por minuto durante 24 h, así que el gráfico incluye el tráfico de antes de que nadie abriera el panel principal.
- Respuesta: `{ step, points: [{ t, rx, tx }], nodes, unavailable }`. `t` está en segundos epoch; `rx`/`tx` son KB/s promediados en el paso; el paso es de 1 min, 5 min o 15 min según el rango. `unavailable` cuenta los nodos que no respondieron (caídos o demasiado antiguos para guardar historial); el pie del gráfico lo muestra.
- El hook se refresca cada 60 s con la pestaña visible, conserva los últimos datos buenos si hay un error y no muestra nada del rango anterior mientras carga uno nuevo. También elimina el antiguo búfer del navegador (clave `aether.dashboard.network-history.v1` de `localStorage`) que se usaba antes de que los nodos guardaran el historial.
- La leyenda muestra la tasa en directo de `useGlobalNetworkMetrics` (diferencias entre lecturas consecutivas de los contadores acumulados de cada nodo, tratando una lectura menor como un reinicio del contador) y, si no la hay, el último punto del historial.
- Ayudas de ejes: `timeTicks()` coloca las etiquetas en horas redondas (cada 15 min, 1 h o 4 h) y `rateTicks()` elige pasos redondos en una sola unidad (B/s, KB/s, MB/s, GB/s).

## Servidores (`features/servers/`)

| Archivo | Contenido |
|---|---|
| `page.tsx` | Lista de servidores: búsqueda (`/` la enfoca), filtro de estado, tarjetas con estadísticas en directo de `useServers`. Los administradores tienen el diálogo de creación; `?create=true` lo abre. |
| `create-server-stepper.tsx` | Asistente de creación en tres pasos (entorno, plantilla, configuración) descrito en [06](./06-pages.md#servidores-servers): las variables de la plantilla rellenan los campos de recursos, los puertos se comprueban contra el nodo mientras se escriben, las versiones de Java se limitan a las que tienen imagen `eclipse-temurin` y los errores llevan el foco al campo. Crea con `PUT /api/servers/:id`. |

## Detalle del servidor (`features/servers/[id]/`)

`page.tsx` carga el servidor, sus permisos en ese servidor (`GET /api/servers/:id?perms`) y su detalle, es dueño del WebSocket de la consola ([02](./02-endpoints.md#websocket-de-la-consola)) y de las acciones de encendido, y renderiza la cabecera y las pestañas. Cada pestaña va envuelta en su propio `ErrorBoundary`, de modo que una pestaña con fallos no deja la página en blanco. Los lectores de pantalla oyen los cambios de estado y las caídas mediante una región en vivo discreta, no cada línea del registro.

| Pestaña / pieza | Archivos | Qué hace |
|---|---|---|
| Consola | `console-view.tsx`, `ai-summary.tsx`, `crash-report.tsx` | Ver [Consola](#consola-console-viewtsx). |
| Resumen | `page.tsx` (`ServerOverviewCards`), `performance-history.tsx`, `peak-hours.tsx`, `metrics-charts.tsx`, `network-usage-chart.tsx`, `crash-report.tsx` | Tarjetas de recursos, historial, horas punta, gráficos en directo e historial de caídas. Ver [Gráficos](#gráficos). |
| Archivos | `file-manager-view.tsx`, `code-editor.tsx`, `file-history-sheet.tsx`, `file-diff-view.tsx`, `file-diff-editor.tsx` | Ver [Gestor de archivos](#gestor-de-archivos). |
| Configuración | `settings-view.tsx`, `version-card.tsx`, `address-card.tsx`, `server-address.tsx`, `notifications-card.tsx` | Variables, recursos, puertos con notas, arranque automático, interruptor de la pestaña Plugins, metadatos; cambio de versión del juego en un clic (experimental: detiene el servidor, crea un punto de restauración, instala y vuelve a arrancar); subdominio (de 3 a 32 letras minúsculas, cifras y guiones interiores); notificaciones del servidor con vista previa al estilo de Discord. |
| Propiedades | `properties-view.tsx`, `server-properties.ts`, `server-properties-catalog.ts` | Editor en formulario de `server.properties` con un catálogo de claves conocidas y un editor de MOTD (experimental, Minecraft Java). |
| Usuarios | `users-view.tsx` | Subusuarios y sus permisos; el dueño solo puede conceder los scopes que tiene (los `OwnerScopes` del backend). `server.view` siempre se incluye. |
| Base de datos | `database-view.tsx` | Bases de datos en los hosts de bases de datos disponibles para el servidor. |
| Copias de seguridad | `backups-view.tsx`, `offsite-card.tsx` | Listar, crear (con nombre opcional), restaurar, descargar y eliminar; las copias hechas por tareas programadas y las copias periódicas se identifican como tales. `offsite-card.tsx` es la tarjeta **Copia al S3**: intervalo y copias a conservar de la copia periódica del servidor. |
| Tareas | `tasks-view.tsx`, `task-dialog.tsx`, `task-schedule.ts` | Tareas programadas: editor de horario y lista ordenada de pasos (comando `stdin`, `sleep`, `restart`, `stop`, `start`, `backup`, este con su frecuencia de subida al S3), ejecutar ahora. |
| Actividad | `activity-view.tsx` | Registro de actividad paginado (`GET /api/servers/:id/activity?limit=&before=&type=`), filtrable por familia. |
| Acceso SFTP | `sftp-view.tsx` | Datos de conexión con botones de copiar. |
| Migración | `external-transfer-view.tsx` | Traer un servidor desde otro panel o llevarlo a otro (crear, descargar, estado, cancelar). El estado de la importación llega como códigos (`STEP_*`, `ERROR_*`, `DONE`...) que la vista traduce en `servers.extransfer.progress`, con el `message` en inglés de la API como respaldo. |
| Splitter | `splitter-view.tsx` | Repartir la CPU, la memoria y el disco del servidor en subservidores, con una franja que muestra lo usado y lo libre. |
| Plugins | `plugins-view.tsx` | Buscar e instalar plugins o mods, quitar los instalados. El texto del buscador depende de la plataforma (plugins en Paper, mods en Fabric, Forge y NeoForge); `ErrPluginNoCompatibleVersion` y `ErrPluginBadFileName` se muestran con su propio texto. |
| Modpacks | `modpacks-view.tsx`, `modpack-install-dialog.tsx`, `modpack-status.tsx`, `modpack-shared.tsx` | Modpacks en un clic desde Modrinth o CurseForge; el diálogo de instalación explica cada paso y deja "mundo nuevo" tras un interruptor apagado por defecto; el progreso se consulta periódicamente. |
| Mapa | `world-map-view.tsx`, `world-map-choice.tsx`, `world-map-frame.tsx`, `world-map-types.ts` | Instalar BlueMap o Dynmap (BlueMap pide aceptar la descarga de Mojang) y mostrar el mapa. |
| Administración | `admin-view.tsx`, `clone-server-card.tsx` | Estado de instalación y reinstalación, transferencia a otro nodo, editar la definición, asignaciones (dueño), suspender, eliminar y clonar (`serverClone` experimental; requiere `server.create` o `admin`). |
| Cabecera | `server-tab-nav.tsx`, `assistant-panel.tsx`, `server-address.tsx` | Barra de pestañas de escritorio con menú "Más" y etiquetas Beta; el asistente de IA; la dirección de conexión (sigue los cambios de subdominio mediante un evento de ventana). |

### Consola (`console-view.tsx`)

La página es dueña del socket y pasa `logs`, `status` y `canSend` (falso para un subusuario que puede leer pero no escribir). La vista:

- Ocupa el espacio hasta el margen inferior de la página (nunca menos de 420 px) y sigue la salida nueva solo mientras está abajo del todo; al subir se pausa y ofrece un botón para volver.
- **Modo ampliado**: el botón de ampliar abre la consola sobre toda la página (en un portal sobre `<body>` con un fondo atenuado); **Esc**, el fondo o el mismo botón la devuelven a su sitio. Mientras tanto, su hueco en la página conserva la altura.
- **Filtros**: todo / errores / avisos, con recuento (líneas que contienen `ERROR`, `FATAL`, `EXCEPTION` o `WARN`).
- **Búsqueda**: **Ctrl/⌘+F** dentro de la consola abre un cuadro de búsqueda (las coincidencias ignoran los códigos ANSI), con anterior/siguiente y la coincidencia actual desplazada a la vista.
- **Historial de comandos**: ↑/↓ recorren los últimos 50 comandos y Tab completa desde el historial; **favoritos** (hasta 12). Ambos se guardan por servidor en `localStorage` (`consoleHistory_<id>`, `consoleFavorites_<id>`).
- Los comandos se envían con `POST /api/servers/:id/console` (cuerpo de texto plano); un fallo muestra un toast y una línea en la consola.
- El color se reserva para avisos, errores y los comandos propios del usuario; los colores ANSI los dibuja `lib/ansi-utils.tsx`. Cuando la mayoría de las líneas ya traen su propia marca de hora, se oculta la columna de hora de recepción.
- Un breve toast de "conectada" aparece cuando la consola reconecta tras un corte (no en la primera conexión).
- Bajo la consola, `ai-summary.tsx` lista las líneas recientes (las últimas 50) que parecen problemas y pide a la IA que explique las seleccionadas, salvo que `panel.settings.hideAiAnalysis` esté activo (ver [04](./04-ai.md)).

### Caídas (`crash-report.tsx`)

- `useCrashReports(serverId)` lee `GET /api/servers/:id/crashes`. Un informe de caída tiene `id`, `time`, `exitCode`, `lines` (la consola antes de la caída), `restarted`, `attempt` y `limit`.
- Cuando el socket envía `{ type: 'crash' }`, la página muestra un toast de error (título con el código de salida y descripción de `crashSummary()`: lo que suele significar el código de salida para 1, 134, 137, 139 y 143 y si el servidor se reinició, con intento / límite). El evento del socket solo trae `{id, exitCode, restarted}`, así que en un reinicio automático la página lee `GET /api/servers/:id/crashes` y toma el intento y el límite del informe con ese `id`; si no puede, la descripción es solo «Reiniciado». Además emite el evento de ventana `aether:server-crash` para que el aviso y el historial se refresquen.
- `CrashBanner` muestra la caída más reciente sobre la consola durante 24 horas o hasta que se descarta. `CrashHistory` lista las caídas recientes en Resumen. `CrashReportDialog` muestra el informe completo, resalta las líneas que parecen errores y ofrece "Explicar con IA".

### Gestor de archivos

- `file-manager-view.tsx`: navegación con migas, subir, descargar, nuevo archivo y carpeta, renombrar/mover, eliminar, extraer archivos comprimidos en la carpeta actual y comprimir un archivo o carpeta ("Comprimir" en el menú de cada fila: pide el nombre, admite `.zip`, `.tar` y `.tar.gz`, añade `.zip` a cualquier otro nombre y no sobrescribe; véase [Archivos y compresión](../general/19-files-compression.md#comprimir-desde-el-gestor-de-archivos)). Una columna de casillas (no en `..`) con «seleccionar todo» para la carpeta actual controla una `SelectionBar` con **Comprimir selección** (una petición de compresión con todas las rutas seleccionadas) y **Eliminar selección** (una confirmación y después un `DELETE` por elemento, que se detiene en el primer fallo); la selección se borra al cambiar de carpeta, después de un borrado múltiple y después de comprimir dos o más elementos ([Seleccionar varios elementos](../general/19-files-compression.md#seleccionar-varios-elementos)).
- `code-editor.tsx`: Monaco, cargado bajo demanda, con tema Tokyo Night, los workers de JSON/CSS/HTML/TypeScript y Ctrl/⌘+S para guardar. `lib/monaco-setup.ts` mantiene los estilos de Monaco colgados de `<html>` para que sobrevivan a los cambios de página de Astro.
- `file-history-sheet.tsx`: versiones anteriores de un archivo de texto que el nodo guarda en cada guardado (hasta 20 por archivo; extensiones de configuración como `properties`, `yml`, `json`, `toml`, `conf`, `cfg`, `ini`, `txt`, `xml`, `js`, `ts`, `lua`, `env`, y archivos sin extensión), con ver, comparar y restaurar. `file-diff-view.tsx` / `file-diff-editor.tsx` muestran la comparación lado a lado, o en una columna en pantallas estrechas, plegando los tramos largos sin cambios.

## Gráficos

| Componente | Datos | Notas |
|---|---|---|
| `features/servers/[id]/metrics-charts.tsx` | Muestras de CPU y memoria en directo que `useServers` reúne en el navegador (cada 5 s) | CPU y memoria son porcentajes de los límites del propio servidor (`maxCpu` de las estadísticas, o los núcleos del nodo). |
| `features/servers/[id]/network-usage-chart.tsx` | Las mismas muestras en directo | Descarga / subida en KB/s, mostradas como B/s…MB/s. |
| `features/servers/[id]/performance-history.tsx` | `GET /api/servers/:id/metrics?range=1h\|24h\|7d` (registrado por el nodo) | Vistas de recursos (CPU y memoria), red (descarga/subida) y jugadores (media y pico); tramos apagados sombreados, huecos donde no se registró nada y caídas marcadas. |
| `features/servers/[id]/peak-hours.tsx` | `GET /api/servers/:id/metrics?range=7d` | Media de jugadores por día de la semana y hora en la zona horaria de quien lo ve; oculto en servidores que nunca han informado de jugadores. |
| `features/dashboard/network-history-chart.tsx` | `GET /api/nodes/network-history` | Ver [Historial de red](#historial-de-red). |
| `components/server-card.tsx` (`Sparkline`) | Muestras en directo | Pequeña línea de tendencia en una escala fija de 0 a 100 para porcentajes. |

Todos toman los colores de `CHART_COLORS` y formatean las horas con `chartLocale()` / `formatClock()` (`lib/chart-theme.ts`).

## Nodos, usuarios, roles, plantillas, hosts de bases de datos

| Carpeta | Contenido |
|---|---|
| `features/nodes/page.tsx` | Lista de nodos con estado (incluido "desplegando"), crear, editar, eliminar e instrucciones de despliegue de un nodo nuevo (directorio, archivo `docker-compose` y `docker compose up -d`, cada uno con botón de copiar). |
| `features/nodes/[id]/page.tsx` | Información del sistema, discos, servidores del nodo, editar (con un aviso para el nodo local), eliminar. |
| `features/nodes/node-disk-alert.tsx` | `NodeDiskAlert`, usado en el panel principal y en las páginas de nodos. |
| `features/users/page.tsx` | Lista de usuarios, crear, editar, permisos. |
| `features/roles/page.tsx` | Roles con sus scopes, agrupados como en el editor de permisos. |
| `features/templates/page.tsx` | Repositorios, plantillas de cada repositorio, editor de plantillas locales y subida. |
| `features/database-hosts/page.tsx` | Hosts de bases de datos con prueba de conexión (`POST /api/databasehosts/test`). |

## Configuración (`features/settings/`)

| Archivo | Pestaña |
|---|---|
| `page.tsx` | Contenedor de pestañas. Los secretos llegan de `GET /api/settings` como `********` y se reenvían así si no se tocan, lo que conserva el valor guardado (ver [Configuración del panel](../features/panel-settings.md#cómo-funciona)); Claves API, Productos y Extensiones solo aparecen para `admin`. |
| `general-tab.tsx` | Secciones: Panel (nombre de la empresa, URL base, IP del nodo maestro), Notificaciones (`panel.settings.notificationLanguage`, español o inglés), Inteligencia artificial (clave de API de Gemini, desactivar el análisis de registros con IA para todos), Registro y seguridad (registro, captcha integrado, claves de Turnstile), Modpacks (clave de CurseForge) y Funciones experimentales (`panel.experimental.versionSwitch`, `propertiesEditor`, `serverClone`). |
| `discord-tab.tsx` | Webhooks de Discord con prueba (ver [Discord y notificaciones](../general/16-discord.md)). |
| `mail-tab.tsx` | Proveedor de correo (SMTP, Mailgun, Mailjet, SendGrid o Depuración) y prueba (ver [Correo](../general/15-email.md)). En Mailjet los campos son **Clave API (pública)** (`panel.email.domain`) y **Clave secreta** (`panel.email.key`); Depuración no muestra campos. |
| `offsite-tab.tsx` | Copias externas en almacenamiento compatible con S3 (`daemon.backups.s3.*`) con prueba de conexión y los valores por defecto de la copia periódica. Los valores que comparte con la tarjeta del servidor y el editor de tareas están en `lib/offsite.ts`. |
| `domains-tab.tsx` | Dominio de Cloudflare para los subdominios de los servidores, con estado y prueba. |
| `api-keys-view.tsx` | Claves de API del panel. |
| `products-view.tsx` | Productos de aprovisionamiento (`/api/provision/products`). |
| `extensions-view.tsx` | Instalar (subir), activar, probar y quitar extensiones. |
| `updates-tab.tsx` | Comprobación de versión y actualización del panel (`/api/settings/update-check`, `/update-status`, `/update-panel`). |

## Perfil (`features/profile/settings/`)

`page.tsx` tiene secciones para los datos de la cuenta, la contraseña, la verificación en dos pasos (alta con código QR, códigos de recuperación, desactivar), las claves SSH (`ssh-keys.tsx`: añadir y eliminar las claves públicas con las que se entra por SFTP; los errores como una clave débil, repetida o privada se muestran traducidos), los clientes OAuth2 y el idioma de la interfaz.

## Extensiones (`features/extensions/`)

- `page.tsx`: la página de administración de una extensión en `/extension/?id=<ext>&page=<página>`.
- `extension-frame.tsx`: muestra una página de extensión en un **iframe aislado** (origen opaco: sin cookies, almacenamiento ni API del panel). Tras cargar recibe `{ type: 'aether:context', serverId, language, theme }` por `postMessage` y puede responder `{ type: 'aether:resize', height }` para ajustarse a su contenido.
- Las páginas de servidor de las extensiones aparecen como pestañas adicionales en el detalle del servidor; las de administración, en el grupo Extensiones de la barra lateral.

## Utilidades (`src/lib/`)

| Archivo | Exporta |
|---|---|
| `api-client.ts` | `api`, `ApiError`, `rememberPageForLogin`, `RETURN_TO_KEY` ([02](./02-endpoints.md)). |
| `paged.ts` | `fetchAllPages(url, key)`. |
| `chart-theme.ts` | `CHART_COLORS`, `chartLocale`, `formatClock`. |
| `toast.ts` | Envoltorio `sileo` ([03](./03-components.md#toasts-libtoastts)). |
| `ansi-utils.tsx` | `AnsiText` e interpretación de ANSI para las líneas de la consola. |
| `data.ts` | Tipos compartidos `Server`, `User`, `Node`. |
| `players.ts` | `parsePlayerQuery()` para `/api/servers/:id/query` (respuestas `{ minecraft }` o `{ source }`). |
| `game-logos.ts` | `gameLogo(template)`: logotipo según el campo `icon` de la plantilla, o por nombre en plantillas antiguas. |
| `keyed.ts` | `keyedByContent()`: claves de React estables para listas sin ids. |
| `stale-build.ts` | `isStaleBuildError`, `reloadForNewBuild` (recarga una vez por minuto tras una actualización del panel). |
| `proof-of-work.ts` | Resolución del captcha integrado (esquema ALTCHA, SHA-256 en JavaScript puro). |
| `monaco-setup.ts` | `ensureMonacoStylesheet`, `markMonacoInUse`. |
| `emblem-3d.ts` | `mountEmblem()`: el emblema 3D, construido por partes para que la página siga respondiendo. |
| `utils.ts` | `cn()`, `formatBytes()`. |

## Convenciones

- **Estado**: los datos viven en hooks y en el estado de los componentes; el estado global se limita a los tres contextos. No hay React Query ni Redux.
- **Formularios**: react-hook-form con esquemas de zod; los mensajes de error salen de las traducciones.
- **Textos**: toda cadena visible pasa por `t()` ([05](./05-translations.md)).
- **Iconos**: se importan desde `@/components/icons`, nunca directamente del paquete de iconos.
- **Diseño adaptable**: cada pantalla debe funcionar desde el móvil hasta el escritorio. Los layouts usan los puntos de corte de Tailwind (`sm`, `md`, `lg`, `xl`), y `useIsMobile()` cambia a paneles deslizantes y selectores por debajo de 768 px.
- **Componente nuevo**: las primitivas van en `components/ui/`, las piezas compartidas en `components/` y el código propio de una pantalla en su carpeta de `features/`. Las nuevas pestañas del servidor se envuelven en `ErrorBoundary` y se condicionan con `can()` en `features/servers/[id]/page.tsx`.
