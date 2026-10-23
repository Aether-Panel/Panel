package servers

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCloneFS(t *testing.T, dir string) files.FileServer {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0755))
	fsys, err := files.NewFileServer(dir, -1, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fsys.Close() })
	return fsys
}

func writeCloneFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), mode))
	require.NoError(t, os.Chmod(path, mode))
}

func TestCopyServerTree_CopiesFilesDirsAndModes(t *testing.T) {
	base := t.TempDir()
	srcDir, dstDir := filepath.Join(base, "src"), filepath.Join(base, "dst")
	writeCloneFile(t, filepath.Join(srcDir, "server.jar"), "jar", 0644)
	writeCloneFile(t, filepath.Join(srcDir, "start.sh"), "#!/bin/sh", 0755)
	writeCloneFile(t, filepath.Join(srcDir, "world", "region", "r.0.0.mca"), "chunks", 0600)
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "empty"), 0700))
	old := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, os.Chtimes(filepath.Join(srcDir, "server.jar"), old, old))
	// a read-only folder must still be filled before it gets its mode
	writeCloneFile(t, filepath.Join(srcDir, "locked", "readme.txt"), "ro", 0444)
	require.NoError(t, os.Chmod(filepath.Join(srcDir, "locked"), 0555))
	t.Cleanup(func() {
		_ = os.Chmod(filepath.Join(srcDir, "locked"), 0755)
		_ = os.Chmod(filepath.Join(dstDir, "locked"), 0755)
	})

	var progress int
	stats, err := CopyServerTree(newCloneFS(t, srcDir), newCloneFS(t, dstDir), CopyOptions{UID: -1, GID: -1, Progress: func(CopyStats) { progress++ }})
	require.NoError(t, err)

	assert.Equal(t, 4, stats.Files)
	assert.Equal(t, int64(len("jar")+len("#!/bin/sh")+len("chunks")+len("ro")), stats.Bytes)
	assert.Equal(t, 4, progress)

	for rel, want := range map[string]string{"server.jar": "jar", "start.sh": "#!/bin/sh", "world/region/r.0.0.mca": "chunks", "locked/readme.txt": "ro"} {
		got, err := os.ReadFile(filepath.Join(dstDir, rel))
		require.NoError(t, err, rel)
		assert.Equal(t, want, string(got), rel)
	}
	assertCloneMode(t, filepath.Join(dstDir, "start.sh"), 0755)
	assertCloneMode(t, filepath.Join(dstDir, "world", "region", "r.0.0.mca"), 0600)
	assertCloneMode(t, filepath.Join(dstDir, "empty"), 0700|fs.ModeDir)
	assertCloneMode(t, filepath.Join(dstDir, "locked"), 0555|fs.ModeDir)

	info, err := os.Stat(filepath.Join(dstDir, "server.jar"))
	require.NoError(t, err)
	assert.True(t, info.ModTime().Equal(old), "modification times are kept")

	// the original is untouched
	got, err := os.ReadFile(filepath.Join(srcDir, "server.jar"))
	require.NoError(t, err)
	assert.Equal(t, "jar", string(got))
}

func assertCloneMode(t *testing.T, path string, want fs.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	require.NoError(t, err, path)
	assert.Equal(t, want, info.Mode()&(fs.ModePerm|fs.ModeDir), path)
}

func TestCopyServerTree_SymlinksStayInsideTheRoot(t *testing.T) {
	base := t.TempDir()
	srcDir, dstDir := filepath.Join(base, "src"), filepath.Join(base, "dst")
	writeCloneFile(t, filepath.Join(srcDir, "config", "real.yml"), "a: 1", 0644)
	writeCloneFile(t, filepath.Join(base, "secret.txt"), "host secret", 0644)

	require.NoError(t, os.Symlink("config/real.yml", filepath.Join(srcDir, "inside.yml")))
	require.NoError(t, os.Symlink(filepath.Join(srcDir, "config", "real.yml"), filepath.Join(srcDir, "config", "absolute.yml")))
	require.NoError(t, os.Symlink("../secret.txt", filepath.Join(srcDir, "escape.txt")))
	require.NoError(t, os.Symlink("../../secret.txt", filepath.Join(srcDir, "config", "escape2.txt")))
	require.NoError(t, os.Symlink(filepath.Join(base, "secret.txt"), filepath.Join(srcDir, "absolute-escape.txt")))
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(srcDir, "passwd")))

	stats, err := CopyServerTree(newCloneFS(t, srcDir), newCloneFS(t, dstDir), CopyOptions{UID: -1, GID: -1})
	require.NoError(t, err)
	assert.Equal(t, 4, stats.Skipped, "the four links leaving the root are skipped")

	link, err := os.Readlink(filepath.Join(dstDir, "inside.yml"))
	require.NoError(t, err)
	assert.Equal(t, "config/real.yml", link)

	link, err = os.Readlink(filepath.Join(dstDir, "config", "absolute.yml"))
	require.NoError(t, err)
	assert.Equal(t, "real.yml", link, "an absolute link into the root points into the copy, not the original")

	for _, rel := range []string{"escape.txt", "config/escape2.txt", "absolute-escape.txt", "passwd"} {
		_, err := os.Lstat(filepath.Join(dstDir, rel))
		assert.True(t, os.IsNotExist(err), "%s must not be copied", rel)
	}
}

