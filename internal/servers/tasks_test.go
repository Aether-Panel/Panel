package servers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/go-co-op/gocron/v2"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
)

func newTestScheduler(t *testing.T) *Scheduler {
	previous := config.ServersFolder.Value()
	_ = config.ServersFolder.Set(t.TempDir(), false)
	t.Cleanup(func() { _ = config.ServersFolder.Set(previous, false) })

	s := NewDefaultScheduler("tasktest")
	if !assert.NoError(t, s.Init()) {
		t.FailNow()
	}
	t.Cleanup(s.Stop)
	return s
}

func restartTask(schedule string) skypanel.Task {
	op := skypanel.ConditionalMetadataType{MetadataType: skypanel.MetadataType{Type: "restart", Metadata: map[string]interface{}{}}}
	return skypanel.Task{Name: "Daily restart", CronSchedule: schedule, Operations: []skypanel.ConditionalMetadataType{op}}
}

func TestScheduler_SetTaskKeepsTheOldTaskWhenTheNewScheduleIsInvalid(t *testing.T) {
	s := newTestScheduler(t)
	assert.NoError(t, s.SetTask("restart", restartTask("CRON_TZ=Europe/Madrid 0 5 * * *")))

	err := s.SetTask("restart", restartTask("not a schedule"))
	assert.ErrorIs(t, err, ErrInvalidTask)

	task, ok := s.GetTask("restart")
	assert.True(t, ok)
	assert.Equal(t, "CRON_TZ=Europe/Madrid 0 5 * * *", task.CronSchedule)
	assert.Len(t, s.GetExecutor().Jobs(), 1)
}

func TestScheduler_SetTaskReplacesTheJob(t *testing.T) {
	s := newTestScheduler(t)
	assert.NoError(t, s.SetTask("restart", restartTask("0 5 * * *")))
	assert.NoError(t, s.SetTask("restart", restartTask("30 6 * * *")))

	assert.Len(t, s.GetExecutor().Jobs(), 1)
	task, _ := s.GetTask("restart")
	assert.Equal(t, "30 6 * * *", task.CronSchedule)
}

func TestScheduler_SetTaskRejectsBadIDsAndTooManyTasks(t *testing.T) {
	s := newTestScheduler(t)
	assert.ErrorIs(t, s.SetTask("../escape", restartTask("0 5 * * *")), ErrInvalidTask)
	assert.ErrorIs(t, s.SetTask("unnamed", skypanel.Task{CronSchedule: "0 5 * * *"}), ErrInvalidTask)

	for i := 0; i < MaxTasks; i++ {
		assert.NoError(t, s.SetTask("task-"+string(rune('a'+i/26))+string(rune('a'+i%26)), restartTask("0 5 * * *")))
	}
	assert.ErrorIs(t, s.SetTask("one-too-many", restartTask("0 5 * * *")), ErrInvalidTask)
	// editing an existing task is still fine at the limit
	assert.NoError(t, s.SetTask("task-aa", restartTask("0 6 * * *")))
}

func TestScheduler_RemoveTaskReportsMissingTasks(t *testing.T) {
	s := newTestScheduler(t)
	assert.ErrorIs(t, s.RemoveTask("missing"), gocron.ErrJobNotFound)

	assert.NoError(t, s.SetTask("restart", restartTask("0 5 * * *")))
	assert.NoError(t, s.RemoveTask("restart"))
	_, ok := s.GetTask("restart")
	assert.False(t, ok)
	assert.Empty(t, s.GetExecutor().Jobs())
}

func TestScheduler_TasksSurviveAReload(t *testing.T) {
	s := newTestScheduler(t)
	assert.NoError(t, s.SetTask("restart", restartTask("0 5 * * *")))

	loaded, err := LoadScheduler("tasktest")
	if !assert.NoError(t, err) {
		return
	}
	defer loaded.Stop()
	task, ok := loaded.GetTask("restart")
	assert.True(t, ok)
	assert.Equal(t, "restart", task.Operations[0].Type)
}

