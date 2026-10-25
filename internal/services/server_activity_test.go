package services

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func setupServerActivityDB(t *testing.T, servers ...string) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "server_activity.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Node{}, &models.Server{}, &models.ServerActivity{}))
	for _, id := range servers {
		require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: id, Name: id, Type: "generic"}).Error)
	}
	return db
}

func addServerActivity(t *testing.T, db *gorm.DB, serverID, action string, at time.Time) *models.ServerActivity {
	entry := &models.ServerActivity{ServerID: serverID, Action: action, Username: "alice", CreatedAt: at}
	require.NoError(t, db.Create(entry).Error)
	return entry
}

func actionsOf(entries []*models.ServerActivity) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Action
	}
	return out
}

func TestServerActivity_ListPaginatesNewestFirst(t *testing.T) {
	db := setupServerActivityDB(t, "srv1", "srv2")
	sa := &ServerActivity{DB: db}
	now := time.Now()
	for i := 0; i < 5; i++ {
		addServerActivity(t, db, "srv1", "server.start", now.Add(time.Duration(i)*time.Minute))
	}
	addServerActivity(t, db, "srv2", "server.stop", now)

	first, next, err := sa.List("srv1", 2, 0, "")
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.NotNil(t, next)
	assert.Greater(t, first[0].ID, first[1].ID)
	assert.Equal(t, first[1].ID, *next)

	second, next, err := sa.List("srv1", 2, *next, "")
	require.NoError(t, err)
	require.Len(t, second, 2)
	assert.Less(t, second[0].ID, first[1].ID)

	last, next, err := sa.List("srv1", 2, *next, "")
	require.NoError(t, err)
	assert.Len(t, last, 1)
	assert.Nil(t, next, "the last page has no cursor")
}

func TestServerActivity_ListCapsTheLimit(t *testing.T) {
	db := setupServerActivityDB(t, "srv1")
	sa := &ServerActivity{DB: db}
	for i := 0; i < ServerActivityMaxLimit+5; i++ {
		addServerActivity(t, db, "srv1", "file.write", time.Now())
	}

	entries, next, err := sa.List("srv1", 1000, 0, "")
	require.NoError(t, err)
	assert.Len(t, entries, ServerActivityMaxLimit)
	assert.NotNil(t, next)
}

func TestServerActivity_ListFilters(t *testing.T) {
	db := setupServerActivityDB(t, "srv1")
	sa := &ServerActivity{DB: db}
	now := time.Now()
	for _, action := range []string{"server.start", "server.create", "file.write", "plugin.install", "backup.restore", "settings.flags", "console.command"} {
		addServerActivity(t, db, "srv1", action, now)
	}

	cases := map[string][]string{
		"power":        {"server.start"},
		"files":        {"plugin.install", "file.write"},
		"settings":     {"settings.flags", "server.create"},
		"console":      {"console.command"},
		"backup":       {"backup.restore"},
		"backup.res":   {"backup.restore"},
		"file.":        {"file.write"},
		"%":            {},
		"file_%":       {},
		"x' OR '1'='1": {},
		"":             {"console.command", "settings.flags", "backup.restore", "plugin.install", "file.write", "server.create", "server.start"},
	}
	for filter, want := range cases {
		entries, _, err := sa.List("srv1", 50, 0, filter)
		require.NoError(t, err, filter)
		assert.Equal(t, want, actionsOf(entries), "filter %q", filter)
	}
}

func TestServerActivity_PruneDropsOldAndOrphanedEntries(t *testing.T) {
	db := setupServerActivityDB(t, "srv1")
	sa := &ServerActivity{DB: db}
	now := time.Now()
	old := addServerActivity(t, db, "srv1", "server.start", now.Add(-ServerActivityRetention-time.Hour))
	recent := addServerActivity(t, db, "srv1", "server.stop", now.Add(-ServerActivityRetention+time.Hour))
	orphan := addServerActivity(t, db, "gone", "server.stop", now)

	require.NoError(t, sa.Prune(now))

	var ids []uint
	require.NoError(t, db.Model(&models.ServerActivity{}).Order("id").Pluck("id", &ids).Error)
	assert.Equal(t, []uint{recent.ID}, ids)
	assert.NotContains(t, ids, old.ID)
	assert.NotContains(t, ids, orphan.ID)
}

func TestServerActivity_RecordPrunesAtMostHourly(t *testing.T) {
	db := setupServerActivityDB(t, "srv1")
	sa := &ServerActivity{DB: db}
	previous := lastServerActivityPrune.Load()
	t.Cleanup(func() { lastServerActivityPrune.Store(previous) })

	addServerActivity(t, db, "srv1", "server.start", time.Now().Add(-ServerActivityRetention-time.Hour))
	lastServerActivityPrune.Store(0)
	require.NoError(t, sa.Record(&models.ServerActivity{ServerID: "srv1", Action: "server.stop"}))

	var count int64
	require.NoError(t, db.Model(&models.ServerActivity{}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "the expired entry is pruned on insert")

	// a second expired row stays until the next hourly pass
	addServerActivity(t, db, "srv1", "server.start", time.Now().Add(-ServerActivityRetention-time.Hour))
	require.NoError(t, sa.Record(&models.ServerActivity{ServerID: "srv1", Action: "server.kill"}))
	require.NoError(t, db.Model(&models.ServerActivity{}).Count(&count).Error)
	assert.Equal(t, int64(3), count)
}

func TestServerActivity_RecordTrimsDetail(t *testing.T) {
	db := setupServerActivityDB(t, "srv1")
	sa := &ServerActivity{DB: db}
	lastServerActivityPrune.Store(time.Now().Unix())

	entry := &models.ServerActivity{ServerID: "srv1", Action: "file.write", Detail: strings.Repeat("ñ", 400)}
	require.NoError(t, sa.Record(entry))

	var stored models.ServerActivity
	require.NoError(t, db.First(&stored, entry.ID).Error)
	assert.Equal(t, models.ServerActivityDetailMax, len([]rune(stored.Detail)))
	assert.True(t, strings.HasSuffix(stored.Detail, "…"))
}

func TestServerActivity_DeleteForServer(t *testing.T) {
	db := setupServerActivityDB(t, "srv1", "srv2")
	sa := &ServerActivity{DB: db}
	addServerActivity(t, db, "srv1", "server.start", time.Now())
	addServerActivity(t, db, "srv2", "server.start", time.Now())

	require.NoError(t, sa.DeleteForServer("srv1"))

	var left []string
	require.NoError(t, db.Model(&models.ServerActivity{}).Pluck("server_id", &left).Error)
	assert.Equal(t, []string{"srv2"}, left)
}

func TestTruncateRunes(t *testing.T) {
	assert.Equal(t, "hola", TruncateRunes("hola", 10))
	assert.Equal(t, "ho…", TruncateRunes("hola!", 3))
	assert.Equal(t, "ñ", TruncateRunes("ññ", 1))
}
