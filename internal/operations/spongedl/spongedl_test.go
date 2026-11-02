package spongedl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerJarURL(t *testing.T) {
	current := []SpongeAPIV2Asset{
		{Classifier: "", DownloadURL: "https://repo/sponge-dev.jar", Extension: "jar"},
		{Classifier: "universal", DownloadURL: "https://repo/sponge-universal.jar", Extension: "jar"},
		{Classifier: "sources", DownloadURL: "https://repo/sponge-sources.jar", Extension: "jar"},
		{Classifier: "universal", DownloadURL: "https://repo/sponge-universal.pom", Extension: "pom"},
	}
	assert.Equal(t, "https://repo/sponge-universal.jar", serverJarURL(current))

	// older builds only have the jar without a classifier
	old := []SpongeAPIV2Asset{
		{Classifier: "", DownloadURL: "https://repo/sponge.jar", Extension: "jar"},
		{Classifier: "sources", DownloadURL: "https://repo/sponge-sources.jar", Extension: "jar"},
	}
	assert.Equal(t, "https://repo/sponge.jar", serverJarURL(old))

	assert.Empty(t, serverJarURL([]SpongeAPIV2Asset{{Classifier: "sources", DownloadURL: "x", Extension: "jar"}}))
}
