# External transfer

Routes that copy a server's files from one Aether Panel to another, independent one (also called federated transfer or "ExTransfer"). The panel the server leaves is the **origin**; the panel it arrives at is the **destination**, which pulls the files into a server that already exists there. What the user sees, the limits and troubleshooting are in [Server transfers](../../features/server-transfers.md); moving a server between nodes of the same panel is [`POST /api/servers/:serverId/transfer`](./03-servers.md#post-apiserversserveridtransfer). Back to the [API reference](../11-api-reference.md).

Code: `internal/web/api/extransfer_logic.go` (handlers), `internal/web/api/servers.go` (per-server routes), `internal/models/extransfer.go` (sessions).

## Routes

### On a server (signed in)

Called by a user or script on their own panel. Both sides use the same routes: the origin creates the token, the destination starts the import.

| Method | Path | Scope | What it does |
|--------|------|-------|--------------|
| POST | `/api/servers/:serverId/extransfer/create` | `server.data.edit.admin` | Origin: opens a session and returns a one-time token |
| POST | `/api/servers/:serverId/extransfer/pull` | `server.data.edit.admin` | Destination: starts importing from the origin panel |
| GET | `/api/servers/:serverId/extransfer/status` | `server.data.edit.admin` | Progress of the import here and the newest session created here |
| POST | `/api/servers/:serverId/extransfer/cancel` | `server.data.edit.admin` | Cancels the import running here and the live sessions created here |

`server.data.edit.admin` is not part of what a server owner gets by default (see the [Permissions reference](../../general/scopes-reference.md)).

### Between panels (public)

Called by the destination panel on the origin panel. They need no session: the first one takes the token, the others a signature made with the destination's key.

| Method | Path | Auth | What it does |
|--------|------|------|--------------|
| POST | `/api/extransfer/validate` | Token | Checks the token and registers the destination's public key |
| POST | `/api/extransfer/consume` | Signature over the nonce | Starts the transfer: the origin archives the server folder |
| POST | `/api/extransfer/heartbeat` | Signature | Keeps the session alive and tells the destination whether to go on |
| GET | `/api/extransfer/download` | Signature | Streams the archive |
| POST | `/api/extransfer/confirm` | Signature | The destination has the data: the session completes |
| POST | `/api/extransfer/cancel` | Signature | The destination gives up: the session is cancelled |

These requests carry no cookies, so the [same-origin guard](./02-authentication.md#same-origin-guard) does not apply. They have no rate limit of their own.

## How the panels trust each other

### The token

`create` generates 32 random bytes and returns them hex-encoded (64 characters). The origin only stores an HMAC-SHA256 of the token, so it cannot be read back from the database. The token is valid for **15 minutes** and is spent by the first successful `validate`: after that the session is no longer in a state `validate` accepts. The one exception is a session that `FAILED` (the origin could not build the archive): it can be validated again with the same token until the session expires.

Creating a token does not revoke earlier ones; each `create` opens a separate session.

### The destination's key

Each panel has an Ed25519 key whose seed is kept in `panel.exTransferKey` (generated on first use and written to the config file, so a restart in the middle of a transfer does not break the session). In `validate` the destination sends its public key; from then on the origin only accepts calls signed with the matching private key, so a token intercepted after validation is useless.

What is signed (keys and signatures are standard base64):

| Call | Signed message |
|------|----------------|
| `consume` | The nonce from `validate` followed by the session ID, with nothing in between |
| `heartbeat` | `HEARTBEAT:<session_id>` |
| `download` | `DOWNLOAD:<session_id>` |
| `confirm` | `CONFIRM:<session_id>` |
| `cancel` | `CANCEL:<session_id>` |

The signature over `DOWNLOAD:<session_id>` does not change, so the same download URL works for every attempt while the session is migrating.

### Reaching the origin

The destination only talks to the origin URL given in `pull`, which must be `http(s)://host[:port]` (no path, query, credentials or fragment; without a scheme `https://` is assumed). `localhost` and private, loopback or unspecified addresses are refused, both in the URL and when the host name resolves. Each call has a 10-second connect timeout and a 30-second overall limit, except the download, which has no overall limit (the session and the heartbeats bound it) but must start answering within a minute.

## Session states and timers

| State | Meaning |
|-------|---------|
| `CREATED` | Token generated, not yet used |
| `VALIDATED` | Token accepted, destination key stored, waiting for `consume` |
| `MIGRATING` | Archive being made and downloaded |
| `COMPLETED` | The destination confirmed it has the data |
| `FAILED` | The origin could not archive the server |
| `CANCELLED` | Cancelled by either side |

| Timer | Value |
|-------|-------|
| Token / session lifetime | 15 minutes from `create` |
| Nonce | 3 minutes from `validate` |
| Heartbeat | Every 20 seconds from the destination; each one moves the expiry to 5 minutes later, never more than **6 hours** after `create`. `consume` also moves it, so a late consume still has time for the first heartbeat |
| Waiting for the archive | The destination asks for the download every 5 seconds while the origin answers `5xx` (`NOT_READY`, `DAEMON_ERROR`) and gives up after **2 hours**; a `4xx` answer stops it at once, and so does a heartbeat reporting the session `CANCELLED` or `FAILED` |
| Outcome shown in `status` | 15 minutes after the session ended or expired |
| Import step shown in `status` | Cleared 10 seconds after the import's last step |

## What moves

Only the files of the server folder:

1. On `consume`, the origin deletes any old `transfer.tar.gz` and `.transfer-partial.tar.gz` in the server folder, archives everything in it to the hidden `.transfer-partial.tar.gz` on its node and, once the archive is complete, renames it to `transfer.tar.gz`. If archiving or the rename fails, the partial file is deleted and the session becomes `FAILED`. The origin server keeps running.
2. The destination streams the archive from the origin's `download` (which reads it from the origin's node) straight into `transfer.tar.gz` on its own node, extracts it into the server folder and deletes it.
3. On `confirm` or a cancel, the origin deletes its `transfer.tar.gz` and any `.transfer-partial.tar.gz` still being written.

