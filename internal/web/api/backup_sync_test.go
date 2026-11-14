package api

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestApplyNodeBackups_DropsFailedManualBackups(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "backups.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Node{}, &models.Server{}, &models.Backup{}))

	server := &models.Server{Identifier: "srv01"}
	bs := &services.Backup{DB: db}
	for _, name := range []string{"done.tar.gz", "broken.tar.gz", "writing.tar.gz", servers.AutoBackupPrefix + "gone.tar.gz"} {
		require.NoError(t, bs.Create(&models.Backup{Name: "backup", FileName: name, ServerID: server.Identifier}))
	}

	failed, err := applyNodeBackups(bs, server, []skypanel.BackupFile{
		{FileName: "done.tar.gz", Size: 10, Location: "local"},
		{FileName: "broken.tar.gz", Location: servers.BackupFailedLocation},
		{FileName: "unknown.tar.gz", Location: servers.BackupFailedLocation},
		{FileName: servers.AutoBackupPrefix + "new.tar.gz", Size: 5, Location: "local"},
	}, true)
	require.NoError(t, err)

	// every failed one is cleared on the node, recorded or not
	assert.ElementsMatch(t, []string{"broken.tar.gz", "unknown.tar.gz"}, failed)

	records, err := bs.GetAllForServer(server.Identifier)
	require.NoError(t, err)
	byName := map[string]*models.Backup{}
	for _, r := range records {
		byName[r.FileName] = r
	}
	assert.NotContains(t, byName, "broken.tar.gz", "its archive was never written")
	assert.NotContains(t, byName, "unknown.tar.gz")
	assert.Contains(t, byName, "writing.tar.gz", "a backup still being written keeps its entry")
	assert.NotContains(t, byName, servers.AutoBackupPrefix+"gone.tar.gz")
	assert.Contains(t, byName, servers.AutoBackupPrefix+"new.tar.gz")
	if assert.Contains(t, byName, "done.tar.gz") {
		assert.Equal(t, int64(10), byName["done.tar.gz"].FileSize)
		assert.Equal(t, "local", byName["done.tar.gz"].Location)
	}
}

func TestSyncBackups_RecordsPeriodicCopies(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "backups.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Node{}, &models.Server{}, &models.Backup{}))

	server := &models.Server{Identifier: "srv01"}
	bs := &services.Backup{DB: db}
	periodic := servers.OffsiteBackupPrefix + "0b5c9a1e-3f2d-4c8e-9a7b-6d1e2f3a4b5c.tar.gz"

	// the node deletes the local archive once uploaded, so the copy is only in the bucket
	_, err = applyNodeBackups(bs, server, []skypanel.BackupFile{
		{FileName: periodic, Size: 42, Location: "offsite"},
	}, true)
	require.NoError(t, err)

	records, err := bs.GetAllForServer(server.Identifier)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "Periodic backup (S3)", records[0].Name)
	assert.Equal(t, periodic, records[0].FileName)
	assert.Equal(t, int64(42), records[0].FileSize)
	assert.Equal(t, "offsite", records[0].Location)

	// a later sync where it is still only in the bucket keeps it
	_, err = applyNodeBackups(bs, server, []skypanel.BackupFile{
		{FileName: periodic, Size: 42, Location: "offsite"},
	}, true)
	require.NoError(t, err)
	records, err = bs.GetAllForServer(server.Identifier)
	require.NoError(t, err)
	assert.Len(t, records, 1)

	// one seen while its archive was still on the node follows it into the bucket
	next := servers.OffsiteBackupPrefix + "7e2d4f60-1a3b-4c5d-8e9f-0a1b2c3d4e5f.tar.gz"
	_, err = applyNodeBackups(bs, server, []skypanel.BackupFile{
		{FileName: periodic, Size: 42, Location: "offsite"},
		{FileName: next, Size: 50, Location: "local"},
	}, true)
	require.NoError(t, err)
	_, err = applyNodeBackups(bs, server, []skypanel.BackupFile{
		{FileName: periodic, Size: 42, Location: "offsite"},
		{FileName: next, Size: 50, Location: "offsite"},
	}, true)
	require.NoError(t, err)
	records, err = bs.GetAllForServer(server.Identifier)
	require.NoError(t, err)
	byName := map[string]*models.Backup{}
	for _, r := range records {
		byName[r.FileName] = r
	}
	if assert.Contains(t, byName, next) {
		assert.Equal(t, "Periodic backup (S3)", byName[next].Name)
		assert.Equal(t, "offsite", byName[next].Location)
	}

	// once the retention removes it from the bucket the node stops listing it
	_, err = applyNodeBackups(bs, server, []skypanel.BackupFile{}, true)
	require.NoError(t, err)
	records, err = bs.GetAllForServer(server.Identifier)
	require.NoError(t, err)
	assert.Empty(t, records)
}

