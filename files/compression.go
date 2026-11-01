package files

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/klauspost/compress/zip"
	"github.com/mholt/archives"
	"golang.org/x/sys/unix"
)

var ErrPathTraversal = errors.New("path traversal detected")

// ErrFileExists is an archive destination that is already there
var ErrFileExists = errors.New("file already exists")

const PathSeparator = "/"

type ExtractOptions struct {
	FileServer   FileServer
	SourceFile   string
	TargetPath   string
	Filter       string
	SkipRoot     bool
	ForcedWalker archives.Extractor
}

func safeJoin(base, path string) (string, error) {
	cleaned := filepath.Clean(filepath.Join(base, path))
	if !strings.HasPrefix(cleaned, filepath.Clean(base)+string(filepath.Separator)) && cleaned != filepath.Clean(base) {
		return "", fmt.Errorf("%w: %s", ErrPathTraversal, path)
	}
	return cleaned, nil
}

func DetermineIfSingleRoot(ctx context.Context, sourceFile string, file io.ReadSeeker) (bool, error) {
	isSingleRoot := true
	var rootName string
	var desired = errors.New("not single root")

	format, _, err := archives.Identify(ctx, sourceFile, file)
	if err != nil {
		return false, err
	}

	extractor, ok := format.(archives.Extractor)
	if !ok {
		return false, errors.New("format is not an extractor")
	}

	// Reset file pointer
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return false, err
	}

	err = extractor.Extract(ctx, file, func(_ context.Context, f archives.FileInfo) error {
		name := getCompressedItemName(f)

		if name == "" || name == PathSeparator {
			return nil
		}
		root := strings.Split(name, PathSeparator)[0]
		if rootName == "" {
			rootName = root
			return nil
		}
		if root != rootName {
			return desired
		}
		return nil
	})

	if err != nil && err != desired {
		isSingleRoot = false
	} else if err == desired {
		isSingleRoot = false
		err = nil
	}

	return isSingleRoot, err
}

