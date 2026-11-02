# Cuentas y usuarios

La cuenta propia (`/api/self`), las preferencias de cada usuario (`/api/userSettings`), la administración de usuarios (`/api/users`), los roles (`/api/roles`) y el registro de accesos (`/api/activity`). Volver a la [referencia de la API](../11-api-reference.md).

## Rutas

| Método | Ruta | Permiso | Qué hace |
|--------|------|---------|----------|
| GET | `/api/self` | `login` | La cuenta propia |
| PUT | `/api/self` | `self.edit` | Cambia el nombre de usuario, el correo o la contraseña |
| GET | `/api/self/otp` | `self.edit` | Indica si la 2FA está activada |
| POST | `/api/self/otp` | `self.edit` | Inicia la activación de la 2FA |
| PUT | `/api/self/otp` | `self.edit` | Confirma la activación y devuelve los códigos de recuperación |
| POST | `/api/self/otp/recovery` | `self.edit` | Genera nuevos códigos de recuperación |
| DELETE | `/api/self/otp/:token` | `self.edit` | Desactiva la 2FA |
| GET | `/api/self/oauth2` | `self.clients` | Los clientes OAuth2 propios |
| POST | `/api/self/oauth2` | `self.clients` | Crea un cliente OAuth2 |
| DELETE | `/api/self/oauth2/:clientID` | `self.clients` | Borra uno |
| GET | `/api/self/sshkeys` | `self.edit` | Las claves públicas SFTP propias |
| POST | `/api/self/sshkeys` | `self.edit` | Añade una clave |
| DELETE | `/api/self/sshkeys/:id` | `self.edit` | Quita una clave |
| GET | `/api/userSettings` | `login` | Las preferencias propias |
| PUT | `/api/userSettings/:key` | `login` | Guarda una preferencia |
| GET | `/api/users` | `users.info.search` | Busca usuarios (paginado) |
| POST | `/api/users` | `users.info.edit` | Crea un usuario |
| GET | `/api/users/:id` | `users.info.view` | Un usuario |
| POST | `/api/users/:id` | `users.info.edit` | Actualiza un usuario |
| DELETE | `/api/users/:id` | `users.info.edit` | Borra un usuario |
| GET | `/api/users/:id/perms` | `users.perms.view` | Los permisos globales del usuario |
| PUT | `/api/users/:id/perms` | `users.perms.edit` | Los establece |
| GET | `/api/roles` | uno de `admin`, `users.info.view`, `users.info.edit` | Lista los roles |
| POST | `/api/roles` | `admin` | Crea un rol |
| GET | `/api/roles/:id` | `admin` | Un rol |
| POST | `/api/roles/:id` | `admin` | Actualiza un rol |
| DELETE | `/api/roles/:id` | `admin` | Borra un rol |
| GET | `/api/activity` | `users.info.search` | Registro de accesos: inicios de sesión y altas de cuentas (paginado) |

## El objeto usuario

```json
{
  "id": 3,
  "username": "alice",
  "email": "alice@example.com",
  "otpActive": false,
  "roleId": 2,
  "roleName": "Usuario",
  "scopes": ["login", "server.create", "templates.view"]
}
```

`scopes` son los permisos globales que tiene la cuenta: los de su propia fila de permisos más los de su rol. `roleName` solo se rellena cuando el rol se ha cargado junto con el usuario.

## Cuenta propia

### `GET /api/self`

El objeto usuario de la cuenta con la que se llama.

### `PUT /api/self`

```json
{ "username": "alice", "email": "alice@example.com", "password": "current password", "newPassword": "optional new one" }
```

`password` (la contraseña actual) es siempre obligatoria. Si `username` o `email` llegan vacíos, se conservan los valores actuales. Cuando cambia el correo, se avisa a la dirección anterior; cuando se envía `newPassword` (8 bytes o más), se cierran todas las demás sesiones de la cuenta y se envía un correo de confirmación. Responde `204`.

