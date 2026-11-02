package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/internal/resolvemode"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var registry = func() *toolkit.Registry {
	r := toolkit.NewRegistry()
	Register(r)
	return r
}()

func tool(t *testing.T, name string) toolkit.Tool {
	t.Helper()
	found, ok := registry.Get(name)
	require.True(t, ok, name)
	return found
}

// writeTree creates files under dir; a name ending in "/" is a folder
func writeTree(t *testing.T, dir string, content map[string]string) {
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
}

// serverContext is the ExecContext of a server whose folder is dir
func serverContext(t *testing.T, dir string) toolkit.ExecContext {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0755))
	fsys, err := files.NewFileServer(dir, -1, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fsys.Close() })
	return toolkit.ExecContext{ServerID: filepath.Base(dir), UserID: 1, Workspace: workspace.NewFileServerWorkspace(fsys)}
}

func newServer(t *testing.T, content map[string]string) (string, toolkit.ExecContext) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "server")
	writeTree(t, dir, content)
	return dir, serverContext(t, dir)
}

// run describes then runs a call, as the agent loop does
func run(t *testing.T, ec toolkit.ExecContext, name string, args map[string]any) (toolkit.Result, error) {
	t.Helper()
	tl := tool(t, name)
	if _, err := tl.Describe(context.Background(), ec, args); err != nil {
		return toolkit.Result{}, err
	}
	return tl.Run(context.Background(), ec, args)
}

