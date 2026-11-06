package servers

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// periodicNow is the fixed time the periodic tests run at
var periodicNow = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

// at is a clock stopped at now
func at(now time.Time) func() time.Time {
	return func() time.Time { return now }
}

// newPeriodicTestServer is a stopped server, the only one loaded, whose panel-wide periodic
// copy is every 24 hours, keeping 4. Its periodic state is forgotten afterwards, since other
// tests reuse its id.
func newPeriodicTestServer(t *testing.T) *Server {
	p := newCrashTestServer(t)
	usePeriodicGlobals(t, 24, 4)
	useLoadedServers(t, p)
	forgetPeriodic(p.ID())
	t.Cleanup(func() { forgetPeriodic(p.ID()) })
	return p
}

// useLoadedServers makes these the servers the node runs, for this test
func useLoadedServers(t *testing.T, servers ...*Server) {
	allServersLock.Lock()
	previous := allServers
	allServers = servers
	allServersLock.Unlock()
	t.Cleanup(func() {
		allServersLock.Lock()
		allServers = previous
		allServersLock.Unlock()
	})
}

// hasPeriodicState says whether the node keeps any periodic state for the server id
func hasPeriodicState(serverID string) bool {
	periodic.Lock()
	defer periodic.Unlock()
	_, ok := periodic.states[serverID]
	return ok
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

	assert.True(t, p.runPeriodic(at(periodicNow)), "an empty bucket means the copy is due")

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

	assert.False(t, p.runPeriodic(at(periodicNow.Add(10*time.Minute))), "the next one waits for the interval")
	assert.Equal(t, 1, fake.uploadCount())
}

func TestPeriodic_WaitsForTheInterval(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	fake.put(p.ID(), OffsiteBackupPrefix+"old.tar.gz", periodicNow.Add(-23*time.Hour))

	assert.False(t, p.runPeriodic(at(periodicNow)))
	assert.Equal(t, 0, fake.uploadCount(), "the last copy in the bucket is only 23 hours old")
	last, ok := LastPeriodic(p.ID())
	assert.True(t, ok, "the last copy is read from the bucket the first time")
	assert.Equal(t, periodicNow.Add(-23*time.Hour), last)

	assert.True(t, p.runPeriodic(at(periodicNow.Add(time.Hour))))
	assert.Equal(t, 1, fake.uploadCount())
}

func TestPeriodic_SeedsOnlyFromItsOwnSeries(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	fake.put(p.ID(), taskBackupPrefix("t1")+"a.tar.gz", periodicNow.Add(-time.Hour))
	fake.put(p.ID(), "manual.tar.gz", periodicNow.Add(-time.Hour))

	assert.True(t, p.runPeriodic(at(periodicNow)), "a task's copy or one made by hand is not a periodic copy")
}

func TestPeriodic_PerServerOffAndDefaults(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)

	require.NoError(t, p.SetOffsiteSettings(OffsiteSettings{PeriodicHours: intPtr(0)}))
	assert.False(t, p.runPeriodic(at(periodicNow)), "the server turned it off although the panel has 24 hours")
	assert.Equal(t, 0, fake.uploadCount())

	require.NoError(t, p.SetOffsiteSettings(OffsiteSettings{}))
	assert.True(t, p.runPeriodic(at(periodicNow)), "without its own value it follows the panel")
}

func TestPeriodic_UnreadableOwnSettingsSkipTheServer(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	logged := captureErrors(t)
	require.NoError(t, os.WriteFile(filepath.Join(config.ServersFolder.Value(), p.ID()+".offsite"), []byte("{broken"), 0600))

	assert.False(t, p.runPeriodic(at(periodicNow)), "the server may have turned it off; the panel's value is not a safe guess")
	assert.False(t, p.runPeriodic(at(periodicNow.Add(10*time.Minute))))
	assert.Equal(t, 0, fake.uploadCount())
	assert.Equal(t, 1, strings.Count(logged.String(), "off-site settings"), "the same error is logged once, not on every pass:\n%s", logged.String())

	require.NoError(t, p.SetOffsiteSettings(OffsiteSettings{}))
	assert.True(t, p.runPeriodic(at(periodicNow.Add(20*time.Minute))), "readable again, it copies")
}

func TestPeriodic_SkipsWhenTheBucketIsNotConfigured(t *testing.T) {
	fake := useFakeBucket(t)
	fake.disabled.Store(true)
	p := newPeriodicTestServer(t)

	assert.False(t, p.runPeriodic(at(periodicNow)))
	assert.Equal(t, 0, fake.uploadCount())
	assert.Empty(t, backupFiles(t, p))
}

