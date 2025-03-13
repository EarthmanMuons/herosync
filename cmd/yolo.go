package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// newYOLOCmd constructs the "yolo" subcommand.
func newYOLOCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "yolo",
		Aliases: []string{"merge"},
		Short:   "Hands-free sync: download, combine, publish",
		RunE:    runYOLO,
	}
	return cmd
}

// runYOLO is the entry point for the "yolo" subcommand.
func runYOLO(cmd *cobra.Command, args []string) error {
	// logger, cfg, err := parseConfigAndLogger(cmd)
	logger, _, err := parseConfigAndLogger(cmd)
	if err != nil {
		return err
	}

	logger.Error("UNIMPLEMENTED", "command", cmd.Use)
	os.Exit(1)

	return nil
}
