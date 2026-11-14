package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// useBackupLimit sets panel.backups.maxPerServer for this test
func useBackupLimit(t *testing.T, limit int) {
	previous := config.BackupsMaxPerServer.Value()
	require.NoError(t, config.BackupsMaxPerServer.Set(limit, false))
	t.Cleanup(func() { _ = config.BackupsMaxPerServer.Set(previous, false) })
}

func TestCreateBackup_BucketOnlyCopiesDoNotCountTowardTheLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "backups.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Node{}, &models.Server{}, &models.Backup{}))
	useBackupLimit(t, 3)

	server := &models.Server{Identifier: "srv01"}
	bs := &services.Backup{DB: db}
	add := func(location string) {
		t.Helper()
		var count int64
		require.NoError(t, db.Model(&models.Backup{}).Count(&count).Error)
		name := fmt.Sprintf("b%d.tar.gz", count)
		require.NoError(t, bs.Create(&models.Backup{Name: "backup", FileName: name, Location: location, ServerID: server.Identifier}))
	}
	// periodic copies after their upload and task copies pruned on the node are only in the
	// bucket; two copies are on the node
	for _, location := range []string{"offsite", "offsite", "offsite", "offsite", "local", "both"} {
		add(location)
	}

	full, err := overBackupLimit(bs, server.Identifier, 3)
	require.NoError(t, err)
	assert.False(t, full, "only the two copies on the node count toward the cap of 3")

	// a record whose location is not known yet (one still being written) is on the node
	add("")
	full, err = overBackupLimit(bs, server.Identifier, 3)
	require.NoError(t, err)
	assert.True(t, full)

	var logs bytes.Buffer
	previous := logging.Error.Writer()
	logging.Error.SetOutput(&logs) // the refusal is logged; that is expected here
	defer logging.Error.SetOutput(previous)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/servers/srv01/backup/create?name=x", nil)
	c.Set("db", db)
	c.Set("server", server)
	createBackup(c)

	assert.Equal(t, http.StatusConflict, w.Code, "refused before the node is called")
	assert.Contains(t, w.Body.String(), "ErrBackupLimit")
}