| Error | Cuándo |
|-------|--------|
| `400 ErrFieldRequired` | Falta `password` |
| `400` | Nombre de usuario o correo no válidos |
| `400 ErrPasswordRequirements` | `newPassword` demasiado corta o demasiado larga |
| `500 ErrInvalidCredentials` | Contraseña actual incorrecta (el código responde `500` en este caso) |

### Autenticación en dos pasos

| Ruta | Cuerpo | Respuesta |
|------|--------|-----------|
| `GET /api/self/otp` | | `{"otpEnabled": false}` |
| `POST /api/self/otp` | | `{"secret": "BASE32...", "img": "data:image/png;base64,..."}`: el secreto y su código QR; `400 ErrOtpAlreadyEnabled` si la 2FA ya está activada |
| `PUT /api/self/otp` | `{"token": "123456"}` | `{"recoveryCodes": ["...", "..."]}`: la 2FA queda activada; `400 ErrInvalidCredentials` si el código es incorrecto |
| `POST /api/self/otp/recovery` | `{"token": "123456"}` | Nuevos `recoveryCodes`; los anteriores dejan de funcionar |
| `DELETE /api/self/otp/:token` | | `204`; `:token` es un código actual o un código de recuperación |

Cada cambio envía un correo a la cuenta. Los códigos de recuperación también sirven en `POST /auth/otp`, una sola vez cada uno.

### Clientes OAuth2

`POST /api/self/oauth2`:

```json
{ "name": "Discord bot", "description": "Restarts the server on command" }
```

```json
{ "client_id": "0f8fad5b-d9cb-469f-a165-70867728950e", "client_secret": "36 random characters", "name": "Discord bot", "description": "Restarts the server on command" }
```

