package update

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

const (
	pollInterval = 2 * time.Second
	logTailLines = "100"
	maxLogTail   = 8000
)

// Image is the ephemeral image used to run the update script on the host.
const Image = "alpine:latest"

// Command is executed inside the ephemeral container, chrooting into the host
// so it can operate on the repository and docker daemon of the host machine.
var Command = []string{"chroot", "/host", "bash", "-c", "cd /opt/skypanel && chmod +x tools/panelUpdate/panelUpdate.sh && ./tools/panelUpdate/panelUpdate.sh"}

// State is a snapshot of an in-flight (or finished) update of this process.
type State struct {
	Running     bool      `json:"running"`
	ContainerID string    `json:"containerId"`
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt"`
	ExitCode    int       `json:"exitCode"`
	LogTail     string    `json:"log"`
	Error       string    `json:"error"`
}

// Tracker keeps the state of the most recent update of this process.
type Tracker struct {
	mu    sync.Mutex
	state State
}

var defaultTracker = &Tracker{}

// Status returns a snapshot of the current update status.
func Status() State {
	return defaultTracker.Snapshot()
}

// RecordFailure marks the current update as failed when it could not even be
// started (e.g. docker connection or container creation errors).
func RecordFailure(errMsg string) {
	if errMsg == "" {
		return
	}
	defaultTracker.finish(-1, "", errMsg)
}

// Snapshot returns a copy of the tracked state.
func (t *Tracker) Snapshot() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

func (t *Tracker) begin(containerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = State{
		Running:     true,
		ContainerID: containerID,
		StartedAt:   time.Now(),
		ExitCode:    -1,
	}
}

func (t *Tracker) setLog(tail string) {
	if tail == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state.LogTail = truncateLogTail(tail)
}

func (t *Tracker) finish(exitCode int, tail string, errStr string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state.Running = false
	t.state.FinishedAt = time.Now()
	t.state.ExitCode = exitCode
	t.state.Error = errStr
	if tail != "" {
		t.state.LogTail = truncateLogTail(tail)
	}
}

func truncateLogTail(s string) string {
	if len(s) <= maxLogTail {
		return s
	}
	return s[len(s)-maxLogTail:]
}

// Trigger starts an update of the host this process runs on. It spins up an
// ephemeral alpine container that chroots into the host and runs the
// panelUpdate script, then tracks progress in the background.
func Trigger() (containerID string, err error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return "", fmt.Errorf("failed to connect to docker: %v", err)
	}
	defer cli.Close()

	ctx := context.Background()

	// The daemon will not pull the image automatically, so do it explicitly.
	if _, err := cli.ImageInspect(ctx, Image); err != nil {
		logging.Info.Printf("Update image %s not found, pulling it...", Image)
		pull, err := cli.ImagePull(ctx, Image, client.ImagePullOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to pull %s: %v", Image, err)
		}
		if err := pull.Wait(ctx); err != nil {
			return "", fmt.Errorf("failed to pull %s: %v", Image, err)
		}
		pull.Close()
	}

	resp, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: "",
		Config: &container.Config{
			Image: Image,
			Cmd:   Command,
		},
		HostConfig: &container.HostConfig{
			Binds: []string{"/:/host"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create update container: %v", err)
	}

	if _, err := cli.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start update container: %v", err)
	}

	defaultTracker.begin(resp.ID)
	go monitor(resp.ID)

	return resp.ID, nil
}

// monitor waits for the update container to exit, captures its logs for
// progress reporting and cleans it up afterwards.
func monitor(containerID string) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		defaultTracker.finish(-1, "", "failed to connect to docker: "+err.Error())
		return
	}
	defer cli.Close()

	ctx := context.Background()

	wait := cli.ContainerWait(ctx, containerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})

	exitCode := int64(-1)
	var waitErr error
	waitDone := false
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for !waitDone {
		select {
		case res, ok := <-wait.Result:
			if ok {
				exitCode = res.StatusCode
			}
			waitDone = true
		case err, ok := <-wait.Error:
			if ok {
				waitErr = err
			}
			waitDone = true
		case <-ticker.C:
			defaultTracker.setLog(readLogTail(ctx, cli, containerID))
		}
	}

	tail := readLogTail(ctx, cli, containerID)

	errStr := ""
	if waitErr != nil {
		errStr = waitErr.Error()
	}
	defaultTracker.finish(int(exitCode), tail, errStr)

	logging.Info.Printf("Update container %s finished with exit code %d", containerID, exitCode)

	if _, err := cli.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{Force: true}); err != nil {
		logging.Error.Printf("Failed to remove update container %s: %v", containerID, err)
	}
}

func readLogTail(ctx context.Context, cli *client.Client, containerID string) string {
	logs, err := cli.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       logTailLines,
	})
	if err != nil {
		return ""
	}
	defer logs.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, logs); err != nil {
		return ""
	}

	return strings.TrimSpace(stdout.String() + stderr.String())
}
