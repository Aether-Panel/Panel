package api

import (
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

// sweepExpiredExTransfers marks the sessions that expired before now unfinished as FAILED and
// removes the archive of those that had started sending it. It answers how many it ended.
func sweepExpiredExTransfers(db *gorm.DB, now time.Time) (int, error) {
	var sessions []models.ExTransferSession
	if err := db.Where("status IN ? AND expires_at < ?", unfinishedExTransferStatuses, now).Order("id").Find(&sessions).Error; err != nil {
		return 0, err
	}

	ended := 0
	withArchive := make(map[string]bool)
	for i := range sessions {
		session := &sessions[i]
		ok, err := expireExTransferSession(db, session, now)
		if err != nil {
			return ended, err
		}
		if !ok {
			continue
		}
		ended++
		// only a consumed session had the archive made; an unused token left nothing behind
		if session.Status != models.StatusMigrating && session.Status != models.StatusConsumed {
			continue
		}
		stopOriginDownloads(session.SessionUUID)
		reportExTransfer(db, session.ServerID, services.NotificationText("extransfer.status.failedOrigin"), services.NotificationText("extransfer.expired"), true)
		withArchive[session.ServerID] = true
	}

	for serverID := range withArchive {
		removeExpiredOriginArchive(db, serverID, now)
	}
	return ended, nil
}

// expireExTransferSession marks the session FAILED unless a heartbeat, confirm or cancel changed
// it since it was read; false when one did.
func expireExTransferSession(db *gorm.DB, session *models.ExTransferSession, now time.Time) (bool, error) {
	expired := false
	err := db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.ExTransferSession{}).
			Where("id = ? AND status = ? AND expires_at < ?", session.ID, session.Status, now).
			Update("status", models.StatusFailed)
		if res.Error != nil || res.RowsAffected == 0 {
			return res.Error
		}
		expired = true
		return logExTransfer(tx, session.SessionUUID, "EXPIRE", "", "The destination stopped answering and the session expired", true)
	})
	return expired && err == nil, err
}

// removeExpiredOriginArchive removes the archive an expired session left, unless the server is
// in another transfer that uses the same file: a newer session sending it, or a pull into it
// running on this panel.
func removeExpiredOriginArchive(db *gorm.DB, serverID string, now time.Time) {
	unlock := lockOriginArchive(serverID)
	defer unlock()

	if isPullRunning(serverID) {
		return
	}
	var live int64
	if err := db.Model(&models.ExTransferSession{}).
		Where("server_id = ? AND status = ? AND expires_at >= ?", serverID, models.StatusMigrating, now).
		Count(&live).Error; err != nil {
		logging.Error.Printf("Could not check the transfers of %s before removing its archive: %s", serverID, err)
		return
	}
	if live > 0 {
		// that session's own end removes the archive
		return
	}
	removeOriginArchive(db, serverID)
}
