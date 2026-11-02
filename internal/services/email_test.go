package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailHostName(t *testing.T) {
	assert.Equal(t, "panel.example.com", emailHostName("https://panel.example.com:8443/", "Acme"))
	assert.Equal(t, "panel.example.com", emailHostName("http://panel.example.com", "Acme"))
	assert.Equal(t, "Acme", emailHostName("", "Acme"), "no Base URL: the company name")
}

// account-creation.html says where the account was made through HOST_NAME, which nothing
// used to fill in, so the email read "created for you on ."
func TestAccountCreationEmail_NamesTheHost(t *testing.T) {
	LoadEmailService()
	tmpl := globalEmailService.templates["accountCreation"]
	require.NotNil(t, tmpl)

	body := &strings.Builder{}
	require.NoError(t, tmpl.Body.Execute(body, map[string]interface{}{
		"COMPANY_NAME": "Acme",
		"MASTER_URL":   "https://panel.example.com",
		"HOST_NAME":    emailHostName("https://panel.example.com", "Acme"),
	}))
	assert.Contains(t, body.String(), "created for you on panel.example.com.")
}