func Extract(fs FileServer, sourceFile, targetPath, filter string, skipRoot bool, forcedType archives.Extractor) error {
	if fs == nil {
		return errors.New("fileserver is required")
	}

	file, err := fs.OpenFile(sourceFile, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer utils.Close(file)

	return extractFile(file, sourceFile, targetPath, filter, skipRoot, forcedType, fs, nil)
}

// WriteLimit wraps the writer of every extracted file, so the caller can count the bytes
// against a quota and stop the extraction (returning an error from Write) once it is used up
type WriteLimit func(io.Writer) io.Writer

// ExtractLimited is Extract with every file written through limit: a small archive can
// unpack to gigabytes, and without a cap it could fill the whole node's disk
func ExtractLimited(fs FileServer, sourceFile, targetPath, filter string, skipRoot bool, limit WriteLimit) error {
	if fs == nil {
		return errors.New("fileserver is required")
	}

	file, err := fs.OpenFile(sourceFile, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer utils.Close(file)

	return extractFile(file, sourceFile, targetPath, filter, skipRoot, nil, fs, limit)
}

// ExtractReader extracts an archive the caller already opened into fs. Backups live outside
// the server's file server, so they are opened directly and extracted through fs, which
// keeps every entry inside the server's root.
func ExtractReader(fs FileServer, reader io.ReadSeeker, sourceFile, targetPath, filter string, skipRoot bool) error {
	if fs == nil {
		return errors.New("fileserver is required")
	}
	return extractFile(reader, sourceFile, targetPath, filter, skipRoot, nil, fs, nil)
}

func ExtractFromReader(reader io.ReadSeeker, sourceFile, targetPath, filter string, skipRoot bool, forcedType archives.Extractor) error {
	if !filepath.IsLocal(targetPath) {
		return fmt.Errorf("%w: %s", ErrPathTraversal, targetPath)
	}
	return extractFile(reader, sourceFile, targetPath, filter, skipRoot, forcedType, nil, nil)
}

func extractFile(file io.ReadSeeker, sourceFile, targetPath, filter string, skipRoot bool, forcedType archives.Extractor, fs FileServer, limit WriteLimit) error {
	ctx := context.Background()

	if skipRoot {
		var err error
		skipRoot, err = DetermineIfSingleRoot(ctx, sourceFile, file)
		if err != nil {
			return err
		}
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
	}

	var extractor archives.Extractor
	if forcedType != nil {
		extractor = forcedType
	} else {
		format, _, err := archives.Identify(ctx, sourceFile, file)
		if err != nil {
			return err
		}
		var ok bool
		extractor, ok = format.(archives.Extractor)
		if !ok {
			return errors.New("format is not an extractor")
		}
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
	}

	return extractor.Extract(ctx, file, walker(fs, targetPath, filter, skipRoot, limit))
}

func Compress(fs FileServer, targetFile string, filesToCompress []string) error {
	return compress(fs, targetFile, filesToCompress, nil)
}

// CompressLimited is Compress with the archive written through limit, so it counts against
// the server's disk quota as it grows. An archive the limit cut short is deleted.
func CompressLimited(fs FileServer, targetFile string, filesToCompress []string, limit WriteLimit) error {
	return compress(fs, targetFile, filesToCompress, limit)
}

func compress(fs FileServer, targetFile string, filesToCompress []string, limit WriteLimit) error {
	if len(filesToCompress) == 0 {
		return errors.New("no files to compress")
	}

	ctx := context.Background()
	relativeTarget := targetFile
	var filesList []archives.FileInfo
	var err error
	if fs != nil {
		if targetFile, err = safeJoin(fs.Prefix(), targetFile); err != nil {
			return err
		}
		filesList, err = serverArchiveEntries(fs, relativeTarget, filesToCompress)
	} else {
		filenames := make(map[string]string)
		for _, f := range filesToCompress {
			filenames[f] = ""
		}
		filesList, err = archives.FilesFromDisk(ctx, nil, filenames)
	}
	if err != nil {
		return err
	}

	// through the file server, the archive cannot be written through a symlink that points
	// outside the server (the path check above only reads the text), and O_EXCL refuses to
	// overwrite an existing file
	var out *os.File
	if fs != nil {
		out, err = fs.OpenFile(relativeTarget, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	} else {
		out, err = os.OpenFile(targetFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	}
	if os.IsExist(err) {
		return ErrFileExists
	}
	if err != nil {
		return err
	}
	defer utils.Close(out)

	err = writeArchive(ctx, out, targetFile, filesList, limit)
	if err != nil && limit != nil {
		// half an archive is no use to anyone, and it is what took the disk past the limit
		utils.Close(out)
		if fs != nil {
			_ = fs.Remove(relativeTarget)
		} else {
			_ = os.Remove(targetFile)
		}
	}
	return err
}

// CompressServer archives the whole folder of fs into targetFile, outside of it (backups and
// restore points), every entry under a folder named like the server's. The files are listed
// and opened beneath the root, so a game that swaps a folder for a symlink while the backup
// runs cannot get host files into it.
func CompressServer(fs FileServer, targetFile string) error {
	walk := &archiveWalk{fs: fs}
	if err := walk.add("", filepath.Base(filepath.Clean(fs.Prefix()))); err != nil {
		return err
	}
	out, err := os.OpenFile(targetFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return ErrFileExists
	}
	if err != nil {
		return err
	}
	defer utils.Close(out)
	return writeArchive(context.Background(), out, targetFile, walk.entries, nil)
}

func writeArchive(ctx context.Context, out io.Writer, targetFile string, filesList []archives.FileInfo, limit WriteLimit) error {
	format, _, err := archives.Identify(ctx, targetFile, nil)
	if err != nil {
		return err
	}
	archiver, ok := format.(archives.Archiver)
	if !ok {
		return errors.New("format is not an archiver")
	}
	if limit != nil {
		out = limit(out)
	}
	return archiver.Archive(ctx, out, filesList)
}

// serverArchiveEntries resolves the requested files inside the server root, expanding a
// wildcard in the last part of a path and leaving out the archive being written.
func serverArchiveEntries(fs FileServer, target string, filesToCompress []string) ([]archives.FileInfo, error) {
	target = prepPath(target)
	walk := &archiveWalk{fs: fs}
	for _, v := range filesToCompress {
		v = prepPath(v)
		if v == "" || !filepath.IsLocal(v) {
			return nil, fmt.Errorf("%w: %s", ErrPathTraversal, v)
		}
		names := []string{v}
		if strings.Contains(v, "*") {
			// matched inside its folder through the file server: a wildcard in a folder
			// ("a*/shadow") would be expanded by the OS, through any symlink the game made
			var err error
			if names, err = fs.Glob(v); err != nil {
				return nil, err
			}
		}
		for _, name := range names {
			if name == target {
				continue
			}
			if err := walk.add(name, filepath.ToSlash(name)); err != nil {
				return nil, err
			}
		}
	}
	return walk.entries, nil
}

// archiveWalk lists files for an archive through the file server: every folder is read and
// every file opened beneath the root, never by its path on disk. Symlinks are stored as links,
// not followed, and pipes and devices are left out (reading a pipe would hang the archive).
type archiveWalk struct {
	fs      FileServer
	entries []archives.FileInfo
}

func (w *archiveWalk) add(name, inArchive string) error {
	info, link, err := lstatBeneath(w.fs, name)
	if err != nil {
		return err
	}
	entry := archives.FileInfo{FileInfo: info, NameInArchive: inArchive}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		entry.LinkTarget = link
	case info.IsDir():
		w.entries = append(w.entries, entry)
		children, err := w.fs.ReadDir(name)
		if err != nil {
			return err
		}
		for _, child := range children {
			err = w.add(filepath.Join(name, child.Name()), path.Join(inArchive, child.Name()))
			if err != nil && !errors.Is(err, fs.ErrNotExist) { // removed while listing
				return err
			}
		}
		return nil
	case info.Mode().IsRegular():
		entry.Open = func() (fs.File, error) { return w.openRegular(name, info.Size()) }
	default:
		return nil
	}
	w.entries = append(w.entries, entry)
	return nil
}

// openRegular opens a listed file to be read into the archive, at most the size it was listed
// with: a log that grew since would otherwise not fit its entry and fail the whole archive
func (w *archiveWalk) openRegular(name string, size int64) (fs.File, error) {
	f, err := w.fs.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		utils.Close(f)
		return nil, fmt.Errorf("%s is no longer a regular file", name)
	}
	return &sizedFile{File: f, r: io.LimitReader(f, size)}, nil
}

type sizedFile struct {
	*os.File
	r io.Reader
}

func (f *sizedFile) Read(b []byte) (int, error) {
	return f.r.Read(b)
}

// lstatBeneath describes name inside the file server without following it when it is a
// symlink, and reads the link's target in that case
func lstatBeneath(server FileServer, name string) (fs.FileInfo, string, error) {
	f, err := server.OpenFile(name, unix.O_PATH|unix.O_NOFOLLOW, 0)
	if errors.Is(err, unix.EXDEV) || errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
		// the way there goes through a symlink out of the server (ENOTDIR where openat2 is
		// off: each part is opened without following links, and a link is no folder)
		return nil, "", fmt.Errorf("%w: %s", ErrPathTraversal, name)
	}
	if err != nil {
		return nil, "", err
	}
	defer utils.Close(f)
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return info, "", err
	}
	buf := make([]byte, unix.PathMax)
	n, err := unix.Readlinkat(int(f.Fd()), "", buf)
	if err != nil {
		return nil, "", err
	}
	return info, string(buf[:n]), nil
}

