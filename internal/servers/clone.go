package servers

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

// Cloning copies the files of one server into a freshly created one on the same node, so the
// owner can try changes on a copy without touching the original.

var (
	ErrCloneSameServer     = errors.New("a server cannot be copied into itself")
	ErrCloneRunning        = errors.New("files are already being copied into this server")
	ErrCloneTargetNotEmpty = errors.New("the new server already has files, so nothing was copied")
)

// cloneSideFiles sit next to a server's folder and describe what is installed in it, so they
// travel with the files. Metrics, crash reports, alerts, tasks and history stay behind.
var cloneSideFiles = []string{".modpack", ".worldmap"}

// cloneProgressEvery spaces out the progress lines on the console
const cloneProgressEvery = 3 * time.Second

// maxPropertiesSize bounds the server.properties read when the copy's ports are updated
const maxPropertiesSize = 1 << 20

type cloneJob struct {
	running   atomic.Bool
	lastError atomic.Value
}

var cloneJobs sync.Map

func (p *Server) cloneState() *cloneJob {
	state, _ := cloneJobs.LoadOrStore(p.ID(), &cloneJob{})
	return state.(*cloneJob)
}

// CloneFrom copies source's files into p in the background. p is marked as installing until
// the copy ends, so it cannot be started half copied; the console shows the progress.
func (p *Server) CloneFrom(source *Server, sourceName string) error {
	if source == nil || source.ID() == p.ID() {
		return ErrCloneSameServer
	}
	if err := p.IsIdle(); err != nil {
		return err
	}
	state := p.cloneState()
	if !state.running.CompareAndSwap(false, true) {
		return ErrCloneRunning
	}

	empty, err := isEmptyDir(p.GetFileServer())
	if err != nil || !empty {
		state.running.Store(false)
		if err != nil {
			return err
		}
		return ErrCloneTargetNotEmpty
	}

	if sourceName == "" {
		sourceName = source.ID()
	}
	state.lastError.Store("")
	p.GetEnvironment().SetInstalling(true)

	go func() {
		defer state.running.Store(false)
		defer p.GetEnvironment().SetInstalling(false)
		if err := p.runClone(source, sourceName); err != nil {
			state.lastError.Store(err.Error())
			p.Log(logging.Error, "Copying files from %s failed: %s", source.ID(), err)
			p.RunningEnvironment.DisplayToConsole(true, "Copying the files failed: %s\n", err)
		}
	}()
	return nil
}

func (p *Server) runClone(source *Server, sourceName string) error {
	env := p.RunningEnvironment
	env.DisplayToConsole(true, "Copying files from %s…\n", sourceName)
	if running, _ := source.IsRunning(); running {
		env.DisplayToConsole(true, "%s is running: files it is writing right now may be copied mid-save\n", sourceName)
	}

	last := time.Now()
	stats, err := CopyServerTree(source.GetFileServer(), p.GetFileServer(), CopyOptions{
		UID: env.GetUID(),
		GID: env.GetGid(),
		Progress: func(s CopyStats) {
			if time.Since(last) < cloneProgressEvery {
				return
			}
			last = time.Now()
			env.DisplayToConsole(true, "Copied %d files (%s) so far…\n", s.Files, formatMegabytes(s.Bytes))
		},
	})
	if err != nil {
		return err
	}

	if err := copySideFiles(config.ServersFolder.Value(), source.ID(), p.ID()); err != nil {
		// the files are there; only the description of an installed modpack or map is missing
		p.Log(logging.Error, "Error copying side files from %s: %s", source.ID(), err)
	}

	if changed, err := remapPropertiesPorts(p.GetFileServer(), ClonePortMapping(source.Variables, p.Variables)); err != nil {
		env.DisplayToConsole(true, "Could not update the ports in server.properties: %s\n", err)
	} else if changed {
		env.DisplayToConsole(true, "server.properties now uses the copy's own ports\n")
	}

	env.DisplayToConsole(true, "Copied %d files (%s)\n", stats.Files, formatMegabytes(stats.Bytes))
	return nil
}

