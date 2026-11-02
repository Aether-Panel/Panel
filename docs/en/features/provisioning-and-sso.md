# Billing integration: provisioning API and sign-in links

Aether Panel exposes a small API for billing systems such as WHMCS or Paymenter. With it the billing system can create a server when a customer pays, suspend it when an invoice is overdue, change its plan, delete it when the service ends, and send the customer to the panel already signed in. This page explains the pieces an administrator sets up and how they behave; the request and response fields of every route are in the [API reference: Provisioning / billing](../reference/api/07-provisioning.md).

## What it does

- **API keys** authenticate the billing system. Each key can be limited to the actions the integration needs.
- **Products** describe a plan: which template to install, on which node, with how much CPU, memory and disk, and optionally from which port range.
- **Provisioning routes** under `/api/v1` act on servers: create, terminate, suspend, unsuspend and change package.
- **Sign-in links (SSO)**: the billing system asks for a one-time link that signs the customer in without a password.

No billing module ships in this repository; the integration on the billing side (WHMCS module, Paymenter extension or your own code) must call these routes.

## How it works

### API keys

- A key looks like `ak_xxxxx_…`: the prefix `ak_` plus 5 hex characters is stored in clear to identify the key; the full token is stored only as a SHA-256 hash. The token is shown once, when it is created.
- The billing system sends it in `X-Api-Key: ak_…` or `Authorization: Bearer ak_…`. Every `/api/v1` route requires a valid key, including `GET /api/v1/ping`.
- Each key holds a list of actions:

| Action | Allows |
|--------|--------|
| `create` | `POST /api/v1/provision` |
| `terminate` | `POST /api/v1/terminate` |
| `suspend` | `POST /api/v1/suspend` and `POST /api/v1/unsuspend` |
| `change-package` | `POST /api/v1/change-package` |
| `sso` | `POST /api/v1/sso` |
| `provision` | Every action. Keys created before keys could be limited hold it, and so does a key created through the API without a list. |

A list that names no known action is refused. Unknown names in a list are dropped.

### Products

| Field | Meaning |
|-------|---------|
| `product_id` | The identifier the billing system sends (unique), e.g. `minecraft_2gb` |
| `display_name` | Used as the server name when the request has no `server_name` |
| `template` | Template name. The panel looks it up in every template repository and uses the first match |
| `cpu` | CPU limit in % (100 = one core). `0` keeps the template's value |
| `memory`, `disk` | Limits in MB. `0` keeps the template's value |
| `default_node` | Node the server is created on (`0` is the local node) |
| `node_id` | Optional, API only: when set and not `0`, it wins over `default_node` |
| `port_range_min`, `port_range_max` | Optional range. `0` in either field means the whole range 1024–65535 |

### Creating a server (`POST /api/v1/provision`)

1. **Idempotency**: if `service_id` is sent and a server already carries it, the panel answers with that server instead of creating another (the password comes back empty).
2. The product is looked up by `product_id`.
3. The customer is found by email. If there is no account, one is created with the given `username` and `password`, or generated ones (`user_` plus up to 8 random characters, and a 16-character password). New accounts get the login permission and, if a role named `Usuario` exists, that role. **An existing account keeps its password**; the response then returns an empty `password`.
4. The template and node are resolved, and a random free port is picked from the product's range (ports used by any server on that node, extra ports included, are skipped).
5. The server record is created with the customer as **owner**, the service id stored as its external id, and the standard owner permissions granted on it.
6. The node creates the server with the assigned port and the product's limits injected into the template variables `port`, `cpu`, `memory` and `disk`. If the node refuses, the record is removed so a retry can start clean.
7. After success: the first installation starts if `panel.servers.autoInstall` is on, an automatic subdomain is scheduled if automatic subdomains are configured and enabled, the "added to server" email is sent, and the `server.created` extension event is published.

The response contains `success`, `server_id`, `username`, `password` and `port`.

### Suspending, unsuspending, terminating, changing package

