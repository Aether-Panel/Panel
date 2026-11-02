# Uso del panel

Un recorrido por la interfaz web: para qué sirve cada parte del menú, quién la ve y dónde está su guía detallada. Las ideas de fondo (nodos, propietarios, plantillas…) están en [Conceptos básicos](./concepts.md); cada funcionalidad en profundidad, en las [guías de funcionalidades](../features/README.md).

## Estructura

- **Barra lateral**: el menú. Se puede contraer a una columna de iconos y en el móvil es un menú desplegable. Cada entrada solo aparece a quien tiene el permiso que necesita.
- **Búsqueda rápida**: `Ctrl+K` (`⌘K` en macOS) para saltar a una página o a un servidor.
- **Menú de la cuenta**: **Configuración de la cuenta** y cerrar sesión.

| Entrada del menú | Se muestra con | Para qué sirve |
|------------------|----------------|----------------|
| **Inicio** | Todos | Panel de inicio: una vista general de la instalación para los operadores, la lista de sus servidores para los demás. |
| **Servidores** | `login` | Los servidores de los que se es propietario, miembro o administrador. |
| **Usuarios** | `users.info.search` | Cuentas, su rol y sus permisos. |
| **Roles** | `admin` | Roles y los scopes del panel que dan. |
| **Nodos** | `nodes.view` | Las máquinas que ejecutan servidores. |
| **Hosts de BD** | `admin` | Servidores MySQL/MariaDB para las bases de datos de los servidores. |
| **Plantillas** | `templates.view` | Repositorios de plantillas y plantillas locales. |
| **Configuración** | `settings.edit` | Ajustes generales del panel. |

Las extensiones instaladas pueden añadir sus propias páginas al menú y pestañas a cada servidor ([Extensiones](../reference/15-extensions.md)).

## Inicio

Los usuarios con `nodes.view` o `users.info.search` ven una vista general: servidores activos, salud general (disponibilidad de 30 días), tráfico de red en directo, avisos de poco disco, CPU, memoria y almacenamiento medios, información del panel, el historial de red de los nodos (1 h, 6 h, 24 h) y los últimos inicios de sesión. Los demás ven **Mis servidores**, una tarjeta por cada servidor que pueden abrir. Ver [Panel de inicio y nodos](../features/dashboard-and-nodes.md).

## Servidores

