package servers

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

func newBackupTestServer(t *testing.T) (*Server, string) {
	root := t.TempDir()
	previous := config.BackupsFolder.Value()
	_ = config.BackupsFolder.Set(filepath.Join(root, "backups"), false)
	t.Cleanup(func() { _ = config.BackupsFolder.Set(previous, false) })

	serverDir := filepath.Join(root, "server")
	if !assert.NoError(t, os.MkdirAll(serverDir, 0755)) {
		t.FailNow()
	}
	fs, err := files.NewFileServer(serverDir, os.Getuid(), os.Getgid())
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	p := CreateProgram()
	p.Identifier = "backuptest"
	p.SetFileServer(fs)
	p.RunningEnvironment = &skypanel.Environment{
		ConsoleBuffer:  skypanel.CreateCache(),
		ConsoleTracker: skypanel.CreateTracker(),
		Implementation: &fakeEnvironment{},
	}
	return p, serverDir
}

// fakeEnvironment is a stopped server: enough for backups and restores, which only touch files
type fakeEnvironment struct {
	running bool
}

func (f *fakeEnvironment) ExecuteAsyncImpl(*skypanel.Environment, skypanel.ExecutionData) error {
	return nil
}
func (f *fakeEnvironment) KillImpl(*skypanel.Environment) error { return nil }
func (f *fakeEnvironment) GetStatsImpl(*skypanel.Environment) (*skypanel.ServerStats, error) {
	return &skypanel.ServerStats{}, nil
}
func (f *fakeEnvironment) SendCodeImpl(*skypanel.Environment, int) error     { return nil }
func (f *fakeEnvironment) GetUIDImpl(*skypanel.Environment) int              { return os.Getuid() }
func (f *fakeEnvironment) GetGidImpl(*skypanel.Environment) int              { return os.Getgid() }
func (f *fakeEnvironment) IsRunningImpl(*skypanel.Environment) (bool, error) { return f.running, nil }

