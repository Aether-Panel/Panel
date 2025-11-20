package main

import (
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of SkyPanel",
	Run:   executeVersion,
}

func executeVersion(cmd *cobra.Command, args []string) {
	fmt.Println(SkyPanel.Display)
}
