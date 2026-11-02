# Authentication

How a request proves who sent it, how the panel decides what it may do, and the sign-in routes. Back to the [API reference](../11-api-reference.md). Response and error formats are in [Conventions](./01-conventions.md).

## Routes

| Method | Path | Auth | What it does |
|--------|------|------|--------------|
| POST | `/auth/login` | Public | Signs in with email and password |
| POST | `/auth/otp` | Public (pending sign-in cookie) | Second step with a 2FA or recovery code |
| POST | `/auth/logout` | Public | Ends a session |
| POST | `/auth/register` | Public (sign-ups open) | Creates an account and signs it in |
| POST | `/auth/forgot-password` | Public | Emails a password reset link |
| POST | `/auth/reset-password` | Public (reset token) | Sets a new password |
| POST | `/auth/reauth` | Session or bearer | Opens a fresh session for the signed-in user |
| GET | `/auth/sso` | One-time link | Opens a session from a billing system link |
| GET | `/auth/challenge` | Public | Built-in anti-bot challenge |
| GET | `/auth/publickey` | Public | The panel's JWT public keys (JWKS) |
| POST | `/oauth2/token` | Client credentials or node secret | Issues a token |
| GET | `/api/config` | Public | Branding, sign-up and anti-bot settings, version |

## Ways to authenticate

| Method | How it is sent | Used for |
|--------|----------------|----------|
| Session cookie | Cookie `skypanel_auth` | The web client |
| Bearer token | `Authorization: Bearer <token>` | Scripts and integrations: a session token or an OAuth2 access token |
| API key | `X-Api-Key: ak_...` or `Authorization: Bearer ak_...` | Only the billing routes under `/api/v1` (see [Provisioning](./07-provisioning.md)) |
| Panel JWT | `Authorization: Bearer <jwt>` | The `/daemon` routes, panel to node, except the public `GET`/`HEAD`/`OPTIONS /daemon` and `GET /daemon/features` (see [Daemon](./09-daemon.md)) |
| Node secret | `Authorization: Bearer <secret>` | Only the OAuth2 password grant, node to panel |
| Signed links | Token in the path or query | `/auth/sso?token=...`, `/api/map-view/...` |
| Ed25519 signatures | In the body or query | `/api/extransfer/*`, panel to panel, except `validate`, which takes the transfer token (see [External transfer](./08-external-transfer.md)) |

On `/api` and `/auth/reauth` the `Authorization` header wins over the cookie. A header that is not exactly `Bearer <token>` answers `401`; a request with neither header nor cookie answers `401`.

### Sessions

A session is a random token the panel stores hashed (SHA-256). It lasts **1 hour** and slides: every authenticated request renews it once less than half of the hour is left, so it only ends after an hour without activity. `POST /auth/login`, `/auth/otp`, `/auth/register`, `/auth/reauth` and `/auth/sso` create one. Expired sessions are refused at once and their rows are deleted later: creating a session also deletes the expired ones, at most once an hour.

The token reaches the client in two ways:

- the `skypanel_auth` cookie: `Path=/`, `HttpOnly`, `SameSite=Lax`, `Max-Age=3600` (refreshed on each cookie-authenticated request), and `Secure` when the request came over HTTPS (directly or with `X-Forwarded-Proto: https`) or `panel.web.cookies.secure` is on;
- the `session` field of the sign-in answer, to use as a bearer token.

A second cookie, `session`, is signed by the panel and only holds the pending 2FA step (the email and the time of the password step). Its attributes come from `panel.web.cookies.*` (`path`, `domain`, `age`, `secure`, `httpOnly`, `sameSite`, default `Strict`).

Every request checks again that the account holds the `login` scope (in its panel-wide permissions or its role). Taking it away ends the open sessions: the next request answers `401`. Changing the password from the profile ends every other session of the account; an administrator changing it (`POST /api/users/:id`) or a password reset ends all of them.

### OAuth2 client credentials

For integrations that act as a user:

1. Create a client with `POST /api/self/oauth2` (scope `self.clients`). The answer holds `client_id` and `client_secret`; the secret is only shown then and is stored hashed (bcrypt).
2. Exchange it at `POST /oauth2/token` with `grant_type=client_credentials`.
3. Send `Authorization: Bearer <access_token>`.

The access token is a panel session tied to the client and its owner: it lasts an hour and slides like any other session. Requests made with it have the owner's permissions. The panel can also bound a client to one server or a scope list, but clients created through the API carry neither, so they act with everything the account may do. A client token cannot call `/auth/reauth` (`403`). Deleting the client ends the sessions it opened.

## Same-origin guard

Every route of the panel (`/api`, `/auth`, `/oauth2`, `/daemon`) goes through a guard that refuses state-changing requests started by another site with the panel's cookies.

A request is refused when **all** of these hold:

