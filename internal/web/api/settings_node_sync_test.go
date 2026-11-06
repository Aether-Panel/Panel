package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// countNodePasses stands in for the node settings sync and counts how often a save asked for a
// pass
func countNodePasses(t *testing.T) *atomic.Int32 {
	var asked atomic.Int32
	previous := requestNodeSettingsSync
	requestNodeSettingsSync = func() { asked.Add(1) }
	t.Cleanup(func() { requestNodeSettingsSync = previous })
	return &asked
}

// keepStringSetting puts the entry back as it was after the test
func keepStringSetting(t *testing.T, entry config.StringEntry) {
	previous := entry.Value()
	t.Cleanup(func() { _ = entry.Set(previous, false) })
}

func settingsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/settings/:key", setSetting)
	r.POST("/settings", setSettings)
	return r
}

func TestSetSetting_ANodeSettingAsksForANodePass(t *testing.T) {
	useScratchConfig(t)
	keepStringSetting(t, config.BackupsS3Prefix)
	asked := countNodePasses(t)

	w := doJSON(settingsRouter(), http.MethodPut, "/settings/"+config.BackupsS3Prefix.Key(), map[string]interface{}{"value": "backups/"})

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, int32(1), asked.Load())
}

func TestSetSetting_APanelOnlySettingAsksForNoNodePass(t *testing.T) {
	useScratchConfig(t)
	keepStringSetting(t, config.CompanyName)
	asked := countNodePasses(t)

	w := doJSON(settingsRouter(), http.MethodPut, "/settings/"+config.CompanyName.Key(), map[string]interface{}{"value": "Acme"})

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Zero(t, asked.Load())
}

func TestSetSettings_NodeSettingsAskForOneNodePass(t *testing.T) {
	useScratchConfig(t)
	keepStringSetting(t, config.BackupsS3Bucket)
	keepStringSetting(t, config.CurseForgeKey)
	keepStringSetting(t, config.CompanyName)
	asked := countNodePasses(t)

	w := doJSON(settingsRouter(), http.MethodPost, "/settings", map[string]interface{}{
		config.BackupsS3Bucket.Key(): "bucket",
		config.CurseForgeKey.Key():   "cf-key",
		config.CompanyName.Key():     "Acme",
	})
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, int32(1), asked.Load(), "one pass for the whole save")

	w = doJSON(settingsRouter(), http.MethodPost, "/settings", map[string]interface{}{config.CompanyName.Key(): "Acme 2"})
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, int32(1), asked.Load(), "panel-only settings ask for none")
}

// The page sends a secret's mask back when it was not changed: nothing changed for the nodes
func TestSetSetting_AnUnchangedSecretAsksForNoNodePass(t *testing.T) {
	useScratchConfig(t)
	asked := countNodePasses(t)

	w := doJSON(settingsRouter(), http.MethodPut, "/settings/"+config.BackupsS3SecretKey.Key(), map[string]interface{}{"value": secretMask})

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Zero(t, asked.Load())
}

func TestNodeSync_ListsEveryRemoteNodeWithoutSettingValues(t *testing.T) {
	keepStringSetting(t, config.BackupsS3SecretKey)
	keepStringSetting(t, config.CurseForgeKey)
	require.NoError(t, config.BackupsS3SecretKey.Set("s3-secret-that-must-not-leak", false))
	require.NoError(t, config.CurseForgeKey.Set("cf-key-that-must-not-leak", false))

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "nodesync.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Node{}))
	for _, name := range []string{"alpha", "beta"} {
		node := &models.Node{Name: name, PublicHost: "192.0.2.1", PrivateHost: "192.0.2.1", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "node-secret-0123456789"}
		require.NoError(t, db.Create(node).Error)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db) })
	r.GET("/settings/node-sync", getNodeSettingsSync)
	w := doJSON(r, http.MethodGet, "/settings/node-sync", nil)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "must-not-leak")
	var answer []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
	require.Len(t, answer, 2)
	for i, name := range []string{"alpha", "beta"} {
		assert.Equal(t, name, answer[i]["name"])
		assert.Equal(t, "pending", answer[i]["state"], "not checked yet")
		assert.Contains(t, answer[i], "id")
		assert.Contains(t, answer[i], "checkedAt")
	}
}

func TestNodeSync_NoRemoteNodesIsAnEmptyList(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "nodesync.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Node{}))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db) })
	r.GET("/settings/node-sync", getNodeSettingsSync)
	w := doJSON(r, http.MethodGet, "/settings/node-sync", nil)

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `[]`, w.Body.String())
}
