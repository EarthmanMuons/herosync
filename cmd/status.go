package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status and statistics",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().String("camera-ip", "", "override camera IP address")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := applyLocalFlags(cmd)
	if err != nil {
		return err
	}

	fmt.Printf("GoPro base URL: %s://%s\n", cfg.Camera.Protocol, cfg.Camera.IP)
	return nil
}
