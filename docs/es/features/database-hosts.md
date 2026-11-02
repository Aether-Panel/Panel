# Hosts de bases de datos y bases de datos de servidor

Los hosts de bases de datos permiten que Aether Panel cree bajo demanda bases de datos MySQL/MariaDB para los servidores de juego. Un administrador registra una vez un servidor de bases de datos (un **host de base de datos**); a partir de ahí, quien gestione un servidor puede crear y borrar bases de datos desde la pestaña **Base de datos** del servidor, y el panel crea automáticamente el esquema, un usuario propio y su contraseña.

En [Uso del panel](../general/usages.md) hay una introducción breve; esta página entra en los detalles.

## Qué hace

- Registra uno o varios servidores MySQL/MariaDB como hosts de bases de datos, cada uno opcionalmente vinculado a un nodo y con un límite de bases de datos opcional.
- Prueba la conexión con un host antes de guardarlo.
- Crea una base de datos por petición, con un usuario propio que solo tiene privilegios sobre ella.
- Lista las bases de datos de un servidor con host, puerto, usuario y contraseña (la contraseña solo para quien puede gestionarlas), y las borra (base de datos y usuario) cuando se pide.
- Elimina todas las bases de datos de un servidor cuando se borra el servidor.

## Cómo funciona

### Hosts de bases de datos

Un host guarda un nombre, el host de conexión (IP, FQDN o nombre de host), el puerto (3306 por defecto), un usuario y una contraseña administrativos, un **límite de bases de datos** (`max_databases`, 0 = ilimitado) y un **nodo** opcional (`node_id`). La contraseña se guarda en la base de datos del panel y la API nunca la devuelve.

Solo se admiten MySQL y MariaDB: el panel se conecta con el controlador MySQL de Go (`go-sql-driver/mysql`) con tiempos de espera de 5 segundos para conectar, leer y escribir.

### Creación de una base de datos

Cuando un servidor pide una base de datos llamada `<nombre>` en un host, el panel:

1. Comprueba que el host acepta el servidor:
   - Si el host tiene un nodo asignado, el servidor debe estar en ese nodo (si no, `ErrDatabaseHostOtherNode`).
   - Si el host tiene límite, el número de bases de datos que aloja debe estar por debajo (si no, `ErrDatabaseHostFull`).
2. Construye el nombre real `s_<serverId>_<nombre>`. Ambas partes solo pueden contener letras, dígitos, `_` y `-`, y el nombre completo no puede superar 64 caracteres (el límite de MySQL). Con los identificadores de servidor de 12 caracteres que genera el panel, quedan 49 caracteres para `<nombre>`. En caso contrario: `ErrInvalidDatabaseName`.
3. Rechaza un nombre que ya esté registrado en ese host (`ErrDatabaseNameTaken`).
4. Genera un usuario `db_` + 10 letras minúsculas o dígitos aleatorios y una contraseña aleatoria de 16 caracteres.
5. En el host ejecuta `CREATE DATABASE` (sin `IF NOT EXISTS`, de modo que un esquema existente no registrado provoca un error en lugar de quedar asignado) con `utf8mb4` / `utf8mb4_unicode_ci`, después `CREATE USER '<usuario>'@'%'`, `GRANT ALL PRIVILEGES ON <bd>.* TO '<usuario>'@'%'` y `FLUSH PRIVILEGES`. Si falla algún paso posterior a `CREATE DATABASE`, el usuario y la base de datos se eliminan de nuevo para que se pueda reintentar con el mismo nombre.
6. Guarda el registro, incluido `remote_connection` (`host:puerto` del host de base de datos).

El prefijo separa a los clientes en un host compartido: un servidor nunca puede crear ni borrar la base de datos de otro servidor ni un esquema del sistema.

### Borrado

Al borrar una base de datos se ejecutan en el host `DROP USER IF EXISTS`, `DROP DATABASE IF EXISTS` y `FLUSH PRIVILEGES`, y después se elimina el registro. La API comprueba que la base de datos pertenece al servidor de la URL.

Cuando se borra un **servidor**, el panel elimina antes todas sus bases de datos y usuarios. Se hace en la medida de lo posible: si el host no responde, el servidor se borra igualmente y el panel registra `server <id>: database <name> and its user could not be dropped from host <id>, remove them by hand`.

Un **host de base de datos** solo se puede borrar cuando ya no tiene bases de datos: mientras le quede alguna, el borrado se rechaza con HTTP 409 `ErrDatabaseHostInUse`. Así no quedan bases de datos ni usuarios en el servidor MySQL sin nada en el panel que permita borrarlos. Primero se borran las bases de datos desde sus servidores y después el host.

