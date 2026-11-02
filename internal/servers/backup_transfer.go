package servers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

// transferredBackupName checks the name of a backup copied between nodes: a plain archive
// name in the server's backup folder. Hidden names are the partial archives and failure
// marks, which are never copied.
func transferredBackupName(fileName string) (string, error) {
	if !filepath.IsLocal(fileName) || filepath.Base(fileName) != fileName ||
		strings.HasPrefix(fileName, ".") || !strings.HasSuffix(fileName, ".tar.gz") {
		return "", fmt.Errorf("invalid backup file name: %s", fileName)
	}
	return fileName, nil
}

// ReceiveBackup writes a backup a node transfer copies here from the server's old node,
// under the name it had there, so the panel's record of it finds it on this node. Like a
// backup made here it is written under a hidden name and moved into place once complete, so
// a copy cut short is never listed or restored. It keeps the time it was made: the list
// order and the retention of automatic backups go by it. A backup with that name already
// here is the same backup and is left alone: the answer is ErrFileExists.
func (p *Server) ReceiveBackup(fileName string, modTime time.Time, contents io.Reader) error {
	name, err := transferredBackupName(fileName)
	if err != nil {
		return err
	}
	dir := p.GetBackupDirectory()
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	file := filepath.Join(dir, name)
	_, err = os.Stat(file)
	if err == nil {
		return skypanel.ErrFileExists
	}
	if !os.IsNotExist(err) {
		return err
	}

	partial := filepath.Join(dir, ".partial-"+name)
	out, err := os.OpenFile(partial, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, contents)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil && !modTime.IsZero() {
		err = os.Chtimes(partial, modTime, modTime)
	}
	if err == nil {
		err = os.Rename(partial, file)
	}
	if err != nil {
		_ = os.Remove(partial)
	}
	return err
}

// RemoveLocalBackup deletes this node's copy of a backup and nothing else. DeleteBackup also
// deletes the off-site copy, which belongs to the server wherever it runs; a node transfer
// that is undone only takes back the copies it brought here.
func (p *Server) RemoveLocalBackup(fileName string) error {
	name, err := transferredBackupName(fileName)
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(p.GetBackupDirectory(), name))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
