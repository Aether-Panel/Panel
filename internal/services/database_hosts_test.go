package services

import (
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDatabaseHostTest(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "dbhosts.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Node{}, &models.Server{}, &models.DatabaseHost{}, &models.Database{}))
	return db
}

func TestUsableHosts_RespectsNodeAndMaxDatabases(t *testing.T) {
	db := setupDatabaseHostTest(t)
	node := &models.Node{Name: "remote", PublicHost: "10.0.0.2", PrivateHost: "10.0.0.2", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "s3cr3t"}
	require.NoError(t, db.Create(node).Error)
	require.NoError(t, db.Create(&models.Server{Name: "s", Identifier: "srv01", Type: "generic", NodeID: node.ID}).Error)

	otherNode := node.ID + 1
	hosts := []*models.DatabaseHost{
		{Name: "any node", Host: "db1.example.com", Port: 3306, Username: "u", Password: "p"},
		{Name: "this node", Host: "db2.example.com", Port: 3306, Username: "u", Password: "p", NodeID: &node.ID},
		{Name: "other node", Host: "db3.example.com", Port: 3306, Username: "u", Password: "p", NodeID: &otherNode},
		{Name: "full", Host: "db4.example.com", Port: 3306, Username: "u", Password: "p", MaxDatabases: 1},
	}
	for _, h := range hosts {
		require.NoError(t, db.Create(h).Error)
	}
	require.NoError(t, db.Create(&models.Database{ServerID: "srv01", DatabaseHostID: hosts[3].ID, DatabaseName: "s_srv01_a", Username: "db_x", Password: "y"}).Error)

	ds := &Database{DB: db}
	usable, err := ds.UsableHosts("srv01")
	require.NoError(t, err)
	names := make([]string, 0, len(usable))
	for _, h := range usable {
		names = append(names, h.Name)
	}
	assert.ElementsMatch(t, []string{"any node", "this node"}, names)

	assert.ErrorIs(t, ds.checkHostAccepts(hosts[2], "srv01"), skypanel.ErrDatabaseHostOtherNode)
	assert.ErrorIs(t, ds.checkHostAccepts(hosts[3], "srv01"), skypanel.ErrDatabaseHostFull)
}

// ?node= used struct conditions on a field that is not a column, so it returned every server
func TestServerSearch_FiltersByNode(t *testing.T) {
	db := setupDatabaseHostTest(t)
	node := &models.Node{Name: "remote", PublicHost: "10.0.0.2", PrivateHost: "10.0.0.2", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "s3cr3t"}
	require.NoError(t, db.Create(node).Error)
	require.NoError(t, db.Create(&models.Server{Name: "remote one", Identifier: "remote01", Type: "generic", NodeID: node.ID}).Error)
	require.NoError(t, db.Create(&models.Server{Name: "local one", Identifier: "local01", Type: "generic"}).Error)

	ss := &Server{DB: db}
	remote, total, err := ss.Search(ServerSearch{NodeID: node.ID, PageSize: 10, Page: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "remote01", remote[0].Identifier)

	local, total, err := ss.Search(ServerSearch{NodeName: "LocalNode", PageSize: 10, Page: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "local01", local[0].Identifier)
}