func writeBackupFile(t *testing.T, p *Server, name string, age time.Duration) {
	dir := p.GetBackupDirectory()
	assert.NoError(t, os.MkdirAll(dir, 0755))
	file := filepath.Join(dir, name)
	assert.NoError(t, os.WriteFile(file, []byte("archive"), 0644))
	when := time.Now().Add(-age)
	assert.NoError(t, os.Chtimes(file, when, when))
}

func backupNames(t *testing.T, p *Server) []string {
	backups, err := p.ListBackups()
	assert.NoError(t, err)
	names := make([]string, 0, len(backups))
	for _, b := range backups {
		names = append(names, b.FileName)
	}
	return names
}

func TestListBackups_NewestFirstAndOnlyFinishedArchives(t *testing.T) {
	p, _ := newBackupTestServer(t)
	assert.Empty(t, backupNames(t, p), "no backup folder yet")

	writeBackupFile(t, p, "old.tar.gz", 2*time.Hour)
	writeBackupFile(t, p, "new.tar.gz", time.Hour)
	writeBackupFile(t, p, ".partial-auto-x.tar.gz", 0)
	writeBackupFile(t, p, "notes.txt", 0)

	assert.Equal(t, []string{"new.tar.gz", "old.tar.gz"}, backupNames(t, p))
}

func TestRunAutoBackup_KeepsOnlyTheNewestAutomaticBackups(t *testing.T) {
	p, dir := newBackupTestServer(t)
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "world.dat"), []byte("world"), 0644))

	writeBackupFile(t, p, "manual-1.tar.gz", 5*time.Hour)
	writeBackupFile(t, p, AutoBackupPrefix+"oldest.tar.gz", 4*time.Hour)
	writeBackupFile(t, p, AutoBackupPrefix+"older.tar.gz", 3*time.Hour)

	name, err := p.RunAutoBackup(AutoBackupOptions{Keep: 2})
	if !assert.NoError(t, err) {
		return
	}
	assert.True(t, strings.HasPrefix(name, AutoBackupPrefix))
	assert.False(t, p.IsBackingUp(), "the task waits for the archive")
	assert.Contains(t, consoleText(p), "Backup complete")

	// the new one and the newest older one stay; the hand-made backup is never touched
	assert.ElementsMatch(t, []string{name, AutoBackupPrefix + "older.tar.gz", "manual-1.tar.gz"}, backupNames(t, p))
}

func TestRunAutoBackup_RefusesWhileAnotherBackupRuns(t *testing.T) {
	p, _ := newBackupTestServer(t)
	p.backupState.Store(backupRunning)
	defer p.backupState.Store(backupIdle)

	_, err := p.RunAutoBackup(AutoBackupOptions{Keep: 5})
	assert.ErrorIs(t, err, skypanel.ErrBackupInProgress)
}

func TestStartBackup_LeavesNoPartialFileBehind(t *testing.T) {
	p, dir := newBackupTestServer(t)
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "world.dat"), []byte("world"), 0644))

	name, err := p.StartBackup()
	assert.NoError(t, err)
	waitIdle(t, p)

	entries, err := os.ReadDir(p.GetBackupDirectory())
	assert.NoError(t, err)
	if assert.Len(t, entries, 1) {
		assert.Equal(t, name, entries[0].Name())
	}
}

func runOperation(p *Server, op string, args map[string]interface{}) error {
	process, err := GenerateProcess([]skypanel.ConditionalMetadataType{{MetadataType: skypanel.MetadataType{Type: op, Metadata: args}}}, p.RunningEnvironment, map[string]interface{}{}, map[string]string{})
	if err != nil {
		return err
	}
	return process.Run(p)
}

func TestTaskOperations(t *testing.T) {
	p, dir := newBackupTestServer(t)
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "world.dat"), []byte("world"), 0644))

	t.Run("restart skips a stopped server", func(t *testing.T) {
		assert.NoError(t, runOperation(p, "restart", nil))
		assert.Contains(t, consoleText(p), "skipping the restart")
	})

	t.Run("stop on a stopped server does nothing", func(t *testing.T) {
		assert.NoError(t, runOperation(p, "stop", nil))
	})

	t.Run("backup keeps the requested number", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			assert.NoError(t, runOperation(p, "backup", map[string]interface{}{"keep": 2}))
			time.Sleep(10 * time.Millisecond)
		}
		assert.Len(t, backupNames(t, p), 2)
	})

	t.Run("backup refuses an out of range keep", func(t *testing.T) {
		assert.Error(t, runOperation(p, "backup", map[string]interface{}{"keep": 0}))
		assert.Error(t, runOperation(p, "backup", map[string]interface{}{"keep": MaxAutoBackupKeep + 1}))
	})
}