// CopyStats counts what a copy wrote
type CopyStats struct {
	Files   int
	Dirs    int
	Bytes   int64
	Skipped int
}

// CopyOptions tunes CopyServerTree
type CopyOptions struct {
	// UID and GID own every copied entry; -1 keeps the source's owner
	UID, GID int
	// Progress is called after every copied file
	Progress func(CopyStats)
}

type pendingDir struct {
	rel  string
	mode fs.FileMode
	info fs.FileInfo
}

// CopyServerTree copies every regular file, directory and in-root symlink of src into dst,
// keeping modes and times. Both sides are opened through their file servers, so nothing
// outside either root is read or written even if the source changes while it is copied.
// Symlinks that leave the source root, devices and sockets are skipped, and files that vanish
// mid-copy (a running server rotating its logs) are ignored.
func CopyServerTree(src, dst files.FileServer, opts CopyOptions) (CopyStats, error) {
	c := &treeCopy{src: src, dst: dst, opts: opts}
	if err := c.copyDir("."); err != nil {
		return c.stats, err
	}
	// directories get their own mode last, so a read-only one could still be filled
	for i := len(c.dirs) - 1; i >= 0; i-- {
		c.finishDir(c.dirs[i])
	}
	return c.stats, nil
}

type treeCopy struct {
	src, dst files.FileServer
	opts     CopyOptions
	stats    CopyStats
	dirs     []pendingDir
}

func (c *treeCopy) copyDir(rel string) error {
	entries, err := c.src.ReadDir(rel)
	if err != nil {
		if rel != "." && vanished(err) {
			c.stats.Skipped++
			return nil
		}
		return fmt.Errorf("reading %s: %w", rel, err)
	}
	for _, entry := range entries {
		child := path.Join(rel, entry.Name())
		if err := c.copyEntry(child, entry); err != nil {
			return err
		}
	}
	return nil
}

func (c *treeCopy) copyEntry(rel string, entry fs.DirEntry) error {
	switch {
	case entry.Type()&fs.ModeSymlink != 0:
		return c.copySymlink(rel)
	case entry.IsDir():
		// entry.Info() would look the name up from the working directory, not the root
		info, err := statInside(c.src, rel)
		if err != nil {
			if vanished(err) {
				c.stats.Skipped++
				return nil
			}
			return fmt.Errorf("reading %s: %w", rel, err)
		}
		if !info.IsDir() {
			c.stats.Skipped++
			return nil
		}
		if err := c.dst.Mkdir(rel, 0755); err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("creating %s: %w", rel, err)
		}
		c.dirs = append(c.dirs, pendingDir{rel: rel, mode: info.Mode().Perm(), info: info})
		c.stats.Dirs++
		return c.copyDir(rel)
	case entry.Type().IsRegular():
		return c.copyFile(rel)
	default:
		// devices, sockets and pipes are not part of a server
		c.stats.Skipped++
		return nil
	}
}

func (c *treeCopy) copyFile(rel string) error {
	in, err := c.src.OpenFile(rel, os.O_RDONLY, 0)
	if err != nil {
		if vanished(err) {
			c.stats.Skipped++
			return nil
		}
		return fmt.Errorf("opening %s: %w", rel, err)
	}
	defer utils.Close(in)

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("reading %s: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		// it changed into something else between listing and opening
		c.stats.Skipped++
		return nil
	}

	out, err := c.dst.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("creating %s: %w", rel, err)
	}
	written, copyErr := io.Copy(out, in)
	c.applyOwner(out, info)
	_ = out.Chmod(info.Mode().Perm())
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("copying %s: %w", rel, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("writing %s: %w", rel, closeErr)
	}
	c.keepTimes(rel, info)

	c.stats.Files++
	c.stats.Bytes += written
	if c.opts.Progress != nil {
		c.opts.Progress(c.stats)
	}
	return nil
}

