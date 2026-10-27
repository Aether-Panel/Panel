package files

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/klauspost/compress/zip"
	"github.com/mholt/archives"
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

	relativeTarget := targetFile
	if fs != nil {
		p := fs.Prefix()

		var err error
		targetFile, err = safeJoin(p, targetFile)
		if err != nil {
			return err
		}

		filesToCompress, err = expandCompressFiles(p, targetFile, filesToCompress)
		if err != nil {
			return err
		}
	}

	ctx := context.Background()

	filenames, err := archiveNames(fs, filesToCompress)
	if err != nil {
		return err
	}

	filesList, err := archives.FilesFromDisk(ctx, nil, filenames)
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

	format, _, err := archives.Identify(ctx, targetFile, nil)
	if err != nil {
		return err
	}

	archiver, ok := format.(archives.Archiver)
	if !ok {
		return errors.New("format is not an archiver")
	}

	var w io.Writer = out
	if limit != nil {
		w = limit(out)
	}
	err = archiver.Archive(ctx, w, filesList)
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

// insideRealRoot checks that dir, with every symlink resolved, is still inside root
func insideRealRoot(root, dir string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if realDir != realRoot && !strings.HasPrefix(realDir, realRoot+string(filepath.Separator)) {
		return fmt.Errorf("%w: %s", ErrPathTraversal, dir)
	}
	return nil
}

// expandCompressFiles resolves the requested files inside the server root, expanding globs
// and leaving out the archive being written.
func expandCompressFiles(prefix, targetFile string, filesToCompress []string) ([]string, error) {
	var expandedFiles []string
	for _, v := range filesToCompress {
		fullPath, err := safeJoin(prefix, v)
		if err != nil {
			return nil, err
		}
		// the files are read by path from disk: a symlinked folder on the way could point
		// anywhere on the host and its files would land in an archive the user downloads
		if err = insideRealRoot(prefix, filepath.Dir(fullPath)); err != nil {
			return nil, err
		}
		if strings.Contains(v, "*") {
			matches, _ := filepath.Glob(fullPath)
			for _, match := range matches {
				if match != targetFile {
					expandedFiles = append(expandedFiles, match)
				}
			}
		} else if fullPath != targetFile {
			expandedFiles = append(expandedFiles, fullPath)
		}
	}
	return expandedFiles, nil
}

// archiveNames builds the mapping for archives.FilesFromDisk.
// The second value in the map is the NameInArchive (relative path inside the tar).
// When a FileServer is used, we MUST set this to the path relative to the server
// root, otherwise archives.FilesFromDisk will use the absolute disk path's last
// component (e.g. "mc-pc-local") as the root directory name inside the archive.
// This causes "path traversal detected" errors during extraction on the destination.
func archiveNames(fs FileServer, filesToCompress []string) (map[string]string, error) {
	filenames := make(map[string]string)
	if fs == nil {
		for _, f := range filesToCompress {
			filenames[f] = ""
		}
		return filenames, nil
	}

	prefix := filepath.Clean(fs.Prefix())
	for _, f := range filesToCompress {
		rel, err := filepath.Rel(prefix, f)
		if err != nil {
			return nil, err
		}
		// Normalize separators to forward slashes for cross-platform tarballs
		rel = filepath.ToSlash(rel)
		filenames[f] = rel
	}
	return filenames, nil
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
