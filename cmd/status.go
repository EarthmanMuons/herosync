package cmd

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
)

// newStatusCmd constructs the "status" subcommand.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"st"},
		Short:   "Display GoPro hardware and storage info",
		RunE:    runStatus,
	}
}

// runStatus is the entry point for the "status" subcommand.
func runStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	logger, cfg, err := parseConfigAndLogger(cmd)
	if err != nil {
		return err
	}

	client, err := gopro.NewClient(logger, cfg.GoPro.Scheme, cfg.GoPro.Host)
	if err != nil {
		return err
	}

	hw, err := client.GetHardwareInfo(ctx)
	if err != nil {
		return err
	}

	cs, err := client.GetCameraState(ctx)
	if err != nil {
		return err
	}

	storageStatus := formatStorageStatus(cs.Status.SDCardCapacity, cs.Status.SDCardRemaining)

	fmt.Printf("Connected to GoPro %s at %s\n", hw.ModelName, client.BaseURL())
	fmt.Printf("Serial Number: %s\n", hw.SerialNumber)
	fmt.Printf("Firmware Version: %s\n", hw.FirmwareVersion)
	fmt.Printf("Storage: %s\n", storageStatus)

	return nil
}

func formatStorageStatus(capacityBytes, remainingBytes int64) string {
	if capacityBytes <= 0 {
		return "no storage detected"
	}

	usedBytes := capacityBytes - remainingBytes
	percentageFull := (float64(usedBytes) / float64(capacityBytes)) * 100.0
	humanRemaining := humanize.Bytes(uint64(remainingBytes))

	return fmt.Sprintf("%.1f%% full (%s free)", percentageFull, humanRemaining)
}
