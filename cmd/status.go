package cmd

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/logging"
)

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "Display GoPro configuration, storage usage, and sync summary",
	RunE:    runStatus,
}

func init() {
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

	client := gopro.NewClient(baseURL, logging.GetLogger())

	hw, err := client.GetHardwareInfo(cmd.Context())
	if err != nil {
		return err
	}

	cs, err := client.GetCameraState(cmd.Context())
	if err != nil {
		return err
	}
	storageStatus := formatStorageStatus(cs.Status.SDCardCapacity, cs.Status.SDCardRemaining)

	fmt.Printf("Connected to GoPro %s at %s\n", hw.ModelName, baseURL)
	fmt.Printf("Serial Number: %s\n", hw.SerialNumber)
	fmt.Printf("Firmware Version: %s\n", hw.FirmwareVersion)
    fmt.Printf("Storage: %s\n", storageStatus)

	return nil
}

func formatStorageStatus(capacityBytes, remainingBytes int64) string {
	usedBytes := capacityBytes - remainingBytes

    // Handle division by zero by returning early with capacity = 0.
    if capacityBytes == 0 {
		return "0% full (0 B free)"
	}

	percentageFull := (float64(usedBytes) / float64(capacityBytes)) * 100.0
	humanRemaining := humanize.Bytes(uint64(remainingBytes))

	return fmt.Sprintf("%.1f%% full (%s free)", percentageFull, humanRemaining)
}
