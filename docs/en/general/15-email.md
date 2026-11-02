# Email

Aether Panel sends transactional emails (account created, password reset, added to a server, …) from HTML templates through one of five providers: SMTP, Mailgun, Mailjet, SendGrid or a debug provider that only logs.

Email is optional. With no provider set, every send returns `email not configured` and the action that triggered it carries on (the error is only logged).

---

## Configuration

Keys from `internal/config/entries.go` (all default to `""`). Environment variables follow the usual rule (`SKYPANEL_` + key in upper case, dots as `_`), see [Configuration](../backend/02-configuration.md).

| Key | Environment variable | Used by |
|-----|----------------------|---------|
| `panel.email.provider` | `SKYPANEL_PANEL_EMAIL_PROVIDER` | All. `smtp`, `mailgun`, `mailjet`, `sendgrid` or `debug` (case-insensitive) |
| `panel.email.from` | `SKYPANEL_PANEL_EMAIL_FROM` | SMTP, Mailgun, Mailjet, SendGrid (required) |
| `panel.email.host` | `SKYPANEL_PANEL_EMAIL_HOST` | SMTP (required) |
| `panel.email.username` | `SKYPANEL_PANEL_EMAIL_USERNAME` | SMTP |
| `panel.email.password` | `SKYPANEL_PANEL_EMAIL_PASSWORD` | SMTP |
| `panel.email.domain` | `SKYPANEL_PANEL_EMAIL_DOMAIN` | Mailgun (sending domain), Mailjet (see below) |
| `panel.email.key` | `SKYPANEL_PANEL_EMAIL_KEY` | Mailgun, Mailjet, SendGrid (required) |
| `panel.email.templateFolder` | `SKYPANEL_PANEL_EMAIL_TEMPLATEFOLDER` | Folder with custom templates (optional, read at start-up) |

Every key except `templateFolder` can be changed from **Settings → Mail** or the settings API (`settings.edit`). The Mail tab offers all five providers: SMTP, Mailgun, Mailjet, SendGrid and **Debug (log only)**, which shows no fields.

A missing required value makes the send fail with `<key> is not configured` (error code `ErrSettingNotConfigured`).

---

## Providers (`internal/email/`)

Each provider registers itself by name in an `init()` function; `email.GetProvider(name)` looks the name up in lower case. All providers send the body as HTML.

### SMTP (`smtp.go`)

Library: `github.com/wneessen/go-mail`.

```json
{
  "panel": {
    "email": {
      "provider": "smtp",
      "host": "smtp.example.com",
      "username": "no-reply@example.com",
      "password": "app-password",
      "from": "no-reply@example.com"
    }
  }
}
```

- `host` is the host name only. There is no port setting: the client is created with `mail.WithSSLPort(true)`, which connects with implicit TLS on port **465** and lets go-mail fall back to its non-SSL port if that fails. Servers that only accept port 587 (STARTTLS) are not supported by a setting.
- Authentication: `SMTPAuthAutoDiscover` (the strongest method the server announces).
- `username` is trimmed; **all spaces are removed** from `password` (so Gmail-style app passwords can be pasted with spaces).
- Each message gets a random `Message-ID`.

### Mailgun (`mailgun.go`)

Library: `mailgun-go/v4`. Requires `domain` (the Mailgun sending domain), `from` and `key` (API key). Uses the library's default API base (US region).

### Mailjet (`mailjet.go`)

Library: `mailjet-apiv3-go/v4`, Send API v3.1. Requires `domain`, `from` and `key`. The client is built as `NewMailjetClient(domain, key)`, so **`panel.email.domain` holds the Mailjet API key (public) and `panel.email.key` the secret key**. In the Mail tab these fields are labelled **API Key (public)** and **Secret Key**.

### SendGrid (`sendgrid.go`)

Library: `sendgrid-go`. Requires `from` and `key` (API key). In the Mail tab: **Sender Address** and **API Key**.

### Debug (`debug.go`)

Writes `DEBUG EMAIL TO <to>`, the subject and the body to the debug log and returns success. For development only.

---

## Templates

### Loading

`LoadEmailService()` runs once when the panel starts (`cmd/panel/run.go`):

1. If `panel.email.templateFolder` is set, it builds a merged file system: the custom folder first, the embedded templates (`assets/email/`) as fallback.
2. Reads `emails.json`, which maps each template name to a subject and a body file:

   ```json
   {
     "passwordReset": {
       "subject": "Your password has been reset",
       "body": "password-reset.html"
     }
   }
   ```

