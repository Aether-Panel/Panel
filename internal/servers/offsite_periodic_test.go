package servers

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// periodicNow is the fixed time the periodic tests run at
var periodicNow = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

// newPeriodicTestServer is a stopped server whose panel-wide periodic copy is every 24 hours,
// keeping 4. Its periodic state is forgotten afterwards, since other tests reuse its id.
func newPeriodicTestServer(t *testing.T) *Server {
	p := newCrashTestServer(t)
	usePeriodicGlobals(t, 24, 4)
	forgetPeriodic(p.ID())
	t.Cleanup(func() { forgetPeriodic(p.ID()) })
	return p
}

// countBackupEvents counts the backup alerts sent from now on, successful or not
func countBackupEvents(t *testing.T) *atomic.Int32 {
	var count atomic.Int32
	previous := EventSink
	EventSink = func(event string, _ map[string]interface{}) {
		if event == "server.backup" {
			count.Add(1)
		}
	}
	t.Cleanup(func() { EventSink = previous })
	return &count
}

// lockedBuffer is a buffer the logger and the test can share
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureErrors collects what is logged as an error from now on
func captureErrors(t *testing.T) *lockedBuffer {
	buf := &lockedBuffer{}
	previous := logging.Error.Writer()
	logging.Error.SetOutput(buf)
	t.Cleanup(func() { logging.Error.SetOutput(previous) })
	return buf
}

// backupFiles lists every file in the server's backup folder, hidden ones included
func backupFiles(t *testing.T, p *Server) []string {
	entries, err := os.ReadDir(p.GetBackupDirectory())
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestPeriodic_UploadsWhenDueAndRemovesTheLocalCopy(t *testing.T) {
	fake := useFakeBucket(t)
	fake.setClock(func() time.Time { return periodicNow })
	p := newPeriodicTestServer(t)
	events := countBackupEvents(t)

	assert.True(t, p.runPeriodic(periodicNow), "an empty bucket means the copy is due")

	names := fake.names(p.ID())
	if assert.Len(t, names, 1) {
		assert.True(t, strings.HasPrefix(names[0], OffsiteBackupPrefix), names[0])
	}
	assert.Empty(t, backupFiles(t, p), "the periodic copy only lives in the bucket")
	assert.Equal(t, int32(0), events.Load(), "a copy that went well tells nobody")
	assert.NoError(t, p.IsIdle())
	last, ok := LastPeriodic(p.ID())
	assert.True(t, ok)
	assert.Equal(t, periodicNow, last)

	assert.False(t, p.runPeriodic(periodicNow.Add(10*time.Minute)), "the next one waits for the interval")
	assert.Equal(t, 1, fake.uploadCount())
}

func TestPeriodic_WaitsForTheInterval(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	fake.put(p.ID(), OffsiteBackupPrefix+"old.tar.gz", periodicNow.Add(-23*time.Hour))

	assert.False(t, p.runPeriodic(periodicNow))
	assert.Equal(t, 0, fake.uploadCount(), "the last copy in the bucket is only 23 hours old")
	last, ok := LastPeriodic(p.ID())
	assert.True(t, ok, "the last copy is read from the bucket the first time")
	assert.Equal(t, periodicNow.Add(-23*time.Hour), last)

	assert.True(t, p.runPeriodic(periodicNow.Add(time.Hour)))
	assert.Equal(t, 1, fake.uploadCount())
}

func TestPeriodic_SeedsOnlyFromItsOwnSeries(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	fake.put(p.ID(), taskBackupPrefix("t1")+"a.tar.gz", periodicNow.Add(-time.Hour))
	fake.put(p.ID(), "manual.tar.gz", periodicNow.Add(-time.Hour))

	assert.True(t, p.runPeriodic(periodicNow), "a task's copy or one made by hand is not a periodic copy")
}

func TestPeriodic_PerServerOffAndDefaults(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)

	require.NoError(t, p.SetOffsiteSettings(OffsiteSettings{PeriodicHours: intPtr(0)}))
	assert.False(t, p.runPeriodic(periodicNow), "the server turned it off although the panel has 24 hours")
	assert.Equal(t, 0, fake.uploadCount())

	require.NoError(t, p.SetOffsiteSettings(OffsiteSettings{}))
	assert.True(t, p.runPeriodic(periodicNow), "without its own value it follows the panel")
}

