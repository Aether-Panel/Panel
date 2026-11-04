package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GitHub: each user connects their own account with a device code and installs one of their
// repositories (a branch of it) into a server. The panel keeps the token and checks the
// repository; the node downloads it straight from GitHub, so big projects never pass through
// the browser or SFTP.

// githubClient builds the client for the configured OAuth App; tests replace it
var githubClient = func() *services.GitHubClient {
	return services.NewGitHubClient(config.GitHubClientID.Value())
}

// githubTokenSecret is the secret GitHub tokens are sealed with; tests replace it
var githubTokenSecret = func() string {
	return config.SessionKey.Value()
}

const githubRequestTimeout = 30 * time.Second

// githubDeviceLogin is a device code a user is entering on GitHub
type githubDeviceLogin struct {
	// mu keeps two polls of the same user (two tabs) from asking GitHub at once
	mu         sync.Mutex
	deviceCode string
	expires    time.Time
	interval   time.Duration
	nextPoll   time.Time
}

// githubLogins holds the device code of each user while they enter it; it never leaves the panel
var githubLogins sync.Map // user id → *githubDeviceLogin

func registerGitHub(g *gin.RouterGroup) {
	perm := middleware.RequiresPermission(scopes.ScopeSelfEdit)
	g.GET("", perm, getGitHubStatus)
	g.DELETE("", perm, unlinkGitHub)
	g.OPTIONS("", response.CreateOptions("GET", "DELETE"))
	g.POST("/device", perm, startGitHubDevice)
	g.OPTIONS("/device", response.CreateOptions("POST"))
	g.POST("/device/poll", perm, pollGitHubDevice)
	g.OPTIONS("/device/poll", response.CreateOptions("POST"))
	g.GET("/repos", perm, listGitHubRepos)
	g.OPTIONS("/repos", response.CreateOptions("GET"))
	g.GET("/repos/:owner/:repo/branches", perm, listGitHubBranches)
	g.OPTIONS("/repos/:owner/:repo/branches", response.CreateOptions("GET"))
}

// registerServerGitHub adds the install routes: they replace every file of the server, so they
// need the reinstall permission
func registerServerGitHub(g *gin.RouterGroup) {
	perm := middleware.RequiresPermission(scopes.ScopeServerInstall)
	g.GET("/:serverId/github/install", perm, middleware.ResolveServerPanel, proxyServerRequest)
	g.POST("/:serverId/github/install", perm, middleware.ResolveServerPanel, installFromGitHub)
	g.OPTIONS("/:serverId/github/install", response.CreateOptions("GET", "POST"))
}

func githubErrorStatus(err error) int {
	switch {
	case errors.Is(err, services.ErrGitHubNotConfigured), errors.Is(err, services.ErrGitHubBadName), errors.Is(err, services.ErrGitHubBadClient):
		return http.StatusBadRequest
	case errors.Is(err, services.ErrGitHubNotConnected):
		return http.StatusConflict
	case errors.Is(err, services.ErrGitHubNotFound):
		return http.StatusNotFound
	}
	// GitHub refused or could not be reached: 422, so the page shows the reason (a proxy like
	// Cloudflare replaces a 502 with its own page)
	return http.StatusUnprocessableEntity
}

