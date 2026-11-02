package main

import (
	"fmt"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// dbMigrateCmd brings the database schema up to date, the same step "run" takes before it
// starts. The Docker entrypoint runs it first, so a failed migration is reported on its own,
// with a non-zero exit code, before the panel starts.
var dbMigrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Applies the pending database migrations",
	RunE:         executeDbMigrate,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
}

func executeDbMigrate(_ *cobra.Command, _ []string) error {
	// only the panel has a database; a node that just runs the daemon has nothing to migrate
	if !config.PanelEnabled.Value() {
		pterm.Info.Println("Panel not enabled, skipping migrations")
		return nil
	}

	db, err := database.GetConnection()
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	defer database.Close()

	if err = migrateDatabase(db); err != nil {
		return err
	}
	pterm.Success.Println("Database migrations complete")
	return nil
}

// migrateDatabase applies the migrations exactly as "run" does
func migrateDatabase(db *gorm.DB) error {
	if err := database.Upgrade(db, false); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}
	return nil
}
