package servers

import (
	"os"
	"path/filepath"
)

// writeFileAtomic replaces path with data in one step: it writes a temporary file next to
// it and renames it over. os.WriteFile truncates first, so a full disk or a crash halfway
// left an empty server definition, and an empty definition stopped the daemon from booting.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() {
		// gone after a successful rename; only a failed write leaves it to clean up
		_ = os.Remove(name)
	}()

	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(name, perm)
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
