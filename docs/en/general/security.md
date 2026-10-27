# Security & Best Practices

## Security Introduction

Security is a fundamental priority in Aether Panel. This document covers the built-in security features and best practices for keeping your installation secure.

Aether Panel includes multiple security layers: Bearer token and HttpOnly cookie authentication, SHA256-hashed sessions, bcrypt encryption for passwords, Ed25519-signed JWTs, a system of 74 granular scopes, and TOTP 2FA support. The panel does not handle TLS directly — you must use a reverse proxy like Nginx or Caddy for HTTPS.

## Authentication

The panel implements a multi-layer authentication system. Tokens are sent as Bearer in the Authorization header, or alternatively in the `skypanel_auth` cookie. Sessions are stored in the database with SHA256-hashed tokens.

### Authentication Methods

#### Web Sessions

The panel uses cookie-based sessions with Gin sessions. On login, a UUID v4 session token is generated. The token is SHA256-hashed before database storage. The `skypanel_auth` cookie is configurable: path, domain, maxAge, Secure, HttpOnly, and SameSite. Sessions expire by default in 1 hour.

#### Bearer Tokens (API)

For API access, the panel prioritizes the `Authorization: Bearer <token>` header. If no header is present, it falls back to the `skypanel_auth` cookie. The token is validated against the database, checking that it exists and its `expiration_time` is in the future.

#### Integrated OAuth2 Server

The panel includes an OAuth2 server. The endpoint is at `/oauth2/token` (generate token). It uses the Client Credentials flow (panel↔node authentication) and Password (SFTP authentication). Public keys are available at `/auth/publickey` in JWKS format. Tokens are Ed25519 (EdDSA) signed JWTs.

- Client Credentials authentication for integrations
- Ed25519-signed JWT tokens (auto-generated key on first start)
- JWKS endpoint for third-party signature verification
- Granular scopes per OAuth2 client
- Token revocation and secret regeneration

#### Two-Factor Authentication (2FA/TOTP)

