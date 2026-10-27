package logging

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	logName       = "skypanel.log"
	archivePrefix = "skypanel-"
	dayFormat     = "2006-01-02"
	// maxLogSize starts a new file early when a single day writes this much
	maxLogSize int64 = 100 << 20
)

// Rotator is the log file writer. Like Pterodactyl's daily logs, the current day goes to
// skypanel.log; when the day changes (or the file reaches maxLogSize) it is renamed to
// skypanel-<day>.log, compressed, and archives older than keepDays are deleted.
// SIGUSR1 still reopens skypanel.log, for setups that rotate it with an external logrotate.
type Rotator struct {
	sync.Mutex
	io.WriteCloser
	backer io.WriteCloser
	dir    string
	// day is the date of the lines in the current file, and size how much it holds
	day  string
	size int64
	// keepDays is how long archives are kept; 0 keeps them forever
	keepDays int
	now      func() time.Time
	// cleanup serialises the background compress-and-prune passes
	cleanup sync.Mutex
	pending sync.WaitGroup
}

// NewRotator opens skypanel.log in dir. A file left over from an earlier day is archived first.
func NewRotator(dir string, keepDays int) (*Rotator, error) {
	r := &Rotator{dir: dir, keepDays: keepDays, now: time.Now}
	if err := r.open(); err != nil {
		return nil, err
	}
	if r.size > 0 && r.day != r.today() {
		r.rotate()
	} else {
		r.startCleanup()
	}
	return r, nil
}

func (r *Rotator) today() string {
	return r.now().Format(dayFormat)
}

// open (re)opens skypanel.log for appending; the day comes from its last write
func (r *Rotator) open() error {
	file, err := os.OpenFile(filepath.Join(r.dir, logName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	r.backer = file
	r.day, r.size = r.today(), 0
	if info, err := file.Stat(); err == nil {
		r.size = info.Size()
		if r.size > 0 {
			r.day = info.ModTime().Format(dayFormat)
		}
	}
	return nil
}

func (r *Rotator) Write(p []byte) (n int, err error) {
	r.Lock()
	defer r.Unlock()
	if r.backer == nil {
		return len(p), nil
	}
	if r.dir != "" && r.size > 0 && (r.day != r.today() || r.size+int64(len(p)) > maxLogSize) {
		r.rotate()
		if r.backer == nil {
			return len(p), nil
		}
	}
	n, err = r.backer.Write(p)
	r.size += int64(n)
	return n, err
}

// rotate archives the current file and starts a new one. The caller holds the lock.
// If the rename fails the same file keeps being used, so no lines are lost.
func (r *Rotator) rotate() {
	_ = r.backer.Close()
	current := filepath.Join(r.dir, logName)
	if archive, ok := r.freeArchiveName(r.day); ok {
		_ = os.Rename(current, archive)
	}
	if err := r.open(); err != nil {
		r.backer = nil
		_, _ = fmt.Fprintf(OriginalStdErr, "[ERROR] reopening %s: %s\n", current, err)
		return
	}
	r.startCleanup()
}

// freeArchiveName returns skypanel-<day>.log, or skypanel-<day>.N.log when that day already
// has an archive (a restart or a size rotation)
func (r *Rotator) freeArchiveName(day string) (string, bool) {
	for i := 0; i < 1000; i++ {
		name := archivePrefix + day + ".log"
		if i > 0 {
			name = fmt.Sprintf("%s%s.%d.log", archivePrefix, day, i)
		}
		path := filepath.Join(r.dir, name)
		if !exists(path) && !exists(path+".gz") {
			return path, true
		}
	}
	return "", false
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// startCleanup runs compressAndPrune in the background; the cutoff is taken now, so the
// goroutine shares nothing with the writer but the folder
func (r *Rotator) startCleanup() {
	var cutoff time.Time
	if r.keepDays > 0 {
		cutoff = r.now().AddDate(0, 0, -r.keepDays)
	}
	r.pending.Add(1)
	go func() {
		defer r.pending.Done()
		r.compressAndPrune(cutoff)
	}()
}

// compressAndPrune gzips the plain archives and deletes the ones last written before cutoff
// (none when it is zero). It only touches skypanel-* files, never other files in the folder.
func (r *Rotator) compressAndPrune(cutoff time.Time) {
	r.cleanup.Lock()
	defer r.cleanup.Unlock()

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || !strings.HasPrefix(name, archivePrefix) {
			continue
		}
		path := filepath.Join(r.dir, name)
		if !cutoff.IsZero() {
			if info, err := entry.Info(); err == nil && info.ModTime().Before(cutoff) {
				_ = os.Remove(path)
				continue
			}
		}
		if strings.HasSuffix(name, ".log") {
			if err := gzipFile(path); err != nil {
				_, _ = fmt.Fprintf(OriginalStdErr, "[ERROR] compressing %s: %s\n", path, err)
			}
		}
	}
}

// gzipFile replaces path with path.gz, keeping its modification time for pruning
func gzipFile(path string) (err error) {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	tmp := path + ".gz.tmp"
	dst, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()

	zw := gzip.NewWriter(dst)
	if _, err = io.Copy(zw, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err = zw.Close(); err != nil {
		_ = dst.Close()
		return err
	}
	if err = dst.Close(); err != nil {
		return err
	}
	if err = os.Chtimes(tmp, info.ModTime(), info.ModTime()); err != nil {
		return err
	}
	if err = os.Rename(tmp, path+".gz"); err != nil {
		return err
	}
	return os.Remove(path)
}

// Wait blocks until the background compress-and-prune passes have finished
func (r *Rotator) Wait() {
	r.pending.Wait()
}

func (r *Rotator) Close() error {
	r.Lock()
	defer r.Unlock()
	if r.backer == nil {
		return nil
	}
	return r.backer.Close()
}

func (r *Rotator) Rotate(newBackend io.WriteCloser) {
	r.Lock()
	defer r.Unlock()
	oldBacker := r.backer
	r.backer = newBackend
	r.day, r.size = r.today(), 0
	if oldBacker != nil {
		_ = oldBacker.Close()
	}
}

func (r *Rotator) StartRotation(dir string) {
	go func(directory string) {
		sig := make(chan os.Signal, 1)
		for {
			signal.Notify(sig, syscall.SIGUSR1)

			<-sig

			newFile, err := os.OpenFile(filepath.Join(directory, logName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				panic(err)
			}

			r.Rotate(newFile)
		}
	}(dir)
}
