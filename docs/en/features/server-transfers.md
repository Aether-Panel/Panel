# Server transfers

Aether Panel can move a server's files in two ways:

- **Node transfer**: moves a server to another node of the **same panel**. The server keeps its identifier, users and settings; only the machine it runs on changes.
- **Panel-to-panel migration** (federated or external transfer, "ExTransfer"): copies a server's files from **another Aether Panel instance**, for example when a customer moves to a different hosting provider.

## What it does

| | Node transfer | Panel-to-panel migration |
|---|---|---|
| Scope | Two nodes of one panel | Two independent panels |
| What travels | Server definition and all files | Files only (an archive of the server folder) |
| Started from | The origin server (**Administration › Transfer**) | Both sides: a token on the origin, an import on the destination (**Migration** tab) |
| Server stopped first | Yes (stopped, killed if it does not stop) | No |
| Progress in the UI | No (only the panel log) | Yes (step text, heartbeat, outcome) |
| Cancellable | No | Yes, from either side |

## How it works

### Node transfer

Before it starts, the panel checks that the server's ports (the main one and the extra ones) are free on the target node: no other server of that node, none of the node's own ports (panel, daemon, SFTP) and no container published there may use them. If any is taken, it answers `409` with `ErrTransferPortsTaken` and the list of ports, and nothing is moved. When they are free, `POST /api/servers/:serverId/transfer` answers `202` straight away and the panel runs these steps in the background:

1. Reads the server definition from the current node and creates the server with it on the target node.
2. Stops the server on the current node and waits; if it is still running, kills it. If it cannot be stopped, the transfer is cancelled.
3. Archives every file into `transfer.tar.gz` on the current node.
4. Streams the archive from the current node to the target node, without buffering it in the panel.
5. Extracts it into the server folder on the target node and deletes the archive there.
6. Points the server's database record at the target node and refreshes its subdomain so it follows the new node's address.
7. Asks the old node to delete the server while keeping its backups (`?keepBackups=true`): backups are **not** moved to the new node and stay on the old node's disk.

The server is not started again on the target node; start it when the transfer is done. The activity log records `server.transfer`.

### Panel-to-panel migration

The panel the server leaves is the **origin**; the panel it arrives at is the **destination**. The destination needs an existing server to import into: the import fills that server's folder.

1. **Token (origin).** The owner generates a token (`POST /api/servers/:serverId/extransfer/create`). It is a random 32-byte value, valid for **15 minutes**, and spent by the first successful validation (a session that ended in `FAILED` can be validated again until the token expires). The origin only stores an HMAC-SHA256 hash of it.
2. **Import (destination).** On the destination server, the owner enters the origin panel URL and the token (`POST /api/servers/:serverId/extransfer/pull`). The destination then:
   1. Calls the origin's `POST /api/extransfer/validate` with the token and its Ed25519 public key. The origin answers with a session ID and a nonce valid for 3 minutes.
   2. Signs nonce + session ID and calls `POST /api/extransfer/consume`. The origin verifies the signature, marks the session `MIGRATING` and starts archiving the server folder (the origin server keeps running). The archive is written as `.transfer-partial.tar.gz` and only renamed to `transfer.tar.gz` once complete, so half an archive is never sent.
   3. Sends a signed heartbeat every 20 seconds (`/api/extransfer/heartbeat`). Each one pushes the session's expiry 5 minutes forward, up to **6 hours** after it was created. If the origin answers that the session was cancelled or failed, the destination stops.
   4. Polls the signed download (`GET /api/extransfer/download`) every 5 seconds until the archive is ready, then streams it to its own node. While the archive is being made the origin answers `503` `NOT_READY` (`retryable: true`) and the destination asks again; it waits for up to **2 hours**, and stops earlier if a heartbeat reports that the origin cancelled or failed.
   5. Extracts it into the server folder, deletes the archive and calls the signed `POST /api/extransfer/confirm`. The origin marks the session `COMPLETED` and deletes its archive.
3. **Cancel.** Either side can cancel. The destination signs `POST /api/extransfer/cancel` on the origin; the origin's owner cancels from its own panel, which the destination learns through the next heartbeat. A cancelled or failed import deletes the partial archive.

