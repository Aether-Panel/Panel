# Aprovisionamiento / facturación

Rutas para sistemas de facturación (WHMCS, Paymenter...): crear, suspender, cambiar de plan y borrar servidores, y enviar al cliente al panel sin contraseña. Los productos, los planes a partir de los cuales se crean esos servidores, se gestionan con la API del panel. Volver a la [referencia de la API](../11-api-reference.md). Cómo encajan las piezas desde el punto de vista del administrador: [Integración con facturación](../../features/provisioning-and-sso.md).

Código: `internal/web/api/provision.go`, `internal/web/api/products.go`.

## Rutas

| Método | Ruta | Autenticación | Qué hace |
|--------|------|---------------|----------|
| GET | `/api/provision/products` | `admin` | Lista los productos |
| POST | `/api/provision/products` | `admin` | Crea un producto |
| PUT | `/api/provision/products/:id` | `admin` | Actualiza un producto |
| DELETE | `/api/provision/products/:id` | `admin` | Borra un producto |
| GET | `/api/v1/ping` | Clave de API | Comprueba la clave y la conexión |
| POST | `/api/v1/provision` | Clave de API, `create` | Crea un servidor para un cliente |
| POST | `/api/v1/terminate` | Clave de API, `terminate` | Borra un servidor y sus subservidores |
| POST | `/api/v1/suspend` | Clave de API, `suspend` | Suspende y detiene |
| POST | `/api/v1/unsuspend` | Clave de API, `suspend` | Levanta una suspensión |
| POST | `/api/v1/change-package` | Clave de API, `change-package` | Cambia los límites del plan |
| POST | `/api/v1/sso` | Clave de API, `sso` | Enlace de acceso de un solo uso para un cliente |

## Productos

Un producto es un plan: qué plantilla, en qué nodo, con qué límites y qué puertos.

```json
{
  "id": 1,
  "product_id": "minecraft_2gb",
  "display_name": "Minecraft 2GB",
  "template": "minecraft-java",
  "node_id": null,
  "cpu": 100,
  "memory": 2048,
  "disk": 10240,
  "default_node": 1,
  "port_range_min": 25565,
  "port_range_max": 25665
}
```

| Campo | Significado |
|-------|-------------|
| `product_id` | Identificador de texto único que envía el sistema de facturación (p. ej. `minecraft_2gb`) |
| `display_name` | Nombre por defecto del servidor |
| `template` | Nombre de la plantilla; se usa el primer repositorio que la tenga |
| `node_id` | Nodo en el que se crean los servidores; si es `null` o `0`, se usa `default_node` |
| `default_node` | Id del nodo de respaldo; no elige el nodo según la carga |
| `cpu`, `memory`, `disk` | Límites: % de un núcleo (100 = un núcleo), MB, MB. `0` conserva el valor de la plantilla |
| `port_range_min`, `port_range_max` | Rango del que se elige el puerto del servidor; `0` en cualquiera de los dos equivale a 1024–65535 |

`GET` los lista; `POST` recibe el objeto sin `id` y lo devuelve con su `id`; `PUT /:id` cambia los campos enviados y devuelve el producto (`404` si no existe); `DELETE /:id` responde `204`. `POST` y `PUT` rechazan un producto que el aprovisionamiento no podría usar: `400` `ErrProductTemplateRequired` sin plantilla, `400` `ErrProductTemplateNotFound` si ningún repositorio tiene una plantilla con ese nombre (buscada igual que al aprovisionar), `502` `ErrProductTemplateUnreadable` si no se pudo leer un repositorio que podría tenerla (por ejemplo, el índice de `templates.url` no responde) y `400` `ErrProductNodeNotFound` si no existe el nodo que usaría el producto (`node_id` o, si no, `default_node`). `PUT` solo comprueba lo que cambia: la plantilla si es otra y el nodo si el producto usaría otro, así que un índice caído no impide cambiar los límites. Mientras un producto los use, su nodo y su plantilla no se pueden borrar: borrar el nodo responde `409` `ErrNodeHasProducts`, la plantilla local `409` `ErrTemplateInUse` (salvo que otro repositorio tenga una plantilla con ese nombre) y un repositorio Git que es el único con la plantilla de un producto `409` `ErrTemplateRepoInUse`; cada uno lista los `product_id` de los productos en `metadata.products`.

## Autenticación

