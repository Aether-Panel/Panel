# Provisioning / billing

Routes for billing systems (WHMCS, Paymenter...): create, suspend, change the plan of and delete servers, and send the customer to the panel without a password. Products, the plans those servers are built from, are managed with the panel API. Back to the [API reference](../11-api-reference.md). How the pieces fit together from the administrator's side: [Provisioning and SSO](../../features/provisioning-and-sso.md).

Code: `internal/web/api/provision.go`, `internal/web/api/products.go`.

## Routes

| Method | Path | Auth | What it does |
|--------|------|------|--------------|
| GET | `/api/provision/products` | `admin` | Lists products |
| POST | `/api/provision/products` | `admin` | Creates a product |
| PUT | `/api/provision/products/:id` | `admin` | Updates a product |
| DELETE | `/api/provision/products/:id` | `admin` | Deletes a product |
| GET | `/api/v1/ping` | API key | Checks the key and the connection |
| POST | `/api/v1/provision` | API key, `create` | Creates a server for a customer |
| POST | `/api/v1/terminate` | API key, `terminate` | Deletes a server and its sub-servers |
| POST | `/api/v1/suspend` | API key, `suspend` | Suspends and stops |
| POST | `/api/v1/unsuspend` | API key, `suspend` | Lifts a suspension |
| POST | `/api/v1/change-package` | API key, `change-package` | Changes the plan's limits |
| POST | `/api/v1/sso` | API key, `sso` | One-time sign-in link for a customer |

## Products

A product is a plan: which template, on which node, with what limits and ports.

```json
{
  "id": 1,
  "product_id": "minecraft_2gb",
  "display_name": "Minecraft 2GB",
  "template": "minecraft-java",
  "node_id": null,
  "cpu": 100,
  "memory": 2048,
  "disk": 10240,
  "default_node": 1,
  "port_range_min": 25565,
  "port_range_max": 25665
}
```

| Field | Meaning |
|-------|---------|
| `product_id` | Unique text id the billing system sends (e.g. `minecraft_2gb`) |
| `display_name` | Default server name |
| `template` | Template name; the first repository that has it is used |
| `node_id` | Node the servers are created on; when `null` or `0`, `default_node` is used |
| `default_node` | Fallback node id; it does not pick a node by load |
| `cpu`, `memory`, `disk` | Limits: % of a core (100 = one core), MB, MB. `0` keeps the template's value |
| `port_range_min`, `port_range_max` | Range the server's port is picked from; `0` in either means 1024 to 65535 |

`GET` lists them; `POST` takes the object without `id` and answers it with its `id`; `PUT /:id` changes the fields sent and answers the product (`404` when unknown); `DELETE /:id` answers `204`. The panel does not validate the values: a wrong template or node only shows when provisioning.

## Authentication

