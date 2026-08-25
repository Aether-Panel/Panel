# Cloudflare Turnstile

Aether Panel integrates Cloudflare Turnstile for CAPTCHA-free bot protection on login and registration forms.

---

## Overview

- **Invisible CAPTCHA**: No user interaction required (behavioral analysis)
- **Privacy-friendly**: No personal data collected
- **Fallback**: Falls back to checkbox if needed

---

## Configuration

```json
{
  "panel": {
    "turnstile": {
      "enabled": true,
      "siteKey": "0x4AAAAA...",
      "secretKey": "0x4AAAAA..."
    }
  }
}
```

**Environment Variables:**
| Variable | Config Key | Required |
|----------|------------|----------|
| `SKYPANEL_PANEL_TURNSTILE_ENABLED` | `panel.turnstile.enabled` | Yes |
| `SKYPANEL_PANEL_TURNSTILE_SITEKEY` | `panel.turnstile.siteKey` | Yes |
| `SKYPANEL_PANEL_TURNSTILE_SECRETKEY` | `panel.turnstile.secretKey` | Yes |

**Aliases (Panel Settings):**
| Config Key | Alias For |
|------------|-----------|
| `panel.settings.turnstileEnabled` | `panel.turnstile.enabled` |
| `panel.settings.turnstileSiteKey` | `panel.turnstile.siteKey` |
| `panel.settings.turnstileSecretKey` | `panel.turnstile.secretKey` |

---

## Backend Verification (`internal/web/auth/turnstile.go`)

```go
const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type turnstileVerifyResponse struct {
	Success bool `json:"success"`
}

// verifyTurnstile validates a Cloudflare Turnstile token against Cloudflare's
// siteverify endpoint. If Turnstile is disabled or no secret key is configured,
// validation is skipped so the panel keeps working without it.
func verifyTurnstile(token string) error {
	if !config.TurnstileEnabled.Value() {
		return nil
	}

	secret := config.TurnstileSecretKey.Value()
	if secret == "" {
		return nil
	}

	params := url.Values{}
	params.Set("secret", secret)
	params.Set("response", token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.PostForm(turnstileVerifyURL, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result turnstileVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Success {
		return errors.New("turnstile verification failed")
	}
	return nil
}
```

**Called from:**
- `LoginPost` (after password validation)
- `RegisterPost` (before user creation)

**Flow:**
1. User submits login/register form
2. Frontend includes Turnstile token in request
3. Backend verifies token with Cloudflare (sends `secret` and `response`)
4. On success: continue with login/registration
5. On failure: return error, prompt retry

**Note:** The verification does NOT send `remoteip` - only `secret` and `response` are sent to Cloudflare.

---

## Frontend Integration (`client/frontend/src/components/Turnstile.tsx`)

```tsx
import { Turnstile } from '@/components/Turnstile';

<form onSubmit={handleSubmit}>
  <Turnstile 
    siteKey={config.turnstileSiteKey}
    onVerify={setTurnstileToken}
    theme="auto"
    size="normal"
  />
  <button type="submit">Login</button>
</form>
```

**Component Props:**
| Prop | Type | Required |
|------|------|----------|
| `siteKey` | string | Yes |
| `onVerify` | `(token: string) => void` | Yes |
| `theme` | `'light' \| 'dark' \| 'auto'` | No |
| `size` | `'normal' \| 'compact'` | No |

**Script Loading:**
- Loads `https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit` dynamically
- Renders invisible widget (no checkbox)
- Calls `onVerify(token)` when challenge completes

---

## Getting Turnstile Keys

1. Go to [Cloudflare Turnstile Dashboard](https://dash.cloudflare.com/turnstile)
2. Create a new site
3. Add your domain(s)
4. Copy **Site Key** and **Secret Key**
5. Add to Panel config

**Widget Types:**
| Type | Description |
|------|-------------|
| Managed | Invisible, automatic |
| Non-interactive | Background only |
| Invisible | Challenge on demand |

---

## Testing

### Development (Disabled)
```json
{
  "panel": {
    "turnstile": {
      "enabled": false
    }
  }
}
```

### Verification
```bash
# Check config
curl -H "Authorization: Bearer $TOKEN" http://panel/api/config

# Response includes:
# { "turnstile": { "enabled": true, "siteKey": "0x4AAAAA..." } }
```

---

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| `invalid site key` | Wrong site key | Verify domain matches Cloudflare dashboard |
| `invalid secret key` | Wrong secret key | Copy secret key from Cloudflare dashboard |
| `timeout` | Network issue | Check firewall, allow outbound to `challenges.cloudflare.com` |
| `token expired` | Token > 30 min old | Frontend auto-refreshes; check clock sync |
| `hostname mismatch` | Domain mismatch | Add panel domain to Turnstile site config |

---

## Security Notes

- **Secret key**: Never exposed to frontend (only backend verification)
- **Site key**: Public, safe to expose in HTML
- **Tokens**: Single-use, 30-minute expiry
- **IP validation**: Not implemented (only `secret` and `response` sent)
- **Rate limiting**: Cloudflare handles rate limiting

---

## API Response Codes

| Code | Meaning |
|------|---------|
| `success: true` | Valid token |
| `success: false` | Invalid/expired token |
| `error-codes: ["timeout-or-duplicate"]` | Token already used or timeout |
| `error-codes: ["invalid-input-secret"]` | Wrong secret key |
| `error-codes: ["invalid-input-response"]` | Invalid/missing token |