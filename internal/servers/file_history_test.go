package servers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFileHistoryTestServer(t *testing.T) (*Server, string) {
	root := t.TempDir()
	previous := config.ServersFolder.Value()
	require.NoError(t, config.ServersFolder.Set(root, false))
	t.Cleanup(func() { _ = config.ServersFolder.Set(previous, false) })

	serverDir := filepath.Join(root, "historytest")
	require.NoError(t, os.MkdirAll(serverDir, 0755))
	fs, err := files.NewFileServer(serverDir, os.Getuid(), os.Getgid())
	require.NoError(t, err)
	t.Cleanup(func() { _ = fs.Close() })

	p := CreateProgram()
	p.Identifier = "historytest"
	p.SetFileServer(fs)
	return p, serverDir
}

func writeServerFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0644))
}

// save mimics putFile: keep the old text, then overwrite the file
func save(t *testing.T, p *Server, dir, name, content string) {
	t.Helper()
	p.RecordFileVersion("/" + name)
	writeServerFile(t, dir, name, content)
}

func versionTexts(t *testing.T, p *Server, name string) []string {
	t.Helper()
	versions, err := p.FileVersions(name)
	require.NoError(t, err)
	texts := make([]string, 0, len(versions))
	for _, v := range versions {
		data, err := p.FileVersionContent(name, v.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(len(data)), v.Size)
		texts = append(texts, string(data))
	}
	return texts
}

func TestNormalizeHistoryPath(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"server.properties", "server.properties", true},
		{"/server.properties", "server.properties", true},
		{"//config/./a.yml", "config/a.yml", true},
		{`config\b.yml`, "config/b.yml", true},
		{"", "", false},
		{"/", "", false},
		{"../outside.yml", "", false},
		{"/config/../../etc/passwd", "", false},
		{"config/..", "", false},
		{"a\x00.yml", "", false},
	}
	for _, tt := range tests {
		got, err := NormalizeHistoryPath(tt.in)
		if tt.ok {
			assert.NoError(t, err, tt.in)
			assert.Equal(t, tt.want, got, tt.in)
		} else {
			assert.ErrorIs(t, err, ErrFileHistoryPath, tt.in)
		}
	}
}

func TestRecordFileVersion_KeepsPreviousTextNewestFirst(t *testing.T) {
	p, dir := newFileHistoryTestServer(t)
	writeServerFile(t, dir, "server.properties", "motd=one")

	save(t, p, dir, "server.properties", "motd=two")
	save(t, p, dir, "server.properties", "motd=three")

	assert.Equal(t, []string{"motd=two", "motd=one"}, versionTexts(t, p, "server.properties"))
	// the leading slash from the file route maps to the same history
	assert.Equal(t, []string{"motd=two", "motd=one"}, versionTexts(t, p, "/server.properties"))

	// history lives next to the server folder, never inside it
	_, err := os.Stat(filepath.Join(config.ServersFolder.Value(), "historytest.history"))
	assert.NoError(t, err)
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1)
}

func TestRecordFileVersion_SkipRules(t *testing.T) {
	p, dir := newFileHistoryTestServer(t)

	writeServerFile(t, dir, "world/level.dat", "not a config")
	writeServerFile(t, dir, "big.yml", strings.Repeat("a", int(fileHistoryMaxFileBytes)+1))
	writeServerFile(t, dir, "binary.json", "{\x00}")
	writeServerFile(t, dir, "notutf8", "\xff\xfe")
	writeServerFile(t, dir, "motd", "Welcome")
	writeServerFile(t, dir, ".env", "A=1")

	for _, name := range []string{"world/level.dat", "big.yml", "binary.json", "notutf8", "missing.yml", "motd", ".env"} {
		p.RecordFileVersion(name)
	}

	for name, want := range map[string]int{
		"world/level.dat": 0, // extension not allowed
		"big.yml":         0, // over the size limit
		"binary.json":     0, // NUL byte
		"notutf8":         0, // no extension and not text
		"missing.yml":     0, // nothing to keep yet
		"motd":            1, // text without an extension
		".env":            1,
	} {
		versions, err := p.FileVersions(name)
		assert.NoError(t, err, name)
		assert.Len(t, versions, want, name)
	}

	// unchanged text is not stored twice
	p.RecordFileVersion("motd")
	versions, _ := p.FileVersions("motd")
	assert.Len(t, versions, 1)
}

func TestFileVersions_EmptyAndUnknown(t *testing.T) {
	p, _ := newFileHistoryTestServer(t)

	versions, err := p.FileVersions("never-saved.yml")
	assert.NoError(t, err)
	assert.NotNil(t, versions)
	assert.Empty(t, versions)

	_, err = p.FileVersionContent("never-saved.yml", "abc-123456")
	assert.ErrorIs(t, err, ErrFileVersionNotFound)
	_, err = p.FileVersionContent("never-saved.yml", "../../etc/passwd")
	assert.ErrorIs(t, err, ErrFileVersionNotFound)
	_, err = p.FileVersions("../other/server.properties")
	assert.ErrorIs(t, err, ErrFileHistoryPath)
	assert.ErrorIs(t, p.RestoreFileVersion("../x.yml", "abc-123456"), ErrFileHistoryPath)
}

