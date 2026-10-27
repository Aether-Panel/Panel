# Captcha / anti-bot

The panel checks that whoever fills in its public forms is not a bot. There are two checks:

- **Built-in**: the browser solves a small calculation (a proof of work) in the background. No checkboxes, no images, no keys and no outside services. It is **on by default**.
- **Cloudflare Turnstile**: optional. It needs a Cloudflare account and two keys.

Only one is used at a time.

## Where it applies

| Form | Endpoint |
|------|----------|
| Sign in | `POST /auth/login` |
| Sign up | `POST /auth/register` |
| Forgot password | `POST /auth/forgot-password` |

The check runs before the password or the email is looked at. It is not asked for in the second sign-in step (`POST /auth/otp`), in the link of the password reset email (`POST /auth/reset-password`) or when confirming your password inside the panel (`POST /auth/reauth`).

## Which check is used

| Situation | Check |
|-----------|-------|
| Turnstile switched on and both keys set (site key and secret key) | Turnstile |
| Otherwise, if the built-in check is on | Built-in |
| Neither | None |

If you switch Turnstile on but one key is missing, the panel keeps using the built-in check. While Turnstile is configured, the built-in switch has no effect.

The panel publishes which one it uses in `GET /api/config` (no session needed), in the `captcha` field: `"turnstile"`, `"builtin"` or `""`.

## Settings

Change them in **Settings → General → Sign-up and security**, or in `config.json` and environment variables:

| Key | Environment variable | Default | What it does |
|-----|----------------------|---------|--------------|
| `panel.captcha.builtin` | `SKYPANEL_PANEL_CAPTCHA_BUILTIN` | `true` | Turns the built-in check on. |
| `panel.turnstile.enabled` | `SKYPANEL_PANEL_TURNSTILE_ENABLED` | `false` | Switches Turnstile on. |
| `panel.turnstile.siteKey` | `SKYPANEL_PANEL_TURNSTILE_SITEKEY` | empty | The widget's public key. |
| `panel.turnstile.secretKey` | `SKYPANEL_PANEL_TURNSTILE_SECRETKEY` | empty | The secret key the panel verifies tokens with. |

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

### Turning the check off

Switch off **Built-in anti-bot check** and leave Turnstile off (or without keys). The forms stop asking for anything. We do not recommend it on a panel open to the internet: the per-account attempt limit still applies, but the sign-up and password reset forms are left without protection against bots.

## Built-in check

It uses the same scheme as ALTCHA:

1. The browser asks `GET /auth/challenge` for a challenge.
2. The panel secretly picks a number between 0 and 100,000 and sends the SHA-256 of `salt + number`, signed with an HMAC.
3. The browser tries numbers until it finds the one that gives that hash. A person does not notice; a bot sending thousands of attempts pays that cost on each one.
4. The form sends the solution in the `captchaToken` field.

Worth knowing:

- A challenge expires after **10 minutes** and each solution works **only once**.
- The key that signs challenges is generated when the panel starts. A restart cancels the challenges handed out before it; reloading the page is enough.
- The key lives in memory. If you run several panel instances behind a load balancer, the challenge and the form must reach the same instance.
- Nothing leaves the panel and no cookie is stored.
- It also works on `http://<ip>:8080`: the browser does not need `https` to compute the hash.
- The form's button waits until the check is done. If it fails, a **Retry** button appears. After each submit a new check starts.

### From the API

A client using `/auth/login`, `/auth/register` or `/auth/forgot-password` has to solve the challenge. For automation, an OAuth2 client or an API key is simpler, since they do not go through these forms.

Ask for a challenge:

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

Find the number `n` between 0 and `maxnumber` for which `sha256(salt + n)`, in hex, equals `challenge`. The token is the solution's JSON, base64-encoded:

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
    "password": "<your-password>",
    "captchaToken": token,
})
```

If the token is missing, expired, already used or wrong, the answer is `400`:

```json
{ "error": { "msg": "the anti-bot check failed, try it again", "code": "ErrCaptchaFailed" } }
```

## Cloudflare Turnstile

### Getting the keys

1. Open the [Cloudflare Turnstile dashboard](https://dash.cloudflare.com/turnstile).
2. Create a site and add the panel's domain.
3. Copy the **Site Key** and the **Secret Key**.
4. Paste them in **Settings → General → Sign-up and security** and switch on **Cloudflare Turnstile Protection**.

### How it is verified

- The browser loads `https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit` and shows the widget in the form.
- The form sends the token in the `turnstileToken` field.
- The panel verifies it at `https://challenges.cloudflare.com/turnstile/v0/siteverify`, sending `secret`, `response` and `remoteip` (the client's IP). It waits at most 5 seconds.
- A missing token is refused without asking Cloudflare.
- If Cloudflare does not answer, the form is refused with `ErrCaptchaFailed`: while Cloudflare is unavailable nobody can sign in. The panel logs `Could not reach Cloudflare Turnstile`.
- After each submit the widget is shown again, because a token is good for one request.

The IP sent as `remoteip` is the one the panel sees. Behind a proxy, set `security.trustedProxies` (and `security.trustedProxyHeader` if needed) so it is the user's and not the proxy's.

The site key is public (it appears in `GET /api/config`). The secret key never leaves the server.

### Common problems

| Problem | Likely cause | Fix |
|---------|--------------|-----|
| The widget does not show | The browser blocks `challenges.cloudflare.com` | Check blocking extensions or the proxy's content policy. |
| Always `ErrCaptchaFailed` | The panel cannot reach `challenges.cloudflare.com` | Allow outbound HTTPS to that domain from the server. |
| Always `ErrCaptchaFailed` | Wrong secret key | Copy it again from Cloudflare. |
| The widget shows a domain error | The panel's domain is not in the Turnstile site | Add it in Cloudflare. |
| Built-in check: "The anti-bot check could not run." | `GET /auth/challenge` failed | Click **Retry**; check that the proxy lets `/auth/` through. |
| Built-in check: `ErrCaptchaFailed` after a restart | The challenge was fetched before the restart | Reload the page. |

## Files

| File | What it holds |
|------|---------------|
| `internal/web/auth/captcha.go` | Choosing the check, `GET /auth/challenge` and verifying the built-in one. |
| `internal/web/auth/turnstile.go` | Verification with Cloudflare. |
| `internal/config/entries.go` | The configuration keys. |
| `client/frontend/src/components/Turnstile.tsx` | `useCaptcha()`: the Turnstile widget or the built-in check's line. |
| `client/frontend/src/lib/proof-of-work.ts` | The calculation the browser does. |
