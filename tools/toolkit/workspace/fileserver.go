package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"golang.org/x/sys/unix"
)

// fileServerWorkspace works only through the server's files.FileServer, which opens every
// path beneath the server folder (openat2 with RESOLVE_BENEATH, or one folder at a time
// without following links): a symlink that leaves the folder is refused by the kernel, not
// by string checks here. On top of that, paths are cleaned by the security package first and
// links are never followed where a mistake would cost data (writes, copies, deletes, search).
type fileServerWorkspace struct {
	fs files.FileServer
}

// NewFileServerWorkspace returns a Workspace over the files of one server
func NewFileServerWorkspace(fsys files.FileServer) Workspace {
	return &fileServerWorkspace{fs: fsys}
}

func (w *fileServerWorkspace) Stat(ctx context.Context, p string) (Entry, error) {
	rel, err := w.prepare(ctx, p, true)
	if err != nil {
		return Entry{}, err
	}
	if security.IsProtected(rel) {
		return Entry{}, protectedError(rel)
	}
	info, err := w.follow(rel)
	if err != nil {
		return Entry{}, err
	}
	return entryFor(rel, info), nil
}

func (w *fileServerWorkspace) List(ctx context.Context, dir string) ([]Entry, error) {
	rel, err := w.prepare(ctx, dir, true)
	if err != nil {
		return nil, err
	}
	if security.IsProtected(rel) {
		return nil, protectedError(rel)
	}
	items, err := w.fs.ReadDir(rel)
	if err != nil {
		return nil, w.translate(err, rel)
	}
	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		child := path.Join(rel, item.Name())
		if security.IsProtected(child) {
			continue
		}
		info, err := w.lstat(child)
		if err != nil {
			// it went away while listing
			continue
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			// a link inside the folder is described by what it points at; one that leaves
			// it, or is broken, is listed as itself and cannot be opened
			if target, err := w.fs.Stat(child); err == nil {
				info = target
			}
		}
		entries = append(entries, entryFor(child, info))
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func (w *fileServerWorkspace) Read(ctx context.Context, p string, maxBytes int64) ([]byte, error) {
	rel, err := w.prepare(ctx, p, false)
	if err != nil {
		return nil, err
	}
	if security.IsProtected(rel) {
		return nil, protectedError(rel)
	}
	if maxBytes <= 0 {
		maxBytes = security.Limits.MaxReadBytes
	}
	f, err := w.fs.OpenFile(rel, os.O_RDONLY, 0)
	if err != nil {
		return nil, w.translate(err, rel)
	}
	defer closeQuietly(f)
	info, err := f.Stat()
	if err != nil {
		return nil, w.translate(err, rel)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrIsDir, rel)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s", ErrNotRegular, rel)
	}
	if info.Size() > maxBytes {
		return nil, tooLarge(rel, maxBytes)
	}
	// the file may grow between the stat and the read
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, w.translate(err, rel)
	}
	if int64(len(data)) > maxBytes {
		return nil, tooLarge(rel, maxBytes)
	}
	return data, nil
}

func (w *fileServerWorkspace) Write(ctx context.Context, p string, data []byte, overwrite bool) error {
	rel, err := w.prepare(ctx, p, false)
	if err != nil {
		return err
	}
	if security.IsProtected(rel) {
		return protectedError(rel)
	}
	info, err := w.lstat(rel)
	switch {
	case err == nil && !overwrite:
		return fmt.Errorf("%w: %s", ErrExists, rel)
	case err == nil:
		if err := writable(rel, info); err != nil {
			return err
		}
	case !errors.Is(err, ErrNotFound):
		return err
	}
	if err := w.mkdirAll(path.Dir(rel)); err != nil {
		return err
	}

	// O_NOFOLLOW: a link put in place after the check above is refused, not written through;
	// O_EXCL makes "create" atomic, so two writers cannot both create the same file
	flags := os.O_WRONLY | os.O_CREATE | unix.O_NOFOLLOW
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	f, err := w.fs.OpenFile(rel, flags, 0644)
	if err != nil {
		return w.translate(err, rel)
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		if !overwrite {
			// a half-written new file is useless; an overwritten one is gone either way
			_ = w.fs.Remove(rel)
		}
		return w.translate(writeErr, rel)
	}
	return nil
}

