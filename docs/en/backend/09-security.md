# Security: Permissions, Tokens, SFTP and HTTP Protections

How the backend decides who may do what, how the panel and its nodes trust each other, how the built-in SFTP server signs people in, and which protections every HTTP answer carries.

Related pages:

- [Security & best practices](../general/security.md): accounts, 2FA, passwords and recommendations for administrators.
- [Permissions reference (scopes)](../general/scopes-reference.md): every scope, owners, sub-users, roles and API keys.
- [unshare & kernel isolation](../general/20-security-unshare.md): process isolation for servers on the host environment.

---

## Permission checks

Routes declare the scope they need with `middleware.RequiresPermission(scope)` (`internal/middleware/middleware.go`). For each request the check gathers:

1. the user's permissions **on the server** in the URL (`:serverId`), if any;
2. the user's **global** permissions;
3. the scopes of the user's **role**, if one is assigned;
4. the **owner scopes**, when the user owns that server.

`admin` satisfies every scope, and `server.admin` satisfies every server scope. A session created from an **OAuth2 client** is additionally bounded by what that client was registered for: a client tied to one server only works on that server, and a client with declared scopes never exceeds them, whatever its user may do. The scope list the check stores on the request (`c.Set("scopes")`, read by handlers through `callerScopes`) is bounded the same way (`boundByClient`), so the second checks handlers make (who sees a database password, who may change a locked environment variable, which scopes may be granted, which scopes a task step may use) and the streams the server WebSocket subscribes to never see the user's wider powers.

Scopes are defined in `internal/scopes/scopes.go`: a *server* scope (`forServer: true`, such as `server.files.edit`) is granted per server, a *panel* scope (such as `nodes.edit`) globally. The full list and what each one unlocks is in the [scopes reference](../general/scopes-reference.md).

---

## Panel and node tokens

The panel signs a short-lived token for every request it sends to a node's daemon (`/daemon/*`), including WebSocket proxies. Code: `internal/services/token.go`, `internal/middleware/decrypt.go`.

### The signing key

- An **Ed25519** key. Its 32-byte seed is stored base64-encoded in `panel.token`; if that key is empty when the panel first needs it, a random seed is generated and saved.
- The public half is published as a JWKS with the fixed key id `SkyPanel` at `GET /auth/publickey`.

### Tokens sent to nodes

| Property | Value |
|----------|-------|
| Algorithm | `EdDSA` (header `kid: SkyPanel`) |
| `iat` / `exp` | Issue time / issue time + **15 minutes** |
| `aud` | `hex(sha256("skypanel-daemon-aud:" + node secret))`: binds the token to one node without putting the secret in it |

The daemon (`middleware.ValidateJWT`) accepts a token only if it is signed with `EdDSA` by the panel's key, has an `exp` (one minute of clock leeway), and its audience matches either the local node's secret (panel and daemon in one process) or `daemon.auth.clientSecret` (a separately deployed node). A token captured on its way to one node is therefore useless against another.

A token that fails these checks is answered `401` with `WWW-Authenticate: Bearer error="invalid_token"` and logged on the node. The panel reports that refusal to the user as `502 ErrNodeAuthFailed` instead of passing the `401` on, so a wrong node secret or a clock that is off does not look like the user's session ending (see [API layer](./05-api-layer.md#panel-to-node-requests)).

How the daemon gets the public key:

