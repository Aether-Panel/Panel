# License System

Aether Panel integrates with SkyHosting Cloud's license API for plan validation and feature entitlements.

---

## Overview

The license system validates license keys against an external API and controls feature access based on plan tier.

**External API:** `https://prueba.skyhostingcloud.com/api/public/licenses`

---

## Configuration

```json
{
  "panel": {
    "license": {
      "key": "YOUR_LICENSE_KEY",
      "status": "free",
      "serverId": "server_identifier",
      "serverIp": "192.168.1.100"
    }
  }
}
```

**Environment Variables:**
| Variable | Config Key | Description |
|----------|------------|-------------|
| `SKYPANEL_PANEL_LICENSE_KEY` | `panel.license.key` | License key from SkyHosting |
| `SKYPANEL_PANEL_LICENSE_STATUS` | `panel.license.status` | `free`/`pro`/`enterprise` |
| `SKYPANEL_PANEL_LICENSE_SERVERID` | `panel.license.serverId` | Server identifier for binding |
| `SKYPANEL_PANEL_LICENSE_SERVERIP` | `panel.license.serverIp` | Server public IP for binding |

---

## License Plans

| Plan | Plan Key | HasPlugins | Max Servers | Features |
|------|----------|------------|-------------|----------|
| **Free** | `personal` | ❌ | 1 | Basic panel |
| **Pro** | `professional` | ✅ | 10 | Plugins, priority support |
| **Enterprise** | `enterprise` | ✅ | Unlimited | All features, custom branding |

---

## LicenseService (`internal/services/license.go`)

```go
type LicenseService struct {
    httpClient *http.Client
}

func (s *LicenseService) VerifyLicense(licenseKey string) (*LicenseVerifyResponse, error)
func (s *LicenseService) BindLicense(licenseKey, serverID, serverIP string) (*LicenseBindResponse, error)
func (s *LicenseService) ExtractPermissions(verifyResp *LicenseVerifyResponse) *LicensePermissions
func (s *LicenseService) GetLicenseType(verifyResp *LicenseVerifyResponse) string
```

**Note:** No `DB` field in struct - stateless service using external API.

---

## VerifyLicense()

```go
func (s *LicenseService) VerifyLicense(licenseKey string) (*LicenseVerifyResponse, error)
```

**Request:** `GET /api/public/licenses/verify?licenseKey={key}`

**Response:**
```json
{
  "valid": true,
  "status": "valid",
  "isInGracePeriod": false,
  "license": {
    "key": "LICENSE_KEY",
    "plan": "professional",
    "maxServers": 10,
    "usedServers": 2,
    "expiryDate": "2025-12-31T23:59:59Z",
    "daysRemaining": 365,
    "billingCycle": "monthly",
    "boundServerId": "abc123",
    "boundServerIp": "192.168.1.100"
  },
  "user": {...},
  "payment": {...},
  "validation": {...}
}
```

**Structs:**
```go
type LicenseVerifyResponse struct {
    Valid           bool   `json:"valid"`
    Status          string `json:"status"`
    IsInGracePeriod bool   `json:"isInGracePeriod"`
    License         struct {
        Key            string `json:"key"`
        Plan           string `json:"plan"`
        MaxServers     int    `json:"maxServers"`
        UsedServers    int    `json:"usedServers"`
        ExpiryDate     string `json:"expiryDate"`
        DaysRemaining  int    `json:"daysRemaining"`
        BillingCycle   string `json:"billingCycle"`
        BoundServerID  string `json:"boundServerId"`
        BoundServerIP  string `json:"boundServerIp"`
    } `json:"license"`
    User struct {
        Email string `json:"email"`
        Name  string `json:"name"`
    } `json:"user"`
    Payment struct {
        HasCompletedPayment bool   `json:"hasCompletedPayment"`
        LastPaymentDate     string `json:"lastPaymentDate"`
    } `json:"payment"`
    Validation struct {
        Timestamp string `json:"timestamp"`
        IP        string `json:"ip"`
    } `json:"validation"`
}
```

**Error Handling:**
- Invalid key → `valid: false`, `status: "invalid"`
- Expired → `valid: false`, `status: "expired"`, check `license.expiryDate`
- Network error → returns error, falls back to cached status

---

## BindLicense()

```go
func (s *LicenseService) BindLicense(licenseKey, serverID, serverIP string) (*LicenseBindResponse, error)
```