// vanishingFS deletes a file right after listing it, like a server rotating its logs
type vanishingFS struct {
	files.FileServer
	victim string
}

func (v *vanishingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := v.FileServer.ReadDir(name)
	if name == "." {
		_ = os.Remove(filepath.Join(v.Prefix(), v.victim))
	}
	return entries, err
}

func TestCopyServerTree_IgnoresFilesThatVanish(t *testing.T) {
	base := t.TempDir()
	srcDir, dstDir := filepath.Join(base, "src"), filepath.Join(base, "dst")
	writeCloneFile(t, filepath.Join(srcDir, "latest.log"), "log", 0644)
	writeCloneFile(t, filepath.Join(srcDir, "world.dat"), "world", 0644)

	stats, err := CopyServerTree(&vanishingFS{FileServer: newCloneFS(t, srcDir), victim: "latest.log"}, newCloneFS(t, dstDir), CopyOptions{UID: -1, GID: -1})
	require.NoError(t, err)
	assert.Equal(t, 1, stats.Files)
	assert.Equal(t, 1, stats.Skipped)
	assert.FileExists(t, filepath.Join(dstDir, "world.dat"))
}

func TestCopySideFiles_OnlyInstalledContentIsCopied(t *testing.T) {
	folder := t.TempDir()
	writeCloneFile(t, filepath.Join(folder, "SRC.modpack"), `{"name":"pack"}`, 0600)
	writeCloneFile(t, filepath.Join(folder, "SRC.worldmap", "map.json"), `{"map":"x"}`, 0644)
	for _, ext := range []string{".metrics", ".crashes", ".notify", ".history", ".cron"} {
		writeCloneFile(t, filepath.Join(folder, "SRC"+ext), "private", 0600)
	}

	require.NoError(t, copySideFiles(folder, "SRC", "NEW"))

	got, err := os.ReadFile(filepath.Join(folder, "NEW.modpack"))
	require.NoError(t, err)
	assert.Equal(t, `{"name":"pack"}`, string(got))
	assertCloneMode(t, filepath.Join(folder, "NEW.modpack"), 0600)
	got, err = os.ReadFile(filepath.Join(folder, "NEW.worldmap", "map.json"))
	require.NoError(t, err)
	assert.Equal(t, `{"map":"x"}`, string(got))
	for _, ext := range []string{".metrics", ".crashes", ".notify", ".history", ".cron"} {
		assert.NoFileExists(t, filepath.Join(folder, "NEW"+ext))
	}

	// nothing to copy is fine
	require.NoError(t, copySideFiles(folder, "NONE", "NEW2"))
}

func TestIsPortVariable(t *testing.T) {
	for key, want := range map[string]bool{
		"port": true, "port2": true, "PORT3": true, "query_port": true, "rcon-port": true,
		"queryport": true, "rconport": true, "steam.port": true,
		"export": false, "support": false, "passport": false, "memory": false, "portal": false, "ports": false,
	} {
		assert.Equal(t, want, IsPortVariable(key), key)
	}
}

func TestClonePortMapping(t *testing.T) {
	source := map[string]skypanel.Variable{
		"port": {Value: float64(25565)}, "query_port": {Value: "25565"}, "rconport": {Value: float64(25575)},
		"memory": {Value: float64(1024)}, "port2": {Value: float64(0)},
	}
	target := map[string]skypanel.Variable{
		"port": {Value: 25568}, "query_port": {Value: "25568"}, "rconport": {Value: 25576},
		"memory": {Value: float64(2048)}, "port2": {Value: 0},
	}
	assert.Equal(t, map[string]string{"25565": "25568", "25575": "25576"}, ClonePortMapping(source, target))
}

