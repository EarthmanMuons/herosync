package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/logging"
)

var yoloCmd = &cobra.Command{
	Use:     "yolo",
	Aliases: []string{"merge"},
	Short:   "Perform a full sync (download -> process -> upload)",
	RunE:    runYolo,
}

func init() {
	rootCmd.AddCommand(yoloCmd)
}

func runYolo(cmd *cobra.Command, args []string) error {
	log := logging.GetLogger()

	// cfg, err := getConfigWithFlags(cmd)
	// if err != nil {
	// 	return err
	// }

	log.Error("UNIMPLEMENTED", "command", cmd.Use)
	os.Exit(1)

	return nil
}
