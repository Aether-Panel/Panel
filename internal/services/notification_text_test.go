package services

import (
	"regexp"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
)

// formatVerb matches one fmt verb, %% included
var formatVerb = regexp.MustCompile(`%[-+# 0]*[0-9.]*[a-zA-Z%]`)

func TestNotificationTexts_SameKeysAndVerbsInEveryLanguage(t *testing.T) {
	assert.ElementsMatch(t, []string{"es", "en"}, keysOf(notificationTexts))
	es, en := notificationTexts["es"], notificationTexts["en"]
	assert.ElementsMatch(t, keysOf(es), keysOf(en), "every text needs both languages")

	for key, text := range es {
		assert.NotEmpty(t, text, key)
		assert.NotEmpty(t, en[key], key)
		// the same values are passed whatever the language, so they must be used in the same order
		assert.Equal(t, formatVerb.FindAllString(text, -1), formatVerb.FindAllString(en[key], -1), key)
	}
}

func TestNotificationTextIn(t *testing.T) {
	assert.Equal(t, "El servidor **survival** está ahora online.", NotificationTextIn("es", "serverOnline.description", "survival"))
	assert.Equal(t, "The server **survival** is online again.", NotificationTextIn("en", "serverOnline.description", "survival"))
	assert.Equal(t, "Mostrando 24 de 30 servidores", NotificationTextIn("es", "systemStatus.showing", 24, 30))
	assert.Equal(t, "survival is using 95% of its memory (the alert starts at 90%).",
		NotificationTextIn("en", "notify.resource", "survival", 95.0, "memory", 90.0))
	assert.Equal(t, "Sí", NotificationTextIn("es", "yes"), "a text without values is returned as it is")
	assert.Equal(t, "Yes", NotificationTextIn("fr", "yes"), "an unknown language falls back to English")
	assert.Equal(t, "no.such.key", NotificationTextIn("es", "no.such.key"), "a missing key shows itself")
}

func TestNotificationLanguage_FollowsTheSetting(t *testing.T) {
	t.Cleanup(func() { _ = config.NotificationLanguage.Set("es", false) })

	_ = config.NotificationLanguage.Set("en", false)
	assert.Equal(t, "en", NotificationLanguage())
	assert.Equal(t, "Server", NotificationText("field.server"))

	_ = config.NotificationLanguage.Set("es", false)
	assert.Equal(t, "Servidor", NotificationText("field.server"))

	for _, value := range []string{"", "fr", "EN"} {
		_ = config.NotificationLanguage.Set(value, false)
		assert.Equal(t, DefaultNotificationLanguage, NotificationLanguage(), "%q", value)
	}

	assert.True(t, IsNotificationLanguage("es"))
	assert.True(t, IsNotificationLanguage("en"))
	assert.False(t, IsNotificationLanguage(""))
	assert.False(t, IsNotificationLanguage("fr"))
}

func keysOf[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