func (c *treeCopy) copySymlink(rel string) error {
	target, err := os.Readlink(filepath.Join(c.src.Prefix(), filepath.FromSlash(rel)))
	if err != nil {
		if vanished(err) {
			c.stats.Skipped++
			return nil
		}
		return fmt.Errorf("reading link %s: %w", rel, err)
	}
	link, ok := symlinkInsideRoot(c.src.Prefix(), rel, target)
	if !ok {
		c.stats.Skipped++
		return nil
	}
	if err := c.dst.Symlink(link, filepath.FromSlash(rel)); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("creating link %s: %w", rel, err)
	}
	return nil
}

// symlinkInsideRoot returns the link text to recreate for the symlink at rel (relative to
// root) pointing at target, and false when it would lead outside root. An absolute target
// inside root is made relative, so the copy does not point back into the original.
func symlinkInsideRoot(root, rel, target string) (string, bool) {
	if target == "" {
		return "", false
	}
	linkDir := filepath.Dir(filepath.FromSlash(rel))
	if filepath.IsAbs(target) {
		inside, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
		if err != nil || !filepath.IsLocal(inside) {
			return "", false
		}
		relative, err := filepath.Rel(linkDir, inside)
		if err != nil {
			return "", false
		}
		return relative, true
	}
	if !filepath.IsLocal(filepath.Join(linkDir, target)) {
		return "", false
	}
	return target, true
}

func (c *treeCopy) finishDir(d pendingDir) {
	dir, err := c.dst.OpenFile(d.rel, os.O_RDONLY, 0)
	if err != nil {
		return
	}
	c.applyOwner(dir, d.info)
	_ = dir.Chmod(d.mode)
	utils.Close(dir)
	c.keepTimes(d.rel, d.info)
}

// applyOwner gives an entry the configured owner, or the source's one; it only works when the
// node runs as root, which is when ownership matters
func (c *treeCopy) applyOwner(f *os.File, info fs.FileInfo) {
	uid, gid := c.opts.UID, c.opts.GID
	if uid < 0 {
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return
		}
		uid, gid = int(st.Uid), int(st.Gid)
	}
	_ = f.Chown(uid, gid)
}

func (c *treeCopy) keepTimes(rel string, info fs.FileInfo) {
	_ = os.Chtimes(filepath.Join(c.dst.Prefix(), filepath.FromSlash(rel)), info.ModTime(), info.ModTime())
}

// statInside reads an entry's details through the file server, so it stays inside the root
func statInside(fsys files.FileServer, rel string) (fs.FileInfo, error) {
	f, err := fsys.OpenFile(rel, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer utils.Close(f)
	return f.Stat()
}

// vanished reports errors of entries that disappeared or turned into links while copying
func vanished(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.EXDEV) || errors.Is(err, syscall.ENOTDIR)
}

func isEmptyDir(fsys files.FileServer) (bool, error) {
	if fsys == nil {
		return false, errors.New("the server has no file access")
	}
	entries, err := fsys.ReadDir(".")
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

// copySideFiles copies the side files that describe installed content (<id>.modpack,
// <id>.worldmap), file or folder, to the new server's id
func copySideFiles(folder, sourceID, targetID string) error {
	var errs []error
	for _, ext := range cloneSideFiles {
		from := filepath.Join(folder, sourceID+ext)
		info, err := os.Lstat(from)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, err)
			}
			continue
		}
		to := filepath.Join(folder, targetID+ext)
		switch {
		case info.Mode().IsRegular():
			err = copyPlainFile(from, to, info.Mode().Perm())
		case info.IsDir():
			err = copyPlainDir(from, to)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", ext, err))
		}
	}
	return errors.Join(errs...)
}

func copyPlainDir(from, to string) error {
	if err := os.MkdirAll(to, 0755); err != nil {
		return err
	}
	src, err := files.NewFileServer(from, -1, -1)
	if err != nil {
		return err
	}
	defer utils.Close(src)
	dst, err := files.NewFileServer(to, -1, -1)
	if err != nil {
		return err
	}
	defer utils.Close(dst)
	_, err = CopyServerTree(src, dst, CopyOptions{UID: -1, GID: -1})
	return err
}

