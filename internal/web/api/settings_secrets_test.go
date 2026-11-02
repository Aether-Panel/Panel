package api

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestMaskedSettingValue_HidesSavedSecrets(t *testing.T) {
	assert.Equal(t, secretMask, maskedSettingValue(config.EmailPassword.Key(), "hunter2"))
	assert.Equal(t, secretMask, maskedSettingValue(config.BackupsS3SecretKey.Key(), "abc"))
	// an empty secret stays empty, so the page can tell it is not set
	assert.Equal(t, "", maskedSettingValue(config.CloudflareAPIToken.Key(), ""))
	// ordinary settings are shown as they are
	assert.Equal(t, "smtp.example.com", maskedSettingValue(config.EmailHost.Key(), "smtp.example.com"))
	assert.Equal(t, "site-key", maskedSettingValue(config.TurnstileSiteKey.Key(), "site-key"))
}

func TestDropMaskedSecrets_KeepsTheSavedValue(t *testing.T) {
	values := map[string]interface{}{
		config.EmailPassword.Key():      secretMask,
		config.CloudflareAPIToken.Key(): "new-token",
		config.EmailHost.Key():          secretMask,
	}

	dropMaskedSecrets(values)

	_, kept := values[config.EmailPassword.Key()]
	assert.False(t, kept, "the mask sent back must not overwrite the saved password")
	assert.Equal(t, "new-token", values[config.CloudflareAPIToken.Key()])
	// only secrets are treated as masked
	assert.Equal(t, secretMask, values[config.EmailHost.Key()])
}

func TestSecretSettingsAreEditable(t *testing.T) {
	editable := map[string]bool{}
	for _, v := range editableStringEntries {
		editable[v.Key()] = true
	}
	for _, v := range secretSettingEntries {
		assert.True(t, editable[v.Key()], v.Key())
	}
}

func TestPasswordFor(t *testing.T) {
	assert.Equal(t, "pw", passwordFor(true, "pw"))
	assert.Equal(t, "", passwordFor(false, "pw"))
}
