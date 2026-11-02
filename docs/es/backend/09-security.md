# Seguridad: permisos, tokens, SFTP y protecciones HTTP

Cómo decide el backend quién puede hacer qué, cómo confían entre sí el panel y sus nodos, cómo autentica el servidor SFTP integrado y qué protecciones lleva cada respuesta HTTP.

Páginas relacionadas:

- [Seguridad y buenas prácticas](../general/security.md): cuentas, verificación en dos pasos, contraseñas y recomendaciones para administradores.
- [Referencia de permisos (scopes)](../general/scopes-reference.md): todos los scopes, propietarios, subusuarios, roles y claves API.
- [unshare y aislamiento del kernel](../general/20-security-unshare.md): aislamiento de procesos para servidores en el entorno host.

---

## Comprobación de permisos

Las rutas declaran el scope que necesitan con `middleware.RequiresPermission(scope)` (`internal/middleware/middleware.go`). En cada petición, la comprobación reúne:

1. los permisos del usuario **en el servidor** de la URL (`:serverId`), si lo hay;
2. los permisos **globales** del usuario;
3. los scopes del **rol** del usuario, si tiene uno asignado;
4. los **scopes de propietario**, cuando el usuario es dueño de ese servidor.

`admin` satisface cualquier scope y `server.admin` satisface cualquier scope de servidor. Una sesión creada desde un **cliente OAuth2** queda además limitada a aquello para lo que se registró el cliente: un cliente ligado a un servidor solo funciona en ese servidor, y un cliente con scopes declarados nunca los supera, aunque su usuario pueda hacer más. La lista de scopes que la comprobación guarda en la petición (`c.Set("scopes")`, que los controladores leen con `callerScopes`) se limita igual (`boundByClient`), así que las segundas comprobaciones de los controladores (quién ve la contraseña de una base de datos, quién puede cambiar una variable de entorno bloqueada, qué scopes se pueden conceder, qué scopes puede usar un paso de una tarea) y los flujos a los que se suscribe el WebSocket del servidor nunca ven los permisos más amplios del usuario.

Los scopes se definen en `internal/scopes/scopes.go`: un scope *de servidor* (`forServer: true`, como `server.files.edit`) se concede por servidor, y uno *del panel* (como `nodes.edit`) de forma global. La lista completa y lo que permite cada uno están en la [referencia de scopes](../general/scopes-reference.md).

---

## Tokens entre el panel y los nodos

El panel firma un token de vida corta para cada petición que envía al daemon de un nodo (`/daemon/*`), incluidos los proxies de WebSocket. Código: `internal/services/token.go`, `internal/middleware/decrypt.go`.

### La clave de firma

- Una clave **Ed25519**. Su semilla de 32 bytes se guarda en base64 en `panel.token`; si esa clave está vacía cuando el panel la necesita por primera vez, se genera una semilla aleatoria y se guarda.
- La mitad pública se publica como JWKS, con el identificador fijo `SkyPanel`, en `GET /auth/publickey`.

### Tokens enviados a los nodos

| Propiedad | Valor |
|-----------|-------|
| Algoritmo | `EdDSA` (cabecera `kid: SkyPanel`) |
| `iat` / `exp` | Momento de emisión / emisión + **15 minutos** |
| `aud` | `hex(sha256("skypanel-daemon-aud:" + secreto del nodo))`: liga el token a un nodo sin incluir el secreto |

El daemon (`middleware.ValidateJWT`) solo acepta un token firmado con `EdDSA` por la clave del panel, con `exp` (un minuto de margen para desfases de reloj) y cuya audiencia coincide con el secreto del nodo local (panel y daemon en un mismo proceso) o con `daemon.auth.clientSecret` (un nodo desplegado aparte). Por tanto, un token capturado de camino a un nodo no sirve contra otro.

