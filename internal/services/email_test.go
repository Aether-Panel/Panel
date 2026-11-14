package services

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	emailAssets "github.com/SkyPanel/SkyPanel/v3/assets/email"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailHostName(t *testing.T) {
	assert.Equal(t, "panel.example.com", emailHostName("https://panel.example.com:8443/", "Acme"))
	assert.Equal(t, "panel.example.com", emailHostName("http://panel.example.com", "Acme"))
	assert.Equal(t, "Acme", emailHostName("", "Acme"), "no Base URL: the company name")
}

// sentEmails are the emails the panel sends (SendEmail's template argument in internal/web).
var sentEmails = []string{
	"accountCreation", "passwordReset", "passwordChanged", "emailChanged", "otpEnabled",
	"recoveryCodesRegenerated", "otpDisabled", "oauthCreated", "oauthDeleted", "addedToServer",
	"deletedServer", "removedFromServer", "test", "login",
}

func withPanel(t *testing.T, masterURL, company string) {
	t.Helper()
	oldURL, oldCompany := config.MasterURL.Value(), config.CompanyName.Value()
	require.NoError(t, config.MasterURL.Set(masterURL, false))
	require.NoError(t, config.CompanyName.Set(company, false))
	t.Cleanup(func() {
		_ = config.MasterURL.Set(oldURL, false)
		_ = config.CompanyName.Set(oldCompany, false)
	})
}

func letterBookForTest(t *testing.T) *letterBook {
	t.Helper()
	book, err := loadLetterBook(emailAssets.FS)
	require.NoError(t, err)
	return book
}

// sampleEmailData is what each email's caller passes, plus what SendEmail adds.
func sampleEmailData(name, lang string) map[string]interface{} {
	server := &models.Server{Identifier: "a1b2c3d4", Name: "Survival"}
	data := map[string]interface{}{}
	switch name {
	case "passwordReset":
		data["RESET_LINK"] = "https://panel.example.com/reset-password/?token=abc"
	case "emailChanged":
		data["NEW_EMAIL"] = "laura.nueva@example.com"
	case "addedToServer":
		data["Server"] = server
		data["RegisterToken"] = ""
	case "deletedServer", "removedFromServer":
		data["Server"] = server
	}
	fillEmailData(data, "laura@example.com", "smtp", lang, "Q7M2-K9XD", time.Date(2026, 11, 8, 17, 30, 0, 0, time.UTC))
	return data
}

// Every email the panel sends exists in every language, and the two languages have the
// same emails.
func TestEmailLetters_EveryEmailInEveryLanguage(t *testing.T) {
	book := letterBookForTest(t)
	for _, lang := range letterLanguages {
		var names []string
		for name := range book.catalogs[lang].emails {
			names = append(names, name)
		}
		want := append([]string(nil), sentEmails...)
		sort.Strings(names)
		sort.Strings(want)
		assert.Equal(t, want, names, "messages.%s.json", lang)
	}
}

// Each email renders a whole letter: its subject, title, the stamp with the reference, no
// field left unfilled and no template code left over.
func TestEmailLetters_Render(t *testing.T) {
	withPanel(t, "https://panel.example.com/", "Acme Hosting")
	book := letterBookForTest(t)
	for _, lang := range letterLanguages {
		for _, name := range sentEmails {
			t.Run(lang+"/"+name, func(t *testing.T) {
				subject, body, err := book.render(lang, name, sampleEmailData(name, lang))
				require.NoError(t, err)
				assert.NotEmpty(t, subject)
				assert.NotContains(t, subject, "\n")
				for _, leftover := range []string{"<no value>", "{{", "}}", "%!"} {
					assert.NotContains(t, subject, leftover)
					assert.NotContains(t, body, leftover)
				}
				assert.Contains(t, body, "Q7M2-K9XD", "the reference on the stamp")
				assert.Contains(t, body, "Acme Hosting", "the company signs the letter")
				assert.Contains(t, body, `lang="`+lang+`"`)
				assert.Contains(t, body, `href="https://panel.example.com"`, "the footer opens the panel")
				// never wider than a phone, or the Gmail app strips every table width to fit it
				assert.Contains(t, body, `class="letter" width="100%"`)
				assert.Contains(t, body, "max-width:600px")
				assert.Contains(t, body, `<!--[if mso]><table role="presentation" width="600"`, "Outlook for Windows keeps 600px")
			})
		}
	}
}

func TestEmailLetters_SpanishTexts(t *testing.T) {
	withPanel(t, "https://panel.example.com", "Acme Hosting")
	book := letterBookForTest(t)

	subject, body, err := book.render("es", "passwordReset", sampleEmailData("passwordReset", "es"))
	require.NoError(t, err)
	assert.Equal(t, "Restablecer tu contraseña", subject)
	assert.Contains(t, body, "8 de noviembre de 2026")
	assert.Contains(t, body, `href="https://panel.example.com/reset-password/?token=abc"`)
	assert.Contains(t, body, "Restablecer contraseña")
	assert.Contains(t, body, "el equipo de Acme Hosting")

	subject, _, err = book.render("en", "passwordReset", sampleEmailData("passwordReset", "en"))
	require.NoError(t, err)
	assert.Equal(t, "Reset your password", subject)
}

