package main

import (
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of SkyPanel",
	Run:   executeVersion,
}

func executeVersion(_ *cobra.Command, _ []string) {
	fmt.Println(skypanel.Display)
}