func TestPeriodic_UnreadableOwnSettingsSkipTheServer(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	logged := captureErrors(t)
	require.NoError(t, os.WriteFile(filepath.Join(config.ServersFolder.Value(), p.ID()+".offsite"), []byte("{broken"), 0600))

	assert.False(t, p.runPeriodic(periodicNow), "the server may have turned it off; the panel's value is not a safe guess")
	assert.False(t, p.runPeriodic(periodicNow.Add(10*time.Minute)))
	assert.Equal(t, 0, fake.uploadCount())
	assert.Equal(t, 1, strings.Count(logged.String(), "off-site settings"), "the same error is logged once, not on every pass:\n%s", logged.String())

	require.NoError(t, p.SetOffsiteSettings(OffsiteSettings{}))
	assert.True(t, p.runPeriodic(periodicNow.Add(20*time.Minute)), "readable again, it copies")
}

func TestPeriodic_SkipsWhenTheBucketIsNotConfigured(t *testing.T) {
	fake := useFakeBucket(t)
	fake.disabled.Store(true)
	p := newPeriodicTestServer(t)

	assert.False(t, p.runPeriodic(periodicNow))
	assert.Equal(t, 0, fake.uploadCount())
	assert.Empty(t, backupFiles(t, p))
}

func TestPeriodic_SkipsBusyOrPausedServers(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)

	p.Scheduler = &Scheduler{Paused: true}
	assert.False(t, p.runPeriodic(periodicNow), "a server being moved")
	p.Scheduler = nil

	p.RunningEnvironment.Installing = true
	assert.False(t, p.runPeriodic(periodicNow), "a server being installed")
	p.RunningEnvironment.Installing = false

	p.backupState.Store(backupRunning)
	assert.False(t, p.runPeriodic(periodicNow), "another backup is running")
	p.backupState.Store(restoreRunning)
	assert.False(t, p.runPeriodic(periodicNow), "a restore is running")
	p.backupState.Store(backupIdle)

	assert.Equal(t, 0, fake.uploadCount())
	assert.Empty(t, backupFiles(t, p))
}

func TestPeriodic_RunsWithTheGameRunning(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	p.RunningEnvironment.Implementation.(*fakeEnvironment).running = true

	assert.True(t, p.runPeriodic(periodicNow), "the copy is taken hot, like one made by hand")
	assert.Equal(t, 1, fake.uploadCount())
}

func TestPeriodic_KeepsOnlyItsOwnSeries(t *testing.T) {
	fake := useFakeBucket(t)
	fake.setClock(func() time.Time { return periodicNow })
	p := newPeriodicTestServer(t)
	usePeriodicGlobals(t, 24, 2)
	fake.put(p.ID(), OffsiteBackupPrefix+"a.tar.gz", periodicNow.Add(-75*time.Hour))
	fake.put(p.ID(), OffsiteBackupPrefix+"b.tar.gz", periodicNow.Add(-50*time.Hour))
	fake.put(p.ID(), OffsiteBackupPrefix+"c.tar.gz", periodicNow.Add(-25*time.Hour))
	fake.put(p.ID(), taskBackupPrefix("t1")+"x.tar.gz", periodicNow.Add(-90*time.Hour))
	fake.put(p.ID(), taskBackupPrefix("t1")+"y.tar.gz", periodicNow.Add(-80*time.Hour))
	fake.put(p.ID(), "manual.tar.gz", periodicNow.Add(-100*time.Hour))

	assert.True(t, p.runPeriodic(periodicNow))

	var periodicCopies, others []string
	for _, name := range fake.names(p.ID()) {
		if strings.HasPrefix(name, OffsiteBackupPrefix) {
			periodicCopies = append(periodicCopies, name)
		} else {
			others = append(others, name)
		}
	}
	assert.Len(t, periodicCopies, 2)
	assert.Contains(t, periodicCopies, OffsiteBackupPrefix+"c.tar.gz", "the newest older copy stays with the new one")
	assert.Equal(t, []string{taskBackupPrefix("t1") + "x.tar.gz", taskBackupPrefix("t1") + "y.tar.gz", "manual.tar.gz"}, others,
		"copies of a task or made by hand are never pruned by the periodic series")
}

