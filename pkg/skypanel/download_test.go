package skypanel

import (
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestCachePath(t *testing.T) {
	cache := t.TempDir()
	previous := config.CacheFolder.Value()
	_ = config.CacheFolder.Set(cache, false)
	t.Cleanup(func() { _ = config.CacheFolder.Set(previous, false) })

	got, ok := cachePath("https://maven.example.com/net/forge/1.0/forge-1.0-installer.jar")
	assert.True(t, ok)
	// the scheme is not part of the path: "https:// " with a space never matched
	assert.Equal(t, filepath.Join(cache, "maven.example.com", "net", "forge", "1.0", "forge-1.0-installer.jar"), got)

	got, ok = cachePath("http://example.com/file.jar")
	assert.True(t, ok)
	assert.Equal(t, filepath.Join(cache, "example.com", "file.jar"), got)

	_, ok = cachePath("https://example.com/../../../etc/passwd")
	assert.False(t, ok, "an address with .. must not be cached outside the cache folder")
}
