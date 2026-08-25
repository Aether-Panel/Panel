# Email & SMTP System

Aether Panel uses a template-based email system with support for multiple providers (SMTP, SendGrid, Mailjet, Mailgun, Debug).

---

## Overview

- **Template-based**: Templates defined in `emails.json`, rendered with Go templates
- **Multi-provider**: SMTP, SendGrid, Mailjet, Mailgun, Debug (log only)
- **Async sending**: Non-blocking goroutine
- **Merged FS**: Custom templates override embedded defaults

---

## Configuration

```json
{
  "panel": {
    "email": {
      "templateFolder": "/etc/SkyPanel/emails",
      "provider": "smtp",
      "from": "panel@example.com",
      "domain": "example.com",
      "host": "smtp.example.com",
      "key": "api_key_or_password",
      "username": "smtp_user",
      "password": "smtp_password"
    }
  }
}
```

**Environment Variables:**
| Variable | Config Key | Required |
|----------|------------|----------|
| `SKYPANEL_PANEL_EMAIL_TEMPLATEFOLDER` | `panel.email.templateFolder` | No |
| `SKYPANEL_PANEL_EMAIL_PROVIDER` | `panel.email.provider` | Yes |
| `SKYPANEL_PANEL_EMAIL_FROM` | `panel.email.from` | Yes |
| `SKYPANEL_PANEL_EMAIL_DOMAIN` | `panel.email.domain` | Mailgun only |
| `SKYPANEL_PANEL_EMAIL_HOST` | `panel.email.host` | SMTP only |
| `SKYPANEL_PANEL_EMAIL_KEY` | `panel.email.key` | Yes |
| `SKYPANEL_PANEL_EMAIL_USERNAME` | `panel.email.username` | SMTP only |
| `SKYPANEL_PANEL_EMAIL_PASSWORD` | `panel.email.password` | SMTP only |

---

## EmailService (`internal/services/email.go`)

```go
type EmailService struct {
    DB       *gorm.DB
    provider EmailProvider
}

func (s *EmailService) SendEmail(to, templateName string, data map[string]interface{}, async bool) error
```

**Parameters:**
- `to`: Recipient email
- `templateName`: Key in `emails.json` (e.g., `passwordReset`)
- `data`: Template variables
- `async`: `true` = goroutine (non-blocking), `false` = blocking

**Returns:** Error only on sync send or provider init failure

---

## Template System

### emails.json Structure

```json
{
  "passwordReset": {
    "subject": "Reset your password",
    "body": "emails/password_reset.tmpl"
  },
  "addedToServer": {
    "subject": "You've been added to a server",
    "body": "emails/added_to_server.tmpl"
  },
  "otpEnroll": {
    "subject": "Your OTP enrollment code",
    "body": "emails/otp_enroll.tmpl"
  }
}
```

### Template Files (Go Templates)

**password_reset.tmpl:**
```html
<h1>Password Reset</h1>
<p>Hello {{.Username}},</p>
<p>Click <a href="{{.ResetLink}}">here</a> to reset your password.</p>
<p>This link expires in 30 minutes.</p>
<p>If you didn't request this, please ignore.</p>
```

**Available Variables (per template):**
| Template | Variables |
|----------|-----------|
| `passwordReset` | `Username`, `ResetLink`, `ExpiryMinutes` |
| `addedToServer` | `Username`, `ServerName`, `ServerURL`, `Scopes` |
| `otpEnroll` | `Username`, `Secret`, `QRCodeURL`, `RecoveryCodes` |
| `licenseExpiring` | `DaysUntilExpiry`, `LicenseKey`, `Plan` |
| `serverOffline` | `ServerName`, `ServerID`, `Timestamp` |
| `backupFailed` | `ServerName`, `BackupName`, `Error` |

### Template Resolution

```go
// 1. Check custom folder (panel.email.templateFolder)
// 2. Fall back to embedded FS (assets/email/)
// 3. Parse with Go text/template
```

**Custom Override:** Place templates in `templateFolder` to override defaults.

---

## Providers (`internal/email/`)

### Provider Interface

```go
type EmailProvider interface {
    Send(to, subject, body string) error
    Name() string
}
```

### 1. SMTP (`internal/email/smtp.go`)

Uses `wneessen/go-mail`.

```go
type SMTPProvider struct {
    host     string
    port     int
    username string
    password string
    from     string
    tls      bool
}
```

**Config:**
```json
{
  "provider": "smtp",
  "host": "smtp.gmail.com",
  "port": 587,
  "username": "user@gmail.com",
  "password": "app_password",
  "from": "panel@example.com"
}
```

**Features:**
- STARTTLS (port 587) or SSL (port 465)
- Authentication: PLAIN/LOGIN
- Connection pooling

---

### 2. SendGrid (`internal/email/sendgrid.go`)

Uses `sendgrid-go`.

```go
type SendGridProvider struct {
    apiKey string
    from   string
}
```

**Config:**
```json
{
  "provider": "sendgrid",
  "key": "SG.xxxxx",
  "from": "panel@example.com"
}
```

---

### 3. Mailjet (`internal/email/mailjet.go`)

Uses `mailjet-go`.

```go
type MailjetProvider struct {
    apiKey    string
    apiSecret string
    from      string
}
```

**Config:**
```json
{
  "provider": "mailjet",
  "key": "api_key",
  "password": "api_secret",
  "from": "panel@example.com"
}
```

---

### 4. Mailgun (`internal/email/mailgun.go`)

