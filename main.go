// The herosync command is a utility for managing GoPro media files.
package main

import (
	"fmt"
	"os"

	"github.com/EarthmanMuons/herosync/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
