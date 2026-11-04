package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// githubFake answers the device flow and the few API routes the panel uses
type githubFake struct {
	mu       sync.Mutex
	polls    int
	approved bool // the user entered the code
}

func (f *githubFake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	write := func(v interface{}) { _ = json.NewEncoder(w).Encode(v) }
	switch {
	case r.URL.Path == "/login/device/code":
		_ = r.ParseForm()
		if r.Form.Get("client_id") != "Ov23liTestClient" || r.Form.Get("scope") != "repo" {
			// GitHub's answer for a Client ID it does not know
			w.WriteHeader(http.StatusNotFound)
			return
		}
		write(map[string]interface{}{"device_code": "dev-123", "user_code": "ABCD-1234", "verification_uri": "https://github.com/login/device", "expires_in": 900, "interval": 0})
	case r.URL.Path == "/login/oauth/access_token":
		f.polls++
		if !f.approved {
			write(map[string]string{"error": "authorization_pending"})
			return
		}
		write(map[string]string{"access_token": "gho_secret", "scope": "repo", "token_type": "bearer"})
	case r.Header.Get("Authorization") != "Bearer gho_secret":
		w.WriteHeader(http.StatusUnauthorized)
		write(map[string]string{"message": "Bad credentials"})
	case r.URL.Path == "/user":
		write(map[string]string{"login": "octo", "avatar_url": "https://avatars.example/octo"})
	case r.URL.Path == "/user/repos":
		write([]map[string]interface{}{
			{"full_name": "octo/app", "name": "app", "private": true, "default_branch": "main", "owner": map[string]string{"login": "octo"}},
		})
	case r.URL.Path == "/repos/octo/app":
		write(map[string]interface{}{"full_name": "octo/app", "name": "app", "private": true, "default_branch": "main", "owner": map[string]string{"login": "octo"}})
	case r.URL.Path == "/repos/octo/app/branches":
		write([]map[string]string{{"name": "main"}, {"name": "dev"}})
	case r.URL.Path == "/repos/octo/app/branches/dev" || r.URL.Path == "/repos/octo/app/branches/main":
		write(map[string]string{"name": strings.TrimPrefix(r.URL.Path, "/repos/octo/app/branches/")})
	case strings.HasPrefix(r.URL.Path, "/repos/octo/app/tarball/"):
		branch := strings.TrimPrefix(r.URL.Path, "/repos/octo/app/tarball/")
		http.Redirect(w, r, "https://codeload.github.com/octo/app/legacy.tar.gz/refs/heads/"+branch+"?token=short-lived", http.StatusFound)
	default:
		w.WriteHeader(http.StatusNotFound)
		write(map[string]string{"message": "Not Found"})
	}
}

type githubTestEnv struct {
	db   *gorm.DB
	fake *githubFake
	user *models.User
}

func setupGitHubTest(t *testing.T, clientID string) *githubTestEnv {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "github_test.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.GitHubAccount{}))

	fake := &githubFake{}
	srv := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(srv.Close)

	previousClient, previousSecret := githubClient, githubTokenSecret
	githubClient = func() *services.GitHubClient {
		gh := services.NewGitHubClient(clientID)
		gh.WebBase, gh.APIBase = srv.URL, srv.URL
		return gh
	}
	githubTokenSecret = func() string { return "panel-secret" }
	t.Cleanup(func() { githubClient, githubTokenSecret = previousClient, previousSecret })

	user := &models.User{ID: 7, Username: "owner"}
	githubLogins.Delete(user.ID)
	t.Cleanup(func() { githubLogins.Delete(user.ID) })
	return &githubTestEnv{db: db, fake: fake, user: user}
}

func (env *githubTestEnv) router() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", env.db)
		c.Set("user", env.user)
	})
	g := r.Group("/self/github")
	g.GET("", getGitHubStatus)
	g.DELETE("", unlinkGitHub)
	g.POST("/device", startGitHubDevice)
	g.POST("/device/poll", pollGitHubDevice)
	g.GET("/repos", listGitHubRepos)
	g.GET("/repos/:owner/:repo/branches", listGitHubBranches)
	r.POST("/servers/:serverId/github/install", installFromGitHub)
	return r
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return out
}

