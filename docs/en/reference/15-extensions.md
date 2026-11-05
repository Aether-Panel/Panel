# Extensions

An extension adds features to the panel without touching its code. It can do two things:

- **Pages**: HTML files the panel shows as a tab on every server or as an entry in the administration menu.
- **Events**: the panel notifies an `https://` address of the extension when something happens (a server is created, starts, crashes…).

An administrator installs them in **Settings → Extensions** by uploading a `.zip`. A freshly installed extension is **switched off**: it shows nothing and receives no events until an administrator switches it on.

Code: `internal/extensions` (manifest, installation, event delivery), `internal/web/api/extensions.go` (API and file serving), `client/frontend/src/features/extensions` (the frame).

## The package

A `.zip` with `extension.json` at its root (or inside a single folder) and the files of its pages:

```
my-extension.zip
├── extension.json
├── index.html
└── js/app.js
```

Limits: 20 MB compressed, 50 MB unpacked, 500 files and 64 KB for `extension.json`. Symbolic links, backslashes and paths that leave the folder are refused; `__MACOSX` entries are ignored. The unpacked size is enforced while writing, whatever sizes the zip declares.

### `extension.json`

```json
{
  "id": "player-stats",
  "name": "Player statistics",
  "version": "1.0.0",
  "author": "Your name",
  "description": "Player charts per server.",
  "pages": [
    { "id": "players", "title": "Players", "placement": "server", "path": "index.html" },
    { "id": "overview", "title": "Overview", "placement": "admin", "path": "admin.html" }
  ],
  "webhook": "https://my-service.example.com/aether",
  "events": ["server.started", "server.stopped", "server.crashed"]
}
```