- **Panel enabled** on the same process: from memory.
- **Daemon-only node** (`panel.enable=false`): it downloads the JWKS from the URL in `token.public` (the panel's `/auth/publickey`). If `token.public` is empty the node logs a warning at start and cannot authenticate the panel.

### Node secrets

Each node has a secret stored by the panel. An administrator with `nodes.deploy` reads it from `GET /api/nodes/:id/deployment` (`clientId` is `.node_<id>`, `clientSecret` is the secret) and puts it in the node's `daemon.auth.clientSecret`. The local node's secret is derived from `panel.sessionKey`, so it stays the same across restarts.

The node uses its secret in the other direction too: to ask the panel whether an SFTP login, with a password or an SSH key, is valid (next section). When the panel forwards a browser request to a node it **removes the `Cookie` header**: the user's session never reaches a node.

---

## SFTP server

Every daemon runs a built-in SFTP server (`internal/sftp`, no OpenSSH needed).

| Key | Default | Description |
|-----|---------|-------------|
| `daemon.sftp.host` | `0.0.0.0:5657` | Listen address |
| `daemon.sftp.key` | `<data>/sftp.key` | Host key. If missing, an Ed25519 key is generated and saved (PKCS#8 PEM) |
| `daemon.sftp.log` | `false` | Log every SFTP request (method, server, path) at debug level |

### Username

```
<email>#<serverId>
```

One login gives access to one server. Clients connect with `sftp -P 5657 'admin@example.com#a1b2c3d4'@panel.example.com`.

### Who may sign in

A login is accepted only when the account may still sign in to the panel (global `login` scope) **and** has `server.sftp` on that server. Owners have it through the owner scopes; `admin` and `server.admin` include it. SFTP does not distinguish `server.files.view` from `server.files.edit`: whoever has `server.sftp` can read and write the whole server folder. Two-factor authentication is not asked for SFTP, because SFTP clients cannot send a code: an account with 2FA can still sign in to SFTP with its password alone. Such accounts should use SSH keys, which every node accepts (see below), and keep a strong password, since the password still opens SFTP. Every failure returns the same "incorrect username or password", so the answer does not reveal whether the email exists.

### Password and key authentication

| Node | Password | SSH key |
|------|----------|---------|
| The panel's own node (panel enabled) | Checked against the database | Yes: any public key the user added in the **SSH keys** section of their account (`/api/self/sshkeys`) |
| A remote, daemon-only node | Checked by the panel over HTTP (below) | Yes: the same keys, checked by the panel over HTTP (below) |

**Remote nodes** send `grant_type=password`, `username`, `password` and `scope=sftp` to `daemon.auth.url` (the full URL of the panel's `/oauth2/token`), authenticated with `Authorization: Bearer <daemon.auth.clientSecret>`. The panel checks that the secret belongs to a node, that the server is on **that** node, the credentials, `login` and `server.sftp`, and answers with the scope `<serverId>:server.sftp`. It deliberately returns no access token: a password alone must not give a full panel session that skips 2FA.

**SSH keys on remote nodes.** A remote node does not hold the users' keys, so it asks the panel (`internal/oauth2/ssh.go`, 15 s timeout): it posts `{"username": "email#serverId", "publicKey": "<authorized_keys line>"}` to `POST /auth/sftp-key` with the same `Authorization: Bearer <daemon.auth.clientSecret>`. The URL comes from `daemon.auth.url`: same scheme and host, with `/oauth2/token` replaced by `/auth/sftp-key` (a path in front of it is kept). The panel (`internal/web/auth/sftp_key.go`) applies the rules of the panel's own node plus one: the server must be on the node asking. The key must be one of the user's, and the account must still hold `login` and `server.sftp` on that server. It answers only the granted scope (`200 {"scope": "<serverId>:server.sftp"}`), never a session; every refusal is the same `400` "no access", and an unknown node secret gets `401`. The SSH library runs this check before the client proves it holds the private key, and grants nothing until that proof succeeds. Both the panel and the node need a version with this route: an older panel answers `404` and the node refuses key logins.

**SSH keys** accepted when added: Ed25519, ECDSA (P-256, P-384, P-521), the security-key variants `sk-ssh-ed25519@openssh.com` and `sk-ecdsa-sha2-nistp256@openssh.com`, and RSA of at least 2048 bits. DSA keys, certificates, `authorized_keys` options and private keys are refused. Up to 25 keys per user; the name (defaulting to the key's comment) has up to 100 characters. At login the offered key is matched by its SHA-256 fingerprint and then compared byte for byte.

### Login attempt limits

Password logins share the panel's attempt limiter (`internal/services/login_attempts.go`):

- **10 attempts per account every 15 minutes**, counted together for the web login, the OAuth2 password check used by remote nodes, SFTP passwords and 2FA codes.
- Counted per account (or per typed email when it matches no account), not per IP, so it behaves the same behind a reverse proxy.
- A successful login resets the counter. Past the limit the answer is "too many attempts" until older attempts leave the window.
- Kept in memory: a panel restart resets it.

Key logins have a budget of their own, also 10 per account every 15 minutes (key `sshkey:user:<id>`), apart from the password one: an SSH client offers each of its keys in turn, and those misses must not lock the owner out of password sign-in. Every key offered for a known account counts, on the panel's own node (`validateSFTPKey` in `internal/services/sftp.go`) as well as in the `/auth/sftp-key` checks for remote nodes; both use the same counter, so the limit is the same on every node and shared between them. A successful key login resets it. Past the limit the panel's own node refuses the key (`ErrTooManyAttempts`) and `/auth/sftp-key` answers `429`.

### What a session can do

- Only `session` channels with the `sftp` subsystem are accepted; shell, exec and port forwarding are refused.
- Every path is resolved through the server's confined file server (see [File access](#file-access)).
- Uploads respect the client's create/exclusive/truncate flags and stop at the server's disk limit.
- `Setstat` (permissions, times) and `Symlink` are accepted but do nothing, so a client cannot create links.

---

## File access

All server file I/O (file manager, SFTP, operations, backups, extraction) goes through `files.FileServer` (`files/filesystem.go`):

- Paths are opened relative to the server folder with `openat2` and `RESOLVE_BENEATH`: the kernel refuses any path, `..` or symlink that would end up outside it.
- At start the daemon tries `openat2` and **refuses to start** if it cannot use it (a kernel without it, or a seccomp profile that blocks it, answering `EPERM`). `security.forceOpenat=true` skips the check and uses a fallback that opens each path component with `openat` and `O_NOFOLLOW`; use it only where `openat2` cannot be used.
- Files written by downloads are created with `O_NOFOLLOW`, so a symlink left where a file goes is refused instead of followed. See [Operations: file safety](./06-operations.md#file-safety).

## Server assistant file tools

The server assistant can read and change files through Gemini function calling (agent mode, `panel.settings.aiFileTools`). Its safeguards, described in full in [AI agent toolkit](./14-ai-toolkit.md):

- **One server.** The server comes from the authenticated route (`/api/servers/:serverId/ai/...`, after `server.console`), never from the model; no tool takes a server argument, and every operation goes to that server's `/daemon/server/<id>/...` routes on its node.
- **Paths** are cleaned by `tools/toolkit/security` on the panel, and again on the node for the `fs/*` routes (no absolute paths, drive letters, `..` or NUL, at most 4096 bytes and 64 elements), then opened by the confined file server above. Symlinks are never followed out of the server folder, writes use `O_NOFOLLOW`, creates `O_EXCL`, copies and searches skip links, and deleting a link removes only the link (the file manager's `DELETE` route too, which also refuses the server folder itself).
- **Permissions.** Tools are offered by scope: reads with `server.files.view`, the rest with `server.files.edit`; scopes are read again on every request and on every confirmation.
- **Confirmations.** Edits, overwrites, moves, renames and deletions wait for the user: a random single-use token bound to the user, the server and the hash of the exact calls, valid 10 minutes, one per user and server, kept in panel memory.
- **No commands**: there is no tool for shell or console commands or power actions.
- **Audit**: every change is recorded in the server's activity log as `ai.<tool>`.

---

## Process and container isolation

| Key | Default | Effect |
|-----|---------|--------|
| `docker.disallowHost` | `false` (`true` in the Docker image) | Servers whose environment is `host` are refused, also when started |
| `docker.serverUser` | `988:988` | uid:gid game containers run as when the panel runs as root (installs stay root). `0:0` keeps root |
| `docker.pidsLimit` | `4096` | Maximum processes/threads per server container; `0` removes the cap |
| `security.disableUnshare` | `false` | Turns off `unshare` isolation for host-environment servers |

Game containers also drop capabilities they do not need (fewer when they run as root), get `no-new-privileges`, and have their own `/tmp` and `/var/tmp` next to the server folder. Host-environment isolation is described in [unshare & kernel isolation](../general/20-security-unshare.md).

---

## HTTP protections

Applied to every request by `internal/web/loader.go` and `internal/middleware/security.go`.

### Security headers

| Header | Value |
|--------|-------|
| `Content-Security-Policy` | `frame-ancestors 'self'; object-src 'none'; base-uri 'self'` |
| `X-Frame-Options` | `SAMEORIGIN` |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `same-origin` |
| `Strict-Transport-Security` | `max-age=31536000`, only when the request arrived over HTTPS (directly or with `X-Forwarded-Proto: https`) |

Pages that serve untrusted content replace the CSP with their own sandbox: [extension files](../reference/15-extensions.md#pages) and the world map viewer.

### Cross-site requests

`SameOriginGuard` refuses requests that carry a panel cookie, change state (anything but `GET`/`HEAD`/`OPTIONS`, plus WebSocket handshakes) and come from another site, with `403 {"error": "cross-origin request refused"}`. Bearer-token clients are not affected. Details in [WebSocket: origin check](./08-websocket.md#origin-check).

### CORS

Any origin may call the API, but **without credentials** (`Access-Control-Allow-Credentials` is off): other sites can use a bearer token, never the user's cookies. Allowed headers: `Authorization`, `Content-Type`, `Accept`, `Origin`; methods `GET`, `POST`, `PUT`, `DELETE`, `OPTIONS`.

### Access log redaction

The request log hides the values of `token`, `code`, `access_token` and `refresh_token` query parameters and the signed token in world map links.

---

## Cookies

| Cookie | Purpose | Attributes |
|--------|---------|------------|
| `skypanel_auth` | The login session token | `HttpOnly`, `SameSite=Lax`, path `/`. `Secure` when the request came over HTTPS (or `X-Forwarded-Proto: https`) or `panel.web.cookies.secure` is `true`, so sign-in also works on `http://IP:8080` |
| `session` | Cookie-based session store (`gin-contrib/sessions`) | Configured with the keys below |

| Key | Default |
|-----|---------|
| `panel.web.cookies.secure` | `false` |
| `panel.web.cookies.httpOnly` | `true` |
| `panel.web.cookies.sameSite` | `Strict` (`Lax` and `None` also accepted) |
| `panel.web.cookies.age` | `2592000` (30 days, in seconds) |
| `panel.web.cookies.path` | `/` |
| `panel.web.cookies.domain` | empty |

The `session` cookie is signed with `panel.sessionKey`, generated on first start.

---

## Secrets in the settings API

`GET /api/settings` and `GET /api/settings/:key` never return a saved secret: mail passwords and keys, the Gemini and CurseForge keys, the four Discord webhooks, the license key, the S3 secret key, the Cloudflare token and the Turnstile secret key are answered as `********` (an unset one stays empty). When a request sends `********` back for one of them (`POST /api/settings`, `PUT /api/settings/:key` and the email, Discord, S3 and Cloudflare tests), the saved value is kept. So a user with `settings.edit` can replace these secrets but cannot read them out of the panel. Code: `internal/web/api/settings_secrets.go`.

---

## API keys

Provisioning integrations (WHMCS, Paymenter) call `/api/v1/*` with an API key instead of a session:

- Format `ak_<5 hex>_<43 hex>`, generated from 24 random bytes. Only its SHA-256 is stored, with the 8-character prefix for lookup; the full key is shown once.
- Sent as `X-Api-Key: <key>` or `Authorization: Bearer <key>` (only values starting with `ak_` are treated as keys).
- Each key carries a list of allowed actions, checked per route.

Creating keys and their permissions: [scopes reference — API keys](../general/scopes-reference.md#api-keys).

---

## Behind a reverse proxy

By default the panel trusts **no** proxy headers for the client address.

| Key | Default | Description |
|-----|---------|-------------|
| `security.trustedProxies` | `[]` | IPs or CIDRs whose `X-Forwarded-For` is believed |
| `security.trustedProxyHeader` | empty | A header that holds the real client IP set by a trusted platform, such as `CF-Connecting-IP` (Cloudflare) or `X-Real-IP`. It is read whatever address the request comes from, so set it only when every request passes through that platform |

```json
{
  "security": {
    "trustedProxies": ["127.0.0.1", "172.16.0.0/12"],
    "trustedProxyHeader": "X-Real-IP"
  }
}
```

The installer's Nginx setup binds the panel to `127.0.0.1` and sends `X-Forwarded-Proto`, which also makes cookies `Secure` and enables HSTS.

---

## Template downloads

`download`, `scraperdl` and the asset download of `githubdl` never connect to loopback, link-local or metadata addresses, and refuse private networks while `security.blockPrivateDownloads` is `true` (the default). See [Operations: download network rules](./06-operations.md#download-network-rules).

---

## Known limitations

- SFTP password logins do not ask for 2FA (see [Who may sign in](#who-may-sign-in)); SSH keys, accepted on every node, avoid sending the password.
- TOTP secrets (`users.otp_secret`, `otp_pending_secret`) are stored in the database unencrypted; protect database access and backups accordingly.
- The license server host is fixed in code and is a test host (see [Services → LicenseService](./11-services.md#22-licenseservice-licensego)).
