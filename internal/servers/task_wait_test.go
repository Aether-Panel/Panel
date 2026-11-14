package servers

import (
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// powerEnvironment is a game that stops when told to and starts again, counting both. Its
// state is atomic: a task step changes it while the test watches.
type powerEnvironment struct {
	running atomic.Bool
	stops   atomic.Int32
	starts  atomic.Int32
}

func (e *powerEnvironment) ExecuteAsyncImpl(*skypanel.Environment, skypanel.ExecutionData) error {
	e.starts.Add(1)
	e.running.Store(true)
	return nil
}
func (e *powerEnvironment) KillImpl(*skypanel.Environment) error {
	e.running.Store(false)
	return nil
}
func (e *powerEnvironment) GetStatsImpl(*skypanel.Environment) (*skypanel.ServerStats, error) {
	return &skypanel.ServerStats{}, nil
}
func (e *powerEnvironment) SendCodeImpl(*skypanel.Environment, int) error {
	e.stops.Add(1)
	e.running.Store(false)
	return nil
}
func (e *powerEnvironment) GetUIDImpl(*skypanel.Environment) int { return 0 }
func (e *powerEnvironment) GetGidImpl(*skypanel.Environment) int { return 0 }
func (e *powerEnvironment) IsRunningImpl(*skypanel.Environment) (bool, error) {
	return e.running.Load(), nil
}

// newPowerTestServer is a server the node runs, whose game is running, stopped with a signal.
// What its stops and starts log is left out of the test's output.
func newPowerTestServer(t *testing.T) (*Server, *powerEnvironment) {
	p := newCrashTestServer(t)
	previous := logging.Info.Writer()
	logging.Info.SetOutput(io.Discard)
	t.Cleanup(func() { logging.Info.SetOutput(previous) })
	env := &powerEnvironment{}
	env.running.Store(true)
	p.RunningEnvironment.Implementation = env
	p.Execution.StopCode = 15
	p.Execution.Command = "game"
	useLoadedServers(t, p)
	return p, env
}

// useBackupWait makes task steps check for a backup in progress every poll, giving up after
// limit, for this test
func useBackupWait(t *testing.T, poll, limit time.Duration) {
	previousPoll, previousLimit := backupWaitPoll, backupWaitLimit
	backupWaitPoll, backupWaitLimit = poll, limit
	t.Cleanup(func() { backupWaitPoll, backupWaitLimit = previousPoll, previousLimit })
}

// runOperationInBackground runs a task step and hands back its result once it ends
func runOperationInBackground(p *Server, op string, args map[string]interface{}) <-chan error {
	done := make(chan error, 1)
	go func() { done <- runOperation(p, op, args) }()
	return done
}

func waitForStep(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(15 * time.Second):
		t.Fatal("the task step never ended")
		return nil
	}
}

func TestRestartTask_WaitsForABackupInProgress(t *testing.T) {
	p, env := newPowerTestServer(t)
	useBackupWait(t, 5*time.Millisecond, time.Minute)
	p.backupState.Store(backupRunning)

	done := runOperationInBackground(p, "restart", nil)

	assert.Never(t, func() bool { return env.stops.Load() > 0 }, 200*time.Millisecond, 5*time.Millisecond,
		"a server is not stopped while a backup would keep it from starting again")
	p.backupState.Store(backupIdle)
	require.NoError(t, waitForStep(t, done))

	assert.Equal(t, int32(1), env.stops.Load())
	assert.Equal(t, int32(1), env.starts.Load())
	assert.True(t, env.running.Load(), "the server is running again")
	assert.Equal(t, 1, strings.Count(consoleText(p), "Waiting for a backup in progress to finish"), "said once, not on every check")
}

func TestRestartTask_GivesUpWithoutStoppingTheServer(t *testing.T) {
	p, env := newPowerTestServer(t)
	captureErrors(t)
	useBackupWait(t, 5*time.Millisecond, 50*time.Millisecond)
	p.backupState.Store(backupRunning)
	defer p.backupState.Store(backupIdle)

	err := runOperation(p, "restart", nil)

	assert.ErrorIs(t, err, skypanel.ErrBackupInProgress)
	assert.Equal(t, int32(0), env.stops.Load(), "a server it could not start again is left running")
	assert.True(t, env.running.Load())
}

func TestStartTask_WaitsForABackupInProgress(t *testing.T) {
	p, env := newPowerTestServer(t)
	env.running.Store(false)
	useBackupWait(t, 5*time.Millisecond, time.Minute)
	p.backupState.Store(restoreRunning)

	done := runOperationInBackground(p, "start", nil)

	assert.Never(t, func() bool { return env.starts.Load() > 0 }, 100*time.Millisecond, 5*time.Millisecond)
	p.backupState.Store(backupIdle)
	require.NoError(t, waitForStep(t, done), "a stop, backup, start task does not leave the server stopped")
	assert.Equal(t, int32(1), env.starts.Load())
}

func TestBackupTask_WaitsForABackupInProgress(t *testing.T) {
	p := newCrashTestServer(t)
	useLoadedServers(t, p)
	useBackupWait(t, 5*time.Millisecond, time.Minute)
	p.backupState.Store(backupRunning)

	done := runOperationInBackground(p, "backup", map[string]interface{}{"keep": 5})

	assert.Never(t, func() bool { return len(backupFiles(t, p)) > 0 }, 100*time.Millisecond, 5*time.Millisecond,
		"the step waits instead of failing")
	p.backupState.Store(backupIdle)
	require.NoError(t, waitForStep(t, done))
	assert.Len(t, backupNames(t, p), 1)
}

func TestStartTask_AServerDeletedWhileWaitingIsNotStarted(t *testing.T) {
	p, env := newPowerTestServer(t)
	env.running.Store(false)
	useBackupWait(t, 5*time.Millisecond, time.Minute)
	p.backupState.Store(backupRunning)

	done := runOperationInBackground(p, "start", nil)
	require.Eventually(t, func() bool { return strings.Contains(consoleText(p), "Waiting for a backup") }, 15*time.Second, 5*time.Millisecond)
	useLoadedServers(t) // deleted while the step waits
	p.backupState.Store(backupIdle)

	assert.ErrorIs(t, waitForStep(t, done), errServerDeleted)
	assert.Equal(t, int32(0), env.starts.Load(), "a deleted server is never started")
}

func TestBackupTask_AServerDeletedWhileWaitingMakesNoBackup(t *testing.T) {
	p := newCrashTestServer(t)
	useLoadedServers(t, p)
	useBackupWait(t, 5*time.Millisecond, time.Minute)
	p.backupState.Store(backupRunning)

	done := runOperationInBackground(p, "backup", map[string]interface{}{"keep": 5})
	require.Eventually(t, func() bool { return strings.Contains(consoleText(p), "Waiting for a backup") }, 15*time.Second, 5*time.Millisecond)
	useLoadedServers(t)
	p.backupState.Store(backupIdle)

	assert.ErrorIs(t, waitForStep(t, done), errServerDeleted)
	assert.NoDirExists(t, p.GetBackupDirectory(), "the deleted server's backup folder is not made again")
}
