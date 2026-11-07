package api

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"gorm.io/gorm"
)

// exTransferSweepEvery is how often the panel ends the federated transfer sessions that expired
// without finishing
const exTransferSweepEvery = 10 * time.Minute

// unfinishedExTransferStatuses are the states a session can still be in when the destination
// stops answering; COMPLETED, CANCELLED and FAILED have ended already
var unfinishedExTransferStatuses = []models.TransferStatus{
	models.StatusCreated, models.StatusValidated, models.StatusMigrating, models.StatusConsumed,
}

// originArchiveLocks keeps a sweep from removing a server's archive while a new one is made:
// both write the same file names in the server's folder
var originArchiveLocks sync.Map // server identifier -> *sync.Mutex

func lockOriginArchive(serverID string) (unlock func()) {
	m, _ := originArchiveLocks.LoadOrStore(serverID, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// SweepExpiredExTransfers ends the federated transfers this panel sends that expired unfinished
// (the destination crashed, lost the network or could not confirm), at start-up and then every
// exTransferSweepEvery. Without it their archive, a full copy of the server, would stay in the
// server's files and count toward its disk for good.
func SweepExpiredExTransfers() {
	db, err := database.GetConnection()
	if err != nil {
		logging.Error.Printf("Could not look for expired federated transfers: %s", err)
		return
	}
	ticker := time.NewTicker(exTransferSweepEvery)
	defer ticker.Stop()
	for {
		if _, err := sweepExpiredExTransfers(db, time.Now()); err != nil {
			logging.Error.Printf("Could not end the expired federated transfers: %s", err)
		}
		<-ticker.C
	}
}

// The log rows that track an archive: EXPIRE for a session that expired with its archive made,
// ARCHIVE_PENDING for a confirmed or cancelled one whose archive the node did not confirm
// removed; the sweep removes both until an ARCHIVE_REMOVED row says it is gone. EXPIRE_UNUSED
// is a token that expired before anything was archived.
const (
	exTransferLogExpired        = "EXPIRE"
	exTransferLogExpiredUnused  = "EXPIRE_UNUSED"
	exTransferLogArchivePending = "ARCHIVE_PENDING"
	exTransferLogArchiveRemoved = "ARCHIVE_REMOVED"
)

// sessionArchiveRemoval removes, when called, the archive the sessions (confirmed or cancelled)
// left, behind the same guards as the sweep, and when it could not (another transfer of the
// server is using the file, or the node did not confirm) records them so the sweep tries again.
// The removal it uses is read now, so callers can run it in a goroutine of its own.
func sessionArchiveRemoval(db *gorm.DB, serverID string, sessionIDs ...string) func() {
	remove := removeOriginArchive
	return func() {
		if removeOriginArchiveGuarded(db, serverID, time.Now(), remove) {
			return
		}
		for _, id := range sessionIDs {
			if err := logExTransfer(db, id, exTransferLogArchivePending, "", "The node did not confirm the archive was removed: the sweep tries again", true); err != nil {
				logging.Error.Printf("Could not record that the archive of session %s is still to be removed: %s", id, err)
			}
		}
	}
}

// sweepExpiredExTransfers marks the sessions that expired before now unfinished as FAILED, then
// removes the archive of every session it expired with one, retrying those a node did not
// confirm on an earlier run. It answers how many sessions it ended; a session it could not end
// is reported in the error and does not stop the rest.
func sweepExpiredExTransfers(db *gorm.DB, now time.Time) (int, error) {
	var errs []error
	var sessions []models.ExTransferSession
	if err := db.Where("status IN ? AND expires_at < ?", unfinishedExTransferStatuses, now).Order("id").Find(&sessions).Error; err != nil {
		errs = append(errs, err)
	}

	ended := 0
	for i := range sessions {
		session := &sessions[i]
		ok, err := expireExTransferSession(db, session, now)
		if err != nil {
			errs = append(errs, fmt.Errorf("session %s: %w", session.SessionUUID, err))
			continue
		}
		if !ok {
			continue
		}
		ended++
		if hadOriginArchive(session.Status) {
			stopOriginDownloads(session.SessionUUID)
			reportExTransfer(db, session.ServerID, services.NotificationText("extransfer.status.failedOrigin"), services.NotificationText("extransfer.expired"), true)
		}
	}

	if err := removeExpiredOriginArchives(db, now); err != nil {
		errs = append(errs, err)
	}
	return ended, errors.Join(errs...)
}

// hadOriginArchive is whether a session in this state had the archive made: only a consumed
// one, an unused token left nothing behind
func hadOriginArchive(status models.TransferStatus) bool {
	return status == models.StatusMigrating || status == models.StatusConsumed
}

// expireExTransferSession marks the session FAILED unless a heartbeat, confirm or cancel changed
// it since it was read; false when one did.
func expireExTransferSession(db *gorm.DB, session *models.ExTransferSession, now time.Time) (bool, error) {
	action := exTransferLogExpiredUnused
	if hadOriginArchive(session.Status) {
		action = exTransferLogExpired
	}
	expired := false
	err := db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.ExTransferSession{}).
			Where("id = ? AND status = ? AND expires_at < ?", session.ID, session.Status, now).
			Update("status", models.StatusFailed)
		if res.Error != nil || res.RowsAffected == 0 {
			return res.Error
		}
		expired = true
		return logExTransfer(tx, session.SessionUUID, action, "", "The destination stopped answering and the session expired", true)
	})
	return expired && err == nil, err
}

