package services

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupNodeTestDB(t *testing.T) *gorm.DB {
	dbPath := filepath.Join(t.TempDir(), "node_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = db.AutoMigrate(&models.Node{}, &models.Server{})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	return db
}

func sslTestNode() *models.Node {
	// TEST-NET-3 address: guaranteed unreachable, so any accidental probe fails
	return &models.Node{
		ID:          7,
		Name:        "ssl-test-node",
		PublicHost:  "203.0.113.9",
		PrivateHost: "203.0.113.9",
		PublicPort:  8080,
		PrivatePort: 8080,
		SFTPPort:    5657,
		Secret:      "ssl-test-secret",
		SSL:         true,
	}
}

// A node that was ever seen speaking TLS must stay on TLS: blocking the TLS
// handshake must not make the panel silently resend its token over plain http.
func TestDoesDaemonUseSSL_PinnedNodeNeverDowngrades(t *testing.T) {
	ns := &Node{DB: setupNodeTestDB(t)}
	node := sslTestNode()

	// the probe cannot return true for an unreachable host, so a (true, nil)
	// answer proves the pin short-circuits any probing/downgrade
	ssl, err := ns.doesDaemonUseSSL(node)
	assert.NoError(t, err)
	assert.True(t, ssl)
}

func TestCreateNodeURL_PinnedNodeIsHTTPS(t *testing.T) {
	ns := &Node{DB: setupNodeTestDB(t)}
	node := sslTestNode()

	u, err := ns.createNodeURL(node, "/daemon/server/abc/status")
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(u, "https://"), "expected https url, got %s", u)
}

func TestPinNodeSSL_PersistsThePin(t *testing.T) {
	db := setupNodeTestDB(t)
	ns := &Node{DB: db}

	node := sslTestNode()
	node.SSL = false
	if !assert.NoError(t, db.Create(node).Error) {
		t.FailNow()
	}

	ns.pinNodeSSL(node)
	assert.True(t, node.SSL)

	var reloaded models.Node
	assert.NoError(t, db.First(&reloaded, node.ID).Error)
	assert.True(t, reloaded.SSL, "the pin must survive a panel restart")
}