type githubStatusView struct {
	// Configured tells whether the administrator added an OAuth App
	Configured  bool       `json:"configured"`
	Connected   bool       `json:"connected"`
	Login       string     `json:"login,omitempty"`
	AvatarURL   string     `json:"avatarUrl,omitempty"`
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`
}

func githubAccountOf(db *gorm.DB, userID uint) (*models.GitHubAccount, error) {
	var account models.GitHubAccount
	err := db.Where("user_id = ?", userID).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, services.ErrGitHubNotConnected
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// githubToken is the user's token, opened
func githubToken(db *gorm.DB, userID uint) (string, error) {
	account, err := githubAccountOf(db, userID)
	if err != nil {
		return "", err
	}
	token, err := services.OpenGitHubToken(githubTokenSecret(), account.Token)
	// a token the panel can no longer open (its secret changed) is dropped, so the page offers
	// to connect again instead of failing on every call
	forgetRevokedGitHub(db, userID, err)
	return token, err
}

// forgetRevokedGitHub drops a connection GitHub no longer accepts, so the page offers to
// connect again instead of failing on every call
func forgetRevokedGitHub(db *gorm.DB, userID uint, err error) {
	if errors.Is(err, services.ErrGitHubTokenRevoked) {
		_ = db.Where("user_id = ?", userID).Delete(&models.GitHubAccount{}).Error
	}
}

// @Summary GitHub connection
// @Description Whether GitHub is set up on the panel and which account the user connected
// @Success 200 {object} githubStatusView
// @Tags Users
// @Router /api/self/github [get]
// @Security OAuth2Application[self.edit]
func getGitHubStatus(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	view := githubStatusView{Configured: githubClient().ClientID != ""}
	account, err := githubAccountOf(middleware.GetDatabase(c), user.ID)
	if err != nil && !errors.Is(err, services.ErrGitHubNotConnected) {
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	}
	if account != nil {
		view.Connected, view.Login, view.AvatarURL = true, account.Login, account.AvatarURL
		connectedAt := account.CreatedAt
		view.ConnectedAt = &connectedAt
	}
	c.JSON(http.StatusOK, view)
}

// @Summary Start connecting GitHub
// @Description Gets a code the user enters at github.com/login/device
// @Success 200 {object} nil
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Tags Users
// @Router /api/self/github/device [post]
// @Security OAuth2Application[self.edit]
func startGitHubDevice(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	ctx, cancel := context.WithTimeout(c.Request.Context(), githubRequestTimeout)
	defer cancel()
	code, err := githubClient().StartDevice(ctx)
	if err != nil {
		response.HandleError(c, err, githubErrorStatus(err))
		return
	}
	interval := time.Duration(code.Interval) * time.Second
	githubLogins.Store(user.ID, &githubDeviceLogin{
		deviceCode: code.DeviceCode,
		expires:    time.Now().Add(time.Duration(code.ExpiresIn) * time.Second),
		interval:   interval,
		nextPoll:   time.Now().Add(interval),
	})
	c.JSON(http.StatusOK, gin.H{
		"userCode":        code.UserCode,
		"verificationUri": code.VerificationURI,
		"expiresIn":       code.ExpiresIn,
		"interval":        code.Interval,
	})
}

// @Summary Finish connecting GitHub
// @Description Asks GitHub once whether the user entered the code. status is pending, connected, expired or denied.
// @Success 200 {object} nil
// @Tags Users
// @Router /api/self/github/device/poll [post]
// @Security OAuth2Application[self.edit]
func pollGitHubDevice(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	value, ok := githubLogins.Load(user.ID)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"status": "expired"})
		return
	}
	login := value.(*githubDeviceLogin)
	login.mu.Lock()
	defer login.mu.Unlock()
	now := time.Now()
	if now.After(login.expires) {
		githubLogins.CompareAndDelete(user.ID, login)
		c.JSON(http.StatusOK, gin.H{"status": "expired"})
		return
	}
	// GitHub refuses polls faster than its interval: the page may ask more often, the panel not
	if now.Before(login.nextPoll) {
		c.JSON(http.StatusOK, gin.H{"status": "pending"})
		return
	}
	login.nextPoll = now.Add(login.interval)

	ctx, cancel := context.WithTimeout(c.Request.Context(), githubRequestTimeout)
	defer cancel()
	gh := githubClient()
	token, scope, err := gh.PollDevice(ctx, login.deviceCode)
	switch {
	case errors.Is(err, services.ErrGitHubPending):
		c.JSON(http.StatusOK, gin.H{"status": "pending"})
		return
	case errors.Is(err, services.ErrGitHubSlowDown):
		login.interval += 5 * time.Second
		login.nextPoll = now.Add(login.interval)
		c.JSON(http.StatusOK, gin.H{"status": "pending"})
		return
	case errors.Is(err, services.ErrGitHubExpired):
		githubLogins.CompareAndDelete(user.ID, login)
		c.JSON(http.StatusOK, gin.H{"status": "expired"})
		return
	case errors.Is(err, services.ErrGitHubDenied):
		githubLogins.CompareAndDelete(user.ID, login)
		c.JSON(http.StatusOK, gin.H{"status": "denied"})
		return
	case err != nil:
		response.HandleError(c, err, githubErrorStatus(err))
		return
	}
	githubLogins.CompareAndDelete(user.ID, login)

	account, err := gh.User(ctx, token)
	if err != nil {
		response.HandleError(c, err, githubErrorStatus(err))
		return
	}
	sealed, err := services.SealGitHubToken(githubTokenSecret(), token)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	db := middleware.GetDatabase(c)
	record := models.GitHubAccount{UserID: user.ID, Login: account.Login, AvatarURL: account.AvatarURL, Token: sealed, Scopes: scope}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.GitHubAccount{}).Error; err != nil {
			return err
		}
		return tx.Create(&record).Error
	})
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "connected", "login": account.Login, "avatarUrl": account.AvatarURL})
}

// @Summary Disconnect GitHub
// @Description Forgets the user's GitHub token; the access can also be revoked on GitHub
// @Success 204 {object} nil
// @Tags Users
// @Router /api/self/github [delete]
// @Security OAuth2Application[self.edit]
func unlinkGitHub(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	githubLogins.Delete(user.ID)
	err := middleware.GetDatabase(c).Where("user_id = ?", user.ID).Delete(&models.GitHubAccount{}).Error
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary GitHub repositories
// @Description One page (100) of the repositories the connected account can see, most recently updated first
// @Success 200 {object} nil
// @Param page query int false "Page, from 1"
// @Tags Users
// @Router /api/self/github/repos [get]
// @Security OAuth2Application[self.edit]
func listGitHubRepos(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	db := middleware.GetDatabase(c)
	token, err := githubToken(db, user.ID)
	if err != nil {
		response.HandleError(c, err, githubErrorStatus(err))
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), githubRequestTimeout)
	defer cancel()
	repos, more, err := githubClient().Repos(ctx, token, page)
	if err != nil {
		forgetRevokedGitHub(db, user.ID, err)
		response.HandleError(c, err, githubErrorStatus(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"repos": repos, "more": more})
}

// @Summary GitHub branches
// @Description The branches of one repository and which one is the default
// @Success 200 {object} nil
// @Tags Users
// @Router /api/self/github/repos/{owner}/{repo}/branches [get]
// @Security OAuth2Application[self.edit]
func listGitHubBranches(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	db := middleware.GetDatabase(c)
	token, err := githubToken(db, user.ID)
	if err != nil {
		response.HandleError(c, err, githubErrorStatus(err))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), githubRequestTimeout)
	defer cancel()
	gh := githubClient()
	repo, err := gh.Repo(ctx, token, c.Param("owner"), c.Param("repo"))
	if err == nil {
		var branches []string
		branches, err = gh.Branches(ctx, token, repo.Owner, repo.Name)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"defaultBranch": repo.DefaultBranch, "branches": branches})
			return
		}
	}
	forgetRevokedGitHub(db, user.ID, err)
	response.HandleError(c, err, githubErrorStatus(err))
}

// githubInstallRequest is what the page sends to install a repository
type githubInstallRequest struct {
	// Repository is owner/name
	Repository string `json:"repository"`
	// Branch is empty for the repository's default branch
	Branch string `json:"branch"`
}

// @Summary Install a GitHub repository
// @Description Empties the server (after a restore point) and puts a branch of one of the user's repositories in it; the node downloads it from GitHub. Runs in the background: GET the same route for its progress.
// @Success 202 {object} nil
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Param id path string true "Server ID"
// @Tags Servers
// @Router /api/servers/{id}/github/install [post]
// @Security OAuth2Application[server.install]
func installFromGitHub(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	db := middleware.GetDatabase(c)

	var body githubInstallRequest
	if response.HandleError(c, c.ShouldBindJSON(&body), http.StatusBadRequest) {
		return
	}
	owner, name, ok := strings.Cut(strings.TrimSpace(body.Repository), "/")
	branch := strings.TrimSpace(body.Branch)
	if !ok || !services.ValidGitHubName(owner) || !services.ValidGitHubName(name) || (branch != "" && !services.ValidGitHubBranch(branch)) {
		response.HandleError(c, services.ErrGitHubBadName, http.StatusBadRequest)
		return
	}
	token, err := githubToken(db, user.ID)
	if err != nil {
		response.HandleError(c, err, githubErrorStatus(err))
		return
	}

	// the account must see the repository and the branch, before anything on the server changes
	ctx, cancel := context.WithTimeout(c.Request.Context(), githubRequestTimeout)
	defer cancel()
	gh := githubClient()
	repo, err := gh.Repo(ctx, token, owner, name)
	if err == nil {
		if branch == "" {
			branch = repo.DefaultBranch
		}
		err = gh.HasBranch(ctx, token, repo.Owner, repo.Name, branch)
	}
	var archive string
	if err == nil {
		// the node gets a short-lived address for this one archive, never the user's token
		archive, err = gh.ArchiveURL(ctx, token, repo.Owner, repo.Name, branch)
	}
	if err != nil {
		forgetRevokedGitHub(db, user.ID, err)
		response.HandleError(c, err, githubErrorStatus(err))
		return
	}

	payload, err := json.Marshal(gin.H{"repository": repo.Owner + "/" + repo.Name, "branch": branch, "url": archive})
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(payload))
	c.Request.ContentLength = int64(len(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	proxyServerRequest(c)
}