func output(t *testing.T, res toolkit.Result) map[string]any {
	t.Helper()
	data, err := json.Marshal(res.Output)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

func readFileOnHost(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestRegisterAddsTheTwelveTools(t *testing.T) {
	want := map[string]toolkit.Kind{
		"list_directory":   toolkit.KindRead,
		"read_file":        toolkit.KindRead,
		"file_info":        toolkit.KindRead,
		"search_files":     toolkit.KindRead,
		"create_file":      toolkit.KindCreate,
		"create_directory": toolkit.KindCreate,
		"copy_path":        toolkit.KindCreate,
		"edit_file":        toolkit.KindModify,
		"write_file":       toolkit.KindModify,
		"move_path":        toolkit.KindModify,
		"rename_path":      toolkit.KindModify,
		"delete_path":      toolkit.KindDestructive,
	}
	all := registry.Available([]string{toolkit.ScopeFilesView, toolkit.ScopeFilesEdit})
	require.Len(t, all, len(want))
	for _, tl := range all {
		kind, ok := want[tl.Name()]
		require.True(t, ok, tl.Name())
		assert.Equal(t, kind, tl.Kind(), tl.Name())
		if kind == toolkit.KindRead {
			assert.Equal(t, toolkit.ScopeFilesView, tl.Scope(), tl.Name())
		} else {
			assert.Equal(t, toolkit.ScopeFilesEdit, tl.Scope(), tl.Name())
		}
		assert.NotEmpty(t, tl.Description(), tl.Name())
	}

	var names []string
	for _, tl := range registry.Available([]string{toolkit.ScopeFilesView}) {
		names = append(names, tl.Name())
	}
	assert.Equal(t, []string{"file_info", "list_directory", "read_file", "search_files"}, names)

	assert.Panics(t, func() { Register(registry) }, "registering twice is a mistake")
}

func TestParametersAreGeminiSchemas(t *testing.T) {
	for _, tl := range Tools() {
		schema := tl.Parameters()
		assert.Equal(t, "object", schema["type"], tl.Name())
		properties, ok := schema["properties"].(map[string]any)
		require.True(t, ok, tl.Name())
		require.NotEmpty(t, properties, tl.Name())
		for name, raw := range properties {
			prop, ok := raw.(map[string]any)
			require.True(t, ok, "%s.%s", tl.Name(), name)
			assert.Contains(t, []string{"string", "integer", "boolean"}, prop["type"], "%s.%s", tl.Name(), name)
			assert.NotEmpty(t, prop["description"], "%s.%s", tl.Name(), name)
			assert.NotContains(t, strings.ToLower(name), "server", "no tool takes a server argument")
		}
		if required, ok := schema["required"].([]string); ok {
			for _, name := range required {
				assert.Contains(t, properties, name, tl.Name())
			}
		}
		_, err := json.Marshal(schema)
		assert.NoError(t, err)

		// every call gets its own copy
		schema["type"] = "changed"
		assert.Equal(t, "object", tl.Parameters()["type"])
	}
}

func TestListDirectory(t *testing.T) {
	_, ec := newServer(t, map[string]string{"server.properties": "motd=x", "plugins/a.jar": "a", "logs/": ""})

	res, err := run(t, ec, "list_directory", map[string]any{})
	require.NoError(t, err)
	out := output(t, res)
	assert.Equal(t, ".", out["path"])
	assert.EqualValues(t, 3, out["total"])
	assert.Equal(t, false, out["truncated"])
	entries := out["entries"].([]any)
	assert.Equal(t, "logs", entries[0].(map[string]any)["path"])
	assert.Equal(t, "server.properties", entries[2].(map[string]any)["name"])
	assert.Equal(t, "Listed the server folder (3 entries)", res.Summary)
	assert.Empty(t, res.Changed)

	res, err = run(t, ec, "list_directory", map[string]any{"path": "plugins"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, output(t, res)["total"])

	summary, err := tool(t, "list_directory").Describe(context.Background(), ec, map[string]any{"path": "./plugins/"})
	require.NoError(t, err)
	assert.Equal(t, toolkit.Summary{Text: "List plugins", Paths: []string{"plugins"}}, summary)

	_, err = run(t, ec, "list_directory", map[string]any{"path": "server.properties"})
	assert.ErrorIs(t, err, workspace.ErrNotDir)
	_, err = run(t, ec, "list_directory", map[string]any{"path": 5})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
}

func TestListDirectory_CapsTheEntries(t *testing.T) {
	content := map[string]string{}
	for i := 0; i <= security.Limits.MaxListEntries; i++ {
		content[fmt.Sprintf("many/f%05d", i)] = ""
	}
	_, ec := newServer(t, content)
	res, err := run(t, ec, "list_directory", map[string]any{"path": "many"})
	require.NoError(t, err)
	out := output(t, res)
	assert.EqualValues(t, security.Limits.MaxListEntries+1, out["total"])
	assert.Equal(t, true, out["truncated"])
	assert.Len(t, out["entries"], security.Limits.MaxListEntries)
}

func TestReadFile(t *testing.T) {
	_, ec := newServer(t, map[string]string{
		"server.properties": "line1\nline2\r\nline3\nline4",
		"empty.txt":         "",
		"world/level.dat":   "\x00\x01binary",
	})

	res, err := run(t, ec, "read_file", map[string]any{"path": "server.properties"})
	require.NoError(t, err)
	out := output(t, res)
	assert.Equal(t, "line1\nline2\r\nline3\nline4", out["content"], "the content comes back byte for byte")
	assert.EqualValues(t, 4, out["totalLines"])
	assert.EqualValues(t, 1, out["startLine"])
	assert.EqualValues(t, 4, out["endLine"])
	assert.Equal(t, false, out["truncated"])
	assert.EqualValues(t, 24, out["size"])
	assert.Equal(t, "Read server.properties (24 bytes)", res.Summary)

	res, err = run(t, ec, "read_file", map[string]any{"path": "server.properties", "offset": 2.0, "limit": "2"})
	require.NoError(t, err)
	out = output(t, res)
	assert.Equal(t, "line2\r\nline3\n", out["content"])
	assert.EqualValues(t, 2, out["startLine"])
	assert.EqualValues(t, 3, out["endLine"])
	assert.Equal(t, true, out["truncated"])

	res, err = run(t, ec, "read_file", map[string]any{"path": "empty.txt"})
	require.NoError(t, err)
	assert.Equal(t, "", output(t, res)["content"])

	_, err = run(t, ec, "read_file", map[string]any{"path": "server.properties", "offset": 9})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
	_, err = run(t, ec, "read_file", map[string]any{"path": "server.properties", "offset": 0})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
	_, err = run(t, ec, "read_file", map[string]any{"path": "server.properties", "limit": 1.5})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
	_, err = run(t, ec, "read_file", map[string]any{"path": "world/level.dat"})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
	assert.Contains(t, err.Error(), "binary")
	_, err = run(t, ec, "read_file", map[string]any{"path": "world"})
	assert.ErrorIs(t, err, workspace.ErrIsDir)
	_, err = run(t, ec, "read_file", map[string]any{"path": "missing.txt"})
	assert.ErrorIs(t, err, toolkit.ErrNotFound)
	_, err = run(t, ec, "read_file", map[string]any{})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
}

func TestReadFile_RefusesFilesOverTheLimit(t *testing.T) {
	_, ec := newServer(t, map[string]string{"big.log": strings.Repeat("x", int(security.Limits.MaxReadBytes)+1)})
	_, err := run(t, ec, "read_file", map[string]any{"path": "big.log"})
	assert.ErrorIs(t, err, toolkit.ErrTooLarge)
	assert.Contains(t, err.Error(), "search_files")
}

func TestFileInfo(t *testing.T) {
	_, ec := newServer(t, map[string]string{"eula.txt": "eula=true", "mods/": ""})

	res, err := run(t, ec, "file_info", map[string]any{"path": "eula.txt"})
	require.NoError(t, err)
	out := output(t, res)
	assert.Equal(t, "file", out["type"])
	assert.EqualValues(t, 9, out["size"])
	assert.Equal(t, "eula.txt", out["name"])

	res, err = run(t, ec, "file_info", map[string]any{"path": "mods"})
	require.NoError(t, err)
	assert.Equal(t, "folder", output(t, res)["type"])

	res, err = run(t, ec, "file_info", map[string]any{"path": "."})
	require.NoError(t, err)
	assert.Equal(t, ".", output(t, res)["path"])

	_, err = run(t, ec, "file_info", map[string]any{"path": "nope"})
	assert.ErrorIs(t, err, toolkit.ErrNotFound)
}

func TestSearchFiles(t *testing.T) {
	_, ec := newServer(t, map[string]string{
		"server.properties":      "motd=A Minecraft Server\nmax-players=20\n",
		"config/paper-world.yml": "motd: not this one\n",
		"logs/latest.log":        "done\n",
	})

	res, err := run(t, ec, "search_files", map[string]any{"query": "paper"})
	require.NoError(t, err)
	out := output(t, res)
	assert.Equal(t, []any{map[string]any{"path": "config/paper-world.yml"}}, out["matches"])
	assert.Equal(t, false, out["truncated"])

	res, err = run(t, ec, "search_files", map[string]any{"query": "MOTD", "in_content": true})
	require.NoError(t, err)
	out = output(t, res)
	assert.EqualValues(t, 2, out["count"])
	first := out["matches"].([]any)[0].(map[string]any)
	assert.Equal(t, "server.properties", first["path"])
	assert.EqualValues(t, 1, first["line"])
	assert.Equal(t, "motd=A Minecraft Server", first["text"])
	assert.Equal(t, `Searched for "MOTD" (2 matches)`, res.Summary)

	res, err = run(t, ec, "search_files", map[string]any{"query": "motd", "in_content": "true", "path": "config", "max_results": 1})
	require.NoError(t, err)
	out = output(t, res)
	assert.EqualValues(t, 1, out["count"])
	assert.Equal(t, true, out["truncated"])

	summary, err := tool(t, "search_files").Describe(context.Background(), ec, map[string]any{"query": "x", "in_content": true})
	require.NoError(t, err)
	assert.Equal(t, `Search file contents for "x" in the server folder`, summary.Text)

	for _, bad := range []map[string]any{
		{},
		{"query": " "},
		{"query": strings.Repeat("q", security.Limits.MaxQueryLength+1)},
		{"query": "x", "in_content": "maybe"},
		{"query": "x", "max_results": -1},
		{"query": "x", "path": "../other"},
	} {
		_, err = run(t, ec, "search_files", bad)
		assert.ErrorIs(t, err, toolkit.ErrInvalidArgs, "%v", bad)
	}
}

func TestCreateFile(t *testing.T) {
	dir, ec := newServer(t, map[string]string{"existing.txt": "keep"})

	res, err := run(t, ec, "create_file", map[string]any{"path": "plugins/Foo/config.yml", "content": "enabled: true\n"})
	require.NoError(t, err)
	assert.Equal(t, "enabled: true\n", readFileOnHost(t, filepath.Join(dir, "plugins", "Foo", "config.yml")))
	assert.Equal(t, []string{"plugins/Foo/config.yml"}, res.Changed)
	assert.Equal(t, "Created plugins/Foo/config.yml (14 bytes)", res.Summary)

	_, err = run(t, ec, "create_file", map[string]any{"path": "empty.txt", "content": ""})
	require.NoError(t, err)
	assert.Equal(t, "", readFileOnHost(t, filepath.Join(dir, "empty.txt")))

	// Describe refuses, and Run on its own refuses too (O_EXCL)
	_, err = run(t, ec, "create_file", map[string]any{"path": "existing.txt", "content": "new"})
	assert.ErrorIs(t, err, toolkit.ErrExists)
	_, err = tool(t, "create_file").Run(context.Background(), ec, map[string]any{"path": "existing.txt", "content": "new"})
	assert.ErrorIs(t, err, toolkit.ErrExists)
	assert.Equal(t, "keep", readFileOnHost(t, filepath.Join(dir, "existing.txt")))

	_, err = run(t, ec, "create_file", map[string]any{"path": "x.txt"})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs, "content is required")
	_, err = run(t, ec, "create_file", map[string]any{"path": "big.txt", "content": strings.Repeat("x", int(security.Limits.MaxWriteBytes)+1)})
	assert.ErrorIs(t, err, toolkit.ErrTooLarge)
	_, err = os.Stat(filepath.Join(dir, "big.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestCreateDirectory(t *testing.T) {
	dir, ec := newServer(t, map[string]string{"file": "x", "plugins/": ""})

	res, err := run(t, ec, "create_directory", map[string]any{"path": "a/b"})
	require.NoError(t, err)
	info, err := os.Stat(filepath.Join(dir, "a", "b"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, []string{"a/b"}, res.Changed)

	res, err = run(t, ec, "create_directory", map[string]any{"path": "plugins"})
	require.NoError(t, err)
	assert.Equal(t, false, output(t, res)["created"])
	assert.Empty(t, res.Changed)

	_, err = run(t, ec, "create_directory", map[string]any{"path": "file"})
	assert.ErrorIs(t, err, toolkit.ErrExists)
}

func TestCopyPath(t *testing.T) {
	dir, ec := newServer(t, map[string]string{"world/level.dat": "level", "world/region/r.mca": "r", "a.txt": "a"})
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(dir, "world", "passwd")))

	res, err := run(t, ec, "copy_path", map[string]any{"source": "world", "destination": "backups/world"})
	require.NoError(t, err)
	assert.Equal(t, "level", readFileOnHost(t, filepath.Join(dir, "backups", "world", "level.dat")))
	assert.Equal(t, "r", readFileOnHost(t, filepath.Join(dir, "backups", "world", "region", "r.mca")))
	_, err = os.Lstat(filepath.Join(dir, "backups", "world", "passwd"))
	assert.True(t, os.IsNotExist(err), "symlinks in a copied tree are skipped")
	assert.Equal(t, []string{"backups/world"}, res.Changed)

	summary, err := tool(t, "copy_path").Describe(context.Background(), ec, map[string]any{"source": "a.txt", "destination": "b.txt"})
	require.NoError(t, err)
	assert.Equal(t, "Copy the file a.txt to b.txt", summary.Text)

	_, err = run(t, ec, "copy_path", map[string]any{"source": "a.txt", "destination": "world/level.dat"})
	assert.ErrorIs(t, err, toolkit.ErrExists)
	_, err = tool(t, "copy_path").Run(context.Background(), ec, map[string]any{"source": "a.txt", "destination": "world/level.dat"})
	assert.ErrorIs(t, err, toolkit.ErrExists)
	assert.Equal(t, "level", readFileOnHost(t, filepath.Join(dir, "world", "level.dat")))
	_, err = run(t, ec, "copy_path", map[string]any{"source": "world", "destination": "world/copy"})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
	_, err = run(t, ec, "copy_path", map[string]any{"source": "a.txt", "destination": "a.txt"})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
}

func TestEditFile(t *testing.T) {
	dir, ec := newServer(t, map[string]string{
		"server.properties": "motd=Old\nmax-players=20\npvp=true\n",
		"twice.yml":         "a: 1\nb: 1\n",
		"windows.cfg":       "first=1\r\nsecond=2\r\n",
	})
	props := filepath.Join(dir, "server.properties")

	summary, err := tool(t, "edit_file").Describe(context.Background(), ec, map[string]any{"path": "server.properties", "old_text": "motd=Old", "new_text": "motd=New"})
	require.NoError(t, err)
	assert.Equal(t, toolkit.Summary{Text: "Replace 1 occurrence in server.properties", Paths: []string{"server.properties"}}, summary)
	assert.Equal(t, "motd=Old\nmax-players=20\npvp=true\n", readFileOnHost(t, props), "Describe changes nothing")

	res, err := tool(t, "edit_file").Run(context.Background(), ec, map[string]any{"path": "server.properties", "old_text": "motd=Old", "new_text": "motd=New"})
	require.NoError(t, err)
	assert.Equal(t, "motd=New\nmax-players=20\npvp=true\n", readFileOnHost(t, props))
	assert.EqualValues(t, 1, output(t, res)["replaced"])
	assert.Equal(t, []string{"server.properties"}, res.Changed)

	// 0 occurrences
	_, err = run(t, ec, "edit_file", map[string]any{"path": "server.properties", "old_text": "motd=Old", "new_text": "x"})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
	assert.Contains(t, err.Error(), "not found")

	// 2 occurrences while 1 is expected
	_, err = run(t, ec, "edit_file", map[string]any{"path": "twice.yml", "old_text": ": 1", "new_text": ": 2"})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
	assert.Contains(t, err.Error(), "appears 2 times")
	assert.Equal(t, "a: 1\nb: 1\n", readFileOnHost(t, filepath.Join(dir, "twice.yml")))

	// 1 occurrence while 2 are expected
	_, err = run(t, ec, "edit_file", map[string]any{"path": "server.properties", "old_text": "pvp=true", "new_text": "pvp=false", "expected_occurrences": 2})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)

	res, err = run(t, ec, "edit_file", map[string]any{"path": "twice.yml", "old_text": ": 1", "new_text": ": 2", "expected_occurrences": 2.0})
	require.NoError(t, err)
	assert.Equal(t, "a: 2\nb: 2\n", readFileOnHost(t, filepath.Join(dir, "twice.yml")))
	assert.EqualValues(t, 2, output(t, res)["replaced"])
	assert.Equal(t, "Replaced 2 occurrences in twice.yml", res.Summary)

	// lines written with \n still match a file with Windows line endings
	_, err = run(t, ec, "edit_file", map[string]any{"path": "windows.cfg", "old_text": "first=1\nsecond=2", "new_text": "first=1\nsecond=3"})
	require.NoError(t, err)
	assert.Equal(t, "first=1\r\nsecond=3\r\n", readFileOnHost(t, filepath.Join(dir, "windows.cfg")))

	for _, bad := range []map[string]any{
		{"path": "server.properties", "old_text": "", "new_text": "x"},
		{"path": "server.properties", "old_text": "pvp", "new_text": "pvp"},
		{"path": "server.properties", "old_text": "pvp"},
		{"path": "server.properties", "old_text": "pvp", "new_text": "x", "expected_occurrences": 0},
		{"path": "../server.properties", "old_text": "pvp", "new_text": "x"},
	} {
		_, err = run(t, ec, "edit_file", bad)
		assert.ErrorIs(t, err, toolkit.ErrInvalidArgs, "%v", bad)
	}
	_, err = run(t, ec, "edit_file", map[string]any{"path": "missing", "old_text": "a", "new_text": "b"})
	assert.ErrorIs(t, err, toolkit.ErrNotFound)
}

func TestEditFile_RefusesAResultOverTheLimit(t *testing.T) {
	size := int(security.Limits.MaxEditBytes) - 10
	_, ec := newServer(t, map[string]string{"big.txt": strings.Repeat("a", size-1) + "X"})
	_, err := run(t, ec, "edit_file", map[string]any{"path": "big.txt", "old_text": "X", "new_text": strings.Repeat("y", 100)})
	assert.ErrorIs(t, err, toolkit.ErrTooLarge)
}

func TestWriteFile(t *testing.T) {
	dir, ec := newServer(t, map[string]string{"ops.json": "[]", "dir/": ""})

	summary, err := tool(t, "write_file").Describe(context.Background(), ec, map[string]any{"path": "ops.json", "content": "[1]"})
	require.NoError(t, err)
	assert.Equal(t, "Overwrite ops.json (2 bytes now, 3 bytes after)", summary.Text)

	res, err := run(t, ec, "write_file", map[string]any{"path": "ops.json", "content": "[1]"})
	require.NoError(t, err)
	assert.Equal(t, "[1]", readFileOnHost(t, filepath.Join(dir, "ops.json")))
	assert.Equal(t, []string{"ops.json"}, res.Changed)

	_, err = run(t, ec, "write_file", map[string]any{"path": "new.json", "content": "{}"})
	assert.ErrorIs(t, err, toolkit.ErrNotFound, "write_file does not create files")
	assert.Contains(t, err.Error(), "create_file")
	_, err = os.Stat(filepath.Join(dir, "new.json"))
	assert.True(t, os.IsNotExist(err))

	_, err = run(t, ec, "write_file", map[string]any{"path": "dir", "content": "x"})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
	_, err = run(t, ec, "write_file", map[string]any{"path": "ops.json", "content": strings.Repeat("x", int(security.Limits.MaxWriteBytes)+1)})
	assert.ErrorIs(t, err, toolkit.ErrTooLarge)
}

func TestMovePath(t *testing.T) {
	dir, ec := newServer(t, map[string]string{"old.txt": "o", "taken.txt": "t", "world/level.dat": "l"})

	res, err := run(t, ec, "move_path", map[string]any{"source": "old.txt", "destination": "archive/2024/old.txt"})
	require.NoError(t, err)
	assert.Equal(t, "o", readFileOnHost(t, filepath.Join(dir, "archive", "2024", "old.txt")))
	assert.Equal(t, []string{"old.txt", "archive/2024/old.txt"}, res.Changed)

	summary, err := tool(t, "move_path").Describe(context.Background(), ec, map[string]any{"source": "world", "destination": "worlds/main"})
	require.NoError(t, err)
	assert.Equal(t, "Move the folder world to worlds/main", summary.Text)

	_, err = run(t, ec, "move_path", map[string]any{"source": "taken.txt", "destination": "world/level.dat"})
	assert.ErrorIs(t, err, toolkit.ErrExists)
	_, err = tool(t, "move_path").Run(context.Background(), ec, map[string]any{"source": "taken.txt", "destination": "world/level.dat"})
	assert.ErrorIs(t, err, toolkit.ErrExists)
	assert.Equal(t, "l", readFileOnHost(t, filepath.Join(dir, "world", "level.dat")))
	_, err = run(t, ec, "move_path", map[string]any{"source": "gone.txt", "destination": "x.txt"})
	assert.ErrorIs(t, err, toolkit.ErrNotFound)
	_, err = run(t, ec, "move_path", map[string]any{"source": "world", "destination": "world/inner"})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
}

func TestRenamePath(t *testing.T) {
	dir, ec := newServer(t, map[string]string{"plugins/Old.jar": "j", "plugins/Taken.jar": "t"})

	summary, err := tool(t, "rename_path").Describe(context.Background(), ec, map[string]any{"path": "plugins/Old.jar", "new_name": "Old.jar.disabled"})
	require.NoError(t, err)
	assert.Equal(t, "Rename the file plugins/Old.jar to Old.jar.disabled", summary.Text)

	res, err := run(t, ec, "rename_path", map[string]any{"path": "plugins/Old.jar", "new_name": " Old.jar.disabled "})
	require.NoError(t, err)
	assert.Equal(t, "j", readFileOnHost(t, filepath.Join(dir, "plugins", "Old.jar.disabled")))
	assert.Equal(t, "plugins/Old.jar.disabled", output(t, res)["newPath"])

	for _, name := range []string{"", "a/b", `a\b`, "..", ".", "../escape", "x\x00y", strings.Repeat("n", 256), "Old.jar.disabled"} {
		_, err = run(t, ec, "rename_path", map[string]any{"path": "plugins/Old.jar.disabled", "new_name": name})
		assert.ErrorIs(t, err, toolkit.ErrInvalidArgs, "%q", name)
	}
	_, err = run(t, ec, "rename_path", map[string]any{"path": "plugins/Old.jar.disabled", "new_name": "Taken.jar"})
	assert.ErrorIs(t, err, toolkit.ErrExists)
	assert.Equal(t, "t", readFileOnHost(t, filepath.Join(dir, "plugins", "Taken.jar")))
}

func TestDeletePath(t *testing.T) {
	dir, ec := newServer(t, map[string]string{"a.txt": "a", "world/region/r.mca": "r"})

	summary, err := tool(t, "delete_path").Describe(context.Background(), ec, map[string]any{"path": "world"})
	require.NoError(t, err)
	assert.Equal(t, "Delete the folder world and everything in it", summary.Text)

	res, err := run(t, ec, "delete_path", map[string]any{"path": "world"})
	require.NoError(t, err)
	assert.Equal(t, []string{"world"}, res.Changed)
	_, err = os.Stat(filepath.Join(dir, "world"))
	assert.True(t, os.IsNotExist(err))

	_, err = run(t, ec, "delete_path", map[string]any{"path": "a.txt"})
	require.NoError(t, err)

	for _, root := range []string{"", ".", "./", " . "} {
		_, err = run(t, ec, "delete_path", map[string]any{"path": root})
		assert.ErrorIs(t, err, toolkit.ErrInvalidArgs, "%q", root)
		assert.Contains(t, err.Error(), "server folder itself")
	}
	_, err = run(t, ec, "delete_path", map[string]any{})
	assert.ErrorIs(t, err, toolkit.ErrInvalidArgs)
	_, err = run(t, ec, "delete_path", map[string]any{"path": "missing"})
	assert.ErrorIs(t, err, toolkit.ErrNotFound)
	_, err = os.Stat(dir)
	assert.NoError(t, err, "the server folder is still there")
}

func TestProtectedPathsAreRefusedByTheTools(t *testing.T) {
	security.ProtectedPaths = []string{"secret"}
	t.Cleanup(func() { security.ProtectedPaths = nil })
	_, ec := newServer(t, map[string]string{"secret/key.txt": "k", "a.txt": "a"})

	for name, args := range map[string]map[string]any{
		"read_file":   {"path": "secret/key.txt"},
		"delete_path": {"path": "secret"},
		"copy_path":   {"source": "secret", "destination": "leak"},
		"move_path":   {"source": "a.txt", "destination": "secret/a.txt"},
		"write_file":  {"path": "secret/key.txt", "content": "x"},
	} {
		_, err := run(t, ec, name, args)
		assert.ErrorIs(t, err, toolkit.ErrProtected, name)
	}
	res, err := run(t, ec, "list_directory", map[string]any{})
	require.NoError(t, err)
	assert.EqualValues(t, 1, output(t, res)["total"])
}

func TestToolsWithoutAWorkspaceFail(t *testing.T) {
	for _, tl := range Tools() {
		args := map[string]any{"path": "a.txt", "source": "a.txt", "destination": "b.txt", "query": "q", "content": "c", "old_text": "o", "new_text": "n", "new_name": "b.txt"}
		_, err := tl.Run(context.Background(), toolkit.ExecContext{}, args)
		assert.Error(t, err, tl.Name())
	}
}

// snapshot records every entry below dir without following links, to prove nothing changed
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		switch {
		case d.Type()&os.ModeSymlink != 0:
			target, _ := os.Readlink(p)
			out[rel] = "link:" + target
		case d.IsDir():
			out[rel] = "dir"
		default:
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out[rel] = "file:" + string(data)
		}
		return nil
	})
	require.NoError(t, err)
	return out
}

