package cmd

import (
	"fmt"

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
	outgoingDir := cfg.OutgoingMediaDir()

	inventory, err := media.NewInventory(ctx, client, incomingDir, outgoingDir)
	if err != nil {
		return err
	}
	inventory, err = inventory.FilterByDisplayInfo(args)
	if err != nil {
		return err
	}

	for _, file := range inventory.Files {
		fmt.Println(file)
	}

	return nil
}
