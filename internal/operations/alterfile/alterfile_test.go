package alterfile

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

func TestAlterFile_ReplacesWithinServerRoot(t *testing.T) {
	root, args := setupRun(t)

	target := filepath.Join(root, "server.properties")
	assert.NoError(t, os.WriteFile(target, []byte("motd=old-message\n"), 0644))

	op := AlterFile{TargetFile: "server.properties", Search: "old-message", Replace: "new-message"}
	res := op.Run(args)
	assert.NoError(t, res.Error)

	data, err := os.ReadFile(target)
	assert.NoError(t, err)
	assert.Equal(t, "motd=new-message\n", string(data))
}

func TestAlterFile_RefusesPathTraversal(t *testing.T) {
	root, args := setupRun(t)

	outside := filepath.Join(filepath.Dir(root), "outside-secret.txt")
	assert.NoError(t, os.WriteFile(outside, []byte("topsecret"), 0644))
	t.Cleanup(func() { _ = os.Remove(outside) })

	rel, err := filepath.Rel(root, outside)
	assert.NoError(t, err)

	op := AlterFile{TargetFile: rel, Search: "topsecret", Replace: "owned"}
	res := op.Run(args)
	assert.Error(t, res.Error, "a path outside the server root must be refused")

	data, err := os.ReadFile(outside)
	assert.NoError(t, err)
	assert.Equal(t, "topsecret", string(data), "the file outside the root must be untouched")
}

func TestAlterFile_RefusesSymlinkEscapingRoot(t *testing.T) {
	root, args := setupRun(t)

	outside := filepath.Join(filepath.Dir(root), "outside-config.txt")
	assert.NoError(t, os.WriteFile(outside, []byte("host-data"), 0644))
	t.Cleanup(func() { _ = os.Remove(outside) })

	// the game process can create arbitrary symlinks inside its own folder
	assert.NoError(t, os.Symlink(outside, filepath.Join(root, "server.properties")))

	op := AlterFile{TargetFile: "server.properties", Search: "host-data", Replace: "owned"}
	res := op.Run(args)
	assert.Error(t, res.Error, "a symlink escaping the server root must be refused")

	data, err := os.ReadFile(outside)
	assert.NoError(t, err)
	assert.Equal(t, "host-data", string(data), "the host file must be untouched")
}