### Qué hosts ve un servidor

`GET /api/servers/:id/databasehosts` devuelve solo los hosts que el servidor puede usar (que sirven a su nodo o a todos y tienen sitio), como pares `{id, name}`: los propietarios de servidores nunca ven la dirección ni las credenciales de un host.

## Uso

### Desde el panel

**Registrar un host** (barra lateral **Hosts de BD**, requiere `admin`):

1. Pulse **Añadir host**. El asistente tiene tres pasos: **Credenciales**, **Instrucciones** y **Configuración**.
2. **Credenciales**: usuario y contraseña de una cuenta MySQL que pueda crear usuarios y bases de datos.
3. **Instrucciones** muestra el SQL para crear esa cuenta, por ejemplo:

   ```sql
   CREATE USER 'aetheruser'@'%' IDENTIFIED BY 'your_strong_password';
   GRANT ALL PRIVILEGES ON *.* TO 'aetheruser'@'%' WITH GRANT OPTION;
   ```

   y, para un host en otra máquina, añadir `bind-address=0.0.0.0` bajo `[mysqld]` en `my.cnf` y abrir el puerto en el cortafuegos. Cuando el panel corre en Docker se conecta desde la IP de su contenedor, por lo que la cuenta debe crearse para `'%'`.
4. **Configuración**: **Nombre del host** (etiqueta en el panel), **Host de conexión**, **Puerto**, **Límite de bases de datos** (en blanco o 0 = ilimitado) y **Nodos vinculados**.
5. Use **Probar conexión** para comprobar las credenciales y después **Crear host**.

Para editar un host, use **Editar** en el menú de su fila; deje la contraseña vacía para conservar la actual.

**Crear una base de datos para un servidor** (página del servidor → pestaña **Base de datos**, requiere `server.data.edit`):

1. Pulse **Nueva base de datos**, escriba un **Nombre de la base de datos** y elija un **Host de la base de datos**.
2. Pulse **Crear base de datos**. Un diálogo muestra host, puerto, nombre, usuario y contraseña: «Guarda estas credenciales».
3. Cada fila muestra usuario, contraseña (oculta por defecto, con controles para mostrarla y copiarla) y **Host y puerto**. Si la API no envía la contraseña (quien la consulta no tiene `server.data.edit`), en su lugar aparece «Solo visible para quien puede gestionar las bases de datos». El icono de papelera borra la base de datos tras confirmar.

### Mediante la API

| Método | Ruta | Permiso | Propósito |
|--------|------|---------|-----------|
| `GET` | `/api/databasehosts` | `admin` | Listar hosts (sin contraseñas) |
| `POST` | `/api/databasehosts` | `admin` | Crear un host |
| `POST` | `/api/databasehosts/test` | `admin` | Probar una conexión (`host`, `port` 3306 por defecto, `username`, `password`) |
| `GET` / `PUT` / `DELETE` | `/api/databasehosts/:id` | `admin` | Leer, actualizar (un `password` vacío conserva la actual) y borrar (409 `ErrDatabaseHostInUse` si aún tiene bases de datos) |
| `GET` | `/api/servers/:serverId/databasehosts` | `server.data.edit` | Hosts que puede usar este servidor (`id`, `name`) |
| `GET` | `/api/servers/:serverId/databases` | `server.view` | Bases de datos del servidor. El campo `password` solo se incluye si quien llama tiene `server.data.edit` sobre ese servidor; si no, se omite |
| `POST` | `/api/servers/:serverId/databases` | `server.data.edit` | Crear una base de datos: `{"database_host_id": 1, "database_name": "tienda"}` |
| `DELETE` | `/api/servers/:serverId/databases/:id` | `server.data.edit` | Borrar una base de datos y su usuario |

Los errores de validación al crear (`ErrInvalidDatabaseName`, `ErrDatabaseNameTaken`, `ErrDatabaseHostFull`, `ErrDatabaseHostOtherNode`) devuelven HTTP 400; los errores de MySQL, 500. Véanse la [referencia de la API](../reference/11-api-reference.md) y la [referencia de permisos](../general/scopes-reference.md).

## Requisitos y configuración

