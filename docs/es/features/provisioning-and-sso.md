# Integración con facturación: API de aprovisionamiento y enlaces de acceso

Aether Panel ofrece una API reducida para sistemas de facturación como WHMCS o Paymenter. Con ella, el sistema de facturación puede crear un servidor cuando el cliente paga, suspenderlo si una factura vence, cambiar su plan, eliminarlo al terminar el servicio y enviar al cliente al panel con la sesión ya iniciada. Esta página explica las piezas que configura el administrador y cómo se comportan; los campos de petición y respuesta de cada ruta están en la [referencia de la API: aprovisionamiento y facturación](../reference/api/07-provisioning.md).

## Qué hace

- **Claves API**: autentican al sistema de facturación. Cada clave puede limitarse a las acciones que necesite la integración.
- **Productos**: describen un plan: qué plantilla instalar, en qué nodo, con cuánta CPU, memoria y disco y, opcionalmente, de qué rango de puertos.
- **Rutas de aprovisionamiento** bajo `/api/v1`: crear, dar de baja, suspender, reactivar y cambiar de plan.
- **Enlaces de acceso (SSO)**: el sistema de facturación pide un enlace de un solo uso que inicia la sesión del cliente sin contraseña.

Este repositorio no incluye ningún módulo de facturación; la parte del sistema de facturación (módulo de WHMCS, extensión de Paymenter o código propio) debe llamar a estas rutas.

## Cómo funciona

### Claves API

- Una clave tiene la forma `ak_xxxxx_…`: el prefijo `ak_` más 5 caracteres hexadecimales se guarda en claro para identificarla; el token completo solo se guarda como hash SHA-256. El token se muestra una única vez, al crearlo.
- El sistema de facturación la envía en `X-Api-Key: ak_…` o en `Authorization: Bearer ak_…`. Todas las rutas `/api/v1` exigen una clave válida, incluida `GET /api/v1/ping`.
- Cada clave guarda una lista de acciones:

| Acción | Permite |
|--------|---------|
| `create` | `POST /api/v1/provision` |
| `terminate` | `POST /api/v1/terminate` |
| `suspend` | `POST /api/v1/suspend` y `POST /api/v1/unsuspend` |
| `change-package` | `POST /api/v1/change-package` |
| `sso` | `POST /api/v1/sso` |
| `provision` | Todas las acciones. La tienen las claves creadas antes de que pudieran limitarse y las creadas mediante la API sin lista. |

Se rechaza una lista que no nombre ninguna acción conocida. Los nombres desconocidos dentro de una lista se descartan.

### Productos

| Campo | Significado |
|-------|-------------|
| `product_id` | Identificador que envía el sistema de facturación (único), p. ej. `minecraft_2gb` |
| `display_name` | Se usa como nombre del servidor cuando la petición no trae `server_name` |
| `template` | Nombre de la plantilla. El panel la busca en todos los repositorios de plantillas y usa la primera coincidencia |
| `cpu` | Límite de CPU en % (100 = un núcleo). `0` conserva el valor de la plantilla |
| `memory`, `disk` | Límites en MB. `0` conserva el valor de la plantilla |
| `default_node` | Nodo en el que se crea el servidor (`0` es el nodo local) |
| `node_id` | Opcional, solo por API: si tiene valor distinto de `0`, prevalece sobre `default_node` |
| `port_range_min`, `port_range_max` | Rango opcional. `0` en cualquiera de los dos campos equivale a todo el rango 1024–65535 |

### Creación de un servidor (`POST /api/v1/provision`)