func TestRemapPropertyPorts(t *testing.T) {
	in := "#Minecraft server properties\r\nserver-port=25565\r\nquery.port=25565\r\nrcon.port=25575\r\nmax-players=25565\r\nmotd=25565\r\n# server-port=25565\r\nlevel-name=world"
	out, changed := RemapPropertyPorts([]byte(in), map[string]string{"25565": "25568", "25575": "25576"})

	assert.True(t, changed)
	assert.Equal(t, "#Minecraft server properties\r\nserver-port=25568\r\nquery.port=25568\r\nrcon.port=25576\r\nmax-players=25565\r\nmotd=25565\r\n# server-port=25565\r\nlevel-name=world", string(out))

	_, changed = RemapPropertyPorts([]byte("server-port=30000\n"), map[string]string{"25565": "25568"})
	assert.False(t, changed)
}

func newCloneTestServer(t *testing.T, id, dir string) *Server {
	t.Helper()
	p := CreateProgram()
	p.Identifier = id
	p.SetFileServer(newCloneFS(t, dir))
	p.RunningEnvironment = &skypanel.Environment{
		RootDirectory:  dir,
		ConsoleBuffer:  skypanel.CreateCache(),
		ConsoleTracker: skypanel.CreateTracker(),
		StatusTracker:  skypanel.CreateTracker(),
		Implementation: &fakeEnvironment{},
	}
	return p
}

func TestCloneFrom_CopiesInTheBackgroundAndBlocksStarting(t *testing.T) {
	folder := t.TempDir()
	previous := config.ServersFolder.Value()
	_ = config.ServersFolder.Set(folder, false)
	t.Cleanup(func() { _ = config.ServersFolder.Set(previous, false) })

	source := newCloneTestServer(t, "SRC", filepath.Join(folder, "SRC"))
	source.Variables["port"] = skypanel.Variable{Value: float64(25565)}
	writeCloneFile(t, filepath.Join(folder, "SRC", "server.properties"), "server-port=25565\n", 0644)
	writeCloneFile(t, filepath.Join(folder, "SRC", "world", "level.dat"), "level", 0644)
	writeCloneFile(t, filepath.Join(folder, "SRC.modpack"), "pack", 0644)

	target := newCloneTestServer(t, "NEW", filepath.Join(folder, "NEW"))
	target.Variables["port"] = skypanel.Variable{Value: 25568}

	assert.ErrorIs(t, target.CloneFrom(target, ""), ErrCloneSameServer)
	require.NoError(t, target.CloneFrom(source, "Survival"))

	deadline := time.Now().Add(10 * time.Second)
	for target.cloneState().running.Load() {
		require.True(t, time.Now().Before(deadline), "the copy never finished")
		time.Sleep(10 * time.Millisecond)
	}

	assert.False(t, target.GetEnvironment().IsInstalling(), "the copy can be started once the files are there")
	got, err := os.ReadFile(filepath.Join(folder, "NEW", "world", "level.dat"))
	require.NoError(t, err)
	assert.Equal(t, "level", string(got))
	got, err = os.ReadFile(filepath.Join(folder, "NEW", "server.properties"))
	require.NoError(t, err)
	assert.Equal(t, "server-port=25568\n", string(got))
	assert.FileExists(t, filepath.Join(folder, "NEW.modpack"))

	console := consoleText(target)
	assert.Contains(t, console, "Copying files from Survival")
	assert.Contains(t, console, "Copied 2 files")

	// a second copy into a server that already has files is refused
	assert.ErrorIs(t, target.CloneFrom(source, "Survival"), ErrCloneTargetNotEmpty)
}

func TestCloneFrom_RefusesABusyTarget(t *testing.T) {
	folder := t.TempDir()
	source := newCloneTestServer(t, "SRC", filepath.Join(folder, "SRC"))
	target := newCloneTestServer(t, "NEW", filepath.Join(folder, "NEW"))
	target.RunningEnvironment.Implementation.(*fakeEnvironment).running = true

	assert.ErrorIs(t, target.CloneFrom(source, ""), skypanel.ErrServerRunning)
}
