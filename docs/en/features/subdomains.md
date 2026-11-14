# Automatic subdomains (Cloudflare)

With a Cloudflare zone connected, every server can have its own address, such as `survival.play.example.com`, instead of an IP and port. The panel creates, updates and removes the DNS records itself through the Cloudflare API. This page covers setup for administrators and daily use for server owners; the internals of how records follow a server are summarised in [Servers — Subdomain (Cloudflare)](../backend/04-servers.md#subdomain-cloudflare).

## What it does

- Lets the owner of a server pick a name (`<name>.<base domain>`) from the server's settings, with a live availability check.
- Optionally gives **every new server** a subdomain automatically, derived from its name.
- Creates the DNS records in the administrator's Cloudflare zone:
  - An `A`, `AAAA` or `CNAME` record with the subdomain's name, pointing at the server's node.
  - For Minecraft Java servers, an `SRV` record (`_minecraft._tcp.<name>.<base domain>`) with the main port, so players can connect **without typing the port**.
- Keeps the records up to date when the server's main port, type or node, the node's address or the DNS target changes, and removes them when the subdomain is removed or the server is deleted.
- Moves existing subdomains to the new domain when the administrator changes the base domain or the zone.
- Shows the address in the server header and on its card in the server list instead of `IP:port`, with a copy button in the header.

## How it works

### Records

All records are created **DNS only** (grey cloud, `proxied: false`), with automatic TTL and the comment `Aether Panel: server <serverId>`. They are never proxied because game traffic is not HTTP.

The `SRV` record is added when the server type is `minecraft` or `minecraft-java` and the server has a main port. Its data is priority `0`, weight `5`, the main port and, as target, the subdomain itself; when the subdomain is a `CNAME`, the target is the node's name instead, because an SRV record must not point at an alias (RFC 2782). Other games (including Bedrock) do not read SRV records, so their address still needs the port: the panel shows `name.domain:port` for them.

### Where the name points

The target is chosen in this order:

1. The node's public host, if it is a public IP address (`A` for IPv4, `AAAA` for IPv6).
2. For the local node, `panel.settings.nodeIp` if it is a public IP.
3. The node's public host as a `CNAME`, if it is a usable hostname (not `localhost`, not an IP, not ending in `.local`, `.internal` or `.localhost`).
4. The **DNS target** setting (`panel.dns.target`), if set: an IP gives an `A`/`AAAA` record, a hostname gives a `CNAME`. It is only used for nodes that have none of the addresses above.

This way every server follows its own node, and the node's name may be under another domain than the subdomains. For example, with the node registered as `usa.example.com` and the base domain `example.es`, server `pablo` gets `pablo.example.es` → `CNAME usa.example.com`. The node's name must be **DNS only** (grey cloud) wherever it is hosted; the panel does not create it.

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
- the server is transferred to another node,
- an administrator changes the node's public host (`PUT /api/nodes/:id`), or the master node IP or Base URL for the local node (only that node's servers),
- an administrator changes the **DNS target** (every server with a subdomain).

The name never changes; an `A` record can become a `CNAME` (or the reverse) and the `SRV` record is created or removed as needed. This is best effort: a Cloudflare failure is logged and does not undo the server change.

When a server is deleted (from the panel or the provisioning API), its records are removed. If Cloudflare is no longer configured at that point, the records stay in the zone and a message is logged.

### Changing the base domain or the zone

Each server keeps the base domain and zone its records were made in (`dns_domain`, `dns_zone_id`). When a different base domain or zone ID is saved, the panel moves every subdomain in the background, one at a time: it creates the same name under the new domain, saves it and removes the old records from their original zone (with the same token, which needs permission on both zones). Until a server has moved, the panel keeps showing its old address, which still works.

If the name already exists in the new zone outside the panel, that server is not moved and keeps its old address; the reason goes to the panel log. Subdomains made before this version have no stored domain: the panel records the current settings into them just before saving the new ones.

### Panel domain behind Cloudflare's proxy

This feature is separate from the panel's own domain (the Base URL). If the panel domain goes through Cloudflare's proxy (orange cloud), it does not lead to the server's real IP, so game ports cannot use it. The installer's domain service detects this and records the server's real IP, which the panel then uses as the local node's address. If that is not the case on your install, set the server's public IP as the **Global Master Node IP** (or as the **DNS target**).

## Using it

### In the panel

**Administrator: connect Cloudflare** (requires `settings.edit`)

