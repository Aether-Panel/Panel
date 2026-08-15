package api

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

const (
	// gitImage is the minimal image used to run git in isolation when the docker
	// socket is mounted. The binary inside the container is never affected by the
	// host PATH, so update checks cannot be influenced by a mutable environment.
	gitImage = "alpine/git:latest"

	// fixedPath is an immutable PATH used for any subprocess spawned by this
	// package. It only contains fixed, non-writable directories so executable
	// resolution can never be redirected by a compromised environment.
	fixedPath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

var (
	gitBinaryOnce sync.Once
	gitBinaryPath string

	dockerOnce sync.Once
	dockerCLI  *client.Client

	selfIDOnce sync.Once
	selfID     string
)

// resolveGitBinary returns an absolute, fixed path to the git binary so it is
// never resolved through the (potentially mutable) PATH environment variable.
func resolveGitBinary() string {
	gitBinaryOnce.Do(func() {
		for _, p := range []string{"/usr/bin/git", "/usr/local/bin/git", "/bin/git", "/usr/lib/git-core/git"} {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				gitBinaryPath = p
				return
			}
		}
		// Last resort: rely on the current environment. This keeps the panel
		// working on hosts that do not have git at a standard location.
		if p, err := exec.LookPath("git"); err == nil {
			gitBinaryPath = p
		}
	})
	return gitBinaryPath
}

// dockerAvailable returns a connected docker client when the docker socket is
// mounted in this container, or nil otherwise.
func dockerAvailable() *client.Client {
	dockerOnce.Do(func() {
		if _, err := os.Stat("/var/run/docker.sock"); err != nil {
			return
		}
		cli, err := client.New(client.FromEnv)
		if err != nil {
			return
		}
		dockerCLI = cli
	})
	return dockerCLI
}

// selfContainerID returns an identifier that the host docker daemon can use to
// reference this panel container. It is used with the VolumesFrom semantics so
// the ephemeral git container inherits this container's mounts (e.g. the
// read-only /repo/.git bind).
func selfContainerID() string {
	selfIDOnce.Do(func() {
		if data, err := os.ReadFile("/proc/self/cgroup"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				idx := strings.Index(line, "/docker/")
				if idx < 0 {
					continue
				}
				id := strings.TrimPrefix(line[idx+len("/docker/"):], "/")
				id = strings.TrimSuffix(id, ".scope")
				id = strings.TrimSpace(id)
				if id != "" {
					selfID = id
					return
				}
			}
		}
		// docker-compose sets the hostname to the container name by default.
		if hn, err := os.Hostname(); err == nil && hn != "" {
			selfID = hn
		}
	})
	return selfID
}

// ensureGitImage makes sure gitImage is present locally, pulling it if missing.
func ensureGitImage(ctx context.Context, cli *client.Client) error {
	ref := gitImage
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		ref += ":latest"
	}

	opts := client.ImageListOptions{All: true, Filters: make(client.Filters)}
	opts.Filters.Add("reference", ref)
	list, err := cli.ImageList(ctx, opts)
	if err != nil {
		return err
	}
	for _, img := range list.Items {
		for _, tag := range img.RepoTags {
			if tag == ref {
				return nil
			}
		}
	}

	rc, err := cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(io.Discard, rc)
	return err
}

// runGitInContainer runs git inside an ephemeral container that reuses this
// container's mounts, returning the git process exit code.
func runGitInContainer(args ...string) (int, error) {
	cli := dockerAvailable()
	if cli == nil {
		return -1, errors.New("docker socket not available")
	}
	self := selfContainerID()
	if self == "" {
		return -1, errors.New("unable to determine own container id")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := ensureGitImage(ctx, cli); err != nil {
		return -1, err
	}

	res, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image: gitImage,
			Cmd:   args,
		},
		HostConfig: &container.HostConfig{
			VolumesFrom: []string{self},
			AutoRemove:  true,
		},
	})
	if err != nil {
		return -1, err
	}
	id := res.ID

	defer func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		_, _ = cli.ContainerRemove(ctx2, id, client.ContainerRemoveOptions{Force: true})
	}()

	if _, err := cli.ContainerStart(ctx, id, client.ContainerStartOptions{}); err != nil {
		return -1, err
	}

	wait := cli.ContainerWait(ctx, id, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning})
	select {
	case err := <-wait.Error:
		return -1, err
	case resp := <-wait.Result:
		return int(resp.StatusCode), nil
	}
}

// runGitBinary executes the git binary at a fixed absolute path with an
// immutable PATH, so executable resolution can never be redirected.
func runGitBinary(args ...string) (int, error) {
	git := resolveGitBinary()
	if git == "" {
		return -1, errors.New("git binary not found")
	}

	cmd := exec.Command(git, args...)
	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "PATH=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, fixedPath)
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

// runGit executes git with the given arguments, preferring an isolated docker
// container and falling back to the fixed git binary path when the docker
// socket or image is unavailable.
func runGit(args ...string) (int, error) {
	if code, err := runGitInContainer(args...); err == nil {
		return code, nil
	}
	return runGitBinary(args...)
}