Un token que no supera estas comprobaciones recibe `401` con `WWW-Authenticate: Bearer error="invalid_token"` y queda registrado en el nodo. El panel muestra ese rechazo al usuario como `502 ErrNodeAuthFailed` en lugar de transmitir el `401`, para que un secreto de nodo equivocado o un reloj desajustado no parezcan el fin de la sesión del usuario (ver [Capa API](./05-api-layer.md#peticiones-del-panel-a-los-nodos)).

Cómo obtiene el daemon la clave pública:

- **Panel activo** en el mismo proceso: de memoria.
- **Nodo solo daemon** (`panel.enable=false`): descarga el JWKS de la URL de `token.public` (el `/auth/publickey` del panel). Si `token.public` está vacío, el nodo deja un aviso al arrancar y no puede autenticar al panel.

### Secretos de los nodos

Cada nodo tiene un secreto que guarda el panel. Un administrador con `nodes.deploy` lo lee en `GET /api/nodes/:id/deployment` (`clientId` es `.node_<id>` y `clientSecret` es el secreto) y lo pone en `daemon.auth.clientSecret` del nodo. El secreto del nodo local se deriva de `panel.sessionKey`, así que no cambia entre reinicios.

El nodo también usa su secreto en sentido contrario: para preguntar al panel si un inicio de sesión SFTP, con contraseña o con clave SSH, es válido (siguiente sección). Cuando el panel reenvía una petición del navegador a un nodo, **quita la cabecera `Cookie`**: la sesión del usuario nunca llega a un nodo.

---

## Servidor SFTP

Cada daemon ejecuta un servidor SFTP integrado (`internal/sftp`, no necesita OpenSSH).

| Clave | Por defecto | Descripción |
|-------|-------------|-------------|
| `daemon.sftp.host` | `0.0.0.0:5657` | Dirección de escucha |
| `daemon.sftp.key` | `<data>/sftp.key` | Clave del host. Si no existe, se genera y guarda una clave Ed25519 (PEM PKCS#8) |
| `daemon.sftp.log` | `false` | Registra cada petición SFTP (método, servidor, ruta) en nivel debug |

### Nombre de usuario

```
<email>#<serverId>
```

Un inicio de sesión da acceso a un servidor. Los clientes se conectan con `sftp -P 5657 'admin@example.com#a1b2c3d4'@panel.example.com`.

### Quién puede entrar

Solo se acepta un inicio de sesión si la cuenta todavía puede entrar en el panel (scope global `login`) **y** tiene `server.sftp` en ese servidor. Los propietarios lo tienen por los scopes de propietario; `admin` y `server.admin` lo incluyen. SFTP no distingue entre `server.files.view` y `server.files.edit`: quien tiene `server.sftp` puede leer y escribir toda la carpeta del servidor. Para SFTP no se pide la verificación en dos pasos, porque los clientes SFTP no pueden enviar un código: una cuenta con 2FA puede entrar por SFTP solo con su contraseña. Esas cuentas deberían usar claves SSH, que aceptan todos los nodos (véase abajo), y mantener una contraseña robusta, porque la contraseña sigue abriendo SFTP. Cualquier fallo devuelve el mismo «incorrect username or password», de modo que la respuesta no revela si el email existe.

### Autenticación por contraseña y por clave

| Nodo | Contraseña | Clave SSH |
|------|------------|-----------|
| El nodo del propio panel (panel activo) | Se comprueba contra la base de datos | Sí: cualquier clave pública que el usuario haya añadido en la sección **Claves SSH** de su cuenta (`/api/self/sshkeys`) |
| Un nodo remoto solo daemon | La comprueba el panel por HTTP (ver abajo) | Sí: las mismas claves, que comprueba el panel por HTTP (ver abajo) |

**Los nodos remotos** envían `grant_type=password`, `username`, `password` y `scope=sftp` a `daemon.auth.url` (la URL completa del `/oauth2/token` del panel), autenticados con `Authorization: Bearer <daemon.auth.clientSecret>`. El panel comprueba que el secreto pertenece a un nodo, que el servidor está en **ese** nodo, las credenciales, `login` y `server.sftp`, y responde con el scope `<serverId>:server.sftp`. A propósito no devuelve un token de acceso: una contraseña sola no debe dar una sesión completa del panel que se salte la verificación en dos pasos.

**Claves SSH en nodos remotos.** Un nodo remoto no tiene las claves de los usuarios, así que pregunta al panel (`internal/oauth2/ssh.go`, límite de 15 s): envía `{"username": "email#idServidor", "publicKey": "<línea de authorized_keys>"}` a `POST /auth/sftp-key` con la misma cabecera `Authorization: Bearer <daemon.auth.clientSecret>`. La URL sale de `daemon.auth.url`: mismo esquema y host, con `/oauth2/token` sustituido por `/auth/sftp-key` (se conserva la ruta que haya delante). El panel (`internal/web/auth/sftp_key.go`) aplica las reglas del nodo propio del panel y una más: el servidor debe estar en el nodo que pregunta. La clave debe ser una de las del usuario, y la cuenta debe seguir teniendo `login` y `server.sftp` sobre ese servidor. Solo responde el scope concedido (`200 {"scope": "<idServidor>:server.sftp"}`), nunca una sesión; cualquier rechazo es el mismo `400` «no access», y un secreto de nodo desconocido recibe `401`. La biblioteca SSH hace esta comprobación antes de que el cliente demuestre que tiene la clave privada, y no concede nada hasta que esa prueba sale bien. Tanto el panel como el nodo necesitan una versión con esta ruta: un panel anterior responde `404` y el nodo rechaza los inicios de sesión con clave.

**Claves SSH** que se aceptan al añadirlas: Ed25519, ECDSA (P-256, P-384, P-521), las variantes con llave de seguridad `sk-ssh-ed25519@openssh.com` y `sk-ecdsa-sha2-nistp256@openssh.com`, y RSA de al menos 2048 bits. Se rechazan las claves DSA, los certificados, las opciones de `authorized_keys` y las claves privadas. Hasta 25 claves por usuario; el nombre (por defecto, el comentario de la clave) admite hasta 100 caracteres. Al entrar, la clave ofrecida se busca por su huella SHA-256 y después se compara byte a byte.

### Límite de intentos de inicio de sesión

Los inicios de sesión con contraseña comparten el limitador de intentos del panel (`internal/services/login_attempts.go`):

- **10 intentos por cuenta cada 15 minutos**, contados en conjunto para el login web, la comprobación de contraseña OAuth2 que usan los nodos remotos, las contraseñas SFTP y los códigos de verificación en dos pasos.
- Se cuentan por cuenta (o por el email escrito cuando no corresponde a ninguna cuenta), no por IP, así que funciona igual detrás de un proxy inverso.
- Un inicio de sesión correcto reinicia el contador. Superado el límite, la respuesta es «demasiados intentos» hasta que los intentos antiguos salen de la ventana.
- Se guarda en memoria: reiniciar el panel lo pone a cero.

Los inicios de sesión con clave en el nodo propio del panel no se cuentan, porque una clave no se puede adivinar. En los nodos remotos, las comprobaciones de clave de `/auth/sftp-key` tienen un cupo propio, también de 10 por cuenta cada 15 minutos (clave `sshkey:user:<id>`), separado del de contraseñas: un cliente SSH ofrece sus claves una tras otra, y esos fallos no deben impedir al dueño entrar con su contraseña. Un inicio de sesión con clave correcto lo reinicia; superado el límite, la ruta responde `429`.

### Qué puede hacer una sesión

- Solo se aceptan canales `session` con el subsistema `sftp`; shell, exec y redirección de puertos se rechazan.
- Cada ruta se resuelve a través del servidor de archivos confinado del servidor (véase [Acceso a archivos](#acceso-a-archivos)).
- Las subidas respetan los indicadores de crear/exclusivo/truncar del cliente y se detienen en el límite de disco del servidor.
- `Setstat` (permisos, fechas) y `Symlink` se aceptan pero no hacen nada, así que un cliente no puede crear enlaces.

---

## Acceso a archivos

Toda la E/S de archivos de los servidores (gestor de archivos, SFTP, operaciones, copias de seguridad, extracción) pasa por `files.FileServer` (`files/filesystem.go`):

- Las rutas se abren relativas a la carpeta del servidor con `openat2` y `RESOLVE_BENEATH`: el kernel rechaza cualquier ruta, `..` o enlace simbólico que acabe fuera de ella.
- Al arrancar, el daemon prueba `openat2` y **se niega a arrancar** si no puede usarla (un kernel sin ella, o un perfil seccomp que la bloquea y responde `EPERM`). `security.forceOpenat=true` omite la comprobación y usa una alternativa que abre cada componente de la ruta con `openat` y `O_NOFOLLOW`; úsela solo donde no se pueda usar `openat2`.
- Los archivos que escriben las descargas se crean con `O_NOFOLLOW`, así que un enlace simbólico dejado donde va un archivo se rechaza en lugar de seguirse. Véase [Operaciones: seguridad de los archivos](./06-operations.md#seguridad-de-los-archivos).

---

## Aislamiento de procesos y contenedores

| Clave | Por defecto | Efecto |
|-------|-------------|--------|
| `docker.disallowHost` | `false` (`true` en la imagen Docker) | Se rechazan los servidores con entorno `host`, también al arrancarlos |
| `docker.serverUser` | `988:988` | uid:gid con el que corren los contenedores de juego cuando el panel corre como root (las instalaciones siguen como root). `0:0` mantiene root |
| `docker.pidsLimit` | `4096` | Máximo de procesos/hilos por contenedor de servidor; `0` quita el límite |
| `security.disableUnshare` | `false` | Desactiva el aislamiento con `unshare` de los servidores en entorno host |

Los contenedores de juego también eliminan las capacidades que no necesitan (menos cuando corren como root), llevan `no-new-privileges` y tienen su propio `/tmp` y `/var/tmp` junto a la carpeta del servidor. El aislamiento del entorno host se describe en [unshare y aislamiento del kernel](../general/20-security-unshare.md).

---

## Protecciones HTTP

Las aplican a cada petición `internal/web/loader.go` e `internal/middleware/security.go`.

### Cabeceras de seguridad

| Cabecera | Valor |
|----------|-------|
| `Content-Security-Policy` | `frame-ancestors 'self'; object-src 'none'; base-uri 'self'` |
| `X-Frame-Options` | `SAMEORIGIN` |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `same-origin` |
| `Strict-Transport-Security` | `max-age=31536000`, solo cuando la petición llegó por HTTPS (directamente o con `X-Forwarded-Proto: https`) |

Las páginas que sirven contenido no confiable sustituyen la CSP por su propio sandbox: los [archivos de extensiones](../reference/15-extensions.md#páginas) y el visor del mapa del mundo.

### Peticiones entre sitios

`SameOriginGuard` rechaza con `403 {"error": "cross-origin request refused"}` las peticiones que llevan una cookie del panel, cambian estado (todo salvo `GET`/`HEAD`/`OPTIONS`, más los handshakes de WebSocket) y vienen de otro sitio. Los clientes con token bearer no se ven afectados. Detalles en [WebSocket: comprobación de origen](./08-websocket.md#comprobación-de-origen).

### CORS

Cualquier origen puede llamar a la API, pero **sin credenciales** (`Access-Control-Allow-Credentials` desactivado): otros sitios pueden usar un token bearer, nunca las cookies del usuario. Cabeceras permitidas: `Authorization`, `Content-Type`, `Accept`, `Origin`; métodos `GET`, `POST`, `PUT`, `DELETE`, `OPTIONS`.

### Datos ocultos en el log de accesos

El log de peticiones oculta los valores de los parámetros `token`, `code`, `access_token` y `refresh_token` y el token firmado de los enlaces del mapa del mundo.

---

## Cookies

| Cookie | Para qué | Atributos |
|--------|----------|-----------|
| `skypanel_auth` | El token de la sesión iniciada | `HttpOnly`, `SameSite=Lax`, ruta `/`. `Secure` cuando la petición llegó por HTTPS (o `X-Forwarded-Proto: https`) o si `panel.web.cookies.secure` es `true`, para que el inicio de sesión funcione también en `http://IP:8080` |
| `session` | Almacén de sesión basado en cookie (`gin-contrib/sessions`) | Se configura con las claves siguientes |

| Clave | Por defecto |
|-------|-------------|
| `panel.web.cookies.secure` | `false` |
| `panel.web.cookies.httpOnly` | `true` |
| `panel.web.cookies.sameSite` | `Strict` (también se aceptan `Lax` y `None`) |
| `panel.web.cookies.age` | `2592000` (30 días, en segundos) |
| `panel.web.cookies.path` | `/` |
| `panel.web.cookies.domain` | vacío |

La cookie `session` se firma con `panel.sessionKey`, que se genera en el primer arranque.

---

## Secretos en la API de ajustes

`GET /api/settings` y `GET /api/settings/:key` nunca devuelven un secreto guardado: las contraseñas y claves del correo, las claves de Gemini y CurseForge, los cuatro webhooks de Discord, la clave de licencia, la clave secreta de S3, el token de Cloudflare y la clave secreta de Turnstile se devuelven como `********` (uno sin definir sigue vacío). Cuando una petición devuelve `********` para uno de ellos (`POST /api/settings`, `PUT /api/settings/:key` y las pruebas de correo, Discord, S3 y Cloudflare), se conserva el valor guardado. Así, un usuario con `settings.edit` puede sustituir estos secretos, pero no leerlos desde el panel. Código: `internal/web/api/settings_secrets.go`.

---

## Claves API

Las integraciones de aprovisionamiento (WHMCS, Paymenter) llaman a `/api/v1/*` con una clave API en lugar de una sesión:

- Formato `ak_<5 hex>_<43 hex>`, generado a partir de 24 bytes aleatorios. Solo se guarda su SHA-256, junto con el prefijo de 8 caracteres para buscarla; la clave completa se muestra una sola vez.
- Se envía como `X-Api-Key: <clave>` o `Authorization: Bearer <clave>` (solo los valores que empiezan por `ak_` se tratan como claves).
- Cada clave lleva una lista de acciones permitidas, que se comprueba en cada ruta.

Crear claves y sus permisos: [referencia de scopes — Claves API](../general/scopes-reference.md#claves-api).

---

## Detrás de un proxy inverso

Por defecto el panel **no** confía en ninguna cabecera de proxy para la dirección del cliente.

| Clave | Por defecto | Descripción |
|-------|-------------|-------------|
| `security.trustedProxies` | `[]` | IP o CIDR cuyo `X-Forwarded-For` se cree |
| `security.trustedProxyHeader` | vacío | Una cabecera con la IP real del cliente que fija una plataforma de confianza, como `CF-Connecting-IP` (Cloudflare) o `X-Real-IP`. Se lee venga de donde venga la petición, así que solo debe fijarse cuando todas las peticiones pasan por esa plataforma |

```json
{
  "security": {
    "trustedProxies": ["127.0.0.1", "172.16.0.0/12"],
    "trustedProxyHeader": "X-Real-IP"
  }
}
```

La configuración de Nginx del instalador enlaza el panel a `127.0.0.1` y envía `X-Forwarded-Proto`, lo que además hace `Secure` las cookies y activa HSTS.

---

## Descargas de las plantillas

`download`, `scraperdl` y la descarga del recurso de `githubdl` nunca se conectan a direcciones loopback, de enlace local o de metadatos, y rechazan las redes privadas mientras `security.blockPrivateDownloads` sea `true` (el valor por defecto). Véase [Operaciones: reglas de red de las descargas](./06-operations.md#reglas-de-red-de-las-descargas).

---

## Limitaciones conocidas

- Los inicios de sesión SFTP con contraseña no piden 2FA (véase [Quién puede entrar](#quién-puede-entrar)); las claves SSH, que aceptan todos los nodos, evitan enviar la contraseña.
- Los secretos TOTP (`users.otp_secret`, `otp_pending_secret`) se guardan en la base de datos sin cifrar; proteja en consecuencia el acceso a la base de datos y sus copias.
- El host del servidor de licencias está fijado en el código y es un host de pruebas (véase [Servicios → LicenseService](./11-services.md#22-licenseservice-licensego)).
