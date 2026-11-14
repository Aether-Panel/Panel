# License System

Aether Panel can verify a license key against an external license API and store the resulting plan in its configuration.

> **Current state:** activation works (verify, bind, store), but **nothing in the panel reads the stored license yet**. No feature is enabled or blocked by the plan, there is no license screen in the frontend, and there is no periodic re-check, cache or expiry handling. The rest of this page describes exactly what the code does today.

---

## External API

`internal/services/license.go` uses a fixed base URL (constant `LicenseAPIBaseURL`, not configurable):

```
https://prueba.skyhostingcloud.com/api/public/licenses
```

Requests use a plain `http.Client` with a 30-second timeout and `Content-Type`/`Accept: application/json`.

| Call | Request | Accepted status |
|------|---------|-----------------|
| Verify | `GET {base}/verify?licenseKey=<key>` | `200` |
| Bind | `POST {base}/verify` with `{"licenseKey","serverId","serverIp"}` | `200` or `201` |

Any other status is returned as an error containing the status code and the response body. On bind, a `200`/`201` whose body is not JSON is treated as success. The key is URL-encoded in the query string, so a key containing `&`, `#` or spaces is sent whole.

### Verify response

Fields the panel decodes (`LicenseVerifyResponse`):

```json
{
  "valid": true,
  "status": "active",
  "isInGracePeriod": false,
  "license": {
    "key": "ABCD-EFGH-IJKL-MNOP",
    "plan": "professional",
    "maxServers": 10,
    "usedServers": 2,
    "expiryDate": "2027-01-01T00:00:00Z",
    "daysRemaining": 61,
    "billingCycle": "monthly",
    "boundServerId": "panel-host",
    "boundServerIp": "203.0.113.10"
  },
  "user": { "email": "…", "name": "…" },
  "payment": { "hasCompletedPayment": true, "lastPaymentDate": "…" },
  "validation": { "timestamp": "…", "ip": "…" }
}
```

Only `valid`, `license.plan` and the `license` fields echoed in the activation response are used.

---

## Configuration keys

Defined in `internal/config/entries.go`. All four are editable from the settings API (`settings.edit`), see [Configuration](../backend/02-configuration.md).

| Key | Environment variable | Default | Description |
|-----|----------------------|---------|-------------|
| `panel.license.key` | `SKYPANEL_PANEL_LICENSE_KEY` | `""` | Last activated key |
| `panel.license.status` | `SKYPANEL_PANEL_LICENSE_STATUS` | `free` | Type derived from the plan: `free`, `pro` or `enterprise` |
| `panel.license.serverId` | `SKYPANEL_PANEL_LICENSE_SERVERID` | `""` | Identifier sent when binding. Generated on first activation |
| `panel.license.serverIp` | `SKYPANEL_PANEL_LICENSE_SERVERIP` | `""` | IP sent when binding. Generated on first activation |

---

## Activation: `POST /api/settings/license/activate`

Permission: `settings.edit`.

Request:

```json
{ "key": "ABCD-EFGH-IJKL-MNOP" }
```

Steps (`activateLicense` in `internal/web/api/settings.go`):

1. `key` is required. With dashes removed and upper-cased it must be exactly **16 characters**, otherwise `400 invalid license key format`.
2. **Verify** the key. A request error or `valid: false` returns `400` with `{"success": false, "message": "…"}`.
3. Fill in the binding data if it is not stored yet, and save it:
   - `serverId`: the host name (`unknown` if it cannot be read).
   - `serverIp`: the first non-loopback IPv4 the host name resolves to, otherwise the first non-loopback IPv4 of the network interfaces, otherwise `127.0.0.1`.
4. **Bind** the key to that `serverId`/`serverIp`. A bind failure is only logged; the activation continues.
5. Save `panel.license.key` and `panel.license.status`.

Response `200`:

```json
{
  "success": true,
  "type": "pro",
  "license": {
    "key": "ABCD-EFGH-IJKL-MNOP",
    "plan": "professional",
    "maxServers": 10,
    "usedServers": 2,
    "expiryDate": "2027-01-01T00:00:00Z",
    "daysRemaining": 61,
    "billingCycle": "monthly"
  },
  "permissions": { "hasPlugins": true },
  "bound": true,
  "message": "License activated successfully"
}
```

`bound` is `false` when the bind call failed or answered `success: false`.

```bash
curl -X POST https://panel.example.com/api/settings/license/activate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key": "ABCD-EFGH-IJKL-MNOP"}'
```

There is no endpoint to read the license status back other than the generic settings API (`GET /api/settings/panel.license.status`).

---

## Plan mapping

`GetLicenseType` turns the verification into the stored type:

| Verification | Stored `panel.license.status` |
|--------------|-------------------------------|
| `valid: false` | `free` (not stored: activation stops before) |
| plan `personal` | `free` |
| plan `professional` | `pro` |
| plan `enterprise` | `enterprise` |
| any other plan | `pro` |

`ExtractPermissions` returns `hasPlugins: true` for `professional` and `enterprise`. This value only appears in the activation response; no endpoint or page checks it.

---

## Security notes

- The key is stored in plain text in the panel configuration. `GET /api/settings` returns it masked (`********`), like the other secrets (see [Panel settings](../features/panel-settings.md#how-it-works)).
- Error messages from the license server (status and body) are logged and returned in the activation response.
- The license API is called over HTTPS; the client is not the restricted (SSRF-safe) client because the URL is a constant.

---

## Known limitations

- The license server is a test host hard-coded in the code (`prueba.skyhostingcloud.com`, see [External API](#external-api)); it cannot be changed without rebuilding.
- The stored license does not enable or block anything yet (see the note at the top).
