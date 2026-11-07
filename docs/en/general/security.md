# Security

This page is for whoever installs and runs Aether Panel. It covers how to expose the panel safely, how sign-in and sessions work, and the protections the panel applies to game servers, files and the web interface. The full permission model is in the [Scopes reference](./scopes-reference.md).

## Summary

| Layer | What the panel does |
|-------|---------------------|
| Exposure | Listens on plain HTTP (`web.host`, `0.0.0.0:8080`). HTTPS comes from Nginx, which the installer sets up when you use a domain; the panel port is then published on `127.0.0.1` only. |
| Sign-in | bcrypt passwords (8 to 72 bytes), optional TOTP 2FA with recovery codes, an anti-bot check, attempt limits per account. |
| Sessions | Random tokens stored only as a SHA-256 hash, 1 hour of inactivity, HttpOnly cookie, `Secure` over HTTPS. |
| Web requests | Cookie requests that change something must come from the panel itself; security headers on every answer; CORS never carries cookies. |
| Permissions | 73 scopes, owners and sub-users, server rows that only hold server scopes, staff that cannot edit stronger accounts. |
| Game servers | Docker only, unprivileged user, dropped capabilities, `no-new-privileges`, a process cap, their own network. |
| Files | Every file operation stays beneath the server folder; links are never followed out of it. |

## Expose the panel with a domain and HTTPS

A panel that serves customers should be reached at `https://your-domain`, not at `http://IP:8080`.

**Why a plain `http://IP` panel is a risk.** Browsers tie cookies to the host, not to the port. A panel at `http://203.0.113.10:8080` sends its session cookie to every port of `203.0.113.10`, and customers' game servers run on that same IP. A web page or plugin on a game server (for example a map viewer on port 8123) receives the administrator's cookie when the administrator opens it. The panel's [same-origin guard](#web-protections) stops such a page from making changes with the cookie, but the cookie itself still travels in clear text over HTTP and to those ports. With a domain and HTTPS the cookie is marked `Secure`, is only sent to the panel's own host, and is encrypted on the way.

**How to set it up.**

1. Point a domain (or subdomain) at the server's IP, with or without Cloudflare's proxy.
2. Either pass it to the installer (`--domain panel.example.com --ssl`) or, on a running panel, enter `https://panel.example.com` in **Settings → General → Base URL**. The `aether-domain` service installed by the installer then sets up Nginx, the Let's Encrypt certificate and the firewall. Details are in the [Installation guide](../setup/01-installation.md).
3. Set the Base URL (`panel.settings.masterUrl`) in any case: password reset and sign-in links are built from it, never from the request's headers.

**What the installer does with a domain.** It publishes the panel's port on `127.0.0.1` only, so the panel is reached through Nginx and nobody can use `http://IP:8080` around HTTPS. Docker publishes container ports ahead of ufw/firewalld, so a firewall rule alone would not close it. Pass `--bind 0.0.0.0` only if you need the port open on purpose.

