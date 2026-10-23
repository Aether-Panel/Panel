package mkdir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

type stubServer struct {
	fs files.FileServer
}

func (s *stubServer) GetFileServer() files.FileServer                       { return s.fs }
func (s *stubServer) Extract(source, destination string) error              { return nil }
func (s *stubServer) ArchiveItems(items []string, destination string) error { return nil }
func (s *stubServer) DataToMap() map[string]interface{}                     { return nil }

func setupRun(t *testing.T) (string, skypanel.RunOperatorArgs) {
	root := t.TempDir()

	fs, err := files.NewFileServer(root, os.Getuid(), os.Getgid())
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	env := &skypanel.Environment{
		RootDirectory:  root,
		ConsoleBuffer:  skypanel.CreateCache(),
		ConsoleTracker: &skypanel.Tracker{},
	}

	return root, skypanel.RunOperatorArgs{
		Environment: env,
		Server:      &stubServer{fs: fs},
	}
}

func TestMkdir_CreatesInsideRoot(t *testing.T) {
	root, args := setupRun(t)

	op := &Mkdir{TargetFile: "plugins/data"}
	res := op.Run(args)
	assert.NoError(t, res.Error)

	info, err := os.Stat(filepath.Join(root, "plugins", "data"))
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestMkdir_RefusesPathTraversal(t *testing.T) {
	root, args := setupRun(t)

	op := &Mkdir{TargetFile: "../escaped-dir"}
	res := op.Run(args)
	assert.Error(t, res.Error)

	_, err := os.Stat(filepath.Join(filepath.Dir(root), "escaped-dir"))
	assert.True(t, os.IsNotExist(err), "no directory may be created outside the root")
}

func TestMkdir_RefusesSymlinkEscapingRoot(t *testing.T) {
	root, args := setupRun(t)

	outside := t.TempDir()
	// the game process can create arbitrary symlinks inside its own folder
	assert.NoError(t, os.Symlink(outside, filepath.Join(root, "data")))

	op := &Mkdir{TargetFile: "data/escaped"}
	res := op.Run(args)
	assert.Error(t, res.Error, "a symlink escaping the server root must be refused")

	_, err := os.Stat(filepath.Join(outside, "escaped"))
	assert.True(t, os.IsNotExist(err), "no directory may be created through the symlink")
}