La lista de servidores a los que se tiene acceso. Los administradores crean servidores con **Añadir servidor** (el asistente se describe paso a paso en [Primeros pasos](./getting-started.md#6-crear-el-primer-servidor)).

### Página del servidor

Las pestañas dependen de los permisos que se tengan en ese servidor (las reglas exactas están en la [Referencia de scopes](./scopes-reference.md#qué-ve-cada-uno-en-la-web)):

| Pestaña | Qué se hace ahí | Guía |
|---------|-----------------|------|
| **Consola** | Salida en directo, comandos (con `server.console.send`), iniciar / reiniciar / detener / forzar. Se puede ampliar sobre la página; una caída muestra un aviso y un informe. | [Informes de caída](../features/crash-reports.md), [Asistente de IA](../features/ai-assistant.md) |
| **Resumen** | Dirección, versión, jugadores y gráficas de CPU, memoria y red. | [Resumen y estadísticas](../features/server-statistics.md), [Versión del juego](../features/game-versions.md) |
| **Archivos** | Navegar, editar, subir, comprimir y extraer archivos; historial de archivos. | [Historial de archivos](../features/file-history.md) |
| **Configuración** | Nombre, variables que la plantilla abre al usuario, inicio y reinicio automático, notas de puertos y puerto principal, notificaciones, subdominio. | [Configuración del servidor](../features/server-settings.md), [Notificaciones](../features/server-notifications.md), [Subdominios](../features/subdomains.md) |
| **Propiedades** (beta) | Editor de `server.properties` para Minecraft: Java Edition. | [Propiedades del servidor](../features/server-properties.md) |
| **Usuarios** | El propietario y los subusuarios, con sus permisos. | [Referencia de scopes](./scopes-reference.md#propietario-y-subusuarios) |
| **Base de datos** | Bases de datos en los hosts que puede usar este servidor. | [Hosts de bases de datos](../features/database-hosts.md) |
| **Copias de seguridad** | Crear, descargar, restaurar y borrar copias. | [Copias de seguridad](../features/backups.md) |
| **Tareas** | Tareas programadas (comandos, acciones de encendido, copias). | [Tareas programadas](../features/scheduled-tasks.md) |
| **Actividad** | Quién hizo qué en el servidor. | [Registro de actividad](../features/activity-and-access-log.md) |
| **Acceso SFTP** | La dirección y el usuario SFTP (`correo#idServidor`). | [Conceptos básicos: SFTP](./concepts.md#sftp) |
| **Migración** | Mover el servidor a otra instalación de Aether Panel (administradores). | [Transferencias de servidores](../features/server-transfers.md) |
| **Splitter** | Dividir el servidor en sub-servidores (propietario o administrador). | [Subservidores](../features/sub-servers.md) |
| **Plugins**, **Modpacks** | Instalar plugins (de Modrinth) y modpacks (Modrinth, CurseForge). | [Modpacks y plugins](../features/modpacks-and-plugins.md) |
| **Mapa** | Mapa del mundo para Minecraft: Java Edition. | [Mapa del mundo](../features/world-map.md) |
| **Administración** | Reinstalar y, para administradores: transferir a otro nodo, definición, suspensión, clonado y borrado. | [Transferencias de servidores](../features/server-transfers.md), [Clonar servidores](../features/cloning.md) |

## Usuarios y roles

**Usuarios** lista las cuentas con su rol. Con `users.info.edit` se crean, editan y borran cuentas; con `users.perms.edit` se cambian sus permisos del panel. El personal no puede editar cuentas con poderes que él no tiene ([Editar otras cuentas](./scopes-reference.md#editar-otras-cuentas)). Solo los administradores eligen el rol de un usuario.

**Roles** (administradores) agrupa scopes del panel bajo un nombre. El panel crea "Administrador" y "Usuario"; los scopes de servidor de un rol no tienen efecto en los servidores. Ver [Roles](./scopes-reference.md#roles).

El acceso a un servidor se da en el propio servidor, en su pestaña **Usuarios**, no aquí.

## Nodos

**Nodos** lista el nodo local y los nodos remotos con su estado y sus recursos. Los administradores añaden nodos remotos (`nodes.create`), los editan y ven sus credenciales de despliegue (`nodes.deploy`). Ver [Conceptos básicos: Nodos](./concepts.md#nodos) y [Panel de inicio y nodos](../features/dashboard-and-nodes.md).

## Hosts de bases de datos

**Hosts de BD** (administradores) guarda los servidores MySQL/MariaDB donde se crean las bases de datos de los servidores. **Añadir host** abre un asistente de tres pasos:

1. **Credenciales**: el usuario y la contraseña de una cuenta de MySQL que pueda crear usuarios y bases de datos.
2. **Instrucciones**: el SQL para crear esa cuenta, con botones para copiarlo:

   ```sql
   CREATE USER 'aetheruser'@'%' IDENTIFIED BY 'your_strong_password';
   GRANT ALL PRIVILEGES ON *.* TO 'aetheruser'@'%' WITH GRANT OPTION;
   ```

   El usuario se crea para `'%'` porque un panel en Docker se conecta desde la IP de su contenedor. Si la base de datos está en otra máquina, también explica cómo permitir conexiones externas (`bind-address=0.0.0.0` bajo `[mysqld]` en `my.cnf`, reiniciar MySQL y abrir el puerto 3306 al panel).
3. **Configuración**: **Nombre del host** (una etiqueta), **Host de conexión** (IP o dominio al que se conecta el panel), **Puerto** (3306), **Límite de bases de datos** (vacío o 0 para ilimitado) y **Nodos vinculados** (opcional). **Probar conexión** comprueba host, usuario y contraseña antes de guardar.

Un host vinculado a un nodo solo se ofrece a los servidores de ese nodo. Los hosts se pueden editar después (dejando usuario y contraseña vacíos para conservarlos) y borrar, pero solo cuando ya no tienen bases de datos.

Después, en la pestaña **Base de datos** de un servidor, quien tiene `server.data.edit` crea una base de datos: el panel la llama `s_<idServidor>_<nombre>`, crea un usuario con nombre y contraseña aleatorios solo para ella y muestra los datos de conexión. Los servidores de juego llegan a la MariaDB del propio panel como `mysql:3306`. Todos los detalles: [Hosts de bases de datos](../features/database-hosts.md).

## Plantillas

**Plantillas** muestra los repositorios (**Comunidad** y **Local**) y sus plantillas. **Usar plantilla** abre el asistente de nuevo servidor con esa plantilla. Con `templates.local.edit` se suben, crean, editan y borran plantillas locales. Ver [Plantillas](../features/templates.md).

## Configuración del panel

**Configuración** (`settings.edit`) tiene estas pestañas: **General** (nombre de la empresa, URL principal, IP del nodo maestro, clave de Gemini, clave de CurseForge, registro, comprobación anti-bots y Turnstile, funciones experimentales), **Notificaciones Discord**, **Correo**, **Claves API** (facturación), **Productos** (planes para la facturación), **Actualizaciones**, **Copias externas** (S3), **Dominios** (subdominios de Cloudflare) y **Extensiones**. En el resto de la documentación esta página se llama **Configuración**. Ver [Configuración del panel](../features/panel-settings.md).

## Configuración de la cuenta

La página propia de cada usuario: datos de la cuenta (nombre de usuario, correo), contraseña, autenticación de dos pasos, claves SSH para SFTP, clientes OAuth2 (con `self.clients`) y preferencias (idioma: inglés o español). Ver [Seguridad](./security.md#inicio-de-sesión-y-sesiones).
