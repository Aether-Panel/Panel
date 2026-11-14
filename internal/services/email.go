package services

import (
	"encoding/json"
	"fmt"
	emailAssets "github.com/SkyPanel/SkyPanel/v3/assets/email"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/email"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"html/template"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"time"
)

// EmailSender renders one of the panel's templates and mails it
type EmailSender interface {
	SendEmail(to string, template string, data map[string]interface{}, async bool) error
}

// emailTemplate is one email of a template folder that keeps the old format: an emails.json
// with a subject and a whole HTML file per email.
type emailTemplate struct {
	Subject *template.Template
	Body    *template.Template
}

type emailDeclaration struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

var globalEmailService *emailService

type emailService struct {
	// letters draws every email as the panel's airmail letter (email_letter.go)
	letters *letterBook
	// legacy holds the emails of a template folder with its own emails.json, which are used
	// as they are written; nil otherwise
	legacy map[string]*emailTemplate
}

// LoadEmailService reads the emails. A template folder (panel.email.templateFolder) can replace
// the layout or the texts of a language (layout.html, messages.<lang>.json); one with its own
// emails.json keeps its emails exactly as before.
func LoadEmailService() {
	globalEmailService = &emailService{}

	var merged fs.ReadFileFS = emailAssets.FS
	if folder := config.EmailTemplateFolder.Value(); folder != "" {
		custom := os.DirFS(folder)
		if _, err := fs.Stat(custom, "emails.json"); err == nil {
			globalEmailService.legacy = loadLegacyEmailTemplates(files.NewMergedFS(custom, emailAssets.FS))
			logging.Info.Printf("Emails use the templates of %s (emails.json)", folder)
			return
		}
		merged = files.NewMergedFS(custom, emailAssets.FS)
	}

	book, err := loadLetterBook(merged)
	if err != nil {
		panic(fmt.Errorf("error loading the email templates: %w", err))
	}
	globalEmailService.letters = book
	for k := range book.catalogs[letterLanguages[0]].emails {
		logging.Debug.Printf("Email template registered: %s", k)
	}
}

func loadLegacyEmailTemplates(merged fs.ReadFileFS) map[string]*emailTemplate {
	templates := make(map[string]*emailTemplate)
	emailDefinition, err := merged.Open("emails.json")
	if err != nil {
		panic(err)
	}
	defer utils.Close(emailDefinition)

	var mapping map[string]*emailDeclaration
	if err = json.NewDecoder(emailDefinition).Decode(&mapping); err != nil {
		panic(err.Error())
	}

	for templateName, data := range mapping {
		subjectTemplate, err := template.New(templateName + "-subject").Parse(data.Subject)
		if err != nil {
			panic(fmt.Errorf("error processing email template subject %s: %s", templateName, err.Error()))
		}
		body, err := merged.ReadFile(data.Body)
		if err != nil {
			panic(fmt.Errorf("error processing email template body %s: %s", templateName, err.Error()))
		}
		renderedTemplate, err := template.New(templateName + "-body").Parse(string(body))
		if err != nil {
			panic(fmt.Errorf("error processing email template body %s: %s", templateName, err.Error()))
		}
		templates[templateName] = &emailTemplate{Subject: subjectTemplate, Body: renderedTemplate}
	}
	return templates
}

func GetEmailService() EmailSender {
	return globalEmailService
}

func (es *emailService) SendEmail(to, template string, data map[string]interface{}, async bool) (err error) {
	provider := config.EmailProvider.Value()
	if provider == "" {
		return skypanel.ErrEmailNotConfigured
	}

	if data == nil {
		data = make(map[string]interface{})
	}
	lang := NotificationLanguage()
	ref := newEmailRef()
	fillEmailData(data, to, provider, lang, ref, time.Now())

	subject, body, err := es.render(lang, template, data)
	if err != nil {
		return err
	}

	svc := email.GetProvider(provider)
	if svc == nil {
		return skypanel.ErrServiceInvalidProvider("email", provider)
	}

	// the reference is on the email's stamp: someone who quotes it can be found here
	logging.Info.Printf("Sending email %s (ref %s) to %s using %s", template, ref, to, provider)

	if async {
		go func(emailService email.Provider, toEmail, subject, body string) {
			err := emailService.Send(toEmail, subject, body)
			if err != nil {
				logging.Error.Printf("Error sending email %s (ref %s): %s", template, ref, err)
			}
		}(svc, to, subject, body)
		return nil
	}

	return svc.Send(to, subject, body)
}

// fillEmailData adds what every email can use to the data its caller passed.
func fillEmailData(data map[string]interface{}, to, provider, lang, ref string, now time.Time) {
	data["COMPANY_NAME"] = config.CompanyName.Value()
	data["MASTER_URL"] = config.MasterURL.Value()
	data["BASE_URL"] = strings.TrimSuffix(config.MasterURL.Value(), "/")
	data["HOST_NAME"] = emailHostName(config.MasterURL.Value(), config.CompanyName.Value())
	data["TO"] = to
	data["PROVIDER"] = provider
	data["REF"] = ref
	data["LANG"] = lang
	data["DATE"] = letterDate(lang, now)
	data["TIME"] = letterTime(now)
}

// render writes an email's subject and body, as a letter or from a legacy template folder.
func (es *emailService) render(lang, name string, data map[string]interface{}) (subject, body string, err error) {
	if es.legacy != nil {
		tmpl := es.legacy[name]
		if tmpl == nil {
			return "", "", skypanel.ErrNoTemplate(name)
		}
		subjectBuilder := &strings.Builder{}
		if err = tmpl.Subject.Execute(subjectBuilder, data); err != nil {
			return "", "", err
		}
		bodyBuilder := &strings.Builder{}
		if err = tmpl.Body.Execute(bodyBuilder, data); err != nil {
			return "", "", err
		}
		return oneLine(subjectBuilder.String()), bodyBuilder.String(), nil
	}
	if es.letters == nil || !es.letters.has(lang, name) {
		return "", "", skypanel.ErrNoTemplate(name)
	}
	return es.letters.render(lang, name, data)
}

// emailHostName is the panel's host name (panel.example.com), which templates use as
// HOST_NAME; the company name when there is no Base URL to take it from
func emailHostName(masterURL, companyName string) string {
	if parsed, err := url.Parse(masterURL); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return companyName
}
