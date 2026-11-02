# Server transfers

Aether Panel can move a server's files in two ways:

- **Node transfer**: moves a server to another node of the **same panel**. The server keeps its identifier, users and settings; only the machine it runs on changes.
- **Panel-to-panel migration** (federated or external transfer, "ExTransfer"): copies a server's files from **another Aether Panel instance**, for example when a customer moves to a different hosting provider.

## What it does

| | Node transfer | Panel-to-panel migration |
|---|---|---|
| Scope | Two nodes of one panel | Two independent panels |
| What travels | Server definition, scheduled tasks, all files and the backups kept on the node, together with its Splitter sub-servers | Files only (an archive of the server folder) |
| Started from | The origin server (**Administration › Transfer**) | Both sides: a token on the origin, an import on the destination (**Migration** tab) |
| Server stopped first | Yes (stopped, killed if it does not stop; started again on the new node if it was running) | No |
| Progress in the UI | No (the panel log; a failure is also written to the server's activity log) | Yes (step text, heartbeat, outcome) |
| Undone on failure | Yes, automatically, also after a panel restart in the middle | The partial archive is deleted |
| Cancellable | No | Yes, from either side |

## How it works

### Node transfer

A server moves together with its **Splitter sub-servers** (see [Sub-servers](sub-servers.md)): they live on their parent's node and take their resources from its plan there. A sub-server that is already on the target node stays where it is. A sub-server cannot be transferred on its own: the request is refused with `400` `ErrTransferSubServer`; transfer its parent instead.

Before anything moves, `POST /api/servers/:serverId/transfer` checks:

- The target is another node (`400` `cannot transfer to the same node` otherwise).
- No transfer of any of these servers is already running, nor recorded as unfinished from before a panel restart: `409` `ErrTransferRunning`.
- The ports of every server that moves (the main one and the extra ones) are free on the target node: no other server of that node, none of the node's own ports (panel, daemon, SFTP) and no container published there may use them. Otherwise `409` `ErrTransferPortsTaken` with the list of ports.
- No backup, restore point or restore of any of them is being written on the old node (asked through `GET /daemon/server/:id/backup/busy`): `409` `ErrTransferBackupRunning`. A node too old to have that route is not asked.

If the checks pass, the panel stores a record of the transfer in the `node_transfers` table (see [Recovery after a panel restart](#recovery-after-a-panel-restart)), answers `202` straight away and runs the transfer in the background. It first drops the panel's records of backups that failed, so none is left pointing at a backup the old node never had, and then:

1. **Prepare**, for each server in turn (the parent first):
   1. Pauses the server's scheduled tasks on the old node (`POST /daemon/server/:id/scheduler/pause`), so no task starts, stops or backs up the server during the copy.
   2. Reads the server definition from the old node and creates the server with it on the new node, pauses its tasks there too, and copies every scheduled task from the old node to the new one (`GET /daemon/server/:id/tasks`, then `PUT /daemon/server/:id/tasks/:taskId` for each).
   3. Stops the server on the old node and waits; if it is still running, kills it. If it cannot be stopped, the transfer fails. A backup that started in the meantime also fails it.
   4. Archives every file into `transfer.tar.gz` on the old node, streams the archive to the new node without buffering it in the panel, extracts it into the server folder there and deletes the archive.
   5. Copies each backup the old node keeps for the server (location `local` or `both`) through the panel to the new node (`PUT /daemon/server/:id/backup/file`), keeping its name and date. A backup kept only off-site (S3) needs no copy: the bucket is the same wherever the server runs. A backup that is already on the new node with the same name is left alone.
2. **Commit**: points every moved server's database record at the new node in one database transaction, which also marks the transfer record as committed. Only the node is written, so other changes made to the server while the transfer ran are kept.
3. **Clean up**: deletes each server from the old node, with its backups there (their copies are on the new node now; off-site copies are untouched). A failure here is only logged: the server already runs from the new node and what is left is a stray copy. Each moved server's subdomain is refreshed so it follows the new node's address.
4. **Start**: resumes the tasks of every moved server on the new node (`POST /daemon/server/:id/scheduler/resume`) and starts there the servers that were running before the transfer. One that does not start is not moved back; the panel records `server.transferStartFailed` in its activity log with the reason.

If any step up to the commit fails, the whole transfer is undone, newest step first, for every server already reached: the backup copies it brought and the server itself are removed from the new node, the archives are removed from the old node, the tasks are resumed on the old node and a server that was running is started again there. The servers stay on their old node and the panel records `server.transferFailed` in each one's activity log with the step and the reason (see [Activity log](activity-and-access-log.md)). A cleanup call that fails is written to the panel log.

The activity log also records the request itself as `server.transfer`. When the transfer ends, successfully or not, its record is deleted.

#### Recovery after a panel restart

The record in `node_transfers` holds, for each server being moved, its old node, the last step begun, whether it was running and which backups were (or were being) copied to the new node. It is updated at each step, and it is marked committed in the same database transaction that points the servers at the new node, so the record and the servers never disagree.

When the panel starts, once its routes are in place (the panel's own node answers through them), it looks for unfinished records and ends each one in a single pass:

- **Not committed**: the transfer is undone like a failed one. Everything that may have been done is undone (undoing a step that was not reached is harmless): the copied backups and the server are removed from the new node, the archives from the old node, the tasks are resumed on the old node and a server that was running is started again there. Each server's activity log gets `server.transferFailed` with the detail `the panel restarted while <step>: the transfer was undone`.
- **Committed**: the transfer is finished. The old copies are deleted, the subdomains are refreshed, the tasks are resumed on the new node and the servers that were running are started there (unless they already run); one that does not start gets `server.transferStartFailed`.

Calls that fail during recovery are written to the panel log and are not retried. The record is deleted either way.

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
3. The panel shows *Transfer started.* When the copy finishes the server, its sub-servers, their scheduled tasks and their backups are on the new node, and the servers that were running are started there. If the transfer fails it is undone and the server's **Activity** shows why.

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
| POST | `/api/servers/:serverId/transfer` | `server.data.edit.admin` | Node transfer of the server and its sub-servers. Body `{"nodeId": 2}`. Answers `202 {"msg": "Transfer started"}`. `400` `ErrTransferSubServer` for a sub-server; `409` `ErrTransferPortsTaken`, `ErrTransferRunning` or `ErrTransferBackupRunning`. |
| POST | `/api/servers/:serverId/extransfer/create` | `server.data.edit.admin` | Origin: creates a session. Answers `session_id`, `token`, `expires_in` (900). |
| POST | `/api/servers/:serverId/extransfer/pull` | `server.data.edit.admin` | Destination: starts the import. Body `{"origin_url": "https://panel.example.com", "token": "…"}`. Answers `202`. |
| GET | `/api/servers/:serverId/extransfer/status` | `server.data.edit.admin` | Both sides: `status` (code of the current import step or outcome: `STEP_*`, `ERROR_*`, `DONE`, `CANCELLED`, `CANCELLED_BY_ORIGIN`), `detail` (what some failures add, such as `HTTP 403`), `message` (English text of a `STEP_*` or `ERROR_*` code, for a client that does not know it), `pulling`, and `origin` (newest session created here, while live or up to 15 minutes after it ended). |
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
| `panel.notifications.discordWebhookExTransfer` | empty | `SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOKEXTRANSFER` | Discord webhook that receives a report when a migration completes, fails or is cancelled. Editable in **Settings › Discord Notifications** (**ExTransfer Webhook**). The report is written in the notification language (`panel.settings.notificationLanguage`, see [Panel settings](panel-settings.md)). |

See [Configuration](../backend/02-configuration.md).

Other requirements:

- Both actions need `server.data.edit.admin` on the server, which a server owner does not hold by default (see the [scopes reference](../general/scopes-reference.md)).
- Node transfer: both nodes online and reachable from the panel; enough free disk on the source node for the archive and on the target for archive plus files and backups. Both nodes must run a version with the `/daemon/server/:id/scheduler/pause` and `/scheduler/resume` routes: with an older daemon on either side, the transfer fails when it pauses the tasks there and is undone (see [Troubleshooting](../setup/troubleshooting.md)). To receive backups the target node also needs the `/daemon/server/:id/backup/file` route, which every version with the scheduler routes has.
- Migration: the origin panel must be reachable from the destination panel over HTTP(S) at a **public** address. Localhost and private or loopback IP addresses are refused. The URL may only be `http(s)://host[:port]` (no path); without a scheme, `https://` is assumed.

## Limits

- **Node transfer**
  - There is no progress indicator: the outcome shows in the server's activity log (on failure) and in the panel log.
  - A transfer cut short by a panel restart is not continued: it is undone, or finished if the panel already pointed at the new node. Recovery is a single pass when the panel starts; a node that is unreachable at that moment keeps what was left on it (a copy on the new node, an archive or paused tasks on the old one), and the panel log lists each failed call. Check both nodes before trying again.
  - The detail of a recovery entry in the activity log (`the panel restarted while …`) is in English.
  - Scheduled tasks are paused on both nodes while the server moves, so runs due meanwhile (and **Run now**) are skipped, not caught up. A task already running when the transfer starts is not interrupted. The tasks themselves are copied, but not the scheduler settings of the `.cron` file (time zone, concurrency limit): the new node uses its defaults (see [Scheduled tasks](scheduled-tasks.md)).
  - Only one level of sub-servers moves with the server: sub-servers of a sub-server are not looked for.
  - The servers are unavailable from the moment they are stopped until they are started on the new node.
- **Panel-to-panel migration**
  - Only files travel. The destination server keeps its own template, variables, resources, users and backups; set them up to match.
  - Files from the archive overwrite files with the same name on the destination; other existing files are kept. The import does not stop the destination server or change its configuration, as the warning in the **Import** tab says: stop it yourself and make a backup first (see [Backups](backups.md)).
  - The origin server is not stopped while it is archived, so a running world can be copied mid-save, and it is not deleted after the migration.
  - The origin must have the archive ready within 2 hours of the first download attempt (polling starts 5 seconds after the consume), otherwise the import fails.
  - The status API answers each import step as a code that the page translates into the viewer's language; the destination's Discord reports use the panel's notification language, and the reason it sends to the origin when it gives up is in English.
  - The archive is written inside the origin server's folder and counts towards its disk.

## Troubleshooting

| Message | Side | Cause | Fix |
|---------|------|-------|-----|
| `cannot transfer to the same node` | Panel | The target node is the current one. | Choose another node. |
| `ports … of this server are already used on the target node: free them or pick another node` (`409`, `ErrTransferPortsTaken`) | Panel | Another server, the node itself or a container uses those ports on the target node. | Free them there, change the server's ports or pick another node. |
| `A sub-server stays on its parent's node…` (`400`, `ErrTransferSubServer`) | Panel | The server is a Splitter sub-server. | Transfer the parent server: its sub-servers go with it. |
| `This server is already being transferred…` (`409`, `ErrTransferRunning`) | Panel | A transfer of this server or one of its sub-servers is running. | Wait for it to end. |
| `A backup or restore of this server is in progress…` (`409`, `ErrTransferBackupRunning`) | Panel | The old node is writing a backup, restore point or restore of one of the servers. | Transfer once it is done. |
| The server is still on the old node; its activity shows `server.transferFailed` | Panel | A step failed and the transfer was undone. The detail names the server, the step and the node's answer. | Fix the cause (disk, node offline, target node too old to receive backups) and try again. The panel log has `Transfer of … failed and was undone`. |
| Activity shows `server.transferFailed` with `the panel restarted while <step>: the transfer was undone` | Panel | The panel stopped during the transfer and undid it when it started again. | Check the panel log for cleanup calls that failed (for example a node that was offline), then transfer again. |
| `pausing its tasks on the old node` / `on the new node` in the failure detail | Panel | That node runs a daemon without the scheduler pause route. | Update the node (see [Troubleshooting](../setup/troubleshooting.md)) and try again. |
| The server moved but is stopped; its activity shows `server.transferStartFailed` | Panel | It could not be started on the new node. | Read the reason in the detail and the server console, then start it by hand. |
| `The origin URL is not valid or points to an address that is not allowed` (`ERROR_ORIGIN_URL`) | Destination | URL with a path, credentials, a private IP or localhost. | Use the origin panel's public `https://host[:port]`. |
| `Network error while connecting to the origin` (`ERROR_ORIGIN_UNREACHABLE`) | Destination | The origin is unreachable. | Check DNS, firewall and TLS of the origin panel. |
| `Invalid or expired token (HTTP 403)` (`ERROR_TOKEN_REJECTED`) | Destination | Token mistyped, already used, cancelled or older than 15 minutes. | Generate a new token on the origin. |
| `The origin refused the transfer` (`ERROR_ORIGIN_REFUSED`) | Destination | The signed consume was refused (nonce older than 3 minutes, session not `VALIDATED`). | Start again with a new token. |
| `The origin did not prepare the package in time (2h0m0s)` (`ERROR_DOWNLOAD_TIMEOUT`) | Destination | Archiving the origin server took more than 2 hours. | Reduce the server's size (old backups, logs) and try again. |
| `The origin refused the download (HTTP …)` (`ERROR_DOWNLOAD_REFUSED`) | Destination | The session was cancelled, failed or expired on the origin. | Check the origin's **Export** tab; generate a new token. |
| `The file could not be uploaded to the daemon` (`ERROR_UPLOAD`) / `The files could not be unpacked on the destination (…)` (`ERROR_EXTRACT`) | Destination | The destination node refused the archive (disk limit, node offline). | Free disk on the destination server or node. |
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
