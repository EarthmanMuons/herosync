package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/config"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status and statistics",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	fmt.Printf("GoPro base URL: %s://%s\n", cfg.Camera.Protocol, cfg.Camera.IP)

	return nil
}