// A new account gets the page to choose its password; an existing one opens the server.
func TestEmailLetters_AddedToServer(t *testing.T) {
	withPanel(t, "https://panel.example.com", "Acme")
	book := letterBookForTest(t)

	data := sampleEmailData("addedToServer", "es")
	subject, body, err := book.render("es", "addedToServer", data)
	require.NoError(t, err)
	assert.Equal(t, "Tienes acceso al servidor Survival", subject)
	assert.Contains(t, body, `href="https://panel.example.com/servers/view/?id=a1b2c3d4"`)
	assert.Contains(t, body, "Abrir el servidor")

	data = sampleEmailData("addedToServer", "es")
	data["RegisterToken"] = "tok"
	_, body, err = book.render("es", "addedToServer", data)
	require.NoError(t, err)
	assert.Contains(t, body, `href="https://panel.example.com/forgot-password/"`)
	assert.Contains(t, body, "Elegir mi contraseña")
	assert.Contains(t, body, "escribe laura@example.com")

	// the provisioning API passes no RegisterToken at all
	data = sampleEmailData("addedToServer", "en")
	delete(data, "RegisterToken")
	_, body, err = book.render("en", "addedToServer", data)
	require.NoError(t, err)
	assert.Contains(t, body, "Open the server")
}

// A server's name is its owner's to choose: it is escaped in the body and cannot break the
// subject onto a second header line.
func TestEmailLetters_ServerNameIsText(t *testing.T) {
	withPanel(t, "https://panel.example.com", "Acme")
	book := letterBookForTest(t)
	data := sampleEmailData("deletedServer", "en")
	data["Server"] = &models.Server{Identifier: "a1b2c3d4", Name: "<b>x</b>\r\nBcc: someone@example.com"}

	subject, body, err := book.render("en", "deletedServer", data)
	require.NoError(t, err)
	assert.NotContains(t, subject, "\n")
	assert.NotContains(t, subject, "\r")
	assert.NotContains(t, body, "<b>x</b>")
	assert.Contains(t, body, "&lt;b&gt;x&lt;/b&gt;")
}

// Without a Base URL there is nowhere to link to: those buttons and the footer link are left
// out, and the password reset keeps its own link.
func TestEmailLetters_WithoutBaseURL(t *testing.T) {
	withPanel(t, "", "Acme")
	book := letterBookForTest(t)

	_, body, err := book.render("es", "otpDisabled", sampleEmailData("otpDisabled", "es"))
	require.NoError(t, err)
	assert.NotContains(t, body, "Ir a mi perfil")
	assert.NotContains(t, body, "Abrir el panel")
	assert.NotContains(t, body, `href=""`)

	_, body, err = book.render("es", "passwordReset", sampleEmailData("passwordReset", "es"))
	require.NoError(t, err)
	assert.Contains(t, body, "Restablecer contraseña")
}

func TestNewEmailRef(t *testing.T) {
	format := regexp.MustCompile(`^[` + emailRefAlphabet + `]{4}-[` + emailRefAlphabet + `]{4}$`)
	seen := map[string]bool{}
	for range 200 {
		ref := newEmailRef()
		assert.Regexp(t, format, ref)
		seen[ref] = true
	}
	assert.Greater(t, len(seen), 195, "references differ from one email to the next")
	assert.NotContainsf(t, emailRefAlphabet, "0", "0 reads like O")
	assert.NotContains(t, emailRefAlphabet, "O")
	assert.NotContains(t, emailRefAlphabet, "1")
	assert.NotContains(t, emailRefAlphabet, "I")
	assert.NotContains(t, emailRefAlphabet, "L")
}

func TestLetterDate(t *testing.T) {
	day := time.Date(2026, 3, 1, 9, 5, 0, 0, time.UTC)
	assert.Equal(t, "1 de marzo de 2026", letterDate("es", day))
	assert.Equal(t, "1 March 2026", letterDate("en", day))
	assert.Equal(t, "09:05 UTC", letterTime(day))
}

// A template folder with its own emails.json keeps its emails as they were written.
func TestLoadEmailService_LegacyTemplateFolder(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "emails.json"), []byte(`{"test": {"subject": "Hi {{ .COMPANY_NAME }}", "body": "test.html"}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.html"), []byte(`<p>Custom {{ .COMPANY_NAME }}</p>`), 0o600))
	old := config.EmailTemplateFolder.Value()
	require.NoError(t, config.EmailTemplateFolder.Set(dir, false))
	t.Cleanup(func() {
		_ = config.EmailTemplateFolder.Set(old, false)
		LoadEmailService()
	})
	withPanel(t, "https://panel.example.com", "Acme")

	LoadEmailService()
	subject, body, err := globalEmailService.render("es", "test", sampleEmailData("test", "es"))
	require.NoError(t, err)
	assert.Equal(t, "Hi Acme", subject)
	assert.Equal(t, "<p>Custom Acme</p>", body)
}

// A template folder without emails.json can replace one language's texts and keep the rest.
func TestLoadEmailService_TemplateFolderOverridesTexts(t *testing.T) {
	dir := t.TempDir()
	raw, err := emailAssets.FS.ReadFile("messages.en.json")
	require.NoError(t, err)
	custom := strings.Replace(string(raw), `"subject": "Reset your password"`, `"subject": "Choose a new password"`, 1)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "messages.en.json"), []byte(custom), 0o600))
	old := config.EmailTemplateFolder.Value()
	require.NoError(t, config.EmailTemplateFolder.Set(dir, false))
	t.Cleanup(func() {
		_ = config.EmailTemplateFolder.Set(old, false)
		LoadEmailService()
	})
	withPanel(t, "https://panel.example.com", "Acme")

	LoadEmailService()
	subject, _, err := globalEmailService.render("en", "passwordReset", sampleEmailData("passwordReset", "en"))
	require.NoError(t, err)
	assert.Equal(t, "Choose a new password", subject)
	subject, _, err = globalEmailService.render("es", "passwordReset", sampleEmailData("passwordReset", "es"))
	require.NoError(t, err)
	assert.Equal(t, "Restablecer tu contraseña", subject)
}