func walker(fs FileServer, targetPath, filter string, skipRoot bool, limit WriteLimit) archives.FileHandler {
	return func(_ context.Context, file archives.FileInfo) (err error) {
		path := getCompressedItemName(file)

		// Trim leading slashes to prevent absolute path false positives from IsLocal
		path = strings.TrimLeft(path, "/\\")

		if !utils.CompareWildcard(file.Name(), filter) {
			return
		}

		if skipRoot {
			path = stripArchiveRoot(path)
		}

		if path == "" {
			return nil
		}
		if !filepath.IsLocal(path) {
			return ErrPathTraversal
		}

		dest, err := resolveExtractTarget(fs, targetPath, path)
		if err != nil || dest == nil {
			return err
		}

		switch {
		case file.IsDir():
			return extractMkdirAll(fs, dest.path)
		case file.Mode().IsRegular():
			return extractRegularFile(fs, file, dest, limit)
		case file.Mode()&os.ModeSymlink != 0:
			return extractSymlink(fs, file, dest)
		}

		return
	}
}

// extractTarget is where one archive entry lands: path is what gets written (relative to
// the file server, or absolute without one), absPath and root are used for the checks.
type extractTarget struct {
	path    string
	absPath string
	root    string
}

// stripArchiveRoot drops the first path element of an archive entry.
func stripArchiveRoot(path string) string {
	parts := strings.Split(path, PathSeparator)
	if len(parts) > 1 {
		return strings.Join(parts[1:], PathSeparator)
	}
	return ""
}