func TestPeriodic_UnreachableBucketWritesNothingAndBacksOff(t *testing.T) {
	fake := useFakeBucket(t)
	fake.failReach.Store(true)
	p := newPeriodicTestServer(t)
	events := countBackupEvents(t)
	captureErrors(t)

	now := periodicNow
	assert.False(t, p.runPeriodic(now))
	assert.Empty(t, backupFiles(t, p), "nothing is written while the bucket does not answer")
	assert.Equal(t, now.Add(time.Hour), periodicOf(p.ID()).retryAt)
	assert.Equal(t, int32(1), events.Load(), "a failure tells the owner")

	assert.False(t, p.runPeriodic(now.Add(30*time.Minute)), "it waits until retryAt")
	assert.Equal(t, now.Add(time.Hour), periodicOf(p.ID()).retryAt, "a skipped pass is not another failure")
	assert.Equal(t, int32(1), events.Load())

	now = now.Add(time.Hour)
	assert.False(t, p.runPeriodic(now))
	assert.Equal(t, now.Add(2*time.Hour), periodicOf(p.ID()).retryAt)

	for i := 0; i < 4; i++ {
		now = periodicOf(p.ID()).retryAt
		assert.False(t, p.runPeriodic(now))
	}
	assert.Equal(t, 6, periodicOf(p.ID()).failures)
	assert.Equal(t, now.Add(24*time.Hour), periodicOf(p.ID()).retryAt, "the wait never goes beyond a day")
	assert.Equal(t, 0, fake.uploadCount())
	assert.Empty(t, backupFiles(t, p))

	fake.failReach.Store(false)
	now = periodicOf(p.ID()).retryAt
	assert.True(t, p.runPeriodic(now))
	state := periodicOf(p.ID())
	assert.Equal(t, 0, state.failures, "a good upload resets the wait")
	assert.True(t, state.retryAt.IsZero())
}

func TestPeriodic_AFailedListingCountsAsAFailure(t *testing.T) {
	fake := useFakeBucket(t)
	fake.failList.Store(true)
	p := newPeriodicTestServer(t)
	captureErrors(t)

	assert.False(t, p.runPeriodic(periodicNow))
	assert.Equal(t, 0, fake.uploadCount())
	assert.Equal(t, periodicNow.Add(time.Hour), periodicOf(p.ID()).retryAt)
	_, ok := LastPeriodic(p.ID())
	assert.False(t, ok, "the bucket was never read")
}

func TestPeriodic_FailedUploadStillRemovesTheLocalCopy(t *testing.T) {
	fake := useFakeBucket(t)
	fake.failUpload.Store(true)
	p := newPeriodicTestServer(t)
	events := countBackupEvents(t)
	captureErrors(t)

	assert.True(t, p.runPeriodic(periodicNow), "the copy was made, only its upload failed")

	assert.Equal(t, 1, fake.uploadCount())
	assert.Empty(t, fake.names(p.ID()))
	assert.Empty(t, backupFiles(t, p), "retries must not fill the node's disk")
	assert.Equal(t, int32(1), events.Load(), "a failed upload tells the owner")
	assert.Equal(t, periodicNow.Add(time.Hour), periodicOf(p.ID()).retryAt)
	_, ok := LastPeriodic(p.ID())
	assert.False(t, ok, "no periodic copy reached the bucket")
	assert.NoError(t, p.IsIdle())
}

func TestPeriodic_ABackupThatCannotStartIsAnnouncedOnce(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	events := countBackupEvents(t)
	captureErrors(t)
	// a file where the backup folder should be: the backup cannot start
	require.NoError(t, os.MkdirAll(filepath.Dir(p.GetBackupDirectory()), 0755))
	require.NoError(t, os.WriteFile(p.GetBackupDirectory(), nil, 0600))

	assert.False(t, p.runPeriodic(periodicNow))

	assert.Equal(t, 0, fake.uploadCount())
	assert.Equal(t, int32(1), events.Load(), "only once, although the backup itself already failed")
	assert.Equal(t, periodicNow.Add(time.Hour), periodicOf(p.ID()).retryAt)
	assert.NoError(t, p.IsIdle())
}

func TestPeriodicPass_GoesThroughEveryServer(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	allServersLock.Lock()
	previous := allServers
	allServers = []*Server{p}
	allServersLock.Unlock()
	t.Cleanup(func() {
		allServersLock.Lock()
		allServers = previous
		allServersLock.Unlock()
	})

	periodicPass(periodicNow)

	assert.Equal(t, 1, fake.uploadCount())
}

func TestForgetPeriodic_ClearsTheState(t *testing.T) {
	updatePeriodic("gone", func(s *periodicState) { s.last = periodicNow; s.failures = 3 })

	forgetPeriodic("gone")

	_, ok := LastPeriodic("gone")
	assert.False(t, ok)
	assert.Equal(t, periodicState{}, periodicOf("gone"), "a server created again with the id starts afresh")
}
