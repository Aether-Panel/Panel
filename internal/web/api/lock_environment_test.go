package api

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

func TestLockEnvironment_NonAdminsCannotReachTheHost(t *testing.T) {
	current := skypanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{
		"image":      "ghcr.io/aether-panel/java:21",
		"hostConfig": map[string]interface{}{"Privileged": false},
	}}
	wanted := skypanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{
		"image":      "ghcr.io/aether-panel/java:17",
		"bindings":   map[string]interface{}{"/": "/host"},
		"hostConfig": map[string]interface{}{"Privileged": true},
	}}

	got := lockEnvironment(current, wanted)

	assert.Equal(t, "docker", got.Type)
	assert.Equal(t, "ghcr.io/aether-panel/java:17", got.Metadata["image"], "the image may change")
	assert.NotContains(t, got.Metadata, "bindings", "no host mounts")
	assert.Equal(t, map[string]interface{}{"Privileged": false}, got.Metadata["hostConfig"], "hostConfig stays as it was")
}

func TestLockEnvironment_TheTypeCannotChange(t *testing.T) {
	current := skypanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{"image": "a"}}

	got := lockEnvironment(current, skypanel.MetadataType{Type: "host", Metadata: map[string]interface{}{}})

	assert.Equal(t, current, got, "switching to a host environment is refused: the old one is kept whole")
}
