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
	"github.com/gofrs/uuid/v5"
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

func TestManualBackup_StaysLocal(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)

	name, err := p.StartBackup()
	if !assert.NoError(t, err) {
		return
	}
	waitIdle(t, p)

	assert.Equal(t, 0, fake.uploadCount(), "a backup made by hand stays on the node")
	assert.FileExists(t, filepath.Join(p.GetBackupDirectory(), name))
}

// seedAutoBackups writes n local copies of task t1, oldest first, an hour apart
func seedAutoBackups(t *testing.T, p *Server, n int) []string {
	assert.NoError(t, os.MkdirAll(p.GetBackupDirectory(), 0755))
	base := time.Now().Add(-time.Duration(n) * time.Hour)
	names := make([]string, 0, n)
	for i := 0; i < n; i++ {
		name := taskBackupPrefix("t1") + uuid.Must(uuid.NewV4()).String() + ".tar.gz"
		file := filepath.Join(p.GetBackupDirectory(), name)
		modTime := base.Add(time.Duration(i) * time.Hour)
		assert.NoError(t, os.WriteFile(file, []byte("x"), 0644))
		assert.NoError(t, os.Chtimes(file, modTime, modTime))
		names = append(names, name)
	}
	return names
}

func localBackupNames(t *testing.T, p *Server) []string {
	entries, err := os.ReadDir(p.GetBackupDirectory())
	assert.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestPruneAutoBackups_LeavesTheBucketAlone(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)
	names := seedAutoBackups(t, p, 3)
	for _, name := range names {
		fake.put(p.ID(), name, time.Now())
	}

	assert.NoError(t, p.pruneAutoBackups(1))

	assert.Equal(t, []string{names[2]}, localBackupNames(t, p), "only the newest stays on the node")
	assert.Len(t, fake.names(p.ID()), 3, "the bucket keeps its own history")
}

func TestPruneAutoBackups_CountsOnlyLocalCopies(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)
	names := seedAutoBackups(t, p, 2)
	// newer than every local copy, and only in the bucket
	fake.put(p.ID(), taskBackupPrefix("t1")+uuid.Must(uuid.NewV4()).String()+".tar.gz", time.Now().Add(time.Hour))

	assert.NoError(t, p.pruneAutoBackups(1))

	assert.Equal(t, []string{names[1]}, localBackupNames(t, p), "a copy only the bucket has does not take a local one's place")
}

func TestDeleteBackup_ByAUserRemovesBothCopies(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)
	names := seedAutoBackups(t, p, 1)
	fake.put(p.ID(), names[0], time.Now())

	assert.NoError(t, p.DeleteBackup(names[0]))

	assert.Empty(t, localBackupNames(t, p))
	assert.Empty(t, fake.names(p.ID()))
}

func TestLocateBackup_FindsACopyOnlyTheBucketHas(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)
	fake.put(p.ID(), "remote.tar.gz", time.Now())

	file, fetch, err := p.locateBackup("remote.tar.gz")
	assert.NoError(t, err)
	assert.True(t, fetch, "it must be downloaded before restoring")
	assert.Equal(t, filepath.Join(p.GetBackupDirectory(), "remote.tar.gz"), file)

	_, _, err = p.locateBackup("nowhere.tar.gz")
	assert.ErrorIs(t, err, skypanel.ErrFileNotFound)
}

// The daemon tells the panel when the bucket part of the list is missing, so the panel does
// not take the off-site copies for deleted
func TestListBackupsChecked_SaysWhenTheBucketCouldNotBeListed(t *testing.T) {
	fake := useFakeBucket(t)
	p, _ := newBackupTestServer(t)
	writeBackupFile(t, p, "local.tar.gz", time.Hour)
	fake.put(p.ID(), "remote.tar.gz", time.Now())

	listedNames := func() ([]string, bool) {
		backups, bucketListed, err := p.ListBackupsChecked()
		assert.NoError(t, err)
		names := make([]string, 0, len(backups))
		for _, b := range backups {
			names = append(names, b.FileName+":"+b.Location)
		}
		return names, bucketListed
	}

	names, bucketListed := listedNames()
	assert.True(t, bucketListed)
	assert.Equal(t, []string{"remote.tar.gz:offsite", "local.tar.gz:local"}, names)

	fake.failList.Store(true)
	captureErrors(t)
	names, bucketListed = listedNames()
	assert.False(t, bucketListed, "the bucket is set up but could not be read")
	assert.Equal(t, []string{"local.tar.gz:local"}, names, "the node's copies are still listed")

	fake.disabled.Store(true)
	names, bucketListed = listedNames()
	assert.True(t, bucketListed, "without a bucket there is nothing missing from the list")
	assert.Equal(t, []string{"local.tar.gz:local"}, names)
}
