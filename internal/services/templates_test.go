package services

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestRepoAuth_PublicRepo(t *testing.T) {
	auth, err := repoAuth(&models.TemplateRepo{URL: "https://github.com/org/templates.git"})
	require.NoError(t, err)
	assert.Nil(t, auth)
}

func TestRepoAuth_HTTP(t *testing.T) {
	auth, err := repoAuth(&models.TemplateRepo{URL: "https://github.com/org/private.git", PAT: "ghp_token"})
	require.NoError(t, err)
	assert.Equal(t, &githttp.BasicAuth{Username: "git", Password: "ghp_token"}, auth)

	auth, err = repoAuth(&models.TemplateRepo{URL: "https://bitbucket.org/org/private.git", Username: "me", PAT: "app-password"})
	require.NoError(t, err)
	assert.Equal(t, &githttp.BasicAuth{Username: "me", Password: "app-password"}, auth)

	auth, err = repoAuth(&models.TemplateRepo{URL: "https://git.example.com/private.git", Username: "me", Password: "secret"})
	require.NoError(t, err)
	assert.Equal(t, &githttp.BasicAuth{Username: "me", Password: "secret"}, auth)
}

func TestRepoAuth_SSH(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(key, "")
	require.NoError(t, err)
	pemKey := string(pem.EncodeToMemory(block))

	auth, err := repoAuth(&models.TemplateRepo{URL: "git@github.com:org/private.git", SSHKey: pemKey})
	require.NoError(t, err)
	if keys, ok := auth.(*gitssh.PublicKeys); assert.True(t, ok, "an SSH address signs in with the key") {
		assert.Equal(t, "git", keys.User)
	}

	auth, err = repoAuth(&models.TemplateRepo{URL: "ssh://deploy@git.example.com/private.git", SSHKey: pemKey})
	require.NoError(t, err)
	if keys, ok := auth.(*gitssh.PublicKeys); assert.True(t, ok) {
		assert.Equal(t, "deploy", keys.User)
	}

	_, err = repoAuth(&models.TemplateRepo{Name: "broken", URL: "git@github.com:org/private.git", SSHKey: "not a key"})
	if assert.Error(t, err) {
		assert.NotContains(t, err.Error(), "not a key", "the key must not end up in an error or log")
	}
}

