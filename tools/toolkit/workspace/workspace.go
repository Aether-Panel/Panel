// Package workspace is the toolkit's view of one server's files. A Workspace is bound to a
// single server when it is made and every path it takes is relative to that server's
// folder, so a tool has no way to name another server. The daemon implements it over the
// confined file server (NewFileServerWorkspace); the panel implements it by calling the
// node that hosts the server.
package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
)

// Entry describes a file or folder. Path is server-relative and slash separated; the server
// folder itself is ".".
type Entry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// Match is a search hit: a file or folder whose name matched, or a line of a file whose
// content matched (Line counts from 1, Text is the trimmed line)
type Match struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
	Text string `json:"text,omitempty"`
}

// SearchOptions tunes Search. Query is matched case-insensitively against entry names, or
// against file lines when InContent is set. Zero limits take the security defaults and
// limits above them are lowered to them (see ClampSearchOptions).
type SearchOptions struct {
	Query        string
	InContent    bool
	MaxResults   int
	MaxDepth     int
	MaxFiles     int
	MaxFileBytes int64
}

// Workspace reaches the files of one server. Paths are server-relative; every
// implementation validates them with the security package before use.
type Workspace interface {
	// Stat describes a file or folder; "" or "." is the server folder
	Stat(ctx context.Context, path string) (Entry, error)
	// List lists a folder, folders first then by name
	List(ctx context.Context, dir string) ([]Entry, error)
	// Read returns a file's content; ErrTooLarge when it is bigger than maxBytes
	Read(ctx context.Context, path string, maxBytes int64) ([]byte, error)
	// Write stores data in a file, making missing parent folders. Without overwrite it fails
	// with ErrExists when something is already there.
	Write(ctx context.Context, path string, data []byte, overwrite bool) error
	// Mkdir makes a folder and its parents; ErrExists when a file is in the way
	Mkdir(ctx context.Context, path string) error
	// Move moves or renames; ErrExists when dst exists
	Move(ctx context.Context, src, dst string) error
	// Copy copies a file or a folder tree; ErrExists when dst exists. Symlinks are never
	// followed: inside a copied folder they are skipped.
	Copy(ctx context.Context, src, dst string) error
	// Delete removes a file, or a folder with everything in it
	Delete(ctx context.Context, path string) error
	// Search looks for Query below dir, breadth first, within the options' limits
	Search(ctx context.Context, dir string, opts SearchOptions) ([]Match, error)
}

// Errors are user-safe: wrapped with a server-relative path at most, never a host path.
// The path errors are the security package's own values, so errors.Is matches either name.
var (
	ErrNotFound    = errors.New("not found")
	ErrExists      = errors.New("already exists")
	ErrTooLarge    = errors.New("too large")
	ErrNotDir      = errors.New("not a folder")
	ErrIsDir       = errors.New("is a folder")
	ErrSymlink     = errors.New("is a symbolic link, which is not followed")
	ErrNotRegular  = errors.New("not a regular file")
	ErrInvalid     = errors.New("invalid request")
	ErrPermission  = errors.New("permission denied")
	ErrNoSpace     = errors.New("no space left on the disk")
	ErrFailed      = errors.New("the file operation failed")
	ErrInvalidPath = security.ErrInvalidPath
	ErrOutsideRoot = security.ErrOutsideRoot
	ErrProtected   = security.ErrProtected
)

// ClampSearchOptions fills zero limits with the security defaults and lowers the ones above
// them. Callers that report truncation compare the number of matches with the clamped
// MaxResults.
func ClampSearchOptions(opts SearchOptions) SearchOptions {
	l := security.Limits
	if opts.MaxResults <= 0 || opts.MaxResults > l.MaxSearchResults {
		opts.MaxResults = l.MaxSearchResults
	}
	if opts.MaxDepth <= 0 || opts.MaxDepth > l.MaxSearchDepth {
		opts.MaxDepth = l.MaxSearchDepth
	}
	if opts.MaxFiles <= 0 || opts.MaxFiles > l.MaxSearchFiles {
		opts.MaxFiles = l.MaxSearchFiles
	}
	if opts.MaxFileBytes <= 0 || opts.MaxFileBytes > l.MaxSearchFileBytes {
		opts.MaxFileBytes = l.MaxSearchFileBytes
	}
	return opts
}
