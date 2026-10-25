package servers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRestorePointReason(t *testing.T) {
	assert.Equal(t, "plugin", RestorePointReason("restore-plugin-1234.tar.gz"))
	assert.Equal(t, "version", RestorePointReason("restore-version-abcd.tar.gz"))
	assert.Equal(t, "", RestorePointReason("auto-1234.tar.gz"))
}

func TestCreateRestorePoint_KeepsTheNewestThreeAndSkipsRepeatedPluginOnes(t *testing.T) {
	p, dir := newBackupTestServer(t)
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "world.dat"), []byte("world"), 0644))

	assert.NoError(t, p.CreateRestorePoint(RestoreReasonPlugin))
	assert.NoError(t, p.CreateRestorePoint(RestoreReasonPlugin), "a second plugin step right after reuses the first point")
	assert.Len(t, p.restorePoints(), 1)

	for i := 0; i < 4; i++ {
		assert.NoError(t, p.CreateRestorePoint(RestoreReasonVersion))
		time.Sleep(10 * time.Millisecond)
	}
	points := p.restorePoints()
	assert.Len(t, points, keepRestorePoints)
	for _, b := range points {
		assert.True(t, strings.HasPrefix(b.FileName, RestorePointPrefix+RestoreReasonVersion), b.FileName)
	}
	assert.Contains(t, consoleText(p), "Restore point saved")
}

func TestCreateRestorePoint_RefusesWhileABackupRuns(t *testing.T) {
	p, _ := newBackupTestServer(t)
	p.backupState.Store(backupRunning)
	defer p.backupState.Store(backupIdle)
	assert.ErrorIs(t, p.CreateRestorePoint(RestoreReasonVersion), errBackupBusy)
}

func TestRestore_SavesARestorePointFirst(t *testing.T) {
	p, dir := newBackupTestServer(t)
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "world.dat"), []byte("old"), 0644))
	name, err := p.StartBackup()
	assert.NoError(t, err)
	waitIdle(t, p)

	assert.NoError(t, os.WriteFile(filepath.Join(dir, "world.dat"), []byte("new"), 0644))
	assert.NoError(t, p.StartRestore(name))
	waitIdle(t, p)

	points := p.restorePoints()
	if assert.Len(t, points, 1) {
		assert.Equal(t, RestoreReasonRestore, RestorePointReason(points[0].FileName))
	}
	content, _ := os.ReadFile(filepath.Join(dir, "world.dat"))
	assert.Equal(t, "old", string(content))
}
