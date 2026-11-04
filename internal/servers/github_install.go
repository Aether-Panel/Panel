package servers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
)

// Installing a GitHub repository: the panel checks the account can see the repository and the
// branch, asks GitHub for the branch's archive and sends the node only the short-lived download
// address GitHub answers with (valid for that one archive, a few minutes). The user's token never
// leaves the panel. The node downloads the archive outside the server folder first, so a failed
// download changes nothing; then it stops the server, takes a restore point, empties the folder
// and unpacks the archive into it. Big projects never pass through the browser or SFTP.

// GitHub install steps, reported as short keys the UI translates
const (
	GitHubStepDownloading  = "downloading"
	GitHubStepStopping     = "stopping"
	GitHubStepRestorePoint = "restorePoint"
	GitHubStepCleaning     = "cleaning"
	GitHubStepExtracting   = "extracting"
	GitHubStepDone         = "done"
	GitHubStepFailed       = "failed"
)

// RestoreReasonGitHub names the restore point taken before a repository replaces the files
const RestoreReasonGitHub = "github"

var (
	ErrGitHubInstalling = errors.New("a GitHub install is running on this server; try again when it finishes")
	ErrGitHubBadRequest = errors.New("invalid repository, branch or download address")
	ErrServerInstalling = errors.New("the server is being installed; try again when it finishes")
)

// githubDownloadTimeout bounds the whole download, so a stalled one does not hold the server
const githubDownloadTimeout = 30 * time.Minute

// githubArchiveOrigin is GitHub's archive host. The node builds every download address on it
// and takes only the path and query from the panel's, so this job can never fetch anything
// else. Tests point it at a fake.
var githubArchiveOrigin = "https://codeload.github.com"

var (
	githubRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)
	githubBranchPattern     = regexp.MustCompile(`^[^\s~^:?*\[\\]{1,255}$`)
)

// GitHubInstallRequest is what the panel sends the node: what is installed, for the progress and
// the console, and where to download it from
type GitHubInstallRequest struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	URL        string `json:"url"`
}

// GitHubInstallStatus is the install that is running or the last one
type GitHubInstallStatus struct {
	Installing bool   `json:"installing"`
	Step       string `json:"step,omitempty"`
	Repository string `json:"repository,omitempty"`
	Branch     string `json:"branch,omitempty"`
	// Downloaded counts the archive's bytes so far; GitHub rarely says the total
	Downloaded int64  `json:"downloaded"`
	LastError  string `json:"lastError,omitempty"`
}

type githubJob struct {
	running    atomic.Bool
	downloaded atomic.Int64
	mu         sync.Mutex
	step       string
	repository string
	branch     string
	lastErr    string
}

var githubJobs sync.Map // server id → *githubJob

func (p *Server) githubState() *githubJob {
	state, _ := githubJobs.LoadOrStore(p.ID(), &githubJob{})
	return state.(*githubJob)
}

// githubInstallRunning tells the other jobs that replace files (reinstall, modpack, version)
// to wait
func (p *Server) githubInstallRunning() bool {
	return p.githubState().running.Load()
}

func (j *githubJob) setStep(step string) {
	j.mu.Lock()
	j.step = step
	j.mu.Unlock()
}

func validGitHubInstall(req GitHubInstallRequest) (*url.URL, bool) {
	if !githubRepositoryPattern.MatchString(req.Repository) || strings.Contains(req.Repository, "..") ||
		!githubBranchPattern.MatchString(req.Branch) || strings.Contains(req.Branch, "..") {
		return nil, false
	}
	origin, err := url.Parse(githubArchiveOrigin)
	if err != nil {
		return nil, false
	}
	given, err := url.Parse(req.URL)
	if err != nil || given.Scheme != origin.Scheme || given.Host != origin.Host || given.User != nil {
		return nil, false
	}
	return &url.URL{Scheme: origin.Scheme, Host: origin.Host, Path: given.Path, RawPath: given.RawPath, RawQuery: given.RawQuery}, true
}

// GitHubInstallStatus reports the install that is running or the last one
func (p *Server) GitHubInstallStatus() GitHubInstallStatus {
	state := p.githubState()
	status := GitHubInstallStatus{Installing: state.running.Load(), Downloaded: state.downloaded.Load()}
	state.mu.Lock()
	status.Step, status.Repository, status.Branch, status.LastError = state.step, state.repository, state.branch, state.lastErr
	state.mu.Unlock()
	return status
}