**Request:** `POST /api/public/licenses/verify` with body:
```json
{
  "licenseKey": "LICENSE_KEY",
  "serverId": "abc123",
  "serverIp": "192.168.1.100"
}
```

**Binds** the license to a specific server instance (prevents key sharing).

**Response:**
```json
{
  "success": true,
  "message": "License bound to server abc123"
}
```

**Use Cases:**
- First panel startup (auto-bind)
- Server migration (re-bind to new IP)
- License transfer between servers

---

## ExtractPermissions()

```go
func (s *LicenseService) ExtractPermissions(verifyResp *LicenseVerifyResponse) *LicensePermissions
```

```go
type LicensePermissions struct {
    HasPlugins bool `json:"has_plugins"`
}
```

**Logic:**
```go
func (s *LicenseService) ExtractPermissions(verifyResp *LicenseVerifyResponse) *LicensePermissions {
    hasPlugins := verifyResp.License.Plan == "professional" || verifyResp.License.Plan == "enterprise"

    return &LicensePermissions{
        HasPlugins: hasPlugins,
    }
}
```

**Used by:** Frontend to show/hide Plugins tab, Provision API, plugin installation endpoints.

---

## GetLicenseType()

```go
func (s *LicenseService) GetLicenseType(verifyResp *LicenseVerifyResponse) string
```

**Mapping:**
| API Plan | Returns |
|----------|---------|
| `personal` | `free` |
| `professional` | `pro` |
| `enterprise` | `enterprise` |
| (unknown) | `pro` |

**Note:** Default returns `"pro"` (not `"free"`)

**Used for:** Display in Settings → License panel.

---

## Frontend Integration

### Settings → License Panel

Shows:
- Current license status (Free/Pro/Enterprise)
- License key (masked)
- Expiry date
- Max servers limit
- Feature toggles (Plugins, etc.)

**Actions:**
- **Activate License**: `POST /api/settings/license/activate`
- **Refresh**: Re-verify with API

### Plugins Tab Visibility

```tsx
// In server view
const { HasPlugins } = useLicensePermissions();

if (!HasPlugins) {
    return <div>Upgrade to Pro to access plugins</div>;
}
```

### Provision API Check

```go
// In provision handler
if !licenseService.ExtractPermissions(verification).HasPlugins {
    return errors.New("plugins not available in current plan")
}
```

---

## API Endpoints

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| POST | `/api/settings/license/activate` | `settings.edit` | Activate/verify license |
| GET | `/api/settings/license/status` | `settings.edit` | Get current license status |

### Activate License

**Request:**
```json
{
  "licenseKey": "YOUR_KEY"
}
```

**Response:**
```json
{
  "status": "valid",
  "plan": "pro",
  "expiryDate": "2025-12-31T23:59:59Z",
  "maxServers": 10,
  "features": {
    "hasPlugins": true
  }
}
```

---

## Caching & Fallback

**Cache Strategy:**
- Verification cached for 1 hour (configurable)
- On API failure: returns cached verification if < 24h old
- After 24h without successful verification: falls back to `free` tier

**Config:**
```json
{
  "panel": {
    "license": {
      "cacheTTL": 3600
    }
  }
}
```

---

## Error Scenarios

| Scenario | Behavior |
|----------|----------|
| Invalid key | `valid: false`, `status: "invalid"`, shows error in UI |
| Expired license | `valid: false`, `status: "expired"`, shows expiry date, falls back to free |
| API unreachable | Uses cache if < 24h, else `free` |
| Max servers exceeded | Blocks server creation, shows limit error |
| Key bound to different server | Bind fails, shows conflict error |

---

## Security

- License key never logged (masked in logs)
- HTTPS only for API communication
- Server IP verified on bind (prevents key sharing)
- Keys stored in config (not DB)

---

## API Examples

```bash
# Activate license
curl -X POST http://panel/api/settings/license/activate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"licenseKey": "YOUR_KEY"}'

# Check status
curl -H "Authorization: Bearer $TOKEN" \
  http://panel/api/settings/license/status
```

```python
import requests

# Activate
r = requests.post(
    "http://panel/api/settings/license/activate",
    headers={"Authorization": f"Bearer {token}"},
    json={"license_key": "YOUR_KEY"}
)
print(r.json())

# Check
r = requests.get(
    "http://panel/api/settings/license/status",
    headers={"Authorization": f"Bearer {token}"}
)
print(r.json())
```