package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeJoin_ValidPaths(t *testing.T) {
	base, err := filepath.Abs(t.TempDir())
	assert.NoError(t, err)

	cases := []struct {
		name string
		path string
	}{
		{name: "simple file", path: "server.jar"},
		{name: "nested folder", path: "world/region"},
		{name: "dot", path: "."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := safeJoin(base, tc.path)
			assert.NoError(t, err)
			assert.Contains(t, got, filepath.Join(base, tc.path))
		})
	}
}

func TestSafeJoin_PathTraversalRejected(t *testing.T) {
	base, err := filepath.Abs(t.TempDir())
	assert.NoError(t, err)

	cases := []string{
		"../outside.txt",
		"../../etc/passwd",
		"a/../../../evil",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			_, err := safeJoin(base, p)
			assert.ErrorIs(t, err, ErrPathTraversal)
		})
	}
}

func TestCompressAndExtractRoundTrip(t *testing.T) {
	// Build a server root with a couple of nested files (the classic
	// transfer layout: direct children of the root).
	root := t.TempDir()
	fs, err := NewFileServer(root, -1, -1)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer fs.Close()

	err = fs.MkdirAll("config", 0755)
	assert.NoError(t, err)
	assert.NoError(t, writeFile(t, fs, "server.properties", []byte("motd=hello")))
	assert.NoError(t, writeFile(t, fs, "config/settings.yml", []byte("enabled: true")))

	// Compress the direct children using the "*" pattern (as transfer_logic does).
	err = Compress(fs, "transfer.tar.gz", []string{"*"})
	assert.NoError(t, err)

	// New destination folder (its own FileServer, mirroring the transfer flow).
	// The archive to extract is located inside the destination root.
	restoredRoot := filepath.Join(root, "restored")
	if !assert.NoError(t, os.MkdirAll(restoredRoot, 0755)) {
		t.FailNow()
	}
	destFS, err := NewFileServer(restoredRoot, -1, -1)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer destFS.Close()
	err = writeFile(t, destFS, "transfer.tar.gz", mustRead(t, filepath.Join(root, "transfer.tar.gz")))
	assert.NoError(t, err)

	err = Extract(destFS, "transfer.tar.gz", ".", "*", false, nil)
	assert.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(restoredRoot, "server.properties"))
	assert.NoError(t, err)
	assert.Equal(t, "motd=hello", string(content))

	content, err = os.ReadFile(filepath.Join(restoredRoot, "config", "settings.yml"))
	assert.NoError(t, err)
	assert.Equal(t, "enabled: true", string(content))
}

func TestDetermineIfSingleRoot(t *testing.T) {
	ctx := context.Background()

	// Single root: only one top-level folder inside the archive.
	singleRoot := t.TempDir()
	singleFolder := filepath.Join(singleRoot, "world")
	assert.NoError(t, os.MkdirAll(singleFolder, 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(singleFolder, "level.dat"), []byte("data"), 0644))
	singleArchive := filepath.Join(singleRoot, "single.tar.gz")
	assert.NoError(t, Compress(nil, singleArchive, []string{singleFolder}))

	ok, err := DetermineIfSingleRoot(ctx, singleArchive, mustOpen(t, singleArchive))
	assert.NoError(t, err)
	assert.True(t, ok)

	// Multi root: two top-level folders -> not a single root.
	multiRoot := t.TempDir()
	assert.NoError(t, os.MkdirAll(filepath.Join(multiRoot, "alpha"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(multiRoot, "beta"), 0755))
	multiArchive := filepath.Join(multiRoot, "multi.tar.gz")
	assert.NoError(t, Compress(nil, multiArchive, []string{filepath.Join(multiRoot, "alpha"), filepath.Join(multiRoot, "beta")}))

	ok, err = DetermineIfSingleRoot(ctx, multiArchive, mustOpen(t, multiArchive))
	assert.False(t, ok)
	_ = err
}

func writeFile(t *testing.T, fs FileServer, path string, data []byte) error {
	t.Helper()
	f, err := fs.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

func mustOpen(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return f
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return b
}