func (w *fileServerWorkspace) Mkdir(ctx context.Context, p string) error {
	rel, err := w.prepare(ctx, p, false)
	if err != nil {
		return err
	}
	if security.IsProtected(rel) {
		return protectedError(rel)
	}
	info, err := w.lstat(rel)
	switch {
	case err == nil && info.IsDir():
		return nil
	case err == nil:
		return fmt.Errorf("%w: %s", ErrExists, rel)
	case !errors.Is(err, ErrNotFound):
		return err
	}
	return w.mkdirAll(rel)
}

func (w *fileServerWorkspace) Move(ctx context.Context, src, dst string) error {
	from, to, err := w.prepareTransfer(ctx, src, dst)
	if err != nil {
		return err
	}
	if security.TouchesProtected(from) || security.TouchesProtected(to) {
		return protectedError(from)
	}
	if _, err := w.lstat(from); err != nil {
		return err
	}
	if err := w.mustNotExist(to); err != nil {
		return err
	}
	if err := w.mkdirAll(path.Dir(to)); err != nil {
		return err
	}
	// Rename works on the entry itself: a symlink is moved as a link, never followed
	if err := w.fs.Rename(from, to); err != nil {
		return w.translate(err, from)
	}
	return nil
}

func (w *fileServerWorkspace) Copy(ctx context.Context, src, dst string) error {
	from, to, err := w.prepareTransfer(ctx, src, dst)
	if err != nil {
		return err
	}
	if security.TouchesProtected(from) || security.TouchesProtected(to) {
		return protectedError(from)
	}
	info, err := w.lstat(from)
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", ErrSymlink, from)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrNotRegular, from)
	}
	if err := w.mustNotExist(to); err != nil {
		return err
	}
	if err := w.mkdirAll(path.Dir(to)); err != nil {
		return err
	}
	if info.IsDir() {
		return w.copyTree(ctx, from, to)
	}
	return w.copyFile(ctx, from, to)
}

func (w *fileServerWorkspace) Delete(ctx context.Context, p string) error {
	rel, err := w.prepare(ctx, p, false)
	if err != nil {
		return err
	}
	if security.TouchesProtected(rel) {
		return protectedError(rel)
	}
	info, err := w.lstat(rel)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		// only the link goes; what it points at is not touched
		err = w.unlinkLink(rel)
	case info.IsDir():
		err = w.fs.RemoveAll(rel)
	default:
		err = w.fs.Remove(rel)
	}
	if err != nil {
		return w.translate(err, rel)
	}
	return nil
}

func (w *fileServerWorkspace) Search(ctx context.Context, dir string, opts SearchOptions) ([]Match, error) {
	rel, err := w.prepare(ctx, dir, true)
	if err != nil {
		return nil, err
	}
	if security.IsProtected(rel) {
		return nil, protectedError(rel)
	}
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return nil, fmt.Errorf("%w: the search text is empty", ErrInvalid)
	}
	if utf8.RuneCountInString(opts.Query) > security.Limits.MaxQueryLength {
		return nil, fmt.Errorf("%w: the search text is longer than %d characters", ErrInvalid, security.Limits.MaxQueryLength)
	}
	opts = ClampSearchOptions(opts)
	if opts.InContent {
		// a content search keeps the spaces the caller asked for
		query = opts.Query
	}
	s := &search{w: w, opts: opts, needle: []byte(strings.ToLower(query)), matches: []Match{}}
	if err := s.run(ctx, rel); err != nil {
		return nil, err
	}
	return s.matches, nil
}

// prepare checks the context and cleans a path argument; dir allows the server folder
func (w *fileServerWorkspace) prepare(ctx context.Context, p string, dir bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if w.fs == nil {
		return "", fmt.Errorf("%w: the server has no file access", ErrFailed)
	}
	if dir {
		return security.CleanDir(p)
	}
	return security.CleanPath(p)
}

