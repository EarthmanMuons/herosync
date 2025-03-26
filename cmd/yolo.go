package cmd

import (
	"fmt"
	"log/slog"

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
	logger := slog.Default()

	commands := []struct {
		name string
		fn   func(*cobra.Command, []string) error
	}{
		{"download", runDownload},
		{"combine", runCombine},
		{"publish", runPublish},
	}

	for _, c := range commands {
		logger.Info("YOLO running", slog.String("subcommand", c.name))
		err := c.fn(cmd, args)
		if err != nil {
			return fmt.Errorf("%s subcommand failed: %w", c.name, err)
		}
	}

	return nil
}