Keys are created in **Settings → API Keys** or with [`POST /api/settings/apikeys`](./06-panel-admin.md#api-keys) (needs `admin`). A key starts with `ak_` and is shown once. Send it in one of these headers:

```
X-Api-Key: ak_YOUR_KEY
Authorization: Bearer ak_YOUR_KEY
```

| Answer | When |
|--------|------|
| `401 {"error": "Unauthorized - Missing or invalid API Key format"}` | No key, or it does not start with `ak_` |
| `401 {"error": "Invalid API Key"}` | Unknown key |
| `403 ErrAPIKeyNotAllowed` | Valid key without the permission for that route |

Each key says what it may do:

| Permission | Routes |
|------------|--------|
| `create` | `POST /api/v1/provision` |
| `terminate` | `POST /api/v1/terminate` |
| `suspend` | `POST /api/v1/suspend` and `POST /api/v1/unsuspend` |
| `change-package` | `POST /api/v1/change-package` |
| `sso` | `POST /api/v1/sso` |
| `provision` | All of them. Keys created before keys could be limited, and new keys created without a list, hold it |

`GET /api/v1/ping` needs no permission and answers `{"success": true, "message": "Pong"}`.

These requests carry no cookies, so the [same-origin guard](./02-authentication.md#same-origin-guard) does not apply.

## Identifying the service

Routes that act on a server take one of these fields:

| Field | Meaning |
|-------|---------|
| `service_id` | The service's id in the billing system, stored when the server was created with `/provision`. Wins when both are sent |
| `server_id` | The server's identifier in the panel |

With neither: `400 ErrServerRequired`. A server that does not exist: `404`.

## `POST /api/v1/provision`

Permission `create`. Creates a server with a product's limits.

| Field | Required | Meaning |
|-------|----------|---------|
| `product_id` | yes | The product's `product_id` (text) |
| `email` | yes | The customer's email |
| `service_id` | no | The service's id in the billing system; makes the call idempotent |
| `server_name` | no | Defaults to the product's `display_name` |
| `username` | no | For a new account; defaults to `user_` plus up to 8 random characters |
| `password` | no | For a new account; defaults to 16 random characters |

Steps:

1. If `service_id` is sent and a server already has it, that server is answered and nothing is created (`password` comes back empty).
2. Finds the product (`404` when unknown).
3. Finds the account by email. When there is none, creates it with the `Usuario` role (when that role exists) and the `login` scope. An existing account keeps its password and `password` comes back empty.
4. Finds the product's template in the repositories: `400` with code `ErrGeneric` and message `template not found` when none has it. The account from step 3 is kept even then.
5. Picks the node (`node_id`, else `default_node`; `404` when unknown) and a free port in the product's range.
6. Creates the server owned by the customer, with the standard member scopes.
7. Sends the definition to the node with the product's `cpu`, `memory` and `disk` (and the port) as variables, so the container starts with the plan's limits. `disk` is the real disk limit.
8. If the node fails, the server is removed from the panel so the call can be retried: `502` when unreachable, or the node's status with `{"error": "Daemon error", "details": "..."}`.
9. Starts the installation (with `panel.servers.autoInstall`), assigns a subdomain when enabled and emails the customer.

Answer `200`:

```json
{ "success": true, "server_id": "3F9A1C2B7D4E", "username": "user_x7k2p9", "password": "generated_password", "port": 25570 }
```

| Field | Meaning |
|-------|---------|
| `username` | The account's username; on a repeated call, the `username` sent (may be empty) |
| `password` | Only for a new account, only the first time |
| `port` | The assigned port; `0` when the range had none free |

```bash
curl -X POST https://panel.example.com/api/v1/provision \
  -H "X-Api-Key: ak_YOUR_KEY" -H "Content-Type: application/json" \
  -d '{ "product_id": "minecraft_2gb", "email": "customer@example.com", "service_id": "42", "server_name": "Survival" }'
```

## `POST /api/v1/terminate`

Permission `terminate`. Body `{"service_id": "42"}` or `{"server_id": "3F9A1C2B7D4E"}`.

Stops and deletes the server and its sub-servers on the node and in the panel, and removes their subdomain records. A server the node no longer knows counts as deleted. If a node does not delete one, the answer is `502` and the server stays in the panel, so the call can be retried. Answer: `{"success": true}`.

## `POST /api/v1/suspend` and `POST /api/v1/unsuspend`

Permission `suspend`. Same body as terminate.

- `suspend` marks the server and its sub-servers as suspended and stops the running ones. If a node cannot be reached or refuses, the answer is `502`: the server is suspended in the panel but may still run.
- `unsuspend` lifts the suspension of the server and its sub-servers. It does not start them.

Answer: `{"success": true}`.

## `POST /api/v1/change-package`

Permission `change-package`. Moves a server to other limits without touching its files.

| Field | Meaning |
|-------|---------|
| `service_id` / `server_id` | The server |
| `product_id` | Target product; when sent, its `cpu`, `memory` and `disk` are used and the fields below are ignored (`404` when unknown) |
| `cpu`, `memory`, `disk` | % of a core, MB, MB |

`0` or a missing field leaves that limit as it is. Negative values, or no limit at all, answer `400`. The node gets the limits first; if it refuses, its status and `{"error": "Daemon error", "details": "..."}` are returned and the panel changes nothing.

```json
{ "success": true, "server_id": "3F9A1C2B7D4E", "restart_required": true }
```

The disk limit counts at once; CPU and memory apply on the next start. `restart_required` is `true` when the server runs or the node could not tell.

```bash
curl -X POST https://panel.example.com/api/v1/change-package \
  -H "X-Api-Key: ak_YOUR_KEY" -H "Content-Type: application/json" \
  -d '{ "service_id": "42", "product_id": "minecraft_4gb" }'
```

## `POST /api/v1/sso`

Permission `sso`. For the billing system's "go to the panel" button: a link that opens the customer's session without password or 2FA.

| Field | Required | Meaning |
|-------|----------|---------|
| `email` | yes | The account's email |
| `service_id` / `server_id` | no | The link lands on that server (the customer needs `server.view` on it); without them, on the server list |

```json
{ "success": true, "url": "https://panel.example.com/auth/sso?token=ONE_TIME_TOKEN", "expires_in": 60 }
```

| Answer | When |
|--------|------|
| `404` | No account with that email, or the server does not exist |
| `403 ErrSSONotForAdmins` | The account holds a panel-wide power: `admin`, `settings.edit`, creating/editing/deleting/deploying nodes, editing templates or repositories, editing users or their permissions, or `oauth2.auth` (own or from its role). A leaked key cannot open an administrator's session |
| `403 ErrNoPermission` | The customer cannot view the requested server |

The link works once (the first use spends it, even if signing in then fails), expires after 60 seconds, and is kept in memory only as a hash, so a panel restart cancels pending links. Its address is `panel.settings.masterUrl` when set, otherwise the address the billing system called. The billing system should send the customer's browser to `url` right away and not store it. What opening it does: [`GET /auth/sso`](./02-authentication.md#get-authssotoken).
