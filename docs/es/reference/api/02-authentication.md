# Autenticación

Cómo demuestra una petición quién la envía, cómo decide el panel qué puede hacer y cuáles son las rutas de inicio de sesión. Volver a la [referencia de la API](../11-api-reference.md). Los formatos de respuesta y de error están en [Convenciones](./01-conventions.md).

## Rutas

| Método | Ruta | Autenticación | Qué hace |
|--------|------|---------------|----------|
| POST | `/auth/login` | Pública | Inicia sesión con correo y contraseña |
| POST | `/auth/otp` | Pública (cookie de inicio de sesión pendiente) | Segundo paso con un código 2FA o de recuperación |
| POST | `/auth/logout` | Pública | Cierra una sesión |
| POST | `/auth/register` | Pública (registros abiertos) | Crea una cuenta: pendiente de aprobación, o con sesión si la aprobación está desactivada |
| POST | `/auth/forgot-password` | Pública | Envía por correo un enlace para restablecer la contraseña |
| POST | `/auth/reset-password` | Pública (token de restablecimiento) | Establece una contraseña nueva |
| POST | `/auth/reauth` | Sesión o bearer | Abre una sesión nueva para el usuario que ya la tiene iniciada |
| GET | `/auth/sso` | Enlace de un solo uso | Abre una sesión desde un enlace del sistema de facturación |
| GET | `/auth/challenge` | Pública | Desafío anti-bots integrado |
| GET | `/auth/publickey` | Pública | Las claves públicas JWT del panel (JWKS) |
| POST | `/oauth2/token` | Credenciales de cliente o secreto del nodo | Emite un token |
| POST | `/auth/sftp-key` | Secreto del nodo | Comprueba un inicio de sesión SFTP con clave SSH para un nodo remoto |
| POST | `/auth/sftp-check` | Secreto del nodo | Si una conexión SFTP abierta de un nodo remoto puede seguir abierta |
| GET | `/api/config` | Pública | Marca, registros, configuración anti-bots y versión |

## Formas de autenticarse

| Método | Cómo se envía | Para qué se usa |
|--------|---------------|-----------------|
| Cookie de sesión | Cookie `skypanel_auth` | El cliente web |
| Token bearer | `Authorization: Bearer <token>` | Scripts e integraciones: un token de sesión o un token de acceso OAuth2 |
| Clave de API | `X-Api-Key: ak_...` o `Authorization: Bearer ak_...` | Solo las rutas de facturación bajo `/api/v1` (véase [Aprovisionamiento](./07-provisioning.md)) |
| JWT del panel | `Authorization: Bearer <jwt>` | Las rutas `/daemon`, del panel al nodo, salvo las públicas `GET`/`HEAD`/`OPTIONS /daemon` y `GET /daemon/features` (véase [Daemon](./09-daemon.md)) |
| Secreto del nodo | `Authorization: Bearer <secreto>` | Del nodo al panel: la concesión de contraseña de OAuth2, `POST /auth/sftp-key` y `POST /auth/sftp-check` (inicios de sesión SFTP) |
| Enlaces firmados | Token en la ruta o en la consulta | `/auth/sso?token=...`, `/api/map-view/...` |
| Firmas Ed25519 | En el cuerpo o en la consulta | `/api/extransfer/*`, de panel a panel, salvo `validate`, que recibe el token de transferencia (véase [Transferencia externa](./08-external-transfer.md)) |

En `/api` y `/auth/reauth`, la cabecera `Authorization` tiene prioridad sobre la cookie. Una cabecera que no sea exactamente `Bearer <token>` responde `401`, y una petición sin cabecera ni cookie también responde `401`.

### Sesiones

Una sesión es un token aleatorio que el panel guarda como hash (SHA-256). Dura **1 hora** y se desliza: cada petición autenticada la renueva cuando queda menos de la mitad de la hora, de modo que solo termina tras una hora sin actividad. La crean `POST /auth/login`, `/auth/otp`, `/auth/register`, `/auth/reauth` y `/auth/sso`. Las sesiones caducadas se rechazan de inmediato y sus filas se borran más tarde: al crear una sesión también se eliminan las caducadas, como mucho una vez por hora.