func TestGitHub_NotConfigured(t *testing.T) {
	env := setupGitHubTest(t, "")
	r := env.router()

	w := doJSON(r, http.MethodGet, "/self/github", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, false, decode(t, w)["configured"])

	w = doJSON(r, http.MethodPost, "/self/github/device", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitHub_UnknownClientID(t *testing.T) {
	env := setupGitHubTest(t, "Ov23liUnknownClient")
	w := doJSON(env.router(), http.MethodPost, "/self/github/device", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ErrGitHubBadClient")
}

func TestGitHub_DeviceFlowConnectsAndSealsTheToken(t *testing.T) {
	env := setupGitHubTest(t, "Ov23liTestClient")
	r := env.router()

	w := doJSON(r, http.MethodPost, "/self/github/device", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	started := decode(t, w)
	assert.Equal(t, "ABCD-1234", started["userCode"])
	assert.NotContains(t, w.Body.String(), "dev-123", "the device code stays in the panel")

	// the interval GitHub gave was 0, so every poll reaches it
	githubLogins.Range(func(_, v interface{}) bool {
		login := v.(*githubDeviceLogin)
		login.interval, login.nextPoll = 0, time.Time{}
		return true
	})
	w = doJSON(r, http.MethodPost, "/self/github/device/poll", nil)
	assert.Equal(t, "pending", decode(t, w)["status"])

	env.fake.approved = true
	w = doJSON(r, http.MethodPost, "/self/github/device/poll", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "connected", decode(t, w)["status"])
	assert.NotContains(t, w.Body.String(), "gho_secret", "the token never reaches the browser")

	var account models.GitHubAccount
	require.NoError(t, env.db.First(&account, "user_id = ?", env.user.ID).Error)
	assert.Equal(t, "octo", account.Login)
	assert.NotContains(t, account.Token, "gho_secret", "the token is sealed in the database")
	opened, err := services.OpenGitHubToken("panel-secret", account.Token)
	require.NoError(t, err)
	assert.Equal(t, "gho_secret", opened)

	w = doJSON(r, http.MethodGet, "/self/github", nil)
	status := decode(t, w)
	assert.Equal(t, true, status["connected"])
	assert.Equal(t, "octo", status["login"])
}

func TestGitHub_PollWithoutACodeIsExpired(t *testing.T) {
	env := setupGitHubTest(t, "Ov23liTestClient")
	w := doJSON(env.router(), http.MethodPost, "/self/github/device/poll", nil)
	assert.Equal(t, "expired", decode(t, w)["status"])
}

func (env *githubTestEnv) connect(t *testing.T, token string) {
	t.Helper()
	sealed, err := services.SealGitHubToken("panel-secret", token)
	require.NoError(t, err)
	require.NoError(t, env.db.Create(&models.GitHubAccount{UserID: env.user.ID, Login: "octo", Token: sealed}).Error)
}

func TestGitHub_ReposAndBranches(t *testing.T) {
	env := setupGitHubTest(t, "Ov23liTestClient")
	env.connect(t, "gho_secret")
	r := env.router()

	w := doJSON(r, http.MethodGet, "/self/github/repos", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	repos := decode(t, w)["repos"].([]interface{})
	require.Len(t, repos, 1)
	repo := repos[0].(map[string]interface{})
	assert.Equal(t, "octo/app", repo["fullName"])
	assert.Equal(t, true, repo["private"])
	assert.Equal(t, "main", repo["defaultBranch"])

	w = doJSON(r, http.MethodGet, "/self/github/repos/octo/app/branches", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	branches := decode(t, w)
	assert.Equal(t, "main", branches["defaultBranch"])
	assert.ElementsMatch(t, []interface{}{"main", "dev"}, branches["branches"])
}

func TestGitHub_ARevokedTokenIsForgotten(t *testing.T) {
	env := setupGitHubTest(t, "Ov23liTestClient")
	env.connect(t, "revoked-token")

	w := doJSON(env.router(), http.MethodGet, "/self/github/repos", nil)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var count int64
	require.NoError(t, env.db.Model(&models.GitHubAccount{}).Count(&count).Error)
	assert.Zero(t, count, "the page offers to connect again")
}

func TestGitHub_Unlink(t *testing.T) {
	env := setupGitHubTest(t, "Ov23liTestClient")
	env.connect(t, "gho_secret")

	w := doJSON(env.router(), http.MethodDelete, "/self/github", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
	var count int64
	require.NoError(t, env.db.Model(&models.GitHubAccount{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestGitHub_InstallChecksBeforeTouchingTheServer(t *testing.T) {
	env := setupGitHubTest(t, "Ov23liTestClient")
	r := env.router()

	w := doJSON(r, http.MethodPost, "/servers/srv1/github/install", map[string]string{"repository": "octo/app"})
	assert.Equal(t, http.StatusConflict, w.Code, "not connected")

	env.connect(t, "gho_secret")
	for _, body := range []map[string]string{
		{"repository": "octo"},
		{"repository": "../x/app"},
		{"repository": "octo/app", "branch": "a..b"},
	} {
		w = doJSON(r, http.MethodPost, "/servers/srv1/github/install", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
	}

	w = doJSON(r, http.MethodPost, "/servers/srv1/github/install", map[string]string{"repository": "octo/app", "branch": "missing"})
	assert.Equal(t, http.StatusNotFound, w.Code, "a branch GitHub does not have")
	w = doJSON(r, http.MethodPost, "/servers/srv1/github/install", map[string]string{"repository": "octo/other"})
	assert.Equal(t, http.StatusNotFound, w.Code, "a repository the account cannot see")
}

func TestSealGitHubToken_NeedsTheSameSecret(t *testing.T) {
	sealed, err := services.SealGitHubToken("one", "gho_x")
	require.NoError(t, err)
	_, err = services.OpenGitHubToken("two", sealed)
	assert.ErrorIs(t, err, services.ErrGitHubTokenRevoked)
}

func TestGitHub_ArchiveURLIsTheShortLivedAddress(t *testing.T) {
	setupGitHubTest(t, "Ov23liTestClient")
	address, err := githubClient().ArchiveURL(context.Background(), "gho_secret", "octo", "app", "feature/login")
	require.NoError(t, err)
	assert.Equal(t, "https://codeload.github.com/octo/app/legacy.tar.gz/refs/heads/feature/login?token=short-lived", address)
	assert.NotContains(t, address, "gho_secret", "the node never gets the user's token")

	_, err = githubClient().ArchiveURL(context.Background(), "revoked", "octo", "app", "main")
	assert.ErrorIs(t, err, services.ErrGitHubTokenRevoked)
}

func TestGitHub_AnUnreadableTokenIsForgotten(t *testing.T) {
	env := setupGitHubTest(t, "Ov23liTestClient")
	require.NoError(t, env.db.Create(&models.GitHubAccount{UserID: env.user.ID, Login: "octo", Token: "sealed-with-another-secret"}).Error)

	w := doJSON(env.router(), http.MethodGet, "/self/github/repos", nil)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var count int64
	require.NoError(t, env.db.Model(&models.GitHubAccount{}).Count(&count).Error)
	assert.Zero(t, count, "the page offers to connect again")
}