Every action names the server with `service_id` (the billing system's id, preferred) or `server_id` (the panel identifier). Sub-servers created from a server follow their parent.

- **Suspend** marks the server and its sub-servers as suspended (the panel refuses to start them) and stops the ones that are running on their node.
- **Unsuspend** clears the flag. It does **not** start the servers.
- **Terminate** deletes the sub-servers first, then the server: each one is stopped if running, deleted on its node (a node that no longer knows it counts as deleted), its subdomain records are released and the panel record is removed. The `server.deleted` extension event is published for each one.
- **Change package** takes a `product_id` (its limits are used) or explicit `cpu`, `memory` and `disk`. The node is updated first; only then the panel record. The server keeps its files. The response says `restart_required: true` when the server is running (or its state cannot be read): new CPU and memory take effect on the next restart; the disk limit counts at once.

### Sign-in links (`POST /api/v1/sso`)

1. The billing system sends the customer's `email` and, optionally, `service_id` or `server_id`.
2. The panel refuses administrator accounts: any account holding a panel-wide privileged permission (`admin`, `settings.edit`, node, template, user or OAuth2 management permissions). A leaked billing key therefore cannot open the admin panel.
3. When a server is named, the customer must have `server.view` on it; the link then lands on that server's page. Otherwise it lands on the server list.
4. The panel returns `url` (`<panel>/auth/sso?token=…`) and `expires_in` (60).
5. The browser opens the link. `GET /auth/sso` spends the token, checks that the account may still sign in, opens a normal session (no two-factor prompt) and redirects to the destination. The sign-in appears in the access log as **Billing sign-in**.

Tokens are random 32-byte values; only their hash is kept, **in memory**. A token works once and for one minute. Restarting the panel discards every pending link.

The panel address in the link is `panel.settings.masterUrl` when it is set in the configuration file or environment; otherwise the address the billing system used to call the panel (`https` when the request came over TLS or with `X-Forwarded-Proto: https`).

## Using it

### In the panel

Both tabs are under **Settings** and are only shown to users with the `admin` permission.

**API keys** (**Settings → API keys**)

1. Press **Generate key**, give it a **Name** (for example "WHMCS server 1").
2. Under **What it can do**, tick only the actions the integration needs: **Create servers**, **Terminate**, **Suspend and unsuspend**, **Change plan**, **Sign-in links**.
3. Press **Generate** and copy the key from the **Key generated** dialog. It is not shown again.
4. To stop a key, use **Revoke key**. Integrations using it fail to authenticate immediately.

**Products** (**Settings → Products**)

1. Press **Add product**.
2. Fill in **Product ID** (what the billing system will send), **Display name**, **Template**, **CPU limit** (%), **Memory limit** (MB), **Disk limit** (MB) and **Default node**.
3. Optionally enable **Assign a port from a range** and set **Min port** and **Max port**.
4. Press **Save product**. Deleting a product only prevents new orders; existing servers are not affected.

The form does not expose `node_id`; set it through the API if needed.

### Via the API

| Method | Path | Auth / permission | Purpose |
|--------|------|-------------------|---------|
| `GET` | `/api/settings/apikeys` | session, `admin` | List keys (name, prefix, permissions, creation date) |
| `POST` | `/api/settings/apikeys` | session, `admin` | Create a key: `{ "name": "...", "permissions": ["create", ...] }`. Returns `token` once |
| `DELETE` | `/api/settings/apikeys/:id` | session, `admin` | Revoke a key |
| `GET`, `POST` | `/api/provision/products` | session, `admin` | List / create products |
| `PUT`, `DELETE` | `/api/provision/products/:id` | session, `admin` | Update / delete a product (`:id` is the numeric id, not `product_id`) |
| `GET` | `/api/v1/ping` | any valid key | Connectivity test: `{ "success": true, "message": "Pong" }` |
| `POST` | `/api/v1/provision` | key with `create` | Create a server from a product |
| `POST` | `/api/v1/suspend` | key with `suspend` | Suspend a server and its sub-servers |
| `POST` | `/api/v1/unsuspend` | key with `suspend` | Lift the suspension |
| `POST` | `/api/v1/change-package` | key with `change-package` | Change limits |
| `POST` | `/api/v1/terminate` | key with `terminate` | Delete a server and its sub-servers |
| `POST` | `/api/v1/sso` | key with `sso` | Get a one-time sign-in link |
| `GET` | `/auth/sso?token=…` | none (the token) | Opened by the customer's browser |

Request bodies, responses and `curl` examples are in the [API reference](../reference/api/07-provisioning.md).

## Requirements and configuration

- The billing system must reach the panel over HTTPS (keys travel in headers).
- At least one product whose template exists in a template repository and whose node is reachable.
- Email configured if customers should receive the "added to server" message.

| Key | Default | Environment variable | Effect here |
|-----|---------|----------------------|-------------|
| `panel.settings.masterUrl` | `http://localhost:8080` | `SKYPANEL_PANEL_SETTINGS_MASTERURL` | Base of the sign-in link, used only when set explicitly in the config file or environment |
| `panel.servers.autoInstall` | `true` | `SKYPANEL_PANEL_SERVERS_AUTOINSTALL` | Runs the template installation right after provisioning |
| `panel.dns.autoAssign` | `false` | `SKYPANEL_PANEL_DNS_AUTOASSIGN` | Gives provisioned servers an automatic subdomain (also needs the Cloudflare settings) |

See [Configuration](../backend/02-configuration.md) for every key.

## Limits

- Sign-in links last 60 seconds, work once, and are lost on a panel restart.
- Sign-in links are never issued for administrator accounts.
- `unsuspend` does not start servers; the customer (or a later call) must start them.
- A product's template is matched by name in the first repository that has it; two repositories with the same template name are not distinguished.
- When no free port is found in the range, the server is created with port `0` and the template's own port setting is left as is. Use ranges large enough for the expected number of servers.
- If suspending succeeds in the panel but a running server cannot be stopped on its node, the call answers `502` while the server stays marked as suspended.

## Troubleshooting

| Message | Cause | Fix |
|---------|-------|-----|
| `401 Unauthorized - Missing or invalid API Key format` | No key, or it does not start with `ak_` | Send the key in `X-Api-Key` or `Authorization: Bearer`. |
| `401 Invalid API Key` | The key was revoked or mistyped | Generate a new key and update the integration. |
| `403` `this API key is not allowed to do that` (`ErrAPIKeyNotAllowed`) | The key lacks the action for that route | Create a key with the needed actions (keys cannot be edited). |
| `pick at least one action, from: create, terminate, suspend, change-package, sso` | Key created with a list containing no known action | Use the action names above. |
| `404` with an empty body on provision | No product with that `product_id`, or the product's node does not exist | Check **Settings → Products** and the product's node. |
| `400` `template not found` | The product's template name is not in any repository | Fix the product's **Template**. |
| `Daemon error` with `details` | The node refused to create the server or apply the limits | Read `details`; check the node and the template. |
| `either service_id or server_id is required` | Suspend/terminate/change-package/SSO without a server reference | Send `service_id` or `server_id`. |
| `404` with an empty body on an action or SSO | No server carries that `service_id`/`server_id`, or no account has that email | The service was never provisioned or was already terminated; check the email. |
| `give a product_id or at least one of cpu, memory and disk` / `resources cannot be negative` | Invalid change-package body | Send a product or positive limits. |
| `502` `internal error, the details are in the panel log` on terminate; the log says `the node refused to delete server … (status N)` or `the node could not delete server …` | The node is down or failed | The record is kept; retry when the node is back. |
| `502` `internal error, the details are in the panel log` on suspend; the log says `server … is suspended, but its node could not be reached to stop it` (or `could not stop it`, `refused to stop it`) | Suspended in the panel, still running on the node | Stop it once the node is reachable, or retry the call. |
| `403` `sign-in links are not issued for administrator accounts` | The email belongs to a privileged account | Use a customer account; admins sign in with their password. |
| `403` `no permission to perform action` on SSO | The customer cannot view the named server | Check the server owner / sub-user permissions. |
| Customer lands on the login page with "That sign-in link was already used or has expired." | Link used twice, opened after 60 s, or the panel restarted | Request a new link each time the button is pressed. |
| "This account is not allowed to sign in to the panel." | The account lost the `login` permission | Restore the login permission or the user's role. |
| Link points to `localhost` or the wrong host | `panel.settings.masterUrl` not set and the billing system calls the panel by an internal address | Set `panel.settings.masterUrl` to the public panel URL. |

## See also

- [API reference: Provisioning / billing](../reference/api/07-provisioning.md) — full request/response details for `/api/v1` and products.
- [Extensions](../reference/15-extensions.md) — `server.created` and `server.deleted` events.
- [Scopes reference](../general/scopes-reference.md) — what the owner permissions include.
- [Configuration](../backend/02-configuration.md).
