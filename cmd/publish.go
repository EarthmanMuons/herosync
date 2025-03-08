package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/logging"
)

var publishCmd = &cobra.Command{
	Use:     "publish",
	Aliases: []string{"pub"},
	Short:   "Upload final videos to YouTube",
	RunE:    runCombine,
}

func init() {
	rootCmd.AddCommand(publishCmd)
}

func runPublish(cmd *cobra.Command, args []string) error {
	log := logging.GetLogger()

	// cfg, err := getConfigWithFlags(cmd)
	// if err != nil {
	// 	return err
	// }

	log.Error("UNIMPLEMENTED", "command", cmd.Use)
	os.Exit(1)

	return nil
}

