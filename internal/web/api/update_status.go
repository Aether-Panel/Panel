package api

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/gin-gonic/gin"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

const (
	updateLogPollInterval = 2 * time.Second
	updateLogTailLines    = "100"
	maxUpdateLogTail      = 8000
)

// updateTrackerState holds the state of the most recent panel update so the
// frontend can poll for progress, logs and the final result.
type updateTrackerState struct {
	mu          sync.Mutex
	running     bool
	containerID string
	startedAt   time.Time
	finishedAt  time.Time
	exitCode    int
	logTail     string
	err         string
}

var updateTracker = &updateTrackerState{}

func (u *updateTrackerState) begin(containerID string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.running = true
	u.containerID = containerID
	u.startedAt = time.Now()
	u.finishedAt = time.Time{}
	u.exitCode = -1
	u.logTail = ""
	u.err = ""
}

func (u *updateTrackerState) setLog(tail string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if tail != "" {
		u.logTail = truncateLogTail(tail)
	}
}

func (u *updateTrackerState) finish(exitCode int, tail string, errStr string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.running = false
	u.finishedAt = time.Now()
	u.exitCode = exitCode
	u.err = errStr
	if tail != "" {
		u.logTail = truncateLogTail(tail)
	}
}

func truncateLogTail(s string) string {
	if len(s) <= maxUpdateLogTail {
		return s
	}
	return s[len(s)-maxUpdateLogTail:]
}

// @Summary Get panel update status
// @Description Returns the status, exit code and log tail of the most recent panel update.
// @Success 200 {object} nil
// @Tags Panel Settings
// @Router /api/settings/update-status [get]
// @Security OAuth2Application[settings.edit]
func updateStatus(c *gin.Context) {
	updateTracker.mu.Lock()
	defer updateTracker.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"running":     updateTracker.running,
		"containerId": updateTracker.containerID,
		"startedAt":   updateTracker.startedAt,
		"finishedAt":  updateTracker.finishedAt,
		"exitCode":    updateTracker.exitCode,
		"log":         updateTracker.logTail,
		"error":       updateTracker.err,
	})
}

// monitorUpdateContainer waits for the ephemeral update container to exit,
// captures its logs for progress reporting and cleans it up afterwards.
func monitorUpdateContainer(containerID string) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		updateTracker.finish(-1, "", "failed to connect to docker: "+err.Error())
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
	ticker := time.NewTicker(updateLogPollInterval)
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
			updateTracker.setLog(readContainerLogTail(ctx, cli, containerID))
		}
	}

	tail := readContainerLogTail(ctx, cli, containerID)

	errStr := ""
	if waitErr != nil {
		errStr = waitErr.Error()
	}
	updateTracker.finish(int(exitCode), tail, errStr)

	logging.Info.Printf("Update container %s finished with exit code %d", containerID, exitCode)

	if _, err := cli.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{Force: true}); err != nil {
		logging.Error.Printf("Failed to remove update container %s: %v", containerID, err)
	}
}

func readContainerLogTail(ctx context.Context, cli *client.Client, containerID string) string {
	logs, err := cli.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       updateLogTailLines,
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
