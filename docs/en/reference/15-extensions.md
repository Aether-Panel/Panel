# Extensions

An extension adds features to the panel without touching its code. It can do two things:

- **Pages**: HTML files the panel shows as a tab on every server or as an entry in the administration menu.
- **Events**: the panel notifies an `https://` address of the extension when something happens (a server is created, starts, crashes…).

An administrator installs them in **Settings → Extensions** by uploading a `.zip`. A freshly installed extension is **switched off**: it shows nothing and receives no events until you switch it on.

## The package

A `.zip` with `extension.json` at its root (or inside a single folder) and the files of its pages:

```
my-extension.zip
├── extension.json
├── index.html
└── js/app.js
```

Limits: 20 MB compressed, 50 MB unpacked and 500 files. Symbolic links and paths that leave the folder are refused.

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
| `id` | yes | 2 to 40 lowercase letters, digits or dashes. Installing another package with the same `id` updates it. |
| `name` | yes | Up to 60 characters. |
| `version` | yes | Up to 20 characters, such as `1.0.0`. |
| `author`, `description` | no | Shown in Settings. |
| `pages` | without a `webhook` | Up to 10. `placement` is `server` (a tab on every server) or `admin` (administration menu, administrators only). `path` is an `.html` of the package. |
| `webhook` | with `events` | `https://` address that receives the events. The administrator can change it in Settings. |
| `events` | no | The events it wants (list below). |

A field the panel does not know makes the installation fail, so a typo does not go unnoticed.

## Pages

The panel shows each page inside an **isolated** frame (`sandbox` without `allow-same-origin`): its scripts run, but in an opaque origin, so they cannot read the administrator's session or call the panel's API on their behalf. They have no `localStorage` or cookies of their own either.

Once loaded, the page receives a message with the context:

```js
window.addEventListener('message', (event) => {
  if (event.data?.type !== 'aether:context') return;
  const { serverId, language, theme } = event.data;
  // serverId: the tab's server (null on an administration page)
  // language: "es" or "en"; theme: "dark" or "light"
});
```

To fit the frame to its content, the page can answer:

```js
parent.postMessage({ type: 'aether:resize', height: document.body.scrollHeight }, '*');
```

When a page needs data, it asks its own service (the webhook's, for example). Page files are public while the extension is switched on: put no keys or secrets in them.

## Events

| Event | When |
|-------|------|
| `server.created` | A server is created (from WHMCS/Paymenter too). |
| `server.deleted` | A server is deleted. |
| `server.started` | The server is up. |
| `server.stopped` | The server is down. |
| `server.crashed` | The server died on its own. Carries `exitCode`, `restarted` and the last console lines in `lines`. |
| `server.backup` | A backup finished. `success` says whether it worked. |
| `server.resources` | CPU or memory stayed high. Carries `resource`, `value` and `threshold`. |

Each event is a JSON `POST`:

```json
{
  "event": "server.crashed",
  "extension": "player-stats",
  "timestamp": "2026-10-26T18:04:11Z",
  "server": { "id": "a1b2c3d4", "name": "Survival" },
  "title": "💥 Caída del servidor",
  "message": "Survival se ha caído (código de salida 1). No se reinició automáticamente.",
  "exitCode": 1,
  "restarted": false,
  "lines": ["..."]
}
```

When the answer is not `2xx`, the panel retries after 5 and 30 seconds. Answer quickly and do the work afterwards.

`server.started`, `server.stopped`, `server.crashed`, `server.backup` and `server.resources` come from the node the server runs on. They are only sent from a node that also runs the panel (the usual single-machine setup).

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

The **Send test** button in Settings posts a signed `test` event to check the address and the key.

## API

| Method | Path | Who |
|--------|------|-----|
| GET | `/api/extensions` | administrators: all of them, with their signing key |
| POST | `/api/extensions` | administrators: install or update (`file` field of a multipart form) |
| PUT | `/api/extensions/:id` | administrators: `{ "enabled": true, "webhookUrl": "https://..." }` |
| DELETE | `/api/extensions/:id` | administrators: uninstall |
| POST | `/api/extensions/:id/test` | administrators: send a test event |
| GET | `/api/extensions/active` | any user: pages of the switched-on extensions (administration pages only for administrators) |
| GET | `/api/extension-files/:id/*` | the pages' files, always with `Content-Security-Policy: sandbox` |
