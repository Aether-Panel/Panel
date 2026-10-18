package steamgamedl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

// Test_getLatestVersion resolves the DepotDownloader release asset against a
// local HTTP server instead of the real GitHub API. The live endpoint is rate
// limited on shared CI runners (unauthenticated requests return HTTP 403),
// which used to make the suite fail.
func Test_getLatestVersion(t *testing.T) {
	t.Run("resolved dd versions", func(t *testing.T) {
		// getLatestVersion matches the asset name against the current GOOS and
		// either "x64" (amd64) or GOARCH, so build a name that matches here.
		arch := "x64"
		if runtime.GOARCH != "amd64" {
			arch = runtime.GOARCH
		}
		assetName := fmt.Sprintf("DepotDownloader-%s-%s.zip", runtime.GOOS, arch)
		want := "https://example.invalid/" + assetName

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
				t.Errorf("unexpected Accept header: %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `[{"assets":[{"name":%q,"browser_download_url":%q}]}]`, assetName, want)
		}))
		defer server.Close()

		orig := repoReleasesURL
		repoReleasesURL = server.URL
		defer func() { repoReleasesURL = orig }()

		got, err := getLatestVersion()
		if err != nil {
			t.Fatalf("getLatestVersion() error = %v", err)
		}
		if got != want {
			t.Errorf("getLatestVersion() = %q, want %q", got, want)
		}
	})
}