// newBackupSyncDB opens an empty panel database for one test
func newBackupSyncDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "backups.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Node{}, &models.Server{}, &models.Backup{}))
	return db
}

// When the node could not list the bucket its list lacks the copies only the bucket has: the
// panel keeps their records, and does not take a copy in both places for a local one
func TestApplyNodeBackups_KeepsOffsiteRecordsWhenTheBucketIsUnknown(t *testing.T) {
	server := &models.Server{Identifier: "srv01"}
	bs := &services.Backup{DB: newBackupSyncDB(t)}
	periodic := servers.OffsiteBackupPrefix + "0b5c9a1e-3f2d-4c8e-9a7b-6d1e2f3a4b5c.tar.gz"
	both := servers.AutoBackupPrefix + "both.tar.gz"
	pruned := servers.AutoBackupPrefix + "pruned.tar.gz"
	gone := servers.AutoBackupPrefix + "gone.tar.gz"
	require.NoError(t, bs.Create(&models.Backup{Name: "Periodic backup (S3)", FileName: periodic, FileSize: 42, Location: "offsite", ServerID: server.Identifier}))
	require.NoError(t, bs.Create(&models.Backup{Name: "Automatic backup", FileName: both, FileSize: 7, Location: "both", ServerID: server.Identifier}))
	require.NoError(t, bs.Create(&models.Backup{Name: "Automatic backup", FileName: pruned, FileSize: 8, Location: "both", ServerID: server.Identifier}))
	require.NoError(t, bs.Create(&models.Backup{Name: "Automatic backup", FileName: gone, FileSize: 9, Location: "local", ServerID: server.Identifier}))
	byName := func() map[string]*models.Backup {
		records, err := bs.GetAllForServer(server.Identifier)
		require.NoError(t, err)
		result := map[string]*models.Backup{}
		for _, r := range records {
			result[r.FileName] = r
		}
		return result
	}
	before := byName()

	_, err := applyNodeBackups(bs, server, []skypanel.BackupFile{
		{FileName: both, Size: 7, Location: "local"},
	}, false)
	require.NoError(t, err)

	records := byName()
	if assert.Contains(t, records, periodic, "a copy only the bucket has keeps its record") {
		assert.Equal(t, before[periodic].ID, records[periodic].ID, "and its id, so open links still work")
		assert.Equal(t, "offsite", records[periodic].Location)
	}
	if assert.Contains(t, records, both) {
		assert.Equal(t, "both", records[both].Location, "the bucket's copy is not known to be gone")
	}
	assert.Contains(t, records, pruned, "its local copy was pruned, the bucket's may still be there")
	assert.NotContains(t, records, gone, "a copy that was only on the node is gone")

	// a node that listed its bucket fine (or an older one) is taken at its word, as before
	_, err = applyNodeBackups(bs, server, []skypanel.BackupFile{
		{FileName: both, Size: 7, Location: "local"},
	}, true)
	require.NoError(t, err)
	records = byName()
	assert.NotContains(t, records, periodic)
	assert.NotContains(t, records, pruned)
	if assert.Contains(t, records, both) {
		assert.Equal(t, "local", records[both].Location)
	}
}

func TestNodeListedBucket_OnlyTheUnavailableMarkCounts(t *testing.T) {
	assert.True(t, nodeListedBucket(http.Header{}), "an older node never sets the header")
	header := http.Header{}
	header.Set(servers.BackupsOffsiteHeader, servers.BackupsOffsiteUnavailable)
	assert.False(t, nodeListedBucket(header))
}