func TestPeriodic_SkipsBusyOrPausedServers(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)

	p.Scheduler = &Scheduler{Paused: true}
	assert.False(t, p.runPeriodic(at(periodicNow)), "a server being moved")
	p.Scheduler = nil

	p.RunningEnvironment.Installing = true
	assert.False(t, p.runPeriodic(at(periodicNow)), "a server being installed")
	p.RunningEnvironment.Installing = false

	p.backupState.Store(backupRunning)
	assert.False(t, p.runPeriodic(at(periodicNow)), "another backup is running")
	p.backupState.Store(restoreRunning)
	assert.False(t, p.runPeriodic(at(periodicNow)), "a restore is running")
	p.backupState.Store(backupIdle)

	assert.Equal(t, 0, fake.uploadCount())
	assert.Empty(t, backupFiles(t, p))
}

func TestPeriodic_RunsWithTheGameRunning(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	p.RunningEnvironment.Implementation.(*fakeEnvironment).running = true

	assert.True(t, p.runPeriodic(at(periodicNow)), "the copy is taken hot, like one made by hand")
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

	assert.True(t, p.runPeriodic(at(periodicNow)))

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
	assert.False(t, p.runPeriodic(at(now)))
	assert.Empty(t, backupFiles(t, p), "nothing is written while the bucket does not answer")
	assert.Equal(t, now.Add(time.Hour), periodicOf(p.ID()).retryAt)
	assert.Equal(t, int32(1), events.Load(), "a failure tells the owner")

	assert.False(t, p.runPeriodic(at(now.Add(30*time.Minute))), "it waits until retryAt")
	assert.Equal(t, now.Add(time.Hour), periodicOf(p.ID()).retryAt, "a skipped pass is not another failure")
	assert.Equal(t, int32(1), events.Load())

	now = now.Add(time.Hour)
	assert.False(t, p.runPeriodic(at(now)))
	assert.Equal(t, now.Add(2*time.Hour), periodicOf(p.ID()).retryAt)

	for i := 0; i < 4; i++ {
		now = periodicOf(p.ID()).retryAt
		assert.False(t, p.runPeriodic(at(now)))
	}
	assert.Equal(t, 6, periodicOf(p.ID()).failures)
	assert.Equal(t, now.Add(24*time.Hour), periodicOf(p.ID()).retryAt, "the wait never goes beyond a day")
	assert.Equal(t, 0, fake.uploadCount())
	assert.Empty(t, backupFiles(t, p))

	fake.failReach.Store(false)
	now = periodicOf(p.ID()).retryAt
	assert.True(t, p.runPeriodic(at(now)))
	state := periodicOf(p.ID())
	assert.Equal(t, 0, state.failures, "a good upload resets the wait")
	assert.True(t, state.retryAt.IsZero())
}

func TestPeriodic_AFailedListingCountsAsAFailure(t *testing.T) {
	fake := useFakeBucket(t)
	fake.failList.Store(true)
	p := newPeriodicTestServer(t)
	captureErrors(t)

	assert.False(t, p.runPeriodic(at(periodicNow)))
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

	assert.True(t, p.runPeriodic(at(periodicNow)), "the copy was made, only its upload failed")

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

	assert.False(t, p.runPeriodic(at(periodicNow)))

	assert.Equal(t, 0, fake.uploadCount())
	assert.Equal(t, int32(1), events.Load(), "only once, although the backup itself already failed")
	assert.Equal(t, periodicNow.Add(time.Hour), periodicOf(p.ID()).retryAt)
	assert.NoError(t, p.IsIdle())
}

func TestPeriodicPass_GoesThroughEveryServer(t *testing.T) {
	fake := useFakeBucket(t)
	newPeriodicTestServer(t)

	periodicPass(at(periodicNow))

	assert.Equal(t, 1, fake.uploadCount())
}

func TestPeriodic_SkipsAServerDeletedDuringThePass(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	events := countBackupEvents(t)
	// the pass took its list before the server was deleted, as Delete does it: out of the
	// cache, then its state forgotten
	useLoadedServers(t)
	forgetPeriodic(p.ID())

	assert.False(t, p.runPeriodic(at(periodicNow)))

	assert.Equal(t, 0, fake.uploadCount())
	assert.Equal(t, int32(0), events.Load())
	assert.NoDirExists(t, p.GetBackupDirectory(), "the deleted server's backup folder is not made again")
	assert.False(t, hasPeriodicState(p.ID()), "nothing is kept for an id that may be created again")
}

