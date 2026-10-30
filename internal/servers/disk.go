package servers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/shirou/gopsutil/disk"
	"github.com/spf13/cast"
)

// diskUsageTTL is how long a measured folder size is trusted before the folder is walked
// again. Writes made through the panel are added to it as they happen, so in between it
// only misses what the game itself writes.
// The stats of a running server read this measure too (every 30 seconds), so one walk serves both.
const diskUsageTTL = 30 * time.Second

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
		p.disk.bytes = folderSize(p.rootDirectory()) + folderSize(p.tmpDirectory())
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
// it counts against the disk limit. Rewriting bytes the file already had is free. name is
// the file's path in the server folder; fresh says the upload started from an empty file,
// and then an upload cut short by the limit is deleted on Close (a resumed one is kept).
func (p *Server) DiskFile(f *os.File, name string, fresh bool) *DiskFile {
	d := &DiskFile{File: f, server: p, name: name, fresh: fresh}
	if info, err := f.Stat(); err == nil {
		d.end = info.Size()
	}
	return d
}

type DiskFile struct {
	*os.File
	server  *Server
	name    string
	mu      sync.Mutex
	end     int64
	refused bool
	fresh   bool
}

func (d *DiskFile) WriteAt(b []byte, off int64) (int, error) {
	d.mu.Lock()
	if grow := off + int64(len(b)) - d.end; grow > 0 {
		if err := d.server.reserveDisk(grow); err != nil {
			d.refused = true
			d.mu.Unlock()
			return 0, err
		}
		d.end += grow
	}
	d.mu.Unlock()
	return d.File.WriteAt(b, off)
}

// Close closes the file, and deletes it when the disk limit cut the upload short: half a
// file is useless and would keep the disk full
func (d *DiskFile) Close() error {
	err := d.File.Close()
	d.mu.Lock()
	refused := d.refused
	d.mu.Unlock()
	if refused && d.fresh {
		if fs := d.server.GetFileServer(); fs != nil {
			_ = fs.Remove(d.name)
		}
	}
	if refused {
		d.server.ForgetDiskUsage()
	}
	return err
}

func (p *Server) rootDirectory() string {
	if p.RunningEnvironment == nil {
		return ""
	}
	return p.RunningEnvironment.GetRootDirectory()
}

// tmpDirectory is the folder a Docker server writes /tmp to; it counts against its disk limit
func (p *Server) tmpDirectory() string {
	if p.RunningEnvironment == nil || p.RunningEnvironment.Type != "docker" {
		return ""
	}
	return p.RunningEnvironment.GetTmpDirectory()
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

// NodeDisk is the space of the disk that holds the servers' folders
type NodeDisk struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"usedPercent"`
	// Low is true when the disk is close to full: under 10% or 5 GB free. The panel warns
	// the administrators then, before an install or a game fills it.
	Low bool `json:"low"`
}

const lowDiskBytes = 5 << 30

// NodeDiskUsage measures the disk the servers' folders live on
func NodeDiskUsage() (*NodeDisk, error) {
	folder := config.ServersFolder.Value()
	usage, err := disk.Usage(folder)
	if err != nil {
		return nil, err
	}
	return &NodeDisk{
		Path:        folder,
		Total:       usage.Total,
		Used:        usage.Used,
		Free:        usage.Free,
		UsedPercent: usage.UsedPercent,
		Low:         usage.Free < lowDiskBytes || usage.Free < usage.Total/10,
	}, nil
}

// checkNodeDiskForInstall refuses an install when the node has less free space than
// daemon.minFreeDiskMB: a game download would fill the disk and stop every server on it.
func (p *Server) checkNodeDiskForInstall() error {
	minFree := uint64(max(config.MinFreeDiskMB.Value(), 0)) << 20
	if minFree == 0 {
		return nil
	}
	usage, err := NodeDiskUsage()
	if err != nil {
		// without a reading the install goes ahead, as before the check existed
		p.Log(logging.Error, "Could not read the node's free disk space: %s", err)
		return nil
	}
	if usage.Free < minFree {
		p.RunningEnvironment.DisplayToConsole(true, "The node only has %s free, and an install needs at least %s. Free some space on the node and install again.\n",
			formatGigabytes(usage.Free), formatGigabytes(minFree))
		return skypanel.ErrNodeDiskFull
	}
	return nil
}

func formatGigabytes(bytes uint64) string {
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1<<30))
}
