package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/internal/resolvemode"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newWorkspace makes a server folder with the given files ("dir/" makes a folder) and a
// workspace over it
func newWorkspace(t *testing.T, dir string, content map[string]string) Workspace {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0755))
	for name, data := range content {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if strings.HasSuffix(name, "/") {
			require.NoError(t, os.MkdirAll(full, 0755))
			continue
		}
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755))
		require.NoError(t, os.WriteFile(full, []byte(data), 0644))
	}
	fsys, err := files.NewFileServer(dir, -1, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fsys.Close() })
	return NewFileServerWorkspace(fsys)
}

func readHost(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	_, err := os.Lstat(path)
	assert.True(t, os.IsNotExist(err), "%s should not exist", path)
}

func TestStat(t *testing.T) {
	resolvemode.Each(t, checkStat)
}

func checkStat(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	ws := newWorkspace(t, root, map[string]string{"server.properties": "motd=hi\n", "world/": ""})

	e, err := ws.Stat(ctx, "server.properties")
	require.NoError(t, err)
	assert.Equal(t, Entry{Name: "server.properties", Path: "server.properties", Size: 8, ModTime: e.ModTime}, e)

	e, err = ws.Stat(ctx, "./world/")
	require.NoError(t, err)
	assert.True(t, e.IsDir)
	assert.Equal(t, "world", e.Path)
	assert.Zero(t, e.Size)

	for _, rootName := range []string{"", "."} {
		e, err = ws.Stat(ctx, rootName)
		require.NoError(t, err)
		assert.Equal(t, ".", e.Path)
		assert.Equal(t, ".", e.Name, "the host folder's name is not shown")
		assert.True(t, e.IsDir)
	}

	_, err = ws.Stat(ctx, "missing.txt")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Equal(t, "not found: missing.txt", err.Error())
	assert.NotContains(t, err.Error(), root)
}

func TestList(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	ws := newWorkspace(t, root, map[string]string{
		"b.txt":           "bb",
		"a.txt":           "a",
		"plugins/x.jar":   "x",
		"config/app.yml":  "k: v",
		"config/sub/z.md": "",
	})

	entries, err := ws.List(ctx, "")
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Path)
	}
	assert.Equal(t, []string{"config", "plugins", "a.txt", "b.txt"}, names, "folders first, then by name")
	assert.Equal(t, int64(2), entries[3].Size)

	entries, err = ws.List(ctx, "config")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "config/sub", entries[0].Path)
	assert.Equal(t, "config/app.yml", entries[1].Path)

	_, err = ws.List(ctx, "a.txt")
	assert.ErrorIs(t, err, ErrNotDir)
	_, err = ws.List(ctx, "nope")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestRead(t *testing.T) {
	ctx := context.Background()
	ws := newWorkspace(t, t.TempDir(), map[string]string{"a.txt": "hello", "big.log": strings.Repeat("x", 100), "dir/": ""})

	data, err := ws.Read(ctx, "a.txt", 10)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))

	data, err = ws.Read(ctx, "big.log", 100)
	require.NoError(t, err)
	assert.Len(t, data, 100, "the limit is inclusive")

	_, err = ws.Read(ctx, "big.log", 99)
	assert.ErrorIs(t, err, ErrTooLarge)

	_, err = ws.Read(ctx, "dir", 10)
	assert.ErrorIs(t, err, ErrIsDir)
	_, err = ws.Read(ctx, "", 10)
	assert.ErrorIs(t, err, ErrInvalidPath)
}