// resolveExtractTarget joins the entry onto the target path and rejects anything that
// escapes it. It returns nil (and no error) for the target root itself, which is skipped.
func resolveExtractTarget(fs FileServer, targetPath, path string) (*extractTarget, error) {
	root, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, err
	}
	joined := path
	if targetPath != "" {
		joined = filepath.Join(targetPath, path)
	}
	absPath, err := filepath.Abs(joined)
	if err != nil {
		return nil, err
	}
	if absPath == root {
		return nil, nil
	}
	relPath := filepath.Clean(joined)

	if fs != nil {
		if !filepath.IsLocal(relPath) {
			return nil, ErrPathTraversal
		}
		path = relPath
	} else {
		if !strings.HasPrefix(absPath, root+string(filepath.Separator)) {
			return nil, ErrPathTraversal
		}
		path = absPath
	}
	return &extractTarget{path: path, absPath: absPath, root: root}, nil
}

func extractMkdirAll(fs FileServer, path string) error {
	if fs != nil {
		return fs.MkdirAll(path, 0755)
	}
	return os.MkdirAll(path, 0755)
}

func extractRegularFile(fs FileServer, file archives.FileInfo, dest *extractTarget, limit WriteLimit) error {
	if err := extractMkdirAll(fs, filepath.Dir(dest.path)); err != nil {
		return err
	}
	var outFile *os.File
	var err error
	if fs != nil {
		outFile, err = fs.OpenFile(dest.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode()|0600)
	} else {
		outFile, err = os.OpenFile(dest.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode()|0600)
	}

	if err != nil {
		return err
	}
	defer utils.Close(outFile)

	r, err := file.Open()
	if err != nil {
		return err
	}
	defer utils.Close(r)

	var w io.Writer = outFile
	if limit != nil {
		w = limit(outFile)
	}
	_, err = io.Copy(w, r)
	return err
}

func extractSymlink(fs FileServer, file archives.FileInfo, dest *extractTarget) error {
	target, err := getLinkTarget(file)
	if err != nil {
		return err
	}
	resolvedTarget, err := filepath.Abs(filepath.Join(filepath.Dir(dest.absPath), target))
	if err != nil {
		return err
	}
	if !filepath.IsLocal(target) || (resolvedTarget != dest.root && !strings.HasPrefix(resolvedTarget, dest.root+string(filepath.Separator))) {
		return ErrPathTraversal
	}
	if err = extractMkdirAll(fs, filepath.Dir(dest.path)); err != nil {
		return err
	}
	if fs != nil {
		return fs.Symlink(target, dest.path)
	}
	return os.Symlink(target, dest.path)
}

// getCompressedItemName Resolves headers in the event the wrapped interface fails
func getCompressedItemName(file archives.FileInfo) string {
	if file.NameInArchive != "" {
		return file.NameInArchive
	}

	switch v := file.Header.(type) {
	case zip.FileHeader:
		return v.Name
	case *tar.Header:
		return v.Name
	default:
		return file.Name()
	}
}

func getLinkTarget(file archives.FileInfo) (string, error) {
	if file.LinkTarget != "" {
		return file.LinkTarget, nil
	}

	switch v := file.Header.(type) {
	case *tar.Header:
		return v.Linkname, nil
	case zip.FileHeader:
		// Not supported out of the box in mholt/archives for zip without manual read
		// but archives.FileInfo provides LinkTarget if it is parsed
		return "", errors.New("zip symlink unsupported")
	default:
		return "", errors.New("format not recognized")
	}
}