var secrets = []string{"B-SECRET", "B-LEVEL", "HOST-SECRET"}

// bMarker stands for server B's host folder in trickPaths
const bMarker = "{B}"

// trickPaths are path arguments a model could use to reach outside server A
func trickPaths() []string {
	return []string{
		"../b/secret.txt",
		"../b",
		"..",
		"../",
		"a/../../b/secret.txt",
		"./../b/secret.txt",
		`..\b\secret.txt`,
		`..\..\b`,
		bMarker + "/secret.txt",
		bMarker,
		"/etc/passwd",
		"/",
		`C:\b\secret.txt`,
		"own.txt/../../b/secret.txt",
		"%2e%2e/b/secret.txt",
		"toB",
		"toB/secret.txt",
		"toB/world/level.dat",
		"toBfile",
		"toBrel",
		"toBrel/secret.txt",
		"toHost",
		"toHost/passwd",
		"toHostFile",
		"own.txt\x00../../b/secret.txt",
	}
}

type toolCall struct {
	tool string
	args map[string]any
}

// callsFor builds every call of every tool that could use p to read, write, copy, move,
// list, search or delete something
func callsFor(p string) []toolCall {
	return []toolCall{
		{"list_directory", map[string]any{"path": p}},
		{"read_file", map[string]any{"path": p}},
		{"file_info", map[string]any{"path": p}},
		{"search_files", map[string]any{"path": p, "query": "secret"}},
		{"search_files", map[string]any{"path": p, "query": "SECRET", "in_content": true}},
		{"create_file", map[string]any{"path": p, "content": "pwned"}},
		{"create_file", map[string]any{"path": p + "/planted.txt", "content": "pwned"}},
		{"create_directory", map[string]any{"path": p + "/planted"}},
		{"copy_path", map[string]any{"source": p, "destination": "stolen"}},
		{"copy_path", map[string]any{"source": "own.txt", "destination": p}},
		{"copy_path", map[string]any{"source": "own.txt", "destination": p + "/planted.txt"}},
		{"edit_file", map[string]any{"path": p, "old_text": "SECRET", "new_text": "pwned"}},
		{"edit_file", map[string]any{"path": p, "old_text": "LEVEL", "new_text": "pwned"}},
		{"write_file", map[string]any{"path": p, "content": "pwned"}},
		{"move_path", map[string]any{"source": p, "destination": "stolen-moved"}},
		{"move_path", map[string]any{"source": "own.txt", "destination": p}},
		{"move_path", map[string]any{"source": "own.txt", "destination": p + "/planted.txt"}},
		{"rename_path", map[string]any{"path": p, "new_name": "renamed"}},
		{"rename_path", map[string]any{"path": "own.txt", "new_name": p}},
		{"delete_path", map[string]any{"path": p}},
	}
}

