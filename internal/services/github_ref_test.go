package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeGitHubRef_KeepsSlashes(t *testing.T) {
	assert.Equal(t, "feature/login", EscapeGitHubRef("feature/login"))
	assert.Equal(t, "fix/a%20b", EscapeGitHubRef("fix/a b"))
	assert.Equal(t, "main", EscapeGitHubRef("main"))
}

func TestValidGitHubNames(t *testing.T) {
	assert.True(t, ValidGitHubName("octo-app.v2"))
	assert.False(t, ValidGitHubName(".."))
	assert.False(t, ValidGitHubName("a/b"))
	assert.True(t, ValidGitHubBranch("feature/login"))
	for _, bad := range []string{"", "a..b", "/x", "x/", "a b", "x.lock", "a~b", "a:b"} {
		assert.False(t, ValidGitHubBranch(bad), bad)
	}
}
