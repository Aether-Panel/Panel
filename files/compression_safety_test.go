package files

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newSafetyServer(t *testing.T) (FileServer, string, string) {
	base := t.TempDir()
	root := filepath.Join(base, "server")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{root, outside} {
		if !assert.NoError(t, os.MkdirAll(dir, 0755)) {
			t.FailNow()
		}
	}
	fs, err := NewFileServer(root, -1, -1)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	t.Cleanup(func() { _ = fs.Close() })
	assert.NoError(t, os.WriteFile(filepath.Join(root, "world.dat"), []byte("world"), 0644))
	return fs, root, outside
}

func TestCompress_RefusesAnExistingDestination(t *testing.T) {
	fs, root, _ := newSafetyServer(t)
	assert.NoError(t, os.WriteFile(filepath.Join(root, "backup.tar.gz"), []byte("keep me"), 0644))

	err := Compress(fs, "backup.tar.gz", []string{"world.dat"})
	assert.ErrorIs(t, err, ErrFileExists)

	content, _ := os.ReadFile(filepath.Join(root, "backup.tar.gz"))
	assert.Equal(t, "keep me", string(content), "the existing file is not overwritten")
}

func TestCompress_DoesNotWriteThroughASymlinkedFolder(t *testing.T) {
	fs, root, outside := newSafetyServer(t)
	assert.NoError(t, os.Symlink(outside, filepath.Join(root, "x")))

	err := Compress(fs, "x/a.tar.gz", []string{"world.dat"})
	assert.Error(t, err)

	_, statErr := os.Stat(filepath.Join(outside, "a.tar.gz"))
	assert.True(t, os.IsNotExist(statErr), "nothing is written outside the server")
}

func TestCompress_DoesNotReadThroughASymlinkedFolder(t *testing.T) {
	fs, root, outside := newSafetyServer(t)
	assert.NoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("secret"), 0644))
	assert.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))

	err := Compress(fs, "out.tar.gz", []string{"link/secret"})
	assert.ErrorIs(t, err, ErrPathTraversal)
}

// limitedWriter refuses writes past max bytes, as the server's disk writer does
type limitedWriter struct {
	w       io.Writer
	written int
	max     int
}

var errQuota = errors.New("quota")

func (l *limitedWriter) Write(b []byte) (int, error) {
	if l.written+len(b) > l.max {
		return 0, errQuota
	}
	l.written += len(b)
	return l.w.Write(b)
}

func TestExtractLimited_StopsAtTheLimit(t *testing.T) {
	fs, root, _ := newSafetyServer(t)
	assert.NoError(t, os.WriteFile(filepath.Join(root, "big.bin"), make([]byte, 64*1024), 0644))
	if !assert.NoError(t, Compress(fs, "big.tar.gz", []string{"big.bin"})) {
		return
	}
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "out"), 0755))

	shared := &limitedWriter{max: 1024}
	err := ExtractLimited(fs, "big.tar.gz", "out", "*", false, func(w io.Writer) io.Writer {
		shared.w = w
		return shared
	})
	assert.ErrorIs(t, err, errQuota)
}

func TestCompressLimited_DeletesAnArchiveTheLimitCutShort(t *testing.T) {
	fs, root, _ := newSafetyServer(t)
	assert.NoError(t, os.WriteFile(filepath.Join(root, "big.bin"), make([]byte, 64*1024), 0644))

	shared := &limitedWriter{max: 1024}
	// a plain tar does not shrink the zeros, so the archive is bigger than the limit
	err := CompressLimited(fs, "big.tar", []string{"big.bin"}, func(w io.Writer) io.Writer {
		shared.w = w
		return shared
	})
	assert.ErrorIs(t, err, errQuota)

	_, statErr := os.Stat(filepath.Join(root, "big.tar"))
	assert.True(t, os.IsNotExist(statErr), "half an archive is not left behind")
}

func TestCompressLimited_KeepsAnArchiveThatFits(t *testing.T) {
	fs, root, _ := newSafetyServer(t)

	shared := &limitedWriter{max: 1 << 20}
	err := CompressLimited(fs, "world.tar.gz", []string{"world.dat"}, func(w io.Writer) io.Writer {
		shared.w = w
		return shared
	})
	assert.NoError(t, err)
	_, statErr := os.Stat(filepath.Join(root, "world.tar.gz"))
	assert.NoError(t, statErr)
}
