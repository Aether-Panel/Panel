package services

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func setupUptimeTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "uptime_test.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permissions{}, &models.Client{}, &models.Server{}, &models.Node{}, &models.UptimeStatus{}))
	return db
}

func addUptimePeriod(t *testing.T, db *gorm.DB, serverID string, running bool, start time.Time, end *time.Time) {
	period := &models.UptimeStatus{ServerID: serverID, IsRunning: running, StartTime: start, EndTime: end}
	if end != nil {
		period.Duration = int64(end.Sub(start).Seconds())
	}
	require.NoError(t, db.Omit(clause.Associations).Create(period).Error)
}

// seedUptime leaves, for a window starting 10 hours ago: 5h up (from a period that began
// 20 hours ago), 3h down, and the current run of 2h up. A period that ended before the
// window does not count.
func seedUptime(t *testing.T, db *gorm.DB, serverID string, now time.Time) time.Time {
	hoursAgo := func(h int) *time.Time {
		at := now.Add(-time.Duration(h) * time.Hour)
		return &at
	}
	addUptimePeriod(t, db, serverID, true, *hoursAgo(30), hoursAgo(15))
	addUptimePeriod(t, db, serverID, true, *hoursAgo(20), hoursAgo(5))
	addUptimePeriod(t, db, serverID, false, *hoursAgo(5), hoursAgo(2))
	addUptimePeriod(t, db, serverID, true, *hoursAgo(2), nil)
	return *hoursAgo(10)
}

func TestUptimePeriod_SecondsInWindow(t *testing.T) {
	now := time.Now()
	since := now.Add(-time.Hour)
	before := now.Add(-2 * time.Hour)
	inside := now.Add(-30 * time.Minute)

	// started before the window: only the part inside counts
	assert.EqualValues(t, 1800, uptimePeriod{StartTime: before, EndTime: &inside}.secondsInWindow(since, now))
	// open period: lasts until now
	assert.EqualValues(t, 1800, uptimePeriod{StartTime: inside}.secondsInWindow(since, now))
	// ended before the window
	assert.EqualValues(t, 0, uptimePeriod{StartTime: before.Add(-time.Hour), EndTime: &before}.secondsInWindow(since, now))
}

// A period that started before the window used to be dropped, so a server up for the whole
// window showed no uptime at all.
func TestUptime_GetUptimeStats_ClipsToWindow(t *testing.T) {
	db := setupUptimeTestDB(t)
	since := seedUptime(t, db, "srv1", time.Now())
	addUptimePeriod(t, db, "other", false, since, nil)

	up, down, percent, err := (&Uptime{DB: db}).GetUptimeStats("srv1", since)

	require.NoError(t, err)
	assert.InDelta(t, 7*3600, up, 5)
	assert.InDelta(t, 3*3600, down, 5)
	assert.InDelta(t, 70.0, percent, 0.1)
}

func TestUptime_GetUptimeStats_NoData(t *testing.T) {
	db := setupUptimeTestDB(t)

	up, down, percent, err := (&Uptime{DB: db}).GetUptimeStats("srv1", time.Now().Add(-time.Hour))

	require.NoError(t, err)
	assert.Zero(t, up)
	assert.Zero(t, down)
	assert.Equal(t, 100.0, percent)
}

func TestUptime_GetAllServerUptime_ClipsToWindow(t *testing.T) {
	db := setupUptimeTestDB(t)
	since := seedUptime(t, db, "srv1", time.Now())

	stats, err := (&Uptime{DB: db}).GetAllServerUptime(since)

	require.NoError(t, err)
	require.Contains(t, stats, "srv1")
	assert.InDelta(t, 7*3600, stats["srv1"]["uptime"].(int64), 5)
	assert.InDelta(t, 3*3600, stats["srv1"]["downtime"].(int64), 5)
	assert.InDelta(t, 70.0, stats["srv1"]["uptimePercent"].(float64), 0.1)
	assert.Equal(t, true, stats["srv1"]["currentStatus"])
}