// tuesday is a fixed run time; noon keeps it on the same day in any timezone the tests run in
var tuesday = time.Date(2026, time.November, 3, 12, 0, 0, 0, time.UTC)

func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

// runTaskBackup runs a task's backup at the given time, as the fake bucket also dates the
// upload, and waits for any upload it started
func runTaskBackup(t *testing.T, p *Server, fake *fakeBucket, at time.Time, opts AutoBackupOptions) string {
	t.Helper()
	fake.setClock(fixedClock(at))
	opts.Now = fixedClock(at)
	name, err := p.RunAutoBackup(opts)
	assert.NoError(t, err)
	waitForUploads(t)
	return name
}

func TestTaskBackup_NamesCarryTheTask(t *testing.T) {
	useFakeBucket(t)
	p, _ := newBackupTestServer(t)

	name, err := p.RunAutoBackup(AutoBackupOptions{Keep: 5, TaskID: "t1", Offsite: OffsiteNever})

	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(name, "auto-t1-"), name)
	id, ok := taskIDFromBackupName(name)
	assert.True(t, ok)
	assert.Equal(t, "t1", id)
}

func TestTaskBackup_WithoutATaskIsNeverUploaded(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)

	name := runTaskBackup(t, p, fake, tuesday, AutoBackupOptions{Keep: 5, Offsite: OffsiteAlways})

	_, ok := taskIDFromBackupName(name)
	assert.False(t, ok, "a run outside the scheduler names no task")
	assert.True(t, strings.HasPrefix(name, AutoBackupPrefix))
	assert.Equal(t, 0, fake.uploadCount())
}

func TestTaskBackup_WeeklyUploadsOncePerWeek(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)
	opts := AutoBackupOptions{Keep: 5, TaskID: "t1", Offsite: OffsiteWeekly, OffsiteKeep: 4}

	runTaskBackup(t, p, fake, tuesday, opts)
	assert.Equal(t, 1, fake.uploadCount(), "the first run of the week uploads")

	runTaskBackup(t, p, fake, tuesday.AddDate(0, 0, 1), opts)
	assert.Equal(t, 1, fake.uploadCount(), "Wednesday's run stays on the node")

	runTaskBackup(t, p, fake, tuesday.AddDate(0, 0, 7), opts)
	assert.Equal(t, 2, fake.uploadCount(), "the next week uploads again")
}

func TestTaskBackup_OldTaskStillUploadsEveryRun(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)

	for i := 0; i < 2; i++ {
		// a task saved before the option existed has no "offsite"
		assert.NoError(t, runOperation(p, "backup", map[string]interface{}{"keep": float64(5), "taskId": "t1"}))
		waitForUploads(t)
	}

	assert.Equal(t, 2, fake.uploadCount())
	assert.Len(t, fake.names(p.ID()), 2)
}

func TestTaskBackup_NeverStaysLocal(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)

	assert.NoError(t, runOperation(p, "backup", map[string]interface{}{"keep": float64(5), "taskId": "t1", "offsite": "never"}))
	waitForUploads(t)

	assert.Equal(t, 0, fake.uploadCount())
	assert.Len(t, backupNames(t, p), 1, "the copy stays on the node")
}