// removeExpiredOriginArchives removes the archive of every session with an EXPIRE or
// ARCHIVE_PENDING row and no ARCHIVE_REMOVED row yet, once per server, and records each removal
// the node confirmed
func removeExpiredOriginArchives(db *gorm.DB, now time.Time) error {
	var pending []models.ExTransferSession
	err := db.Select("session_uuid", "server_id").
		Where("session_uuid IN (?)", db.Model(&models.ExTransferLog{}).Select("session_id").Where("action IN ?", []string{exTransferLogExpired, exTransferLogArchivePending})).
		Where("session_uuid NOT IN (?)", db.Model(&models.ExTransferLog{}).Select("session_id").Where("action = ?", exTransferLogArchiveRemoved)).
		Order("id").Find(&pending).Error
	if err != nil {
		return err
	}

	byServer := make(map[string][]string)
	for _, session := range pending {
		byServer[session.ServerID] = append(byServer[session.ServerID], session.SessionUUID)
	}
	var errs []error
	for serverID, sessionIDs := range byServer {
		if !removeOriginArchiveGuarded(db, serverID, now, removeOriginArchive) {
			continue
		}
		for _, id := range sessionIDs {
			if err := logExTransfer(db, id, exTransferLogArchiveRemoved, "", "The archive the session left was removed", false); err != nil {
				errs = append(errs, fmt.Errorf("session %s: %w", id, err))
			}
		}
	}
	return errors.Join(errs...)
}

// removeOriginArchiveGuarded removes the server's archive with remove and reports whether the
// node confirmed it is gone. It leaves it, for a later run, while the server is in another
// transfer that uses the same file: a session still sending it (MIGRATING and live), or a
// pull into it running on this panel. The server's archive lock keeps a new archive from being
// started meanwhile.
func removeOriginArchiveGuarded(db *gorm.DB, serverID string, now time.Time, remove func(*gorm.DB, string) bool) bool {
	unlock := lockOriginArchive(serverID)
	defer unlock()

	if isPullRunning(serverID) {
		return false
	}
	var live int64
	if err := db.Model(&models.ExTransferSession{}).
		Where("server_id = ? AND status = ? AND expires_at >= ?", serverID, models.StatusMigrating, now).
		Count(&live).Error; err != nil {
		logging.Error.Printf("Could not check the transfers of %s before removing its archive: %s", serverID, err)
		return false
	}
	if live > 0 {
		return false
	}
	return remove(db, serverID)
}