func TestWrite(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	ws := newWorkspace(t, root, map[string]string{"old.txt": "old", "dir/": ""})

	require.NoError(t, ws.Write(ctx, "new/deep/file.txt", []byte("new"), false))
	assert.Equal(t, "new", readHost(t, filepath.Join(root, "new", "deep", "file.txt")), "parents are made")

	err := ws.Write(ctx, "old.txt", []byte("changed"), false)
	assert.ErrorIs(t, err, ErrExists, "creating never replaces")
	assert.Equal(t, "old", readHost(t, filepath.Join(root, "old.txt")))

	require.NoError(t, ws.Write(ctx, "old.txt", []byte("replaced"), true))
	assert.Equal(t, "replaced", readHost(t, filepath.Join(root, "old.txt")))
	require.NoError(t, ws.Write(ctx, "old.txt", []byte("x"), true))
	assert.Equal(t, "x", readHost(t, filepath.Join(root, "old.txt")), "an overwrite truncates")

	assert.ErrorIs(t, ws.Write(ctx, "dir", []byte("x"), true), ErrIsDir)
	assert.ErrorIs(t, ws.Write(ctx, "old.txt/inside", []byte("x"), false), ErrNotDir)
}

func TestWrite_RefusesWritingThroughALink(t *testing.T) {
	resolvemode.Each(t, checkWriteRefusesWritingThroughALink)
}

func checkWriteRefusesWritingThroughALink(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	root := filepath.Join(base, "server")
	ws := newWorkspace(t, root, map[string]string{"real.txt": "real"})
	require.NoError(t, os.WriteFile(filepath.Join(base, "host.txt"), []byte("host"), 0644))
	require.NoError(t, os.Symlink(filepath.Join(base, "host.txt"), filepath.Join(root, "out.txt")))
	require.NoError(t, os.Symlink("real.txt", filepath.Join(root, "in.txt")))

	assert.ErrorIs(t, ws.Write(ctx, "out.txt", []byte("pwned"), true), ErrSymlink)
	assert.ErrorIs(t, ws.Write(ctx, "in.txt", []byte("changed"), true), ErrSymlink)
	assert.ErrorIs(t, ws.Write(ctx, "out.txt", []byte("pwned"), false), ErrExists)
	assert.Equal(t, "host", readHost(t, filepath.Join(base, "host.txt")))
	assert.Equal(t, "real", readHost(t, filepath.Join(root, "real.txt")))
}

