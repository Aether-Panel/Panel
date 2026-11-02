package oauth2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenURL(t *testing.T) {
	for _, c := range []struct{ configured, expected string }{
		// only the panel's address, as in the default: the token path is added
		{"http://localhost:8080", "http://localhost:8080/oauth2/token"},
		{"https://panel.example.com/", "https://panel.example.com/oauth2/token"},
		{"http://10.0.0.5:8080?x=1", "http://10.0.0.5:8080/oauth2/token?x=1"},
		// already the full token URL, or a panel under a sub path: kept as written
		{"http://localhost:8080/oauth2/token", "http://localhost:8080/oauth2/token"},
		{"https://example.com/panel/oauth2/token", "https://example.com/panel/oauth2/token"},
		// not an absolute URL: left alone, the request reports the problem
		{"", ""},
		{"localhost", "localhost"},
	} {
		assert.Equal(t, c.expected, tokenURL(c.configured), c.configured)
	}
}