func (w *fileServerWorkspace) prepareTransfer(ctx context.Context, src, dst string) (string, string, error) {
	from, err := w.prepare(ctx, src, false)
	if err != nil {
		return "", "", err
	}
	to, err := w.prepare(ctx, dst, false)
	if err != nil {
		return "", "", err
	}
	if from == to {
		return "", "", fmt.Errorf("%w: %s", ErrExists, to)
	}
	if strings.HasPrefix(to, from+"/") {
		return "", "", fmt.Errorf("%w: %s cannot be put inside itself", ErrInvalid, from)
	}
	return from, to, nil
}

// lstat describes rel without following it when it is a symlink. The way there is still
// resolved beneath the root: a folder on the way that links out is refused.
func (w *fileServerWorkspace) lstat(rel string) (fs.FileInfo, error) {
	if security.IsRoot(rel) {
		info, err := w.fs.Stat(security.Root)
		return info, w.translate(err, rel)
	}
	f, err := w.fs.OpenFile(rel, unix.O_PATH|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, w.translate(err, rel)
	}
	defer closeQuietly(f)
	info, err := f.Stat()
	if err != nil {
		return nil, w.translate(err, rel)
	}
	return info, nil
}

// follow describes rel, following a symlink that stays inside the server folder
func (w *fileServerWorkspace) follow(rel string) (fs.FileInfo, error) {
	info, err := w.lstat(rel)
	if err != nil || info.Mode()&fs.ModeSymlink == 0 {
		return info, err
	}
	target, err := w.fs.Stat(rel)
	if err != nil {
		return nil, w.translate(err, rel)
	}
	return target, nil
}

func (w *fileServerWorkspace) mustNotExist(rel string) error {
	_, err := w.lstat(rel)
	if err == nil {
		return fmt.Errorf("%w: %s", ErrExists, rel)
	}
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// mkdirAll makes dir and its parents and checks a folder is there in the end: MkdirAll
// ignores "exists", which a file in the way would also give
func (w *fileServerWorkspace) mkdirAll(dir string) error {
	if security.IsRoot(dir) {
		return nil
	}
	if err := w.fs.MkdirAll(dir, 0755); err != nil {
		return w.translate(err, dir)
	}
	info, err := w.fs.Stat(dir)
	if err != nil {
		return w.translate(err, dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrNotDir, dir)
	}
	return nil
}

// unlinkLink removes the symlink rel itself. The file server's Remove would open what the
// link points at, so the link is unlinked from its folder, opened beneath the root.
func (w *fileServerWorkspace) unlinkLink(rel string) error {
	folder, err := w.fs.OpenFile(path.Dir(rel), os.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer closeQuietly(folder)
	return unix.Unlinkat(int(folder.Fd()), path.Base(rel), 0)
}

func (w *fileServerWorkspace) copyFile(ctx context.Context, from, to string) error {
	in, err := w.fs.OpenFile(from, os.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return w.translate(err, from)
	}
	defer closeQuietly(in)
	info, err := in.Stat()
	if err != nil {
		return w.translate(err, from)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrNotRegular, from)
	}
	out, err := w.fs.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW, info.Mode().Perm())
	if err != nil {
		return w.translate(err, to)
	}
	copyErr := copyWithContext(ctx, out, in)
	closeErr := out.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		_ = w.fs.Remove(to)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return w.translate(copyErr, to)
	}
	return nil
}