El token llega al cliente de dos formas:

- la cookie `skypanel_auth`: `Path=/`, `HttpOnly`, `SameSite=Lax`, `Max-Age=3600` (se renueva en cada petición autenticada con la cookie) y `Secure` cuando la petición llegó por HTTPS (directamente o con `X-Forwarded-Proto: https`) o cuando `panel.web.cookies.secure` está activado;
- el campo `session` de la respuesta de inicio de sesión, para usarlo como token bearer.

Una segunda cookie, `session`, la firma el panel y solo contiene el paso de 2FA pendiente (el correo y la hora del paso de la contraseña). Sus atributos salen de `panel.web.cookies.*` (`path`, `domain`, `age`, `secure`, `httpOnly`, `sameSite`, por defecto `Strict`).

En cada petición se comprueba de nuevo que la cuenta tenga el scope `login` (en sus permisos globales del panel o en su rol). Retirarlo cierra las sesiones abiertas: la siguiente petición responde `401`. Cambiar la contraseña desde el perfil cierra todas las demás sesiones de la cuenta; si la cambia un administrador (`POST /api/users/:id`) o se restablece, se cierran todas.

### Credenciales de cliente OAuth2

Para integraciones que actúan en nombre de un usuario:

1. Cree un cliente con `POST /api/self/oauth2` (scope `self.clients`). La respuesta contiene `client_id` y `client_secret`; el secreto solo se muestra en ese momento y se guarda como hash (bcrypt).
2. Cámbielo en `POST /oauth2/token` con `grant_type=client_credentials`.
3. Envíe `Authorization: Bearer <access_token>`.