// InstallFromGitHub starts replacing the server's files with a branch of a repository and returns
// at once; GitHubInstallStatus follows it
func (p *Server) InstallFromGitHub(req GitHubInstallRequest) error {
	req.Repository, req.Branch = strings.TrimSpace(req.Repository), strings.TrimSpace(req.Branch)
	source, ok := validGitHubInstall(req)
	if !ok {
		return ErrGitHubBadRequest
	}
	if p.IsBackingUp() || p.IsRestoring() {
		return ErrModpackServerBusy
	}
	if p.versionState().running.Load() {
		return ErrVersionChanging
	}
	if p.modpackState().running.Load() {
		return ErrModpackInstalling
	}
	if p.installRunning.Load() || (p.RunningEnvironment != nil && p.RunningEnvironment.IsInstalling()) {
		return ErrServerInstalling
	}
	state := p.githubState()
	if !state.running.CompareAndSwap(false, true) {
		return ErrGitHubInstalling
	}

	state.downloaded.Store(0)
	state.mu.Lock()
	state.step, state.lastErr = GitHubStepDownloading, ""
	state.repository, state.branch = req.Repository, req.Branch
	state.mu.Unlock()

	go func() {
		defer state.running.Store(false)
		if err := p.runGitHubInstall(req, source, state); err != nil {
			state.mu.Lock()
			state.lastErr = err.Error()
			state.step = GitHubStepFailed
			state.mu.Unlock()
			p.Log(logging.Error, "GitHub install failed: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "GitHub install failed: %s\n", err)
		}
	}()
	return nil
}

func (p *Server) runGitHubInstall(req GitHubInstallRequest, source *url.URL, state *githubJob) error {
	env := p.RunningEnvironment
	env.DisplayToConsole(true, "Installing %s (branch %s) from GitHub\n", req.Repository, req.Branch)

	// the download comes first and lands outside the server folder: a failure changes nothing,
	// and the archive is neither in the restore point nor emptied with the folder
	archive, err := p.downloadGitHubArchive(source, state)
	if archive != nil {
		defer func() {
			_ = archive.Close()
			_ = os.Remove(archive.Name())
			p.ForgetDiskUsage()
		}()
	}
	if err != nil {
		return err
	}
	env.DisplayToConsole(true, "Downloaded %s\n", formatGitHubSize(state.downloaded.Load()))

	state.setStep(GitHubStepStopping)
	running, err := p.IsRunning()
	if err != nil {
		return err
	}
	if running {
		if err := p.Stop(); err != nil {
			return err
		}
		if err := p.GetEnvironment().WaitForMainProcessFor(stopTimeout); err != nil {
			return err
		}
	}

	// nobody may start the server while its files are replaced
	env.SetInstalling(true)
	defer env.SetInstalling(false)

	state.setStep(GitHubStepRestorePoint)
	if err := p.CreateRestorePoint(RestoreReasonGitHub); err != nil {
		return fmt.Errorf("could not create a restore point, nothing was changed: %w", err)
	}

	fs := p.GetFileServer()
	state.setStep(GitHubStepCleaning)
	if err := emptyServerFolder(fs); err != nil {
		return fmt.Errorf("could not empty the server folder (a restore point was saved in Backups): %w", err)
	}
	// the files just removed must not count against the quota while the archive unpacks
	p.ForgetDiskUsage()

	// GitHub's archive holds one folder (owner-repo-commit); its contents go to the server root
	state.setStep(GitHubStepExtracting)
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	err = files.ExtractReaderLimited(fs, archive, "repository.tar.gz", "", "*", true, p.DiskWriter)
	p.ForgetDiskUsage()
	if err != nil {
		return fmt.Errorf("could not unpack the repository (a restore point was saved in Backups): %w", err)
	}

	state.setStep(GitHubStepDone)
	env.DisplayToConsole(true, "%s (branch %s) installed from GitHub\n", req.Repository, req.Branch)
	return nil
}

// githubArchiveFolder is where the download waits: the server's own tmp folder on Docker, which
// counts against its disk limit and stays out of the file manager and the backups; next to the
// server folder otherwise
func (p *Server) githubArchiveFolder() string {
	if dir := p.tmpDirectory(); dir != "" {
		return dir
	}
	if root := p.RunningEnvironment.GetRootDirectory(); root != "" {
		return filepath.Dir(filepath.Clean(root))
	}
	return os.TempDir()
}

// downloadGitHubArchive saves the archive in githubArchiveFolder and returns it open
func (p *Server) downloadGitHubArchive(source *url.URL, state *githubJob) (*os.File, error) {
	ctx, cancel := context.WithTimeout(context.Background(), githubDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.String(), nil)
	if err != nil {
		return nil, err
	}
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, upstream("GitHub", redactURLError(err))
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return nil, upstream("GitHub", fmt.Errorf("the download was refused (%d); try again", res.StatusCode))
	}

	if err := p.CheckDiskSpace(false); err != nil {
		return nil, err
	}
	folder := p.githubArchiveFolder()
	if err := os.MkdirAll(folder, 0o750); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(folder, ".aether-github-*.tar.gz")
	if err != nil {
		return nil, err
	}
	counter := &countingWriter{w: p.DiskWriter(file), n: &state.downloaded}
	if _, err := io.Copy(counter, res.Body); err != nil {
		return file, fmt.Errorf("the download stopped: %w", redactURLError(err))
	}
	return file, nil
}

// redactURLError drops the query of the address in a request error: GitHub's download address
// carries a short-lived token, and the error ends up in the console
func redactURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if u, parseErr := url.Parse(urlErr.URL); parseErr == nil {
			u.RawQuery = ""
			return &url.Error{Op: urlErr.Op, URL: u.String(), Err: urlErr.Err}
		}
	}
	return err
}

// countingWriter adds what passes through it to n, for the progress the page shows
type countingWriter struct {
	w io.Writer
	n *atomic.Int64
}

func (c *countingWriter) Write(b []byte) (int, error) {
	written, err := c.w.Write(b)
	c.n.Add(int64(written))
	return written, err
}

// emptyServerFolder removes everything in the server folder
func emptyServerFolder(fs files.FileServer) error {
	entries, err := fs.ReadDir(".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var err error
		if entry.IsDir() {
			// the file server walks the folder and unlinks links without following them
			err = fs.RemoveAll(entry.Name())
		} else {
			// a file or a link: the link itself goes, never what it points at. The name comes
			// from listing the root, a single entry that cannot reach outside it.
			err = os.Remove(filepath.Join(fs.Prefix(), entry.Name()))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func formatGitHubSize(n int64) string {
	const mb = 1024 * 1024
	if n >= mb {
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	}
	return fmt.Sprintf("%.1f KB", float64(n)/1024)
}