| Field | Required | What it is |
|-------|----------|------------|
| `id` | yes | 2 to 40 lowercase letters, digits or dashes, starting with a letter or digit. Installing another package with the same `id` updates it. |
| `name` | yes | Up to 60 characters. |
| `version` | yes | Up to 20 characters among letters, digits, `.`, `+` and `-`, such as `1.0.0`. |
| `author` | no | Up to 60 characters. Shown in Settings. |
| `description` | no | Up to 300 characters. Shown in Settings. |
| `pages` | without a `webhook` | Up to 10 (see below). |
| `webhook` | with `events` | `https://` address that receives the events, without `user:password@`. The administrator can change it in Settings. |
| `events` | no | The events it wants ([list below](#events)). Requires `webhook`. |
| `servers` | no | The servers the extension is for ([see below](#only-for-some-servers)). Without it, all of them. |

An extension needs at least one page or a webhook. A field the panel does not know makes the installation fail, so a typo does not go unnoticed.

Each page:

| Field | What it is |
|-------|------------|
| `id` | 1 to 40 lowercase letters, digits or dashes, unique within the extension |
| `title` | 1 to 40 characters: the tab or menu label |
| `placement` | `server` (a tab on every server) or `admin` (administration menu, administrators only) |
| `path` | An `.html` file of the package, as a clean relative path (`index.html`, `pages/a.html`) |

### Only for some servers

An extension made for one game or kind of server says so with `servers`: its server pages then show only on those servers and its events only come from them. Administration pages do not change.

```json
"servers": { "types": ["armareforger"] }
```

| Field | What it is |
|-------|------------|
| `types` | Server types: the `type` of the template the server was made from (`armareforger`, `minecraft-java`, `nodejs`…) |
| `icons` | Games: the template's `icon`, for those that share a type (`csgo`, `rust` and `tf2` are all `srcds`) |

A server matches when its type is in `types` **or** its icon in `icons`. Each list takes up to 30 values, lowercase (they are converted), with letters, digits, `.`, `_` and `-`. An empty `servers` fails the install. A server whose type and icon are unknown matches no extension that has `servers`.

## Installing, updating and removing

- **Install**: the package is checked, unpacked into a staging folder and then moved into `<panel.extensions.folder>/<id>` (default `<data>/extensions/<id>`). A new extension gets its own random signing key and starts switched off.
- **Update**: uploading a package with an `id` that is already installed replaces its files atomically (a failed update leaves the previous version untouched) and keeps its on/off switch, its signing key and, if the administrator changed it, its webhook.
- **Uninstall**: removes the record and the extension's folder.

Changes take effect at once: the list of active extensions is reloaded after every install, update, switch or uninstall.

## Pages

The panel shows each page inside an **isolated** frame: `sandbox="allow-scripts allow-forms allow-popups allow-popups-to-escape-sandbox allow-modals allow-downloads"`, without `allow-same-origin`, and `referrerpolicy="no-referrer"`. Its scripts run, but in an opaque origin, so they cannot read the administrator's session or call the panel's API on their behalf. They have no `localStorage` or cookies of their own either. A link with `target="_blank"` opens the other site in a normal tab (`allow-popups-to-escape-sandbox`), with its own session: that way a page can take people to its service without it staying isolated.

The files themselves are served from `/api/extension-files/<id>/<path>` with the same sandbox as their `Content-Security-Policy`, so the isolation holds even if someone opens a file directly. Only regular files of a **switched-on** extension are served (anything else answers `404`), with `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: SAMEORIGIN` and `Cache-Control: no-cache`; the content type comes from the file extension. Page files are public while the extension is switched on: put no keys or secrets in them.

Once loaded, the page receives a message with the context, and again whenever the user moves to another server or changes the language:

```js
window.addEventListener('message', (event) => {
  if (event.source !== window.parent || event.data?.type !== 'aether:context') return;
  const { serverId, panelUrl, language, theme } = event.data;
  // serverId: the tab's server (null on an administration page)
  // panelUrl: the panel's address (https://panel.example.com), the one in Settings: the same as the events' panel field; the page cannot see it by itself
  // language: "es" or "en"; theme: "dark" or "light" (as it was when the message was sent)
});
```

To fit the frame to its content, the page can answer:

```js
parent.postMessage({ type: 'aether:resize', height: document.body.scrollHeight }, '*');
```

The panel only accepts that message from the page's own frame (whose origin is always the opaque `"null"`), and keeps the height between 200 and 4000 pixels.

When a page needs data, it asks its own service (the webhook's, for example).

## Events

| Event | When | Extra fields |
|-------|------|--------------|
| `server.created` | A server is created (from the panel, or from WHMCS/Paymenter through `/api/v1`). | — |
| `server.deleted` | A server is deleted. | — |
| `server.started` | The server is up. | `running: true` |
| `server.stopped` | The server is down. | `running: false` |
| `server.crashed` | The server died on its own. | `exitCode`, `restarted`, `lines` (up to the last 12 console lines) |
| `server.backup` | A backup finished. | `success` |
| `server.resources` | CPU or memory stayed high. | `resource`, `value`, `threshold` |

Each event is a JSON `POST` with `Content-Type: application/json` and `User-Agent: AetherPanel-Webhook/1`:

```json
{
  "event": "server.crashed",
  "extension": "player-stats",
  "timestamp": "2026-10-26T18:04:11Z",
  "panel": "https://panel.example.com",
  "server": { "id": "a1b2c3d4", "name": "Survival", "type": "minecraft-java", "icon": "minecraft-java" },
  "title": "💥 Caída del servidor",
  "message": "Survival se ha caído (código de salida 1). No se reinició automáticamente.",
  "exitCode": 1,
  "restarted": false,
  "lines": ["..."]
}
```

Every event has `event`, `extension`, `timestamp` (UTC, RFC 3339) and `server`, with its `id`, its `name` and, when known, its template's `type` and `icon`. The node events (`started`, `stopped`, `crashed`, `backup`, `resources`) also carry the `title` and `message` of the panel's server notifications, which are currently written in Spanish; rely on `event` and the extra fields, not on that text.

Delivery:

- Each attempt has a 10-second timeout. When it fails or the answer is not `2xx`, the panel retries after 5 and then 30 seconds; after the third failure the event is only logged. Answer quickly and do the work afterwards.
- At most 32 deliveries are in flight at once across all extensions. Beyond that, new events are dropped (and logged) rather than queued.
- Events are sent whatever the server owner's own notification settings are.

`server.started`, `server.stopped`, `server.crashed`, `server.backup` and `server.resources` come from the node the server runs on. They are only sent from a node that also runs the panel (the usual single-machine setup); a daemon-only node does not deliver them.

### Checking the signature

Every event carries the `X-Aether-Signature: sha256=<hex>` header: the HMAC-SHA256 of the body as received, with the signing key shown in Settings → Extensions. Reject notices whose signature does not match:

```js
import crypto from 'node:crypto';

function validSignature(body, header, key) {
  const expected = 'sha256=' + crypto.createHmac('sha256', key).update(body).digest('hex');
  return header?.length === expected.length &&
    crypto.timingSafeEqual(Buffer.from(header), Buffer.from(expected));
}
```

The **Send test** button in Settings posts a signed `test` event (`event`, `extension`, `timestamp` and `panel` only) to check the address and the key.

### Checking which panel sent it

The signing key proves the sender holds that key, not which panel it is: any panel's administrator sees their own. A service that receives events from many panels (for example, an extension hosting companies install for their customers) needs to know which one sends, so every event also carries:

- `panel` in the body: the panel's address (`panel.settings.masterUrl`, without a trailing slash and with `https://` in front when only the host name was written).
- The `X-Aether-Panel-Signature: ed25519=<base64>` header: the Ed25519 signature of the body as received, made with the **panel's identity key**. It is the key the panel signs transfers between panels with; it is created once and kept in `panel.exTransferKey`.

The panel publishes its public key at its own address, without a session:

```
GET https://panel.example.com/.well-known/aether-panel.json
{ "url": "https://panel.example.com", "publicKey": "<base64>", "algorithm": "ed25519", "signatureHeader": "X-Aether-Panel-Signature" }
```

The service downloads that key over HTTPS from the address of the panel it links to (HTTPS proves it comes from that domain), keeps it, and checks every event with it. `panel` inside the signed body must also be that same address. Nobody else can sign as that panel: the private key never leaves it.

```js
import crypto from 'node:crypto';
function fromPanel(body, header, publicKeyBase64) {
  const signature = Buffer.from(String(header).replace(/^ed25519=/, ''), 'base64');
  const key = crypto.createPublicKey({ key: Buffer.concat([Buffer.from('302a300506032b6570032100', 'hex'), Buffer.from(publicKeyBase64, 'base64')]), format: 'der', type: 'spki' });
  return crypto.verify(null, body, key, signature);
}
```

If the panel's key changes (for example, reinstalled without its configuration), the service has to download it again.

## API

| Method | Path | Who | Answer |
|--------|------|-----|--------|
| GET | `/api/extensions` | administrators | Every extension with its pages (each with its `url`), `events`, `manifestWebhook`, `webhookUrl`, `enabled` and signing key (`secret`) |
| POST | `/api/extensions` | administrators | Install or update: the zip in the `file` field of a multipart form. `200` with the extension, or `400` with `ErrExtensionInvalid` and the reason |
| PUT | `/api/extensions/:id` | administrators | `{ "enabled": true, "webhookUrl": "https://..." }`, both optional. An empty `webhookUrl` stops events. `404` if not installed |
| DELETE | `/api/extensions/:id` | administrators | Uninstall. `204`, or `404` |
| POST | `/api/extensions/:id/test` | administrators | Send a test event. `204`; `400` with `ErrExtensionNoWebhook` or `ErrExtensionWebhook` (the delivery error) |
| GET | `/api/extensions/active` | any signed-in user | Pages of the switched-on extensions: `[{ id, name, pages, servers }]` (`servers` only when the extension declares it). Administration pages only for administrators |
| GET, HEAD | `/api/extension-files/:id/*path` | public | The pages' files, always with `Content-Security-Policy: sandbox ...` |

"Administrators" means the `admin` scope; "any signed-in user" means the `login` scope.
