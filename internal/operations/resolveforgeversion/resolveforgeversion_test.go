package resolveforgeversion

import (
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
)

func pickAll(folders []string, mcVersion string) string {
	var current string
	var ver *version.Version
	for _, f := range folders {
		current, ver = pickVersion(current, ver, f, mcVersion)
	}
	return current
}

func TestPickVersion_NewestForTheMinecraftVersion(t *testing.T) {
	folders := []string{"1.20.1-47.1.0", "1.20.1-47.2.0", "1.19.2-43.2.0", "1.20.1-47.1.3"}
	assert.Equal(t, "1.20.1-47.2.0", pickAll(folders, "1.20.1"))
}

func TestPickVersion_AFolderForTheWantedVersionReplacesTheFirstOne(t *testing.T) {
	folders := []string{"1.19.2-43.2.0", "1.20.1-47.1.0"}
	assert.Equal(t, "1.20.1-47.1.0", pickAll(folders, "1.20.1"))
}

func TestPickVersion_KeepsTheFirstWhenNoneMatches(t *testing.T) {
	folders := []string{"1.19.2-43.2.0", "1.18.2-40.2.0"}
	assert.Equal(t, "1.19.2-43.2.0", pickAll(folders, "1.20.1"))
}