func TestTaskBackup_PrunesOnlyItsOwnSeries(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)
	named := func(prefix string) string { return prefix + uuid.Must(uuid.NewV4()).String() + ".tar.gz" }
	ownNewest := ""
	for i := 0; i < 5; i++ {
		at := tuesday.AddDate(0, 0, -10+i)
		ownNewest = named(taskBackupPrefix("a"))
		fake.put(p.ID(), ownNewest, at)
		fake.put(p.ID(), named(taskBackupPrefix("a-b")), at)
	}
	older := named(AutoBackupPrefix)
	fake.put(p.ID(), older, tuesday.AddDate(-1, 0, 0))

	name := runTaskBackup(t, p, fake, tuesday, AutoBackupOptions{Keep: 5, TaskID: "a", Offsite: OffsiteAlways, OffsiteKeep: 2})

	var own, other []string
	for _, n := range fake.names(p.ID()) {
		switch id, _ := taskIDFromBackupName(n); id {
		case "a":
			own = append(own, n)
		case "a-b":
			other = append(other, n)
		}
	}
	assert.ElementsMatch(t, []string{name, ownNewest}, own, "only the newest 2 of the task stay")
	assert.Len(t, other, 5, "the copies of task a-b are not task a's")
	assert.Contains(t, fake.names(p.ID()), older, "a copy that names no task is never pruned")
}

func TestTaskBackup_SkipsWhileTheSameSeriesUploads(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)
	fake.holdUploads()
	opts := AutoBackupOptions{Keep: 5, TaskID: "t1", Offsite: OffsiteAlways, OffsiteKeep: 4}

	_, err := p.RunAutoBackup(opts)
	assert.NoError(t, err)
	assert.Eventually(t, func() bool { return fake.uploadCount() == 1 }, 15*time.Second, 5*time.Millisecond)

	_, err = p.RunAutoBackup(opts)
	assert.NoError(t, err, "the task does not fail because of the upload in progress")
	fake.release()
	waitForUploads(t)

	assert.Equal(t, 1, fake.uploadCount(), "the second run left its copy on the node")
	assert.Len(t, fake.names(p.ID()), 1)
}

func TestTaskBackup_AListingErrorDoesNotFailTheTask(t *testing.T) {
	fake := useFakeBucket(t)
	fake.failList.Store(true)
	p, _ := newBackupTestServer(t)

	runTaskBackup(t, p, fake, tuesday, AutoBackupOptions{Keep: 5, TaskID: "t1", Offsite: OffsiteAlways, OffsiteKeep: 4})

	assert.Equal(t, 0, fake.uploadCount())
	assert.Contains(t, consoleText(p), "Off-site copy skipped")
	assert.True(t, startSeries(p.ID(), "t1"), "the series is free for the next run")
	endSeries(p.ID(), "t1")
}

func TestScheduler_InjectsTaskID(t *testing.T) {
	useFakeBucket(t)
	p, _ := newBackupTestServer(t)
	p.Scheduler = newTestScheduler(t)
	allServersLock.Lock()
	allServers = append(allServers, p)
	allServersLock.Unlock()
	t.Cleanup(func() {
		allServersLock.Lock()
		defer allServersLock.Unlock()
		for i, s := range allServers {
			if s == p {
				allServers = append(allServers[:i], allServers[i+1:]...)
				break
			}
		}
	})

	stored := map[string]interface{}{"keep": float64(5), "offsite": "never", "taskId": "otra"}
	task := skypanel.Task{Name: "Backup", Operations: []skypanel.ConditionalMetadataType{
		{MetadataType: skypanel.MetadataType{Type: "backup", Metadata: stored}},
	}}
	assert.NoError(t, p.Scheduler.SetTask("t1", task))

	runScheduledTask(p.ID(), "t1")

	names := backupNames(t, p)
	if assert.Len(t, names, 1) {
		id, _ := taskIDFromBackupName(names[0])
		assert.Equal(t, "t1", id, "the scheduler names the task, never the stored arguments")
	}
	saved, _ := p.Scheduler.GetTask("t1")
	assert.Equal(t, "otra", saved.Operations[0].Metadata["taskId"], "the stored task is left as it was")
}

func TestScheduler_LocationFollowsTheTimezone(t *testing.T) {
	s := NewDefaultScheduler("tz")
	assert.Equal(t, time.Local, s.Location(), "the default Local")
	s.Timezone = ""
	assert.Equal(t, time.Local, s.Location())
	s.Timezone = "Europe/Madrid"
	assert.Equal(t, "Europe/Madrid", s.Location().String())
	var none *Scheduler
	assert.Equal(t, time.Local, none.Location(), "a server without a scheduler")
}
