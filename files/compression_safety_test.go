package files

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
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
	assert.ErrorIs(t, err, ErrPathTraversal, "the file server refuses to go through the link")
}

func TestCompress_AWildcardFolderDoesNotFollowLinks(t *testing.T) {
	fs, root, outside := newSafetyServer(t)
	assert.NoError(t, os.WriteFile(filepath.Join(outside, "shadow"), []byte("root:secret"), 0644))
	assert.NoError(t, os.Symlink(outside, filepath.Join(root, "alink")))

	// the wildcard in the folder used to be expanded by the OS, through the link
	err := Compress(fs, "out.zip", []string{"a*/shadow"})
	assert.Error(t, err)
	_, statErr := os.Stat(filepath.Join(root, "out.zip"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestCompress_StoresLinksAsLinks(t *testing.T) {
	fs, root, outside := newSafetyServer(t)
	assert.NoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("root:secret"), 0644))
	assert.NoError(t, os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "leak")))
	assert.NoError(t, unix.Mkfifo(filepath.Join(root, "pipe"), 0644))

	if !assert.NoError(t, Compress(fs, "all.tar", []string{"*"})) {
		return
	}
	data, err := os.ReadFile(filepath.Join(root, "all.tar"))
	assert.NoError(t, err)
	assert.NotContains(t, string(data), "root:secret", "the target of a link is not read into the archive")
	assert.Contains(t, string(data), "world", "the server's own files are")
}

func TestCompressServer_KeepsTheServerFolderAsRoot(t *testing.T) {
	fs, root, outside := newSafetyServer(t)
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "world", "region"), 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "world", "region", "r.0.0.mca"), []byte("chunk"), 0644))
	target := filepath.Join(outside, "backup.tar.gz")

	if !assert.NoError(t, CompressServer(fs, target)) {
		return
	}

	restored := filepath.Join(t.TempDir(), "restored")
	assert.NoError(t, os.MkdirAll(restored, 0755))
	dest, err := NewFileServer(restored, -1, -1)
	if !assert.NoError(t, err) {
		return
	}
	defer dest.Close()
	in, err := os.Open(target)
	if !assert.NoError(t, err) {
		return
	}
	defer in.Close()
	assert.NoError(t, ExtractReader(dest, in, target, "", "*", true))
	content, err := os.ReadFile(filepath.Join(restored, "world", "region", "r.0.0.mca"))
	assert.NoError(t, err)
	assert.Equal(t, "chunk", string(content))
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
