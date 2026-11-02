# API conventions

What every route of the panel has in common: where it lives, what it accepts, what it answers and how errors, pages and limits work. Back to the [API reference](../11-api-reference.md).

## Base paths

| Prefix | What it is | Authentication |
|--------|------------|----------------|
| `/api/...` | Panel API | Session cookie or bearer token, except the public routes below |
| `/api/v1/...` | Billing / provisioning API | API key (`ak_...`) |
| `/auth/...` | Sign-in, sign-up, password reset | Public, except `/auth/reauth` |
| `/oauth2/token` | OAuth2 token endpoint | Client credentials (form) |
| `/daemon/...` | Node API, called by the panel | Panel JWT (`/daemon`, `/daemon/features` are public) |
| `/swagger/*` | Generated API docs | Signed-in administrator (`admin`) |

Public routes under `/api` (no session needed):

| Route | Why it is public |
|-------|------------------|
| `GET /api/config` | The sign-in page reads the branding and the anti-bot settings before anyone signs in |
| `/api/extransfer/*` | Called by another panel; each call is signed (see [External transfer](./08-external-transfer.md)) |
| `GET/HEAD /api/map-view/:serverId/:token/*path` | Loaded by a sandboxed iframe; the path carries a signed link (see [World map](./04-server-content.md#world-map)) |
| `GET/HEAD /api/extension-files/:id/*path` | Files of switched-on extensions, loaded by a sandboxed iframe (see [Extensions](./06-panel-admin.md#extensions)) |
| `/api/v1/*` | Uses an API key instead of a session |

Unknown paths under `/api/`, `/oauth2/` and `/daemon/` answer `404` with no body; every other unknown path returns the web client.

## Requests

- Bodies are JSON (`Content-Type: application/json`) unless the route says otherwise.
- Exceptions: `POST /oauth2/token` takes a form (`application/x-www-form-urlencoded`); `POST /api/servers/:serverId/console` takes the command as a plain text body; file uploads take the raw body or a multipart field `file`; `POST /api/templates/upload` takes multipart `files`; `POST /api/extensions` takes multipart `file`.
- IDs in paths: servers use their identifier (letters, digits, `-` and `_`, 1 to 20 characters); users, nodes, roles, backups, database hosts, products and API keys use numeric IDs.
- Text filters on list routes (`name`, `username`, `email`) accept `*` as a wildcard.

## Responses

Successful answers have no envelope: the route returns the object or the array itself. Routes that return nothing answer `204 No Content`.

### Error format

Most errors use this shape:

```json
{
  "error": {
    "code": "ErrFieldRequired",
    "msg": "name is required",
    "metadata": { "field": "name" }
  }
}
```

`code` is stable and meant for programs; `msg` is English text for people; `metadata` is only present for some codes. The codes are defined in `pkg/skypanel/errors.go` (a few next to the route that raises them). The web client does not show `msg`: it looks the code up in its own translations (`apiErrors` in `client/frontend/src/lib/locales/{en,es}.json`) and fills each `{{name}}` in them from `metadata`, joining lists with commas; it falls back to `msg` for a code it does not know.

Exceptions to that shape:

| Case | Answer |
|------|--------|
| Not signed in, or a bad `Authorization` header | `401` with no body and `WWW-Authenticate: Bearer realm=""` |
| Missing scope | `403` with no body |
| [Same-origin guard](./02-authentication.md#same-origin-guard) | `403 {"error": "cross-origin request refused"}` |
| API key middleware | `401 {"error": "Unauthorized - Missing or invalid API Key format"}` or `401 {"error": "Invalid API Key"}` |
| `/oauth2/token` and `/auth/sftp-key` | OAuth2 style: `{"error": "invalid_client", "error_description": "..."}` |
| `/api/extransfer/*` and the server's `extransfer` routes | `{"error_code": "...", "message": "...", "retryable": false}` |
| Some routes (AI, server id validation, ranges, some checks) | `{"error": "plain text"}` |
| Routes forwarded to the node | The node's status and body, passed through as they are, except when the node refuses the panel itself (see [Node authentication failures](#node-authentication-failures)) |

### Internal errors

An unexpected failure (database, file system, network) answers its status (usually `500`) with code `ErrInternal` and the message "internal error, the details are in the panel log". The real reason is only written to the panel log. Errors the panel raises on purpose keep their code even with a `5xx` status (for example `ErrDatabaseNotAvailable`, `ErrNodeHasServers`).

For that reason, routes whose failure the administrator must read (email, Discord, Cloudflare and S3 tests, modpack sources, subdomain changes) answer `422` instead of `502`, so the text reaches the browser.

### Node authentication failures

A node only checks the panel's token; the panel has already checked the user's permissions. So when a node answers `401`, or `403` with a `WWW-Authenticate` header (a proxy in front of the node asking for credentials), the problem is between the panel and the node, not the user's session. The routes that forward to a node (the server proxy routes, server create, edit and delete, backups, the world map install and the provisioning API) then answer `502` with `ErrNodeAuthFailed` instead of passing the `401` on, which would make the browser think its own session ended and send the user back to the sign-in page. The panel log names the node and the likely causes: the node's `daemon.auth.clientSecret` does not match the node's secret in the panel, or one of the two clocks is off. Any other `403`, and every `400` or `409` from the node, is still passed through as it is.

### Status codes

| Code | Meaning in this API |
|------|---------------------|
| `200` | OK, with a body |
| `202` | Accepted; the work goes on in the background (power actions without `?wait`, node transfer, external pull, game version change, modpack and world map installs, clone file copy) |
| `204` | Done, no body |
| `400` | Invalid body, query or value (port out of range, repeated, used by the panel or by another server: `ErrPortOutOfRange`, `ErrPortDuplicated`, `ErrPortUsedByPanel`, `ErrPortTaken`); a sub-server transferred on its own (`ErrTransferSubServer`); or the server's template does not allow the action |
| `401` | No valid session, token or key |
| `403` | Missing permission, cross-origin request refused, suspended server, experimental feature off (`ErrFeatureDisabled`), AI log analysis off (`ErrAIAnalysisDisabled`), sign-ups closed (`ErrInviteSignupsClosed`) |
| `404` | The object does not exist, or the caller may not know it exists |
| `409` | Conflict with the current state: disk full (`ErrDiskLimitExceeded`), port taken, backup cap (`ErrBackupLimit`), server owner (`ErrServerOwner`), database host still in use (`ErrDatabaseHostInUse`), server ports taken on the transfer target (`ErrTransferPortsTaken`), the server already being transferred (`ErrTransferRunning`) or writing a backup (`ErrTransferBackupRunning`), archive name taken (`ErrFileExists`), the assistant's confirmation expired or already answered (`ErrAIConfirmationInvalid`), subdomain taken, another job running |
| `413` | Task body too large |
| `422` | An external service refused (mail server, Discord, Cloudflare, S3, Modrinth/CurseForge) |
| `429` | Too many attempts (`ErrTooManyAttempts`), AI quota, subdomain or notification test sent too often |
| `500` | Internal error (see above) |
| `502` | The node or an upstream service could not be reached (some routes); the node refused the panel's credentials (`ErrNodeAuthFailed`); Modrinth offered a file that is not a plain `.jar` name (`ErrPluginBadFileName`) |
| `503` | External transfer: the origin's archive is not ready yet (`NOT_READY`, retryable) |

### Common error codes

| Code | When |
|------|------|
| `ErrFieldRequired`, `ErrFieldLength`, `ErrFieldTooSmall`, `ErrFieldNotEmail`, `ErrFieldMustBePrintable`, `ErrFieldIsInvalidHost` | Validation of a body or query field (`metadata.field` names it) |
| `ErrInvalidCredentials` | Wrong email, password or 2FA code |
| `ErrTooManyAttempts` | Sign-in throttled (see [Rate limits](#rate-limits)) |
| `ErrLoginNotPermitted` | The account lacks the `login` scope |
| `ErrCaptchaFailed` | Anti-bot check missing or wrong |
| `ErrPasswordRequirements` | Password under 8 bytes or over the bcrypt limit |
| `ErrTokenInvalid` | Password reset token unknown, used or expired |
| `ErrInvalidSession`, `ErrSessionExpired` | The 2FA step has no pending sign-in, or it is older than 5 minutes |
| `ErrNoPermission` | The action is not allowed for this account |
| `ErrUserNotFound`, `ErrServerNotFound` | Unknown user or server |
| `ErrNodeInvalid`, `ErrNodeHasServers` | Unknown node; a node with servers cannot be deleted |
| `ErrNodeAuthFailed` | The node refused the panel's token (`502`, see [Node authentication failures](#node-authentication-failures)) |
| `ErrServerOwner` | The owner of a server cannot be limited or removed |
| `ErrInviteSignupsClosed` | Inviting an unknown email while sign-ups are closed |
| `ErrBackupLimit`, `ErrBackupServerRunning`, `ErrBackupInProgress` | Backup cap reached; restore while running; another backup running |
| `ErrDiskLimitExceeded`, `ErrNodeDiskFull` | Server disk full; node below `daemon.minFreeDiskMB` before an install |
| `ErrFileExists`, `ErrIllegalFileAccess` | Move or archive onto an existing path, or `PUT …/file/*?create=true` onto one; path outside the server folder, or a `DELETE` of the server folder itself |
| `ErrFeatureDisabled` | Experimental feature switched off in the settings |
| `ErrAIAnalysisDisabled` | AI log analysis switched off (`panel.settings.hideAiAnalysis`) |
| `ErrAIConfirmationInvalid` | `POST /ai/chat/confirm` (`409`): the server assistant's request expired, was already answered or replaced, belongs to another user or server, or the file tools were turned off (see [Servers → AI](./03-servers.md#post-apiserversserveridaichatconfirm)) |
| `ErrFileNotFound`, `ErrFileTooLarge`, `ErrFileNotAFolder`, `ErrFileIsAFolder`, `ErrFileSymlink`, `ErrFileNotRegular`, `ErrFileInvalidPath`, `ErrFileOutsideServer`, `ErrFileProtected`, `ErrFileInvalidRequest`, `ErrFilePermission`, `ErrFileNoSpace`, `ErrFileOperationFailed` | Node only: answers of the server assistant's file routes `/daemon/server/:serverId/fs/*` (see [Daemon → Files](./09-daemon.md#files)); the panel turns them into the assistant's step errors |
| `ErrAPIKeyNotAllowed`, `ErrAPIKeyPermissions` | API key lacks the action; key created with no known action |
| `ErrSSONotForAdmins` | Sign-in link asked for a privileged account |
| `ErrDatabaseHostFull`, `ErrDatabaseHostOtherNode`, `ErrDatabaseNameTaken`, `ErrInvalidDatabaseName` | Database creation refused |
| `ErrDatabaseHostInUse` | A database host that still has databases cannot be deleted |
| `ErrTransferPortsTaken` | Node transfer refused: some ports of the server or of its sub-servers are in use on the target node (`metadata.ports` lists them) |
| `ErrTransferRunning`, `ErrTransferBackupRunning`, `ErrTransferSubServer` | Node transfer refused: the server is already being transferred; a backup, restore point or restore of it is being written; a sub-server moves only with its parent |
| `ErrPortOutOfRange`, `ErrPortDuplicated`, `ErrPortUsedByPanel`, `ErrPortTaken` | A server port below 1024, listed twice, used by the panel on that node or by another server there (`metadata.port`: the number, or the number and its template variable, such as `27015 (queryport)`) |
| `ErrPortBusy` | Start refused (`409`): something on the node already holds a host port of the server (`metadata.port`) |
| `ErrPluginNoCompatibleVersion`, `ErrPluginBadFileName` | The Modrinth project has no version for the server's loader and Minecraft version; the file Modrinth names is not a plain `.jar` file name |
| `ErrSubdomainInvalid`, `ErrSubdomainReserved`, `ErrSubdomainTaken`, `ErrSubdomainDisabled`, `ErrSubdomainRateLimited`, `ErrSubdomainNoTarget`, `ErrDNSBaseDomainInvalid` | Subdomain change refused (see [Subdomain](./03-servers.md#subdomain)) |
| `ErrCloudflareNotConfigured`, `ErrCloudflareZoneID`, `ErrCloudflareRecordID`, `ErrCloudflareDomainNotInZone`, `ErrCloudflareUnreachable`, `ErrCloudflareBadAnswer`, `ErrCloudflareRefused`, `ErrCloudflareRefusedCode` | Cloudflare token or zone missing or malformed, or Cloudflare could not be reached, answered something unreadable or refused (`metadata.message` carries Cloudflare's own message, `metadata.cloudflareCode` its code) |
| `ErrDiscordNoWebhooks`, `ErrDiscordTestFailed` | Discord test with no webhook set; the test did not reach some webhooks (`metadata.webhooks`: `alerts`, `reports`, `status`, `transfers`) |
| `ErrNotificationLanguage` | `panel.settings.notificationLanguage` is not `es` or `en` |
| `ErrSSHKeyInvalid`, `ErrSSHKeyIsPrivate`, `ErrSSHKeyWeak`, `ErrSSHKeyExists`, `ErrSSHKeyLimit`, `ErrSSHKeyName` | SSH key refused |
| `ErrOtpAlreadyEnabled` | 2FA enrollment while 2FA is on |
| `ErrInternal` | Unexpected failure, see the panel log |

## Pagination

Lists that can grow without bound are paged. Two styles exist.

**Page and limit.** `GET /api/servers`, `GET /api/users` and `GET /api/activity` take `page` (default `1`) and `limit` (default `20`; `10` for `/api/activity`; at most `100`). The answer carries a `paging` object next to the list:

```json
{
  "servers": [ ... ],
  "paging": { "page": 1, "pageSize": 20, "maxSize": 100, "total": 57 }
}
```

`total` is the number of matching rows across all pages. On `GET /api/servers` a `page` or `limit` of `0` or less answers `400 ErrFieldTooSmall`; a `limit` above `100` is lowered to `100` on all three routes.

**Cursor.** `GET /api/servers/:serverId/activity` takes `limit` (default `50`, at most `100`) and `before` (an entry id). The answer is `{"items": [...], "nextCursor": 123}`; pass `nextCursor` as `before` to get the next page. `nextCursor` is `null` on the last page.

Every other list returns all its rows at once.

## Headers

| Header | Where |
|--------|-------|
| `Cache-Control: no-store` | Every `/api` answer; `/oauth2/token` also sends `Pragma: no-cache` |
| `X-API-Version` | Authenticated `/api` answers: the panel version |
| `Content-Security-Policy: frame-ancestors 'self'; object-src 'none'; base-uri 'self'` | Every answer (extension files and the world map viewer send a sandbox policy instead) |
| `X-Frame-Options: SAMEORIGIN`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin` | Every answer |
| `Strict-Transport-Security: max-age=31536000` | When the request arrived over HTTPS, directly or with `X-Forwarded-Proto: https` |

### CORS

The panel answers cross-origin requests from any origin but never with credentials (`Access-Control-Allow-Credentials` is not sent). Allowed request headers: `Authorization`, `Content-Type`, `Accept`, `Origin`; methods: `GET`, `POST`, `PUT`, `DELETE`, `OPTIONS`. In practice another website can call the API with a bearer token, never with a visitor's session cookie. Browser preflights are answered by the CORS layer; a plain `OPTIONS` request to a route answers `200` with an `Allow` header listing its methods.

## Rate limits

All counters live in memory and reset when the panel restarts.

| What | Limit | Answer |
|------|-------|--------|
| Password and 2FA attempts (panel sign-in, OAuth2 password grant, SFTP) | 10 per account per 15 minutes; a success resets the counter | `429 ErrTooManyAttempts` |
| SFTP key checks for remote nodes (`POST /auth/sftp-key`) | 10 per account per 15 minutes, counted apart from passwords; a success resets the counter | `429 {"error": "invalid_request", "error_description": "too many failed attempts, wait a few minutes"}` |
| Password reset requests | 5 per email per hour | `204` as always (the limit is not revealed) |
| AI (log analysis and chat together) | 30 per user per 10 minutes | `429 {"error": "too many AI requests, try again in a few minutes"}` |
| Subdomain changes | One per server every 10 seconds | `429 ErrSubdomainRateLimited` |
| Notification test | One per server every 10 seconds | `429` |
| Built-in anti-bot challenge | Each solution works once and expires after 10 minutes | `400 ErrCaptchaFailed` |

## Swagger

The panel serves generated OpenAPI docs at `/swagger/index.html`. Only a signed-in user with the `admin` scope can open them (the session cookie is checked, so open them from the browser where you are signed in). They come from code annotations. The methods, paths and scopes of several backup, task, definition and world map routes were corrected in the annotations, but the files the panel serves have not been regenerated yet, so they still show the old values; a few annotations still name scopes that do not exist, such as `server.query`. Trust this reference where they differ.
