# Uso del Panel

## Navegación por la Interfaz

La interfaz del panel está dividida en varias secciones principales, accesibles desde la barra lateral. El dashboard proporciona una vista general, mientras que otras secciones te permiten gestionar recursos específicos como servidores, usuarios y nodos. El panel se compone de dos partes: un backend en Go que sirve la API REST y el frontend web, y un frontend moderno construido con Astro y React.

### Secciones Principales

#### Dashboard

Vista general con estadísticas, servidores activos y estado del sistema con gráficas de uso de recursos.

#### Servidores

Gestiona todos tus servidores de juegos desde aquí: consola en tiempo real, archivos, backups, bases de datos, plugins y más.

#### Usuarios

Administra usuarios, permisos granulares por scopes y roles del panel.

#### Nodos

Configura y gestiona los nodos físicos o virtuales donde se ejecutan los servidores.

#### Administración

Configuración avanzada del panel: Database Hosts, plantillas, roles globales y más.

## Arquitectura del Panel

Aether Panel es un panel open source construido con Go 1.25. El backend expone una API RESTful usando Gin (framework web) y GORM (ORM). El panel puede ejecutarse en dos modos: panel (interfaz web y API) y daemon (ejecución de servidores). En instalaciones estándar, ambos modos se ejecutan en un solo binario.

### Stack Tecnológico

#### Backend (Go)

- Go 1.25 con Gin framework para la API REST
- GORM para acceso a base de datos (SQLite por defecto, PostgreSQL/MySQL soportados)
- CLI con Cobra para comandos de administración
- Autenticación mediante tokens Bearer, sesiones y cookies
- OAuth2 server integrado para autenticación de terceros
- SFTP server integrado para transferencia de archivos
- WebSocket para consola y estadísticas en tiempo real
- Docker para entornos de servidores aislados

#### Frontend (Dashboard)

- Astro + React 19 con TypeScript
- Tailwind CSS 3.4 para estilos
- shadcn/ui + Radix UI para componentes de interfaz
- Panel de control completo con diseño responsive
- Editor de código integrado con resaltado de sintaxis
- Gráficas en tiempo real con Recharts
- Consola web interactiva con WebSocket

## Gestión de Servidores

Para añadir un nuevo servidor, navega a la sección "Servidores" y haz clic en "Crear Nuevo". Se te pedirá que selecciones un nodo, elijas una plantilla y configures sus ajustes. Los servidores se ejecutan en contenedores Docker para aislamiento y seguridad.

### Crear un Servidor

- Haz clic en el botón "Crear Servidor"
- Selecciona el nodo donde se ejecutará el servidor
- Elige una plantilla (Minecraft, Terraria, etc.)
- Configura el nombre, puerto, IP y recursos
- Asigna usuarios al servidor con sus permisos
- Haz clic en "Crear" — el panel instalará el servidor automáticamente

### Ciclo de Vida del Servidor

El panel expone endpoints REST para controlar completamente el ciclo de vida del servidor. Cada acción requiere autenticación y permisos específicos.

- Iniciar (start) — enciende el servidor
- Detener (stop) — apaga el servidor gracefulmente
- Reiniciar (restart) — detiene e inicia nuevamente
- Matar (kill) — forzar detención inmediata
- Instalar/Reinstalar (install) — ejecuta el proceso de instalación de la plantilla
- Recargar (reload) — recarga la configuración del servidor

### Pestañas del Servidor

#### Consola

Accede a la consola del servidor en tiempo real mediante WebSocket. Puedes enviar comandos y ver los logs en vivo con auto-scroll. La consola incluye timestamps y modo de pantalla completa.

#### Archivos

Gestor de archivos completo con editor de código integrado. Puedes navegar, editar, renombrar, crear, eliminar, comprimir y extraer archivos. El editor incluye resaltado de sintaxis y múltiples atajos de teclado.

#### Bases de Datos

Crea y gestiona bases de datos MySQL/MariaDB para tu servidor. Las credenciales se generan automáticamente y la información de conexión se muestra con opción de copiado rápido.

#### SFTP

Acceso SFTP integrado para transferir archivos usando clientes como FileZilla o WinSCP. Muestra el servidor, puerto, usuario y contraseña de conexión.

#### Backups

Crea, descarga, restaura y elimina backups de tus servidores. Cada backup se almacena comprimido en el nodo. Puedes nombrar cada backup para identificarlo fácilmente.

#### Plugins

Busca, instala y gestiona plugins desde el panel. Compatible con servidores Minecraft (Spigot/Paper). Busca por nombre en los repositorios públicos de plugins.

#### Usuarios del Servidor

Gestiona qué usuarios tienen acceso al servidor y con qué permisos. Puedes añadir o eliminar usuarios y ajustar scopes individualmente.

#### Configuración

Variables de entorno, flags de inicio, tipo de servidor, nombre y configuración avanzada del servidor.

#### Estadísticas

Gráficas en tiempo real de CPU, RAM, disco y red del servidor. Los datos se actualizan mediante WebSocket para una experiencia fluida.

