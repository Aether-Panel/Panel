package servers

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

// diskUsageTTL is how long a measured folder size is trusted before the folder is walked
// again. Writes made through the panel are added to it as they happen, so in between it
// only misses what the game itself writes.
const diskUsageTTL = 15 * time.Second

// diskUsage is the server folder's size as last measured plus what the panel wrote since.
type diskUsage struct {
	mu    sync.Mutex
	bytes int64
	at    time.Time
}

// DiskLimit is the server's "disk" variable in bytes, 0 when it has no limit.
func (p *Server) DiskLimit() int64 {
	v, ok := p.Variables["disk"]
	if !ok {
		return 0
	}
	mb, err := cast.ToInt64E(v.Value)
	if err != nil || mb <= 0 {
		return 0
	}
	return mb * 1024 * 1024
}

// DiskUsage is how many bytes the server folder holds, measured at most diskUsageTTL ago.
func (p *Server) DiskUsage() int64 {
	p.disk.mu.Lock()
	defer p.disk.mu.Unlock()
	return p.diskUsageLocked(false)
}

func (p *Server) diskUsageLocked(fresh bool) int64 {
	if fresh || p.disk.at.IsZero() || time.Since(p.disk.at) > diskUsageTTL {
		p.disk.bytes = folderSize(p.rootDirectory())
		p.disk.at = time.Now()
	}
	return p.disk.bytes
}

// ForgetDiskUsage makes the next check measure the folder again, after an operation that
// changes its size in ways the panel does not count (extracting, compressing, deleting).
func (p *Server) ForgetDiskUsage() {
	p.disk.mu.Lock()
	p.disk.at = time.Time{}
	p.disk.mu.Unlock()
}

// CheckDiskSpace returns ErrDiskLimitExceeded when the server has already used all of its
// disk. fresh measures the folder now instead of trusting the last measure.
func (p *Server) CheckDiskSpace(fresh bool) error {
	limit := p.DiskLimit()
	if limit == 0 {
		return nil
	}
	p.disk.mu.Lock()
	defer p.disk.mu.Unlock()
	if p.diskUsageLocked(fresh) >= limit {
		return skypanel.ErrDiskLimitExceeded
	}
	return nil
}

// reserveDisk counts n more bytes against the limit, or refuses them when they do not fit.
func (p *Server) reserveDisk(n int64) error {
	limit := p.DiskLimit()
	p.disk.mu.Lock()
	defer p.disk.mu.Unlock()
	used := p.diskUsageLocked(false)
	if limit > 0 && used+n > limit {
		return skypanel.ErrDiskLimitExceeded
	}
	p.disk.bytes = used + n
	return nil
}

// ReleaseDisk takes n bytes back off the counted usage, for a file about to be overwritten.
func (p *Server) ReleaseDisk(n int64) {
	p.disk.mu.Lock()
	if !p.disk.at.IsZero() {
		p.disk.bytes = max(p.disk.bytes-n, 0)
	}
	p.disk.mu.Unlock()
}

// DiskWriter wraps w so every byte written through it counts against the disk limit; a
// write that would go over the limit fails with ErrDiskLimitExceeded and writes nothing.
func (p *Server) DiskWriter(w io.Writer) io.Writer {
	return &diskWriter{server: p, w: w}
}

type diskWriter struct {
	server *Server
	w      io.Writer
}

func (d *diskWriter) Write(b []byte) (int, error) {
	if err := d.server.reserveDisk(int64(len(b))); err != nil {
		return 0, err
	}
	return d.w.Write(b)
}

// DiskFile wraps a file opened for writing (by SFTP, which writes at offsets) so growing
// it counts against the disk limit. Rewriting bytes the file already had is free.
func (p *Server) DiskFile(f *os.File) *DiskFile {
	return &DiskFile{File: f, server: p}
}

type DiskFile struct {
	*os.File
	server *Server
	mu     sync.Mutex
	end    int64
}

func (d *DiskFile) WriteAt(b []byte, off int64) (int, error) {
	d.mu.Lock()
	if grow := off + int64(len(b)) - d.end; grow > 0 {
		if err := d.server.reserveDisk(grow); err != nil {
			d.mu.Unlock()
			return 0, err
		}
		d.end += grow
	}
	d.mu.Unlock()
	return d.File.WriteAt(b, off)
}

func (p *Server) rootDirectory() string {
	if p.RunningEnvironment == nil {
		return ""
	}
	return p.RunningEnvironment.GetRootDirectory()
}

func folderSize(root string) int64 {
	if root == "" {
		return 0
	}
	var size int64
	_ = filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if info, err := entry.Info(); err == nil {
			size += info.Size()
		}
		return nil
	})
	return size
}
