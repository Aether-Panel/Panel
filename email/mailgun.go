package email

import (
	"context"
	"github.com/mailgun/mailgun-go/v4"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/config"
)

type mailgunProvider struct {
	Provider
}

func init() {
	providers["mailgun"] = mailgunProvider{}
}

func (mailgunProvider) Send(to, subject, body string) error {
	domain := config.EmailDomain.Value()
	if domain == "" {
		return SkyPanel.ErrSettingNotConfigured(config.EmailDomain.Key())
	}

	from := config.EmailFrom.Value()
	if from == "" {
		return SkyPanel.ErrSettingNotConfigured(config.EmailFrom.Key())
	}

	key := config.EmailKey.Value()
	if key == "" {
		return SkyPanel.ErrSettingNotConfigured(config.EmailKey.Key())
	}

	message := mailgun.NewMessage(from, subject, "", to)
	message.SetHTML(body)

	_, _, err := mailgun.NewMailgun(domain, key).Send(context.Background(), message)
	return err
}
