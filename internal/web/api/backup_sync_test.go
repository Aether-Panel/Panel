package api

import (
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
	})
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
	})
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
	})
	require.NoError(t, err)
	records, err = bs.GetAllForServer(server.Identifier)
	require.NoError(t, err)
	assert.Len(t, records, 1)

	// one seen while its archive was still on the node follows it into the bucket
	next := servers.OffsiteBackupPrefix + "7e2d4f60-1a3b-4c5d-8e9f-0a1b2c3d4e5f.tar.gz"
	_, err = applyNodeBackups(bs, server, []skypanel.BackupFile{
		{FileName: periodic, Size: 42, Location: "offsite"},
		{FileName: next, Size: 50, Location: "local"},
	})
	require.NoError(t, err)
	_, err = applyNodeBackups(bs, server, []skypanel.BackupFile{
		{FileName: periodic, Size: 42, Location: "offsite"},
		{FileName: next, Size: 50, Location: "offsite"},
	})
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
	_, err = applyNodeBackups(bs, server, []skypanel.BackupFile{})
	require.NoError(t, err)
	records, err = bs.GetAllForServer(server.Identifier)
	require.NoError(t, err)
	assert.Empty(t, records)
}
