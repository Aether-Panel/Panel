package email

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/mailjet/mailjet-apiv3-go/v4"
)

type mailjetProvider struct {
	Provider
}

func init() {
	providers["mailjet"] = mailjetProvider{}
}

func (mailjetProvider) Send(to, subject, body string) error {
	domain := config.EmailDomain.Value()
	if domain == "" {
		return skypanel.ErrSettingNotConfigured(config.EmailDomain.Key())
	}

	from := config.EmailFrom.Value()
	if from == "" {
		return skypanel.ErrSettingNotConfigured(config.EmailFrom.Key())
	}

	key := config.EmailKey.Value()
	if key == "" {
		return skypanel.ErrSettingNotConfigured(config.EmailKey.Key())
	}

	m := mailjet.NewMailjetClient(domain, key)

	messagesInfo := []mailjet.InfoMessagesV31{
		{
			From: &mailjet.RecipientV31{
				Email: from,
			},
			To: &mailjet.RecipientsV31{
				mailjet.RecipientV31{
					Email: to,
				},
			},
			Subject:  subject,
			HTMLPart: body,
		},
	}
	message := mailjet.MessagesV31{Info: messagesInfo}

	_, err := m.SendMailV31(&message)
	return err
}