func TestPeriodic_SkipsAServerReplacedByANewOneWithItsID(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	again := CreateProgram()
	again.Identifier = p.ID()
	useLoadedServers(t, again)

	assert.False(t, p.runPeriodic(at(periodicNow)), "the old server is not the one the node runs")
	assert.Equal(t, 0, fake.uploadCount())
	assert.False(t, hasPeriodicState(p.ID()))
}

func TestPeriodic_AFailureOfADeletedServerIsNotRecorded(t *testing.T) {
	p := newPeriodicTestServer(t)
	useLoadedServers(t)

	assert.False(t, p.recordPeriodicFailure(periodicNow))
	assert.False(t, p.updatePeriodic(func(s *periodicState) { s.last = periodicNow }))
	assert.False(t, hasPeriodicState(p.ID()))
}

func TestForgetPeriodic_ClearsTheState(t *testing.T) {
	p := newPeriodicTestServer(t)
	p.updatePeriodic(func(s *periodicState) { s.last = periodicNow; s.failures = 3 })

	forgetPeriodic(p.ID())

	_, ok := LastPeriodic(p.ID())
	assert.False(t, ok)
	assert.False(t, hasPeriodicState(p.ID()), "a server created again with the id starts afresh")
}

func TestSweepPeriodicLeftovers_RemovesOnlyPeriodicArchives(t *testing.T) {
	p := newPeriodicTestServer(t)
	require.NoError(t, os.MkdirAll(p.GetBackupDirectory(), 0755))
	for _, name := range []string{
		OffsiteBackupPrefix + "a.tar.gz", partialBackupPrefix + OffsiteBackupPrefix + "b.tar.gz",
		taskBackupPrefix("t1") + "x.tar.gz", "manual.tar.gz", partialBackupPrefix + "manual.tar.gz",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(p.GetBackupDirectory(), name), []byte("x"), 0600))
	}

	sweepPeriodicLeftovers()

	assert.ElementsMatch(t, []string{taskBackupPrefix("t1") + "x.tar.gz", "manual.tar.gz", partialBackupPrefix + "manual.tar.gz"},
		backupFiles(t, p), "a restart mid-upload leaves the periodic archive behind; nothing else is touched")
}

// startThenLater is a clock at start for its first reading and at later afterwards, as when
// writing and uploading a copy takes that long
func startThenLater(start, later time.Time) func() time.Time {
	var calls atomic.Int32
	return func() time.Time {
		if calls.Add(1) == 1 {
			return start
		}
		return later
	}
}

func TestPeriodic_WaitsFromWhenTheAttemptEnded(t *testing.T) {
	fake := useFakeBucket(t)
	fake.failUpload.Store(true)
	p := newPeriodicTestServer(t)
	captureErrors(t)
	ended := periodicNow.Add(2 * time.Hour)

	assert.True(t, p.runPeriodic(startThenLater(periodicNow, ended)))
	assert.Equal(t, ended.Add(time.Hour), periodicOf(p.ID()).retryAt, "a long upload must not eat the wait")

	fake.failUpload.Store(false)
	done := ended.Add(3 * time.Hour)
	assert.True(t, p.runPeriodic(startThenLater(ended.Add(time.Hour), done)))
	last, _ := LastPeriodic(p.ID())
	assert.Equal(t, done, last, "the interval counts from when the copy reached the bucket")
}

func TestPeriodic_FailuresCountOnlyInARow(t *testing.T) {
	fake := useFakeBucket(t)
	fake.failList.Store(true)
	p := newPeriodicTestServer(t)
	captureErrors(t)
	fake.put(p.ID(), OffsiteBackupPrefix+"recent.tar.gz", periodicNow.Add(-time.Hour))

	assert.False(t, p.runPeriodic(at(periodicNow)))
	assert.Equal(t, 1, periodicOf(p.ID()).failures)

	fake.failList.Store(false)
	assert.False(t, p.runPeriodic(at(periodicNow.Add(time.Hour))), "the bucket answers, and the last copy is recent")
	assert.Equal(t, 0, periodicOf(p.ID()).failures, "the run of failures is over")

	fake.failReach.Store(true)
	later := periodicNow.Add(30 * time.Hour)
	assert.False(t, p.runPeriodic(at(later)))
	assert.Equal(t, later.Add(time.Hour), periodicOf(p.ID()).retryAt, "a failure a day later waits 1 hour, not 2")
}