**Behind your own proxy.** The panel always trusts this machine, private networks and Cloudflare's edge, which covers the installer's Nginx, with or without Cloudflare in front (see [Behind a reverse proxy](../backend/09-security.md#behind-a-reverse-proxy)). If your proxy has a public address, list it in `security.trustedProxies` (and `security.trustedProxyHeader` if it uses a custom header) so the panel sees the real client IP. Send `X-Forwarded-Proto: https`: the panel uses it to mark the cookie `Secure` and to send HSTS.

## Sign-in and sessions

The sign-in, sign-up and password reset forms go through an anti-bot check, described in [Captcha / anti-bot](17-turnstile.md).

### Sessions

- Signing in creates a random token (UUID v4). The database only keeps its SHA-256 hash.
- A session lasts **1 hour without activity**. While the user is active it is renewed (the database expiry once more than half of it has passed, the cookie on every request), so an active user is not signed out.
- The token is sent back in the `skypanel_auth` cookie and in the JSON answer (`session`), for clients that use `Authorization: Bearer <token>`. The panel reads the header first and the cookie second.
- Signing out deletes the session. The panel also checks the `login` permission on every request (see [Taking away the login permission](#taking-away-the-login-permission)).

### Cookies

| Cookie | What it holds | Attributes |
|--------|---------------|------------|
| `skypanel_auth` | The session token | `HttpOnly`, `SameSite=Lax`, path `/`, lifetime 1 hour (renewed while active). `Secure` when the request came over HTTPS (directly or with `X-Forwarded-Proto: https`) or when `panel.web.cookies.secure` is on. |
| `session` | Short-lived sign-in state, such as the pending 2FA step (signed with `panel.sessionKey`) | Set by `panel.web.cookies.*`: `path` (`/`), `domain` (empty), `age` (30 days), `secure` (`false`), `httpOnly` (`true`), `sameSite` (`Strict`; `Lax` and `None` also accepted). |

`skypanel_auth` is not marked `Secure` on plain HTTP on purpose: browsers drop a `Secure` cookie sent over HTTP, which made sign-in impossible on `http://IP:8080`. On HTTPS it always is. The panel also stores a few preferences in the browser's local storage (language, last tab opened, dismissed notices); they hold no credentials.

### Attempt limits

| What | Limit | Answer once over it |
|------|-------|---------------------|
| Password (web, OAuth2 and SFTP share the counter) | 10 attempts in 15 minutes per account | `429` `ErrTooManyAttempts` |
| 2FA code | 10 attempts in 15 minutes per account | `429` `ErrTooManyAttempts` |
| SSH key (SFTP on the panel's own node, and `POST /auth/sftp-key` for remote nodes) | 10 attempts in 15 minutes per account, on a counter of its own shared by every node | The SFTP sign-in is refused (`ErrTooManyAttempts` on the panel's own node, `429` to a remote node) |
| Password reset emails | 5 per address per hour | `204`, no email sent |

- The counter belongs to the **account**, not to the IP, so it works the same behind a proxy. Variants of the same email that the database treats as equal (case, accents) share one counter.
- For an email with no account the counter is per address typed, and the answer takes as long as a wrong password, so timing does not reveal which emails exist.
- A successful sign-in resets the counter. It lives in memory and also resets when the panel restarts.
- SSH key checks have their own counter, apart from the password one: an SFTP client offers each of its keys in turn, and every key offered counts as one attempt, so those misses must not lock the owner out of signing in with their password. The panel's own node and the remote nodes (through `POST /auth/sftp-key`) count on the same counter, so the limit is the same on every node.
- After the 2FA step succeeds, the pending sign-in state in the `session` cookie is cleared: the same cookie cannot be used to try more codes.

### Passwords

- Stored with bcrypt (cost 10), with a new salt every time.
- Must be 8 to 72 bytes (bcrypt's limit; accented letters and emoji take more than one byte).
- Recommended: 12 characters or more, a password manager, and 2FA on every administrator account.

### Two-step verification (2FA)

6-digit TOTP codes, compatible with Google Authenticator, Authy, Microsoft Authenticator and similar apps.

**Turning it on**: **Account Settings → Two-factor authentication → Enable 2FA**. Scan the QR code and type the code the app shows. Until you confirm it, 2FA is not on.

Once confirmed, the panel shows **10 recovery codes** of 12 characters. Keep them: the panel only stores their hash and cannot show them again. Each one works **once** in place of the app's code.

| Action | What it asks for | What happens |
|--------|------------------|--------------|
| Regenerate codes | A code from the app or a recovery code | The old ones stop working. |
| Turn off | A code from the app or a recovery code | The secret and the codes are deleted. |
| Change phone | Turn it off and on again | You cannot enrol over an active one (`ErrOtpAlreadyEnabled`). |

If email is configured, the panel sends a notice when 2FA is turned on, turned off and when codes are regenerated.

**If both the phone and the codes are lost**, an administrator removes it on the server with `SkyPanel user edit` → **Remove 2FA** (see [CLI](../backend/07-cli.md)).

**When signing in**, `POST /auth/login` answers `{"otpNeeded": true}` and the code goes to `POST /auth/otp` in the same browser session, within **5 minutes**. After that the password must be typed again (`ErrSessionExpired`).

```bash
# 1. Password (plus the anti-bot token): answers {"otpNeeded": true}
curl -c cookies.txt -X POST https://panel.example.com/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email": "ana@example.com", "password": "<your-password>", "captchaToken": "<token>"}'

# 2. The app's code (or a recovery code), with the same cookie
curl -b cookies.txt -X POST https://panel.example.com/auth/otp \
  -H 'Content-Type: application/json' \
  -d '{"token": "123456"}'
```

| Method and path | Body | Response |
|-----------------|------|----------|
| `GET /api/self/otp` | — | `{"otpEnabled": true}` |
| `POST /api/self/otp` | — | `{"secret": "...", "img": "data:image/png;base64,..."}` |
| `PUT /api/self/otp` | `{"token": "123456"}` | `{"recoveryCodes": ["...", ...]}` |
| `POST /api/self/otp/recovery` | `{"token": "123456"}` | `{"recoveryCodes": ["...", ...]}` |
| `DELETE /api/self/otp/{token}` | — | `204` |

All of them need the `self.edit` permission.

SFTP does not ask for the 2FA code: it signs in with the password or with an [SSH key](#ssh-keys-for-sftp). Users with 2FA on should use SSH keys, which work on every node (see [Known limitations](#known-limitations)).

### Taking away the login permission

The panel checks the `login` permission on **every request**, not only at sign-in. It counts `login` from the user's panel-wide permissions or from their role; `login` on a single server does not count.

If you take it away from a user (or take away the role that gave it), their session ends on their next request: the panel deletes it and answers `401`.

Other things that end sessions:

| What | Sessions ended |
|------|----------------|
| The user changes their password in their profile | All but the current one |
| An administrator sets a new password for them | All |
| The password is reset with the email link | All |
| An OAuth2 client is deleted | All of that client's sessions |

Taking away `login` also closes SFTP: the panel checks it on every connection, by password or SSH key, as well as `server.sftp`. Connections already open are checked again every 30 seconds, so deleting the user or taking away `login`, `server.sftp` or their access to the server (or removing the SSH key a connection signed in with) ends them within that time; if the panel's database (or, on a remote node, the panel) cannot be reached, open connections stay until it answers.

### SSH keys for SFTP

A user can sign in to SFTP with a public key instead of their password. The username is still `email#serverID`.

**Adding a key**: **Account Settings → SSH keys → Add key**. Paste the line from your `.pub` file (for example `~/.ssh/id_ed25519.pub`). If you leave the name empty, the key's comment is used.

| Rule | Detail |
|------|--------|
| Accepted types | `ssh-ed25519`, ECDSA (256, 384 and 521), security keys (`sk-ssh-ed25519@openssh.com`, `sk-ecdsa-sha2-nistp256@openssh.com`) and RSA of 2048 bits or more. |
| Refused | DSA, certificates, lines with options (`from=`, `command=`…) and private keys. |
| Name | Up to 100 characters. |
| Maximum | 25 keys per account. The same key cannot be added twice. |

The panel shows each key's SHA256 fingerprint, the same one `ssh-keygen -l` prints.

**Connecting**:

```bash
sftp -P 5657 -i ~/.ssh/id_ed25519 -o User='ana@example.com#1a2b3c4d' panel.example.com
```

The user still needs `server.sftp` on that server (or to own it). Deleting a key stops new connections with it and, within 30 seconds, ends the ones it opened; deleting the user deletes their keys.

Keys work on every node:

- **Panel's own node**: the SFTP server checks the key in the panel's database.
- **Remote node**: the node does not have the users' keys, so it asks the panel. It sends `POST /auth/sftp-key` to the panel set in its `daemon.auth.url` (the `/oauth2/token` part, if present, is replaced by `/auth/sftp-key`; a path prefix is kept), signed with its own secret (`Authorization: Bearer <daemon.auth.clientSecret>`), with the login and the offered public key. The panel applies the same rules as on its own node, and also checks that the server is on the node asking. It answers only the granted scope (`<serverID>:server.sftp`), never a session. Every refused login gets the same answer (`400`, `no access`), so it does not reveal whether the account, the server or the key was wrong. The node waits at most 15 seconds for the answer. The SSH protocol then makes the client prove it holds the private key before anything is granted. Details of the route are in the [authentication API reference](../reference/api/02-authentication.md).

A remote node running a version from before this check only accepts passwords: update it to use keys there.

| Method and path | Body | Response |
|-----------------|------|----------|
| `GET /api/self/sshkeys` | — | List of keys |
| `POST /api/self/sshkeys` | `{"name": "Laptop", "publicKey": "ssh-ed25519 AAAA... ana@laptop"}` | The created key |
| `DELETE /api/self/sshkeys/{id}` | — | `204` |

```json
{
  "id": 3,
  "name": "Laptop",
  "fingerprint": "SHA256:…",
  "type": "ssh-ed25519",
  "createdAt": "2026-10-26T16:00:00Z"
}
```

All of them need the `self.edit` permission. Errors: `ErrSSHKeyInvalid`, `ErrSSHKeyIsPrivate`, `ErrSSHKeyWeak`, `ErrSSHKeyExists`, `ErrSSHKeyLimit`, `ErrSSHKeyName`.

### Password reset

- The **Forgot password** form goes through the anti-bot check.
- It always answers `204`, whether the account exists or not and even when the email limit is reached.
- The email goes to the address stored on the account, not to what was typed in the form.
- The link is built from the panel's **Base URL** (`panel.settings.masterUrl`), never from the request's headers. Set it: otherwise the link uses `http://localhost:8080` and the panel warns about it in the log.
- The link expires after **30 minutes** and works **once**. Asking for another cancels the previous one. Only its hash is stored in the database.
- The new password must be 8 to 72 bytes. This is checked before the link is spent, so a password that is too short can simply be typed again.
- Once changed, every session of the account ends and a notice email is sent.

## Web protections

Every answer of the panel goes through these middlewares, in this order: CORS, security headers, same-origin guard, then panic recovery.

### Same-origin guard

A request that **carries a panel cookie** (`skypanel_auth` or `session`) and **changes something** (any method other than `GET`, `HEAD` and `OPTIONS`, plus every websocket handshake) must come from the panel's own pages. The panel checks it like this:

1. If the browser sent `Sec-Fetch-Site` (every current browser does, and pages cannot forge it), it must be `same-origin` or `none`.
2. Otherwise, if there is an `Origin` header, its host must match the request's `Host`, the `X-Forwarded-Host` header, or the host of the Base URL.
3. A request with neither header did not come from a web page and is let through.

Anything else gets `403 {"error": "cross-origin request refused"}`. Requests without cookies (bearer tokens, OAuth2 clients, nodes, billing API keys) are not affected.

This is what `SameSite=Lax` alone does not cover: for a browser, `http://IP:25566` (a customer's game server) and `http://IP:8080` (the panel) are the **same site**, so a form or a websocket opened from a page on a game server would carry the administrator's cookie. Before this guard, such a page could create an administrator account.

### Security headers

| Header | Value | Why |
|--------|-------|-----|
| `Content-Security-Policy` | `frame-ancestors 'self'; object-src 'none'; base-uri 'self'` | No other site can frame the panel (clickjacking). |
| `X-Frame-Options` | `SAMEORIGIN` | The same, for older browsers. |
| `X-Content-Type-Options` | `nosniff` | Browsers do not guess content types. |
| `Referrer-Policy` | `same-origin` | Full URLs (some carry tokens, like world map links) do not leak to other sites. |
| `Strict-Transport-Security` | `max-age=31536000` | Only over HTTPS (or with `X-Forwarded-Proto: https`): once seen, the browser keeps to HTTPS for a year. |

Pages that show untrusted content (extension pages, world maps) replace the policy with their own sandbox.

### CORS and other details

- CORS accepts any origin with `Authorization`, `Content-Type`, `Accept` and `Origin` headers, but **never with credentials**: other sites can call the API with a bearer token, never with the user's cookie.
- The API docs at `/swagger/` are only served to signed-in administrators.
- The access log leaves out world map tokens and `token`, `code`, `access_token` and `refresh_token` query values.
- When the panel calls a node, it strips the browser's `Cookie` header: nodes authenticate with the panel's own token.
- The page to return to after signing in is not taken from the address bar, so a crafted link cannot send the user elsewhere after login.

## Permissions

The full model is in the [Scopes reference](./scopes-reference.md). The rules that matter most for security:

- `admin` is everything. Give it only to people who run the panel.
- A server's permission row only holds **server scopes**: a panel scope such as `admin` in it is dropped when saved and ignored when read. See [Sub-users](./scopes-reference.md#sub-users).
- Owners cannot change their server's memory, CPU, disk, IP or ports, even when the template opens those variables. See [Variables only an administrator changes](./scopes-reference.md#variables-only-an-administrator-changes).
- Staff with `users.info.edit` or `users.perms.edit` cannot edit, delete or change the permissions of accounts that hold powers they lack. See [Editing other accounts](./scopes-reference.md#editing-other-accounts).
- With sign-ups closed, only administrators can invite someone without an account to a server.
- Billing API keys only do the actions they were given and cannot open sign-in links to accounts with panel powers. See [API keys](./scopes-reference.md#api-keys).
- A session from an OAuth2 client stays bounded by the client in each route's internal checks too, and the console socket grants its streams from those same scopes. See [Where permissions come from](./scopes-reference.md#where-permissions-come-from).
- A server database's password is only returned to callers with `server.data.edit` on that server. See [Database hosts](../features/database-hosts.md).

## Game server containers

Servers are created in Docker only. The panel's Docker install sets `docker.disallowHost` (`SKYPANEL_DOCKER_DISALLOWHOST=true` in `docker-compose.yml` and in the image), which turns off the environments that run processes directly on the node (`host`, `tty`, `standard`, `bubblewrap`). It is checked when a server is created or redefined, and also on every install and start, so a definition that reached the node another way (a restored backup, an older panel) does not run outside a container either.

Each game server container is hardened like this:

| Protection | Detail | Setting |
|------------|--------|---------|
| Unprivileged user | When the panel runs as root (the Docker install does), the game runs as uid:gid `988:988`. Installs still run as root, because their scripts install packages. A template that names its own user keeps it. Before each start the panel gives the server's files to that user. | `docker.serverUser` (`988:988`; `0:0` keeps root) |
| No container user from customers | A `User` key in the container config is removed when a non-administrator edits the definition or creates a sub-server. | — |
| Dropped capabilities | `setpcap`, `mknod`, `audit_write`, `net_raw`, `dac_override`, `fowner`, `fsetid`, `net_bind_service`, `sys_chroot`, `setfcap`. A container that runs as root (an install) keeps the ones package installs need and only drops `setpcap`, `mknod`, `audit_write`, `net_raw` and `sys_chroot`. The template's own lists are kept. | — |
| No privilege gain | `no-new-privileges`: setuid binaries inside the container do not raise privileges. | — |
| Process cap | Processes and threads per container, so a fork bomb in a plugin cannot take the machine down. A template's own cap wins. | `docker.pidsLimit` (`4096`; `0` removes it) |
| Own `/tmp` | `/tmp` and `/var/tmp` are a folder next to the server's, emptied before each container starts and counted in its disk limit. | — |
| Own network | Game servers go to the `skypanel-servers` network. They reach MariaDB (which joins both networks) but not the panel's network. | `docker.serversNetwork` |
| No downloads from private networks | Template downloads refuse loopback, link-local (cloud metadata) and unspecified addresses always, and private addresses (the Docker networks, the LAN) by default. Some templates take a URL from a variable the owner edits. It also applies to GitHub release assets (`githubdl`), whose address comes from the release. | `security.blockPrivateDownloads` (`true`) |

Other limits that protect the node:

- `panel.backups.maxPerServer` (`10`): backups a server's users can keep on the node (those only in the bucket do not count); administrators are not capped. Backups do not count against the server's disk.
- `daemon.minFreeDiskMB` (`2048`): an install that would start with less free space in the servers folder is refused, before a game download fills the disk.
- The server's disk limit is enforced; a server that already filled it does not start until files are deleted.

## Server files

All file operations (file manager, SFTP, archives, backups, installs) work beneath the server's folder and never follow a link out of it. See [Files and compression](19-files-compression.md) and [openat2 and unshare](20-security-unshare.md) for the mechanics. Recent hardening:

- **Extracting**: links inside an archive are created from their parent folder, opened beneath the root. Links that point outside the folder are left out (game files often hold absolute links such as `steamclient.so`) instead of failing the whole archive.
- **Compressing and backups**: every file is listed and opened beneath the server folder. A folder named with wildcards or swapped for a link cannot pull host files into the archive. Links are stored as links; named pipes are left out.
- **Restoring a backup**: the whole archive is read and every entry checked before any of the server's files are deleted. A backup with a path that leaves the folder is refused.
- **Install downloads** (Mojang, Paper, Forge, Fabric, NeoForge, Sponge, CurseForge, GitHub, generic URLs) are written through the file server and never through a link.
- **Ownership change before a start** walks the folder by file descriptor, so a folder swapped for a link to `/` is not walked into.
- **Server definitions** must be regular files; a named pipe no longer hangs the file manager or the file history.
- **Cloning** a server reads links beneath the source folder.

## Tokens and secrets

| What | How it is protected |
|------|--------------------|
| Session tokens | Random UUID v4; only the SHA-256 hash is stored. |
| Panel ↔ node requests | JWTs signed with Ed25519 (`panel.token`, generated on first start and saved in the config). Request tokens live 15 minutes and are bound to the node they are sent to. The public key is at `/auth/publickey` (JWKS). |
| Node credentials | Each remote node has a client ID `.node_<id>` and a secret. `GET /api/nodes/:id/deployment` shows them and needs `nodes.deploy`. The local node's secret is derived from `panel.sessionKey`. |
| Session cookie key | `panel.sessionKey`, generated on first start. |
| Federated transfer key | `panel.exTransferKey`, an Ed25519 seed generated once and saved in the config. |
| Password reset and recovery codes | Only their hash is stored. |
| Billing API keys | Shown once (`ak_...`); only the hash is stored. |
| Configuration secrets (mail, Gemini, CurseForge, S3, Cloudflare, Turnstile and license keys, Discord webhooks) | Stored in the config file. The settings API never returns them: it answers `********`. See [Panel settings](../features/panel-settings.md#how-it-works). |
| 2FA TOTP secret | Stored unencrypted in the database (see [Known limitations](#known-limitations)). |

The config file (`/etc/SkyPanel/config.json` inside the container, `storage/skypanel-config` on the host) holds these keys: keep it, and the installer's `/etc/SkyPanel/credenciales.txt`, readable by root only, and back them up.

## Firewall and ports

| Port | Service | Open to the Internet? |
|------|---------|-----------------------|
| 80, 443 | Nginx (HTTP, and HTTPS with a domain) | Yes, when you use Nginx. |
| 8080 | Panel | Only without a domain. With a domain the installer binds it to `127.0.0.1`. |
| 5657 | SFTP | Yes, for users who need SFTP. |
| Game server ports | Game servers | Docker publishes them itself, ahead of ufw/firewalld. |
| 3307 | MariaDB (`127.0.0.1:3307` → 3306) | No. Bound to `127.0.0.1` (`DB_BIND_ADDRESS`). |
| 8081 | Adminer (`127.0.0.1:8081`) | No. Use an SSH tunnel: `ssh -L 8081:127.0.0.1:8081 user@server`. |

The installer opens SSH, SFTP, 80/443 and the panel port (only when it is not bound to `127.0.0.1`) with ufw or firewalld. Remember that **Docker's published ports go around ufw**: to limit a published port, bind it to an address (as the installer does with `--bind`) or add rules to the `DOCKER-USER` iptables chain.

## Best practices

- Use a domain with HTTPS for any panel with customers, and set the Base URL.
- Turn on 2FA for every administrator; keep the recovery codes offline. With 2FA on, use SSH keys for SFTP (they work on every node).
- Give `admin` only to people who run the panel. Use roles for staff, with the fewest scopes they need.
- Leave `docker.disallowHost`, `docker.serverUser`, `docker.pidsLimit` and `security.blockPrivateDownloads` at their defaults unless you know why you change them.
- Close public sign-ups (`panel.registrationEnabled`, in **Settings**) if customers come from billing.
- With sign-ups open, keep **Approve new accounts** on (`panel.registrationApproval`): a registered account cannot sign in until an administrator approves it in **Users**. Nobody uses an account under someone else's email without an administrator looking at it first.
- Keep the panel updated (**Settings → Updates**, or `install.sh --update`) and the operating system and Docker too.
- Back up the database, the config folder and the backups folder; test a restore.
- Do not expose MariaDB or Adminer to the Internet.
- Review the panel's activity feed (sign-ins, failed sign-ins, sign-outs, sign-ups, password resets and billing sign-in links, with IP and browser; kept 90 days) and each server's activity.

## Known limitations

- **SFTP and 2FA**: SFTP clients cannot ask for a code, so an SFTP sign-in with a password does not ask for 2FA, and password sign-in to SFTP cannot be turned off. Users with 2FA on should sign in to SFTP with an [SSH key](#ssh-keys-for-sftp), which works on every node that runs the current version.
- **Unencrypted TOTP secret**: the 2FA secret is stored in clear in the database (`users.otp_secret`). Anyone who can read the database can generate codes: protect the database and its backups.

## If you suspect a compromise

1. Take the server off the network or stop the panel (`docker compose stop` in `/opt/skypanel`).
2. Change the administrator passwords and the database passwords; revoke billing API keys and OAuth2 clients.
3. Rotate `panel.sessionKey` and `panel.token` by deleting them from the config and restarting: new keys are generated on start. Remote nodes read the panel's public key from `token.public` (`/auth/publickey`); check that they still accept the panel afterwards and restart them if they do not.
4. Review users, roles and global permissions for anything you did not grant; review server owners.
5. Review the activity feed and the logs (`docker compose logs skypanel`, and the panel log folder).
6. Restore from a known good backup if files were changed, update the panel, and tell affected users.
