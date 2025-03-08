package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/logging"
)

var cleanupCmd = &cobra.Command{
	Use:     "cleanup",
	Aliases: []string{"clean"},
	Short:   "Delete transferred media from GoPro storage",
	RunE:    runCombine,
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	log := logging.GetLogger()

	// cfg, err := getConfigWithFlags(cmd)
	// if err != nil {
	// 	return err
	// }

	log.Error("UNIMPLEMENTED", "command", cmd.Use)
	os.Exit(1)

	return nil
}

