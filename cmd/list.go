package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/media"
)

var listCmd = &cobra.Command{
	Use:     "list [FILENAME]...",
	Aliases: []string{"ls"},
	Short:   "Show media inventory and sync state details",
	Args:    cobra.ArbitraryArgs,
	RunE:    runList,

	// only stored on gopro = File exists only on the GoPro
	// only stored on local = File exists only locally
	// file has been synced = File exists on both, with matching sizes
	// SIZES ARE MISMATCHED = File exists on both, but sizes differ
}

func runList(cmd *cobra.Command, args []string) error {
	logger := slog.Default()

	cfg, err := getConfigWithFlags(cmd)
	if err != nil {
		return err
	}

	baseURL, err := cfg.GoProURL()
	if err != nil {
		return fmt.Errorf("failed to resolve GoPro connection: %v", err)
	}

	client := gopro.NewClient(baseURL, logger)

	inventory, err := media.NewInventory(cmd.Context(), client, cfg.RawMediaDir())
	if err != nil {
		return err
	}

	// Apply filename filtering if any were provided.
	if len(args) > 0 {
		logger.Debug("filtering by filename", slog.Any("args", args))
		inventory = inventory.FilterByFilename(args)

		if len(inventory.Files) == 0 {
			logger.Error("no matching files", slog.Any("args", args))
			os.Exit(1)
		}
	}

	for _, file := range inventory.Files {
		createdAt := file.CreatedAt.Format(time.DateTime)
		humanSize := humanize.Bytes(uint64(file.Size))
		fmt.Printf("%s %-15s %s %8s %22s\n", file.Status.Symbol(), file.Filename, createdAt, humanSize, file.Status)
	}

	return nil
}
