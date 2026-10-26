package servers

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

// newDiskTestServer is a stopped server with a 1 MB disk limit whose folder holds `used` bytes.
func newDiskTestServer(t *testing.T, used int) (*Server, string) {
	p, dir := newBackupTestServer(t)
	p.RunningEnvironment.RootDirectory = dir
	p.Variables = map[string]skypanel.Variable{"disk": {Value: 1}}
	if used > 0 {
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "world.dat"), make([]byte, used), 0644))
	}
	return p, dir
}

func TestCheckDiskSpace_NoLimitNeverRefuses(t *testing.T) {
	p, _ := newDiskTestServer(t, 2*1024*1024)
	p.Variables = nil
	assert.NoError(t, p.CheckDiskSpace(true))
}

func TestCheckDiskSpace_RefusesAFullDisk(t *testing.T) {
	p, _ := newDiskTestServer(t, 1024*1024)
	assert.ErrorIs(t, p.CheckDiskSpace(true), skypanel.ErrDiskLimitExceeded)
}

func TestCheckDiskSpace_DeletedFilesCountOnceForgotten(t *testing.T) {
	p, dir := newDiskTestServer(t, 1024*1024)
	assert.ErrorIs(t, p.CheckDiskSpace(false), skypanel.ErrDiskLimitExceeded)

	assert.NoError(t, os.Remove(filepath.Join(dir, "world.dat")))
	p.ForgetDiskUsage()
	assert.NoError(t, p.CheckDiskSpace(false))
}

func TestDiskWriter_StopsAtTheLimit(t *testing.T) {
	p, _ := newDiskTestServer(t, 512*1024)
	var out bytes.Buffer
	w := p.DiskWriter(&out)

	_, err := w.Write(make([]byte, 256*1024))
	assert.NoError(t, err)
	n, err := w.Write(make([]byte, 512*1024))
	assert.ErrorIs(t, err, skypanel.ErrDiskLimitExceeded)
	assert.Zero(t, n, "a refused write writes nothing")
	assert.Equal(t, 256*1024, out.Len())
}

func TestReleaseDisk_OverwritingAFileDoesNotCountItTwice(t *testing.T) {
	p, _ := newDiskTestServer(t, 768*1024)
	assert.NoError(t, p.CheckDiskSpace(false))

	p.ReleaseDisk(768 * 1024)
	_, err := p.DiskWriter(&bytes.Buffer{}).Write(make([]byte, 768*1024))
	assert.NoError(t, err)
}

func TestDiskFile_OnlyGrowingTheFileCounts(t *testing.T) {
	p, dir := newDiskTestServer(t, 512*1024)
	f, err := os.Create(filepath.Join(dir, "upload.bin"))
	if !assert.NoError(t, err) {
		return
	}
	d := p.DiskFile(f)
	defer func() { _ = d.Close() }()

	_, err = d.WriteAt(make([]byte, 256*1024), 0)
	assert.NoError(t, err)
	// rewriting the same bytes is free
	_, err = d.WriteAt(make([]byte, 256*1024), 0)
	assert.NoError(t, err)
	_, err = d.WriteAt(make([]byte, 512*1024), 256*1024)
	assert.ErrorIs(t, err, skypanel.ErrDiskLimitExceeded)
}

func TestStart_RefusesAFullDisk(t *testing.T) {
	p, _ := newDiskTestServer(t, 2*1024*1024)
	assert.ErrorIs(t, p.Start(), skypanel.ErrDiskLimitExceeded)
	assert.Contains(t, consoleText(p), "disk space")
}
