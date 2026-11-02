# Automatic subdomains (Cloudflare)

With a Cloudflare zone connected, every server can have its own address, such as `survival.play.example.com`, instead of an IP and port. The panel creates, updates and removes the DNS records itself through the Cloudflare API. This page covers setup for administrators and daily use for server owners; the internals of how records follow a server are summarised in [Servers — Subdomain (Cloudflare)](../backend/04-servers.md#subdomain-cloudflare).

## What it does

- Lets the owner of a server pick a name (`<name>.<base domain>`) from the server's settings, with a live availability check.
- Optionally gives **every new server** a subdomain automatically, derived from its name.
- Creates the DNS records in the administrator's Cloudflare zone:
  - An `A`, `AAAA` or `CNAME` record with the subdomain's name, pointing at the server's node.
  - For Minecraft Java servers, an `SRV` record (`_minecraft._tcp.<name>.<base domain>`) with the main port, so players can connect **without typing the port**.
- Keeps the records up to date when the server's main port, type or node changes, and removes them when the subdomain is removed or the server is deleted.
- Shows the address in the server header instead of `IP:port`, with a copy button.

## How it works

### Records

All records are created **DNS only** (grey cloud, `proxied: false`), with automatic TTL and the comment `Aether Panel: server <serverId>`. They are never proxied because game traffic is not HTTP.

The `SRV` record is added when the server type is `minecraft` or `minecraft-java` and the server has a main port. Its data is priority `0`, weight `5`, the main port and the subdomain as target. Other games (including Bedrock) do not read SRV records, so their address still needs the port: the panel shows `name.domain:port` for them.

### Where the name points

The target is chosen in this order:

1. The node's public host, if it is a public IP address (`A` for IPv4, `AAAA` for IPv6).
2. The **DNS target** setting (`panel.dns.target`), if set: an IP gives an `A`/`AAAA` record, a hostname gives a `CNAME`.
3. For the local node, `panel.settings.nodeIp` if it is a public IP.
4. The node's public host as a `CNAME`, if it is a usable hostname (not `localhost`, not an IP, not ending in `.local`, `.internal` or `.localhost`).

If none applies, the change is refused with `ErrSubdomainNoTarget`. Private, loopback, link-local and unspecified addresses are never used as targets.

### Assigning and changing a name

When a name is saved, the panel:

1. Validates it (see [Limits](#limits)) and checks that no other server uses it.
2. Lists the zone's records with the same names. If a record exists there that this server does not own, the name is treated as **taken**: records created outside the panel are never overwritten.
3. Creates the new records, saves the name and record IDs on the server, and only then deletes the old records. If anything fails before saving, the records just created are removed again and the server keeps its old address.

Changes are serialised panel-wide, so two servers cannot claim the same free name at once.

### Automatic assignment

With **Give every new server a subdomain** on, a server created from the panel or through the [provisioning API](../reference/11-api-reference.md) gets a subdomain in the background, a moment after creation. The name is the server name in lowercase, with accents removed and any other character turned into a hyphen, cut to 32 characters (`My Survival` → `my-survival`). A name shorter than 3 characters falls back to `srv-<serverId>`. If the name is taken, reserved or already used in the zone, `-2`, `-3`… is appended (up to 20 attempts). Failures are only written to the panel log.

### Keeping records in sync

The records are updated in the background (about half a second after the change is saved) when:

- the main port or the server type changes (`PUT /api/servers/:serverId/definition`),
- an administrator changes the main port (`PUT /api/servers/:serverId/data`),
- the main port is changed in the port settings (`PUT /api/servers/:serverId/port-settings`),
- the server is transferred to another node.

The name never changes; an `A` record can become a `CNAME` (or the reverse) and the `SRV` record is created or removed as needed. This is best effort: a Cloudflare failure is logged and does not undo the server change.

When a server is deleted (from the panel or the provisioning API), its records are removed. If Cloudflare is no longer configured at that point, the records stay in the zone and a message is logged.

### Panel domain behind Cloudflare's proxy

This feature is separate from the panel's own domain (the Base URL). If the panel domain goes through Cloudflare's proxy (orange cloud), it does not lead to the server's real IP, so game ports cannot use it. The installer's domain service detects this and records the server's real IP, which the panel then uses as the local node's address. If that is not the case on your install, set **DNS target** to the server's public IP.

## Using it

### In the panel

**Administrator: connect Cloudflare** (requires `settings.edit`)

1. In Cloudflare, go to **My Profile → API Tokens → Create Token** and use the **Edit zone DNS** template (permission *Zone → DNS → Edit*). Under *Zone Resources*, limit it to your domain's zone. Copy the token: it is shown only once.
2. Copy the **Zone ID** from the domain's *Overview* page (right-hand column, *API* section). It has 32 hexadecimal characters.
3. In the panel, open **Settings → Domains** and fill in:
   - **Base domain**: the zone or a subdomain of it, for example `play.example.com` (no `http://`, no slashes).
   - **DNS target (optional)**: only needed when nodes are registered by hostname or private address, or the panel domain is proxied.
   - **API token** and **Zone ID**.
4. Click **Test connection**. The test uses the values on screen, saved or not: it reads the zone, checks that the base domain belongs to it and creates and deletes a temporary `TXT` record (`_aether-panel-test.<domain>`). On success it shows the zone name.
5. Optionally turn on **Give every new server a subdomain**, then save.

Subdomains are active once the token, the zone ID and the base domain are all set.

**Server owner: pick an address** (requires `server.flags.edit` or `server.data.edit` on the server)

1. Open the server's **Settings** tab and find the **Server address** card. It is hidden when the administrator has not configured Cloudflare.
2. Type a name. The status updates as you type: **Available**, **This is the current address**, **Another server already uses it**, **That name is reserved**, or the format error.
3. Click **Save**. **Show DNS records** lists the records the panel manages.
4. To go back to `IP:port`, click **Remove** and confirm (**Remove? Click again**).

DNS changes take about a minute to reach players. Users without the permission above keep seeing `IP:port` in the server header.

### Via the API

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| `GET` | `/api/servers/:serverId/subdomain` | `server.flags.edit` or `server.data.edit` | Whether subdomains are enabled, the base domain, the server's subdomain, full address, `srv`, port and planned records. |
| `GET` | `/api/servers/:serverId/subdomain/check?name=<name>` | `server.flags.edit` or `server.data.edit` | `{"available": bool, "reason": "" \| "current" \| "invalid" \| "reserved" \| "taken"}`. Does not query Cloudflare. |
| `PUT` | `/api/servers/:serverId/subdomain` | `server.flags.edit` or `server.data.edit` | Body `{"subdomain": "survival"}`. Creates the records and returns the same view as `GET`. |
| `DELETE` | `/api/servers/:serverId/subdomain` | `server.flags.edit` or `server.data.edit` | Removes the records. Answers `204`. |
| `POST` | `/api/settings/test/cloudflare` | `settings.edit` | Tests the connection. Optional body overrides the saved `panel.dns.cloudflare.token`, `panel.dns.cloudflare.zoneId` and `panel.dns.baseDomain` (a `********` token, what the page receives instead of the saved one, uses the saved value). Returns `{"zone": "<zone name>"}`. |

The Cloudflare keys are saved with the panel settings endpoints (`POST /api/settings`). Full details are in the [API reference](../reference/11-api-reference.md).

## Requirements and configuration

- A domain whose DNS is hosted on Cloudflare, and an API token with *Zone → DNS → Edit* on that zone.
- The panel needs outbound HTTPS access to `api.cloudflare.com`.
- Nodes must be reachable from the internet at the address the records point to.

| Key | Default | Environment variable | Description |
|-----|---------|----------------------|-------------|
| `panel.dns.cloudflare.token` | `""` | `SKYPANEL_PANEL_DNS_CLOUDFLARE_TOKEN` | API token. Sent only as a `Bearer` header and replaced by `[token]` in any error message. |
| `panel.dns.cloudflare.zoneId` | `""` | `SKYPANEL_PANEL_DNS_CLOUDFLARE_ZONEID` | Zone ID, 32 hexadecimal characters. |
| `panel.dns.baseDomain` | `""` | `SKYPANEL_PANEL_DNS_BASEDOMAIN` | Domain under which names are created. |
| `panel.dns.target` | `""` | `SKYPANEL_PANEL_DNS_TARGET` | IP or hostname to point at when a node has no public IP. Empty: each node's public address. |
| `panel.dns.autoAssign` | `false` | `SKYPANEL_PANEL_DNS_AUTOASSIGN` | Give new servers a subdomain automatically. |

All five are editable from **Settings → Domains**. The token is stored in the panel's configuration file and is returned to administrators with `settings.edit`. See [Configuration](../backend/02-configuration.md).

## Limits

- **Name format**: 3 to 32 characters, lowercase letters, digits and hyphens, not starting or ending with a hyphen. Input is lower-cased and trimmed.
- **Reserved names**: `www`, `mail`, `panel`, `api`, `admin`, `ftp`, `ns1`–`ns4`, `smtp`, `imap`, `pop`, `pop3`, `webmail`, `mx`, `dns`, `cdn`, `status`, `dashboard`, `billing`, `support`, `help`, `docs`, `autodiscover`, `autoconfig`, `localhost`, `root`, `node`, `sftp`, `ssh`, `vpn`, `cpanel`, `whm`, `app`, `auth`, `login`, `static`, `assets`, `dev`, `test`.
- One subdomain per server, and each name can be used by only one server.
- A server can change or remove its subdomain at most once every **10 seconds**.
- Each Cloudflare call times out after 15 seconds; one change is bounded to 45 seconds.
- Only one zone and one base domain per panel.
- The SRV record is only created for Minecraft Java (`minecraft`, `minecraft-java`).

## Troubleshooting

Errors from the panel are returned in Spanish; the codes in brackets are stable.

| Message | Cause and fix |
|---------|---------------|
| `los subdominios automáticos no están configurados en este panel` (`ErrSubdomainDisabled`, 400) | Token, zone ID or base domain missing. Complete **Settings → Domains**. |
| `el subdominio solo puede tener letras minúsculas, números y guiones…` (`ErrSubdomainInvalid`, 400) | Name too short or long, or with invalid characters. |
| `ese subdominio está reservado` (`ErrSubdomainReserved`, 400) | The name is on the reserved list. Pick another. |
| `ese subdominio ya está en uso` (`ErrSubdomainTaken`, 409) | Another server uses it, **or** a record with that name (or its `_minecraft._tcp.` SRV name) already exists in the zone outside the panel. Delete the foreign record in Cloudflare or pick another name. |
| `espera unos segundos antes de volver a cambiar el subdominio` (`ErrSubdomainRateLimited`, 429) | Two changes within 10 seconds. Wait and retry. |
| `el nodo de este servidor no tiene una dirección pública; configura un destino DNS…` (`ErrSubdomainNoTarget`, 422) | The node is registered with a private IP, `localhost` or an internal hostname. Set **DNS target** to the public IP or hostname, or register the node with its public address. |
| `faltan el token de la API o el ID de zona de Cloudflare` (400, test) | Token or zone ID empty. |
| `el ID de zona de Cloudflare debe tener 32 caracteres hexadecimales` | Wrong zone ID (often the account ID was pasted). Copy the *Zone ID* from the domain's Overview page. |
| `el dominio base no es un nombre de dominio válido` (400, test) | The base domain contains `http://`, slashes or invalid characters. |
| `el dominio <domain> no pertenece a la zona <zone>` (422, test) | The base domain is not the zone or a subdomain of it. Use a domain inside the selected zone. |
| `Cloudflare respondió: … (código <n>)` (422) | Cloudflare refused the call. The panel shows Cloudflare's own message and code as received; the usual causes are an invalid token or a token without DNS edit permission on that zone. Recreate the token with the *Edit zone DNS* template restricted to the right zone. |
| `no se pudo contactar con Cloudflare: …` | The panel cannot reach `api.cloudflare.com` (firewall, DNS, no outbound internet). |
| The address resolves but players cannot connect | The record points at a proxied domain or the wrong IP. Open **Show DNS records** and check the target; if it is the panel's proxied domain, set **DNS target** to the server's public IP. Also check that the game port is open on the node. |
| Records left in the zone after deleting a server | Cloudflare was not configured, or the call failed, when the server was deleted. Remove the records manually; they carry the comment `Aether Panel: server <serverId>`. |

## See also

- [Servers — Subdomain (Cloudflare)](../backend/04-servers.md#subdomain-cloudflare)
- [Installation](../setup/01-installation.md)
- [Configuration](../backend/02-configuration.md)
- [Scopes reference](../general/scopes-reference.md)
- [API reference](../reference/11-api-reference.md)