El token de acceso es una sesión del panel ligada al cliente y a su propietario: dura una hora y se desliza como cualquier otra sesión. Las peticiones hechas con él tienen los permisos del propietario. Un cliente puede limitarse al crearlo a un servidor (`serverId`) y a una lista de permisos de servidor (`scopes`); entonces solo puede lo que la cuenta pueda y además esté dentro de esos límites (ver [Clientes OAuth2](./05-accounts.md#clientes-oauth2)). Sin límites actúa con todo lo que la cuenta puede hacer. Un token de cliente no puede llamar a `/auth/reauth` (`403`). Eliminar el cliente cierra las sesiones que abrió.

## Protección de mismo origen

Todas las rutas del panel (`/api`, `/auth`, `/oauth2`, `/daemon`) pasan por una protección que rechaza las peticiones que cambian el estado iniciadas desde otro sitio con las cookies del panel.

Una petición se rechaza cuando se cumplen **todas** estas condiciones:

1. Lleva una cookie `skypanel_auth` o `session` no vacía.
2. Cambia el estado: cualquier método salvo `GET`, `HEAD` y `OPTIONS`, además de un `GET` que abre un WebSocket (`Upgrade: websocket`).
3. No es del mismo origen:
   - con una cabecera `Sec-Fetch-Site` (la envían todos los navegadores actuales), solo pasan `same-origin` y `none`;
   - sin ella, pasa una petición sin `Origin`; en otro caso, el host de `Origin` debe coincidir con el `Host` de la petición, con su `X-Forwarded-Host` o con el host de `panel.settings.masterUrl`.

La respuesta es `403 {"error": "cross-origin request refused"}`.

Motivo: `SameSite=Lax` compara sitios, y un sitio es el host sin el puerto. La página web de un servidor de juego en `http://IP:25566` es el mismo sitio que un panel en `http://IP:8080`, así que el navegador adjuntaría la cookie del panel a un formulario que esa página enviara o a un WebSocket que abriera para leer una consola. Las peticiones sin las cookies del panel (tokens bearer, claves de API, clientes OAuth2, el daemon) nunca se ven afectadas: los scripts deben enviar un token bearer y ninguna cookie.

## Permisos (scopes)

Cada ruta indica el scope que necesita (aparece en todas las tablas de rutas). La comprobación:

- `admin` cubre todos los scopes; `server.admin` cubre todos los scopes de servidor en ese servidor.
- Un scope de servidor (`server.*`) necesita un `:serverId` en la ruta. Los scopes de quien llama en ese servidor son su fila de permisos en ese servidor, su fila global del panel, el conjunto del propietario si el servidor es suyo y los scopes globales de su rol. Los scopes de servidor de un rol no cuentan en los servidores.
- Una ruta marcada con «cualquiera de» se supera con uno de los scopes indicados.
- Si falta el scope, la respuesta es `403` sin cuerpo.
- Las sesiones abiertas desde un cliente OAuth2 quedan limitadas además por los límites propios del cliente, si los tiene. La lista de scopes que resuelve la ruta se limita igual antes de que la vean los controladores, así que las comprobaciones que hace un controlador por su cuenta (si se muestra la contraseña de una base de datos, si se puede cambiar una variable de entorno bloqueada, qué scopes se pueden conceder, qué scopes puede usar un paso de una tarea) y los flujos que recibe el WebSocket de un servidor nunca van más allá de lo que el cliente puede usar.

Las rutas marcadas como «Sesión iniciada» solo necesitan una sesión válida; `login` significa que la cuenta debe tener el scope `login`, que tienen todas las cuentas que pueden iniciar sesión.

La lista de scopes, qué cubre cada uno y quién los recibe por defecto está en la [Referencia de permisos](../../general/scopes-reference.md).

## Rutas de inicio de sesión

### `POST /auth/login`

```json
{ "email": "admin@example.com", "password": "...", "captchaToken": "..." }
```

| Campo | Notas |
|-------|-------|
| `email`, `password` | Obligatorios |
| `turnstileToken` | Obligatorio cuando `GET /api/config` indica `"captcha": "turnstile"` |
| `captchaToken` | Obligatorio cuando indica `"captcha": "builtin"`: véase [`GET /auth/challenge`](#get-authchallenge) |

Respuesta cuando la cuenta no tiene 2FA (además, establece la cookie `skypanel_auth`):

```json
{
  "scopes": ["login", "server.create", "templates.view"],
  "session": "6f1c0e9a-..."
}
```

`scopes` son los scopes globales del panel de la cuenta (los suyos y los de su rol). Con la 2FA activada, la respuesta es `{"otpNeeded": true}` junto con la cookie `session`, y el inicio de sesión se completa con `/auth/otp`.

| Error | Cuándo |
|-------|--------|
| `400 ErrCaptchaFailed` | Respuesta anti-bots ausente, incorrecta, caducada o reutilizada |
| `400 ErrInvalidCredentials` | Correo desconocido o contraseña incorrecta (la misma respuesta para ambos casos) |
| `403 ErrLoginNotPermitted` | La cuenta no tiene `login` |
| `429 ErrTooManyAttempts` | 10 intentos fallidos en 15 minutos para esta cuenta |

Los intentos fallidos sobre una cuenta conocida se anotan en el registro de acceso como `login_failed`, y los correctos como `login`.

### `POST /auth/otp`

```json
{ "token": "123456" }
```

`token` es el código TOTP actual o uno de los códigos de recuperación de la cuenta (cada código de recuperación sirve una sola vez). La petición debe llevar la cookie `session` del paso de la contraseña, con un máximo de 5 minutos de antigüedad. La respuesta es la misma que la de un inicio de sesión sin 2FA. Tras un acierto, el paso pendiente se borra, de modo que la cookie no puede abrir más sesiones.

Errores: `400 ErrInvalidSession` (no hay inicio de sesión pendiente), `400 ErrSessionExpired` (más de 5 minutos), `400 ErrInvalidCredentials`, `403 ErrLoginNotPermitted`, `429 ErrTooManyAttempts`.

### `POST /auth/logout`

Cierra la sesión indicada en `?token=...` o, si no se indica, la de la cookie `skypanel_auth`, y borra la cookie. Siempre `204`.

### `POST /auth/register`

Solo mientras los registros están abiertos (`panel.registrationEnabled`); en otro caso, `404`.

```json
{ "username": "newplayer", "email": "new@example.com", "password": "at-least-8", "captchaToken": "..." }
```

`username` tiene de 5 a 100 caracteres ASCII imprimibles; `email` debe ser válido; la contraseña necesita al menos 8 bytes. Con **Aprobar cuentas nuevas** activado (`panel.registrationApproval`, por defecto), la cuenta queda pendiente: sin el scope `login`, sin sesión ni correo, y la respuesta es `202 {"pending": true}`. Al iniciar sesión con la contraseña correcta recibe `403 ErrAccountPending` hasta que un administrador la apruebe con `POST /api/users/{id}/approve` (`users.perms.edit`), que le da `login` y envía el correo de bienvenida. Rechazarla es borrarla (`DELETE /api/users/{id}`). Con la aprobación desactivada, la cuenta recibe el scope `login`, un correo de bienvenida y una sesión (la misma respuesta que el inicio de sesión). Errores: `400` por validación, `400 ErrCaptchaFailed`, `400 ErrPasswordRequirements`.

### `POST /auth/forgot-password`

```json
{ "email": "admin@example.com", "captchaToken": "..." }
```

Siempre `204`, exista o no la cuenta. Si existe, se envía un correo con `<URL maestra>/reset-password/?token=...`. El enlace usa siempre `panel.settings.masterUrl`, nunca las cabeceras de la petición. Las solicitudes que superan 5 por correo y hora se descartan sin avisar. Errores: `400` por un correo no válido, `400 ErrCaptchaFailed`.

### `POST /auth/reset-password`

```json
{ "token": "...", "password": "new-password" }
```

`204` si todo va bien. La contraseña se comprueba antes de gastar el token, así que una contraseña demasiado corta puede volver a escribirse. Se cierran todas las sesiones de la cuenta y se envía un correo de confirmación. Errores: `400 ErrPasswordRequirements`, `400 ErrTokenInvalid`.

### `POST /auth/reauth`

Abre una sesión nueva para el usuario de la sesión actual y responde como un inicio de sesión. Las sesiones abiertas desde un cliente OAuth2 se rechazan (`403`), de modo que un secreto de cliente nunca puede convertirse en una sesión de navegador.

### `GET /auth/sso?token=...`

La ruta de destino de un enlace de un solo uso generado por [`POST /api/v1/sso`](./07-provisioning.md#post-apiv1sso). Abre la sesión, anota `login_sso` en el registro de acceso y redirige (`303`) al servidor o a `/servers/`. Un enlace usado, caducado o desconocido redirige a `/login/?sso=expired`; una cuenta sin `login`, a `/login/?sso=denied`. Las respuestas llevan `Referrer-Policy: no-referrer` y `Cache-Control: no-store`.

### `GET /auth/challenge`

La comprobación anti-bots integrada, que usan el inicio de sesión, el registro y el restablecimiento de contraseña cuando Turnstile no está configurado y `panel.captcha.builtin` está activado (valor por defecto).

```json
{
  "algorithm": "SHA-256",
  "challenge": "<hex SHA-256 of salt + number>",
  "salt": "<hex>?expires=1793580000",
  "maxnumber": 100000,
  "signature": "<hex HMAC>"
}
```

Hay que encontrar el entero `number` entre `0` y `maxnumber` tal que `SHA-256(salt + number)` sea igual a `challenge`, y después enviar `captchaToken` como el base64 de este JSON:

```json
{ "algorithm": "SHA-256", "challenge": "...", "number": 41234, "salt": "...", "signature": "..." }
```

Un desafío caduca a los 10 minutos, cada solución se acepta una sola vez y un reinicio del panel anula los desafíos entregados antes. Cloudflare Turnstile se describe en [Captcha / anti-bots](../../general/17-turnstile.md).

### `GET /auth/publickey`

El conjunto de claves públicas del panel (JWKS, Ed25519, id de clave `SkyPanel`). Los nodos lo usan para verificar los JWT que el panel firma para `/daemon` (`token.public` en la configuración del nodo).

## OAuth2

### `POST /oauth2/token`

Campos del formulario (`application/x-www-form-urlencoded`): `grant_type`, `client_id`, `client_secret`, `username`, `password`.

**`grant_type=client_credentials`**

```bash
curl -X POST https://panel.example.com/oauth2/token \
  -d grant_type=client_credentials -d client_id=CLIENT_ID -d client_secret=CLIENT_SECRET
```

```json
{ "access_token": "0b7c2f4e-...", "token_type": "Bearer", "expires_in": 3600, "scope": "login server.create" }
```

`scope` enumera los scopes de la fila de permisos global del propietario (no incluye los del rol). El propietario necesita `login` en esa fila; si no lo tiene, la respuesta es `403` sin cuerpo.

**`grant_type=password`** (solo nodos)

Un nodo la usa para comprobar un inicio de sesión SFTP. Envía `Authorization: Bearer <secreto del nodo>` (el `clientSecret` de su despliegue), `username=<email>#<serverId>` y la contraseña del usuario. El panel comprueba que el servidor esté en ese nodo, la contraseña, el scope `login` y `server.sftp` en el servidor. La respuesta no contiene token de acceso, solo `{"token_type": "Bearer", "expires_in": 3600, "scope": "<serverId>:server.sftp"}`: una contraseña sola nunca abre una sesión del panel (se saltaría la 2FA). El propio inicio de sesión SFTP no pide el código 2FA, porque los clientes SFTP no pueden enviarlo: las cuentas con 2FA deberían usar [claves SSH](./05-accounts.md#claves-ssh) para SFTP, que aceptan todos los nodos (los nodos remotos las comprueban con [`POST /auth/sftp-key`](#post-authsftp-key)).

| Error | Cuándo |
|-------|--------|
| `400 {"error": "invalid_request", ...}` | Formulario incorrecto, cliente desconocido, nombre de usuario incorrecto, contraseña incorrecta, sin acceso (`"error_description": "no access"`) |
| `400 {"error": "invalid_client"}` | Secreto de cliente incorrecto |
| `401 {"error": "invalid_client"}` | Concesión de contraseña sin cabecera bearer |
| `400 {"error": "unsupported_grant_type"}` | Cualquier otro tipo de concesión |

### `POST /auth/sftp-key`

Así comprueba un nodo remoto un inicio de sesión SFTP con clave SSH: las claves de los usuarios están en la base de datos del panel, que solo puede leer el nodo propio del panel. El servidor SFTP del nodo consulta esta ruta por cada clave que ofrece un cliente, antes de que el cliente demuestre que tiene la clave privada; el nodo no concede nada hasta que esa prueba es correcta.

El nodo se identifica con su secreto, como en la concesión de contraseña: `Authorization: Bearer <secreto del nodo>` (`daemon.auth.clientSecret` en su configuración). Encuentra el panel a partir de `daemon.auth.url`: la misma dirección con la ruta del token (`/oauth2/token`) cambiada por `/auth/sftp-key`, conservando el prefijo de ruta bajo el que se sirva el panel; esa URL debe ser una dirección `http(s)` absoluta. Cada comprobación espera como máximo 15 segundos.

```json
{ "username": "player@example.com#a1b2c3d4", "publicKey": "ssh-ed25519 AAAAC3Nza... laptop" }
```

| Campo | Notas |
|-------|-------|
| `username` | El usuario SFTP, `<email>#<serverId>` |
| `publicKey` | La clave que ofreció el cliente, como una línea de `authorized_keys` |

El cuerpo puede ocupar como máximo 16 KiB. Las comprobaciones son las mismas que en un inicio de sesión SFTP con clave en el nodo propio del panel, más una: el servidor debe estar en el nodo que pregunta. La clave debe ser una de las [claves SSH](./05-accounts.md#claves-ssh) de la cuenta, y la cuenta debe seguir teniendo `login` y `server.sftp` en el servidor.

Respuesta (`Cache-Control: no-store`); como la concesión de contraseña, nunca contiene una sesión:

```json
{ "scope": "a1b2c3d4:server.sftp" }
```

| Error | Cuándo |
|-------|--------|
| `401 {"error": "invalid_client"}` | Sin cabecera bearer, o el secreto no es el de ningún nodo (con `WWW-Authenticate: Bearer`) |
| `400 {"error": "invalid_request", "error_description": "bad request"}` | El cuerpo no es JSON válido o es demasiado grande |
| `400 {"error": "invalid_request", "error_description": "no access"}` | Usuario mal formado, clave ilegible, servidor que no está en el nodo que pregunta, cuenta desconocida, clave que no es de la cuenta, o sin `login` / `server.sftp`: la misma respuesta para todo, para no revelar qué parte falló |
| `429 {"error": "invalid_request", "error_description": "too many failed attempts, wait a few minutes"}` | 10 comprobaciones en 15 minutos para esta cuenta sin ningún acierto |
| `500 {"error": "server_error"}` | El panel no pudo buscar el nodo |

El contador de intentos es independiente del de contraseñas (clave `sshkey:user:<id>`): un cliente SSH ofrece sus claves una tras otra, y esos fallos no deben impedir al propietario entrar con su contraseña. Los inicios de sesión con clave en el nodo propio del panel cuentan en el mismo contador, así que el límite es igual en todos los nodos. Un acierto lo reinicia. Un nodo con una versión anterior solo admite contraseñas. Un nodo cuyo panel aún no tiene esta ruta, o cuyo `daemon.auth.url` no es una URL `http(s)` absoluta, rechaza los inicios de sesión con clave y registra el motivo; las contraseñas siguen funcionando.

### `POST /auth/sftp-check`

Cómo comprueba un nodo remoto las conexiones SFTP que ya dejó entrar. El nodo pregunta cada 30 segundos por cada conexión abierta, con el usuario con el que entró, y la cierra cuando la respuesta es `400`: el usuario se borró, perdió `login`, perdió `server.sftp` en el servidor (o el propio servidor), o el servidor ya no está en ese nodo. Cualquier otra respuesta, o ninguna (panel inalcanzable, un panel sin esta ruta), mantiene la conexión abierta, para que una caída no eche a todo el mundo. Se identifica con el secreto del nodo, como `/auth/sftp-key`, espera como máximo 15 segundos y no cuenta nada en los límites de intentos: la contraseña o la clave ya se comprobaron al entrar.

```json
{ "username": "player@example.com#a1b2c3d4" }
```

Respuesta (`Cache-Control: no-store`), nunca una sesión:

```json
{ "scope": "a1b2c3d4:server.sftp" }
```

| Error | Cuándo |
|-------|--------|
| `401 {"error": "invalid_client"}` | Sin cabecera bearer, o el secreto no es de un nodo |
| `400 {"error": "invalid_request", "error_description": "bad request"}` | El cuerpo no es JSON válido o pasa de 4 KiB |
| `400 {"error": "invalid_request", "error_description": "no access"}` | Usuario mal formado, servidor borrado o que no está en el nodo que pregunta, cuenta desconocida, o sin `login` / `server.sftp` |
| `500 {"error": "server_error"}` | El panel no pudo leer su base de datos; el nodo mantiene la conexión |

El nodo propio del panel hace la misma comprobación contra la base de datos, con el id de la cuenta, así que una cuenta nueva que reciba el id de un usuario borrado (SQLite los reutiliza) o un cambio de email también cierran la conexión.

## Configuración pública

### `GET /api/config`

Sin autenticación. La página de inicio de sesión la lee antes de que nadie inicie sesión.

```json
{
  "themes": { "active": "SkyPanel", "settings": "{}", "available": [] },
  "branding": { "name": "Aether Panel" },
  "registrationEnabled": true,
  "turnstileSiteKey": "",
  "turnstileEnabled": false,
  "captcha": "builtin",
  "hideAiAnalysis": false,
  "experimental": { "versionSwitch": false, "propertiesEditor": false, "serverClone": false },
  "version": "3.x.y"
}
```

| Campo | Significado |
|-------|-------------|
| `themes.available` | Archivos de tema (`*.tar`) encontrados en `<panel.web.files>/theme` |
| `captcha` | Qué comprobación anti-bots necesitan los formularios: `turnstile` (activado y con las dos claves configuradas), `builtin`, o vacío si no hay ninguna |
| `hideAiAnalysis` | `panel.settings.hideAiAnalysis`: el análisis de registros con IA está desactivado para todos los usuarios; el cliente web oculta sus botones y `POST /api/servers/:serverId/ai/analyze` responde `403 ErrAIAnalysisDisabled` |
| `experimental` | Funciones experimentales activadas en la configuración; sus rutas responden `403 ErrFeatureDisabled` mientras están desactivadas |
