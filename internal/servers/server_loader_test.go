package servers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
)

func useServersFolder(t *testing.T) string {
	dir := t.TempDir()
	previous := config.ServersFolder.Value()
	_ = config.ServersFolder.Set(dir, false)
	t.Cleanup(func() { _ = config.ServersFolder.Set(previous, false) })
	return dir
}

func TestLoad_AnEmptyDefinitionIsAnError(t *testing.T) {
	dir := useServersFolder(t)
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "empty01.json"), nil, 0644))

	server, err := Load("empty01")
	assert.Error(t, err, "an empty file must not load as a nil server, which crashed the boot")
	assert.Nil(t, server)
}

func TestWriteFileAtomic_ReplacesTheFileAndLeavesNoTemporaries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "srv.json")
	assert.NoError(t, os.WriteFile(path, []byte("old"), 0600))

	assert.NoError(t, writeFileAtomic(path, []byte("new"), 0664))

	content, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "new", string(content))
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1, "the temporary file is renamed over, not left behind")
}

func TestForgetServerState_DropsOnlyThatServer(t *testing.T) {
	for _, id := range []string{"forgetgone", "forgetkept"} {
		stateTrackingLock.Lock()
		serverStateTracking[id] = &serverState{lastAlert: map[string]time.Time{}}
		stateTrackingLock.Unlock()
		queryLock.Lock()
		queryCache[id] = cachedQuery{ok: true, at: time.Now()}
		sourceQueryMisses[id] = time.Now()
		queryLock.Unlock()
	}
	t.Cleanup(func() { forgetServerState("forgetkept") })

	forgetServerState("forgetgone")

	stateTrackingLock.RLock()
	_, gone := serverStateTracking["forgetgone"]
	_, kept := serverStateTracking["forgetkept"]
	stateTrackingLock.RUnlock()
	assert.False(t, gone)
	assert.True(t, kept)

	queryLock.Lock()
	_, goneQuery := queryCache["forgetgone"]
	_, goneMiss := sourceQueryMisses["forgetgone"]
	_, keptQuery := queryCache["forgetkept"]
	queryLock.Unlock()
	assert.False(t, goneQuery)
	assert.False(t, goneMiss)
	assert.True(t, keptQuery)
}

func TestDelete_ForgetsTheServersInMemoryState(t *testing.T) {
	p := newCrashTestServer(t)
	p.Scheduler = NewDefaultScheduler(p.ID())
	p.RunningEnvironment.RootDirectory = filepath.Join(config.ServersFolder.Value(), p.ID())
	assert.NoError(t, os.MkdirAll(p.RunningEnvironment.RootDirectory, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(config.ServersFolder.Value(), p.ID()+".json"), []byte("{}"), 0o600))
	useLoadedServers(t, p)
	t.Cleanup(func() { forgetServerState(p.ID()) })

	stateTrackingLock.Lock()
	serverStateTracking[p.ID()] = &serverState{lastAlert: map[string]time.Time{}}
	stateTrackingLock.Unlock()
	queryLock.Lock()
	queryCache[p.ID()] = cachedQuery{ok: true, at: time.Now()}
	queryLock.Unlock()

	if !assert.NoError(t, Delete(p.ID())) {
		return
	}

	assert.Nil(t, GetFromCache(p.ID()))
	stateTrackingLock.RLock()
	_, tracked := serverStateTracking[p.ID()]
	stateTrackingLock.RUnlock()
	queryLock.Lock()
	_, cached := queryCache[p.ID()]
	queryLock.Unlock()
	assert.False(t, tracked, "a server made later with this id must not inherit its alert state")
	assert.False(t, cached)
}

func TestDelete_AMissingDefinitionStillSucceeds(t *testing.T) {
	p := newCrashTestServer(t)
	p.Scheduler = NewDefaultScheduler(p.ID())
	p.RunningEnvironment.RootDirectory = filepath.Join(config.ServersFolder.Value(), p.ID())
	assert.NoError(t, os.MkdirAll(p.RunningEnvironment.RootDirectory, 0o755))
	useLoadedServers(t, p)
	t.Cleanup(func() { forgetServerState(p.ID()) })

	// everything of the server goes; a .json already missing must not fail the delete, or
	// the panel keeps the server it asked the node to remove
	assert.NoError(t, Delete(p.ID()))
	assert.Nil(t, GetFromCache(p.ID()))
	assert.NoDirExists(t, p.RunningEnvironment.RootDirectory)
}

func TestDelete_ADefinitionThatCannotGoIsReportedAfterTheCleanUp(t *testing.T) {
	p := newCrashTestServer(t)
	p.Scheduler = NewDefaultScheduler(p.ID())
	p.RunningEnvironment.RootDirectory = filepath.Join(config.ServersFolder.Value(), p.ID())
	assert.NoError(t, os.MkdirAll(p.RunningEnvironment.RootDirectory, 0o755))
	// a folder with something in it where the .json should be: os.Remove cannot take it
	stuck := filepath.Join(config.ServersFolder.Value(), p.ID()+".json")
	assert.NoError(t, os.MkdirAll(filepath.Join(stuck, "inside"), 0o755))
	useLoadedServers(t, p)
	t.Cleanup(func() { forgetServerState(p.ID()) })

	assert.Error(t, Delete(p.ID()), "the panel must keep the row while the .json is still there")
	assert.Nil(t, GetFromCache(p.ID()), "the rest of the clean-up still runs")
	assert.NoDirExists(t, p.RunningEnvironment.RootDirectory)
}
