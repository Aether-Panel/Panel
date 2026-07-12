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

	var err error
	targetPath, err = safeJoin(fs.Prefix(), targetPath)
	if err != nil {
		return err
	}

	file, err := fs.OpenFile(sourceFile, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer utils.Close(file)

	return extractFile(file, sourceFile, targetPath, filter, skipRoot, forcedType, fs)
}

func ExtractFromReader(reader io.ReadSeeker, sourceFile, targetPath, filter string, skipRoot bool, forcedType archives.Extractor) error {
	if !filepath.IsLocal(targetPath) {
		return fmt.Errorf("%w: %s", ErrPathTraversal, targetPath)
	}
	return extractFile(reader, sourceFile, targetPath, filter, skipRoot, forcedType, nil)
}

func extractFile(file io.ReadSeeker, sourceFile, targetPath, filter string, skipRoot bool, forcedType archives.Extractor, fs FileServer) error {
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

	return extractor.Extract(ctx, file, walker(fs, targetPath, filter, skipRoot))
}

func Compress(fs FileServer, targetFile string, filesToCompress []string) error {
	if len(filesToCompress) == 0 {
		return errors.New("no files to compress")
	}

	if fs != nil {
		p := fs.Prefix()

		var err error
		targetFile, err = safeJoin(p, targetFile)
		if err != nil {
			return err
		}

		var expandedFiles []string
		for _, v := range filesToCompress {
			fullPath, err := safeJoin(p, v)
			if err != nil {
				return err
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
		filesToCompress = expandedFiles
	}

	ctx := context.Background()

	// Create mapping for archives.FilesFromDisk
	filenames := make(map[string]string)
	for _, f := range filesToCompress {
		filenames[f] = ""
	}

	filesList, err := archives.FilesFromDisk(ctx, nil, filenames)
	if err != nil {
		return err
	}

	out, err := os.Create(targetFile)
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

	return archiver.Archive(ctx, out, filesList)
}

func walker(fs FileServer, targetPath, filter string, skipRoot bool) archives.FileHandler {
	return func(_ context.Context, file archives.FileInfo) (err error) {
		path := getCompressedItemName(file)

		if !utils.CompareWildcard(file.Name(), filter) {
			return
		}

		if skipRoot {
			path = strings.Join(strings.Split(path, PathSeparator)[1:], PathSeparator)
		}

		joined, err := safeJoin(targetPath, path)
		if err != nil {
			return err
		}
		parent := filepath.Dir(joined)
		path = joined

		switch {
		case file.IsDir():
			if fs != nil {
				if err = fs.MkdirAll(path, 0755); err != nil {
					return err
				}
			} else {
				if err = os.MkdirAll(path, 0755); err != nil {
					return err
				}
			}
		case file.Mode().IsRegular():
			if fs != nil {
				if err = fs.MkdirAll(parent, 0755); err != nil {
					return err
				}
			} else {
				if err = os.MkdirAll(parent, 0755); err != nil {
					return err
				}
			}
			var outFile *os.File
			if fs != nil {
				outFile, err = fs.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode()|0600)
			} else {
				outFile, err = os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode()|0600)
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

			if _, err = io.Copy(outFile, r); err != nil {
				return err
			}
		case file.Mode()&os.ModeSymlink != 0:
			target, err := getLinkTarget(file)
			if err != nil {
				return err
			}

			if fs != nil {
				if err = fs.MkdirAll(parent, 0755); err != nil {
					return err
				}
				if err = fs.Symlink(target, path); err != nil {
					return err
				}
			} else {
				if err = os.MkdirAll(parent, 0755); err != nil {
					return err
				}
				if err = os.Symlink(target, path); err != nil {
					return err
				}
			}
		}

		return
	}
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
