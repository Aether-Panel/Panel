package database

import (
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func findMigration(t *testing.T, id string) *gormigrate.Migration {
	for _, group := range migrations {
		for _, m := range group {
			if m.ID == id {
				return m
			}
		}
	}
	t.Fatalf("migration %s not found", id)
	return nil
}

// An install from before the extra-ports and internal-id migrations, with servers in it,
// must upgrade: the extra-ports step used to AutoMigrate today's Server model, which tried
// to add internal_id NOT NULL to the existing rows before its own migration filled it.
func TestServerMigrations_UpgradeAnOldServersTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "old.db")), &gorm.Config{})
	if !assert.NoError(t, err) {
		return
	}
	assert.NoError(t, db.Exec(`CREATE TABLE servers (
		identifier varchar(20) PRIMARY KEY, name varchar(40) NOT NULL, node_id integer,
		ip text, port integer, type text NOT NULL DEFAULT 'generic', icon text)`).Error)
	assert.NoError(t, db.Exec(`INSERT INTO servers (identifier, name, type) VALUES ('a1', 'one', 'generic'), ('b2', 'two', 'generic')`).Error)

	assert.NoError(t, findMigration(t, "20260819-server-extra-ports").Migrate(db))
	assert.NoError(t, findMigration(t, "20261016-server-internal-id").Migrate(db))

	m := db.Migrator()
	assert.True(t, m.HasColumn(&models.Server{}, "Ports"))
	var ids []string
	assert.NoError(t, db.Table("servers").Pluck("internal_id", &ids).Error)
	assert.Len(t, ids, 2)
	assert.NotEmpty(t, ids[0])
	assert.NotEqual(t, ids[0], ids[1], "every server gets its own internal id")
}
