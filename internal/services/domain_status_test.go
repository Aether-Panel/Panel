package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func useConfigDir(t *testing.T) string {
	dir := t.TempDir()
	previous := viper.ConfigFileUsed()
	viper.SetConfigFile(filepath.Join(dir, "config.json"))
	t.Cleanup(func() { viper.SetConfigFile(previous) })
	return dir
}

func TestLocalNodeHost_ServerIPWhenTheDomainIsBehindAProxy(t *testing.T) {
	db := setupOtpTestDB(t)
	assert.NoError(t, db.AutoMigrate(&models.PanelSetting{}))
	dir := useConfigDir(t)
	status := `{"state":"ok","url":"https://panel.example.com","domain":"panel.example.com","https":true,"proxied":true,"serverIp":"203.0.113.7"}`
	assert.NoError(t, os.WriteFile(filepath.Join(dir, domainStatusFile), []byte(status), 0o644))
	previous := *models.LocalNode
	t.Cleanup(func() { *models.LocalNode = previous })

	(&Node{DB: db}).syncLocalNodeHost()

	assert.Equal(t, "203.0.113.7", models.LocalNode.PublicHost, "players cannot reach a game port through Cloudflare's proxy")
}

func TestLocalNodeHost_MasterNodeIPWins(t *testing.T) {
	db := setupOtpTestDB(t)
	assert.NoError(t, db.AutoMigrate(&models.PanelSetting{}))
	dir := useConfigDir(t)
	status := `{"state":"ok","proxied":true,"serverIp":"203.0.113.7"}`
	assert.NoError(t, os.WriteFile(filepath.Join(dir, domainStatusFile), []byte(status), 0o644))
	assert.NoError(t, db.Create(&models.PanelSetting{Key: "master_node_ip", Value: "198.51.100.9"}).Error)
	previous := *models.LocalNode
	t.Cleanup(func() { *models.LocalNode = previous })

	(&Node{DB: db}).syncLocalNodeHost()

	assert.Equal(t, "198.51.100.9", models.LocalNode.PublicHost)
}

func TestProxiedDomainServerIP_EmptyWithoutAProxy(t *testing.T) {
	dir := useConfigDir(t)
	assert.Empty(t, proxiedDomainServerIP(), "no status file")

	assert.NoError(t, os.WriteFile(filepath.Join(dir, domainStatusFile), []byte(`{"state":"ok","proxied":false,"serverIp":"203.0.113.7"}`), 0o644))
	assert.Empty(t, proxiedDomainServerIP())

	assert.NoError(t, os.WriteFile(filepath.Join(dir, domainStatusFile), []byte(`{"state":"ok","proxied":true,"serverIp":"not-an-ip"}`), 0o644))
	assert.Empty(t, proxiedDomainServerIP(), "only an IP is used")
}