1. **Idempotencia**: si se envía `service_id` y ya hay un servidor con ese valor, el panel responde con ese servidor en lugar de crear otro (la contraseña vuelve vacía).
2. Se busca el producto por `product_id`.
3. Se busca al cliente por correo. Si no tiene cuenta, se crea con el `username` y la `password` indicados, o con valores generados (`user_` más hasta 8 caracteres aleatorios y una contraseña de 16 caracteres). Las cuentas nuevas reciben el permiso de inicio de sesión y, si existe un rol llamado `Usuario`, ese rol. **Una cuenta existente conserva su contraseña**; en ese caso la respuesta devuelve `password` vacío.
4. Se resuelven la plantilla y el nodo, y se elige al azar un puerto libre del rango del producto (se excluyen los puertos que usa cualquier servidor de ese nodo, incluidos los puertos adicionales).
5. Se crea el registro del servidor con el cliente como **propietario**, el identificador del servicio guardado como identificador externo y los permisos estándar de propietario concedidos.
6. El nodo crea el servidor con el puerto asignado y los límites del producto inyectados en las variables de plantilla `port`, `cpu`, `memory` y `disk`. Si el nodo lo rechaza, el registro se elimina para que un reintento empiece de cero.
7. Tras el éxito: se inicia la primera instalación si `panel.servers.autoInstall` está activo, se programa un subdominio automático si los subdominios automáticos están configurados y activados, se envía el correo de «añadido al servidor» y se publica el evento de extensión `server.created`.

La respuesta contiene `success`, `server_id`, `username`, `password` y `port`.

### Suspender, reactivar, dar de baja y cambiar de plan

Cada acción identifica el servidor con `service_id` (el identificador del sistema de facturación, preferente) o `server_id` (el identificador del panel). Los subservidores creados a partir de un servidor siguen a su servidor padre.

- **Suspender** marca el servidor y sus subservidores como suspendidos (el panel se niega a arrancarlos) y detiene en su nodo los que estén en marcha.
- **Reactivar** quita la marca. **No** arranca los servidores.
- **Dar de baja** elimina primero los subservidores y después el servidor: cada uno se detiene si está en marcha, se borra en su nodo (si el nodo ya no lo conoce se considera borrado), se liberan sus registros de subdominio y se elimina el registro del panel. Se publica el evento de extensión `server.deleted` por cada uno.
- **Cambiar de plan** acepta un `product_id` (se usan sus límites) o valores explícitos de `cpu`, `memory` y `disk`. Primero se actualiza el nodo y solo después el registro del panel. El servidor conserva sus archivos. La respuesta indica `restart_required: true` cuando el servidor está en marcha (o no se puede leer su estado): la nueva CPU y memoria se aplican en el siguiente reinicio; el límite de disco cuenta de inmediato.

### Enlaces de acceso (`POST /api/v1/sso`)

1. El sistema de facturación envía el `email` del cliente y, opcionalmente, `service_id` o `server_id`.
2. El panel rechaza las cuentas de administrador: cualquier cuenta con un permiso global privilegiado (`admin`, `settings.edit` o permisos de gestión de nodos, plantillas, usuarios u OAuth2). Así, una clave de facturación filtrada no puede abrir el panel de administración.
3. Si se indica un servidor, el cliente debe tener `server.view` sobre él; el enlace lleva entonces a la página de ese servidor. Si no, lleva a la lista de servidores.
4. El panel devuelve `url` (`<panel>/auth/sso?token=…`) y `expires_in` (60).
5. El navegador abre el enlace. `GET /auth/sso` consume el token, comprueba que la cuenta sigue pudiendo iniciar sesión, abre una sesión normal (sin pedir el segundo factor) y redirige al destino. El acceso aparece en el registro de accesos como **Acceso desde facturación**.

Los tokens son valores aleatorios de 32 bytes; solo se guarda su hash, **en memoria**. Un token sirve una vez y durante un minuto. Al reiniciar el panel se pierden todos los enlaces pendientes.

La dirección del panel en el enlace es `panel.settings.masterUrl` cuando está definida en el archivo de configuración o en el entorno; si no, la dirección con la que el sistema de facturación llamó al panel (`https` si la petición llegó por TLS o con `X-Forwarded-Proto: https`).

## Uso

### Desde el panel

Las dos pestañas están en **Configuración** y solo se muestran a usuarios con el permiso `admin`.

**Claves API** (**Configuración → Claves API**)

1. Pulsar **Generar clave** y darle un **Nombre** (por ejemplo, «WHMCS servidor 1»).
2. En **Qué puede hacer**, marcar solo las acciones que necesite la integración: **Crear servidores**, **Dar de baja**, **Suspender y reactivar**, **Cambiar de plan**, **Acceso directo**.
3. Pulsar **Generar** y copiar la clave del diálogo **Clave generada**. No se vuelve a mostrar.
4. Para inutilizar una clave, usar **Revocar clave**. Las integraciones que la usen dejan de autenticarse de inmediato.

