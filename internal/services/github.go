package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

// GitHub links a panel user's account through the OAuth device flow and reads the repositories
// that account can see. Only the Client ID of an OAuth App is needed (no secret, no callback
// URL), so it works whatever domain the panel has. The token is only ever sent as a header to
// api.github.com and is kept sealed in the database (SealGitHubToken).

const (
	// GitHubWebBase serves the device flow
	GitHubWebBase = "https://github.com"
	// GitHubAPIBase is the REST API
	GitHubAPIBase = "https://api.github.com"

	// GitHubScope is what the panel asks for: "repo" is the only OAuth scope that reaches
	// private repositories (GitHub has no read-only one for OAuth Apps)
	GitHubScope = "repo"

	githubTimeout     = 20 * time.Second
	githubAPIVersion  = "2022-11-28"
	githubJSON        = "application/json"
	githubReposPage   = 100
	githubMaxBranches = 1000
)

var (
	ErrGitHubNotConfigured = skypanel.CreateError("GitHub is not set up on this panel: an administrator has to add the Client ID of a GitHub OAuth App in the settings", "ErrGitHubNotConfigured")
	ErrGitHubNotConnected  = skypanel.CreateError("connect your GitHub account first", "ErrGitHubNotConnected")
	ErrGitHubPending       = errors.New("the code has not been entered on GitHub yet")
	ErrGitHubSlowDown      = errors.New("GitHub asked to poll more slowly")
	ErrGitHubExpired       = skypanel.CreateError("the GitHub code expired; start again", "ErrGitHubCodeExpired")
	ErrGitHubDenied        = skypanel.CreateError("the access was refused on GitHub", "ErrGitHubDenied")
	ErrGitHubTokenRevoked  = skypanel.CreateError("GitHub no longer accepts this connection; connect your account again", "ErrGitHubTokenRevoked")
	ErrGitHubNotFound      = skypanel.CreateError("GitHub could not find that repository or branch, or your account cannot see it", "ErrGitHubNotFound")
	ErrGitHubBadName       = skypanel.CreateError("that is not a valid repository or branch name", "ErrGitHubBadName")
	ErrGitHubBadClient     = skypanel.CreateError("GitHub does not know that Client ID, or its OAuth App has Device Flow off; check it in Settings → GitHub", "ErrGitHubBadClient")
)

// GitHubError is any other refusal from GitHub, with its own message
type GitHubError struct {
	Status  int
	Message string
}

func (e *GitHubError) Error() string {
	if e.Message == "" {
		return "GitHub answered " + strconv.Itoa(e.Status)
	}
	return "GitHub answered " + strconv.Itoa(e.Status) + ": " + e.Message
}

var (
	// githubNamePattern is an owner or repository name
	githubNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)
	// githubBranchPattern refuses what git itself refuses in a branch name, plus spaces
	githubBranchPattern = regexp.MustCompile(`^[^\s~^:?*\[\\]{1,255}$`)
)

// ValidGitHubName tells whether name can be an owner or a repository
func ValidGitHubName(name string) bool {
	return githubNamePattern.MatchString(name) && name != "." && name != ".."
}

// ValidGitHubBranch tells whether name can be a branch
func ValidGitHubBranch(name string) bool {
	return githubBranchPattern.MatchString(name) && !strings.Contains(name, "..") &&
		!strings.HasPrefix(name, "/") && !strings.HasSuffix(name, "/") && !strings.HasSuffix(name, ".lock")
}

// GitHubClient talks to GitHub for one OAuth App; tests point the bases at a fake
type GitHubClient struct {
	ClientID string
	WebBase  string
	APIBase  string
	HTTP     *http.Client
}

// NewGitHubClient is a client against the real GitHub
func NewGitHubClient(clientID string) *GitHubClient {
	return &GitHubClient{
		ClientID: strings.TrimSpace(clientID),
		WebBase:  GitHubWebBase,
		APIBase:  GitHubAPIBase,
		HTTP:     &http.Client{Timeout: githubTimeout},
	}
}

// GitHubDeviceCode is what the user types on GitHub, and what the panel polls with
type GitHubDeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// GitHubUser is the connected account
type GitHubUser struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubRepo is a repository the account can see
type GitHubRepo struct {
	FullName      string    `json:"fullName"`
	Name          string    `json:"name"`
	Owner         string    `json:"owner"`
	Private       bool      `json:"private"`
	DefaultBranch string    `json:"defaultBranch"`
	Description   string    `json:"description"`
	UpdatedAt     time.Time `json:"updatedAt"`
	// SizeKB is GitHub's own estimate of the repository's size
	SizeKB int64 `json:"sizeKb"`
}