Every call between panels after the token is signed with the destination's Ed25519 key (`<ACTION>:<session_id>`), so only the panel that validated the token can continue the session. Session states are `CREATED`, `VALIDATED`, `MIGRATING`, `COMPLETED`, `FAILED` and `CANCELLED`. The protocol version is `1.0`: the destination sends it in `protocol_version` when validating and the origin refuses any other with `400` `UNSUPPORTED_PROTOCOL`. Each panel's signing key is generated once and stored in `panel.exTransferKey`, so a restart does not invalidate a session in progress.

The activity log records `extransfer.create`, `extransfer.pull` and `extransfer.cancel`.

## Using it

### In the panel

**Node transfer**

1. Open the server and go to **Administration › Transfer**.
2. Click **Transfer**, choose the **Target node** and click **Start transfer**.
3. The panel shows *Transfer started.* The server disappears from the old node when the copy finishes.

**Panel-to-panel migration**

On the origin panel:

1. Open the server, go to the **Migration** tab and stay on **Export**.
2. Click **Generate token**. The panel shows **This panel's URL**, the **Migration token** and a countdown (**Expires in**). The token stays visible for 15 minutes even after leaving the page (it is kept in the browser's local storage).
3. Give the URL and token to the destination provider. While the transfer runs, the tab shows **Transfer in progress** and the **Last signal from the destination**; at the end, **Migration completed**.

On the destination panel:

1. Create the server that will receive the data (same game and template), open it and go to **Migration › Import**.
2. Enter the **Origin panel URL** and the **Migration token**, then click **Start import**.
3. The tab shows the current step until the migration completes, fails or is cancelled.

Both sides offer **Cancel transfer** while a transfer is live.

### Via the API

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| POST | `/api/servers/:serverId/transfer` | `server.data.edit.admin` | Node transfer. `409` `ErrTransferPortsTaken` when a port of the server is taken on the target. Body `{"nodeId": 2}`. Answers `202 {"msg": "Transfer started"}`. |
| POST | `/api/servers/:serverId/extransfer/create` | `server.data.edit.admin` | Origin: creates a session. Answers `session_id`, `token`, `expires_in` (900). |
| POST | `/api/servers/:serverId/extransfer/pull` | `server.data.edit.admin` | Destination: starts the import. Body `{"origin_url": "https://panel.example.com", "token": "…"}`. Answers `202`. |
| GET | `/api/servers/:serverId/extransfer/status` | `server.data.edit.admin` | Both sides: `status` (current import step), `pulling`, and `origin` (newest session created here, while live or up to 15 minutes after it ended). |
| POST | `/api/servers/:serverId/extransfer/cancel` | `server.data.edit.admin` | Cancels the live origin sessions and the running import of this server. `409 NO_ACTIVE_TRANSFER` when there is none. |
| POST | `/api/extransfer/validate` | public (token) | Called by the destination. |
| POST | `/api/extransfer/consume` | public (signature) | Called by the destination. |
| POST | `/api/extransfer/heartbeat` | public (signature) | Called by the destination. |
| GET | `/api/extransfer/download` | public (signature) | Called by the destination. |
| POST | `/api/extransfer/confirm` | public (signature) | Called by the destination. |
| POST | `/api/extransfer/cancel` | public (signature) | Called by the destination. |

Errors from the public routes use `{"error_code", "message", "retryable"}` (the type also has a `session_id` field, but no error fills it). See [API reference: External transfer](../reference/api/08-external-transfer.md).

## Requirements and configuration

| Key | Default | Environment variable | Meaning |
|-----|---------|----------------------|---------|
| `panel.exTransferKey` | generated on first use | `SKYPANEL_PANEL_EXTRANSFERKEY` | Seed of this panel's Ed25519 signing key. Written to the config file automatically; do not change it during a transfer. |
| `panel.notifications.discordWebhookExTransfer` | empty | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKEXTRANSFER` | Discord webhook that receives a report when a migration completes, fails or is cancelled. Editable in **Settings › Discord Notifications** (**ExTransfer Webhook**). |

See [Configuration](../backend/02-configuration.md).

Other requirements:

- Both actions need `server.data.edit.admin` on the server, which a server owner does not hold by default (see the [scopes reference](../general/scopes-reference.md)).
- Node transfer: both nodes online and reachable from the panel; enough free disk on the source node for the archive and on the target for archive plus files.
- Migration: the origin panel must be reachable from the destination panel over HTTP(S) at a **public** address. Localhost and private or loopback IP addresses are refused. The URL may only be `http(s)://host[:port]` (no path); without a scheme, `https://` is assumed.

