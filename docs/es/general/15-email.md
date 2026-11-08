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

### Cómo se ven

Todos los correos son una **carta por avión**: papel claro envejecido con franjas rojas, blancas y azules arriba y abajo, el nombre de la empresa (`panel.settings.companyName`) con la fecha, un sello azul con la **referencia** del correo, el título, el texto a máquina, un recuadro con los datos (cuenta, servidor, fechas), un botón cuando hay algo que hacer (con el enlace escrito debajo por si el botón no funciona), la firma «el equipo de <empresa>» y un pie con el enlace al panel.

- Está hecho con tablas y estilos en línea, y solo usa letras que tienen todos los equipos (Courier New y Georgia), para que se vea igual en Gmail, Outlook y Apple Mail. En el móvil ocupa todo el ancho.
- **Idioma**: el de **Configuración → General → Idioma de las notificaciones** (`panel.settings.notificationLanguage`, `es` o `en`, español por defecto), el mismo de los avisos de Discord, Telegram y webhooks. Se lee en cada envío.
- **Referencia**: cada correo lleva una propia de 8 caracteres (`Q7M2-K9XD`, sin letras que se confundan como 0/O o 1/I/L). El log del panel la anota al enviar: `Sending email passwordReset (ref Q7M2-K9XD) to laura@example.com using smtp`. Si alguien cita la referencia, se encuentra el correo en el log.
- El asunto se escribe siempre en una sola línea, aunque el nombre de un servidor tenga saltos de línea.

### Archivos

En `assets/email/`:

| Archivo | Qué es |
|---------|--------|
| `layout.html` | La carta, común a todos los correos (`html/template`) |
| `messages.es.json`, `messages.en.json` | Los textos de cada correo en ese idioma |

Cada correo de `messages.<idioma>.json` tiene `subject`, `preheader` (la línea que el buzón muestra junto al asunto), `title`, `paragraphs`, `details` (pares `label`/`value`) y, si lo necesita, `action` (`label` y `url` del botón). `common` tiene los textos de todos (saludo, firma, pie…). Cada texto es una plantilla `text/template` de Go con las variables de abajo, por ejemplo `"Tienes acceso al servidor {{ .Server.Name }}"`; la carta escapa lo que sale. Un párrafo, dato o botón que queda vacío no se muestra: así un texto puede depender de una condición (`{{ if .RegisterToken }}…{{ end }}`) y un botón que necesita `BASE_URL` desaparece si no hay URL del panel.

### Plantillas propias

`LoadEmailService()` lee las plantillas una vez al arrancar el panel (`cmd/panel/run.go`); un cambio necesita reiniciar. Un archivo que falta o un error de sintaxis **detiene el panel al arrancar** (panic).

- **Carpeta con `layout.html` o `messages.<idioma>.json`** (`panel.email.templateFolder`): esos archivos sustituyen a los integrados y el resto se sigue leyendo de `assets/email/`. Por ejemplo, un `messages.es.json` propio cambia los textos en español y deja el diseño y el inglés como están. Un `messages.<idioma>.json` propio debe tener todos los correos: uno que le falte no se envía y el log lo anota.
- **Carpeta con `emails.json`** (el formato anterior): el panel usa esas plantillas tal cual, como antes, sin la carta ni el idioma. `emails.json` asigna a cada correo un asunto y un archivo HTML completo:

  ```json
  {
    "passwordReset": {
      "subject": "Reset your password",
      "body": "password-reset.html"
    }
  }
  ```

  Debe declarar todos los correos que envía el panel. Desde que existe la carta, `assets/email/` ya no trae los HTML del formato anterior: los archivos de cuerpo tienen que estar en la carpeta propia.

### Variables

Cada envío añade estas variables a las del correo:

| Variable | Origen |
|----------|--------|
| `COMPANY_NAME` | `panel.settings.companyName` (por defecto `SkyPanel`) |
| `MASTER_URL` | `panel.settings.masterUrl` tal cual (por defecto `http://localhost:8080`) |
| `BASE_URL` | `panel.settings.masterUrl` sin la barra final; la usan los enlaces de los botones |
| `HOST_NAME` | El nombre de host de `panel.settings.masterUrl` (por ejemplo `panel.example.com`); si no se puede extraer, `panel.settings.companyName` |
| `TO` | La dirección a la que va el correo |
| `DATE`, `TIME` | Fecha («8 de noviembre de 2026» / «8 November 2026») y hora con zona («17:30 CET») del envío |
| `REF` | La referencia del correo |
| `LANG` | `es` o `en` |
| `PROVIDER` | El proveedor de correo (`smtp`, `mailgun`…) |

### Correos

| Correo | Asunto (es) | Se envía cuando | Variables adicionales |
|--------|-------------|-----------------|-----------------------|
| `accountCreation` | Tu cuenta en <empresa> está lista | Un usuario se registra (`/auth/register`) o un administrador aprueba su cuenta | — |
| `passwordReset` | Restablecer tu contraseña | Se solicita un restablecimiento de contraseña (el enlace caduca en 30 minutos) | `RESET_LINK` (`<panel>/reset-password/?token=…`) |
| `passwordChanged` | Tu contraseña ha cambiado | Se cambia la contraseña (perfil o restablecimiento) | — |
| `emailChanged` | El correo de tu cuenta ha cambiado | Se cambia el correo; se envía a la dirección **anterior** | `NEW_EMAIL` |
| `otpEnabled` | Verificación en dos pasos activada | Se activa la verificación en dos pasos | — |
| `otpDisabled` | Verificación en dos pasos desactivada | Se desactiva la verificación en dos pasos | — |
| `recoveryCodesRegenerated` | Nuevos códigos de recuperación | Se regeneran los códigos de recuperación | — |
| `oauthCreated` | Nuevo cliente OAuth en tu cuenta | El usuario crea un cliente OAuth | — |
| `oauthDeleted` | Cliente OAuth eliminado | El usuario borra un cliente OAuth | — |
| `addedToServer` | Tienes acceso al servidor <nombre> | Se añade un usuario a un servidor, se crea un servidor para él o se aprovisiona | `Server` (`.Server.Name`, `.Server.Identifier`), `RegisterToken`, `Email` |
| `removedFromServer` | Ya no tienes acceso a <nombre> | Se quita a un usuario de un servidor | `Server` |
| `deletedServer` | Se ha eliminado el servidor <nombre> | Se borra un servidor; se envía a los usuarios que tenían acceso a él | `Server` |
| `login` | Nuevo inicio de sesión en tu cuenta | Ningún código la envía | — |
| `test` | Correo de prueba de <empresa> | Botón de prueba / endpoint de prueba | — |

`addedToServer`: si el usuario no existía y el panel le creó la cuenta (`RegisterToken` no vacío), el botón lleva a `<panel>/forgot-password/` para que elija su contraseña con su dirección; si no, abre el servidor (`<panel>/servers/view/?id=<identificador>`).

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
| El panel no arranca tras editar plantillas | Falta un archivo (un cuerpo listado en `emails.json`) o una plantilla tiene un error de sintaxis |
| No llega nada y la interfaz no muestra error | Los correos asíncronos solo registran los fallos; revise el log del panel |

En producción, verifique la dirección o el dominio `from` con el proveedor y configure SPF/DKIM/DMARC para él.