// vpsServer serves an index with two templates and counts the index downloads
func vpsServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	indexHits := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			indexHits.Add(1)
			_, _ = fmt.Fprint(w, `{"paper":{"url":"paper.json"},"vanilla":{"url":"vanilla.json"}}`)
		case "/paper.json", "/vanilla.json":
			_, _ = fmt.Fprint(w, `{"display":"Minecraft"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, indexHits
}

func TestGetAllFromVps_ReusesTheIndex(t *testing.T) {
	vpsCache = vpsCacheEntry{}
	t.Cleanup(func() { vpsCache = vpsCacheEntry{} })
	server, indexHits := vpsServer(t)
	ts := &Template{}

	for i := 0; i < 3; i++ {
		list, err := ts.getAllFromVps(server.URL + "/index.json")
		require.NoError(t, err)
		assert.Len(t, list, 2)
	}
	assert.Equal(t, int32(1), indexHits.Load(), "the index was downloaded on every request")

	tmpl, err := ts.getFromVps(server.URL+"/index.json", "paper")
	require.NoError(t, err)
	assert.Equal(t, "paper", tmpl.Name)
	assert.Equal(t, int32(1), indexHits.Load())

	// another templates.url is never answered from the old one
	_, err = ts.getAllFromVps(server.URL + "/index.json?v=2")
	require.NoError(t, err)
	assert.Equal(t, int32(2), indexHits.Load())

	// nor is an old copy
	vpsCacheLock.Lock()
	vpsCache.fetched = time.Now().Add(-vpsCacheTTL)
	vpsCacheLock.Unlock()
	_, err = ts.getAllFromVps(server.URL + "/index.json?v=2")
	require.NoError(t, err)
	assert.Equal(t, int32(3), indexHits.Load())
}

// a community template, written with its variables out of alphabetical order and a field
// the panel does not know
const authoredTemplate = `{"display":"Paper","data":{"version":{"value":"1.21","hint":"x"},"memory":{"type":"integer","value":1024},"eula":{"type":"boolean","value":false}},"run":{"command":"java"}}`

func TestGetFromVps_ServesTheTemplateAsWritten(t *testing.T) {
	vpsCache = vpsCacheEntry{}
	t.Cleanup(func() { vpsCache = vpsCacheEntry{} })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			_, _ = fmt.Fprint(w, `{"paper":{"url":"paper.json"}}`)
		case "/paper.json":
			_, _ = fmt.Fprint(w, authoredTemplate)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	ts := &Template{}

	tmpl, err := ts.getFromVps(server.URL+"/index.json", "paper")
	require.NoError(t, err)
	assert.Equal(t, 1024, tmpl.Variables["memory"].Value, "Go still reads it typed")
	out, err := json.Marshal(tmpl)
	require.NoError(t, err)
	assert.Equal(t, authoredTemplate[:len(authoredTemplate)-1]+`,"name":"paper"}`, string(out))

	list, err := ts.getAllFromVps(server.URL + "/index.json")
	require.NoError(t, err)
	require.Len(t, list, 1)
	out, err = json.Marshal(list[0])
	require.NoError(t, err)
	assert.Equal(t, authoredTemplate[:len(authoredTemplate)-1]+`,"name":"paper"}`, string(out))
}

func TestReadTemplateFromDisk_ServesTheTemplateAsWritten(t *testing.T) {
	cache := t.TempDir()
	require.NoError(t, config.CacheFolder.Set(cache, false))
	t.Cleanup(func() { _ = config.CacheFolder.Set("cache", false) })
	path := filepath.Join(cache, "template-repos", "1", "minecraft", "paper.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(authoredTemplate), 0o600))

	tmpl, err := readTemplateFromDisk("paper", path)
	require.NoError(t, err)
	tmpl.Readme = "# Paper"
	out, err := json.Marshal(tmpl)
	require.NoError(t, err)
	assert.Equal(t, authoredTemplate[:len(authoredTemplate)-1]+`,"name":"paper","readme":"# Paper"}`, string(out))
}

func TestSummarizeTemplate_KeepsTheIcon(t *testing.T) {
	tmpl := &models.Template{}
	require.NoError(t, tmpl.SetDefinition([]byte(`{"display":"My survival","icon":"rust","type":"rust","data":{"seed":{"value":"1"}}}`)))

	out, err := json.Marshal(summarizeTemplate("my-survival", tmpl))
	require.NoError(t, err)

	var summary map[string]any
	require.NoError(t, json.Unmarshal(out, &summary))
	assert.Equal(t, "rust", summary["icon"], "the list shows the logo picked in the editor")
	assert.Equal(t, "My survival", summary["display"])
	assert.Equal(t, "my-survival", summary["name"])
	assert.Nil(t, summary["data"], "the summary still leaves the variables out")
}

func TestDeleteRepo_RemovesTheCheckoutAndItsCacheEntry(t *testing.T) {
	cache := t.TempDir()
	require.NoError(t, config.CacheFolder.Set(cache, false))
	t.Cleanup(func() { _ = config.CacheFolder.Set("cache", false) })
	db := setupDeleteCleanupDB(t, &models.TemplateRepo{})
	repo := &models.TemplateRepo{Name: "community", URL: "https://example.com/templates.git", Branch: "main"}
	require.NoError(t, db.Create(repo).Error)

	checkout := repoCheckoutPath(repo.ID)
	require.NoError(t, os.MkdirAll(filepath.Join(checkout, "minecraft"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(checkout, "minecraft", "paper.json"), []byte(authoredTemplate), 0o600))
	other := filepath.Join(cache, "template-repos", "999")
	require.NoError(t, os.MkdirAll(other, 0o755))
	pathLock.Lock()
	existingPaths[repo.ID] = repoCache{LastRefreshed: time.Now(), Path: checkout}
	pathLock.Unlock()
	t.Cleanup(func() {
		pathLock.Lock()
		delete(existingPaths, repo.ID)
		pathLock.Unlock()
	})

	require.NoError(t, (&Template{DB: db}).DeleteRepo(repo.ID))

	assert.NoDirExists(t, checkout)
	assert.DirExists(t, other, "another repo's checkout stays")
	pathLock.Lock()
	_, cached := existingPaths[repo.ID]
	pathLock.Unlock()
	assert.False(t, cached, "a repo made later with this id must not reuse the old checkout")
}
