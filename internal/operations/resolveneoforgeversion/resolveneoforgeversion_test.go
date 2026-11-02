package resolveneoforgeversion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubServer hands the operation a file server on the test folder
type stubServer struct {
	fs files.FileServer
}

func (s *stubServer) GetFileServer() files.FileServer {
	return s.fs
}

func (s *stubServer) Extract(_, _ string) error {
	return nil
}

func (s *stubServer) ArchiveItems(_ []string, _ string) error {
	return nil
}

func (s *stubServer) DataToMap() map[string]interface{} {
	return nil
}

func runIn(t *testing.T, root string, op ResolveNeoForgeVersion) skypanel.OperationResult {
	t.Helper()
	fs, err := files.NewFileServer(root, -1, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fs.Close() })
	env := &skypanel.Environment{RootDirectory: root, ConsoleBuffer: skypanel.CreateCache(), ConsoleTracker: skypanel.CreateTracker()}
	return op.Run(skypanel.RunOperatorArgs{Environment: env, Server: &stubServer{fs: fs}})
}

// installed leaves a NeoForge version folder the way the installer does
func installed(t *testing.T, root, name string, withArgs bool) {
	t.Helper()
	dir := filepath.Join(root, "libraries", "net", "neoforged", "neoforge", name)
	require.NoError(t, os.MkdirAll(dir, 0755))
	if withArgs {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "unix_args.txt"), []byte("-cp x"), 0644))
	}
}

func TestRunFindsTheInstalledNeoForge(t *testing.T) {
	// the NeoForge installer leaves its launch arguments under libraries/net/neoforged/neoforge
	root := t.TempDir()
	installed(t, root, "21.4.150", true)

	result := runIn(t, root, ResolveNeoForgeVersion{MinecraftVersion: "1.21.4", OutputVariable: "resolvedNeoForgeVersion"})
	require.NoError(t, result.Error)
	assert.Equal(t, "21.4.150", result.VariableOverrides["resolvedNeoForgeVersion"])
}

func TestRunPicksTheNewestForTheMinecraftVersion(t *testing.T) {
	root := t.TempDir()
	installed(t, root, "21.1.77", true)
	installed(t, root, "21.4.100", true)
	installed(t, root, "21.4.150", true)
	installed(t, root, "21.4.200", false) // no unix_args.txt: not supported
	installed(t, root, "21.5.1", true)

	result := runIn(t, root, ResolveNeoForgeVersion{MinecraftVersion: "1.21.4", OutputVariable: "v"})
	require.NoError(t, result.Error)
	assert.Equal(t, "21.4.150", result.VariableOverrides["v"])

	result = runIn(t, root, ResolveNeoForgeVersion{MinecraftVersion: "1.21.1", OutputVariable: "v"})
	require.NoError(t, result.Error)
	assert.Equal(t, "21.1.77", result.VariableOverrides["v"])
}

func TestRunWithoutNeoForgeInstalled(t *testing.T) {
	result := runIn(t, t.TempDir(), ResolveNeoForgeVersion{MinecraftVersion: "1.21.4", OutputVariable: "v"})
	require.NoError(t, result.Error)
	assert.Equal(t, "", result.VariableOverrides["v"])
}

func TestRunDoesNotFollowALinkOutOfTheServer(t *testing.T) {
	outside := t.TempDir()
	installed(t, outside, "21.4.150", true)

	root := t.TempDir()
	require.NoError(t, os.Symlink(filepath.Join(outside, "libraries"), filepath.Join(root, "libraries")))

	result := runIn(t, root, ResolveNeoForgeVersion{MinecraftVersion: "1.21.4", OutputVariable: "v"})
	assert.NotEqual(t, "21.4.150", result.VariableOverrides["v"], "a folder outside the server must not be read")
}

func TestNeoForgePrefix(t *testing.T) {
	assert.Equal(t, "21.4.", neoForgePrefix("1.21.4"))
	assert.Equal(t, "21.0.", neoForgePrefix("1.21"))
	assert.Equal(t, "", neoForgePrefix(""))
}