Uses `mailgun-go`.

```go
type MailgunProvider struct {
    apiKey string
    domain string
    from   string
}
```

**Config:**
```json
{
  "provider": "mailgun",
  "key": "api_key",
  "domain": "mg.example.com",
  "from": "panel@example.com"
}
```

---

### 5. Debug (`internal/email/debug.go`)

Logs email to stdout (development only).

```go
type DebugProvider struct{}
```

**Output:**
```
=== EMAIL DEBUG ===
To: user@example.com
Subject: Password Reset
Body: <html>...</html>
===================
```

---

## Provider Registration

```go
// internal/email/provider.go
func init() {
    RegisterProvider("smtp", NewSMTPProvider)
    RegisterProvider("sendgrid", NewSendGridProvider)
    RegisterProvider("mailjet", NewMailjetProvider)
    RegisterProvider("mailgun", NewMailgunProvider)
    RegisterProvider("debug", NewDebugProvider)
}

func GetProvider(name string) (EmailProvider, error)
```

**Custom Provider:** Add to `init()` and implement `EmailProvider` interface.

---

## Async Sending

```go
func (s *EmailService) SendEmail(to, templateName string, data map[string]interface{}, async bool) error {
    if async {
        go func() {
            if err := s.sendEmailSync(to, templateName, data); err != nil {
                logging.Errorf("Async email failed: %v", err)
            }
        }()
        return nil
    }
    return s.sendEmailSync(to, templateName, data)
}
```

**Use Cases:**
| async=true | async=false |
|------------|-------------|
| Password reset | Critical alerts |
| Server notifications | License expiry warnings |
| User invites | Provision confirmations |

---

## Built-in Templates

| Template | Trigger | Key Variables |
|----------|---------|---------------|
| `passwordReset` | Forgot password | `Username`, `ResetLink`, `ExpiryMinutes` |
| `otpEnroll` | OTP setup | `Username`, `Secret`, `QRCodeURL`, `RecoveryCodes` |
| `otpRecovery` | Recovery code used | `Username`, `RemainingCodes` |
| `addedToServer` | User added to server | `Username`, `ServerName`, `ServerURL`, `Scopes` |
| `removedFromServer` | User removed | `Username`, `ServerName` |
| `inviteToServer` | Server invite | `InviterName`, `ServerName`, `InviteLink` |
| `licenseExpiring` | License near expiry | `DaysUntilExpiry`, `LicenseKey`, `Plan` |
| `serverOffline` | Server goes offline | `ServerName`, `ServerID`, `Timestamp` |
| `serverOnline` | Server comes online | `ServerName`, `ServerID` |
| `backupFailed` | Backup fails | `ServerName`, `BackupName`, `Error` |
| `backupSuccess` | Backup succeeds | `ServerName`, `BackupName`, `Size` |
| `resourceAlert` | CPU/RAM threshold | `ServerName`, `Resource`, `Current`, `Threshold` |
| `nodeOffline` | Node disconnects | `NodeName`, `NodeID` |
| `exTransferCreated` | Transfer created | `Token`, `ServerName`, `Expiry` |

---

## Discord Fallback

If email fails and Discord webhook configured, sends to Discord instead:

```go
func (s *EmailService) SendEmail(...) error {
    err := s.provider.Send(...)
    if err != nil && s.hasDiscordWebhook() {
        s.discord.SendAlert("Email Failed", fmt.Sprintf("To: %s, Template: %s, Error: %v", to, templateName, err))
    }
    return err
}
```

---

## Testing

### Development (Debug Provider)

```json
{
  "panel": {
    "email": {
      "provider": "debug",
      "from": "test@example.com"
    }
  }
}
```

**Output:**
```
=== EMAIL DEBUG ===
To: user@example.com
Subject: Password Reset
Body: <html><h1>Password Reset</h1><p>Hello user,...</p></html>
===================
```

### Production Checklist

- [ ] Valid SMTP credentials / API keys
- [ ] `from` address verified with provider
- [ ] SPF/DKIM/DMARC configured for domain
- [ ] Test email sent on startup (`/api/settings/test/email`)
- [ ] Custom templates in `templateFolder` if needed
- [ ] Async=true for non-critical emails
- [ ] Rate limits respected (provider dependent)

---

## API Endpoints

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| POST | `/api/settings/test/email` | `settings.edit` | Send test email |

**Request:**
```json
{
  "to": "test@example.com"
}
```

**Response:** `204` on success, error details on failure

---

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| `connection refused` | Wrong host/port | Verify host:port, firewall |
| `authentication failed` | Wrong credentials | Check username/password/API key |
| `certificate verify failed` | TLS cert issue | Use `InsecureSkipVerify` (dev only) |
| `template not found` | Missing template | Check `emails.json` + file exists |
| `rate limit exceeded` | Provider limit | Reduce frequency, check limits |

---

## Custom Provider Example

```go
// internal/email/custom.go
package email

type CustomProvider struct {
    apiEndpoint string
    apiKey      string
}

func NewCustomProvider(config map[string]string) (EmailProvider, error) {
    return &CustomProvider{
        apiEndpoint: config["endpoint"],
        apiKey:      config["key"],
    }, nil
}

func (c *CustomProvider) Send(to, subject, body string) error {
    // Custom HTTP request
    return nil
}

func (c *CustomProvider) Name() string { return "custom" }

// Register in provider.go init():
// RegisterProvider("custom", NewCustomProvider)
```