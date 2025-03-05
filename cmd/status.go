package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "Show sync status and statistics",
	RunE:    runStatus,
}

func init() {
	statusCmd.Flags().String("gopro-host", "", "GoPro host (hostname:port or IP)")
	statusCmd.Flags().String("gopro-scheme", "", "GoPro scheme (http/https)")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := applyLocalFlags(cmd)
	if err != nil {
		return err
	}

	url, err := cfg.GetGoPro()
	if err != nil {
		return fmt.Errorf("failed to resolve GoPro connection: %v", err)
	}

	fmt.Printf("GoPro base URL: %s\n", url)
	return nil
}
