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
