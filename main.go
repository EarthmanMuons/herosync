// The herosync command is a utility for managing GoPro media files.
package main

import (
	"os"

	"github.com/EarthmanMuons/herosync/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