1. In Cloudflare, go to **My Profile → API Tokens → Create Token** and use the **Edit zone DNS** template (permission *Zone → DNS → Edit*). Under *Zone Resources*, limit it to your domain's zone. Copy the token: it is shown only once.
2. Copy the **Zone ID** from the domain's *Overview* page (right-hand column, *API* section). It has 32 hexadecimal characters.
3. In the panel, open **Settings → Domains** and fill in:
   - **Base domain**: the zone or a subdomain of it, for example `play.example.com` (no `http://`, no slashes).
   - **DNS target (optional)**: only needed for nodes without a public address (a private IP or an internal name).
   - **API token** and **Zone ID**.
4. Click **Test connection**. The test uses the values on screen, saved or not: it reads the zone, checks that the base domain belongs to it and creates and deletes a temporary `TXT` record (`_aether-panel-test.<domain>`). On success it shows the zone name.
5. Optionally turn on **Give every new server a subdomain**, then save.

Subdomains are active once the token, the zone ID and the base domain are all set.

**Server owner: pick an address** (requires `server.flags.edit` or `server.data.edit` on the server)

1. Open the server's **Settings** tab and find the **Server address** card. It is hidden when the administrator has not configured Cloudflare.
2. Type a name. The status updates as you type: **Available**, **This is the current address**, **Another server already uses it**, **That name is reserved**, or the format error.
3. Click **Save**. **Show DNS records** lists the records the panel manages.
4. To go back to `IP:port`, click **Remove** and confirm (**Remove? Click again**).

DNS changes take about a minute to reach players. Users without the permission above see the address on the server's card in the server list, but keep seeing `IP:port` in the server header.

**Nodes with their own name.** When a node's public host is a name (`usa.example.com`), the server list shows that name instead of the IP on the cards of its servers without a subdomain. For the local node, set the name in **Settings → General → Global master node IP**.

### Via the API

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| `GET` | `/api/servers/:serverId/subdomain` | `server.flags.edit` or `server.data.edit` | Whether subdomains are enabled, the base domain, the server's subdomain, full address, `srv`, port and planned records. |
| `GET` | `/api/servers/:serverId/subdomain/check?name=<name>` | `server.flags.edit` or `server.data.edit` | `{"available": bool, "reason": "" \| "current" \| "invalid" \| "reserved" \| "taken"}`. Does not query Cloudflare. |
| `PUT` | `/api/servers/:serverId/subdomain` | `server.flags.edit` or `server.data.edit` | Body `{"subdomain": "survival"}`. Creates the records and returns the same view as `GET`. |
| `DELETE` | `/api/servers/:serverId/subdomain` | `server.flags.edit` or `server.data.edit` | Removes the records. Answers `204`. |
| `POST` | `/api/settings/test/cloudflare` | `settings.edit` | Tests the connection. Optional body overrides the saved `panel.dns.cloudflare.token`, `panel.dns.cloudflare.zoneId` and `panel.dns.baseDomain` (a `********` token, what the page receives instead of the saved one, uses the saved value). Returns `{"zone": "<zone name>"}`. |

The Cloudflare keys are saved with the panel settings endpoints (`POST /api/settings`). Each server's information (`GET /api/servers`, `GET /api/servers/:serverId`) includes `subdomain` (the full address, when it has records) and `subdomainSrv` (`true` when the address needs no port). Full details are in the [API reference](../reference/11-api-reference.md).

## Requirements and configuration

- A domain whose DNS is hosted on Cloudflare, and an API token with *Zone → DNS → Edit* on that zone.
- The panel needs outbound HTTPS access to `api.cloudflare.com`.
- Nodes must be reachable from the internet at the address the records point to.

| Key | Default | Environment variable | Description |
|-----|---------|----------------------|-------------|
| `panel.dns.cloudflare.token` | `""` | `SKYPANEL_PANEL_DNS_CLOUDFLARE_TOKEN` | API token. Sent only as a `Bearer` header and replaced by `[token]` in any error message. |
| `panel.dns.cloudflare.zoneId` | `""` | `SKYPANEL_PANEL_DNS_CLOUDFLARE_ZONEID` | Zone ID, 32 hexadecimal characters. |
| `panel.dns.baseDomain` | `""` | `SKYPANEL_PANEL_DNS_BASEDOMAIN` | Domain under which names are created. |
| `panel.dns.target` | `""` | `SKYPANEL_PANEL_DNS_TARGET` | IP or hostname to point at when a node has neither a public IP nor a usable name. Empty: each node's public address. |
| `panel.dns.autoAssign` | `false` | `SKYPANEL_PANEL_DNS_AUTOASSIGN` | Give new servers a subdomain automatically. |

All five are editable from **Settings → Domains**. The token is stored in the panel's configuration file and is returned to administrators with `settings.edit`. See [Configuration](../backend/02-configuration.md).

## Limits

