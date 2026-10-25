package services

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildDatabaseName_PrefixesWithServer(t *testing.T) {
	name, err := buildDatabaseName("abc123", "shop")
	assert.NoError(t, err)
	assert.Equal(t, "s_abc123_shop", name)

	// a reserved-looking request is harmless once namespaced under the server
	name, err = buildDatabaseName("abc123", "mysql")
	assert.NoError(t, err)
	assert.Equal(t, "s_abc123_mysql", name)
}

func TestBuildDatabaseName_RejectsInvalidInput(t *testing.T) {
	_, err := buildDatabaseName("abc123", "shop;DROP DATABASE mysql")
	assert.Error(t, err)

	_, err = buildDatabaseName("abc123", "")
	assert.Error(t, err)

	_, err = buildDatabaseName("", "shop")
	assert.Error(t, err)

	// MySQL caps schema names at 64 characters
	_, err = buildDatabaseName("12345678901234567890", strings.Repeat("a", 60))
	assert.Error(t, err)
}

func TestCreateDatabaseStatement_FailsOnExistingSchemas(t *testing.T) {
	stmt := createDatabaseStatement("s_abc123_shop")
	assert.Contains(t, stmt, "CREATE DATABASE `s_abc123_shop`")
	// colliding with a schema the panel does not track must fail loudly instead of
	// granting the caller privileges over someone else's database
	assert.NotContains(t, stmt, "IF NOT EXISTS")
}

func TestEnsureDatabaseNameAvailable_RefusesNamesTakenOnHost(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "dbname_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	if !assert.NoError(t, db.AutoMigrate(&models.Database{}, &models.DatabaseHost{}, &models.Server{}, &models.Node{}, &models.User{})) {
		t.FailNow()
	}

	existing := &models.Database{
		ServerID:       "other01",
		DatabaseHostID: 1,
		DatabaseName:   "s_other01_shop",
		Username:       "db_existing",
		Password:       "secretpw",
	}
	if !assert.NoError(t, db.Create(existing).Error) {
		t.FailNow()
	}

	ds := &Database{DB: db}

	// taken on the same host, even though another server owns it
	err = ds.ensureDatabaseNameAvailable(1, "s_other01_shop")
	assert.ErrorIs(t, err, skypanel.ErrDatabaseNameTaken)

	assert.NoError(t, ds.ensureDatabaseNameAvailable(1, "s_mine01_shop"))
	// same name on a different host is fine
	assert.NoError(t, ds.ensureDatabaseNameAvailable(2, "s_other01_shop"))
}
