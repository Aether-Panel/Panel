package main

import (
	"fmt"

	"github.com/SkyPanel/SkyPanel/v3/internal/shared/config"
)

func main() {
	err := config.LoadConfigFile("config.json")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	fmt.Printf("DataRootFolder: %s\n", config.DataRootFolder.Value())
	fmt.Printf("BinariesFolder: %s\n", config.BinariesFolder.Value())
}
