package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/logging"
)

var yoloCmd = &cobra.Command{
	Use:     "yolo",
	Aliases: []string{"merge"},
	Short:   "Hands-free: download, combine, publish, cleanup",
	RunE:    runYolo,
}

func init() {
	yoloCmd.Flags().String("gopro-host", "", "GoPro host (hostname:port or IP)")
	yoloCmd.Flags().String("gopro-scheme", "", "GoPro scheme (http/https)")
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
