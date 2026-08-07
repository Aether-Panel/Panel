package githubdl

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/extract"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

// stubServer implements skypanel.DaemonServer using an in-memory FileServer so
// the githubdl -> extract flow can be exercised without Docker or a real node.
type stubServer struct {
	fs   files.FileServer
	root string
}

func (s *stubServer) GetFileServer() files.FileServer { return s.fs }
func (s *stubServer) Extract(source, destination string) error {
	return files.Extract(s.fs, source, destination, "*", false, nil)
}
func (s *stubServer) ArchiveItems(filesToArchive []string, destination string) error {
	return nil
}
func (s *stubServer) DataToMap() map[string]interface{} { return nil }

// TestGithubDl_OutputVariableIsLocalFilename reproduces the install path used by
// the terraria-tshock / terraria-tmodloader templates: githubdl downloads a
// release asset, then a downstream `extract source: "${outputVariable}"` and a
// `rm "${outputVariable}"` reference it as a *local filename*. The operation
// must therefore expose the on-disk basename (grab always saves using the URL
// basename into the server root), never the full download URL.
func TestGithubDl_OutputVariableIsLocalFilename(t *testing.T) {
	// Build a small zip asset with one file inside it.
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	fw, err := zw.Create("server/TerrariaServer")
	if err != nil {
		t.Fatalf("creating zip entry: %v", err)
	}
	if _, err := io.WriteString(fw, "server-binary-content"); err != nil {
		t.Fatalf("writing zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zip: %v", err)
	}
	zipBytes := zipBuf.Bytes()

	const assetName = "TShock-6.1.0-linux-x64-Release.zip"

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/Pryaxis/TShock/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"assets":[{"name":%q,"browser_download_url":%q}]}`,
				assetName, ts.URL+"/download/"+assetName)
		case "/download/" + assetName:
			http.ServeContent(w, r, assetName, time.Time{}, bytes.NewReader(zipBytes))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// Point the operation at the fake GitHub API.
	origBase := githubAPIBase
	githubAPIBase = ts.URL
	defer func() { githubAPIBase = origBase }()

	rootDir := t.TempDir()
	env := &skypanel.Environment{
		Type:           "host",
		RootDirectory:  rootDir,
		ConsoleBuffer:  skypanel.CreateCache(),
		ConsoleTracker: skypanel.CreateTracker(),
	}
	fs, err := files.NewFileServer(rootDir, 0, 0)
	if err != nil {
		t.Fatalf("creating file server: %v", err)
	}
	srv := &stubServer{fs: fs, root: rootDir}
	args := skypanel.RunOperatorArgs{Environment: env, Server: srv}

	op := GithubDl{
		Repository:     "Pryaxis/TShock",
		AssetMatch:     "linux-x64-Release.zip",
		OutputVariable: "tshockAsset",
	}

	res := op.Run(args)
	if res.Error != nil {
		t.Fatalf("githubdl.Run error: %v", res.Error)
	}

	got, ok := res.VariableOverrides["tshockAsset"].(string)
	if !ok {
		t.Fatalf("expected output variable tshockAsset to be set, got: %v", res.VariableOverrides)
	}

	// Must be the local filename (basename), never the download URL. A URL here
	// is exactly the regression that made installs fail.
	if got != assetName {
		t.Fatalf("expected output variable to be local basename %q, got %q", assetName, got)
	}
	if filepath.Dir(got) != "." {
		t.Fatalf("output variable must be a bare filename with no path separators, got %q", got)
	}

	// grab saves the asset into the server root using the URL basename, so the
	// file referenced by the variable must exist on disk there.
	if _, err := os.Stat(filepath.Join(rootDir, got)); err != nil {
		t.Fatalf("downloaded asset %q not found at server root: %v", got, err)
	}

	// Exercise the downstream extract step using the resolved variable. This is
	// the call that previously failed because the variable held a URL.
	extractOp := extract.Extract{Source: got, Destination: "."}
	if r := extractOp.Run(args); r.Error != nil {
		t.Fatalf("extract.Run after githubdl failed: %v", r.Error)
	}
	if _, err := os.Stat(filepath.Join(rootDir, "server", "TerrariaServer")); err != nil {
		t.Fatalf("extracted file not found: %v", err)
	}

	// The template also does `rm "${tshockAsset}"`; confirm that resolves to the
	// local filename and removes the downloaded archive.
	if err := os.Remove(filepath.Join(rootDir, got)); err != nil {
		t.Fatalf("rm of downloaded asset failed: %v", err)
	}
}