- Un servidor MySQL o MariaDB accesible y una cuenta con `CREATE`, `CREATE USER`, `DROP`, `DROP USER` y `GRANT OPTION` (el `GRANT ALL ... WITH GRANT OPTION` anterior los incluye).
- El host de conexión debe ser accesible **desde el panel** (que crea las bases de datos) y **desde los servidores de juego** (que usan las credenciales): la dirección guardada como host de conexión es la que se muestra a los usuarios.
- En la instalación con Docker, la MariaDB incluida (`skypanel-mysql`, `mariadb:10.11`) está en la red del panel y en la de los servidores de juego (`skypanel-servers`), por lo que puede registrarse con host de conexión `mysql` y puerto `3306`. No se registra automáticamente, y su puerto en el host (`127.0.0.1:3307`) solo se publica en la interfaz de loopback.
- Ninguna clave de configuración afecta a esta función. La base de datos propia del panel (`panel.database.*`) es independiente; véase [Configuración](../backend/02-configuration.md).

## Límites

- Solo MySQL/MariaDB.
- Nombres de base de datos: letras, dígitos, `_` y `-`; 64 caracteres en total contando el prefijo `s_<serverId>_`.
- Los usuarios se crean para `'%'` (cualquier host); conviene limitar el acceso de red al puerto de la base de datos con un cortafuegos.
- El **Límite de bases de datos** cuenta las bases de datos del host de todos los servidores; no hay límite por servidor.
- `max_connections` existe en el registro de la base de datos, pero el panel no lo define ni lo aplica.
- Las contraseñas de las bases de datos de servidor se guardan en la base de datos del panel. La API solo las devuelve a quien tiene `server.data.edit` sobre el servidor; con solo `server.view` se ven el host, el puerto y el usuario.
- Un host con bases de datos no se puede borrar (`ErrDatabaseHostInUse`).

## Problemas conocidos y soluciones

| Mensaje | Causa | Solución |
|---------|-------|----------|
| `connection failed: …` (Probar conexión) | Host, puerto o credenciales incorrectos, o MySQL rechaza conexiones remotas | Revisar `bind-address`, el cortafuegos y que el usuario exista para `'%'` |
| «El intento de conexión ha superado el tiempo de espera.» | Sin respuesta en 15 s (límite de la interfaz) | Revisar el enrutamiento o el cortafuegos entre el panel y el host |
| `host and username are required` | Se enviaron campos vacíos al endpoint de prueba | Rellenar host y usuario |
| `failed to connect to MySQL host (check credentials and host permissions)` | Las credenciales guardadas ya no funcionan | Editar el host con la contraseña correcta |
| `failed to create database (ensure user has CREATE permissions and the name is free)` | Falta un privilegio o ya existe un esquema con ese nombre en el host | Conceder `CREATE`; borrar o renombrar el esquema sobrante |
| `failed to create MySQL user (ensure user has CREATE USER permissions)` | Falta `CREATE USER` | Concederlo a la cuenta del host |
| `failed to grant privileges (ensure user has GRANT OPTION)` | La cuenta no tiene `WITH GRANT OPTION` | Repetir el `GRANT` con `WITH GRANT OPTION` |
| `database name may only use letters, digits, '_' and '-', within the length limit` | Caracteres no válidos o nombre demasiado largo | Usar un nombre más corto con caracteres permitidos |
| `a database with that name already exists on this host` | El nombre ya está registrado en ese host | Elegir otro nombre |
| `this database host has reached its database limit` | El host ha alcanzado su **Límite de bases de datos** | Subir el límite o usar otro host |
| `this database host serves another node` | El host está vinculado a un nodo distinto del del servidor | Usar un host de ese nodo o uno sin nodo |
| `database does not belong to this server` (403) | El ID de la base de datos pertenece a otro servidor | Usar el ID de servidor correcto |
| No aparece la pestaña **Base de datos** | El usuario no tiene `server.data.edit` sobre ese servidor | Concederlo (los propietarios lo tienen por defecto) |
| `this database host still has databases, delete them first` (409) | Se intentó borrar un host que aún tiene bases de datos | Borrar sus bases de datos desde la pestaña **Base de datos** de cada servidor y repetir |
| La lista de hosts del diálogo de creación está vacía | Ningún host sirve al nodo del servidor o todos están llenos | Añadir un host, vincularlo al nodo o subir el límite |

## Véase también

- [Uso del panel: hosts de bases de datos](../general/usages.md)
- [Conceptos básicos](../general/concepts.md)
- [Referencia de permisos](../general/scopes-reference.md)
- [Configuración](../backend/02-configuration.md)
- [Referencia de la API](../reference/11-api-reference.md)
