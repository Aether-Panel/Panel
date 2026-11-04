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
)

// EmailSender renders one of the panel's templates and mails it
type EmailSender interface {
	SendEmail(to string, template string, data map[string]interface{}, async bool) error
}

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
	templates map[string]*emailTemplate
}

func LoadEmailService() {
	globalEmailService = &emailService{templates: make(map[string]*emailTemplate)}

	var merged fs.ReadFileFS
	if config.EmailTemplateFolder.Value() != "" {
		merged = files.NewMergedFS(os.DirFS(config.EmailTemplateFolder.Value()), emailAssets.FS)
	} else {
		merged = emailAssets.FS
	}

	emailDefinition, err := merged.Open("emails.json")
	if err != nil {
		panic(err)
	}
	defer utils.Close(emailDefinition)

	var mapping map[string]*emailDeclaration
	err = json.NewDecoder(emailDefinition).Decode(&mapping)
	if err != nil {
		panic(err.Error())
	}

	for templateName, data := range mapping {
		subjectTemplate, err := template.New(templateName + "-subject").Parse(data.Subject)
		if err != nil {
			panic(fmt.Errorf("error processing email template subject %s: %s", templateName, err.Error()))
		}

		body, err := merged.ReadFile(data.Body)
		if err != nil {
			panic(fmt.Errorf("error processing email template subject %s: %s", templateName, err.Error()))
		}

		renderedTemplate, err := template.New(templateName + "-body").Parse(string(body))
		if err != nil {
			panic(fmt.Errorf("error processing email template body %s: %s", templateName, err.Error()))
		}

		globalEmailService.templates[templateName] = &emailTemplate{
			Subject: subjectTemplate,
			Body:    renderedTemplate,
		}
	}

	var logger = logging.Debug
	for k := range globalEmailService.templates {
		logger.Printf("Email template registered: %s", k)
	}
}

func GetEmailService() EmailSender {
	return globalEmailService
}

func (es *emailService) SendEmail(to, template string, data map[string]interface{}, async bool) (err error) {
	provider := config.EmailProvider.Value()
	if provider == "" {
		return skypanel.ErrEmailNotConfigured
	}

	tmpl := es.templates[template]

	if tmpl == nil {
		return skypanel.ErrNoTemplate(template)
	}

	if data == nil {
		data = make(map[string]interface{})
	}

	data["COMPANY_NAME"] = config.CompanyName.Value()
	data["MASTER_URL"] = config.MasterURL.Value()
	data["HOST_NAME"] = emailHostName(config.MasterURL.Value(), config.CompanyName.Value())

	subjectBuilder := &strings.Builder{}
	err = tmpl.Subject.Execute(subjectBuilder, data)
	if err != nil {
		return err
	}

	bodyBuilder := &strings.Builder{}
	err = tmpl.Body.Execute(bodyBuilder, data)
	if err != nil {
		return err
	}

	logging.Debug.Printf("Sending email to %s using %s", to, provider)

	svc := email.GetProvider(provider)
	if svc == nil {
		return skypanel.ErrServiceInvalidProvider("email", provider)
	}

	if async {
		go func(emailService email.Provider, toEmail, subject, body string) {
			err := emailService.Send(toEmail, subject, body)
			if err != nil {
				logging.Error.Printf("Error sending email: %s", err)
			}
		}(svc, to, subjectBuilder.String(), bodyBuilder.String())
		return nil
	}

	return svc.Send(to, subjectBuilder.String(), bodyBuilder.String())
}

// emailHostName is the panel's host name (panel.example.com), which templates use as
// HOST_NAME; the company name when there is no Base URL to take it from
func emailHostName(masterURL, companyName string) string {
	if parsed, err := url.Parse(masterURL); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return companyName
}
