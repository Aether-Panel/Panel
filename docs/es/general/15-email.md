# Correo electrónico

Aether Panel envía correos transaccionales (cuenta creada, restablecimiento de contraseña, añadido a un servidor, …) a partir de plantillas HTML mediante uno de cinco proveedores: SMTP, Mailgun, Mailjet, SendGrid o un proveedor de depuración que solo escribe en el log.

El correo es opcional. Sin proveedor configurado, cada envío devuelve `email not configured` y la acción que lo provocó sigue adelante (el error solo se registra en el log).

---

## Configuración

Claves de `internal/config/entries.go` (todas con valor por defecto `""`). Las variables de entorno siguen la regla habitual (`SKYPANEL_` + clave en mayúsculas, con `_` en lugar de puntos), ver [Configuración](../backend/02-configuration.md).

| Clave | Variable de entorno | La usa |
|-------|---------------------|--------|
| `panel.email.provider` | `SKYPANEL_PANEL_EMAIL_PROVIDER` | Todos. `smtp`, `mailgun`, `mailjet`, `sendgrid` o `debug` (sin distinguir mayúsculas) |
| `panel.email.from` | `SKYPANEL_PANEL_EMAIL_FROM` | SMTP, Mailgun, Mailjet, SendGrid (obligatoria) |
| `panel.email.host` | `SKYPANEL_PANEL_EMAIL_HOST` | SMTP (obligatoria) |
| `panel.email.username` | `SKYPANEL_PANEL_EMAIL_USERNAME` | SMTP |
| `panel.email.password` | `SKYPANEL_PANEL_EMAIL_PASSWORD` | SMTP |
| `panel.email.domain` | `SKYPANEL_PANEL_EMAIL_DOMAIN` | Mailgun (dominio de envío), Mailjet (ver más abajo) |
| `panel.email.key` | `SKYPANEL_PANEL_EMAIL_KEY` | Mailgun, Mailjet, SendGrid (obligatoria) |
| `panel.email.templateFolder` | `SKYPANEL_PANEL_EMAIL_TEMPLATEFOLDER` | Carpeta con plantillas propias (opcional, se lee al arrancar) |

Todas las claves salvo `templateFolder` se pueden cambiar desde **Configuración → Correo** o la API de ajustes (`settings.edit`). La pestaña Correo ofrece los cinco proveedores: SMTP, Mailgun, Mailjet, SendGrid y **Depuración (solo registro)**, que no muestra campos.

Si falta un valor obligatorio, el envío falla con `<clave> is not configured` (código de error `ErrSettingNotConfigured`).

---

## Proveedores (`internal/email/`)

Cada proveedor se registra por nombre en una función `init()`; `email.GetProvider(nombre)` busca el nombre en minúsculas. Todos los proveedores envían el cuerpo como HTML.

### SMTP (`smtp.go`)

Biblioteca: `github.com/wneessen/go-mail`.

```json
{
  "panel": {
    "email": {
      "provider": "smtp",
      "host": "smtp.example.com",
      "username": "no-reply@example.com",
      "password": "contraseña-de-aplicación",
      "from": "no-reply@example.com"
    }
  }
}
```

- `host` es solo el nombre del host. No hay ajuste de puerto: el cliente se crea con `mail.WithSSLPort(true)`, que conecta con TLS implícito en el puerto **465** y deja que go-mail recurra a su puerto sin SSL si falla. Los servidores que solo aceptan el puerto 587 (STARTTLS) no se pueden configurar con ningún ajuste.
- Autenticación: `SMTPAuthAutoDiscover` (el método más fuerte que anuncia el servidor).
- A `username` se le quitan los espacios de los extremos; a `password` se le **quitan todos los espacios** (así se pueden pegar contraseñas de aplicación tipo Gmail con espacios).
- Cada mensaje lleva un `Message-ID` aleatorio.

### Mailgun (`mailgun.go`)

Biblioteca: `mailgun-go/v4`. Requiere `domain` (el dominio de envío de Mailgun), `from` y `key` (clave de API). Usa la URL base por defecto de la biblioteca (región de EE. UU.).

### Mailjet (`mailjet.go`)

Biblioteca: `mailjet-apiv3-go/v4`, Send API v3.1. Requiere `domain`, `from` y `key`. El cliente se crea con `NewMailjetClient(domain, key)`, así que **`panel.email.domain` contiene la clave de API pública de Mailjet y `panel.email.key` la clave secreta**. En la pestaña Correo esos campos se llaman **Clave API (pública)** y **Clave secreta**.

### SendGrid (`sendgrid.go`)

Biblioteca: `sendgrid-go`. Requiere `from` y `key` (clave de API). En la pestaña Correo: **Dirección del remitente** y **Clave API**.

### Depuración (`debug.go`)

Escribe `DEBUG EMAIL TO <destino>`, el asunto y el cuerpo en el log de depuración y devuelve éxito. Solo para desarrollo.

---

## Plantillas

### Carga

`LoadEmailService()` se ejecuta una vez al arrancar el panel (`cmd/panel/run.go`):

1. Si `panel.email.templateFolder` tiene valor, crea un sistema de archivos combinado: primero la carpeta propia y, como respaldo, las plantillas integradas (`assets/email/`).
2. Lee `emails.json`, que asigna a cada nombre de plantilla un asunto y un archivo de cuerpo:

   ```json
   {
     "passwordReset": {
       "subject": "Your password has been reset",
       "body": "password-reset.html"
     }
   }
   ```

