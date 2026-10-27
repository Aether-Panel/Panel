# Captcha / anti-bots

El panel comprueba que quien rellena los formularios públicos no es un bot. Hay dos comprobaciones:

- **Integrada**: el navegador resuelve un pequeño cálculo (una prueba de trabajo) en segundo plano. Sin casillas, sin imágenes, sin claves y sin servicios externos. Está **activada por defecto**.
- **Cloudflare Turnstile**: opcional. Necesita una cuenta de Cloudflare y dos claves.

Solo se usa una a la vez.

## Dónde se aplica

| Formulario | Endpoint |
|------------|----------|
| Iniciar sesión | `POST /auth/login` |
| Registro | `POST /auth/register` |
| He olvidado mi contraseña | `POST /auth/forgot-password` |

La comprobación se hace antes de mirar la contraseña o el correo. No se pide en el segundo paso del inicio de sesión (`POST /auth/otp`), en el enlace del correo de cambio de contraseña (`POST /auth/reset-password`) ni al confirmar la contraseña dentro del panel (`POST /auth/reauth`).

## Qué comprobación se usa

| Situación | Comprobación |
|-----------|--------------|
| Turnstile encendido y con las dos claves (site key y secret key) | Turnstile |
| Si no, y la integrada está activada | Integrada |
| Ninguna de las dos | Ninguna |

Si enciendes Turnstile pero falta una de las claves, el panel sigue usando la integrada. Mientras Turnstile esté configurado, el interruptor de la integrada no tiene efecto.

El panel publica cuál usa en `GET /api/config` (no necesita sesión), en el campo `captcha`: `"turnstile"`, `"builtin"` o `""`.

## Ajustes

Se cambian en **Ajustes → General → Registro y seguridad**, o en `config.json` y variables de entorno:

| Clave | Variable de entorno | Por defecto | Qué hace |
|-------|---------------------|-------------|----------|
| `panel.captcha.builtin` | `SKYPANEL_PANEL_CAPTCHA_BUILTIN` | `true` | Activa la comprobación integrada. |
| `panel.turnstile.enabled` | `SKYPANEL_PANEL_TURNSTILE_ENABLED` | `false` | Enciende Turnstile. |
| `panel.turnstile.siteKey` | `SKYPANEL_PANEL_TURNSTILE_SITEKEY` | vacío | Clave pública del widget. |
| `panel.turnstile.secretKey` | `SKYPANEL_PANEL_TURNSTILE_SECRETKEY` | vacío | Clave secreta con la que el panel verifica los tokens. |

```json
{
  "panel": {
    "captcha": { "builtin": true },
    "turnstile": {
      "enabled": false,
      "siteKey": "",
      "secretKey": ""
    }
  }
}
```

### Apagar la comprobación

Desactiva **Comprobación anti-bots integrada** y deja Turnstile apagado (o sin claves). Los formularios dejan de pedir nada. No lo recomendamos en un panel abierto a internet: el límite de intentos por cuenta sigue activo, pero el formulario de registro y el de cambio de contraseña quedan sin protección frente a bots.

## Comprobación integrada

Usa el mismo esquema que ALTCHA:

1. El navegador pide un reto a `GET /auth/challenge`.
2. El panel elige en secreto un número entre 0 y 100 000 y envía el SHA-256 de `salt + número`, firmado con HMAC.
3. El navegador prueba números hasta dar con el que produce ese hash. Una persona no lo nota; un bot que envía miles de intentos paga ese coste en cada uno.
4. El formulario envía la solución en el campo `captchaToken`.

Lo que conviene saber:

- El reto caduca a los **10 minutos** y cada solución sirve **una sola vez**.
- La clave que firma los retos se genera al arrancar el panel. Un reinicio invalida los retos entregados antes; basta con recargar la página.
- La clave vive en memoria. Si pones varias instancias del panel detrás de un balanceador, el reto y el formulario tienen que llegar a la misma instancia.
- No sale nada del panel ni se guarda ninguna cookie.
- Funciona también en `http://<ip>:8080`: el navegador no necesita `https` para calcular el hash.
- El botón del formulario espera a que termine la comprobación. Si falla, aparece **Reintentar**. Tras cada intento de envío se hace una comprobación nueva.

### Desde la API

Un cliente que use `/auth/login`, `/auth/register` o `/auth/forgot-password` tiene que resolver el reto. Para automatizar tareas es más sencillo un cliente OAuth2 o una API key, que no pasan por estos formularios.

