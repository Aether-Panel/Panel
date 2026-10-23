package servers

import (
	"errors"
	"os"
	"path"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gofrs/uuid/v5"
)

// Restore points are quick backups taken just before something risky (installing or removing
// a plugin, changing the game version, restoring another backup), so the step can be undone.
// They are kept apart from regular backups: only the newest few stay, and they never count
// against the owner's automatic-backup retention.

// RestorePointPrefix starts the file name of every restore point; the reason follows it,
// e.g. "restore-plugin-<uuid>.tar.gz"
const RestorePointPrefix = "restore-"

// Reasons a restore point is taken
const (
	RestoreReasonPlugin  = "plugin"
	RestoreReasonVersion = "version"
	RestoreReasonRestore = "restore"
	RestoreReasonModpack = "modpack"
)

// keepRestorePoints is how many restore points a server keeps
const keepRestorePoints = 3

// pluginRestoreGap avoids a new restore point for every plugin in a row of installs
const pluginRestoreGap = 30 * time.Minute

var errBackupBusy = errors.New("a backup or restore is already running")

// RestorePointReason reads the reason from a restore point's file name ("" if it is not one)
func RestorePointReason(fileName string) string {
	if !strings.HasPrefix(fileName, RestorePointPrefix) {
		return ""
	}
	rest := strings.TrimPrefix(fileName, RestorePointPrefix)
	if i := strings.Index(rest, "-"); i > 0 {
		return rest[:i]
	}
	return ""
}

// CreateRestorePoint takes a restore point before a risky step. For plugins it is skipped when
// a recent one exists. It waits for the archive, so the caller can go ahead safely afterwards.
func (p *Server) CreateRestorePoint(reason string) error {
	if reason == RestoreReasonPlugin && p.hasRecentRestorePoint(pluginRestoreGap) {
		return nil
	}
	if !p.backupState.CompareAndSwap(backupIdle, backupRunning) {
		return errBackupBusy
	}
	defer p.backupState.Store(backupIdle)
	return p.writeRestorePoint(reason)
}

// writeRestorePoint writes the archive; the caller already holds the backup state
func (p *Server) writeRestorePoint(reason string) error {
	id, err := uuid.NewV4()
	if err != nil {
		return err
	}
	p.RunningEnvironment.DisplayToConsole(true, "Creating a restore point")
	dir := p.GetBackupDirectory()
	file := path.Join(dir, RestorePointPrefix+reason+"-"+id.String()+".tar.gz")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := p.writeArchive(file); err != nil {
		p.Log(logging.Error, "Error creating restore point: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Could not create a restore point: %s", err)
		return err
	}
	p.RunningEnvironment.DisplayToConsole(true, "Restore point saved")
	if err := p.pruneRestorePoints(); err != nil {
		p.Log(logging.Error, "Error removing old restore points: %s", err)
	}
	return nil
}

func (p *Server) restorePoints() []skypanel.BackupFile {
	backups, err := p.ListBackups()
	if err != nil {
		return nil
	}
	points := make([]skypanel.BackupFile, 0)
	for _, b := range backups {
		if strings.HasPrefix(b.FileName, RestorePointPrefix) {
			points = append(points, b)
		}
	}
	return points
}

func (p *Server) hasRecentRestorePoint(within time.Duration) bool {
	for _, b := range p.restorePoints() {
		if time.Since(b.ModTime) < within {
			return true
		}
	}
	return false
}

// pruneRestorePoints keeps the newest few (ListBackups is newest first)
func (p *Server) pruneRestorePoints() error {
	for i, b := range p.restorePoints() {
		if i < keepRestorePoints {
			continue
		}
		if err := p.DeleteBackup(b.FileName); err != nil {
			return err
		}
	}
	return nil
}