3. Parses the subject and body with Go's `html/template`. A missing file or a syntax error **stops the panel at start-up** (panic).

Because the whole `emails.json` is taken from the first location that has it, a custom `emails.json` replaces the embedded list: it must declare every template the panel uses. Body files missing from the custom folder are still read from the embedded set. Template changes need a panel restart.

### Variables

Every send adds three variables:

| Variable | Source |
|----------|--------|
| `COMPANY_NAME` | `panel.settings.companyName` (default `SkyPanel`) |
| `MASTER_URL` | `panel.settings.masterUrl` (default `http://localhost:8080`) |
| `HOST_NAME` | The host name of `panel.settings.masterUrl` (for example `panel.example.com`); when none can be taken from it, `panel.settings.companyName`. Used by the `accountCreation` template |

### Built-in templates

The built-in subjects are in English; they can be changed with a custom template folder.

| Template | Subject | Sent when | Extra variables |
|----------|---------|-----------|-----------------|
| `accountCreation` | Your account has been created | A user registers (`/auth/register`) | — |
| `passwordReset` | Your password has been reset | A password reset is requested | `RESET_LINK` (`<panel>/reset-password/?token=…`) |
| `passwordChanged` | Your password has been changed | The password is changed (profile or reset) | — |
| `emailChanged` | Your email has been changed | The email is changed; sent to the **old** address | `NEW_EMAIL` |
| `otpEnabled` | OTP has been enabled for your account | 2FA turned on | — |
| `otpDisabled` | OTP has been disabled for your account | 2FA turned off | — |
| `recoveryCodesRegenerated` | OTP Recovery Codes have been regenerated for your account | Recovery codes regenerated | — |
| `oauthCreated` | OAuth client created | The user creates an OAuth client | — |
| `oauthDeleted` | OAuth client deleted | The user deletes an OAuth client | — |
| `addedToServer` | You have been added to a server | A user is added to a server, a server is created for them, or provisioned | `Server` (`.Server.Name`, `.Server.Identifier`), `RegisterToken`, `Email` |
| `removedFromServer` | You have been removed from a server | A user is removed from a server | `Server` |
| `deletedServer` | A server you have access to has been deleted | A server is deleted; sent to the users who had access to it | `Server` |
| `login` | Someone has logged into your account | Not sent by any code path | — |
| `test` | Email Test | Test button / test endpoint | — |

`addedToServer`: when the user did not exist and the panel created the account (`RegisterToken` not empty), the email asks them to choose a password at `<MASTER_URL>/forgot-password/` with their address; otherwise it shows the login link.

All emails are sent **asynchronously** (errors only go to the log), except the test email, which is sent synchronously so the error can be shown.

---

## Test email: `POST /api/settings/test/email`

Permission: `settings.edit`. The **Test Email** button in **Settings → Mail** calls it.

- The email (template `test`) goes to the **logged-in user's own address**; there is no `to` field.
- The body may contain unsaved values to test before saving. Recognised keys: `panel.email.provider`, `panel.email.from`, `panel.email.host`, `panel.email.username`, `panel.email.password`, `panel.email.domain`, `panel.email.key`. Non-empty values are applied for the test only and the previous values are restored afterwards. `panel.email.password` or `panel.email.key` set to `********` (what the page receives instead of the saved secret) are ignored and the saved value is used.

```bash
curl -X POST https://panel.example.com/api/settings/test/email \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"panel.email.provider": "smtp", "panel.email.host": "smtp.example.com"}'
```

| Status | Meaning |
|--------|---------|
| `204` | Sent |
| `422` | `the test email could not be sent: <reason>` (the provider's error, e.g. wrong password or TLS failure) |

---

## Troubleshooting

| Symptom | Cause |
|---------|-------|
| `email not configured` | `panel.email.provider` is empty |
| `panel.email.… is not configured` | A required key for the chosen provider is empty |
| `email does not support <name>` | `panel.email.provider` is not one of the five names |
| SMTP timeout or TLS error | The server does not offer implicit TLS on port 465 |
| Mailjet `401` | `domain`/`key` are not the Mailjet public/secret API key pair |
| Panel does not start after editing templates | A body file listed in `emails.json` is missing or a template has a syntax error |
| Nothing arrives, no error in the UI | Asynchronous emails only log failures; check the panel log |

For production, verify the `from` address or domain with the provider and set up SPF/DKIM/DMARC for it.