func TestRecordFileVersion_PrunesPerFile(t *testing.T) {
	p, dir := newFileHistoryTestServer(t)
	previous := fileHistoryKeepPerFile
	fileHistoryKeepPerFile = 3
	t.Cleanup(func() { fileHistoryKeepPerFile = previous })

	writeServerFile(t, dir, "a.yml", "v0")
	for i := 1; i <= 5; i++ {
		save(t, p, dir, "a.yml", fmt.Sprintf("v%d", i))
	}

	assert.Equal(t, []string{"v4", "v3", "v2"}, versionTexts(t, p, "a.yml"))
	folder := fileHistoryFolder(filepath.Join(config.ServersFolder.Value(), "historytest.history"), "a.yml")
	stored, _ := filepath.Glob(filepath.Join(folder, "*.v"))
	assert.Len(t, stored, 3, "dropped versions must be deleted from disk")
}

func TestRecordFileVersion_PrunesPerServer(t *testing.T) {
	p, dir := newFileHistoryTestServer(t)
	previous := fileHistoryMaxServerBytes
	fileHistoryMaxServerBytes = 25
	t.Cleanup(func() { fileHistoryMaxServerBytes = previous })

	// three files of 10 bytes each: only the newest two fit
	for _, name := range []string{"one.yml", "two.yml", "three.yml"} {
		writeServerFile(t, dir, name, "0123456789")
		save(t, p, dir, name, "changed")
	}

	one, _ := p.FileVersions("one.yml")
	two, _ := p.FileVersions("two.yml")
	three, _ := p.FileVersions("three.yml")
	assert.Empty(t, one, "the oldest version goes first")
	assert.Len(t, two, 1)
	assert.Len(t, three, 1)

	_, err := os.Stat(fileHistoryFolder(filepath.Join(config.ServersFolder.Value(), "historytest.history"), "one.yml"))
	assert.True(t, os.IsNotExist(err), "a file left without versions loses its folder")
}

func TestRestoreFileVersion_KeepsCurrentTextFirst(t *testing.T) {
	p, dir := newFileHistoryTestServer(t)
	writeServerFile(t, dir, "config/a.yml", "first")
	save(t, p, dir, "config/a.yml", "second")

	versions, err := p.FileVersions("config/a.yml")
	require.NoError(t, err)
	require.Len(t, versions, 1)

	require.NoError(t, p.RestoreFileVersion("/config/a.yml", versions[0].ID))

	content, err := os.ReadFile(filepath.Join(dir, "config", "a.yml"))
	require.NoError(t, err)
	assert.Equal(t, "first", string(content))
	// the text that was replaced can be restored in turn
	assert.Equal(t, []string{"second", "first"}, versionTexts(t, p, "config/a.yml"))

	assert.ErrorIs(t, p.RestoreFileVersion("config/a.yml", "zzz-000000"), ErrFileVersionNotFound)
}

func TestRestoreFileVersion_OldestVersionSurvivesPruning(t *testing.T) {
	p, dir := newFileHistoryTestServer(t)
	previous := fileHistoryKeepPerFile
	fileHistoryKeepPerFile = 2
	t.Cleanup(func() { fileHistoryKeepPerFile = previous })

	writeServerFile(t, dir, "a.yml", "v0")
	save(t, p, dir, "a.yml", "v1")
	save(t, p, dir, "a.yml", "v2")
	versions, _ := p.FileVersions("a.yml")
	require.Len(t, versions, 2)
	oldest := versions[1].ID

	// keeping "v2" pushes "v0" out of the history, but it is read before that
	require.NoError(t, p.RestoreFileVersion("a.yml", oldest))
	content, _ := os.ReadFile(filepath.Join(dir, "a.yml"))
	assert.Equal(t, "v0", string(content))
}

func TestRecordFileVersion_ConcurrentSaves(t *testing.T) {
	p, dir := newFileHistoryTestServer(t)
	names := []string{"a.yml", "b.json", "c.properties"}
	for _, name := range names {
		writeServerFile(t, dir, name, "start")
	}

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := names[i%len(names)]
			p.RecordFileVersion(name)
			_ = os.WriteFile(filepath.Join(dir, name), []byte(fmt.Sprintf("text %d %d", i, time.Now().UnixNano())), 0644)
		}(i)
	}
	wg.Wait()

	for _, name := range names {
		versions, err := p.FileVersions(name)
		require.NoError(t, err, name)
		assert.NotEmpty(t, versions, name)
		seen := map[string]bool{}
		for _, v := range versions {
			assert.False(t, seen[v.ID], "version ids are unique")
			seen[v.ID] = true
			_, err := p.FileVersionContent(name, v.ID)
			assert.NoError(t, err, "every listed version can be read")
		}
	}
}

func TestDeleteFileHistory(t *testing.T) {
	p, dir := newFileHistoryTestServer(t)
	writeServerFile(t, dir, "a.yml", "v0")
	save(t, p, dir, "a.yml", "v1")

	require.NoError(t, DeleteFileHistory("historytest"))
	_, err := os.Stat(filepath.Join(config.ServersFolder.Value(), "historytest.history"))
	assert.True(t, os.IsNotExist(err))
	assert.Error(t, DeleteFileHistory("../escape"))
}
