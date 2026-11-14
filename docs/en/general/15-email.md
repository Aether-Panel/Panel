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

### How they look

Every email is an **airmail letter**: light aged paper with red, white and blue stripes at the top and bottom, the company name (`panel.settings.companyName`) with the date, a blue stamp with the email's **reference**, the title, typewriter text, a box with the details (account, server, dates), a button when there is something to do (with the link written underneath in case the button does not work), the signature "the <company> team" and a footer linking to the panel.

- It is built with tables and inline styles and only uses fonts every device has (Courier New and Georgia), so it looks the same in Gmail, Outlook and Apple Mail. On a phone it takes the full width.
- **Language**: the one in **Settings → General → Notification language** (`panel.settings.notificationLanguage`, `es` or `en`, Spanish by default), the same as the Discord, Telegram and webhook alerts. It is read on every send.
- **Reference**: each email gets its own 8-character reference (`Q7M2-K9XD`, without characters that read alike such as 0/O or 1/I/L). The panel log records it when sending: `Sending email passwordReset (ref Q7M2-K9XD) to laura@example.com using smtp`. When someone quotes the reference, the email can be found in the log.
- The subject is always written on one line, even when a server's name has line breaks.

### Files

In `assets/email/`:

| File | What it is |
|------|------------|
| `layout.html` | The letter, shared by every email (`html/template`) |
| `messages.es.json`, `messages.en.json` | The texts of each email in that language |

Each email in `messages.<language>.json` has `subject`, `preheader` (the line the inbox shows next to the subject), `title`, `paragraphs`, `details` (`label`/`value` pairs) and, when it needs one, `action` (the button's `label` and `url`). `common` holds the texts every email uses (greeting, signature, footer…). Every text is a Go `text/template` over the variables below, for example `"You now have access to {{ .Server.Name }}"`; the letter escapes what comes out. A paragraph, detail or button that renders empty is left out: a text can depend on a condition (`{{ if .RegisterToken }}…{{ end }}`), and a button that needs `BASE_URL` disappears when the panel has no URL.

### Custom templates

`LoadEmailService()` reads the templates once when the panel starts (`cmd/panel/run.go`); a change needs a restart. A missing file or a syntax error **stops the panel at start-up** (panic).

- **A folder with `layout.html` or `messages.<language>.json`** (`panel.email.templateFolder`): those files replace the built-in ones and the rest is still read from `assets/email/`. For example, a custom `messages.es.json` changes the Spanish texts and keeps the design and the English ones. A custom `messages.<language>.json` must have every email: one it lacks is not sent, and the log says so.
- **A folder with `emails.json`** (the previous format): the panel uses those templates as they are, as before, without the letter or the language. `emails.json` maps each email to a subject and a whole HTML file:

  ```json
  {
    "passwordReset": {
      "subject": "Reset your password",
      "body": "password-reset.html"
    }
  }
  ```

  It must declare every email the panel sends. Since the letter, `assets/email/` no longer ships the HTML files of the previous format: the body files must be in the custom folder.

### Variables

Every send adds these variables to the email's own:

| Variable | Source |
|----------|--------|
| `COMPANY_NAME` | `panel.settings.companyName` (default `SkyPanel`) |
| `MASTER_URL` | `panel.settings.masterUrl` as it is (default `http://localhost:8080`) |
| `BASE_URL` | `panel.settings.masterUrl` without the trailing slash; the button links use it |
| `HOST_NAME` | The host name of `panel.settings.masterUrl` (for example `panel.example.com`); when none can be taken from it, `panel.settings.companyName` |
| `TO` | The address the email goes to |
| `DATE`, `TIME` | Date ("8 de noviembre de 2026" / "8 November 2026") and time with zone ("17:30 CET") of the send |
| `REF` | The email's reference |
| `LANG` | `es` or `en` |
| `PROVIDER` | The email provider (`smtp`, `mailgun`…) |

### Emails

| Email | Subject (en) | Sent when | Extra variables |
|-------|--------------|-----------|-----------------|
| `accountCreation` | Your <company> account is ready | A user registers (`/auth/register`) or an administrator approves their account | — |
| `passwordReset` | Reset your password | A password reset is requested (the link expires in 30 minutes) | `RESET_LINK` (`<panel>/reset-password/?token=…`) |
| `passwordChanged` | Your password has changed | The password is changed (profile or reset) | — |
| `emailChanged` | Your account's email has changed | The email is changed; sent to the **old** address | `NEW_EMAIL` |
| `otpEnabled` | Two-step verification turned on | 2FA turned on | — |
| `otpDisabled` | Two-step verification turned off | 2FA turned off | — |
| `recoveryCodesRegenerated` | New recovery codes | Recovery codes regenerated | — |
| `oauthCreated` | New OAuth client on your account | The user creates an OAuth client | — |
| `oauthDeleted` | OAuth client deleted | The user deletes an OAuth client | — |
| `addedToServer` | You now have access to <name> | A user is added to a server, a server is created for them, or provisioned | `Server` (`.Server.Name`, `.Server.Identifier`), `RegisterToken`, `Email` |
| `removedFromServer` | You no longer have access to <name> | A user is removed from a server | `Server` |
| `deletedServer` | The server <name> was deleted | A server is deleted; sent to the users who had access to it | `Server` |
| `login` | New sign-in to your account | Not sent by any code path | — |
| `test` | <company> test email | Test button / test endpoint | — |

`addedToServer`: when the user did not exist and the panel created the account (`RegisterToken` not empty), the button goes to `<panel>/forgot-password/` so they choose a password with their address; otherwise it opens the server (`<panel>/servers/view/?id=<identifier>`).

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
| Panel does not start after editing templates | A file is missing (a body listed in `emails.json`) or a template has a syntax error |
| Nothing arrives, no error in the UI | Asynchronous emails only log failures; check the panel log |

For production, verify the `from` address or domain with the provider and set up SPF/DKIM/DMARC for it.
