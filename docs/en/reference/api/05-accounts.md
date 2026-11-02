# Accounts and users

Your own account (`/api/self`), per-user preferences (`/api/userSettings`), user administration (`/api/users`), roles (`/api/roles`) and the access log (`/api/activity`). Back to the [API reference](../11-api-reference.md).

## Routes

| Method | Path | Scope | What it does |
|--------|------|-------|--------------|
| GET | `/api/self` | `login` | Your account |
| PUT | `/api/self` | `self.edit` | Changes your username, email or password |
| GET | `/api/self/otp` | `self.edit` | Whether 2FA is on |
| POST | `/api/self/otp` | `self.edit` | Starts 2FA enrollment |
| PUT | `/api/self/otp` | `self.edit` | Confirms enrollment, returns recovery codes |
| POST | `/api/self/otp/recovery` | `self.edit` | New recovery codes |
| DELETE | `/api/self/otp/:token` | `self.edit` | Turns 2FA off |
| GET | `/api/self/oauth2` | `self.clients` | Your OAuth2 clients |
| POST | `/api/self/oauth2` | `self.clients` | Creates an OAuth2 client |
| DELETE | `/api/self/oauth2/:clientID` | `self.clients` | Deletes one |
| GET | `/api/self/sshkeys` | `self.edit` | Your SFTP public keys |
| POST | `/api/self/sshkeys` | `self.edit` | Adds a key |
| DELETE | `/api/self/sshkeys/:id` | `self.edit` | Removes a key |
| GET | `/api/userSettings` | `login` | Your preferences |
| PUT | `/api/userSettings/:key` | `login` | Saves one preference |
| GET | `/api/users` | `users.info.search` | Searches users (paged) |
| POST | `/api/users` | `users.info.edit` | Creates a user |
| GET | `/api/users/:id` | `users.info.view` | One user |
| POST | `/api/users/:id` | `users.info.edit` | Updates a user |
| DELETE | `/api/users/:id` | `users.info.edit` | Deletes a user |
| GET | `/api/users/:id/perms` | `users.perms.view` | The user's panel-wide scopes |
| PUT | `/api/users/:id/perms` | `users.perms.edit` | Sets them |
| GET | `/api/roles` | any of `admin`, `users.info.view`, `users.info.edit` | Lists roles |
| POST | `/api/roles` | `admin` | Creates a role |
| GET | `/api/roles/:id` | `admin` | One role |
| POST | `/api/roles/:id` | `admin` | Updates a role |
| DELETE | `/api/roles/:id` | `admin` | Deletes a role |
| GET | `/api/activity` | `users.info.search` | Access log: sign-ins and account creations (paged) |

## The user object

```json
{
  "id": 3,
  "username": "alice",
  "email": "alice@example.com",
  "otpActive": false,
  "roleId": 2,
  "roleName": "Usuario",
  "scopes": ["login", "server.create", "templates.view"]
}
```

`scopes` are the panel-wide scopes the account holds: its own permission row plus its role's. `roleName` is only filled when the role was loaded with the user.

## Your account

### `GET /api/self`

Your user object.

### `PUT /api/self`

```json
{ "username": "alice", "email": "alice@example.com", "password": "current password", "newPassword": "optional new one" }
```

`password` (your current one) is always required. Empty `username` and `email` keep the current values. When the email changes, the old address is told; when `newPassword` is set (8 bytes or more), every other session of the account ends and a confirmation email is sent. `204`.

| Error | When |
|-------|------|
| `400 ErrFieldRequired` | No `password` |
| `400` | Username or email not valid |
| `400 ErrPasswordRequirements` | `newPassword` too short or too long |
| `500 ErrInvalidCredentials` | Wrong current password (the code answers `500` here) |

### Two-factor authentication

| Route | Body | Answer |
|-------|------|--------|
| `GET /api/self/otp` | | `{"otpEnabled": false}` |
| `POST /api/self/otp` | | `{"secret": "BASE32...", "img": "data:image/png;base64,..."}`: the secret and its QR code; `400 ErrOtpAlreadyEnabled` when 2FA is on |
| `PUT /api/self/otp` | `{"token": "123456"}` | `{"recoveryCodes": ["...", "..."]}`: 2FA is now on; `400 ErrInvalidCredentials` for a wrong code |
| `POST /api/self/otp/recovery` | `{"token": "123456"}` | New `recoveryCodes`, the old ones stop working |
| `DELETE /api/self/otp/:token` | | `204`; `:token` is a current code or a recovery code |

Each change sends an email to the account. Recovery codes also work at `POST /auth/otp`, once each.

### OAuth2 clients

`POST /api/self/oauth2`:

```json
{ "name": "Discord bot", "description": "Restarts the server on command" }
```

```json
{ "client_id": "0f8fad5b-d9cb-469f-a165-70867728950e", "client_secret": "36 random characters", "name": "Discord bot", "description": "Restarts the server on command" }
```

