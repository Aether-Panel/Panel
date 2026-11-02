// Package security holds the checks every toolkit path and size goes through before the
// workspace touches a file: path cleaning, size and search limits and protected paths. It
// has no dependency on the rest of the toolkit, so every layer can use it.
package security

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"strings"
)

// Errors are user-safe: they never carry a host path, only the reason
var (
	ErrInvalidPath = errors.New("invalid path")
	ErrOutsideRoot = errors.New("the path must stay inside the server folder")
	ErrProtected   = errors.New("this path is protected and cannot be used")
)

const (
	// MaxPathLength bounds a path argument, in bytes
	MaxPathLength = 4096

	// MaxPathElements bounds how many folders deep a path argument may go
	MaxPathElements = 64

	// Root is how the server folder itself is written after CleanDir
	Root = "."
)

// LimitSet groups the size and search limits the toolkit applies; sizes are in bytes.
// MaxReadBytes is the largest file read_file returns, MaxWriteBytes the largest content
// create_file and write_file accept and MaxEditBytes the largest file edit_file works on,
// before and after the change. MaxListEntries is how many entries list_directory returns.
// A search goes at most MaxSearchDepth folders below its start, looks at MaxSearchFiles
// entries, returns MaxSearchResults matches, reads files up to MaxSearchFileBytes and takes
// a query of MaxQueryLength characters; a content match carries at most MaxMatchText
// characters of its line. SniffBytes is how much of a file is checked for NUL bytes to call
// it binary.
type LimitSet struct {
	MaxReadBytes       int64
	MaxWriteBytes      int64
	MaxEditBytes       int64
	MaxListEntries     int
	MaxSearchDepth     int
	MaxSearchFiles     int
	MaxSearchResults   int
	MaxSearchFileBytes int64
	MaxQueryLength     int
	MaxMatchText       int
	SniffBytes         int
}

// Limits are the limits in force. They are fixed values: nothing changes them at run time.
var Limits = LimitSet{
	MaxReadBytes:       1 << 20,
	MaxWriteBytes:      1 << 20,
	MaxEditBytes:       1 << 20,
	MaxListEntries:     1000,
	MaxSearchDepth:     8,
	MaxSearchFiles:     5000,
	MaxSearchResults:   200,
	MaxSearchFileBytes: 1 << 20,
	MaxQueryLength:     200,
	MaxMatchText:       300,
	SniffBytes:         8 << 10,
}

// ProtectedPaths lists server-relative paths (files or folders) the toolkit must not read or
// change. Empty by default: the panel's own files live next to the server folder, out of
// reach. It is meant to be set once at start-up, before any tool runs.
var ProtectedPaths []string

// CleanPath validates a path argument that must name a file or folder inside the server
// folder (not the folder itself) and returns it in clean, slash-separated form without a
// leading "./". Paths are always relative to the server folder: absolute paths, drive
// letters and ".." elements are refused rather than resolved, so nothing a model writes
// can point elsewhere.
func CleanPath(p string) (string, error) {
	cleaned, err := clean(p)
	if err != nil {
		return "", err
	}
	if cleaned == Root {
		return "", fmt.Errorf("%w: a file or folder name is required, not the server folder itself", ErrInvalidPath)
	}
	return cleaned, nil
}

// CleanDir validates a folder argument like CleanPath, but "" and "." mean the server folder
// itself, returned as Root (".")
func CleanDir(p string) (string, error) {
	return clean(p)
}

// IsRoot reports whether a cleaned path is the server folder itself
func IsRoot(p string) bool {
	return p == Root || p == ""
}

func clean(p string) (string, error) {
	p = strings.TrimSpace(p)
	if len(p) > MaxPathLength {
		return "", fmt.Errorf("%w: the path is longer than %d characters", ErrInvalidPath, MaxPathLength)
	}
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("%w: the path contains a NUL byte", ErrInvalidPath)
	}
	p = strings.ReplaceAll(p, `\`, "/")
	if hasDriveLetter(p) {
		return "", fmt.Errorf("%w: use a path relative to the server folder, not a drive letter", ErrOutsideRoot)
	}
	if strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("%w: use a path relative to the server folder, without a leading slash", ErrOutsideRoot)
	}
	for _, element := range strings.Split(p, "/") {
		if element == ".." {
			return "", fmt.Errorf("%w: \"..\" is not allowed in paths", ErrOutsideRoot)
		}
	}
	if p == "" {
		return Root, nil
	}
	cleaned := path.Clean(p)
	if cleaned != Root && strings.Count(cleaned, "/")+1 > MaxPathElements {
		return "", fmt.Errorf("%w: the path is more than %d folders deep", ErrInvalidPath, MaxPathElements)
	}
	return cleaned, nil
}

// hasDriveLetter spots Windows paths like "C:" or "c:/x", which would be a folder named "C:"
// on Linux but are never what the caller meant
func hasDriveLetter(p string) bool {
	if len(p) < 2 || p[1] != ':' {
		return false
	}
	c := p[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// IsProtected reports whether the cleaned path p is a protected path or inside one
func IsProtected(p string) bool {
	for _, protected := range ProtectedPaths {
		prot, err := CleanPath(protected)
		if err != nil {
			continue
		}
		if within(p, prot) {
			return true
		}
	}
	return false
}

// TouchesProtected reports whether changing the cleaned path p (moving or deleting it, or
// writing a folder there) would affect a protected path: p is protected, inside one, or
// contains one
func TouchesProtected(p string) bool {
	if IsProtected(p) {
		return true
	}
	for _, protected := range ProtectedPaths {
		prot, err := CleanPath(protected)
		if err != nil {
			continue
		}
		if within(prot, p) {
			return true
		}
	}
	return false
}

// within reports whether p is base or below it; the root contains everything
func within(p, base string) bool {
	if IsRoot(base) {
		return true
	}
	return p == base || strings.HasPrefix(p, base+"/")
}

// LooksBinary reports whether data (the start of a file) holds a NUL byte in its first
// SniffBytes bytes, the usual sign of a file that is not text
func LooksBinary(data []byte) bool {
	if len(data) > Limits.SniffBytes {
		data = data[:Limits.SniffBytes]
	}
	return bytes.IndexByte(data, 0) >= 0
}