func TestMkdir(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	ws := newWorkspace(t, root, map[string]string{"file": "x", "dir/": ""})

	require.NoError(t, ws.Mkdir(ctx, "a/b/c"))
	info, err := os.Stat(filepath.Join(root, "a", "b", "c"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	assert.NoError(t, ws.Mkdir(ctx, "dir"), "an existing folder is fine")
	assert.ErrorIs(t, ws.Mkdir(ctx, "file"), ErrExists)
	assert.ErrorIs(t, ws.Mkdir(ctx, "file/sub"), ErrNotDir)
	assert.ErrorIs(t, ws.Mkdir(ctx, "."), ErrInvalidPath)
}

func TestMove(t *testing.T) {
	resolvemode.Each(t, checkMove)
}

func checkMove(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	ws := newWorkspace(t, root, map[string]string{"a.txt": "a", "b.txt": "b", "dir/x.txt": "x"})

	require.NoError(t, ws.Move(ctx, "a.txt", "backup/old/a.txt"))
	assert.Equal(t, "a", readHost(t, filepath.Join(root, "backup", "old", "a.txt")))
	assertMissing(t, filepath.Join(root, "a.txt"))

	assert.ErrorIs(t, ws.Move(ctx, "b.txt", "dir/x.txt"), ErrExists)
	assert.Equal(t, "x", readHost(t, filepath.Join(root, "dir", "x.txt")), "the target is not replaced")
	assert.ErrorIs(t, ws.Move(ctx, "missing", "elsewhere"), ErrNotFound)
	assert.ErrorIs(t, ws.Move(ctx, "dir", "dir/inner"), ErrInvalid)
	assert.ErrorIs(t, ws.Move(ctx, "dir", "dir"), ErrExists)

	require.NoError(t, ws.Move(ctx, "dir", "renamed"))
	assert.Equal(t, "x", readHost(t, filepath.Join(root, "renamed", "x.txt")))
}

func TestCopy(t *testing.T) {
	resolvemode.Each(t, checkCopy)
}

func checkCopy(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	root := filepath.Join(base, "server")
	ws := newWorkspace(t, root, map[string]string{
		"a.txt":                   "a",
		"world/level.dat":         "level",
		"world/region/r.0.0.mca":  "chunks",
		"world/empty/":            "",
		"other/existing.txt":      "keep",
		"world/data/scoreboard.d": "s",
	})
	require.NoError(t, os.WriteFile(filepath.Join(base, "host.txt"), []byte("host"), 0644))
	require.NoError(t, os.Symlink(filepath.Join(base, "host.txt"), filepath.Join(root, "world", "escape.txt")))
	require.NoError(t, os.Symlink(base, filepath.Join(root, "world", "escape-dir")))
	require.NoError(t, os.Symlink("level.dat", filepath.Join(root, "world", "inside-link")))

	require.NoError(t, ws.Copy(ctx, "a.txt", "copies/a.txt"))
	assert.Equal(t, "a", readHost(t, filepath.Join(root, "copies", "a.txt")))

	require.NoError(t, ws.Copy(ctx, "world", "world_copy"))
	assert.Equal(t, "level", readHost(t, filepath.Join(root, "world_copy", "level.dat")))
	assert.Equal(t, "chunks", readHost(t, filepath.Join(root, "world_copy", "region", "r.0.0.mca")))
	assert.Equal(t, "s", readHost(t, filepath.Join(root, "world_copy", "data", "scoreboard.d")))
	info, err := os.Stat(filepath.Join(root, "world_copy", "empty"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	for _, link := range []string{"escape.txt", "escape-dir", "inside-link"} {
		assertMissing(t, filepath.Join(root, "world_copy", link))
	}

	assert.ErrorIs(t, ws.Copy(ctx, "a.txt", "other/existing.txt"), ErrExists)
	assert.Equal(t, "keep", readHost(t, filepath.Join(root, "other", "existing.txt")))
	assert.ErrorIs(t, ws.Copy(ctx, "world", "other"), ErrExists)
	assert.ErrorIs(t, ws.Copy(ctx, "world", "world/inner"), ErrInvalid)
	assert.ErrorIs(t, ws.Copy(ctx, "world/escape.txt", "stolen.txt"), ErrSymlink)
	assert.ErrorIs(t, ws.Copy(ctx, "world/escape-dir", "stolen"), ErrSymlink)
	assert.ErrorIs(t, ws.Copy(ctx, "missing", "x"), ErrNotFound)
	assertMissing(t, filepath.Join(root, "stolen.txt"))
	assertMissing(t, filepath.Join(root, "stolen"))
}

func TestDelete(t *testing.T) {
	resolvemode.Each(t, checkDelete)
}

func checkDelete(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	root := filepath.Join(base, "server")
	ws := newWorkspace(t, root, map[string]string{"a.txt": "a", "world/region/r.mca": "r", "keep.txt": "k"})
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(outside, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "precious"), []byte("p"), 0644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))

	require.NoError(t, ws.Delete(ctx, "a.txt"))
	assertMissing(t, filepath.Join(root, "a.txt"))

	require.NoError(t, ws.Delete(ctx, "world"))
	assertMissing(t, filepath.Join(root, "world"))

	assert.Error(t, ws.Delete(ctx, "link/precious"), "nothing is reached through a link out")
	require.NoError(t, ws.Delete(ctx, "link"), "the link itself can go")
	assertMissing(t, filepath.Join(root, "link"))
	assert.Equal(t, "p", readHost(t, filepath.Join(outside, "precious")), "what it pointed at stays")

	assert.ErrorIs(t, ws.Delete(ctx, ""), ErrInvalidPath)
	assert.ErrorIs(t, ws.Delete(ctx, "."), ErrInvalidPath)
	assert.ErrorIs(t, ws.Delete(ctx, "missing"), ErrNotFound)
	assert.Equal(t, "k", readHost(t, filepath.Join(root, "keep.txt")))
}

func TestSearch_Names(t *testing.T) {
	ctx := context.Background()
	ws := newWorkspace(t, t.TempDir(), map[string]string{
		"Server.properties":     "",
		"config/server-x.yml":   "",
		"config/other.yml":      "",
		"plugins/ServerKit/a.y": "",
	})

	matches, err := ws.Search(ctx, "", SearchOptions{Query: "SERVER"})
	require.NoError(t, err)
	assert.Equal(t, []Match{{Path: "Server.properties"}, {Path: "config/server-x.yml"}, {Path: "plugins/ServerKit"}}, matches, "breadth first, case-insensitive")

	matches, err = ws.Search(ctx, "config", SearchOptions{Query: "yml"})
	require.NoError(t, err)
	assert.Equal(t, []Match{{Path: "config/other.yml"}, {Path: "config/server-x.yml"}}, matches)

	_, err = ws.Search(ctx, "", SearchOptions{Query: "  "})
	assert.ErrorIs(t, err, ErrInvalid)
	_, err = ws.Search(ctx, "", SearchOptions{Query: strings.Repeat("q", security.Limits.MaxQueryLength+1)})
	assert.ErrorIs(t, err, ErrInvalid)
	_, err = ws.Search(ctx, "Server.properties", SearchOptions{Query: "x"})
	assert.ErrorIs(t, err, ErrNotDir)
}

func TestSearch_Content(t *testing.T) {
	ctx := context.Background()
	long := strings.Repeat("é", 400) + " needle"
	ws := newWorkspace(t, t.TempDir(), map[string]string{
		"server.properties": "motd=Hello\nmax-players=20\n   motd-extra = Hello again  \r\n",
		"binary.dat":        "Hello\x00\x01",
		"logs/latest.log":   "nothing\n" + long + "\n",
	})

	matches, err := ws.Search(ctx, "", SearchOptions{Query: "hello", InContent: true})
	require.NoError(t, err)
	assert.Equal(t, []Match{
		{Path: "server.properties", Line: 1, Text: "motd=Hello"},
		{Path: "server.properties", Line: 3, Text: "motd-extra = Hello again"},
	}, matches, "binary files are skipped, lines are trimmed")

	matches, err = ws.Search(ctx, "", SearchOptions{Query: "NEEDLE", InContent: true})
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, 2, matches[0].Line)
	assert.Equal(t, security.Limits.MaxMatchText, len([]rune(matches[0].Text)), "long lines are cut")

	matches, err = ws.Search(ctx, "", SearchOptions{Query: "motd", InContent: true, MaxResults: 1})
	require.NoError(t, err)
	assert.Len(t, matches, 1)
}

func TestSearch_Limits(t *testing.T) {
	ctx := context.Background()
	content := map[string]string{
		"big.txt":          strings.Repeat("match\n", 50),
		"a/b/c/d/deep.txt": "match",
	}
	for i := 0; i < 30; i++ {
		content["many/f"+string(rune('a'+i%26))+strings.Repeat("x", i/26)+".txt"] = "match"
	}
	ws := newWorkspace(t, t.TempDir(), content)

	matches, err := ws.Search(ctx, "", SearchOptions{Query: "match", InContent: true, MaxFileBytes: 10})
	require.NoError(t, err)
	for _, m := range matches {
		assert.NotEqual(t, "big.txt", m.Path, "files over the size limit are not read")
	}

	matches, err = ws.Search(ctx, "", SearchOptions{Query: "deep", MaxDepth: 4})
	require.NoError(t, err)
	assert.Empty(t, matches, "a/b/c/d/deep.txt is five levels down")
	matches, err = ws.Search(ctx, "", SearchOptions{Query: "deep", MaxDepth: 5})
	require.NoError(t, err)
	assert.Equal(t, []Match{{Path: "a/b/c/d/deep.txt"}}, matches)

	matches, err = ws.Search(ctx, "many", SearchOptions{Query: ".txt", MaxFiles: 5})
	require.NoError(t, err)
	assert.Len(t, matches, 5, "the walk stops after MaxFiles entries")

	matches, err = ws.Search(ctx, "", SearchOptions{Query: "match", InContent: true, MaxResults: 1000})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(matches), security.Limits.MaxSearchResults)
}

