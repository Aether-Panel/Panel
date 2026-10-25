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

	name, err := p.RunAutoBackup(2)
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

	_, err := p.RunAutoBackup(5)
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
