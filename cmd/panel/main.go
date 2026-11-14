package main

import (
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cobra"
	"os"
	"runtime/debug"
	// the image ships without a zoneinfo database, and scheduled tasks run in the time zone
	// of the person who created them (CRON_TZ=...)
	_ "time/tzdata"
)

func main() {
	// a failed command (unknown, bad flags, an error it returned) exits non-zero, so the
	// Docker entrypoint and the installer can tell it did not work
	if err := runCLI(); err != nil {
		os.Exit(1)
	}
}

func runCLI() error {
	/*if !groups.IsUserIn(groups.SkyPanelGroup) {
		fmt.Println("You do not have permission to use this command")
		return
	}*/

	defer logging.Close()

	defer func() {
		if err := recover(); err != nil {
			stacktrace := debug.Stack()
			logging.Error.Printf("%s\n%s\n", err, stacktrace)

			os.Exit(2)
		}
	}()

	return Execute()
}

var rootCmd = &cobra.Command{
	Use:   "SkyPanel",
	Short: "Game Server Management Panel",
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

var configFile string
var workDir string

func init() {
	rootCmd.PersistentFlags().StringVar(&workDir, "workDir", "", "Set working directory")
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Location of config")

	cobra.OnInitialize(setWorkDir, loadConfig)

	rootCmd.AddCommand(
		runCmd,
		versionCmd,
		userCmd,
		runServiceCmd,
		dbCmd,
	)
}

func setWorkDir() {
	if workDir != "" {
		err := os.Chdir(workDir)
		if err != nil {
			panic(err)
		}
	}
}

func loadConfig() {
	err := config.LoadConfigFile(configFile)
	if err != nil {
		fmt.Printf("Error loading config, this may impact features:\n%s\n", err.Error())
	}
}

// Execute runs the command line. Cobra already prints the error (as "Error: ..."), so it is
// only returned here, for the exit code.
func Execute() error {
	rootCmd.SetVersionTemplate(skypanel.Display)

	return rootCmd.Execute()
}