1. It carries a non-empty `skypanel_auth` or `session` cookie.
2. It changes state: any method except `GET`, `HEAD` and `OPTIONS`, plus a `GET` that opens a WebSocket (`Upgrade: websocket`).
3. It is not same-origin:
   - with a `Sec-Fetch-Site` header (sent by every current browser), only `same-origin` and `none` pass;
   - without it, a request with no `Origin` passes; otherwise the `Origin` host must equal the request's `Host`, its `X-Forwarded-Host`, or the host of `panel.settings.masterUrl`.

The answer is `403 {"error": "cross-origin request refused"}`.

Why: `SameSite=Lax` compares sites, and a site is the host without the port. A game server's web page at `http://IP:25566` is the same site as a panel at `http://IP:8080`, so the browser would attach the panel cookie to a form it posts or a WebSocket it opens to read a console. Requests without the panel's cookies (bearer tokens, API keys, OAuth2 clients, the daemon) are never affected: scripts should send a bearer token and no cookies.

## Permissions (scopes)

Each route names the scope it needs (shown in every route table). The check:

- `admin` covers every scope; `server.admin` covers every server scope on that server.
- A server scope (`server.*`) needs a `:serverId` in the path. The caller's scopes there are their permission row on that server, their panel-wide row, the owner's set when they own the server, and the panel-wide scopes of their role. A role's server scopes do not count on servers.
- A route marked "any of" passes with one of the listed scopes.
- Missing the scope answers `403` with no body.
- Sessions opened from an OAuth2 client are also limited by the client's own bounds when it has them. The scope list the route resolves is bounded the same way before handlers see it, so the checks a handler makes on its own (whether a database password is shown, whether a locked environment variable may change, which scopes may be granted, which scopes a task step may use) and the streams a server WebSocket receives never reach past what the client may use.

Routes marked "Signed in" only need a valid session; `login` means the account must hold the `login` scope, which every account that can sign in has.

The list of scopes, what each covers and who gets them by default is in the [Permissions reference](../../general/scopes-reference.md).

## Sign-in routes

### `POST /auth/login`

```json
{ "email": "admin@example.com", "password": "...", "captchaToken": "..." }
```

| Field | Notes |
|-------|-------|
| `email`, `password` | Required |
| `turnstileToken` | Required when `GET /api/config` reports `"captcha": "turnstile"` |
| `captchaToken` | Required when it reports `"captcha": "builtin"`: see [`GET /auth/challenge`](#get-authchallenge) |

Answer when the account has no 2FA (it also sets the `skypanel_auth` cookie):

```json
{
  "scopes": ["login", "server.create", "templates.view"],
  "session": "6f1c0e9a-..."
}
```

`scopes` are the account's panel-wide scopes (its own and its role's). With 2FA on, the answer is `{"otpNeeded": true}` plus the `session` cookie, and the sign-in finishes with `/auth/otp`.

| Error | When |
|-------|------|
| `400 ErrCaptchaFailed` | Anti-bot answer missing, wrong, expired or reused |
| `400 ErrInvalidCredentials` | Unknown email or wrong password (same answer for both) |
| `403 ErrLoginNotPermitted` | The account lacks `login` |
| `429 ErrTooManyAttempts` | 10 failed attempts in 15 minutes for this account |

Failed attempts on a known account are recorded in the access log as `login_failed`; successful ones as `login`.

### `POST /auth/otp`

```json
{ "token": "123456" }
```

`token` is the current TOTP code or one of the account's recovery codes (each recovery code works once). The request must carry the `session` cookie from the password step, at most 5 minutes old. The answer is the same as a sign-in without 2FA. After success the pending step is cleared, so the cookie cannot open more sessions.

Errors: `400 ErrInvalidSession` (no pending sign-in), `400 ErrSessionExpired` (older than 5 minutes), `400 ErrInvalidCredentials`, `403 ErrLoginNotPermitted`, `429 ErrTooManyAttempts`.

### `POST /auth/logout`

Ends the session given as `?token=...`, or else the one in the `skypanel_auth` cookie, and clears the cookie. Always `204`.

### `POST /auth/register`

Only while sign-ups are open (`panel.registrationEnabled`); otherwise `404`.

```json
{ "username": "newplayer", "email": "new@example.com", "password": "at-least-8", "captchaToken": "..." }
```

`username` is 5 to 100 printable ASCII characters; `email` must be valid; the password needs at least 8 bytes. The account gets the `login` scope, a welcome email, and a session (same answer as sign-in). Errors: `400` for validation, `400 ErrCaptchaFailed`, `400 ErrPasswordRequirements`.

### `POST /auth/forgot-password`

```json
{ "email": "admin@example.com", "captchaToken": "..." }
```

