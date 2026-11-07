# Seguridad

Esta página es para quien instala y administra Aether Panel. Explica cómo exponer el panel de forma segura, cómo funcionan el inicio de sesión y las sesiones, y las protecciones que el panel aplica a los servidores de juego, a los archivos y a la interfaz web. El modelo completo de permisos está en la [Referencia de scopes](./scopes-reference.md).

## Resumen

| Capa | Qué hace el panel |
|------|-------------------|
| Exposición | Escucha en HTTP sin cifrar (`web.host`, `0.0.0.0:8080`). El HTTPS lo pone Nginx, que el instalador configura cuando se usa un dominio; entonces el puerto del panel se publica solo en `127.0.0.1`. |
| Inicio de sesión | Contraseñas con bcrypt (de 8 a 72 bytes), 2FA TOTP opcional con códigos de recuperación, comprobación anti-bots y límite de intentos por cuenta. |
| Sesiones | Tokens aleatorios guardados solo como hash SHA-256, 1 hora de inactividad, cookie HttpOnly, `Secure` sobre HTTPS. |
| Peticiones web | Las peticiones con cookie que cambian algo deben venir del propio panel; cabeceras de seguridad en cada respuesta; CORS nunca lleva cookies. |
| Permisos | 73 scopes, propietarios y subusuarios, filas de servidor que solo guardan scopes de servidor, personal que no puede editar cuentas con más poder. |
| Servidores de juego | Solo Docker, usuario sin privilegios, capacidades eliminadas, `no-new-privileges`, límite de procesos y red propia. |
| Archivos | Toda operación con archivos se queda dentro de la carpeta del servidor; los enlaces nunca se siguen hacia fuera. |

## Exponer el panel con un dominio y HTTPS

Un panel con clientes debe abrirse en `https://su-dominio`, no en `http://IP:8080`.

