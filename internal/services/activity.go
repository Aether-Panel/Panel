package services

import (
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"gorm.io/gorm"
)

// ActivityRetention is how long an access record is kept. Sign-in evidence is
// useful for a quarter; older rows are pruned whenever the feed is read.
const ActivityRetention = 90 * 24 * time.Hour

type Activity struct {
	DB *gorm.DB
}

// Record appends one access event. Because the caller must never fail the
// action it describes, callers log the error and keep going.
func (a *Activity) Record(event models.ActivityEvent, userID *uint, username string, email string, ip string, userAgent string) error {
	entry := &models.ActivityLog{
		Event:     event,
		UserID:    userID,
		Username:  username,
		Email:     email,
		IP:        ip,
		UserAgent: userAgent,
	}

	return a.DB.Create(entry).Error
}

// List returns the newest events first, with the total count for the requested
// filter. Rows past the retention window are dropped on the way in.
func (a *Activity) List(limit int, page int, event string) ([]*models.ActivityLog, int64, error) {
	_ = a.DB.
		Where("created_at < ?", time.Now().Add(-ActivityRetention)).
		Delete(&models.ActivityLog{}).
		Error

	query := func() *gorm.DB {
		tx := a.DB.Model(&models.ActivityLog{})
		if event != "" {
			tx = tx.Where("event = ?", event)
		}
		return tx
	}

	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	entries := make([]*models.ActivityLog, 0)
	offset := 0
	if page > 1 {
		offset = (page - 1) * limit
	}

	err := query().
		Order("created_at DESC, id DESC").
		Limit(limit).
		Offset(offset).
		Find(&entries).
		Error

	return entries, total, err
}
