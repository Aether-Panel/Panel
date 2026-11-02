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

// addTrackedServer creates the server row TrackStatus checks for
func addTrackedServer(t *testing.T, db *gorm.DB, serverID string) {
	require.NoError(t, db.Create(&models.Server{Name: serverID, Identifier: serverID, Type: "generic"}).Error)
}

// confirmedAt moves the time the server's open period was last confirmed
func confirmedAt(t *testing.T, db *gorm.DB, serverID string, at time.Time) {
	require.NoError(t, db.Model(&models.UptimeStatus{}).
		Where("server_id = ? AND end_time IS NULL", serverID).
		UpdateColumn("updated_at", at).Error)
}

func uptimeRows(t *testing.T, db *gorm.DB, serverID string) []models.UptimeStatus {
	var rows []models.UptimeStatus
	require.NoError(t, db.Where("server_id = ?", serverID).Order("start_time ASC, id ASC").Find(&rows).Error)
	return rows
}

func TestUptime_RecordNodeStatuses_StartsAndChangesPeriods(t *testing.T) {
	db := setupUptimeTestDB(t)
	addTrackedServer(t, db, "srv1")
	us := &Uptime{DB: db}

	require.NoError(t, us.RecordNodeStatuses([]string{"srv1"}, map[string]bool{"srv1": true}, time.Minute))
	require.NoError(t, us.RecordNodeStatuses([]string{"srv1"}, map[string]bool{"srv1": true}, time.Minute))

	rows := uptimeRows(t, db, "srv1")
	require.Len(t, rows, 1, "the same state keeps the same period")
	assert.True(t, rows[0].IsRunning)
	assert.Nil(t, rows[0].EndTime)

	require.NoError(t, us.RecordNodeStatuses([]string{"srv1"}, map[string]bool{"srv1": false}, time.Minute))

	rows = uptimeRows(t, db, "srv1")
	require.Len(t, rows, 2)
	assert.True(t, rows[0].IsRunning)
	assert.NotNil(t, rows[0].EndTime, "the running period ends when the server stops")
	assert.False(t, rows[1].IsRunning)
	assert.Nil(t, rows[1].EndTime)
}

// A node that does not answer used to be impossible to tell from one whose servers all stopped.
// Now nothing is recorded for it and its open periods are left alone.
func TestUptime_RecordNodeStatuses_UnreachableNodeRecordsNothing(t *testing.T) {
	db := setupUptimeTestDB(t)
	addTrackedServer(t, db, "srv1")
	us := &Uptime{DB: db}
	require.NoError(t, us.RecordNodeStatuses([]string{"srv1"}, map[string]bool{"srv1": true}, time.Hour))

	require.NoError(t, us.RecordNodeStatuses([]string{"srv1"}, nil, time.Hour))

	rows := uptimeRows(t, db, "srv1")
	require.Len(t, rows, 1, "no stopped period for a node that did not answer")
	assert.True(t, rows[0].IsRunning)
	assert.Nil(t, rows[0].EndTime, "the open period stays open")
}

// A server the node left out of its answer is unknown, not stopped
func TestUptime_RecordNodeStatuses_MissingServerRecordsNothing(t *testing.T) {
	db := setupUptimeTestDB(t)
	addTrackedServer(t, db, "srv1")
	addTrackedServer(t, db, "srv2")

	require.NoError(t, (&Uptime{DB: db}).RecordNodeStatuses([]string{"srv1", "srv2"}, map[string]bool{"srv1": true}, time.Hour))

	assert.Len(t, uptimeRows(t, db, "srv1"), 1)
	assert.Empty(t, uptimeRows(t, db, "srv2"))
}

// When the node stays out of reach, the open period is closed at its last confirmation, so the
// outage counts as neither up nor down; when the node answers again a new period starts.
func TestUptime_RecordNodeStatuses_LongOutageLeavesAGap(t *testing.T) {
	db := setupUptimeTestDB(t)
	addTrackedServer(t, db, "srv1")
	us := &Uptime{DB: db}
	now := time.Now()
	started := now.Add(-3 * time.Hour)
	lastSeen := now.Add(-2 * time.Hour)
	addUptimePeriod(t, db, "srv1", true, started, nil)
	confirmedAt(t, db, "srv1", lastSeen)

	require.NoError(t, us.RecordNodeStatuses([]string{"srv1"}, nil, 5*time.Minute))

	rows := uptimeRows(t, db, "srv1")
	require.Len(t, rows, 1, "nothing new is recorded while the node is out of reach")
	require.NotNil(t, rows[0].EndTime)
	assert.WithinDuration(t, lastSeen, *rows[0].EndTime, time.Second, "closed when it was last confirmed")
	assert.InDelta(t, 3600, rows[0].Duration, 1)

	up, down, _, err := us.GetUptimeStats("srv1", now.Add(-4*time.Hour))
	require.NoError(t, err)
	assert.InDelta(t, 3600, up, 5, "the outage is not counted as up")
	assert.Zero(t, down, "nor as down")

	require.NoError(t, us.RecordNodeStatuses([]string{"srv1"}, map[string]bool{"srv1": true}, 5*time.Minute))

	rows = uptimeRows(t, db, "srv1")
	require.Len(t, rows, 2)
	assert.True(t, rows[1].IsRunning)
	assert.Nil(t, rows[1].EndTime)
	assert.WithinDuration(t, time.Now(), rows[1].StartTime, 5*time.Second, "the new period starts when the node answers")
}

// A period confirmed recently survives a missed poll
func TestUptime_RecordNodeStatuses_ShortOutageKeepsThePeriod(t *testing.T) {
	db := setupUptimeTestDB(t)
	addTrackedServer(t, db, "srv1")
	addUptimePeriod(t, db, "srv1", true, time.Now().Add(-time.Hour), nil)
	confirmedAt(t, db, "srv1", time.Now().Add(-time.Minute))

	require.NoError(t, (&Uptime{DB: db}).RecordNodeStatuses([]string{"srv1"}, nil, 5*time.Minute))

	rows := uptimeRows(t, db, "srv1")
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].EndTime)
}