## Limits

- **Node transfer**
  - There is no progress indicator and no automatic rollback. If a step fails, the error goes to the panel log only; the server may already exist on the target node while still running from the source node, or stay stopped after step 2.
  - Only the chosen server moves. Sub-servers stay on their node (see [Sub-servers](sub-servers.md)).
  - Backups are not moved: they stay on the old node and the server's **Backups** tab no longer lists them. Download the ones you want to keep before transferring (see [Backups](backups.md)).
- **Panel-to-panel migration**
  - Only files travel. The destination server keeps its own template, variables, resources, users and backups; set them up to match.
  - Files from the archive overwrite files with the same name on the destination; other existing files are kept. The import does not stop the destination server or change its configuration, as the warning in the **Import** tab says: stop it yourself and make a backup first (see [Backups](backups.md)).
  - The origin server is not stopped while it is archived, so a running world can be copied mid-save, and it is not deleted after the migration.
  - The origin must have the archive ready within 2 hours of the first download attempt (polling starts 5 seconds after the consume), otherwise the import fails.
  - Import step messages are produced by the destination's backend in Spanish, whatever language the interface uses.
  - The archive is written inside the origin server's folder and counts towards its disk.

## Troubleshooting

| Message | Side | Cause | Fix |
|---------|------|-------|-----|
| `cannot transfer to the same node` | Panel | The target node is the current one. | Choose another node. |
| `ports … of this server are already used on the target node: free them or pick another node` (`409`, `ErrTransferPortsTaken`) | Panel | Another server, the node itself or a container uses those ports on the target node. | Free them there, change the server's ports or pick another node. |
| Node transfer never finishes; the server is still on the old node | Panel | A background step failed. | Check the panel log for `Failed to …` / `Transfer of … cancelled`, fix the node problem and retry. |
| `ERROR: La URL de origen no es válida o apunta a una dirección no permitida` | Destination | URL with a path, credentials, a private IP or localhost. | Use the origin panel's public `https://host[:port]`. |
| `ERROR: Fallo de red al conectar con origen` | Destination | The origin is unreachable. | Check DNS, firewall and TLS of the origin panel. |
| `ERROR: Token inválido o expirado (Status 403)` | Destination | Token mistyped, already used, cancelled or older than 15 minutes. | Generate a new token on the origin. |
| `ERROR: El origen rechazó la transferencia` | Destination | The signed consume was refused (nonce older than 3 minutes, session not `VALIDATED`). | Start again with a new token. |
| `ERROR: El origen no preparó el paquete a tiempo (timeout 2h0m0s)` | Destination | Archiving the origin server took more than 2 hours. | Reduce the server's size (old backups, logs) and try again. |
| `ERROR: El origen rechazó la descarga (Status …)` | Destination | The session was cancelled, failed or expired on the origin. | Check the origin's **Export** tab; generate a new token. |
| `ERROR: Error al subir archivo al daemon` / `ERROR: Error al descomprimir en el destino: …` | Destination | The destination node refused the archive (disk limit, node offline). | Free disk on the destination server or node. |
| `A transfer is already running for this server` (`409`) | Destination | An import is already in progress. | Wait for it or cancel it. |
| `There is no transfer in progress for this server` (`409 NO_ACTIVE_TRANSFER`) | Either | Nothing to cancel. | — |
| `UNSUPPORTED_PROTOCOL` (`400`) | Origin (public routes) | The destination speaks a protocol version other than `1.0`. | Update both panels to compatible versions. |
| `TOKEN_ERROR` / `CONSUME_ERROR` / `INVALID_SIGNATURE` / `SESSION_EXPIRED` | Origin (public routes) | Invalid token, wrong signature or expired session. | Start a new migration. |

## See also

- [Core concepts: external transfer](../general/concepts.md)
- [Sub-servers](sub-servers.md)
- [Cloning servers](cloning.md) (a copy on the same node)
- [API reference](../reference/11-api-reference.md)
- [Configuration](../backend/02-configuration.md)
