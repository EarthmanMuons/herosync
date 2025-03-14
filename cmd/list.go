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
	ctx, logger, cfg, err := contextLoggerConfig(cmd)
	if err != nil {
		return err
	}

	client, err := gopro.NewClient(logger, cfg.GoPro.Scheme, cfg.GoPro.Host)
	if err != nil {
		return err
	}

	incomingDir := cfg.IncomingMediaDir()

	inventory, err := media.NewInventory(ctx, client, incomingDir)
	if err != nil {
		return err
	}
	inventory, err = inventory.FilterByFilename(args)
	if err != nil {
		return err
	}

	printInventory(inventory)

	return nil
}

// printInventory prints the inventory in a human-readable format.
func printInventory(inventory *media.Inventory) {
	for _, file := range inventory.Files {
		createdAt := file.CreatedAt.Format(time.DateTime)
		humanSize := humanize.Bytes(uint64(file.Size))
		fmt.Printf("%s %-15s %s %8s %22s\n", file.Status.Symbol(), file.Filename, createdAt, humanSize, file.Status)
	}
}
