package email

import (
	"context"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/mailgun/mailgun-go/v4"
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

	message := mailgun.NewMessage(from, subject, "", to)
	message.SetHTML(body)

	_, _, err := mailgun.NewMailgun(domain, key).Send(context.Background(), message)
	return err
}