func TestPeriodic_ATaskNamedPeriodicIsAnotherSeries(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	require.True(t, startSeries(p.ID(), "periodic"), "a task with the id periodic is uploading")
	defer endSeries(p.ID(), "periodic")

	assert.True(t, p.runPeriodic(at(periodicNow)))
	assert.Equal(t, 1, fake.uploadCount())
}

// unreadableFiles are a server's files whose folders cannot be listed, so its archive cannot
// be written. then, when set, runs first, as something that happens while the archive is made.
type unreadableFiles struct {
	files.FileServer
	then func()
}

func (f unreadableFiles) ReadDir(string) ([]fs.DirEntry, error) {
	if f.then != nil {
		f.then()
	}
	return nil, errors.New("fake unreadable folder")
}

// deleteDuringTheTest takes the server off the node as Delete does: out of the cache, then its
// periodic state forgotten
func deleteDuringTheTest(t *testing.T, p *Server) {
	useLoadedServers(t)
	forgetPeriodic(p.ID())
}

func TestPeriodic_AnArchiveThatCannotBeWrittenIsAnnouncedOnce(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	events := countBackupEvents(t)
	captureErrors(t)
	p.SetFileServer(unreadableFiles{FileServer: p.GetFileServer()})

	assert.False(t, p.runPeriodic(at(periodicNow)))

	assert.Equal(t, 0, fake.uploadCount())
	assert.Equal(t, int32(1), events.Load(), "the owner hears of it once")
	assert.Equal(t, periodicNow.Add(time.Hour), periodicOf(p.ID()).retryAt)
	assert.Empty(t, backupFiles(t, p), "nothing is left half written")
	assert.NoError(t, p.IsIdle())
}

func TestPeriodic_AServerDeletedDuringItsArchiveIsNotAnnounced(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	events := countBackupEvents(t)
	captureErrors(t)
	// deleting a server removes its files, so the archive under way fails
	p.SetFileServer(unreadableFiles{FileServer: p.GetFileServer(), then: func() { deleteDuringTheTest(t, p) }})

	assert.False(t, p.runPeriodic(at(periodicNow)))

	assert.Equal(t, 0, fake.uploadCount())
	assert.Equal(t, int32(0), events.Load(), "nobody is told a deleted server's copy failed")
	assert.False(t, hasPeriodicState(p.ID()))
	assert.NoError(t, p.IsIdle())
}

func TestPeriodic_AServerDeletedDuringItsUploadKeepsItsBucket(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	var older []string
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("%sold-%d.tar.gz", OffsiteBackupPrefix, i)
		fake.put(p.ID(), name, periodicNow.Add(-time.Duration(25+i)*time.Hour))
		older = append(older, name)
	}
	fake.holdUploads()
	made := make(chan bool, 1)
	go func() { made <- p.runPeriodic(at(periodicNow)) }()
	require.Eventually(t, func() bool { return fake.uploadCount() == 1 }, 15*time.Second, 5*time.Millisecond)

	deleteDuringTheTest(t, p)
	fake.release()
	select {
	case <-made:
	case <-time.After(15 * time.Second):
		t.Fatal("the periodic copy never ended")
	}

	names := fake.names(p.ID())
	assert.Len(t, names, 5, "the copy that was uploading and every older one stay")
	for _, name := range older {
		assert.Contains(t, names, name, "a server deleted meanwhile prunes nothing: its id may be created again")
	}
	assert.False(t, hasPeriodicState(p.ID()))
}

// dailyAt is a daily schedule at the clock time of when, in the node's timezone
func dailyAt(when time.Time) string {
	when = when.In(time.Local)
	return fmt.Sprintf("%d %d * * *", when.Minute(), when.Hour())
}

func TestPeriodic_SkipsAServerWithATaskDueSoon(t *testing.T) {
	fake := useFakeBucket(t)
	p := newPeriodicTestServer(t)
	s := newTestScheduler(t)
	s.Start()
	p.Scheduler = s
	// gocron runs on the real clock, whatever the clock the periodic copy is given
	require.NoError(t, s.SetTask("restart", restartTask(dailyAt(time.Now().Add(30*time.Minute)))))

	assert.False(t, p.runPeriodic(at(periodicNow)), "a restart in half an hour would find the copy under way")
	assert.Equal(t, 0, fake.uploadCount())

	require.NoError(t, s.SetTask("restart", restartTask(dailyAt(time.Now().Add(3*time.Hour)))))
	assert.True(t, p.runPeriodic(at(periodicNow)), "a task three hours away leaves time for the copy")
}