// copyTree copies the folder from into the new folder to: folders and regular files only.
// Every entry is described without following it, so a symlink (to another server, the
// host, or inside the folder) is skipped and never read through; so are devices and pipes.
func (w *fileServerWorkspace) copyTree(ctx context.Context, from, to string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.fs.Mkdir(to, 0755); err != nil {
		return w.translate(err, to)
	}
	items, err := w.fs.ReadDir(from)
	if err != nil {
		return w.translate(err, from)
	}
	for _, item := range items {
		src := path.Join(from, item.Name())
		dst := path.Join(to, item.Name())
		info, err := w.lstat(src)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			err = w.copyTree(ctx, src, dst)
		case info.Mode().IsRegular():
			err = w.copyFile(ctx, src, dst)
		default:
			// a symlink (described, not followed, it is neither of the above), a device, a
			// socket or a pipe: not copied
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// search walks breadth first: shallow matches come before deep ones and the limits cut the
// deepest part of the tree
type search struct {
	w       *fileServerWorkspace
	opts    SearchOptions
	needle  []byte
	visited int
	matches []Match
}

type searchDir struct {
	rel   string
	depth int
}

func (s *search) run(ctx context.Context, start string) error {
	info, err := s.w.follow(start)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrNotDir, start)
	}
	queue := []searchDir{{rel: start}}
	for len(queue) > 0 && !s.done() {
		if err := ctx.Err(); err != nil {
			return err
		}
		current := queue[0]
		queue = queue[1:]
		subfolders, err := s.searchDir(current, current.rel == start)
		if err != nil {
			return err
		}
		queue = append(queue, subfolders...)
	}
	return nil
}

// searchDir looks at the entries of one folder and returns its subfolders still to walk. A
// folder that went away or cannot be read is left out, unless it is where the search starts.
func (s *search) searchDir(current searchDir, isStart bool) ([]searchDir, error) {
	items, err := s.w.fs.ReadDir(current.rel)
	if err != nil {
		if isStart {
			return nil, s.w.translate(err, current.rel)
		}
		return nil, nil
	}
	// the file server does not sort; a stable order makes results repeatable for the model
	sort.Slice(items, func(i, j int) bool { return items[i].Name() < items[j].Name() })
	var subfolders []searchDir
	for _, item := range items {
		if s.done() {
			break
		}
		s.visited++
		if next, ok := s.visit(current, item.Name()); ok {
			subfolders = append(subfolders, next)
		}
	}
	return subfolders, nil
}

// visit matches one entry and reports a subfolder to walk when it is one within the depth
// limit. Links are described, never followed: a link's name can match, nothing behind it is
// searched.
func (s *search) visit(current searchDir, name string) (searchDir, bool) {
	child := path.Join(current.rel, name)
	if security.IsProtected(child) {
		return searchDir{}, false
	}
	info, err := s.w.lstat(child)
	if err != nil {
		return searchDir{}, false
	}
	if !s.opts.InContent && s.nameMatches(name) {
		s.matches = append(s.matches, Match{Path: child})
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return searchDir{}, false
	case info.IsDir():
		return searchDir{rel: child, depth: current.depth + 1}, current.depth+1 < s.opts.MaxDepth
	case s.opts.InContent && info.Mode().IsRegular() && info.Size() <= s.opts.MaxFileBytes:
		s.searchFile(child)
	}
	return searchDir{}, false
}

func (s *search) done() bool {
	return len(s.matches) >= s.opts.MaxResults || s.visited >= s.opts.MaxFiles
}

func (s *search) nameMatches(name string) bool {
	return bytes.Contains([]byte(strings.ToLower(name)), s.needle)
}

// searchFile adds the lines of a text file that hold the query; binary files are skipped
func (s *search) searchFile(rel string) {
	f, err := s.w.fs.OpenFile(rel, os.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return
	}
	defer closeQuietly(f)
	data, err := io.ReadAll(io.LimitReader(f, s.opts.MaxFileBytes+1))
	if err != nil || int64(len(data)) > s.opts.MaxFileBytes || security.LooksBinary(data) {
		return
	}
	for number := 1; len(data) > 0 && len(s.matches) < s.opts.MaxResults; number++ {
		line := data
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			line, data = data[:i], data[i+1:]
		} else {
			data = nil
		}
		if bytes.Contains(bytes.ToLower(line), s.needle) {
			s.matches = append(s.matches, Match{Path: rel, Line: number, Text: clip(strings.TrimSpace(string(line)), security.Limits.MaxMatchText)})
		}
	}
}