Always `204`, whether the account exists or not. When it does, an email with `<master URL>/reset-password/?token=...` is sent. The link always uses `panel.settings.masterUrl`, never the request's headers. Requests past 5 per email per hour are silently dropped. Errors: `400` for an invalid email, `400 ErrCaptchaFailed`.

### `POST /auth/reset-password`

```json
{ "token": "...", "password": "new-password" }
```

`204` on success. The password is checked before the token is spent, so a short password can be retyped. Every session of the account ends and a confirmation email is sent. Errors: `400 ErrPasswordRequirements`, `400 ErrTokenInvalid`.

### `POST /auth/reauth`

Opens a new session for the user of the current one and answers like a sign-in. Sessions opened from an OAuth2 client are refused (`403`), so a client secret can never turn into a browser session.

### `GET /auth/sso?token=...`

The landing route of a one-time link made by [`POST /api/v1/sso`](./07-provisioning.md#post-apiv1sso). It opens the session, records `login_sso` in the access log and redirects (`303`) to the server or to `/servers/`. A used, expired or unknown link redirects to `/login/?sso=expired`; an account without `login` to `/login/?sso=denied`. Answers carry `Referrer-Policy: no-referrer` and `Cache-Control: no-store`.

### `GET /auth/challenge`

The built-in anti-bot check, used by sign-in, sign-up and password reset when Turnstile is not configured and `panel.captcha.builtin` is on (the default).

```json
{
  "algorithm": "SHA-256",
  "challenge": "<hex SHA-256 of salt + number>",
  "salt": "<hex>?expires=1793580000",
  "maxnumber": 100000,
  "signature": "<hex HMAC>"
}
```

Find the integer `number` between `0` and `maxnumber` such that `SHA-256(salt + number)` equals `challenge`, then send `captchaToken` as the base64 of this JSON:

```json
{ "algorithm": "SHA-256", "challenge": "...", "number": 41234, "salt": "...", "signature": "..." }
```

A challenge expires after 10 minutes, each solution is accepted once, and a panel restart cancels the challenges handed out before it. Cloudflare Turnstile is described in [Turnstile](../../general/17-turnstile.md).

### `GET /auth/publickey`

The panel's public key set (JWKS, Ed25519, key id `SkyPanel`). Nodes use it to check the JWTs the panel signs for `/daemon` (`token.public` in the node's configuration).

## OAuth2

### `POST /oauth2/token`

Form fields (`application/x-www-form-urlencoded`): `grant_type`, `client_id`, `client_secret`, `username`, `password`.

**`grant_type=client_credentials`**

```bash
curl -X POST https://panel.example.com/oauth2/token \
  -d grant_type=client_credentials -d client_id=CLIENT_ID -d client_secret=CLIENT_SECRET
```

```json
{ "access_token": "0b7c2f4e-...", "token_type": "Bearer", "expires_in": 3600, "scope": "login server.create" }
```

`scope` lists the scopes of the owner's panel-wide permission row (role scopes are not listed). The owner needs `login` in that row; otherwise the answer is `403` with no body.

**`grant_type=password`** (nodes only)

A node uses it to check an SFTP login. It sends `Authorization: Bearer <node secret>` (the `clientSecret` of its deployment), `username=<email>#<serverId>` and the user's password. The panel checks that the server is on that node, the password, the `login` scope and `server.sftp` on the server. The answer has no access token, only `{"token_type": "Bearer", "expires_in": 3600, "scope": "<serverId>:server.sftp"}`: a password alone never opens a panel session (2FA would be skipped). The SFTP login itself does not ask for the 2FA code, because SFTP clients cannot send one: accounts with 2FA should use [SSH keys](./05-accounts.md#ssh-keys) for SFTP, which the panel's own node accepts (remote nodes only take passwords).

| Error | When |
|-------|------|
| `400 {"error": "invalid_request", ...}` | Bad form, unknown client, bad username, wrong password, no access (`"error_description": "no access"`) |
| `400 {"error": "invalid_client"}` | Wrong client secret |
| `401 {"error": "invalid_client"}` | Password grant without a bearer header |
| `400 {"error": "unsupported_grant_type"}` | Any other grant type |

## Public configuration

### `GET /api/config`

No authentication. The sign-in page reads it before anyone signs in.

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

| Field | Meaning |
|-------|---------|
| `themes.available` | Theme archives (`*.tar`) found in `<panel.web.files>/theme` |
| `captcha` | Which anti-bot check the forms need: `turnstile` (enabled and both keys set), `builtin`, or empty for none |
| `hideAiAnalysis` | `panel.settings.hideAiAnalysis`: the AI log analysis is off for every user; the web client hides its buttons and `POST /api/servers/:serverId/ai/analyze` answers `403 ErrAIAnalysisDisabled` |
| `experimental` | Experimental features switched on in the settings; their routes answer `403 ErrFeatureDisabled` while off |