**Por qué un panel en `http://IP` es un riesgo.** Los navegadores asocian las cookies al host, no al puerto. Un panel en `http://203.0.113.10:8080` envía su cookie de sesión a todos los puertos de `203.0.113.10`, y los servidores de juego de los clientes funcionan en esa misma IP. Una página web o un plugin de un servidor de juego (por ejemplo, un visor de mapas en el puerto 8123) recibe la cookie del administrador cuando este la abre. La [protección de mismo origen](#protecciones-web) del panel impide que esa página haga cambios con la cookie, pero la cookie sigue viajando sin cifrar por HTTP y hacia esos puertos. Con un dominio y HTTPS la cookie se marca como `Secure`, solo se envía al host del panel y viaja cifrada.

**Cómo configurarlo.**

1. Apuntar un dominio (o subdominio) a la IP del servidor, con o sin el proxy de Cloudflare.
2. Pasarlo al instalador (`--domain panel.example.com --ssl`) o, con el panel ya funcionando, escribir `https://panel.example.com` en **Configuración → General → URL principal**. El servicio `aether-domain` que deja el instalador configura entonces Nginx, el certificado de Let's Encrypt y el firewall. Los detalles están en la [Guía de instalación](../setup/01-installation.md).
3. Configurar la URL principal (`panel.settings.masterUrl`) en cualquier caso: los enlaces para restablecer la contraseña y de acceso directo se construyen con ella, nunca con las cabeceras de la petición.

**Qué hace el instalador con un dominio.** Publica el puerto del panel solo en `127.0.0.1`, así que al panel se entra por Nginx y nadie puede usar `http://IP:8080` saltándose el HTTPS. Docker publica los puertos de los contenedores por delante de ufw/firewalld, de modo que una regla del firewall no bastaría para cerrarlo. Use `--bind 0.0.0.0` solo si necesita ese puerto abierto a propósito.

**Detrás de un proxy propio.** El panel siempre confía en esta máquina, las redes privadas y el borde de Cloudflare, lo que cubre el Nginx del instalador, con o sin Cloudflare delante (ver [Detrás de un proxy inverso](../backend/09-security.md#detrás-de-un-proxy-inverso)). Si su proxy tiene una dirección pública, póngala en `security.trustedProxies` (y `security.trustedProxyHeader` si usa una cabecera propia) para que el panel vea la IP real del cliente. Envíe `X-Forwarded-Proto: https`: el panel lo usa para marcar la cookie como `Secure` y para enviar HSTS.

## Inicio de sesión y sesiones

Los formularios de inicio de sesión, registro y cambio de contraseña pasan una comprobación anti-bots, descrita en [Captcha / anti-bots](17-turnstile.md).

### Sesiones

- Al iniciar sesión se crea un token aleatorio (UUID v4). La base de datos solo guarda su hash SHA-256.
- Una sesión dura **1 hora sin actividad**. Mientras el usuario está activo se renueva (la caducidad en la base de datos cuando ha pasado más de la mitad, la cookie en cada petición), así que un usuario activo no pierde la sesión.
- El token se devuelve en la cookie `skypanel_auth` y en la respuesta JSON (`session`), para los clientes que usan `Authorization: Bearer <token>`. El panel lee primero la cabecera y después la cookie.
- Cerrar sesión borra la sesión. Además, el panel comprueba el permiso `login` en cada petición (ver [Quitar el permiso de entrar](#quitar-el-permiso-de-entrar)).

### Cookies

| Cookie | Qué guarda | Atributos |
|--------|------------|-----------|
| `skypanel_auth` | El token de sesión | `HttpOnly`, `SameSite=Lax`, ruta `/`, duración 1 hora (se renueva con la actividad). `Secure` cuando la petición llegó por HTTPS (directamente o con `X-Forwarded-Proto: https`) o cuando `panel.web.cookies.secure` está activado. |
| `session` | Estado breve del inicio de sesión, como el paso pendiente de 2FA (firmada con `panel.sessionKey`) | La configuran las claves `panel.web.cookies.*`: `path` (`/`), `domain` (vacío), `age` (30 días), `secure` (`false`), `httpOnly` (`true`), `sameSite` (`Strict`; también acepta `Lax` y `None`). |

`skypanel_auth` no se marca como `Secure` en HTTP a propósito: los navegadores descartan una cookie `Secure` enviada por HTTP, y eso impedía iniciar sesión en `http://IP:8080`. Sobre HTTPS siempre lo es. El panel también guarda algunas preferencias en el almacenamiento local del navegador (idioma, última pestaña abierta, avisos cerrados); no contienen credenciales.

### Límite de intentos

| Qué | Límite | Respuesta al pasarlo |
|-----|--------|----------------------|
| Contraseña (web, OAuth2 y SFTP comparten contador) | 10 intentos en 15 minutos por cuenta | `429` `ErrTooManyAttempts` |
| Código 2FA | 10 intentos en 15 minutos por cuenta | `429` `ErrTooManyAttempts` |
| Clave SSH (SFTP en el propio nodo del panel y `POST /auth/sftp-key` para los nodos remotos) | 10 intentos en 15 minutos por cuenta, con un contador propio que comparten todos los nodos | Se rechaza el inicio de sesión SFTP (`ErrTooManyAttempts` en el propio nodo del panel, `429` a un nodo remoto) |
| Correos de cambio de contraseña | 5 por dirección y hora | `204`, sin enviar correo |

- El contador es de la **cuenta**, no de la IP, así que funciona igual detrás de un proxy. Las variantes de un mismo correo que la base de datos considera iguales (mayúsculas, tildes) comparten contador.
- Para un correo sin cuenta, el contador va por la dirección escrita, y la respuesta tarda lo mismo que con una contraseña incorrecta, para que el tiempo no revele qué correos existen.
- Un inicio de sesión correcto pone el contador a cero. Vive en memoria y también se pone a cero al reiniciar el panel.
- Las comprobaciones de claves SSH tienen su propio contador, separado del de la contraseña: un cliente SFTP ofrece sus claves una tras otra, y cada clave ofrecida cuenta como un intento, así que esos fallos no deben impedir al propietario entrar con su contraseña. El propio nodo del panel y los nodos remotos (mediante `POST /auth/sftp-key`) cuentan en el mismo contador, así que el límite es igual en todos los nodos.
- Cuando el paso de 2FA sale bien, el estado pendiente guardado en la cookie `session` se borra: la misma cookie no sirve para probar más códigos.

### Contraseñas

- Se guardan con bcrypt (coste 10), con una sal nueva cada vez.
- Deben tener entre 8 y 72 bytes (el límite de bcrypt; las letras con tilde y los emojis ocupan más de un byte).
- Recomendado: 12 caracteres o más, un gestor de contraseñas y 2FA en todas las cuentas de administrador.

### Verificación en dos pasos (2FA)

Códigos TOTP de 6 dígitos, compatibles con Google Authenticator, Authy, Microsoft Authenticator y similares.

**Activarla**: **Configuración de la cuenta → Autenticación de dos pasos → Habilitar 2FA**. Se escanea el código QR y se escribe el código que da la aplicación. Hasta que no se confirma, la 2FA no se activa.

Al confirmarla, el panel muestra **10 códigos de recuperación** de 12 caracteres. Hay que guardarlos: el panel solo almacena su hash y no los puede volver a mostrar. Cada uno sirve **una vez** en lugar del código de la aplicación.

| Acción | Qué pide | Qué pasa |
|--------|----------|----------|
| Regenerar códigos | Un código de la aplicación o de recuperación | Los anteriores dejan de funcionar. |
| Desactivar | Un código de la aplicación o de recuperación | Se borran la clave y los códigos. |
| Cambiar de teléfono | Desactivarla y volver a activarla | No se puede activar otra encima de una activa (`ErrOtpAlreadyEnabled`). |

Si el correo está configurado, el panel avisa al activarla, al desactivarla y al regenerar los códigos.

**Si se pierden el teléfono y los códigos**, un administrador la quita desde el servidor con `SkyPanel user edit` → **Remove 2FA** (ver [CLI](../backend/07-cli.md)).

**Al iniciar sesión**, `POST /auth/login` responde `{"otpNeeded": true}` y el código se envía a `POST /auth/otp` en la misma sesión del navegador, antes de **5 minutos**. Pasado ese tiempo hay que volver a escribir la contraseña (`ErrSessionExpired`).

```bash
# 1. Contraseña (más el token anti-bots): responde {"otpNeeded": true}
curl -c cookies.txt -X POST https://panel.example.com/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email": "ana@example.com", "password": "<su-contraseña>", "captchaToken": "<token>"}'

# 2. Código de la aplicación (o uno de recuperación), con la misma cookie
curl -b cookies.txt -X POST https://panel.example.com/auth/otp \
  -H 'Content-Type: application/json' \
  -d '{"token": "123456"}'
```

| Método y ruta | Cuerpo | Respuesta |
|---------------|--------|-----------|
| `GET /api/self/otp` | — | `{"otpEnabled": true}` |
| `POST /api/self/otp` | — | `{"secret": "...", "img": "data:image/png;base64,..."}` |
| `PUT /api/self/otp` | `{"token": "123456"}` | `{"recoveryCodes": ["...", ...]}` |
| `POST /api/self/otp/recovery` | `{"token": "123456"}` | `{"recoveryCodes": ["...", ...]}` |
| `DELETE /api/self/otp/{token}` | — | `204` |

Todas necesitan el permiso `self.edit`.

El SFTP no pide el código 2FA: entra con la contraseña o con una [clave SSH](#claves-ssh-para-sftp). Quien tenga la 2FA activada debería usar claves SSH, que funcionan en todos los nodos (véase [Limitaciones conocidas](#limitaciones-conocidas)).

### Quitar el permiso de entrar

El panel comprueba el permiso `login` en **cada petición**, no solo al iniciar sesión. Cuenta el `login` de los permisos generales del usuario o el de su rol; el de un servidor concreto no cuenta.

Si se le quita a un usuario (o se le quita el rol que se lo daba), su sesión se cierra en la siguiente petición: el panel la borra y responde `401`.

Otras situaciones que cierran sesiones:

| Qué | Sesiones que se cierran |
|-----|-------------------------|
| El usuario cambia su contraseña en su perfil | Todas menos la actual |
| Un administrador le pone una contraseña nueva | Todas |
| Se restablece la contraseña con el enlace del correo | Todas |
| Se borra un cliente OAuth2 | Todas las de ese cliente |

Quitar `login` también cierra el SFTP: el panel lo comprueba en cada conexión, con contraseña o con clave SSH, además de `server.sftp`. Las conexiones ya abiertas se vuelven a comprobar cada 30 segundos, así que borrar el usuario o quitarle `login`, `server.sftp` o el acceso al servidor las cierra en ese tiempo; si no se puede llegar a la base de datos del panel (o, en un nodo remoto, al panel), las conexiones abiertas siguen hasta que responda.

### Claves SSH para SFTP

Un usuario puede entrar por SFTP con una clave pública en lugar de su contraseña. El usuario sigue siendo `correo#idServidor`.

**Añadir una clave**: **Configuración de la cuenta → Claves SSH → Añadir clave**. Se pega la línea del archivo `.pub` (por ejemplo `~/.ssh/id_ed25519.pub`). Si no se le pone nombre, se usa el comentario de la clave.

| Regla | Detalle |
|-------|---------|
| Tipos aceptados | `ssh-ed25519`, ECDSA (256, 384 y 521), llaves de seguridad (`sk-ssh-ed25519@openssh.com`, `sk-ecdsa-sha2-nistp256@openssh.com`) y RSA de 2048 bits o más. |
| Rechazadas | DSA, certificados, líneas con opciones (`from=`, `command=`…) y claves privadas. |
| Nombre | Hasta 100 caracteres. |
| Máximo | 25 claves por cuenta. La misma clave no se puede añadir dos veces. |

El panel muestra la huella SHA256 de cada clave, la misma que imprime `ssh-keygen -l`.

**Conectar**:

```bash
sftp -P 5657 -i ~/.ssh/id_ed25519 -o User='ana@example.com#1a2b3c4d' panel.example.com
```

El usuario necesita igualmente `server.sftp` en ese servidor (o ser su propietario). Borrar una clave impide las conexiones nuevas con ella; borrar el usuario borra sus claves.

Las claves funcionan en todos los nodos:

- **Nodo del propio panel**: el servidor SFTP comprueba la clave en la base de datos del panel.
- **Nodo remoto**: el nodo no tiene las claves de los usuarios, así que pregunta al panel. Envía `POST /auth/sftp-key` al panel indicado en su `daemon.auth.url` (la parte `/oauth2/token`, si la hay, se sustituye por `/auth/sftp-key`; un prefijo de ruta se conserva), firmado con su propio secreto (`Authorization: Bearer <daemon.auth.clientSecret>`), con el usuario y la clave pública ofrecida. El panel aplica las mismas reglas que en su propio nodo y además comprueba que el servidor está en el nodo que pregunta. Solo responde el permiso concedido (`<serverID>:server.sftp`), nunca una sesión. Todo inicio de sesión rechazado recibe la misma respuesta (`400`, `no access`), para no revelar si fallaba la cuenta, el servidor o la clave. El nodo espera como mucho 15 segundos la respuesta. Después, el protocolo SSH obliga al cliente a demostrar que tiene la clave privada antes de conceder nada. Los detalles de la ruta están en la [referencia de la API de autenticación](../reference/api/02-authentication.md).

Un nodo remoto con una versión anterior a esta comprobación solo acepta contraseñas: actualícelo para usar claves en él.

| Método y ruta | Cuerpo | Respuesta |
|---------------|--------|-----------|
| `GET /api/self/sshkeys` | — | Lista de claves |
| `POST /api/self/sshkeys` | `{"name": "Portátil", "publicKey": "ssh-ed25519 AAAA... ana@portatil"}` | La clave creada |
| `DELETE /api/self/sshkeys/{id}` | — | `204` |

```json
{
  "id": 3,
  "name": "Portátil",
  "fingerprint": "SHA256:…",
  "type": "ssh-ed25519",
  "createdAt": "2026-10-26T16:00:00Z"
}
```

Todas necesitan el permiso `self.edit`. Errores: `ErrSSHKeyInvalid`, `ErrSSHKeyIsPrivate`, `ErrSSHKeyWeak`, `ErrSSHKeyExists`, `ErrSSHKeyLimit`, `ErrSSHKeyName`.

### Restablecer la contraseña

- El formulario **He olvidado mi contraseña** pasa la comprobación anti-bots.
- Responde siempre `204`, exista o no la cuenta y aunque se haya pasado el límite de correos.
- El correo va a la dirección guardada en la cuenta, no a lo que se escribió en el formulario.
- El enlace se construye con la **URL principal** del panel (`panel.settings.masterUrl`), nunca con las cabeceras de la petición. Debe configurarse: si no, el enlace usa `http://localhost:8080` y el panel lo avisa en el log.
- El enlace caduca a los **30 minutos** y sirve **una vez**. Pedir otro anula el anterior. En la base de datos solo se guarda su hash.
- La contraseña nueva debe tener entre 8 y 72 bytes. Se comprueba antes de gastar el enlace, así que una contraseña demasiado corta se puede volver a escribir.
- Al cambiarla se cierran todas las sesiones de la cuenta y se envía un correo de aviso.

## Protecciones web

Cada respuesta del panel pasa por estos middlewares, en este orden: CORS, cabeceras de seguridad, protección de mismo origen y, por último, recuperación ante pánicos.

### Protección de mismo origen

Una petición que **lleva una cookie del panel** (`skypanel_auth` o `session`) y **cambia algo** (cualquier método salvo `GET`, `HEAD` y `OPTIONS`, y además cualquier apertura de websocket) debe venir de las páginas del propio panel. El panel lo comprueba así:

1. Si el navegador envió `Sec-Fetch-Site` (todos los navegadores actuales lo hacen y las páginas no pueden falsificarlo), debe ser `same-origin` o `none`.
2. Si no, y hay cabecera `Origin`, su host debe coincidir con el `Host` de la petición, con la cabecera `X-Forwarded-Host` o con el host de la URL principal.
3. Una petición sin ninguna de las dos cabeceras no viene de una página web y se deja pasar.

Cualquier otra recibe `403 {"error": "cross-origin request refused"}`. Las peticiones sin cookies (tokens bearer, clientes OAuth2, nodos, claves API de facturación) no se ven afectadas.

Esto cubre lo que `SameSite=Lax` no cubre por sí solo: para un navegador, `http://IP:25566` (el servidor de juego de un cliente) y `http://IP:8080` (el panel) son el **mismo sitio**, así que un formulario o un websocket abierto desde una página de un servidor de juego llevaría la cookie del administrador. Antes de esta protección, una página así podía crear una cuenta de administrador.

### Cabeceras de seguridad

| Cabecera | Valor | Para qué |
|----------|-------|----------|
| `Content-Security-Policy` | `frame-ancestors 'self'; object-src 'none'; base-uri 'self'` | Ningún otro sitio puede incrustar el panel (clickjacking). |
| `X-Frame-Options` | `SAMEORIGIN` | Lo mismo, para navegadores antiguos. |
| `X-Content-Type-Options` | `nosniff` | Los navegadores no adivinan el tipo de contenido. |
| `Referrer-Policy` | `same-origin` | Las URL completas (algunas llevan tokens, como los enlaces del mapa del mundo) no se filtran a otros sitios. |
| `Strict-Transport-Security` | `max-age=31536000` | Solo sobre HTTPS (o con `X-Forwarded-Proto: https`): una vez visto, el navegador usa siempre HTTPS durante un año. |

Las páginas que muestran contenido no fiable (páginas de extensiones, mapas del mundo) sustituyen la política por su propio sandbox.

### CORS y otros detalles

- CORS acepta cualquier origen con las cabeceras `Authorization`, `Content-Type`, `Accept` y `Origin`, pero **nunca con credenciales**: otros sitios pueden llamar a la API con un token bearer, nunca con la cookie del usuario.
- La documentación de la API en `/swagger/` solo se sirve a administradores con sesión iniciada.
- El log de accesos omite los tokens del mapa del mundo y los valores `token`, `code`, `access_token` y `refresh_token` de la URL.
- Cuando el panel llama a un nodo, quita la cabecera `Cookie` del navegador: los nodos se autentican con el token propio del panel.
- La página a la que se vuelve tras iniciar sesión no se toma de la barra de direcciones, así que un enlace manipulado no puede llevar al usuario a otro sitio después del inicio de sesión.

## Permisos

El modelo completo está en la [Referencia de scopes](./scopes-reference.md). Las reglas más importantes para la seguridad:

- `admin` lo permite todo. Debe darse solo a quienes administran el panel.
- La fila de permisos de un servidor solo guarda **scopes de servidor**: un scope del panel como `admin` se descarta al guardar y se ignora al leer. Ver [Subusuarios](./scopes-reference.md#subusuarios).
- Los propietarios no pueden cambiar la memoria, la CPU, el disco, la IP ni los puertos de su servidor, aunque la plantilla abra esas variables. Ver [Variables que solo cambia un administrador](./scopes-reference.md#variables-que-solo-cambia-un-administrador).
- El personal con `users.info.edit` o `users.perms.edit` no puede editar, borrar ni cambiar los permisos de cuentas que tienen poderes que él no tiene. Ver [Editar otras cuentas](./scopes-reference.md#editar-otras-cuentas).
- Con el registro cerrado, solo los administradores pueden invitar a un servidor a alguien sin cuenta.
- Las claves API de facturación solo hacen las acciones que se les dieron y no pueden abrir enlaces de acceso a cuentas con poderes en el panel. Ver [Claves API](./scopes-reference.md#claves-api).
- Una sesión de un cliente OAuth2 queda limitada por el cliente también en las comprobaciones internas de cada ruta, y el socket de la consola concede sus flujos con esos mismos scopes. Ver [De dónde salen los permisos](./scopes-reference.md#de-dónde-salen-los-permisos).
- La contraseña de una base de datos de servidor solo se devuelve a quien tiene `server.data.edit` sobre él. Ver [Hosts de bases de datos](../features/database-hosts.md).

## Contenedores de los servidores de juego

Los servidores se crean solo en Docker. La instalación Docker del panel activa `docker.disallowHost` (`SKYPANEL_DOCKER_DISALLOWHOST=true` en `docker-compose.yml` y en la imagen), que desactiva los entornos que ejecutan procesos directamente en el nodo (`host`, `tty`, `standard`, `bubblewrap`). Se comprueba al crear o redefinir un servidor, y también en cada instalación y cada arranque, así que una definición que llegó al nodo por otro camino (una copia restaurada, un panel antiguo) tampoco se ejecuta fuera de un contenedor.

Cada contenedor de un servidor de juego se endurece así:

| Protección | Detalle | Ajuste |
|------------|---------|--------|
| Usuario sin privilegios | Cuando el panel se ejecuta como root (la instalación Docker lo hace), el juego se ejecuta con uid:gid `988:988`. Las instalaciones siguen ejecutándose como root, porque sus scripts instalan paquetes. Una plantilla que indica su propio usuario lo conserva. Antes de cada arranque el panel da los archivos del servidor a ese usuario. | `docker.serverUser` (`988:988`; `0:0` mantiene root) |
| Sin usuario de contenedor elegido por clientes | La clave `User` de la configuración del contenedor se elimina cuando alguien que no es administrador edita la definición o crea un sub-servidor. | — |
| Capacidades eliminadas | `setpcap`, `mknod`, `audit_write`, `net_raw`, `dac_override`, `fowner`, `fsetid`, `net_bind_service`, `sys_chroot`, `setfcap`. Un contenedor que se ejecuta como root (una instalación) conserva las que necesitan la instalación de paquetes y solo quita `setpcap`, `mknod`, `audit_write`, `net_raw` y `sys_chroot`. Las listas propias de la plantilla se respetan. | — |
| Sin ganar privilegios | `no-new-privileges`: los binarios setuid dentro del contenedor no elevan privilegios. | — |
| Límite de procesos | Procesos e hilos por contenedor, para que una fork bomb en un plugin no tumbe la máquina. El límite propio de una plantilla tiene prioridad. | `docker.pidsLimit` (`4096`; `0` lo quita) |
| `/tmp` propio | `/tmp` y `/var/tmp` son una carpeta junto a la del servidor, que se vacía antes de arrancar cada contenedor y cuenta en su límite de disco. | — |
| Red propia | Los servidores de juego van a la red `skypanel-servers`. Llegan a MariaDB (que está en las dos redes), pero no a la red del panel. | `docker.serversNetwork` |
| Sin descargas desde redes privadas | Las descargas de las plantillas rechazan siempre las direcciones de loopback, link-local (metadatos de la nube) y sin especificar, y por defecto también las privadas (las redes de Docker, la LAN). Algunas plantillas toman una URL de una variable que edita el propietario. Se aplica también a los archivos de las versiones de GitHub (`githubdl`), cuya dirección viene de la versión publicada. | `security.blockPrivateDownloads` (`true`) |

Otros límites que protegen el nodo:

- `panel.backups.maxPerServer` (`10`): copias de seguridad que pueden tener en el nodo los usuarios de un servidor (las que solo están en el bucket no cuentan); los administradores no tienen límite. Las copias no cuentan en el disco del servidor.
- `daemon.minFreeDiskMB` (`2048`): se rechaza una instalación que empezaría con menos espacio libre en la carpeta de servidores, antes de que una descarga del juego llene el disco.
- El límite de disco del servidor se aplica; un servidor que ya lo llenó no arranca hasta que se borren archivos.

## Archivos de los servidores

Todas las operaciones con archivos (gestor de archivos, SFTP, archivos comprimidos, copias de seguridad, instalaciones) trabajan dentro de la carpeta del servidor y nunca siguen un enlace hacia fuera. El funcionamiento está en [Archivos y compresión](19-files-compression.md) y en [openat2 y unshare](20-security-unshare.md). Endurecimiento reciente:

- **Extraer**: los enlaces de un archivo comprimido se crean desde su carpeta padre, abierta dentro de la raíz. Los enlaces que apuntan fuera de la carpeta se omiten (los archivos de los juegos suelen tener enlaces absolutos, como `steamclient.so`) en lugar de hacer fallar todo el archivo.
- **Comprimir y copias de seguridad**: cada archivo se lista y se abre dentro de la carpeta del servidor. Una carpeta con comodines en el nombre o cambiada por un enlace no puede meter archivos del host en el comprimido. Los enlaces se guardan como enlaces; las tuberías con nombre se omiten.
- **Restaurar una copia**: se lee todo el archivo y se comprueba cada entrada antes de borrar ningún archivo del servidor. Una copia con una ruta que sale de la carpeta se rechaza.
- **Descargas de instalación** (Mojang, Paper, Forge, Fabric, NeoForge, Sponge, CurseForge, GitHub, URL genéricas): se escriben a través del servidor de archivos y nunca a través de un enlace.
- **Cambio de propietario antes de arrancar**: recorre la carpeta por descriptor de archivo, así que una carpeta cambiada por un enlace a `/` no se recorre.
- **Definiciones de servidor**: deben ser archivos normales; una tubería con nombre ya no bloquea el gestor de archivos ni el historial de archivos.
- **Clonar** un servidor lee los enlaces dentro de la carpeta de origen.

## Tokens y secretos

| Qué | Cómo se protege |
|-----|-----------------|
| Tokens de sesión | UUID v4 aleatorio; solo se guarda el hash SHA-256. |
| Peticiones panel ↔ nodo | JWT firmados con Ed25519 (`panel.token`, generado en el primer arranque y guardado en la configuración). Los tokens de petición duran 15 minutos y van ligados al nodo al que se envían. La clave pública está en `/auth/publickey` (JWKS). |
| Credenciales de los nodos | Cada nodo remoto tiene un ID de cliente `.node_<id>` y un secreto. `GET /api/nodes/:id/deployment` los muestra y pide `nodes.deploy`. El secreto del nodo local se deriva de `panel.sessionKey`. |
| Clave de la cookie de sesión | `panel.sessionKey`, generada en el primer arranque. |
| Clave de transferencias federadas | `panel.exTransferKey`, una semilla Ed25519 generada una vez y guardada en la configuración. |
| Enlaces de restablecimiento y códigos de recuperación | Solo se guarda su hash. |
| Claves API de facturación | Se muestran una vez (`ak_...`); solo se guarda el hash. |
| Secretos de la configuración (claves de correo, Gemini, CurseForge, S3, Cloudflare, Turnstile y licencia, webhooks de Discord) | Se guardan en el archivo de configuración. La API de ajustes nunca los devuelve: responde `********`. Ver [Configuración del panel](../features/panel-settings.md#cómo-funciona). |
| Secreto TOTP de la 2FA | Se guarda sin cifrar en la base de datos (véase [Limitaciones conocidas](#limitaciones-conocidas)). |

El archivo de configuración (`/etc/SkyPanel/config.json` dentro del contenedor, `storage/skypanel-config` en el host) guarda estas claves: debe ser legible solo por root, igual que el `/etc/SkyPanel/credenciales.txt` del instalador, y conviene incluirlo en las copias de seguridad.

## Firewall y puertos

| Puerto | Servicio | ¿Abierto a Internet? |
|--------|----------|----------------------|
| 80, 443 | Nginx (HTTP, y HTTPS con dominio) | Sí, cuando se usa Nginx. |
| 8080 | Panel | Solo sin dominio. Con dominio el instalador lo publica en `127.0.0.1`. |
| 5657 | SFTP | Sí, para los usuarios que necesiten SFTP. |
| Puertos de los servidores de juego | Servidores de juego | Docker los publica él mismo, por delante de ufw/firewalld. |
| 3307 | MariaDB (`127.0.0.1:3307` → 3306) | No. Publicado en `127.0.0.1` (`DB_BIND_ADDRESS`). |
| 8081 | Adminer (`127.0.0.1:8081`) | No. Use un túnel SSH: `ssh -L 8081:127.0.0.1:8081 usuario@servidor`. |

El instalador abre SSH, SFTP, 80/443 y el puerto del panel (solo cuando no está publicado en `127.0.0.1`) con ufw o firewalld. Hay que tener en cuenta que **los puertos que publica Docker se saltan ufw**: para limitar un puerto publicado, publíquelo en una dirección concreta (como hace el instalador con `--bind`) o añada reglas a la cadena `DOCKER-USER` de iptables.

## Buenas prácticas

- Usar un dominio con HTTPS en cualquier panel con clientes, y configurar la URL principal.
- Activar la 2FA en todos los administradores y guardar los códigos de recuperación fuera de línea. Con la 2FA activada, usar claves SSH para el SFTP (funcionan en todos los nodos).
- Dar `admin` solo a quienes administran el panel. Usar roles para el personal, con los mínimos scopes necesarios.
- Dejar `docker.disallowHost`, `docker.serverUser`, `docker.pidsLimit` y `security.blockPrivateDownloads` con sus valores por defecto salvo que se sepa por qué se cambian.
- Cerrar el registro público (`panel.registrationEnabled`, en **Configuración**) si los clientes llegan desde la facturación.
- Con el registro abierto, dejar activado **Aprobar cuentas nuevas** (`panel.registrationApproval`): una cuenta registrada no puede entrar hasta que un administrador la aprueba en **Usuarios**. Así nadie usa una cuenta con el email de otra persona sin que un administrador la revise.
- Mantener el panel actualizado (**Configuración → Actualizaciones**, o `install.sh --update`), y también el sistema operativo y Docker.
- Hacer copias de la base de datos, de la carpeta de configuración y de la carpeta de copias de seguridad, y probar a restaurarlas.
- No exponer MariaDB ni Adminer a Internet.
- Revisar la actividad del panel (inicios de sesión, fallidos, cierres de sesión, registros, cambios de contraseña y accesos directos desde la facturación, con IP y navegador; se guardan 90 días) y la actividad de cada servidor.

## Limitaciones conocidas

- **SFTP y 2FA**: los clientes SFTP no pueden pedir un código, así que el inicio de sesión por SFTP con contraseña no pide la 2FA, y el acceso SFTP con contraseña no se puede desactivar. Quien tenga la 2FA activada debería entrar por SFTP con una [clave SSH](#claves-ssh-para-sftp), que funciona en todos los nodos con la versión actual.
- **Secreto TOTP sin cifrar**: el secreto de la 2FA se guarda en claro en la base de datos (`users.otp_secret`). Quien lea la base de datos puede generar códigos: proteja la base de datos y sus copias.

## Si sospecha que el panel está comprometido

1. Desconecte el servidor de la red o detenga el panel (`docker compose stop` en `/opt/skypanel`).
2. Cambie las contraseñas de los administradores y de la base de datos; revoque las claves API de facturación y los clientes OAuth2.
3. Rote `panel.sessionKey` y `panel.token` borrándolas de la configuración y reiniciando: se generan claves nuevas al arrancar. Los nodos remotos leen la clave pública del panel de `token.public` (`/auth/publickey`); compruebe después que siguen aceptando al panel y reinícielos si no.
4. Revise usuarios, roles y permisos globales en busca de algo que no haya concedido; revise los propietarios de los servidores.
5. Revise la actividad y los logs (`docker compose logs skypanel` y la carpeta de logs del panel).
6. Restaure desde una copia sana si se modificaron archivos, actualice el panel e informe a los usuarios afectados.