// clip shortens s to at most limit characters, marking the cut
func clip(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit-1]) + "…"
}

func entryFor(rel string, info fs.FileInfo) Entry {
	e := Entry{Name: path.Base(rel), Path: rel, IsDir: info.IsDir(), ModTime: info.ModTime()}
	if security.IsRoot(rel) {
		e.Name, e.Path = security.Root, security.Root
	}
	if !e.IsDir {
		e.Size = info.Size()
	}
	return e
}

// writable checks an existing entry may be overwritten as a file
func writable(rel string, info fs.FileInfo) error {
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return fmt.Errorf("%w: %s", ErrSymlink, rel)
	case info.IsDir():
		return fmt.Errorf("%w: %s", ErrIsDir, rel)
	case !info.Mode().IsRegular():
		return fmt.Errorf("%w: %s", ErrNotRegular, rel)
	}
	return nil
}

func protectedError(rel string) error {
	return fmt.Errorf("%w: %s", ErrProtected, rel)
}

func tooLarge(rel string, limit int64) error {
	return fmt.Errorf("%w: %s is bigger than %d bytes", ErrTooLarge, rel, limit)
}

// translate turns a file server error into a user-safe one. The file server's own errors
// can name the host folder (opening the root goes by its host path), so only the
// server-relative rel is shown; the original stays reachable through errors.Unwrap for logs.
func (w *fileServerWorkspace) translate(err error, rel string) error {
	if err == nil {
		return nil
	}
	var known error
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, fs.ErrNotExist):
		known = ErrNotFound
	case errors.Is(err, fs.ErrExist), errors.Is(err, unix.ENOTEMPTY):
		known = ErrExists
	case errors.Is(err, unix.EXDEV), errors.Is(err, unix.ELOOP), errors.Is(err, files.ErrPathTraversal):
		// openat2 answers EXDEV for a way out of the root; ELOOP is a link where none is followed
		known = ErrOutsideRoot
	case errors.Is(err, unix.ENOTDIR) && w.throughLink(rel):
		// without openat2 every folder on the way is opened without following links, and a
		// link is no folder: say why instead of "not a folder"
		return &safeError{msg: fmt.Sprintf("%s: %s goes through a symbolic link, which is not followed", ErrOutsideRoot.Error(), rel), kind: ErrOutsideRoot, cause: err}
	case errors.Is(err, unix.ENOTDIR):
		known = ErrNotDir
	case errors.Is(err, unix.EISDIR):
		known = ErrIsDir
	case errors.Is(err, fs.ErrPermission):
		known = ErrPermission
	case errors.Is(err, unix.ENOSPC), errors.Is(err, unix.EDQUOT):
		known = ErrNoSpace
	default:
		known = ErrFailed
	}
	return &safeError{msg: fmt.Sprintf("%s: %s", known.Error(), rel), kind: known, cause: err}
}

// throughLink reports whether a folder on the way to rel is a symlink
func (w *fileServerWorkspace) throughLink(rel string) bool {
	parts := strings.Split(rel, "/")
	for i := 1; i < len(parts); i++ {
		f, err := w.fs.OpenFile(strings.Join(parts[:i], "/"), unix.O_PATH|unix.O_NOFOLLOW, 0)
		if err != nil {
			return false
		}
		info, err := f.Stat()
		closeQuietly(f)
		if err != nil {
			return false
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

// safeError shows a user-safe message while errors.Is still matches both the toolkit
// sentinel and the original cause
type safeError struct {
	msg   string
	kind  error
	cause error
}

func (e *safeError) Error() string { return e.msg }

func (e *safeError) Unwrap() []error { return []error{e.kind, e.cause} }

// copyWithContext copies src to dst in chunks, stopping a long copy when the request is
// cancelled
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) error {
	buf := make([]byte, 32<<10)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func closeQuietly(f *os.File) {
	_ = f.Close()
}
