package servers

import (
	"io"
	"sync"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

// reattachEnvironment is a running container the daemon lost track of: re-attaching gives it
// a console and remembers the exit callback, as the docker environment does
type reattachEnvironment struct {
	fakeEnvironment
	mu       sync.Mutex
	attaches int
	callback func(int)
	fail     error
}

type discardConn struct{ io.Writer }

func (discardConn) Close() error { return nil }

func (r *reattachEnvironment) ReattachImpl(env *skypanel.Environment, steps skypanel.ExecutionData) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail != nil {
		return r.fail
	}
	r.attaches++
	r.callback = steps.Callback
	env.CreateConsoleStdinProxy(steps.StdInConfig, discardConn{io.Discard})
	return nil
}

func newDetachedServer(t *testing.T, running bool) (*Server, *reattachEnvironment) {
	p, _ := newBackupTestServer(t)
	impl := &reattachEnvironment{fakeEnvironment: fakeEnvironment{running: running}}
	p.RunningEnvironment.Implementation = impl
	p.RunningEnvironment.Wait = &sync.WaitGroup{}
	p.Execution.Command = "java -jar server.jar"
	return p, impl
}

func TestReattach_ReconnectsARunningServerOnce(t *testing.T) {
	p, impl := newDetachedServer(t, true)

	assert.NoError(t, p.Reattach())
	assert.NoError(t, p.Reattach(), "a second call finds the console and does nothing")

	assert.Equal(t, 1, impl.attaches)
	assert.NotNil(t, p.RunningEnvironment.Console, "commands can be sent again")
	assert.NotNil(t, impl.callback, "the exit goes through afterExit like a normal run")
	assert.Contains(t, consoleText(p), "Reconnected to the running server")
}

func TestReattach_LeavesStoppedServersAlone(t *testing.T) {
	p, impl := newDetachedServer(t, false)

	assert.NoError(t, p.Reattach())

	assert.Zero(t, impl.attaches)
	assert.Nil(t, p.RunningEnvironment.Console)
}

func TestReattach_HostProcessesCannotBeReached(t *testing.T) {
	p, _ := newBackupTestServer(t)
	p.RunningEnvironment.Implementation = &fakeEnvironment{running: true}
	p.Execution.Command = "./server"

	assert.ErrorIs(t, p.Reattach(), skypanel.ErrNotImplemented)
	assert.Nil(t, p.RunningEnvironment.Console, "it stays detached: commands answer ErrConsoleDetached")
}

func TestReattach_AFailedAttachKeepsTodaysState(t *testing.T) {
	p, impl := newDetachedServer(t, true)
	impl.fail = skypanel.ErrServerOffline

	assert.ErrorIs(t, p.Reattach(), skypanel.ErrServerOffline)
	assert.Nil(t, p.RunningEnvironment.Console)
	assert.ErrorIs(t, p.RunningEnvironment.ExecuteInMainProcess("say hi"), skypanel.ErrConsoleDetached)
}

func TestCommandsReattachFirst(t *testing.T) {
	p, impl := newDetachedServer(t, true)

	assert.NoError(t, p.Execute("say hello"), "the command reaches the server after reconnecting")
	assert.Equal(t, 1, impl.attaches)
}

func TestStopReattachesFirstSoTheExitIsSeen(t *testing.T) {
	p, impl := newDetachedServer(t, true)
	p.Execution.StopCommand = "stop"

	assert.NoError(t, p.Stop())

	assert.Equal(t, 1, impl.attaches)
	assert.True(t, p.stopRequested.Load(), "the coming exit counts as a requested stop, not a crash")
}
