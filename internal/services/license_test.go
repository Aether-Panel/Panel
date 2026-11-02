package services

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A key with '&' or '#' used to be cut short or split into other query parameters
func TestLicenseVerifyURL_EscapesKey(t *testing.T) {
	key := "AB&CD#EF G+H"

	parsed, err := url.Parse(licenseVerifyURL(key))

	require.NoError(t, err)
	assert.Equal(t, key, parsed.Query().Get("licenseKey"))
	assert.Len(t, parsed.Query(), 1)
}