Aether Panel supports TOTP (Time-based One-Time Password) for two-factor authentication. How to turn it on, recovery codes and the API are in [Two-step verification (2FA)](#two-step-verification-2fa).

### Password Security

Passwords in Aether Panel are protected with:

- bcrypt hashing (default cost: 10) — never stored in plain text
- Each SetPassword() call generates a new hash with automatic salt
- Passwords must be between 8 and 72 bytes (bcrypt's limit)

#### Password Recommendations

- Minimum 12 characters (recommended 16+)
- Mix of uppercase, lowercase, numbers, and symbols
- Don't reuse passwords from other services
- Use a password manager (Bitwarden, 1Password, etc.)
- Rotate passwords regularly, especially admin accounts

## Sign-in and account

The sign-in, sign-up and password reset forms go through an anti-bot check. It is described in [Captcha / anti-bot](17-turnstile.md).

### Attempt limits

| What | Limit | Answer once over it |
|------|-------|---------------------|
| Password (panel and SFTP share the counter) | 10 failures in 15 minutes per account | `429` `ErrTooManyAttempts` |
| 2FA code | 10 failures in 15 minutes per account | `429` `ErrTooManyAttempts` |
| Password reset emails | 5 per address per hour | `204`, no email sent |

The counter is per account (email), not per IP, so it works the same behind a proxy. It lives in memory: it resets when the panel restarts. A successful sign-in resets it for that account.

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

SFTP does not ask for the 2FA code: it signs in with the password or with an [SSH key](#ssh-keys-for-sftp).

### Taking away the login permission

The panel checks the `login` permission on **every request**, not only at sign-in. It counts `login` from the user's panel-wide permissions or from their role; `login` on a single server does not count.

If you take it away from a user (or take away the role that gave it), their session ends on their next request: the panel deletes it and answers `401`.

Other things that end sessions:

| What | Sessions ended |
|------|----------------|
| The user changes their password in their profile | All but the current one |
| An administrator sets a new password for them | All |
| The password is reset with the email link | All |

Taking away `login` also closes SFTP: the panel checks it on every connection, by password or SSH key, as well as `server.sftp`.

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

The user still needs `server.sftp` on that server (or to own it). Deleting a key stops new connections with it; deleting the user deletes their keys.

Keys only work on the node the panel runs on. Remote nodes check SFTP sign-ins against the panel with username and password, so on their servers you sign in with the password.

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

## Authorization & Permissions

The panel uses a permission system based on 74 granular scopes. Each API route requires one or more specific scopes. The `RequiresPermission` middleware verifies scopes before processing each request.

### Scope System

Scopes control access to specific features. Examples of the 74 defined scopes (50 server-specific + 24 global):

- admin — Full administrative access (inherits all scopes)
- server.view — View server information
- server.start / server.stop / server.kill — Server state control
- server.console / server.console.send — Console access
- server.files.view / server.files.edit — File management
- server.backup.create / server.backup.restore — Backups
- server.sftp — SFTP access
- nodes.view / nodes.create / nodes.edit / nodes.delete — Node management
- users.info.view / users.info.edit — User management

#### Scope Hierarchy

The `admin` scope grants all global permissions. The `server.admin` scope grants all permissions on a specific server. Scopes marked as `forServer` require a server ID in the route. Users can have global permissions (affect the entire instance) and per-server permissions.

### Roles & Groups

Roles group multiple scopes for mass assignment to users. Permission verification combines: (1) the user's role scopes, (2) the user's global scopes, and (3) per-server scopes.

- Create specific roles for different types of users
- Regularly review the permissions assigned to each role
- Do not grant the `admin` scope unless absolutely necessary

## Token Security

The panel handles several types of tokens with different security levels.

### Session Tokens

Random UUID v4  SHA256 hash  stored in DB. The original token is never persisted, only its hash. Sessions expire in 1 hour and are validated on every request.

### JWT Tokens (OAuth2/Daemon)

Signed with Ed25519 (EdDSA). The private key is randomly generated on first start and stored in the configuration. The public key is exposed via JWKS. Key ID is 'SkyPanel'.

### Node Secrets

Each node has a unique secret generated as a UUID without dashes. The secret is stored in the database and directly compared to authenticate node-panel communication. Local nodes use direct comparison with the LocalNode model.

## Middleware Chain

Each API request passes through a middleware chain that guarantees security:

- CORS — Allows all origins, GET/POST/PUT/DELETE/OPTIONS methods, Authorization/Content-Type/Accept/Origin headers
- Recovery — Catches panics and returns 500 without exposing internal information
- ResponseAndRecover — Error handling with structured JSON responses
- NeedsDatabase — Verifies database connection before processing
- AuthMiddleware — Extracts token from Authorization header or skypanel_auth cookie, validates against DB
- RequiresPermission — Verifies the user has the required scope (returns 403 if not)
- ResolveServerPanel — For routes with :serverId, loads the server from DB
- HasTransaction — Wraps the operation in a DB transaction
- AddVersionHeader — Adds version headers to the response

## Security Best Practices

Follow these recommendations to keep your installation secure:

### Reverse Proxy & HTTPS

- The panel does NOT handle TLS internally. Use Nginx, Caddy, or Traefik as a reverse proxy with HTTPS.
- Configure `security_trusted_proxies` in config.json so the panel trusts your proxy's IPs.
- Configure `security_trusted_proxy_header` if your proxy uses a custom header.
- Auto-renew Let's Encrypt certificates with Certbot.
- Consider Cloudflare Proxy for additional DDoS protection.

### Cookie Configuration

- Set `panel_web_cookies_samesite` to 'Strict' to prevent CSRF.
- Enable `panel_web_cookies_secure` only if using HTTPS (required).
- Set `panel_web_cookies_httponly` to true (default).
- Adjust `panel_web_cookies_age` based on your session policy (default 1 hour).
- Configure `panel_web_cookies_path` and `panel_web_cookies_domain` for your deployment.

### Updates & Maintenance

- Keep the panel updated to the latest version for security patches.
- Update the operating system and Docker regularly.
- Review panel logs for suspicious activity.
- Maintain up-to-date backups of the database and configuration.

### Passwords & Authentication

- Use strong passwords (12+ characters with variety).
- Enable 2FA/TOTP for all administrators.
- Rotate passwords regularly, especially for admin accounts.
- Each user should have their own account — don't share credentials.

### Permissions & Scopes

- Apply the principle of least privilege: only grant necessary scopes.
- Regularly review and audit user permissions.
- Use roles instead of assigning scopes individually.
- Deactivate accounts for users who no longer need access.

### Network & Firewall

- Configure the firewall to only open necessary ports (8080, 5657, 8081).
- Use fail2ban to protect against SSH brute force attacks.
- Consider a VPN for remote administrative access.
- Change the default SSH port from 22.

### Nodes & Daemon

- Keep node secrets secure — they are the access key to the daemon.
- If using remote nodes, secure communication via VPN or SSH tunnel.
- Ensure nodes only expose necessary ports.
- Configure nodes to only accept connections from the panel.


## Firewall Configuration

Ensure your server firewall is properly configured:

### Required Ports

Default panel web port (HTTP). Requires a reverse proxy for HTTPS.

- **port**: 8080/TCP
- **restrict**: Restrict to specific IPs or use Cloudflare Proxy if possible.
SFTP port for file transfer.

- **port**: 5657/TCP
- **restrict**: Only accessible to users who need SFTP.
Gatus monitoring port (uptime). Optional.

- **port**: 8081/TCP
- **restrict**: Can be restricted to internal access.

### UFW Configuration (Ubuntu/Debian)

To configure the firewall with UFW:

```text
# Allow required ports
sudo ufw allow 8080/tcp
sudo ufw allow 5657/tcp
sudo ufw allow 8081/tcp

# Enable firewall
sudo ufw enable

# Check status
sudo ufw status
```

### firewalld Configuration (Fedora/RHEL/CentOS)

To configure the firewall with firewalld:

```text
# Allow required ports
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=5657/tcp
sudo firewall-cmd --permanent --add-port=8081/tcp

# Apply changes
sudo firewall-cmd --reload

# Check rules
sudo firewall-cmd --list-ports
```

## Reverse Proxy & SSL/TLS

Aether Panel listens on plain HTTP. For HTTPS you must use a reverse proxy (Nginx, Caddy, Traefik). Configure `security_trusted_proxies` so the panel logs the real client IPs.

### Let's Encrypt Certificates (via reverse proxy)

Configure an Nginx reverse proxy with Certbot for automatic SSL:

Let's Encrypt certificates expire every 90 days. Certbot renews them automatically.

- Install Nginx: sudo apt install nginx (Ubuntu/Debian)
- Configure Nginx as a reverse proxy to localhost:8080
- Install Certbot: sudo apt install certbot python3-certbot-nginx
- Get certificate: sudo certbot --nginx -d your-domain.com
- Verify auto-renewal: sudo certbot renew --dry-run

### Self-Signed Certificates (Development Only)

Self-signed certificates should only be used in development environments.

- **warning**: Browsers will show security warnings with self-signed certificates.
## Monitoring & Logs

The panel uses a structured logging system with multiple levels (Info, Warn, Error, Debug). Panel logs are written to file and stdout.

### Log Review

Regularly review panel logs to detect:

- Authentication errors and invalid token attempts
- Requests to routes without sufficient permissions (403)
- Node or database connection errors
- Abnormal API activity or suspicious patterns
- Panics recovered by the recovery middleware

#### Log Locations

- **native**: On native installations: according to Go logging system configuration
- **docker**: On Docker: docker compose logs -f skypanel
## Incident Response

If you suspect your installation has been compromised:

- Disconnect the server from the network immediately
- Change all passwords (panel, database, SSH, etc.)
- Review panel logs for suspicious activity
- Revoke all OAuth2 tokens and regenerate client secrets
- Regenerate the panel's private key (ed25519) by removing it from the configuration
- Review and remove any suspicious users or permissions
- Restore from a known safe backup if necessary
- Update the panel and all dependencies
- Notify affected users if appropriate

### Prevention

The best incident response is prevention. Follow all best practices, keep the panel updated, monitor logs, and apply the principle of least privilege when assigning scopes.

## Security Checklist

Use this checklist to ensure your installation is securely configured:

- Panel updated to the latest version
- Operating system and Docker updated
- Reverse proxy configured with HTTPS and valid certificate
- security_trusted_proxies configured correctly
- 2FA/TOTP enabled for all administrators
- Firewall configured (only ports 8080, 5657, 8081 open)
- Strong passwords for all accounts (bcrypt hashed)
- User permissions reviewed and limited to minimum necessary
- Scopes audited regularly
- Logs being monitored (including authentication errors)
- OAuth2 tokens and node secrets protected
- Backups configured and regularly tested
- Cookie session configured with SameSite=Strict and HttpOnly=true
- Panel's Ed25519 private key backed up securely

