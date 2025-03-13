package cmd

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/media"
)

// newListCmd constructs the "list" subcommand.
func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list [FILENAME]...",
		Aliases: []string{"ls"},
		Short:   "Show media inventory and sync state details",
		Args:    cobra.ArbitraryArgs,
		RunE:    runList,
	}
}

// runList is the entry point for the "list" subcommand.
func runList(cmd *cobra.Command, args []string) error {
	logger, cfg, err := parseConfigAndLogger(cmd)
	if err != nil {
		return err
	}

	client, err := gopro.NewClient(logger, cfg.GoPro.Scheme, cfg.GoPro.Host)
	if err != nil {
		return fmt.Errorf("failed to initialize GoPro client: %w", err)
	}

	inventory, err := media.NewInventory(cmd.Context(), client, cfg.OriginalMediaDir())
	if err != nil {
		return err
	}

	// Apply filename filtering if any were provided.
	inventory, err = inventory.FilterByFilename(args)
	if err != nil {
		return err
	}

	printInventory(inventory)

	return nil
}

// printInventory outputs the inventory in a human-readable format.
func printInventory(inventory *media.Inventory) {
	for _, file := range inventory.Files {
		createdAt := file.CreatedAt.Format(time.DateTime)
		humanSize := humanize.Bytes(uint64(file.Size))
		fmt.Printf("%s %-15s %s %8s %22s\n", file.Status.Symbol(), file.Filename, createdAt, humanSize, file.Status)
	}
}