Pide un reto:

```bash
curl https://panel.example.com/auth/challenge
```

```json
{
  "algorithm": "SHA-256",
  "challenge": "3f1c…",
  "salt": "9b2e…?expires=1793030400",
  "maxnumber": 100000,
  "signature": "a7d0…"
}
```

Busca el número `n` entre 0 y `maxnumber` con el que `sha256(salt + n)`, en hexadecimal, es igual a `challenge`. El token es el JSON de la solución codificado en base64:

```python
import base64, hashlib, json, requests

ch = requests.get("https://panel.example.com/auth/challenge").json()
n = next(i for i in range(ch["maxnumber"] + 1)
         if hashlib.sha256(f'{ch["salt"]}{i}'.encode()).hexdigest() == ch["challenge"])
token = base64.b64encode(json.dumps({
    "algorithm": ch["algorithm"],
    "challenge": ch["challenge"],
    "number": n,
    "salt": ch["salt"],
    "signature": ch["signature"],
}).encode()).decode()

requests.post("https://panel.example.com/auth/login", json={
    "email": "ana@example.com",
    "password": "<tu-contraseña>",
    "captchaToken": token,
})
```

Si falta el token, está caducado, ya se usó o no es correcto, la respuesta es `400`:

```json
{ "error": { "msg": "the anti-bot check failed, try it again", "code": "ErrCaptchaFailed" } }
```

## Cloudflare Turnstile

### Obtener las claves

1. Entra en el [panel de Turnstile de Cloudflare](https://dash.cloudflare.com/turnstile).
2. Crea un sitio y añade el dominio del panel.
3. Copia la **Site Key** y la **Secret Key**.
4. Pégalas en **Ajustes → General → Registro y seguridad** y enciende **Protección Cloudflare Turnstile**.

### Cómo se verifica

- El navegador carga `https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit` y muestra el widget en el formulario.
- El formulario envía el token en el campo `turnstileToken`.
- El panel lo verifica en `https://challenges.cloudflare.com/turnstile/v0/siteverify` enviando `secret`, `response` y `remoteip` (la IP del cliente). Espera como mucho 5 segundos.
- Un token que falta se rechaza sin consultar a Cloudflare.
- Si Cloudflare no responde, el formulario se rechaza con `ErrCaptchaFailed`: mientras Cloudflare no esté disponible no se puede iniciar sesión. El panel deja en el log `Could not reach Cloudflare Turnstile`.
- Tras cada intento de envío el widget se vuelve a mostrar, porque un token sirve para una sola petición.

La IP que se envía como `remoteip` es la que ve el panel. Detrás de un proxy, configura `security.trustedProxies` (y `security.trustedProxyHeader` si hace falta) para que sea la del usuario y no la del proxy.

La site key es pública (aparece en `GET /api/config`). La secret key no sale nunca del servidor.

### Problemas frecuentes

| Problema | Causa probable | Solución |
|----------|----------------|----------|
| El widget no aparece | El navegador bloquea `challenges.cloudflare.com` | Revisa extensiones de bloqueo o la política de contenido del proxy. |
| Siempre `ErrCaptchaFailed` | El panel no llega a `challenges.cloudflare.com` | Permite la salida HTTPS hacia ese dominio desde el servidor. |
| Siempre `ErrCaptchaFailed` | Secret key equivocada | Cópiala otra vez desde Cloudflare. |
| El widget da error de dominio | El dominio del panel no está en el sitio de Turnstile | Añádelo en Cloudflare. |
| Comprobación integrada: «No se pudo hacer la comprobación anti-bots» | Falló `GET /auth/challenge` | Pulsa **Reintentar**; revisa que el proxy deje pasar `/auth/`. |
| Comprobación integrada: `ErrCaptchaFailed` tras reiniciar | El reto se pidió antes del reinicio | Recarga la página. |

## Archivos

| Archivo | Qué contiene |
|---------|--------------|
| `internal/web/auth/captcha.go` | Elección de la comprobación, `GET /auth/challenge` y verificación de la integrada. |
| `internal/web/auth/turnstile.go` | Verificación con Cloudflare. |
| `internal/config/entries.go` | Las claves de configuración. |
| `client/frontend/src/components/Turnstile.tsx` | `useCaptcha()`: el widget de Turnstile o la línea de la comprobación integrada. |
| `client/frontend/src/lib/proof-of-work.ts` | El cálculo que hace el navegador. |
