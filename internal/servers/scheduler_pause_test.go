package servers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler_PauseSurvivesARestart(t *testing.T) {
	s := newTestScheduler(t)
	require.NoError(t, s.SetTask("restart", restartTask("0 5 * * *")))

	require.NoError(t, s.SetPaused(true))
	assert.True(t, s.IsPaused())

	loaded, err := LoadScheduler("tasktest")
	require.NoError(t, err)
	defer loaded.Stop()
	assert.True(t, loaded.IsPaused(), "a node that restarts during a transfer keeps the tasks paused")
	_, ok := loaded.GetTask("restart")
	assert.True(t, ok, "pausing keeps the tasks")

	require.NoError(t, loaded.SetPaused(false))
	again, err := LoadScheduler("tasktest")
	require.NoError(t, err)
	defer again.Stop()
	assert.False(t, again.IsPaused())
}

func TestRunScheduledTask_SkipsAPausedServer(t *testing.T) {
	p, _ := newBackupTestServer(t)
	s := newTestScheduler(t)
	p.Identifier = "tasktest"
	p.Scheduler = s
	require.NoError(t, s.SetTask("restart", restartTask("0 5 * * *")))
	require.NoError(t, s.SetPaused(true))

	previous := GetAll()
	addToCache(p)
	t.Cleanup(func() {
		allServersLock.Lock()
		allServers = previous
		allServersLock.Unlock()
	})

	runScheduledTask("tasktest", "restart")

	assert.NotContains(t, consoleText(p), "Running task", "a paused task does not run")
}