**Productos** (**Configuración → Productos**)

1. Pulsar **Añadir producto**.
2. Rellenar **ID del producto** (lo que enviará el sistema de facturación), **Nombre visible**, **Plantilla**, **Límite de CPU** (%), **Límite de memoria** (MB), **Límite de disco** (MB) y **Nodo por defecto**.
3. Opcionalmente, activar **Asignar puerto de un rango** e indicar **Puerto mínimo** y **Puerto máximo**.
4. Pulsar **Guardar producto**. Eliminar un producto solo impide nuevos pedidos; los servidores existentes no se ven afectados.

El formulario no muestra `node_id`; si hace falta, se define mediante la API.

### Mediante la API

| Método | Ruta | Autenticación / permiso | Uso |
|--------|------|-------------------------|-----|
| `GET` | `/api/settings/apikeys` | sesión, `admin` | Listar claves (nombre, prefijo, permisos, fecha de creación) |
| `POST` | `/api/settings/apikeys` | sesión, `admin` | Crear una clave: `{ "name": "...", "permissions": ["create", ...] }`. Devuelve `token` una sola vez |
| `DELETE` | `/api/settings/apikeys/:id` | sesión, `admin` | Revocar una clave |
| `GET`, `POST` | `/api/provision/products` | sesión, `admin` | Listar / crear productos |
| `PUT`, `DELETE` | `/api/provision/products/:id` | sesión, `admin` | Modificar / eliminar un producto (`:id` es el identificador numérico, no `product_id`) |
| `GET` | `/api/v1/ping` | cualquier clave válida | Prueba de conexión: `{ "success": true, "message": "Pong" }` |
| `POST` | `/api/v1/provision` | clave con `create` | Crear un servidor a partir de un producto |
| `POST` | `/api/v1/suspend` | clave con `suspend` | Suspender un servidor y sus subservidores |
| `POST` | `/api/v1/unsuspend` | clave con `suspend` | Levantar la suspensión |
| `POST` | `/api/v1/change-package` | clave con `change-package` | Cambiar los límites |
| `POST` | `/api/v1/terminate` | clave con `terminate` | Eliminar un servidor y sus subservidores |
| `POST` | `/api/v1/sso` | clave con `sso` | Obtener un enlace de acceso de un solo uso |
| `GET` | `/auth/sso?token=…` | ninguna (el token) | Lo abre el navegador del cliente |

Los cuerpos de petición, las respuestas y los ejemplos con `curl` están en la [referencia de la API](../reference/api/07-provisioning.md).

## Requisitos y configuración

- El sistema de facturación debe llegar al panel por HTTPS (las claves viajan en cabeceras).
- Al menos un producto cuya plantilla exista en un repositorio de plantillas y cuyo nodo esté accesible.
- Correo configurado si los clientes deben recibir el mensaje de «añadido al servidor».

| Clave | Valor por defecto | Variable de entorno | Efecto aquí |
|-------|-------------------|---------------------|-------------|
| `panel.settings.masterUrl` | `http://localhost:8080` | `SKYPANEL_PANEL_SETTINGS_MASTERURL` | Base del enlace de acceso; solo se usa si se define explícitamente en el archivo de configuración o en el entorno |
| `panel.servers.autoInstall` | `true` | `SKYPANEL_PANEL_SERVERS_AUTOINSTALL` | Ejecuta la instalación de la plantilla justo después de aprovisionar |
| `panel.dns.autoAssign` | `false` | `SKYPANEL_PANEL_DNS_AUTOASSIGN` | Asigna un subdominio automático a los servidores aprovisionados (requiere también los ajustes de Cloudflare) |

Todas las claves están en [Configuración](../backend/02-configuration.md).

## Límites

