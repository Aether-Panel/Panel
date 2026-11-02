package api

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/spf13/cast"
)

// secretMask is what the settings API answers in place of a saved secret. The settings page
// sends it back untouched when the field was not edited, so any request value equal to it
// means "keep the saved one".
const secretMask = "********"

// secretSettingEntries are the settings that grant access to something outside the panel
// (mail, storage, DNS, webhooks, API keys). They are written through the API but never read
// back: anyone holding settings.edit could otherwise copy them out of the panel.
var secretSettingEntries = []config.StringEntry{
	config.EmailKey,
	config.EmailPassword,
	config.GeminiAPIKey,
	config.CurseForgeKey,
	config.DiscordWebhook,
	config.DiscordWebhookSystem,
	config.DiscordWebhookNode,
	config.DiscordWebhookExTransfer,
	config.LicenseKey,
	config.BackupsS3SecretKey,
	config.CloudflareAPIToken,
	config.TurnstileSecretKey,
}

func isSecretSetting(key string) bool {
	for _, v := range secretSettingEntries {
		if v.Key() == key {
			return true
		}
	}
	return false
}

// maskedSettingValue hides a saved secret, keeping an empty one empty so the page can still
// tell whether it is set.
func maskedSettingValue(key string, value interface{}) interface{} {
	if isSecretSetting(key) && cast.ToString(value) != "" {
		return secretMask
	}
	return value
}

// dropMaskedSecrets removes the secrets a client sent back unchanged, so saving or testing
// a form keeps the saved values instead of writing the mask over them.
func dropMaskedSecrets(values map[string]interface{}) {
	for key, value := range values {
		if isSecretSetting(key) && cast.ToString(value) == secretMask {
			delete(values, key)
		}
	}
}