// TestServerAIsolatedFromServerB gives server A's tools every path trick, and links inside A
// that point at server B and at a host folder, with both Describe and Run. Nothing of B or
// of the host may be read, listed, found, copied, changed, moved or deleted.
func TestServerAIsolatedFromServerB(t *testing.T) {
	resolvemode.Each(t, checkServerAIsolatedFromServerB)
}

func checkServerAIsolatedFromServerB(t *testing.T) {
	for _, trick := range trickPaths() {
		t.Run(strings.ReplaceAll(trick, "/", "|"), func(t *testing.T) {
			base := t.TempDir()
			aDir, bDir, hostDir := filepath.Join(base, "a"), filepath.Join(base, "b"), filepath.Join(base, "host")
			writeTree(t, aDir, map[string]string{"own.txt": "mine", "plugins/x.yml": "x: 1"})
			writeTree(t, bDir, map[string]string{"secret.txt": "B-SECRET", "world/level.dat": "B-LEVEL"})
			writeTree(t, hostDir, map[string]string{"passwd": "HOST-SECRET"})
			for link, target := range map[string]string{
				"toB":        bDir,
				"toBfile":    filepath.Join(bDir, "secret.txt"),
				"toBrel":     "../b",
				"toHost":     hostDir,
				"toHostFile": filepath.Join(hostDir, "passwd"),
			} {
				require.NoError(t, os.Symlink(target, filepath.Join(aDir, link)))
			}
			p := strings.ReplaceAll(trick, bMarker, bDir)
			ecA := serverContext(t, aDir)
			ecB := serverContext(t, bDir)
			beforeB, beforeHost := snapshot(t, bDir), snapshot(t, hostDir)

			for _, c := range callsFor(p) {
				tl := tool(t, c.tool)
				var texts []string
				if summary, err := tl.Describe(context.Background(), ecA, c.args); err != nil {
					texts = append(texts, err.Error())
				} else {
					texts = append(texts, summary.Text)
				}
				res, err := tl.Run(context.Background(), ecA, c.args)
				if err != nil {
					texts = append(texts, err.Error())
				} else {
					data, jsonErr := json.Marshal(res.Output)
					require.NoError(t, jsonErr)
					texts = append(texts, string(data), res.Summary)
				}
				for _, text := range texts {
					for _, secret := range secrets {
						assert.NotContains(t, text, secret, "%s %v", c.tool, c.args)
					}
					assert.NotContains(t, text, base, "no host path in %s %v", c.tool, c.args)
				}
			}

			assert.Equal(t, beforeB, snapshot(t, bDir), "server B is untouched")
			assert.Equal(t, beforeHost, snapshot(t, hostDir), "the host folder is untouched")
			for rel, entry := range snapshot(t, aDir) {
				for _, secret := range secrets {
					assert.NotContains(t, entry, secret, "nothing of B or the host was copied into A (%s)", rel)
				}
			}

			// B's own workspace still sees its files as they were
			res, err := tool(t, "read_file").Run(context.Background(), ecB, map[string]any{"path": "secret.txt"})
			require.NoError(t, err)
			assert.Equal(t, "B-SECRET", output(t, res)["content"])
		})
	}
}

