package servers

import (
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
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))

	var wg sync.WaitGroup
	var lock sync.Mutex
	started := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := p.StartBackup(); err == nil {
				lock.Lock()
				started++
				lock.Unlock()
			}
		}()
	}
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
