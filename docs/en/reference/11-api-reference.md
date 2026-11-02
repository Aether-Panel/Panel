# API reference

Aether Panel is driven by an HTTP API: the web client uses it, and so can scripts, bots and billing systems. This page is the entry point. Each area has its own page with a route table at the top and the details (body, query, response, errors) below.

> The product is called Aether Panel. The Go module, the binary and some identifiers (cookie `skypanel_auth`, `SKYPANEL_*` variables) still use the name SkyPanel.

Everything here was checked against the routes the code registers (`internal/web/api`, `internal/web/auth`, `internal/web/oauth2`, `internal/web/daemon`). When the generated Swagger page (`/swagger/index.html`, administrators only) disagrees with this reference, this reference is right: several annotations were corrected in the code but the served Swagger files have not been regenerated yet, and a few still name scopes that do not exist (see [Conventions → Swagger](./api/01-conventions.md#swagger)).

## Pages

| Page | Contents |
|------|----------|
| [Conventions](./api/01-conventions.md) | Base paths, request and response format, errors, status codes, pagination, headers, CORS, rate limits |
| [Authentication](./api/02-authentication.md) | Session cookie, bearer tokens, OAuth2 clients, API keys, the same-origin guard, scopes, `/auth/*`, `/oauth2/token`, `/api/config` |
| [Servers](./api/03-servers.md) | Server list, create, delete, power, console, stats, definition, variables, ports, users, activity, subdomain, clone, transfer, AI, WebSocket |
| [Server content](./api/04-server-content.md) | Files, archives, file history, plugins, modpacks, world map, backups, tasks, databases |
| [Accounts and users](./api/05-accounts.md) | Your own account (profile, 2FA, OAuth2 clients, SSH keys), user settings, users, roles, access activity |
| [Panel administration](./api/06-panel-admin.md) | Nodes, settings, API keys, templates, game versions, database hosts, extensions, uptime |
| [Provisioning / billing](./api/07-provisioning.md) | Products and the `/api/v1` routes for WHMCS, Paymenter and similar |
| [External transfer](./api/08-external-transfer.md) | Copying a server's files from another Aether Panel |
| [Daemon](./api/09-daemon.md) | The `/daemon` routes the panel calls on each node |

## Quick start

Create an OAuth2 client in **Account Settings → OAuth** (or with `POST /api/self/oauth2`), exchange it for a token and send the token as a bearer header. The client acts with all the permissions of your account:

```bash
TOKEN=$(curl -s -X POST https://panel.example.com/oauth2/token \
  -d grant_type=client_credentials \
  -d client_id=CLIENT_ID \
  -d client_secret=CLIENT_SECRET | jq -r .access_token)

curl -s -H "Authorization: Bearer $TOKEN" https://panel.example.com/api/servers
curl -s -X POST -H "Authorization: Bearer $TOKEN" https://panel.example.com/api/servers/a1b2c3d4/start
```

Requests with a bearer token are not affected by the [same-origin guard](./api/02-authentication.md#same-origin-guard); requests that carry the panel's cookies are.

## Route map

Each area below links to the page that documents it.

### Authentication

`/auth/login`, `/auth/otp`, `/auth/logout`, `/auth/register`, `/auth/forgot-password`, `/auth/reset-password`, `/auth/reauth`, `/auth/sso`, `/auth/challenge`, `/auth/publickey`, `/oauth2/token` and the public `GET /api/config`. See [Authentication](./api/02-authentication.md).

### Servers

`/api/servers` and `/api/servers/:serverId/...`: list, create, delete, suspend, rename, definition, variables, port settings, flags, power actions, console, stats, status, query, metrics, crash reports, game version, notifications, users and owner, activity log, subdomain, clone, node transfer, AI. See [Servers](./api/03-servers.md).

### WebSocket

`GET /api/servers/:serverId/socket` streams the console, stats and status. See [Servers → WebSocket](./api/03-servers.md#websocket).

### Files

`/api/servers/:serverId/file/*`, `archive/*`, `extract/*` and `file-history`. See [Server content → Files](./api/04-server-content.md#files).

### Plugins and modpacks

`/api/servers/:serverId/plugins` and `/modpacks`. See [Server content](./api/04-server-content.md#plugins).

### World map

`/api/servers/:serverId/map` and the signed viewer `/api/map-view/...`. See [Server content → World map](./api/04-server-content.md#world-map).

### Backups

`/api/servers/:serverId/backup/...`: list, create (`409 ErrBackupLimit` past `panel.backups.maxPerServer`), restore, download, delete. See [Server content → Backups](./api/04-server-content.md#backups).

### Tasks

`/api/servers/:serverId/tasks/...`. See [Server content → Tasks](./api/04-server-content.md#tasks).

### Server databases

`/api/servers/:serverId/databases` and `/databasehosts`. See [Server content → Databases](./api/04-server-content.md#databases).

### Account

`/api/self/...` (profile, 2FA, OAuth2 clients, SSH keys) and `/api/userSettings`. See [Accounts and users](./api/05-accounts.md).

### Users and roles

`/api/users`, `/api/roles` and the access log `/api/activity`. See [Accounts and users](./api/05-accounts.md#users).

### Nodes

`/api/nodes/...`, including the port check and the network history. See [Panel administration → Nodes](./api/06-panel-admin.md#nodes).

### Settings

`/api/settings/...`: values, tests (email, Discord, S3, Cloudflare), license, updates, domain status. See [Panel administration → Settings](./api/06-panel-admin.md#settings).

### API keys

`/api/settings/apikeys`. See [Panel administration → API keys](./api/06-panel-admin.md#api-keys).

### Templates

`/api/templates/...` and `/api/game-versions/:platform`. See [Panel administration → Templates](./api/06-panel-admin.md#templates).

### Database hosts

`/api/databasehosts/...`. See [Panel administration → Database hosts](./api/06-panel-admin.md#database-hosts).

### Extensions

`/api/extensions/...` and `/api/extension-files/...`. See [Panel administration → Extensions](./api/06-panel-admin.md#extensions).

### Uptime

`/api/uptime` and `/api/uptime/:serverId`. See [Panel administration → Uptime](./api/06-panel-admin.md#uptime).

### Provisioning / billing

`/api/provision/products` and `/api/v1/*`. See [Provisioning / billing](./api/07-provisioning.md).

### External transfer

`/api/servers/:serverId/extransfer/*` and the public `/api/extransfer/*`. See [External transfer](./api/08-external-transfer.md).

### Daemon

`/daemon`, `/daemon/features`, `/daemon/system`, `/daemon/ports`, `/daemon/network/history`, `/daemon/update`, `/daemon/update-status` and `/daemon/server/:serverId/...`. See [Daemon](./api/09-daemon.md).

## Related documentation

The previous single-file version of this page also carried internal details that are not part of the HTTP API. They are documented here:

- Configuration keys and environment variables: [Configuration](../backend/02-configuration.md)
- Command line: [CLI commands](./14-cli-commands.md)
- Database tables: [Database](../backend/03-database.md)
- Server definitions, environments, scheduler, disk limit: [Servers](../backend/04-servers.md)
- Middleware and routing internals: [API layer](../backend/05-api-layer.md)
- WebSocket internals (trackers, console buffer): [WebSocket](../backend/08-websocket.md)
- JWT, SFTP and API key internals: [Security, SFTP, JWT and API keys](../backend/09-security.md)
- How the web client calls the API: [API consumption from the frontend](../frontend/02-endpoints.md)
- Permission names and who gets them: [Permissions reference](../general/scopes-reference.md)
- Extension packages and events: [Extensions](./15-extensions.md)