3. Analiza el asunto y el cuerpo con `html/template` de Go. Un archivo que falta o un error de sintaxis **detiene el panel al arrancar** (panic).

Como el `emails.json` completo se toma del primer sitio que lo tenga, un `emails.json` propio sustituye a la lista integrada: debe declarar todas las plantillas que usa el panel. Los archivos de cuerpo que falten en la carpeta propia se siguen leyendo de las integradas. Los cambios en las plantillas requieren reiniciar el panel.

### Variables

Cada envío añade tres variables:

| Variable | Origen |
|----------|--------|
| `COMPANY_NAME` | `panel.settings.companyName` (por defecto `SkyPanel`) |
| `MASTER_URL` | `panel.settings.masterUrl` (por defecto `http://localhost:8080`) |
| `HOST_NAME` | El nombre de host de `panel.settings.masterUrl` (por ejemplo `panel.example.com`); si no se puede extraer, `panel.settings.companyName`. Lo usa la plantilla `accountCreation` |

### Plantillas integradas

Los asuntos integrados están en inglés; se pueden cambiar con una carpeta de plantillas propia.

| Plantilla | Asunto | Se envía cuando | Variables adicionales |
|-----------|--------|-----------------|-----------------------|
| `accountCreation` | Your account has been created | Un usuario se registra (`/auth/register`) | — |
| `passwordReset` | Your password has been reset | Se solicita un restablecimiento de contraseña | `RESET_LINK` (`<panel>/reset-password/?token=…`) |
| `passwordChanged` | Your password has been changed | Se cambia la contraseña (perfil o restablecimiento) | — |
| `emailChanged` | Your email has been changed | Se cambia el correo; se envía a la dirección **anterior** | `NEW_EMAIL` |
| `otpEnabled` | OTP has been enabled for your account | Se activa la verificación en dos pasos | — |
| `otpDisabled` | OTP has been disabled for your account | Se desactiva la verificación en dos pasos | — |
| `recoveryCodesRegenerated` | OTP Recovery Codes have been regenerated for your account | Se regeneran los códigos de recuperación | — |
| `oauthCreated` | OAuth client created | El usuario crea un cliente OAuth | — |
| `oauthDeleted` | OAuth client deleted | El usuario borra un cliente OAuth | — |
| `addedToServer` | You have been added to a server | Se añade un usuario a un servidor, se crea un servidor para él o se aprovisiona | `Server` (`.Server.Name`, `.Server.Identifier`), `RegisterToken`, `Email` |
| `removedFromServer` | You have been removed from a server | Se quita a un usuario de un servidor | `Server` |
| `deletedServer` | A server you have access to has been deleted | Se borra un servidor; se envía a los usuarios que tenían acceso a él | `Server` |
| `login` | Someone has logged into your account | Ningún código la envía | — |
| `test` | Email Test | Botón de prueba / endpoint de prueba | — |

`addedToServer`: si el usuario no existía y el panel le creó la cuenta (`RegisterToken` no vacío), el correo le pide que elija contraseña en `<MASTER_URL>/forgot-password/` con su dirección; si no, muestra el enlace de inicio de sesión.

Todos los correos se envían de forma **asíncrona** (los errores solo van al log), salvo el de prueba, que se envía de forma síncrona para poder mostrar el error.

---

## Correo de prueba: `POST /api/settings/test/email`

Permiso: `settings.edit`. Lo llama el botón **Email de prueba** de **Configuración → Correo**.

- El correo (plantilla `test`) va a la **dirección del propio usuario conectado**; no hay campo `to`.
- El cuerpo puede incluir valores sin guardar para probarlos antes de guardar. Claves reconocidas: `panel.email.provider`, `panel.email.from`, `panel.email.host`, `panel.email.username`, `panel.email.password`, `panel.email.domain`, `panel.email.key`. Los valores no vacíos se aplican solo durante la prueba y después se restauran los anteriores. `panel.email.password` o `panel.email.key` con el valor `********` (lo que la página recibe en lugar del secreto guardado) se ignoran y se usa el valor guardado.

```bash
curl -X POST https://panel.example.com/api/settings/test/email \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"panel.email.provider": "smtp", "panel.email.host": "smtp.example.com"}'
```

| Estado | Significado |
|--------|-------------|
| `204` | Enviado |
| `422` | `the test email could not be sent: <motivo>` (el error del proveedor, p. ej. contraseña incorrecta o fallo de TLS) |

---

## Solución de problemas

| Síntoma | Causa |
|---------|-------|
| `email not configured` | `panel.email.provider` está vacío |
| `panel.email.… is not configured` | Falta una clave obligatoria del proveedor elegido |
| `email does not support <nombre>` | `panel.email.provider` no es ninguno de los cinco nombres |
| Tiempo de espera agotado o error TLS con SMTP | El servidor no ofrece TLS implícito en el puerto 465 |
| `401` en Mailjet | `domain`/`key` no son el par de claves de API pública/secreta de Mailjet |
| El panel no arranca tras editar plantillas | Falta un archivo de cuerpo listado en `emails.json` o una plantilla tiene un error de sintaxis |
| No llega nada y la interfaz no muestra error | Los correos asíncronos solo registran los fallos; revise el log del panel |

En producción, verifique la dirección o el dominio `from` con el proveedor y configure SPF/DKIM/DMARC para él.
