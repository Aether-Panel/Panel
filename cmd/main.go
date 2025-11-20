package main

import (
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/config"
	"github.com/SkyPanel/SkyPanel/v3/logging"
	"github.com/spf13/cobra"
	"os"
	"runtime/debug"
)

func main() {
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

	Execute()
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

func Execute() {
	rootCmd.SetVersionTemplate(SkyPanel.Display)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}
