package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareWildcard(t *testing.T) {
	tests := []struct {
		source, match string
		want          bool
	}{
		{"server.jar", "", true},
		{"server.jar", "*", true},
		{"server.jar", "server.jar", true},
		{"server.jar", "other.jar", false},
		{"server.jar", "*.jar", true},
		{"server.zip", "*.jar", false},
		{"forge-1.20.1-installer.jar", "forge-*-installer.jar", true},
		{"forge-1.20.1-installer.jar", "forge-*", true},
		{"world/level.dat", "world/*", true},
		// the dot is literal, not "any character"
		{"serverxjar", "server.*", false},
		{"a.b", "a?b*", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, CompareWildcard(tt.source, tt.match), "CompareWildcard(%q, %q)", tt.source, tt.match)
	}
}
