package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/logging"
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
	cfg, err := getConfigWithFlags(cmd)
	if err != nil {
		return err
	}

	baseURL, err := cfg.GetGoProURL()
	if err != nil {
		return fmt.Errorf("failed to resolve GoPro connection: %v", err)
	}

	client := gopro.NewClient(baseURL, logging.Logger)

	hwInfo, err := client.GetHardwareInfo(cmd.Context())
	if err != nil {
		return err
	}

	fmt.Printf("Connected to GoPro %s at %s\n", hwInfo.ModelName, baseURL)
	fmt.Printf("Serial Number: %s\n", hwInfo.SerialNumber)
	fmt.Printf("Firmware Version: %s\n", hwInfo.FirmwareVersion)

	return nil
}