type githubRepoWire struct {
	FullName      string    `json:"full_name"`
	Name          string    `json:"name"`
	Private       bool      `json:"private"`
	DefaultBranch string    `json:"default_branch"`
	Description   string    `json:"description"`
	UpdatedAt     time.Time `json:"updated_at"`
	Size          int64     `json:"size"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (w githubRepoWire) repo() GitHubRepo {
	return GitHubRepo{
		FullName: w.FullName, Name: w.Name, Owner: w.Owner.Login, Private: w.Private,
		DefaultBranch: w.DefaultBranch, Description: w.Description, UpdatedAt: w.UpdatedAt, SizeKB: w.Size,
	}
}

// StartDevice asks GitHub for a code the user enters at VerificationURI
func (g *GitHubClient) StartDevice(ctx context.Context) (*GitHubDeviceCode, error) {
	if g.ClientID == "" {
		return nil, ErrGitHubNotConfigured
	}
	form := url.Values{"client_id": {g.ClientID}, "scope": {GitHubScope}}
	var code GitHubDeviceCode
	if err := g.postForm(ctx, "/login/device/code", form, &code); err != nil {
		// GitHub answers 404 (or 401) when the Client ID is not an OAuth App it knows
		if errors.Is(err, ErrGitHubNotFound) || errors.Is(err, ErrGitHubTokenRevoked) {
			return nil, ErrGitHubBadClient
		}
		return nil, err
	}
	if code.DeviceCode == "" || code.UserCode == "" {
		return nil, &GitHubError{Status: http.StatusBadGateway, Message: "no device code in GitHub's answer"}
	}
	if code.Interval <= 0 {
		code.Interval = 5
	}
	return &code, nil
}

// PollDevice asks once whether the user entered the code. It returns the token, or
// ErrGitHubPending / ErrGitHubSlowDown while waiting, ErrGitHubExpired or ErrGitHubDenied.
func (g *GitHubClient) PollDevice(ctx context.Context, deviceCode string) (token, scope string, err error) {
	form := url.Values{
		"client_id":   {g.ClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	var answer struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := g.postForm(ctx, "/login/oauth/access_token", form, &answer); err != nil {
		return "", "", err
	}
	switch answer.Error {
	case "":
		if answer.AccessToken == "" {
			return "", "", &GitHubError{Status: http.StatusBadGateway, Message: "no token in GitHub's answer"}
		}
		return answer.AccessToken, answer.Scope, nil
	case "authorization_pending":
		return "", "", ErrGitHubPending
	case "slow_down":
		return "", "", ErrGitHubSlowDown
	case "expired_token", "token_expired":
		return "", "", ErrGitHubExpired
	case "access_denied":
		return "", "", ErrGitHubDenied
	default:
		return "", "", &GitHubError{Status: http.StatusBadRequest, Message: strings.TrimSpace(answer.Error + " " + answer.Description)}
	}
}

// User is the account a token belongs to
func (g *GitHubClient) User(ctx context.Context, token string) (*GitHubUser, error) {
	var user GitHubUser
	if err := g.get(ctx, token, "/user", &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// Repos lists one page of the repositories the account owns, collaborates on or reaches
// through an organisation, most recently updated first; more tells whether a next page exists
func (g *GitHubClient) Repos(ctx context.Context, token string, page int) (repos []GitHubRepo, more bool, err error) {
	if page < 1 {
		page = 1
	}
	path := "/user/repos?per_page=" + strconv.Itoa(githubReposPage) + "&page=" + strconv.Itoa(page) +
		"&sort=updated&affiliation=owner,collaborator,organization_member"
	var wire []githubRepoWire
	if err := g.get(ctx, token, path, &wire); err != nil {
		return nil, false, err
	}
	repos = make([]GitHubRepo, 0, len(wire))
	for _, w := range wire {
		repos = append(repos, w.repo())
	}
	return repos, len(wire) == githubReposPage, nil
}

// Repo reads one repository, which also proves the account can see it
func (g *GitHubClient) Repo(ctx context.Context, token, owner, name string) (*GitHubRepo, error) {
	if !ValidGitHubName(owner) || !ValidGitHubName(name) {
		return nil, ErrGitHubBadName
	}
	var wire githubRepoWire
	if err := g.get(ctx, token, githubRepoPath(owner, name), &wire); err != nil {
		return nil, err
	}
	repo := wire.repo()
	return &repo, nil
}

// Branches lists a repository's branch names, the default one included
func (g *GitHubClient) Branches(ctx context.Context, token, owner, name string) ([]string, error) {
	if !ValidGitHubName(owner) || !ValidGitHubName(name) {
		return nil, ErrGitHubBadName
	}
	var names []string
	for page := 1; len(names) < githubMaxBranches; page++ {
		var wire []struct {
			Name string `json:"name"`
		}
		path := githubRepoPath(owner, name) + "/branches?per_page=100&page=" + strconv.Itoa(page)
		if err := g.get(ctx, token, path, &wire); err != nil {
			return nil, err
		}
		for _, b := range wire {
			names = append(names, b.Name)
		}
		if len(wire) < 100 {
			break
		}
	}
	return names, nil
}

// HasBranch tells whether the repository has that branch
func (g *GitHubClient) HasBranch(ctx context.Context, token, owner, name, branch string) error {
	if !ValidGitHubName(owner) || !ValidGitHubName(name) || !ValidGitHubBranch(branch) {
		return ErrGitHubBadName
	}
	return g.get(ctx, token, githubRepoPath(owner, name)+"/branches/"+EscapeGitHubRef(branch), nil)
}

// ArchiveURL asks GitHub where to download a branch as a .tar.gz. GitHub answers with a redirect
// to codeload.github.com whose address carries a short-lived token for that one archive, so the
// node that downloads it never needs the user's token.
func (g *GitHubClient) ArchiveURL(ctx context.Context, token, owner, name, branch string) (string, error) {
	if !ValidGitHubName(owner) || !ValidGitHubName(name) || !ValidGitHubBranch(branch) {
		return "", ErrGitHubBadName
	}
	address := strings.TrimRight(g.APIBase, "/") + githubRepoPath(owner, name) + "/tarball/" + EscapeGitHubRef(branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)

	client := &http.Client{
		Timeout:       githubTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	if g.HTTP != nil && g.HTTP.Transport != nil {
		client.Transport = g.HTTP.Transport
	}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return "", ErrGitHubTokenRevoked
	case res.StatusCode == http.StatusNotFound:
		return "", ErrGitHubNotFound
	case res.StatusCode < 300 || res.StatusCode >= 400 || res.Header.Get("Location") == "":
		return "", &GitHubError{Status: res.StatusCode, Message: "GitHub did not give a download address"}
	}
	return res.Header.Get("Location"), nil
}

// githubRepoPath is a repository's route in the API; owner and name were checked with
// ValidGitHubName, so neither can add a path segment
func githubRepoPath(owner, name string) string {
	return "/repos/" + owner + "/" + name
}

// EscapeGitHubRef escapes each part of a branch name but keeps its slashes, which GitHub reads
// as part of the name (feature/login); an escaped %2F would not find the branch
func EscapeGitHubRef(ref string) string {
	parts := strings.Split(ref, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func (g *GitHubClient) postForm(ctx context.Context, path string, form url.Values, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(g.WebBase, "/")+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", githubJSON)
	return g.do(req, out)
}

func (g *GitHubClient) get(ctx context.Context, token, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(g.APIBase, "/")+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	return g.do(req, out)
}

func (g *GitHubClient) do(req *http.Request, out interface{}) error {
	client := g.HTTP
	if client == nil {
		client = &http.Client{Timeout: githubTimeout}
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return ErrGitHubTokenRevoked
	case res.StatusCode == http.StatusNotFound:
		return ErrGitHubNotFound
	case res.StatusCode >= 300:
		var answer struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &answer)
		return &GitHubError{Status: res.StatusCode, Message: answer.Message}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

// githubSealContext separates the key that seals GitHub tokens from other uses of the secret
const githubSealContext = "aether-panel github token v1"

func githubTokenKey(secret string) []byte {
	sum := sha256.Sum256([]byte(githubSealContext + "\x00" + secret))
	return sum[:]
}

// SealGitHubToken encrypts a token with AES-GCM under a key derived from the panel's secret, so a
// copy of the database alone does not give the tokens away
func SealGitHubToken(secret, token string) (string, error) {
	block, err := aes.NewCipher(githubTokenKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(token), nil)), nil
}

// OpenGitHubToken reverses SealGitHubToken; a changed secret makes every token unreadable, and
// the user connects again
func OpenGitHubToken(secret, sealed string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", ErrGitHubTokenRevoked
	}
	block, err := aes.NewCipher(githubTokenKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", ErrGitHubTokenRevoked
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrGitHubTokenRevoked
	}
	return string(plain), nil
}
