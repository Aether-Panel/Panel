# Seguridad y Mejores Prácticas

## Introducción a la Seguridad

La seguridad es una prioridad fundamental en Aether Panel. Este documento cubre las características de seguridad integradas en el panel y las mejores prácticas para mantener tu instalación segura.

Aether Panel incluye múltiples capas de seguridad: autenticación por tokens Bearer y cookies HttpOnly, sesiones con hash SHA256, cifrado bcrypt para contraseñas, JWT con firma Ed25519, un sistema de 74 scopes granulares, y soporte para 2FA vía TOTP. El panel no maneja TLS directamente — debe usarse un proxy reverso como Nginx o Caddy para HTTPS.

## Autenticación

El panel implementa un sistema de autenticación multicapa. Los tokens se envían como Bearer en el header Authorization, o alternativamente en la cookie `skypanel_auth`. Las sesiones se almacenan en la base de datos con el token hasheado en SHA256.

### Métodos de Autenticación

#### Sesiones Web

El panel usa sesiones basadas en cookies con Gin sessions. Al iniciar sesión, se genera un UUID v4 como token de sesión. El token se hashea con SHA256 antes de almacenarse en la base de datos. La cookie `skypanel_auth` es configurable: path, domain, maxAge, Secure, HttpOnly y SameSite. La sesión expira por defecto en 1 hora.

#### Tokens Bearer (API)

Para acceso mediante API, el panel prioriza el header `Authorization: Bearer <token>`. Si no hay header, busca la cookie `skypanel_auth`. El token se valida consultando la base de datos, verificando que exista y que `expiration_time` sea futura.

#### OAuth2 Server Integrado

El panel incluye un servidor OAuth2. El endpoint está en `/oauth2/token` (generar token). Usa los flujos Client Credentials (autenticación panel↔nodo) y Password (autenticación SFTP). Las claves públicas están disponibles en `/auth/publickey` en formato JWKS. Los tokens son JWT firmados con Ed25519 (EdDSA).

- Autenticación Client Credentials para integraciones
- Tokens JWT con firma Ed25519 (clave autogenerada en el primer inicio)
- Endpoint JWKS para verificación de firmas por terceros
- Scopes granulares por cliente OAuth2
- Revocación de tokens y regeneración de secrets

#### Autenticación de Dos Factores (2FA/TOTP)

