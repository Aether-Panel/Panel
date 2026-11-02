package main

import (
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// A command that does not exist has to fail, so scripts see a non-zero exit code.
func TestExecute_UnknownCommandFails(t *testing.T) {
	rootCmd.SetArgs([]string{"no-such-command"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	assert.Error(t, Execute())
}

// "db migrate" is what the Docker entrypoint runs before the panel: it has to apply the
// migrations, and running it again on an up to date database does nothing.
func TestDbMigrate_AppliesTheMigrations(t *testing.T) {
	require.NotNil(t, dbMigrateCmd.RunE, "db migrate must do something")

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "panel.db")), &gorm.Config{
		// as the panel opens SQLite (internal/database/loader.go)
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	require.NoError(t, migrateDatabase(db))
	assert.True(t, db.Migrator().HasTable(&models.User{}))
	assert.True(t, db.Migrator().HasTable("migrations"))

	assert.NoError(t, migrateDatabase(db))
}

// "user edit" applies the API's rules: a rejected name or address is not saved and the user
// keeps the old one. The database is never reached, so the service has none.
func TestChangeUsername_RejectsWhatTheAPIRejects(t *testing.T) {
	for _, value := range []string{"", "bad\x01name", "ñandú"} {
		user := &models.User{Username: "old"}
		assert.Error(t, changeUsername(user, &services.User{}, value), value)
		assert.Equal(t, "old", user.Username)
	}
}

func TestChangeEmail_RejectsWhatTheAPIRejects(t *testing.T) {
	for _, value := range []string{"", "not-an-email", "a@"} {
		user := &models.User{Email: "old@example.com"}
		assert.Error(t, changeEmail(user, &services.User{}, value), value)
		assert.Equal(t, "old@example.com", user.Email)
	}
}

func TestValidateUserFields_AcceptValidValues(t *testing.T) {
	assert.NoError(t, validateUsername("admin"))
	assert.NoError(t, validateEmail("admin@example.com"))
}