El secreto solo aparece en esta respuesta. El cliente actúa con todos los permisos de la cuenta (ver [Credenciales de cliente OAuth2](./02-authentication.md#credenciales-de-cliente-oauth2)); la API no admite para él una lista de permisos ni un servidor concreto. `GET /api/self/oauth2` lista los clientes propios (con `client_secret` vacío). `DELETE /api/self/oauth2/:clientID` borra uno y cierra las sesiones que abrió; responde `404` si el cliente no es de la cuenta. Crear y borrar envían un correo.

### Claves SSH

Claves públicas con las que se puede entrar por SFTP en lugar de con la contraseña; el usuario SFTP sigue siendo `email#serverId`.

`POST /api/self/sshkeys`:

```json
{ "name": "laptop", "publicKey": "ssh-ed25519 AAAAC3Nza... me@laptop" }
```

```json
{ "id": 7, "name": "laptop", "fingerprint": "SHA256:...", "type": "ssh-ed25519", "createdAt": "2026-11-01T09:00:00Z" }
```

`GET /api/self/sshkeys` las lista; `DELETE /api/self/sshkeys/:id` quita una (`204`).

| Error (`400`) | Cuándo |
|---------------|--------|
| `ErrSSHKeyInvalid` | No es una única línea de clave pública válida |
| `ErrSSHKeyIsPrivate` | Se ha pegado una clave privada |
| `ErrSSHKeyWeak` | RSA de menos de 2048 bits |
| `ErrSSHKeyExists` | La clave ya está en la cuenta |
| `ErrSSHKeyLimit` | La cuenta ya tiene 25 claves |
| `ErrSSHKeyName` | Falta el nombre o supera los 100 caracteres |

## Preferencias de usuario

Preferencias libres del usuario con sesión iniciada, como el tema. `GET /api/userSettings` responde `[{"key": "theme", "value": "dark"}]`. `PUT /api/userSettings/:key` recibe `{"value": "dark"}` (una cadena) y responde `204`.

## Usuarios

### `GET /api/users`

| Parámetro | Valor por defecto |
|-----------|-------------------|
| `username` | `*` (comodín) |
| `email` | `*` |
| `page`, `limit` | `1`, `20` (máximo 100) |

```json
{ "users": [ { "id": 3, "username": "alice", "...": "..." } ], "paging": { "page": 1, "pageSize": 20, "maxSize": 100, "total": 1 } }
```

### `POST /api/users`

```json
{ "username": "bob", "email": "bob@example.com", "password": "...", "roleId": 2 }
```

`username` (de 1 a 100 caracteres imprimibles), `email` y `password` son obligatorios; `roleId` solo se aplica cuando quien llama es administrador. Responde con el nuevo objeto usuario y anota `register` en el registro de accesos. Los errores de validación responden `400 {"error": "..."}`. Aquí no se aplica la regla de robustez de contraseña del registro.

### `GET /api/users/:id`

El objeto usuario; `404` si no existe.

### `POST /api/users/:id`

El mismo cuerpo que en la creación, con todos los campos opcionales; los campos vacíos conservan su valor. Cambiar `password` cierra todas las sesiones del usuario. Cuando quien llama es administrador, el rol pasa a ser el `roleId` enviado: si se omite, se quita el rol. Responde `204`.

Quien no tiene `admin` no puede editar ni borrar una cuenta que tenga un poder global del que carece (`admin`, `settings.edit`, los permisos de gestión de nodos, plantillas y usuarios, `oauth2.auth` o un `server.admin` global): responde `403`.

### `DELETE /api/users/:id`

Responde `204`; se aplica la misma restricción. Con la cuenta se eliminan sus filas de permisos, clientes OAuth2, sesiones, códigos de recuperación, restablecimientos de contraseña, claves SSH y preferencias de usuario, y sus servidores se quedan sin propietario.

### `GET /api/users/:id/perms`

La fila de permisos globales propia del usuario (sin incluir el rol): `{"scopes": ["login", "server.create"]}`.

### `PUT /api/users/:id/perms`

`{"scopes": ["login", "server.create"]}`. Un administrador establece la lista tal como se envía. Cualquier otro usuario solo puede añadir o quitar permisos globales que él mismo tenga (nunca permisos de servidor, que se aplicarían en todos los servidores) y no puede editar una cuenta con más poder. Responde `204`.

## Roles

Un rol es una lista de permisos con nombre que se asigna a los usuarios mediante `roleId`. Sus permisos de servidor no se aplican en los servidores (ver la [referencia de permisos](../../general/scopes-reference.md)).

```json
{ "id": 2, "name": "Usuario", "description": "Default customer role", "scopes": ["login", "templates.view", "uptime.view"], "createdAt": "...", "updatedAt": "..." }
```

- `POST /api/roles` recibe `name` (de 1 a 100 caracteres imprimibles, único), `description` y `scopes`; responde con el rol.
- `POST /api/roles/:id` sustituye esos campos; responde con el rol.
- `DELETE /api/roles/:id` desvincula el rol de sus usuarios y lo borra; responde `204`. Los roles llamados `admin`, `Administrador` y `Usuario` no se pueden borrar (`400`); `404` si no existe.

## Registro de accesos

### `GET /api/activity`

Inicios de sesión y altas de cuentas, de más reciente a más antiguo. Parámetros: `event`, `page` (por defecto 1), `limit` (por defecto 10, máximo 100).

```json
{
  "activity": [ { "id": 51, "event": "login", "username": "alice", "email": "alice@example.com", "ip": "203.0.113.7", "userAgent": "Firefox on Linux", "createdAt": "2026-11-01T08:59:00Z" } ],
  "paging": { "page": 1, "pageSize": 10, "maxSize": 100, "total": 51 }
}
```

Eventos registrados: `login`, `login_failed` (solo de cuentas existentes), `login_sso` y `register`. `userAgent` se reduce al navegador y la plataforma, con la forma `Firefox on Linux`. Más detalles: [Registro de actividad y registros de acceso](../../features/activity-and-access-log.md).