Aether Panel soporta TOTP (Time-based One-Time Password) para autenticación de dos factores. Cómo se activa, los códigos de recuperación y la API están en [Verificación en dos pasos (2FA)](#verificación-en-dos-pasos-2fa).

### Seguridad de Contraseñas

Las contraseñas en Aether Panel están protegidas con:

- Hashing con bcrypt (costo por defecto: 10) — nunca se almacenan en texto plano
- Cada llamada a SetPassword() genera un nuevo hash con salt automático
- Las contraseñas deben tener entre 8 y 72 bytes (el límite de bcrypt)

#### Recomendaciones para Contraseñas

- Mínimo 12 caracteres (recomendado 16+)
- Combinación de mayúsculas, minúsculas, números y símbolos
- No reutilices contraseñas de otros servicios
- Usa un gestor de contraseñas (Bitwarden, 1Password, etc.)
- Cambia las contraseñas regularmente, especialmente las de administrador

## Inicio de sesión y cuenta

Los formularios de inicio de sesión, registro y cambio de contraseña pasan una comprobación anti-bots. Está en [Captcha / anti-bots](17-turnstile.md).

### Límite de intentos

| Qué | Límite | Respuesta al pasarlo |
|-----|--------|----------------------|
| Contraseña (panel y SFTP comparten contador) | 10 fallos en 15 minutos por cuenta | `429` `ErrTooManyAttempts` |
| Código 2FA | 10 fallos en 15 minutos por cuenta | `429` `ErrTooManyAttempts` |
| Correos de cambio de contraseña | 5 por dirección y hora | `204`, sin enviar correo |

El contador va por cuenta (correo), no por IP, así que funciona igual detrás de un proxy. Vive en memoria: se pone a cero al reiniciar el panel. Un inicio de sesión correcto lo pone a cero para esa cuenta.

### Verificación en dos pasos (2FA)

Códigos TOTP de 6 dígitos, compatibles con Google Authenticator, Authy, Microsoft Authenticator y similares.

**Activarla**: **Configuración de la cuenta → Autenticación de dos pasos → Habilitar 2FA**. Escanea el código QR y escribe el código que da la aplicación. Hasta que no lo confirmas, la 2FA no se activa.

Al confirmarla, el panel muestra **10 códigos de recuperación** de 12 caracteres. Guárdalos: el panel solo guarda su hash y no los puede volver a mostrar. Cada uno sirve **una vez** en lugar del código de la aplicación.

| Acción | Qué pide | Qué pasa |
|--------|----------|----------|
| Regenerar códigos | Un código de la aplicación o de recuperación | Los anteriores dejan de funcionar. |
| Desactivar | Un código de la aplicación o de recuperación | Se borran la clave y los códigos. |
| Cambiar de teléfono | Desactívala y vuelve a activarla | No se puede activar otra encima de una activa (`ErrOtpAlreadyEnabled`). |

Si el correo está configurado, el panel avisa al activarla, al desactivarla y al regenerar los códigos.

**Si se pierden el teléfono y los códigos**, un administrador la quita desde el servidor con `SkyPanel user edit` → **Remove 2FA** (ver [CLI](../backend/07-cli.md)).

**Al iniciar sesión**, `POST /auth/login` responde `{"otpNeeded": true}` y el código se envía a `POST /auth/otp` en la misma sesión del navegador, antes de **5 minutos**. Pasado ese tiempo hay que volver a escribir la contraseña (`ErrSessionExpired`).

```bash
# 1. Contraseña (más el token anti-bots): responde {"otpNeeded": true}
curl -c cookies.txt -X POST https://panel.example.com/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email": "ana@example.com", "password": "<tu-contraseña>", "captchaToken": "<token>"}'

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

El SFTP no pide el código 2FA: entra con la contraseña o con una [clave SSH](#claves-ssh-para-sftp).

### Quitar el permiso de entrar

El panel comprueba el permiso `login` en **cada petición**, no solo al iniciar sesión. Cuenta el `login` de los permisos generales del usuario o el de su rol; el de un servidor concreto no cuenta.

Si se lo quitas a un usuario (o le quitas el rol que se lo daba), su sesión se cierra en la siguiente petición: el panel la borra y responde `401`.

Otras situaciones que cierran sesiones:

| Qué | Sesiones que se cierran |
|-----|-------------------------|
| El usuario cambia su contraseña en su perfil | Todas menos la actual |
| Un administrador le pone una contraseña nueva | Todas |
| Restablece la contraseña con el enlace del correo | Todas |

Quitar `login` también cierra el SFTP: el panel lo comprueba en cada conexión, con contraseña o con clave SSH, además de `server.sftp`.

### Claves SSH para SFTP

Un usuario puede entrar por SFTP con una clave pública en lugar de su contraseña. El usuario sigue siendo `correo#idServidor`.

**Añadir una clave**: **Configuración de la cuenta → Claves SSH → Añadir clave**. Pega la línea de tu archivo `.pub` (por ejemplo `~/.ssh/id_ed25519.pub`). Si no le pones nombre, se usa el comentario de la clave.

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

El usuario necesita igualmente `server.sftp` en ese servidor (o ser su dueño). Borrar una clave impide las conexiones nuevas con ella; borrar el usuario borra sus claves.

Las claves solo sirven en el nodo donde corre el panel. Los nodos remotos comprueban el SFTP contra el panel con usuario y contraseña, así que en sus servidores hay que entrar con la contraseña.

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
- El enlace se construye con la **URL principal** del panel (`panel.settings.masterUrl`), nunca con las cabeceras de la petición. Configúrala: si no, el enlace usa `http://localhost:8080` y el panel lo avisa en el log.
- El enlace caduca a los **30 minutos** y sirve **una vez**. Pedir otro anula el anterior. En la base de datos solo se guarda su hash.
- La contraseña nueva debe tener entre 8 y 72 bytes. Se comprueba antes de gastar el enlace, así que una contraseña demasiado corta se puede volver a escribir.
- Al cambiarla se cierran todas las sesiones de la cuenta y se envía un correo de aviso.

## Autorización y Permisos

El panel utiliza un sistema de permisos basado en 74 scopes granulares. Cada ruta de la API requiere uno o más scopes específicos. El middleware `RequiresPermission` verifica los scopes antes de procesar cada petición.

### Sistema de Scopes

Los scopes controlan el acceso a funcionalidades específicas. Ejemplos de los 74 scopes definidos (50 específicos de servidor + 24 globales):

- admin — Acceso administrativo completo (hereda todos los scopes)
- server.view — Ver información de servidores
- server.start / server.stop / server.kill — Control de estado
- server.console / server.console.send — Acceso a consola
- server.files.view / server.files.edit — Gestión de archivos
- server.backup.create / server.backup.restore — Backups
- server.sftp — Acceso SFTP
- nodes.view / nodes.create / nodes.edit / nodes.delete — Gestión de nodos
- users.info.view / users.info.edit — Gestión de usuarios

#### Jerarquía de Scopes

El scope `admin` concede todos los permisos globales. El scope `server.admin` concede todos los permisos sobre un servidor específico. Los scopes marcados como `forServer` requieren un ID de servidor en la ruta. Los usuarios pueden tener permisos globales (afectan toda la instancia) y permisos por servidor.

### Roles y Grupos

Los roles agrupan múltiples scopes para asignación masiva a usuarios. La verificación de permisos combina: (1) los scopes del rol del usuario, (2) los scopes globales del usuario, y (3) los scopes por servidor.

- Crea roles específicos para diferentes tipos de usuarios
- Revisa regularmente los permisos asignados a cada rol
- No otorgues el scope `admin` a menos que sea absolutamente necesario

## Seguridad de Tokens

El panel maneja varios tipos de tokens con diferentes niveles de seguridad.

### Tokens de Sesión

UUID v4 aleatorio  SHA256 hash  almacenado en BD. El token original nunca se persiste, solo su hash. La sesión expira en 1 hora y se valida en cada petición.

### Tokens JWT (OAuth2/Daemon)

Firmados con Ed25519 (EdDSA). La clave privada se genera aleatoriamente en el primer inicio y se almacena en la configuración. La clave pública se expone via JWKS. El key ID es 'SkyPanel'.

### Secretos de Nodos

Cada nodo tiene un secreto único generado como UUID sin guiones. El secreto se almacena en la base de datos y se compara directamente para autenticar la comunicación nodo-panel. Los nodos locales usan una comparación directa con el modelo LocalNode.

## Cadena de Middleware

Cada petición a la API pasa por una cadena de middleware que garantiza la seguridad:

- CORS — Permite todas las origins, métodos GET/POST/PUT/DELETE/OPTIONS, headers Authorization/Content-Type/Accept/Origin
- Recovery — Captura panics y retorna 500 sin exponer información interna
- ResponseAndRecover — Manejo de errores con respuestas JSON estructuradas
- NeedsDatabase — Verifica conexión a BD antes de procesar
- AuthMiddleware — Extrae token del header Authorization o cookie skypanel_auth, valida contra BD
- RequiresPermission — Verifica que el usuario tenga el scope necesario (retorna 403 si no)
- ResolveServerPanel — Para rutas con :serverId, carga el servidor desde BD
- HasTransaction — Envuelve la operación en una transacción de BD
- AddVersionHeader — Agrega headers de versión a la respuesta

## Mejores Prácticas de Seguridad

Sigue estas recomendaciones para mantener tu instalación segura:

### Proxy Reverso y HTTPS

- El panel NO maneja TLS internamente. Usa Nginx, Caddy o Traefik como proxy reverso con HTTPS.
- Configura `security_trusted_proxies` en config.json para que el panel confíe en las IPs de tu proxy.
- Configura `security_trusted_proxy_header` si tu proxy usa un header personalizado.
- Renueva automáticamente certificados Let's Encrypt con Certbot.
- Considera Cloudflare Proxy para protección DDoS adicional.

### Configuración de Cookies

- Configura `panel_web_cookies_samesite` como 'Strict' para evitar CSRF.
- Habilita `panel_web_cookies_secure` solo si usas HTTPS (requerido).
- Configura `panel_web_cookies_httponly` como true (por defecto).
- Ajusta `panel_web_cookies_age` según tu política de sesión (por defecto 1 hora).
- Configura `panel_web_cookies_path` y `panel_web_cookies_domain` según tu despliegue.

### Actualizaciones y Mantenimiento

- Mantén el panel actualizado a la última versión para tener los últimos parches de seguridad.
- Actualiza el sistema operativo y Docker regularmente.
- Revisa los logs del panel en busca de actividad sospechosa.
- Mantén backups actualizados de la base de datos y configuración.

### Contraseñas y Autenticación

- Usa contraseñas seguras (12+ caracteres con variedad de tipos).
- Habilita 2FA/TOTP para todos los administradores.
- Rota las contraseñas regularmente, especialmente para cuentas administrativas.
- Cada usuario debe tener su propia cuenta — no compartas credenciales.

### Permisos y Scopes

- Aplica el principio de menor privilegio: solo otorga los scopes necesarios.
- Revisa y audita los permisos de usuario periódicamente.
- Usa roles en lugar de asignar scopes individualmente a cada usuario.
- Desactiva cuentas de usuarios que ya no necesitan acceso.

### Red y Firewall

- Configura el firewall para abrir solo los puertos necesarios (8080, 5657, 8081).
- Usa fail2ban para proteger contra ataques de fuerza bruta en SSH.
- Considera una VPN para acceso administrativo remoto.
- Cambia el puerto SSH del 22 por defecto.

### Nodos y Daemon

- Mantén los secretos de nodos seguros — son la llave de acceso al daemon.
- Si usas nodos remotos, asegura la comunicación via VPN o túnel SSH.
- Revisa que los nodos solo expongan los puertos necesarios.
- Configura los nodos para que acepten conexiones solo desde el panel.


## Configuración del Firewall

Asegúrate de que el firewall de tu servidor esté configurado correctamente:

### Puertos Necesarios

Puerto predeterminado del panel web (HTTP). Requiere proxy reverso para HTTPS.

- **port**: 8080/TCP
- **restrict**: Restringe a IPs específicas o usa Cloudflare Proxy si es posible.
Puerto SFTP para transferencia de archivos.

- **port**: 5657/TCP
- **restrict**: Solo accesible para usuarios que necesitan SFTP.
Puerto para Gatus (monitoreo de uptime). Opcional.

- **port**: 8081/TCP
- **restrict**: Puede restringirse a acceso interno.

### Configuración con UFW (Ubuntu/Debian)

Para configurar el firewall con UFW:

```text
# Permitir puertos necesarios
sudo ufw allow 8080/tcp
sudo ufw allow 5657/tcp
sudo ufw allow 8081/tcp

# Habilitar firewall
sudo ufw enable

# Verificar estado
sudo ufw status
```

### Configuración con firewalld (Fedora/RHEL/CentOS)

Para configurar el firewall con firewalld:

```text
# Permitir puertos necesarios
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=5657/tcp
sudo firewall-cmd --permanent --add-port=8081/tcp

# Aplicar cambios
sudo firewall-cmd --reload

# Verificar reglas
sudo firewall-cmd --list-ports
```

## Proxy Reverso y SSL/TLS

Aether Panel escucha en HTTP plano. Para HTTPS debes usar un proxy reverso (Nginx, Caddy, Traefik). Configura `security_trusted_proxies` para que el panel registre las IPs reales de los clientes.

### Certificados con Let's Encrypt (vía proxy reverso)

Configura un proxy reverso Nginx con Certbot para SSL automático:

Los certificados de Let's Encrypt expiran cada 90 días. Certbot los renueva automáticamente.

- Instala Nginx: sudo apt install nginx (Ubuntu/Debian)
- Configura Nginx como proxy reverso hacia localhost:8080
- Instala Certbot: sudo apt install certbot python3-certbot-nginx
- Obtén certificado: sudo certbot --nginx -d tu-dominio.com
- Verifica renovación automática: sudo certbot renew --dry-run

### Certificados Auto-firmados (Solo Desarrollo)

Los certificados auto-firmados solo deben usarse en entornos de desarrollo.

- **warning**: Los navegadores mostrarán advertencias de seguridad con certificados auto-firmados.
## Monitoreo y Logs

El panel utiliza un sistema de logging estructurado con diferentes niveles (Info, Warn, Error, Debug). Los logs del panel se escriben al archivo y a stdout.

### Revisión de Logs

Revisa regularmente los logs del panel para detectar:

- Errores de autenticación y tokens inválidos
- Peticiones a rutas sin permisos suficientes (403)
- Errores de conexión a nodos o base de datos
- Actividad anormal de API o patrones sospechosos
- Panics recuperados por el middleware de recovery

#### Ubicación de Logs

- **native**: En instalaciones nativas: según la configuración del sistema de logging de Go
- **docker**: En Docker: docker compose logs -f skypanel
## Respuesta a Incidentes

Si sospechas que tu instalación ha sido comprometida:

- Desconecta el servidor de la red inmediatamente
- Cambia todas las contraseñas (panel, base de datos, SSH, etc.)
- Revisa los logs del panel buscando actividad sospechosa
- Revoca todos los tokens OAuth2 y regenera client secrets
- Regenera la clave privada del panel (ed25519) eliminándola de la configuración
- Revisa y elimina cualquier usuario o permiso sospechoso
- Restaura desde un backup conocido seguro si es necesario
- Actualiza el panel y todas las dependencias
- Notifica a los usuarios afectados si es apropiado

### Prevención

La mejor respuesta a incidentes es prevenirlos. Sigue todas las mejores prácticas, mantén el panel actualizado, monitorea los logs y aplica el principio de menor privilegio en los scopes.

## Checklist de Seguridad

Usa este checklist para asegurarte de que tu instalación está configurada de forma segura:

- Panel actualizado a la última versión
- Sistema operativo y Docker actualizados
- Proxy reverso configurado con HTTPS y certificado válido
- security_trusted_proxies configurado correctamente
- 2FA/TOTP habilitado para todos los administradores
- Firewall configurado (solo puertos 8080, 5657, 8081 abiertos)
- Contraseñas seguras para todas las cuentas (bcrypt)
- Permisos de usuario revisados y limitados al mínimo necesario
- Scopes auditados regularmente
- Logs siendo monitoreados (incluyendo errores de autenticación)
- Tokens OAuth2 y secrets de nodos protegidos
- Backups configurados y probados regularmente
- Cookie session configurada con SameSite=Strict y HttpOnly=true
- Clave privada Ed25519 del panel respaldada de forma segura

