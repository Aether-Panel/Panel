package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var dbUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Runs the database upgrades",
	Run:   executeDbUpgrade,
}

func executeDbUpgrade(_ *cobra.Command, _ []string) {
	var currentFile string
	var backupFile string

	if !config.PanelEnabled.Value() {
		pterm.Info.Printfln("Panel not enabled, skipping upgrade")
		os.Exit(0)
	}

	if database.GetDialect() == "sqlite3" {
		connStr := database.GetConnectionString()
		// extract file path from "file:path?params" format
		currentFile = strings.TrimPrefix(connStr, "file:")
		if idx := strings.Index(currentFile, "?"); idx != -1 {
			currentFile = currentFile[:idx]
		}
		currentFile, _ = url.PathUnescape(currentFile)
		if currentFile == "" {
			currentFile = "skypanel.db"
		}
		abs, err := filepath.Abs(currentFile)
		if err == nil {
			currentFile = abs
		}

		// look for a new name we can give this....
		suffix := "backup"
		num := 0
		for {
			backupFile = fmt.Sprintf("%s.%d.%s", currentFile, num, suffix)
			fi, err := os.Lstat(backupFile)
			if os.IsNotExist(err) && fi == nil {
				break
			}
			num++
		}

		err = copyDatabaseFile(currentFile, backupFile)
		if err != nil {
			pterm.Error.Printfln("error backing up database: %s", err.Error())
			os.Exit(1)
			return
		}
	}

	db, err := database.GetConnection()
	if err != nil {
		pterm.Error.Printfln("error connecting to database: %s", err.Error())
		rollback(backupFile, currentFile)
		os.Exit(1)
		return
	}

	pterm.Info.Printfln("Starting database upgrade")
	err = database.Upgrade(db, true)
	if err != nil {
		pterm.Error.Printfln("Database upgrade failed: %s", err.Error())
		rollback(backupFile, currentFile)
		os.Exit(1)
		return
	}
}

func rollback(backup, overrideTo string) {
	if backup == "" || overrideTo == "" {
		return
	}
	err := copyDatabaseFile(backup, overrideTo)
	if err != nil {
		pterm.Error.Printfln("error restoring database: %s", err.Error())
		return
	}
}

func copyDatabaseFile(src, dest string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer utils.Close(source)

	err = os.MkdirAll(filepath.Dir(dest), 0750)
	if err != nil {
		return err
	}
	destination, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer utils.Close(destination)
	_, err = io.Copy(destination, source)
	return err
}