func copyPlainFile(from, to string, mode fs.FileMode) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer utils.Close(in)
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		utils.Close(out)
		return err
	}
	return out.Close()
}

var numberedPortKey = regexp.MustCompile(`^port\d*$`)

// knownPortKeys are template variables holding a port that do not follow the port/portN or
// *_port naming
var knownPortKeys = map[string]bool{
	"queryport": true, "rconport": true, "steamport": true, "gameport": true,
	"serverport": true, "voiceport": true, "webport": true, "httpport": true, "appport": true,
}

// IsPortVariable reports whether a template variable holds a network port that a copy of the
// server must not share with the original (port, port2…, query_port, rconport…)
func IsPortVariable(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if numberedPortKey.MatchString(k) || knownPortKeys[k] {
		return true
	}
	return strings.HasSuffix(k, "_port") || strings.HasSuffix(k, "-port") || strings.HasSuffix(k, ".port")
}

// PortValue reads a port variable's value; 0 when it is not a usable port
func PortValue(value interface{}) uint16 {
	n, err := cast.ToIntE(value)
	if err != nil || n <= 0 || n > 65535 {
		return 0
	}
	return uint16(n)
}

// ClonePortMapping pairs each port of the original with the one its copy got, by comparing
// the port variables both servers share
func ClonePortMapping(source, target map[string]skypanel.Variable) map[string]string {
	mapping := make(map[string]string)
	keys := make([]string, 0, len(source))
	for k := range source {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !IsPortVariable(k) {
			continue
		}
		t, ok := target[k]
		if !ok {
			continue
		}
		from, to := PortValue(source[k].Value), PortValue(t.Value)
		if from == 0 || to == 0 || from == to {
			continue
		}
		mapping[strconv.Itoa(int(from))] = strconv.Itoa(int(to))
	}
	return mapping
}

// remapPropertiesPorts points the copied server.properties at the copy's ports: it was written
// once at install time with the original's ports, and templates only write it when missing
func remapPropertiesPorts(fsys files.FileServer, mapping map[string]string) (bool, error) {
	if len(mapping) == 0 || fsys == nil {
		return false, nil
	}
	in, err := fsys.OpenFile("server.properties", os.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	data, err := io.ReadAll(io.LimitReader(in, maxPropertiesSize+1))
	utils.Close(in)
	if err != nil {
		return false, err
	}
	if len(data) > maxPropertiesSize {
		return false, errors.New("server.properties is too large")
	}

	updated, changed := RemapPropertyPorts(data, mapping)
	if !changed {
		return false, nil
	}
	out, err := fsys.OpenFile("server.properties", os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return false, err
	}
	if _, err := out.Write(updated); err != nil {
		utils.Close(out)
		return false, err
	}
	return true, out.Close()
}

// RemapPropertyPorts rewrites the values of key=value lines that equal one of the original's
// ports (server-port, query.port, rcon.port…) to the copy's port. Comments and other lines are
// kept byte for byte.
func RemapPropertyPorts(data []byte, mapping map[string]string) ([]byte, bool) {
	lines := strings.SplitAfter(string(data), "\n")
	changed := false
	for i, line := range lines {
		body := strings.TrimRight(line, "\r\n")
		ending := line[len(body):]
		trimmed := strings.TrimSpace(body)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			continue
		}
		eq := strings.IndexAny(body, "=:")
		if eq < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(body[:eq]))
		if !strings.Contains(key, "port") {
			continue
		}
		replacement, ok := mapping[strings.TrimSpace(body[eq+1:])]
		if !ok {
			continue
		}
		lines[i] = body[:eq+1] + replacement + ending
		changed = true
	}
	return []byte(strings.Join(lines, "")), changed
}

func formatMegabytes(bytes int64) string {
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}
