package servers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// githubTarball is an archive shaped like GitHub's: a global header with the commit, then one
// folder named owner-repo-commit holding the branch
func githubTarball(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": "abc123"}}))
	require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "octo-app-abc123/", Mode: 0o755}))
	for name, content := range entries {
		require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "octo-app-abc123/" + name, Mode: 0o644, Size: int64(len(content))}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// fakeGitHubArchive serves one archive at the short-lived address GitHub's API hands the panel,
// and lets the node fetch from it; it returns that address
func fakeGitHubArchive(t *testing.T, archive []byte) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/octo/app/legacy.tar.gz/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "short-lived" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(archive)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	previous := githubDownloadAllowed
	githubDownloadAllowed = func(u *url.URL) bool { return u.Host == strings.TrimPrefix(srv.URL, "http://") }
	t.Cleanup(func() { githubDownloadAllowed = previous })
	return srv.URL + "/octo/app/legacy.tar.gz/refs/heads/dev?token=short-lived"
}

// newGitHubTestServer is the backup test server with the status stream an install announces
// itself on
func newGitHubTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	p, dir := newBackupTestServer(t)
	p.RunningEnvironment.StatusTracker = skypanel.CreateTracker()
	return p, dir
}

func waitForGitHubInstall(t *testing.T, p *Server) GitHubInstallStatus {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if status := p.GitHubInstallStatus(); !status.Installing {
			return status
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the install did not finish")
	return GitHubInstallStatus{}
}

func TestInstallFromGitHub_ReplacesTheFilesWithTheBranch(t *testing.T) {
	p, dir := newGitHubTestServer(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "old.txt"), []byte("old"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "node_modules", "x"), 0o755))
	address := fakeGitHubArchive(t, githubTarball(t, map[string]string{
		"package.json": `{"name":"app"}`,
		"src/index.js": "console.log('hi')",
	}))

	require.NoError(t, p.InstallFromGitHub(GitHubInstallRequest{Repository: "octo/app", Branch: "dev", URL: address}))
	status := waitForGitHubInstall(t, p)
	require.Empty(t, status.LastError)
	assert.Equal(t, GitHubStepDone, status.Step)
	assert.Equal(t, "octo/app", status.Repository)
	assert.Positive(t, status.Downloaded)

	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	require.NoError(t, err)
	assert.Equal(t, `{"name":"app"}`, string(data), "the branch is at the root, without GitHub's folder")
	assert.FileExists(t, filepath.Join(dir, "src", "index.js"))
	assert.NoFileExists(t, filepath.Join(dir, "old.txt"), "the folder was emptied first")
	assert.NoDirExists(t, filepath.Join(dir, "node_modules"))
	leftovers, _ := filepath.Glob(filepath.Join(p.githubArchiveFolder(), ".aether-github-*"))
	assert.Empty(t, leftovers, "the download is removed")
	assert.NoFileExists(t, filepath.Join(dir, "pax_global_header"))

	restorePoints, err := filepath.Glob(filepath.Join(p.GetBackupDirectory(), RestorePointPrefix+RestoreReasonGitHub+"-*"))
	require.NoError(t, err)
	assert.Len(t, restorePoints, 1, "a restore point keeps the old files")
}

func TestInstallFromGitHub_AFailedDownloadChangesNothing(t *testing.T) {
	p, dir := newGitHubTestServer(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep"), 0o644))
	address := fakeGitHubArchive(t, githubTarball(t, map[string]string{"a.txt": "a"}))

	expired := strings.Replace(address, "short-lived", "expired", 1)
	require.NoError(t, p.InstallFromGitHub(GitHubInstallRequest{Repository: "octo/app", Branch: "dev", URL: expired}))
	status := waitForGitHubInstall(t, p)
	assert.Equal(t, GitHubStepFailed, status.Step)
	assert.Contains(t, status.LastError, "refused")
	assert.NotContains(t, status.LastError, "expired", "the address's token never reaches the console")
	assert.FileExists(t, filepath.Join(dir, "keep.txt"))
	restorePoints, _ := filepath.Glob(filepath.Join(p.GetBackupDirectory(), RestorePointPrefix+"*"))
	assert.Empty(t, restorePoints, "nothing happened before the download")
}

func TestInstallFromGitHub_OtherJobsWait(t *testing.T) {
	p, _ := newGitHubTestServer(t)
	p.githubState().running.Store(true)
	t.Cleanup(func() { p.githubState().running.Store(false) })

	assert.ErrorIs(t, p.InstallModpack(ModpackInstallRequest{Source: ModpackSourceModrinth, ProjectID: "abc", VersionID: "def"}), ErrGitHubInstalling)
	assert.ErrorIs(t, p.Install(), ErrGitHubInstalling)
	assert.ErrorIs(t, p.InstallFromGitHub(GitHubInstallRequest{Repository: "octo/app", Branch: "dev", URL: "https://codeload.github.com/x"}), ErrGitHubInstalling)
}

func TestInstallFromGitHub_RefusesBadRequests(t *testing.T) {
	p, _ := newGitHubTestServer(t)
	good := "https://codeload.github.com/octo/app/legacy.tar.gz/refs/heads/main?token=x"
	for _, req := range []GitHubInstallRequest{
		{Repository: "octo/app", Branch: "main", URL: "https://evil.example/archive.tar.gz"},
		{Repository: "octo/app", Branch: "main", URL: "http://codeload.github.com/octo/app"},
		{Repository: "../etc/app", Branch: "main", URL: good},
		{Repository: "octo/..", Branch: "main", URL: good},
		{Repository: "octo", Branch: "main", URL: good},
		{Repository: "octo/app", Branch: "a..b", URL: good},
		{Repository: "octo/app", Branch: "has space", URL: good},
		{Repository: "octo/app", Branch: "", URL: good},
	} {
		assert.ErrorIs(t, p.InstallFromGitHub(req), ErrGitHubBadRequest, "%+v", req)
	}
}

func TestEmptyServerFolder_RemovesEverything(t *testing.T) {
	fsys, root := mpNewFS(t)
	mpWriteFile(t, root, "a.txt", "a")
	mpWriteFile(t, root, "dir/b.txt", "b")
	mpWriteFile(t, root, ".env", "SECRET=1")
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(root, "outside-link")))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "nested"), 0o755))
	require.NoError(t, os.Symlink("/etc", filepath.Join(root, "nested", "dir-link")))

	require.NoError(t, emptyServerFolder(fsys))
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.FileExists(t, "/etc/passwd", "links are removed, never what they point at")
}