- **Name format**: 3 to 32 characters, lowercase letters, digits and hyphens, not starting or ending with a hyphen. Input is lower-cased and trimmed.
- **Reserved names**: `www`, `mail`, `panel`, `api`, `admin`, `ftp`, `ns1`–`ns4`, `smtp`, `imap`, `pop`, `pop3`, `webmail`, `mx`, `dns`, `cdn`, `status`, `dashboard`, `billing`, `support`, `help`, `docs`, `autodiscover`, `autoconfig`, `localhost`, `root`, `node`, `sftp`, `ssh`, `vpn`, `cpanel`, `whm`, `app`, `auth`, `login`, `static`, `assets`, `dev`, `test`.
- One subdomain per server, and each name can be used by only one server.
- A server can change or remove its subdomain at most once every **10 seconds**.
- Each Cloudflare call times out after 15 seconds; one change is bounded to 45 seconds.
- Only one active zone and base domain per panel; changing them moves the subdomains (see [Changing the base domain or the zone](#changing-the-base-domain-or-the-zone)).
- The SRV record is only created for Minecraft Java (`minecraft`, `minecraft-java`).

## Troubleshooting

Errors from the panel carry a stable code (in brackets) and an English message; the panel shows each one translated into the viewer's language, with the values the error carries (port, domain, Cloudflare's message) filled in.

| Message | Cause and fix |
|---------|---------------|
| `Automatic subdomains are not set up on this panel.` (`ErrSubdomainDisabled`, 400) | Token, zone ID or base domain missing. Complete **Settings → Domains**. |
| `A subdomain may only have lowercase letters, digits and hyphens…` (`ErrSubdomainInvalid`, 400) | Name too short or long, or with invalid characters. |
| `That subdomain is reserved.` (`ErrSubdomainReserved`, 400) | The name is on the reserved list. Pick another. |
| `That subdomain is already in use.` (`ErrSubdomainTaken`, 409) | Another server uses it, **or** a record with that name (or its `_minecraft._tcp.` SRV name) already exists in the zone outside the panel. Delete the foreign record in Cloudflare or pick another name. |
| `Wait a few seconds before changing the subdomain again.` (`ErrSubdomainRateLimited`, 429) | Two changes within 10 seconds. Wait and retry. |
| `This server's node has no public address. Set a DNS target…` (`ErrSubdomainNoTarget`, 422) | The node is registered with a private IP, `localhost` or an internal hostname. Register the node with its public IP or name, or set **DNS target** to the public IP or hostname. |
| `The Cloudflare API token or zone ID is missing.` (`ErrCloudflareNotConfigured`, 400, test) | Token or zone ID empty. |
| `The Cloudflare zone ID must be 32 hexadecimal characters.` (`ErrCloudflareZoneID`) | Wrong zone ID (often the account ID was pasted). Copy the *Zone ID* from the domain's Overview page. |
| `The base domain is not a valid domain name.` (`ErrDNSBaseDomainInvalid`, 400, test) | The base domain contains `http://`, slashes or invalid characters. |
| `The domain <domain> does not belong to the zone <zone>.` (`ErrCloudflareDomainNotInZone`, 422, test) | The base domain is not the zone or a subdomain of it. Use a domain inside the selected zone. |
| `Cloudflare answered: … (code <n>)` (`ErrCloudflareRefusedCode`, or `ErrCloudflareRefused` without a code; 422) | Cloudflare refused the call. The panel shows Cloudflare's own message and code as received; the usual causes are an invalid token or a token without DNS edit permission on that zone. Recreate the token with the *Edit zone DNS* template restricted to the right zone. |
| `Could not reach Cloudflare: …` (`ErrCloudflareUnreachable`) | The panel cannot reach `api.cloudflare.com` (firewall, DNS, no outbound internet). |
| The address resolves but players cannot connect | The record points at a proxied name or the wrong IP. Open **Show DNS records** and check the target; if it is the node's name, make it **DNS only** (grey cloud) or register the node with its public IP. Also check that the game port is open on the node. |
| After changing the base domain, a server still has the old address | The name already existed in the new zone outside the panel, or the token lacks permission on the old or the new zone. The reason is in the panel log. Remove the foreign record or pick another name from the **Server address** card. |
| Records left in the zone after deleting a server | Cloudflare was not configured, or the call failed, when the server was deleted. Remove the records manually; they carry the comment `Aether Panel: server <serverId>`. |

## See also

- [Servers — Subdomain (Cloudflare)](../backend/04-servers.md#subdomain-cloudflare)
- [Installation](../setup/01-installation.md)
- [Configuration](../backend/02-configuration.md)
- [Scopes reference](../general/scopes-reference.md)
- [API reference](../reference/11-api-reference.md)