### Transferencia entre Nodos

Puedes transferir servidores entre nodos del panel. La transferencia incluye todos los archivos y la configuración. También existe la opción de transferencia externa federada entre paneles independientes.

## Gestión de Usuarios y Roles

En la sección "Usuarios", puedes crear nuevas cuentas de usuario y asignarlas a roles con permisos específicos. El sistema de permisos está basado en scopes que controlan granularmente cada acción dentro del panel.

### Crear Usuario

- Haz clic en "Crear Usuario"
- Completa el formulario con nombre de usuario, email y contraseña
- Asigna permisos globales o roles
- Guarda el usuario

### Sistema de Permisos (Scopes)

Aether Panel usa 74 scopes granulares (50 específicos de servidor + 24 globales) que controlan cada acción. Los permisos pueden asignarse a nivel global (afectan a todos los servidores) o a nivel de servidor individual. También existen roles que agrupan scopes para asignación masiva.

#### Categorías de Scopes

- **server**: Control del servidor: view, start, stop, kill, restart, install, reload, status, stats, console, send commands, rename
- **files**: Archivos: view, edit, sftp, compress, extract
- **backups**: Backups: view, create, restore, delete, download
- **users**: Usuarios del servidor: view, create, edit, delete
- **data**: Datos del servidor: view, edit (usuario), edit (admin)
- **tasks**: Tareas programadas: view, run, create, edit, delete
- **admin**: Scopes de administración global: admin, settings.edit, nodes (view/create/edit/delete/deploy), users (search/view/edit), templates, uptime
## Gestión de Nodos

Los nodos son servidores físicos o virtuales donde se ejecutan los contenedores Docker de tus servidores de juegos. El panel puede gestionar múltiples nodos desde una sola interfaz. Cada nodo ejecuta un daemon que se comunica con el panel vía API HTTP.

### Crear un Nodo

- Ve a la sección "Nodos" en el panel de administración
- Haz clic en "Crear Nodo"
- Configura el nombre, IP, puerto y carpetas de servidores
- Configura las credenciales de comunicación (secreto compartido)
- Verifica la conexión con el nuevo nodo
- Guarda el nodo — ahora puedes crear servidores en él

### Nodo Local

El nodo local es el mismo servidor donde está instalado el panel. Se configura automáticamente durante la instalación y ejecuta el daemon en el mismo proceso.

### Características del Nodo

Cada nodo expone sus capacidades al panel: tipo de entorno (Docker), sistema de archivos, información del sistema (CPU, RAM, disco). Puedes consultar las features de cualquier nodo desde la interfaz.

## Database Hosts

Los Database Hosts permiten gestionar servidores MySQL/MariaDB para crear bases de datos automáticamente para tus servidores de juegos.

### Crear un Database Host

- Ve a Admin  Database Hosts
- Haz clic en "Crear Database Host"
- Paso 1: Ingresa las credenciales de MySQL con permisos GRANT ALL
- Paso 2: Configura el host, puerto, nombre y nodos vinculados
- Guarda el Database Host

### Configuración de MySQL

Para que el panel pueda crear bases de datos automáticamente, necesitas:

- Un usuario MySQL con permisos GRANT ALL PRIVILEGES ON *.* WITH GRANT OPTION
- MySQL configurado para aceptar conexiones externas (bind-address=0.0.0.0)
- Puerto 3306 abierto en el firewall

### Usar Database Hosts

Una vez creado un Database Host, puedes crear bases de datos desde la pestaña "Database" del servidor. El panel generará automáticamente:

- Nombre de base de datos único
- Usuario con permisos específicos
- Contraseña segura
- Información de conexión remota

## SFTP

Aether Panel incluye un servidor SFTP integrado que permite a los usuarios acceder a los archivos de sus servidores usando cualquier cliente SFTP estándar (FileZilla, WinSCP, Cyberduck).

- El servidor SFTP se ejecuta en el puerto configurado (por defecto 5657)
- Cada usuario solo puede acceder a los servidores donde tiene permisos
- La autenticación se realiza mediante las credenciales del panel
- Los usuarios deben tener el scope server.sftp para acceder vía SFTP

## Conexión WebSocket en Tiempo Real

El panel proporciona una conexión WebSocket a través de la API REST que permite recibir datos en tiempo real del servidor. El socket se conecta mediante proxy desde el panel al daemon del nodo.

- Consola interactiva del servidor con logs en vivo
- Estadísticas de CPU, RAM y disco en tiempo real
- Estado del servidor (encendido/apagado/instalando)
- Autenticación mediante token Bearer del panel

## Interfaz de Línea de Comandos (CLI)

El panel incluye un CLI completo basado en Cobra para administración desde la terminal.

- SkyPanel run — Inicia el panel y el daemon
- SkyPanel version — Muestra la versión del panel
- SkyPanel user — Comandos de gestión de usuarios (crear, modificar, eliminar)
- SkyPanel db — Comandos de base de datos (migrar, actualizar)
- SkyPanel runservice — Ejecuta como servicio del sistema

