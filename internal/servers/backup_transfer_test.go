package servers

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingReader gives some bytes, then fails, like a copy whose connection drops
type failingReader struct{ sent bool }

func (f *failingReader) Read(b []byte) (int, error) {
	if f.sent {
		return 0, errors.New("connection lost")
	}
	f.sent = true
	return copy(b, "half an archive"), nil
}

func TestReceiveBackup_WritesTheCopyWithItsTime(t *testing.T) {
	p, _ := newBackupTestServer(t)
	made := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	require.NoError(t, p.ReceiveBackup("auto-abc.tar.gz", made, strings.NewReader("archive")))

	file := filepath.Join(p.GetBackupDirectory(), "auto-abc.tar.gz")
	content, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "archive", string(content))
	info, err := os.Stat(file)
	require.NoError(t, err)
	assert.True(t, info.ModTime().Equal(made), "the backup keeps the time it was made")

	listed, err := p.ListBackups()
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "auto-abc.tar.gz", listed[0].FileName)
}

func TestReceiveBackup_LeavesABackupAlreadyHere(t *testing.T) {
	p, _ := newBackupTestServer(t)
	require.NoError(t, p.ReceiveBackup("b.tar.gz", time.Time{}, strings.NewReader("first")))

	err := p.ReceiveBackup("b.tar.gz", time.Time{}, strings.NewReader("second"))

	assert.ErrorIs(t, err, skypanel.ErrFileExists)
	content, _ := os.ReadFile(filepath.Join(p.GetBackupDirectory(), "b.tar.gz"))
	assert.Equal(t, "first", string(content))
}

func TestReceiveBackup_ACopyCutShortLeavesNothing(t *testing.T) {
	p, _ := newBackupTestServer(t)

	assert.Error(t, p.ReceiveBackup("b.tar.gz", time.Time{}, &failingReader{}))

	entries, err := os.ReadDir(p.GetBackupDirectory())
	require.NoError(t, err)
	assert.Empty(t, entries, "neither the backup nor its partial file")
}

func TestReceiveBackup_RefusesNamesOutsideTheFolder(t *testing.T) {
	p, _ := newBackupTestServer(t)
	for _, name := range []string{"../x.tar.gz", "a/b.tar.gz", ".failed-b.tar.gz", "b.zip", ""} {
		assert.Error(t, p.ReceiveBackup(name, time.Time{}, strings.NewReader("x")), name)
		assert.Error(t, p.RemoveLocalBackup(name), name)
	}
	assert.NoFileExists(t, filepath.Join(filepath.Dir(p.GetBackupDirectory()), "x.tar.gz"))
}

func TestRemoveLocalBackup_RemovesOnlyThatCopy(t *testing.T) {
	p, _ := newBackupTestServer(t)
	require.NoError(t, p.ReceiveBackup("a.tar.gz", time.Time{}, strings.NewReader("a")))
	require.NoError(t, p.ReceiveBackup("b.tar.gz", time.Time{}, strings.NewReader("b")))

	require.NoError(t, p.RemoveLocalBackup("a.tar.gz"))
	assert.NoError(t, p.RemoveLocalBackup("a.tar.gz"), "a copy already gone is fine")

	assert.NoFileExists(t, filepath.Join(p.GetBackupDirectory(), "a.tar.gz"))
	assert.FileExists(t, filepath.Join(p.GetBackupDirectory(), "b.tar.gz"))
}
