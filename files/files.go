package files

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"golang.org/x/sys/unix"
)

// CreateIn creates (or empties) dest for writing, dest being a path inside root. It is opened
// beneath root through the file server and the file itself is never a symlink: installs run
// as root, and a game that left server.jar pointing at /etc/... would otherwise have the
// download overwrite that host file. Missing folders on the way are made.
func CreateIn(root, dest string) (*os.File, error) {
	rel := dest
	if filepath.IsAbs(dest) {
		var err error
		if rel, err = filepath.Rel(filepath.Clean(root), filepath.Clean(dest)); err != nil {
			return nil, err
		}
	}
	rel = filepath.Clean(rel)
	if !filepath.IsLocal(rel) {
		return nil, fmt.Errorf("%w: %s", ErrPathTraversal, dest)
	}
	server, err := NewFileServer(root, -1, -1)
	if err != nil {
		return nil, err
	}
	defer utils.Close(server)
	if dir := filepath.Dir(rel); dir != "." {
		if err = server.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}
	return server.OpenFile(rel, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|unix.O_NOFOLLOW, 0644)
}

// CopyFile copies src, a file of the panel (its cache), to dest inside root
func CopyFile(src, root, dest string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer utils.Close(source)
	return WriteFile(source, root, dest)
}

// WriteFile writes src to dest inside root (see CreateIn)
func WriteFile(src io.Reader, root, dest string) error {
	destination, err := CreateIn(root, dest)
	if err != nil {
		return err
	}
	defer utils.Close(destination)
	_, err = io.Copy(destination, src)
	return err
}