The template, variables, limits, members, backups, databases and subdomain do not travel, and the origin server is neither stopped nor deleted. The destination server is not stopped either; extracted files overwrite those with the same name. See [Server transfers → Limits](../../features/server-transfers.md#limits).

`transfer.tar.gz` only exists once the archive is complete, so `download` never sends half an archive: until then it answers `503 NOT_READY` (`retryable: true`) and the destination asks again.

## Routes on a server

### `POST /api/servers/:serverId/extransfer/create`

No body. Answer:

```json
{ "session_id": "6f1c2a9e-0b7d-4c55-9a51-2f0e8c3d7b14", "token": "9e4f...64 hex characters...", "expires_in": 900 }
```

Give the origin panel's URL and the `token` to whoever runs the import. `500 {"error": "Failed to create session"}` when it cannot be stored.

```bash
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  https://origin.example.com/api/servers/a1b2c3d4/extransfer/create
```

### `POST /api/servers/:serverId/extransfer/pull`

Run on the destination panel, on the server that will receive the files.

```json
{ "origin_url": "https://origin.example.com", "token": "TRANSFER_TOKEN" }
```

Both fields are required (`400 {"error": "Invalid JSON payload"}` otherwise); spaces around them are trimmed. The import runs in the background and the answer comes at once: `202 {"status": "ACCEPTED", "message": "Pull process initiated"}`. A bad URL or token is not reported here but in `status`. `409 {"error": "A transfer is already running for this server"}` while another import of this server runs.

In the background the destination calls `validate`, `consume`, sends heartbeats, waits for and streams the `download` to its node, extracts it, deletes the archive and calls `confirm`. When it stops because of an error or because its owner cancelled, it calls the origin's `cancel` (best effort) with the reason. A failed `confirm` is only logged: the import still ends as done.

```bash
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"origin_url": "https://origin.example.com", "token": "TRANSFER_TOKEN"}' \
  https://destination.example.com/api/servers/e5f6a7b8/extransfer/pull
```

### `GET /api/servers/:serverId/extransfer/status`

```json
{
  "status": "Descomprimiendo archivos...",
  "pulling": true,
  "origin": {
    "session_id": "6f1c2a9e-0b7d-4c55-9a51-2f0e8c3d7b14",
    "status": "MIGRATING",
    "expired": false,
    "expires_at": "2026-11-02T10:20:00Z",
    "last_heartbeat_at": "2026-11-02T10:15:20Z",
    "updated_at": "2026-11-02T10:15:20Z"
  }
}
```

| Field | Meaning |
|-------|---------|
| `status` | The import's current step on this panel (destination side), or `""` when there is none |
| `pulling` | Whether an import of this server is running here |
| `origin` | Only when this panel created a session for the server (origin side): the newest one, while it is live and up to 15 minutes after it ended or expired |

The step texts come from the backend in Spanish, whatever the interface language: `Iniciando...`, `Validando conexión con el panel de origen...`, `Iniciando la transferencia en el servidor origen...`, `Esperando a que el origen comprima los archivos...`, `Descargando paquete de datos desde el origen...`, `Subiendo archivos al daemon local (esto puede tardar)...`, `Descomprimiendo archivos...`, `Limpiando archivos temporales...`. The last step is one of:

| Value | Meaning |
|-------|---------|
| `DONE` | The import finished |
| `CANCELLED` | Cancelled on this panel |
| `CANCELLED_BY_ORIGIN` | The origin cancelled, or its session failed |
| `ERROR: ...` | The import failed; the text says why (the messages are listed in [Server transfers → Troubleshooting](../../features/server-transfers.md#troubleshooting)) |

### `POST /api/servers/:serverId/extransfer/cancel`

No body. Stops the import of this server running on this panel and cancels every session of the server created on this panel that is `CREATED`, `VALIDATED`, `MIGRATING` or `FAILED` and has not expired. Downloads of those sessions in progress are cut off and the origin's archive is deleted. The destination learns about it through its next heartbeat.

```json
{ "cancelled_pull": false, "cancelled_sessions": 1 }
```

`409 {"error_code": "NO_ACTIVE_TRANSFER", "message": "There is no transfer in progress for this server", "retryable": false}` when there was nothing to cancel; `500 INTERNAL_ERROR` when the sessions could not be updated.

## Routes between panels

### Errors

These routes answer errors in their own format, not the panel's usual [error format](./01-conventions.md#error-format):

```json
{ "error_code": "INVALID_SIGNATURE", "message": "Invalid signature", "retryable": false }
```

`retryable` is `true` for database errors (`INTERNAL_ERROR`), `DAEMON_ERROR` and `NOT_READY`, `false` for everything else. The structure also defines a `session_id` field, but no route fills it.

Errors shared by `heartbeat`, `download`, `confirm` and `cancel`:

| Status | `error_code` | When |
|--------|--------------|------|
| `400` | `BAD_REQUEST` | Body is not JSON or `session_id` / `signature` is missing (`download`: either query parameter is missing) |
| `404` | `NOT_FOUND` | Unknown session |
| `400` | `INVALID_SIGNATURE_FORMAT` | Signature or stored key is not valid base64, or the key is not an Ed25519 public key |
| `403` | `INVALID_SIGNATURE` | The signature does not match the key given at `validate` |
| `403` | `SESSION_EXPIRED` | The session expired |
| `500` | `INTERNAL_ERROR` | Database error (`retryable: true`) |

### `POST /api/extransfer/validate`

```json
{ "token": "TRANSFER_TOKEN", "target_public_key": "BASE64_ED25519_PUBLIC_KEY", "protocol_version": "1.0" }
```

All three fields are required; the token is trimmed. `protocol_version` must be `1.0`, the only version this panel speaks (it sends the same when it is the destination). The session must be `CREATED` (or `FAILED`, see [The token](#the-token)) and not expired. The origin generates a nonce valid for 3 minutes, stores the key and moves the session to `VALIDATED`.

```json
{
  "session_id": "6f1c2a9e-0b7d-4c55-9a51-2f0e8c3d7b14",
  "nonce": "0c9d8e7f-1a2b-4c3d-8e9f-a0b1c2d3e4f5",
  "capabilities_required": { "virtualization": "docker", "cpu_cores_min": 1, "ram_mb_min": 1024 }
}
```

`capabilities_required` is a fixed value that the destination does not check. The key's format is only checked at `consume`.

| Status | Answer |
|--------|--------|
| `400` | `BAD_REQUEST`: missing field or not JSON |
| `400` | `UNSUPPORTED_PROTOCOL`: `protocol_version` is not `1.0` (`retryable: false`) |
| `403` | `TOKEN_ERROR`, with `message` `not_found` (unknown token), `invalid_state` (already used or cancelled) or `expired` |

```bash
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"token": "TRANSFER_TOKEN", "target_public_key": "BASE64_ED25519_PUBLIC_KEY", "protocol_version": "1.0"}' \
  https://origin.example.com/api/extransfer/validate
```

### `POST /api/extransfer/consume`

```json
{ "session_id": "6f1c2a9e-0b7d-4c55-9a51-2f0e8c3d7b14", "signature": "BASE64_SIGNATURE" }
```

`signature` signs the nonce followed by the session ID. The session must be `VALIDATED` and the nonce younger than 3 minutes. The session moves to `MIGRATING` and the origin starts archiving the server folder in the background. Answer: `202 {"status": "MIGRATING", "message": "Stream initialized"}`.

If archiving fails the session becomes `FAILED`; if the session was cancelled while the archive was being made, the archive is deleted.

| Status | Answer |
|--------|--------|
| `400` | `BAD_REQUEST` |
| `403` | `CONSUME_ERROR`, with `message` `invalid_state` (unknown session or not `VALIDATED`), `nonce_expired`, `invalid_signature_format` or `invalid_signature` |

### `POST /api/extransfer/heartbeat`

Body: `session_id` and `signature` (`HEARTBEAT:<session_id>`). For a `MIGRATING` session it records the heartbeat and pushes the expiry:

```json
{ "status": "MIGRATING", "expires_at": "2026-11-02T10:20:20Z" }
```

A `CANCELLED` or `FAILED` session answers `200` with that `status`, which tells the destination to stop. Any other state answers `403 INVALID_STATE` ("Session not in migrating state").

### `GET /api/extransfer/download?session_id=...&signature=...`

`signature` signs `DOWNLOAD:<session_id>` and must be URL-encoded. The session must be `MIGRATING` and not expired (`403 INVALID_STATE` / `SESSION_EXPIRED`). The answer streams the archive as `application/octet-stream` with `Content-Disposition: attachment; filename=transfer.tar.gz`.

`503 NOT_READY` (`retryable: true`) while the archive is still being made (the node has no `transfer.tar.gz` yet); `500 DAEMON_ERROR` (`retryable: true`) when the origin's node cannot be reached or answers another error. The destination keeps asking after both. `500 INTERNAL_ERROR` (`retryable: false`) when the server no longer exists on the origin. A cancel cuts off a download in progress.

### `POST /api/extransfer/confirm`

Body: `session_id` and `signature` (`CONFIRM:<session_id>`). The session must be `MIGRATING`; it becomes `COMPLETED`, can no longer be downloaded, and the origin deletes its archive. Confirming a `COMPLETED` session again answers `200` too. Answer: `{"status": "COMPLETED"}`. Other states: `403 INVALID_STATE`.

### `POST /api/extransfer/cancel`

```json
{ "session_id": "6f1c2a9e-0b7d-4c55-9a51-2f0e8c3d7b14", "signature": "BASE64_SIGNATURE", "reason": "Disk full on the destination" }
```

`signature` signs `CANCEL:<session_id>`; `reason` is optional and cut to 500 characters. Allowed while the session is `VALIDATED`, `MIGRATING` or `FAILED`; cancelling a `CANCELLED` session again answers `200` too. The origin stops downloads in progress and deletes its archive. Answer: `{"status": "CANCELLED"}`. `CREATED` and `COMPLETED` sessions answer `403 INVALID_STATE` ("Session can no longer be cancelled").

## Records and notifications

- **Activity log** of the server: `extransfer.create`, `extransfer.pull` and `extransfer.cancel` for the routes on a server (see [`GET /api/servers/:serverId/activity`](./03-servers.md#get-apiserversserveridactivity)).
- **Transfer log** (a table in the origin's database): `CONFIRM`, `CANCEL` (with the destination's reason) and `CANCEL_BY_OWNER`, each with the caller's IP. There is no API route to read it.
- **Discord**: when `panel.notifications.discordWebhookExTransfer` is set, both panels post a report when a transfer completes, fails or is cancelled. The report texts are in Spanish. See [Server transfers → Requirements and configuration](../../features/server-transfers.md#requirements-and-configuration).