func TestSearch_DoesNotFollowLinks(t *testing.T) {
	resolvemode.Each(t, checkSearchDoesNotFollowLinks)
}

func checkSearchDoesNotFollowLinks(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	root := filepath.Join(base, "server")
	ws := newWorkspace(t, root, map[string]string{"own.txt": "nothing here"})
	require.NoError(t, os.MkdirAll(filepath.Join(base, "secret"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "secret", "token.txt"), []byte("TOKEN"), 0644))
	require.NoError(t, os.Symlink(filepath.Join(base, "secret"), filepath.Join(root, "linkdir")))
	require.NoError(t, os.Symlink(filepath.Join(base, "secret", "token.txt"), filepath.Join(root, "linkfile")))

	matches, err := ws.Search(ctx, "", SearchOptions{Query: "token", InContent: true})
	require.NoError(t, err)
	assert.Empty(t, matches)

	matches, err = ws.Search(ctx, "", SearchOptions{Query: "token.txt"})
	require.NoError(t, err)
	assert.Empty(t, matches, "the linked folder is not walked")

	_, err = ws.Search(ctx, "linkdir", SearchOptions{Query: "token"})
	assert.ErrorIs(t, err, ErrOutsideRoot)
}

func TestClampSearchOptions(t *testing.T) {
	l := security.Limits
	got := ClampSearchOptions(SearchOptions{Query: "q"})
	assert.Equal(t, SearchOptions{Query: "q", MaxResults: l.MaxSearchResults, MaxDepth: l.MaxSearchDepth, MaxFiles: l.MaxSearchFiles, MaxFileBytes: l.MaxSearchFileBytes}, got)

	got = ClampSearchOptions(SearchOptions{MaxResults: 1e6, MaxDepth: 99, MaxFiles: 1e9, MaxFileBytes: 1 << 40})
	assert.Equal(t, l.MaxSearchResults, got.MaxResults)
	assert.Equal(t, l.MaxSearchDepth, got.MaxDepth)
	assert.Equal(t, l.MaxSearchFiles, got.MaxFiles)
	assert.Equal(t, l.MaxSearchFileBytes, got.MaxFileBytes)

	got = ClampSearchOptions(SearchOptions{MaxResults: 3, MaxDepth: 2, MaxFiles: 10, MaxFileBytes: 100})
	assert.Equal(t, SearchOptions{MaxResults: 3, MaxDepth: 2, MaxFiles: 10, MaxFileBytes: 100}, got)
}

func TestCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ws := newWorkspace(t, t.TempDir(), map[string]string{"a.txt": "a"})
	_, err := ws.Read(ctx, "a.txt", 10)
	assert.ErrorIs(t, err, context.Canceled)
	assert.ErrorIs(t, ws.Delete(ctx, "a.txt"), context.Canceled)
}

func TestProtectedPathsAreRefused(t *testing.T) {
	ctx := context.Background()
	security.ProtectedPaths = []string{"secret"}
	t.Cleanup(func() { security.ProtectedPaths = nil })
	ws := newWorkspace(t, t.TempDir(), map[string]string{"secret/key.txt": "k", "dir/a.txt": "a"})

	_, err := ws.Read(ctx, "secret/key.txt", 10)
	assert.ErrorIs(t, err, ErrProtected)
	assert.ErrorIs(t, ws.Delete(ctx, "secret"), ErrProtected)
	assert.ErrorIs(t, ws.Move(ctx, "dir", "secret/dir"), ErrProtected)
	entries, err := ws.List(ctx, "")
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotEqual(t, "secret", e.Path)
	}
	matches, err := ws.Search(ctx, "", SearchOptions{Query: "key"})
	require.NoError(t, err)
	assert.Empty(t, matches)
}