- Los enlaces de acceso duran 60 segundos, sirven una vez y se pierden al reiniciar el panel.
- Nunca se emiten enlaces de acceso para cuentas de administrador.
- `unsuspend` no arranca los servidores; debe hacerlo el cliente (o una llamada posterior).
- La plantilla de un producto se busca por nombre en el primer repositorio que la tenga; dos repositorios con el mismo nombre de plantilla no se distinguen.
- Si no queda ningún puerto libre en el rango, el servidor se crea con el puerto `0` y la variable de puerto de la plantilla no se modifica. Conviene usar rangos amplios para el número de servidores previsto.
- Si la suspensión se guarda en el panel pero un servidor en marcha no puede detenerse en su nodo, la llamada responde `502` y el servidor queda marcado como suspendido.

## Problemas conocidos y soluciones

| Mensaje | Causa | Solución |
|---------|-------|----------|
| `401 Unauthorized - Missing or invalid API Key format` | Falta la clave o no empieza por `ak_` | Enviarla en `X-Api-Key` o `Authorization: Bearer`. |
| `401 Invalid API Key` | La clave se revocó o está mal copiada | Generar una clave nueva y actualizar la integración. |
| `403` `this API key is not allowed to do that` (`ErrAPIKeyNotAllowed`) | La clave no tiene la acción de esa ruta | Crear una clave con las acciones necesarias (las claves no se pueden editar). |
| `pick at least one action, from: create, terminate, suspend, change-package, sso` | Clave creada con una lista sin ninguna acción conocida | Usar los nombres de acción indicados arriba. |
| `404` con cuerpo vacío al aprovisionar | No hay ningún producto con ese `product_id`, o el nodo del producto no existe | Revisar **Configuración → Productos** y el nodo del producto. |
| `400` `template not found` | La plantilla del producto no está en ningún repositorio | Corregir la **Plantilla** del producto. |
| `Daemon error` con `details` | El nodo rechazó crear el servidor o aplicar los límites | Leer `details`; revisar el nodo y la plantilla. |
| `either service_id or server_id is required` | Suspender, dar de baja, cambiar de plan o SSO sin indicar el servidor | Enviar `service_id` o `server_id`. |
| `404` con cuerpo vacío en una acción o en SSO | Ningún servidor tiene ese `service_id`/`server_id`, o ninguna cuenta tiene ese correo | El servicio no llegó a aprovisionarse o ya se dio de baja; revisar el correo. |
| `give a product_id or at least one of cpu, memory and disk` / `resources cannot be negative` | Cuerpo de cambio de plan no válido | Enviar un producto o límites positivos. |
| `502` `internal error, the details are in the panel log` al dar de baja; el registro del panel indica `the node refused to delete server … (status N)` o `the node could not delete server …` | El nodo está caído o falló | El registro se conserva; reintentar cuando vuelva el nodo. |
| `502` `internal error, the details are in the panel log` al suspender; el registro del panel indica `server … is suspended, but its node could not be reached to stop it` (o `could not stop it`, `refused to stop it`) | Suspendido en el panel pero sigue en marcha en el nodo | Detenerlo cuando el nodo esté accesible o repetir la llamada. |
| `403` `sign-in links are not issued for administrator accounts` | El correo pertenece a una cuenta privilegiada | Usar una cuenta de cliente; los administradores entran con su contraseña. |
| `403` `no permission to perform action` en SSO | El cliente no puede ver el servidor indicado | Revisar el propietario y los permisos de subusuario del servidor. |
| El cliente llega a la página de inicio de sesión con «El enlace de acceso ya se usó o ha caducado.» | Enlace usado dos veces, abierto pasados 60 s o panel reiniciado | Pedir un enlace nuevo cada vez que se pulse el botón. |
| «Esta cuenta no tiene permiso para entrar al panel.» | La cuenta perdió el permiso `login` | Restaurar el permiso de inicio de sesión o el rol del usuario. |
| El enlace apunta a `localhost` o a un host incorrecto | `panel.settings.masterUrl` no está definido y el sistema de facturación llama al panel por una dirección interna | Definir `panel.settings.masterUrl` con la URL pública del panel. |

## Véase también

- [Referencia de la API: aprovisionamiento y facturación](../reference/api/07-provisioning.md): detalle completo de `/api/v1` y de los productos.
- [Extensiones](../reference/15-extensions.md): eventos `server.created` y `server.deleted`.
- [Referencia de permisos](../general/scopes-reference.md): qué incluyen los permisos de propietario.
- [Configuración](../backend/02-configuration.md).