func TestServerAIsolation_RefusalReasons(t *testing.T) {
	resolvemode.Each(t, checkServerAIsolationRefusalReasons)
}

func checkServerAIsolationRefusalReasons(t *testing.T) {
	base := t.TempDir()
	aDir, bDir := filepath.Join(base, "a"), filepath.Join(base, "b")
	writeTree(t, aDir, map[string]string{"own.txt": "mine"})
	writeTree(t, bDir, map[string]string{"secret.txt": "B-SECRET"})
	require.NoError(t, os.Symlink(bDir, filepath.Join(aDir, "toB")))
	require.NoError(t, os.Symlink(filepath.Join(bDir, "secret.txt"), filepath.Join(aDir, "toBfile")))
	require.NoError(t, os.Symlink("/etc", filepath.Join(aDir, "etc")))
	ec := serverContext(t, aDir)

	for _, p := range []string{"../b/secret.txt", `..\b\secret.txt`, "/etc/passwd", filepath.Join(bDir, "secret.txt"), `C:\x`} {
		_, err := run(t, ec, "read_file", map[string]any{"path": p})
		assert.ErrorIs(t, err, toolkit.ErrOutsideRoot, p)
		assert.ErrorIs(t, err, toolkit.ErrInvalidArgs, p)
	}
	for _, p := range []string{"toB/secret.txt", "toBfile", "etc/passwd", "etc/hostname"} {
		_, err := run(t, ec, "read_file", map[string]any{"path": p})
		assert.ErrorIs(t, err, toolkit.ErrOutsideRoot, p)
		_, err = run(t, ec, "file_info", map[string]any{"path": p})
		assert.ErrorIs(t, err, toolkit.ErrOutsideRoot, p)
	}
	for _, p := range []string{"toB", "etc"} {
		_, err := run(t, ec, "list_directory", map[string]any{"path": p})
		assert.ErrorIs(t, err, toolkit.ErrOutsideRoot, p)
		_, err = run(t, ec, "search_files", map[string]any{"path": p, "query": "secret"})
		assert.ErrorIs(t, err, toolkit.ErrOutsideRoot, p)
	}

	// a server argument is not part of any tool and changes nothing
	_, err := run(t, ec, "read_file", map[string]any{"path": "secret.txt", "server": "b", "serverId": "b"})
	assert.ErrorIs(t, err, toolkit.ErrNotFound)

	// searching A from its root never walks into the links
	res, err := run(t, ec, "search_files", map[string]any{"query": "secret", "in_content": true})
	require.NoError(t, err)
	assert.EqualValues(t, 0, output(t, res)["count"])
	res, err = run(t, ec, "search_files", map[string]any{"query": "secret"})
	require.NoError(t, err)
	var found []string
	for _, m := range output(t, res)["matches"].([]any) {
		found = append(found, m.(map[string]any)["path"].(string))
	}
	sort.Strings(found)
	assert.Empty(t, found, "secret.txt is only in B")

	// deleting the link removes the link only
	_, err = run(t, ec, "delete_path", map[string]any{"path": "toBfile"})
	assert.ErrorIs(t, err, toolkit.ErrOutsideRoot, "Describe refuses to describe what is outside")
	require.NoError(t, ec.Workspace.Delete(context.Background(), "toBfile"))
	assert.Equal(t, "B-SECRET", readFileOnHost(t, filepath.Join(bDir, "secret.txt")))
}
