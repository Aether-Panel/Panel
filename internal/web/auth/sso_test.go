package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeRedirect_StaysInsideThePanel(t *testing.T) {
	assert.Equal(t, "/servers/abc/", safeRedirect("/servers/abc/"))
	for _, outside := range []string{"", "https://evil.example", "//evil.example", "/\\evil.example", "servers"} {
		assert.Equal(t, "/", safeRedirect(outside), outside)
	}
}