Las claves se crean en **Configuración → Claves API** o con [`POST /api/settings/apikeys`](./06-panel-admin.md#claves-de-api) (requiere `admin`). Una clave empieza por `ak_` y se muestra una sola vez. Se envía en una de estas cabeceras:

```
X-Api-Key: ak_YOUR_KEY
Authorization: Bearer ak_YOUR_KEY
```

| Respuesta | Cuándo |
|-----------|--------|
| `401 {"error": "Unauthorized - Missing or invalid API Key format"}` | No hay clave, o no empieza por `ak_` |
| `401 {"error": "Invalid API Key"}` | Clave desconocida |
| `403 ErrAPIKeyNotAllowed` | Clave válida sin el permiso de esa ruta |

Cada clave indica qué puede hacer:

| Permiso | Rutas |
|---------|-------|
| `create` | `POST /api/v1/provision` |
| `terminate` | `POST /api/v1/terminate` |
| `suspend` | `POST /api/v1/suspend` y `POST /api/v1/unsuspend` |
| `change-package` | `POST /api/v1/change-package` |
| `sso` | `POST /api/v1/sso` |
| `provision` | Todas. La tienen las claves creadas antes de que pudieran limitarse y las claves nuevas creadas sin lista |

`GET /api/v1/ping` no requiere ningún permiso y responde `{"success": true, "message": "Pong"}`.

Estas peticiones no llevan cookies, por lo que la [protección de mismo origen](./02-authentication.md#protección-de-mismo-origen) no se aplica.

## Identificar el servicio

Las rutas que actúan sobre un servidor reciben uno de estos campos:

| Campo | Significado |
|-------|-------------|
| `service_id` | El id del servicio en el sistema de facturación, guardado al crear el servidor con `/provision`. Tiene prioridad si se envían los dos |
| `server_id` | El identificador del servidor en el panel |

Sin ninguno de los dos: `400 ErrServerRequired`. Si el servidor no existe: `404`.

## `POST /api/v1/provision`

Permiso `create`. Crea un servidor con los límites de un producto.

| Campo | Obligatorio | Significado |
|-------|-------------|-------------|
| `product_id` | sí | El `product_id` del producto (texto) |
| `email` | sí | El correo del cliente |
| `service_id` | no | El id del servicio en el sistema de facturación; hace que la llamada sea idempotente |
| `server_name` | no | Por defecto, el `display_name` del producto |
| `username` | no | Para una cuenta nueva; por defecto, `user_` más hasta 8 caracteres aleatorios |
| `password` | no | Para una cuenta nueva; por defecto, 16 caracteres aleatorios |

Pasos:

1. Si se envía `service_id` y un servidor ya lo tiene, se devuelve ese servidor y no se crea nada (`password` vuelve vacío).
2. Busca el producto (`404` si no existe).
3. Busca la cuenta por correo. Si no existe, la crea con el rol `Usuario` (si ese rol existe) y el permiso `login`. Una cuenta existente conserva su contraseña y `password` vuelve vacío.
4. Busca la plantilla del producto en los repositorios: `400` con el código `ErrGeneric` y el mensaje `template not found` si ninguno la tiene. Aun así, la cuenta del paso 3 se mantiene.
5. Elige el nodo (`node_id`, si no `default_node`; `404` si no existe) y un puerto libre dentro del rango del producto.
6. Crea el servidor, propiedad del cliente, con los permisos estándar de miembro.
7. Envía la definición al nodo con `cpu`, `memory` y `disk` del producto (y el puerto) como variables, de modo que el contenedor arranca con los límites del plan. `disk` es el límite real de disco.
8. Si el nodo falla, el servidor se quita del panel para que la llamada pueda reintentarse: `502` si el nodo no responde, o el estado que devuelva el nodo con `{"error": "Daemon error", "details": "..."}`.
9. Inicia la instalación (con `panel.servers.autoInstall`), asigna un subdominio si está activado y envía un correo al cliente.

Respuesta `200`:

```json
{ "success": true, "server_id": "3F9A1C2B7D4E", "username": "user_x7k2p9", "password": "generated_password", "port": 25570 }
```

| Campo | Significado |
|-------|-------------|
| `username` | El nombre de usuario de la cuenta; en una llamada repetida, el `username` enviado (puede estar vacío) |
| `password` | Solo para una cuenta nueva, y solo la primera vez |
| `port` | El puerto asignado; `0` si el rango no tenía ninguno libre |

```bash
curl -X POST https://panel.example.com/api/v1/provision \
  -H "X-Api-Key: ak_YOUR_KEY" -H "Content-Type: application/json" \
  -d '{ "product_id": "minecraft_2gb", "email": "customer@example.com", "service_id": "42", "server_name": "Survival" }'
```

## `POST /api/v1/terminate`

Permiso `terminate`. Cuerpo `{"service_id": "42"}` o `{"server_id": "3F9A1C2B7D4E"}`.

Detiene y borra el servidor y sus subservidores en el nodo y en el panel, y elimina sus registros de subdominio. Un servidor que el nodo ya no conoce cuenta como borrado. Si un nodo no borra alguno, la respuesta es `502` y el servidor se queda en el panel, para que la llamada pueda reintentarse. Respuesta: `{"success": true}`.

## `POST /api/v1/suspend` y `POST /api/v1/unsuspend`

Permiso `suspend`. El mismo cuerpo que terminate.

- `suspend` marca como suspendidos el servidor y sus subservidores, y detiene los que estén en ejecución. Si no se puede contactar con un nodo o este se niega, la respuesta es `502`: el servidor queda suspendido en el panel, pero puede seguir en ejecución.
- `unsuspend` levanta la suspensión del servidor y de sus subservidores. No los arranca.

Respuesta: `{"success": true}`.

## `POST /api/v1/change-package`

Permiso `change-package`. Pasa un servidor a otros límites sin tocar sus archivos.

| Campo | Significado |
|-------|-------------|
| `service_id` / `server_id` | El servidor |
| `product_id` | Producto de destino; si se envía, se usan sus `cpu`, `memory` y `disk` y se ignoran los campos siguientes (`404` si no existe) |
| `cpu`, `memory`, `disk` | % de un núcleo, MB, MB |

Un campo a `0` o ausente deja ese límite como está. Los valores negativos, o no indicar ningún límite, responden `400`. Primero recibe los límites el nodo; si los rechaza, se devuelve su estado con `{"error": "Daemon error", "details": "..."}` y el panel no cambia nada.

```json
{ "success": true, "server_id": "3F9A1C2B7D4E", "restart_required": true }
```

El límite de disco se aplica de inmediato; la CPU y la memoria, en el siguiente arranque. `restart_required` es `true` cuando el servidor está en ejecución o el nodo no ha podido indicarlo.

```bash
curl -X POST https://panel.example.com/api/v1/change-package \
  -H "X-Api-Key: ak_YOUR_KEY" -H "Content-Type: application/json" \
  -d '{ "service_id": "42", "product_id": "minecraft_4gb" }'
```

## `POST /api/v1/sso`

Permiso `sso`. Para el botón «ir al panel» del sistema de facturación: un enlace que abre la sesión del cliente sin contraseña ni 2FA.

| Campo | Obligatorio | Significado |
|-------|-------------|-------------|
| `email` | sí | El correo de la cuenta |
| `service_id` / `server_id` | no | El enlace lleva a ese servidor (el cliente necesita `server.view` en él); sin ellos, a la lista de servidores |

```json
{ "success": true, "url": "https://panel.example.com/auth/sso?token=ONE_TIME_TOKEN", "expires_in": 60 }
```

| Respuesta | Cuándo |
|-----------|--------|
| `404` | No hay ninguna cuenta con ese correo, o el servidor no existe |
| `403 ErrSSONotForAdmins` | La cuenta tiene un poder global sobre el panel: `admin`, `settings.edit`, crear, editar, borrar o desplegar nodos, editar plantillas o repositorios, editar usuarios o sus permisos, u `oauth2.auth` (propio o de su rol). Así, una clave filtrada no puede abrir la sesión de un administrador |
| `403 ErrNoPermission` | El cliente no puede ver el servidor solicitado |

El enlace funciona una sola vez (el primer uso lo consume, aunque después falle el inicio de sesión), caduca a los 60 segundos y solo se guarda en memoria como hash, de modo que reiniciar el panel anula los enlaces pendientes. Su dirección es `panel.settings.masterUrl` cuando está definida y, si no, la dirección a la que llamó el sistema de facturación. El sistema de facturación debe enviar el navegador del cliente a `url` de inmediato y no guardarla. Qué ocurre al abrirlo: [`GET /auth/sso`](./02-authentication.md#get-authssotoken).
