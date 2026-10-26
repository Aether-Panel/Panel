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

// Servers from before owners existed get the member with the most scopes as their owner, so
// customers who ran their server through the "Usuario" role keep running it.
func TestServerOwnersBackfill(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "owners.db")), &gorm.Config{})
	if !assert.NoError(t, err) {
		return
	}
	assert.NoError(t, db.Exec(`CREATE TABLE servers (identifier varchar(20) PRIMARY KEY, name varchar(40))`).Error)
	assert.NoError(t, db.Exec(`CREATE TABLE permissions (id integer PRIMARY KEY, user_id integer, client_id integer, server_identifier varchar(20), scopes varchar(1000))`).Error)
	assert.NoError(t, db.Exec(`INSERT INTO servers (identifier, name) VALUES ('paid', 'p'), ('shared', 's'), ('tie', 't'), ('empty', 'e')`).Error)
	assert.NoError(t, db.Exec(`INSERT INTO permissions (id, user_id, server_identifier, scopes) VALUES
		(1, 7, 'paid', 'server.view'),
		(2, 3, 'paid', 'server.view,server.start,server.console'),
		(3, 4, 'shared', 'server.view,server.files.view'),
		(4, 5, 'shared', 'server.view'),
		(5, 8, 'tie', 'server.view'),
		(6, 9, 'tie', 'server.view'),
		(7, 1, NULL, 'admin')`).Error)

	assert.NoError(t, findMigration(t, "20261026-server-owners").Migrate(db))

	owners := map[string]*uint{}
	var rows []struct {
		Identifier string
		OwnerID    *uint
	}
	assert.NoError(t, db.Table("servers").Select("identifier, owner_id").Scan(&rows).Error)
	for _, r := range rows {
		owners[r.Identifier] = r.OwnerID
	}
	want := map[string]uint{"paid": 3, "shared": 4, "tie": 8}
	for server, user := range want {
		if assert.NotNil(t, owners[server], server) {
			assert.Equal(t, user, *owners[server], server)
		}
	}
	assert.Nil(t, owners["empty"], "a server nobody belongs to has no owner")
}
