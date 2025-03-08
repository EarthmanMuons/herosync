package cmd

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/logging"
	"github.com/EarthmanMuons/herosync/internal/media"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List media files on connected GoPro",
	RunE:    runList,

	// only stored on gopro = File exists only on the GoPro
	// only stored on local = File exists only locally
	// file has been synced = File exists on both, with matching sizes
	// SIZES ARE MISMATCHED = File exists on both, but sizes differ
}

func init() {
	listCmd.Flags().String("gopro-host", "", "GoPro host (hostname:port or IP)")
	listCmd.Flags().String("gopro-scheme", "", "GoPro scheme (http/https)")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := getConfigWithFlags(cmd)
	if err != nil {
		return err
	}

	baseURL, err := cfg.GetGoProURL()
	if err != nil {
		return fmt.Errorf("failed to resolve GoPro connection: %v", err)
	}

	client := gopro.NewClient(baseURL, logging.GetLogger())

	inventory, err := media.NewMediaInventory(cmd.Context(), client, cfg.Output.Dir)
	if err != nil {
		return err
	}

	for _, file := range inventory.Files {
		createdAt := file.CreatedAt.Format(time.DateTime)
		humanSize := humanize.Bytes(uint64(file.Size))
		fmt.Printf("%s %-15s %s %8s %22s\n", file.Status.Symbol(), file.Filename, createdAt, humanSize, file.Status)
	}

	return nil
}
