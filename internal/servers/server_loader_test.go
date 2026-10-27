package servers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
)

func useServersFolder(t *testing.T) string {
	dir := t.TempDir()
	previous := config.ServersFolder.Value()
	_ = config.ServersFolder.Set(dir, false)
	t.Cleanup(func() { _ = config.ServersFolder.Set(previous, false) })
	return dir
}

func TestLoad_AnEmptyDefinitionIsAnError(t *testing.T) {
	dir := useServersFolder(t)
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "empty01.json"), nil, 0644))

	server, err := Load("empty01")
	assert.Error(t, err, "an empty file must not load as a nil server, which crashed the boot")
	assert.Nil(t, server)
}

func TestWriteFileAtomic_ReplacesTheFileAndLeavesNoTemporaries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "srv.json")
	assert.NoError(t, os.WriteFile(path, []byte("old"), 0600))

	assert.NoError(t, writeFileAtomic(path, []byte("new"), 0664))

	content, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "new", string(content))
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1, "the temporary file is renamed over, not left behind")
}