The secret is only shown in this answer. The client acts with all the permissions of your account (see [OAuth2 client credentials](./02-authentication.md#oauth2-client-credentials)); the API takes no scope list or server for it. `GET /api/self/oauth2` lists your clients (with an empty `client_secret`). `DELETE /api/self/oauth2/:clientID` deletes one and ends the sessions it opened; `404` for a client that is not yours. Creating and deleting send an email.

### SSH keys

Public keys you can sign in to SFTP with instead of your password; the SFTP username stays `email#serverId`. They work on every node: the panel's own node checks them in its database, and remote nodes ask the panel with [`POST /auth/sftp-key`](./02-authentication.md#post-authsftp-key).

`POST /api/self/sshkeys`:

```json
{ "name": "laptop", "publicKey": "ssh-ed25519 AAAAC3Nza... me@laptop" }
```

```json
{ "id": 7, "name": "laptop", "fingerprint": "SHA256:...", "type": "ssh-ed25519", "createdAt": "2026-11-01T09:00:00Z" }
```

`GET /api/self/sshkeys` lists them; `DELETE /api/self/sshkeys/:id` removes one (`204`).

| Error (`400`) | When |
|---------------|------|
| `ErrSSHKeyInvalid` | Not one valid public key line |
| `ErrSSHKeyIsPrivate` | A private key was pasted |
| `ErrSSHKeyWeak` | RSA under 2048 bits |
| `ErrSSHKeyExists` | Already on your account |
| `ErrSSHKeyLimit` | 25 keys already |
| `ErrSSHKeyName` | Name missing or over 100 characters |

## User settings

Free-form preferences of the signed-in user, such as the theme. `GET /api/userSettings` answers `[{"key": "theme", "value": "dark"}]`. `PUT /api/userSettings/:key` takes `{"value": "dark"}` (a string) and answers `204`.

## Users

### `GET /api/users`

| Query | Default |
|-------|---------|
| `username` | `*` (wildcard) |
| `email` | `*` |
| `page`, `limit` | `1`, `20` (at most 100) |

```json
{ "users": [ { "id": 3, "username": "alice", "...": "..." } ], "paging": { "page": 1, "pageSize": 20, "maxSize": 100, "total": 1 } }
```

### `POST /api/users`

```json
{ "username": "bob", "email": "bob@example.com", "password": "...", "roleId": 2 }
```

`username` (1 to 100 printable characters), `email` and `password` are required; `roleId` is only applied when the caller is an administrator. Answers the new user object and records `register` in the access log. Validation errors answer `400 {"error": "..."}`. The password strength rule of sign-up is not applied here.

### `GET /api/users/:id`

The user object; `404` when unknown.

### `POST /api/users/:id`

Same body as creation, every field optional; empty fields keep their value. Setting `password` ends all the user's sessions. When the caller is an administrator the role is set to the `roleId` sent: leaving it out removes the role. `204`.

A caller without `admin` cannot edit or delete an account holding a panel-wide power they lack (`admin`, `settings.edit`, node, template and user management scopes, `oauth2.auth`, or a global `server.admin`): `403`.

### `DELETE /api/users/:id`

`204`; same restriction. The account's permission rows, OAuth2 clients, sessions, recovery codes, password resets, SSH keys and user settings are deleted with it, and its servers are left without an owner.

### `GET /api/users/:id/perms`

The user's own panel-wide permission row (the role is not included): `{"scopes": ["login", "server.create"]}`.

### `PUT /api/users/:id/perms`

`{"scopes": ["login", "server.create"]}`. An administrator sets the list as sent. Anyone else can only add or remove panel-wide scopes they hold themselves (never server scopes, which would apply on every server) and cannot edit a stronger account. `204`.

## Roles

A role is a named list of scopes given to users with `roleId`. Its server scopes do not apply on servers (see the [Permissions reference](../../general/scopes-reference.md)).

```json
{ "id": 2, "name": "Usuario", "description": "Default customer role", "scopes": ["login", "templates.view", "uptime.view"], "createdAt": "...", "updatedAt": "..." }
```

- `POST /api/roles` takes `name` (1 to 100 printable characters, unique), `description` and `scopes`; answers the role.
- `POST /api/roles/:id` replaces those fields; answers the role.
- `DELETE /api/roles/:id` detaches the role from its users and deletes it; `204`. The roles named `admin`, `Administrador` and `Usuario` cannot be deleted (`400`); `404` when unknown.

## Access activity

### `GET /api/activity`

Sign-ins and account creations, newest first. Query: `event`, `page` (default 1), `limit` (default 10, at most 100).

```json
{
  "activity": [ { "id": 51, "event": "login", "username": "alice", "email": "alice@example.com", "ip": "203.0.113.7", "userAgent": "Firefox on Linux", "createdAt": "2026-11-01T08:59:00Z" } ],
  "paging": { "page": 1, "pageSize": 10, "maxSize": 100, "total": 51 }
}
```

Recorded events: `login`, `login_failed` (known accounts only), `login_sso` and `register`. `userAgent` is reduced to "browser on platform". Details: [Activity and access log](../../features/activity-and-access-log.md).