func waitIdle(t *testing.T, p *Server) {
	deadline := time.Now().Add(15 * time.Second)
	for p.IsBackingUp() || p.IsRestoring() {
		if time.Now().After(deadline) {
			t.Fatal("backup or restore never finished")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func consoleText(p *Server) string {
	data, _ := p.RunningEnvironment.ConsoleBuffer.Read()
	return string(data)
}

func TestStartRestore_MissingBackupLeavesFilesAlone(t *testing.T) {
	p, dir := newBackupTestServer(t)
	world := filepath.Join(dir, "world.dat")
	assert.NoError(t, os.WriteFile(world, []byte("players' world"), 0644))

	err := p.StartRestore("does-not-exist.tar.gz")

	assert.ErrorIs(t, err, skypanel.ErrFileNotFound)
	content, readErr := os.ReadFile(world)
	assert.NoError(t, readErr, "the restore used to delete every file before noticing the backup was missing")
	assert.Equal(t, "players' world", string(content))
	assert.NoError(t, p.IsIdle())
}

func TestStartRestore_RefusesARunningServer(t *testing.T) {
	p, _ := newBackupTestServer(t)
	name, err := p.StartBackup()
	if !assert.NoError(t, err) {
		return
	}
	waitIdle(t, p)

	p.RunningEnvironment.Implementation.(*fakeEnvironment).running = true
	assert.ErrorIs(t, p.StartRestore(name), skypanel.ErrServerRunning)
}

func TestStartRestore_UnreadableBackupLeavesFilesAlone(t *testing.T) {
	p, dir := newBackupTestServer(t)
	world := filepath.Join(dir, "world.dat")
	assert.NoError(t, os.WriteFile(world, []byte("players' world"), 0644))
	assert.NoError(t, os.MkdirAll(p.GetBackupDirectory(), 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(p.GetBackupDirectory(), "broken.tar.gz"), []byte("not an archive"), 0644))

	assert.NoError(t, p.StartRestore("broken.tar.gz"))
	waitIdle(t, p)

	content, err := os.ReadFile(world)
	assert.NoError(t, err, "a backup that cannot be read must not wipe the server")
	assert.Equal(t, "players' world", string(content))
	assert.Contains(t, consoleText(p), "Restore failed")
}

func TestStartBackup_OnlyOneRunsAtATime(t *testing.T) {
	p, dir := newBackupTestServer(t)
	// incompressible data keeps the first backup busy long enough for every request to
	// arrive while it runs; a tiny file finished before the slowest goroutines got going
	noise := make([]byte, 8<<20)
	_, _ = rand.Read(noise)
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "a.bin"), noise, 0644))

	var wg sync.WaitGroup
	var lock sync.Mutex
	started := 0
	// every goroutine waits here, so the requests really are simultaneous
	start := make(chan struct{})
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := p.StartBackup(); err == nil {
				lock.Lock()
				started++
				lock.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()
	waitIdle(t, p)

	assert.Equal(t, 1, started, "simultaneous requests must not start several backups")
	entries, _ := os.ReadDir(p.GetBackupDirectory())
	assert.Len(t, entries, 1)
}

func TestBackupAndRestore_RoundTrip(t *testing.T) {
	p, dir := newBackupTestServer(t)
	file := filepath.Join(dir, "server.properties")
	assert.NoError(t, os.WriteFile(file, []byte("motd=original"), 0644))

	name, err := p.StartBackup()
	if !assert.NoError(t, err) {
		return
	}
	waitIdle(t, p)

	assert.NoError(t, os.WriteFile(file, []byte("motd=changed"), 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0644))

	if !assert.NoError(t, p.StartRestore(name)) {
		return
	}
	waitIdle(t, p)

	content, _ := os.ReadFile(file)
	assert.Equal(t, "motd=original", string(content), consoleText(p))
	_, statErr := os.Stat(filepath.Join(dir, "new.txt"))
	assert.True(t, os.IsNotExist(statErr), "files added after the backup are gone")
	assert.True(t, strings.Contains(consoleText(p), "Restore complete"))
}

// failArchive makes the next archive written to name fail: its hidden partial name is taken
func failArchive(t *testing.T, p *Server, name string) string {
	dir := p.GetBackupDirectory()
	assert.NoError(t, os.MkdirAll(filepath.Join(dir, ".partial-"+name), 0755))
	return filepath.Join(dir, name)
}

func TestFinishBackup_MarksAFailedManualBackup(t *testing.T) {
	p, _ := newBackupTestServer(t)
	p.backupState.Store(backupRunning)

	assert.False(t, p.finishBackup(failArchive(t, p, "manual.tar.gz")))

	failed, err := p.FailedBackups()
	assert.NoError(t, err)
	assert.Equal(t, []string{"manual.tar.gz"}, failed, "the panel is told to drop its entry")
	listed, err := p.ListBackups()
	assert.NoError(t, err)
	assert.Empty(t, listed, "the mark is not a backup")

	// deleting the backup, as the panel does when it drops the entry, removes the mark
	assert.NoError(t, p.DeleteBackup("manual.tar.gz"))
	failed, err = p.FailedBackups()
	assert.NoError(t, err)
	assert.Empty(t, failed)
}

func TestFinishBackup_NoMarkForAScheduledBackup(t *testing.T) {
	p, _ := newBackupTestServer(t)
	p.backupState.Store(backupRunning)

	assert.False(t, p.finishBackup(failArchive(t, p, AutoBackupPrefix+"x.tar.gz")))

	failed, err := p.FailedBackups()
	assert.NoError(t, err)
	assert.Empty(t, failed, "the panel keeps no entry for it")
}

func TestFailedBackups_NoFolderYet(t *testing.T) {
	p, _ := newBackupTestServer(t)
	failed, err := p.FailedBackups()
	assert.NoError(t, err)
	assert.Empty(t, failed)
}

func TestDeleteBackups_RemovesOnlyThatServersBackups(t *testing.T) {
	p, _ := newBackupTestServer(t)
	name, err := p.StartBackup()
	if !assert.NoError(t, err) {
		return
	}
	waitIdle(t, p)
	other := filepath.Join(config.BackupsFolder.Value(), "otherserver")
	assert.NoError(t, os.MkdirAll(other, 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(other, "keep.tar.gz"), []byte("x"), 0644))
	assert.FileExists(t, filepath.Join(p.GetBackupDirectory(), name))

	assert.NoError(t, DeleteBackups(p.ID()))

	assert.NoDirExists(t, p.GetBackupDirectory())
	assert.FileExists(t, filepath.Join(other, "keep.tar.gz"))
	assert.Error(t, DeleteBackups("."), "never the folder of every server")
	assert.Error(t, DeleteBackups("../x"))
	assert.FileExists(t, filepath.Join(other, "keep.tar.gz"))
}
