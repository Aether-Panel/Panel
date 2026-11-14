package files

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeZip makes name in dir from entries: a name ending in "/" is a folder entry, the rest
// are files holding their own name. No folder entries are added for the files, as many tools
// (Windows' "Send to compressed folder" among them) leave them out.
func writeZip(t *testing.T, dir, name string, entries ...string) {
	t.Helper()
	out, err := os.Create(filepath.Join(dir, name))
	require.NoError(t, err)
	w := zip.NewWriter(out)
	for _, e := range entries {
		f, err := w.Create(e)
		require.NoError(t, err)
		if e[len(e)-1] != '/' {
			_, err = f.Write([]byte(e))
			require.NoError(t, err)
		}
	}
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())
}

func newExtractServer(t *testing.T) (FileServer, string) {
	root := t.TempDir()
	fs, err := NewFileServer(root, -1, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fs.Close() })
	return fs, root
}

// tree lists every file under root as "path=content", folders as "path/"
func tree(t *testing.T, root string, skip string) []string {
	t.Helper()
	var out []string
	require.NoError(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		require.NoError(t, err)
		rel, _ := filepath.Rel(root, path)
		if rel == "." || rel == skip {
			return nil
		}
		if d.IsDir() {
			out = append(out, rel+"/")
			return nil
		}
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		out = append(out, rel+"="+string(data))
		return nil
	}))
	return out
}

func TestExtract_SingleFileArchiveIsNotEmptiedBySkipRoot(t *testing.T) {
	for _, skipRoot := range []bool{true, false} {
		fs, root := newExtractServer(t)
		writeZip(t, root, "plugin.zip", "plugin.jar")
		require.NoError(t, ExtractLimited(fs, "plugin.zip", ".", "*", skipRoot, nil))
		assert.Equal(t, []string{"plugin.jar=plugin.jar"}, tree(t, root, "plugin.zip"), "skipRoot=%v", skipRoot)
	}
}

func TestExtract_SkipRootStillStripsALoneFolder(t *testing.T) {
	fs, root := newExtractServer(t)
	writeZip(t, root, "repo.zip", "repo-main/", "repo-main/a.txt", "repo-main/src/b.txt")
	require.NoError(t, ExtractLimited(fs, "repo.zip", ".", "*", true, nil))
	assert.Equal(t, []string{"a.txt=repo-main/a.txt", "src/", "src/b.txt=repo-main/src/b.txt"}, tree(t, root, "repo.zip"))
}

func TestExtract_AsPackedKeepsTheFolder(t *testing.T) {
	// what the file manager's unarchive does: the archive's layout lands as it is
	fs, root := newExtractServer(t)
	require.NoError(t, os.Mkdir(filepath.Join(root, "uploads"), 0755))
	writeZip(t, filepath.Join(root, "uploads"), "plugins.zip", "plugins/a.jar", "plugins/cfg/x.yml", "readme.txt")
	require.NoError(t, ExtractLimited(fs, "uploads/plugins.zip", "uploads", "*", false, nil))
	assert.Equal(t, []string{
		"plugins/", "plugins/a.jar=plugins/a.jar", "plugins/cfg/", "plugins/cfg/x.yml=plugins/cfg/x.yml",
		"readme.txt=readme.txt",
	}, tree(t, filepath.Join(root, "uploads"), "plugins.zip"))
}

func TestExtract_LooseFilesAndAFolderAreNotSingleRoot(t *testing.T) {
	fs, root := newExtractServer(t)
	writeZip(t, root, "mixed.zip", "config/a.yml", "server.jar")
	require.NoError(t, ExtractLimited(fs, "mixed.zip", ".", "*", true, nil))
	assert.Equal(t, []string{"config/", "config/a.yml=config/a.yml", "server.jar=server.jar"}, tree(t, root, "mixed.zip"))
}
